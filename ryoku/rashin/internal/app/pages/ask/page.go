// Package ask is the Ask destination: questions about this machine, answered
// by the free-model router.
package ask

import (
	"sort"

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
	// route picked before a session exists
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

	models protocol.ModelsResult
	loaded bool
	// the route select's rows, built once when the models arrive
	routes   []look.SelectOption
	routeIDs []string

	rename  renameDialog
	remove  removeDialog
	handoff handoffDialog
}

func New() *Page { return &Page{} }

func (p *Page) Select(e *env.Env, id string) {
	p.composer.Focus()
	if p.selected == id {
		return
	}
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
	params := protocol.SessionNewParams{
		Kind: protocol.KindAsk, Origin: protocol.OriginApp, Model: p.draftRoute,
		Mode: p.approvals.Value("", e.Snapshot().Settings.Ask.Approvals),
	}
	e.Call(protocol.MSessionNew, params, &info, func(err error) {
		if err != nil {
			p.starting = false
			p.composer.SetText(submission.Text)
			e.Notify(err.Error())
			return
		}
		p.selected = info.ID
		e.Attach(info.ID, func(error) {})
		e.Navigate(protocol.TabAsk, info.ID)
		e.Submit(info.ID, false, submission)
	})
}

func (p *Page) loadModels(e *env.Env) {
	if p.loaded {
		return
	}
	p.loaded = true
	e.Call(protocol.MModelsList, protocol.ModelsParams{}, &p.models, func(err error) {
		if err != nil {
			p.loaded = false
			return
		}
		p.routes, p.routeIDs = routeList(p.models)
	})
}

// the routes first, then every model a provider in use offers, named with where it comes from
func routeList(models protocol.ModelsResult) (options []look.SelectOption, ids []string) {
	for _, route := range models.Routes {
		options, ids = append(options, look.SelectOption{Label: route.Name, Detail: route.Description}), append(ids, route.ID)
	}
	offered := append([]protocol.ModelInfo(nil), models.Models...)
	sort.SliceStable(offered, func(i, j int) bool { return offered[i].Provider+offered[i].Name < offered[j].Provider+offered[j].Name })
	for _, model := range offered {
		if model.Available {
			id := model.Provider + "/" + model.ID
			options, ids = append(options, look.SelectOption{Label: model.Name, Detail: id}), append(ids, id)
		}
	}
	return options, ids
}

func (p *Page) routeOptions(current string) (options []look.SelectOption, ids []string, selected int) {
	if len(p.routes) == 0 {
		return []look.SelectOption{{Label: i18n.T("Auto")}}, []string{protocol.RouteAuto}, 0
	}
	for index, id := range p.routeIDs {
		if id == current {
			selected = index
		}
	}
	return p.routes, p.routeIDs, selected
}

func (p *Page) Overlay(gtx layout.Context, e *env.Env) bool {
	p.rename.layout(gtx, e)
	p.remove.layout(gtx, e, p)
	p.handoff.layout(gtx, e)
	return look.AnyOpen(&p.rename.dialog, &p.remove.confirm.Dialog, &p.handoff.dialog)
}
