package env

import (
	"gioui.org/layout"

	"ryoku-rashin/internal/app/look"
)

// Page is what the frame asks of a destination. The frame owns the panes, the
// top bar and the sidebar's own rows; a page fills the regions it is given and
// never draws a pane.
type Page interface {
	// List draws the destination's list in the sidebar and reports where its
	// selected row stands, so the sidebar can move its one selection slab there.
	List(gtx layout.Context, e *Env) look.ListDimensions
	// Content draws the content pane: the page's header and its body.
	Content(gtx layout.Context, e *Env) layout.Dimensions
	// New starts the destination's new thing; NewLabel names it on the
	// sidebar's New plaque.
	New(e *Env)
	NewLabel() string
	// Overlay draws the dialogs the page owns, above the frame, and reports
	// whether one is open: the frame keeps its shortcuts out of a dialog.
	Overlay(gtx layout.Context, e *Env) bool
}

// Homed is a page with a place the keyboard belongs when nothing else holds
// it, which for a conversation is its composer. The frame returns focus there
// when the search field or a dialog closes.
type Homed interface {
	// FocusHome is that place as a focus target, or nil when it has none now.
	FocusHome() any
}

// Paneled is a page that can show work beside its content.
type Paneled interface {
	// HasPanel reports whether what the page shows now has a side panel.
	HasPanel(e *Env) bool
	// Panel draws it; the frame docks it or raises it as a sheet.
	Panel(gtx layout.Context, e *Env) layout.Dimensions
}

// Indexed is a page whose list is its own index. The sidebar shows it while
// the sidebar stands open; otherwise the page shows the index in its content.
type Indexed interface {
	// ListShown tells the page, before Content, whether the sidebar shows
	// its list this frame.
	ListShown(shown bool)
}

// Views of a project, as NavigateWork names them.
const (
	ViewOverview = "overview"
	ViewGraph    = "graph"
	ViewChanges  = "changes"
	ViewTasks    = "tasks"
	ViewActivity = "activity"
)
