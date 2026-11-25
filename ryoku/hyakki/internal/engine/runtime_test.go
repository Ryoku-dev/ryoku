package engine

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"ryoku-rashin/internal/core"
	"ryoku-rashin/internal/engine/exec"
	"ryoku-rashin/internal/engine/tool"
	"ryoku-rashin/internal/engine/tools/rashin"
	"ryoku-rashin/internal/llm"
	"ryoku-rashin/internal/protocol"
	"ryoku-rashin/internal/rpcpipe"
	"ryoku-rashin/internal/sandbox"
)

type runtimeTestHost struct {
	mu      sync.Mutex
	methods []string
	params  []json.RawMessage
	fail    bool
}

func (h *runtimeTestHost) Call(_ context.Context, method string, params, result any) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.fail {
		return errors.New("host rejected the report")
	}
	encoded, err := json.Marshal(params)
	if err != nil {
		return err
	}
	h.methods = append(h.methods, method)
	h.params = append(h.params, encoded)
	if result != nil {
		return json.Unmarshal([]byte(`{"text":"Accepted."}`), result)
	}
	return nil
}
func (h *runtimeTestHost) count() int { h.mu.Lock(); defer h.mu.Unlock(); return len(h.methods) }
func (h *runtimeTestHost) last(t *testing.T) protocol.AgentReportParams {
	t.Helper()
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.params) == 0 {
		t.Fatal("expected an authenticated report")
	}
	var report protocol.AgentReportParams
	if err := json.Unmarshal(h.params[len(h.params)-1], &report); err != nil {
		t.Fatal(err)
	}
	return report
}

type runtimeTestSink struct {
	mu          sync.Mutex
	text        string
	permissions chan protocol.PermissionData
}

func (s *runtimeTestSink) Emit(kind string, data any) {
	if kind == protocol.EvPermission && s.permissions != nil {
		s.permissions <- data.(protocol.PermissionData)
	}
	if kind == protocol.EvText {
		s.mu.Lock()
		s.text += data.(protocol.TextData).Delta
		s.mu.Unlock()
	}
}
func (s *runtimeTestSink) Update(fn func(*protocol.SessionInfo)) { fn(&protocol.SessionInfo{}) }

func runtimeTestOptions(t *testing.T, kind string) core.AgentOptions {
	t.Helper()
	folder := t.TempDir()
	if err := os.Mkdir(filepath.Join(folder, "runtime"), 0700); err != nil {
		t.Fatal(err)
	}
	return core.AgentOptions{
		Session: protocol.SessionInfo{ID: "runtime-session", Cwd: folder, Mode: protocol.ApprovalsAuto, Temporary: true},
		Cwd:     folder, Approvals: func() string { return protocol.ApprovalsAuto },
		Settings: func() protocol.Settings { return protocol.Settings{} },
		Sandbox:  &sandbox.Spec{Folder: folder, RuntimeDir: filepath.Join(folder, "runtime"), Env: map[string]string{"PATH": "/usr/bin:/bin", "HOME": folder}, ClearEnv: true},
		Tools:    core.ToolContext{Team: &core.TeamToolContext{Team: "team", Task: "task", Member: "member", Token: "private-run-token", Runtime: &protocol.MemberRuntime{Kind: kind}}},
	}
}
func runtimeTestFactory(h *runtimeTestHost, executor exec.Executor) *Factory {
	return New(Deps{Host: func(protocol.SessionInfo) tool.Host { return h }, Executor: func(protocol.SessionInfo, *sandbox.Spec) exec.Executor { return executor }, Tools: tool.NewRegistry(rashin.Tools()...)})
}
func runtimeTestNew(t *testing.T, f *Factory, opts core.AgentOptions) *runtimeRunner {
	t.Helper()
	runner, err := f.NewAgent(context.Background(), opts)
	if err != nil {
		t.Fatal(err)
	}
	r, ok := runner.(*runtimeRunner)
	if !ok {
		t.Fatalf("selected %T instead of external runtime", runner)
	}
	t.Cleanup(func() { _ = r.Stop() })
	return r
}

