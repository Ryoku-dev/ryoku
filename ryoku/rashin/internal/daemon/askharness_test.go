package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"ryoku-rashin/internal/client"
	"ryoku-rashin/internal/core"
	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/protocol"
)

// failingHarnesses is a harness that cannot start, as one not installed
type failingHarnesses struct{ fakeHarnesses }

func (f *failingHarnesses) NewAgent(context.Context, core.HarnessOptions) (core.Runner, error) {
	return nil, errors.New("The selected agent harness is not installed. Install it and try again.")
}

func askHarnessManager(t *testing.T, harnesses core.Harnesses, asked *bool) *sessionManager {
	t.Helper()
	isolateDaemon(t)
	settings := func() protocol.Settings {
		return protocol.Settings{Ask: protocol.AskSettings{Model: protocol.RouteAuto, Harness: "codex"}, Agent: protocol.AgentSettings{Approvals: protocol.ApprovalsAuto, Harness: "claude"}}
	}
	asker := &fakeRunner{prompt: func(context.Context, core.PromptInput, core.Sink) (core.TurnResult, error) {
		*asked = true
		return core.TurnResult{Stop: protocol.StopEnd}, nil
	}}
	manager := newSessionManager(fakeAsker{runner: asker}, harnesses, nil, settings, func(string, any) {}, func() {}, func(string, ...any) {})
	manager.askBrief = func(info protocol.SessionInfo) string { return "ask brief in " + info.Mode }
	manager.brief = func(protocol.SessionInfo) string { return "agent brief" }
	manager.tools = func(protocol.SessionInfo) []core.MCPServer { return []core.MCPServer{{Name: "rashin"}} }
	return manager
}

// An Ask the app starts on a harness runs through that harness: from the
// home folder, on the harness's own model, with Ask's brief, Rashin's tools
// and Ask's own mode deciding its permissions, never the agent approvals.
func TestAnAskOnAHarnessRunsThroughItWithAsksBriefAndMode(t *testing.T) {
	runner := &fakeRunner{prompt: func(_ context.Context, _ core.PromptInput, sink core.Sink) (core.TurnResult, error) {
		sink.Emit(protocol.EvText, protocol.TextData{Message: "m", Delta: "from the harness"})
		return core.TurnResult{Stop: protocol.StopEnd}, nil
	}}
	harnesses := &fakeHarnesses{runner: runner}
	asked := false
	manager := askHarnessManager(t, harnesses, &asked)
	info, err := manager.create(protocol.SessionNewParams{Kind: protocol.KindAsk, Harness: "omp", Mode: protocol.ApprovalsReadOnly})
	if err != nil {
		t.Fatal(err)
	}
	if info.Harness != "omp" || info.Cwd != paths.Home() || info.Model != "" {
		t.Fatalf("the Ask was made as %#v", info)
	}
	collector := newCollector()
	if _, err := manager.attach(info.ID, 0, collector); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Prompt(context.Background(), protocol.PromptParams{ID: info.ID, Text: "What kernel runs here?"}); err != nil {
		t.Fatal(err)
	}
	events := waitTurnEnd(t, collector)
	if asked {
		t.Fatal("Rashin's own Ask answered a session a harness answers")
	}
	answered := false
	for _, event := range events {
		if event.Kind == protocol.EvText && strings.Contains(string(event.Data), "from the harness") {
			answered = true
		}
	}
	if !answered {
		t.Fatalf("the harness's answer never streamed: %#v", events)
	}
	opts := harnesses.opts
	if !opts.Ask || opts.Harness != "omp" || opts.Cwd != paths.Home() || len(opts.Tools) != 1 {
		t.Fatalf("the harness was started with %#v", opts)
	}
	if got := opts.Approvals(); got != protocol.ApprovalsReadOnly {
		t.Fatalf("the harness is held to %q, not the Ask's mode", got)
	}
	if got := opts.Brief(); got != "ask brief in read-only" {
		t.Fatalf("the harness was briefed %q", got)
	}
	if _, err := manager.Prompt(context.Background(), protocol.PromptParams{ID: info.ID, Run: "uname -r"}); err == nil {
		t.Fatal("a harness Ask took a command to run as Rashin's Ask does")
	}
}

// An Ask made without a harness stays on Rashin's providers whatever the
// settings say: the app passes the harness, so bar and terminal asks never
// leave the providers, and sessions made before keep their engine.
func TestAnAskWithoutAHarnessStaysOnTheProviders(t *testing.T) {
	harnesses := &fakeHarnesses{}
	asked := false
	manager := askHarnessManager(t, harnesses, &asked)
	info, err := manager.create(protocol.SessionNewParams{Kind: protocol.KindAsk, Origin: protocol.OriginBar})
	if err != nil {
		t.Fatal(err)
	}
	if info.Harness != "" || info.Model != protocol.RouteAuto {
		t.Fatalf("a bar Ask was made as %#v", info)
	}
	collector := newCollector()
	if _, err := manager.attach(info.ID, 0, collector); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Prompt(context.Background(), protocol.PromptParams{ID: info.ID, Text: "hi"}); err != nil {
		t.Fatal(err)
	}
	waitTurnEnd(t, collector)
	if !asked || harnesses.opts.Harness != "" {
		t.Fatalf("the Ask answered %v, a harness was started with %#v", asked, harnesses.opts)
	}
}

