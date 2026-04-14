package orchestrator

import (
	"context"
	"sync"
	"testing"
)

func TestBlackboardSetAndGet(t *testing.T) {
	bb := NewBlackboard()
	bb.Set("key1", "value1", "agent_a")

	val, ok := bb.Get("key1")
	if !ok {
		t.Fatal("expected key1 to exist")
	}
	if val != "value1" {
		t.Errorf("expected value1, got %v", val)
	}
}

func TestBlackboardGetMissing(t *testing.T) {
	bb := NewBlackboard()
	_, ok := bb.Get("nonexistent")
	if ok {
		t.Fatal("expected key to not exist")
	}
}

func TestBlackboardOverwrite(t *testing.T) {
	bb := NewBlackboard()
	bb.Set("key1", "v1", "agent_a")
	bb.Set("key1", "v2", "agent_b")

	val, ok := bb.Get("key1")
	if !ok {
		t.Fatal("expected key1 to exist")
	}
	if val != "v2" {
		t.Errorf("expected v2, got %v", val)
	}
}

func TestBlackboardGetAll(t *testing.T) {
	bb := NewBlackboard()
	bb.Set("k1", "v1", "a1")
	bb.Set("k2", "v2", "a2")

	all := bb.GetAll()
	if len(all) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(all))
	}
	if all["k1"] != "v1" || all["k2"] != "v2" {
		t.Errorf("unexpected values: %v", all)
	}

	all["k3"] = "v3"
	_, ok := bb.Get("k3")
	if ok {
		t.Fatal("GetAll snapshot should not modify blackboard")
	}
}

func TestBlackboardDelete(t *testing.T) {
	bb := NewBlackboard()
	bb.Set("key1", "value1", "agent_a")
	bb.Delete("key1", "agent_b")

	_, ok := bb.Get("key1")
	if ok {
		t.Fatal("expected key1 to be deleted")
	}
}

func TestBlackboardAuditTrail(t *testing.T) {
	bb := NewBlackboard()
	bb.Set("k1", "v1", "agent_a")
	bb.Set("k2", "v2", "agent_b")
	bb.Delete("k1", "agent_c")

	changes := bb.GetChanges()
	if len(changes) != 3 {
		t.Fatalf("expected 3 changes, got %d", len(changes))
	}

	if changes[0].Key != "k1" || changes[0].AgentName != "agent_a" {
		t.Errorf("unexpected first change: %+v", changes[0])
	}
	if changes[1].Key != "k2" || changes[1].AgentName != "agent_b" {
		t.Errorf("unexpected second change: %+v", changes[1])
	}
	if changes[2].Key != "k1" || changes[2].AgentName != "agent_c" || changes[2].Value != nil {
		t.Errorf("unexpected delete change: %+v", changes[2])
	}
}

func TestBlackboardGetChangesSnapshot(t *testing.T) {
	bb := NewBlackboard()
	bb.Set("k1", "v1", "a1")

	changes := bb.GetChanges()
	if len(changes) != 1 {
		t.Fatalf("expected 1 change, got %d", len(changes))
	}

	bb.Set("k2", "v2", "a2")
	if len(changes) != 1 {
		t.Fatal("GetChanges snapshot should not grow after new writes")
	}
}

func TestBlackboardConcurrency(t *testing.T) {
	bb := NewBlackboard()
	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			key := "key"
			bb.Set(key, n, "agent")
			bb.Get(key)
			bb.GetAll()
		}(i)
	}
	wg.Wait()

	_, ok := bb.Get("key")
	if !ok {
		t.Fatal("expected key to exist after concurrent writes")
	}

	changes := bb.GetChanges()
	if len(changes) != 100 {
		t.Errorf("expected 100 changes, got %d", len(changes))
	}
}

func TestBlackboardContextInjection(t *testing.T) {
	ctx := context.Background()

	bb := GetBlackboard(ctx)
	if bb != nil {
		t.Fatal("expected nil blackboard from bare context")
	}

	newBb := NewBlackboard()
	newBb.Set("test", "data", "agent_x")
	ctx = WithBlackboard(ctx, newBb)

	retrieved := GetBlackboard(ctx)
	if retrieved == nil {
		t.Fatal("expected blackboard from context")
	}

	val, ok := retrieved.Get("test")
	if !ok || val != "data" {
		t.Errorf("expected data, got %v", val)
	}
}
