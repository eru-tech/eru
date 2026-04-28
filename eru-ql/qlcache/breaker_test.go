package qlcache

import (
	"testing"
	"time"
)

func resetBreaker() {
	bk.mu.Lock()
	bk.failures = bk.failures[:0]
	bk.openedUntil = time.Time{}
	bk.mu.Unlock()
}

func TestBreaker_OpensAfterThreshold(t *testing.T) {
	resetBreaker()
	oldFail := bkConfig.failuresToOpen
	oldOpen := bkConfig.openDuration
	bkConfig.failuresToOpen = 3
	bkConfig.openDuration = 200 * time.Millisecond
	defer func() {
		bkConfig.failuresToOpen = oldFail
		bkConfig.openDuration = oldOpen
		resetBreaker()
	}()

	if !breakerAllow() {
		t.Fatalf("breaker should start closed")
	}
	for i := 0; i < 3; i++ {
		breakerRecordFailure()
	}
	if breakerAllow() {
		t.Fatalf("breaker should be open after 3 failures")
	}
	time.Sleep(250 * time.Millisecond)
	if !breakerAllow() {
		t.Fatalf("breaker should close after openDuration")
	}
}

func TestBreaker_SuccessResetsFailures(t *testing.T) {
	resetBreaker()
	oldFail := bkConfig.failuresToOpen
	bkConfig.failuresToOpen = 3
	defer func() {
		bkConfig.failuresToOpen = oldFail
		resetBreaker()
	}()

	breakerRecordFailure()
	breakerRecordFailure()
	breakerRecordSuccess()
	breakerRecordFailure()
	breakerRecordFailure()
	if !breakerAllow() {
		t.Fatalf("breaker should still be closed; success resets the counter")
	}
}

func TestBreaker_WindowExpires(t *testing.T) {
	resetBreaker()
	oldFail := bkConfig.failuresToOpen
	oldWin := bkConfig.failureWindow
	bkConfig.failuresToOpen = 3
	bkConfig.failureWindow = 50 * time.Millisecond
	defer func() {
		bkConfig.failuresToOpen = oldFail
		bkConfig.failureWindow = oldWin
		resetBreaker()
	}()

	breakerRecordFailure()
	breakerRecordFailure()
	time.Sleep(80 * time.Millisecond)
	breakerRecordFailure()
	breakerRecordFailure()
	if !breakerAllow() {
		t.Fatalf("breaker should be closed — first two failures aged out")
	}
}
