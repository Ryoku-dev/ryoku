// Package diffview draws protocol diffs for the Rashin app: file headers with
// counts, hunks with old and new line numbers, added and removed lines tinted
// with Lithic roles, folding of long files, line comments with a gutter
// control, and per-file stage, unstage, discard and open actions. The Changes
// page, the graph's commit details and the chat's per-turn changes all use it.
//
// This file is the seam other app packages code against; the package owner
// replaces the bodies.
package diffview

import (
	"gioui.org/layout"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/theme"

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
	// Comments are shown under the lines they pin; Commentable adds the gutter
	// control that opens a comment field on a line.
	Comments    []protocol.ReviewComment
	Commentable bool
	// FileActions adds stage, unstage and discard controls to file headers;
	// Staged means the diff shows staged changes (Unstage instead of Stage).
	FileActions bool
	Staged      bool
	// FoldOver folds files longer than this many diff lines until opened
	// (0 never folds).
	FoldOver int
	// Wrap wraps long lines instead of scrolling them sideways.
	Wrap bool
}

// View keeps scroll position, folded files, the open comment field and
// pending actions.
type View struct {
	scroll       look.Scroller
	folded       map[string]bool
	headers      map[string]*fileHeader
	lines        map[lineKey]*lineControl
	comments     map[string]*commentControl
	commentPath  string
	commentLine  int
	commentSide  string
	commentFocus bool
	commentField kit.TextField
	pending      []Action
	scrollPath   string
}

// Layout draws d and records the actions taken this frame.
func (v *View) Layout(gtx layout.Context, th *theme.Theme, d protocol.DiffResult, o Options) layout.Dimensions {
	return v.layout(gtx, th, d, o)
}

func (v *View) Actions() []Action {
	actions := v.pending
	v.pending = nil
	return actions
}

// ScrollToFile brings path's header to the top on the next frame.
func (v *View) ScrollToFile(path string) { v.scrollPath = path }

// FromTexts builds a one-file diff between two texts, for agent tool diffs that
// arrive as old and new contents.
func FromTexts(path, old, new string) protocol.FileDiff { return fromTexts(path, old, new) }
