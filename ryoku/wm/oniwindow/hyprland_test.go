package window

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"testing"
	"time"

	"ryoku-rashin/internal/oni/spec"
)

func hyprDesk() *fakeDesk {
	return newDesk(map[string]string{
		"hyprctl -j monitors":  "hyprland/monitors.json",
		"hyprctl -j clients":   "hyprland/clients.json",
		"hyprctl -j cursorpos": "hyprland/cursorpos.json",
	})
}

func TestHyprlandSnapshotAcrossOutputs(t *testing.T) {
	d := hyprDesk()
	p := newHyprland(d.run, nil)
	s, err := p.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	wantOutputs := []Output{
		{Name: "eDP-1", X: 0, Y: 0, W: 1920, H: 1200, Scale: 1.5},
		{Name: "DP-1", X: 1920, Y: 0, W: 2560, H: 1440, Scale: 1},
	}
	if !slices.Equal(s.Outputs, wantOutputs) {
		t.Fatalf("outputs %+v, want logical sizes %+v", s.Outputs, wantOutputs)
	}
	calc := byID(t, s, "0x5a1e2b3c9e20")
	wantBox(t, calc, Box{X: 1180, Y: 210, W: 360, H: 620})
	if !calc.Focused || calc.FocusOrder != 0 || !calc.Floating || calc.Output != "eDP-1" {
		t.Fatalf("calculator %+v", calc)
	}
	// Tiled windows are placed on Hyprland too.
	wantBox(t, byID(t, s, "0x5a1e2b3c4d50"), Box{X: 12, Y: 52, W: 944, H: 1136})
	// Global 1932 on the output at 1920.
	term := byID(t, s, "0x5a1e2b4f0a10")
	wantBox(t, term, Box{X: 12, Y: 12, W: 2536, H: 1416})
	if term.Output != "DP-1" {
		t.Fatalf("terminal output %q", term.Output)
	}
	// A shown special workspace counts as visible.
	wantBox(t, byID(t, s, "0x5a1e2b5d7c80"), Box{X: 480, Y: 300, W: 800, H: 500})
	// A tab hidden in a group, and a window on a workspace no monitor shows.
	for _, id := range []ID{"0x5a1e2b6a1b30", "0x5a1e2b7e2c40"} {
		w := byID(t, s, id)
		if w.Visible {
			t.Fatalf("%s is not visible: %+v", id, w)
		}
		wantUnplaced(t, w, UnplacedOffView)
	}
}

func TestHyprlandRotatedMonitorAndFullscreenForms(t *testing.T) {
	m := hyprMonitor{Width: 2560, Height: 1440, Scale: 1.25, Transform: 1}
	if w, h := m.logical(); w != 1152 || h != 2048 {
		t.Fatalf("rotated logical %dx%d, want 1152x2048", w, h)
	}
	for raw, want := range map[string]bool{"0": false, "1": false, "2": true, "3": true, "true": true, "false": false} {
		c := hyprClient{Fullscreen: json.RawMessage(raw)}
		if c.fullscreen() != want {
			t.Fatalf("fullscreen %s read as %v", raw, !want)
		}
	}
}

func TestHyprlandPointerAndHitTest(t *testing.T) {
	d := hyprDesk()
	p := newHyprland(d.run, nil)
	pt, ok, err := p.Pointer(context.Background())
	if err != nil || !ok || pt != (spec.Point{X: 2100, Y: 640}) {
		t.Fatalf("pointer %+v %v %v", pt, ok, err)
	}
	s, err := p.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	var tr Tracker
	cases := []struct {
		at   spec.Point
		want ID
	}{
		{pt, "0x5a1e2b4f0a10"},                          // tiled terminal on the second output
		{spec.Point{X: 2500, Y: 400}, "0x5a1e2b5d7c80"}, // floating over the tiled terminal
		{spec.Point{X: 1200, Y: 300}, "0x5a1e2b3c9e20"},
		{spec.Point{X: 500, Y: 500}, "0x5a1e2b3c4d50"}, // not the hidden group tab in the same rect
	}
	for _, c := range cases {
		w, ok := tr.HitTest(c.at, s)
		if !ok || w.ID != c.want {
			t.Fatalf("hit at %+v: %q %v, want %q", c.at, w.ID, ok, c.want)
		}
	}
	if w, ok := tr.HitTest(spec.Point{X: 1700, Y: 1100}, s); ok {
		t.Fatalf("empty desktop hit %q", w.ID)
	}
}

