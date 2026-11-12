package backend

import "sync/atomic"

// Calls counts an engine's calls in flight, for Busy.
type Calls struct{ n atomic.Int64 }

// Begin marks one call in flight; the returned func ends it.
func (c *Calls) Begin() func() {
	c.n.Add(1)
	return func() { c.n.Add(-1) }
}

// Busy reports whether a call is in flight.
func (c *Calls) Busy() bool { return c.n.Load() > 0 }
