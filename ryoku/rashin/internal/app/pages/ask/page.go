// Package ask is the Ask destination: questions about this machine, answered
// by the model router.
package ask

import (
	"gioui.org/layout"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/anim"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/chat"
	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/protocol"
)

// a new conversation has no session, the first message makes it
type Page struct {
	selected string
	// held from sending the first message until its session is listed, the answer can arrive before the list does
	starting bool
	// route picked before a session exists; a model picked here is held to, never swapped for another
	draftRoute string
	// what was typed in a conversation and not sent, kept while another one is open
	drafts map[string]chat.Draft

	list        look.SideList
	rowMenu     kit.Menu
	header      look.PaneHeader
	view        chat.View
	composer    chat.Composer
	route       look.Select
	approvals   env.Approvals
	rise        look.Rise
	travel      anim.Tween
	suggestions [4]kit.Plaque
	ready       readiness
	usage       env.Usage

	menu env.ModelMenu
	// the open model menu asks for the providers' allowances
	limits env.LimitsWatch

	temporary env.Temporary
	memory    memoryChoice
	lines     env.MemoryLines
	first     firstRun
	// a temporary conversation that was kept: its copy is in history, so it ends without a word
	kept string

	rename  renameDialog
	remove  removeDialog
	handoff handoffDialog
	keep    keepDialog
}

func New() *Page { return &Page{} }

func (p *Page) Select(e *env.Env, id string) {
	p.composer.Focus()
	if p.selected == id {
		return
	}
	p.leave(e)
	p.keepDraft(id)
	p.selected, p.starting = id, false
	if id == "" {
		return
	}
	e.Attach(id, func(err error) {
		if err != nil {
			e.Notify(err.Error())
		}
	})
}

func (p *Page) FocusHome() any { return p.composer.FocusTag() }

func (p *Page) keepDraft(next string) {
	if p.drafts == nil {
		p.drafts = make(map[string]chat.Draft)
	}
	p.drafts[p.selected] = p.composer.Draft()
	p.composer.SetDraft(p.drafts[next])
	delete(p.drafts, next)
}

func (p *Page) New(e *env.Env) {
	e.Navigate(protocol.TabAsk, "")
	p.composer.Focus()
}

func (p *Page) NewLabel() string { return i18n.T("New ask") }

func (p *Page) start(e *env.Env, submission chat.Submission) {
	p.starting = true
	var info protocol.SessionInfo
	settings := e.Snapshot().Settings.Ask
	params := protocol.SessionNewParams{
		Kind: protocol.KindAsk, Origin: protocol.OriginApp, Model: p.draftRoute,
		Mode:      p.approvals.Value("", settings.Approvals),
		Temporary: p.temporary.On, Memory: memoryValue(p.memory.draft, p.temporary.On),
	}
	// the next new conversation is an ordinary one again
	p.temporary.On = false
	e.Call(protocol.MSessionNew, params, &info, func(err error) {
		if err != nil {
			p.starting, p.temporary.On = false, params.Temporary
			p.composer.SetText(submission.Text)
			e.Notify(err.Error())
			return
		}
		p.selected = info.ID
		e.Attach(info.ID, func(error) {})
		e.Navigate(protocol.TabAsk, info.ID)
		if !pinned(params.Model) {
			e.Submit(info.ID, false, submission)
			return
		}
		// a model the reader picked answers or says why it can't; session.new alone lets it fail over
		e.Call(protocol.MSessionSetModel, protocol.SetModelParams{ID: info.ID, Model: params.Model}, new(protocol.Empty), func(err error) {
			if err != nil {
				p.composer.SetText(submission.Text)
				e.Notify(err.Error())
				return
			}
			e.Submit(info.ID, false, submission)
		})
	})
}

// leaving a temporary conversation ends it, opening another or a new one alike
func (p *Page) leave(e *env.Env) {
	info, ok := e.Snapshot().Sessions[p.selected]
	if !ok || !info.Temporary {
		return
	}
	e.EndTemporary(info.ID, info.ID != p.kept)
}

func (p *Page) Overlay(gtx layout.Context, e *env.Env) bool {
	p.rename.layout(gtx, e)
	p.remove.layout(gtx, e, p)
	p.handoff.layout(gtx, e)
	p.keep.layout(gtx, e, p)
	return look.AnyOpen(&p.rename.dialog, &p.remove.dialog, &p.handoff.dialog, &p.keep.confirm.Dialog)
}
