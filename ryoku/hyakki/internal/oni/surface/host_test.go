package surface

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"ryoku-rashin/internal/oni/feature"
	"ryoku-rashin/internal/oni/placement"
	"ryoku-rashin/internal/oni/profile"
	"ryoku-rashin/internal/oni/sound"
	"ryoku-rashin/internal/oni/spec"
	"ryoku-rashin/internal/oni/strand"
)

// fakeDisplay commits everything at once and has whatever outputs the test
// gives it. Posted work (pacing timers, daemon goroutines) is held, never
// run unless the test drains it, so only the test steps the host.
type fakeDisplay struct {
	outs      []placement.Output
	presented []presentation
	strands   []strandPresentation
	unmapped  int // hideStrand calls that took a strand down
	mapped    bool
	cover     bool

	overlays [overlayCount][]overlayPresentation
	ovMapped [overlayCount]bool
	ovHidden [overlayCount]int

	// picks is every setPick call; picked the outputs mapped now
	picks  [][]string
	picked []string

	// glows is every glow commit; glowOn the outputs whose glow surfaces
	// exist; glowWait holds back configuring them
	glows    []glowPresentation
	glowOn   map[string]bool
	glowWait bool

	mu     sync.Mutex
	posted int
	queue  []func()
}

func (f *fakeDisplay) run(ctx context.Context) error { <-ctx.Done(); return nil }
func (f *fakeDisplay) post(fn func()) {
	f.mu.Lock()
	f.posted++
	f.queue = append(f.queue, fn)
	f.mu.Unlock()
}

// drain runs posted work on the test goroutine, as the display loop would,
// until done holds or 3 s pass.
func (f *fakeDisplay) drain(t *testing.T, done func() bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for {
		f.mu.Lock()
		q := f.queue
		f.queue = nil
		f.mu.Unlock()
		for _, fn := range q {
			fn()
		}
		if done() {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("posted work never got there")
		}
		time.Sleep(2 * time.Millisecond)
	}
}
func (f *fakeDisplay) outputs() []placement.Output { return f.outs }
func (f *fakeDisplay) scale(string) float64        { return 1 }
func (f *fakeDisplay) present(p presentation) (image.Point, bool, error) {
	f.presented = append(f.presented, p)
	return p.origin, true, nil
}
func (f *fakeDisplay) hide()                  {}
func (f *fakeDisplay) covered(string) bool    { return f.cover }
func (f *fakeDisplay) foreignToplevels() bool { return false }
func (f *fakeDisplay) close()                 {}
func (f *fakeDisplay) strandScale(_ string, limit float64) float64 {
	return min(1, limit)
}
func (f *fakeDisplay) presentStrand(p strandPresentation) (bool, error) {
	f.strands = append(f.strands, p)
	f.mapped = true
	return true, nil
}
func (f *fakeDisplay) hideStrand() {
	if f.mapped {
		f.unmapped++
	}
	f.mapped = false
}
func (f *fakeDisplay) presentOverlay(which overlay, p overlayPresentation) (bool, error) {
	f.overlays[which] = append(f.overlays[which], p)
	f.ovMapped[which] = true
	return true, nil
}
func (f *fakeDisplay) hideOverlay(which overlay) {
	if f.ovMapped[which] {
		f.ovHidden[which]++
	}
	f.ovMapped[which] = false
}
func (f *fakeDisplay) setPick(outputs []string) {
	f.picks = append(f.picks, append([]string(nil), outputs...))
	f.picked = append([]string(nil), outputs...)
}

func (f *fakeDisplay) glowScale(string) float64 { return glowDPR }
func (f *fakeDisplay) presentGlow(p glowPresentation) (bool, error) {
	if f.glowOn == nil {
		f.glowOn = map[string]bool{}
	}
	f.glowOn[p.output] = true
	if f.glowWait {
		return false, nil
	}
	f.glows = append(f.glows, p)
	return true, nil
}
func (f *fakeDisplay) hideGlow(keep []string) {
	for id := range f.glowOn {
		if !slices.Contains(keep, id) {
			delete(f.glowOn, id)
		}
	}
}

