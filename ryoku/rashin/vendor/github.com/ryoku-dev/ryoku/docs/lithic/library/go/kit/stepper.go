package kit

import (
	"image"
	"strconv"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/symbols"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/theme"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/tokens"
)

var (
	iconStepRemove = Icon(symbols.Remove)
	iconStepAdd    = Icon(symbols.Add)
)

// Stepper changes an integer value between bounds.
type Stepper struct {
	Value   int
	Min     int
	Max     int
	Step    int
	Unit    string
	Preview State
	minus   IconPlaque
	plus    IconPlaque
	changed bool
}

// Changed reports and clears the value-change latch.
func (s *Stepper) Changed() bool {
	changed := s.changed
	s.changed = false
	return changed
}

// Layout draws and operates the stepper.
func (s *Stepper) Layout(gtx layout.Context, th *theme.Theme) layout.Dimensions {
	gtx.Constraints.Min = image.Point{}
	if s.Max < s.Min {
		s.Max = s.Min
	}
	s.Value = min(max(s.Value, s.Min), s.Max)
	step := s.Step
	if step <= 0 {
		step = 1
	}
	if s.minus.Clicked(gtx) && s.Value > s.Min {
		s.Value = max(s.Min, s.Value-step)
		s.changed = true
	}
	if s.plus.Clicked(gtx) && s.Value < s.Max {
		s.Value = min(s.Max, s.Value+step)
		s.changed = true
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
	if s.Value < s.Max {
		s.plus.Preview = s.Preview
	} else {
		s.minus.Preview = s.Preview
	}

	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle, Gap: gtx.Dp(unit.Dp(tokens.SizeStepperGap))}.Layout(gtx,
		layout.Rigid(func(layout.Context) layout.Dimensions {
			return s.minus.Layout(minusGTX, th)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			var children [3]layout.FlexChild
			count := 0
			children[count] = layout.Rigid(rawText(th, th.T.Label, disabledInk(gtx, th.R.Fg), strconv.Itoa(s.Value)))
			count++
			if s.Unit != "" {
				children[count] = layout.Rigid(layout.Spacer{Width: unit.Dp(tokens.SizeStepperUnitGap)}.Layout)
				count++
				children[count] = layout.Rigid(rawText(th, th.T.Caption, disabledInk(gtx, th.R.Fg3), s.Unit))
				count++
			}
			height := gtx.Dp(unit.Dp(tokens.SizeIconPlaqueSmall))
			valueGTX := gtx
			valueGTX.Constraints.Max.Y = height
			record := op.Record(gtx.Ops)
			value := layout.Flex{Axis: layout.Horizontal, Alignment: layout.Baseline}.Layout(valueGTX, children[:count]...)
			call := record.Stop()
			size := constrainSize(gtx, image.Pt(value.Size.X+2*gtx.Dp(th.Sp.S2), height))
			drawFieldSurface(gtx, th, image.Rectangle{Max: size}, gtx.Dp(th.Cut.Small), th.R.Field, 0, false)
			offset := op.Offset(image.Pt((size.X-value.Size.X)/2, (size.Y-value.Size.Y)/2)).Push(gtx.Ops)
			call.Add(gtx.Ops)
			offset.Pop()
			return layout.Dimensions{Size: size}
		}),
		layout.Rigid(func(layout.Context) layout.Dimensions {
			return s.plus.Layout(plusGTX, th)
		}),
	)
}