func TestRuntimeNativeSelectionAndFailClosedConfiguration(t *testing.T) {
	f := New(Deps{})
	defer f.Close()
	opts := runtimeTestOptions(t, protocol.MemberRuntimeNative)
	r, err := f.NewAgent(context.Background(), opts)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := r.(*runner); !ok {
		t.Fatalf("native mode changed to %T", r)
	}
	for _, kind := range []string{protocol.MemberRuntimeCLI, protocol.MemberRuntimeCommand, protocol.MemberRuntimeWebhook, "unrecognized"} {
		opts.Tools.Team.Runtime = &protocol.MemberRuntime{Kind: kind, Command: []string{"program"}, URL: "https://example.invalid/task"}
		if _, err := f.NewAgent(context.Background(), opts); err == nil {
			t.Fatalf("untrusted %s runtime was accepted without its boundary", kind)
		}
	}
}

func TestRuntimeSecretResolutionAndVersionedState(t *testing.T) {
	opts := runtimeTestOptions(t, protocol.MemberRuntimeCLI)
	opts.Tools.Team.Runtime.Command = []string{"configured-agent", "acp"}
	opts.Tools.Team.Runtime.Model = "model-one"
	opts.Tools.Team.Runtime.SecretEnv = map[string]string{"API_KEY": "vault/provider"}
	opts.RuntimeEnvironment = func(context.Context) (map[string]string, error) {
		return map[string]string{"API_KEY": "private-value", "NOT_GRANTED": "must-not-cross"}, nil
	}
	f := runtimeTestFactory(&runtimeTestHost{}, nil)
	defer f.Close()
	r := runtimeTestNew(t, f, opts)
	environment, err := r.environment(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if environment["NOT_GRANTED"] != "" {
		t.Fatal("an unconfigured environment entry crossed the resolver boundary")
	}
	r.session = "external-session"
	state := r.State()
	if strings.Contains(string(state), "private-value") || strings.Contains(string(state), "private-run-token") || strings.Contains(string(state), "vault/provider") {
		t.Fatalf("runtime persisted sensitive data: %s", state)
	}
	resume := opts
	resume.Resume = state
	r2 := runtimeTestNew(t, f, resume)
	if r2.session != "external-session" || r2.model != "model-one" {
		t.Fatal("compatible versioned state did not resume")
	}
	resume.Resume = json.RawMessage(`{"version":99,"session":"foreign","allowed":true}`)
	if runtimeTestNew(t, f, resume).session != "" {
		t.Fatal("unknown runtime state was restored")
	}
	changed := *opts.Tools.Team.Runtime
	changed.Command = []string{"different-agent"}
	resume.Tools.Team = &core.TeamToolContext{Token: "private-run-token", Runtime: &changed}
	resume.Resume = state
	if runtimeTestNew(t, f, resume).session != "" {
		t.Fatal("state crossed a changed runtime configuration")
	}
	opts.RuntimeEnvironment = func(context.Context) (map[string]string, error) { return nil, errors.New("private-value") }
	r3 := runtimeTestNew(t, f, opts)
	if _, err := r3.environment(context.Background()); err == nil || strings.Contains(err.Error(), "private-value") {
		t.Fatal("vault failure was accepted or leaked its cause")
	}
	for _, key := range []string{"LD_PRELOAD", "GCONV_PATH", "HYAKKI_SOCKET", "BAD=KEY"} {
		if runtimeEnvKey(key) {
			t.Fatalf("unsafe loader/identity environment %q accepted", key)
		}
	}
}

type runtimeRecordingExecutor struct {
	exec.Executor
	mu      sync.Mutex
	started chan struct{}
	req     exec.StartRequest
}

func (x *runtimeRecordingExecutor) Start(ctx context.Context, req exec.StartRequest, out exec.OutputFunc) (string, error) {
	id, err := x.Executor.Start(ctx, req, out)
	x.mu.Lock()
	x.req = req
	x.mu.Unlock()
	if x.started != nil {
		close(x.started)
		x.started = nil
	}
	return id, err
}

func TestRuntimeCommandObservedExitRedactedReportAndStdin(t *testing.T) {
	opts := runtimeTestOptions(t, protocol.MemberRuntimeCommand)
	opts.Tools.Team.Runtime.Command = []string{"/bin/sh", "-c", "printf '%s\\n' \"$API_KEY\"; cat"}
	opts.Tools.Team.Runtime.SecretEnv = map[string]string{"API_KEY": "provider"}
	opts.RuntimeEnvironment = func(context.Context) (map[string]string, error) {
		return map[string]string{"API_KEY": "private-value"}, nil
	}
	x := &runtimeRecordingExecutor{Executor: exec.Host(exec.HostOptions{Env: []string{"PATH=/usr/bin:/bin"}})}
	host := &runtimeTestHost{}
	f := runtimeTestFactory(host, x)
	defer f.Close()
	r := runtimeTestNew(t, f, opts)
	sink := &runtimeTestSink{}
	result, err := r.Prompt(context.Background(), core.PromptInput{Turn: 2, Text: "Do the configured work"}, sink)
	if err != nil {
		t.Fatal(err)
	}
	if result.Usage == nil || !result.Usage.Unmetered || result.Stop != protocol.StopEnd {
		t.Fatalf("external work was mislabeled: %+v", result)
	}
	report := host.last(t)
	if report.Session != opts.Session.ID || report.Token != opts.Tools.Team.Token || strings.Contains(report.Body, "private-value") || strings.Contains(sink.text, "private-value") {
		t.Fatalf("bad authenticated redacted report: %+v", report)
	}
	var work runtimeWork
	if err := json.Unmarshal(x.req.Stdin, &work); err != nil {
		t.Fatal(err)
	}
	if work.Task != "task" || work.Turn != 2 || strings.Contains(string(x.req.Stdin), opts.Tools.Team.Token) || strings.Contains(string(x.req.Stdin), "private-value") {
		t.Fatalf("bad work envelope: %s", x.req.Stdin)
	}
	if r.Alive() || len(x.Running()) != 0 {
		t.Fatal("command left work alive after completion")
	}
}

func TestRuntimeCommandErrorsNeverReportSuccess(t *testing.T) {
	for _, command := range [][]string{{"/bin/sh", "-c", "echo actual-failure; exit 7"}, {"/definitely/missing/runtime-command"}} {
		t.Run(command[len(command)-1], func(t *testing.T) {
			opts := runtimeTestOptions(t, protocol.MemberRuntimeCommand)
			opts.Tools.Team.Runtime.Command = command
			host := &runtimeTestHost{}
			f := runtimeTestFactory(host, exec.Host(exec.HostOptions{Env: []string{"PATH=/usr/bin:/bin"}}))
			defer f.Close()
			r := runtimeTestNew(t, f, opts)
			result, err := r.Prompt(context.Background(), core.PromptInput{Text: "work"}, &runtimeTestSink{})
			if err == nil || result.Stop != protocol.StopError || host.count() != 0 || r.Alive() {
				t.Fatalf("failed command looked complete: %+v %v reports=%d", result, err, host.count())
			}
		})
	}
}

func TestRuntimeStopCancelsAndReapsConfiguredCommand(t *testing.T) {
	opts := runtimeTestOptions(t, protocol.MemberRuntimeCommand)
	opts.Tools.Team.Runtime.Command = []string{"/bin/sh", "-c", "sleep 30 & wait"}
	started := make(chan struct{})
	x := &runtimeRecordingExecutor{Executor: exec.Host(exec.HostOptions{Env: []string{"PATH=/usr/bin:/bin"}}), started: started}
	host := &runtimeTestHost{}
	f := runtimeTestFactory(host, x)
	defer f.Close()
	r := runtimeTestNew(t, f, opts)
	done := make(chan error, 1)
	go func() {
		_, err := r.Prompt(context.Background(), core.PromptInput{Text: "work"}, &runtimeTestSink{})
		done <- err
	}()
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("command did not start")
	}
	if err := r.Stop(); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("cancelled task succeeded")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Stop left Prompt alive")
	}
	if r.Alive() || len(x.Running()) != 0 || host.count() != 0 {
		t.Fatal("Stop left a worker/process or completed cancelled work")
	}
}

