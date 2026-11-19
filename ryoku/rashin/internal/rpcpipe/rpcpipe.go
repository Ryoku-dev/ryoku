// Package rpcpipe speaks JSON-RPC 2.0 over a pair of byte streams, one
// message per line: the daemon and the tool host child talk over its stdin
// and stdout. Either side may call, answer and notify. A call whose context
// ends sends "$/cancelRequest" so the other side stops working on it.
package rpcpipe

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
)

// LineLimit caps one encoded message. A file read or edit (two contents of
// up to 4 MiB, base64 in JSON) fits with room to spare.
const LineLimit = 64 << 20

// JSON-RPC error codes. CodeTooLarge is this package's own: a result that
// would not fit in LineLimit.
const (
	CodeParse          = -32700
	CodeInvalidRequest = -32600
	CodeMethodNotFound = -32601
	CodeInvalidParams  = -32602
	CodeInternal       = -32603
	CodeTooLarge       = -32001
)

// CancelMethod is the notification a caller sends when it stops waiting for
// a request; its params are {"id": <request id>}.
const CancelMethod = "$/cancelRequest"

// Error is a JSON-RPC error object. A handler returning one sends it as it
// is; any other error is sent as CodeInternal with its text.
type Error struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

func (e *Error) Error() string {
	if e.Message == "" {
		return "rpc error " + strconv.Itoa(e.Code)
	}
	return e.Message
}

var (
	// ErrClosed is returned by calls on a connection that has ended.
	ErrClosed = errors.New("rpcpipe: connection closed")
	// ErrTooLarge is returned when a message would exceed LineLimit.
	ErrTooLarge = errors.New("rpcpipe: message too large")
)

// Handler answers one request. ctx ends when the caller cancels or the
// connection closes. The result is encoded as JSON; nil sends null.
type Handler func(ctx context.Context, method string, params json.RawMessage) (any, error)

// Options configure a connection.
type Options struct {
	// Handle answers incoming requests, each on its own goroutine. Nil
	// answers every request with CodeMethodNotFound.
	Handle Handler
	// Notify receives incoming notifications on the read goroutine, in the
	// order they arrived and before any later response is delivered.
	Notify func(method string, params json.RawMessage)
	// Closed runs once when the connection ends, with the read error (nil
	// on a clean end of stream or Close).
	Closed func(error)
	// LineLimit overrides the package LineLimit when positive.
	LineLimit int
}

type message struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *Error          `json:"error,omitempty"`
}

type reply struct {
	result json.RawMessage
	err    error
}

// Conn is one end of the pipe. It is safe for concurrent use.
type Conn struct {
	w       io.Writer
	opts    Options
	limit   int
	writeMu sync.Mutex
	nextID  atomic.Int64

	base   context.Context
	cancel context.CancelFunc

	mu       sync.Mutex
	pending  map[string]chan reply
	handling map[string]context.CancelFunc
	closed   bool
	err      error
	done     chan struct{}
}

// New starts a connection reading r and writing w. It reads until r ends or
// fails; Close stops it from the caller's side (closing r and w stays the
// caller's job).
func New(r io.Reader, w io.Writer, opts Options) *Conn {
	limit := opts.LineLimit
	if limit <= 0 {
		limit = LineLimit
	}
	base, cancel := context.WithCancel(context.Background())
	c := &Conn{
		w:        w,
		opts:     opts,
		limit:    limit,
		base:     base,
		cancel:   cancel,
		pending:  make(map[string]chan reply),
		handling: make(map[string]context.CancelFunc),
		done:     make(chan struct{}),
	}
	go c.readLoop(r)
	return c
}

// Call sends a request and decodes its result into result (which may be
// nil). An error answer comes back as *Error.
func (c *Conn) Call(ctx context.Context, method string, params, result any) error {
	id := c.nextID.Add(1)
	key := strconv.FormatInt(id, 10)
	ch := make(chan reply, 1)
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return ErrClosed
	}
	c.pending[key] = ch
	c.mu.Unlock()
	raw, err := json.Marshal(params)
	if err == nil {
		err = c.send(message{JSONRPC: "2.0", ID: json.RawMessage(key), Method: method, Params: raw})
	}
	if err != nil {
		c.removePending(key, ch)
		return err
	}
	select {
	case r, ok := <-ch:
		if !ok {
			return ErrClosed
		}
		if r.err != nil {
			return r.err
		}
		if result == nil || len(r.result) == 0 {
			return nil
		}
		return json.Unmarshal(r.result, result)
	case <-ctx.Done():
		c.removePending(key, ch)
		_ = c.Notify(CancelMethod, map[string]json.RawMessage{"id": json.RawMessage(key)})
		return ctx.Err()
	case <-c.done:
		return ErrClosed
	}
}

