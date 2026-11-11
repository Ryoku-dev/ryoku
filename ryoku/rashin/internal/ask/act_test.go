package ask

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"ryoku-rashin/internal/core"
	"ryoku-rashin/internal/llm"
	"ryoku-rashin/internal/protocol"
)

// lockedSink records events from the turn and the answer, which arrive on two
// goroutines once a request waits.
type lockedSink struct {
	mu     sync.Mutex
	events []recordedEvent
	info   protocol.SessionInfo
	wake   chan struct{}
}

func newLockedSink() *lockedSink { return &lockedSink{wake: make(chan struct{}, 64)} }

func (s *lockedSink) Emit(kind string, data any) {
	s.mu.Lock()
	s.events = append(s.events, recordedEvent{kind: kind, data: data})
	s.mu.Unlock()
	select {
	case s.wake <- struct{}{}:
	default:
	}
}

func (s *lockedSink) Update(change func(*protocol.SessionInfo)) {
	s.mu.Lock()
	change(&s.info)
	s.mu.Unlock()
}

func (s *lockedSink) of(kind string) []any {
	s.mu.Lock()
	defer s.mu.Unlock()
	var found []any
	for _, event := range s.events {
		if event.kind == kind {
			found = append(found, event.data)
		}
	}
	return found
}

// request waits for the nth permission request of the turn.
func (s *lockedSink) request(t *testing.T, nth int) protocol.PermissionData {
	t.Helper()
	deadline := time.After(5 * time.Second)
	for {
		if asked := s.of(protocol.EvPermission); len(asked) >= nth {
			return asked[nth-1].(protocol.PermissionData)
		}
		select {
		case <-s.wake:
		case <-deadline:
			t.Fatalf("no request number %d was put to the user", nth)
		}
	}
}

// toolOutcome is the last status and output a tool call reported.
func (s *lockedSink) toolOutcome(call string) (status, output string) {
	for _, data := range s.of(protocol.EvTool) {
		tool := data.(protocol.ToolData)
		if tool.Call != call {
			continue
		}
		if tool.Status != "" {
			status = tool.Status
		}
		if tool.Output != "" {
			output = tool.Output
		}
	}
	return status, output
}

func call(id, name string, arguments any) llm.ToolCall {
	encoded, _ := json.Marshal(arguments)
	return llm.ToolCall{ID: id, Name: name, Arguments: string(encoded)}
}

// actingAsk is an Ask in mode whose model makes calls, then says "Done.".
func actingAsk(t *testing.T, mode string, calls ...llm.ToolCall) (*runner, *scriptedClient, *lockedSink) {
	t.Helper()
	client := &scriptedClient{}
	for _, toolCall := range calls {
		client.streams = append(client.streams, finalStream(llm.Delta{ToolCalls: []llm.ToolCall{toolCall}}))
	}
	client.streams = append(client.streams, finalStream(llm.Delta{Text: "Done."}))
	ask := testRunner(t, client).(*runner)
	sink := newLockedSink()
	if err := ask.SetMode(context.Background(), mode, sink); err != nil {
		t.Fatal(err)
	}
	return ask, client, sink
}

func turn(t *testing.T, ask *runner, sink *lockedSink) chan core.TurnResult {
	t.Helper()
	done := make(chan core.TurnResult, 1)
	go func() {
		result, err := ask.Prompt(context.Background(), core.PromptInput{Turn: 1, Text: "Do it"}, sink)
		if err != nil {
			t.Errorf("the turn failed: %v", err)
		}
		done <- result
	}()
	return done
}

func finished(t *testing.T, done chan core.TurnResult) core.TurnResult {
	t.Helper()
	select {
	case result := <-done:
		return result
	case <-time.After(5 * time.Second):
		t.Fatal("the turn never ended")
		return core.TurnResult{}
	}
}

func offered(request llm.Request, name string) bool {
	for _, tool := range request.Tools {
		if tool.Name == name {
			return true
		}
	}
	return false
}

