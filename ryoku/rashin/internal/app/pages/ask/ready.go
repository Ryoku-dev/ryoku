package ask

import (
	"gioui.org/layout"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/protocol"
)

// what can answer right now, said under a new ask before the question is written
type readiness struct{ providers kit.Plaque }

func (r *readiness) layout(gtx layout.Context, e *env.Env) layout.Dimensions {
	th := e.Theme
	word, tone, detail := readyWords(e.Snapshot().Providers)
	r.providers.Kind, r.providers.Small, r.providers.Label = kit.PlaqueQuiet, true, i18n.T("Providers")
	if tone != palette.Alive {
		r.providers.Kind, r.providers.Label = kit.PlaqueTonal, i18n.T("Open providers")
	}
	if r.providers.Clicked(gtx) && e.OpenSettings != nil {
		e.OpenSettings("providers")
	}
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(kit.Claim(th.Theme, tone, word)),
		layout.Rigid(layout.Spacer{Width: th.Sp.S3}.Layout),
		layout.Flexed(1, kit.Text(th.Theme, th.T.Detail, th.R.Fg2, detail)),
		layout.Rigid(layout.Spacer{Width: th.Sp.S3}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions { return r.providers.Layout(gtx, th.Theme) }),
	)
}

func readyWords(providers []protocol.ProviderInfo) (word string, tone palette.Tone, detail string) {
	ready, models := 0, 0
	for _, provider := range providers {
		if provider.Enabled && provider.Available > 0 {
			ready++
			models += provider.Available
		}
	}
	switch {
	case ready == 0:
		return i18n.T("Nothing can answer"), palette.Attention, i18n.T("Every provider is resting or needs a key. A free key adds one.")
	case ready == 1 && models == 1:
		return i18n.T("Ready"), palette.Alive, i18n.T("One provider can answer now, with one model.")
	case ready == 1:
		return i18n.T("Ready"), palette.Alive, i18n.Tf("One provider can answer now, with %d models.", models)
	}
	return i18n.T("Ready"), palette.Alive, i18n.Tf("%d providers can answer now, with %d models between them.", ready, models)
}
