package spec

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"
)

const docPath = "../../../release/docs/oni-animation-spec.md"

func mustLoad(t *testing.T) *OniSpec {
	t.Helper()
	s, err := Load()
	if err != nil {
		t.Fatalf("embedded spec rejected: %v", err)
	}
	return s
}

func TestEveryStateGestureAndMotifResolves(t *testing.T) {
	s := mustLoad(t)
	if len(StateIDs) != 9 || len(GestureIDs) != 10 {
		t.Fatalf("want 9 states and 10 gestures, have %d and %d", len(StateIDs), len(GestureIDs))
	}
	for _, id := range StateIDs {
		st, ok := s.State(id)
		if !ok {
			t.Fatalf("state %q does not resolve", id)
		}
		if !(st.Scale > 0 && st.Tempo > 0 && st.Hover[0] > 0) {
			t.Errorf("state %q resolves to an unusable pose %+v", id, st)
		}
		hex, ok := s.StateColour(id)
		if !ok || !hexColour.MatchString(hex) {
			t.Errorf("state %q has no colour (%q)", id, hex)
		}
	}
	for _, id := range GestureIDs {
		g, ok := s.Gesture(id)
		if !ok {
			t.Fatalf("gesture %q does not resolve", id)
		}
		if !(g.DurationS > 0) || len(g.Motif.Notes) == 0 {
			t.Errorf("gesture %q resolves without duration or motif: %+v", id, g)
		}
	}
	for _, id := range MotifIDs {
		if m, ok := s.Motif(id); !ok || len(m.Notes) == 0 || !(m.LengthS > 0) {
			t.Errorf("motif %q does not resolve to a playable motif", id)
		}
	}
	if _, ok := s.State("dancing"); ok {
		t.Error("an unknown state resolved")
	}
	if _, ok := s.Gesture("reach"); ok {
		t.Error("a move motif resolved as a gesture")
	}
	if _, ok := s.StateColour("dancing"); ok {
		t.Error("an unknown state resolved a colour")
	}
	sleepHex, _ := s.StateColour("sleeping")
	idleHex, _ := s.StateColour("idle")
	workHex, _ := s.StateColour("working")
	thinkHex, _ := s.StateColour("thinking")
	if sleepHex != idleHex || workHex != thinkHex {
		t.Error("sleeping must wear idle white and working must wear thinking amber")
	}
}

func TestFrameTiersAndBatterySaver(t *testing.T) {
	s := mustLoad(t)
	for _, c := range []struct {
		activity string
		battery  bool
		want     int
	}{
		{"asleep", false, 20}, {"asleep", true, 20},
		{"idle", false, 30}, {"idle", true, 30},
		{"breathing", false, 60}, {"breathing", true, 30},
		{"motion", false, 60}, {"motion", true, 60},
	} {
		got, ok := s.TargetFPS(c.activity, c.battery)
		if !ok || got != c.want {
			t.Errorf("TargetFPS(%q, battery=%v) = %d, %v; want %d", c.activity, c.battery, got, ok, c.want)
		}
	}
	asleep, _ := s.TargetFPS("asleep", false)
	for _, a := range ActivityIDs {
		normal, _ := s.TargetFPS(a, false)
		saver, _ := s.TargetFPS(a, true)
		if saver > normal || normal < asleep {
			t.Errorf("%s: battery saver %d vs normal %d vs asleep %d", a, saver, normal, asleep)
		}
	}
	if _, ok := s.TargetFPS("sprinting", false); ok {
		t.Error("an unknown activity resolved a frame rate")
	}
}

func TestGestureKeyframesShapeEveryGesture(t *testing.T) {
	s := mustLoad(t)
	for _, id := range GestureIDs {
		g, ok := s.Gesture(id)
		if !ok {
			t.Fatalf("gesture %q does not resolve", id)
		}
		moved := false
		for _, ch := range KeyframeChannels {
			if v := g.Keyframes.Sample(ch, 0); v != 0 {
				t.Errorf("%s.%s starts at %v, want 0", id, ch, v)
			}
			if v := g.Keyframes.Sample(ch, 1); v != 0 {
				t.Errorf("%s.%s ends at %v, want 0", id, ch, v)
			}
			for i := 1; i < 100; i++ {
				if g.Keyframes.Sample(ch, float64(i)/100) != 0 {
					moved = true
				}
			}
		}
		if !moved {
			t.Errorf("gesture %q has no visible motion", id)
		}
	}
	poke := s.Gestures.Keyframes["poke"]
	for i, at := range poke["at"] {
		if got := poke.Sample("leanX", at); i < len(poke["at"])-1 && i > 0 && math.Abs(got-poke["leanX"][i]) > 1e-12 {
			t.Errorf("poke leanX at key %v = %v, want the key value %v", at, got, poke["leanX"][i])
		}
	}
	if v := poke.Sample("bright", 0.3); v != 0 {
		t.Errorf("absent channel sampled %v", v)
	}
	if v := poke.Sample("at", 0.3); v != 0 {
		t.Errorf("the progress row sampled as a channel: %v", v)
	}
	shake := s.Gestures.Keyframes["error"]
	if got := shake.Sample("leanX", shake["at"][1]); got != s.Gestures.ErrorShakeLeanX[0] {
		t.Errorf("error shake first key %v, want %v", got, s.Gestures.ErrorShakeLeanX[0])
	}
}