// fakeTimer stands in for the sample timer: each arm reports its delay, a
// test fires the armed tick by hand, and live says whether one is armed.
type fakeTimer struct {
	mu   sync.Mutex
	cur  chan time.Time
	arms chan time.Duration
}

func newFakeTimer() *fakeTimer {
	return &fakeTimer{arms: make(chan time.Duration, 16)}
}

func (f *fakeTimer) new(d time.Duration) (<-chan time.Time, func()) {
	c := make(chan time.Time, 1)
	f.mu.Lock()
	f.cur = c
	f.mu.Unlock()
	f.arms <- d
	return c, func() {
		f.mu.Lock()
		if f.cur == c {
			f.cur = nil
		}
		f.mu.Unlock()
	}
}

func (f *fakeTimer) live() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.cur != nil
}

// fire delivers the armed tick.
func (f *fakeTimer) fire(t *testing.T) {
	t.Helper()
	f.mu.Lock()
	c := f.cur
	f.mu.Unlock()
	if c == nil {
		t.Fatal("fired with no sample armed")
	}
	c <- time.Now()
}

// armed waits for the next arm and checks its delay.
func (f *fakeTimer) armed(t *testing.T, want time.Duration) {
	t.Helper()
	select {
	case d := <-f.arms:
		if d != want {
			t.Fatalf("next sample in %v, want %v", d, want)
		}
	case <-time.After(time.Second):
		t.Fatalf("no sample armed, want one in %v", want)
	}
}

// quiet checks that nothing was armed.
func (f *fakeTimer) quiet(t *testing.T) {
	t.Helper()
	select {
	case d := <-f.arms:
		t.Fatalf("a sample was armed in %v", d)
	case <-time.After(50 * time.Millisecond):
	}
}

// reads waits until the desk has answered n reads in all.
func reads(t *testing.T, d *fakeDesk, n int) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for d.count() < n {
		if time.Now().After(deadline) {
			t.Fatalf("%d reads, want %d", d.count(), n)
		}
		time.Sleep(2 * time.Millisecond)
	}
	time.Sleep(20 * time.Millisecond)
	if d.count() != n {
		t.Fatalf("%d reads, want %d", d.count(), n)
	}
}

// watchHypr runs Watch over a piped event socket with the fake timer.
func watchHypr(t *testing.T, d *fakeDesk) (*hyprland, *fakeTimer, io.Writer, chan Snapshot) {
	t.Helper()
	events, feed := io.Pipe()
	p := newHyprland(d.run, func(context.Context) (io.ReadCloser, error) { return events, nil })
	tm := newFakeTimer()
	p.newTimer = tm.new
	ctx, cancel := context.WithCancel(context.Background())
	snaps := make(chan Snapshot, 16)
	done := make(chan error, 1)
	go func() { done <- p.Watch(ctx, func(s Snapshot) { snaps <- s }) }()
	t.Cleanup(func() {
		cancel()
		if err := <-done; err != nil {
			t.Errorf("Watch after cancel: %v", err)
		}
		if tm.live() {
			t.Error("a sample is left armed after Watch ended")
		}
	})
	return p, tm, feed, snaps
}

const calcID ID = "0x5a1e2b3c9e20"