// A read-only Ask is not handed the tools that change the machine, and a model
// that calls one anyway is refused rather than obeyed.
func TestAReadOnlyAskCannotChangeTheMachine(t *testing.T) {
	target := filepath.Join(t.TempDir(), "made")
	ask, client, sink := actingAsk(t, protocol.ApprovalsReadOnly, call("c1", "run_command", map[string]string{"command": "touch " + target, "why": "make it"}))
	finished(t, turn(t, ask, sink))
	if offered(client.calls[0], "run_command") || offered(client.calls[0], "write_file") {
		t.Fatal("a read-only Ask was offered a tool that changes the machine")
	}
	if !strings.Contains(client.calls[0].Messages[0].Content, "you never change it") {
		t.Fatal("a read-only Ask was not told it never changes the machine")
	}
	if _, err := os.Stat(target); err == nil {
		t.Fatal("a read-only Ask ran a command that changes the machine")
	}
	if status, output := sink.toolOutcome("c1"); status != protocol.ToolFailed || !strings.Contains(output, "read-only") {
		t.Fatalf("the refused call reported %q %q", status, output)
	}
}

// With approvals "ask" a change waits for the user, runs once allowed, and the
// model is told what the command printed.
func TestAskFirstWaitsForTheUserBeforeAChange(t *testing.T) {
	target := filepath.Join(t.TempDir(), "made")
	command := "touch " + target + " && echo made"
	ask, client, sink := actingAsk(t, protocol.ApprovalsAsk, call("c1", "run_command", map[string]string{"command": command, "why": "Make the marker file"}))
	done := turn(t, ask, sink)
	request := sink.request(t, 1)
	if _, err := os.Stat(target); err == nil {
		t.Fatal("the command ran before the user answered")
	}
	if request.Title != "Make the marker file" || request.Input != command || request.Call != "c1" {
		t.Fatalf("the request reads %#v", request)
	}
	if len(request.Options) != 3 || request.Options[0].Kind != protocol.OptAllowOnce || request.Options[2].Kind != protocol.OptRejectOnce {
		t.Fatalf("the request offers %#v", request.Options)
	}
	if err := ask.Permission(request.Request, optionAllow); err != nil {
		t.Fatal(err)
	}
	finished(t, done)
	if _, err := os.Stat(target); err != nil {
		t.Fatal("the allowed command did not run")
	}
	if status, output := sink.toolOutcome("c1"); status != protocol.ToolDone || output != "made" {
		t.Fatalf("the call reported %q %q", status, output)
	}
	if !offered(client.calls[0], "run_command") || !strings.Contains(client.calls[0].Messages[0].Content, "allowed you to change this machine") {
		t.Fatal("an Ask that may act was not handed the tool or not told so")
	}
	told := client.calls[1].Messages[len(client.calls[1].Messages)-1]
	if told.Role != llm.RoleTool || told.Content != "made" {
		t.Fatalf("the model was told %#v", told)
	}
}

func TestARefusedChangeDoesNotRunAndTheModelIsTold(t *testing.T) {
	target := filepath.Join(t.TempDir(), "made")
	ask, client, sink := actingAsk(t, protocol.ApprovalsAsk, call("c1", "run_command", map[string]string{"command": "touch " + target, "why": "make it"}))
	done := turn(t, ask, sink)
	if err := ask.Permission(sink.request(t, 1).Request, optionRefuse); err != nil {
		t.Fatal(err)
	}
	finished(t, done)
	if _, err := os.Stat(target); err == nil {
		t.Fatal("a refused command ran")
	}
	told := client.calls[1].Messages[len(client.calls[1].Messages)-1].Content
	if !strings.Contains(told, "refused") {
		t.Fatalf("the model was told %q", told)
	}
	resolved := sink.of(protocol.EvPermissionResolved)
	if len(resolved) != 1 || resolved[0].(protocol.PermissionResolvedData).Option != optionRefuse {
		t.Fatalf("the request resolved as %#v", resolved)
	}
}

