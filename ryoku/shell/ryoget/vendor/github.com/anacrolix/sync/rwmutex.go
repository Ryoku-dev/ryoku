package sync

import "sync"

// TODO: No lock times currently, was in the wrapped sync.Mutex before.
type RWMutex struct {
	inner   sync.RWMutex // Real McCoy
	mu      sync.Mutex
	holders []*int
}

func (me *RWMutex) Lock() {
	withBlocked(func() {
		me.inner.Lock()
	})
	me.addHolder()
}

func (me *RWMutex) TryLock() bool {
	if me.inner.TryLock() {
		me.addHolder()
		return true
	}
	return false
}

func (me *RWMutex) Unlock() {
	me.removeHolder()
	me.inner.Unlock()
}

func (me *RWMutex) RLock() {
	withBlocked(func() {
		me.inner.RLock()
	})
	me.addHolder()
}
func (me *RWMutex) RUnlock() {
	me.removeHolder()
	me.inner.RUnlock()
}

func (me *RWMutex) TryRLock() bool {
	if me.inner.TryRLock() {
		me.addHolder()
		return true
	}
	return false
}

func (me *RWMutex) addHolder() {
	if !contentionOn {
		return
	}
	v := new(int)
	me.mu.Lock()
	me.holders = append(me.holders, v)
	me.mu.Unlock()
	lockHolders.Add(v, 0)
}

// Currently we just evict the last profile added. If it's a write lock there should only be one. If
// it's a read lock... Well that needs more context.
func (me *RWMutex) removeHolder() {
	if !contentionOn {
		return
	}
	me.mu.Lock()
	v := me.holders[len(me.holders)-1]
	me.holders = me.holders[:len(me.holders)-1]
	me.mu.Unlock()
	lockHolders.Remove(v)
}
