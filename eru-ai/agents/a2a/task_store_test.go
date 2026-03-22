package a2a

import (
	"sync"
	"testing"
	"time"
)

func makeTask(id, contextId string, state TaskState) *Task {
	return &Task{
		Kind:      "task",
		Id:        id,
		ContextId: contextId,
		Status: TaskStatus{
			State:     state,
			Timestamp: time.Now().UTC().Format(time.RFC3339),
		},
	}
}

func TestTaskStoreSaveAndGet(t *testing.T) {
	store := NewTaskStore()
	task := makeTask("task-1", "ctx-1", TaskStateSubmitted)
	store.Save(task)

	got, err := store.Get("task-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Id != "task-1" {
		t.Errorf("expected task-1, got %s", got.Id)
	}
	if got.ContextId != "ctx-1" {
		t.Errorf("expected ctx-1, got %s", got.ContextId)
	}
}

func TestTaskStoreGetNotFound(t *testing.T) {
	store := NewTaskStore()
	_, err := store.Get("nonexistent")
	if err == nil {
		t.Fatal("expected error for non-existent task")
	}
}

func TestTaskStoreUpdateStatus(t *testing.T) {
	store := NewTaskStore()
	task := makeTask("task-2", "ctx-2", TaskStateSubmitted)
	store.Save(task)

	newStatus := TaskStatus{State: TaskStateCompleted, Timestamp: time.Now().UTC().Format(time.RFC3339)}
	if err := store.UpdateStatus("task-2", newStatus); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, _ := store.Get("task-2")
	if got.Status.State != TaskStateCompleted {
		t.Errorf("expected completed, got %s", got.Status.State)
	}
}

func TestTaskStoreUpdateStatusNotFound(t *testing.T) {
	store := NewTaskStore()
	err := store.UpdateStatus("nonexistent", TaskStatus{State: TaskStateWorking})
	if err == nil {
		t.Fatal("expected error for non-existent task")
	}
}

func TestTaskStoreCancelWorking(t *testing.T) {
	store := NewTaskStore()
	task := makeTask("task-3", "ctx-3", TaskStateWorking)
	store.Save(task)

	cancelled, err := store.Cancel("task-3")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cancelled.Status.State != TaskStateCanceled {
		t.Errorf("expected canceled, got %s", cancelled.Status.State)
	}
}

func TestTaskStoreCancelCompleted(t *testing.T) {
	store := NewTaskStore()
	task := makeTask("task-4", "ctx-4", TaskStateCompleted)
	store.Save(task)

	_, err := store.Cancel("task-4")
	if err == nil {
		t.Fatal("expected error canceling completed task")
	}
}

func TestTaskStoreCancelFailed(t *testing.T) {
	store := NewTaskStore()
	task := makeTask("task-5", "ctx-5", TaskStateFailed)
	store.Save(task)

	_, err := store.Cancel("task-5")
	if err == nil {
		t.Fatal("expected error canceling failed task")
	}
}

func TestTaskStoreCancelNotFound(t *testing.T) {
	store := NewTaskStore()
	_, err := store.Cancel("nonexistent")
	if err == nil {
		t.Fatal("expected error for non-existent task")
	}
}

func TestTaskStoreConcurrentAccess(t *testing.T) {
	store := NewTaskStore()
	const numTasks = 100

	var wg sync.WaitGroup
	wg.Add(numTasks * 2)

	for i := 0; i < numTasks; i++ {
		taskId := "task-concurrent-" + string(rune('A'+i%26)) + string(rune('0'+i/26))
		task := makeTask(taskId, "ctx", TaskStateSubmitted)
		go func(t *Task) {
			defer wg.Done()
			store.Save(t)
		}(task)
		go func(id string) {
			defer wg.Done()
			store.Get(id)
		}(taskId)
	}
	wg.Wait()
}

func TestTaskStoreOverwrite(t *testing.T) {
	store := NewTaskStore()
	task1 := makeTask("task-6", "ctx-6a", TaskStateSubmitted)
	store.Save(task1)

	task2 := makeTask("task-6", "ctx-6b", TaskStateWorking)
	store.Save(task2)

	got, _ := store.Get("task-6")
	if got.ContextId != "ctx-6b" {
		t.Errorf("expected overwrite to ctx-6b, got %s", got.ContextId)
	}
}

func TestAllTaskStates(t *testing.T) {
	states := []TaskState{
		TaskStateSubmitted, TaskStateWorking, TaskStateInputRequired,
		TaskStateCompleted, TaskStateCanceled, TaskStateFailed,
		TaskStateRejected, TaskStateAuthRequired, TaskStateUnknown,
	}
	for _, s := range states {
		store := NewTaskStore()
		task := makeTask("t", "c", s)
		store.Save(task)
		got, err := store.Get("t")
		if err != nil {
			t.Errorf("failed to get task with state %s: %v", s, err)
		}
		if got.Status.State != s {
			t.Errorf("expected state %s, got %s", s, got.Status.State)
		}
	}
}
