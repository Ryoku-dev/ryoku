package kit

import (
	"image"
	"image/color"

	"gioui.org/io/semantic"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/anim"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/theme"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/tokens"
)

// IconPlaque is a square icon-only plaque with an accessible description.
type IconPlaque struct {
	Click
	Kind        PlaqueKind
	Icon        *widget.Icon
	Description string
	Small       bool
	Busy        bool
	Notch       color.NRGBA
	Preview     State
	press       anim.Tween
	hover       anim.Tween
	focus       anim.Tween
}

// Clicked reports one activation unless the plaque is busy.
func (p *IconPlaque) Clicked(gtx layout.Context) bool {
	if !p.Busy {
		return p.Click.Clicked(gtx)
	}
	for p.Click.Clicked(gtx) {
	}
	return false
}

// Layout draws the icon plaque.
func (p *IconPlaque) Layout(gtx layout.Context, th *theme.Theme) layout.Dimensions {
	return p.layoutWith(gtx, th, &p.Click)
}

// layoutWith draws the plaque's look while click owns its interaction, so a
// component can expose that click as its own field.
func (p *IconPlaque) layoutWith(gtx layout.Context, th *theme.Theme, click *Click) layout.Dimensions {
	size := unit.Dp(tokens.SizeIconPlaque)
	iconSize := unit.Dp(tokens.SizeIconPlaqueIcon)
	if p.Small {
		size = unit.Dp(tokens.SizeIconPlaqueSmall)
		iconSize = unit.Dp(tokens.SizeIconPlaqueIconSmall)
	}
	pixels := gtx.Dp(size)
	original := gtx
	interaction := gtx
	if p.Busy {
		interaction = interaction.Disabled()
	}
	cut := gtx.Dp(th.Cut.Small)
	dims := click.Layout(interaction, func(gtx layout.Context) layout.Dimensions {
		gtx.Source = original.Source
		semantic.Button.Add(gtx.Ops)
		semantic.DescriptionOp(p.Description).Add(gtx.Ops)
		semantic.EnabledOp(original.Source.Enabled() && !p.Busy).Add(gtx.Ops)
		pressed := previewTransition(original, &p.press, p.Preview, StatePressed, click.Pressed() && !p.Busy, th.Motion.Press, th.Motion.Press)
		cut = gtx.Dp(th.Cut.Small) + int(float32(gtx.Dp(unit.Dp(tokens.SizeControlPressCutGrow)))*pressed+.5)
		sink := int(float32(gtx.Dp(unit.Dp(tokens.SizeControlPressSink)))*pressed + .5)
		hover := previewTransition(original, &p.hover, p.Preview, StateHover, click.Hovered() && !p.Busy, th.Motion.HoverIn, th.Motion.HoverOut)
		bounds := image.Rectangle{Max: constrainSize(gtx, image.Pt(pixels, pixels))}
		ink := drawPlaqueSurface(original, th, bounds, cut, p.Kind, hover)
		is := gtx.Dp(iconSize)
		offset := op.Offset(image.Pt((bounds.Dx()-is)/2, (bounds.Dy()-is)/2+sink)).Push(gtx.Ops)
		if p.Busy {
			Flake(exactContext(gtx, image.Pt(is, is)), th, palette.Becoming, float64(tokens.AlphaWorkingBase), iconSize)
		} else if p.Icon != nil {
			p.Icon.Layout(exactContext(gtx, image.Pt(is, is)), ink)
		}
		offset.Pop()
		if p.Notch.A > 0 {
			Notch(gtx.Ops, bounds.Min, gtx.Dp(th.Cut.Notch), disabledInk(original, p.Notch))
		}
		return layout.Dimensions{Size: bounds.Size()}
	})
	focus := previewTransition(original, &p.focus, p.Preview, StateFocus, original.Focused(&click.Clickable), th.Motion.FocusIn, th.Motion.FocusOut)
	FocusRingAlpha(original, th, image.Rectangle{Max: dims.Size}, cut, focus)
	return dims
}
