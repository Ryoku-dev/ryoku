package sound

import (
	"slices"
	"testing"
	"time"

	"ryoku-rashin/internal/oni/profile"
	"ryoku-rashin/internal/oni/spec"
)

var t0 = time.Unix(1_000_000, 0)

func ms(n float64) time.Duration { return time.Duration(n * float64(time.Millisecond)) }

func openGate(t *testing.T) *Gate {
	t.Helper()
	g := NewGate(loadSpec(t).Sound.Synth)
	g.Enable(true)
	g.Voice("idle")
	return g
}

func TestGateFailsClosed(t *testing.T) {
	var zero Gate
	if zero.Allow(t0, "done") {
		t.Fatal("the zero gate allowed a sound")
	}
	g := NewGate(loadSpec(t).Sound.Synth)
	g.Voice("idle")
	if g.Allow(t0, "done") {
		t.Fatal("a disabled gate allowed a sound")
	}
	g = NewGate(loadSpec(t).Sound.Synth)
	g.Enable(true)
	if g.Allow(t0, "done") {
		t.Fatal("an enabled gate with no voice frame yet allowed a sound")
	}
	g.Voice("idle")
	if !g.Allow(t0, "done") {
		t.Fatal("an enabled gate with an idle microphone held the sound")
	}
	if g.Allow(t0.Add(time.Second), "") {
		t.Fatal("an empty motif was allowed")
	}
}

func TestGateSilentWhileMicLive(t *testing.T) {
	for _, phase := range []string{"listening", "transcribing", "speaking", "", "capturing"} {
		g := openGate(t)
		g.Voice(phase)
		if g.Allow(t0, "done") {
			t.Fatalf("voice phase %q allowed a sound", phase)
		}
		g.Voice("idle")
		if !g.Allow(t0, "done") {
			t.Fatalf("idle after %q still held the sound", phase)
		}
	}
}

func TestGateLinkLossFailsClosedUntilIdle(t *testing.T) {
	g := openGate(t)
	cancelled := 0
	g.Track(func() { cancelled++ })
	g.LinkLost()
	if cancelled != 1 {
		t.Fatalf("link loss cancelled the playing sound %d times", cancelled)
	}
	if g.Allow(t0, "done") {
		t.Fatal("a sound passed after the link dropped")
	}
	g.Enable(true)
	if g.Allow(t0, "done") {
		t.Fatal("re-enabling opened the gate before an idle voice frame")
	}
	g.Voice("idle")
	if !g.Allow(t0, "done") {
		t.Fatal("an idle voice frame did not reopen the gate")
	}
}

func TestGateDebounce(t *testing.T) {
	s := loadSpec(t)
	g := openGate(t)
	db := seconds(s.Sound.Synth.DebounceS)
	if db != ms(90) {
		t.Fatalf("debounce %v, want the spec's 90ms", db)
	}
	if !g.Allow(t0, "done") {
		t.Fatal("first sound held")
	}
	if g.Allow(t0.Add(db-time.Millisecond), "poke") {
		t.Fatal("a sound inside the debounce passed")
	}
	if !g.Allow(t0.Add(db), "poke") {
		t.Fatal("a sound at the debounce was held")
	}
	if g.Allow(t0.Add(db+ms(10)), "done") {
		t.Fatal("the debounce did not restart from the last allowed sound")
	}
	var zero Gate
	zero.Enable(true)
	zero.Voice("idle")
	if !zero.Allow(t0, "done") || zero.Allow(t0.Add(db-time.Millisecond), "done") || !zero.Allow(t0.Add(db), "done") {
		t.Fatal("the zero gate does not use the built-in debounce")
	}
}

func TestGateCancelsOnClose(t *testing.T) {
	g := openGate(t)
	var calls []string
	g.Track(func() { calls = append(calls, "a") })
	g.Track(func() { calls = append(calls, "b") })
	if !slices.Equal(calls, []string{"a"}) {
		t.Fatalf("a new sound did not replace the old: %v", calls)
	}
	g.Enable(false)
	if !slices.Equal(calls, []string{"a", "b"}) {
		t.Fatalf("turning sounds off did not cancel: %v", calls)
	}
	g.Voice("listening")
	g.Track(func() { calls = append(calls, "c") })
	if !slices.Equal(calls, []string{"a", "b", "c"}) {
		t.Fatalf("a sound tracked after the gate closed kept playing: %v", calls)
	}
}

