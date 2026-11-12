package harness

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"ryoku-rashin/internal/brief"
	"ryoku-rashin/internal/core"
	"ryoku-rashin/internal/protocol"
	"ryoku-rashin/internal/sandbox"
)

// fakeAskTurn is a turn of the scripted agent an Ask runs on: it says
// whether it heard Ask's brief and a changed mode, then asks to read a file
func fakeAskTurn(write func(any), params json.RawMessage) {
	var prompt struct {
		Prompt []struct {
			Text string `json:"text"`
		} `json:"prompt"`
	}
	_ = json.Unmarshal(params, &prompt)
	heard := make([]string, 0, 2)
	for _, block := range prompt.Prompt {
		if strings.Contains(block.Text, "This is Ask") && strings.Contains(block.Text, "End of context from Rashin.") {
			heard = append(heard, "briefed")
		}
		if strings.Contains(block.Text, "the user changed what this conversation may do") {
			heard = append(heard, "changed")
		}
	}
	writeUpdate(write, map[string]any{"sessionUpdate": "agent_message_chunk", "content": map[string]string{"type": "text", "text": strings.Join(heard, " ") + ";"}})
	writeAskPermission(write, 100, "read-1", "read", map[string]string{"path": "/tmp/value"})
}

// fakeAskAnswered says what each answer was and makes the next call: an
// edit, a destructive command, then a terminal that writes a file
func fakeAskAnswered(write func(any), message rpcMessage, promptID json.RawMessage) {
	answer := "refused"
	if message.Error == nil {
		var result struct {
			Outcome struct {
				OptionID string `json:"optionId"`
			} `json:"outcome"`
			TerminalID string `json:"terminalId"`
		}
		_ = json.Unmarshal(message.Result, &result)
		answer = result.Outcome.OptionID
		if result.TerminalID != "" {
			answer = "ran"
		}
	}
	say := func(text string) {
		writeUpdate(write, map[string]any{"sessionUpdate": "agent_message_chunk", "content": map[string]string{"type": "text", "text": text}})
	}
	switch rpcID(message.ID) {
	case "100":
		say("read=" + answer + ";")
		writeAskPermission(write, 101, "edit-1", "edit", map[string]string{"path": "/tmp/value"})
	case "101":
		say("edit=" + answer + ";")
		writeAskPermission(write, 102, "exec-1", "execute", map[string]string{"command": "rm -rf /"})
	case "102":
		say("exec=" + answer + ";")
		write(map[string]any{"jsonrpc": "2.0", "id": 103, "method": "terminal/create", "params": map[string]any{
			"sessionId": "fake-session", "command": "sh", "args": []string{"-c", "printf made > " + os.Getenv("RASHIN_ACP_TEST_FILE")},
		}})
	case "103":
		say("terminal=" + answer + ";")
		writePromptResult(write, promptID, "end_turn")
	}
}

func writeAskPermission(write func(any), id int, call, kind string, input map[string]string) {
	write(map[string]any{"jsonrpc": "2.0", "id": id, "method": "session/request_permission", "params": map[string]any{
		"sessionId": "fake-session",
		"toolCall":  map[string]any{"toolCallId": call, "title": call, "kind": kind, "rawInput": input},
		"options": []map[string]string{
			{"optionId": "allow", "name": "Allow once", "kind": "allow_once"},
			{"optionId": "always", "name": "Always allow", "kind": "allow_always"},
			{"optionId": "reject", "name": "Reject", "kind": "reject_once"},
		},
	}})
}

