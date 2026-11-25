// Package anim provides Lithic motion that requests frames only while moving.
package anim

import (
	"math"
	"sync/atomic"
	"time"

	"gioui.org/layout"
	"gioui.org/op"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/tokens"
)

var reduce atomic.Bool

// SetReduce turns every animation into an instant state change.
func SetReduce(enabled bool) { reduce.Store(enabled) }

// Reduced reports whether reduced motion is active.
func Reduced() bool { return reduce.Load() }

// Spring is a critically damped spring toward its target.
type Spring struct {
	Omega  float32
	Rest   float32
	pos    float32
	vel    float32
	target float32
	last   time.Time
	init   bool
}

// Set changes the target, placing an uninitialized spring there directly.
func (spring *Spring) Set(target float32) {
	if !spring.init {
		spring.pos, spring.target, spring.init = target, target, true
		return
	}
	spring.target = target
}

// Snap jumps to value and settles.
func (spring *Spring) Snap(value float32) {
	spring.pos, spring.target, spring.vel, spring.init = value, value, 0, true
}

// Target returns the current target.
func (spring *Spring) Target() float32 { return spring.target }

// Moving reports whether position or velocity is outside the rest threshold.
func (spring *Spring) Moving() bool {
	rest := float64(spring.Rest)
	if rest <= 0 {
		rest = 0.001
	}
	return math.Abs(float64(spring.pos-spring.target)) > rest || math.Abs(float64(spring.vel)) > rest
}

// Value advances the spring to gtx.Now.
func (spring *Spring) Value(gtx layout.Context) float32 {
	if !spring.init {
		return spring.target
	}
	now := gtx.Now
	if Reduced() || spring.last.IsZero() {
		if Reduced() {
			spring.pos, spring.vel = spring.target, 0
		}
		spring.last = now
		if spring.Moving() {
			gtx.Execute(op.InvalidateCmd{})
		}
		return spring.pos
	}
	delta := float32(now.Sub(spring.last).Seconds())
	spring.last = now
	if delta > 0.05 {
		delta = 0.05
	}
	omega := spring.Omega
	if omega <= 0 {
		omega = 18
	}
	distance := spring.pos - spring.target
	decay := float32(math.Exp(float64(-omega * delta)))
	spring.pos = spring.target + (distance+(spring.vel+omega*distance)*delta)*decay
	spring.vel = (spring.vel - omega*(spring.vel+omega*distance)*delta) * decay
	if spring.Moving() {
		gtx.Execute(op.InvalidateCmd{})
	} else {
		spring.pos, spring.vel = spring.target, 0
	}
	return spring.pos
}

// Tween moves between two values over a fixed duration.
type Tween struct {
	Duration time.Duration
	Ease     func(float32) float32
	from     float32
	to       float32
	start    time.Time
	init     bool
}

// Set starts a run from the current value to value.
func (tween *Tween) Set(gtx layout.Context, value float32) {
	if !tween.init {
		tween.from, tween.to, tween.init = value, value, true
		return
	}
	if value == tween.to {
		return
	}
	tween.from, tween.to, tween.start = tween.Value(gtx), value, gtx.Now
}

// Value returns the current value and requests another frame while moving.
func (tween *Tween) Value(gtx layout.Context) float32 {
	if !tween.init || Reduced() || tween.Duration <= 0 || tween.start.IsZero() {
		return tween.to
	}
	progress := float32(gtx.Now.Sub(tween.start)) / float32(tween.Duration)
	if progress >= 1 {
		return tween.to
	}
	progress = max(progress, 0)
	gtx.Execute(op.InvalidateCmd{})
	ease := tween.Ease
	if ease == nil {
		ease = EaseChisel
	}
	return tween.from + (tween.to-tween.from)*ease(progress)
}

// Phase returns a looping zero-to-one phase and requests another frame.
func Phase(gtx layout.Context, period time.Duration) float32 {
	if Reduced() || period <= 0 {
		return 0
	}
	gtx.Execute(op.InvalidateCmd{})
	return float32(gtx.Now.UnixNano()%int64(period)) / float32(period)
}

// EaseChisel evaluates cubic-bezier(.2, 0, 0, 1).
func EaseChisel(progress float32) float32 {
	if progress <= 0 {
		return 0
	}
	if progress >= 1 {
		return 1
	}
	bezier := func(value, first, second float32) float32 {
		inverse := 1 - value
		return 3*inverse*inverse*value*first + 3*inverse*value*value*second + value*value*value
	}
	curve := tokens.MotionCurve
	low, high := float32(0), float32(1)
	for range 24 {
		middle := (low + high) / 2
		if bezier(middle, curve[0], curve[2]) < progress {
			low = middle
		} else {
			high = middle
		}
	}
	return bezier((low+high)/2, curve[1], curve[3])
}
