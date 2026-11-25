package motion

import "math"

// SetState moves toward a daemon state (see spec.StateIDs). Unknown and
// repeated states are ignored. The reason is the host's concern (hidden or
// off are presentation decisions); the pose only follows the state.
func (e *Engine) SetState(id, reason string) {
	if _, ok := e.s.States[id]; !ok || id == e.state {
		return
	}
	e.state = id
	e.sleepT, e.committed, e.asleep = 0, false, false
	if !e.lap.beating() {
		e.lap.set(e.colour())
	}
	e.event = true
}

// targets sets every spring's target from the state and, unless still, the
// input's leans, stretch and mic lift.
func (e *Engine) targets(still bool) {
	st := e.s.States[e.state]
	e.shapeT = [3]float64{st.Flat, st.Face, st.Loose}
	e.poseT = [nCh]float64{chScale: st.Scale, chTempo: st.Tempo, chBright: 1}
	e.ampT, e.breatT = st.Hover[0], st.Hover[2]
	if still {
		return
	}
	m := e.s.Motion
	if p := e.in.Pointer; p != nil && e.state == "listening" {
		if d := math.Hypot(p.X, p.Y); d >= m.Leans.ListeningMinPx && d > 0 {
			e.poseT[chLeanX] += m.Leans.Listening * p.X / d
			e.poseT[chLeanY] += m.Leans.Listening * p.Y / d
		}
	}
	if dr := e.in.Drag; dr != nil {
		v := dr.Velocity
		e.poseT[chLeanX] += min(max(-v.X/m.Leans.DragPerSpeed, -m.Leans.DragMax), m.Leans.DragMax)
		e.poseT[chLeanY] += min(max(-v.Y/m.Leans.DragPerSpeed, -m.Leans.DragMax), m.Leans.DragMax)
		e.poseT[chSquash] -= min(math.Hypot(v.X, v.Y)*m.Stretch.Per, m.Stretch.Max)
	}
	e.poseT[chBright] += m.Mic.Bright * unit(e.in.Mic)
}

// compose builds the pose from the springs, the gesture offset, speech,
// breath and hover float, clamped to the spec ranges.
func (e *Engine) compose(still bool) Pose {
	p := Pose{
		State:   e.state,
		Gesture: e.g.id,
		Flat:    e.shape[0].X,
		Face:    e.shape[1].X,
		Loose:   e.shape[2].X,
		Spin:    wrap(e.spin),
	}
	var v [nCh]float64
	for i := range v {
		v[i] = e.pose[i].X
	}
	if !still {
		d := e.g.delta(e.s)
		for i := range v {
			v[i] += d[i]
		}
		if level := unit(e.in.Speech); level > 0 {
			sp := e.s.Motion.Speaking
			w := sp.Gain*math.Sin(e.speakPhase*sp.RateA) + (1-sp.Gain)*math.Sin(e.speakPhase*sp.RateB)
			v[chScale] += sp.Scale * w * level
			v[chSquash] += sp.Squash * w * level
			v[chBright] += sp.Bright * w * level
		}
		v[chScale] *= 1 + e.breath.X*math.Sin(e.hoverPhase)
		p.HoverY = e.hoverY()
	}
	r := e.s.Springs.PoseRanges
	p.Scale = clampRange(v[chScale], r.Scale)
	p.Squash = clampRange(v[chSquash], r.Squash)
	p.LeanX = clampRange(v[chLeanX], r.Lean)
	p.LeanY = clampRange(v[chLeanY], r.Lean)
	p.Unravel = clampRange(v[chUnravel], r.Unravel)
	p.Straighten = clampRange(v[chStraighten], r.Unravel)
	p.Tempo = clampRange(v[chTempo], r.Tempo)
	p.Bright = clampRange(v[chBright], r.Bright)
	p.Lap = e.lap.pose(e.s, e.in.Mic, still)
	return p
}
