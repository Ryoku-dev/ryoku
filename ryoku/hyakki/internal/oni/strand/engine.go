package strand

import (
	"math"
	"slices"
	"time"

	"ryoku-rashin/internal/oni/spec"
)

// kind is the resolved shape of the current target.
type kind uint8

const (
	kPoint kind = iota
	kPerch
	kLasso
	kRegion
)

// eps absorbs float drift when a phase's elapsed time meets its duration.
const eps = 1e-9

// Engine drives one character's strand. Target and Retract are events;
// Step advances by dt under the host's Env and returns the frame to paint.
type Engine struct {
	s *spec.OniSpec

	tgt     Target
	k       kind
	seq     uint64
	pending bool

	phase Phase
	el    float64 // seconds into the phase

	// Main thread: a path captured from a0 (point, perch, a swung lasso),
	// or nil when the main thread is a live lead from the anchor.
	a0     spec.Point
	path   []spec.Point
	u      float64
	uFrom  float64
	reachS float64
	tapV   float64

	from  []spec.Point // swing: the old main thread
	fades []Stroke     // strokes a swing left behind, fading out
	fadeT float64

	pr    perchGeom
	curl  float64
	ls    lassoGeom
	lasso bool // a ring is part of the frame
	kRev  float64
	k0    float64
	cinch float64
	age   float64
	rg    regionGeom
	pen   float64 // σ drawn along the region path

	rw rewindFrom

	hold  float64
	burst float64 // seconds since landing; negative when no burst

	arrivedNow bool
	skipNow    string

	// reframe rebuilds the perch round the target's new frame at the next
	// Step; relanding marks a Reperch, whose landing is not a new arrival.
	reframe   bool
	relanding bool

	reduced bool
	event   bool
	stopped bool
	drawn   bool
	last    Frame
	bbox    Rect
}

func New(s *spec.OniSpec) *Engine {
	e := &Engine{s: s, burst: -1}
	e.home()
	e.event = true
	return e
}

// Target sends the thread to t. A target whose Seq is not newer than the
// last one is stale and ignored (Seq 0 is always taken). While the thread
// is out the tip swings to the new target without going home first.
func (e *Engine) Target(t Target) {
	if t.Seq != 0 && t.Seq <= e.seq {
		return
	}
	if t.Seq != 0 {
		e.seq = t.Seq
	}
	e.tgt = t
	e.pending = true
	e.event = true
	e.reframe, e.relanding = false, false
}

// Retract winds the thread home if seq is the current target; any other
// seq is stale and ignored.
func (e *Engine) Retract(seq uint64) {
	if seq != e.tgt.Seq {
		return
	}
	if e.pending {
		e.pending = false
		e.event = true
	}
	if e.phase == PhaseHome || e.phase == PhaseRewind {
		return
	}
	e.startRewind()
	e.event = true
}

// Reframe moves the current perch-mode target onto frame, keeping its seq:
// the perched window moved or resized and the character jumped with it. If
// the thread is out, the next Step rebuilds the curl round the same corner
// of the new frame from the live anchor, keeping the phase's progress (no
// reach replayed, no new arrival, no burst) and restarting the drawn bounds
// at the new geometry; while home or rewinding the frame is only stored. It
// reports false and changes nothing when the current target is not a
// perch-mode target.
func (e *Engine) Reframe(frame Rect) bool {
	if e.tgt.Mode != ModePerch {
		return false
	}
	e.setFrame(frame)
	if !e.pending && e.k == kPerch && e.phase != PhaseHome && e.phase != PhaseRewind {
		e.reframe = true
		e.event = true
	}
	return true
}

// Reperch sends the current perch-mode target out again onto frame, keeping
// its seq: after the long reattach the character has flown to its new seat
// and the thread reaches and curls once more, from where it is. The landing
// is quiet and is not a new arrival; a frame centred off the output stays
// home without a report. It reports false and changes nothing when the
// current target is not a perch-mode target.
func (e *Engine) Reperch(frame Rect) bool {
	if e.tgt.Mode != ModePerch {
		return false
	}
	e.setFrame(frame)
	e.reframe = false
	e.relanding = true
	e.pending = true
	e.event = true
	return true
}

func (e *Engine) setFrame(frame Rect) {
	f := frame
	e.tgt.Box, e.tgt.Frame = frame, &f
}

// Out is true while the thread is drawn, moving or about to leave.
func (e *Engine) Out() bool { return e.pending || e.phase != PhaseHome }