// fakeSink feeds each sound through the real pacing with a fake clock:
// sounds advance only when the test pumps them.
type fakeSink struct {
	now     *time.Time
	plays   []*fakePlay
	refuses error
}

type fakePlay struct {
	pcm     []float32
	start   time.Time
	st      *stream
	out     []float32
	cancels []time.Time
}

func (f *fakeSink) Play(pcm []float32, rateHz int) (func(), error) {
	if f.refuses != nil {
		return nil, f.refuses
	}
	p := &fakePlay{pcm: pcm, start: *f.now, st: newStream(pcm, rateHz, muteRamp())}
	f.plays = append(f.plays, p)
	return func() {
		p.cancels = append(p.cancels, *f.now)
		p.st.cancel(p.played(*f.now))
	}, nil
}

func (p *fakePlay) played(now time.Time) int { return samplesOf(now.Sub(p.start), RateHz) }

func (p *fakePlay) pump(now time.Time) { p.out = append(p.out, p.st.pump(p.played(now))...) }

func newPlayer(t *testing.T, notes map[string][]int) (*Player, *fakeSink, *time.Time, *int) {
	t.Helper()
	now := t0
	sink := &fakeSink{now: &now}
	opens := 0
	p := NewPlayer(loadSpec(t), notes, func() (Sink, error) {
		opens++
		return sink, nil
	})
	return p, sink, &now, &opens
}

func TestPlayerOpensNothingWhileOffOrIdle(t *testing.T) {
	p, sink, now, opens := newPlayer(t, nil)
	playAll := func() {
		for _, ev := range spec.EventIDs {
			*now = now.Add(time.Second)
			if got, err := p.Play(*now, ev); got != "" || err != nil {
				t.Fatalf("%s played %q (err %v) through a closed gate", ev, got, err)
			}
		}
	}
	playAll()
	p.Gate().Enable(true)
	playAll()
	p.Gate().Voice("listening")
	playAll()
	if *opens != 0 || len(sink.plays) != 0 {
		t.Fatalf("closed gate opened the sink %d times, %d plays", *opens, len(sink.plays))
	}
	p.Gate().Voice("idle")
	if got, err := p.Play(*now, "gesture-done"); got != "done" || err != nil {
		t.Fatalf("open gate played %q, %v", got, err)
	}
	*now = now.Add(time.Second)
	if got, _ := p.Play(*now, "strand-arrive"); got != "arrive" {
		t.Fatalf("second event played %q", got)
	}
	if *opens != 1 || len(sink.plays) != 2 {
		t.Fatalf("sink opened %d times for %d plays, want once", *opens, len(sink.plays))
	}
	if got, _ := p.Play(*now, "no-such-event"); got != "" {
		t.Fatalf("an unknown event played %q", got)
	}
}

func TestPlayerSurfacesSinkErrors(t *testing.T) {
	p, sink, now, _ := newPlayer(t, nil)
	p.Gate().Enable(true)
	p.Gate().Voice("idle")
	sink.refuses = errTest
	if _, err := p.Play(*now, "gesture-done"); err != errTest {
		t.Fatalf("sink error %v, want %v", err, errTest)
	}
}

var errTest = &testError{}

type testError struct{}

func (*testError) Error() string { return "refused" }

// A microphone opening mid-sound cancels it at once, and the sound is
// silent by the spec's quiet bound after that instant.
func TestMicOpenSilencesWithinQuietBound(t *testing.T) {
	s := loadSpec(t)
	p, sink, now, _ := newPlayer(t, nil)
	p.Gate().Enable(true)
	p.Gate().Voice("idle")
	if got, _ := p.Play(*now, "gesture-done"); got != "done" {
		t.Fatalf("played %q", got)
	}
	play := sink.plays[0]
	step := ms(tickMs)
	for range 10 { // 50 ms into the motif
		*now = now.Add(step)
		play.pump(*now)
	}
	open := *now
	p.Gate().Voice("listening")
	if len(play.cancels) != 1 || !play.cancels[0].Equal(open) {
		t.Fatalf("mic open cancelled at %v, want exactly %v", play.cancels, open)
	}
	for !play.st.done() {
		*now = now.Add(step)
		play.pump(*now)
	}
	quiet := samplesOf(millis(s.Sound.Gating.QuietMs), RateHz)
	cut := play.played(open)
	last := -1
	for i, v := range play.out {
		if v != 0 {
			last = i
		}
	}
	if last >= cut+quiet {
		t.Fatalf("sound continued to sample %d, past the mic-open %d + %d", last, cut, quiet)
	}
	if len(play.out) >= len(play.pcm) {
		t.Fatal("the cancelled sound played to its end")
	}
	if p.Gate().Allow(now.Add(time.Second), "done") {
		t.Fatal("gate opened while the microphone was live")
	}
}

