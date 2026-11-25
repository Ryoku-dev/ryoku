package session

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"ryoku-rashin/internal/protocol"
)

const CoalesceWindow = 50 * time.Millisecond

type emitFunc func(kind string, data any) error

type pendingEvent struct {
	kind string
	data any
	key  string
}

// Coalescer combines consecutive streaming deltas without changing replay.
type Coalescer struct {
	mu      sync.Mutex
	emit    emitFunc
	pending *pendingEvent
	timer   *time.Timer
	err     error
	closed  bool
}

func NewCoalescer(emit emitFunc) *Coalescer {
	return &Coalescer{emit: emit}
}

// Emit queues coalescible deltas and flushes them before any other event.
func (c *Coalescer) Emit(kind string, data any) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return fmt.Errorf("session coalescer is closed")
	}
	if c.err != nil {
		return c.err
	}
	key, normalized, ok := coalescible(kind, data)
	if !ok {
		if err := c.flushLocked(); err != nil {
			return err
		}
		return c.emit(kind, data)
	}
	if c.pending != nil && (c.pending.kind != kind || c.pending.key != key) {
		if err := c.flushLocked(); err != nil {
			return err
		}
	}
	if c.pending == nil {
		c.pending = &pendingEvent{kind: kind, data: normalized, key: key}
		c.timer = time.AfterFunc(CoalesceWindow, c.onTimer)
		return nil
	}
	mergePending(c.pending, normalized)
	return nil
}

func coalescible(kind string, data any) (string, any, bool) {
	switch kind {
	case protocol.EvText, protocol.EvThought:
		var v protocol.TextData
		if !convert(data, &v) || v.Message == "" {
			return "", nil, false
		}
		return v.Message, v, true
	case protocol.EvTerminal:
		var v protocol.TerminalData
		if !convert(data, &v) || v.Terminal == "" {
			return "", nil, false
		}
		return v.Terminal, v, true
	default:
		return "", nil, false
	}
}

func convert(in, out any) bool {
	b, err := json.Marshal(in)
	return err == nil && json.Unmarshal(b, out) == nil
}

func mergePending(p *pendingEvent, data any) {
	switch old := p.data.(type) {
	case protocol.TextData:
		v := data.(protocol.TextData)
		old.Delta += v.Delta
		p.data = old
	case protocol.TerminalData:
		v := data.(protocol.TerminalData)
		if v.Command != "" {
			old.Command = v.Command
		}
		old.Delta += v.Delta
		if v.Exit != nil {
			old.Exit = v.Exit
		}
		old.Done = old.Done || v.Done
		p.data = old
	}
}

func (c *Coalescer) onTimer() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed || c.pending == nil {
		return
	}
	c.timer = nil
	if err := c.flushLocked(); err != nil {
		c.err = err
	}
}

func (c *Coalescer) flushLocked() error {
	if c.pending == nil {
		return c.err
	}
	if c.timer != nil {
		c.timer.Stop()
		c.timer = nil
	}
	p := c.pending
	c.pending = nil
	if err := c.emit(p.kind, p.data); err != nil {
		c.err = err
		return err
	}
	return c.err
}

func (c *Coalescer) Flush() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.flushLocked()
}

func (c *Coalescer) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return c.err
	}
	err := c.flushLocked()
	c.closed = true
	return err
}
