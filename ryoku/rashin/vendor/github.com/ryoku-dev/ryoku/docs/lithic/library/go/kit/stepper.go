package kit

import (
	"image"
	"math"
	"strconv"
	"strings"

	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/anim"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/symbols"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/theme"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/tokens"
)

var (
	iconStepRemove = Icon(symbols.Remove)
	iconStepAdd    = Icon(symbols.Add)
)

// Stepper changes an integer value between bounds. An Editable stepper also
// takes a typed value, for ranges too wide to step through such as a port:
// Return, Enter or leaving the field commits it clamped to the bounds and
// snapped to the step, Escape drops an unsaved edit and otherwise passes on,
// and the arrow keys still step.
type Stepper struct {
	Value    int
	Min      int
	Max      int
	Step     int
	Unit     string
	Editable bool
	Preview  State
	minus    IconPlaque
	plus     IconPlaque
	changed  bool
	editor   widget.Editor
	shown    string
	editing  bool
	focus    anim.Tween
}

// Changed reports and clears the value-change latch.
func (s *Stepper) Changed() bool {
	changed := s.changed
	s.changed = false
	return changed
}

// Focus moves keyboard focus into an editable stepper's field.
func (s *Stepper) Focus(gtx layout.Context) {
	gtx.Execute(key.FocusCmd{Tag: &s.editor})
}

func (s *Stepper) step() int {
	if s.Step <= 0 {
		return 1
	}
	return s.Step
}

func (s *Stepper) set(value int) {
	value = min(max(value, s.Min), s.Max)
	if value != s.Value {
		s.Value = value
		s.changed = true
	}
}

// stepperTyped reads a typed value, snapped to the step from min and clamped.
func stepperTyped(typed string, low, high, step int) (int, bool) {
	value, err := strconv.Atoi(strings.TrimSpace(typed))
	if err != nil {
		return 0, false
	}
	if step > 0 {
		value = low + int(math.Round(float64(value-low)/float64(step)))*step
	}
	return min(max(value, low), high), true
}

// show puts the value in the field, dropping anything typed.
func (s *Stepper) show() {
	s.shown = strconv.Itoa(s.Value)
	s.editor.SetText(s.shown)
	s.editor.SetCaret(len(s.shown), 0)
}

func (s *Stepper) commit() {
	if value, ok := stepperTyped(s.editor.Text(), s.Min, s.Max, s.step()); ok {
		s.set(value)
	}
	s.show()
}

// update runs an editable stepper's keys and edits for this frame.
func (s *Stepper) update(gtx layout.Context) {
	s.editor.SingleLine = true
	s.editor.Submit = true
	s.editor.Filter = "-0123456789"
	s.editor.Alignment = text.Middle
	dirty := s.editor.Text() != s.shown
	if !dirty && s.shown != strconv.Itoa(s.Value) {
		s.show()
	}
	filters := []event.Filter{
		key.Filter{Focus: &s.editor, Name: key.NameUpArrow},
		key.Filter{Focus: &s.editor, Name: key.NameDownArrow},
	}
	// with nothing typed, Escape is left to whatever holds it, such as a dialog
	if dirty {
		filters = append(filters, key.Filter{Focus: &s.editor, Name: key.NameEscape})
	}
	for {
		event, ok := gtx.Event(filters...)
		if !ok {
			break
		}
		press, ok := event.(key.Event)
		if !ok || press.State != key.Press {
			continue
		}
		switch press.Name {
		case key.NameUpArrow:
			s.set(s.Value + s.step())
			s.show()
		case key.NameDownArrow:
			s.set(s.Value - s.step())
			s.show()
		case key.NameEscape:
			s.show()
		}
		gtx.Execute(op.InvalidateCmd{})
	}
	for {
		event, ok := s.editor.Update(gtx)
		if !ok {
			break
		}
		if _, ok := event.(widget.SubmitEvent); ok {
			s.commit()
		}
	}
	focused := gtx.Focused(&s.editor)
	if focused && !s.editing {
		s.editor.SetCaret(len(s.editor.Text()), 0)
	}
	if !focused && s.editing {
		s.commit()
	}
	s.editing = focused
}