// Step advances the strand by dt (capped at the spec's frame cap).
func (e *Engine) Step(dt time.Duration, env Env) Tick {
	if env.Reduced != e.reduced {
		e.reduced = env.Reduced
		e.event = true
	}
	event := e.event
	e.event = false
	if !event && e.stopped {
		return Tick{Frame: e.last, Stop: true}
	}
	e.stopped = false
	e.arrivedNow, e.skipNow = false, ""
	if e.pending {
		e.begin(env)
	}
	if e.reframe {
		e.reframe = false
		e.rebuildPerch(env)
	}
	if env.Reduced {
		e.advance(math.MaxFloat32, env)
		e.fades, e.fadeT = nil, 0
		return e.tick(e.compose(env), env, true)
	}
	d := dt.Seconds()
	if !(d > 0) {
		d = 0
	}
	d = min(d, e.s.Frames.DtCapMs/1000)
	e.advance(d, env)
	f := e.compose(env)
	stop := e.phase == PhaseHome || (e.phase == PhaseHold && e.hold*1000 >= e.s.Strand.HoldStopMs-eps)
	return e.tick(f, env, stop)
}

func (e *Engine) tick(f Frame, env Env, stop bool) Tick {
	t := Tick{Frame: f, Redraw: !e.drawn || !samePixels(f, e.last)}
	e.drawn = true
	e.last = f
	t.Frame.Arrived, t.Frame.Skipped = e.arrivedNow, e.skipNow
	if stop {
		e.stopped = true
		t.Stop = true
		return t
	}
	activity := "motion"
	burstDone := e.burst < 0 || e.burst >= e.s.Strand.Burst.DurationS
	if e.phase == PhaseHold && burstDone && len(e.fades) == 0 {
		activity = "breathing"
	}
	t.FPS, _ = e.s.TargetFPS(activity, env.BatterySaver)
	if t.FPS > 0 {
		t.Next = time.Second / time.Duration(t.FPS)
	}
	return t
}

func samePixels(a, b Frame) bool {
	if a.Loosen != b.Loosen || a.Bbox != b.Bbox || len(a.Strokes) != len(b.Strokes) {
		return false
	}
	if (a.Char == nil) != (b.Char == nil) || (a.Char != nil && *a.Char != *b.Char) {
		return false
	}
	for i := range a.Strokes {
		x, y := a.Strokes[i], b.Strokes[i]
		if x.Width != y.Width || x.Alpha != y.Alpha || !slices.Equal(x.Pts, y.Pts) {
			return false
		}
	}
	return true
}

// home drops everything drawn.
func (e *Engine) home() {
	e.phase, e.el = PhaseHome, 0
	e.path, e.from, e.fades = nil, nil, nil
	e.u, e.uFrom, e.tapV, e.fadeT = 0, 0, 0, 0
	e.curl, e.lasso, e.kRev, e.k0, e.cinch, e.age, e.pen = 0, false, 0, 0, 0, 0, 0
	e.hold, e.burst = 0, -1
	e.bbox = Rect{}
}

// kindOf resolves a target's shape: region needs a region, a point target
// perches when it and its frame are window-sized.
func (e *Engine) kindOf(t Target) kind {
	switch t.Mode {
	case ModeLasso:
		return kLasso
	case ModeRegion:
		if t.Region != nil && !t.Region.empty() {
			return kRegion
		}
		return kLasso
	}
	if perches(e.s, t) {
		return kPerch
	}
	return kPoint
}

// skipReason applies the session's skip rules: a target covering at least
// skipCover of the output, or centred off it. A perch-mode target is judged
// by its frame and is never skipped for its size.
func (e *Engine) skipReason(t Target, env Env) string {
	if env.Screen.empty() {
		return "" // an unknown output skips nothing
	}
	box := t.Box
	if t.Mode == ModePerch {
		box = perchFrame(t)
	}
	if !env.Screen.contains(box.centre()) {
		return SkipOffOutput
	}
	sc := e.s.Session.SkipCover
	if t.Mode != ModePerch && box.W >= sc[0]*env.Screen.W && box.H >= sc[1]*env.Screen.H {
		return SkipCoversScreen
	}
	return ""
}