func testHost(t *testing.T, outs ...placement.Output) (*host, *fakeDisplay, *bytes.Buffer) {
	t.Helper()
	sp, err := spec.Load()
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	ctx, cancel := context.WithCancel(context.Background())
	h := newHost(ctx, cancel, sp, Options{Lab: Lab{TimeScale: 1}, Ledger: newLedger(&buf, "wayland-test", time.Now)})
	t.Cleanup(func() { cancel(); h.stopTimer() })
	d := &fakeDisplay{outs: outs}
	h.be = d
	// no daemon here: calls for one queue instead of dialling
	h.linking = true
	loaded := profile.Default()
	h.setCharacter(loaded.Profile.ID, loaded.Profile, loaded.Files)
	return h, d, &buf
}

func ledgerLines(t *testing.T, buf *bytes.Buffer, ev string) []Entry {
	t.Helper()
	var out []Entry
	for _, line := range strings.Split(strings.TrimSpace(buf.String()), "\n") {
		var e Entry
		if err := json.Unmarshal([]byte(line), &e); err != nil {
			t.Fatalf("ledger line %q: %v", line, err)
		}
		if e.Ev == ev {
			out = append(out, e)
		}
	}
	return out
}

func TestAScaleChangeOnTheCharactersOutputIsADisplayChange(t *testing.T) {
	h, d, buf := testHost(t, placement.Output{ID: "winit", Rect: image.Rect(0, 0, 1920, 1080), Scale: 1})
	if n := len(ledgerLines(t, buf, EvOutput)); n != 1 {
		t.Fatalf("output lines after start = %d, want 1", n)
	}
	d.outs = []placement.Output{{ID: "winit", Rect: image.Rect(0, 0, 1280, 720), Scale: 1.5}}
	h.outputsChanged()
	outs := ledgerLines(t, buf, EvOutput)
	if len(outs) != 2 {
		t.Fatalf("output lines after the scale change = %d, want 2", len(outs))
	}
	got := outs[1]
	if got.Output != "winit" || got.Scale != 1.5 || got.Rect == nil {
		t.Fatalf("scale change line = %+v", got)
	}
	if r := got.Rect; r[0]+r[2] > 1280 || r[1]+r[3] > 720 {
		t.Fatalf("the character's rect %v is outside the 1280x720 output", *r)
	}
	h.outputsChanged()
	if n := len(ledgerLines(t, buf, EvOutput)); n != 2 {
		t.Fatalf("an unchanged output was recorded again: %d lines", n)
	}
}

func TestLosingItsOutputFliesTheCharacterBackInSight(t *testing.T) {
	a := placement.Output{ID: "A", Rect: image.Rect(0, 0, 1920, 1080), Scale: 1}
	b := placement.Output{ID: "B", Rect: image.Rect(1920, 0, 3840, 1080), Scale: 1}
	h, d, buf := testHost(t, a, b)
	h.applyPlacement(&feature.PlacementFrame{Seq: 1, Size: "normal", Active: "B",
		Outputs: []feature.PlacementOutput{{ID: "A", FX: 0.3, FY: 0.4}, {ID: "B", FX: 0.5, FY: 0.5}}})
	if h.fl != nil || h.out.ID != "B" {
		t.Fatalf("the first placement: flying=%v on %s, want a jump onto B", h.fl != nil, h.out.ID)
	}
	d.outs = []placement.Output{a}
	h.outputsChanged()
	if h.out.ID != "A" || h.fl == nil {
		t.Fatalf("after B went away: on %s, flying=%v; want a flight on A", h.out.ID, h.fl != nil)
	}
	ext := h.extent()
	if r := (image.Rectangle{Min: h.origin, Max: h.origin.Add(ext)}); !r.In(a.Rect) {
		t.Fatalf("the flight starts at %v, not on A where it can be seen", r)
	}
	want := placement.Resolve(a, ext, "", &placement.Saved{FX: 0.3, FY: 0.4}, h.margin())
	if end := h.fl.path(1); end != want {
		t.Fatalf("the flight ends at %v, want A's remembered place %v", end, want)
	}
	outs := ledgerLines(t, buf, EvOutput)
	if last := outs[len(outs)-1]; last.Output != "A" {
		t.Fatalf("last output line = %+v, want A", last)
	}
}