func runtimeWebhookBody(t *testing.T, usage *protocol.TokenUsage) string {
	t.Helper()
	r := runtimeResponse{Version: 1, Session: "runtime-session", Task: "task", Turn: 3, Status: "completed", Usage: usage}
	r.Report.Title, r.Report.Body = "Done", "Actual result"
	raw, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}
func TestRuntimeWebhookExactCompletionAndUsageContract(t *testing.T) {
	for _, usage := range []*protocol.TokenUsage{nil, {Input: 17, Output: 9}} {
		opts := runtimeTestOptions(t, protocol.MemberRuntimeWebhook)
		opts.Tools.Team.Runtime.URL = "https://example.invalid/work"
		calls := 0
		opts.RuntimeHTTP = func(_ context.Context, payload []byte) (protocol.AgentHTTPResult, error) {
			calls++
			if strings.Contains(string(payload), "private-run-token") {
				t.Fatal("webhook received the run token")
			}
			return protocol.AgentHTTPResult{Status: 200, Body: runtimeWebhookBody(t, usage)}, nil
		}
		host := &runtimeTestHost{}
		f := runtimeTestFactory(host, nil)
		r := runtimeTestNew(t, f, opts)
		result, err := r.Prompt(context.Background(), core.PromptInput{Turn: 3, Text: "Work"}, &runtimeTestSink{})
		if err != nil {
			t.Fatal(err)
		}
		if calls != 1 || host.count() != 1 || result.Usage == nil || (usage == nil) != result.Usage.Unmetered {
			t.Fatalf("incorrect webhook accounting/completion: %+v", result)
		}
		if usage != nil && (result.Usage.Input != 17 || result.Usage.Output != 9) {
			t.Fatalf("reported usage lost: %+v", result.Usage)
		}
		_ = f.Close()
	}
}
func TestRuntimeWebhookRejectsAcceptanceStaleTruncatedAndMalformedReports(t *testing.T) {
	body := runtimeWebhookBody(t, nil)
	for name, response := range map[string]protocol.AgentHTTPResult{
		"accepted":       {Status: 202, Body: body},
		"pending":        {Status: 200, Body: strings.Replace(body, "completed", "running", 1)},
		"stale":          {Status: 200, Body: strings.Replace(body, "runtime-session", "other-run", 1)},
		"old-turn":       {Status: 200, Body: strings.Replace(body, `"turn":3`, `"turn":2`, 1)},
		"truncated":      {Status: 200, Body: body, Truncated: true},
		"oversized":      {Status: 200, Body: strings.Repeat("x", runtimeOutputLimit+1)},
		"trailing":       {Status: 200, Body: body + " not-json"},
		"two-values":     {Status: 200, Body: body + " {}"},
		"no-report":      {Status: 200, Body: `{"version":1,"session":"runtime-session","task":"task","turn":3,"status":"completed"}`},
		"negative-usage": {Status: 200, Body: runtimeWebhookBody(t, &protocol.TokenUsage{Input: -1})},
	} {
		t.Run(name, func(t *testing.T) {
			opts := runtimeTestOptions(t, protocol.MemberRuntimeWebhook)
			opts.Tools.Team.Runtime.URL = "https://example.invalid/work"
			calls := 0
			opts.RuntimeHTTP = func(context.Context, []byte) (protocol.AgentHTTPResult, error) { calls++; return response, nil }
			host := &runtimeTestHost{}
			f := runtimeTestFactory(host, nil)
			defer f.Close()
			r := runtimeTestNew(t, f, opts)
			result, err := r.Prompt(context.Background(), core.PromptInput{Turn: 3}, &runtimeTestSink{})
			if err == nil || result.Stop != protocol.StopError || host.count() != 0 || calls != 1 {
				t.Fatalf("invalid response completed or retried: %+v %v calls=%d", result, err, calls)
			}
		})
	}
}

