package main

import "sync"

// stateTopic is a coalescing retained topic: a subscriber gets the last frame
// at once, then a fresh frame on every change; byte-identical frames are
// suppressed so an unchanged value never wakes a binding. Mirrors ryoku-shell's
// Go stateTopic and the Rust StateTopic it replaces.
type stateTopic struct {
	mu   sync.Mutex
	last string
	has  bool
	subs map[chan string]struct{}
}

func newStateTopic() *stateTopic {
	return &stateTopic{subs: map[chan string]struct{}{}}
}

func (t *stateTopic) publish(frame string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.has && t.last == frame {
		return
	}
	t.last = frame
	t.has = true
	for ch := range t.subs {
		select {
		case ch <- frame:
		default:
			// A stalled subscriber never blocks the publisher; it catches up on
			// the next publish (frames are full state, not deltas).
		}
	}
}

// subscribe returns the retained frame (if any) plus a channel of future
// frames; the caller must unsubscribe with the returned channel.
func (t *stateTopic) subscribe() (string, bool, chan string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	ch := make(chan string, 16)
	t.subs[ch] = struct{}{}
	return t.last, t.has, ch
}

func (t *stateTopic) unsubscribe(ch chan string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.subs, ch)
}

// eventHub fans broadcast events (already-serialized Event JSON lines) out to
// subscribed connections. No retention: events are notifications, not state.
type eventHub struct {
	mu      sync.Mutex
	subs    map[chan string]struct{}
	control chan string
}

func newEventHub() *eventHub {
	return &eventHub{subs: map[chan string]struct{}{}}
}

func (h *eventHub) subscribe() chan string {
	h.mu.Lock()
	defer h.mu.Unlock()
	ch := make(chan string, 64)
	h.subs[ch] = struct{}{}
	return ch
}

func (h *eventHub) unsubscribe(ch chan string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.subs, ch)
	if h.control == ch {
		h.control = nil
	}
}

func (h *eventHub) setControl(ch chan string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, subscribed := h.subs[ch]; subscribed {
		h.control = ch
	}
}

func (h *eventHub) publish(line string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for ch := range h.subs {
		select {
		case ch <- line:
		default:
		}
	}
}

// publishControl makes room for a user action when the picker has filled its
// event queue. Other subscribers keep the ordinary lossy behavior: an
// irrelevant picker control never displaces one of their queued events.
func (h *eventHub) publishControl(line string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for ch := range h.subs {
		select {
		case ch <- line:
			continue
		default:
		}
		if ch != h.control {
			continue
		}
		select {
		case <-ch:
		default:
		}
		ch <- line
	}
}
