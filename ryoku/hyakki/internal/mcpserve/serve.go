// Package mcpserve is hyakki mcp: a Model Context Protocol server over
// stdin and stdout that an outside agent program starts. It offers the
// tools the user turned on under Oni's settings and nothing more, and it
// reaches the daemon only as an mcp client, whose calls the daemon checks
// against its own closed table: the bridge's switch, the client's approval
// and each tool's switch. It never widens what the agent may reach.
//
// Every call is bounded in size, time and number at once, and the log
// holds codes only: never a question, a note, a reminder's words or an
// answer.
package mcpserve

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	i18n "ryoku-i18n"

	"ryoku-rashin/internal/protocol"
	"ryoku-rashin/internal/rpcpipe"
	"ryoku-rashin/internal/sensitive"
)

// Bounds on the bridge.
const (
	// LineLimit caps one message from the agent.
	LineLimit = 256 << 10
	// ArgsLimit caps one tool call's arguments.
	ArgsLimit = 8 << 10
	// OutputLimit caps the text one tool call returns.
	OutputLimit = 8 << 10
	// InFlight is the most calls answered at once; more are refused.
	InFlight = 4
	// CallTimeout bounds every tool but screen_ask.
	CallTimeout = 30 * time.Second
	// AskTimeout bounds screen_ask, which waits for the person's answer to
	// the consent card and then for the model.
	AskTimeout = 4 * time.Minute
	// NameMax bounds the client's name, in runes.
	NameMax = 64
	// TextMax bounds a reminder's or a notice's words, in runes.
	TextMax = 120
	// NoteMax bounds a suggested note and a question, in runes.
	NoteMax = 500
	// QueryMax bounds a notes search, in runes.
	QueryMax = 200
	// SearchMax is the most notes a search returns.
	SearchMax = 10
)

// Daemon is the daemon connection the bridge calls through; the CLI's
// client.Conn is one.
type Daemon interface {
	Call(ctx context.Context, method string, params, result any) error
	Done() <-chan struct{}
	Close() error
}

// Dial connects to the daemon as the bridge for the client the agent
// named. The daemon refuses it while the bridge is off.
type Dial func(ctx context.Context, name string) (Daemon, error)

// Options configure Serve.
type Options struct {
	Dial Dial
	// Version is reported to the agent as the server's version.
	Version string
	// Log receives one code per event; nil drops them.
	Log func(code string)
}

// Serve answers one agent on in and out until in ends or ctx ends. It
// dials the daemon only once the agent has said who it is, and again when
// the daemon stopped in between.
func Serve(ctx context.Context, in io.Reader, out io.Writer, opts Options) error {
	if opts.Dial == nil {
		return errors.New("mcpserve: no dial")
	}
	s := &server{opts: opts}
	local := rpcpipe.New(in, out, rpcpipe.Options{LineLimit: LineLimit, Handle: s.handle})
	defer local.Close()
	defer s.close()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-local.Done():
		return local.Err()
	}
}

type server struct {
	opts Options

	mu       sync.Mutex
	daemon   Daemon
	name     string
	inFlight int
	closed   bool
}

func (s *server) log(code string) {
	if s.opts.Log != nil {
		s.opts.Log(code)
	}
}

func (s *server) close() {
	s.mu.Lock()
	d := s.daemon
	s.closed = true
	s.mu.Unlock()
	if d != nil {
		_ = d.Close()
	}
}

func (s *server) handle(ctx context.Context, method string, params json.RawMessage) (any, error) {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil, rpcpipe.ErrClosed
	}
	if s.inFlight >= InFlight {
		s.mu.Unlock()
		s.log(method + " busy")
		return nil, &rpcpipe.Error{Code: rpcpipe.CodeInvalidRequest, Message: i18n.T("Too many calls at once. Wait for one to finish.")}
	}
	s.inFlight++
	s.mu.Unlock()
	defer func() { s.mu.Lock(); s.inFlight--; s.mu.Unlock() }()
	switch method {
	case "initialize":
		return s.initialize(ctx, params)
	case "ping":
		return map[string]any{}, nil
	case "tools/list":
		d, err := s.ready(ctx)
		if err != nil {
			return nil, err
		}
		tools, err := s.offered(ctx, d)
		if err != nil {
			return nil, err
		}
		list := make([]map[string]any, 0, len(tools))
		for _, t := range tools {
			list = append(list, map[string]any{"name": t.id, "description": t.description(), "inputSchema": t.schema})
		}
		s.log(fmt.Sprintf("tools/list %d", len(list)))
		return map[string]any{"tools": list}, nil
	case "tools/call":
		return s.call(ctx, params)
	}
	s.log("method unsupported")
	return nil, &rpcpipe.Error{Code: rpcpipe.CodeMethodNotFound, Message: i18n.T("Hyakki's MCP server does not offer this method.")}
}

