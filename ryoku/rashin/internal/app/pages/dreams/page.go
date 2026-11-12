// Package dreams is the Dreams destination of Teams mode: every dream the
// user started, a dream's own page with where its manager stands, what it
// asks, its team, its work and what it presented, and the dialog that starts
// a new one.
package dreams

import (
	"gioui.org/layout"

	"ryoku-i18n"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/protocol"
)

// Page is the Dreams destination.
type Page struct {
	place env.TeamsPlace

	list   look.SideList
	board  board
	view   dreamView
	create createDialog
	notes  notesDialog
}

// New returns the page showing every dream.
func New() *Page { return &Page{} }

// Select shows a dream, the list of them, or the list with the New dream
// dialog open.
func (p *Page) Select(e *env.Env, place env.TeamsPlace) {
	if place.Draft == env.DraftDream {
		p.create.open()
		place.Draft = ""
	}
	p.place = place
}

// Content draws the open dream or the list of every dream.
func (p *Page) Content(gtx layout.Context, e *env.Env) layout.Dimensions {
	if dream, ok := dreamByID(e.Snapshot().Teams, p.place.Team); ok {
		return p.view.layout(gtx, e, p, dream)
	}
	return p.board.layout(gtx, e, p)
}

// New opens the New dream dialog.
func (p *Page) New(e *env.Env) { p.create.open() }

// NewLabel names the New action.
func (p *Page) NewLabel() string { return i18n.T("New dream") }

// Overlay draws the page's dialogs over the whole window and reports whether
// one is open.
func (p *Page) Overlay(gtx layout.Context, e *env.Env) bool {
	p.create.layout(gtx, e)
	p.notes.layout(gtx, e)
	return look.AnyOpen(&p.create.dialog, &p.notes.dialog)
}

func dreamByID(teams []protocol.Team, id string) (protocol.Team, bool) {
	for _, team := range teams {
		if team.ID == id && team.Dream != nil {
			return team, true
		}
	}
	return protocol.Team{}, false
}

func openDream(e *env.Env, id string) {
	e.NavigateTeams(env.TeamsPlace{Tab: protocol.TabDreams, Team: id})
}