func TestPlacementSizes(t *testing.T) {
	s := mustLoad(t)
	for id, want := range map[string]float64{"small": 0.7, "normal": 1, "big": 1.35} {
		if got, ok := s.Size(id); !ok || got != want {
			t.Errorf("Size(%q) = %v, %v; want %v", id, got, ok, want)
		}
	}
	if _, ok := s.Size("huge"); ok {
		t.Error("an unknown size resolved")
	}
	if s.Placement.CornerMarginPx != 40 || s.Placement.DragThresholdPx != 4 {
		t.Errorf("placement margin %d threshold %d", s.Placement.CornerMarginPx, s.Placement.DragThresholdPx)
	}
	if !s.Frames.AsleepStopsAfterCommit {
		t.Error("asleep must stop after the sleep commit")
	}
}

func TestBezierEase(t *testing.T) {
	curves := []Ease{{0.45, 0, 0.2, 1}, {0.16, 1, 0.3, 1}, {0.35, 0, 0.15, 1}, {0.22, 1.12, 0.36, 1}, {0.4, 0, 1, 1}}
	for _, e := range curves {
		for _, c := range []struct{ t, want float64 }{{0, 0}, {1, 1}, {-3, 0}, {4, 1}, {math.NaN(), 0}} {
			if got := e.At(c.t); got != c.want {
				t.Errorf("%v at %v = %v, want %v", e, c.t, got, c.want)
			}
		}
	}
	for i := range 41 {
		x := float64(i) / 40
		if got := BezierEase(0, 0, 1, 1, x); math.Abs(got-x) > 1e-9 {
			t.Fatalf("linear curve at %v = %v", x, got)
		}
		if got := BezierEase(1.0/3, 1.0/3, 2.0/3, 2.0/3, x); math.Abs(got-x) > 1e-9 {
			t.Fatalf("thirds curve at %v = %v", x, got)
		}
	}
	if got := BezierEase(0.42, 0, 0.58, 1, 0.5); math.Abs(got-0.5) > 1e-9 {
		t.Errorf("symmetric curve midpoint = %v", got)
	}
	if BezierEase(0.22, 1.12, 0.36, 1, 0.7) <= 1 {
		t.Error("the overshooting open curve must pass above 1")
	}
}

func TestReachAndRewindTiming(t *testing.T) {
	if got := Reach(0.740); got != 1 {
		t.Fatalf("Reach(0.740) = %v, want 1", got)
	}
	if got := Reach(0); got != 0 {
		t.Fatalf("Reach(0) = %v", got)
	}
	if got := Reach(3); got != 1 {
		t.Fatalf("Reach past the end = %v", got)
	}
	if m := Reach(0.37); !(m > 0 && m < 1) {
		t.Fatalf("Reach mid-flight = %v", m)
	}
	prev := 0.0
	for ms := range 741 {
		u := Reach(float64(ms) / 1000)
		if u < prev-1e-12 || u < 0 || u > 1 {
			t.Fatalf("Reach not monotone in [0,1] at %d ms: %v after %v", ms, u, prev)
		}
		prev = u
	}
	if Reach(0.739) >= 1 {
		t.Fatal("Reach landed before 0.740 s")
	}

	if got := Rewind(0); got != 1 {
		t.Fatalf("Rewind(0) = %v, want 1", got)
	}
	if got := Rewind(0.520); got != 0 {
		t.Fatalf("Rewind(0.520) = %v, want 0", got)
	}
	if got := Rewind(9); got != 0 {
		t.Fatalf("Rewind past the end = %v", got)
	}
	prev = 1
	for ms := range 521 {
		u := Rewind(float64(ms) / 1000)
		if u > prev+1e-12 || u < 0 || u > 1 {
			t.Fatalf("Rewind not monotone in [0,1] at %d ms: %v after %v", ms, u, prev)
		}
		prev = u
	}
	if Rewind(0.519) <= 0 {
		t.Fatal("Rewind finished before 0.520 s")
	}

	for _, c := range []struct{ px, want float64 }{{0, 0.2}, {30, 0.2}, {150, 0.5}, {222, 0.74}, {1e6, 0.74}} {
		if got := SurfaceReachS(c.px); math.Abs(got-c.want) > 1e-12 {
			t.Errorf("SurfaceReachS(%v) = %v, want %v", c.px, got, c.want)
		}
	}
}

func TestReachIgnoresMutatedLoadedCopy(t *testing.T) {
	s := mustLoad(t)
	s.Strand.Reach.DurationS = 10
	s.Strand.BowedPath.Points = 3
	if Reach(0.74) != 1 || len(BowedPath(Point{}, Point{X: 100}, Point{Y: 50})) != 97 {
		t.Fatal("mutating a loaded spec leaked into the pure functions")
	}
}