// askOnFake is an Ask answered by the scripted agent, starting in mode; the
// sink holds the session's mode, which the runner reads as Ask's
func askOnFake(t *testing.T, mode string) (askRunner, *testSink, string) {
	t.Helper()
	t.Setenv("RASHIN_ACP_TEST_MODE", "options-ask")
	made := filepath.Join(t.TempDir(), "made")
	t.Setenv("RASHIN_ACP_TEST_FILE", made)
	sink := newTestSink()
	sink.info = protocol.SessionInfo{ID: "ask-on-fake", Kind: protocol.KindAsk, Harness: "fake", Mode: mode}
	current := func() protocol.SessionInfo {
		sink.mu.Lock()
		defer sink.mu.Unlock()
		return sink.info
	}
	r := askRunner{newRunner(core.HarnessOptions{
		Session: current(), Harness: "fake", Cwd: t.TempDir(), Ask: true,
		Approvals: func() string { return current().Mode },
		Brief:     func() string { return brief.Ask(brief.Session{Info: current()}) },
	}, []string{os.Args[0], "-test.run=^TestACPHelperProcess$"})}
	t.Cleanup(func() { _ = r.Stop() })
	return r, sink, made
}

func answerText(sink *testSink) string {
	sink.mu.Lock()
	defer sink.mu.Unlock()
	var text strings.Builder
	for _, event := range sink.events {
		if event.kind == protocol.EvText {
			text.WriteString(event.data.(protocol.TextData).Delta)
		}
	}
	return text.String()
}

func TestAReadOnlyAskOnAHarnessReadsAndRefusesWhatChanges(t *testing.T) {
	r, sink, made := askOnFake(t, protocol.ApprovalsReadOnly)
	result, err := r.Prompt(context.Background(), core.PromptInput{Turn: 1, Text: "What kernel runs here?"}, sink)
	if err != nil || result.Stop != protocol.StopEnd {
		t.Fatalf("the turn ended %#v, %v", result, err)
	}
	if got, want := answerText(sink), "briefed;read=allow;edit=reject;exec=reject;terminal=refused;"; got != want {
		t.Fatalf("the agent heard %q, want %q", got, want)
	}
	if _, err := os.Stat(made); !os.IsNotExist(err) {
		t.Fatalf("a Read only Ask ran a command that writes: %v", err)
	}
	// each refusal shows as a request Rashin answered itself, never one left to the user
	if refused, answered := countOf(sink, protocol.EvPermission), countOf(sink, protocol.EvPermissionResolved); refused != 3 || answered < 3 {
		t.Fatalf("the user was shown %d refusals, %d answered; want 3", refused, answered)
	}
	if last := lastOf[protocol.PermissionResolvedData](t, sink, protocol.EvPermissionResolved); last.By != "policy" {
		t.Fatalf("a refusal was left to %q", last.By)
	}
	if shown := lastOf[protocol.PermissionData](t, sink, protocol.EvPermission); !strings.Contains(shown.Input, "Read only") {
		t.Fatalf("the refusal reads %q", shown.Input)
	}
}

func TestAnAskFirstAskOnAHarnessPutsChangesToTheUser(t *testing.T) {
	r, sink, made := askOnFake(t, protocol.ApprovalsAsk)
	done := make(chan error, 1)
	go func() {
		_, err := r.Prompt(context.Background(), core.PromptInput{Turn: 1, Text: "Make the file."}, sink)
		done <- err
	}()
	edit := sink.waitKind(t, protocol.EvPermission, 1)[0].data.(protocol.PermissionData)
	if edit.Call != "edit-1" {
		t.Fatalf("the first question to the user is %q; the read should have run unasked", edit.Call)
	}
	// "always" stays with Rashin: the harness hears once and asks again
	if err := r.Permission(edit.Request, "always"); err != nil {
		t.Fatal(err)
	}
	exec := sink.waitKind(t, protocol.EvPermission, 2)[1].data.(protocol.PermissionData)
	for _, option := range exec.Options {
		if option.Kind == protocol.OptAllowAlways {
			t.Fatalf("a destructive command offers %q", option.Label)
		}
	}
	if err := r.Permission(exec.Request, "reject"); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the turn never ended")
	}
	if got, want := answerText(sink), "briefed;read=allow;edit=allow;exec=reject;terminal=ran;"; got != want {
		t.Fatalf("the agent heard %q, want %q", got, want)
	}
	if !madeSoon(made) {
		t.Fatal("the allowed terminal wrote nothing")
	}
}

