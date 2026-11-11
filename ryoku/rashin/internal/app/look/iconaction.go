package look

import (
	"image"
	"image/color"

	"gioui.org/io/semantic"
	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/anim"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/tokens"
)

// no surface so a list under the pointer doesn't fill with outlined plaques, frame level actions keep kit.IconPlaque
type IconAction struct {
	kit.Click
	Icon        *widget.Icon
	Description string
	// default to 28 and 18
	Box, Glyph unit.Dp
	Disabled   bool
	Preview    kit.State

	hover, press, focus anim.Tween
}

func (action *IconAction) Clicked(gtx layout.Context) bool {
	if !action.Disabled {
		return action.Click.Clicked(gtx)
	}
	for action.Click.Clicked(gtx) {
	}
	return false
}

func (action *IconAction) Layout(gtx layout.Context, th *Theme, ink color.NRGBA) layout.Dimensions {
	box, glyph := action.Box, action.Glyph
	if box == 0 {
		box = 28
	}
	if glyph == 0 {
		glyph = 18
	}
	size := gtx.Dp(box)
	original := gtx
	if action.Disabled {
		gtx = gtx.Disabled()
		ink = palette.WithAlpha(ink, tokens.AlphaDisabled)
	}
	cut := gtx.Dp(th.Cut.Small)
	dimensions := action.Click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		semantic.Button.Add(gtx.Ops)
		semantic.DescriptionOp(action.Description).Add(gtx.Ops)
		semantic.EnabledOp(!action.Disabled).Add(gtx.Ops)
		hovered := action.Hovered() || action.Preview == kit.StateHover || action.Preview == kit.StatePressed
		pressed := action.Pressed() || action.Preview == kit.StatePressed
		hover := transition(original, &action.hover, hovered && !action.Disabled, th.Motion.HoverIn, th.Motion.HoverOut)
		press := transition(original, &action.press, pressed && !action.Disabled, th.Motion.Press, th.Motion.Press)
		bounds := image.Rectangle{Max: image.Pt(size, size)}
		if fill := tokens.AlphaHoverFill*hover + .08*press; fill > 0 {
			kit.FillCut(gtx.Ops, bounds, cut, palette.WithAlpha(th.R.Fg, fill))
		}
		pixels := gtx.Dp(glyph)
		sink := int(float32(gtx.Dp(1))*press + .5)
		PlaceIcon(gtx, action.Icon, image.Pt((size-pixels)/2, (size-pixels)/2+sink), glyph, palette.Mix(ink, th.R.Fg, hover))
		return layout.Dimensions{Size: bounds.Max}
	})
	focused := original.Focused(&action.Click.Clickable) || action.Preview == kit.StateFocus
	focus := transition(original, &action.focus, focused, th.Motion.FocusIn, th.Motion.FocusOut)
	kit.FocusRingAlpha(original, th.Theme, image.Rectangle{Max: dimensions.Size}, cut, focus)
	return dimensions
}
