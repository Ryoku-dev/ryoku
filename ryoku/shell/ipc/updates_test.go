package main

import (
	"sync/atomic"
	"testing"
	"time"
)

type updatesHarness struct {
	s        *updatesState
	clk      *fakeClock
	runs     *fakeRuns
	interval atomic.Int64
}

// newUpdatesHarness builds an update checker on a fake clock whose check only
// counts, so no `ryoku status` is ever spawned.
func newUpdatesHarness(t *testing.T) *updatesHarness {
	t.Helper()
	h := &updatesHarness{clk: newFakeClock(), runs: newFakeRuns()}
	h.interval.Store(int64(24 * time.Hour))
	quit := make(chan struct{})
	t.Cleanup(func() { close(quit) })
	h.s = newUpdatesState(newStateTopic(), quit)
	h.s.now, h.s.after = h.clk.now, h.clk.after
	h.s.interval = func() time.Duration { return time.Duration(h.interval.Load()) }
	h.s.check = h.runs.hit
	go h.s.run()
	return h
}

// sync proves the loop is idle and has checked exactly want times: a manual
// check must add exactly one on top.
func (h *updatesHarness) sync(t *testing.T, want int) {
	t.Helper()
	h.s.wake <- struct{}{}
	h.runs.wait(t)
	if got := h.runs.count(); got != want+1 {
		t.Fatalf("checks = %d, want %d before the manual check", got-1, want)
	}
}

func TestUpdatesNoSubscriberNoSpawnForADay(t *testing.T) {
	h := newUpdatesHarness(t)
	for range 24 * 60 {
		h.clk.advance(time.Minute)
	}
	if n := h.clk.created(); n != 0 {
		t.Fatalf("armed %d timers with no subscriber", n)
	}
	// updates.check still runs with nobody subscribed.
	h.sync(t, 0)
}

func TestUpdatesFirstSubscriberChecksAtOnce(t *testing.T) {
	h := newUpdatesHarness(t)
	h.s.setActive(true)
	h.runs.wait(t)
	if d := h.clk.waitArmed(t); d != 24*time.Hour {
		t.Fatalf("next check in %s, want 24h", d)
	}
}

func TestUpdatesResubscribeResumesOrChecks(t *testing.T) {
	h := newUpdatesHarness(t)
	h.s.setActive(true)
	h.runs.wait(t)
	h.clk.waitArmed(t)
	h.s.setActive(false)
	h.clk.waitNoLive(t)

	// Back within the interval: no check, the remainder is armed.
	h.clk.advance(6 * time.Hour)
	h.s.setActive(true)
	if d := h.clk.waitArmed(t); d != 18*time.Hour {
		t.Fatalf("resumed check in %s, want the 18h remainder", d)
	}
	if n := h.runs.count(); n != 1 {
		t.Fatalf("checks = %d, want 1", n)
	}
	h.s.setActive(false)
	h.clk.waitNoLive(t)

	// Back after the interval: check at once.
	h.clk.advance(30 * time.Hour)
	h.s.setActive(true)
	h.runs.wait(t)
	if d := h.clk.waitArmed(t); d != 24*time.Hour {
		t.Fatalf("next check in %s, want 24h", d)
	}
	h.s.setActive(false)
	h.clk.waitNoLive(t)
	h.clk.advance(72 * time.Hour)
	h.sync(t, 2)
}

func TestUpdatesOffKeepsOnlyBaseline(t *testing.T) {
	h := newUpdatesHarness(t)
	h.interval.Store(0)
	h.s.setActive(true)
	h.runs.wait(t)
	for range 7 * 24 {
		h.clk.advance(time.Hour)
	}
	h.sync(t, 1)
	if n := h.clk.created(); n != 0 {
		t.Fatalf("armed %d timers with update_interval off", n)
	}
}

func TestUpdatesDemandFollowsTopicSubscribers(t *testing.T) {
	h := newUpdatesHarness(t)
	h.s.topic.onDemand(h.s.setActive)
	sub := h.s.topic.subscribe()
	h.runs.wait(t)
	h.clk.waitArmed(t)
	h.s.topic.unsubscribe(sub)
	h.clk.waitNoLive(t)
}
