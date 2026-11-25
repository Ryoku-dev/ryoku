package engine

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"ryoku-rashin/internal/engine/tool"
	"ryoku-rashin/internal/llm"
	"ryoku-rashin/internal/protocol"
)

func TestContextNotesPersistAndSeedFreshWindow(t *testing.T) {
	isolateState(t)
	info := protocol.SessionInfo{ID: "notes"}
	tr, _ := OpenTranscript(info, nil)
	defer DropTranscript(info.ID)
	tr.BeginTurn(1)
	tr.Append(llm.Message{Role: llm.RoleUser, Content: "initial request"}, llm.Message{Role: llm.RoleAssistant, Content: "earlier answer"})
	tr.BeginTurn(2)
	tr.Append(llm.Message{Role: llm.RoleUser, Content: "current request"})

	var notes, fresh tool.Tool
	for _, candidate := range ContextTools() {
		switch candidate.Name() {
		case tool.ContextNotes:
			notes = candidate
		case tool.NewContext:
			fresh = candidate
		}
	}
	if notes == nil || fresh == nil {
		t.Fatal("context tools were not registered")
	}
	env := tool.Env{Session: info}
	if got := fresh.Run(context.Background(), tool.Call{}, env); !got.IsError {
		t.Fatal("new_context without notes loses the user's context")
	}
	write := tool.Call{Args: json.RawMessage(`{"text":"Next: address the current request."}`)}
	if got := notes.Run(context.Background(), write, env); got.IsError {
		t.Fatalf("save notes: %s", got.Text)
	}
	if got := notes.Run(context.Background(), tool.Call{}, env); got.Text != "Next: address the current request." {
		t.Fatalf("read notes: %q", got.Text)
	}
	if got := fresh.Run(context.Background(), tool.Call{}, env); got.IsError || !tr.Rollover() {
		t.Fatalf("new_context did not roll over: %+v", got)
	}
	msgs := tr.Messages()
	if len(msgs) != 1 || !strings.Contains(msgs[0].Content, "Next: address the current request.") || !strings.Contains(msgs[0].Content, "current request") || strings.Contains(msgs[0].Content, "initial request") {
		t.Fatalf("fresh context failed to keep notes and the latest request alone: %+v", msgs)
	}
	again, lost := OpenTranscript(info, tr.State())
	if lost || again.Notes() != "Next: address the current request." || len(again.Messages()) != 1 {
		t.Fatalf("fresh context did not survive restart: lost=%v notes=%q messages=%+v", lost, again.Notes(), again.Messages())
	}
}

func TestContextNotesEnforcesByteLimitWithoutLosingSavedNotes(t *testing.T) {
	tr := NewMemoryTranscript()
	if err := tr.SetNotes("saved"); err != nil {
		t.Fatal(err)
	}
	if err := tr.SetNotes(strings.Repeat("界", MaxNotesBytes/3+1)); err == nil {
		t.Fatal("an oversized notebook must be refused")
	}
	if got := tr.Notes(); got != "saved" {
		t.Fatalf("a refused update changed the saved notebook: %q", got)
	}
}
