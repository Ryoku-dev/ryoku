package look

import (
	"image"
	"math"
	"time"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/paint"
	"gioui.org/unit"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/anim"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
)

// the Hub's page head puts SpecStripAbove between its words and the strip
const (
	SpecStripAbove unit.Dp = 16

	specGap      unit.Dp = 18
	specLabelGap unit.Dp = 6
	specTracking         = 1.2
	revealRise   unit.Dp = 8
)

// one figure of a page's spec strip: the label arrives translated and is set in spaced capitals, the value as it reads, Lit for one worth the accent
type Spec struct {
	Label, Value string
	Lit          bool
}

// SpecStrip sets a page's figures on one line under its head, as the Hub's
// page head does: "SESSIONS 12   NEED YOU 02". It allocates nothing for
// figures already shown.
func SpecStrip(gtx layout.Context, th *Theme, specs []Spec) layout.Dimensions {
	if len(specs) == 0 {
		return layout.Dimensions{}
	}
	gtx.Constraints.Min = image.Point{}
	labelGap, gap := gtx.Dp(specLabelGap), gtx.Dp(specGap)
	x, height, baseline := 0, 0, 0
	for index, spec := range specs {
		if index > 0 {
			x += gap
		}
		offset := op.Offset(image.Pt(x, 0)).Push(gtx.Ops)
		label := kit.LayoutSpacedCaps(gtx, th.Theme, th.T.MonoSm, th.R.Fg3, specTracking, spec.Label)
		offset.Pop()
		x += label.Size.X + labelGap
		ink := th.R.Fg
		if spec.Lit {
			ink = th.R.Sel
		}
		offset = op.Offset(image.Pt(x, 0)).Push(gtx.Ops)
		value := kit.LayoutSpacedCaps(gtx, th.Theme, th.T.MonoSm, ink, 0, spec.Value)
		offset.Pop()
		x += value.Size.X
		height = max(height, label.Size.Y, value.Size.Y)
		baseline = max(baseline, label.Baseline)
	}
	return layout.Dimensions{Size: gtx.Constraints.Constrain(image.Pt(x, height)), Baseline: baseline}
}

// Reveal is a page's entrance as the Hub plays it: its first five groups
// each fade in and settle from 8 dp below, 40 ms after the group before, once.
// A refresh moves nothing until Reset; under reduced motion nothing plays.
type Reveal struct {
	groups [anim.RevealGroups]anim.Reveal
}

// Reset arms the groups again, for a page that has just opened.
func (r *Reveal) Reset() { r.groups = [anim.RevealGroups]anim.Reveal{} }

// Group lays w out as the page's group at order, 0 for the first, and rises
// it in on its turn; later groups stand.
func (r *Reveal) Group(gtx layout.Context, th *Theme, order int, w layout.Widget) layout.Dimensions {
	if order < 0 || order >= len(r.groups) {
		return w(gtx)
	}
	group := &r.groups[order]
	group.Order = order
	if group.Done() {
		return w(gtx)
	}
	opacity, offset := group.Step(gtx, th.Motion.Navigate, th.Motion.RevealStagger, float32(gtx.Dp(revealRise)))
	if opacity >= 1 && offset == 0 {
		return w(gtx)
	}
	// drawn even at nothing, so the group's handlers are in the frame while it arrives
	fade := paint.PushOpacity(gtx.Ops, opacity)
	rise := op.Offset(image.Pt(0, int(offset+.5))).Push(gtx.Ops)
	dimensions := w(gtx)
	rise.Pop()
	fade.Pop()
	return dimensions
}

// CountUp runs a board's figure up to its value when it first shows and from
// the old value to a new one after, over th.Motion.CountUp; under reduced
// motion the figure is simply there.
type CountUp struct {
	ready    bool
	from, to int
	start    time.Time
}

func (c *CountUp) Value(gtx layout.Context, th *Theme, target int) int {
	duration := th.Motion.CountUp
	if anim.Reduced() || duration <= 0 || gtx.Now.IsZero() {
		c.ready, c.from, c.to = true, target, target
		return target
	}
	if !c.ready || target != c.to {
		from := 0
		if c.ready {
			from = c.at(gtx.Now, duration)
		}
		c.ready, c.from, c.to, c.start = true, from, target, gtx.Now
	}
	value := c.at(gtx.Now, duration)
	if value != c.to {
		gtx.Execute(op.InvalidateCmd{})
	}
	return value
}

func (c *CountUp) at(now time.Time, duration time.Duration) int {
	elapsed := now.Sub(c.start)
	if elapsed >= duration {
		return c.to
	}
	progress := anim.EaseOutCubic(float32(elapsed) / float32(duration))
	return c.from + int(math.Round(float64(float32(c.to-c.from)*progress)))
}
