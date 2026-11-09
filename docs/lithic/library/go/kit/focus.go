package kit

import (
	"image"
	"image/color"
	"time"

	"gioui.org/layout"

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

func disabledInk(gtx layout.Context, ink color.NRGBA) color.NRGBA {
	if !gtx.Source.Enabled() {
		return palette.WithAlpha(ink, tokens.AlphaDisabled)
	}
	return ink
}

// FocusRingAlpha draws the shared focus ring at alpha.
func FocusRingAlpha(gtx layout.Context, th *theme.Theme, bounds image.Rectangle, cut int, alpha float32) {
	if alpha <= 0 {
		return
	}
	gap := gtx.Dp(2)
	width := max(1, gtx.Dp(2))
	StrokeCut(gtx.Ops, bounds.Inset(-gap), cut+gap, width, palette.WithAlpha(th.R.Fg2, min(alpha, 1)))
}

// FocusRing draws the shared focus ring.
func FocusRing(gtx layout.Context, th *theme.Theme, bounds image.Rectangle, cut int) {
	FocusRingAlpha(gtx, th, bounds, cut, 1)
}
