package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"reflect"
	"sync"
	"testing"
	"time"
)

// fakeAgent scripts the other end of the ACP wire: it reads client lines from
// r and writes agent lines to w, mimicking hermes acp closely enough to prove
// the handshake, streaming translation, permission round trip, and cancel.
type fakeAgent struct {
	lines chan rpcMsg
	w     io.Writer
	t     *testing.T
}

// read pops the next client frame. A dedicated goroutine drains the pipe so
// client writes never block on the synchronous io.Pipe (a real hermes stdin
// is OS-buffered; the fake must not be stricter than the real thing).
func (f *fakeAgent) read() rpcMsg {
	f.t.Helper()
	select {
	case m, ok := <-f.lines:
		if !ok {
			f.t.Fatal("fake agent: client closed early")
		}
		return m
	case <-time.After(3 * time.Second):
		f.t.Fatal("fake agent: timeout waiting for client frame")
	}
	return rpcMsg{}
}

func (f *fakeAgent) write(v any) {
	f.t.Helper()
	b, _ := json.Marshal(v)
	if _, err := f.w.Write(append(b, '\n')); err != nil {
		f.t.Fatalf("fake agent write: %v", err)
	}
}

func (f *fakeAgent) respond(id int64, result any) {
	r, _ := json.Marshal(result)
	f.write(rpcMsg{JSONRPC: "2.0", ID: &id, Result: r})
}

func (f *fakeAgent) update(session string, update map[string]any) {
	p, _ := json.Marshal(map[string]any{"sessionId": session, "update": update})
	f.write(rpcMsg{JSONRPC: "2.0", Method: "session/update", Params: p})
}

func newTestPair(t *testing.T) (*acpConn, *fakeAgent) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	cr, cw := io.Pipe() // client -> agent
	ar, aw := io.Pipe() // agent -> client
	conn := newACPConn(cw, ar, cw)
	fa := &fakeAgent{lines: make(chan rpcMsg, 64), w: aw, t: t}
	go func() {
		sc := bufio.NewScanner(cr)
		for sc.Scan() {
			var m rpcMsg
			if json.Unmarshal(sc.Bytes(), &m) == nil {
				fa.lines <- m
			}
		}
		close(fa.lines)
	}()
	return conn, fa
}

func expectEvent(t *testing.T, ch <-chan AcpEvent, typ string) AcpEvent {
	t.Helper()
	select {
	case ev, ok := <-ch:
		if !ok {
			t.Fatalf("event channel closed waiting for %s", typ)
		}
		if ev.Type != typ {
			t.Fatalf("got event %q, want %q (%+v)", ev.Type, typ, ev)
		}
		return ev
	case <-time.After(3 * time.Second):
		t.Fatalf("timeout waiting for %s event", typ)
	}
	return AcpEvent{}
}

