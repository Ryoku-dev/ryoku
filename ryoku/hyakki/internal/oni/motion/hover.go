package motion

import "math"

// hoverY is the float offset, quantised so a slow float does not repaint
// for sub-pixel changes.
func (e *Engine) hoverY() float64 {
	hf := e.s.Motion.HoverFloat
	y := (math.Sin(e.hoverPhase) + hf.Secondary*math.Sin(e.hoverPhase/2+hf.SecondaryPhase)) * e.amp.X
	return quantise(y, hf.QuantizePx)
}

func quantise(v, q float64) float64 {
	if !(q > 0) {
		return v
	}
	return math.Round(v/q)*q + 0
}
