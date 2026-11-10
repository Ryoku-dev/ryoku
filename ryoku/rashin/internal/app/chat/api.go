// Package chat is the conversation surface of the Rashin app: the transcript
// model reduced from session events, the transcript view (Markdown, code
// blocks, tool calls, diffs, terminal output, plans, permission prompts, answer
// actions) and the multi-line composer. It depends only on the protocol and the
// Lithic library, so pages compose it without knowing how it draws.
//
// This file is the seam the app shell codes against; the package owner
// replaces the bodies.
package chat

import (
	"gioui.org/layout"
	"sync"

	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/protocol"
)

// ActionKind says what the user asked for from inside the transcript.
type ActionKind int

const (
	// ActPermission answers a permission request: Request and Option are set.
	ActPermission ActionKind = iota + 1
	// ActCopy copies Value to the clipboard.
	ActCopy
	// ActOpenPath opens the file or folder at Value.
	ActOpenPath
	// ActOpenURL opens the link in Value.
	ActOpenURL
	// ActRunCommand runs the command in Value in a terminal window.
	ActRunCommand
	// ActHandoff moves this Ask session to an Agent session.
	ActHandoff
	// ActCancel cancels the running turn.
	ActCancel
	// ActUnqueue removes the queued prompt named by Value.
	ActUnqueue
	// ActTurnChanges opens the checkpoint diff for Turn.
	ActTurnChanges
	// ActRevertTurn asks the host to confirm and revert files to before Turn.
	ActRevertTurn
	// ActFork forks the session at event Seq.
	ActFork
	// ActRetry asks again what the reader asked in Turn; Value is that question.
	ActRetry
)

// Action is one user request raised by the view.
type Action struct {
	Kind    ActionKind
	Request string
	Option  string
	Value   string
	Seq     int64
	Turn    int
}

// Model is the transcript of one session.
type Model struct {
	mu    sync.RWMutex
	info  protocol.SessionInfo
	state *modelState
}

// NewModel starts an empty transcript for info.
func NewModel(info protocol.SessionInfo) *Model {
	return &Model{info: cloneSessionInfo(info), state: newModelState()}
}

// Info is the session's current info.
func (m *Model) Info() protocol.SessionInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return cloneSessionInfo(m.info)
}

// SetInfo replaces the session info (state, model, title).
func (m *Model) SetInfo(info protocol.SessionInfo) {
	m.mu.Lock()
	m.info = cloneSessionInfo(info)
	m.mu.Unlock()
}

// Reset replaces the transcript with a full replay.
func (m *Model) Reset(info protocol.SessionInfo, events []protocol.SessionEvent) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.info = cloneSessionInfo(info)
	m.state = newModelState()
	for _, event := range events {
		m.state.apply(m, event)
	}
}

// Apply folds one event in. Events at or below LastSeq are ignored, so a
// replay overlapping live events is harmless.
func (m *Model) Apply(event protocol.SessionEvent) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.state == nil {
		m.state = newModelState()
	}
	m.state.apply(m, event)
}

// LastSeq is the highest seq applied.
func (m *Model) LastSeq() int64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.state == nil {
		return 0
	}
	return m.state.lastSeq
}

// Busy reports a running turn.
func (m *Model) Busy() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.state != nil && m.state.activeTurn != nil
}

// Pending lists permission requests not yet resolved, oldest first.
func (m *Model) Pending() []protocol.PermissionData {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.state == nil {
		return nil
	}
	return m.state.pendingPermissions()
}

// ModelChoice returns the latest model selection and available options.
func (m *Model) ModelChoice() protocol.ChoiceData {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.state == nil {
		return protocol.ChoiceData{}
	}
	return cloneChoiceData(m.state.model)
}

// ModeChoice returns the latest harness mode selection and available options.
func (m *Model) ModeChoice() protocol.ChoiceData {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.state == nil {
		return protocol.ChoiceData{}
	}
	return cloneChoiceData(m.state.mode)
}

// Commands returns the latest slash commands offered by the harness.
func (m *Model) Commands() []protocol.Command {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.state == nil {
		return nil
	}
	return append([]protocol.Command(nil), m.state.commands.Commands...)
}

// Prompts returns what the reader sent in this conversation, oldest first.
func (m *Model) Prompts() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.state == nil {
		return nil
	}
	var prompts []string
	for index := range m.state.rows {
		if row := &m.state.rows[index]; row.kind == rowUser && row.user.Text != "" {
			prompts = append(prompts, row.user.Text)
		}
	}
	return prompts
}