func TestHyprlandGeometryPollRunsOnlyWhilePerched(t *testing.T) {
	d := hyprDesk()
	p, tm, feed, snaps := watchHypr(t, d)
	first := <-snaps
	wantBox(t, byID(t, first, calcID), Box{X: 1180, Y: 210, W: 360, H: 620})

	// Irrelevant events neither sample nor read.
	lines, err := os.ReadFile("testdata/hyprland/events.txt")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := feed.Write(lines); err != nil {
		t.Fatal(err)
	}
	tm.quiet(t)
	if n := d.count(); n != 1 {
		t.Fatalf("with no perch: %d reads, want the initial one", n)
	}

	// A perch goes out: a sample is due at once, and it re-reads geometry.
	p.Geometry(calcID)
	tm.armed(t, hyprPollFast)
	d.set("hyprctl -j clients", "hyprland/clients-moved.json")
	tm.fire(t)
	moved := <-snaps
	var tr Tracker
	tr.Track(calcID)
	tr.Update(first)
	if got := kinds(tr.Update(moved)); !slices.Equal(got, []string{ChangeMoved, ChangeResized, ChangeFocus}) {
		t.Fatalf("changes %v", got)
	}
	if !byID(t, moved, "0x5a1e2b4f0a10").Fullscreen {
		t.Fatalf("fullscreen mode 2 is fullscreen")
	}
	tm.armed(t, hyprPollFast)

	// Detached: the sample stops and nothing reads until an event arrives.
	p.Geometry("")
	deadline := time.Now().Add(time.Second)
	for tm.live() {
		if time.Now().After(deadline) {
			t.Fatal("the sample is still armed after the perch ended")
		}
		time.Sleep(2 * time.Millisecond)
	}
	before := d.count()
	tm.quiet(t)
	if d.count() != before {
		t.Fatalf("reads continued after the perch ended")
	}

	// A relevant event re-reads after the debounce, and arms no sample.
	d.set("hyprctl -j clients", "hyprland/clients.json")
	if _, err := feed.Write([]byte("changefloatingmode>>5a1e2b3c9e20,1\nmovewindowv2>>5a1e2b3c9e20,1,1\n")); err != nil {
		t.Fatal(err)
	}
	back := <-snaps
	wantBox(t, byID(t, back, calcID), Box{X: 1180, Y: 210, W: 360, H: 620})
	reads(t, d, before+1)
	tm.quiet(t)
}

// A perched window that holds still is sampled less and less often, down to
// hyprPollSlow; the first sample that finds it moved is fast again.
func TestHyprlandGeometrySampleBacksOffWhileTheWindowHoldsStill(t *testing.T) {
	d := hyprDesk()
	p, tm, _, snaps := watchHypr(t, d)
	<-snaps
	p.Geometry(calcID)
	tm.armed(t, hyprPollFast)
	// each sample is one request: the batched monitor and client lists
	n := d.count()
	for _, next := range []time.Duration{500 * time.Millisecond, time.Second, hyprPollSlow, hyprPollSlow} {
		tm.fire(t)
		tm.armed(t, next)
		n++
		reads(t, d, n)
	}
	d.set("hyprctl -j clients", "hyprland/clients-moved.json")
	tm.fire(t)
	<-snaps
	tm.armed(t, hyprPollFast)
	// still there: backing off again
	tm.fire(t)
	tm.armed(t, 500*time.Millisecond)
}

// An event naming the perched window samples it at once and makes the
// sample fast again; an event about another window that leaves the perched
// one where it was reads once and keeps the backoff.
func TestHyprlandEventNamingThePerchedWindowWakesTheSample(t *testing.T) {
	d := hyprDesk()
	p, tm, feed, snaps := watchHypr(t, d)
	<-snaps
	p.Geometry(calcID)
	tm.armed(t, hyprPollFast)
	for _, next := range []time.Duration{500 * time.Millisecond, time.Second, hyprPollSlow} {
		tm.fire(t)
		tm.armed(t, next)
	}
	n := d.count()
	if _, err := feed.Write([]byte("windowtitlev2>>5a1e2b4f0a10,build\n")); err != nil {
		t.Fatal(err)
	}
	reads(t, d, n+1)
	tm.quiet(t)
	if _, err := feed.Write([]byte("activewindowv2>>5a1e2b3c9e20\n")); err != nil {
		t.Fatal(err)
	}
	reads(t, d, n+2)
	tm.armed(t, hyprPollFast)
	// and a read on that event that finds the window moved does too
	tm.fire(t)
	tm.armed(t, 500*time.Millisecond)
	d.set("hyprctl -j clients", "hyprland/clients-moved.json")
	if _, err := feed.Write([]byte("workspace>>2\n")); err != nil {
		t.Fatal(err)
	}
	<-snaps
	tm.armed(t, hyprPollFast)
}