func TestBowedPathOverDistanceSweep(t *testing.T) {
	var capped float64
	for _, d := range []float64{0.5, 1, 10, 50, 100, 250, 399, 400, 401, 800, 2000, 10000} {
		for _, ang := range []float64{0, 0.7, math.Pi / 2, 2.5, -2} {
			ux, uy := math.Cos(ang), math.Sin(ang)
			nx, ny := -uy, ux
			a := Point{X: 120, Y: 340}
			b := Point{X: a.X + d*ux, Y: a.Y + d*uy}
			mid := Point{X: (a.X + b.X) / 2, Y: (a.Y + b.Y) / 2}
			centre := Point{X: mid.X + 500*nx, Y: mid.Y + 500*ny}

			pts := BowedPath(a, b, centre)
			if len(pts) != 97 {
				t.Fatalf("d=%v: %d points, want 97", d, len(pts))
			}
			if pts[0] != a || pts[96] != b {
				t.Fatalf("d=%v: path does not start at a and end at b", d)
			}
			bow := math.Min(100, d*0.25) * 0.20
			perp := func(p Point) float64 { return (p.X-mid.X)*nx + (p.Y-mid.Y)*ny }
			along := func(p Point) float64 { return (p.X-mid.X)*ux + (p.Y-mid.Y)*uy }
			// The cubic's midpoint sits 3/4 of the control offset off the chord.
			if got := perp(pts[48]); math.Abs(got-0.75*bow) > 1e-9 {
				t.Fatalf("d=%v ang=%v: mid offset %v, want %v", d, ang, got, 0.75*bow)
			}
			if got := along(pts[48]); math.Abs(got) > 1e-9 {
				t.Fatalf("d=%v: bow is not symmetric (%v along the chord)", d, got)
			}
			for i, p := range pts {
				if o := perp(p); o < -1e-9 || o > bow+1e-9 {
					t.Fatalf("d=%v: point %d leaves the bow band (%v of %v)", d, i, o, bow)
				}
			}
			if d == 10000 && ang == 0 {
				capped = perp(pts[48])
			}
		}
	}
	if math.Abs(capped-0.75*100*0.20) > 1e-9 {
		t.Fatalf("long threads must cap the bow, got %v", capped)
	}
}

func TestBowedPathDegenerateInputs(t *testing.T) {
	a, b := Point{X: 10, Y: 10}, Point{X: 210, Y: 10}
	for i, p := range BowedPath(a, b, Point{X: 110, Y: 10}) {
		if math.Abs(p.Y-10) > 1e-9 {
			t.Fatalf("centre on the chord midpoint must not bow, point %d at %v", i, p)
		}
	}
	for i, p := range BowedPath(a, a, Point{X: 500, Y: 500}) {
		if math.Abs(p.X-a.X) > 1e-9 || math.Abs(p.Y-a.Y) > 1e-9 {
			t.Fatalf("zero-length thread moved, point %d at %v", i, p)
		}
	}
}

func namedSprings(s *OniSpec) map[string]Spring {
	out := map[string]Spring{}
	walkValues(reflect.ValueOf(*s), "", func(p string, v reflect.Value) {
		if v.Type() == springType {
			out[p] = v.Interface().(Spring)
		}
	})
	return out
}

func TestSpringsConvergeAtDisplayRates(t *testing.T) {
	s := mustLoad(t)
	springs := namedSprings(s)
	if len(springs) < 20 {
		t.Fatalf("found %d springs, want shape, pose and spin-stop springs", len(springs))
	}
	for _, hz := range []float64{20, 60, 165} {
		dt := 1 / hz
		for name, sp := range springs {
			for _, start := range []struct{ x, v, target float64 }{{0, 0, 1}, {1, 40, -0.3}, {0.5, -200, 0.5}} {
				x, v := start.x, start.v
				settled := false
				for step := range int(30 * hz) {
					x, v = SpringStep(x, v, start.target, sp, dt)
					if math.IsNaN(x) || math.IsNaN(v) || math.IsInf(x, 0) || math.IsInf(v, 0) {
						t.Fatalf("%s at %v Hz went non-finite at step %d", name, hz, step)
					}
					if x == start.target && v == 0 {
						settled = true
						break
					}
				}
				if !settled {
					t.Fatalf("%s at %v Hz did not snap to %v (x=%v v=%v)", name, hz, start.target, x, v)
				}
				if nx, nv := SpringStep(x, v, start.target, sp, dt); nx != start.target || nv != 0 {
					t.Fatalf("%s at %v Hz left rest after settling", name, hz)
				}
			}
		}
	}
}

func TestSpringStepCapsDtAndGuards(t *testing.T) {
	sp := mustLoad(t).Springs.Pose.Scale
	capX, capV := SpringStep(0, 0, 1, sp, 0.05)
	for _, dt := range []float64{0.06, 1, 30} {
		if x, v := SpringStep(0, 0, 1, sp, dt); x != capX || v != capV {
			t.Errorf("dt %v not capped to 50 ms", dt)
		}
	}
	if x, _ := SpringStep(0, 0, 1, sp, 0.04); x == capX {
		t.Error("a 40 ms frame was treated like the cap")
	}
	for _, dt := range []float64{0, -0.016, math.NaN()} {
		if x, v := SpringStep(0.3, 2, 1, sp, dt); x != 0.3 || v != 2 {
			t.Errorf("dt %v moved the spring", dt)
		}
	}
	if x, v := SpringStep(math.NaN(), 0, 0.7, sp, 0.016); x != 0.7 || v != 0 {
		t.Errorf("non-finite state not landed: %v %v", x, v)
	}
	if x, v := SpringStep(0, math.Inf(1), 0.7, sp, 0.016); x != 0.7 || v != 0 {
		t.Errorf("infinite velocity not landed: %v %v", x, v)
	}
}

