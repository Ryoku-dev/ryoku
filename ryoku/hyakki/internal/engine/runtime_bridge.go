package engine

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"slices"
	"sync"

	"ryoku-rashin/internal/core"
	"ryoku-rashin/internal/engine/tool"
	"ryoku-rashin/internal/protocol"
	"ryoku-rashin/internal/rpcpipe"
	"ryoku-rashin/internal/sandbox"
)

const runtimeBridgeSocket = "/tmp/hyakki-team-mcp/mcp.sock"

// ServeRuntimeMCP is the private oni __runtime-mcp <socket> command. It is
// invoked by the ACP agent inside its existing sandbox, not a public service.
// The only socket it accepts is the private per-turn bind below; it carries
// MCP, not daemon methods, run tokens or arbitrary method dispatch.
func ServeRuntimeMCP(ctx context.Context, socket string, input io.Reader, output io.Writer) error {
	if socket != runtimeBridgeSocket {
		return errors.New("invalid private runtime MCP socket")
	}
	connection, err := (&net.Dialer{}).DialContext(ctx, "unix", socket)
	if err != nil {
		return errors.New("private runtime MCP bridge is unavailable")
	}
	defer connection.Close()
	remote := rpcpipe.New(connection, connection, rpcpipe.Options{LineLimit: runtimeWireLimit})
	defer remote.Close()
	local := rpcpipe.New(input, output, rpcpipe.Options{LineLimit: runtimeWireLimit,
		Handle: func(callCtx context.Context, method string, params json.RawMessage) (any, error) {
			var result json.RawMessage
			err := remote.Call(callCtx, method, params, &result)
			return result, err
		},
		Notify: func(method string, params json.RawMessage) { _ = remote.Notify(method, params) },
	})
	defer local.Close()
	defer func() {
		if closer, ok := input.(io.Closer); ok {
			_ = closer.Close()
		}
	}()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-remote.Done():
		return remote.Err()
	case <-local.Done():
		return local.Err()
	}
}

type runtimeBridge struct {
	r           *runtimeRunner
	ctx         context.Context
	cancel      context.CancelFunc
	dir         string
	listener    net.Listener
	done        chan struct{}
	finished    chan struct{}
	mu          sync.Mutex
	closed      bool
	inFlight    int
	connections map[net.Conn]bool
	handlers    sync.WaitGroup
	watchers    sync.WaitGroup
	closeOnce   sync.Once
	serial      sync.Mutex
	env         tool.Env
	active      bool
	reported    bool
	yielded     bool
}

func (r *runtimeRunner) newBridge(ctx context.Context, spec *sandbox.Spec, sink core.Sink) (*runtimeBridge, error) {
	if r.f.deps.Tools == nil {
		return nil, errors.New("ACP runtime requires the existing team tool registry")
	}
	dir, err := os.MkdirTemp("", "hyakki-runtime-mcp-")
	if err != nil {
		return nil, errors.New("private runtime MCP directory could not be created")
	}
	listener, err := net.Listen("unix", filepath.Join(dir, "mcp.sock"))
	if err != nil {
		_ = os.RemoveAll(dir)
		return nil, errors.New("private runtime MCP socket could not be opened")
	}
	bridgeCtx, cancel := context.WithCancel(ctx)
	profile := tool.Profile{Mode: ModeForAgent(r.opts), Class: tool.ClassFull, ReadOnly: r.approvals() == protocol.ApprovalsReadOnly}
	b := &runtimeBridge{r: r, ctx: bridgeCtx, cancel: cancel, dir: dir, listener: listener, done: make(chan struct{}), finished: make(chan struct{}), connections: map[net.Conn]bool{}}
	b.env = tool.Env{Session: r.opts.Session, Cwd: r.opts.Cwd, Profile: profile, Limits: profile.Limits(), Context: r.opts.Tools, Sandbox: spec, Permissions: r.gate, Host: runtimeBridgeHost{r: r, bridge: b}, Settings: r.opts.Settings, Sink: sink}
	// Only the bridge directory is reintroduced; the daemon/run socket binds
	// removed by processSpec are never restored to the external process.
	spec.Binds = append(spec.Binds, sandbox.Bind{Source: dir, Target: filepath.Dir(runtimeBridgeSocket)})
	go b.accept()
	return b, nil
}

