package engine

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"maps"
	"os"
	osexec "os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"syscall"
	"time"

	"ryoku-rashin/internal/core"
	"ryoku-rashin/internal/engine/exec"
	"ryoku-rashin/internal/protocol"
	"ryoku-rashin/internal/rpcpipe"
	"ryoku-rashin/internal/sandbox"
)

type runtimeBuffer struct {
	mu        sync.Mutex
	text      string
	limit     int
	truncated bool
}

func (b *runtimeBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	room := b.limit - len(b.text)
	if len(p) > room {
		b.text += string(p[:max(room, 0)])
		b.truncated = true
	} else {
		b.text += string(p)
	}
	return len(p), nil
}
func (b *runtimeBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.truncated {
		return b.text + "\n[output truncated]"
	}
	return b.text
}

type runtimeACP struct {
	r         *runtimeRunner
	ctx       context.Context
	spec      *sandbox.Spec
	conn      *rpcpipe.Conn
	abort     func()
	mu        sync.Mutex
	session   string
	active    bool
	closed    bool
	inFlight  int
	handlers  sync.WaitGroup
	text      runtimeBuffer
	terminals map[string]bool
}

func (r *runtimeRunner) cli(ctx context.Context, in core.PromptInput, sink core.Sink, spec *sandbox.Spec, environment map[string]string) (core.TurnResult, error) {
	result := core.TurnResult{Stop: protocol.StopEnd, Usage: &protocol.TokenUsage{Unmetered: true}}
	bridge, err := r.newBridge(ctx, spec, sink)
	if err != nil {
		return result, err
	}
	defer bridge.close()
	program, err := os.Executable()
	if err != nil {
		return result, errors.New("private runtime MCP command is unavailable")
	}
	// Secrets are inherited environment, NEVER bwrap --setenv argv. The
	// inherited environment is freshly built from the trusted sandbox base,
	// not the daemon's login environment or real agent homes.
	launchSpec := *spec
	launchSpec.Env, launchSpec.ClearEnv = nil, false
	argv, err := sandbox.Command(launchSpec, r.config.Command)
	if err != nil {
		return result, errors.New("configured ACP sandbox could not be constructed")
	}
	bwrap, err := osexec.LookPath(argv[0])
	if err != nil {
		return result, errors.New("bubblewrap is unavailable; refusing host ACP execution")
	}
	env := maps.Clone(spec.Env)
	if env == nil {
		env = map[string]string{}
	}
	maps.Copy(env, environment)
	delete(env, "HYAKKI_SOCKET")
	cmd := &osexec.Cmd{Path: bwrap, Args: argv, Dir: r.opts.Cwd, SysProcAttr: &syscall.SysProcAttr{Setpgid: true}, WaitDelay: time.Second}
	for _, key := range slices.Sorted(maps.Keys(env)) {
		cmd.Env = append(cmd.Env, key+"="+env[key])
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return result, errors.New("ACP stdin could not be opened")
	}
	defer stdin.Close()
	stdinDeadline, ok := stdin.(interface{ SetWriteDeadline(time.Time) error })
	if !ok {
		return result, errors.New("ACP stdin does not support bounded cancellation")
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return result, errors.New("ACP stdout could not be opened")
	}
	defer stdout.Close()
	stderr := &runtimeBuffer{limit: 8192}
	cmd.Stderr = stderr
	if err := ctx.Err(); err != nil {
		return result, err
	}
	if err := cmd.Start(); err != nil {
		return result, errors.New("configured ACP command could not start in the team sandbox")
	}
	waited := make(chan struct{})
	go func() { _ = cmd.Wait(); close(waited) }()
	a := &runtimeACP{r: r, ctx: ctx, spec: spec, text: runtimeBuffer{limit: runtimeOutputLimit}, terminals: map[string]bool{}}
	a.abort = func() {
		_ = stdin.Close()
		_ = stdout.Close()
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
	a.conn = rpcpipe.New(stdout, stdin, rpcpipe.Options{LineLimit: runtimeWireLimit, Handle: a.handle, Notify: a.notify})
	// Closing pipes first also releases a blocked write if a broken agent
	// stops reading. SIGKILL reaches the bwrap process group; its private PID
	// namespace and --die-with-parent reap all descendants as well.
	var cleanupOnce sync.Once
	cleanup := func() {
		cleanupOnce.Do(func() {
			a.mu.Lock()
			a.closed = true
			a.active = false
			a.mu.Unlock()
			_ = stdin.Close()
			_ = stdout.Close()
			_ = a.conn.Close()
			_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
			<-waited
			a.handlers.Wait()
		})
	}
	defer cleanup()
	watchDone := make(chan struct{})
	watchExited := make(chan struct{})
	defer func() { close(watchDone); <-watchExited }()
	go func() {
		defer close(watchExited)
		select {
		case <-ctx.Done():
			a.mu.Lock()
			session := a.session
			a.mu.Unlock()
			if err := stdinDeadline.SetWriteDeadline(time.Now().Add(100 * time.Millisecond)); err == nil && session != "" {
				_ = a.conn.Notify("session/cancel", map[string]string{"sessionId": session})
			}
			_ = stdin.Close()
			_ = stdout.Close()
			_ = a.conn.Close()
			_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		case <-watchDone:
		case <-bridge.finished:
			a.mu.Lock()
			a.active = false
			a.mu.Unlock()
			// A successful governed handoff/report ends execution even when
			// the peer never answers session/prompt or stops reading stdin.
			// Close the transport before Call can send a blocking cancel.
			a.abort()
			_ = a.conn.Close()
		}
	}()
	var initialized struct {
		ProtocolVersion   int `json:"protocolVersion"`
		AgentCapabilities struct {
			LoadSession bool `json:"loadSession"`
		} `json:"agentCapabilities"`
	}
	if err := a.call(ctx, "initialize", map[string]any{"protocolVersion": 1, "clientInfo": map[string]string{"name": "hyakki", "version": "1"}, "clientCapabilities": map[string]any{"fs": map[string]bool{"readTextFile": true, "writeTextFile": r.approvals() != protocol.ApprovalsReadOnly}, "terminal": true}}, &initialized); err != nil {
		return result, a.failure("ACP initialize failed", stderr)
	}
	if initialized.ProtocolVersion != 1 {
		return result, errors.New("ACP agent negotiated an unsupported protocol version")
	}
	r.mu.Lock()
	session, model := r.session, r.model
	r.mu.Unlock()
	method := "session/new"
	params := map[string]any{"cwd": r.opts.Cwd, "mcpServers": []any{map[string]any{"name": "hyakki-team", "command": program, "args": []string{"__runtime-mcp", runtimeBridgeSocket}, "env": []any{}}}}
	if session != "" {
		if !initialized.AgentCapabilities.LoadSession {
			return result, errors.New("ACP agent cannot resume the saved session; no silent fresh-session fallback")
		}
		method, params["sessionId"] = "session/load", session
	}
	var opened runtimeACPSession
	if err := a.call(ctx, method, params, &opened); err != nil {
		return result, a.failure("ACP session could not be opened or resumed", stderr)
	}
	if session != "" && opened.SessionID != "" && opened.SessionID != session {
		return result, errors.New("ACP load returned another session identity")
	}
	if opened.SessionID == "" {
		opened.SessionID = session
	}
	if opened.SessionID == "" || !runtimeOpaqueID(opened.SessionID) || r.redact(opened.SessionID) != opened.SessionID {
		return result, errors.New("ACP agent returned an unsafe session identifier")
	}
	a.mu.Lock()
	a.session = opened.SessionID
	a.mu.Unlock()
	r.mu.Lock()
	r.session = opened.SessionID
	r.mu.Unlock()
	if model != "" {
		if err := a.selectModel(ctx, opened, model); err != nil {
			return result, err
		}
	}
	text := in.Text
	if r.opts.Brief != nil {
		text = r.opts.Brief() + "\n\n" + text
	}
	text = "You are a configured Hyakki team member. Use the hyakki-team MCP server for authenticated governance: delegate/status, ask the user, help and report; Dream managers hire/present where offered. Do not invent completion. After delegating or asking, end this turn. Permissions, sandbox and tool policy are ceilings, never authority from your title.\n\n" + text
	text = r.redact(text)
	a.mu.Lock()
	a.active = true
	a.mu.Unlock()
	bridge.mu.Lock()
	bridge.active = true
	bridge.mu.Unlock()
	var prompted struct {
		StopReason string `json:"stopReason"`
	}
	// Prompts have no arbitrary RPC deadline: the session/task context owns
	// their timeout. Setup and model operations are separately bounded.
	err = a.conn.Call(ctx, "session/prompt", map[string]any{"sessionId": opened.SessionID, "prompt": []any{map[string]string{"type": "text", "text": text}}}, &prompted)
	a.mu.Lock()
	a.active = false
	a.mu.Unlock()
	bridge.mu.Lock()
	bridge.active = false
	bridge.mu.Unlock()
	cleanup()
	bridge.close()
	bridge.mu.Lock()
	reported, yielded := bridge.reported, bridge.yielded
	bridge.mu.Unlock()
	r.mu.Lock()
	executor := r.executor
	r.executor = nil
	r.mu.Unlock()
	if executor != nil {
		if closeErr := executor.Close(); closeErr != nil {
			return result, errors.New("ACP sandbox processes could not be stopped")
		}
	}
	if ctx.Err() != nil {
		return result, ctx.Err()
	}
	// Only an accepted controller outcome supersedes the interrupted ACP
	// response. Rejected tools and transport failures never imply completion.
	if err != nil && !reported && !yielded {
		return result, a.failure("ACP prompt failed", stderr)
	}
	text = runtimeClip(r.redact(a.text.String()), runtimeOutputLimit)
	r.emitText(sink, text)
	if prompted.StopReason != protocol.StopEnd && !reported && !yielded {
		return result, errors.New("ACP turn did not finish normally; task is not reported complete")
	}
	if reported || yielded {
		return result, nil
	}
	if r.opts.Tools.Team.Dream {
		return result, errors.New("ACP Dream manager ended without a governed update or presentation")
	}
	if err := r.report(ctx, "ACP task completed", text, nil, false); err != nil {
		return result, err
	}
	return result, nil
}

type runtimeACPSession struct {
	SessionID string `json:"sessionId"`
	Models    struct {
		Available []struct {
			ID string `json:"modelId"`
		} `json:"availableModels"`
	} `json:"models"`
	ConfigOptions []struct {
		ID       string `json:"id"`
		Category string `json:"category"`
		Type     string `json:"type"`
		Options  []struct {
			Value string `json:"value"`
		} `json:"options"`
	} `json:"configOptions"`
}

func (a *runtimeACP) call(ctx context.Context, method string, params, result any) error {
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	stop := context.AfterFunc(ctx, func() {
		if a.abort != nil {
			a.abort()
		}
		_ = a.conn.Close()
	})
	defer stop()
	return a.conn.Call(ctx, method, params, result)
}
func (a *runtimeACP) failure(message string, stderr *runtimeBuffer) error {
	if a.ctx.Err() != nil {
		return a.ctx.Err()
	}
	detail := strings.TrimSpace(a.r.redact(stderr.String()))
	if detail != "" {
		return errors.New(message + ": " + runtimeClip(detail, 8192))
	}
	return errors.New(message)
}
func (a *runtimeACP) selectModel(ctx context.Context, opened runtimeACPSession, model string) error {
	for _, option := range opened.ConfigOptions {
		if option.Category != "model" || option.Type != "select" {
			continue
		}
		for _, value := range option.Options {
			if value.Value == model {
				if err := a.call(ctx, "session/set_config_option", map[string]string{"sessionId": opened.SessionID, "configId": option.ID, "value": model}, nil); err != nil {
					return errors.New("ACP agent rejected the configured model")
				}
				return nil
			}
		}
		return errors.New("configured model is not offered by this ACP agent")
	}
	for _, offered := range opened.Models.Available {
		if offered.ID == model {
			if err := a.call(ctx, "session/set_model", map[string]string{"sessionId": opened.SessionID, "modelId": model}, nil); err != nil {
				return errors.New("ACP agent rejected the configured model")
			}
			return nil
		}
	}
	return errors.New("ACP agent does not advertise the configured model; refusing an unverified model selection")
}

func (a *runtimeACP) notify(method string, raw json.RawMessage) {
	if method != "session/update" {
		return
	}
	var p struct {
		Session string `json:"sessionId"`
		Update  struct {
			Kind    string `json:"sessionUpdate"`
			Content struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
		} `json:"update"`
	}
	if json.Unmarshal(raw, &p) != nil {
		return
	}
	a.mu.Lock()
	active := a.active && p.Session == a.session
	a.mu.Unlock()
	if active && p.Update.Kind == "agent_message_chunk" && p.Update.Content.Type == "text" {
		// Buffer before redaction: a credential split over ACP chunks must
		// never leak through separate supposedly redacted delta events.
		_, _ = a.text.Write([]byte(p.Update.Content.Text))
	}
}

func (a *runtimeACP) handle(ctx context.Context, method string, raw json.RawMessage) (any, error) {
	a.mu.Lock()
	if a.closed {
		a.mu.Unlock()
		return nil, rpcpipe.ErrClosed
	}
	if a.inFlight >= 16 {
		a.mu.Unlock()
		if a.abort != nil {
			a.abort()
		}
		a.r.Cancel()
		return nil, errors.New("ACP client request concurrency limit exceeded")
	}
	a.inFlight++
	a.handlers.Add(1)
	a.mu.Unlock()
	defer func() { a.mu.Lock(); a.inFlight--; a.mu.Unlock(); a.handlers.Done() }()
	ctx, cancel := context.WithCancel(ctx)
	stop := context.AfterFunc(a.ctx, cancel)
	defer stop()
	defer cancel()
	var p struct {
		Session string `json:"sessionId"`
	}
	if json.Unmarshal(raw, &p) != nil {
		return nil, &rpcpipe.Error{Code: rpcpipe.CodeInvalidParams, Message: "invalid ACP client request"}
	}
	a.mu.Lock()
	active := a.active && a.session != "" && p.Session == a.session
	a.mu.Unlock()
	if !active {
		return nil, &rpcpipe.Error{Code: rpcpipe.CodeInvalidParams, Message: "ACP client request is not for the active turn"}
	}
	switch method {
	case "session/request_permission":
		return a.permission(ctx, raw)
	case "fs/read_text_file", "fs/write_text_file":
		return a.file(ctx, method, raw)
	case "terminal/create", "terminal/output", "terminal/wait_for_exit", "terminal/kill", "terminal/release":
		return a.terminal(ctx, method, raw)
	default:
		return nil, &rpcpipe.Error{Code: rpcpipe.CodeMethodNotFound, Message: "ACP client capability is unsupported"}
	}
}

func (a *runtimeACP) permission(ctx context.Context, raw json.RawMessage) (any, error) {
	var p struct {
		ToolCall struct {
			Kind string `json:"kind"`
		} `json:"toolCall"`
		Options []struct {
			ID   string `json:"optionId"`
			Kind string `json:"kind"`
		} `json:"options"`
	}
	if json.Unmarshal(raw, &p) != nil {
		return nil, &rpcpipe.Error{Code: rpcpipe.CodeInvalidParams, Message: "invalid permission request"}
	}
	// Never trust a foreign agent's "read" classification to authorize an
	// unrestricted child operation. Read-only rejects ACP write grants; the
	// actual filesystem is read-only too. Other requests use the central gate.
	outcome := map[string]string{"outcome": "cancelled"}
	if a.r.approvals() == protocol.ApprovalsReadOnly {
		return map[string]any{"outcome": outcome}, nil
	}
	if err := a.r.allow(ctx, protocol.TierDanger, "Allow the external ACP agent's requested operation within its sandbox"); err != nil {
		return map[string]any{"outcome": outcome}, nil
	}
	for _, option := range p.Options {
		if option.Kind == "allow_once" {
			return map[string]any{"outcome": map[string]string{"outcome": "selected", "optionId": option.ID}}, nil
		}
	}
	return map[string]any{"outcome": outcome}, nil
}

func (a *runtimeACP) file(ctx context.Context, method string, raw json.RawMessage) (any, error) {
	var p struct {
		Path    string `json:"path"`
		Content string `json:"content"`
		Line    *int   `json:"line"`
		Limit   *int   `json:"limit"`
	}
	if json.Unmarshal(raw, &p) != nil || !filepath.IsAbs(p.Path) || len(p.Content) > runtimeOutputLimit {
		return nil, &rpcpipe.Error{Code: rpcpipe.CodeInvalidParams, Message: "invalid bounded absolute file request"}
	}
	x := a.r.getExecutor(a.spec)
	if x == nil {
		return nil, errors.New("sandbox executor is unavailable")
	}
	if method == "fs/write_text_file" {
		if err := a.r.allow(ctx, protocol.TierWrite, "Write a file through the team sandbox"); err != nil {
			return nil, errors.New("file write denied by central policy")
		}
		_, err := x.Write(ctx, exec.WriteRequest{Path: p.Path, Content: []byte(p.Content)})
		if err != nil {
			return nil, errors.New("sandbox file write failed")
		}
		return map[string]any{}, nil
	}
	if err := a.r.allow(ctx, protocol.TierRead, "Read a file through the team sandbox"); err != nil {
		return nil, errors.New("file read denied by central policy")
	}
	read, err := x.Read(ctx, exec.ReadRequest{Path: p.Path, Limit: runtimeOutputLimit + 1})
	if err != nil {
		return nil, errors.New("sandbox file read failed")
	}
	if read.Binary || read.Truncated || len(read.Data) > runtimeOutputLimit {
		return nil, errors.New("file is binary or exceeds the bounded ACP text read limit")
	}
	lines := strings.Split(string(read.Data), "\n")
	start := 0
	if p.Line != nil {
		if *p.Line < 1 {
			return nil, errors.New("file line must be positive")
		}
		start = min(*p.Line-1, len(lines))
	}
	end := len(lines)
	if p.Limit != nil {
		if *p.Limit < 0 {
			return nil, errors.New("file line limit must be nonnegative")
		}
		end = start + min(*p.Limit, len(lines)-start)
	}
	return map[string]string{"content": a.r.redact(strings.Join(lines[start:end], "\n"))}, nil
}

func (a *runtimeACP) terminal(ctx context.Context, method string, raw json.RawMessage) (any, error) {
	var p struct {
		Terminal string   `json:"terminalId"`
		Command  string   `json:"command"`
		Args     []string `json:"args"`
		Cwd      string   `json:"cwd"`
		Env      []struct {
			Name  string `json:"name"`
			Value string `json:"value"`
		} `json:"env"`
		OutputByteLimit *int `json:"outputByteLimit"`
	}
	if json.Unmarshal(raw, &p) != nil {
		return nil, errors.New("invalid terminal request")
	}
	x := a.r.getExecutor(a.spec)
	if x == nil {
		return nil, errors.New("sandbox executor is unavailable")
	}
	if method == "terminal/create" {
		if p.Command == "" || len(p.Args) > 256 {
			return nil, errors.New("invalid terminal argv")
		}
		// Arbitrary shell/program execution cannot be proven read-only by an
		// agent's own description. It is denied, not merely prompted, there.
		if err := a.r.allow(ctx, protocol.TierDanger, "Run an ACP terminal program inside the team sandbox"); err != nil {
			return nil, errors.New("terminal execution denied by central policy")
		}
		if p.Cwd == "" {
			p.Cwd = a.r.opts.Cwd
		}
		if !filepath.IsAbs(p.Cwd) {
			return nil, errors.New("terminal cwd must be absolute")
		}
		env := map[string]string{}
		for _, entry := range p.Env {
			if !runtimeEnvKey(entry.Name) || strings.ContainsRune(entry.Value, 0) {
				return nil, errors.New("unsafe terminal environment")
			}
			env[entry.Name] = entry.Value
		}
		keep := runtimeOutputLimit
		if p.OutputByteLimit != nil {
			if *p.OutputByteLimit <= 0 {
				return nil, errors.New("terminal output limit must be positive")
			}
			keep = min(*p.OutputByteLimit, keep)
		}
		a.mu.Lock()
		if len(a.terminals) >= 16 {
			a.mu.Unlock()
			return nil, errors.New("ACP terminal limit reached")
		}
		// Reserve a slot across Start without holding a mutex while doing IO.
		reservation := "pending-" + p.Command
		if a.terminals[reservation] {
			a.mu.Unlock()
			return nil, errors.New("terminal start is already pending")
		}
		a.terminals[reservation] = true
		a.mu.Unlock()
		id, err := x.Start(ctx, exec.StartRequest{Argv: append([]string{p.Command}, p.Args...), Dir: p.Cwd, Env: env, Keep: keep}, nil)
		a.mu.Lock()
		delete(a.terminals, reservation)
		if err == nil {
			a.terminals[id] = true
		}
		a.mu.Unlock()
		if err != nil {
			return nil, errors.New("sandbox terminal could not start")
		}
		return map[string]string{"terminalId": id}, nil
	}
	a.mu.Lock()
	owned := a.terminals[p.Terminal]
	a.mu.Unlock()
	if !owned || strings.HasPrefix(p.Terminal, "pending-") {
		return nil, errors.New("terminal does not belong to this ACP turn")
	}
	switch method {
	case "terminal/release":
		if err := x.Release(ctx, p.Terminal); err != nil {
			return nil, errors.New("terminal could not be released")
		}
		a.mu.Lock()
		delete(a.terminals, p.Terminal)
		a.mu.Unlock()
		return map[string]any{}, nil
	case "terminal/kill":
		if err := x.Kill(ctx, p.Terminal); err != nil {
			return nil, errors.New("terminal could not be killed")
		}
		return map[string]any{}, nil
	}
	var exit exec.Exit
	var err error
	if method == "terminal/wait_for_exit" {
		exit, err = x.Wait(ctx, p.Terminal)
	} else {
		exit, err = x.Peek(ctx, p.Terminal)
	}
	if err != nil {
		return nil, errors.New("sandbox terminal observation failed")
	}
	status := map[string]any{}
	if !exit.Running {
		status["exitCode"] = exit.Code
		if exit.Signal != "" {
			status["signal"] = exit.Signal
		}
	}
	if method == "terminal/wait_for_exit" {
		return status, nil
	}
	response := map[string]any{"output": a.r.redact(exit.Output), "truncated": exit.Truncated}
	if !exit.Running {
		response["exitStatus"] = status
	}
	return response, nil
}

var _ io.Writer = (*runtimeBuffer)(nil)
