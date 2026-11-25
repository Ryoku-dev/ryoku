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
		if draft := place.DreamDraft; draft != nil {
			p.create.statement.SetText(draft.Statement)
			p.create.name.SetText(draft.Name)
			p.create.folder.SetText(draft.Folder)
			p.create.engine.Set(draft.Model, draft.Options)
			if draft.MonthTokens > 0 {
				p.create.budget.Selected, p.create.limit.Value = 1, draft.MonthTokens
			}
			if e.Feature("research") && draft.Preset == "research" {
				p.create.presetID = "research"
			} else if draft.Preset == "research" {
				e.Notify(i18n.T("Turn on Deep research in Settings to use this preset"))
			}
		}
		place.DreamDraft = nil
		place.Draft = ""
	}
	// arriving at the board plays its rise again; a dream's page plays its own when another dream opens
	if place.Team == "" && p.place.Team != "" {
		p.board.reveal.Reset()
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

// Sheet names the open dream with its phase and standing.
func (p *Page) Sheet(e *env.Env) look.SheetPage {
	snapshot := e.Snapshot()
	if dream, ok := dreamByID(snapshot.Teams, p.place.Team); ok {
		page := look.PartPage(protocol.TabDreams, 1, dream.Name, "")
		word, _, _ := standing(snapshot, dream)
		page.Facts = []string{word}
		if !finished(dream) {
			page.Facts = []string{phaseWord(dream.Dream.Phase), word}
		}
		return page
	}
	return look.PageFor(protocol.TabDreams)
}

func (p *Page) SheetProgress() float32 {
	if p.place.Team != "" {
		return p.view.scroll.Progress(p.view.rows)
	}
	return p.board.scroll.Progress(p.board.rows)
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
