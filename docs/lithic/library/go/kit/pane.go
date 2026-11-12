package kit

import (
	"image"

	"gioui.org/layout"
	"gioui.org/op"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/theme"
)

// PaneKind selects the plane used by a pane.
type PaneKind uint8

const (
	PaneList PaneKind = iota
	PaneWork
)

// Pane is a flat application plane with cut top corners.
type Pane struct {
	Kind PaneKind
}

// Layout paints the pane behind body.
func (pane Pane) Layout(gtx layout.Context, th *theme.Theme, body layout.Widget) layout.Dimensions {
	record := op.Record(gtx.Ops)
	var dimensions layout.Dimensions
	if body != nil {
		dimensions = body(gtx)
	} else {
		dimensions.Size = gtx.Constraints.Min
	}
	call := record.Stop()
	ink := th.R.Stone1
	if pane.Kind == PaneWork {
		ink = th.R.Stone2
	}
	bounds := image.Rectangle{Max: dimensions.Size}
	cut := gtx.Dp(th.Cut.Large)
	FillCutTop(gtx.Ops, bounds, cut, ink)

	call.Add(gtx.Ops)
	return dimensions
}