// Notify sends a notification.
func (c *Conn) Notify(method string, params any) error {
	raw, err := json.Marshal(params)
	if err != nil {
		return err
	}
	return c.send(message{JSONRPC: "2.0", Method: method, Params: raw})
}

// Done is closed when the connection has ended.
func (c *Conn) Done() <-chan struct{} { return c.done }

// Err is why the connection ended: nil while it runs, after Close or after a
// clean end of stream.
func (c *Conn) Err() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.err
}

// Close ends the connection: pending calls fail with ErrClosed and running
// handlers see their context end.
func (c *Conn) Close() error {
	c.close(nil)
	return nil
}

func (c *Conn) send(m message) error {
	b, err := json.Marshal(m)
	if err != nil {
		return err
	}
	if len(b) >= c.limit {
		return ErrTooLarge
	}
	c.mu.Lock()
	closed := c.closed
	c.mu.Unlock()
	if closed {
		return ErrClosed
	}
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	b = append(b, '\n')
	for len(b) > 0 {
		n, err := c.w.Write(b)
		if err != nil {
			return err
		}
		if n == 0 {
			return io.ErrShortWrite
		}
		b = b[n:]
	}
	return nil
}

func (c *Conn) removePending(key string, ch chan reply) {
	c.mu.Lock()
	if c.pending[key] == ch {
		delete(c.pending, key)
	}
	c.mu.Unlock()
}

func (c *Conn) readLoop(r io.Reader) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 64<<10), c.limit+1)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var m message
		if json.Unmarshal(line, &m) != nil {
			continue
		}
		switch {
		case len(m.ID) != 0 && m.Method != "":
			c.handle(m)
		case len(m.ID) != 0:
			key := idKey(m.ID)
			c.mu.Lock()
			ch := c.pending[key]
			delete(c.pending, key)
			if ch != nil {
				if m.Error != nil {
					ch <- reply{err: m.Error}
				} else {
					ch <- reply{result: m.Result}
				}
			}
			c.mu.Unlock()
		case m.Method == CancelMethod:
			var p struct {
				ID json.RawMessage `json:"id"`
			}
			if json.Unmarshal(m.Params, &p) == nil && len(p.ID) != 0 {
				c.mu.Lock()
				cancel := c.handling[idKey(p.ID)]
				c.mu.Unlock()
				if cancel != nil {
					cancel()
				}
			}
		case m.Method != "":
			if c.opts.Notify != nil {
				c.opts.Notify(m.Method, m.Params)
			}
		}
	}
	err := scanner.Err()
	if err == nil {
		err = io.EOF
	}
	c.close(err)
}

func (c *Conn) handle(m message) {
	key := idKey(m.ID)
	ctx, cancel := context.WithCancel(c.base)
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		cancel()
		return
	}
	c.handling[key] = cancel
	c.mu.Unlock()
	go func() {
		defer func() {
			c.mu.Lock()
			delete(c.handling, key)
			c.mu.Unlock()
			cancel()
		}()
		if c.opts.Handle == nil {
			_ = c.respondError(m.ID, &Error{Code: CodeMethodNotFound, Message: "method not found: " + m.Method})
			return
		}
		result, err := c.opts.Handle(ctx, m.Method, m.Params)
		if err != nil {
			var rpcErr *Error
			if !errors.As(err, &rpcErr) {
				rpcErr = &Error{Code: CodeInternal, Message: err.Error()}
			}
			_ = c.respondError(m.ID, rpcErr)
			return
		}
		raw, err := json.Marshal(result)
		if err != nil {
			_ = c.respondError(m.ID, &Error{Code: CodeInternal, Message: err.Error()})
			return
		}
		if err := c.send(message{JSONRPC: "2.0", ID: m.ID, Result: raw}); errors.Is(err, ErrTooLarge) {
			_ = c.respondError(m.ID, &Error{Code: CodeTooLarge, Message: fmt.Sprintf("result of %s too large", m.Method)})
		}
	}()
}

func (c *Conn) respondError(id json.RawMessage, e *Error) error {
	return c.send(message{JSONRPC: "2.0", ID: id, Error: e})
}

func (c *Conn) close(reason error) {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return
	}
	c.closed = true
	if !errors.Is(reason, io.EOF) {
		c.err = reason
	}
	for key, ch := range c.pending {
		close(ch)
		delete(c.pending, key)
	}
	close(c.done)
	c.mu.Unlock()
	c.cancel()
	if c.opts.Closed != nil {
		c.opts.Closed(c.err)
	}
}

// idKey reads a request id as a map key: a string id by its text, a number
// by its digits.
func idKey(raw json.RawMessage) string {
	var text string
	if json.Unmarshal(raw, &text) == nil {
		return text
	}
	return strings.TrimSpace(string(raw))
}
