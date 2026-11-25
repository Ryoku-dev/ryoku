package kit

import (
	"image"
	"image/color"
	"time"

	"gioui.org/layout"
	"gioui.org/unit"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/anim"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/theme"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/tokens"
)

func transitionValue(gtx layout.Context, tween *anim.Tween, on bool, in, out time.Duration) float32 {
	tween.Ease = anim.EaseChisel
	if on {
		tween.Duration = in
		tween.Set(gtx, 1)
	} else {
		tween.Duration = out
		tween.Set(gtx, 0)
	}
	return tween.Value(gtx)
}

func hoverValue(gtx layout.Context, th *theme.Theme, tween *anim.Tween, hovered bool) float32 {
	return transitionValue(gtx, tween, hovered && gtx.Source.Enabled(), th.Motion.HoverIn, th.Motion.HoverOut)
}

func focusValue(gtx layout.Context, th *theme.Theme, tween *anim.Tween, keyboardFocused bool) float32 {
	return transitionValue(gtx, tween, keyboardFocused && gtx.Source.Enabled(), th.Motion.FocusIn, th.Motion.FocusOut)
}

// disabledInk fades ink for a disabled control. Gio blends in linear light and
// the Hub (Qt) in sRGB, so the fade is solved with palette.SRGBAlpha over the
// ground to land where Qt's blend does. An ink that is already translucent
// keeps its share: the two alphas multiply. Pass plain inks, never one already
// solved; faintInk carries a faint ink and the disabled fade together.
func disabledInk(gtx layout.Context, th *theme.Theme, ink color.NRGBA) color.NRGBA {
	if !gtx.Source.Enabled() {
		return palette.SRGBAlpha(ink, th.R.Ground, tokens.AlphaDisabled)
	}
	return ink
}

// faintInk is ink at the faint share alpha (a hover, press or hatch wash) over
// under, the plane it lies on, solved with palette.SRGBAlpha so Gio's linear
// blend lands on Qt's sRGB one. A disabled control fades it further.
func faintInk(gtx layout.Context, th *theme.Theme, ink, under color.NRGBA, alpha float32) color.NRGBA {
	if !gtx.Source.Enabled() {
		alpha *= tokens.AlphaDisabled
	}
	return palette.SRGBAlpha(ink, under, alpha)
}

// FocusRingAlpha draws the shared focus ring at alpha around a control of
// bounds and cut. As FocusRing.qml, the ring stands a gap outside the control,
// its own box outset by gap and ring width, and wears the control's cut grown
// by focusCutGrow, so a round design rounds it with the control.
func FocusRingAlpha(gtx layout.Context, th *theme.Theme, bounds image.Rectangle, cut int, alpha float32) {
	if alpha <= 0 {
		return
	}
	ring, width := focusRingBox(gtx, bounds)
	cut += gtx.Dp(unit.Dp(tokens.SizeFocusCutGrow))
	Stroke(gtx, th, ring, cut, width, palette.WithAlpha(th.R.Fg2, min(alpha, 1)))
}

// focusRingCorners draws the focus ring of a control whose corners differ;
// only a cut corner grows, a square one stays square.
func focusRingCorners(gtx layout.Context, th *theme.Theme, bounds image.Rectangle, northwest, northeast, southeast, southwest int, alpha float32) {
	if alpha <= 0 {
		return
	}
	ring, width := focusRingBox(gtx, bounds)
	grow := gtx.Dp(unit.Dp(tokens.SizeFocusCutGrow))
	grown := func(cut int) int {
		if cut <= 0 {
			return 0
		}
		return cut + grow
	}
	StrokeCorners(gtx, th, ring, grown(northwest), grown(northeast), grown(southeast), grown(southwest), width, palette.WithAlpha(th.R.Fg2, min(alpha, 1)))
}

// focusRingBox returns the ring's box, bounds outset by the focus gap and the
// ring width (FocusRing.qml outset), and the ring width.
func focusRingBox(gtx layout.Context, bounds image.Rectangle) (image.Rectangle, int) {
	width := max(1, gtx.Dp(unit.Dp(tokens.SizeFocusRingWidth)))
	return bounds.Inset(-(gtx.Dp(unit.Dp(tokens.SizeFocusRingGap)) + width)), width
}

// FocusRing draws the shared focus ring.
func FocusRing(gtx layout.Context, th *theme.Theme, bounds image.Rectangle, cut int) {
	FocusRingAlpha(gtx, th, bounds, cut, 1)
}
