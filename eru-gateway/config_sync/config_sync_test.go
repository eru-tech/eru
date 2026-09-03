package config_sync

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/eru-tech/eru/eru-cache/cache"
	"github.com/eru-tech/eru/eru-gateway/registry"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
	eru_models "github.com/eru-tech/eru/eru-models"
)

func init() {
	logs.LogInit("eru-gateway-test", "test")
}

func newTestRegistry(t *testing.T) *registry.Registry {
	t.Helper()
	c := cache.GetCacheStore("INMEMORY", "")
	if c == nil {
		t.Fatal("could not create an in-memory cache store")
	}
	return registry.NewRegistry(c, time.Minute)
}

type loadServer struct {
	*httptest.Server
	hits atomic.Int32
}

func newLoadServer(status int) *loadServer {
	ls := &loadServer{}
	ls.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/store/load" {
			ls.hits.Add(1)
		}
		w.WriteHeader(status)
	}))
	return ls
}

func waitFor(t *testing.T, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("condition not met before deadline")
}

func TestNotifySkipsOriginatingInstance(t *testing.T) {
	source := newLoadServer(http.StatusOK)
	defer source.Close()
	peer := newLoadServer(http.StatusOK)
	defer peer.Close()

	reg := newTestRegistry(t)
	ctx := t.Context()
	if err := reg.Register(ctx, eru_models.ServiceInstance{Id: "eru-ql-1", Name: "eru-ql", Address: source.URL}); err != nil {
		t.Fatal(err)
	}
	if err := reg.Register(ctx, eru_models.ServiceInstance{Id: "eru-ql-2", Name: "eru-ql", Address: peer.URL}); err != nil {
		t.Fatal(err)
	}

	New(reg).Notify("eru-ql", "eru-ql-1")

	waitFor(t, func() bool { return peer.hits.Load() == 1 })
	if source.hits.Load() != 0 {
		t.Errorf("the originating instance must not be refreshed, got %d hits", source.hits.Load())
	}
}

func TestNotifySkipsOtherServices(t *testing.T) {
	ql := newLoadServer(http.StatusOK)
	defer ql.Close()
	functions := newLoadServer(http.StatusOK)
	defer functions.Close()

	reg := newTestRegistry(t)
	ctx := t.Context()
	if err := reg.Register(ctx, eru_models.ServiceInstance{Id: "eru-ql-1", Name: "eru-ql", Address: ql.URL}); err != nil {
		t.Fatal(err)
	}
	if err := reg.Register(ctx, eru_models.ServiceInstance{Id: "eru-functions-1", Name: "eru-functions", Address: functions.URL}); err != nil {
		t.Fatal(err)
	}

	New(reg).Notify("eru-ql", "eru-ql-other")

	waitFor(t, func() bool { return ql.hits.Load() == 1 })
	time.Sleep(100 * time.Millisecond)
	if functions.hits.Load() != 0 {
		t.Errorf("instances of another service must not be refreshed, got %d hits", functions.hits.Load())
	}
}

func TestNotifyFansOutToAllPeers(t *testing.T) {
	peers := make([]*loadServer, 5)
	reg := newTestRegistry(t)
	ctx := t.Context()
	for i := range peers {
		peers[i] = newLoadServer(http.StatusOK)
		defer peers[i].Close()
		if err := reg.Register(ctx, eru_models.ServiceInstance{
			Id: string(rune('a' + i)), Name: "eru-ql", Address: peers[i].URL}); err != nil {
			t.Fatal(err)
		}
	}

	New(reg).Notify("eru-ql", "not-registered")

	for i := range peers {
		idx := i
		waitFor(t, func() bool { return peers[idx].hits.Load() == 1 })
	}
}

func TestNotifyToleratesFailingPeer(t *testing.T) {
	healthy := newLoadServer(http.StatusOK)
	defer healthy.Close()
	broken := newLoadServer(http.StatusInternalServerError)
	defer broken.Close()

	reg := newTestRegistry(t)
	ctx := t.Context()
	if err := reg.Register(ctx, eru_models.ServiceInstance{Id: "1", Name: "eru-ql", Address: healthy.URL}); err != nil {
		t.Fatal(err)
	}
	if err := reg.Register(ctx, eru_models.ServiceInstance{Id: "2", Name: "eru-ql", Address: broken.URL}); err != nil {
		t.Fatal(err)
	}

	New(reg).Notify("eru-ql", "source")

	waitFor(t, func() bool { return healthy.hits.Load() == 1 && broken.hits.Load() == 1 })
}

func TestNotifyIsNoOpWithoutServiceName(t *testing.T) {
	peer := newLoadServer(http.StatusOK)
	defer peer.Close()

	reg := newTestRegistry(t)
	if err := reg.Register(t.Context(), eru_models.ServiceInstance{Id: "1", Name: "eru-ql", Address: peer.URL}); err != nil {
		t.Fatal(err)
	}

	New(reg).Notify("", "source")

	time.Sleep(150 * time.Millisecond)
	if peer.hits.Load() != 0 {
		t.Error("an empty service name must not trigger a fan-out")
	}
}

func TestNilNotifierIsSafe(t *testing.T) {
	var n *Notifier
	n.Notify("eru-ql", "source")
	if New(nil) != nil {
		t.Error("New(nil) must return a nil notifier")
	}
}