func cloneSessionInfo(info protocol.SessionInfo) protocol.SessionInfo {
	info.Queue = append([]protocol.QueuedPrompt(nil), info.Queue...)
	return info
}

func cloneChoiceData(choice protocol.ChoiceData) protocol.ChoiceData {
	choice.Options = append([]protocol.Choice(nil), choice.Options...)
	return choice
}

// View lays out a transcript. It keeps scroll position, selection and
// expansion state, and follows the end while a turn streams unless the user
// scrolled up.
type View struct {
	state *viewState
}

// Layout draws m and records the actions the user took this frame. Menus and
// tooltips open through overlays.
func (v *View) Layout(gtx layout.Context, th *look.Theme, overlays *look.Overlays, m *Model) layout.Dimensions {
	m.mu.RLock()
	defer m.mu.RUnlock()
	state := v.ensureState()
	state.overlays = overlays
	return state.layout(gtx, th, m.info, m.state)
}

// Actions returns and clears the actions recorded since the last call.
func (v *View) Actions() []Action { return v.ensureState().takeActions() }

// Scrolled reports whether the transcript has moved beneath its top edge, so
// the header above it can draw its vein.
func (v *View) Scrolled() bool {
	position := v.ensureState().scroller.Position
	return position.First > 0 || position.Offset > 0
}

// ComposerOptions configure one frame of the composer.
type ComposerOptions struct {
	Placeholder string
	// Busy changes the submit actions to Queue and Steer and shows Stop.
	Busy bool
	// Queue lists follow-up prompts waiting for the running turn to end.
	Queue []protocol.QueuedPrompt
	// Controls are the caller's compact controls beside the attach control,
	// such as the model and mode choices.
	Controls []layout.Widget
	// CanAttach shows the attach control.
	CanAttach bool
	// Disabled greys the composer (Rashin off or no session).
	Disabled bool
	// Attach is called when the attach control is pressed, to ask the system
	// for files. Without it the composer opens its own path field.
	Attach func()
	// PasteImage is called on a paste, so the page can attach an image the
	// clipboard holds. The editor itself only pastes text.
	PasteImage func()
	// History returns the prompts already sent here, oldest first. Up in an
	// empty composer walks back through them. It is only called on that key.
	History func() []string
	// Commands returns the slash commands the harness offers. Typing a slash
	// lists them above the field. It is only called while a command is typed.
	Commands func() []protocol.Command
}

// Submission is what the user sent. Steer cancels the running turn and sends
// this submission at once; otherwise a busy session queues it.
type Submission struct {
	Text   string
	Images []string
	Steer  bool
}

// Composer is the multi-line prompt editor: Enter sends, Shift+Enter breaks a
// line, it grows to a bounded height and scrolls past it.
type Composer struct {
	state *composerState
}

// Layout draws the composer at the width it is given.
func (c *Composer) Layout(gtx layout.Context, th *look.Theme, opts ComposerOptions) layout.Dimensions {
	return c.ensureState().layout(gtx, th.Theme, opts)
}

// Submitted returns a submission made since the last call.
func (c *Composer) Submitted() (Submission, bool) {
	return c.ensureState().submittedValue()
}

// Actions returns and clears actions raised by controls around the composer,
// such as removing a queued prompt.
func (c *Composer) Actions() []Action { return c.ensureState().takeActions() }

// StopClicked reports a press of Stop since the last call.
func (c *Composer) StopClicked() bool { return c.ensureState().takeStop() }

// Focus asks for keyboard focus on the next frame.
func (c *Composer) Focus() { c.ensureState().wantFocus = true }

// FocusTag is the composer's text as a focus target, for whatever hands the
// keyboard back to it: a menu that closed, a dialog, the search field.
func (c *Composer) FocusTag() any { return &c.ensureState().editor }

// SetText replaces the composer's text (used to prefill a follow-up).
func (c *Composer) SetText(text string) { c.ensureState().setText(text) }

// AddAttachment adds an image or file path to the composer.
func (c *Composer) AddAttachment(path string) { c.ensureState().addAttachment(path) }

// ShowAttachField opens the path field, for when no system chooser answers.
func (c *Composer) ShowAttachField() { c.ensureState().showAttachField() }

// Draft is what a composer holds before it is sent.
type Draft struct {
	Text        string
	Attachments []string
}

// Draft returns what the composer holds now.
func (c *Composer) Draft() Draft {
	state := c.ensureState()
	return Draft{Text: state.editor.Text(), Attachments: append([]string(nil), state.images...)}
}

// SetDraft replaces what the composer holds, as when another conversation
// opens with the draft it was left with.
func (c *Composer) SetDraft(draft Draft) { c.ensureState().setDraft(draft) }
