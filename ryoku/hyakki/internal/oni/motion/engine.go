// Package motion integrates Oni's pose between daemon events: springs toward
// the state targets, keyframed gestures, hover float, the accent lap and the
// frame tier. It imports only spec, so every renderer and the ledger replay
// drive the same numbers.
package motion

import (
	"math"
	"time"

	"ryoku-rashin/internal/oni/spec"
)

// Input is what the host knows each frame. State, when set, applies like
// SetState without a reason; hosts that call SetState leave it empty.
// Pointer is the pointer's offset from the character centre in logical px
// (nil when unknown); Drag is set while the character is being dragged.
type Input struct {
	State        string      `json:"state,omitempty"`
	Reduced      bool        `json:"reduced"`
	BatterySaver bool        `json:"batterySaver"`
	Mic          float64     `json:"mic"`
	Speech       float64     `json:"speech"`
	Pointer      *spec.Point `json:"pointer,omitempty"`
	Drag         *Drag       `json:"drag,omitempty"`
}

// Drag carries the drag velocity in logical px/s.
type Drag struct {
	Velocity spec.Point `json:"velocity"`
}

// Pose is one frame of the character. Flat, Face and Loose are shape
// weights; Spin is an angle in radians in [0, 2π); Scale and Bright are
// multipliers around 1; Squash is signed (+ wider and shorter); leans are in
// spec.Springs.PoseRanges.Lean; HoverY is logical px (+ down), quantised.
type Pose struct {
	State      string  `json:"state"`
	Gesture    string  `json:"gesture"`
	Flat       float64 `json:"flat"`
	Face       float64 `json:"face"`
	Loose      float64 `json:"loose"`
	Spin       float64 `json:"spin"`
	Scale      float64 `json:"scale"`
	Squash     float64 `json:"squash"`
	LeanX      float64 `json:"leanX"`
	LeanY      float64 `json:"leanY"`
	Unravel    float64 `json:"unravel"`
	Straighten float64 `json:"straighten"`
	Tempo      float64 `json:"tempo"`
	Bright     float64 `json:"bright"`
	HoverY     float64 `json:"hoverY"`
	Lap        Lap     `json:"lap"`
}

// Lap is the accent colour change: From and To are spec palette ids,
// Progress runs 0→1 round the character, Glow is the accent glow in [0,1].
type Lap struct {
	From     string  `json:"from"`
	To       string  `json:"to"`
	Progress float64 `json:"progress"`
	Glow     float64 `json:"glow"`
}

// Tick is one Step's result. Redraw is set when Pose differs from the last
// committed pose. Next is the delay until the next Step at FPS. Stop means
// no timer is needed until the next event (SetState, Play, Reset or an
// Input change); Next and FPS are then zero.
type Tick struct {
	Pose   Pose
	Redraw bool
	Next   time.Duration
	Stop   bool
	FPS    int
}

type spring struct{ X, V float64 }

func (s *spring) step(target float64, sp spec.Spring, d float64) {
	s.X, s.V = spec.SpringStep(s.X, s.V, target, sp, d)
}

func (s *spring) snap(target float64) { s.X, s.V = target, 0 }

func (s spring) rest(target float64) bool { return s.X == target && s.V == 0 }

type Engine struct {
	s *spec.OniSpec

	state string
	in    Input

	shape        [3]spring
	shapeT       [3]float64
	pose         [nCh]spring
	poseT        [nCh]float64
	amp, breath  spring
	ampT, breatT float64

	spin       float64
	spinV      float64
	spinning   bool
	spinStop   bool
	spinTarget float64

	hoverPhase float64
	speakPhase float64

	g   gestureRun
	lap lapState

	sleepT    float64
	committed bool
	asleep    bool

	settledT float64
	event    bool
	stopped  bool
	drawn    bool
	last     Pose
}

func New(s *spec.OniSpec) *Engine {
	e := &Engine{s: s}
	e.Reset()
	return e
}

// Reset returns to the idle pose at rest, as if just created.
func (e *Engine) Reset() {
	*e = Engine{s: e.s, state: "idle"}
	e.g.clear()
	e.targets(true)
	e.snapAll()
	e.lap.reset(e.colour())
	e.event = true
}

func (e *Engine) colour() string { return e.s.Colour.StateColour[e.state] }

// Step advances the pose by dt (capped at the spec's frame cap) under in.
func (e *Engine) Step(dt time.Duration, in Input) Tick {
	if in.State != "" {
		e.SetState(in.State, "")
	}
	if e.inputEvent(in) {
		e.event = true
	}
	e.in = clone(in)
	event := e.event
	e.event = false
	if !event && e.stopped {
		return Tick{Pose: e.last, Stop: true}
	}
	if event {
		e.stopped = false
		e.settledT = 0
	}
	if in.Reduced {
		return e.still()
	}

	d := dt.Seconds()
	if !(d > 0) {
		d = 0
	}
	d = min(d, e.s.Frames.DtCapMs/1000)
	e.advance(d)

	stop, commit := false, false
	if e.state == "sleeping" && !e.committed && e.sleepT >= e.s.Sleep.CommitS && !e.g.active(e.s) {
		e.targets(false)
		e.snapAll()
		e.spinning, e.spinStop, e.spinV = false, false, 0
		e.spin = wrap(e.spin)
		e.lap.complete()
		e.committed, commit = true, true
		if e.s.Frames.AsleepStopsAfterCommit {
			e.asleep, stop = true, true
		}
	}
	pose := e.compose(false)
	if !stop && e.settledT*1000 >= e.s.Frames.SettledStopMs && !e.moving() {
		stop = true
	}
	return e.tick(pose, commit, stop)
}