func TestRuntimeCentralPermissionRefusalAndCancellation(t *testing.T) {
	for _, cancelRun := range []bool{false, true} {
		opts := runtimeTestOptions(t, protocol.MemberRuntimeWebhook)
		opts.Tools.Team.Runtime.URL = "https://example.invalid/work"
		opts.Approvals = func() string { return protocol.ApprovalsAsk }
		called := false
		opts.RuntimeHTTP = func(context.Context, []byte) (protocol.AgentHTTPResult, error) {
			called = true
			return protocol.AgentHTTPResult{}, nil
		}
		f := runtimeTestFactory(&runtimeTestHost{}, nil)
		r := runtimeTestNew(t, f, opts)
		sink := &runtimeTestSink{permissions: make(chan protocol.PermissionData, 4)}
		done := make(chan error, 1)
		go func() { _, err := r.Prompt(context.Background(), core.PromptInput{Text: "work"}, sink); done <- err }()
		var card protocol.PermissionData
		select {
		case card = <-sink.permissions:
		case <-time.After(3 * time.Second):
			t.Fatal("central permission card missing")
		}
		if cancelRun {
			r.Cancel()
		} else if err := r.Permission(card.Request, optionRefuse); err != nil {
			t.Fatal(err)
		}
		select {
		case err := <-done:
			if err == nil {
				t.Fatal("refused/cancelled webhook succeeded")
			}
		case <-time.After(3 * time.Second):
			t.Fatal("permission did not resolve")
		}
		if called {
			t.Fatal("webhook ran without central approval")
		}
		_ = f.Close()
	}
}

