package surface

import "time"

// pacingSlack lets a frame callback that lands a hair early still step, so a
// tier at the refresh rate never needs a timer.
const pacingSlack = 2 * time.Millisecond

// fallbackInterval paces a running engine whose surface took no buffer yet
// (no frame callback to wait on) until the compositor configures it.
const fallbackInterval = 16 * time.Millisecond

// clock paces the engine. Frame callbacks are the clock: a step that commits
// arms one, and nothing steps again until it fires, so a covered or unmapped
// surface does no work. A tier slower than the refresh waits out the rest of
// its interval on a one-shot timer after the callback. A stopped engine arms
// neither.
type clock struct {
	last    time.Time
	next    time.Duration
	armed   bool
	waiting bool
	stopped bool
}

// idle reports whether an event may step at once: nothing is armed or
// pending, so the event would otherwise be lost.
func (c *clock) idle() bool { return !c.armed && !c.waiting }

// dt is the time since the last step; zero for the first after a halt.
func (c *clock) dt(now time.Time) time.Duration {
	if c.last.IsZero() || now.Before(c.last) {
		return 0
	}
	return now.Sub(c.last)
}

// frameDone handles a frame callback: step now, or wait out the tier.
func (c *clock) frameDone(now time.Time) (step bool, wait time.Duration) {
	c.armed = false
	if c.stopped || c.waiting {
		return false, 0
	}
	el := c.dt(now)
	if c.last.IsZero() || el+pacingSlack >= c.next {
		return true, 0
	}
	c.waiting = true
	return false, c.next - el
}

// timerFired handles the pacing timer.
func (c *clock) timerFired() bool {
	c.waiting = false
	return !c.armed && !c.stopped
}

// stepped records a step at now. committed means a frame callback is armed.
// It returns how long to wait before stepping again without a callback.
func (c *clock) stepped(now time.Time, stop, committed bool, next time.Duration) (wait time.Duration) {
	c.last = now
	c.next = next
	c.stopped = stop
	if stop {
		c.armed, c.waiting = false, false
		return 0
	}
	if committed {
		c.armed = true
		return 0
	}
	c.waiting = true
	if next <= 0 {
		return fallbackInterval
	}
	return next
}

// halt forgets everything in flight: the surface is gone with its callback.
func (c *clock) halt() { *c = clock{stopped: true} }
