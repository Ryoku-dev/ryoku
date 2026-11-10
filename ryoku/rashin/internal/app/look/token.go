package look

import (
	"fmt"
	"image"
	"image/color"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/tokens"
)

func TokenChip(th *Theme, value string) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		record := op.Record(gtx.Ops)
		dimensions := layout.Inset{Top: 2, Right: 6, Bottom: 2, Left: 6}.Layout(
			gtx,
			kit.Text(th.Theme, th.T.Mono, th.R.Fg2, value),
		)
		call := record.Stop()
		kit.FillCut(gtx.Ops, image.Rectangle{Max: dimensions.Size}, gtx.Dp(3), th.R.Stone3)
		call.Add(gtx.Ops)
		return dimensions
	}
}

// no chip, for places that already stand on a control
func DiffCount(th *Theme, added, removed int, on bool) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		addInk := tokenTextInk(th.R.Jade, th.R.Stone2, th.R.Fg)
		removeInk := tokenTextInk(th.R.Jasper, th.R.Stone2, th.R.Fg)
		if on {
			addInk, removeInk = th.R.OnSel, th.R.OnSel
		}
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(kit.RawText(th.Theme, th.T.MonoSm, addInk, fmt.Sprintf("+%d", added))),
			layout.Rigid(layout.Spacer{Width: unit.Dp(6)}.Layout),
			layout.Rigid(kit.RawText(th.Theme, th.T.MonoSm, removeInk, fmt.Sprintf("−%d", removed))),
		)
	}
}

func tokenTextInk(ink, background, toward color.NRGBA) color.NRGBA {
	if palette.ContrastRatio(ink, background) >= float64(tokens.ContrastMinimum) {
		return ink
	}
	for step := range 100 {
		candidate := palette.Mix(ink, toward, float32(step+1)/100)
		if palette.ContrastRatio(candidate, background) >= float64(tokens.ContrastMinimum) {
			return candidate
		}
	}
	return toward
}
