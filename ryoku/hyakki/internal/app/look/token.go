package look

import (
	"fmt"
	"image"
	"image/color"

	"gioui.org/io/semantic"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/tokens"
)

func TokenChip(th *Theme, value string) layout.Widget {
	return chip(th, value, func(gtx layout.Context) layout.Dimensions {
		return kit.LayoutTracked(gtx, th.Theme, th.T.Mono, th.R.Fg2, 0, value)
	})
}

// Chip is a word on a field plane, for a trait worth one glance: what a model
// can do, what a file is. A tone inks the word and its icon; Idle is quiet.
func Chip(th *Theme, icon *widget.Icon, word string, tone palette.Tone) layout.Widget {
	ink := th.R.Fg2
	if tone != palette.Idle {
		ink = tokenTextInk(th.R.Stone(tone), th.R.Field, th.R.Fg)
	}
	words := func(gtx layout.Context) layout.Dimensions {
		return kit.LayoutTracked(gtx, th.Theme, th.T.Caption, ink, 0, word)
	}
	if icon == nil {
		return chip(th, word, words)
	}
	return chip(th, word, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(IconWidget(icon, 14, ink)),
			layout.Rigid(layout.Spacer{Width: th.Sp.S1}.Layout),
			layout.Rigid(words),
		)
	})
}

func chip(th *Theme, name string, content layout.Widget) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min = image.Point{}
		record := op.Record(gtx.Ops)
		dimensions := layout.Inset{Top: 2, Right: 6, Bottom: 2, Left: 6}.Layout(gtx, content)
		call := record.Stop()
		kit.Fill(gtx, th.Theme, image.Rectangle{Max: dimensions.Size}, gtx.Dp(th.Cut.Token), th.R.Field)
		area := clip.Rect(image.Rectangle{Max: dimensions.Size}).Push(gtx.Ops)
		if name != "" {
			semantic.LabelOp(name).Add(gtx.Ops)
		}
		call.Add(gtx.Ops)
		area.Pop()
		return dimensions
	}
}

// no chip, for places that already stand on a control
func DiffCount(th *Theme, added, removed int, on bool) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		pane := th.Planes().Pane
		addInk := tokenTextInk(th.R.Jade, pane, th.R.Fg)
		removeInk := tokenTextInk(th.R.Jasper, pane, th.R.Fg)
		if on {
			addInk, removeInk = th.R.OnSel, th.R.OnSel
		}
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(kit.RawText(th.Theme, th.T.Caption, addInk, fmt.Sprintf("+%d", added))),
			layout.Rigid(layout.Spacer{Width: unit.Dp(6)}.Layout),
			layout.Rigid(kit.RawText(th.Theme, th.T.Caption, removeInk, fmt.Sprintf("−%d", removed))),
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
