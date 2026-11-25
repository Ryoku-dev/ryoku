package sound

import (
	"encoding/json"
	"flag"
	"math"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"ryoku-rashin/internal/oni/spec"
)

var update = flag.Bool("update", false, "rewrite sound/testdata/motifs.json")

func loadSpec(t *testing.T) *spec.OniSpec {
	t.Helper()
	s, err := spec.Load()
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// goertzel is the power of x at f Hz.
func goertzel(x []float32, f, rate float64) float64 {
	c := 2 * math.Cos(2*math.Pi*f/rate)
	var s1, s2 float64
	for _, v := range x {
		s1, s2 = float64(v)+c*s1-s2, s1
	}
	return s1*s1 + s2*s2 - c*s1*s2
}

// window returns x[from:from+n], zero-padded outside x.
func window(x []float32, from, n int) []float32 {
	out := make([]float32, n)
	for i := range out {
		if j := from + i; j >= 0 && j < len(x) {
			out[i] = x[j]
		}
	}
	return out
}

// onsetPitch is the MIDI note whose power rises most when a note starts at
// on: the power over the n samples after on minus the n samples before.
func onsetPitch(x []float32, on, n int) (note int, rise func(hz float64) float64) {
	before, after := window(x, on-n, n), window(x, on, n)
	rise = func(hz float64) float64 {
		return goertzel(after, hz, RateHz) - goertzel(before, hz, RateHz)
	}
	best, bestRise := -1, math.Inf(-1)
	for m := 36; m <= 108; m++ {
		if r := rise(NoteHz(m)); r > bestRise {
			best, bestRise = m, r
		}
	}
	return best, rise
}

// checkMotif asserts the timing and pitch of a rendered motif.
func checkMotif(t *testing.T, id string, m spec.Motif, syn spec.Synth, pcm []float32) {
	t.Helper()
	on := Onsets(m, RateHz)
	want := on[len(on)-1] + int(math.Round((m.LengthS+syn.ReleaseS)*RateHz))
	if len(pcm) != want {
		t.Fatalf("%s: %d samples, want %d", id, len(pcm), want)
	}
	if d := int(math.Round(m.DelayS * RateHz)); on[0] != d {
		t.Fatalf("%s: first onset %d, want the delay %d", id, on[0], d)
	}
	if slices.ContainsFunc(pcm[:on[0]+1], func(v float32) bool { return v != 0 }) || pcm[on[0]+1] == 0 {
		t.Fatalf("%s: sound does not start exactly at sample %d", id, on[0])
	}
	limit := syn.Master * syn.Peak
	high := 0.0
	for i, v := range pcm {
		if math.IsNaN(float64(v)) || math.IsInf(float64(v), 0) {
			t.Fatalf("%s: sample %d is %v", id, i, v)
		}
		high = max(high, math.Abs(float64(v)))
	}
	if high > limit*(1+1e-6) || high < limit/4 {
		t.Fatalf("%s: peak %.6f outside (%.6f/4, %.6f]", id, high, limit, limit)
	}
	if tail := math.Abs(float64(pcm[len(pcm)-1])); tail > limit*1e-3 {
		t.Fatalf("%s: last sample %.6g has not released", id, tail)
	}
	n := int(math.Round(m.SpacingS * RateHz))
	for k, note := range m.Notes {
		got, rise := onsetPitch(pcm, on[k], n)
		if got != note {
			t.Fatalf("%s note %d: pitch rising at sample %d is MIDI %d, want %d", id, k, on[k], got, note)
		}
		hz := NoteHz(note)
		for _, off := range []float64{-0.5, 0.5} {
			if r := rise(hz * math.Pow(2, off/12)); r >= rise(hz) {
				t.Fatalf("%s note %d: %+.1f semitone rises %.3g >= %.3g at the note", id, k, off, r, rise(hz))
			}
		}
		if !slices.Contains(m.Notes[:k], note) {
			before := goertzel(window(pcm, on[k]-n, n), hz, RateHz)
			if after := goertzel(window(pcm, on[k], n), hz, RateHz); before > after/100 {
				t.Fatalf("%s note %d: %.1f Hz already sounds before sample %d (%.3g vs %.3g)", id, k, hz, on[k], before, after)
			}
		}
	}
}

func TestRenderEveryMotif(t *testing.T) {
	s := loadSpec(t)
	for _, id := range spec.MotifIDs {
		m, ok := s.Motif(id)
		if !ok {
			t.Fatalf("spec has no motif %q", id)
		}
		pcm := Render(m, s.Sound.Synth, RateHz)
		checkMotif(t, id, m, s.Sound.Synth, pcm)
		again := Render(m, s.Sound.Synth, RateHz)
		if !slices.Equal(encode(nil, pcm), encode(nil, again)) {
			t.Fatalf("%s: two renders differ", id)
		}
	}
}

func TestRenderRejectsNothingToPlay(t *testing.T) {
	s := loadSpec(t)
	m, _ := s.Motif(spec.MotifIDs[0])
	if Render(m, s.Sound.Synth, 0) != nil {
		t.Fatal("rate 0 rendered samples")
	}
	m.Notes = nil
	if Render(m, s.Sound.Synth, RateHz) != nil {
		t.Fatal("a motif without notes rendered samples")
	}
}

type goldenMotif struct {
	Notes   []int     `json:"notes"`
	Hz      []float64 `json:"hz"`
	Onsets  []int     `json:"onsets"`
	Samples int       `json:"samples"`
	Peak    float64   `json:"peak"`
	RMS     float64   `json:"rms"`
	Probe   []float64 `json:"probe"` // samples at Samples·i/16
}

func summarize(m spec.Motif, pcm []float32) goldenMotif {
	g := goldenMotif{Notes: m.Notes, Onsets: Onsets(m, RateHz), Samples: len(pcm)}
	for _, n := range m.Notes {
		g.Hz = append(g.Hz, round6(NoteHz(n)))
	}
	sq := 0.0
	for _, v := range pcm {
		g.Peak = max(g.Peak, math.Abs(float64(v)))
		sq += float64(v) * float64(v)
	}
	g.Peak, g.RMS = round6(g.Peak), round6(math.Sqrt(sq/float64(len(pcm))))
	for i := range 16 {
		g.Probe = append(g.Probe, round6(float64(pcm[len(pcm)*i/16])))
	}
	return g
}

func round6(v float64) float64 { return math.Round(v*1e6) / 1e6 }

// The golden pins every motif's pitches, sample timings and shape so a
// later renderer (the Ryoku shell) can check itself against the same file.
func TestRenderGolden(t *testing.T) {
	s := loadSpec(t)
	got := map[string]goldenMotif{}
	for _, id := range spec.MotifIDs {
		m, _ := s.Motif(id)
		got[id] = summarize(m, Render(m, s.Sound.Synth, RateHz))
	}
	path := filepath.Join("testdata", "motifs.json")
	if *update {
		data, err := json.MarshalIndent(got, "", " ")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("missing golden (run with -update): %v", err)
	}
	var want map[string]goldenMotif
	if err := json.Unmarshal(data, &want); err != nil {
		t.Fatal(err)
	}
	if len(want) != len(got) {
		t.Fatalf("golden has %d motifs, spec %d", len(want), len(got))
	}
	near := func(a, b float64) bool { return math.Abs(a-b) <= 2e-6 }
	for id, g := range got {
		w, ok := want[id]
		if !ok {
			t.Fatalf("golden lacks %s", id)
		}
		if !slices.Equal(g.Notes, w.Notes) || !slices.Equal(g.Onsets, w.Onsets) || g.Samples != w.Samples ||
			!slices.EqualFunc(g.Hz, w.Hz, near) || !near(g.Peak, w.Peak) || !near(g.RMS, w.RMS) ||
			!slices.EqualFunc(g.Probe, w.Probe, near) {
			t.Fatalf("%s drifted from the golden:\n got %+v\nwant %+v", id, g, w)
		}
	}
}

func TestEventMapsToSpecMotifs(t *testing.T) {
	for _, id := range spec.EventIDs {
		m, ok := Event(id)
		if !ok || !slices.Contains(spec.MotifIDs, m) {
			t.Fatalf("event %q: motif %q ok=%v", id, m, ok)
		}
	}
	if _, ok := Event("no-such-event"); ok {
		t.Fatal("an unknown event mapped to a motif")
	}
}
