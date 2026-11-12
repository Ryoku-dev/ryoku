// Package teams is the Teams destination: every team as a tile, a team's own
// page with its crew, work, routines, spend and settings, and the draft that
// creates a team from a template.
package teams

import (
	"gioui.org/layout"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/app/pages/teamsui"
	"ryoku-rashin/internal/protocol"
)

// the views of a team page in the order its tabs stand
var teamViews = [...]string{env.TeamOverview, env.TeamWork, env.TeamRoutines, env.TeamSpend, env.TeamSettings}

// Page is the Teams destination.
type Page struct {
	place env.TeamsPlace

	list   look.SideList
	header look.PaneHeader
	tabs   kit.Tabs
	rise   look.Rise

	templates teamsui.Templates

	board    board
	overview overview
	work     workTab
	routines routinesTab
	spend    spendTab
	settings settingsTab
	draft    draft

	member  memberDialog
	drop    dropMember
	remove  removeTeam
	imports importDialog
}

// New returns the page showing every team.
func New() *Page { return &Page{} }

// Select shows a place of the Teams destination: the board, a team's view or
// the draft of a new team.
func (p *Page) Select(e *env.Env, place env.TeamsPlace) {
	known := false
	for _, view := range teamViews {
		known = known || place.View == view
	}
	if !known {
		place.View = env.TeamOverview
	}
	if place.Draft == env.DraftTeam {
		p.draft.open(place.Template)
	}
	// opening a team's page reads its folder again, the folder has no event of its own
	p.overview.files.stale()
	p.place = place
}

// Content draws the draft, the open team or the board of every team.
func (p *Page) Content(gtx layout.Context, e *env.Env) layout.Dimensions {
	if p.place.Draft == env.DraftTeam {
		return p.draft.layout(gtx, e, p)
	}
	if team, ok := teamsui.TeamByID(e.Snapshot().Teams, p.place.Team); ok {
		return p.layoutTeam(gtx, e, team)
	}
	return p.board.layout(gtx, e, p)
}

// New opens the draft of a new team.
func (p *Page) New(e *env.Env) {
	e.NavigateTeams(env.TeamsPlace{Tab: protocol.TabTeams, Draft: env.DraftTeam})
}

// NewLabel names the New action.
func (p *Page) NewLabel() string { return i18n.T("New team") }

// Overlay draws the page's dialogs over the whole window and reports whether
// one is open.
func (p *Page) Overlay(gtx layout.Context, e *env.Env) bool {
	p.member.layout(gtx, e)
	p.drop.layout(gtx, e)
	p.remove.layout(gtx, e)
	p.imports.layout(gtx, e)
	return look.AnyOpen(&p.member.dialog, &p.drop.confirm.Dialog, &p.remove.confirm.Dialog, &p.imports.dialog)
}

// the Overview's Memory section previews the team's brief beside the content
func (p *Page) HasPanel(e *env.Env) bool {
	_, ok := teamsui.TeamByID(e.Snapshot().Teams, p.place.Team)
	return ok && p.place.Draft == "" && p.place.View == env.TeamOverview && p.overview.memory.sheet.IsOpen()
}

func (p *Page) Panel(gtx layout.Context, e *env.Env) layout.Dimensions {
	team, _ := teamsui.TeamByID(e.Snapshot().Teams, p.place.Team)
	return p.overview.memory.layoutSheet(gtx, e, team)
}

func openTeam(e *env.Env, team, view string) {
	e.NavigateTeams(env.TeamsPlace{Tab: protocol.TabTeams, Team: team, View: view})
}
