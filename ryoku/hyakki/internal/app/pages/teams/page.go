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
	sheet  look.Sheet
	page   look.SheetPage
	reveal look.Reveal

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
	p.reveal.Reset()
}

// Sheet gives the frame this team's place, its view and the facts beneath its number
func (p *Page) Sheet(e *env.Env) look.SheetPage {
	p.page = look.PageFor(protocol.TabTeams)
	if p.place.Draft == env.DraftTeam {
		p.page = look.PartPage(protocol.TabTeams, 1, i18n.T("New team"), "")
		return p.page
	}
	if team, ok := teamsui.TeamByID(e.Snapshot().Teams, p.place.Team); ok {
		view := p.place.View
		for index, candidate := range teamViews {
			if view == candidate {
				p.page = look.PartPage(protocol.TabTeams, index+1, p.teamViewName(index), "")
				break
			}
		}
		p.page.Where = look.Where(i18n.T("Teams"), team.Name, p.teamViewName(p.teamViewIndex()))
		p.page.Facts = []string{membersWord(len(team.Members))}
	}
	return p.page
}

func (p *Page) teamViewIndex() int {
	for index, view := range teamViews {
		if p.place.View == view {
			return index
		}
	}
	return 0
}

func (p *Page) teamViewName(index int) string {
	return [...]string{i18n.T("Overview"), i18n.T("Work"), i18n.T("Routines"), i18n.T("Spend"), i18n.T("Settings")}[index]
}

func (p *Page) SheetProgress() float32 {
	if p.place.Draft == env.DraftTeam {
		return p.draft.scroll.Progress(1)
	}
	if p.place.Team == "" {
		return p.board.scroll.Progress(1)
	}
	switch p.place.View {
	case env.TeamOverview:
		return p.overview.scroll.Progress(1)
	case env.TeamWork:
		return p.work.scroll.Progress(1)
	case env.TeamRoutines:
		return p.routines.scroll.Progress(1)
	case env.TeamSpend:
		return p.spend.scroll.Progress(1)
	case env.TeamSettings:
		return p.settings.scroll.Progress(sectionCount + 4)
	}
	return -1
}

// Content draws the draft, the open team or the board of every team.
func (p *Page) Content(gtx layout.Context, e *env.Env) layout.Dimensions {
	if p.place.Draft == env.DraftTeam {
		return p.draft.layout(gtx, e, p)
	}
	if team, ok := teamsui.TeamByID(e.Snapshot().Teams, p.place.Team); ok {
		if p.place.View == env.TeamSettings {
			return p.sheet.Layout(gtx, e.Theme, p.page, func(gtx layout.Context) layout.Dimensions {
				return p.layoutTeam(gtx, e, team)
			})
		}
		return p.layoutTeam(gtx, e, team)
	}
	return p.sheet.Layout(gtx, e.Theme, p.page, func(gtx layout.Context) layout.Dimensions {
		return p.board.layout(gtx, e, p)
	})
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