func TestLightDesktopHysteresis(t *testing.T) {
	ld := mustLoad(t).LightDesktop
	on := false
	for i, c := range []struct {
		luma float64
		want bool
	}{{0.55, false}, {0.62, false}, {0.7, true}, {0.55, true}, {0.5, true}, {0.49, false}, {0.6, false}, {0.63, true}} {
		on = ld.Outlined(on, c.luma)
		if on != c.want {
			t.Fatalf("step %d luma %v: outlined=%v, want %v", i, c.luma, on, c.want)
		}
	}
}

func decodeEmbedded(t *testing.T) map[string]any {
	t.Helper()
	dec := json.NewDecoder(bytes.NewReader(embedded))
	dec.UseNumber()
	var root map[string]any
	if err := dec.Decode(&root); err != nil {
		t.Fatal(err)
	}
	return root
}

func node(root map[string]any, path string) map[string]any {
	for _, p := range strings.Split(path, ".") {
		root = root[p].(map[string]any)
	}
	return root
}

func edited(t *testing.T, edit func(root map[string]any)) []byte {
	t.Helper()
	root := decodeEmbedded(t)
	edit(root)
	out, err := json.Marshal(root)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func replaceOnce(t *testing.T, s, old, repl string) string {
	t.Helper()
	if n := strings.Count(s, old); n != 1 {
		t.Fatalf("%q occurs %d times, want exactly once", old, n)
	}
	return strings.Replace(s, old, repl, 1)
}

func TestParseRoundTripAccepts(t *testing.T) {
	if _, err := Parse(edited(t, func(map[string]any) {})); err != nil {
		t.Fatalf("re-encoded embedded spec rejected: %v", err)
	}
}

func TestStrictLoadRejects(t *testing.T) {
	raw := string(embedded)
	num := func(s string) json.Number { return json.Number(s) }
	cases := []struct {
		name string
		data []byte
		want string
	}{
		{"unknown top-level key", edited(t, func(r map[string]any) { r["bonus"] = num("1") }), "bonus: unknown key"},
		{"unknown nested key", edited(t, func(r map[string]any) { node(r, "strand.bowedPath")["bend"] = num("1") }), "strand.bowedPath.bend: unknown key"},
		{"missing field", edited(t, func(r map[string]any) { delete(node(r, "strand.tap"), "bobPx") }), "strand.tap.bobPx: missing key"},
		{"NaN literal", []byte(replaceOnce(t, raw, `"durationS": 0.74`, `"durationS": NaN`)), "invalid character"},
		{"infinite number", []byte(replaceOnce(t, raw, `"idleMs": 90000`, `"idleMs": 1e999`)), "sleep.idleMs: number 1e999 is not finite"},
		{"negative reach duration", edited(t, func(r map[string]any) { node(r, "strand.reach")["durationS"] = num("-0.74") }), "strand.reach.durationS: negative"},
		{"negative gesture duration", edited(t, func(r map[string]any) { node(r, "gestures.durationS")["wake"] = num("-0.82") }), "gestures.durationS.wake: negative"},
		{"negative sleep delay", edited(t, func(r map[string]any) { node(r, "sleep")["idleMs"] = num("-1") }), "sleep.idleMs: negative"},
		{"negative unsigned range", edited(t, func(r map[string]any) { node(r, "springs.poseRanges")["scale"] = []any{num("-0.7"), num("1.3")} }), "springs.poseRanges.scale[0]: negative"},
		{"zero gesture duration", edited(t, func(r map[string]any) { node(r, "gestures.durationS")["wake"] = num("0") }), "gestures.durationS.wake: must be positive"},
		{"missing state", edited(t, func(r map[string]any) { delete(node(r, "states"), "reading") }), "states: missing [reading]"},
		{"extra state", edited(t, func(r map[string]any) { node(r, "states")["dancing"] = node(r, "states")["idle"] }), "unknown [dancing]"},
		{"missing gesture", edited(t, func(r map[string]any) { delete(node(r, "gestures.durationS"), "gulp") }), "gestures.durationS: missing [gulp]"},
		{"missing motif", edited(t, func(r map[string]any) { delete(node(r, "sound.motifs"), "intro") }), "sound.motifs: missing [intro]"},
		{"missing cue", edited(t, func(r map[string]any) { delete(node(r, "sound.cues.items"), "tick") }), "sound.cues.items: missing [tick]"},
		{"short spring", edited(t, func(r map[string]any) { node(r, "springs.shape")["flat"] = []any{num("120")} }), "springs.shape.flat: want 2 elements"},
		{"string version", edited(t, func(r map[string]any) { r["version"] = "1" }), "version: want a number"},
		{"future version", edited(t, func(r map[string]any) { r["version"] = num("2") }), "version: unsupported 2"},
		{"fractional count", edited(t, func(r map[string]any) { node(r, "strand.bowedPath")["points"] = num("97.5") }), "strand.bowedPath.points: want an integer"},
		{"point count drift", edited(t, func(r map[string]any) { node(r, "strand.bowedPath")["points"] = num("98") }), "points must equal segments+1"},
		{"unknown state colour", edited(t, func(r map[string]any) { node(r, "colour.stateColour")["ready"] = "violet" }), "colour.stateColour.ready: unknown palette entry"},
		{"bad palette colour", edited(t, func(r map[string]any) { node(r, "colour.palette")["done"] = "green" }), "colour.palette.done: colour is not #RRGGBB"},
		{"two shape weights", edited(t, func(r map[string]any) { node(r, "states.idle")["face"] = num("1") }), "states.idle: shape weights must sum to 1"},
		{"zero damping", edited(t, func(r map[string]any) { node(r, "springs.pose")["tempo"] = []any{num("100"), num("0")} }), "springs.pose.tempo: spring needs positive"},
		{"battery raises rate", edited(t, func(r map[string]any) { node(r, "frames.batteryTier")["idle"] = "active" }), "frames.batteryTier.idle: battery saver may not raise"},
		{"unknown tier", edited(t, func(r map[string]any) { node(r, "frames.tier")["motion"] = "turbo" }), "frames.tier.motion: unknown tier"},
		{"inverted luma thresholds", edited(t, func(r map[string]any) { node(r, "lightDesktop")["onAbove"] = num("0.4") }), "lightDesktop: want 0 <= offBelow < onAbove"},
		{"looping reduced motion", edited(t, func(r map[string]any) { node(r, "reducedMotion")["loop"] = true }), "reducedMotion: must not loop"},
		{"missing gesture keyframes", edited(t, func(r map[string]any) { delete(node(r, "gestures.keyframes"), "gulp") }), "gestures.keyframes: missing [gulp]"},
		{"keyframe not returning to rest", edited(t, func(r map[string]any) {
			node(r, "gestures.keyframes.settle")["scale"] = []any{num("0"), num("-0.02"), num("0.01"), num("0.02")}
		}), "gestures.keyframes.settle.scale: must start and end at 0"},
		{"keyframe length drift", edited(t, func(r map[string]any) {
			node(r, "gestures.keyframes.poke")["squash"] = []any{num("0"), num("0.12"), num("0")}
		}), "gestures.keyframes.poke.squash: want 4 keys, got 3"},
		{"unknown keyframe channel", edited(t, func(r map[string]any) {
			node(r, "gestures.keyframes.poke")["wobble"] = []any{num("0"), num("0.1"), num("0"), num("0")}
		}), "gestures.keyframes.poke.wobble: unknown channel"},
		{"keyframes out of order", edited(t, func(r map[string]any) {
			node(r, "gestures.keyframes.arrive")["at"] = []any{num("0"), num("0.7"), num("0.3"), num("1")}
		}), "gestures.keyframes.arrive.at: keys must strictly increase"},
		{"error shake drift", edited(t, func(r map[string]any) {
			node(r, "gestures")["errorShakeLeanX"] = []any{num("-0.2"), num("0.12"), num("-0.05")}
		}), "gestures.keyframes.error.leanX: interior keys must equal"},
		{"missing size", edited(t, func(r map[string]any) { delete(node(r, "placement.sizes"), "big") }), "placement.sizes: missing [big]"},
		{"sizes out of order", edited(t, func(r map[string]any) { node(r, "placement.sizes")["small"] = num("1.2") }), "placement.sizes: want 0 < small < normal < big"},
		{"fractional margin", edited(t, func(r map[string]any) { node(r, "placement")["cornerMarginPx"] = num("40.5") }), "placement.cornerMarginPx: want an integer"},
		{"missing anchor", edited(t, func(r map[string]any) { delete(node(r, "strand"), "anchor") }), "strand.anchor: missing key"},
		{"anchor off the art", edited(t, func(r map[string]any) { node(r, "strand.anchor")["y"] = num("1.2") }), "strand.anchor: x and y must lie in [0,1]"},
		{"anchor overfollows", edited(t, func(r map[string]any) { node(r, "strand.anchor.follow")["lean"] = num("1.5") }), "strand.anchor.follow.lean: weight must lie in [0,1]"},
		{"missing retarget", edited(t, func(r map[string]any) { delete(node(r, "strand"), "retarget") }), "strand.retarget: missing key"},
		{"retarget from full reach", edited(t, func(r map[string]any) { node(r, "strand.retarget")["reachFrom"] = num("1") }), "strand.retarget.reachFrom: want 0 < reachFrom < 1"},
		{"missing wobble seed", edited(t, func(r map[string]any) { delete(node(r, "strand.region"), "wobbleSeed") }), "strand.region.wobbleSeed: missing key"},
		{"fractional wobble seed", edited(t, func(r map[string]any) { node(r, "strand.region")["wobbleSeed"] = num("9.5") }), "strand.region.wobbleSeed: want an integer"},
		{"wide wobble seed", edited(t, func(r map[string]any) { node(r, "strand.region")["wobbleSeed"] = num("4294967296") }), "strand.region.wobbleSeed: must fit in 32 bits"},
		{"missing arrival timeout", edited(t, func(r map[string]any) { delete(node(r, "session"), "arrivalTimeoutMs") }), "session.arrivalTimeoutMs: missing key"},
		{"arrival timeout cuts a landing", edited(t, func(r map[string]any) { node(r, "session")["arrivalTimeoutMs"] = num("3000") }), "session.arrivalTimeoutMs: must exceed the slowest landing"},
		{"missing perch rules", edited(t, func(r map[string]any) { delete(r, "perch") }), "perch: missing key"},
		{"missing perch reattach", edited(t, func(r map[string]any) { delete(node(r, "perch"), "reattach") }), "perch.reattach: missing key"},
		{"unknown perch key", edited(t, func(r map[string]any) { node(r, "perch.seat")["side"] = "left" }), "perch.seat.side: unknown key"},
		{"negative seat lift", edited(t, func(r map[string]any) { node(r, "perch.seat")["liftPx"] = num("-6") }), "perch.seat.liftPx: negative"},
		{"seat on the curl", edited(t, func(r map[string]any) { node(r, "perch.seat")["gapPx"] = num("22") }), "perch.seat.gapPx: must clear the perch curl"},
		{"zero snap distance", edited(t, func(r map[string]any) { node(r, "perch.reattach")["snapPx"] = num("0") }), "perch.reattach.snapPx: must be positive"},
		{"zero settle time", edited(t, func(r map[string]any) { node(r, "perch.reattach")["settleMs"] = num("0") }), "perch.reattach.settleMs: must be positive"},
		{"invisible highlight", edited(t, func(r map[string]any) { node(r, "perch.highlight")["widthPx"] = num("0") }), "perch.highlight.widthPx: must be positive"},
		{"highlight over the window", edited(t, func(r map[string]any) { node(r, "perch.highlight")["padPx"] = num("2") }), "perch.highlight.padPx: the outline must stay outside the window"},
		{"chip with no room", edited(t, func(r map[string]any) { node(r, "perch.chip")["maxWidthPx"] = num("27") }), "perch.chip.maxWidthPx: leaves no room for text"},
		{"chip without text", edited(t, func(r map[string]any) { node(r, "perch.chip")["textPx"] = num("0") }), "perch.chip.textPx: must be positive"},
		{"missing sound events", edited(t, func(r map[string]any) { delete(node(r, "sound"), "events") }), "sound.events: missing key"},
		{"missing sound event", edited(t, func(r map[string]any) { delete(node(r, "sound.events"), "strand-arrive") }), "sound.events: missing [strand-arrive]"},
		{"extra sound event", edited(t, func(r map[string]any) { node(r, "sound.events")["strand-wiggle"] = "reach" }), "unknown [strand-wiggle]"},
		{"event plays unknown motif", edited(t, func(r map[string]any) { node(r, "sound.events")["strand-arrive"] = "fanfare" }), "sound.events.strand-arrive: unknown motif"},
		{"motif nothing plays", edited(t, func(r map[string]any) { node(r, "sound.events")["intro-trace"] = "bubble" }), "sound.motifs.intro: no sound.events entry plays it"},
		{"missing scale", edited(t, func(r map[string]any) { delete(node(r, "sound"), "scale") }), "sound.scale: missing key"},
		{"empty scale", edited(t, func(r map[string]any) { node(r, "sound")["scale"] = []any{} }), "sound.scale: empty"},
		{"pitch class past B", edited(t, func(r map[string]any) { node(r, "sound")["scale"] = []any{num("0"), num("12")} }), "sound.scale[1]: pitch class 12 outside 0-11"},
		{"pitch class twice", edited(t, func(r map[string]any) { node(r, "sound")["scale"] = []any{num("0"), num("0")} }), "sound.scale[1]: pitch class 0 listed twice"},
		{"motif note off the scale", edited(t, func(r map[string]any) {
			node(r, "sound.motifs.acknowledge")["notes"] = []any{num("72"), num("73")}
		}), "sound.motifs.acknowledge.notes: MIDI note 73 is outside sound.scale"},
		{"cue note off the scale", edited(t, func(r map[string]any) { node(r, "sound.cues.items.tick")["notes"] = []any{num("85")} }), "sound.cues.items.tick.notes: MIDI note 85 is outside sound.scale"},
		{"duplicate key", []byte(replaceOnce(t, raw, `"version": 1,`, `"version": 1, "version": 1,`)), "version: duplicate key"},
		{"trailing data", append(bytes.Clone(embedded), []byte(`{}`)...), "trailing data"},
		{"not an object", []byte(`[1, 2]`), "(root): want an object"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s, err := Parse(c.data)
			if err == nil {
				t.Fatalf("accepted; got spec version %d", s.Version)
			}
			if !strings.Contains(err.Error(), c.want) {
				t.Fatalf("error %q does not name %q", err, c.want)
			}
		})
	}
}