func TestACPHandshakePromptStreamPermissionCancel(t *testing.T) {
	conn, fa := newTestPair(t)
	defer conn.Close()

	done := make(chan error, 1)
	go func() { done <- conn.Initialize("/tmp/vault") }()

	init := fa.read()
	if init.Method != "initialize" {
		t.Fatalf("first request %q, want initialize", init.Method)
	}
	fa.respond(*init.ID, map[string]any{"protocolVersion": 1})

	newSess := fa.read()
	if newSess.Method != "session/new" {
		t.Fatalf("second request %q, want session/new", newSess.Method)
	}
	var np struct {
		Cwd string `json:"cwd"`
	}
	_ = json.Unmarshal(newSess.Params, &np)
	if np.Cwd != "/tmp/vault" {
		t.Fatalf("session cwd %q, want /tmp/vault", np.Cwd)
	}
	fa.respond(*newSess.ID, map[string]any{"sessionId": "s1"})

	if err := <-done; err != nil {
		t.Fatalf("Initialize: %v", err)
	}

	// A fresh session always emits one models event carrying the agent name.
	expectEvent(t, conn.Events(), "models")

	// One user turn with streaming chunks and a tool call.
	conn.Prompt("hello", nil)
	prompt := fa.read()
	if prompt.Method != "session/prompt" {
		t.Fatalf("got %q, want session/prompt", prompt.Method)
	}
	fa.update("s1", map[string]any{
		"sessionUpdate": "agent_message_chunk",
		"content":       map[string]string{"type": "text", "text": "Hi "},
	})
	fa.update("s1", map[string]any{
		"sessionUpdate": "tool_call", "toolCallId": "t1",
		"title": "Reading system.md", "kind": "read", "status": "in_progress",
	})
	fa.update("s1", map[string]any{
		"sessionUpdate": "tool_call_update", "toolCallId": "t1", "status": "completed",
	})

	ev := expectEvent(t, conn.Events(), "agent_text")
	if ev.Text != "Hi " {
		t.Fatalf("chunk text %q", ev.Text)
	}
	ev = expectEvent(t, conn.Events(), "tool")
	if ev.Tool.ID != "t1" || ev.Tool.Status != "in_progress" {
		t.Fatalf("tool event %+v", ev)
	}
	ev = expectEvent(t, conn.Events(), "tool")
	if ev.Tool.Status != "completed" {
		t.Fatalf("tool update %+v", ev)
	}

	// Permission round trip: agent asks, client answers allow.
	permID := int64(77)
	pp, _ := json.Marshal(map[string]any{
		"sessionId": "s1",
		"toolCall":  map[string]string{"title": "Run ls"},
		"options": []map[string]string{
			{"optionId": "allow", "name": "Allow", "kind": "allow_once"},
			{"optionId": "deny", "name": "Deny", "kind": "reject_once"},
		},
	})
	fa.write(rpcMsg{JSONRPC: "2.0", ID: &permID, Method: "session/request_permission", Params: pp})

	expectEvent(t, conn.Events(), "tool")
	ev = expectEvent(t, conn.Events(), "permission")
	if ev.RequestID != "77" || len(ev.Options) != 2 || ev.PermTitle != "Run ls" {
		t.Fatalf("permission event %+v", ev)
	}
	conn.RespondPermission(77, "allow")
	permResp := fa.read()
	if permResp.ID == nil || *permResp.ID != 77 {
		t.Fatalf("permission response id %+v", permResp.ID)
	}
	var pr struct {
		Outcome struct {
			Outcome  string `json:"outcome"`
			OptionID string `json:"optionId"`
		} `json:"outcome"`
	}
	_ = json.Unmarshal(permResp.Result, &pr)
	if pr.Outcome.Outcome != "selected" || pr.Outcome.OptionID != "allow" {
		t.Fatalf("permission outcome %+v", pr)
	}
	ev = expectEvent(t, conn.Events(), "permission_resolved")
	if ev.RequestID != "77" || ev.Outcome != "allowed" {
		t.Fatalf("resolved event %+v", ev)
	}
	// A second answer (another surface racing) is dropped, not re-sent.
	conn.RespondPermission(77, "deny")

	// Turn end.
	fa.respond(*prompt.ID, map[string]any{"stopReason": "end_turn"})
	ev = expectEvent(t, conn.Events(), "turn_end")
	if ev.StopReason != "end_turn" {
		t.Fatalf("stop reason %q", ev.StopReason)
	}

	// Cancel is a notification.
	conn.Cancel()
	cancel := fa.read()
	if cancel.Method != "session/cancel" || cancel.ID != nil {
		t.Fatalf("cancel frame %+v", cancel)
	}
}