func TestAnAskOnAHarnessShowsItsModelsAndKeepsItsOwnMode(t *testing.T) {
	r, sink, _ := askOnFake(t, protocol.ApprovalsReadOnly)
	if err := r.Start(context.Background(), sink); err != nil {
		t.Fatal(err)
	}
	model := lastOf[protocol.ChoiceData](t, sink, protocol.EvModel)
	if model.Current != "big" || len(model.Options) != 2 || model.Options[1].ID != "small" {
		t.Fatalf("the model list reads %#v", model)
	}
	if modes := countOf(sink, protocol.EvMode); modes != 0 {
		t.Fatalf("the harness's own mode reached the Ask %d times", modes)
	}
	if err := r.SetModel(context.Background(), "small", sink); err != nil {
		t.Fatal(err)
	}
	if err := r.SetMode(context.Background(), protocol.ApprovalsAuto, sink); err != nil {
		t.Fatal(err)
	}
	sink.mu.Lock()
	info := sink.info
	sink.mu.Unlock()
	if info.Model != "small" || info.Mode != protocol.ApprovalsAuto {
		t.Fatalf("the Ask reads model %q mode %q", info.Model, info.Mode)
	}
	if err := r.SetMode(context.Background(), "plan", sink); err == nil {
		t.Fatal("an Ask took the harness's own mode")
	}
}

func TestYOLOOnAHarnessStillAsksBeforeDestroyingAndHearsTheNewMode(t *testing.T) {
	r, sink, made := askOnFake(t, protocol.ApprovalsReadOnly)
	if _, err := r.Prompt(context.Background(), core.PromptInput{Turn: 1, Text: "Look."}, sink); err != nil {
		t.Fatal(err)
	}
	if err := r.SetMode(context.Background(), protocol.ApprovalsAuto, sink); err != nil {
		t.Fatal(err)
	}
	if !r.Alive() {
		t.Fatal("the harness stopped")
	}
	first := len(answerText(sink))
	done := make(chan error, 1)
	go func() {
		_, err := r.Prompt(context.Background(), core.PromptInput{Turn: 2, Text: "Now change it."}, sink)
		done <- err
	}()
	// the Read only turn left its three refusals before this
	exec := sink.waitKind(t, protocol.EvPermission, 4)[3].data.(protocol.PermissionData)
	if exec.Call != "exec-1" {
		t.Fatalf("YOLO asked about %q", exec.Call)
	}
	if err := r.Permission(exec.Request, "reject"); err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if got, want := answerText(sink)[first:], "changed;read=allow;edit=allow;exec=reject;terminal=ran;"; got != want {
		t.Fatalf("the second turn heard %q, want %q", got, want)
	}
	if !madeSoon(made) {
		t.Fatal("YOLO did not run the terminal")
	}
}

// madeSoon waits for the scripted terminal's file, which it writes after the
// agent has moved on
func madeSoon(path string) bool {
	for range 100 {
		if body, err := os.ReadFile(path); err == nil && string(body) == "made" {
			return true
		}
		time.Sleep(20 * time.Millisecond)
	}
	return false
}

// fakeAskWrites writes from the agent's own process, asking no one, as some
// harnesses do: the test's file, a file in its state under HOME and one in
// /tmp, then says which worked
func fakeAskWrites(write func(any)) {
	tried := func(path string) string {
		if err := os.WriteFile(path, []byte("made"), 0o600); err != nil {
			return "failed"
		}
		return "ok"
	}
	private := "/tmp/rashin-ask-writes-" + strconv.Itoa(os.Getpid())
	said := "file=" + tried(os.Getenv("RASHIN_ACP_TEST_FILE")) +
		";state=" + tried(filepath.Join(os.Getenv("HOME"), ".omp", "made")) +
		";tmp=" + tried(private) + ";"
	_ = os.Remove(private)
	writeUpdate(write, map[string]any{"sessionUpdate": "agent_message_chunk", "content": map[string]string{"type": "text", "text": said}})
}

