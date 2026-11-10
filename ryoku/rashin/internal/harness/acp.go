package harness

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"ryoku-i18n"
)

const (
	acpProtocolVersion = 1
	acpLineLimit       = 16 << 20
	acpRequestTimeout  = 15 * time.Second
)

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type rpcMessage struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcReply struct {
	result json.RawMessage
	err    error
}

type connection struct {
	in      io.Writer
	writeMu sync.Mutex
	nextID  atomic.Int64

	mu      sync.Mutex
	pending map[string]chan rpcReply
	closed  bool
	done    chan struct{}

	onRequest func(rpcMessage)
	onNotify  func(string, json.RawMessage)
	onClose   func(error)
}

func newConnection(in io.Writer) *connection {
	return &connection{
		in:      in,
		pending: make(map[string]chan rpcReply),
		done:    make(chan struct{}),
	}
}

func (c *connection) start(out io.Reader) {
	go c.readLoop(out)
}

func (c *connection) request(ctx context.Context, method string, params any) (json.RawMessage, error) {
	id := c.nextID.Add(1)
	key := fmt.Sprint(id)
	ch := make(chan rpcReply, 1)
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return nil, errors.New(i18n.T("The agent connection closed. Start the turn again."))
	}
	c.pending[key] = ch
	c.mu.Unlock()
	if err := c.sendRequest(id, method, params); err != nil {
		c.removePending(key, ch)
		return nil, err
	}
	requestCtx := ctx
	if _, ok := ctx.Deadline(); !ok && method != "session/prompt" {
		var cancel context.CancelFunc
		requestCtx, cancel = context.WithTimeout(ctx, acpRequestTimeout)
		defer cancel()
	}
	select {
	case reply, ok := <-ch:
		if !ok {
			return nil, errors.New(i18n.T("The agent connection closed. Start the turn again."))
		}
		return reply.result, reply.err
	case <-requestCtx.Done():
		c.removePending(key, ch)
		return nil, requestCtx.Err()
	case <-c.done:
		return nil, errors.New(i18n.T("The agent connection closed. Start the turn again."))
	}
}

func (c *connection) notify(method string, params any) error {
	return c.send(map[string]any{"jsonrpc": "2.0", "method": method, "params": params})
}

func (c *connection) respond(id json.RawMessage, result any) error {
	var encoded json.RawMessage
	if result != nil {
		b, err := json.Marshal(result)
		if err != nil {
			return err
		}
		encoded = b
	} else {
		encoded = json.RawMessage(`{}`)
	}
	return c.send(rpcMessage{JSONRPC: "2.0", ID: id, Result: encoded})
}

func (c *connection) respondError(id json.RawMessage, code int, message string) error {
	return c.send(rpcMessage{JSONRPC: "2.0", ID: id, Error: &rpcError{Code: code, Message: message}})
}

func (c *connection) sendRequest(id int64, method string, params any) error {
	b, err := json.Marshal(params)
	if err != nil {
		return err
	}
	return c.send(rpcMessage{
		JSONRPC: "2.0",
		ID:      json.RawMessage(fmt.Sprint(id)),
		Method:  method,
		Params:  b,
	})
}

func (c *connection) send(value any) error {
	b, err := json.Marshal(value)
	if err != nil {
		return err
	}
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	_, err = c.in.Write(append(b, '\n'))
	return err
}

func (c *connection) removePending(key string, ch chan rpcReply) {
	c.mu.Lock()
	if c.pending[key] == ch {
		delete(c.pending, key)
	}
	c.mu.Unlock()
}

func (c *connection) readLoop(out io.Reader) {
	scanner := bufio.NewScanner(out)
	scanner.Buffer(make([]byte, 64<<10), acpLineLimit)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var message rpcMessage
		if json.Unmarshal(line, &message) != nil {
			continue
		}
		switch {
		case len(message.ID) != 0 && message.Method != "":
			if c.onRequest != nil {
				go c.onRequest(message)
			}
		case len(message.ID) != 0:
			key := rpcID(message.ID)
			c.mu.Lock()
			ch := c.pending[key]
			delete(c.pending, key)
			c.mu.Unlock()
			if ch != nil {
				if message.Error != nil {
					ch <- rpcReply{err: errors.New(i18n.Tf("The agent request failed: %s. Check the harness logs and try again.", message.Error.Message))}
				} else {
					ch <- rpcReply{result: message.Result}
				}
			}
		case message.Method != "":
			if c.onNotify != nil {
				c.onNotify(message.Method, message.Params)
			}
		}
	}
	c.close(scanner.Err())
}

func (c *connection) close(reason error) {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return
	}
	c.closed = true
	for key, ch := range c.pending {
		close(ch)
		delete(c.pending, key)
	}
	close(c.done)
	c.mu.Unlock()
	if c.onClose != nil {
		c.onClose(reason)
	}
}

func (c *connection) Closed() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.closed
}

func rpcID(raw json.RawMessage) string {
	var text string
	if json.Unmarshal(raw, &text) == nil {
		return text
	}
	return strings.TrimSpace(string(raw))
}

type initResult struct {
	ProtocolVersion   int `json:"protocolVersion"`
	AgentCapabilities struct {
		LoadSession        bool `json:"loadSession"`
		PromptCapabilities struct {
			Image           bool `json:"image"`
			Audio           bool `json:"audio"`
			EmbeddedContext bool `json:"embeddedContext"`
		} `json:"promptCapabilities"`
	} `json:"agentCapabilities"`
}

type sessionResult struct {
	SessionID     string         `json:"sessionId"`
	Models        *modelState    `json:"models"`
	Modes         *modeState     `json:"modes"`
	ConfigOptions []configOption `json:"configOptions"`
}

type modelState struct {
	Available []struct {
		ModelID     string `json:"modelId"`
		Name        string `json:"name"`
		Description string `json:"description"`
	} `json:"availableModels"`
	Current string `json:"currentModelId"`
}

type modeState struct {
	Available []struct {
		ID          string `json:"id"`
		Name        string `json:"name"`
		Description string `json:"description"`
	} `json:"availableModes"`
	Current string `json:"currentModeId"`
}

type configOption struct {
	ID           string `json:"id"`
	Category     string `json:"category"`
	Type         string `json:"type"`
	CurrentValue string `json:"currentValue"`
	Options      []struct {
		Value       string `json:"value"`
		Name        string `json:"name"`
		Description string `json:"description"`
	} `json:"options"`
}
