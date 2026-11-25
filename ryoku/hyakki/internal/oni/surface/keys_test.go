package surface

import (
	"bytes"
	"encoding/json"
	"errors"
	"image"
	"image/color"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/design"

	"ryoku-rashin/internal/oni/feature"
	"ryoku-rashin/internal/oni/keys"
	"ryoku-rashin/internal/oni/render"
	"ryoku-rashin/internal/protocol"
)

// fakeKeys stands in for the keyboards: the test sends the edges.
type fakeKeys struct {
	mu      sync.Mutex
	started []string
	live    *fakeSource
	err     error
}

type fakeSource struct {
	edge    func(keys.Edge)
	stopped bool
}

func (s *fakeSource) Stop()        { s.stopped = true }
func (s *fakeSource) Kind() string { return "evdev" }

func (f *fakeKeys) start(chord string, edge func(keys.Edge)) (keySource, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.started = append(f.started, chord)
	if f.err != nil {
		return nil, f.err
	}
	f.live = &fakeSource{edge: edge}
	return f.live, nil
}

func (f *fakeKeys) press(t *testing.T, d *fakeDisplay, edges ...keys.Edge) {
	t.Helper()
	for _, e := range edges {
		f.live.edge(e)
	}
	d.drain(t, func() bool { return true })
}

func keyedHost(t *testing.T) (*host, *fakeDisplay, *bytes.Buffer, *fakeDaemon, *fakeKeys) {
	t.Helper()
	h, d, buf, fd := linkedHost(t)
	fk := &fakeKeys{}
	h.startKeys = fk.start
	return h, d, buf, fd, fk
}

func talks(t *testing.T, fd *fakeDaemon, n int) []string {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for len(fd.Talks()) < n {
		if time.Now().After(deadline) {
			t.Fatalf("talks = %q, want %d", fd.Talks(), n)
		}
		time.Sleep(2 * time.Millisecond)
	}
	time.Sleep(20 * time.Millisecond)
	return fd.Talks()
}

func voiceOn(chord string) *feature.MotionFrame {
	return &feature.MotionFrame{Reduce: "follow", Courtesy: true, Chord: chord}
}

// markLook is a look with every role the mark wears set apart.
func markLook(d design.Style) render.Look {
	return render.Look{
		Ink:    color.NRGBA{R: 20, G: 20, B: 24, A: 255},
		White:  color.NRGBA{R: 240, G: 236, B: 228, A: 255},
		Body:   color.NRGBA{R: 120, G: 118, B: 112, A: 255},
		Accent: map[string]color.NRGBA{"listening": {R: 230, G: 170, B: 40, A: 255}},
		Design: d,
	}
}

func TestTheKeyReaderRunsOnlyWhileVoiceIsOn(t *testing.T) {
	h, _, buf, _, fk := keyedHost(t)
	send(t, h, &feature.MotionFrame{Reduce: "follow", Courtesy: true})
	send(t, h, &feature.MotionFrame{Reduce: "on", Courtesy: true})
	if len(fk.started) != 0 || h.keySrc != nil {
		t.Fatalf("voice off started a reader: %q", fk.started)
	}
	send(t, h, voiceOn("ctrl+alt"))
	first := fk.live
	send(t, h, &feature.MotionFrame{Reduce: "on", Courtesy: true, Chord: "ctrl+alt"})
	send(t, h, voiceOn("alt+super+t"))
	if !slices.Equal(fk.started, []string{"ctrl+alt", "alt+super+t"}) || !first.stopped || fk.live.stopped {
		t.Fatalf("started %q, first stopped %v", fk.started, first.stopped)
	}
	send(t, h, &feature.MotionFrame{Reduce: "follow", Courtesy: true})
	if !fk.live.stopped || h.keySrc != nil {
		t.Fatal("turning voice off left the reader running")
	}
	var reasons []string
	for _, e := range ledgerLines(t, buf, EvKeys) {
		reasons = append(reasons, e.Reason)
	}
	if !slices.Equal(reasons, []string{"evdev", "stopped", "evdev", "stopped"}) {
		t.Fatalf("keys lines %q", reasons)
	}
}

func TestChordEdgesBecomeTalkCallsInOrder(t *testing.T) {
	h, d, buf, fd, fk := keyedHost(t)
	send(t, h, voiceOn("ctrl+alt"))
	fk.press(t, d, keys.Down, keys.Up, keys.Down, keys.Cancel)
	want := []string{protocol.OniVoiceStart, protocol.OniVoiceStop, protocol.OniVoiceStart, protocol.OniVoiceCancel}
	if got := talks(t, fd, 4); !slices.Equal(got, want) {
		t.Fatalf("talks %q, want %q", got, want)
	}
	var events []string
	for _, e := range ledgerLines(t, buf, EvChord) {
		events = append(events, e.Event)
	}
	if !slices.Equal(events, []string{"down", "up", "down", "cancel"}) {
		t.Fatalf("chord lines %q", events)
	}
	// no ledger line names a key
	for _, line := range strings.Split(buf.String(), "\n") {
		var raw map[string]any
		if line == "" || json.Unmarshal([]byte(line), &raw) != nil {
			continue
		}
		for _, f := range []string{"code", "key", "keycode", "keys"} {
			if _, ok := raw[f]; ok {
				t.Fatalf("ledger line carries %s: %s", f, line)
			}
		}
	}
	// an edge from a reader voice-off stopped is dropped
	stale := fk.live
	send(t, h, &feature.MotionFrame{Reduce: "follow", Courtesy: true})
	stale.edge(keys.Down)
	d.drain(t, func() bool { return true })
	if got := talks(t, fd, 4); len(got) != 4 {
		t.Fatalf("a stopped reader talked: %q", got)
	}
}

