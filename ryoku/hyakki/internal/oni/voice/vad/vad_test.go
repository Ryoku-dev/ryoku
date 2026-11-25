package vad

import (
	"fmt"
	"math"
	"slices"
	"strings"
	"testing"
	"time"

	"ryoku-rashin/internal/oni/spec"
)

const rate = 16000

func specConfig(t *testing.T, m Mode) Config {
	t.Helper()
	sp, err := spec.Load()
	if err != nil {
		t.Fatal(err)
	}
	return FromSpec(sp, m)
}

func samples(d time.Duration) int { return int(int64(d) * rate / int64(time.Second)) }

// tone is a voiced fixture: a 220 Hz sine well above the speech threshold.
func tone(d time.Duration) []int16 {
	out := make([]int16, samples(d))
	for i := range out {
		out[i] = int16(3000 * math.Sin(2*math.Pi*220*float64(i)/rate))
	}
	return out
}

// hiss is room noise: uniform values within ±200, far under the threshold.
func hiss(d time.Duration) []int16 {
	out := make([]int16, samples(d))
	x := uint32(2463534242)
	for i := range out {
		x ^= x << 13
		x ^= x >> 17
		x ^= x << 5
		out[i] = int16(int32(x%401) - 200)
	}
	return out
}

func silence(d time.Duration) []int16 { return make([]int16, samples(d)) }

func cat(parts ...[]int16) []int16 { return slices.Concat(parts...) }

func msec(n int) time.Duration { return time.Duration(n) * time.Millisecond }