func (b *runtimeBridge) accept() {
	defer close(b.done)
	for {
		connection, err := b.listener.Accept()
		if err != nil {
			return
		}
		b.mu.Lock()
		if b.closed || len(b.connections) >= 4 {
			b.mu.Unlock()
			_ = connection.Close()
			continue
		}
		b.connections[connection] = true
		b.mu.Unlock()
		var rpc *rpcpipe.Conn
		rpc = rpcpipe.New(connection, connection, rpcpipe.Options{LineLimit: runtimeWireLimit, Handle: b.handle,
			Closed: func(error) {
				_ = connection.Close()
				b.mu.Lock()
				delete(b.connections, connection)
				b.mu.Unlock()
			},
		})
		b.watchers.Add(1)
		go func() {
			defer b.watchers.Done()
			select {
			case <-b.ctx.Done():
				_ = rpc.Close()
				_ = connection.Close()
			case <-rpc.Done():
			}
		}()
	}
}

func (b *runtimeBridge) close() {
	b.closeOnce.Do(func() {
		b.cancel()
		_ = b.listener.Close()
		<-b.done
		b.mu.Lock()
		b.closed = true
		for connection := range b.connections {
			_ = connection.Close()
		}
		b.mu.Unlock()
		b.handlers.Wait()
		b.watchers.Wait()
		_ = os.RemoveAll(b.dir)
	})
}

// finishLocked revokes this prompt's execution authority after an accepted
// controller operation. The ACP watcher must also stop the whole process:
// refusing further MCP calls alone cannot contain a peer that never returns.
func (b *runtimeBridge) finishLocked(reported bool) {
	b.reported = b.reported || reported
	b.yielded = b.yielded || !reported
	if b.finished != nil {
		select {
		case <-b.finished:
		default:
			close(b.finished)
		}
	}
}

func runtimeGovernanceTool(name string) bool {
	switch name {
	case tool.Report, tool.AskUser, tool.Delegate, tool.TeamStatus, tool.Hire, tool.Release, tool.DreamStatus, tool.Present,
		tool.RequestHelp, tool.Escalate, tool.ProposePlan, tool.RequestHands:
		return true
	}
	return false
}

func (b *runtimeBridge) offered(name string) (tool.Tool, bool) {
	if !runtimeGovernanceTool(name) || !slices.Contains(tool.Names(b.env.Profile.Mode, tool.ClassFull), name) {
		return nil, false
	}
	t, ok := b.r.f.deps.Tools.Get(name)
	if !ok || b.env.Profile.ReadOnly && !t.ReadOnly() {
		return nil, false
	}
	if conditional, ok := t.(tool.Conditional); ok && !conditional.Enabled(b.env) {
		return nil, false
	}
	return t, true
}

