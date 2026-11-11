// Package routines is the Routines destination: what the teams do by
// themselves on a schedule, when each runs next, and the editor that sets it.
package routines

import (
	"gioui.org/layout"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/app/pages/teamsui"
	"ryoku-rashin/internal/protocol"
)

type Page struct {
	// selected is the routine open in the editor
	selected string

	list    look.SideList
	rowMenu kit.Menu
	rise    look.Rise

	board  board
	editor editor
	remove removeDialog

	schedules teamsui.Schedules
}

func New() *Page { return &Page{} }

func (p *Page) Select(e *env.Env, place env.TeamsPlace) {
	switch {
	case place.Draft == env.DraftRoutine:
		p.selected = ""
		// a draft in progress survives a second visit to its own place
		if !p.editor.creating || (place.Team != "" && place.Team != p.editor.teamID) {
			p.editor.startNew(e.Snapshot(), place.Team)
		}
	case place.Routine != "":
		p.selected = place.Routine
		p.editor.open(place.Routine)
	default:
		p.selected = ""
		p.editor.close()
	}
}

func (p *Page) Content(gtx layout.Context, e *env.Env) layout.Dimensions {
	key := p.selected
	if p.editor.creating {
		key = env.DraftRoutine
	}
	return p.rise.Layout(gtx, e.Theme, key, 8, func(gtx layout.Context) layout.Dimensions {
		if p.editor.creating {
			return p.editor.layout(gtx, e, p, protocol.Routine{})
		}
		if p.editor.id != "" {
			if routine, ok := p.editor.sync(e.Snapshot()); ok {
				return p.editor.layout(gtx, e, p, routine)
			}
			// a routine deleted while open leaves for the board; one not heard of yet is waited for
			if p.editor.loaded {
				p.selected = ""
				p.editor.close()
				openBoard(e)
			}
		}
		return p.board.layout(gtx, e, p)
	})
}

func (p *Page) New(e *env.Env) {
	e.NavigateTeams(env.TeamsPlace{Tab: protocol.TabRoutines, Draft: env.DraftRoutine})
}

func (p *Page) NewLabel() string { return i18n.T("New routine") }

func (p *Page) Overlay(gtx layout.Context, e *env.Env) bool {
	p.remove.layout(gtx, e, p)
	return look.AnyOpen(&p.remove.confirm.Dialog)
}