func newSeg(t *testing.T, c Config, now func() time.Time) *Segmenter {
	t.Helper()
	s, err := New(c, now)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// run pushes pcm in chunks of n samples, then ends the press.
func run(t *testing.T, c Config, pcm []int16, n int) []Event {
	t.Helper()
	s := newSeg(t, c, nil)
	var out []Event
	for len(pcm) > 0 {
		k := min(n, len(pcm))
		out = append(out, s.Push(pcm[:k])...)
		pcm = pcm[k:]
	}
	return append(out, s.End()...)
}

func describe(evs []Event) []string {
	out := make([]string, len(evs))
	for i, e := range evs {
		s := fmt.Sprintf("%s@%v", e.Kind, e.At)
		if e.Kind == Final {
			s += fmt.Sprintf("/%d", e.Reason)
		}
		out[i] = s
	}
	return out
}

func only(evs []Event, k Kind) []Event {
	var out []Event
	for _, e := range evs {
		if e.Kind == k {
			out = append(out, e)
		}
	}
	return out
}

func TestSpeechStartsBeforeTheFirstPartialAndPauseEndsIt(t *testing.T) {
	c := specConfig(t, Hold)
	lead := hiss(msec(500))
	pcm := cat(lead, tone(msec(2000)), silence(msec(1000)))
	evs := run(t, c, pcm, 1024)

	starts, partials, finals, pauses := only(evs, SpeechStart), only(evs, Partial), only(evs, Final), only(evs, PauseEnd)
	if len(starts) != 1 || len(finals) != 1 || len(pauses) != 1 || len(partials) == 0 {
		t.Fatalf("events %v", describe(evs))
	}
	if evs[0].Kind != SpeechStart {
		t.Fatalf("first event %v, want speech-start", describe(evs))
	}
	if got := starts[0]; got.Start != msec(500) || got.At != msec(500)+c.MinSpeech {
		t.Errorf("speech start at %v (onset %v), want %v (onset 500ms)", got.At, got.Start, msec(500)+c.MinSpeech)
	}
	if !(starts[0].At < partials[0].At) {
		t.Errorf("speech started at %v, not before the first partial at %v", starts[0].At, partials[0].At)
	}
	if got := partials[0].At - partials[0].Start; got != c.FirstPartial {
		t.Errorf("first partial %v after onset, want %v", got, c.FirstPartial)
	}
	for i := 1; i < len(partials); i++ {
		if gap := partials[i].At - partials[i-1].At; gap != c.PartialEvery {
			t.Errorf("partial %d came %v after the last, want %v", i, gap, c.PartialEvery)
		}
	}
	for _, p := range partials {
		if want := samples(c.PreRoll + p.At - p.Start); len(p.PCM) != want {
			t.Errorf("partial at %v carries %d samples, want %d", p.At, len(p.PCM), want)
		}
	}
	speechEnd := msec(2500)
	if pauses[0].At != speechEnd+c.Pause || pauses[0].At != msec(2950) {
		t.Errorf("pause end at %v, want 450ms after speech ended (2.95s)", pauses[0].At)
	}
	f := finals[0]
	if f.Reason != ReasonPause || f.At != pauses[0].At {
		t.Errorf("final %+v, want a pause final at the pause end", describe(finals))
	}
	want := cat(lead[samples(msec(300)):], tone(msec(2000)))
	if !slices.Equal(f.PCM, want) {
		t.Errorf("final holds %d samples, want the 200ms pre-roll plus the speech (%d)", len(f.PCM), len(want))
	}
}

func TestPauseEndsAtExactlyThePause(t *testing.T) {
	c := specConfig(t, Hold)
	for _, gap := range []time.Duration{c.Pause - c.Frame, c.Pause} {
		t.Run(gap.String(), func(t *testing.T) {
			pcm := cat(tone(msec(1000)), silence(gap), tone(msec(1000)), silence(msec(1000)))
			evs := run(t, c, pcm, 333)
			finals, pauses := only(evs, Final), only(evs, PauseEnd)
			if gap < c.Pause {
				if len(finals) != 1 || len(pauses) != 1 {
					t.Fatalf("a %v gap split the segment: %v", gap, describe(evs))
				}
				if got := len(finals[0].PCM); got != samples(2000*time.Millisecond+gap) {
					t.Errorf("the final holds %d samples, want both words and the gap", got)
				}
				return
			}
			if len(finals) != 2 || len(pauses) != 2 {
				t.Fatalf("a %v gap did not end the segment: %v", gap, describe(evs))
			}
			if pauses[0].At != msec(1000)+c.Pause {
				t.Errorf("pause end at %v, want %v", pauses[0].At, msec(1000)+c.Pause)
			}
			if second := only(evs, SpeechStart)[1]; second.Start != msec(1000)+gap {
				t.Errorf("second segment starts at %v, want %v", second.Start, msec(1000)+gap)
			}
		})
	}
}

func TestSegmentIsCutAtMaxSegment(t *testing.T) {
	c := specConfig(t, Hold)
	evs := run(t, c, tone(msec(12000)), 4096)
	finals := only(evs, Final)
	if len(finals) != 2 {
		t.Fatalf("events %v", describe(evs))
	}
	if f := finals[0]; f.Reason != ReasonMaxSegment || f.At != c.MaxSegment || f.At != 9*time.Second || len(f.PCM) != samples(c.MaxSegment) {
		t.Errorf("first final %v with %d samples, want a max-segment final at 9s holding 9s", describe(finals[:1]), len(f.PCM))
	}
	if f := finals[1]; f.Reason != ReasonEnd || f.Start != c.MaxSegment || len(f.PCM) != samples(c.PreRoll+3*time.Second) {
		t.Errorf("second final %v from %v with %d samples, want the rest after 9s with its pre-roll", describe(finals[1:]), f.Start, len(f.PCM))
	}
	if len(only(evs, SpeechStart)) != 2 || len(only(evs, PauseEnd)) != 0 {
		t.Errorf("events %v", describe(evs))
	}
}

func TestTapsAndNoiseYieldNothing(t *testing.T) {
	c := specConfig(t, Hold)
	for _, tc := range []struct {
		name string
		pcm  []int16
	}{
		{"0.3s tap", tone(msec(300))},
		{"0.2s tap", tone(msec(200))},
		{"room noise", hiss(5 * time.Second)},
		{"silence", silence(2 * time.Second)},
		{"a blip under min speech", cat(tone(msec(340)), silence(msec(1000)))},
		{"blips spaced by pauses", cat(tone(msec(200)), silence(msec(450)), tone(msec(200)), silence(msec(450)), tone(msec(200)))},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, n := range []int{1, 160, 4096} {
				if evs := run(t, c, tc.pcm, n); len(evs) != 0 {
					t.Fatalf("chunks of %d: %v", n, describe(evs))
				}
			}
		})
	}
}

// fakeClock is a wall clock the test moves.
type fakeClock struct{ t time.Time }

func (f *fakeClock) now() time.Time { return f.t }

func TestATapIsDroppedEvenWhenItHeldSpeech(t *testing.T) {
	c := specConfig(t, Hold)
	c.MinSpeech, c.FirstPartial = msec(100), msec(200)
	clk := &fakeClock{t: time.Unix(1e9, 0)}

	s := newSeg(t, c, clk.now)
	if evs := s.Push(tone(msec(250))); len(evs) != 0 {
		t.Fatalf("a press still under the tap drop emitted %v", describe(evs))
	}
	clk.t = clk.t.Add(msec(250))
	if evs := s.End(); len(evs) != 0 {
		t.Fatalf("a 250ms tap yielded %v", describe(evs))
	}

	s = newSeg(t, c, clk.now)
	evs := s.Push(tone(msec(250)))
	if len(evs) != 0 {
		t.Fatalf("held back too early: %v", describe(evs))
	}
	evs = s.Push(tone(msec(150)))
	if len(evs) == 0 || evs[0].Kind != SpeechStart || evs[0].At != msec(100) {
		t.Fatalf("once the press outlasts the tap drop the held events go out in order, got %v", describe(evs))
	}
	if f := only(s.End(), Final); len(f) != 1 || f[0].Reason != ReasonEnd || len(f[0].PCM) != samples(msec(400)) {
		t.Fatalf("release final %v", describe(f))
	}
}

