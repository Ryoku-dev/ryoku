// Package cdp is Hyakki's own client for a Chromium-family browser's
// DevTools protocol, spoken over --remote-debugging-pipe: two file
// descriptors, JSON messages each ended by a NUL byte. There is no network
// at all. Sessions are flattened, so every message for a tab carries its
// sessionId on the one pipe.
//
// Every command passes Allowed first: an exact method and an exact
// parameter shape. Page script runs only from the compiled-in function
// table in functions.go; no caller text is ever evaluated.
package cdp

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync"
)

// MaxMessage bounds one message either way. A viewport picture fits; a
// page that grows a reply past it ends the connection.
const MaxMessage = 32 << 20

// EventBuffer is how many forwarded events may wait for the reader. A
// reader that falls this far behind ends the connection (ErrFlood): an
// event is never dropped silently.
const EventBuffer = 1024

var (
	// ErrRefused means the method or its parameters are not on the list.
	ErrRefused = errors.New("cdp: method or parameters not allowed")
	// ErrClosed means the connection is gone.
	ErrClosed = errors.New("cdp: connection closed")
	// ErrTooLarge means a message passed MaxMessage.
	ErrTooLarge = errors.New("cdp: message too large")
	// ErrFlood means events arrived faster than they were read.
	ErrFlood = errors.New("cdp: event flood")
)

// Error is a protocol error the browser answered with.
type Error struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (e *Error) Error() string { return fmt.Sprintf("cdp: browser error %d: %s", e.Code, e.Message) }

// Event is one forwarded notification. Session is empty for the browser
// itself. Only methods named in Events are forwarded.
type Event struct {
	Session string
	Method  string
	Params  json.RawMessage
}

// Events are the notifications forwarded to the reader; every other
// notification is dropped as it is read.
var Events = map[string]bool{
	"Target.targetCreated":         true,
	"Target.targetDestroyed":       true,
	"Target.targetInfoChanged":     true,
	"Target.targetCrashed":         true,
	"Target.detachedFromTarget":    true,
	"Inspector.targetCrashed":      true,
	"Inspector.detached":           true,
	"Page.frameNavigated":          true,
	"Page.javascriptDialogOpening": true,
	"Page.javascriptDialogClosed":  true,
	"Page.windowOpen":              true,
	"Page.downloadWillBegin":       true,
	"Runtime.bindingCalled":        true,
}

type reply struct {
	result json.RawMessage
	err    error
}

type outgoing struct {
	ID        int64  `json:"id"`
	Method    string `json:"method"`
	Params    any    `json:"params,omitempty"`
	SessionID string `json:"sessionId,omitempty"`
}

type incoming struct {
	ID        int64           `json:"id"`
	Method    string          `json:"method"`
	Params    json.RawMessage `json:"params"`
	Result    json.RawMessage `json:"result"`
	Error     *Error          `json:"error"`
	SessionID string          `json:"sessionId"`
}

// Conn is one pipe connection.
type Conn struct {
	rw     io.ReadWriteCloser
	wmu    sync.Mutex
	mu     sync.Mutex
	next   int64
	wait   map[int64]chan reply
	err    error
	done   chan struct{}
	events chan Event
	once   sync.Once
}

// Dial starts reading rw. Close ends it.
func Dial(rw io.ReadWriteCloser) *Conn {
	c := &Conn{rw: rw, wait: map[int64]chan reply{}, done: make(chan struct{}), events: make(chan Event, EventBuffer)}
	go c.read()
	return c
}

// Events delivers forwarded notifications as they arrive; it is closed
// when the connection ends. Nothing polls.
func (c *Conn) Events() <-chan Event { return c.events }

// Done is closed when the connection ends.
func (c *Conn) Done() <-chan struct{} { return c.done }

// Err is why the connection ended, nil while it runs.
func (c *Conn) Err() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.err
}

// Close ends the connection; calls waiting get ErrClosed.
func (c *Conn) Close() error {
	c.fail(ErrClosed)
	return nil
}

func (c *Conn) fail(err error) {
	c.once.Do(func() {
		c.mu.Lock()
		c.err = err
		waits := c.wait
		c.wait = map[int64]chan reply{}
		c.mu.Unlock()
		_ = c.rw.Close()
		for _, w := range waits {
			w <- reply{err: err}
		}
		close(c.done)
	})
}

// Call sends one allowed command and decodes its result into out (nil
// skips it). session is empty for the browser. A refused command is never
// written.
func (c *Conn) Call(ctx context.Context, session, method string, params, out any) error {
	if !Allowed(method, params) {
		return fmt.Errorf("%w: %s", ErrRefused, method)
	}
	c.mu.Lock()
	if c.err != nil {
		err := c.err
		c.mu.Unlock()
		return err
	}
	c.next++
	id := c.next
	w := make(chan reply, 1)
	c.wait[id] = w
	c.mu.Unlock()
	drop := func() {
		c.mu.Lock()
		delete(c.wait, id)
		c.mu.Unlock()
	}
	b, err := json.Marshal(outgoing{ID: id, Method: method, Params: params, SessionID: session})
	if err != nil {
		drop()
		return err
	}
	if len(b)+1 > MaxMessage {
		drop()
		return ErrTooLarge
	}
	c.wmu.Lock()
	_, err = c.rw.Write(append(b, 0))
	c.wmu.Unlock()
	if err != nil {
		drop()
		c.fail(fmt.Errorf("%w: %v", ErrClosed, err))
		return c.Err()
	}
	select {
	case r := <-w:
		if r.err != nil {
			return r.err
		}
		if out != nil && len(r.result) > 0 {
			return json.Unmarshal(r.result, out)
		}
		return nil
	case <-ctx.Done():
		drop()
		return ctx.Err()
	}
}

func (c *Conn) read() {
	defer close(c.events)
	br := bufio.NewReaderSize(c.rw, 64<<10)
	for {
		msg, err := readMessage(br)
		if err != nil {
			if errors.Is(err, io.EOF) {
				err = ErrClosed
			}
			c.fail(err)
			return
		}
		var in incoming
		if json.Unmarshal(msg, &in) != nil {
			continue
		}
		if in.ID != 0 {
			c.mu.Lock()
			w, ok := c.wait[in.ID]
			delete(c.wait, in.ID)
			c.mu.Unlock()
			if ok {
				r := reply{result: in.Result}
				if in.Error != nil {
					r.err = in.Error
				}
				w <- r
			}
			continue
		}
		if !Events[in.Method] {
			continue
		}
		select {
		case c.events <- Event{Session: in.SessionID, Method: in.Method, Params: in.Params}:
		default:
			c.fail(ErrFlood)
			return
		}
	}
}

// readMessage returns one NUL-ended message without its NUL.
func readMessage(br *bufio.Reader) ([]byte, error) {
	var msg []byte
	for {
		part, err := br.ReadSlice(0)
		if len(msg)+len(part) > MaxMessage {
			return nil, ErrTooLarge
		}
		switch {
		case err == nil:
			if msg == nil {
				return append([]byte(nil), part[:len(part)-1]...), nil
			}
			return append(msg, part[:len(part)-1]...), nil
		case errors.Is(err, bufio.ErrBufferFull):
			msg = append(msg, part...)
		default:
			return nil, err
		}
	}
}