func (e *Engine) tick(pose Pose, force, stop bool) Tick {
	t := Tick{Pose: pose, Redraw: force || !e.drawn || pose != e.last}
	if t.Redraw {
		e.last, e.drawn = pose, true
	}
	if stop {
		e.stopped = true
		t.Stop = true
		return t
	}
	t.FPS = e.fps()
	if t.FPS > 0 {
		t.Next = time.Second / time.Duration(t.FPS)
	}
	return t
}

// still is reduced motion: each event lands straight on its static pose,
// paints once and stops.
func (e *Engine) still() Tick {
	e.g.clear()
	e.lap.reset(e.colour())
	e.targets(true)
	e.snapAll()
	e.spinning, e.spinStop, e.spinV = false, false, 0
	e.spin = wrap(e.spin)
	e.committed = e.state == "sleeping"
	e.asleep = e.committed
	return e.tick(e.compose(true), true, true)
}

func (e *Engine) advance(d float64) {
	e.settledT += d
	if e.state == "sleeping" {
		e.sleepT += d
	}
	e.g.advance(d)
	if e.lap.advance(d, e.s) {
		e.lap.set(e.colour())
	}
	e.targets(false)
	sh := e.s.Springs.Shape
	for i, sp := range [3]spec.Spring{sh.Flat, sh.Face, sh.Loose} {
		e.shape[i].step(e.shapeT[i], sp, d)
	}
	ps := e.s.Springs.Pose
	for i, sp := range [nCh]spec.Spring{ps.Scale, ps.Squash, ps.LeanX, ps.LeanY, ps.Unravel, ps.Straighten, ps.Tempo, ps.Bright} {
		e.pose[i].step(e.poseT[i], sp, d)
	}
	e.amp.step(e.ampT, sh.Slack, d)
	e.breath.step(e.breatT, sh.Slack, d)
	e.advanceSpin(d)
	st := e.s.States[e.state]
	e.hoverPhase = math.Mod(e.hoverPhase+st.Hover[1]*d, 4*math.Pi)
	if e.in.Speech > 0 {
		e.speakPhase += d
	}
}

func (e *Engine) advanceSpin(d float64) {
	ss := e.s.Motion.SpinStop
	if rate := e.s.States[e.state].Spin; rate > 0 {
		e.spin = wrap(e.spin + rate*d)
		e.spinV, e.spinning, e.spinStop = rate, true, false
		return
	}
	if e.spinning {
		div := 2 * math.Pi / float64(max(ss.Divisions, 1))
		e.spinTarget = math.Ceil((e.spin+ss.Lead)/div) * div
		e.spinning, e.spinStop = false, true
	}
	if e.spinStop {
		e.spin, e.spinV = spec.SpringStep(e.spin, e.spinV, e.spinTarget, ss.Spring, d)
		if e.spin == e.spinTarget && e.spinV == 0 {
			e.spin, e.spinStop = wrap(e.spin), false
		}
	}
}

func (e *Engine) snapAll() {
	for i := range e.shape {
		e.shape[i].snap(e.shapeT[i])
	}
	for i := range e.pose {
		e.pose[i].snap(e.poseT[i])
	}
	e.amp.snap(e.ampT)
	e.breath.snap(e.breatT)
}

func (e *Engine) springsAtRest() bool {
	for i := range e.shape {
		if !e.shape[i].rest(e.shapeT[i]) {
			return false
		}
	}
	for i := range e.pose {
		if !e.pose[i].rest(e.poseT[i]) {
			return false
		}
	}
	return e.amp.rest(e.ampT) && e.breath.rest(e.breatT)
}

// moving is any time-bounded motion still under way; the hover, breath,
// spin and glow loops are not.
func (e *Engine) moving() bool {
	return e.g.active(e.s) || e.lap.moving(e.s) || e.spinStop || !e.springsAtRest()
}

func (e *Engine) inputEvent(in Input) bool {
	if e.asleep {
		return in.Drag != nil && e.in.Drag == nil
	}
	if in.Reduced != e.in.Reduced {
		return true
	}
	if in.Reduced {
		return false
	}
	return in.BatterySaver != e.in.BatterySaver || in.Mic != e.in.Mic || in.Speech != e.in.Speech ||
		!samePoint(in.Pointer, e.in.Pointer) || !sameDrag(in.Drag, e.in.Drag)
}

func samePoint(a, b *spec.Point) bool {
	return a == nil && b == nil || a != nil && b != nil && *a == *b
}

func sameDrag(a, b *Drag) bool {
	return a == nil && b == nil || a != nil && b != nil && *a == *b
}

func clone(in Input) Input {
	if in.Pointer != nil {
		p := *in.Pointer
		in.Pointer = &p
	}
	if in.Drag != nil {
		d := *in.Drag
		in.Drag = &d
	}
	in.State = ""
	return in
}

func wrap(a float64) float64 {
	a = math.Mod(a, 2*math.Pi)
	if a < 0 {
		a += 2 * math.Pi
	}
	return a + 0
}

func unit(f float64) float64 { return min(max(f, 0), 1) }

func clampRange(f float64, r spec.Range) float64 { return min(max(f, r[0]), r[1]) }