func (b *runtimeBridge) handle(ctx context.Context, method string, params json.RawMessage) (any, error) {
	b.mu.Lock()
	if b.closed {
		b.mu.Unlock()
		return nil, rpcpipe.ErrClosed
	}
	if b.inFlight >= 8 {
		b.mu.Unlock()
		b.cancel()
		b.r.Cancel()
		return nil, errors.New("private runtime MCP concurrency limit exceeded")
	}
	b.inFlight++
	b.handlers.Add(1)
	b.mu.Unlock()
	defer func() { b.mu.Lock(); b.inFlight--; b.mu.Unlock(); b.handlers.Done() }()
	ctx, cancel := context.WithCancel(ctx)
	stop := context.AfterFunc(b.ctx, cancel)
	defer stop()
	defer cancel()
	// MCP operations are serialized: governance actions/report may end a run,
	// so they must not race another action bearing the same live identity.
	b.serial.Lock()
	defer b.serial.Unlock()
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	switch method {
	case "initialize":
		var p struct {
			ProtocolVersion string `json:"protocolVersion"`
		}
		if json.Unmarshal(params, &p) != nil {
			return nil, &rpcpipe.Error{Code: rpcpipe.CodeInvalidParams, Message: "invalid MCP initialize"}
		}
		version := p.ProtocolVersion
		if version != "2024-11-05" && version != "2025-03-26" && version != "2025-06-18" {
			version = "2024-11-05"
		}
		return map[string]any{"protocolVersion": version, "capabilities": map[string]any{"tools": map[string]any{}}, "serverInfo": map[string]string{"name": "hyakki-team", "version": "1"}, "instructions": "Use the team's governed tools. Delegate/status/ask/report follow your actual reporting authority, never your title. After delegating or asking the user, end the turn. Dream managers present rather than report."}, nil
	case "ping":
		return map[string]any{}, nil
	case "tools/list":
		tools := []map[string]any{}
		for _, name := range tool.Names(b.env.Profile.Mode, tool.ClassFull) {
			if t, ok := b.offered(name); ok {
				spec := t.Spec(b.env.Profile)
				tools = append(tools, map[string]any{"name": spec.Name, "description": spec.Description, "inputSchema": spec.Parameters})
			}
		}
		return map[string]any{"tools": tools}, nil
	case "tools/call":
		b.mu.Lock()
		active := b.active && !b.reported && !b.yielded
		b.mu.Unlock()
		if !active {
			return nil, &rpcpipe.Error{Code: rpcpipe.CodeInvalidRequest, Message: "team tools require a current task prompt that has not reported or yielded"}
		}
		var p struct {
			Name      string          `json:"name"`
			Arguments json.RawMessage `json:"arguments"`
		}
		if json.Unmarshal(params, &p) != nil || len(p.Arguments) > runtimeOutputLimit {
			return nil, &rpcpipe.Error{Code: rpcpipe.CodeInvalidParams, Message: "invalid bounded tool arguments"}
		}
		t, ok := b.offered(p.Name)
		if !ok {
			return nil, &rpcpipe.Error{Code: rpcpipe.CodeInvalidParams, Message: "team tool is unavailable under this run's policy"}
		}
		if b.env.Exec == nil {
			b.env.Exec = b.r.getExecutor(b.env.Sandbox)
		}
		result := t.Run(ctx, tool.Call{ID: "runtime-team", Name: p.Name, Args: p.Arguments}, b.env)
		if !result.IsError && result.End {
			b.mu.Lock()
			b.finishLocked(false)
			b.mu.Unlock()
		}
		return map[string]any{"content": []map[string]string{{"type": "text", "text": runtimeClip(b.r.redact(result.Text), runtimeOutputLimit)}}, "isError": result.IsError}, nil
	default:
		return nil, &rpcpipe.Error{Code: rpcpipe.CodeMethodNotFound, Message: "private runtime MCP method is not supported"}
	}
}

// runtimeBridgeHost preserves the registered tools' trusted AgentRef while
// redacting their untrusted text fields before durable daemon calls. It tracks
// real accepted governance operations, not an external agent's claim of them.
type runtimeBridgeHost struct {
	r      *runtimeRunner
	bridge *runtimeBridge
}

func (h runtimeBridgeHost) Call(ctx context.Context, method string, params, result any) error {
	if h.r.f.deps.Host == nil {
		return errors.New("team runtime host is unavailable")
	}
	raw, err := json.Marshal(params)
	if err != nil {
		return errors.New("invalid team tool arguments")
	}
	var clean map[string]any
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if decoder.Decode(&clean) != nil {
		return errors.New("invalid team tool parameters")
	}
	// Auth comes only from the configured live run, never model arguments.
	delete(clean, "session")
	delete(clean, "token")
	runtimeRedactObject(clean, h.r.redact)
	clean["session"], clean["token"] = h.r.opts.Session.ID, h.r.opts.Tools.Team.Token
	host := h.r.f.deps.Host(h.r.opts.Session)
	if host == nil {
		return errors.New("team runtime host is unavailable")
	}
	if err := host.Call(ctx, method, clean, result); err != nil {
		return errors.New("team runtime action was rejected or failed")
	}
	h.bridge.mu.Lock()
	switch method {
	case protocol.MTeamsAgentReport:
		h.bridge.finishLocked(true)
	case protocol.MTeamsAgentDelegate, protocol.MTeamsAgentAsk, protocol.MTeamsAgentPresent, protocol.MTeamsAgentDream,
		protocol.MTeamsAgentPlan, protocol.MTeamsAgentHelp, protocol.MTeamsAgentEscalate, protocol.MTeamsAgentHands:
		h.bridge.finishLocked(false)
	}
	h.bridge.mu.Unlock()
	return nil
}

func runtimeRedactObject(value map[string]any, redact func(string) string) {
	for key, item := range value {
		switch typed := item.(type) {
		case string:
			value[key] = redact(typed)
		case map[string]any:
			runtimeRedactObject(typed, redact)
		case []any:
			for i, element := range typed {
				if text, ok := element.(string); ok {
					typed[i] = redact(text)
				}
				if object, ok := element.(map[string]any); ok {
					runtimeRedactObject(object, redact)
				}
			}
		}
	}
}