func TestAPressHoldsTheLayerSurfaceWhereItWasPressed(t *testing.T) {
	h, d, _ := testHost(t, placement.Output{ID: "winit", Rect: image.Rect(0, 0, 1920, 1080), Scale: 1})
	start := h.origin
	ext := h.extent()
	layer := spriteOrigin{applied: start}
	// every presentation reaches a compositor that draws it at once
	present := func() presentation {
		h.step(time.Now())
		p := d.presented[len(d.presented)-1]
		if layer.place(p) {
			layer.rendered()
		}
		return p
	}
	centre := spec.Point{X: float64(start.X + ext.X/2), Y: float64(start.Y + ext.Y/2)}
	h.pointerMotion(centre)
	h.button(true)
	for i := 1; i <= 4; i++ {
		h.pointerMotion(spec.Point{X: centre.X - float64(50*i), Y: centre.Y - float64(25*i)})
		p := present()
		if want := start.Add(image.Pt(-50*i, -25*i)); p.origin != want {
			t.Fatalf("drag step %d: sprite at %v, want %v", i, p.origin, want)
		}
		if layer.requested() != start {
			t.Fatalf("drag step %d: the layer surface moved to %v during the press", i, layer.requested())
		}
	}
	h.button(false)
	p := present()
	if p.grab || layer.requested() != p.origin || p.origin != start.Add(image.Pt(-200, -100)) {
		t.Fatalf("after the drop: grab=%v layer surface at %v, sprite at %v", p.grab, layer.requested(), p.origin)
	}
}

// linkedHost is a shown host on one 1920x1080 output whose link is a
// scripted daemon that records strand arrivals.
func linkedHost(t *testing.T) (*host, *fakeDisplay, *bytes.Buffer, *fakeDaemon) {
	t.Helper()
	h, d, buf := testHost(t, placement.Output{ID: "winit", Rect: image.Rect(0, 0, 1920, 1080), Scale: 1})
	fd := newFakeDaemon(nil)
	t.Cleanup(func() { fd.Close() })
	h.link, h.linking = fd, false
	if !h.shown {
		t.Fatal("the test host is not on screen")
	}
	return h, d, buf, fd
}

func send(t *testing.T, h *host, p feature.Payload) {
	t.Helper()
	h.frame(h.link, encode(t, p))
}

// drive steps the host n times, 16 ms of time apart, as frame callbacks
// would.
func drive(h *host, now time.Time, n int) time.Time {
	for range n {
		now = now.Add(16 * time.Millisecond)
		h.step(now)
	}
	return now
}

// arrivals waits for n oni.strandArrived calls, then a little longer so a
// duplicate would show.
func arrivals(t *testing.T, fd *fakeDaemon, n int) []Arrival {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for len(fd.Arrivals()) < n && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	time.Sleep(50 * time.Millisecond)
	return fd.Arrivals()
}

func TestTheStrandReportsEachArrivalOnce(t *testing.T) {
	h, d, buf, fd := linkedHost(t)
	h.lab.Power = PowerSaver
	now := time.Now()
	send(t, h, &feature.StrandTargetFrame{Seq: 1, X: 0.3, Y: 0.2, W: 0.04, H: 0.03})
	now = drive(h, now, 90)
	send(t, h, &feature.StrandTargetFrame{Seq: 2, X: 0.5, Y: 0.4, W: 0.04, H: 0.03})
	now = drive(h, now, 90)
	send(t, h, &feature.StrandTargetFrame{Seq: 2, X: 0.1, Y: 0.1, W: 0.04, H: 0.03})
	send(t, h, &feature.StrandTargetFrame{Seq: 1, X: 0.1, Y: 0.1, W: 0.04, H: 0.03})
	drive(h, now, 60)

	got := arrivals(t, fd, 2)
	if len(got) != 2 || got[0].Seq != 1 || got[1].Seq != 2 || got[0].Skipped != "" || got[1].Skipped != "" {
		t.Fatalf("arrivals = %+v, want 1 then 2, each once and landed", got)
	}
	if n := len(ledgerLines(t, buf, EvStrandTarget)); n != 2 {
		t.Fatalf("the engine took %d targets, want 2 (stale seqs dropped)", n)
	}
	if n := len(ledgerLines(t, buf, EvStrandArrive)); n != 2 {
		t.Fatalf("%d arrive lines, want 2", n)
	}
	if len(d.strands) == 0 {
		t.Fatal("the strand was never presented")
	}
	reaching := 0
	for _, e := range ledgerLines(t, buf, EvStep) {
		if e.Strand != nil && e.Strand.Phase == strand.PhaseReach {
			reaching++
			if e.FPS != 60 {
				t.Fatalf("a reach step on battery saver ran at %d fps, want the 60 fps motion tier", e.FPS)
			}
		}
	}
	if reaching == 0 {
		t.Fatal("no step drew the reach")
	}
}