func TestRuntimeACPTransportTurnBindingAndChunkRedaction(t *testing.T) {
	opts := runtimeTestOptions(t, protocol.MemberRuntimeCLI)
	opts.Tools.Team.Runtime.Command = []string{"agent"}
	opts.Tools.Team.Runtime.SecretEnv = map[string]string{"API_KEY": "fixture"}
	opts.RuntimeEnvironment = func(context.Context) (map[string]string, error) {
		return map[string]string{"API_KEY": "split-secret"}, nil
	}
	f := runtimeTestFactory(&runtimeTestHost{}, exec.Host(exec.HostOptions{Env: []string{}}))
	defer f.Close()
	r := runtimeTestNew(t, f, opts)
	if _, err := r.environment(context.Background()); err != nil {
		t.Fatal(err)
	}
	a := &runtimeACP{r: r, ctx: context.Background(), spec: r.processSpec(), session: "external", active: true, text: runtimeBuffer{limit: runtimeOutputLimit}, terminals: map[string]bool{}}
	left, right := net.Pipe()
	defer left.Close()
	defer right.Close()
	server := rpcpipe.New(left, left, rpcpipe.Options{Handle: a.handle, Notify: a.notify, LineLimit: runtimeWireLimit})
	defer server.Close()
	client := rpcpipe.New(right, right, rpcpipe.Options{LineLimit: runtimeWireLimit})
	defer client.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := client.Call(ctx, "fs/read_text_file", map[string]string{"sessionId": "foreign", "path": "/etc/hosts"}, nil); err == nil {
		t.Fatal("ACP request crossed session identity")
	}
	if err := client.Call(ctx, "client/arbitrary_host_call", map[string]string{"sessionId": "external"}, nil); err == nil {
		t.Fatal("unsupported capability was accepted")
	}
	for _, chunk := range []string{"visible ", "split-", "secret", " done"} {
		a.notify("session/update", json.RawMessage(`{"sessionId":"external","update":{"sessionUpdate":"agent_message_chunk","content":{"type":"text","text":"`+chunk+`"}}}`))
	}
	if got := r.redact(a.text.String()); strings.Contains(got, "split-secret") || !strings.HasPrefix(got, "visible ") || !strings.HasSuffix(got, " done") {
		t.Fatalf("chunked credential disclosure or lost result: %q", got)
	}
	a.active = false
	if err := client.Call(ctx, "terminal/create", map[string]string{"sessionId": "external", "command": "/bin/true"}, nil); err == nil {
		t.Fatal("inactive ACP turn started a terminal")
	}
}