func TestAChordWithTheDaemonGoneStartsItAndTalksOnceItOpens(t *testing.T) {
	h, d, buf, fd, fk := keyedHost(t)
	send(t, h, voiceOn("ctrl+alt"))
	// a second loss goes dormant rather than redialling
	h.retried = true
	h.lost(fd)
	// the redial is held as already dialling, as the test host does
	h.linking = true
	fk.press(t, d, keys.Down)
	next := newFakeDaemon(nil)
	t.Cleanup(func() { next.Close() })
	h.opened(next, protocol.OniOpenResult{Presenter: true}, nil)
	if got := talks(t, next, 1); !slices.Equal(got, []string{protocol.OniVoiceStart}) {
		t.Fatalf("talks after open = %q", got)
	}
	fk.press(t, d, keys.Up)
	if got := talks(t, next, 2); !slices.Equal(got, []string{protocol.OniVoiceStart, protocol.OniVoiceStop}) {
		t.Fatalf("talks = %q", got)
	}
	// a tap that ends before the daemon answers sends nothing at all
	h.retried = true
	h.lost(next)
	h.linking = true
	fk.press(t, d, keys.Down, keys.Up)
	third := newFakeDaemon(nil)
	t.Cleanup(func() { third.Close() })
	h.opened(third, protocol.OniOpenResult{Presenter: true}, nil)
	time.Sleep(30 * time.Millisecond)
	if got := third.Talks(); len(got) != 0 {
		t.Fatalf("a finished tap talked to the new daemon: %q", got)
	}
	if len(fd.Talks()) != 0 {
		t.Fatalf("the lost daemon was talked to: %q", fd.Talks())
	}
	_ = buf
}

func TestUnreadableKeyboardsAreReportedNotRetried(t *testing.T) {
	h, _, buf, _, fk := keyedHost(t)
	fk.err = &keys.Unavailable{Reason: keys.ReasonPermission}
	send(t, h, voiceOn("ctrl+alt"))
	lines := ledgerLines(t, buf, EvKeys)
	if len(lines) != 1 || lines[0].Reason != string(keys.ReasonPermission) || h.keySrc != nil {
		t.Fatalf("keys lines %+v", lines)
	}
	send(t, h, &feature.StateFrame{State: "idle"})
	if len(fk.started) != 1 {
		t.Fatalf("the reader was retried: %q", fk.started)
	}
}

func TestTheKeysFIFOIsLabOnly(t *testing.T) {
	env := map[string]string{"HYAKKI_ONI_KEYS_FIFO": "/tmp/oni-lab/keys/chord.fifo"}
	l, err := LabFromEnv(func(k string) string { return env[k] })
	if err != nil || l.KeysFIFO != "" {
		t.Fatalf("outside the lab KeysFIFO = %q, %v", l.KeysFIFO, err)
	}
	env["HYAKKI_ONI_LAB"] = "1"
	if l, _ = LabFromEnv(func(k string) string { return env[k] }); l.KeysFIFO != env["HYAKKI_ONI_KEYS_FIFO"] {
		t.Fatalf("in the lab KeysFIFO = %q", l.KeysFIFO)
	}
	// a lab hook built for a FIFO still refuses it without the lab
	// switch in the process's environment, before any keyboard is touched
	t.Setenv("HYAKKI_ONI_LAB", "")
	src, err := (Lab{KeysFIFO: env["HYAKKI_ONI_KEYS_FIFO"]}).startKeys()("ctrl+alt", func(keys.Edge) {})
	if !errors.Is(err, keys.ErrFIFORefused) || src != nil {
		t.Fatalf("FIFO outside the lab: %v, %v", src, err)
	}
}

