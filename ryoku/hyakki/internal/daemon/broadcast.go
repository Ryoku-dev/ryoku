package daemon

import (
	"sync"

	"ryoku-rashin/internal/core"
)

// Broadcaster connects subsystem events to subscribed daemon clients.
type Broadcaster struct {
	mu        sync.RWMutex
	next      uint64
	listeners map[uint64]func(string, any)
}

var _ core.Broadcaster = (*Broadcaster)(nil)

func NewBroadcaster() *Broadcaster {
	return &Broadcaster{listeners: make(map[uint64]func(string, any))}
}
func (b *Broadcaster) Broadcast(event string, data any) {
	b.mu.RLock()
	listeners := make([]func(string, any), 0, len(b.listeners))
	for _, listener := range b.listeners {
		listeners = append(listeners, listener)
	}
	b.mu.RUnlock()
	for _, listener := range listeners {
		listener(event, data)
	}
}

func (b *Broadcaster) add(listener func(string, any)) func() {
	b.mu.Lock()
	b.next++
	id := b.next
	b.listeners[id] = listener
	b.mu.Unlock()
	return func() { b.mu.Lock(); delete(b.listeners, id); b.mu.Unlock() }
}