// initialize names the client and dials the daemon as its bridge.
func (s *server) initialize(ctx context.Context, params json.RawMessage) (any, error) {
	var p struct {
		ProtocolVersion string `json:"protocolVersion"`
		ClientInfo      struct {
			Name string `json:"name"`
		} `json:"clientInfo"`
	}
	if json.Unmarshal(params, &p) != nil {
		s.log("initialize invalid")
		return nil, &rpcpipe.Error{Code: rpcpipe.CodeInvalidParams, Message: i18n.T("The initialize request is not valid.")}
	}
	name := strings.TrimSpace(p.ClientInfo.Name)
	if !validName(name) {
		s.log("initialize unnamed")
		return nil, &rpcpipe.Error{Code: rpcpipe.CodeInvalidParams, Message: i18n.T("Say who you are in clientInfo.name: plain text, at most 64 characters.")}
	}
	s.mu.Lock()
	if s.daemon != nil {
		s.mu.Unlock()
		s.log("initialize again")
		return nil, &rpcpipe.Error{Code: rpcpipe.CodeInvalidRequest, Message: i18n.T("This server is already initialized.")}
	}
	s.mu.Unlock()
	dialCtx, cancel := context.WithTimeout(ctx, CallTimeout)
	defer cancel()
	d, err := s.opts.Dial(dialCtx, name)
	if err != nil {
		s.log("initialize refused " + errorCode(err))
		return nil, &rpcpipe.Error{Code: rpcpipe.CodeInvalidRequest, Message: errorText(err)}
	}
	s.mu.Lock()
	if s.daemon != nil || s.closed {
		s.mu.Unlock()
		_ = d.Close()
		return nil, &rpcpipe.Error{Code: rpcpipe.CodeInvalidRequest, Message: i18n.T("This server is already initialized.")}
	}
	s.daemon, s.name = d, name
	s.mu.Unlock()
	version := p.ProtocolVersion
	if version != "2024-11-05" && version != "2025-03-26" && version != "2025-06-18" {
		version = "2024-11-05"
	}
	s.log("initialize ok")
	serverVersion := s.opts.Version
	if serverVersion == "" {
		serverVersion = "dev"
	}
	return map[string]any{
		"protocolVersion": version,
		"capabilities":    map[string]any{"tools": map[string]any{}},
		"serverInfo":      map[string]string{"name": "hyakki", "version": serverVersion},
		"instructions":    i18n.T("Hyakki offers only the tools its user turned on and approved for you. Each says what it does and what it never does. Nothing you send is kept as a fact about the user: a note waits until the user keeps it."),
	}, nil
}

// ready is the daemon connection, dialled again when the daemon went
// away: the bridge never keeps it up, so it may have stopped while idle.
func (s *server) ready(ctx context.Context) (Daemon, error) {
	s.mu.Lock()
	d, name := s.daemon, s.name
	s.mu.Unlock()
	if d == nil {
		return nil, &rpcpipe.Error{Code: rpcpipe.CodeInvalidRequest, Message: i18n.T("Initialize first.")}
	}
	select {
	case <-d.Done():
	default:
		return d, nil
	}
	dialCtx, cancel := context.WithTimeout(ctx, CallTimeout)
	defer cancel()
	fresh, err := s.opts.Dial(dialCtx, name)
	if err != nil {
		s.log("redial refused " + errorCode(err))
		return nil, &rpcpipe.Error{Code: rpcpipe.CodeInternal, Message: errorText(err)}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		_ = fresh.Close()
		return nil, rpcpipe.ErrClosed
	}
	if s.daemon != d {
		// another call dialled first
		_ = fresh.Close()
		return s.daemon, nil
	}
	s.daemon = fresh
	s.log("redial ok")
	return fresh, nil
}

// offered is the tools this client may use now: the bridge's switch, this
// client's approval and each tool's switch, as the daemon reports them.
func (s *server) offered(ctx context.Context, d Daemon) ([]tool, error) {
	ctx, cancel := context.WithTimeout(ctx, CallTimeout)
	defer cancel()
	var status protocol.OniMCPStatus
	if err := d.Call(ctx, protocol.MOniMCP, protocol.Empty{}, &status); err != nil {
		s.log("status refused " + errorCode(err))
		return nil, &rpcpipe.Error{Code: rpcpipe.CodeInternal, Message: errorText(err)}
	}
	if !status.Enabled || !approved(status, s.name) {
		return nil, nil
	}
	out := make([]tool, 0, len(tools))
	for _, t := range tools {
		if slices.Contains(status.Tools, t.id) {
			out = append(out, t)
		}
	}
	return out, nil
}

