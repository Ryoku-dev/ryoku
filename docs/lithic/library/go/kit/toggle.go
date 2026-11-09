package kit

import (
	"image"

	"gioui.org/io/key"
	"gioui.org/io/semantic"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/anim"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/theme"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/tokens"
)

// Toggle is a labelled binary switch.
type Toggle struct {
	Click
	Value   bool
	Label   string
	Preview State
	slide   anim.Tween
	tone    anim.Tween
	hover   anim.Tween
	focus   anim.Tween
	changed bool
}

// Changed reports and clears the value-change latch.
func (t *Toggle) Changed() bool {
	changed := t.changed
	t.changed = false
	return changed
}

// Layout draws and operates the switch.
func (t *Toggle) Layout(gtx layout.Context, th *theme.Theme) layout.Dimensions {
	if t.Clicked(gtx) {
		t.Value = !t.Value
		t.changed = true
	}
	for {
		event, ok := gtx.Event(key.Filter{Focus: &t.Clickable, Name: key.NameSpace})
		if !ok {
			break
		}
		if event, ok := event.(key.Event); ok && event.State == key.Press && gtx.Source.Enabled() {
			t.Value = !t.Value
			t.changed = true
			gtx.Execute(op.InvalidateCmd{})
		}
	}

	original := gtx
	gtx.Constraints.Min = image.Point{}
	trackWidth := gtx.Dp(unit.Dp(tokens.SizeToggleWidth))
	trackHeight := gtx.Dp(unit.Dp(tokens.SizeToggleHeight))
	trackCut := gtx.Dp(th.Cut.ToggleTrack)
	dims := t.Click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		semantic.Switch.Add(gtx.Ops)
		semantic.SelectedOp(t.Value).Add(gtx.Ops)
		semantic.EnabledOp(gtx.Source.Enabled()).Add(gtx.Ops)
		if t.Label != "" {
			semantic.LabelOp(t.Label).Add(gtx.Ops)
		}
		target := float32(0)
		if t.Value {
			target = 1
		}
		t.slide.Duration = th.Motion.Select
		t.slide.Ease = anim.EaseChisel
		t.tone.Duration = th.Motion.Field
		t.tone.Ease = anim.EaseChisel
		t.slide.Set(gtx, target)
		t.tone.Set(gtx, target)
		position := t.slide.Value(gtx)
		tone := t.tone.Value(gtx)
		hover := previewTransition(gtx, &t.hover, t.Preview, StateHover, t.Hovered(), th.Motion.HoverIn, th.Motion.HoverOut)
		if t.Preview == StatePressed || t.Pressed() {
			hover = 1
		}

		var children [3]layout.FlexChild
		count := 0
		children[count] = layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			size := constrainSize(gtx, image.Pt(trackWidth, trackHeight))
			track := image.Rectangle{Max: size}
			trackColor := palette.Mix(th.R.Stone4, th.R.Sel, tone)
			trackColor = palette.Mix(trackColor, th.R.Fg, tokens.AlphaHoverPlane*hover)
			FillCut(gtx.Ops, track, trackCut, disabledInk(gtx, trackColor))
			if tone < 1 {
				StrokeCut(gtx.Ops, track, trackCut, max(1, gtx.Dp(unit.Dp(tokens.SizeHairline))), disabledInk(gtx, palette.WithAlpha(th.R.Vein, 1-tone)))
			}

			knobSize := min(gtx.Dp(unit.Dp(tokens.SizeToggleKnob)), min(size.X, size.Y))
			knobCut := min(gtx.Dp(th.Cut.ToggleKnob), knobSize/2)
			left := min(gtx.Dp(unit.Dp(tokens.SizeToggleInset)), max(0, size.X-knobSize))
			right := max(left, size.X-gtx.Dp(unit.Dp(tokens.SizeToggleInset))-knobSize)
			x := int(lerp32(float32(left), float32(right), position) + .5)
			y := (size.Y - knobSize) / 2
			knobColor := palette.Mix(th.R.Fg2, th.R.OnSel, position)
			offset := op.Offset(image.Pt(x, y)).Push(gtx.Ops)
			FillCut(gtx.Ops, image.Rect(0, 0, knobSize, knobSize), knobCut, disabledInk(gtx, knobColor))
			offset.Pop()
			return layout.Dimensions{Size: size}
		})
		count++
		if t.Label != "" {
			children[count] = layout.Rigid(layout.Spacer{Width: unit.Dp(tokens.SizeToggleLabelGap)}.Layout)
			count++
			children[count] = layout.Rigid(rawText(th, th.T.Label, disabledInk(gtx, th.R.Fg), t.Label))
			count++
		}
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx, children[:count]...)
	})
	focus := previewTransition(original, &t.focus, t.Preview, StateFocus, original.Focused(&t.Clickable), th.Motion.FocusIn, th.Motion.FocusOut)
	FocusRingAlpha(original, th, image.Rect(0, 0, min(dims.Size.X, trackWidth), trackHeight), trackCut, focus)
	return dims
}
