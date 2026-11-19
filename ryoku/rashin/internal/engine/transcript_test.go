package engine

import (
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"

	"ryoku-rashin/internal/llm"
	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/protocol"
)

func isolateState(t *testing.T) {
	t.Helper()
	t.Setenv("XDG_STATE_HOME", t.TempDir())
}

func TestTranscriptRoundTripsThroughItsFile(t *testing.T) {
	isolateState(t)
	info := protocol.SessionInfo{ID: "round-trip"}
	tr, lost := OpenTranscript(info, nil)
	if lost {
		t.Fatal("a new session must not report lost history")
	}
	defer DropTranscript(info.ID)
	msgs := []llm.Message{
		{Role: llm.RoleUser, Content: "read main.go", Images: []llm.Image{{Mime: "image/png", Data: []byte{1, 2, 3}}}},
		{Role: llm.RoleAssistant, Thinking: []llm.Thinking{{Text: "look first", Signature: "sig"}},
			ToolCalls: []llm.ToolCall{{ID: "c1", Name: "read", Arguments: `{"path":"main.go"}`}}},
		{Role: llm.RoleTool, ToolCallID: "c1", Name: "read", Content: "package main"},
		{Role: llm.RoleAssistant, Content: "It is the main package."},
	}
	if err := tr.BeginTurn(1); err != nil {
		t.Fatal(err)
	}
	if err := tr.Append(msgs...); err != nil {
		t.Fatal(err)
	}
	if err := tr.SetNotes("task: explain main.go"); err != nil {
		t.Fatal(err)
	}

	again, lost := OpenTranscript(info, tr.State())
	if lost {
		t.Fatal("reopening our own state must not report lost history")
	}
	if got := again.Messages(); !reflect.DeepEqual(got, msgs) {
		t.Fatalf("reloaded messages differ:\n got %+v\nwant %+v", got, msgs)
	}
	if again.Notes() != "task: explain main.go" {
		t.Fatalf("notes not reloaded: %q", again.Notes())
	}
	raw, err := ReadTranscript(info.ID)
	if err != nil || len(raw) != len(msgs) {
		t.Fatalf("ReadTranscript: %d messages, %v", len(raw), err)
	}
}

func TestTranscriptCapsToolResults(t *testing.T) {
	tr := NewMemoryTranscript()
	big := strings.Repeat("x", TranscriptToolCap*2)
	tr.Append(llm.Message{Role: llm.RoleTool, ToolCallID: "c", Content: big})
	got := tr.Messages()
	if n := len(got[len(got)-1].Content); n > TranscriptToolCap+64 {
		t.Fatalf("tool result kept %d bytes, cap %d", n, TranscriptToolCap)
	}
}

func TestOldSessionStateReportsLostHistory(t *testing.T) {
	isolateState(t)
	if _, lost := OpenTranscript(protocol.SessionInfo{ID: "harness"}, json.RawMessage(`{"sessionId":"acp-1","harness":"claude"}`)); !lost {
		t.Error("a harness runner state must report lost history")
	}
	DropTranscript("harness")

	info := protocol.SessionInfo{ID: "gone"}
	tr, _ := OpenTranscript(info, nil)
	tr.Append(llm.Message{Role: llm.RoleUser, Content: "hi"})
	state := tr.State()
	os.Remove(paths.EngineTranscript(info.ID))
	if _, lost := OpenTranscript(info, json.RawMessage(`{"model":"auto","transcript":`+string(state)+`}`)); !lost {
		t.Error("a state naming a deleted transcript must report lost history")
	}
	DropTranscript(info.ID)
}

func appendTurns(tr *Transcript, n int) {
	for turn := 1; turn <= n; turn++ {
		tr.BeginTurn(turn)
		tr.Append(
			llm.Message{Role: llm.RoleUser, Content: "question " + string(rune('0'+turn))},
			llm.Message{Role: llm.RoleAssistant, Content: "answer " + string(rune('0'+turn))},
		)
	}
}

func TestForkCopiesWholeTurns(t *testing.T) {
	isolateState(t)
	from := protocol.SessionInfo{ID: "parent"}
	tr, _ := OpenTranscript(from, nil)
	defer DropTranscript(from.ID)
	appendTurns(tr, 3)

	to := protocol.SessionInfo{ID: "fork"}
	if err := CopyTranscript(from, to, 2); err != nil {
		t.Fatal(err)
	}
	got, err := ReadTranscript(to.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 4 || got[3].Content != "answer 2" {
		t.Fatalf("fork at turn 2 kept %+v", got)
	}

	// A temporary session's transcript forks from memory into memory.
	tmp := protocol.SessionInfo{ID: "tmp-parent", Temporary: true}
	mem, _ := OpenTranscript(tmp, nil)
	defer DropTranscript(tmp.ID)
	appendTurns(mem, 2)
	tmpFork := protocol.SessionInfo{ID: "tmp-fork", Temporary: true}
	defer DropTranscript(tmpFork.ID)
	if err := CopyTranscript(tmp, tmpFork, 1); err != nil {
		t.Fatal(err)
	}
	forked, _ := OpenTranscript(tmpFork, nil)
	if n := forked.Len(); n != 2 {
		t.Fatalf("temporary fork holds %d messages, want 2", n)
	}
	if _, err := os.Stat(paths.EngineTranscript(tmpFork.ID)); !os.IsNotExist(err) {
		t.Fatal("a temporary fork must not be written to disk")
	}

	if err := CopyTranscript(protocol.SessionInfo{ID: "missing"}, protocol.SessionInfo{ID: "nothing"}, 3); err != nil {
		t.Fatalf("missing source: %v", err)
	}
}

func TestRevertNoteJoinsTheNextPrompt(t *testing.T) {
	isolateState(t)
	info := protocol.SessionInfo{ID: "revert"}
	tr, _ := OpenTranscript(info, nil)
	defer DropTranscript(info.ID)
	appendTurns(tr, 2)
	if err := NoteRevert(info, 2); err != nil {
		t.Fatal(err)
	}
	tr.BeginTurn(3)
	tr.Append(llm.Message{Role: llm.RoleUser, Content: "try again"})
	got := tr.Messages()
	last := got[len(got)-1]
	if last.Role != llm.RoleUser || !strings.Contains(last.Content, "before turn 2") || !strings.HasSuffix(last.Content, "try again") {
		t.Fatalf("revert note not told with the next prompt: %+v", last)
	}
}

func TestDanglingToolCallGetsAnAnswer(t *testing.T) {
	tr := NewMemoryTranscript()
	tr.Append(
		llm.Message{Role: llm.RoleUser, Content: "go"},
		llm.Message{Role: llm.RoleAssistant, ToolCalls: []llm.ToolCall{{ID: "a", Name: "bash"}, {ID: "b", Name: "read"}}},
		llm.Message{Role: llm.RoleTool, ToolCallID: "a", Content: "ok"},
		llm.Message{Role: llm.RoleUser, Content: "next"},
	)
	got := tr.Messages()
	if len(got) != 5 || got[3].Role != llm.RoleTool || got[3].ToolCallID != "b" || !got[3].IsError {
		t.Fatalf("cancelled call left unanswered: %+v", got)
	}
}
