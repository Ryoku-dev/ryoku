package main

import (
	"sync"
	"testing"
	"time"
)

type fakeClock struct {
	mu     sync.Mutex
	t      time.Time
	timers []*fakeTimer
}

type fakeTimer struct {
	at      time.Time
	c       chan time.Time
	stopped bool
	fired   bool
}

func newFakeClock() *fakeClock {
	return &fakeClock{t: time.Date(2026, 11, 7, 8, 0, 0, 0, time.Local)}
}

func (c *fakeClock) now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *fakeClock) after(d time.Duration) (<-chan time.Time, func() bool) {
	c.mu.Lock()
	ft := &fakeTimer{at: c.t.Add(d), c: make(chan time.Time, 1)}
	c.timers = append(c.timers, ft)
	c.mu.Unlock()
	return ft.c, func() bool {
		c.mu.Lock()
		defer c.mu.Unlock()
		live := !ft.stopped && !ft.fired
		ft.stopped = true
		return live
	}
}

func (c *fakeClock) created() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.timers)
}

func (c *fakeClock) advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t = c.t.Add(d)
	for _, ft := range c.timers {
		if !ft.stopped && !ft.fired && !ft.at.After(c.t) {
			ft.fired = true
			ft.c <- c.t
		}
	}
}

func (c *fakeClock) live() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	n := 0
	for _, ft := range c.timers {
		if !ft.stopped && !ft.fired {
			n++
		}
	}
	return n
}

// the loop may re-arm an identical timer when a demand signal races its start, so count live timers
func (c *fakeClock) waitArmed(t *testing.T) time.Duration {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for {
		c.mu.Lock()
		var live []*fakeTimer
		for _, ft := range c.timers {
			if !ft.stopped && !ft.fired {
				live = append(live, ft)
			}
		}
		now := c.t
		c.mu.Unlock()
		if len(live) == 1 {
			return live[0].at.Sub(now)
		}
		if time.Now().After(deadline) {
			t.Fatalf("%d live timers, want 1", len(live))
		}
		time.Sleep(time.Millisecond)
	}
}

func (c *fakeClock) waitNoLive(t *testing.T) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for c.live() != 0 {
		if time.Now().After(deadline) {
			t.Fatalf("%d timers still armed", c.live())
		}
		time.Sleep(time.Millisecond)
	}
}

type fakeRuns struct {
	mu   sync.Mutex
	n    int
	each chan struct{}
}

func newFakeRuns() *fakeRuns { return &fakeRuns{each: make(chan struct{}, 1024)} }

func (r *fakeRuns) hit() {
	r.mu.Lock()
	r.n++
	r.mu.Unlock()
	r.each <- struct{}{}
}

func (r *fakeRuns) count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.n
}

func (r *fakeRuns) wait(t *testing.T) {
	t.Helper()
	select {
	case <-r.each:
	case <-time.After(2 * time.Second):
		t.Fatal("producer did not run")
	}
}
