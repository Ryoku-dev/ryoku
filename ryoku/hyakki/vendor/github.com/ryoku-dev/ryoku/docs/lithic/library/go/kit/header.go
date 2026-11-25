package kit

import (
	"image"

	"gioui.org/layout"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/theme"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/tokens"
)

// HeaderContent describes a selected resource header.
type HeaderContent struct {
	Tone     palette.Tone
	Mark     layout.Widget
	Name     string
	Progress float64
	Proof    []Span
	Note     string
	Marks    []Mark
	Actions  []*Plaque
	Below    layout.Widget
}

// Header owns the animated mineral field behind a selected resource.
type Header struct {
	field MineralField
}

// Layout draws the full selected-resource header.
func (header *Header) Layout(gtx layout.Context, th *theme.Theme, content HeaderContent) layout.Dimensions {
	return header.field.Layout(gtx, th, content.Tone, func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{
			Top: tokens.SizeHeaderTop, Right: tokens.SizeHeaderSide,
			Bottom: tokens.SizeHeaderBottom, Left: tokens.SizeHeaderSide,
		}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			children := make([]layout.FlexChild, 0, 6)
			children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				compact := gtx.Constraints.Max.X < gtx.Dp(tokens.SizeHeaderNameCompactAt)
				name := func(gtx layout.Context) layout.Dimensions {
					if content.Progress >= 0 {
						return (BecomingName{Name: content.Name, Fraction: content.Progress, Tone: content.Tone, Compact: compact}).Layout(gtx, th)
					}
					style := th.T.Name
					if compact {
						style = th.T.Sentence
					}
					return Label{Style: style, Color: th.R.Fg, MaxLines: 1}.Layout(gtx, th, content.Name)
				}
				row := make([]layout.FlexChild, 0, 3)
				if content.Mark != nil {
					row = append(row,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							size := gtx.Dp(tokens.SizeHeaderMark)
							gtx.Constraints = layout.Exact(image.Pt(size, size))
							return layout.Center.Layout(gtx, content.Mark)
						}),
						layout.Rigid(layout.Spacer{Width: tokens.SizeHeaderMarkGap}.Layout),
					)
				}
				row = append(row, layout.Flexed(1, name))
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx, row...)
			}))
			if len(content.Proof) > 0 {
				children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{Top: tokens.SizeHeaderProofGap}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return Sentence(gtx, th, th.T.Proof, th.R.Fg2, content.Proof)
					})
				}))
			}
			if content.Note != "" {
				children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{Top: tokens.SizeHeaderNoteGap}.Layout(gtx, Text(th, th.T.Caption, th.R.Fg3, content.Note))
				}))
			}
			if len(content.Marks) > 0 {
				children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{Top: tokens.SizeHeaderMarksGap}.Layout(gtx, Marks(th, content.Marks))
				}))
			}
			validActions := make([]*Plaque, 0, len(content.Actions))
			for _, action := range content.Actions {
				if action != nil {
					validActions = append(validActions, action)
				}
			}
			if len(validActions) > 0 {
				children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{Top: tokens.SizeHeaderActionsGap}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						items := make([]layout.FlexChild, 0, len(validActions))
						for _, action := range validActions {
							action := action
							items = append(items, layout.Rigid(func(gtx layout.Context) layout.Dimensions { return action.Layout(gtx, th) }))
						}
						return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle, Gap: gtx.Dp(th.Sp.S2)}.Layout(gtx, items...)
					})
				}))
			}
			if content.Below != nil {
				children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{Top: tokens.SizeHeaderContentGap}.Layout(gtx, content.Below)
				}))
			}
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
		})
	})
}
