package daemon

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"net"
	"sync"
	"time"

	"ryoget/internal/proto"
)

// Client speaks the socket protocol for the CLI and tests.
type Client struct {
	nc      net.Conn
	mu      sync.Mutex
	next    int64
	pending map[int64]chan frame
	events  chan Event
	done    chan struct{}
}

type Event struct {
	Name string
	Data json.RawMessage
}

type frame struct {
	ID     *int64          `json:"id"`
	Result json.RawMessage `json:"result"`
	Error  *proto.Error    `json:"error"`
	Event  string          `json:"event"`
	Data   json.RawMessage `json:"data"`
}

var ErrClosed = errors.New("ryoget daemon closed the connection")

func Dial(ctx context.Context, socket string) (*Client, error) {
	var d net.Dialer
	nc, err := d.DialContext(ctx, "unix", socket)
	if err != nil {
		return nil, err
	}
	c := &Client{nc: nc, pending: map[int64]chan frame{}, events: make(chan Event, 256), done: make(chan struct{})}
	go c.read()
	return c, nil
}

func (c *Client) Close() error { return c.nc.Close() }

// Events delivers pushed events after Hello; it closes with the connection.
func (c *Client) Events() <-chan Event { return c.events }

func (c *Client) read() {
	defer close(c.done)
	defer close(c.events)
	sc := bufio.NewScanner(c.nc)
	sc.Buffer(make([]byte, 0, 64<<10), proto.MaxFrame)
	for sc.Scan() {
		var f frame
		if json.Unmarshal(sc.Bytes(), &f) != nil {
			continue
		}
		if f.Event != "" {
			select {
			case c.events <- Event{Name: f.Event, Data: f.Data}:
			default:
			}
			continue
		}
		if f.ID == nil {
			continue
		}
		c.mu.Lock()
		ch := c.pending[*f.ID]
		delete(c.pending, *f.ID)
		c.mu.Unlock()
		if ch != nil {
			ch <- f
		}
	}
}

// Call sends one request and decodes its result into out (which may be nil).
func (c *Client) Call(ctx context.Context, method string, params, out any) error {
	c.mu.Lock()
	c.next++
	id := c.next
	ch := make(chan frame, 1)
	c.pending[id] = ch
	c.mu.Unlock()
	raw, err := json.Marshal(params)
	if err != nil {
		return err
	}
	b, _ := json.Marshal(proto.Request{ID: id, Method: method, Params: raw})
	_ = c.nc.SetWriteDeadline(time.Now().Add(5 * time.Second))
	if _, err := c.nc.Write(append(b, '\n')); err != nil {
		return err
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-c.done:
		return ErrClosed
	case f := <-ch:
		if f.Error != nil {
			return f.Error
		}
		if out != nil && len(f.Result) > 0 {
			return json.Unmarshal(f.Result, out)
		}
		return nil
	}
}

func (c *Client) Hello(ctx context.Context, client string) (proto.Hello, error) {
	var h proto.Hello
	err := c.Call(ctx, "hello", map[string]any{"client": client, "version": proto.Version}, &h)
	return h, err
}
