package config_sync

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/eru-tech/eru/eru-gateway/registry"
	eru_models "github.com/eru-tech/eru/eru-models"
)

type instanceServer struct {
	*httptest.Server
	updatedAt atomic.Value
	loads     atomic.Int32
	statuses  atomic.Int32
	failLoad  bool
}

func newInstanceServer(service, id string, updatedAt time.Time) *instanceServer {
	is := &instanceServer{}
	is.updatedAt.Store(updatedAt)
	is.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/store/load":
			is.loads.Add(1)
			if is.failLoad {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusOK)
		case "/config/status":
			is.statuses.Add(1)
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"service":          service,
				"instance_id":      id,
				"config_update_at": is.updatedAt.Load().(time.Time),
			})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	return is
}

func registerAll(t *testing.T, reg *registry.Registry, entries map[string]*instanceServer, service string) {
	t.Helper()
	for id, srv := range entries {
		if err := reg.Register(t.Context(), eru_models.ServiceInstance{Id: id, Name: service, Address: srv.URL}); err != nil {
			t.Fatal(err)
		}
	}
}

func findService(report Report, name string) *ServiceStatus {
	for i := range report.Services {
		if report.Services[i].Service == name {
			return &report.Services[i]
		}
	}
	return nil
}

func TestStatusReportsInSyncWhenTimesMatch(t *testing.T) {
	at := time.Now().UTC().Truncate(time.Second)
	a := newInstanceServer("eru-ql", "ql-1", at)
	defer a.Close()
	b := newInstanceServer("eru-ql", "ql-2", at)
	defer b.Close()

	reg := newTestRegistry(t)
	registerAll(t, reg, map[string]*instanceServer{"ql-1": a, "ql-2": b}, "eru-ql")

	report, err := New(reg).Status(t.Context(), "eru-ql")
	if err != nil {
		t.Fatal(err)
	}
	if !report.InSync {
		t.Errorf("expected in_sync, got %+v", report)
	}
	svc := findService(report, "eru-ql")
	if svc == nil || len(svc.Instances) != 2 {
		t.Fatalf("expected 2 instances, got %+v", svc)
	}
	if a.loads.Load() != 0 || b.loads.Load() != 0 {
		t.Error("status must not reload any instance")
	}
}

func TestStatusReportsDriftWhenTimesDiffer(t *testing.T) {
	a := newInstanceServer("eru-ql", "ql-1", time.Now().UTC().Truncate(time.Second))
	defer a.Close()
	b := newInstanceServer("eru-ql", "ql-2", time.Now().UTC().Add(-time.Hour).Truncate(time.Second))
	defer b.Close()

	reg := newTestRegistry(t)
	registerAll(t, reg, map[string]*instanceServer{"ql-1": a, "ql-2": b}, "eru-ql")

	report, err := New(reg).Status(t.Context(), "eru-ql")
	if err != nil {
		t.Fatal(err)
	}
	if report.InSync {
		t.Error("expected drift to be reported as out of sync")
	}
	if svc := findService(report, "eru-ql"); svc == nil || svc.InSync {
		t.Error("expected the service itself to be marked out of sync")
	}
}

func TestStatusCoversAllServicesWhenNameOmitted(t *testing.T) {
	at := time.Now().UTC().Truncate(time.Second)
	ql := newInstanceServer("eru-ql", "ql-1", at)
	defer ql.Close()
	fn := newInstanceServer("eru-functions", "fn-1", at)
	defer fn.Close()

	reg := newTestRegistry(t)
	registerAll(t, reg, map[string]*instanceServer{"ql-1": ql}, "eru-ql")
	registerAll(t, reg, map[string]*instanceServer{"fn-1": fn}, "eru-functions")

	report, err := New(reg).Status(t.Context(), "")
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Services) != 2 {
		t.Fatalf("expected 2 services, got %d", len(report.Services))
	}
	if report.Services[0].Service != "eru-functions" || report.Services[1].Service != "eru-ql" {
		t.Errorf("expected services sorted by name, got %s then %s",
			report.Services[0].Service, report.Services[1].Service)
	}
}

func TestStatusMarksUnreachableInstanceOutOfSync(t *testing.T) {
	at := time.Now().UTC().Truncate(time.Second)
	healthy := newInstanceServer("eru-ql", "ql-1", at)
	defer healthy.Close()
	dead := newInstanceServer("eru-ql", "ql-2", at)
	dead.Close()

	reg := newTestRegistry(t)
	registerAll(t, reg, map[string]*instanceServer{"ql-1": healthy, "ql-2": dead}, "eru-ql")

	report, err := New(reg).Status(t.Context(), "eru-ql")
	if err != nil {
		t.Fatal(err)
	}
	if report.InSync {
		t.Error("an unreachable instance must make the service out of sync")
	}
	svc := findService(report, "eru-ql")
	if svc.Instances[1].Error == "" {
		t.Error("expected the unreachable instance to carry an error")
	}
}

func TestForceSyncReloadsEveryInstance(t *testing.T) {
	at := time.Now().UTC().Truncate(time.Second)
	a := newInstanceServer("eru-ql", "ql-1", at)
	defer a.Close()
	b := newInstanceServer("eru-ql", "ql-2", at)
	defer b.Close()

	reg := newTestRegistry(t)
	registerAll(t, reg, map[string]*instanceServer{"ql-1": a, "ql-2": b}, "eru-ql")

	report, err := New(reg).ForceSync(t.Context(), "eru-ql")
	if err != nil {
		t.Fatal(err)
	}
	if a.loads.Load() != 1 || b.loads.Load() != 1 {
		t.Errorf("expected each instance reloaded once, got %d and %d", a.loads.Load(), b.loads.Load())
	}
	svc := findService(report, "eru-ql")
	for _, inst := range svc.Instances {
		if inst.Reloaded == nil || !*inst.Reloaded {
			t.Errorf("expected %s to report reloaded=true", inst.InstanceId)
		}
	}
}

func TestForceSyncReportsFailedReload(t *testing.T) {
	at := time.Now().UTC().Truncate(time.Second)
	broken := newInstanceServer("eru-ql", "ql-1", at)
	broken.failLoad = true
	defer broken.Close()

	reg := newTestRegistry(t)
	registerAll(t, reg, map[string]*instanceServer{"ql-1": broken}, "eru-ql")

	report, err := New(reg).ForceSync(t.Context(), "eru-ql")
	if err != nil {
		t.Fatal(err)
	}
	svc := findService(report, "eru-ql")
	inst := svc.Instances[0]
	if inst.Reloaded == nil || *inst.Reloaded {
		t.Error("expected reloaded=false for a failing instance")
	}
	if inst.Error == "" {
		t.Error("expected an error to be reported")
	}
	if report.InSync {
		t.Error("a failed reload must not report in_sync")
	}
}

func TestStatusOnEmptyRegistry(t *testing.T) {
	report, err := New(newTestRegistry(t)).Status(t.Context(), "eru-ql")
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Services) != 0 {
		t.Errorf("expected no services, got %+v", report.Services)
	}
	if !report.InSync {
		t.Error("an empty registry is trivially in sync")
	}
}

func TestStatusOnNilNotifier(t *testing.T) {
	var n *Notifier
	if _, err := n.Status(t.Context(), ""); err == nil {
		t.Error("expected an error from a nil notifier")
	}
}