func TestASkippedTargetReportsWhy(t *testing.T) {
	h, d, buf, fd := linkedHost(t)
	now := drive(h, time.Now(), 2)
	send(t, h, &feature.StrandTargetFrame{Seq: 1, X: 0.1, Y: 0.1, W: 0.8, H: 0.8})
	drive(h, now, 3)
	d.cover = true
	h.coveredChanged()
	send(t, h, &feature.StrandTargetFrame{Seq: 2, X: 0.3, Y: 0.2, W: 0.04, H: 0.03})

	got := arrivals(t, fd, 2)
	if len(got) != 2 || got[0] != (Arrival{T: got[0].T, Seq: 1, Skipped: feature.SkipCoversScreen}) ||
		got[1] != (Arrival{T: got[1].T, Seq: 2, Skipped: feature.SkipCovered}) {
		t.Fatalf("arrivals = %+v, want 1 covers-screen then 2 covered", got)
	}
	if len(d.strands) != 0 {
		t.Fatalf("a skipped target drew %d strand frames", len(d.strands))
	}
	if n := len(ledgerLines(t, buf, EvStrandSkip)); n != 2 {
		t.Fatalf("%d skip lines, want 2", n)
	}
}

func TestReducedMotionCommitsOneStrandFramePerEvent(t *testing.T) {
	h, d, buf, fd := linkedHost(t)
	send(t, h, &feature.MotionFrame{Reduce: "on", Courtesy: true})
	now := drive(h, time.Now(), 5)
	send(t, h, &feature.StrandTargetFrame{Seq: 1, X: 0.3, Y: 0.2, W: 0.04, H: 0.03, Mode: feature.StrandModeLasso})
	now = drive(h, now, 20)
	if len(d.strands) != 1 {
		t.Fatalf("a reduced-motion target committed %d strand frames, want 1", len(d.strands))
	}
	if got := arrivals(t, fd, 1); len(got) != 1 || got[0].Seq != 1 || got[0].Skipped != "" {
		t.Fatalf("arrivals = %+v, want 1 landed at once", got)
	}
	send(t, h, &feature.StrandRetractFrame{Seq: 1})
	drive(h, now, 20)
	if len(d.strands) != 1 || d.unmapped != 1 {
		t.Fatalf("after the retract: %d strand frames, %d unmaps; want the one frame and one unmap", len(d.strands), d.unmapped)
	}
	if !h.clk.stopped || h.clk.armed || h.clk.waiting || h.strandRun {
		t.Fatalf("at rest the clock is stopped=%v armed=%v waiting=%v, strand running=%v", h.clk.stopped, h.clk.armed, h.clk.waiting, h.strandRun)
	}
	if h.strandImg != nil {
		t.Fatal("the strand's buffer outlived the thread")
	}
	if n := len(ledgerLines(t, buf, EvStrandArrive)); n != 1 {
		t.Fatalf("%d arrive lines, want 1", n)
	}
}

func TestCourtesyHideTakesTheStrandDown(t *testing.T) {
	h, d, buf, fd := linkedHost(t)
	send(t, h, &feature.StrandTargetFrame{Seq: 1, X: 0.3, Y: 0.2, W: 0.04, H: 0.03})
	now := drive(h, time.Now(), 10)
	if len(d.strands) == 0 {
		t.Fatal("the reach drew nothing")
	}
	drawn := len(d.strands)
	d.cover = true
	h.coveredChanged()
	if d.unmapped != 1 || h.strandShown || h.strd.Out() {
		t.Fatalf("after the courtesy hide: unmaps=%d shown=%v out=%v", d.unmapped, h.strandShown, h.strd.Out())
	}
	if got := arrivals(t, fd, 1); len(got) != 1 || got[0].Seq != 1 || got[0].Skipped != feature.SkipCovered {
		t.Fatalf("arrivals = %+v, want 1 skipped covered", got)
	}
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	hid := -1
	for i, line := range lines {
		if strings.Contains(line, `"ev":"hide"`) && strings.Contains(line, `"part":"strand"`) {
			hid = i
		}
	}
	if hid < 0 {
		t.Fatal("no strand hide line")
	}
	for _, line := range lines[hid:] {
		if strings.Contains(line, `"ev":"commit"`) {
			t.Fatalf("a commit after the hide: %s", line)
		}
	}

	d.cover = false
	h.coveredChanged()
	drive(h, now, 30)
	if len(d.strands) != drawn || d.mapped {
		t.Fatalf("the strand came back after the hide: %d frames, mapped=%v", len(d.strands), d.mapped)
	}
	if got := arrivals(t, fd, 1); len(got) != 1 {
		t.Fatalf("arrivals after showing again = %+v", got)
	}
}

