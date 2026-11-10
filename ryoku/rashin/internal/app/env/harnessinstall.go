package env

import (
	"gioui.org/layout"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/app/state"
	"ryoku-rashin/internal/protocol"
)

// the way to a harness that is not on this machine: its install page, and a second look once that is done
type HarnessInstall struct {
	pages   map[string]*kit.Plaque
	again   kit.Plaque
	looking bool
}

func (h *HarnessInstall) Rows(gtx layout.Context, e *Env, snapshot state.Snapshot) []look.Setting {
	th := e.Theme
	if h.pages == nil {
		h.pages = make(map[string]*kit.Plaque)
	}
	var rows []look.Setting
	for _, harness := range snapshot.Harnesses {
		if harness.Installed || harness.Custom {
			continue
		}
		row := look.Setting{Title: harness.Name, Detail: i18n.T("Not on this machine.")}
		if harness.Install != "" {
			page := h.pages[harness.ID]
			if page == nil {
				page = new(kit.Plaque)
				h.pages[harness.ID] = page
			}
			page.Kind, page.Small, page.Label, page.Icon = kit.PlaqueQuiet, true, i18n.T("How to install"), look.Icon.OpenInNew
			if page.Clicked(gtx) {
				e.Open(harness.Install)
			}
			row.Control = func(gtx layout.Context) layout.Dimensions { return page.Layout(gtx, th.Theme) }
		}
		rows = append(rows, row)
	}
	return rows
}

func (h *HarnessInstall) Again(gtx layout.Context, e *Env) layout.Dimensions {
	h.again.Kind, h.again.Small, h.again.Label, h.again.Busy = kit.PlaqueQuiet, true, i18n.T("Look again"), h.looking
	h.again.Icon = look.Icon.Retry
	if h.again.Clicked(gtx) && !h.looking {
		h.looking = true
		e.Call(protocol.MHarnessesList, protocol.HarnessesListParams{Refresh: true}, new(protocol.HarnessesResult), func(err error) {
			h.looking = false
			if err != nil {
				e.Notify(err.Error())
			}
		})
	}
	return h.again.Layout(gtx, e.Theme.Theme)
}
