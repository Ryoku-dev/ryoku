// Package strand is the one motion source for Oni's thread: the line that
// leaves the character, reaches a target on screen, taps, holds, swings to
// the next target, perches on a window corner, throws a lasso round a
// control, pens a region and winds home. It is pure: it imports only spec,
// draws nothing and arms no timer, so the native surface, a later Ryoku
// renderer and the ledger replay all drive the same frames (the goldens in
// testdata/trajectories are that contract).
package strand

import (
	"math"
	"time"

	"ryoku-rashin/internal/oni/spec"
)

// Rect is an axis-aligned box in output-local logical px.
type Rect struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
	W float64 `json:"w"`
	H float64 `json:"h"`
}

// Mode is how a target is marked.
type Mode uint8

const (
	// ModePoint reaches the control and taps it (or perches on the window
	// frame when the target is a whole window or big panel).
	ModePoint Mode = iota
	// ModeLasso throws a ring round the control.
	ModeLasso
	// ModeRegion pens the named region first, then lassos the control.
	ModeRegion
	// ModePerch curls on the perched window's own frame: Box is the frame
	// (Frame, when set, wins), Corner names the corner, the size minimum
	// does not apply and session.skipCover does not skip it.
	ModePerch
)

// Target is one strand.target from the daemon. Box is the control; Frame is
// the window frame to perch on (nil when the target is a screen section);
// Region is the area a ModeRegion target pens. Quiet targets open no card,
// so the tip lands without the opening burst. Corner picks the perch corner
// (CornerTopLeft or CornerTopRight); empty or unknown takes the corner
// nearer the anchor.
type Target struct {
	Seq    uint64 `json:"seq"`
	Box    Rect   `json:"box"`
	Frame  *Rect  `json:"frame,omitempty"`
	Region *Rect  `json:"region,omitempty"`
	Mode   Mode   `json:"mode"`
	Quiet  bool   `json:"quiet,omitempty"`
	Corner string `json:"corner,omitempty"`
}

// Env is what the host knows each frame: the thread root on the character
// (pose-driven), the screen centre the path bows toward, the character's
// output, and the reduced-motion and battery-saver switches.
type Env struct {
	Anchor       spec.Point `json:"anchor"`
	Centre       spec.Point `json:"centre"`
	Screen       Rect       `json:"screen"`
	Reduced      bool       `json:"reduced"`
	BatterySaver bool       `json:"batterySaver"`
}

// Phase names what the thread is doing.
type Phase string

const (
	PhaseHome   Phase = "home"
	PhaseReach  Phase = "reach"
	PhaseTap    Phase = "tap"
	PhaseHold   Phase = "hold"
	PhaseSwing  Phase = "swing"
	PhaseCurl   Phase = "curl"
	PhaseLasso  Phase = "lasso"
	PhaseRegion Phase = "region"
	PhaseRewind Phase = "rewind"
)

// Skip reasons reported in Frame.Skipped (the strand.arrive wire values).
const (
	SkipCoversScreen = "covers-screen"
	SkipOffOutput    = "off-output"
)

// Stroke is one polyline. The painter draws the ink pass then the white
// pass for every stroke; Width is the white width in logical px.
type Stroke struct {
	Pts   []spec.Point
	Width float64
	Alpha float64
}

// CharPose overrides the character's position and scale; it is set only
// while a region pen rides the tip.
type CharPose struct {
	Pos   spec.Point
	Scale float64
}

// Frame is one strand frame. U is the leading element's progress: the main
// thread's extension for point and perch targets, the throw's reveal for a
// lasso and the pen's share of its path while a region is drawn. Strokes
// hold the main thread first, then the ring, the pen and the burst, then any
// fading strokes left by a swing. Bbox is the padded bounds of everything
// drawn since the thread left home, monotonic until Home. Arrived is true on
// exactly the Step that lands the tip (or reports a skip, with Skipped set).
type Frame struct {
	Seq     uint64
	Phase   Phase
	U       float64
	Strokes []Stroke
	Tip     spec.Point
	Char    *CharPose
	Loosen  float64
	Bbox    Rect
	Arrived bool
	Skipped string
}

// Approach is the unit direction the leading stroke last travelled into its
// tip (the opaque stroke ending at Tip), for BubbleRect; zero when nothing
// is drawn.
func (f Frame) Approach() spec.Point {
	var lead []spec.Point
	for _, st := range f.Strokes {
		if st.Alpha == 1 && len(st.Pts) >= 2 && st.Pts[len(st.Pts)-1] == f.Tip {
			lead = st.Pts
		}
	}
	for i := len(lead) - 2; i >= 0; i-- {
		d := sub(f.Tip, lead[i])
		if l := math.Hypot(d.X, d.Y); l > 1e-9 {
			return spec.Point{X: d.X / l, Y: d.Y / l}
		}
	}
	return spec.Point{}
}

// Tick is one Step's result, with motion.Tick's contract: Redraw is set when
// Frame differs from the last committed frame; Next is the delay until the
// next Step at FPS; Stop means no timer is needed until the next event
// (Target, Retract or a reduced-motion change), and Next and FPS are then
// zero.
type Tick struct {
	Frame  Frame
	Redraw bool
	Next   time.Duration
	Stop   bool
	FPS    int
}

func (r Rect) centre() spec.Point { return spec.Point{X: r.X + r.W/2, Y: r.Y + r.H/2} }

func (r Rect) grow(d float64) Rect { return Rect{r.X - d, r.Y - d, r.W + 2*d, r.H + 2*d} }

func (r Rect) empty() bool { return !(r.W > 0) || !(r.H > 0) }

func (r Rect) contains(p spec.Point) bool {
	return p.X >= r.X && p.X <= r.X+r.W && p.Y >= r.Y && p.Y <= r.Y+r.H
}

// overlaps is true when the interiors intersect; touching edges do not.
func (r Rect) overlaps(o Rect) bool {
	return r.X < o.X+o.W && o.X < r.X+r.W && r.Y < o.Y+o.H && o.Y < r.Y+r.H
}

func (r Rect) union(o Rect) Rect {
	if r.empty() {
		return o
	}
	if o.empty() {
		return r
	}
	x0, y0 := math.Min(r.X, o.X), math.Min(r.Y, o.Y)
	x1, y1 := math.Max(r.X+r.W, o.X+o.W), math.Max(r.Y+r.H, o.Y+o.H)
	return Rect{x0, y0, x1 - x0, y1 - y0}
}

func add(a, b spec.Point) spec.Point           { return spec.Point{X: a.X + b.X, Y: a.Y + b.Y} }
func sub(a, b spec.Point) spec.Point           { return spec.Point{X: a.X - b.X, Y: a.Y - b.Y} }
func scale(a spec.Point, k float64) spec.Point { return spec.Point{X: a.X * k, Y: a.Y * k} }
func lerp(a, b spec.Point, w float64) spec.Point {
	return spec.Point{X: a.X + (b.X-a.X)*w, Y: a.Y + (b.Y-a.Y)*w}
}
func dist(a, b spec.Point) float64 { return math.Hypot(a.X-b.X, a.Y-b.Y) }

func unit(p spec.Point) spec.Point {
	if l := math.Hypot(p.X, p.Y); l > 1e-12 {
		return spec.Point{X: p.X / l, Y: p.Y / l}
	}
	return spec.Point{}
}

func clamp01(x float64) float64 { return math.Min(1, math.Max(0, x)) }
