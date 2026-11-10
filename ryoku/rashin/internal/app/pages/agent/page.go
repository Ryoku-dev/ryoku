// Package agent is the Agents destination: the board of what every agent
// session is doing across projects, and the sessions themselves.
package agent

import (
	"gioui.org/layout"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/chat"
	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/protocol"
)

type Page struct {
	selected string

	list    look.SideList
	rowMenu kit.Menu
	header  look.PaneHeader
	rise    look.Rise

	board    board
	view     chat.View
	composer chat.Composer
	model    look.Select
	mode     look.Select
	panel    changesPanel

	create createDialog
	rename renameDialog
	remove removeDialog
	revert revertDialog

	// the session we were told to open, until the session list has heard of it
	opening string

	// daemon connection the worktrees were last read on
	loadedEpoch int
}

func New() *Page { return &Page{} }

func (p *Page) Select(e *env.Env, id string) {
	if id == "" {
		p.selected, p.opening = "", ""
		return
	}
	if p.selected == id {
		return
	}
	p.selected, p.opening = id, id
	e.Attach(id, func(err error) {
		if err != nil {
			// can't be opened, stop waiting so the page goes back to its board
			if p.opening == id {
				p.opening = ""
			}
			e.Notify(err.Error())
			return
		}
		// opening a session counts as reading it
		e.Call(protocol.MSessionRead, protocol.SessionRef{ID: id}, new(protocol.Empty), nil)
	})
}

func (p *Page) New(e *env.Env) {
	e.Navigate(protocol.TabAgent, "")
	p.board.focus(e)
}

func (p *Page) NewLabel() string { return i18n.T("New session") }

func (p *Page) NeedsYou(e *env.Env) int {
	return len(groupSessions(e.Snapshot())[groupNeeds])
}

// sessions name their worktree by path, its name, base and changes come from here
func (p *Page) loadWorktrees(e *env.Env) {
	snapshot := e.Snapshot()
	if p.loadedEpoch == snapshot.Epoch {
		return
	}
	p.loadedEpoch = snapshot.Epoch
	for _, project := range snapshot.Projects {
		e.LoadWorktrees(project.ID, nil)
	}
}

// a session still being opened isn't lost, it's waited for until the session list has it
func (p *Page) lost(known bool) bool {
	if known {
		p.opening = ""
		return false
	}
	return p.selected != "" && p.opening != p.selected
}

func (p *Page) Content(gtx layout.Context, e *env.Env) layout.Dimensions {
	p.loadWorktrees(e)
	snapshot := e.Snapshot()
	info, known := snapshot.Sessions[p.selected]
	if p.lost(known) {
		p.selected = ""
		e.Navigate(protocol.TabAgent, "")
	}
	return p.rise.Layout(gtx, e.Theme, p.selected, 8, func(gtx layout.Context) layout.Dimensions {
		if !known {
			return p.board.layout(gtx, e, p)
		}
		return p.layoutSession(gtx, e, info)
	})
}

func (p *Page) Overlay(gtx layout.Context, e *env.Env) bool {
	p.create.layout(gtx, e)
	p.rename.layout(gtx, e)
	p.remove.layout(gtx, e, p)
	p.revert.layout(gtx, e)
	return look.AnyOpen(&p.create.dialog, &p.rename.dialog, &p.remove.confirm.Dialog, &p.revert.confirm.Dialog)
}

func open(e *env.Env, id string) { e.Navigate(protocol.TabAgent, id) }
