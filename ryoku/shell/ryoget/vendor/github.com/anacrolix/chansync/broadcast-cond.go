package chansync

import (
	"sync/atomic"
	"unsafe"

	"github.com/anacrolix/chansync/events"
)

// Can be used as zero-value. Due to the caller needing to bring their own synchronization, an
// equivalent to "sync".Cond.Signal is not provided. BroadcastCond is intended to be selected on
// with other channels.
// chanWord is an opaque stand-in for the runtime's channel header. We only ever hold pointers to
// it; the pointee is never allocated or dereferenced. A named type documents intent better than
// byte, and unlike uintptr it remains a real pointer the GC tracks, keeping the channel alive.
type chanWord struct{}

type BroadcastCond struct {
	// Holds the channel's own internal pointer directly, rather than a *chan wrapper, to avoid an
	// extra allocation per Signaled.
	ch atomic.Pointer[chanWord]
}

// A chan is a single word holding its internal pointer; reinterpret it as a raw pointer so it can
// be stored directly.
func packChan(c chan struct{}) *chanWord {
	return (*chanWord)(*(*unsafe.Pointer)(unsafe.Pointer(&c)))
}

func unpackChan(p *chanWord) chan struct{} {
	up := unsafe.Pointer(p)
	return *(*chan struct{})(unsafe.Pointer(&up))
}

func (me *BroadcastCond) Broadcast() {
	if old := me.ch.Swap(nil); old != nil {
		close(unpackChan(old))
	}
}

// Should be called before releasing locks on resources that might trigger subsequent Broadcasts.
// The channel is closed when the condition changes.
func (me *BroadcastCond) Signaled() events.Signaled {
	for {
		if p := me.ch.Load(); p != nil {
			return unpackChan(p)
		}
		c := make(chan struct{})
		if me.ch.CompareAndSwap(nil, packChan(c)) {
			return c
		}
		// Lost the race to install the channel; loop and load the winner.
	}
}
