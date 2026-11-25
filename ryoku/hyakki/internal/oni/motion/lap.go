package motion

import (
	"math"

	"ryoku-rashin/internal/oni/spec"
)

// lapState runs the accent colour round the character on every colour
// change, and holds gesture colour beats before lapping back.
type lapState struct {
	from, to string
	t        float64
	glowT    float64
	beatLeft float64
}

func (l *lapState) reset(colour string) {
	*l = lapState{from: colour, to: colour, t: math.Inf(1)}
}

func (l *lapState) complete() { l.from, l.t, l.beatLeft = l.to, math.Inf(1), 0 }

func (l *lapState) set(colour string) {
	if colour == l.to {
		return
	}
	l.from, l.to, l.t = l.to, colour, 0
}

func (l *lapState) beat(b spec.Beat) {
	l.set(b.Colour)
	l.beatLeft = b.HoldS
}

func (l *lapState) beating() bool { return l.beatLeft > 0 }

// advance reports whether a beat just ended, so the state colour returns.
func (l *lapState) advance(d float64, s *spec.OniSpec) bool {
	l.t += d
	if p := s.Colour.Glow.PeriodS; p > 0 {
		l.glowT = math.Mod(l.glowT+d, p)
	}
	if l.beatLeft > 0 {
		l.beatLeft -= d
		if l.beatLeft <= 0 {
			l.beatLeft = 0
			return true
		}
	}
	return false
}

func (l *lapState) moving(s *spec.OniSpec) bool {
	c := s.Colour.Lap
	return l.beatLeft > 0 || l.t < max(c.DurationS, c.ArriveS, c.LeaveS)
}

func accent(id string) bool { return id != "" && id != "idle" }

func (l *lapState) pose(s *spec.OniSpec, mic float64, still bool) Lap {
	c, g := s.Colour.Lap, s.Colour.Glow
	out := Lap{From: l.from, To: l.to, Progress: 1}
	if c.DurationS > 0 {
		out.Progress = unit(l.t / c.DurationS)
	}
	if still {
		if accent(l.to) {
			out.Glow = unit(g.Strength * g.High)
		}
		return out
	}
	env := 0.0
	switch {
	case accent(l.to) && accent(l.from):
		env = 1
	case accent(l.to):
		env = ramp(l.t, c.ArriveS)
	case accent(l.from):
		env = 1 - ramp(l.t, c.LeaveS)
	}
	breath := g.High
	if g.PeriodS > 0 {
		breath = g.Low + (g.High-g.Low)*(0.5-0.5*math.Cos(2*math.Pi*l.glowT/g.PeriodS))
	}
	out.Glow = unit(g.Strength*breath*env + s.Colour.Mic.Glow*unit(mic))
	return out
}

func ramp(t, over float64) float64 {
	if !(over > 0) {
		return 1
	}
	return unit(t / over)
}