func TestStreamPacingAndMuteRamp(t *testing.T) {
	s := loadSpec(t)
	ramp := samplesOf(millis(s.Sound.Gating.MuteRampMs), RateHz)
	if ramp != 960 {
		t.Fatalf("mute ramp %d samples, want 20 ms at 48 kHz", ramp)
	}
	pcm := make([]float32, RateHz) // one second of a constant level
	for i := range pcm {
		pcm[i] = 0.5
	}
	st := newStream(pcm, RateHz, muteRamp())
	var out []float32
	played := 0
	tick := samplesOf(millis(tickMs), RateHz)
	for range 20 {
		out = append(out, st.pump(played)...)
		if len(out) > played+st.lead {
			t.Fatalf("wrote %d samples at play position %d, more than the lead ahead", len(out), played)
		}
		played += tick
	}
	cut := played - tick/2
	st.cancel(cut)
	from := len(out)
	for !st.done() {
		out = append(out, st.pump(played)...)
		played += tick
	}
	if len(out) != cut+ramp {
		t.Fatalf("cancelled stream wrote %d samples, want the cancel %d + ramp %d", len(out), cut, ramp)
	}
	if out[from-1] != 0.5 {
		t.Fatal("samples written before the cancel changed")
	}
	for i := from; i < len(out); i++ {
		if out[i] > out[i-1] || out[i] < 0 {
			t.Fatalf("ramp not falling at %d: %v after %v", i, out[i], out[i-1])
		}
	}
	if end := float64(out[len(out)-1]); end > 0.5/float64(cut+ramp-from)+1e-7 {
		t.Fatalf("ramp ends at %v, not silence", end)
	}
	st.cancel(played)
	if st.pump(played+RateHz) != nil {
		t.Fatal("a second cancel resumed writing")
	}
}

func TestStreamCancelNearTheEnd(t *testing.T) {
	pcm := make([]float32, 1000)
	for i := range pcm {
		pcm[i] = 1
	}
	st := newStream(pcm, RateHz, muteRamp())
	out := st.pump(990 - st.lead)
	st.cancel(985)
	out = append(out, st.pump(2000)...)
	if len(out) != len(pcm) || out[len(out)-1] >= 1 {
		t.Fatalf("a cancel near the end wrote %d samples ending at %v", len(out), out[len(out)-1])
	}
}

func TestProfileMotifOverride(t *testing.T) {
	s := loadSpec(t)
	var a profile.Animations
	a.Motifs = map[string][]int{"done": {60, 67}, "poke": {200}, "missing": {60}}
	motifs := Motifs(&s.Sound, a.Motifs)
	spDone, _ := s.Motif("done")
	spPoke, _ := s.Motif("poke")
	done := motifs["done"]
	if !slices.Equal(done.Notes, []int{60, 67}) || done.SpacingS != spDone.SpacingS ||
		done.LengthS != spDone.LengthS || done.DelayS != spDone.DelayS {
		t.Fatalf("override gave %+v from %+v", done, spDone)
	}
	if !slices.Equal(motifs["poke"].Notes, spPoke.Notes) {
		t.Fatal("an out-of-range override replaced the spec's notes")
	}
	if _, ok := motifs["missing"]; ok || len(motifs) != len(s.Sound.Motifs) {
		t.Fatal("an unknown motif id was added")
	}

	p, sink, now, _ := newPlayer(t, a.Motifs)
	p.Gate().Enable(true)
	p.Gate().Voice("idle")
	if got, _ := p.Play(*now, "gesture-done"); got != "done" {
		t.Fatalf("played %q", got)
	}
	checkMotif(t, "done (override)", done, s.Sound.Synth, sink.plays[0].pcm)
	if !slices.Equal(sink.plays[0].pcm, Render(done, s.Sound.Synth, RateHz)) {
		t.Fatal("player rendered a different buffer than Render")
	}
}