func TestRuntimeACPReadOnlyRejectsFileAndTerminalWrites(t *testing.T) {
	opts := runtimeTestOptions(t, protocol.MemberRuntimeCLI)
	opts.Tools.Team.Runtime.Command = []string{"agent"}
	opts.Approvals = func() string { return protocol.ApprovalsReadOnly }
	f := runtimeTestFactory(&runtimeTestHost{}, exec.Host(exec.HostOptions{Env: []string{}}))
	defer f.Close()
	r := runtimeTestNew(t, f, opts)
	a := &runtimeACP{r: r, ctx: context.Background(), spec: r.processSpec(), session: "external", active: true, terminals: map[string]bool{}}
	file := filepath.Join(opts.Cwd, "forbidden.txt")
	params, _ := json.Marshal(map[string]string{"sessionId": "external", "path": file, "content": "must not write"})
	if _, err := a.handle(context.Background(), "fs/write_text_file", params); err == nil {
		t.Fatal("read-only file request succeeded")
	}
	if _, err := os.Stat(file); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("file was written despite read-only policy")
	}
	if _, err := a.handle(context.Background(), "terminal/create", json.RawMessage(`{"sessionId":"external","command":"/bin/true"}`)); err == nil {
		t.Fatal("read-only terminal was allowed based on a claim")
	}
	_ = r.executor.Close()
	r.executor = nil
}

func TestRuntimeMCPBridgeUsesExistingAuthorityAndRejectsStartupCalls(t *testing.T) {
	opts := runtimeTestOptions(t, protocol.MemberRuntimeCLI)
	opts.Tools.Team.Runtime.Command = []string{"agent"}
	opts.Tools.Team.Runtime.SecretEnv = map[string]string{"API_KEY": "fixture"}
	opts.Tools.Session = "not-the-supervised-session"
	opts.RuntimeEnvironment = func(context.Context) (map[string]string, error) {
		return map[string]string{"API_KEY": "secret\"with-quote"}, nil
	}
	host := &runtimeTestHost{}
	f := runtimeTestFactory(host, exec.Host(exec.HostOptions{Env: []string{}}))
	defer f.Close()
	r := runtimeTestNew(t, f, opts)
	if _, err := r.environment(context.Background()); err != nil {
		t.Fatal(err)
	}
	b, err := r.newBridge(context.Background(), r.processSpec(), &runtimeTestSink{})
	if err != nil {
		t.Fatal(err)
	}
	defer b.close()
	call := json.RawMessage(`{"name":"report","arguments":{"title":"Done","body":"secret\"with-quote","session":"attacker","token":"attacker"}}`)
	if _, err := b.handle(context.Background(), "tools/call", call); err == nil {
		t.Fatal("process startup could complete a task")
	}
	b.active = true
	if _, ok := b.offered(tool.Delegate); ok {
		t.Fatal("a leaf was offered manager authority")
	}
	if _, ok := b.offered(tool.Hire); ok {
		t.Fatal("a leaf was offered Dream authority")
	}
	if _, err := b.handle(context.Background(), "teams.agent_report", call); err == nil {
		t.Fatal("bridge accepted arbitrary daemon method forwarding")
	}
	rejected, err := b.handle(context.Background(), "tools/call", call)
	if err == nil {
		data, marshalErr := json.Marshal(rejected)
		if marshalErr != nil {
			t.Fatal(marshalErr)
		}
		var response struct {
			IsError bool `json:"isError"`
		}
		if err := json.Unmarshal(data, &response); err != nil || !response.IsError {
			t.Fatal("tool arguments could inject a session or run token")
		}
	}
	if host.count() != 0 || b.reported {
		t.Fatal("rejected identity injection mutated the authenticated task")
	}
	call = json.RawMessage(`{"name":"report","arguments":{"title":"Done","body":"legitimate result secret\"with-quote tail"}}`)
	accepted, err := b.handle(context.Background(), "tools/call", call)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(accepted)
	if err != nil {
		t.Fatal(err)
	}
	var response struct {
		IsError bool `json:"isError"`
	}
	if err := json.Unmarshal(encoded, &response); err != nil || response.IsError {
		t.Fatalf("authenticated report was rejected: %s", encoded)
	}
	report := host.last(t)
	if report.Token != "private-run-token" || report.Session != "runtime-session" || strings.Contains(report.Body, "secret\"with-quote") || !strings.HasPrefix(report.Body, "legitimate result ") || !strings.HasSuffix(report.Body, " tail") || !b.reported {
		t.Fatalf("bridge lost authenticated report or disclosed a credential: %+v", report)
	}
	if _, err := b.handle(context.Background(), "tools/call", call); err == nil {
		t.Fatal("completed task accepted a second report")
	}
}