// Reading needs no answer in any mode, so a look around never stops to ask.
func TestAReadNeverWaits(t *testing.T) {
	ask, _, sink := actingAsk(t, protocol.ApprovalsAsk, call("c1", "run_command", map[string]string{"command": "printf seen", "why": "look"}))
	finished(t, turn(t, ask, sink))
	if asked := sink.of(protocol.EvPermission); len(asked) != 0 {
		t.Fatalf("a command that only reads was put to the user: %#v", asked)
	}
	if status, output := sink.toolOutcome("c1"); status != protocol.ToolDone || output != "seen" {
		t.Fatalf("the call reported %q %q", status, output)
	}
}

// Without asking means without asking, except for a command the classifier
// rates destructive or cannot read through, here one that hides what it runs
// inside a substitution.
func TestWithoutAskingStillStopsAtWhatCannotBeTakenBack(t *testing.T) {
	root := t.TempDir()
	plain := filepath.Join(root, "plain")
	hidden := filepath.Join(root, "hidden")
	ask, _, sink := actingAsk(t, protocol.ApprovalsAuto,
		call("c1", "run_command", map[string]string{"command": "touch " + plain, "why": "make it"}),
		call("c2", "run_command", map[string]string{"command": "touch " + hidden + " $(true)", "why": "make another"}),
	)
	done := turn(t, ask, sink)
	request := sink.request(t, 1)
	if request.Call != "c2" {
		t.Fatalf("the first request was for %q, want the second call", request.Call)
	}
	if _, err := os.Stat(plain); err != nil {
		t.Fatal("the plain change did not run by itself")
	}
	for _, option := range request.Options {
		if option.Kind == protocol.OptAllowAlways {
			t.Fatal("a command of the destructive tier offered to stop the asking")
		}
	}
	if err := ask.Permission(request.Request, optionRefuse); err != nil {
		t.Fatal(err)
	}
	finished(t, done)
	if _, err := os.Stat(hidden); err == nil {
		t.Fatal("the refused command ran")
	}
}

// Allowing for the conversation stops the asking until the session goes back
// to read-only.
func TestAllowingForTheConversationStopsTheAsking(t *testing.T) {
	root := t.TempDir()
	ask, _, sink := actingAsk(t, protocol.ApprovalsAsk,
		call("c1", "run_command", map[string]string{"command": "touch " + filepath.Join(root, "one"), "why": "first"}),
		call("c2", "run_command", map[string]string{"command": "touch " + filepath.Join(root, "two"), "why": "second"}),
	)
	done := turn(t, ask, sink)
	if err := ask.Permission(sink.request(t, 1).Request, optionAlways); err != nil {
		t.Fatal(err)
	}
	finished(t, done)
	if asked := sink.of(protocol.EvPermission); len(asked) != 1 {
		t.Fatalf("%d requests were put to the user, want the first only", len(asked))
	}
	if _, err := os.Stat(filepath.Join(root, "two")); err != nil {
		t.Fatal("the second change did not run")
	}
	if err := ask.SetMode(context.Background(), protocol.ApprovalsReadOnly, sink); err != nil {
		t.Fatal(err)
	}
	if err := ask.SetMode(context.Background(), protocol.ApprovalsAsk, sink); err != nil {
		t.Fatal(err)
	}
	if ask.allowedAll() {
		t.Fatal("going back to read-only kept the earlier allowance")
	}
}