func TestStrandAnchorRetargetAndArrivalResolve(t *testing.T) {
	s := mustLoad(t)
	an := s.Strand.Anchor
	if !(an.X > 0 && an.X < 1 && an.Y > 0.5 && an.Y < 1) {
		t.Errorf("anchor %+v is not on the lower half of the art", an)
	}
	if f := an.Follow; f.Scale != 1 || f.Squash != 1 || f.Lean != 1 || f.Lift != 1 || f.Hover != 1 {
		t.Errorf("anchor must ride every pose transform with the art: %+v", f)
	}
	if rt := s.Strand.Retarget; rt.ReachFrom != 0.35 || rt.Ease != s.Strand.Reach.Ease {
		t.Errorf("retarget = %+v, want reachFrom 0.35 on the reach ease", rt)
	}
	if s.Strand.Region.WobbleSeed != 9 {
		t.Errorf("wobble seed = %d, want 9", s.Strand.Region.WobbleSeed)
	}
	if got := s.Session.ArrivalTimeoutMs; got != 10000 || !(got > s.slowestLandingMs()) {
		t.Errorf("arrival timeout %v must be 10000 ms and exceed the slowest landing %v ms", got, s.slowestLandingMs())
	}
}

func TestPerchRulesResolve(t *testing.T) {
	s := mustLoad(t)
	if got := s.CurlReachPx(); math.Abs(got-22.4) > 1e-9 {
		t.Errorf("curl reach = %v px, want radius 18 + half the 3.2 px core + 2.8 px ink", got)
	}
	p := s.Perch
	if !(p.Seat.GapPx > s.CurlReachPx()) {
		t.Errorf("seat gap %v does not clear the curl (%v px)", p.Seat.GapPx, s.CurlReachPx())
	}
	if !(p.Seat.MarginPx > 0 && p.Seat.LiftPx > 0) {
		t.Errorf("seat %+v must keep a margin and lift", p.Seat)
	}
	// A move the reattach snaps must be shorter than any window the thread
	// perches on by size, or a window could slide its own width unseen.
	if !(p.Reattach.SnapPx < s.Strand.Perch.MinHeightPx && p.Reattach.SnapPx < s.Strand.Perch.MinWidthPx) {
		t.Errorf("reattach snap %v px reaches a whole perchable window", p.Reattach.SnapPx)
	}
	if hl := p.Highlight; hl.PadPx < hl.WidthPx/2+hl.RimPx || !(hl.FadeInS > 0 && hl.FadeOutS > 0) {
		t.Errorf("highlight %+v covers the window or snaps on and off", hl)
	}
	if ch := p.Chip; !(ch.MaxWidthPx > 2*ch.PadXPx+ch.TextPx) || !(ch.GapPx > 0) {
		t.Errorf("chip %+v has no room for its text or touches the character", ch)
	}
}

