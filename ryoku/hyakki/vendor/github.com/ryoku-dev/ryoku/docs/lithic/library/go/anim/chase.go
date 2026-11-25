package anim

import (
	"math"
	"time"

	"gioui.org/layout"
	"gioui.org/op"
)

// EaseOutCubic is the Hub's working curve, Qt's Easing.OutCubic: 1 - (1 - t)^3.
func EaseOutCubic(t float32) float32 {
	if t <= 0 {
		return 0
	}
	if t >= 1 {
		return 1
	}
	inverse := 1 - t
	return 1 - inverse*inverse*inverse
}

// Chase is a value that follows its target smoothly and at once, as the Hub's Chase.qml does:
// each frame it closes a share of the gap that depends only on the time passed, so it never
// overshoots, never runs away, and looks the same at 60 Hz or 165 Hz. It asks for frames only
// while a gap remains; under reduced motion it jumps.
//
// The zero Chase stands at 0 with Rate 28 (90 % of a move in about 80 ms) and Epsilon 0.001.
// Use Snap for a first placement.
type Chase struct {
	// Rate is how fast the gap closes, per second; zero means 28.
	Rate float32
	// Epsilon is the gap that counts as closed; zero means 0.001.
	Epsilon float32

	value  float32
	target float32
	last   time.Time
}

// chaseRate and chaseEpsilon are Chase.qml's defaults (Chase.qml:13-14).
const (
	chaseRate    = 28
	chaseEpsilon = 0.001
	// chaseLongest caps one frame's step at 100 ms, as Chase.qml does, so a stalled frame
	// does not throw the value across the gap.
	chaseLongest = 0.1
)

// Set moves the target. Under reduced motion the value stands there at once.
func (chase *Chase) Set(target float32) {
	chase.target = target
	if Reduced() {
		chase.value, chase.last = target, time.Time{}
	}
}

// Snap stands at v at once.
func (chase *Chase) Snap(v float32) {
	chase.value, chase.target, chase.last = v, v, time.Time{}
}

// Target returns the value being chased.
func (chase *Chase) Target() float32 { return chase.target }

// Value advances the chase to gtx.Now and returns it, requesting a frame only while a gap
// remains.
func (chase *Chase) Value(gtx layout.Context) float32 {
	epsilon := chase.Epsilon
	if epsilon <= 0 {
		epsilon = chaseEpsilon
	}
	if Reduced() || abs32(chase.target-chase.value) <= epsilon {
		chase.value, chase.last = chase.target, time.Time{}
		return chase.value
	}
	if !chase.last.IsZero() {
		rate := chase.Rate
		if rate <= 0 {
			rate = chaseRate
		}
		delta := min(gtx.Now.Sub(chase.last).Seconds(), chaseLongest)
		if delta > 0 {
			step := float32(1 - math.Exp(-delta*float64(rate)))
			next := chase.value + (chase.target-chase.value)*step
			if abs32(chase.target-next) <= epsilon {
				next = chase.target
			}
			chase.value = next
		}
	}
	if chase.value == chase.target {
		chase.last = time.Time{}
		return chase.value
	}
	chase.last = gtx.Now
	gtx.Execute(op.InvalidateCmd{})
	return chase.value
}

func abs32(v float32) float32 {
	if v < 0 {
		return -v
	}
	return v
}