func TestStoppingTheTurnClosesAnOpenRequest(t *testing.T) {
	target := filepath.Join(t.TempDir(), "made")
	ask, _, sink := actingAsk(t, protocol.ApprovalsAsk, call("c1", "run_command", map[string]string{"command": "touch " + target, "why": "make it"}))
	done := turn(t, ask, sink)
	request := sink.request(t, 1)
	ask.Cancel()
	if result := finished(t, done); result.Stop != protocol.StopCancelled {
		t.Fatalf("the turn stopped as %q", result.Stop)
	}
	resolved := sink.of(protocol.EvPermissionResolved)
	if len(resolved) != 1 || resolved[0].(protocol.PermissionResolvedData).Outcome != "cancelled" {
		t.Fatalf("the request resolved as %#v", resolved)
	}
	if err := ask.Permission(request.Request, optionAllow); err == nil {
		t.Fatal("a request closed by stopping could still be answered")
	}
	if _, err := os.Stat(target); err == nil {
		t.Fatal("the command ran after the turn was stopped")
	}
}

// sudo asks for a password on a terminal Ask does not have, so it is refused
// with where to take it instead of hanging on a prompt nobody sees.
func TestACommandThatNeedsAPasswordIsRefused(t *testing.T) {
	ask, _, sink := actingAsk(t, protocol.ApprovalsAuto, call("c1", "run_command", map[string]string{"command": "sudo pacman -Syu", "why": "update"}))
	finished(t, turn(t, ask, sink))
	if status, output := sink.toolOutcome("c1"); status != protocol.ToolFailed || !strings.Contains(output, "propose") {
		t.Fatalf("the call reported %q %q", status, output)
	}
	if asked := sink.of(protocol.EvPermission); len(asked) != 0 {
		t.Fatal("a command Ask cannot run was put to the user")
	}
}