func TestACPInitializeTimesOutAndClearsPendingRequest(t *testing.T) {
	oldTimeout := acpRequestTimeout
	acpRequestTimeout = 50 * time.Millisecond
	t.Cleanup(func() { acpRequestTimeout = oldTimeout })
	conn, agent := newTestPair(t)
	defer conn.Close()

	done := make(chan error, 1)
	go func() { done <- conn.Initialize("/tmp/vault") }()
	request := agent.read()
	if request.Method != "initialize" {
		t.Fatalf("method = %q, want initialize", request.Method)
	}
	select {
	case err := <-done:
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("initialize error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("initialize did not time out")
	}
	conn.mu.Lock()
	pending := len(conn.pending)
	conn.mu.Unlock()
	if pending != 0 {
		t.Fatalf("pending requests after timeout = %d", pending)
	}
}

func TestACPCloseKillsAChildThatDoesNotExit(t *testing.T) {
	oldDelay := acpCloseKillDelay
	acpCloseKillDelay = 10 * time.Millisecond
	t.Cleanup(func() { acpCloseKillDelay = oldDelay })
	killed := make(chan struct{}, 1)
	conn := &acpConn{
		pending: map[int64]chan rpcMsg{},
		killProcess: func() error {
			killed <- struct{}{}
			return nil
		},
	}
	conn.Close()
	select {
	case <-killed:
	case <-time.After(time.Second):
		t.Fatal("close did not kill the child")
	}
}

func TestACPDeadChildEmitsDeadState(t *testing.T) {
	cr, cw := io.Pipe()
	ar, aw := io.Pipe()
	conn := newACPConn(cw, ar, cw)
	_ = cr
	_ = aw.Close() // child dies immediately

	ev := expectEvent(t, conn.Events(), "state")
	if ev.State != "dead" {
		t.Fatalf("state %q, want dead", ev.State)
	}
}

// A prompt still running when the agent exits must not crash the daemon (#595).
func TestACPEmitAfterAgentExitDoesNotPanic(t *testing.T) {
	for range 100 {
		cr, cw := io.Pipe()
		ar, aw := io.Pipe()
		go func() { _, _ = io.Copy(io.Discard, cr) }()
		conn := newACPConn(cw, ar, cw)
		var wg sync.WaitGroup
		for range 4 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for range 100 {
					conn.emit(AcpEvent{Type: "turn_end"})
				}
			}()
		}
		_ = aw.Close()
		go func() {
			for range conn.Events() {
			}
		}()
		wg.Wait()
		conn.Close()
	}
}

func TestACPV2ImagesModelsSessions(t *testing.T) {
	conn, fa := newTestPair(t)
	defer conn.Close()

	done := make(chan error, 1)
	go func() { done <- conn.Initialize("/tmp/vault") }()
	init := fa.read()
	fa.respond(*init.ID, map[string]any{"protocolVersion": 1, "agentCapabilities": map[string]any{"loadSession": true, "promptCapabilities": map[string]any{"image": true}}})
	newSess := fa.read()
	fa.respond(*newSess.ID, map[string]any{
		"sessionId": "s1",
		"models": map[string]any{
			"currentModelId": "openai-codex:gpt-5.5",
			"availableModels": []map[string]string{
				{"modelId": "openai-codex:gpt-5.5", "name": "GPT 5.5"},
				{"modelId": "openai-codex:gpt-5.5-mini", "name": "GPT 5.5 mini"},
			},
		},
	})
	if err := <-done; err != nil {
		t.Fatalf("Initialize: %v", err)
	}

	// Models from session/new surface as a models event.
	ev := expectEvent(t, conn.Events(), "models")
	if ev.CurrentModel != "openai-codex:gpt-5.5" || len(ev.Models) != 2 {
		t.Fatalf("models event %+v", ev)
	}

	// Image blocks ride the prompt.
	conn.Prompt("what is this", []PromptImage{{Data: "aGk=", MimeType: "image/png"}})
	prompt := fa.read()
	var pp struct {
		Prompt []map[string]any `json:"prompt"`
	}
	_ = json.Unmarshal(prompt.Params, &pp)
	if len(pp.Prompt) != 2 || pp.Prompt[1]["type"] != "image" || pp.Prompt[1]["mimeType"] != "image/png" {
		t.Fatalf("prompt blocks %+v", pp.Prompt)
	}
	fa.respond(*prompt.ID, map[string]any{"stopReason": "end_turn"})
	expectEvent(t, conn.Events(), "turn_end")

	// available_commands_update and usage_update translate.
	fa.update("s1", map[string]any{
		"sessionUpdate": "available_commands_update",
		"availableCommands": []map[string]any{
			{"name": "help", "description": "List commands"},
			{"name": "model", "description": "Switch model", "input": map[string]string{"hint": "model id"}},
		},
	})
	ev = expectEvent(t, conn.Events(), "commands")
	if len(ev.Commands) != 2 || ev.Commands[1].Hint != "model id" {
		t.Fatalf("commands event %+v", ev)
	}
	fa.update("s1", map[string]any{"sessionUpdate": "usage_update", "size": 200000, "used": 12345})
	ev = expectEvent(t, conn.Events(), "usage")
	if ev.UsageSize != 200000 || ev.UsageUsed != 12345 {
		t.Fatalf("usage event %+v", ev)
	}

	// set_model round trip.
	setDone := make(chan error, 1)
	go func() { setDone <- conn.SetModel("openai-codex:gpt-5.5-mini") }()
	setReq := fa.read()
	if setReq.Method != "session/set_model" {
		t.Fatalf("got %q, want session/set_model", setReq.Method)
	}
	fa.respond(*setReq.ID, map[string]any{})
	if err := <-setDone; err != nil {
		t.Fatalf("SetModel: %v", err)
	}

	// session/list translates to SessionMeta.
	listDone := make(chan []SessionMeta, 1)
	go func() { listDone <- conn.ListSessions() }()
	listReq := fa.read()
	if listReq.Method != "session/list" {
		t.Fatalf("got %q, want session/list", listReq.Method)
	}
	fa.respond(*listReq.ID, map[string]any{
		"sessions": []map[string]string{
			{"sessionId": "old1", "title": "Keybinds", "updatedAt": "2026-07-02T02:00:00Z"},
		},
	})
	sessions := <-listDone
	if len(sessions) != 1 || sessions[0].Title != "Keybinds" {
		t.Fatalf("sessions %+v", sessions)
	}

	// load replays: replay_start, replayed user chunk, replay_end.
	loadDone := make(chan error, 1)
	go func() { loadDone <- conn.LoadSession("old1") }()
	expectEvent(t, conn.Events(), "replay_start")
	loadReq := fa.read()
	if loadReq.Method != "session/load" {
		t.Fatalf("got %q, want session/load", loadReq.Method)
	}
	fa.update("old1", map[string]any{
		"sessionUpdate": "user_message_chunk",
		"content":       map[string]string{"type": "text", "text": "old question"},
	})
	ev = expectEvent(t, conn.Events(), "user_text")
	if ev.Text != "old question" {
		t.Fatalf("user_text %+v", ev)
	}
	fa.respond(*loadReq.ID, map[string]any{"sessionId": "old1"})
	if err := <-loadDone; err != nil {
		t.Fatalf("LoadSession: %v", err)
	}
	expectEvent(t, conn.Events(), "models")
	expectEvent(t, conn.Events(), "replay_end")
}