func TestRuntimeMCPManagerToolsFollowExistingConditionalPolicy(t *testing.T) {
	opts := runtimeTestOptions(t, protocol.MemberRuntimeCLI)
	opts.Tools.Team.Runtime.Command = []string{"agent"}
	opts.Tools.Team.Lead = true
	opts.Tools.Team.Delegate = true
	opts.Tools.Team.Dream = true
	host := &runtimeTestHost{}
	f := runtimeTestFactory(host, nil)
	defer f.Close()
	r := runtimeTestNew(t, f, opts)
	b, err := r.newBridge(context.Background(), r.processSpec(), &runtimeTestSink{})
	if err != nil {
		t.Fatal(err)
	}
	defer b.close()
	for _, name := range []string{tool.Delegate, tool.TeamStatus, tool.Hire, tool.Release, tool.DreamStatus, tool.Present} {
		if _, ok := b.offered(name); !ok {
			t.Fatalf("authorized manager cannot use %s", name)
		}
	}
	if _, ok := b.offered(tool.Report); ok {
		t.Fatal("Dream manager can bypass presentation with a final report")
	}
	if _, ok := b.offered(tool.Bash); ok {
		t.Fatal("governance bridge exposes an unrestricted execution tool")
	}
}

func TestRuntimeGovernanceWaitActionsYieldWithoutFinishingTask(t *testing.T) {
	for _, method := range []string{protocol.MTeamsAgentPlan, protocol.MTeamsAgentHelp, protocol.MTeamsAgentEscalate} {
		t.Run(method, func(t *testing.T) {
			opts := runtimeTestOptions(t, protocol.MemberRuntimeCLI)
			opts.Tools.Team.Runtime.Command = []string{"agent"}
			host := &runtimeTestHost{}
			f := runtimeTestFactory(host, nil)
			defer f.Close()
			r := runtimeTestNew(t, f, opts)
			bridge := &runtimeBridge{}
			boundary := runtimeBridgeHost{r: r, bridge: bridge}
			if err := boundary.Call(context.Background(), method, map[string]any{"session": "forged", "token": "forged", "title": "Need a decision"}, nil); err != nil {
				t.Fatal(err)
			}
			if !bridge.yielded || bridge.reported {
				t.Fatal("governance wait was treated as task completion")
			}
			host.mu.Lock()
			called := host.methods[0]
			params := append(json.RawMessage(nil), host.params[0]...)
			host.mu.Unlock()
			var identity protocol.AgentRef
			if err := json.Unmarshal(params, &identity); err != nil {
				t.Fatal(err)
			}
			if called != method || host.count() != 1 || identity.Session != opts.Session.ID || identity.Token != opts.Tools.Team.Token {
				t.Fatal("governance action lost its live run identity or emitted an extra report")
			}
		})
	}
}

