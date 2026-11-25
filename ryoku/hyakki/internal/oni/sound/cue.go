package sound

import (
	"math"
	"time"

	"ryoku-rashin/internal/oni/spec"
)

// The struck shape of a cue: each note is a strike whose partials ring out
// over cueRingS (the higher partials faster), cueSpacingS after the one
// before, opened by a click cueClickS long at cueClickGain of a partial.
const (
	cueSpacingS  = 0.09
	cueRingS     = 0.35
	cueClickS    = 0.002
	cueClickGain = 0.15
	cueSeed      = 0x9e3779b9
)

// RenderCue synthesises cue c as mono PCM at rateHz: every note struck in
// turn as the cues' partials (their frequency ratios and amplitudes), each
// starting pitchBendMs sharp and bending down onto its note, a short click
// from a fixed-seed xorshift at each strike, the sum scaled by the cues'
// gain and the cue's own gain in dB and kept within [-1, 1]. The same input
// gives the same samples.
func RenderCue(c spec.Cue, cs spec.Cues, rateHz int) []float32 {
	if rateHz <= 0 || len(c.Notes) == 0 || len(cs.Partials) == 0 {
		return nil
	}
	rate := float64(rateHz)
	ring := samples(cueRingS, rateHz)
	step := samples(cueSpacingS, rateHz)
	sum := make([]float64, step*(len(c.Notes)-1)+ring)
	bend := cs.PitchBendMs / 1000
	norm := 0.0
	for _, a := range cs.Amps {
		norm += math.Abs(a)
	}
	if norm == 0 {
		norm = 1
	}
	seed := uint32(cueSeed)
	click := samples(cueClickS, rateHz)
	for k, note := range c.Notes {
		f := NoteHz(note)
		at := k * step
		for p, ratio := range cs.Partials {
			amp := 0.0
			if p < len(cs.Amps) {
				amp = cs.Amps[p] / norm
			}
			tau := cueRingS / 4 / ratio
			phase := 0.0
			for i := range ring {
				t := float64(i) / rate
				// a semitone sharp at the strike, onto the note by bend
				hz := f * ratio
				if bend > 0 && t < bend {
					hz *= math.Pow(2, (1-t/bend)/12)
				}
				phase += 2 * math.Pi * hz / rate
				sum[at+i] += amp * math.Exp(-t/tau) * math.Sin(phase)
			}
		}
		for i := range min(click, ring) {
			seed ^= seed << 13
			seed ^= seed >> 17
			seed ^= seed << 5
			noise := float64(seed)/float64(math.MaxUint32)*2 - 1
			sum[at+i] += cueClickGain * noise * (1 - float64(i)/float64(click))
		}
	}
	gain := cs.Gain * math.Pow(10, c.GainDb/20)
	out := make([]float32, len(sum))
	for i, v := range sum {
		out[i] = float32(max(-1, min(1, v*gain)))
	}
	return out
}

// PlayCue sounds cue id (one of spec.CueIDs) if the gate allows it at now,
// on the same sink and gate as the motifs. It returns id when it played, ""
// when the cue is unknown or the gate held it back.
func (p *Player) PlayCue(now time.Time, id string) (string, error) {
	c, ok := p.cues.Items[id]
	if !ok || !p.gate.Allow(now, "cue-"+id) {
		return "", nil
	}
	p.mu.Lock()
	if p.sink == nil {
		sink, err := p.open()
		if err != nil {
			p.mu.Unlock()
			return "", err
		}
		p.sink = sink
	}
	key := "cue-" + id
	pcm, ok := p.pcm[key]
	if !ok {
		pcm = RenderCue(c, p.cues, RateHz)
		p.pcm[key] = pcm
	}
	sink := p.sink
	p.mu.Unlock()
	cancel, err := sink.Play(pcm, RateHz)
	if err != nil {
		return "", err
	}
	p.gate.Track(cancel)
	return id, nil
}