func TestMergeHermesSessionsUsesStoredWorkspace(t *testing.T) {
	agent := []SessionMeta{
		{ID: "agent", Title: "Agent", Cwd: ".", UpdatedAt: "old"},
		{ID: "live", Title: "Live", Cwd: "/tmp/live"},
	}
	stored := []SessionMeta{
		{ID: "agent", Title: "Agent", Cwd: "/tmp/rashin", UpdatedAt: "new"},
		{ID: "chat", Title: "Chat", Cwd: "/tmp/rashin-chat"},
	}
	got := mergeHermesSessions(agent, stored)
	if len(got) != 3 {
		t.Fatalf("merged sessions = %+v", got)
	}
	if got[0].ID != "agent" || got[0].Cwd != "/tmp/rashin" || got[0].UpdatedAt != "new" {
		t.Fatalf("stored workspace did not replace ACP placeholder: %+v", got[0])
	}
	if got[1].ID != "chat" || got[2].ID != "live" {
		t.Fatalf("stored and live-only sessions were not retained: %+v", got)
	}
}

func stubChatProwlProfileSets(t *testing.T, lookup func(context.Context) (string, []string, error)) {
	t.Helper()
	previous := chatProwlProfileSets
	chatProwlProfileSets = lookup
	t.Cleanup(func() {
		chatProwlProfileSets = previous
	})
}

