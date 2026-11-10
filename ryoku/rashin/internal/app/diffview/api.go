// Package diffview draws protocol diffs for the Rashin app: a row per file
// that unfolds to its hunks, lines with old and new numbers tinted by their
// mineral, long files cut until asked for, review comments under the lines
// they pin, and per-file stage, unstage, discard and open actions. The Changes
// page, the graph's commit details, the Agents side panel and a tool call's
// diff in a conversation all use it.
package diffview

import (
	"gioui.org/layout"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"

	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/protocol"
)

// ActionKind says what the user asked for inside the diff.
type ActionKind int

const (
	// ActComment adds Text as a comment at Path, Line, Side.
	ActComment ActionKind = iota + 1
	// ActResolve resolves (or reopens) comment ID.
	ActResolve
	// ActDelete deletes comment ID.
	ActDelete
	// ActStage stages Path.
	ActStage
	// ActUnstage unstages Path.
	ActUnstage
	// ActDiscard discards the changes to Path (the page confirms first).
	ActDiscard
	// ActOpen opens Path at Line in the user's editor.
	ActOpen
)

// Action is one user request raised by the view.
type Action struct {
	Kind ActionKind
	Path string
	Line int
	Side string
	ID   string
	Text string
}

// Options configure one frame of the view.
type Options struct {
	// Overlays shows the tooltips of file actions.
	Overlays *look.Overlays
	// Comments are shown under the lines they pin; Commentable adds the gutter
	// control that opens a comment field on a line.
	Comments    []protocol.ReviewComment
	Commentable bool
	// FileActions adds stage, unstage and discard controls to file rows;
	// Staged means the diff shows staged changes (Unstage instead of Stage).
	FileActions bool
	Staged      bool
	// FoldOver cuts files longer than this many diff lines until the reader
	// asks for the rest (0 never cuts).
	FoldOver int
	// Collapsed starts every file as its row alone; the reader unfolds the
	// ones worth reading.
	Collapsed bool
	// Headless draws no file rows, for a single file whose name stands
	// elsewhere.
	Headless bool
}

// View keeps scroll position, which files are unfolded, the open comment
// field and pending actions.
type View struct {
	scroll       look.Scroller
	toggled      map[string]bool
	whole        map[string]bool
	headers      map[string]*fileHeader
	lines        map[lineKey]*lineControl
	comments     map[string]*commentControl
	more         map[string]*kit.Click
	commentPath  string
	commentLine  int
	commentSide  string
	commentFocus bool
	commentField kit.TextField
	pending      []Action
	scrollPath   string
}

// Layout draws d and records the actions taken this frame.
func (v *View) Layout(gtx layout.Context, th *look.Theme, d protocol.DiffResult, o Options) layout.Dimensions {
	return v.layout(gtx, th, d, o)
}

// Actions returns and clears the actions recorded since the last call.
func (v *View) Actions() []Action {
	actions := v.pending
	v.pending = nil
	return actions
}

// ScrollToFile brings path's row to the top on the next frame and unfolds it.
func (v *View) ScrollToFile(path string) { v.scrollPath = path }

// Scrolled reports whether the diff has moved beneath its top edge.
func (v *View) Scrolled() bool {
	return v.scroll.Position.First > 0 || v.scroll.Position.Offset > 0
}

// FromTexts builds a one-file diff between two texts, for agent tool diffs that
// arrive as old and new contents.
func FromTexts(path, old, new string) protocol.FileDiff { return fromTexts(path, old, new) }
