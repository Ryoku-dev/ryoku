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

	list        look.SideList
	rowMenu     kit.Menu
	header      look.PaneHeader
	view        chat.View
	composer    chat.Composer
	route       look.Select
	rise        look.Rise
	travel      anim.Tween
	suggestions [4]kit.Plaque

	models protocol.ModelsResult
	loaded bool

	rename  renameDialog
	remove  removeDialog
	handoff handoffDialog
}

func New() *Page { return &Page{} }

func (p *Page) Select(e *env.Env, id string) {
	if id == "" {
		p.selected, p.starting = "", false
		p.composer.Focus()
		return
	}
	if p.selected == id {
		return
	}
	p.selected, p.starting = id, false
	e.Attach(id, func(err error) {
		if err != nil {
			e.Notify(err.Error())
		}
	})
}

func (p *Page) New(e *env.Env) {
	e.Navigate(protocol.TabAsk, "")
	p.composer.Focus()
}

func (p *Page) NewLabel() string { return i18n.T("New ask") }

func (p *Page) start(e *env.Env, submission chat.Submission) {
	p.starting = true
	var info protocol.SessionInfo
	params := protocol.SessionNewParams{Kind: protocol.KindAsk, Origin: protocol.OriginApp, Model: p.draftRoute}
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
		}
	})
}

func (p *Page) routeOptions(current string) (labels, ids []string, selected int) {
	for _, route := range p.models.Routes {
		labels, ids = append(labels, route.Name), append(ids, route.ID)
	}
	models := append([]protocol.ModelInfo(nil), p.models.Models...)
	sort.SliceStable(models, func(i, j int) bool { return models[i].Provider+models[i].Name < models[j].Provider+models[j].Name })
	for _, model := range models {
		if model.Available {
			labels, ids = append(labels, model.Name), append(ids, model.Provider+"/"+model.ID)
		}
	}
	if len(labels) == 0 {
		return []string{i18n.T("Auto")}, []string{protocol.RouteAuto}, 0
	}
	for index, id := range ids {
		if id == current {
			selected = index
		}
	}
	return labels, ids, selected
}

func (p *Page) Overlay(gtx layout.Context, e *env.Env) bool {
	p.rename.layout(gtx, e)
	p.remove.layout(gtx, e, p)
	p.handoff.layout(gtx, e)
	return look.AnyOpen(&p.rename.dialog, &p.remove.confirm.Dialog, &p.handoff.dialog)
}
