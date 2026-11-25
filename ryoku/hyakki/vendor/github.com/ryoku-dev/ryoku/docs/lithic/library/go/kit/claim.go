package kit

import (
	"gioui.org/layout"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/theme"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/tokens"
)

// Claim pairs a tone flake with its plain state word.
func Claim(th *theme.Theme, tone palette.Tone, word string) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		ink := th.R.Stone(tone)
		if tone == palette.Idle {
			ink = th.R.Fg2
		}
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return Flake(gtx, th, tone, 1, tokens.SizeFlakeNominal)
			}),
			layout.Rigid(layout.Spacer{Width: th.Sp.S2}.Layout),
			layout.Rigid(Text(th, th.T.Label, ink, word)),
		)
	}
}
