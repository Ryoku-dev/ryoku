// Package placement is the pure geometry that puts the character on an
// output: corner pins, saved per-output fractions, the edge clamp, output
// choice, the drag threshold and the display-change flight. All points and
// sizes are logical px in the compositor's global layout; a point is the
// top-left of the character frame.
package placement

import (
	"image"
	"math"
	"slices"
)

// Output is one display; Scale is its (fractional) buffer scale, carried
// for the host since the geometry itself is logical.
type Output struct {
	ID    string
	Rect  image.Rectangle
	Scale float64
}

// Saved is a free position as fractions of the output's free span, so it
// survives resolution and scale changes.
type Saved struct{ FX, FY float64 }

// Place is the persisted placement: a corner pin ("" when free), one saved
// fraction per output, and the output the character last sat on.
type Place struct {
	Corner  string
	Outputs map[string]Saved
	Active  string
}

// Corners are the pin ids; "" means free placement.
var Corners = []string{"tl", "tr", "bl", "br"}

// DefaultCorner is where a character with no saved position sits.
const DefaultCorner = "br"

// Resolve places a frame on an output: pinned to corner when it is a known
// corner, else at the saved fraction, else at DefaultCorner. The result is
// always clamped margin px inside the output.
func Resolve(o Output, frame image.Point, corner string, saved *Saved, margin int) image.Point {
	if !slices.Contains(Corners, corner) {
		if saved != nil {
			p := image.Pt(
				o.Rect.Min.X+int(math.Round(unit(saved.FX)*float64(o.Rect.Dx()-frame.X))),
				o.Rect.Min.Y+int(math.Round(unit(saved.FY)*float64(o.Rect.Dy()-frame.Y))),
			)
			return Clamp(o, p, frame, margin)
		}
		corner = DefaultCorner
	}
	p := image.Pt(o.Rect.Min.X+margin, o.Rect.Min.Y+margin)
	if corner == "tr" || corner == "br" {
		p.X = o.Rect.Max.X - margin - frame.X
	}
	if corner == "bl" || corner == "br" {
		p.Y = o.Rect.Max.Y - margin - frame.Y
	}
	return Clamp(o, p, frame, margin)
}

// Clamp keeps the frame at least margin px inside every edge of the output;
// an axis too small for the frame and both margins centres it instead.
func Clamp(o Output, p, frame image.Point, margin int) image.Point {
	return image.Pt(
		clampAxis(p.X, o.Rect.Min.X, o.Rect.Max.X, frame.X, margin),
		clampAxis(p.Y, o.Rect.Min.Y, o.Rect.Max.Y, frame.Y, margin),
	)
}

func clampAxis(v, lo, hi, size, margin int) int {
	minV, maxV := lo+margin, hi-margin-size
	if minV > maxV {
		return lo + (hi-lo-size)/2
	}
	return min(max(v, minV), maxV)
}

// Fraction is the inverse of a free Resolve: the frame's top-left as a
// fraction of the output's free span on each axis.
func Fraction(o Output, p, frame image.Point) Saved {
	return Saved{
		FX: fraction(p.X-o.Rect.Min.X, o.Rect.Dx()-frame.X),
		FY: fraction(p.Y-o.Rect.Min.Y, o.Rect.Dy()-frame.Y),
	}
}

func fraction(off, span int) float64 {
	if span <= 0 {
		return 0.5
	}
	return unit(float64(off) / float64(span))
}

func unit(f float64) float64 {
	if math.IsNaN(f) {
		return 0.5
	}
	return min(max(f, 0), 1)
}

// Choose picks the output the character belongs on: the remembered active
// output while it exists, else the focused one, else the first. It reports
// false only when there are no outputs.
func Choose(outs []Output, p Place, focused string) (Output, bool) {
	if len(outs) == 0 {
		return Output{}, false
	}
	for _, id := range []string{p.Active, focused} {
		if id == "" {
			continue
		}
		if i := slices.IndexFunc(outs, func(o Output) bool { return o.ID == id }); i >= 0 {
			return outs[i], true
		}
	}
	return outs[0], true
}