// A written file shows its change before the user answers, and what it held
// before stays on disk beside the session.
func TestWritingAFileShowsTheChangeAndKeepsWhatWasThere(t *testing.T) {
	ask, _, sink := actingAsk(t, protocol.ApprovalsAsk)
	target := filepath.Join(os.Getenv("HOME"), "notes.conf")
	if err := os.WriteFile(target, []byte("size 11\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	ask.factory.client.(*scriptedClient).streams = []*scriptedStream{
		finalStream(llm.Delta{ToolCalls: []llm.ToolCall{call("c1", "write_file", map[string]string{"path": "~/notes.conf", "content": "size 13\n", "why": "Make the text larger"})}}),
		finalStream(llm.Delta{Text: "Done."}),
	}
	done := turn(t, ask, sink)
	request := sink.request(t, 1)
	if request.Kind != protocol.ToolEdit || request.Input != target || request.Title != "Make the text larger" {
		t.Fatalf("the request reads %#v", request)
	}
	var shown *protocol.Diff
	for _, data := range sink.of(protocol.EvTool) {
		if tool := data.(protocol.ToolData); tool.Diff != nil {
			shown = tool.Diff
		}
	}
	if shown == nil || shown.Old != "size 11\n" || shown.New != "size 13\n" {
		t.Fatalf("the change shown before the answer is %#v", shown)
	}
	if body, _ := os.ReadFile(target); string(body) != "size 11\n" {
		t.Fatal("the file changed before the user answered")
	}
	if err := ask.Permission(request.Request, optionAllow); err != nil {
		t.Fatal(err)
	}
	finished(t, done)
	if body, _ := os.ReadFile(target); string(body) != "size 13\n" {
		t.Fatalf("the file holds %q", body)
	}
	if info, _ := os.Stat(target); info.Mode().Perm() != 0o640 {
		t.Fatalf("the file's mode became %v", info.Mode().Perm())
	}
	_, output := sink.toolOutcome("c1")
	kept := output[strings.LastIndex(output, " ")+1:]
	kept = strings.TrimSuffix(kept, ".")
	if body, err := os.ReadFile(kept); err != nil || string(body) != "size 11\n" {
		t.Fatalf("the earlier version at %q reads %q (%v)", kept, body, err)
	}
}

func TestWritingRefusesSecretsAndKeepsALinkALink(t *testing.T) {
	ask, _, sink := actingAsk(t, protocol.ApprovalsAuto)
	home := os.Getenv("HOME")
	if err := os.MkdirAll(filepath.Join(home, ".ssh"), 0o700); err != nil {
		t.Fatal(err)
	}
	real := filepath.Join(home, "real.conf")
	link := filepath.Join(home, "link.conf")
	if err := os.WriteFile(real, []byte("old\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(real, link); err != nil {
		t.Fatal(err)
	}
	ask.factory.client.(*scriptedClient).streams = []*scriptedStream{
		finalStream(llm.Delta{ToolCalls: []llm.ToolCall{call("c1", "write_file", map[string]string{"path": "~/.ssh/authorized_keys", "content": "ssh-ed25519 AAAA someone\n", "why": "add a key"})}}),
		finalStream(llm.Delta{ToolCalls: []llm.ToolCall{call("c2", "write_file", map[string]string{"path": link, "content": "new\n", "why": "update"})}}),
		finalStream(llm.Delta{Text: "Done."}),
	}
	finished(t, turn(t, ask, sink))
	if status, _ := sink.toolOutcome("c1"); status != protocol.ToolFailed {
		t.Fatalf("writing a key file reported %q", status)
	}
	if _, err := os.Stat(filepath.Join(home, ".ssh", "authorized_keys")); err == nil {
		t.Fatal("a key file was written")
	}
	if status, output := sink.toolOutcome("c2"); status != protocol.ToolDone {
		t.Fatalf("writing through a link reported %q %q", status, output)
	}
	if info, err := os.Lstat(link); err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Fatal("the link was replaced by a file")
	}
	if body, _ := os.ReadFile(real); string(body) != "new\n" {
		t.Fatalf("the linked file holds %q", body)
	}
}

func TestAskModeIsOneOfThree(t *testing.T) {
	ask, _, sink := actingAsk(t, protocol.ApprovalsAuto)
	if sink.info.Mode != protocol.ApprovalsAuto {
		t.Fatalf("the session's mode reads %q", sink.info.Mode)
	}
	if err := ask.SetMode(context.Background(), "plan", sink); err == nil {
		t.Fatal("Ask took a mode that is not an approvals value")
	}
	if ask.currentMode() != protocol.ApprovalsAuto {
		t.Fatal("a refused mode still changed the session")
	}
}

// A model reaches for system_query whatever the command does. In a session
// allowed to act the mode decides what happens to one that changes something:
// it used to be turned down with "use propose", so with "without asking" set
// the user was still handed a command to run by hand.
func TestAQueryThatChangesSomethingFollowsTheMode(t *testing.T) {
	free := filepath.Join(t.TempDir(), "free")
	ask, _, sink := actingAsk(t, protocol.ApprovalsAuto, call("c1", "system_query", map[string]string{"command": "touch " + free + " && echo made"}))
	finished(t, turn(t, ask, sink))
	if _, err := os.Stat(free); err != nil {
		t.Fatal("without asking, the command did not run")
	}
	if status, output := sink.toolOutcome("c1"); status != protocol.ToolDone || output != "made" {
		t.Fatalf("the call reported %q %q", status, output)
	}
	if asked := sink.of(protocol.EvPermission); len(asked) != 0 {
		t.Fatalf("without asking, the user was asked: %#v", asked)
	}

	asked := filepath.Join(t.TempDir(), "asked")
	ask, _, sink = actingAsk(t, protocol.ApprovalsAsk, call("c1", "system_query", map[string]string{"command": "touch " + asked}))
	done := turn(t, ask, sink)
	request := sink.request(t, 1)
	if _, err := os.Stat(asked); err == nil {
		t.Fatal("asking first, the command ran before the answer")
	}
	if err := ask.Permission(request.Request, optionAllow); err != nil {
		t.Fatal(err)
	}
	finished(t, done)
	if _, err := os.Stat(asked); err != nil {
		t.Fatal("the allowed command did not run")
	}

	never := filepath.Join(t.TempDir(), "never")
	ask, client, sink := actingAsk(t, protocol.ApprovalsReadOnly, call("c1", "system_query", map[string]string{"command": "touch " + never}))
	finished(t, turn(t, ask, sink))
	if _, err := os.Stat(never); err == nil {
		t.Fatal("a read-only Ask ran a command that changes the machine")
	}
	if told := client.calls[1].Messages[len(client.calls[1].Messages)-1].Content; !strings.Contains(told, "Use propose") {
		t.Fatalf("a read-only Ask was told %q", told)
	}
}

// A model proposes out of habit. In a session allowed to act it is told once
// to run the commands itself, and the answer carries no plan for the user. A
// plan that needs root, or a second proposal, stands.
func TestAProposalAskCouldCarryOutIsHandedBackOnce(t *testing.T) {
	target := filepath.Join(t.TempDir(), "made")
	plan := map[string]any{"summary": "Make it", "commands": []map[string]string{{"command": "touch " + target}}}
	ask, client, sink := actingAsk(t, protocol.ApprovalsAuto,
		call("c1", "propose", plan),
		call("c2", "run_command", map[string]string{"command": "touch " + target, "why": "make it"}),
	)
	finished(t, turn(t, ask, sink))
	if told := client.calls[1].Messages[len(client.calls[1].Messages)-1]; told.ToolCallID != "c1" || !strings.Contains(told.Content, "Call run_command") {
		t.Fatalf("the model was told %#v", told)
	}
	if _, err := os.Stat(target); err != nil {
		t.Fatal("the command was not run")
	}
	if answer := sink.of(protocol.EvAnswer); len(answer) != 1 || answer[0].(protocol.AnswerData).Plan != nil {
		t.Fatalf("the answer still hands the user a plan: %#v", answer)
	}

	ask, _, sink = actingAsk(t, protocol.ApprovalsAuto, call("c1", "propose", plan), call("c2", "propose", plan))
	finished(t, turn(t, ask, sink))
	if answer := sink.of(protocol.EvAnswer); len(answer) != 1 || answer[0].(protocol.AnswerData).Plan == nil {
		t.Fatal("a plan proposed a second time was dropped")
	}

	root := map[string]any{"commands": []map[string]string{{"command": "sudo pacman -S ripgrep"}}}
	ask, _, sink = actingAsk(t, protocol.ApprovalsAuto, call("c1", "propose", root))
	finished(t, turn(t, ask, sink))
	if answer := sink.of(protocol.EvAnswer); len(answer) != 1 || answer[0].(protocol.AnswerData).Plan == nil {
		t.Fatal("a plan that needs root was not handed to the user")
	}
}

// What a command prints goes to the model, so one that names a credential
// does not run even where nothing is asked.
func TestACommandThatNamesACredentialNeverRuns(t *testing.T) {
	ask, client, sink := actingAsk(t, protocol.ApprovalsAuto,
		call("c1", "run_command", map[string]string{"command": "cat ~/.ssh/id_ed25519", "why": "look"}),
		call("c2", "system_query", map[string]string{"command": "cat ~/.ssh/id_ed25519 > /dev/null && cat ~/.ssh/id_ed25519"}),
	)
	key := filepath.Join(os.Getenv("HOME"), ".ssh", "id_ed25519")
	if err := os.MkdirAll(filepath.Dir(key), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(key, []byte("PRIVATE-KEY-BYTES"), 0o600); err != nil {
		t.Fatal(err)
	}
	finished(t, turn(t, ask, sink))
	for _, id := range []string{"c1", "c2"} {
		if status, output := sink.toolOutcome(id); status != protocol.ToolFailed || !strings.Contains(output, "Refused") {
			t.Fatalf("%s reported %q %q", id, status, output)
		}
	}
	for _, request := range client.calls {
		for _, message := range request.Messages {
			if strings.Contains(message.Content, "PRIVATE-KEY-BYTES") {
				t.Fatal("the key was sent to the model")
			}
		}
	}
}
