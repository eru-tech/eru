package store

import (
	"context"
	"sync"
	"testing"
)

func TestMarkConfigChangedSetsFlag(t *testing.T) {
	change := &ConfigChange{}
	ctx := ContextWithConfigChange(context.Background(), change)

	if change.Changed() {
		t.Error("a fresh ConfigChange must not report a change")
	}
	MarkConfigChanged(ctx)
	if !change.Changed() {
		t.Error("expected the change to be recorded")
	}
}

func TestMarkConfigChangedWithoutHolderIsSafe(t *testing.T) {
	MarkConfigChanged(context.Background())
}

func TestNilConfigChangeIsSafe(t *testing.T) {
	var change *ConfigChange
	change.Mark()
	if change.Changed() {
		t.Error("a nil ConfigChange must never report a change")
	}
}

func TestConfigChangeIsConcurrencySafe(t *testing.T) {
	change := &ConfigChange{}
	ctx := ContextWithConfigChange(context.Background(), change)

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			MarkConfigChanged(ctx)
			_ = change.Changed()
		}()
	}
	wg.Wait()

	if !change.Changed() {
		t.Error("expected the change to be recorded")
	}
}
