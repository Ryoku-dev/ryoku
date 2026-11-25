package memory

import (
	"gioui.org/layout"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/app/pages/memoryui"
	"ryoku-rashin/internal/protocol"
)

// What Ask sees stands in the side panel, which docks beside the column or rises over it as the window allows
func (p *Page) openSheet(e *env.Env) {
	p.card = nil
	p.sheet.Open(e)
}

func (p *Page) handleSheet(gtx layout.Context, e *env.Env) {
	if p.close.Clicked(gtx) {
		p.sheet.Close(e)
	}
	if p.rebuild.Clicked(gtx) && !p.rebuild.Busy {
		p.rebuild.Busy = true
		e.MemoryCard(true, func(card protocol.MemoryCard, err error) {
			p.rebuild.Busy = false
			if err != nil {
				e.Report(err)
				return
			}
			p.card = &card
		})
	}
}

func (p *Page) HasPanel(*env.Env) bool { return p.sheet.IsOpen() }

func (p *Page) Panel(gtx layout.Context, e *env.Env) layout.Dimensions {
	th := e.Theme
	card := memoryStatus(e).Card
	if p.card != nil {
		card = *p.card
	}
	proof := i18n.Tf("Built from %d memories about you, %s %s.", card.From, memoryui.Day(gtx.Now, card.Built), card.Built.Local().Format("15:04"))
	text := card.Text
	if text == "" {
		proof, text = "", i18n.T("Nothing is put into Ask's prompt yet. Memories about you build it.")
	}
	p.rebuild.Kind, p.rebuild.Label = kit.PlaqueTonal, i18n.T("Rebuild")
	p.close.Kind, p.close.Label = kit.PlaqueQuiet, i18n.T("Close")
	return layout.UniformInset(th.Sp.S5).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return memoryui.SheetBody(gtx, th, i18n.T("What Ask sees"), proof, text, i18n.T("This is made from the list. To change it, change the memories."),
			func(gtx layout.Context) layout.Dimensions { return p.rebuild.Layout(gtx, th.Theme) },
			func(gtx layout.Context) layout.Dimensions {
				return look.Named(gtx, i18n.T("Close What Ask sees"), func(gtx layout.Context) layout.Dimensions { return p.close.Layout(gtx, th.Theme) })
			},
		)
	})
}
