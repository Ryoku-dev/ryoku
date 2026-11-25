package motion

import (
	"math"

	"ryoku-rashin/internal/oni/spec"
)

const (
	chScale = iota
	chSquash
	chLeanX
	chLeanY
	chUnravel
	chStraighten
	chTempo
	chBright
	nCh
)

var channels = [nCh]string{"scale", "squash", "leanX", "leanY", "unravel", "straighten", "tempo", "bright"}

type gestureRun struct {
	id     string
	t, dur float64
	keys   spec.Keyframes
	mirror bool

	residual [nCh]float64
	handoffT float64
}

func (g *gestureRun) clear() {
	*g = gestureRun{handoffT: math.Inf(1)}
}

func (g *gestureRun) advance(d float64) {
	if g.id != "" {
		g.t += d
		if g.t >= g.dur {
			g.id, g.keys = "", nil
		}
	}
	g.handoffT += d
}

func (g *gestureRun) handingOff(s *spec.OniSpec) bool {
	return g.handoffT < s.Gestures.HandoffMs/1000
}

func (g *gestureRun) active(s *spec.OniSpec) bool {
	return g.id != "" || g.handingOff(s)
}

// delta is the gesture's offset on each pose channel plus whatever the
// replaced gesture still hands off.
func (g *gestureRun) delta(s *spec.OniSpec) [nCh]float64 {
	var d [nCh]float64
	if g.id != "" {
		p := g.t / g.dur
		for i, ch := range channels {
			d[i] = g.keys.Sample(ch, p)
		}
		if g.mirror {
			d[chLeanX] = -d[chLeanX]
		}
	}
	if h := s.Gestures.HandoffMs / 1000; g.handoffT < h {
		f := 1 - g.handoffT/h
		for i := range d {
			d[i] += g.residual[i] * f
		}
	}
	return d
}

// Play starts a gesture (see spec.GestureIDs), handing any running
// gesture's offset off over the spec handoff; it reports false for an
// unknown id. Poke mirrors its lean when the pointer is on the right.
func (e *Engine) Play(gesture string) bool {
	gs, ok := e.s.Gesture(gesture)
	if !ok {
		return false
	}
	residual := e.g.delta(e.s)
	e.g = gestureRun{id: gesture, dur: gs.DurationS, keys: gs.Keyframes, handoffT: math.Inf(1)}
	if residual != ([nCh]float64{}) {
		e.g.residual, e.g.handoffT = residual, 0
	}
	e.g.mirror = gesture == "poke" && e.in.Pointer != nil && e.in.Pointer.X > 0
	switch gesture {
	case "done":
		e.lap.beat(e.s.Colour.Beats.Done)
	case "error":
		e.lap.beat(e.s.Colour.Beats.Error)
	}
	e.committed, e.asleep = false, false
	e.event = true
	return true
}
