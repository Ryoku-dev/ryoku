// Package ask is the Ask destination: questions about this machine, answered
// by the model router.
package ask

import (
	"strconv"
	"time"

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
	selected     string
	archivedOpen bool
	newAsk       bool
	// held from sending the first message until its session is listed, the answer can arrive before the list does
	starting bool
	// route picked before a session exists; a model picked here is held to, never swapped for another
	draftRoute string
	// what was typed in a conversation and not sent, kept while another one is open
	drafts map[string]chat.Draft
	// Fresh-tour drafts live in the same map, under non-session keys.
	// Their order keeps repeated fresh starts from replacing earlier work.
	freshDrafts  []string
	restoreDraft kit.Plaque

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
func (p *Page) Sheet(e *env.Env) look.SheetPage {
	head := look.PageFor(protocol.TabAsk)
	if info, ok := e.Snapshot().Sessions[p.selected]; ok {
		head.Where = look.Where(head.Where, sessionTitle(info))
		if info.Model != "" {
			head.Facts = []string{env.ModelWords(e, info.Model)}
		}
	}
	return head
}

func (p *Page) SheetProgress() float32 {
	if p.selected == "" {
		return -1
	}
	return p.view.Progress()
}

func (p *Page) Select(e *env.Env, id string) {
	p.composer.RefreshAskTools()
	if id == "" && !p.newAsk && e.Feature("history") && p.composer.Draft().Text == "" {
		snapshot := e.Snapshot()
		if minutes := snapshot.Settings.Ask.ResumeMinutes; minutes > 0 {
			id = resumeSession(snapshot.Order, snapshot.Sessions, time.Now(), time.Duration(minutes)*time.Minute)
			if id != "" {
				e.Navigate(protocol.TabAsk, id)
			}
		}
	}
	p.newAsk = false
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

func (p *Page) FocusHome() any     { return p.composer.FocusTag() }
func (p *Page) SearchAtRest() bool { return p.composer.Draft().Text == "" }

func (p *Page) Hidden(e *env.Env) { p.composer.CancelAskTools() }

func (p *Page) WritingRequest() string { return p.composer.WritingRequest() }

func (p *Page) keepDraft(next string) {
	if p.drafts == nil {
		p.drafts = make(map[string]chat.Draft)
	}
	p.drafts[p.selected] = p.composer.Draft()
	p.composer.SetDraft(p.drafts[next])
	delete(p.drafts, next)
}

func (p *Page) New(e *env.Env) {
	p.newAsk = true
	e.Navigate(protocol.TabAsk, "")
	p.composer.Focus()
}

// Fresh starts a genuinely blank Ask without resuming history or restoring
// the older blank-session draft. Existing session drafts keep their keys.
func (p *Page) Fresh(e *env.Env) {
	if p.selected == "" {
		p.keepFreshDraft(p.composer.Draft())
		p.composer.SetDraft(chat.Draft{})
	} else {
		p.keepFreshDraft(p.drafts[""])
		delete(p.drafts, "")
	}
	p.New(e)
}

func (p *Page) keepFreshDraft(draft chat.Draft) {
	if draft.Text == "" && len(draft.Attachments) == 0 {
		return
	}
	if p.drafts == nil {
		p.drafts = make(map[string]chat.Draft)
	}
	// NUL cannot occur in a real session ID.
	id := "\x00fresh/" + strconv.Itoa(len(p.freshDrafts))
	p.drafts[id] = draft
	p.freshDrafts = append(p.freshDrafts, id)
}

func (p *Page) restorePreviousDraft() {
	if len(p.freshDrafts) == 0 {
		return
	}
	last := len(p.freshDrafts) - 1
	id := p.freshDrafts[last]
	draft, current := p.drafts[id], p.composer.Draft()
	if current.Text != "" || len(current.Attachments) != 0 {
		// Restoring must not discard a new question typed in the meantime.
		p.drafts[id] = current
	} else {
		delete(p.drafts, id)
		p.freshDrafts = p.freshDrafts[:last]
	}
	p.composer.SetDraft(draft)
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

func resumeSession(order []string, sessions map[string]protocol.SessionInfo, now time.Time, window time.Duration) string {
	var newest string
	var newestAt time.Time
	for _, id := range order {
		info := sessions[id]
		if info.Kind != protocol.KindAsk || info.Temporary || info.Archived || info.Turns == 0 {
			continue
		}
		at := info.LastMessageAt
		if at.IsZero() {
			continue
		}
		if elapsed := now.Sub(at); elapsed < 0 || elapsed > window {
			continue
		}
		if newest == "" || at.After(newestAt) {
			newest, newestAt = info.ID, at
		}
	}
	return newest
}

func (p *Page) Overlay(gtx layout.Context, e *env.Env) bool {
	p.rename.layout(gtx, e)
	p.remove.layout(gtx, e, p)
	p.handoff.layout(gtx, e)
	p.keep.layout(gtx, e, p)
	return look.AnyOpen(&p.rename.dialog, &p.remove.dialog, &p.handoff.dialog, &p.keep.confirm.Dialog)
}