type fakeSink struct {
	mu             sync.Mutex
	plays, cancels int
}

func (s *fakeSink) Play([]float32, int) (func(), error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.plays++
	return func() {
		s.mu.Lock()
		s.cancels++
		s.mu.Unlock()
	}, nil
}

func (s *fakeSink) counts() (plays, cancels int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.plays, s.cancels
}

func TestSoundsStaySilentUnlessTheMicIsKnownIdle(t *testing.T) {
	h, _, buf, _ := linkedHost(t)
	sink := &fakeSink{}
	opened := 0
	h.openSink = func() (sound.Sink, error) { opened++; return sink, nil }
	h.newPlayer(profile.Default().Profile)
	send(t, h, &feature.MotionFrame{Reduce: "follow", Courtesy: true, Sounds: true})

	h.play("poke")
	send(t, h, &feature.VoiceFrame{Phase: "listening"})
	h.play("poke")
	if p, _ := sink.counts(); opened != 0 || p != 0 {
		t.Fatalf("before an idle voice frame: sink opened %d times, %d plays", opened, p)
	}
	send(t, h, &feature.VoiceFrame{Phase: "idle"})
	h.play("poke")
	if p, c := sink.counts(); opened != 1 || p != 1 || c != 0 {
		t.Fatalf("with the mic idle: opened %d, %d plays, %d cancels; want one sound", opened, p, c)
	}
	send(t, h, &feature.VoiceFrame{Phase: "listening"})
	if _, c := sink.counts(); c != 1 {
		t.Fatalf("the mic opening cancelled %d sounds, want the one playing", c)
	}
	send(t, h, &feature.VoiceFrame{Phase: "idle"})
	h.linking = true // no redial from the loss
	h.lost(h.link)
	time.Sleep(100 * time.Millisecond) // past the debounce: only the gate can hold it
	h.play("poke")
	if p, _ := sink.counts(); p != 1 {
		t.Fatalf("after the link dropped: %d plays, want none new", p)
	}
	var reasons []string
	for _, e := range ledgerLines(t, buf, EvSound) {
		reasons = append(reasons, e.Reason)
	}
	if strings.Join(reasons, ",") != "gated,gated,played,gated" {
		t.Fatalf("sound lines = %v", reasons)
	}
}

// armed is the microphone open for the wake phrase or open mic: the gate
// closes and the motif playing is cut, exactly as for a press.
func TestArmedClosesTheSoundGateAndCutsTheMotif(t *testing.T) {
	h, _, buf, _ := linkedHost(t)
	sink := &fakeSink{}
	h.openSink = func() (sound.Sink, error) { return sink, nil }
	h.newPlayer(profile.Default().Profile)
	send(t, h, &feature.MotionFrame{Reduce: "follow", Courtesy: true, Sounds: true})
	send(t, h, &feature.VoiceFrame{Phase: feature.VoiceIdle})
	h.play("poke")
	if p, c := sink.counts(); p != 1 || c != 0 {
		t.Fatalf("with the mic idle: %d plays, %d cancels", p, c)
	}
	for _, source := range []string{feature.VoiceSourceWake, feature.VoiceSourceOpen} {
		send(t, h, &feature.VoiceFrame{Phase: feature.VoiceArmed, Source: source})
		h.play("poke")
		send(t, h, &feature.VoiceFrame{Phase: feature.VoiceIdle})
	}
	if p, c := sink.counts(); p != 1 || c != 1 {
		t.Fatalf("armed: %d plays, %d cancels; want the playing motif cut and none new", p, c)
	}
	var reasons []string
	for _, e := range ledgerLines(t, buf, EvSound) {
		reasons = append(reasons, e.Reason)
	}
	if strings.Join(reasons, ",") != "played,gated,gated" {
		t.Fatalf("sound lines = %v", reasons)
	}
}