// A harness that cannot start says why, and nothing else answers.
func TestAnAskWhoseHarnessCannotStartSaysWhyAndNothingElseAnswers(t *testing.T) {
	asked := false
	manager := askHarnessManager(t, &failingHarnesses{}, &asked)
	info, err := manager.create(protocol.SessionNewParams{Kind: protocol.KindAsk, Harness: "codex"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = manager.Prompt(context.Background(), protocol.PromptParams{ID: info.ID, Text: "hi"})
	if err == nil || !strings.Contains(err.Error(), "Codex answers this Ask and cannot start") || !strings.Contains(err.Error(), "not installed") {
		t.Fatalf("the failure reads %v", err)
	}
	if asked {
		t.Fatal("Rashin's own Ask answered in the harness's place")
	}
}

// Which harness answers Ask is the app's to change, as the harness
// instances are.
func TestOnlyTheAppChoosesTheHarnessThatAnswersAsk(t *testing.T) {
	for patch, appOnly := range map[string]bool{
		`{"ask":{"harness":"omp"}}`:       true,
		`{"ask":{"model":"auto:fast"}}`:   false,
		`{"ask":null}`:                    true,
		`{"agent":{"approvals":"ask"}}`:   false,
		`{"agent":{"instances":[]}}`:      true,
		`{"ask":{"approvals":"ask"}}`:     false,
		`{"ask":{"harness":""}}`:          true,
		`{"skills":{"off":[]}}`:           true,
		`{"index":{"semantic":false}}`:    false,
		`{"ask":{"habits":false,"x":1}}`:  false,
		`[]`:                              true,
		`{"agent":"replaced everything"}`: true,
	} {
		if got := settingsForApp([]byte(patch)); got != appOnly {
			t.Errorf("%s: app only %v, want %v", patch, got, appOnly)
		}
	}
}

// A read-only Ask's harness runs with the whole host read-only and nothing
// hidden, and reaches the daemon only through the Ask's own socket: there it
// reads, searches and proposes as Ask's tools do, and starts or steers
// nothing that could act.
func TestAReadOnlyAsksOwnSocketServesOnlyWhatAsksToolsCall(t *testing.T) {
	isolateDaemon(t)
	shortRuntime(t)
	crew := &boundTeams{handled: make(chan string, 8)}
	memory := &recordingMemory{}
	d := New(Deps{Teams: crew, Memory: memory})
	ask, err := d.manager.create(protocol.SessionNewParams{Kind: protocol.KindAsk, Harness: "omp", Mode: protocol.ApprovalsReadOnly})
	if err != nil {
		t.Fatal(err)
	}
	spec, err := d.askSandbox(ask)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(d.closeAgentSockets)
	runtime := paths.Resolved(paths.RuntimeDir())
	home := paths.Resolved(paths.Home())
	if spec.Folder != "" || spec.Dir != home || len(spec.Writable) != 0 || len(spec.Hidden) != 0 || !slices.Contains(spec.ReadOnly, home) {
		t.Fatalf("the Ask's sandbox reads %+v", spec)
	}
	if len(spec.Binds) != 1 || spec.Binds[0].Target != runtime || spec.Env["RASHIN_SOCKET"] != filepath.Join(runtime, "rashin.sock") {
		t.Fatalf("the Ask's socket is not where the daemon's is inside: %+v", spec)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, err := client.Dial(ctx, client.Options{Client: protocol.ClientCLI, Socket: paths.AgentSocket(ask.ID)})
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if err := conn.Call(ctx, protocol.MTeamsList, protocol.Empty{}, nil); err != nil {
		t.Fatalf("reading the teams: %v", err)
	}
	if method := <-crew.handled; method != protocol.MTeamsList {
		t.Fatalf("Teams heard %s", method)
	}
	// its memory is this conversation's, whatever the call names
	if err := conn.Call(ctx, protocol.MMemorySearch, map[string]string{"query": "kernel", "session": "someone-else"}, nil); err != nil {
		t.Fatalf("searching memory: %v", err)
	}
	if heard := memory.heard(); len(heard) != 1 || heard[0].from != (core.MemoryCaller{Session: ask.ID}) {
		t.Fatalf("memory heard %+v", heard)
	}
	for _, refused := range []struct {
		method string
		params any
	}{
		{protocol.MSessionNew, protocol.SessionNewParams{Kind: protocol.KindAgent, Cwd: paths.Home()}},
		{protocol.MSessionGoal, protocol.GoalParams{ID: ask.ID, Op: protocol.GoalGet}},
		{protocol.MSessionSetMode, protocol.SetModeParams{ID: ask.ID, Mode: protocol.ApprovalsAuto}},
		{protocol.MSessionPrompt, protocol.PromptParams{ID: ask.ID, Text: "write it"}},
		{protocol.MAsk, protocol.AskParams{Question: "write it"}},
		{protocol.MSettingsSet, protocol.SettingsSetParams{Patch: json.RawMessage(`{"ask":{"approvals":"auto"}}`)}},
		{protocol.MTeamsCreate, protocol.TeamCreateParams{}},
		{protocol.MIndexAdd, protocol.IndexAddParams{Path: paths.Home()}},
	} {
		if err := conn.Call(ctx, refused.method, refused.params, nil); !forbidden(err) {
			t.Fatalf("%s on a read-only Ask's socket: %v, want forbidden", refused.method, err)
		}
	}
}
