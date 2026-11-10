// Package projects is the Projects destination: the projects Rashin knows,
// and one project's worktrees, commit graph, changes, tasks and activity.
package projects

import (
	"gioui.org/layout"

	"ryoku-i18n"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
)

// Project views, as navigation names them.
const (
	TabOverview = env.ViewOverview
	TabGraph    = env.ViewGraph
	TabChanges  = env.ViewChanges
	TabTasks    = env.ViewTasks
	TabActivity = env.ViewActivity
)

// Page is the Projects destination.
type Page struct {
	state pageState
}

// New returns the page with no project open.
func New() *Page {
	page := &Page{}
	page.state.init()
	return page
}

// Open shows a project's view, on a worktree when one is named. An empty
// project shows the page without one.
func (p *Page) Open(e *env.Env, project, view, worktree string) {
	p.state.open(e, project, view, worktree)
}

// Home is where the destination opens: the project last shown if it is still
// here, otherwise the first one. Projects has no page of its own above them.
func (p *Page) Home(e *env.Env) (project, view, worktree string, ok bool) {
	projects := e.Snapshot().Projects
	for _, info := range projects {
		if info.ID == p.state.project {
			return p.state.project, p.state.tab, p.state.worktree, true
		}
	}
	if len(projects) == 0 {
		return "", "", "", false
	}
	return projects[0].ID, TabOverview, "", true
}

// Hidden tells the page it left the screen, so it stops watching its project.
func (p *Page) Hidden(e *env.Env) { p.state.hidden(e) }

// Content draws the open project, or the way to add the first one.
func (p *Page) Content(gtx layout.Context, e *env.Env) layout.Dimensions {
	return p.state.layout(gtx, e)
}

// New asks for a folder to add as a project.
func (p *Page) New(e *env.Env) { p.state.add.ask(e) }

// NewLabel names the New action.
func (p *Page) NewLabel() string { return i18n.T("Add project") }

// Overlay draws the page's dialogs and those of its views over the whole
// window, so a dialog dims everything it interrupts, and reports whether one
// is open.
func (p *Page) Overlay(gtx layout.Context, e *env.Env) bool {
	state := &p.state
	state.add.layout(gtx, e)
	state.forget.layout(gtx, e, state)
	state.settings.layout(gtx, e, state)
	modal := look.AnyOpen(&state.add.dialog, &state.forget.confirm.Dialog, &state.settings.dialog)
	if state.project == "" {
		return modal
	}
	overview, changes, tasks := &state.overview, &state.changes, &state.tasks
	overview.layoutDialogs(gtx, e, state)
	changes.layoutConfirms(gtx, e, state)
	tasks.layoutDialogs(gtx, e, state)
	return state.graph.Overlay(gtx, e) || modal || look.AnyOpen(
		&overview.create.dialog, &overview.move.dialog, &overview.remove.dialog,
		&overview.prune.confirm.Dialog, &overview.cleanup.confirm.Dialog,
		&changes.discard.Dialog, &changes.force.Dialog,
		&tasks.create.dialog, &tasks.planner.dialog, &tasks.question.dialog, &tasks.remove.confirm.Dialog,
	)
}