// stepInputs drives the host and returns the mic and speech levels its
// steps handed the motion engine.
func stepInputs(t *testing.T, h *host, buf *bytes.Buffer, now time.Time) (mic, speech float64) {
	t.Helper()
	buf.Reset()
	drive(h, now, 3)
	steps := ledgerLines(t, buf, EvStep)
	if len(steps) == 0 {
		t.Fatal("the host did not step")
	}
	raw, err := json.Marshal(steps[len(steps)-1].Input)
	if err != nil {
		t.Fatal(err)
	}
	var in struct{ Mic, Speech float64 }
	if err := json.Unmarshal(raw, &in); err != nil {
		t.Fatal(err)
	}
	return in.Mic, in.Speech
}

// Level frames move the character only while the daemon is listening or
// speaking; the levels drop to nothing when that phase or the link ends.
func TestLevelsFeedTheMotionEngineOnlyWhileListeningOrSpeaking(t *testing.T) {
	h, _, buf, _ := linkedHost(t)
	now := time.Now()
	level := &feature.LevelFrame{Mic: 0.6, Speech: 0.3}
	send(t, h, &feature.VoiceFrame{Phase: feature.VoiceArmed, Source: feature.VoiceSourceWake})
	send(t, h, level)
	if mic, speech := stepInputs(t, h, buf, now); mic != 0 || speech != 0 {
		t.Fatalf("a level while armed moved Oni: mic %v speech %v", mic, speech)
	}
	send(t, h, &feature.VoiceFrame{Phase: feature.VoiceListening, Source: feature.VoiceSourceWake})
	send(t, h, level)
	if mic, speech := stepInputs(t, h, buf, now.Add(time.Second)); mic != 0.6 || speech != 0.3 {
		t.Fatalf("listening fed mic %v speech %v", mic, speech)
	}
	send(t, h, &feature.VoiceFrame{Phase: feature.VoiceTranscribing, Source: feature.VoiceSourceWake, Text: "hello"})
	if mic, speech := stepInputs(t, h, buf, now.Add(2*time.Second)); mic != 0 || speech != 0 {
		t.Fatalf("the levels outlived listening: mic %v speech %v", mic, speech)
	}
	send(t, h, level)
	if mic, _ := stepInputs(t, h, buf, now.Add(3*time.Second)); mic != 0 {
		t.Fatal("a stale level moved Oni while transcribing")
	}
	send(t, h, &feature.VoiceFrame{Phase: feature.VoiceSpeaking, Source: feature.VoiceSourceWake, Text: "It's three."})
	send(t, h, &feature.LevelFrame{Speech: 0.8})
	if mic, speech := stepInputs(t, h, buf, now.Add(4*time.Second)); mic != 0 || speech != 0.8 {
		t.Fatalf("speaking fed mic %v speech %v", mic, speech)
	}
	h.linking = true // no redial from the loss
	h.lost(h.link)
	if h.level != (feature.LevelFrame{}) {
		t.Fatalf("the link dropped and the levels stayed: %+v", h.level)
	}
}

// A dictation's words never reach the ledger: a frame that carried them is
// refused whole, and a valid dictation frame is logged by kind alone.
func TestADictationsWordsNeverReachTheLedger(t *testing.T) {
	h, _, buf, _ := linkedHost(t)
	const words = "hunter2 is my password"
	h.frame(h.link, json.RawMessage(`{"kind":"voice","data":{"phase":"transcribing","text":"`+words+`","source":"dictation"}}`))
	if h.voice == feature.VoiceTranscribing {
		t.Fatal("the host took a dictation frame carrying its words")
	}
	send(t, h, &feature.VoiceFrame{Phase: feature.VoiceListening, Source: feature.VoiceSourceDictation})
	send(t, h, &feature.VoiceFrame{Phase: feature.VoiceTranscribing, Source: feature.VoiceSourceDictation})
	if h.voice != feature.VoiceTranscribing {
		t.Fatalf("the host is in %q", h.voice)
	}
	if strings.Contains(buf.String(), "hunter2") {
		t.Fatalf("the ledger holds the dictation:\n%s", buf.String())
	}
	if got := ledgerLines(t, buf, EvFrame); len(got) < 3 || got[len(got)-3].Reason != "invalid" {
		t.Fatalf("frame lines = %+v", got)
	}
}
