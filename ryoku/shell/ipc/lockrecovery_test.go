package main

import (
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
