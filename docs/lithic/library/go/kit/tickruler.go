package kit

import (
	"image"
	"image/color"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/anim"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/theme"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/tokens"
)

// RulerStyle picks one of the Hub's two tick rulers.
type RulerStyle uint8

const (
	// RulerMasthead is the ruler under the sheet's masthead (Masthead.qml:123-145): 7 tall, a
	// 1 px tick every 8, every fifth tick 7 tall and the others 4, standing on a 1 px
	// baseline at 6; vein.
	RulerMasthead RulerStyle = iota
	// RulerMotto is the ruler under Home's motto, as on ryoku.dev (TickRuler.qml:4-31): s2
	// tall, a hairline tick every s2, every fifth s2 tall and the others s1, and a first
	// segment s4 long and 2 tall in Lead.
	RulerMotto
)

// TickRuler is the Hub's tick ruler, read by no style. The masthead ruler fills the width it
// is given; the motto ruler takes min(width, 233) as Home sets it (Home.qml:757-760). The
// ticks are recorded once per width, scale and ink and replayed; only the scroll marker is
// drawn live, and it asks for frames only while it moves.
type TickRuler struct {
	Style RulerStyle
	// Lead is the motto ruler's first segment; zero means th.R.Fg.
	Lead color.NRGBA

	marker anim.Tween

	ops  op.Ops
	call op.CallOp
	key  rulerKey
}

type rulerKey struct {
	width int
	scale float32
	ink   color.NRGBA
	lead  color.NRGBA
	style RulerStyle
}

// Masthead ruler geometry, in dp (Masthead.qml:139-152).
const (
	rulerPitch       = 8
	rulerTall        = 7
	rulerShort       = 4
	rulerBaselineY   = 6
	rulerEvery       = 5
	rulerMarkerWide  = 24
	rulerMarkerTall  = 3
	rulerMottoNature = 233
)

// Layout draws the ruler. progress places the masthead's marker, the 24 by 3 accent block
// that stands where the page is read to (0..1); a negative progress hides it. The marker
// eases 120 ms (th.Motion.Quick) on OutCubic.
func (ruler *TickRuler) Layout(gtx layout.Context, th *theme.Theme, progress float32) layout.Dimensions {
	height := gtx.Dp(rulerTall)
	if ruler.Style == RulerMotto {
		height = gtx.Dp(tokens.SpaceS2)
	}
	width := gtx.Constraints.Max.X
	if ruler.Style == RulerMotto {
		width = min(width, gtx.Dp(rulerMottoNature))
	}
	size := gtx.Constraints.Constrain(image.Pt(width, height))
	lead := ruler.Lead
	if lead == (color.NRGBA{}) {
		lead = th.R.Fg
	}
	key := rulerKey{width: size.X, scale: gtx.Metric.PxPerDp, ink: th.R.Vein, lead: lead, style: ruler.Style}
	if key != ruler.key || ruler.call == (op.CallOp{}) {
		ruler.key = key
		ruler.record(gtx, size)
	}
	ruler.call.Add(gtx.Ops)

	if progress >= 0 {
		ruler.marker.Duration = th.Motion.Quick
		ruler.marker.Ease = anim.EaseOutCubic
		ruler.marker.Set(gtx, min(max(progress, 0), 1))
		wide, tall := gtx.Dp(rulerMarkerWide), gtx.Dp(rulerMarkerTall)
		x := int(ruler.marker.Value(gtx)*float32(max(0, size.X-wide)) + 0.5)
		paint.FillShape(gtx.Ops, th.R.Sel, clip.Rect(image.Rect(x, size.Y-tall, x+wide, size.Y)).Op())
	}
	return layout.Dimensions{Size: size}
}

// record draws the ticks into the ruler's own ops.
func (ruler *TickRuler) record(gtx layout.Context, size image.Point) {
	ruler.ops.Reset()
	macro := op.Record(&ruler.ops)
	hair := max(1, gtx.Dp(tokens.SizeHairline))
	pitch, tall, short := gtx.Dp(rulerPitch), size.Y, gtx.Dp(rulerShort)
	if ruler.Style == RulerMotto {
		pitch, short = gtx.Dp(tokens.SpaceS2), gtx.Dp(tokens.SpaceS1)
	}
	var path clip.Path
	path.Begin(&ruler.ops)
	for i := 0; i*pitch <= size.X && pitch > 0; i++ {
		h := short
		if i%rulerEvery == 0 {
			h = tall
		}
		addRect(&path, image.Rect(i*pitch, size.Y-h, i*pitch+hair, size.Y))
	}
	if ruler.Style == RulerMasthead {
		y := gtx.Dp(rulerBaselineY)
		addRect(&path, image.Rect(0, y, size.X, y+hair))
	}
	clipped := clip.Outline{Path: path.End()}.Op().Push(&ruler.ops)
	paint.ColorOp{Color: ruler.key.ink}.Add(&ruler.ops)
	paint.PaintOp{}.Add(&ruler.ops)
	clipped.Pop()
	if ruler.Style == RulerMotto {
		lead := gtx.Dp(2)
		paint.FillShape(&ruler.ops, ruler.key.lead, clip.Rect(image.Rect(0, size.Y-lead, gtx.Dp(tokens.SpaceS4), size.Y)).Op())
	}
	ruler.call = macro.Stop()
}

// addRect adds r to path as a closed clockwise contour, so overlapping rectangles in one
// path fill once under the non-zero rule.
func addRect(path *clip.Path, r image.Rectangle) {
	if r.Empty() {
		return
	}
	path.MoveTo(layout.FPt(r.Min))
	path.LineTo(layout.FPt(image.Pt(r.Max.X, r.Min.Y)))
	path.LineTo(layout.FPt(r.Max))
	path.LineTo(layout.FPt(image.Pt(r.Min.X, r.Max.Y)))
	path.Close()
}