func hermesAdvertisedModels() []ModelInfo {
	return []ModelInfo{
		{ID: "openai-codex:gpt-5.6-sol", Name: "GPT 5.6 sol"},
		{ID: "openai-codex:gpt-5.6-mini", Name: "GPT 5.6 mini"},
		{ID: "openai-codex:gpt-5.5", Name: "GPT 5.5"},
		{ID: "openai-codex:gpt-5.5-mini", Name: "GPT 5.5 mini"},
		{ID: "openai-codex:gpt-5.4", Name: "GPT 5.4"},
		{ID: "openai-codex:gpt-5.3", Name: "GPT 5.3"},
		{ID: "openai-codex:gpt-5.2", Name: "GPT 5.2"},
		{ID: "custom:auto", Name: "Custom endpoint · auto", Description: "Provider: Custom endpoint • current"},
		{ID: "prowl:auto", Name: "prowl · auto", Description: "Provider: prowl"},
		{ID: "custom:prowl:auto", Name: "auto", Description: "Provider: prowl"},
	}
}

func hermesSessionResult(models []ModelInfo) map[string]any {
	available := make([]map[string]string, 0, len(models))
	for _, model := range models {
		available = append(available, map[string]string{
			"modelId": model.ID, "name": model.Name, "description": model.Description,
		})
	}
	return map[string]any{
		"sessionId": "s1",
		"models": map[string]any{
			"currentModelId":  "custom:auto",
			"availableModels": available,
		},
	}
}

func TestChatOfferedModelsMergesProwlAliasesAndKeepsHarnessModels(t *testing.T) {
	modelPickTestEnv(t)
	lookups := 0
	stubChatProwlProfileSets(t, func(context.Context) (string, []string, error) {
		lookups++
		return "Everything free", nil, nil
	})
	models := hermesAdvertisedModels()
	conn := &acpConn{
		agentID:      "hermes",
		prowl:        chatAgentRouting{Active: true},
		startedModel: "custom:auto",
	}
	conn.cacheProwlActiveSet(models)
	conn.emitModels(modelState{models: models, current: "custom:auto", ok: true})

	want := []ModelInfo{{ID: "custom:prowl:auto", Name: "Active set", Description: "Prowl · Everything free"}}
	want = append(want, models[:7]...)
	if got := conn.offeredModelsSnapshot(); !reflect.DeepEqual(got, want) {
		t.Fatalf("offered models = %#v, want %#v", got, want)
	}
	if lookups != 1 {
		t.Fatalf("profile lookups = %d, want 1", lookups)
	}
}

func TestChatOfferedModelsLookupFailureUsesPlainProwlDescription(t *testing.T) {
	modelPickTestEnv(t)
	stubChatProwlProfileSets(t, func(context.Context) (string, []string, error) {
		return "", nil, errors.New("gateway unavailable")
	})
	models := hermesAdvertisedModels()
	conn := &acpConn{
		agentID:      "hermes",
		prowl:        chatAgentRouting{Active: true},
		startedModel: "custom:auto",
	}
	conn.cacheProwlActiveSet(models)
	conn.emitModels(modelState{models: models, current: "custom:auto", ok: true})
	got := conn.offeredModelsSnapshot()
	if len(got) != 8 || got[0] != (ModelInfo{ID: "custom:prowl:auto", Name: "Active set", Description: "Prowl"}) {
		t.Fatalf("offered models = %#v", got)
	}
}

func TestModelsEventNormalizesProwlAliasCurrent(t *testing.T) {
	models := hermesAdvertisedModels()
	modelPickTestEnv(t)
	conn := &acpConn{
		agentID:      "hermes",
		prowl:        chatAgentRouting{Active: true},
		startedModel: "custom:auto",
	}
	conn.emitModels(modelState{models: models, current: "custom:auto", ok: true})
	offered := conn.offeredModelsSnapshot()
	if got := conn.modelsEvent(offered, "custom:auto").CurrentModel; got != "custom:prowl:auto" {
		t.Fatalf("alias current = %q, want custom:prowl:auto", got)
	}
	if got := conn.modelsEvent(offered, "openai-codex:gpt-5.6-sol").CurrentModel; got != "openai-codex:gpt-5.6-sol" {
		t.Fatalf("direct current = %q", got)
	}
}

