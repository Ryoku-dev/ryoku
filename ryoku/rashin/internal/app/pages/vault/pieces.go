package vault

import (
	"image"

	"gioui.org/layout"
	"gioui.org/op"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"

	"ryoku-rashin/internal/app/look"
)

// a host or a fingerprint as a small engraved chip
func chip(th *look.Theme, value string) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		record := op.Record(gtx.Ops)
		dimensions := layout.Inset{Top: 2, Right: 6, Bottom: 2, Left: 6}.Layout(gtx, kit.Text(th.Theme, th.T.MonoSm, th.R.Fg2, value))
		call := record.Stop()
		kit.FillCut(gtx.Ops, image.Rectangle{Max: dimensions.Size}, gtx.Dp(3), th.R.Stone3)
		call.Add(gtx.Ops)
		return dimensions
	}
}

func chips(th *look.Theme, values []string) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		return look.Flow{Gap: 6, LineGap: 6}.Layout(gtx, len(values), func(gtx layout.Context, index int) layout.Dimensions {
			return chip(th, values[index])(gtx)
		})
	}
}

// a stone2 plane cut like a tablet under content inset 16
func plane(gtx layout.Context, th *look.Theme, body layout.Widget) layout.Dimensions {
	record := op.Record(gtx.Ops)
	dimensions := layout.UniformInset(16).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return body(gtx)
	})
	call := record.Stop()
	kit.FillCut(gtx.Ops, image.Rectangle{Max: dimensions.Size}, gtx.Dp(th.Cut.Mid), th.R.Stone2)
	call.Add(gtx.Ops)
	return dimensions
}