func TestEveryEventPlaysAnOriginalMotif(t *testing.T) {
	s := mustLoad(t)
	played := map[string]bool{}
	for _, ev := range EventIDs {
		id, ok := s.Sound.Events[ev]
		if !ok {
			t.Fatalf("event %q has no motif", ev)
		}
		if m, ok := s.Motif(id); !ok || len(m.Notes) == 0 {
			t.Fatalf("event %q plays %q, which does not resolve", ev, id)
		}
		played[id] = true
	}
	for _, g := range GestureIDs {
		if s.Sound.Events["gesture-"+g] != g {
			t.Errorf("gesture %q must play its own motif, plays %q", g, s.Sound.Events["gesture-"+g])
		}
	}
	seen := map[string]string{}
	for _, id := range MotifIDs {
		if !played[id] {
			t.Errorf("motif %q is never played", id)
		}
		m, _ := s.Motif(id)
		key := fmt.Sprint(m.Notes)
		if other, dup := seen[key]; dup {
			t.Errorf("motifs %q and %q share the notes %s", other, id, key)
		}
		seen[key] = id
		for _, n := range m.Notes {
			// The synthesiser's pitch proof separates semitones from G3 up,
			// below the lowpass.
			if n < 55 || n > 96 || !slices.Contains(s.Sound.Scale, n%12) {
				t.Errorf("motif %q note %d is outside G3-C7 or the scale", id, n)
			}
		}
	}
	for _, id := range CueIDs {
		for _, n := range s.Sound.Cues.Items[id].Notes {
			if n < 55 || !slices.Contains(s.Sound.Scale, n%12) {
				t.Errorf("cue %q note %d is below G3 or off the scale", id, n)
			}
		}
	}
}