func approved(status protocol.OniMCPStatus, name string) bool {
	for _, c := range status.Clients {
		if c.Name == name {
			return c.State == protocol.OniMCPClientApproved
		}
	}
	return false
}

// call runs one tool: it must be offered now, its arguments bounded and
// valid; a daemon refusal comes back as the tool's error, in words.
func (s *server) call(ctx context.Context, params json.RawMessage) (any, error) {
	d, err := s.ready(ctx)
	if err != nil {
		return nil, err
	}
	var p struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if json.Unmarshal(params, &p) != nil || len(p.Arguments) > ArgsLimit {
		s.log("tools/call invalid")
		return nil, &rpcpipe.Error{Code: rpcpipe.CodeInvalidParams, Message: i18n.T("Tool arguments must be a JSON object of at most 8 KiB.")}
	}
	offered, err := s.offered(ctx, d)
	if err != nil {
		return nil, err
	}
	i := slices.IndexFunc(offered, func(t tool) bool { return t.id == p.Name })
	if i < 0 {
		s.log("tools/call unavailable")
		return nil, &rpcpipe.Error{Code: rpcpipe.CodeInvalidParams, Message: i18n.T("That tool is not offered: it is unknown, turned off, or this client is not approved in Hyakki's settings.")}
	}
	t := offered[i]
	args := p.Arguments
	if len(args) == 0 || string(args) == "null" {
		args = json.RawMessage("{}")
	}
	timeout := CallTimeout
	if t.id == protocol.OniMCPScreenAsk {
		timeout = AskTimeout
	}
	callCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	text, err := t.run(callCtx, d, args)
	if err != nil {
		code := errorCode(err)
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(callCtx.Err(), context.DeadlineExceeded) {
			code = "timeout"
		}
		s.log("tools/call " + t.id + " refused " + code)
		return toolResult(errorText(err), true), nil
	}
	s.log("tools/call " + t.id + " ok")
	return toolResult(text, false), nil
}

func toolResult(text string, isError bool) map[string]any {
	return map[string]any{"content": []map[string]string{{"type": "text", "text": clip(text, OutputLimit)}}, "isError": isError}
}

// argError is a tool's own refusal of its arguments.
type argError struct{ message string }

func (e *argError) Error() string { return e.message }

func badArgs(message string) error { return &argError{message: message} }

// errorCode is the code an error is logged as: the daemon's code and
// reason, or the bridge's own.
func errorCode(err error) string {
	var wire *protocol.Error
	var arg *argError
	switch {
	case errors.As(err, &wire) && wire.Reason != "":
		return wire.Code + ":" + wire.Reason
	case errors.As(err, &wire):
		return wire.Code
	case errors.As(err, &arg):
		return "arguments"
	case errors.Is(err, context.DeadlineExceeded):
		return "timeout"
	case errors.Is(err, context.Canceled):
		return "cancelled"
	}
	return "error"
}

// errorText is what the agent is told: the daemon's sentence, the
// bridge's own, or a plain line for anything else.
func errorText(err error) string {
	var wire *protocol.Error
	var arg *argError
	switch {
	case errors.As(err, &wire):
		return wire.Message
	case errors.As(err, &arg):
		return arg.message
	case errors.Is(err, context.DeadlineExceeded):
		return i18n.T("That took too long and was stopped.")
	}
	return i18n.T("Hyakki could not do that.")
}

func validName(name string) bool {
	if name == "" || !utf8.ValidString(name) || utf8.RuneCountInString(name) > NameMax {
		return false
	}
	return strings.IndexFunc(name, func(r rune) bool { return unicode.IsControl(r) || unicode.Is(unicode.Cf, r) }) < 0
}

// plain checks words an agent sends: bounded, plain text, no private
// number or secret.
func plain(text string, limit int) (string, error) {
	text = strings.TrimSpace(text)
	switch {
	case text == "":
		return "", badArgs(i18n.T("Give some words."))
	case !utf8.ValidString(text) || utf8.RuneCountInString(text) > limit:
		return "", badArgs(i18n.Tf("Use at most %d characters.", limit))
	case strings.IndexFunc(text, func(r rune) bool { return unicode.IsControl(r) || unicode.Is(unicode.Cf, r) }) >= 0:
		return "", badArgs(i18n.T("Use plain text on one line."))
	case sensitive.Number(text) != sensitive.None || sensitive.Secret(text):
		return "", badArgs(i18n.T("That holds a private number or a secret. Hyakki does not take it."))
	}
	return text, nil
}

// clip cuts s to at most n bytes on a rune boundary.
func clip(s string, n int) string {
	if len(s) <= n {
		return s
	}
	s = s[:n]
	for !utf8.ValidString(s) {
		s = s[:len(s)-1]
	}
	return s + "…"
}
