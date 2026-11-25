package sound

import (
	"math"
	"slices"
	"testing"
	"time"

	"ryoku-rashin/internal/oni/spec"
)

// A cue strikes its notes in order at their pitch, stays within full scale,
// is louder for a higher gain in dB and renders the same every time.
func TestCuesStrikeTheirNotesInOrder(t *testing.T) {
	s := loadSpec(t)
	cs := s.Sound.Cues
	for _, id := range spec.CueIDs {
		c := cs.Items[id]
		pcm := RenderCue(c, cs, RateHz)
		if len(pcm) == 0 {
			t.Fatalf("%s rendered nothing", id)
		}
		if again := RenderCue(c, cs, RateHz); !slices.Equal(pcm, again) {
			t.Fatalf("%s renders differently twice", id)
		}
		for i, v := range pcm {
			if v < -1 || v > 1 || math.IsNaN(float64(v)) {
				t.Fatalf("%s sample %d is %v", id, i, v)
			}
		}
		step := samples(cueSpacingS, RateHz)
		n := samples(0.04, RateHz)
		for k, note := range c.Notes {
			// after the bend, in the note's own strike
			w := window(pcm, k*step+samples(cs.PitchBendMs/1000, RateHz), n)
			own := goertzel(w, NoteHz(note), RateHz)
			for j, other := range c.Notes {
				if j != k && other != note && goertzel(w, NoteHz(other), RateHz) >= own {
					t.Errorf("%s: strike %d sounds note %d over its own %d", id, k, other, note)
				}
			}
		}
	}
	loud, quiet := cs.Items["hands-start"], cs.Items["hands-start"]
	quiet.GainDb -= 6
	peak := func(x []float32) float64 {
		m := 0.0
		for _, v := range x {
			m = max(m, math.Abs(float64(v)))
		}
		return m
	}
	if a, b := peak(RenderCue(loud, cs, RateHz)), peak(RenderCue(quiet, cs, RateHz)); !(b < a*0.6) {
		t.Fatalf("6 dB less is %v against %v", b, a)
	}
	if RenderCue(spec.Cue{}, cs, RateHz) != nil || RenderCue(loud, cs, 0) != nil {
		t.Fatal("nothing to play rendered something")
	}
}

// Cues pass the motifs' gate: nothing plays while sounds are off or the
// microphone is live, and an unknown cue never plays.
func TestCuesPassTheSameGate(t *testing.T) {
	p, sink, now, opens := newPlayer(t, nil)
	for _, id := range []string{"hands-start", "hands-stop"} {
		if got, err := p.PlayCue(*now, id); got != "" || err != nil {
			t.Fatalf("%s played %q (%v) with sounds off", id, got, err)
		}
	}
	p.Gate().Enable(true)
	p.Gate().Voice("speaking")
	if got, _ := p.PlayCue(*now, "hands-start"); got != "" {
		t.Fatalf("a cue played while the microphone was live: %q", got)
	}
	if *opens != 0 || len(sink.plays) != 0 {
		t.Fatalf("a closed gate opened the sink %d times", *opens)
	}
	p.Gate().Voice("idle")
	if got, err := p.PlayCue(*now, "hands-start"); got != "hands-start" || err != nil {
		t.Fatalf("an open gate played %q, %v", got, err)
	}
	*now = now.Add(time.Second)
	if got, _ := p.PlayCue(*now, "hands-stop"); got != "hands-stop" {
		t.Fatalf("hands-stop played %q", got)
	}
	*now = now.Add(time.Second)
	if got, _ := p.PlayCue(*now, "no-such-cue"); got != "" {
		t.Fatalf("an unknown cue played %q", got)
	}
	if len(sink.plays) != 2 || slices.Equal(sink.plays[0].pcm, sink.plays[1].pcm) {
		t.Fatalf("plays = %d, start and stop the same sound", len(sink.plays))
	}
}
