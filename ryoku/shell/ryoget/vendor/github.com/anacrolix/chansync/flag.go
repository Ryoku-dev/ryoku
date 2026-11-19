package chansync

import (
	"sync"
	"sync/atomic"

	"github.com/anacrolix/chansync/events"
)

type ReadOnlyFlag interface {
	Bool() bool
	On() events.Active
	Off() events.Active
}

// Flag wraps a boolean value that starts as false (off). You can wait for it to be on or off, and
// set the value as needed.
type Flag struct {
	mu sync.Mutex
	// It could be possible to optimize this to only allocate channels when one doesn't exist.
	on    chan struct{}
	off   chan struct{}
	state atomic.Bool
	// It could be possible to optimize this away to just checking if the desired on or off channel
	// is present.
	inited bool
}

// To match the SetOnce API.
func (me *Flag) IsSet() bool {
	return me.Bool()
}

func (me *Flag) Bool() bool {
	return me.state.Load()
}

func (me *Flag) On() events.Active {
	me.mu.Lock()
	defer me.mu.Unlock()
	me.init()
	return me.on
}

func (me *Flag) Off() events.Active {
	me.mu.Lock()
	defer me.mu.Unlock()
	me.init()
	return me.off
}

// Everywhere this is called then checks state and potentially modifies channels so it's probably
// not worth using sync.Once.
func (me *Flag) init() {
	if me.inited {
		return
	}
	me.on = make(chan struct{})
	me.off = make(chan struct{})
	close(me.off)
	me.inited = true
}

func (me *Flag) SetBool(b bool) (changed bool) {
	if b {
		return me.Set()
	} else {
		return me.Clear()
	}
}

// Is Swap or CompareAndSwap faster for Bool?

// Returns true if the flag value was changed.
func (me *Flag) Set() bool {
	if me.state.Load() {
		return false
	}
	me.mu.Lock()
	defer me.mu.Unlock()
	// TODO: Can this be optimized to not need to allocate channels based on the new value?
	me.init()
	if !me.state.CompareAndSwap(false, true) {
		return false
	}
	close(me.on)
	me.off = make(chan struct{})
	return true
}

// Returns true if the flag value was changed.
func (me *Flag) Clear() bool {
	if !me.state.Load() {
		return false
	}
	me.mu.Lock()
	defer me.mu.Unlock()
	// TODO: Can this be optimized to not need to allocate channels based on the new value?
	me.init()
	if !me.state.CompareAndSwap(true, false) {
		return false
	}
	close(me.off)
	me.on = make(chan struct{})
	return true
}