// begin starts the pending target from where the thread is now.
func (e *Engine) begin(env Env) {
	e.pending = false
	t := e.tgt
	if r := e.skipReason(t, env); r != "" {
		if e.relanding {
			e.relanding = false // a reperch off the output stays home unreported
		} else {
			e.arrivedNow, e.skipNow = true, r
		}
		if e.phase != PhaseHome && e.phase != PhaseRewind {
			e.startRewind()
		}
		return
	}
	uOld := e.extent() // of the thread that is out, before the kind changes
	e.k = e.kindOf(t)
	a := env.Anchor
	var land spec.Point
	switch e.k {
	case kPoint:
		land = tipAnchor(e.s, t.Box, a)
	case kPerch:
		frame := perchFrame(t)
		corner := t.Corner
		if corner != CornerTopLeft && corner != CornerTopRight {
			corner = PerchCorner(e.s, frame, a, env.Centre, env.Screen)
		}
		e.pr = newPerch(e.s, frame, corner)
		land = e.pr.land
	case kLasso:
		e.ls = newLasso(e.s, t.Box, a)
		land = e.ls.at(e.s, 0, e.s.Strand.Lasso.ThrownPx)
	case kRegion:
		e.rg = newRegion(e.s, *t.Region, a)
		land = e.rg.at(0)
	}
	out := e.phase != PhaseHome && len(e.last.Strokes) > 0 && len(e.last.Strokes[0].Pts) >= 2
	if out {
		e.startSwing(a, land, uOld, env)
	} else {
		e.home()
		e.startFresh(a, land, env)
	}
	e.hold, e.burst = 0, -1
}

func (e *Engine) startFresh(a, land spec.Point, env Env) {
	e.el = 0
	switch e.k {
	case kPoint, kPerch:
		e.a0 = a
		e.path = bowed(e.s, a, land, env.Centre, e.s.Strand.BowedPath.Segments)
		e.u, e.uFrom = 0, 0
		e.reachS = e.s.Strand.Reach.DurationS
		if e.k == kPerch {
			e.reachS = e.s.Strand.Reach.PerchS
		}
		e.phase = PhaseReach
	case kLasso:
		e.startLasso(false)
	case kRegion:
		e.path = nil
		e.pen = 0
		e.phase = PhaseRegion
	}
}

// rebuildPerch moves a perch that is out onto the target's new frame: the
// same corner, the main path from the live anchor to the new landing, the
// phase and its progress kept. A swing's old line is let go, and the drawn
// bounds restart at the new geometry.
func (e *Engine) rebuildPerch(env Env) {
	e.pr = newPerch(e.s, perchFrame(e.tgt), e.pr.corner())
	e.a0 = env.Anchor
	e.path = bowed(e.s, e.a0, e.pr.land, env.Centre, e.s.Strand.BowedPath.Segments)
	if e.phase == PhaseSwing {
		e.from = resample(visible(e.path, e.u), e.s.Strand.BowedPath.Points)
	}
	e.fades, e.fadeT = nil, 0
	e.bbox = Rect{}
}

// advance runs the phase machine for d seconds, carrying leftover time
// across phase ends.
func (e *Engine) advance(d float64, env Env) {
	if len(e.fades) > 0 {
		e.fadeT += d
		if e.fadeT*1000 >= e.s.Strand.RetargetBlendMs-eps {
			e.fades, e.fadeT = nil, 0
		}
	}
	for {
		var dur float64
		switch e.phase {
		case PhaseHome:
			return
		case PhaseHold:
			e.hold += d
			if e.burst >= 0 {
				e.burst += d
			}
			return
		case PhaseReach:
			dur = e.reachS
		case PhaseTap:
			dur = e.s.Strand.Tap.DurationS
		case PhaseSwing:
			dur = e.s.Strand.RetargetBlendMs / 1000
		case PhaseCurl:
			dur = e.s.Strand.Perch.CurlS
		case PhaseLasso:
			dur = e.s.Strand.Lasso.DrawS + e.s.Strand.Lasso.CinchS
		case PhaseRegion:
			dur = e.rg.dur
		case PhaseRewind:
			dur = e.rw.dur
		}
		rem := dur - e.el
		if d < rem-eps {
			e.el += d
			e.progress()
			return
		}
		d = math.Max(0, d-math.Max(0, rem))
		e.el = dur
		e.progress()
		e.finishPhase(env)
	}
}

// progress sets the phase's animated values at e.el.
func (e *Engine) progress() {
	switch e.phase {
	case PhaseReach:
		e.u = e.uFrom + (1-e.uFrom)*ease(e.s.Strand.Reach.Ease, e.el, e.reachS)
	case PhaseTap:
		e.tapV = clamp01(e.el / e.s.Strand.Tap.DurationS)
	case PhaseSwing:
		e.u = e.uFrom + (1-e.uFrom)*ease(e.s.Strand.Retarget.Ease, e.el, e.s.Strand.RetargetBlendMs/1000)
	case PhaseCurl:
		e.curl = ease(e.s.Strand.Reach.Ease, e.el, e.s.Strand.Perch.CurlS)
	case PhaseLasso:
		e.lassoProgress()
	case PhaseRegion:
		e.pen = e.rg.sigmaAt(e.el)
	case PhaseRewind:
		e.rewindProgress()
	}
}