func TestAReadOnlyAskHoldsWhatTheHarnessWritesItselfAndYOLODoesNot(t *testing.T) {
	if err := sandbox.Available(); err != nil {
		t.Skipf("no sandbox here: %v", err)
	}
	t.Setenv("RASHIN_ACP_TEST_MODE", "ask-writes")
	root := t.TempDir()
	home, folder, runtime := filepath.Join(root, "home"), filepath.Join(root, "folder"), filepath.Join(root, "run")
	for _, dir := range []string{filepath.Join(home, ".omp"), folder, runtime} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	made := filepath.Join(folder, "made")
	t.Setenv("RASHIN_ACP_TEST_FILE", made)
	sink := newTestSink()
	sink.info = protocol.SessionInfo{ID: "ask-writes", Kind: protocol.KindAsk, Harness: "omp", Mode: protocol.ApprovalsReadOnly}
	current := func() protocol.SessionInfo {
		sink.mu.Lock()
		defer sink.mu.Unlock()
		return sink.info
	}
	environment := map[string]string{"HOME": home}
	// the daemon's askSandbox, with the test binary shown again as Rashin's programs are
	base := sandbox.Spec{Dir: folder, ReadOnly: []string{filepath.Dir(os.Args[0]), root}, RuntimeDir: runtime}
	r := askRunner{newRunner(core.HarnessOptions{
		Session: current(), Harness: "omp", Cwd: folder, Ask: true,
		Approvals: func() string { return current().Mode },
	}, []string{os.Args[0], "-test.run=^TestACPHelperProcess$"},
		withEnvironment(environment), withAskSandbox(askSandboxFor(base, "omp", []string{"omp", "acp"}, environment)))}
	t.Cleanup(func() { _ = r.Stop() })

	if _, err := r.Prompt(context.Background(), core.PromptInput{Turn: 1, Text: "Write it."}, sink); err != nil {
		t.Fatal(err)
	}
	if got, want := answerText(sink), "file=failed;state=ok;tmp=ok;"; got != want {
		t.Fatalf("in Read only the harness said %q, want %q", got, want)
	}
	if _, err := os.Stat(made); !os.IsNotExist(err) {
		t.Fatalf("a Read only Ask's harness wrote the file itself: %v", err)
	}
	if _, err := os.Stat(filepath.Join(home, ".omp", "made")); err != nil {
		t.Fatalf("the harness could not keep its own state: %v", err)
	}

	if err := r.SetMode(context.Background(), protocol.ApprovalsAuto, sink); err != nil {
		t.Fatal(err)
	}
	first := len(answerText(sink))
	if _, err := r.Prompt(context.Background(), core.PromptInput{Turn: 2, Text: "Now write it."}, sink); err != nil {
		t.Fatal(err)
	}
	if got, want := answerText(sink)[first:], "file=ok;state=ok;tmp=ok;"; got != want {
		t.Fatalf("in YOLO the harness said %q, want %q", got, want)
	}
	if body, err := os.ReadFile(made); err != nil || string(body) != "made" {
		t.Fatalf("YOLO left the file %q, %v", body, err)
	}

	// back to Read only puts the harness back in its sandbox
	_ = os.Remove(made)
	if err := r.SetMode(context.Background(), protocol.ApprovalsReadOnly, sink); err != nil {
		t.Fatal(err)
	}
	second := len(answerText(sink))
	if _, err := r.Prompt(context.Background(), core.PromptInput{Turn: 3, Text: "Again."}, sink); err != nil {
		t.Fatal(err)
	}
	if got := answerText(sink)[second:]; !strings.HasPrefix(got, "file=failed;") {
		t.Fatalf("back in Read only the harness said %q", got)
	}
}
