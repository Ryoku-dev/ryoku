package kit

import (
	"image"

	"gioui.org/layout"
	"gioui.org/text"
	"gioui.org/widget"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/theme"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/tokens"
)

// Empty presents an empty state and its next actions.
type Empty struct {
	Icon    *widget.Icon
	Title   string
	Body    string
	Actions []*Plaque
}

// Layout centres the empty state in the available plane.
func (empty Empty) Layout(gtx layout.Context, th *theme.Theme) layout.Dimensions {
	return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		children := []layout.FlexChild{
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				diameter := gtx.Dp(tokens.SizeEmptyFrame)
				size := gtx.Constraints.Constrain(image.Pt(diameter, diameter))
				StrokeCut(gtx.Ops, image.Rectangle{Max: size}, gtx.Dp(th.Cut.Large), max(1, gtx.Dp(tokens.SizeHairline)), th.R.Vein)
				if empty.Icon != nil {
					frameContext := gtx
					frameContext.Constraints = layout.Exact(size)
					layout.Center.Layout(frameContext, func(gtx layout.Context) layout.Dimensions {
						iconSize := min(gtx.Dp(tokens.SizeEmptyIcon), min(size.X, size.Y))
						gtx.Constraints = layout.Exact(image.Pt(iconSize, iconSize))
						return empty.Icon.Layout(gtx, th.R.Fg3)
					})
				}
				return layout.Dimensions{Size: size}
			}),
			layout.Rigid(layout.Spacer{Height: th.Sp.S4}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return Label{Style: th.T.Title, Color: th.R.Fg, MaxLines: 2, Align: text.Middle}.Layout(gtx, th, empty.Title)
			}),
			layout.Rigid(layout.Spacer{Height: th.Sp.S2}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints.Max.X = min(gtx.Constraints.Max.X, gtx.Dp(tokens.SizeEmptyBodyMax))
				return Label{Style: th.T.Body, Color: th.R.Fg2, MaxLines: 0, Align: text.Middle}.Layout(gtx, th, empty.Body)
			}),
		}
		valid := make([]*Plaque, 0, len(empty.Actions))
		for _, action := range empty.Actions {
			if action != nil {
				valid = append(valid, action)
			}
		}
		if len(valid) > 0 {
			children = append(children,
				layout.Rigid(layout.Spacer{Height: th.Sp.S4}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					items := make([]layout.FlexChild, 0, len(valid))
					for _, action := range valid {
						action := action
						items = append(items, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return action.Layout(gtx, th)
						}))
					}
					return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle, Gap: gtx.Dp(th.Sp.S2)}.Layout(gtx, items...)
				}),
			)
		}
		return layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle}.Layout(gtx, children...)
	})
}
