package kit

import (
	"image"

	"gioui.org/io/semantic"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	material "golang.org/x/exp/shiny/materialdesign/icons"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/anim"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/theme"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/tokens"
)

var newAddIcon = Icon(material.ContentAdd)

// NewPlaque is the rail's sole gold action.
type NewPlaque struct {
	Click
	Preview State

	press, hover, focus anim.Tween
}

// Layout draws expanded or compact New state, changing add to close while open.
func (p *NewPlaque) Layout(gtx layout.Context, th *theme.Theme, compact, open bool) layout.Dimensions {
	width, height := gtx.Dp(tokens.SizeNewPlaqueWidth), gtx.Dp(tokens.SizeNewPlaqueHeight)
	iconSize, cut := gtx.Dp(tokens.SizeNewPlaqueIcon), gtx.Dp(th.Cut.Mid)
	if compact {
		width, height = gtx.Dp(tokens.SizeNewPlaqueWidthCompact), gtx.Dp(tokens.SizeNewPlaqueHeightCompact)
		iconSize, cut = gtx.Dp(tokens.SizeNewPlaqueIconCompact), gtx.Dp(th.Cut.Small)
	}
	finalCut := cut
	dimensions := p.Click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		semantic.Button.Add(gtx.Ops)
		semantic.LabelOp("New").Add(gtx.Ops)
		semantic.EnabledOp(gtx.Source.Enabled()).Add(gtx.Ops)
		pressed := p.Pressed() || p.Preview == StatePressed
		p.press.Duration = th.Motion.Press
		p.press.Ease = anim.EaseChisel
		if pressed && gtx.Source.Enabled() {
			p.press.Set(gtx, 1)
		} else {
			p.press.Set(gtx, 0)
		}
		press := p.press.Value(gtx)
		finalCut = cut + int(float32(gtx.Dp(tokens.SizeControlPressCutGrow))*press+.5)
		sink := int(float32(gtx.Dp(tokens.SizeControlPressSink))*press + .5)
		bounds := image.Rectangle{Max: gtx.Constraints.Constrain(image.Pt(width, height))}
		hovered := p.Hovered() || p.Preview == StateHover
		hover := hoverValue(gtx, th, &p.hover, hovered)
		fill := palette.Mix(th.R.Gold, th.R.OnGold, tokens.AlphaHoverFill*hover)
		FillCut(gtx.Ops, bounds, finalCut, fill)
		icon := newAddIcon
		if open {
			icon = iconClose
		}
		if compact {
			offset := op.Offset(image.Pt((bounds.Dx()-iconSize)/2, (bounds.Dy()-iconSize)/2+sink)).Push(gtx.Ops)
			icon.Layout(exactContext(gtx, image.Pt(iconSize, iconSize)), th.R.OnGold)
			offset.Pop()
		} else {
			contentContext := exactContext(gtx, bounds.Size())
			offset := op.Offset(image.Pt(0, sink)).Push(gtx.Ops)
			layout.Center.Layout(contentContext, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return icon.Layout(exactContext(gtx, image.Pt(iconSize, iconSize)), th.R.OnGold)
					}),
					layout.Rigid(layout.Spacer{Height: unit.Dp(tokens.SizeNewPlaqueLabelGap)}.Layout),
					layout.Rigid(rawText(th, th.T.Label, th.R.OnGold, "New")),
				)
			})
			offset.Pop()
		}
		return layout.Dimensions{Size: bounds.Size()}
	})
	focused := gtx.Focused(&p.Click.Clickable) || p.Preview == StateFocus
	focus := focusValue(gtx, th, &p.focus, focused)
	FocusRingAlpha(gtx, th, image.Rectangle{Max: dimensions.Size}, finalCut, focus)
	return dimensions
}
