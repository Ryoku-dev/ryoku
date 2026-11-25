package engine

// Derived from oh-my-pi (MIT), packages/coding-agent/src/tools/context-notes.ts,
// packages/coding-agent/src/session/context-notes.ts and the prompts
// packages/coding-agent/src/prompts/{tools/context-notes,tools/context-notes-text,
// tools/new-context,system/context-notes}.md@c7fff67, rewritten for small models.

import (
	"context"
	"encoding/json"
	"fmt"

	i18n "ryoku-i18n"

	"ryoku-rashin/internal/engine/tool"
	"ryoku-rashin/internal/llm"
	"ryoku-rashin/internal/protocol"
)

// MaxNotesBytes caps the context notebook.
const MaxNotesBytes = 16 << 10

const notesWrapper = `<context-notes>
Your own notes from earlier in this conversation. Check them against the files before you rely on them.

%s
</context-notes>`

func notesContext(notes string) string { return fmt.Sprintf(notesWrapper, notes) }

// ContextTools are the context notebook tools, context_notes and
// new_context, for the daemon's tool registry. Each finds the session's
// transcript through the session it serves, so it is offered only to a
// runner whose transcript is open.
func ContextTools() []tool.Tool { return []tool.Tool{contextNotesTool{}, newContextTool{}} }

type contextNotesTool struct{}

func (contextNotesTool) Name() string    { return tool.ContextNotes }
func (contextNotesTool) Kind() tool.Kind { return protocol.ToolThink }
func (contextNotesTool) ReadOnly() bool  { return true }

func (contextNotesTool) Enabled(env tool.Env) bool { return lookupTranscript(env.Session.ID) != nil }

func (contextNotesTool) Spec(p tool.Profile) llm.Tool {
	desc := "Read or replace your notebook for this conversation. It is kept when the conversation is compacted and seeds a fresh window (new_context). Keep it a short current index: task, decisions, changed files, blockers, next steps, one line each. Replace stale lines instead of adding."
	text := fmt.Sprintf("The whole new notebook, up to %d bytes. Leave out to read the notebook; an empty string clears it.", MaxNotesBytes)
	if p.Compact() {
		desc = "Read or replace your notebook; it survives compaction."
		text = "Whole new notebook; leave out to read."
	}
	return llm.Tool{
		Name:        tool.ContextNotes,
		Description: desc,
		Parameters:  json.RawMessage(fmt.Sprintf(`{"type":"object","properties":{"text":{"type":"string","description":%q}}}`, text)),
	}
}

type notesArgs struct {
	Text *string `json:"text"`
}

func (contextNotesTool) Present(args json.RawMessage) tool.Presentation {
	var a notesArgs
	if json.Unmarshal(args, &a) == nil && a.Text != nil {
		return tool.Presentation{Title: i18n.T("Saving context notes")}
	}
	return tool.Presentation{Title: i18n.T("Reading context notes")}
}

func (contextNotesTool) Run(ctx context.Context, call tool.Call, env tool.Env) tool.Result {
	t := lookupTranscript(env.Session.ID)
	if t == nil {
		return tool.Failed(i18n.T("Context notes are not available in this session."))
	}
	var a notesArgs
	if len(call.Args) > 0 {
		if err := json.Unmarshal(call.Args, &a); err != nil {
			return tool.Failed(i18n.T("context_notes takes {\"text\": string}; leave text out to read."))
		}
	}
	if a.Text == nil {
		notes := t.Notes()
		if notes == "" {
			return tool.Result{Text: i18n.T("No context notes are saved yet.")}
		}
		return tool.Result{Text: notes}
	}
	if n := len(*a.Text); n > MaxNotesBytes {
		return tool.Failed(i18n.Tf("The notes are %d bytes; the limit is %d. Shorten them and save again.", n, MaxNotesBytes))
	}
	if err := t.SetNotes(*a.Text); err != nil {
		return tool.Failed(i18n.T("The notes were kept for now but could not be written to disk: ") + err.Error())
	}
	if *a.Text == "" {
		return tool.Result{Text: i18n.T("Context notes cleared."), Shown: i18n.T("Notes cleared")}
	}
	return tool.Result{Text: i18n.T("Context notes saved."), Shown: i18n.Tf("Saved %d bytes of notes", len(*a.Text))}
}

type newContextTool struct{}

func (newContextTool) Name() string    { return tool.NewContext }
func (newContextTool) Kind() tool.Kind { return protocol.ToolThink }
func (newContextTool) ReadOnly() bool  { return true }

func (newContextTool) Enabled(env tool.Env) bool { return lookupTranscript(env.Session.ID) != nil }

func (newContextTool) Spec(p tool.Profile) llm.Tool {
	desc := "Start a fresh context window after this step. The new window holds your context notes, the user's latest request and nothing else, so save everything you still need with context_notes first."
	if p.Compact() {
		desc = "Start a fresh window from your notes; save them first."
	}
	return llm.Tool{
		Name:        tool.NewContext,
		Description: desc,
		Parameters:  json.RawMessage(`{"type":"object","properties":{}}`),
	}
}

func (newContextTool) Present(json.RawMessage) tool.Presentation {
	return tool.Presentation{Title: i18n.T("Starting a fresh context window")}
}

func (newContextTool) Run(ctx context.Context, call tool.Call, env tool.Env) tool.Result {
	t := lookupTranscript(env.Session.ID)
	if t == nil {
		return tool.Failed(i18n.T("A fresh context window is not available in this session."))
	}
	if t.Notes() == "" {
		return tool.Failed(i18n.T("Save your notes with context_notes first: the fresh window starts from them."))
	}
	t.requestRollover()
	return tool.Result{
		Text:  i18n.T("New context window requested. It starts after this step with your notes and the user's latest request."),
		Shown: i18n.T("The next step starts in a fresh window"),
	}
}
