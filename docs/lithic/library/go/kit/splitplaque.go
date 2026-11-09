package kit

import (
	"image"

	"gioui.org/io/semantic"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget"
	material "golang.org/x/exp/shiny/materialdesign/icons"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/anim"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/theme"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/tokens"
)

var (
	iconExpandMore = Icon(material.NavigationExpandMore)
	iconExpandLess = Icon(material.NavigationExpandLess)
)

// SplitPlaque joins a primary action and an independent menu action.
type SplitPlaque struct {
	Main, More           Click
	Kind                 PlaqueKind
	Icon                 *widget.Icon
	Label                string
	Open                 bool
	Preview              State
	mainPress, morePress anim.Tween
	mainHover, moreHover anim.Tween
	mainFocus, moreFocus anim.Tween
}

// MainClicked reports a primary-action activation.
func (s *SplitPlaque) MainClicked(gtx layout.Context) bool { return s.Main.Clicked(gtx) }

// MoreClicked reports a menu-action activation.
func (s *SplitPlaque) MoreClicked(gtx layout.Context) bool { return s.More.Clicked(gtx) }

// Layout draws both independently interactive plaque segments.
func (s *SplitPlaque) Layout(gtx layout.Context, th *theme.Theme) layout.Dimensions {
	gtx.Constraints.Min = image.Point{}
	height := gtx.Dp(unit.Dp(tokens.SizePlaqueHeight))
	gap := gtx.Dp(unit.Dp(tokens.SizeSplitPlaqueGap))
	moreWidth := gtx.Dp(unit.Dp(tokens.SizeSplitPlaqueMoreWidth))
	baseCut := gtx.Dp(th.Cut.Mid)

	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle, Gap: gap}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			cut := baseCut
			dims := s.Main.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				semantic.Button.Add(gtx.Ops)
				semantic.LabelOp(s.Label).Add(gtx.Ops)
				semantic.EnabledOp(gtx.Source.Enabled()).Add(gtx.Ops)
				pressed := previewTransition(gtx, &s.mainPress, s.Preview, StatePressed, s.Main.Pressed(), th.Motion.Press, th.Motion.Press)
				cut = baseCut + int(float32(gtx.Dp(unit.Dp(tokens.SizeControlPressCutGrow)))*pressed+.5)
				sink := int(float32(gtx.Dp(unit.Dp(tokens.SizeControlPressSink)))*pressed + .5)
				hover := previewTransition(gtx, &s.mainHover, s.Preview, StateHover, s.Main.Hovered(), th.Motion.HoverIn, th.Motion.HoverOut)
				colors := interactivePlaqueColors(gtx, th, s.Kind, hover)

				contentGTX := gtx
				contentGTX.Constraints.Min = image.Point{}
				contentGTX.Constraints.Max.Y = height
				record := op.Record(gtx.Ops)
				content := layout.Inset{Left: unit.Dp(tokens.SizePlaqueInset), Right: unit.Dp(tokens.SizePlaqueInset)}.Layout(contentGTX, func(gtx layout.Context) layout.Dimensions {
					var children [3]layout.FlexChild
					count := 0
					if s.Icon != nil {
						children[count] = layout.Rigid(iconLayout(s.Icon, unit.Dp(tokens.SizePlaqueIcon), colors.ink))
						count++
						children[count] = layout.Rigid(layout.Spacer{Width: unit.Dp(tokens.SizePlaqueGap)}.Layout)
						count++
					}
					children[count] = layout.Rigid(rawText(th, th.T.Label, colors.ink, s.Label))
					count++
					return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx, children[:count]...)
				})
				call := record.Stop()
				size := constrainSize(gtx, image.Pt(content.Size.X, height))
				bounds := image.Rectangle{Max: size}
				FillCutCorners(gtx.Ops, bounds, cut, 0, 0, cut, colors.fill)
				if colors.stroke {
					StrokeCutCorners(gtx.Ops, bounds, cut, 0, 0, cut, max(1, gtx.Dp(unit.Dp(tokens.SizeHairline))), colors.outline)
				}
				offset := op.Offset(image.Pt((size.X-content.Size.X)/2, (height-content.Size.Y)/2+sink)).Push(gtx.Ops)
				call.Add(gtx.Ops)
				offset.Pop()
				return layout.Dimensions{Size: size}
			})
			focus := previewTransition(gtx, &s.mainFocus, s.Preview, StateFocus, gtx.Focused(&s.Main.Clickable), th.Motion.FocusIn, th.Motion.FocusOut)
			if focus > 0 {
				pad := gtx.Dp(unit.Dp(tokens.SizeFocusRingGap))
				StrokeCutCorners(gtx.Ops, image.Rectangle{Max: dims.Size}.Inset(-pad), cut+pad, 0, 0, cut+pad, max(1, gtx.Dp(unit.Dp(tokens.SizeFocusRingWidth))), palette.WithAlpha(th.R.Fg2, focus))
			}
			return dims
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min.X, gtx.Constraints.Max.X = moreWidth, moreWidth
			cut := baseCut
			dims := s.More.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				semantic.Button.Add(gtx.Ops)
				semantic.DescriptionOp("More actions").Add(gtx.Ops)
				semantic.EnabledOp(gtx.Source.Enabled()).Add(gtx.Ops)
				pressed := previewTransition(gtx, &s.morePress, s.Preview, StatePressed, s.More.Pressed(), th.Motion.Press, th.Motion.Press)
				cut = baseCut + int(float32(gtx.Dp(unit.Dp(tokens.SizeControlPressCutGrow)))*pressed+.5)
				sink := int(float32(gtx.Dp(unit.Dp(tokens.SizeControlPressSink)))*pressed + .5)
				hover := previewTransition(gtx, &s.moreHover, s.Preview, StateHover, s.More.Hovered(), th.Motion.HoverIn, th.Motion.HoverOut)
				colors := interactivePlaqueColors(gtx, th, s.Kind, hover)
				bounds := image.Rect(0, 0, moreWidth, height)
				FillCutCorners(gtx.Ops, bounds, 0, cut, cut, 0, colors.fill)
				if colors.stroke {
					StrokeCutCorners(gtx.Ops, bounds, 0, cut, cut, 0, max(1, gtx.Dp(unit.Dp(tokens.SizeHairline))), colors.outline)
				}
				arrow := iconExpandMore
				if s.Open {
					arrow = iconExpandLess
				}
				iconSize := gtx.Dp(unit.Dp(tokens.SizeIconPlaqueIcon))
				offset := op.Offset(image.Pt((moreWidth-iconSize)/2, (height-iconSize)/2+sink)).Push(gtx.Ops)
				arrow.Layout(exactContext(gtx, image.Pt(iconSize, iconSize)), colors.ink)
				offset.Pop()
				return layout.Dimensions{Size: image.Pt(moreWidth, height)}
			})
			focus := previewTransition(gtx, &s.moreFocus, s.Preview, StateFocus, gtx.Focused(&s.More.Clickable), th.Motion.FocusIn, th.Motion.FocusOut)
			if focus > 0 {
				pad := gtx.Dp(unit.Dp(tokens.SizeFocusRingGap))
				StrokeCutCorners(gtx.Ops, image.Rectangle{Max: dims.Size}.Inset(-pad), 0, cut+pad, cut+pad, 0, max(1, gtx.Dp(unit.Dp(tokens.SizeFocusRingWidth))), palette.WithAlpha(th.R.Fg2, focus))
			}
			return dims
		}),
	)
}