func TestTheMicMarkShowsOnlyWhileArmed(t *testing.T) {
	h, d, buf, _ := linkedHost(t)
	h.look = markLook(design.Lithic)
	now := time.Now()
	lastCommit := func() Entry {
		t.Helper()
		lines := ledgerLines(t, buf, EvCommit)
		if len(lines) == 0 {
			t.Fatal("nothing committed")
		}
		return lines[len(lines)-1]
	}
	markPixel := func() bool {
		ext := h.extent()
		r := render.MicMarkRect(ext)
		c := h.img.NRGBAAt(int(r.X+r.W/2), int(r.Y+0.3*r.W))
		want, ok := h.look.Accent["listening"]
		if !ok || want.A == 0 {
			want = h.look.Ink
		}
		return c == want
	}
	now = drive(h, now, 3)
	if lastCommit().Mark || h.micMark {
		t.Fatal("the mark shows with the microphone closed")
	}
	for _, phase := range []string{feature.VoiceListening, feature.VoiceTranscribing, feature.VoiceSpeaking} {
		text := "what time is it"
		if phase == feature.VoiceListening {
			text = ""
		}
		send(t, h, &feature.VoiceFrame{Phase: phase, Source: feature.VoiceSourceChord, Text: text})
		if h.micMark {
			t.Fatalf("%s showed the mark: a press is not an open microphone", phase)
		}
	}
	send(t, h, &feature.VoiceFrame{Phase: feature.VoiceIdle})
	send(t, h, &feature.VoiceFrame{Phase: feature.VoiceArmed, Source: feature.VoiceSourceWake})
	now = drive(h, now, 2)
	if !lastCommit().Mark || !markPixel() {
		t.Fatal("armed without the mark drawn")
	}
	// clicks and drags do not take it away
	h.click()
	now = drive(h, now, 2)
	if !lastCommit().Mark {
		t.Fatal("a click hid the mark")
	}
	// Oni hidden: the mark stays on screen alone, taking no input
	send(t, h, &feature.StateFrame{State: "idle", Reason: "hidden"})
	if h.shown || !h.markOnly {
		t.Fatalf("hidden and armed: shown %v, mark alone %v", h.shown, h.markOnly)
	}
	p := d.presented[len(d.presented)-1]
	if !p.input.Empty() || !markPixel() || !lastCommit().Mark {
		t.Fatalf("the mark alone: input %v", p.input)
	}
	ext := h.extent()
	if c := h.img.NRGBAAt(ext.X/2, ext.Y/2); c.A != 0 {
		t.Fatalf("the hidden character was drawn under the mark: %v", c)
	}
	// disarmed: the mark goes, with the surface while Oni is hidden
	send(t, h, &feature.VoiceFrame{Phase: feature.VoiceIdle})
	if h.markOnly || h.micMark {
		t.Fatal("the mark outlived the microphone")
	}
	hides := ledgerLines(t, buf, EvHide)
	if last := hides[len(hides)-1]; last.Reason != "mic-mark" {
		t.Fatalf("last hide %+v", last)
	}
	// shown again and armed again, then the daemon goes: its microphone
	// went with it
	send(t, h, &feature.StateFrame{State: "idle"})
	send(t, h, &feature.VoiceFrame{Phase: feature.VoiceArmed, Source: feature.VoiceSourceOpen})
	if !h.micMark {
		t.Fatal("open mic without the mark")
	}
	h.retried = true
	h.lost(h.link)
	if h.micMark {
		t.Fatal("the mark outlived the daemon")
	}
	var marks []string
	for _, e := range ledgerLines(t, buf, EvMicMark) {
		marks = append(marks, e.Reason)
	}
	if !slices.Equal(marks, []string{"on", "off", "on", "off"}) {
		t.Fatalf("micmark lines %q", marks)
	}
	// a voice frame's ledger line has its phase and source, never words
	for _, e := range ledgerLines(t, buf, EvFrame) {
		if e.Kind == feature.KindVoice && e.Phase == "" {
			t.Fatalf("voice frame line without its phase: %+v", e)
		}
	}
	if strings.Contains(buf.String(), "words") {
		t.Fatal("voice words reached the ledger")
	}
	_ = now
}

// The surface paints the mark exactly as render does for the desktop's
// design (render's tests prove each design's corners), so Lithic's mark and
// the round designs' differ on screen too.
func TestTheMicMarkWearsTheDesktopsDesign(t *testing.T) {
	drawn := map[design.Style][]byte{}
	for _, d := range []design.Style{design.Lithic, design.Material, design.Adwaita, design.Fluent} {
		h, _, _, _ := linkedHost(t)
		h.look = markLook(d)
		send(t, h, &feature.VoiceFrame{Phase: feature.VoiceArmed, Source: feature.VoiceSourceWake})
		// hidden, the surface carries the mark alone
		send(t, h, &feature.StateFrame{State: "idle", Reason: "hidden"})
		want := image.NewNRGBA(h.img.Bounds())
		render.PaintMicMark(want, render.MicMarkRect(h.extent()), h.look, 1)
		if !bytes.Equal(h.img.Pix, want.Pix) {
			t.Errorf("%s: the surface's mark differs from the design's", d)
		}
		drawn[d] = bytes.Clone(h.img.Pix)
	}
	if bytes.Equal(drawn[design.Lithic], drawn[design.Material]) {
		t.Fatal("lithic and material draw the same mark")
	}
}