func TestSessionsAreCut(t *testing.T) {
	for _, tc := range []struct {
		mode Mode
		want time.Duration
	}{{Hold, 20 * time.Second}, {OpenMic, 180 * time.Second}, {Wake, 15 * time.Second}} {
		t.Run(fmt.Sprint(tc.mode), func(t *testing.T) {
			c := specConfig(t, tc.mode)
			if c.MaxSession != tc.want {
				t.Fatalf("max session %v, want %v", c.MaxSession, tc.want)
			}
			if (tc.mode == Hold) != (c.TapDrop == msec(300)) {
				t.Fatalf("tap drop %v in mode %d", c.TapDrop, tc.mode)
			}
			s := newSeg(t, c, nil)
			one := tone(time.Second)
			var evs []Event
			for range int(tc.want/time.Second) + 5 {
				evs = append(evs, s.Push(one)...)
			}
			cuts := only(evs, Cut)
			if len(cuts) != 1 || cuts[0].At != tc.want || evs[len(evs)-1].Kind != Cut {
				t.Fatalf("cuts %v, last %v", describe(cuts), describe(evs[len(evs)-1:]))
			}
			last := only(evs, Final)
			// 180s is a whole number of segments, so that segment ends on its own cap.
			if f := last[len(last)-1]; (f.Reason != ReasonCut && f.Reason != ReasonMaxSegment) || f.At != tc.want {
				t.Errorf("the speech under way at the cut ends in %v", describe(last[len(last)-1:]))
			}
			for _, e := range evs {
				if e.At > tc.want {
					t.Fatalf("event after the cut: %v", describe([]Event{e}))
				}
			}
			if more := append(s.Push(one), s.End()...); len(more) != 0 {
				t.Fatalf("a cut session still emits %v", describe(more))
			}
		})
	}
}

func TestTheClockCutsAStalledHold(t *testing.T) {
	c := specConfig(t, Hold)
	clk := &fakeClock{t: time.Unix(1e9, 0)}
	s := newSeg(t, c, clk.now)
	evs := s.Push(tone(time.Second))
	clk.t = clk.t.Add(c.MaxSession)
	evs = append(evs, s.Push(nil)...)
	if got := describe(evs[len(evs)-2:]); !slices.Equal(got, []string{"final@1s/4", "cut@1s"}) {
		t.Fatalf("after 20s of wall time with 1s of audio: %v", describe(evs))
	}
}

func TestChunkingDoesNotChangeTheEvents(t *testing.T) {
	c := specConfig(t, Hold)
	pcm := cat(hiss(msec(700)), tone(msec(1800)), hiss(msec(300)), tone(msec(600)), silence(msec(600)), tone(msec(9500)), hiss(msec(400)))
	want := describe(run(t, c, pcm, len(pcm)))
	if len(want) < 10 {
		t.Fatalf("fixture too quiet: %v", want)
	}
	for _, n := range []int{1, 37, 159, 160, 161, 4096} {
		if got := describe(run(t, c, pcm, n)); !slices.Equal(got, want) {
			t.Errorf("chunks of %d:\n got %s\nwant %s", n, strings.Join(got, " "), strings.Join(want, " "))
		}
	}
}

func TestEventAudioStaysPutAfterLaterPushes(t *testing.T) {
	c := specConfig(t, Hold)
	s := newSeg(t, c, nil)
	evs := s.Push(tone(msec(1000)))
	partials := only(evs, Partial)
	if len(partials) == 0 {
		t.Fatal("no partial in a second of speech")
	}
	held := slices.Clone(partials[0].PCM)
	_ = s.Push(hiss(msec(3000)))
	_ = s.Push(tone(msec(2000)))
	_ = s.End()
	if !slices.Equal(partials[0].PCM, held) {
		t.Fatal("a partial's audio changed after later pushes")
	}
}

func TestNewRejectsNumbersOffTheFrameGrid(t *testing.T) {
	good := specConfig(t, Hold)
	for _, tc := range []struct {
		name string
		edit func(*Config)
	}{
		{"zero frame", func(c *Config) { c.Frame = 0 }},
		{"frame off the sample grid", func(c *Config) { c.Frame = 30 * time.Microsecond }},
		{"pause off the frame grid", func(c *Config) { c.Pause = msec(455) }},
		{"zero pause", func(c *Config) { c.Pause = 0 }},
		{"zero session", func(c *Config) { c.MaxSession = 0 }},
		{"negative pre-roll", func(c *Config) { c.PreRoll = -c.Frame }},
		{"zero threshold", func(c *Config) { c.SpeechRMS = 0 }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := good
			tc.edit(&c)
			if _, err := New(c, nil); err == nil {
				t.Fatal("accepted")
			}
		})
	}
	if _, err := New(good, nil); err != nil {
		t.Fatal(err)
	}
}