// finishPhase moves to the phase after the current one ends.
func (e *Engine) finishPhase(env Env) {
	e.el = 0
	switch e.phase {
	case PhaseReach:
		e.u = 1
		if e.k == kPerch {
			e.phase = PhaseCurl
		} else {
			e.phase, e.tapV = PhaseTap, 0
		}
	case PhaseTap:
		e.tapV = 0
		e.land()
	case PhaseSwing:
		e.endSwing()
	case PhaseCurl:
		e.curl = 1
		e.land()
	case PhaseLasso:
		e.land()
	case PhaseRegion:
		e.pen = e.rg.total
		e.endRegion(env)
	case PhaseRewind:
		e.home()
	}
}

// land is the tip arriving: reported once for the current seq. A reperch
// lands quietly and reports nothing.
func (e *Engine) land() {
	e.phase, e.el = PhaseHold, 0
	e.hold = 0
	e.burst = -1
	if e.relanding {
		e.relanding = false
		return
	}
	e.arrivedNow = true
	if !e.tgt.Quiet {
		e.burst = 0
	}
}

// compose builds the frame from the current state.
func (e *Engine) compose(env Env) Frame {
	f := Frame{Seq: e.tgt.Seq, Phase: e.phase}
	if e.phase == PhaseHome {
		return f
	}
	w := e.s.Strand.Stroke.WidthPx
	var main, ring, pen []spec.Point
	tip := env.Anchor
	loosen := 0.0
	switch {
	case e.phase == PhaseSwing:
		main = e.swingPoints(env)
		f.U = e.u
		loosen = 1
	case e.k == kPoint || e.k == kPerch:
		main = e.mainPoints(env)
		f.U = e.u
		loosen = e.u
	case e.k == kLasso || (e.k == kRegion && e.lasso):
		main, ring = e.lassoPoints(env)
		f.U = e.kRev / e.ls.total(e.s)
		loosen = math.Min(1, e.kRev/float64(e.ls.mainSegs))
		if e.k == kRegion {
			pen = e.rg.visible(e.pen)
		}
	case e.k == kRegion:
		pen = e.rg.visible(e.pen)
		if len(pen) >= 2 {
			main = bowed(e.s, env.Anchor, pen[len(pen)-1], env.Centre, e.s.Strand.Region.LeadPoints-1)
			loosen = 1
		}
		f.U = e.pen / e.rg.total
		if e.phase == PhaseRegion && len(pen) >= 2 {
			f.Char = e.rg.char(e.s, e.pen, pen[len(pen)-1])
		}
	}
	// The leading element's end is the tip.
	switch {
	case len(ring) >= 2:
		tip = ring[len(ring)-1]
	case e.k == kRegion && !e.lasso && len(pen) >= 2:
		tip = pen[len(pen)-1]
	case len(main) >= 2:
		tip = main[len(main)-1]
	}
	push := func(pts []spec.Point, width, alpha float64) {
		if len(pts) >= 2 {
			f.Strokes = append(f.Strokes, Stroke{Pts: pts, Width: width, Alpha: alpha})
		}
	}
	push(main, w, 1)
	push(ring, w, 1)
	push(pen, w, 1)
	if e.phase == PhaseHold && e.burst >= 0 && e.burst < e.s.Strand.Burst.DurationS {
		for _, b := range burstLines(e.s, tip, e.burst) {
			push(b.Pts, b.Width, b.Alpha)
		}
	}
	if len(e.fades) > 0 {
		k := 1 - e.fadeT/(e.s.Strand.RetargetBlendMs/1000)
		for _, st := range e.fades {
			push(st.Pts, st.Width, st.Alpha*k)
		}
	}
	f.Tip = tip
	f.Loosen = e.s.Strand.TailLoosen.UnravelHold * clamp01(loosen)
	f.Bbox = e.grownBbox(f.Strokes)
	return f
}

// grownBbox pads the drawn bounds and keeps them from shrinking.
func (e *Engine) grownBbox(strokes []Stroke) Rect {
	x0, y0, x1, y1 := math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)
	for _, st := range strokes {
		for _, p := range st.Pts {
			x0, y0 = math.Min(x0, p.X), math.Min(y0, p.Y)
			x1, y1 = math.Max(x1, p.X), math.Max(y1, p.Y)
		}
	}
	if x0 > x1 {
		return e.bbox
	}
	pad := e.s.Strand.Stroke.BboxPadPx
	e.bbox = e.bbox.union(Rect{x0 - pad, y0 - pad, x1 - x0 + 2*pad, y1 - y0 + 2*pad})
	return e.bbox
}
