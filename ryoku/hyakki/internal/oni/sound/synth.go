// Package sound synthesises the character's short motifs, gates them against
// the microphone and the sounds setting, and hands the PCM to a sink: a
// pw-cat or paplay child process, or a file in the lab. Nothing loops and
// there are no audio files; every motif is rendered from spec numbers.
package sound

import (
	"math"

	"ryoku-rashin/internal/oni/spec"
)

// RateHz is the rate motifs render and sinks play at.
const RateHz = 48000

// lowpassQ is a Butterworth response: flat passband, no resonant bump.
const lowpassQ = math.Sqrt2 / 2

// NoteHz is the equal-tempered frequency of a MIDI note (69 = 440 Hz).
func NoteHz(note int) float64 { return 440 * math.Pow(2, float64(note-69)/12) }

// Onsets returns the sample index each note of m starts at.
func Onsets(m spec.Motif, rateHz int) []int {
	out := make([]int, len(m.Notes))
	for k := range out {
		out[k] = samples(m.DelayS+float64(k)*m.SpacingS, rateHz)
	}
	return out
}

// Render synthesises one motif as mono PCM. Each note is a sine plus an
// octave harmonic, shaped by an attack/decay/sustain/release envelope that
// holds for the motif's length and then releases; the notes are summed,
// lowpassed and scaled by the master gain, and the result never exceeds
// master·peak. The buffer opens with the motif's delay as silence and ends
// when the last note's release does. The same input gives the same samples.
func Render(m spec.Motif, s spec.Synth, rateHz int) []float32 {
	if rateHz <= 0 || len(m.Notes) == 0 {
		return nil
	}
	onsets := Onsets(m, rateHz)
	span := samples(m.LengthS+s.ReleaseS, rateHz)
	sum := make([]float64, onsets[len(onsets)-1]+span)
	rate := float64(rateHz)
	norm := 1 / (1 + s.Harmonic)
	for k, note := range m.Notes {
		w := 2 * math.Pi * NoteHz(note) / rate
		at := onsets[k]
		for i := range min(span, len(sum)-at) {
			env := envelope(float64(i)/rate, m.LengthS, s)
			if env == 0 {
				continue
			}
			ph := w * float64(i)
			sum[at+i] += s.Peak * env * norm * (math.Sin(ph) + s.Harmonic*math.Sin(2*ph))
		}
	}
	lowpass(sum, s.LowpassHz, rate)
	limit := s.Master * s.Peak
	high := 0.0
	for i, v := range sum {
		v *= s.Master
		sum[i] = v
		high = max(high, math.Abs(v))
	}
	gain := 1.0
	if high > limit {
		gain = limit / high
	}
	out := make([]float32, len(sum))
	for i, v := range sum {
		out[i] = float32(v * gain)
	}
	return out
}

func samples(seconds float64, rateHz int) int {
	return int(math.Round(seconds * float64(rateHz)))
}

// envelope is the note's gain t seconds after its onset: a linear attack to
// 1, a decay to the sustain level, a hold until length, then a linear
// release to 0 from wherever the note stood at length.
func envelope(t, length float64, s spec.Synth) float64 {
	held := func(t float64) float64 {
		switch {
		case t < s.AttackS:
			return t / s.AttackS
		case t < s.AttackS+s.DecayS:
			return 1 - (1-s.Sustain)*(t-s.AttackS)/s.DecayS
		default:
			return s.Sustain
		}
	}
	switch {
	case t < 0:
		return 0
	case t < length:
		return held(t)
	case t < length+s.ReleaseS:
		return held(length) * (1 - (t-length)/s.ReleaseS)
	default:
		return 0
	}
}

// lowpass filters x in place with a second-order lowpass at cutoff Hz.
func lowpass(x []float64, cutoff, rate float64) {
	if !(cutoff > 0) || cutoff >= rate/2 {
		return
	}
	w0 := 2 * math.Pi * cutoff / rate
	cos, alpha := math.Cos(w0), math.Sin(w0)/(2*lowpassQ)
	a0 := 1 + alpha
	b0 := (1 - cos) / 2 / a0
	b1 := (1 - cos) / a0
	b2 := b0
	a1 := -2 * cos / a0
	a2 := (1 - alpha) / a0
	var x1, x2, y1, y2 float64
	for i, in := range x {
		y := b0*in + b1*x1 + b2*x2 - a1*y1 - a2*y2
		x2, x1 = x1, in
		y2, y1 = y1, y
		x[i] = y
	}
}
