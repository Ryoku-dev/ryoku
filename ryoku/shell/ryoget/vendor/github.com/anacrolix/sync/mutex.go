package sync

import (
	"runtime"
	"sync"
	"time"
	"unique"
)

type Mutex struct {
	hold *int // Unique value for passing to pprof.
	// Values if lockTimes tracking is enabled.
	*lockTimes
	// Last for struct size reasons.
	mu sync.Mutex
}

// Data for tracking lock time on a Mutex.
type lockTimes struct {
	stack   unique.Handle[callerArray] // The stack for the current holder.
	start   time.Time                  // When the lock was obtained.
	entries int                        // Number of entries returned from runtime.Callers.
}

type callerArray = [32]uintptr

func (m *Mutex) Lock() {
	if contentionOn {
		v := new(int)
		lockBlockers.Add(v, 0)
		m.mu.Lock()
		lockBlockers.Remove(v)
		m.hold = v
		lockHolders.Add(v, 0)
	} else {
		m.mu.Lock()
	}
	if lockTimesOn {
		// We're holding the lock here so it's safe to check.
		if m.lockTimes == nil {
			m.lockTimes = new(lockTimes)
		}
		var stack callerArray
		m.entries = runtime.Callers(2, stack[:])
		m.stack = unique.Make(stack)
		m.start = time.Now()
	}
}

func (m *Mutex) TryLock() bool {
	if !m.mu.TryLock() {
		return false
	}
	if contentionOn {
		v := new(int)
		lockHolders.Add(v, 0)
		m.hold = v
	}
	if lockTimesOn {
		// We're holding the lock here so it's safe to check.
		if m.lockTimes == nil {
			m.lockTimes = new(lockTimes)
		}
		var stack callerArray
		m.entries = runtime.Callers(2, stack[:])
		m.stack = unique.Make(stack)
		m.start = time.Now()
	}
	return true
}

func (m *Mutex) Unlock() {
	if lockTimesOn {
		d := time.Since(m.start)
		key := m.stack.Value()
		lockStatsMu.Lock()
		v, ok := lockStatsByStack[key]
		if !ok {
			v.Init()
		}
		v.Add(d)
		lockStatsByStack[key] = v
		lockStatsMu.Unlock()
	}
	if contentionOn {
		lockHolders.Remove(m.hold)
	}
	m.mu.Unlock()
}
