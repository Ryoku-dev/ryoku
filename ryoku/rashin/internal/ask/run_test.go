package ask

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"ryoku-rashin/internal/core"
	"ryoku-rashin/internal/llm"
	"ryoku-rashin/internal/protocol"
)

// shownOutput is what the user was shown of a command, piece by piece, and
// whether its terminal was closed with an exit status.
func (s *lockedSink) shownOutput(terminal string) (pieces []string, exit *int, done bool) {
	for _, data := range s.of(protocol.EvTerminal) {
		piece := data.(protocol.TerminalData)
		if piece.Terminal != terminal {
			continue
		}
		if piece.Delta != "" {
			pieces = append(pieces, piece.Delta)
		}
		if piece.Done {
			exit, done = piece.Exit, true
		}
	}
	return pieces, exit, done
}

func (s *lockedSink) terminalOf(call string) string {
	for _, data := range s.of(protocol.EvTool) {
		if tool := data.(protocol.ToolData); tool.Call == call && tool.Terminal != "" {
			return tool.Terminal
		}
	}
	return ""
}

func runTurn(t *testing.T, ask core.Runner, sink *lockedSink, line string) chan core.TurnResult {
	t.Helper()
	done := make(chan core.TurnResult, 1)
	go func() {
		result, err := ask.Prompt(context.Background(), core.PromptInput{Turn: 2, Run: line}, sink)
		if err != nil {
			t.Errorf("the turn failed: %v", err)
		}
		done <- result
	}()
	return done
}

// A command the user starts from the conversation runs in the background, in
// any mode: its output is shown as it comes, and Ask is then handed that
// output and answers from it. Opening a terminal that sat silent until the
// command ended read as a button that does nothing.
func TestAUserRunIsWatchedAndThenAnswered(t *testing.T) {
	client := &scriptedClient{streams: []*scriptedStream{finalStream(llm.Delta{Text: "The largest file is big.iso."})}}
	ask := testRunner(t, client)
	sink := newLockedSink()
	finished(t, runTurn(t, ask, sink, `printf '5.7G\t/home/x/big.iso\n'; printf 'warning\n' >&2`))

	if terminal := sink.terminalOf("run-2"); terminal != "run-2" {
		t.Fatalf("the run's output belongs to terminal %q", terminal)
	}
	pieces, exit, done := sink.shownOutput("run-2")
	if shown := strings.Join(pieces, ""); !strings.Contains(shown, "5.7G\t/home/x/big.iso") || !strings.Contains(shown, "warning") {
		t.Fatalf("the user was shown %q", shown)
	}
	if !done || exit == nil || *exit != 0 {
		t.Fatalf("the terminal closed %v with %v", done, exit)
	}
	if status, _ := sink.toolOutcome("run-2"); status != protocol.ToolDone {
		t.Fatalf("the run reported %q", status)
	}
	if len(client.calls) != 1 {
		t.Fatalf("the model was called %d times", len(client.calls))
	}
	if offered(client.calls[0], "run_command") {
		t.Fatal("running a command by hand handed a read-only Ask the tools that change the machine")
	}
	told := client.calls[0].Messages[len(client.calls[0].Messages)-1]
	for _, want := range []string{"big.iso", "exit status 0", "printf '5.7G", "Do not propose this command again"} {
		if told.Role != llm.RoleUser || !strings.Contains(told.Content, want) {
			t.Fatalf("the model was told %q, which lacks %q", told.Content, want)
		}
	}
	if text := sink.of(protocol.EvText); len(text) == 0 {
		t.Fatal("Ask gave no answer after the run")
	}
}

// What a slow command prints is shown while it still runs, not when it ends.
func TestOutputIsShownWhileTheCommandStillRuns(t *testing.T) {
	client := &scriptedClient{streams: []*scriptedStream{finalStream(llm.Delta{Text: "Done."})}}
	ask := testRunner(t, client)
	sink := newLockedSink()
	done := runTurn(t, ask, sink, "printf first; sleep 0.4; printf second; exit 3")
	deadline := time.After(3 * time.Second)
	for {
		if pieces, _, ended := sink.shownOutput("run-2"); len(pieces) > 0 {
			if ended {
				t.Fatal("the first output only showed once the command had ended")
			}
			if pieces[0] != "first" {
				t.Fatalf("the first piece shown was %q", pieces[0])
			}
			break
		}
		select {
		case <-sink.wake:
		case <-deadline:
			t.Fatal("nothing was shown while the command ran")
		}
	}
	finished(t, done)
	if _, exit, ended := sink.shownOutput("run-2"); !ended || exit == nil || *exit != 3 {
		t.Fatalf("the terminal closed %v with %v", ended, exit)
	}
	if status, _ := sink.toolOutcome("run-2"); status != protocol.ToolFailed {
		t.Fatalf("a command that exited 3 reported %q", status)
	}
	if told := client.calls[0].Messages[len(client.calls[0].Messages)-1].Content; !strings.Contains(told, "exit status 3") || !strings.Contains(told, "firstsecond") {
		t.Fatalf("the model was told %q", told)
	}
}

