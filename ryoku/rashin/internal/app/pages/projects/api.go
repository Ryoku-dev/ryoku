// Package projects is the Work side of the Rashin app: the project list, and a
// project with its Overview (worktree cards, new worktree, cleanup), Graph,
// Changes (review with comments, stage, commit, push, conflicts), Tasks
// (board, task detail, plan, gates) and Activity tabs.
//
// This file is the seam the app shell codes against; the package owner
// replaces the bodies.
package projects

import (
	"gioui.org/layout"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"

	"ryoku-rashin/internal/app/env"
)

// Project tabs.
const (
	TabOverview = "overview"
	TabGraph    = "graph"
	TabChanges  = "changes"
	TabTasks    = "tasks"
	TabActivity = "activity"
)

// Page is the Projects destination of the rail.
type Page struct {
	state pageState
}

// New returns the page with the project list showing.
func New() *Page {
	page := &Page{}
	page.state.init()
	return page
}

// Open shows project (its tab, and the worktree within that tab when set). An
// empty project returns to the list.
func (p *Page) Open(e *env.Env, project, tab, worktree string) {
	p.state.open(e, project, tab, worktree)
}

// Layout draws the page.
func (p *Page) Layout(gtx layout.Context, e *env.Env, class kit.FrameClass) layout.Dimensions {
	return p.state.layout(gtx, e, class)
}

// Hidden tells the page it left the screen so it drops its project watch.
func (p *Page) Hidden(e *env.Env) { p.state.hidden(e) }
