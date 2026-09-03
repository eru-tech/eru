package registry

import (
	"testing"
	"time"

	"github.com/eru-tech/eru/eru-cache/cache"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
	eru_models "github.com/eru-tech/eru/eru-models"
)

func init() {
	logs.LogInit("eru-gateway-test", "test")
}

const testTTL = 90 * time.Second

func newTestRegistry(t *testing.T) *Registry {
	t.Helper()
	c := cache.GetCacheStore("INMEMORY", "")
	if c == nil {
		t.Fatal("could not create an in-memory cache store")
	}
	return NewRegistry(c, testTTL)
}

func fetch(t *testing.T, r *Registry, id string) eru_models.ServiceInstance {
	t.Helper()
	instances, err := r.ListAllServices(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	for _, instance := range instances {
		if instance.Id == id {
			return instance
		}
	}
	t.Fatalf("instance %s not found in registry", id)
	return eru_models.ServiceInstance{}
}

func TestRegisterStampsAliveUntilIgnoringCallerValue(t *testing.T) {
	r := newTestRegistry(t)
	stale := time.Now().Add(-4 * time.Hour)

	before := time.Now()
	if err := r.Register(t.Context(), eru_models.ServiceInstance{
		Id: "ql-1", Name: "eru-ql", Address: "http://10.0.0.1:8087", AliveUntil: stale,
	}); err != nil {
		t.Fatal(err)
	}

	got := fetch(t, r, "ql-1").AliveUntil
	if !got.After(time.Now()) {
		t.Errorf("a freshly registered instance must read as alive, got %v", got)
	}
	if got.Before(before.Add(testTTL)) || got.After(time.Now().Add(testTTL)) {
		t.Errorf("expected AliveUntil ~now+%v, got %v", testTTL, got)
	}
}

func TestReRegisterAfterEvictionReadsAsAlive(t *testing.T) {
	r := newTestRegistry(t)
	client := eru_models.ServiceInstance{
		Id: "ql-1", Name: "eru-ql", Address: "http://10.0.0.1:8087",
		AliveUntil: time.Now().Add(-4 * time.Hour),
	}

	if err := r.Register(t.Context(), client); err != nil {
		t.Fatal(err)
	}
	if err := r.Deregister(t.Context(), "ql-1"); err != nil {
		t.Fatal(err)
	}
	if err := r.Register(t.Context(), client); err != nil {
		t.Fatal(err)
	}

	if got := fetch(t, r, "ql-1").AliveUntil; !got.After(time.Now()) {
		t.Errorf("a re-registered instance must read as alive, got %v", got)
	}
}

func TestHeartbeatMovesExpiryForward(t *testing.T) {
	r := newTestRegistry(t)
	if err := r.Register(t.Context(), eru_models.ServiceInstance{
		Id: "ql-1", Name: "eru-ql", Address: "http://10.0.0.1:8087",
	}); err != nil {
		t.Fatal(err)
	}
	first := fetch(t, r, "ql-1").AliveUntil

	time.Sleep(10 * time.Millisecond)
	if err := r.Heartbeat(t.Context(), "ql-1"); err != nil {
		t.Fatal(err)
	}
	second := fetch(t, r, "ql-1").AliveUntil

	if !second.After(first) {
		t.Errorf("heartbeat must push expiry forward: %v then %v", first, second)
	}
	if second.After(time.Now().Add(testTTL)) {
		t.Errorf("heartbeat must not accumulate beyond one TTL window, got %v", second)
	}
}

func TestRepeatedHeartbeatsDoNotAccumulate(t *testing.T) {
	r := newTestRegistry(t)
	if err := r.Register(t.Context(), eru_models.ServiceInstance{
		Id: "ql-1", Name: "eru-ql", Address: "http://10.0.0.1:8087",
	}); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 5; i++ {
		if err := r.Heartbeat(t.Context(), "ql-1"); err != nil {
			t.Fatal(err)
		}
	}
	if got := fetch(t, r, "ql-1").AliveUntil; got.After(time.Now().Add(testTTL)) {
		t.Errorf("expiry drifted past one TTL window after repeated beats: %v", got)
	}
}

func TestHeartbeatOnUnknownInstanceErrors(t *testing.T) {
	r := newTestRegistry(t)
	if err := r.Heartbeat(t.Context(), "missing"); err == nil {
		t.Error("expected an error so the client re-registers")
	}
}