func TestChatOfferedModelsLeavesClaudeListUnchanged(t *testing.T) {
	modelPickTestEnv(t)
	stubChatProwlProfileSets(t, func(context.Context) (string, []string, error) {
		t.Fatal("Claude must not look up Prowl's active set")
		return "", nil, nil
	})
	models := []ModelInfo{
		{ID: "anthropic:opus", Name: "Opus"},
		{ID: "anthropic:sonnet", Name: "Sonnet"},
	}
	conn := &acpConn{agentID: "claude", prowl: chatAgentRouting{Active: true}}
	conn.cacheProwlActiveSet(models)
	conn.emitModels(modelState{models: models, current: models[0].ID, ok: true})
	if got := conn.offeredModelsSnapshot(); !reflect.DeepEqual(got, models) {
		t.Fatalf("offered models = %#v, want %#v", got, models)
	}
}

func TestPendingRoutingMergesMarkedAliasesOnly(t *testing.T) {
	modelPickTestEnv(t)
	stubChatProwlProfileSets(t, func(context.Context) (string, []string, error) {
		return "Everything free", nil, nil
	})
	models := hermesAdvertisedModels()
	conn := &acpConn{
		agentID:      "hermes",
		prowl:        chatAgentRouting{Pending: true, Reason: "no provider"},
		startedModel: "custom:auto",
	}
	conn.cacheProwlActiveSet(models)
	conn.emitModels(modelState{models: models, current: "custom:auto", ok: true})
	want := []ModelInfo{{ID: "custom:prowl:auto", Name: "Active set", Description: "Prowl · Everything free"}}
	want = append(want, models[:8]...)
	event := conn.modelsEvent(conn.offeredModelsSnapshot(), "custom:auto")
	if event.Prowl != "pending" || event.ProwlReason != "no provider" || !reflect.DeepEqual(event.Models, want) {
		t.Fatalf("models event = %#v, want %#v", event, want)
	}
	if event.CurrentModel != "custom:auto" {
		t.Fatalf("current model = %q, want custom:auto", event.CurrentModel)
	}
}

func TestSetModelSendsHarnessModelWhileRouted(t *testing.T) {
	conn, agent := newTestPair(t)
	defer conn.Close()
	conn.agentID = "hermes"
	conn.prowl = chatAgentRouting{Active: true}
	conn.sessionID = "s1"
	conn.sessionReady = true

	done := make(chan error, 1)
	go func() { done <- conn.SetModel("openai-codex:gpt-5.6-sol") }()
	request := agent.read()
	if request.Method != "session/set_model" {
		t.Fatalf("method = %q, want session/set_model", request.Method)
	}
	var params struct {
		ModelID string `json:"modelId"`
	}
	if err := json.Unmarshal(request.Params, &params); err != nil {
		t.Fatal(err)
	}
	if params.ModelID != "openai-codex:gpt-5.6-sol" {
		t.Fatalf("model id = %q", params.ModelID)
	}
	agent.respond(*request.ID, map[string]any{})
	if err := <-done; err != nil {
		t.Fatalf("SetModel: %v", err)
	}
	if got := conn.currentModelsEvent().CurrentModel; got != "openai-codex:gpt-5.6-sol" {
		t.Fatalf("tracked current model = %q", got)
	}
}