func canonNumber(s string) (string, bool) {
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return "", false
	}
	return strconv.FormatFloat(f, 'g', -1, 64), true
}

func canonAtom(s string) string {
	s = strings.TrimSpace(s)
	if n, ok := canonNumber(s); ok {
		return n
	}
	return s
}

func canonCell(cell string) string {
	cell = strings.TrimSpace(cell)
	if strings.HasPrefix(cell, "[") && strings.HasSuffix(cell, "]") {
		parts := strings.Split(cell[1:len(cell)-1], ",")
		for i := range parts {
			parts[i] = canonAtom(parts[i])
		}
		return "[" + strings.Join(parts, ", ") + "]"
	}
	return canonAtom(cell)
}

func canonScalar(v any) (string, error) {
	switch x := v.(type) {
	case json.Number:
		n, ok := canonNumber(x.String())
		if !ok {
			return "", fmt.Errorf("bad number %s", x)
		}
		return n, nil
	case bool:
		return strconv.FormatBool(x), nil
	case string:
		return x, nil
	}
	return "", fmt.Errorf("unsupported value %T", v)
}

func flattenJSON(data []byte) (map[string]string, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	var root any
	if err := dec.Decode(&root); err != nil {
		return nil, err
	}
	out := map[string]string{}
	var walk func(path string, v any) error
	walk = func(path string, v any) error {
		switch x := v.(type) {
		case map[string]any:
			for k, e := range x {
				if err := walk(join(path, k), e); err != nil {
					return err
				}
			}
		case []any:
			parts := make([]string, len(x))
			for i, e := range x {
				s, err := canonScalar(e)
				if err != nil {
					return fmt.Errorf("%s[%d]: %w", path, i, err)
				}
				parts[i] = s
			}
			out[path] = "[" + strings.Join(parts, ", ") + "]"
		default:
			s, err := canonScalar(x)
			if err != nil {
				return fmt.Errorf("%s: %w", path, err)
			}
			out[path] = s
		}
		return nil
	}
	return out, walk("", root)
}

