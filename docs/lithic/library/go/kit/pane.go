package kit

import (
	"image"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/theme"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/tokens"
)

// PaneKind selects the plane a pane lays and how its foot is drawn.
type PaneKind uint8

const (
	// PaneList is the list pane of a list and detail window. In Lithic it is
	// stone1 beside the work pane's stone2; a round design lays both panes in
	// its page plane.
	PaneList PaneKind = iota
	// PaneWork is the work pane of a list and detail window: the design's page
	// plane, running to the window's foot, so its foot stays square.
	PaneWork
	// PanePage is the Hub's page pane, standing clear of the window on every
	// side: the design's page plane with the large cut at the top corners and
	// the mid cut at the foot (Hub.qml:912-924).
	PanePage
)

// Pane is an application plane in the theme's design: the page plane and its
// stroke from th.Planes(), with cut top corners that a round design rounds.
// In Lithic the pane lies on the ground's own paper and its vein hairline
// draws the cut; Material and Adwaita lift it a tone with no line, and Fluent
// outlines it.
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
	planes := th.Planes()
	ink, line := planes.Pane, planes.PaneStroke
	top, foot := gtx.Dp(th.Cut.Large), 0
	switch pane.Kind {
	case PaneList:
		if !th.Design.Profile().Round {
			ink, line = th.R.Stone1, th.R.Vein
		}
	case PanePage:
		foot = gtx.Dp(th.Cut.Mid)
	}
	bounds := image.Rectangle{Max: dimensions.Size}
	FillCorners(gtx, th, bounds, top, top, foot, foot, ink)
	StrokeCorners(gtx, th, bounds, top, top, foot, foot, max(1, gtx.Dp(unit.Dp(tokens.SizeHairline))), line)

	call.Add(gtx.Ops)
	return dimensions
}
