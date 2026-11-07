package main

import (
	"sync/atomic"
	"testing"
	"time"
)

func TestLockRecoveryRelaunchesAcrossCompositorChurnAndStops(t *testing.T) {
	now := time.Unix(100, 0)
	launches := 0
	policy := newLockRecoveryPolicy(func() time.Time { return now }, func() error {
		launches++
		return nil
	})
	policy.confirmed()

	for _, elapsed := range []time.Duration{0, 2 * time.Second, 6 * time.Second, 14 * time.Second} {
		now = time.Unix(100, 0).Add(elapsed)
		launched, err := policy.step(false)
		if err != nil || !launched {
			t.Fatalf("recovery at %v = launched %v, err %v", elapsed, launched, err)
		}
	}
	if launches != 4 {
		t.Fatalf("launches through compositor churn = %d, want 4", launches)
	}

	now = time.Unix(100, 0).Add(15 * time.Second)
	if launched, err := policy.step(true); err != nil || launched {
		t.Fatalf("secure confirmation launched again: launched %v, err %v", launched, err)
	}
	for range 3 {
		now = now.Add(time.Minute)
		if launched, err := policy.step(true); err != nil || launched {
			t.Fatalf("confirmed lock kept retrying: launched %v, err %v", launched, err)
		}
	}
	if launches != 4 {
		t.Fatalf("launches after confirmation = %d, want 4", launches)
	}
}

func TestLockRecoveryIsBounded(t *testing.T) {
	start := time.Unix(200, 0)
	now := start
	launches := 0
	policy := newLockRecoveryPolicy(func() time.Time { return now }, func() error {
		launches++
		return nil
	})
	policy.confirmed()

	for _, elapsed := range []time.Duration{
		0, 2 * time.Second, 6 * time.Second, 14 * time.Second,
		26 * time.Second, 42 * time.Second, time.Minute, 5 * time.Minute,
	} {
		now = start.Add(elapsed)
		_, _ = policy.step(false)
	}
	if launches != len(lockRecoverySchedule) {
		t.Fatalf("persistent failure launched %d times, want bounded %d", launches, len(lockRecoverySchedule))
	}
}

func TestLockRecoveryStopsWhenUnlockStarts(t *testing.T) {
	now := time.Unix(300, 0)
	launches := 0
	policy := newLockRecoveryPolicy(func() time.Time { return now }, func() error {
		launches++
		return nil
	})
	policy.confirmed()
	if launched, err := policy.step(false); err != nil || !launched {
		t.Fatalf("initial recovery = launched %v, err %v", launched, err)
	}

	policy.unlocked()
	for range 4 {
		now = now.Add(time.Minute)
		if launched, err := policy.step(false); err != nil || launched {
			t.Fatalf("unlock allowed relaunch: launched %v, err %v", launched, err)
		}
	}
	if launches != 1 {
		t.Fatalf("launches after unlock = %d, want 1", launches)
	}
}

// The proof is only polled while a session lock is held: an unlocked session
// costs no wakeups, a confirmed lock starts the poll, and an unlock stops it.
func TestLockRecoveryWatchPollsOnlyWhileHeld(t *testing.T) {
	policy := newLockRecoveryPolicy(time.Now, func() error { return nil })
	var checks atomic.Int64
	secure := func() bool { checks.Add(1); return true }
	quit := make(chan struct{})
	done := make(chan struct{})
	go func() {
		policy.watch(quit, time.Millisecond, func() bool { return false }, secure)
		close(done)
	}()
	defer func() {
		close(quit)
		<-done
	}()

	time.Sleep(40 * time.Millisecond)
	if n := checks.Load(); n != 0 {
		t.Fatalf("unlocked session polled the proof %d times, want 0", n)
	}

	policy.confirmed()
	deadline := time.Now().Add(2 * time.Second)
	for checks.Load() == 0 {
		if time.Now().After(deadline) {
			t.Fatal("confirmed lock never polled the proof")
		}
		time.Sleep(time.Millisecond)
	}

	policy.unlocked()
	time.Sleep(20 * time.Millisecond)
	settled := checks.Load()
	time.Sleep(40 * time.Millisecond)
	if n := checks.Load(); n != settled {
		t.Fatalf("proof polled %d more times after unlock, want 0", n-settled)
	}
}
