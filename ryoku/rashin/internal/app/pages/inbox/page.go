// Package inbox is the home of Teams mode: what needs the user, the reports
// their teams sent back, what works now and what comes next.
package inbox

import (
	"gioui.org/layout"

	"ryoku-i18n"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/app/pages/teamsui"
)

type Page struct {
	list        look.SideList
	board       board
	suggestions teamsui.Suggestions
}

func New() *Page { return &Page{} }

// a place naming a task brings what that task waits on into view
func (p *Page) Select(e *env.Env, place env.TeamsPlace) {
	if place.Task != "" {
		p.board.reveal = place.Task
	}
}

func (p *Page) Content(gtx layout.Context, e *env.Env) layout.Dimensions {
	return p.board.layout(gtx, e, p.needs(e))
}

// what needs the user, with the memories waiting to be kept or dismissed among them
func (p *Page) needs(e *env.Env) []teamsui.Need {
	p.suggestions.Want(e)
	return teamsui.WithSuggestions(teamsui.Needs(e.Snapshot()), p.suggestions.Needs())
}

// Count is how many cards wait in the Inbox: every need, and one for each place memories were suggested from.
func (p *Page) Count(e *env.Env) int { return len(p.needs(e)) }

func (p *Page) New(e *env.Env) { e.NewTeamTask("") }

func (p *Page) NewLabel() string { return i18n.T("New task") }

// the inbox decides inline, it opens no dialog of its own
func (p *Page) Overlay(gtx layout.Context, e *env.Env) bool { return false }
