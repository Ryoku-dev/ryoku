// Package graph is the commit graph of a project in the Rashin app: lanes laid
// out by internal/gitgraph, ref labels, worktree badges, a WIP row per
// worktree, the details panel, search, the ref finder, scopes and the row
// actions with their confirmations.
//
// This file is the seam the projects page codes against; the package owner
// replaces the bodies.
package graph

import (
	"gioui.org/layout"

	"ryoku-rashin/internal/app/env"
)

// ActionKind is a navigation the host performs for the graph.
type ActionKind int

const (
	// ActOpenWorktree shows Worktree in the project's Overview.
	ActOpenWorktree ActionKind = iota + 1
	// ActOpenChanges shows Worktree in the Changes tab.
	ActOpenChanges
	// ActNewSession starts an Agent session in Worktree.
	ActNewSession
	// ActOpenSession opens the Agent session Session.
	ActOpenSession
)

// Action is one navigation request.
type Action struct {
	Kind     ActionKind
	Project  string
	Worktree string
	Session  string
}

// View draws one project's graph and keeps its paging, lane state, selection
// and scope.
type View struct {
	state viewState
}

// New returns an empty view.
func New() *View {
	view := &View{}
	view.state.reset("")
	return view
}

// Layout draws the graph of project.
func (v *View) Layout(gtx layout.Context, e *env.Env, project string) layout.Dimensions {
	return v.state.layout(gtx, e, project)
}

// Actions returns and clears the navigation requests since the last call.
func (v *View) Actions() []Action {
	actions := v.state.actions
	v.state.actions = nil
	return actions
}

// Refresh reloads refs, WIP rows and the first page (after git.changed).
func (v *View) Refresh() {
	v.state.refreshRequested = true
}
