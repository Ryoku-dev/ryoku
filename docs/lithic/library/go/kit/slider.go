package kit

import (
	"image"
	"strconv"

	"gioui.org/f32"
	"gioui.org/io/key"
	"gioui.org/io/semantic"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/anim"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/theme"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/tokens"
)

// Slider selects one value in a bounded range.
type Slider struct {
	Click
	Value   float32
	Min     float32
	Max     float32
	Step    float32
	Label   string
	Preview State
	drag    widget.Float
	hover   anim.Tween
	focus   anim.Tween
	changed bool
}

// Changed reports and clears the value-change latch.
func (s *Slider) Changed() bool {
	changed := s.changed
	s.changed = false
	return changed
}

// Layout draws and operates the slider.
func (s *Slider) Layout(gtx layout.Context, th *theme.Theme) layout.Dimensions {
	minimum, maximum := s.Min, s.Max
	if maximum < minimum {
		minimum, maximum = maximum, minimum
	}
	if maximum == minimum {
		maximum = minimum + 1
	}
	value := min(max(s.Value, minimum), maximum)
	s.Value = value
	normalized := (value - minimum) / (maximum - minimum)
	s.drag.Value = normalized
	if s.drag.Update(gtx) {
		next := minimum + (maximum-minimum)*s.drag.Value
		if next != s.Value {
			s.Value = next
			s.changed = true
		}
	}
	if s.Clicked(gtx) {
		gtx.Execute(key.FocusCmd{Tag: &s.Clickable})
	}
	step := s.Step
	if step <= 0 {
		step = (maximum - minimum) * tokens.SizeSliderKeyboardStep
	}
	for {
		event, ok := gtx.Event(
			key.Filter{Focus: &s.Clickable, Name: key.NameLeftArrow},
			key.Filter{Focus: &s.Clickable, Name: key.NameRightArrow},
			key.Filter{Focus: &s.Clickable, Name: key.NameHome},
			key.Filter{Focus: &s.Clickable, Name: key.NameEnd},
		)
		if !ok {
			break
		}
		ke, ok := event.(key.Event)
		if !ok || ke.State != key.Press || !gtx.Source.Enabled() {
			continue
		}
		next := s.Value
		switch ke.Name {
		case key.NameLeftArrow:
			next -= step
		case key.NameRightArrow:
			next += step
		case key.NameHome:
			next = minimum
		case key.NameEnd:
			next = maximum
		}
		next = min(max(next, minimum), maximum)
		if next != s.Value {
			s.Value = next
			s.changed = true
			gtx.Execute(op.InvalidateCmd{})
		}
	}

	width := gtx.Constraints.Max.X
	if width <= 0 {
		width = gtx.Dp(unit.Dp(tokens.SizeSliderWidth))
	}
	height := gtx.Dp(unit.Dp(tokens.SizeSliderHeight))
	size := gtx.Constraints.Constrain(image.Pt(width, height))
	original := gtx
	var handleBounds image.Rectangle
	handleCut := 0
	dims := s.Click.Layout(exactContext(gtx, size), func(gtx layout.Context) layout.Dimensions {
		semantic.Editor.Add(gtx.Ops)
		if s.Label != "" {
			semantic.LabelOp(s.Label).Add(gtx.Ops)
		}
		semantic.DescriptionOp(strconv.FormatFloat(float64(s.Value), 'g', -1, 32)).Add(gtx.Ops)
		semantic.EnabledOp(gtx.Source.Enabled()).Add(gtx.Ops)

		hover := previewTransition(gtx, &s.hover, s.Preview, StateHover, s.Hovered(), th.Motion.HoverIn, th.Motion.HoverOut)
		pressed := s.Preview == StatePressed || s.Pressed() || s.drag.Dragging()
		trackHeight := max(1, gtx.Dp(unit.Dp(tokens.SizeSliderTrack)))
		trackY := (size.Y - trackHeight) / 2
		track := image.Rect(0, trackY, size.X, trackY+trackHeight)
		trackInk := palette.Mix(th.R.Field, th.R.Fg, tokens.AlphaHoverPlane*hover)
		FillCut(gtx.Ops, track, 0, disabledInk(gtx, trackInk))

		handleWidth := max(1, gtx.Dp(unit.Dp(tokens.SizeSliderHandleWidth)))
		handleHeight := max(1, gtx.Dp(unit.Dp(tokens.SizeSliderHandleHeight)))
		handleX := int(float32(max(0, size.X-handleWidth))*s.drag.Value + .5)
		doneEnd := min(size.X, handleX+handleWidth/2)
		if doneEnd > 0 {
			Blade(gtx.Ops, image.Rect(0, trackY, doneEnd, trackY+trackHeight), disabledInk(gtx, th.R.Fg2))
		}
		handleBounds = image.Rect(handleX, (size.Y-handleHeight)/2, handleX+handleWidth, (size.Y+handleHeight)/2)
		handleCut = min(handleBounds.Dx()/2, handleBounds.Dy()/4)
		points := sliderHandlePoints(handleBounds)
		handleInk := th.R.Sel
		if hover > 0 || pressed {
			handleInk = palette.Mix(handleInk, th.R.Fg, tokens.AlphaHoverFill*max(hover, 1))
		}
		paint.FillShape(gtx.Ops, disabledInk(gtx, handleInk), clip.Outline{Path: pathFromPoints(gtx.Ops, points[:])}.Op())
		return s.drag.Layout(exactContext(gtx, size), layout.Horizontal, unit.Dp(tokens.SizeSliderHandleWidth))
	})
	focus := previewTransition(original, &s.focus, s.Preview, StateFocus, original.Focused(&s.Clickable), th.Motion.FocusIn, th.Motion.FocusOut)
	if focus > 0 {
		gap := original.Dp(unit.Dp(tokens.SizeFocusRingGap))
		width := max(1, original.Dp(unit.Dp(tokens.SizeFocusRingWidth)))
		StrokeCutCorners(original.Ops, handleBounds.Inset(-gap), handleCut+gap, 0, handleCut+gap, 0, width, palette.WithAlpha(th.R.Fg2, focus))
	}
	return dims
}

func sliderHandlePoints(bounds image.Rectangle) [4]f32.Point {
	cut := min(bounds.Dx()/2, bounds.Dy()/4)
	return [4]f32.Point{
		f32.Pt(float32(bounds.Min.X+cut), float32(bounds.Min.Y)),
		f32.Pt(float32(bounds.Max.X), float32(bounds.Min.Y)),
		f32.Pt(float32(bounds.Max.X-cut), float32(bounds.Max.Y)),
		f32.Pt(float32(bounds.Min.X), float32(bounds.Max.Y)),
	}
}