// The real reader asks Hyprland's request socket for the allowlisted reads
// and starts no process; anything else is refused before it dials.
func TestHyprlandRequestsGoOverTheRequestSocket(t *testing.T) {
	dir := t.TempDir()
	sig := "0e7f5c1a_1763800000_1234567"
	if err := os.MkdirAll(filepath.Join(dir, "hypr", sig), 0o700); err != nil {
		t.Fatal(err)
	}
	ln, err := net.Listen("unix", filepath.Join(dir, "hypr", sig, ".socket.sock"))
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	answers := map[string]string{
		"j/cursorpos": "hyprland/cursorpos.json",
	}
	// Hyprland answers a batch with each reply in order, joined by two blank
	// lines, on the one connection
	mons, err := os.ReadFile("testdata/hyprland/monitors.json")
	if err != nil {
		t.Fatal(err)
	}
	clients, err := os.ReadFile("testdata/hyprland/clients.json")
	if err != nil {
		t.Fatal(err)
	}
	batch := string(mons) + "\n\n\n" + string(clients)
	var mu sync.Mutex
	var asked []string
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			req, _ := io.ReadAll(c)
			mu.Lock()
			asked = append(asked, string(req))
			mu.Unlock()
			if string(req) == "[[BATCH]]j/monitors;j/clients" {
				_, _ = io.WriteString(c, batch)
			} else if file, ok := answers[string(req)]; ok {
				b, _ := os.ReadFile(filepath.Join("testdata", file))
				_, _ = c.Write(b)
			} else {
				_, _ = io.WriteString(c, "unknown request")
			}
			_ = c.Close()
		}
	}()
	env := []string{"XDG_RUNTIME_DIR=" + dir, "HYPRLAND_INSTANCE_SIGNATURE=" + sig}
	req := HyprlandRequests(env)
	p := newHyprland(req, nil)
	s, err := p.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	wantBox(t, byID(t, s, calcID), Box{X: 1180, Y: 210, W: 360, H: 620})
	if pt, ok, err := p.Pointer(context.Background()); err != nil || !ok || pt != (spec.Point{X: 2100, Y: 640}) {
		t.Fatalf("pointer %+v %v %v", pt, ok, err)
	}
	for _, argv := range [][]string{
		{"-j", "workspaces"}, {"dispatch", "killactive"}, {"-j", "clients", "--batch"},
		{"--batch", "j/monitors;j/clients;dispatch killactive"}, {"--batch", "dispatch killactive"},
	} {
		if _, err := req(context.Background(), "hyprctl", argv...); !errors.Is(err, ErrNotAllowed) {
			t.Fatalf("%q: %v", argv, err)
		}
	}
	mu.Lock()
	got := slices.Clone(asked)
	mu.Unlock()
	if !slices.Equal(got, []string{"[[BATCH]]j/monitors;j/clients", "j/cursorpos"}) {
		t.Fatalf("asked %q", got)
	}
	if _, err := HyprlandRequests([]string{"XDG_RUNTIME_DIR=" + dir})(context.Background(), "hyprctl", "-j", "clients"); err == nil {
		t.Fatal("read with no instance signature")
	}
}

func TestHyprlandWatchEndsWhenTheCompositorCloses(t *testing.T) {
	d := hyprDesk()
	events, feed := io.Pipe()
	p := newHyprland(d.run, func(context.Context) (io.ReadCloser, error) { return events, nil })
	done := make(chan error, 1)
	go func() { done <- p.Watch(context.Background(), func(Snapshot) {}) }()
	time.Sleep(20 * time.Millisecond)
	_ = feed.Close()
	if err := <-done; !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatalf("Watch returned %v", err)
	}
	if err := newHyprland(d.run, nil).Watch(context.Background(), func(Snapshot) {}); !errors.Is(err, ErrNoStream) {
		t.Fatalf("no event socket: %v", err)
	}
}