func tableCells(line string) []string {
	line = strings.TrimSpace(line)
	line = strings.TrimSuffix(strings.TrimPrefix(line, "|"), "|")
	cells := strings.Split(line, "|")
	for i := range cells {
		cells[i] = strings.TrimSpace(cells[i])
	}
	return cells
}

func isTableRow(line string) bool {
	line = strings.TrimSpace(line)
	return len(line) > 1 && strings.HasPrefix(line, "|") && strings.HasSuffix(line, "|")
}

func isSeparator(line string) bool {
	if !isTableRow(line) {
		return false
	}
	for _, c := range tableCells(line) {
		if strings.Trim(c, ":-") != "" || !strings.Contains(c, "-") {
			return false
		}
	}
	return true
}

// docValues reads every table: the first header cell is a path prefix
// ("root" for the top level); a "value" column names prefix.row, any other
// column names prefix.row.column.
func docValues(md string) (map[string]string, error) {
	lines := strings.Split(md, "\n")
	out := map[string]string{}
	for i := 0; i+1 < len(lines); i++ {
		if !isTableRow(lines[i]) || !isSeparator(lines[i+1]) {
			continue
		}
		header := tableCells(lines[i])
		if len(header) < 2 {
			return nil, fmt.Errorf("line %d: table header needs a prefix and a column", i+1)
		}
		prefix := header[0]
		if prefix == "root" {
			prefix = ""
		}
		j := i + 2
		for ; j < len(lines) && isTableRow(lines[j]); j++ {
			row := tableCells(lines[j])
			if len(row) != len(header) {
				return nil, fmt.Errorf("line %d: %d cells, header has %d", j+1, len(row), len(header))
			}
			for k, col := range header[1:] {
				p := join(prefix, row[0])
				if col != "value" {
					p = join(p, col)
				}
				if _, dup := out[p]; dup {
					return nil, fmt.Errorf("line %d: %s listed twice", j+1, p)
				}
				out[p] = canonCell(row[k+1])
			}
		}
		i = j - 1
	}
	return out, nil
}

func parity(md, js []byte) ([]string, error) {
	doc, err := docValues(string(md))
	if err != nil {
		return nil, err
	}
	want, err := flattenJSON(js)
	if err != nil {
		return nil, err
	}
	var diffs []string
	for p, v := range want {
		d, ok := doc[p]
		switch {
		case !ok:
			diffs = append(diffs, p+": missing from the doc")
		case d != v:
			diffs = append(diffs, fmt.Sprintf("%s: doc %s, json %s", p, d, v))
		}
	}
	for p := range doc {
		if _, ok := want[p]; !ok {
			diffs = append(diffs, p+": not in the json")
		}
	}
	slices.Sort(diffs)
	return diffs, nil
}

func TestDocTablesMatchJSON(t *testing.T) {
	md, err := os.ReadFile(docPath)
	if err != nil {
		t.Fatalf("the distributed motion tables are required: %v", err)
	}
	diffs, err := parity(md, embedded)
	if err != nil {
		t.Fatal(err)
	}
	if len(diffs) > 0 {
		t.Fatalf("release/docs/oni-animation-spec.md drifted from oni-spec.json:\n%s", strings.Join(diffs, "\n"))
	}
}

func TestDocDriftCheckBites(t *testing.T) {
	md, err := os.ReadFile(docPath)
	if err != nil {
		t.Fatal(err)
	}
	expectOnly := func(t *testing.T, md, js []byte, path string) {
		t.Helper()
		diffs, err := parity(md, js)
		if err != nil {
			t.Fatal(err)
		}
		if len(diffs) != 1 || !strings.HasPrefix(diffs[0], path+":") {
			t.Fatalf("want one drift on %s, got %q", path, diffs)
		}
	}

	t.Run("doc constant changed in a temp copy", func(t *testing.T) {
		copyPath := filepath.Join(t.TempDir(), "oni-animation-spec.md")
		if err := os.WriteFile(copyPath, []byte(replaceOnce(t, string(md), "| durationS | 0.74 |", "| durationS | 0.75 |")), 0o600); err != nil {
			t.Fatal(err)
		}
		mutated, err := os.ReadFile(copyPath)
		if err != nil {
			t.Fatal(err)
		}
		expectOnly(t, mutated, embedded, "strand.reach.durationS")
	})
	t.Run("doc row dropped", func(t *testing.T) {
		expectOnly(t, []byte(replaceOnce(t, string(md), "| tipGapPx | 9 |\n", "")), embedded, "strand.bowedPath.tipGapPx")
	})
	t.Run("json constant changed", func(t *testing.T) {
		js := []byte(replaceOnce(t, string(embedded), `"idleMs": 90000`, `"idleMs": 90001`))
		if _, err := Parse(js); err != nil {
			t.Fatalf("the mutated spec is still valid, only parity can catch it: %v", err)
		}
		expectOnly(t, md, js, "sleep.idleMs")
	})
}