func TestSessionOpenReconcilesExplicitModelPick(t *testing.T) {
	for _, test := range []struct {
		name        string
		method      string
		pick        string
		offered     []ModelInfo
		wantSet     bool
		unavailable bool
	}{
		{
			name: "pick is applied after load", method: "session/load",
			pick:    "openai-codex:gpt-5.6-sol",
			offered: []ModelInfo{{ID: "openai-codex:gpt-5.6-sol", Name: "GPT 5.6 sol"}},
			wantSet: true,
		},
		{
			name: "provider prefix proves capped pick available", method: "session/new",
			pick:    "openai-codex:gpt-6-luna",
			offered: []ModelInfo{{ID: "openai-codex:gpt-5.5", Name: "GPT 5.5"}},
			wantSet: true,
		},
		{
			name: "absent provider keeps session model", method: "session/new",
			pick:        "anthropic:opus",
			offered:     []ModelInfo{{ID: "openai-codex:gpt-5.5", Name: "GPT 5.5"}},
			unavailable: true,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			conn, agent := newTestPair(t)
			defer conn.Close()
			conn.agentID = "hermes"
			conn.agentName = "Hermes"
			if err := setModelPick("hermes", test.pick); err != nil {
				t.Fatal(err)
			}
			done := make(chan error, 1)
			go func() {
				done <- conn.openSession(test.method, map[string]any{"cwd": "/tmp/vault", "sessionId": "loaded"})
			}()
			request := agent.read()
			if request.Method != test.method {
				t.Fatalf("method = %q, want %q", request.Method, test.method)
			}
			result := hermesSessionResult(test.offered)
			if test.method == "session/load" {
				result["sessionId"] = ""
			}
			agent.respond(*request.ID, result)
			if test.wantSet {
				setRequest := agent.read()
				if setRequest.Method != "session/set_model" {
					t.Fatalf("method = %q, want session/set_model", setRequest.Method)
				}
				var params struct {
					ModelID string `json:"modelId"`
				}
				if err := json.Unmarshal(setRequest.Params, &params); err != nil {
					t.Fatal(err)
				}
				if params.ModelID != test.pick {
					t.Fatalf("model id = %q, want %q", params.ModelID, test.pick)
				}
				agent.respond(*setRequest.ID, map[string]any{})
			}
			if err := <-done; err != nil {
				t.Fatalf("open session: %v", err)
			}
			event := expectEvent(t, conn.Events(), "models")
			if event.Pick != test.pick || event.PickUnavailable != test.unavailable {
				t.Fatalf("models event = %+v", event)
			}
			if test.wantSet {
				event = expectEvent(t, conn.Events(), "models")
				if event.CurrentModel != test.pick {
					t.Fatalf("reconciled current = %q", event.CurrentModel)
				}
			}
		})
	}
}

func TestPromptWaitsForModelReconciliation(t *testing.T) {
	modelPickTestEnv(t)
	conn, agent := newTestPair(t)
	defer conn.Close()
	conn.agentID = "hermes"
	conn.agentName = "Hermes"
	pick := "openai-codex:gpt-5.6-sol"
	if err := setModelPick("hermes", pick); err != nil {
		t.Fatal(err)
	}

	opened := make(chan error, 1)
	go func() {
		opened <- conn.openSession("session/new", map[string]any{"cwd": "/tmp/vault"})
	}()
	newRequest := agent.read()
	if newRequest.Method != "session/new" {
		t.Fatalf("method = %q, want session/new", newRequest.Method)
	}
	agent.respond(*newRequest.ID, hermesSessionResult([]ModelInfo{
		{ID: pick, Name: "GPT 5.6 sol"},
	}))
	setRequest := agent.read()
	if setRequest.Method != "session/set_model" {
		t.Fatalf("method = %q, want session/set_model", setRequest.Method)
	}

	conn.Prompt("start after the model is ready", nil)
	select {
	case request := <-agent.lines:
		t.Fatalf("sent %q before session/set_model completed", request.Method)
	case <-time.After(125 * time.Millisecond):
	}

	agent.respond(*setRequest.ID, map[string]any{})
	if err := <-opened; err != nil {
		t.Fatalf("open session: %v", err)
	}
	promptRequest := agent.read()
	if promptRequest.Method != "session/prompt" {
		t.Fatalf("method = %q, want session/prompt", promptRequest.Method)
	}
	agent.respond(*promptRequest.ID, map[string]any{"stopReason": "end_turn"})
}

func TestModelUpdateTracksTrueCurrentModel(t *testing.T) {
	modelPickTestEnv(t)
	conn, _ := newTestPair(t)
	defer conn.Close()
	conn.agentID = "hermes"
	conn.models = []ModelInfo{{ID: "openai-codex:gpt-6-luna", Name: "Luna"}}
	conn.offeredModels = append([]ModelInfo(nil), conn.models...)
	params, err := json.Marshal(map[string]any{
		"sessionId": "s1",
		"update": map[string]any{
			"sessionUpdate":  "current_model_update",
			"currentModelId": "openai-codex:gpt-6-luna",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	conn.handleUpdate(params)
	event := expectEvent(t, conn.Events(), "models")
	if event.CurrentModel != "openai-codex:gpt-6-luna" {
		t.Fatalf("updated current = %q", event.CurrentModel)
	}
}