// Layout draws and operates the stepper.
func (s *Stepper) Layout(gtx layout.Context, th *theme.Theme) layout.Dimensions {
	gtx.Constraints.Min = image.Point{}
	if s.Max < s.Min {
		s.Max = s.Min
	}
	s.Value = min(max(s.Value, s.Min), s.Max)
	step := s.step()
	if s.minus.Clicked(gtx) && s.Value > s.Min {
		s.set(s.Value - step)
	}
	if s.plus.Clicked(gtx) && s.Value < s.Max {
		s.set(s.Value + step)
	}
	if s.Editable {
		s.update(gtx)
	}

	minusGTX, plusGTX := gtx, gtx
	if s.Value <= s.Min {
		minusGTX = minusGTX.Disabled()
	}
	if s.Value >= s.Max {
		plusGTX = plusGTX.Disabled()
	}
	s.minus.Small = true
	s.minus.Kind = PlaqueQuiet
	s.minus.Icon = iconStepRemove
	s.minus.Description = "Decrease"
	s.minus.Preview = StateRest
	s.plus.Small = true
	s.plus.Kind = PlaqueQuiet
	s.plus.Icon = iconStepAdd
	s.plus.Description = "Increase"
	s.plus.Preview = StateRest
	if !s.Editable {
		if s.Value < s.Max {
			s.plus.Preview = s.Preview
		} else {
			s.minus.Preview = s.Preview
		}
	}

	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle, Gap: gtx.Dp(unit.Dp(tokens.SizeStepperGap))}.Layout(gtx,
		layout.Rigid(func(layout.Context) layout.Dimensions {
			return s.minus.Layout(minusGTX, th)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return s.layoutField(gtx, th)
		}),
		layout.Rigid(func(layout.Context) layout.Dimensions {
			return s.plus.Layout(plusGTX, th)
		}),
	)
}

// layoutField draws the value, or the editable field sized for the widest
// bound, with the unit after it on the field plane.
func (s *Stepper) layoutField(gtx layout.Context, th *theme.Theme) layout.Dimensions {
	height := gtx.Dp(unit.Dp(tokens.SizeIconPlaqueSmall))
	var children [3]layout.FlexChild
	count := 0
	if s.Editable {
		// the widest bound plus the caret's hairline
		width := max(stepperTextWidth(gtx, th, strconv.Itoa(s.Min)), stepperTextWidth(gtx, th, strconv.Itoa(s.Max))) + max(1, gtx.Dp(tokens.SizeHairline))
		lineHeight := gtx.Sp(th.T.Label.Line)
		children[count] = layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layoutTextEditor(exactContext(gtx, image.Pt(width, lineHeight)), th, &s.editor, th.T.Label, "")
		})
	} else {
		children[count] = layout.Rigid(rawText(th, th.T.Label, disabledInk(gtx, th.R.Fg), strconv.Itoa(s.Value)))
	}
	count++
	if s.Unit != "" {
		children[count] = layout.Rigid(layout.Spacer{Width: unit.Dp(tokens.SizeStepperUnitGap)}.Layout)
		count++
		children[count] = layout.Rigid(rawText(th, th.T.Caption, disabledInk(gtx, th.R.Fg3), s.Unit))
		count++
	}
	valueGTX := gtx
	valueGTX.Constraints.Max.Y = height
	record := op.Record(gtx.Ops)
	value := layout.Flex{Axis: layout.Horizontal, Alignment: layout.Baseline}.Layout(valueGTX, children[:count]...)
	call := record.Stop()
	size := constrainSize(gtx, image.Pt(value.Size.X+2*gtx.Dp(th.Sp.S2), height))
	focus := float32(0)
	if s.Editable {
		focus = previewTransition(gtx, &s.focus, s.Preview, StateFocus, gtx.Focused(&s.editor), th.Motion.FocusIn, th.Motion.FocusOut)
	}
	drawFieldSurface(gtx, th, image.Rectangle{Max: size}, gtx.Dp(th.Cut.Small), th.R.Field, focus, false)
	offset := op.Offset(image.Pt((size.X-value.Size.X)/2, (size.Y-value.Size.Y)/2)).Push(gtx.Ops)
	call.Add(gtx.Ops)
	offset.Pop()
	return layout.Dimensions{Size: size}
}

// stepperTextWidth measures a bound in the value's type, discarding the drawing.
func stepperTextWidth(gtx layout.Context, th *theme.Theme, value string) int {
	gtx.Constraints = layout.Constraints{Max: image.Pt(1<<24, 1<<24)}
	record := op.Record(gtx.Ops)
	dims := rawText(th, th.T.Label, th.R.Fg, value)(gtx)
	record.Stop()
	return dims.Size.X
}
