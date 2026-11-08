package main

import (
	"sync/atomic"
	"testing"
	"time"
)

type wxHarness struct {
	s    *wxState
	clk  *fakeClock
	runs *fakeRuns
	ok   atomic.Bool
	sun  chan struct{}
}

func newWxHarness(t *testing.T) *wxHarness {
	t.Helper()
	h := &wxHarness{clk: newFakeClock(), runs: newFakeRuns(), sun: make(chan struct{}, 1)}
	h.ok.Store(true)
	quit := make(chan struct{})
	t.Cleanup(func() { close(quit) })
	h.s = newWxState(newStateTopic(), &sunState{}, quit)
	h.s.now, h.s.after = h.clk.now, h.clk.after
	h.s.sunWant = h.sun
	h.s.fetch = func() bool {
		h.runs.hit()
		return h.ok.Load()
	}
	return h
}

func (h *wxHarness) start() { go h.s.run() }

// an explicit retry must add exactly one fetch on top, that proves the loop is idle
func (h *wxHarness) sync(t *testing.T, want int) {
	t.Helper()
	h.s.signalWake()
	h.runs.wait(t)
	if got := h.runs.count(); got != want+1 {
		t.Fatalf("fetches = %d, want %d before the explicit retry", got-1, want)
	}
}

func TestWeatherNoSubscriberNoFetchForADay(t *testing.T) {
	h := newWxHarness(t)
	h.start()
	for range 24 * 60 {
		h.clk.advance(time.Minute)
	}
	if n := h.clk.created(); n != 0 {
		t.Fatalf("armed %d timers with no subscriber", n)
	}
	h.sync(t, 0)
}

func TestWeatherPlanSimulatedDay(t *testing.T) {
	for _, tc := range []struct {
		name   string
		active bool
		want   int
	}{
		{"unsubscribed", false, 0},
		{"subscribed", true, 24 * 4},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newWxHarness(t)
			h.s.setActive(tc.active)
			for range 24 * 60 {
				if wait, armed := h.s.plan(h.clk.now()); armed && wait <= 0 {
					h.s.attempt()
				}
				h.clk.advance(time.Minute)
			}
			if got := h.runs.count(); got != tc.want {
				t.Fatalf("fetches over a day = %d, want %d", got, tc.want)
			}
		})
	}
}

func TestWeatherFirstSubscriberFetchesWhenStale(t *testing.T) {
	h := newWxHarness(t)
	h.s.healthy = true
	h.s.lastOK = h.clk.now().Add(-20 * time.Minute)
	h.s.lastAttempt = h.s.lastOK
	h.start()
	h.s.setActive(true)
	h.runs.wait(t)
	if d := h.clk.waitArmed(t); d != wxPollInterval {
		t.Fatalf("next poll in %s, want %s", d, wxPollInterval)
	}
}

func TestWeatherFirstSubscriberResumesWhenFresh(t *testing.T) {
	h := newWxHarness(t)
	h.s.healthy = true
	h.s.lastOK = h.clk.now().Add(-5 * time.Minute)
	h.s.lastAttempt = h.s.lastOK
	h.start()
	h.s.setActive(true)
	if d := h.clk.waitArmed(t); d != 10*time.Minute {
		t.Fatalf("resumed poll in %s, want the 10m remainder", d)
	}
	if n := h.runs.count(); n != 0 {
		t.Fatalf("fresh reading refetched %d times", n)
	}
	h.clk.advance(10 * time.Minute)
	h.runs.wait(t)
	if d := h.clk.waitArmed(t); d != wxPollInterval {
		t.Fatalf("next poll in %s, want %s", d, wxPollInterval)
	}
}

func TestWeatherLastUnsubscribeStopsTimer(t *testing.T) {
	h := newWxHarness(t)
	h.start()
	h.s.setActive(true)
	h.runs.wait(t)
	h.clk.waitArmed(t)
	h.s.setActive(false)
	h.clk.waitNoLive(t)
	h.clk.advance(24 * time.Hour)
	h.sync(t, 1)
}

func TestWeatherRecoveryOnlyWhileSubscribed(t *testing.T) {
	h := newWxHarness(t)
	h.ok.Store(false)
	h.start()
	h.s.setActive(true)
	h.runs.wait(t)
	if d := h.clk.waitArmed(t); d != wxRecoverInterval {
		t.Fatalf("recovery in %s, want %s", d, wxRecoverInterval)
	}
	h.clk.advance(wxRecoverInterval)
	h.runs.wait(t)
	h.clk.waitArmed(t)
	h.s.setActive(false)
	h.clk.waitNoLive(t)
	h.clk.advance(time.Hour)
	h.sync(t, 2)
}

func TestWeatherSunNeedFetchesUntilWindowLands(t *testing.T) {
	h := newWxHarness(t)
	h.ok.Store(false)
	h.start()
	h.sun <- struct{}{}
	h.runs.wait(t)
	if d := h.clk.waitArmed(t); d != wxRecoverInterval {
		t.Fatalf("sun recovery in %s, want %s", d, wxRecoverInterval)
	}
	h.s.sun.observe("2026-11-07T07:10", "2026-11-07T16:40")
	h.ok.Store(true)
	h.clk.advance(wxRecoverInterval)
	h.runs.wait(t)
	for range 24 {
		h.clk.advance(time.Hour)
	}
	h.sync(t, 2)
	if n := h.clk.live(); n != 0 {
		t.Fatalf("%d timers armed once the sun window landed", n)
	}
}

func TestWeatherDemandFollowsTopicSubscribers(t *testing.T) {
	h := newWxHarness(t)
	h.s.topic.onDemand(h.s.setActive)
	h.start()
	sub := h.s.topic.subscribe()
	h.runs.wait(t)
	h.clk.waitArmed(t)
	h.s.topic.unsubscribe(sub)
	h.clk.waitNoLive(t)
}