func TestNativeReportEndsOnlyAnAcceptedTurn(t *testing.T) {
	for _, rejected := range []bool{false, true} {
		t.Run(map[bool]string{false: "accepted", true: "rejected"}[rejected], func(t *testing.T) {
			g := &scriptedGateway{replies: []scriptedReply{
				scriptedTurn(llm.FinishToolCalls, "", llm.ToolCall{ID: "report", Name: tool.Report, Arguments: `{"title":"Checked input","body":"The input is verified."}`}),
				scriptedTurn(llm.FinishStop, "Continue only when the report was rejected."),
			}}
			host := &runtimeTestHost{fail: rejected}
			f := New(Deps{Gateway: g, Host: func(protocol.SessionInfo) tool.Host { return host }, Tools: tool.NewRegistry(rashin.Tools()...)})
			defer f.Close()
			options := runtimeTestOptions(t, protocol.MemberRuntimeNative)
			options.Session.Kind, options.Session.Model, options.Session.TeamTask = protocol.KindAgent, "openrouter/test", "task"
			runner, err := f.NewAgent(context.Background(), options)
			if err != nil {
				t.Fatal(err)
			}
			result, err := runner.Prompt(context.Background(), core.PromptInput{Turn: 1, Text: "Hand in the verified report."}, &engineSink{})
			if err != nil {
				t.Fatal(err)
			}
			expected := 1
			if rejected {
				expected = 2
			}
			if result.Stop != protocol.StopEnd || len(g.requests) != expected {
				t.Fatalf("report acceptance did not govern turn termination: stop=%q model calls=%d, want %d", result.Stop, len(g.requests), expected)
			}
		})
	}
}

func TestNativeDelegationEndsOnlyAnAcceptedTurn(t *testing.T) {
	for _, rejected := range []bool{false, true} {
		t.Run(map[bool]string{false: "accepted", true: "rejected"}[rejected], func(t *testing.T) {
			g := &scriptedGateway{replies: []scriptedReply{
				scriptedTurn(llm.FinishToolCalls, "",
					llm.ToolCall{ID: "delegate", Name: tool.Delegate, Arguments: `{"member":"report","title":"One piece","brief":"Check only the assigned input."}`},
					llm.ToolCall{ID: "after-handoff", Name: tool.TeamStatus, Arguments: `{}`}),
				scriptedTurn(llm.FinishStop, "Continue only when delegation was refused."),
			}}
			host := &runtimeTestHost{fail: rejected}
			f := New(Deps{Gateway: g, Host: func(protocol.SessionInfo) tool.Host { return host }, Tools: tool.NewRegistry(rashin.Tools()...)})
			defer f.Close()
			options := runtimeTestOptions(t, protocol.MemberRuntimeNative)
			options.Session.Kind, options.Session.Model, options.Session.TeamTask = protocol.KindAgent, "openrouter/test", "task"
			options.Tools.Team.Lead, options.Tools.Team.Delegate = true, true
			runner, err := f.NewAgent(context.Background(), options)
			if err != nil {
				t.Fatal(err)
			}
			result, err := runner.Prompt(context.Background(), core.PromptInput{Turn: 1, Text: "Delegate the reviewed piece and wait for its report."}, &engineSink{})
			if err != nil {
				t.Fatal(err)
			}
			expected := 1
			if rejected {
				expected = 2
			}
			if result.Stop != protocol.StopEnd || len(g.requests) != expected {
				t.Fatalf("delegation acceptance did not govern turn termination: stop=%q model calls=%d, want %d", result.Stop, len(g.requests), expected)
			}
			if !rejected && host.count() != 1 {
				t.Fatal("accepted delegation retained authority to run a later tool in the same response")
			}
		})
	}
}