// Stop ends a run at once, with its whole process group, and nothing is asked
// of the model about a command the user called off.
func TestStoppingARunKillsItAndAsksNothing(t *testing.T) {
	client := &scriptedClient{streams: []*scriptedStream{finalStream(llm.Delta{Text: "unused"})}}
	ask := testRunner(t, client)
	sink := newLockedSink()
	done := runTurn(t, ask, sink, "printf started; sleep 30 | cat")
	deadline := time.After(3 * time.Second)
	for {
		if pieces, _, _ := sink.shownOutput("run-2"); len(pieces) > 0 {
			break
		}
		select {
		case <-sink.wake:
		case <-deadline:
			t.Fatal("the command never started")
		}
	}
	began := time.Now()
	ask.Cancel()
	if result := finished(t, done); result.Stop != protocol.StopCancelled {
		t.Fatalf("the turn ended as %q", result.Stop)
	}
	if took := time.Since(began); took > 4*time.Second {
		t.Fatalf("stopping took %v", took)
	}
	if len(client.calls) != 0 {
		t.Fatal("the model was asked about a run the user stopped")
	}
	if _, _, ended := sink.shownOutput("run-2"); !ended {
		t.Fatal("the terminal was left open")
	}
}

// A command that needs a password has no keyboard here, so it is not started.
func TestARunThatNeedsATerminalIsNotStarted(t *testing.T) {
	ask := testRunner(t, &scriptedClient{})
	sink := newLockedSink()
	_, err := ask.Prompt(context.Background(), core.PromptInput{Turn: 2, Run: "sudo true"}, sink)
	if err == nil || !strings.Contains(err.Error(), "needs a terminal") {
		t.Fatalf("the run was answered with %v", err)
	}
	if tools := sink.of(protocol.EvTool); len(tools) != 0 {
		t.Fatalf("something was started: %#v", tools)
	}
}

// A command Ask runs itself is shown the same way once it has run long enough
// to be worth watching. A quick one stays a plain result, so a transcript of
// short looks does not open and shut a terminal for each.
func TestACommandAskRunsIsWatchedOnlyOnceItTakesAWhile(t *testing.T) {
	ask, _, sink := actingAsk(t, protocol.ApprovalsAuto,
		call("quick", "system_query", map[string]string{"command": "printf seen"}),
		call("slow", "system_query", map[string]string{"command": "printf early; sleep 1; printf late"}),
		call("made", "run_command", map[string]string{"command": "printf begun; sleep 1; printf ended", "why": "wait"}),
	)
	finished(t, turn(t, ask, sink))
	if terminal := sink.terminalOf("quick"); terminal != "" {
		t.Fatalf("a quick look was given terminal %q", terminal)
	}
	if status, output := sink.toolOutcome("quick"); status != protocol.ToolDone || output != "seen" {
		t.Fatalf("the quick look reported %q %q", status, output)
	}
	for call, want := range map[string]string{"slow": "earlylate", "made": "begunended"} {
		if terminal := sink.terminalOf(call); terminal != call {
			t.Fatalf("%s was given terminal %q", call, terminal)
		}
		pieces, exit, done := sink.shownOutput(call)
		if len(pieces) < 2 || strings.Join(pieces, "") != want || !done || exit == nil || *exit != 0 {
			t.Fatalf("%s was shown %q, closed %v with %v", call, pieces, done, exit)
		}
		if status, output := sink.toolOutcome(call); status != protocol.ToolDone || output != want {
			t.Fatalf("%s reported %q %q", call, status, output)
		}
	}
}

// A character the command wrote in two pieces arrives whole.
func TestACharacterSplitAcrossWritesIsShownWhole(t *testing.T) {
	sink := newLockedSink()
	output := &watched{sink: sink, call: "c", kept: limitedBuffer{limit: 1024}}
	output.attach()
	whole := []byte("größe")
	_, _ = output.Write(whole[:3])
	_, _ = output.Write(whole[3:])
	output.end(0, "")
	pieces, _, _ := sink.shownOutput("c")
	if strings.Join(pieces, "") != "größe" || strings.ContainsRune(strings.Join(pieces, ""), '\uFFFD') {
		t.Fatalf("shown as %q", pieces)
	}
}

// A later question still knows a command was run, and a proposal marks the
// commands that cannot run in the conversation.
func TestARunIsRememberedAndAPlanSaysWhatNeedsATerminal(t *testing.T) {
	ran, _ := json.Marshal(protocol.UserData{Run: "du -sh ~"})
	history := historyMessages([]protocol.SessionEvent{{Seq: 1, Kind: protocol.EvUser, Data: ran}}, historyBudgetBytes)
	if len(history) != 1 || history[0].Role != llm.RoleUser || !strings.Contains(history[0].Content, "du -sh ~") {
		t.Fatalf("history holds %#v", history)
	}
	plan, err := proposedPlan(`{"commands":[{"command":"du -sh ~"},{"command":"sudo pacman -Syu"},{"command":"cat ~/.ssh/id_ed25519"}]}`)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Commands[0].Terminal || !plan.Commands[1].Terminal || !plan.Commands[2].Terminal {
		t.Fatalf("the plan reads %#v", plan.Commands)
	}
}
