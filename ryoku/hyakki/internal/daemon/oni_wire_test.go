package daemon

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"io"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"ryoku-rashin/internal/client"
	"ryoku-rashin/internal/config"
	"ryoku-rashin/internal/core"
	"ryoku-rashin/internal/llm"
	"ryoku-rashin/internal/oni/capture"
	"ryoku-rashin/internal/oni/feature"
	"ryoku-rashin/internal/oni/presence"
	"ryoku-rashin/internal/oni/screenask"
	"ryoku-rashin/internal/oni/spec"
	"ryoku-rashin/internal/oni/window"
	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/protocol"
)

// These tests drive the perch and the screen questions over the daemon's
// own socket. The desktop, grim and the model are fakes: no compositor is
// asked, no screen is captured and nothing leaves the machine.

// fakeDesktop is a window.Provider over a snapshot the test sets.
type fakeDesktop struct {
	mu        sync.Mutex
	snap      window.Snapshot
	watchers  map[int]func(window.Snapshot)
	next      int
	geometry  window.ID
	snapshots int
	changed   chan struct{}
}

func newDesktop(s window.Snapshot) *fakeDesktop {
	return &fakeDesktop{snap: s, watchers: map[int]func(window.Snapshot){}, changed: make(chan struct{}, 64)}
}

func (f *fakeDesktop) Caps() window.Caps {
	return window.Caps{Provider: "fake", Geometry: window.GeomFloating, Events: true}
}

func (f *fakeDesktop) Snapshot(context.Context) (window.Snapshot, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.snapshots++
	return cloneSnapshot(f.snap), nil
}

func (f *fakeDesktop) Watch(ctx context.Context, fn func(window.Snapshot)) error {
	f.mu.Lock()
	id := f.next
	f.next++
	f.watchers[id] = fn
	f.mu.Unlock()
	f.notify()
	<-ctx.Done()
	f.mu.Lock()
	delete(f.watchers, id)
	f.mu.Unlock()
	f.notify()
	return ctx.Err()
}

func (f *fakeDesktop) Geometry(id window.ID) {
	f.mu.Lock()
	f.geometry = id
	f.mu.Unlock()
	f.notify()
}

func (f *fakeDesktop) Pointer(context.Context) (spec.Point, bool, error) {
	return spec.Point{}, false, nil
}

func (f *fakeDesktop) notify() {
	select {
	case f.changed <- struct{}{}:
	default:
	}
}

// set replaces the desktop and hands it to every watch, as an event would.
func (f *fakeDesktop) set(s window.Snapshot) {
	f.mu.Lock()
	f.snap = cloneSnapshot(s)
	fns := make([]func(window.Snapshot), 0, len(f.watchers))
	for _, fn := range f.watchers {
		fns = append(fns, fn)
	}
	f.mu.Unlock()
	for _, fn := range fns {
		fn(cloneSnapshot(s))
	}
}

func (f *fakeDesktop) state() (watching int, geometry window.ID) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.watchers), f.geometry
}

// await waits until the watch count and the sampled window are as wanted
// ("" for no geometry sampling).
func (f *fakeDesktop) await(t *testing.T, watching int, geometry window.ID) {
	t.Helper()
	deadline := time.After(5 * time.Second)
	for {
		if w, g := f.state(); w == watching && g == geometry {
			return
		}
		select {
		case <-f.changed:
		case <-time.After(20 * time.Millisecond):
		case <-deadline:
			w, g := f.state()
			t.Fatalf("watching %d sampling %q, want %d and %q", w, g, watching, geometry)
		}
	}
}

func cloneSnapshot(s window.Snapshot) window.Snapshot {
	out := window.Snapshot{Outputs: slices.Clone(s.Outputs)}
	for _, w := range s.Windows {
		if w.Box != nil {
			b := *w.Box
			w.Box = &b
		}
		out.Windows = append(out.Windows, w)
	}
	return out
}

var dp1 = window.Output{Name: "DP-1", W: 1920, H: 1080, Scale: 1}

func editorWindow() window.Window {
	return window.Window{ID: "0x1", App: "editor", Title: "notes.txt", Output: "DP-1", Box: &window.Box{X: 100, Y: 80, W: 800, H: 600}, Focused: true, Floating: true, Visible: true}
}

func desktopWindows() window.Snapshot {
	return window.Snapshot{
		Outputs: []window.Output{dp1},
		Windows: []window.Window{
			editorWindow(),
			{ID: "12", App: "term", Title: "shell", Output: "DP-1", Unplaced: window.UnplacedTiled, Visible: true, FocusOrder: 1},
			{ID: "0x3", App: "mail", Title: "Inbox", Output: "DP-1", Box: &window.Box{X: 1000, Y: 100, W: 600, H: 400}, Minimized: true, FocusOrder: 2},
			{ID: "0x4", App: "viewer", Title: "photo", Output: "DP-1", Box: &window.Box{X: 1000, Y: 500, W: 700, H: 500}, Floating: true, Visible: true, FocusOrder: 3},
		},
	}
}

// fakeGrim answers grim's region grab with a dark picture the size asked
// for, and counts every grab.
type fakeGrim struct {
	mu    sync.Mutex
	calls [][]string
}

func (g *fakeGrim) run(_ context.Context, name string, args ...string) ([]byte, error) {
	g.mu.Lock()
	g.calls = append(g.calls, append([]string{name}, args...))
	g.mu.Unlock()
	if name != "grim" || len(args) < 2 || args[0] != "-g" {
		return nil, errors.New("fake grim: only region grabs")
	}
	_, size, _ := strings.Cut(args[1], " ")
	ws, hs, _ := strings.Cut(size, "x")
	w, _ := strconv.Atoi(ws)
	h, _ := strconv.Atoi(hs)
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for i := range img.Pix {
		img.Pix[i] = 40
		if i%4 == 3 {
			img.Pix[i] = 255
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (g *fakeGrim) count() int {
	g.mu.Lock()
	defer g.mu.Unlock()
	return len(g.calls)
}

// shotLog keeps every picture the capture made, to prove each was cleared,
// and the masks each was taken with.
type shotLog struct {
	mu    sync.Mutex
	inner capture.Service
	shots []*capture.Shot
	masks [][]window.Box
}

func (l *shotLog) service() capture.Service {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.inner
}

func (l *shotLog) Probe(ctx context.Context) []capture.Capability { return l.service().Probe(ctx) }

func (l *shotLog) Capture(ctx context.Context, w window.Window, masks []window.Box) (*capture.Shot, capture.Disclosure, error) {
	shot, d, err := l.service().Capture(ctx, w, masks)
	if shot != nil {
		l.mu.Lock()
		l.shots = append(l.shots, shot)
		l.masks = append(l.masks, slices.Clone(masks))
		l.mu.Unlock()
	}
	return shot, d, err
}

// lastMasks are the masks the latest picture was taken with.
func (l *shotLog) lastMasks() []window.Box {
	l.mu.Lock()
	defer l.mu.Unlock()
	if len(l.masks) == 0 {
		return nil
	}
	return l.masks[len(l.masks)-1]
}

// cleared reports that every picture taken so far was zeroed.
func (l *shotLog) cleared() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, s := range l.shots {
		if s.Size() != (image.Point{}) {
			return false
		}
	}
	return true
}

func (l *shotLog) count() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.shots)
}

// fakeModel plans one route and answers with the replies queued, keeping a
// copy of every request it was sent. With hold set, a reply waits until
// hold closes, whatever the request's context says, as a late reply would.
type fakeModel struct {
	core.Gateway
	mu       sync.Mutex
	plan     llm.Plan
	planErr  error
	replies  []string
	requests []llm.Request
	hold     chan struct{}
}

func (m *fakeModel) Plan(context.Context, llm.Request) (llm.Plan, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.plan, m.planErr
}

func (m *fakeModel) Stream(_ context.Context, req llm.Request) (llm.Stream, error) {
	m.mu.Lock()
	kept := req
	kept.Messages = nil
	for _, msg := range req.Messages {
		c := msg
		c.Images = nil
		for _, img := range msg.Images {
			c.Images = append(c.Images, llm.Image{Mime: img.Mime, Data: bytes.Clone(img.Data)})
		}
		kept.Messages = append(kept.Messages, c)
	}
	m.requests = append(m.requests, kept)
	reply := ""
	if len(m.replies) > 0 {
		reply, m.replies = m.replies[0], m.replies[1:]
	}
	hold := m.hold
	m.mu.Unlock()
	if hold != nil {
		<-hold
	}
	return &fakeReply{text: reply}, nil
}

func (m *fakeModel) Close() error { return nil }

func (m *fakeModel) reply(texts ...string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.replies = append(m.replies, texts...)
}

func (m *fakeModel) sent() []llm.Request {
	m.mu.Lock()
	defer m.mu.Unlock()
	return slices.Clone(m.requests)
}

type fakeReply struct {
	text string
	done bool
}

func (r *fakeReply) Next() (llm.Delta, error) {
	if r.done {
		return llm.Delta{}, io.EOF
	}
	r.done = true
	return llm.Delta{Text: r.text}, nil
}

func (r *fakeReply) Final() llm.Final { return llm.Final{} }
func (r *fakeReply) Close() error     { return nil }

// askClock arms the consent expiry; Advance fires it.
type askClock struct {
	mu     sync.Mutex
	now    time.Time
	timers []*askTimer
}

type askTimer struct {
	clock *askClock
	at    time.Time
	f     func()
	done  bool
}

func (t *askTimer) Stop() bool {
	t.clock.mu.Lock()
	defer t.clock.mu.Unlock()
	was := !t.done
	t.done = true
	return was
}

func (c *askClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *askClock) AfterFunc(d time.Duration, f func()) presence.Timer {
	c.mu.Lock()
	defer c.mu.Unlock()
	t := &askTimer{clock: c, at: c.now.Add(d), f: f}
	c.timers = append(c.timers, t)
	return t
}

func (c *askClock) Advance(d time.Duration) {
	c.mu.Lock()
	c.now = c.now.Add(d)
	var due []*askTimer
	for _, t := range c.timers {
		if !t.done && !t.at.After(c.now) {
			t.done = true
			due = append(due, t)
		}
	}
	c.mu.Unlock()
	for _, t := range due {
		t.f()
	}
}

func (c *askClock) Armed() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	n := 0
	for _, t := range c.timers {
		if !t.done {
			n++
		}
	}
	return n
}

// wireRig is a serving daemon with fakes for the desktop, grim, the model,
// tesseract and the settle pictures.
type wireRig struct {
	d       *Daemon
	desk    *fakeDesktop
	grim    *fakeGrim
	shots   *shotLog
	model   *fakeModel
	clock   *askClock
	tess    *fakeOCR
	screen  *fakeScreen
	app     *atomic.Bool
	detects atomic.Int32
}

func serveWire(t *testing.T, inspect bool, plan llm.Plan) *wireRig {
	t.Helper()
	isolateDaemon(t)
	shortRuntime(t)
	t.Setenv("HYAKKI_SOCKET", "")
	settings, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	settings.Enabled = true
	settings.Oni.Master = true
	settings.Oni.Inspect = inspect
	if err := config.Save(settings); err != nil {
		t.Fatal(err)
	}
	r := &wireRig{desk: newDesktop(desktopWindows()), grim: &fakeGrim{}, model: &fakeModel{plan: plan}, clock: &askClock{now: time.Unix(1_800_000_000, 0)}, app: &atomic.Bool{}}
	r.shots = &shotLog{}
	r.tess = &fakeOCR{}
	r.screen = newFakeScreen()
	deps := Deps{
		Version: "oni-wire-test",
		Gateway: r.model,
		OniWindows: func() (window.Provider, error) {
			r.detects.Add(1)
			return r.desk, nil
		},
		OniCapture: func(p window.Provider) capture.Service {
			r.shots.mu.Lock()
			r.shots.inner = capture.New(r.grim.run, []capture.Path{capture.PathOutputCrop}, capture.Layout(p.Snapshot))
			r.shots.mu.Unlock()
			return r.shots
		},
		OniGuide: OniGuideDeps{
			OCR:    r.tess,
			Settle: r.screen.take,
			Clock:  r.clock,
		},
	}
	d := New(refuseRealDesktop(t, deps))
	d.appPeer = func(int32) bool { return r.app.Load() }
	d.oni.ask.clock = r.clock
	r.d = d
	ctx, stop := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- d.Run(ctx) }()
	t.Cleanup(func() {
		stop()
		select {
		case <-done:
		case <-time.After(10 * time.Second):
			t.Error("daemon did not stop")
		}
	})
	deadline := time.Now().Add(5 * time.Second)
	for !client.Reachable(context.Background(), paths.Socket()) {
		if time.Now().After(deadline) {
			t.Fatal("the daemon never listened")
		}
		time.Sleep(10 * time.Millisecond)
	}
	return r
}

var localEyes = llm.Plan{Provider: "fixture", Model: "eyes", Local: true, Vision: true}

// dialApp is a connection the daemon takes for the Hyakki app's.
func (r *wireRig) dialApp(t *testing.T) *socketPeer {
	t.Helper()
	r.app.Store(true)
	defer r.app.Store(false)
	return dialSocket(t, protocol.ClientApp)
}

// presenting opens a surface drawing Oni and an observer that watches.
func (r *wireRig) presenting(t *testing.T) (surface, watcher *socketPeer) {
	t.Helper()
	surface = dialSocket(t, protocol.ClientSurface)
	if !surface.open(protocol.OniRendererNative).Presenter {
		t.Fatal("the surface is no presenter")
	}
	watcher = dialSocket(t, protocol.ClientCLI)
	watcher.open(protocol.OniRendererObserver)
	return surface, watcher
}

func reasonOf(err error) (code, reason string) {
	var perr *protocol.Error
	if !errors.As(err, &perr) {
		return "", ""
	}
	return perr.Code, perr.Reason
}

func perchOn(t *testing.T, p *socketPeer, window string) protocol.OniPerchResult {
	t.Helper()
	var r protocol.OniPerchResult
	if err := p.call(protocol.MOniPerch, protocol.OniPerchParams{Window: window}, &r); err != nil {
		t.Fatalf("perch on %s: %v", window, err)
	}
	return r
}

func TestOniPerchRefusesWithItsReasonAndSendsNoFrame(t *testing.T) {
	r := serveWire(t, true, localEyes)
	cli := dialSocket(t, protocol.ClientCLI)
	if code, reason := reasonOf(cli.call(protocol.MOniPerch, protocol.OniPerchParams{Window: "0x1"}, nil)); code != protocol.CodeUnavailable || reason != feature.PerchPresenterLost {
		t.Fatalf("a perch with nothing drawing Oni: %s %s", code, reason)
	}
	surface, watcher := r.presenting(t)
	cases := []struct {
		params protocol.OniPerchParams
		code   string
		reason string
	}{
		{protocol.OniPerchParams{Window: "0x99"}, protocol.CodeNotFound, feature.PerchClosed},
		{protocol.OniPerchParams{Window: "12"}, protocol.CodeUnavailable, feature.PerchNoGeometry},
		{protocol.OniPerchParams{Window: "0x3"}, protocol.CodeUnavailable, feature.PerchHidden},
	}
	for _, c := range cases {
		err := cli.call(protocol.MOniPerch, c.params, nil)
		if code, reason := reasonOf(err); code != c.code || reason != c.reason {
			t.Errorf("perch on %s: %v (%s %s), want %s %s", c.params.Window, err, code, reason, c.code, c.reason)
		}
	}
	// the presenter's drop on empty desktop finds no window
	empty := protocol.OniPerchParams{Output: "DP-1", X: 1900, Y: 1060}
	if code, reason := reasonOf(surface.call(protocol.MOniPerch, empty, nil)); code != protocol.CodeNotFound || reason != feature.PerchClosed {
		t.Fatalf("a drop on empty desktop: %s %s", code, reason)
	}
	if err := surface.call(protocol.MOniPerch, protocol.OniPerchParams{Output: "HDMI-A-9", X: 10, Y: 10}, nil); err == nil {
		t.Fatal("a drop on an unknown output perched")
	}
	var hover protocol.OniPerchResult
	empty.Hover = true
	if err := surface.call(protocol.MOniPerch, empty, &hover); err != nil || hover.Reason != feature.PerchClosed || hover.Window != "" {
		t.Fatalf("a hover over empty desktop = %+v, %v", hover, err)
	}
	hover = protocol.OniPerchResult{}
	if err := surface.call(protocol.MOniPerch, protocol.OniPerchParams{Output: "DP-1", X: 200, Y: 100, Hover: true}, &hover); err != nil || hover.Reason != "" || hover.Window != "0x1" || hover.Title != "notes.txt" || *hover.Box != (protocol.OniRect{X: 100, Y: 80, W: 800, H: 600}) {
		t.Fatalf("a hover over the editor = %+v, %v", hover, err)
	}
	surface.quiet()
	watcher.quiet()
	if w, g := r.desk.state(); w != 0 || g != "" {
		t.Fatalf("refusals and hovers left a watch (%d) or the poll (%q) running", w, g)
	}
}

func TestOniPerchFollowsTheWindowAndDetachesWhenItCloses(t *testing.T) {
	r := serveWire(t, true, localEyes)
	surface, watcher := r.presenting(t)
	cli := dialSocket(t, protocol.ClientCLI)
	res := perchOn(t, cli, "0x1")
	if res.Seq != 1 || res.Window != "0x1" || res.App != "editor" || res.Output != "DP-1" || (res.Corner != "top-left" && res.Corner != "top-right") {
		t.Fatalf("perch = %+v", res)
	}
	r.desk.await(t, 1, "0x1")
	frames := surface.frames(2)
	perch, ok := frames[0].(*feature.PerchFrame)
	if !ok || perch.Seq != 1 || !perch.Attached || perch.Window != "0x1" || *perch.Box != (feature.Box{X: 100, Y: 80, W: 800, H: 600}) {
		t.Fatalf("perch frames %q: %+v", frameKinds(frames), frames[0])
	}
	if curl, ok := frames[1].(*feature.StrandTargetFrame); !ok || curl.Mode != feature.StrandModePerch || curl.W != 1 || curl.H != 1 {
		t.Fatalf("no perch curl after the perch frame: %+v", frames[1])
	}
	watcher.frames(2)

	// a move sends a new seq with the new box, and nothing else
	moved := desktopWindows()
	moved.Windows[0].Box = &window.Box{X: 160, Y: 120, W: 800, H: 600}
	r.desk.set(moved)
	if f := surface.frames(1); f[0].(*feature.PerchFrame).Seq != 2 || *f[0].(*feature.PerchFrame).Box != (feature.Box{X: 160, Y: 120, W: 800, H: 600}) {
		t.Fatalf("a move sent %+v", f[0])
	}
	surface.quiet()
	r.desk.set(moved)
	surface.quiet()

	// a new output re-perches and curls again
	two := cloneSnapshot(moved)
	two.Outputs = append(two.Outputs, window.Output{Name: "HDMI-A-1", X: 1920, W: 1280, H: 1024, Scale: 1})
	two.Windows[0].Output = "HDMI-A-1"
	r.desk.set(two)
	if f := surface.frames(2); f[0].(*feature.PerchFrame).Seq != 3 || f[0].(*feature.PerchFrame).Output != "HDMI-A-1" || f[1].(*feature.StrandTargetFrame).Mode != feature.StrandModePerch {
		t.Fatalf("an output change sent %q", frameKinds(f))
	}
	watcher.frames(3)

	// closing the window detaches: the strand rewinds, then the perch lets go
	closed := cloneSnapshot(two)
	closed.Windows = closed.Windows[1:]
	r.desk.set(closed)
	frames = surface.frames(2)
	if _, ok := frames[0].(*feature.StrandRetractFrame); !ok {
		t.Fatalf("close sent %q", frameKinds(frames))
	}
	if p, ok := frames[1].(*feature.PerchFrame); !ok || p.Attached || p.Seq != 4 {
		t.Fatalf("close sent %+v", frames[1])
	}
	r.desk.await(t, 0, "")
}

func TestOniDetachReasonsFanOutAsARetractAndADetachedPerch(t *testing.T) {
	r := serveWire(t, true, localEyes)
	surface, watcher := r.presenting(t)
	cli := dialSocket(t, protocol.ClientCLI)
	detached := func(name string) {
		t.Helper()
		frames := watcher.frames(2)
		if got := frameKinds(frames); !slices.Equal(got, []string{feature.KindStrandRetract, feature.KindPerch}) {
			t.Fatalf("%s sent %q", name, got)
		}
		if frames[1].(*feature.PerchFrame).Attached {
			t.Fatalf("%s left the perch attached", name)
		}
		r.desk.await(t, 0, "")
	}
	perch := func() {
		t.Helper()
		perchOn(t, cli, "0x1")
		watcher.frames(2)
		r.desk.await(t, 1, "0x1")
	}

	perch()
	if err := cli.call(protocol.MOniDetach, protocol.OniDetachParams{Reason: feature.PerchDraggedOff}, nil); !forbidden(err) {
		t.Fatalf("a CLI said Oni was dragged off: %v", err)
	}
	if err := cli.call(protocol.MOniDetach, protocol.OniDetachParams{}, nil); err != nil {
		t.Fatal(err)
	}
	detached("the CLI")

	perch()
	if err := surface.call(protocol.MOniDetach, protocol.OniDetachParams{Reason: feature.PerchDraggedOff}, nil); err != nil {
		t.Fatal(err)
	}
	detached("a drag off")

	perch()
	gone := desktopWindows()
	gone.Outputs = nil
	r.desk.set(gone)
	detached("the output going")
	r.desk.set(desktopWindows())

	perch()
	hidden := desktopWindows()
	hidden.Windows[0].Minimized = true
	r.desk.set(hidden)
	detached("the window minimising")
	r.desk.set(desktopWindows())

	perch()
	if _, err := r.d.applySettings(context.Background(), json.RawMessage(`{"oni":{"hidden":true}}`)); err != nil {
		t.Fatal(err)
	}
	detached("hiding Oni")
	watcher.frames(1)
	if _, err := r.d.applySettings(context.Background(), json.RawMessage(`{"oni":{"hidden":false}}`)); err != nil {
		t.Fatal(err)
	}
	watcher.frames(1)

	perch()
	surface.conn.Close()
	detached("the presenter leaving")

	again := dialSocket(t, protocol.ClientSurface)
	again.open(protocol.OniRendererNative)
	perch()
	if _, err := r.d.applySettings(context.Background(), json.RawMessage(`{"oni":{"master":false}}`)); err != nil {
		t.Fatal(err)
	}
	detached("turning Oni off")
}

func TestOniFocusMovesThePerchOnlyWhenTheSettingSaysSo(t *testing.T) {
	r := serveWire(t, true, localEyes)
	surface, _ := r.presenting(t)
	cli := dialSocket(t, protocol.ClientCLI)
	perchOn(t, cli, "0x1")
	surface.frames(2)
	r.desk.await(t, 1, "0x1")
	focus := func(id window.ID) window.Snapshot {
		s := desktopWindows()
		for i := range s.Windows {
			s.Windows[i].Focused = s.Windows[i].ID == id
		}
		return s
	}
	r.desk.set(focus("0x4"))
	surface.quiet()
	if _, err := r.d.applySettings(context.Background(), json.RawMessage(`{"oni":{"perchFollowsFocus":true}}`)); err != nil {
		t.Fatal(err)
	}
	r.desk.set(focus("0x1"))
	r.desk.set(focus("0x4"))
	frames := surface.frames(2)
	if p, ok := frames[0].(*feature.PerchFrame); !ok || p.Window != "0x4" || p.Seq != 2 || !p.Attached {
		t.Fatalf("focus moved the perch as %q %+v", frameKinds(frames), frames[0])
	}
	r.desk.await(t, 1, "0x4")
	// focus on a window Oni cannot perch on leaves the perch where it is
	r.desk.set(focus("12"))
	surface.quiet()
}

// inspectFor asks oni.inspect on p and returns the consent it got.
func inspectFor(t *testing.T, p *socketPeer) protocol.OniConsent {
	t.Helper()
	var res protocol.OniAskResult
	if err := p.call(protocol.MOniInspect, protocol.Empty{}, &res); err != nil {
		t.Fatal(err)
	}
	if res.Status != protocol.OniAskConsent || res.Consent == nil {
		t.Fatalf("inspect = %+v", res)
	}
	return *res.Consent
}

func TestOniInspectShowsTheRouteBeforeAnythingLeaves(t *testing.T) {
	r := serveWire(t, true, localEyes)
	surface, _ := r.presenting(t)
	cli := dialSocket(t, protocol.ClientCLI)
	if code, _ := reasonOf(cli.call(protocol.MOniInspect, protocol.Empty{}, nil)); code != protocol.CodeConflict {
		t.Fatalf("inspect with no perch: %s", code)
	}
	if r.grim.count() != 0 {
		t.Fatal("a refused inspect captured the screen")
	}
	perchOn(t, cli, "0x1")
	surface.frames(2)
	if r.grim.count() != 0 {
		t.Fatal("perching captured the screen")
	}

	consent := inspectFor(t, cli)
	if consent.Route != "fixture/eyes" || consent.Destination != "local" || consent.Path != string(capture.PathOutputCrop) || consent.Disclosure == "" || consent.ExpiresMs != feature.ConsentTTL.Milliseconds() {
		t.Fatalf("consent = %+v", consent)
	}
	if consent.Width != 800 || consent.Height != 600 {
		t.Fatalf("the picture would be %dx%d", consent.Width, consent.Height)
	}
	if cfg, err := jpeg.DecodeConfig(bytes.NewReader(consent.Preview)); err != nil || max(cfg.Width, cfg.Height) > oniPreviewPx {
		t.Fatalf("preview = %+v, %v", cfg, err)
	}
	f := surface.frames(1)[0].(*feature.ConsentFrame)
	if f.ID != consent.ID || f.Feature != feature.Inspect || f.Class != feature.ConsentDisclosure || f.Destination != "local" || f.Route != "fixture/eyes" {
		t.Fatalf("consent frame = %+v", f)
	}
	if feat, _ := feature.Lookup(f.Feature); feat.Egress != feature.EgressDisclosed {
		t.Fatalf("the frame's feature egress is %s", feat.Egress)
	}
	if r.grim.count() != 1 || len(r.model.sent()) != 0 {
		t.Fatalf("before an answer: %d grabs, %d requests", r.grim.count(), len(r.model.sent()))
	}

	// no clears the picture and sends nothing
	var res protocol.OniAskResult
	if err := cli.call(protocol.MOniConsentAnswer, protocol.OniConsentAnswerParams{ID: consent.ID, Answer: "no"}, &res); err != nil || res.Status != protocol.OniAskDeclined {
		t.Fatalf("no = %+v, %v", res, err)
	}
	if len(r.model.sent()) != 0 || !r.shots.cleared() {
		t.Fatal("a no sent the picture or kept it")
	}
	if code, _ := reasonOf(cli.call(protocol.MOniConsentAnswer, protocol.OniConsentAnswerParams{ID: consent.ID, Answer: "yes"}, nil)); code != protocol.CodeNotFound {
		t.Fatalf("a second answer: %s", code)
	}

	// the question expires after the TTL and its picture with it
	consent = inspectFor(t, cli)
	surface.frames(1)
	r.clock.Advance(feature.ConsentTTL)
	if !r.shots.cleared() {
		t.Fatal("an expired question kept its picture")
	}
	if code, _ := reasonOf(cli.call(protocol.MOniConsentAnswer, protocol.OniConsentAnswerParams{ID: consent.ID, Answer: "yes"}, nil)); code != protocol.CodeNotFound || len(r.model.sent()) != 0 {
		t.Fatalf("an expired question was answered: %s", code)
	}

	// only the asker or the app answers; yes sends exactly the picture
	consent = inspectFor(t, cli)
	surface.frames(1)
	other := dialSocket(t, protocol.ClientCLI)
	if err := other.call(protocol.MOniConsentAnswer, protocol.OniConsentAnswerParams{ID: consent.ID, Answer: "yes"}, nil); !forbidden(err) {
		t.Fatalf("another connection answered: %v", err)
	}
	r.model.reply("A text editor with a short note open.")
	if err := cli.call(protocol.MOniConsentAnswer, protocol.OniConsentAnswerParams{ID: consent.ID, Answer: "yes"}, &res); err != nil || res.Status != protocol.OniAskAnswered || res.Answer == nil {
		t.Fatalf("yes = %+v, %v", res, err)
	}
	if res.Answer.Text != "A text editor with a short note open." || res.Answer.Mode != screenask.ModeDescribe || res.Answer.Route != "fixture/eyes" || res.Answer.Destination != "local" || res.Answer.Path != string(capture.PathOutputCrop) {
		t.Fatalf("answer = %+v", res.Answer)
	}
	sent := r.model.sent()
	if len(sent) != 1 {
		t.Fatalf("%d requests", len(sent))
	}
	req := sent[0]
	if req.Model != "fixture/eyes" || !req.Strict || !req.LocalOnly || len(req.Tools) != 0 || req.ToolChoice != "" {
		t.Fatalf("request = %+v", req)
	}
	images := 0
	for _, m := range req.Messages {
		for _, img := range m.Images {
			images++
			cfg, err := jpeg.DecodeConfig(bytes.NewReader(img.Data))
			if err != nil || img.Mime != "image/jpeg" || max(cfg.Width, cfg.Height) > capture.AnswerLongSide {
				t.Fatalf("the model got %s %+v, %v", img.Mime, cfg, err)
			}
		}
	}
	if images != 1 {
		t.Fatalf("the model got %d pictures", images)
	}
	if !r.shots.cleared() {
		t.Fatal("a sent picture was kept")
	}
	if b := surface.frames(1)[0].(*feature.BubbleFrame); b.Tone != "answer" || b.Text != res.Answer.Text {
		t.Fatalf("bubble = %+v", b)
	}

	// once a session: the same route is not asked again
	r.model.reply("Still the editor.")
	if err := cli.call(protocol.MOniInspect, protocol.Empty{}, &res); err != nil || res.Status != protocol.OniAskAnswered {
		t.Fatalf("a second inspect this session = %+v, %v", res, err)
	}
	// every time: it is asked again
	if _, err := r.d.applySettings(context.Background(), json.RawMessage(`{"oni":{"disclosure":"always"}}`)); err != nil {
		t.Fatal(err)
	}
	inspectFor(t, cli)
	if r.clock.Armed() != 1 {
		t.Fatalf("%d expiries armed for one question", r.clock.Armed())
	}
	// detaching drops the question and its picture
	if err := cli.call(protocol.MOniDetach, protocol.Empty{}, nil); err != nil {
		t.Fatal(err)
	}
	if !r.shots.cleared() || r.clock.Armed() != 0 {
		t.Fatal("a detach kept the question or its expiry")
	}
}

func TestOniAskPointsInsideTheWindowAndRefusesPrivateFields(t *testing.T) {
	r := serveWire(t, true, localEyes)
	surface, _ := r.presenting(t)
	app := r.dialApp(t)
	perchOn(t, app, "0x1")
	surface.frames(2)
	// the presenter masks its own pixels where it sits over the window
	if err := surface.call(protocol.MOniOverlay, protocol.OniOverlayParams{Output: "DP-1", Rects: []protocol.OniRect{{X: 100, Y: 80, W: 200, H: 150}}}, nil); err != nil {
		t.Fatal(err)
	}
	ask := func(text string) protocol.OniAskResult {
		t.Helper()
		var res protocol.OniAskResult
		if err := app.call(protocol.MOniAsk, protocol.OniAskParams{Text: text}, &res); err != nil {
			t.Fatal(err)
		}
		if res.Status == protocol.OniAskConsent {
			surface.frames(1)
			if err := app.call(protocol.MOniConsentAnswer, protocol.OniConsentAnswerParams{ID: res.Consent.ID, Answer: "yes"}, &res); err != nil {
				t.Fatal(err)
			}
		}
		return res
	}
	r.model.reply("Save sits at the top left.\n[POINT:0.5,0.5,0.125,0.0625:Save][CONF:0.9]")
	res := ask("Where do I save?")
	a := res.Answer
	if a == nil || a.Point == nil || *a.Point != (protocol.OniRect{X: 0.5, Y: 0.5, W: 0.125, H: 0.0625}) || a.Seq == 0 || a.Refused != "" {
		t.Fatalf("answer = %+v", a)
	}
	frames := surface.frames(2)
	target, ok := frames[0].(*feature.StrandTargetFrame)
	if !ok || target.Seq != a.Seq || target.X != 0.5 || target.Y != 0.5 || target.Mode != feature.StrandModePoint {
		t.Fatalf("frames %q: %+v", frameKinds(frames), frames[0])
	}
	if b, ok := frames[1].(*feature.BubbleFrame); !ok || b.Seq != a.Seq || b.Tone != "answer" {
		t.Fatalf("bubble = %+v", frames[1])
	}
	// the overlay is white in what the model saw, and the rest is the window
	pic := r.model.sent()[0].Messages[1].Images[0].Data
	img, err := jpeg.Decode(bytes.NewReader(pic))
	if err != nil {
		t.Fatal(err)
	}
	if !nearly(img.At(20, 20), 255) || !nearly(img.At(600, 500), 40) {
		t.Fatalf("masked pixel %v, window pixel %v", img.At(20, 20), img.At(600, 500))
	}

	// a private field is refused in code, with no strand sent
	r.model.reply("Type it here.\n[POINT:0.25,0.25,0.125,0.0625:Password][CONF:0.9]")
	res = ask("Where is the password box?")
	if res.Answer == nil || res.Answer.Refused != screenask.CodeSensitive || res.Answer.Text != screenask.RefusalLine || res.Answer.Point != nil {
		t.Fatalf("a private field = %+v", res.Answer)
	}
	if b := surface.frames(1)[0].(*feature.BubbleFrame); b.Tone != "warn" || b.Text != screenask.RefusalLine {
		t.Fatalf("refusal bubble = %+v", b)
	}

	// a question better answered in words is asked again as one
	r.model.reply("[ANSWER]", "It is a notes editor.")
	res = ask("What is this window for?")
	if res.Answer == nil || res.Answer.Text != "It is a notes editor." || res.Answer.Mode != screenask.ModeAsk {
		t.Fatalf("a words answer = %+v", res.Answer)
	}
	if !r.shots.cleared() {
		t.Fatal("a picture outlived its question")
	}
}

// The presenter reports a moving part (the guide card gliding to its new
// place) as it commits each frame, before the screen shows it there. A
// capture asked for mid-glide waits until the reports hold still and masks
// where the part came to rest; one that keeps moving past the wait masks
// every place it was reported at lately.
func TestOniCaptureMasksThePresenterOnceItHoldsStill(t *testing.T) {
	r := serveWire(t, true, localEyes)
	surface, _ := r.presenting(t)
	app := r.dialApp(t)
	perchOn(t, app, "0x1")
	surface.frames(2)
	r.allowRoute()
	at := func(x float64) protocol.OniOverlayParams {
		return protocol.OniOverlayParams{Output: "DP-1", Rects: []protocol.OniRect{{X: x, Y: 80, W: 200, H: 120}}}
	}
	glide := func(from, steps int, every time.Duration) chan error {
		done := make(chan error, 1)
		go func() {
			for i := range steps {
				if err := surface.call(protocol.MOniOverlay, at(float64(from+10*i)), nil); err != nil {
					done <- err
					return
				}
				time.Sleep(every)
			}
			done <- nil
		}()
		return done
	}
	ask := func() {
		t.Helper()
		r.model.reply("Save sits at the top left.\n[POINT:0.5,0.5,0.125,0.0625:Save][CONF:0.9]")
		var res protocol.OniAskResult
		if err := app.call(protocol.MOniAsk, protocol.OniAskParams{Text: "Where do I save?"}, &res); err != nil || res.Status != protocol.OniAskAnswered {
			t.Fatalf("ask = %+v, %v", res, err)
		}
	}

	// a glide of ten frames, 30 ms apart; the ask comes in its first frames
	done := glide(100, 10, 30*time.Millisecond)
	time.Sleep(40 * time.Millisecond)
	ask()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if got, want := r.shots.lastMasks(), []window.Box{{X: 190, Y: 80, W: 200, H: 120}}; !slices.Equal(got, want) {
		t.Fatalf("mid-glide the capture masked %+v, want where the card came to rest %+v", got, want)
	}

	// a part that never holds still: the capture goes ahead after the wait
	// and masks the places reported in the trail as well
	done = glide(400, 120, 20*time.Millisecond)
	time.Sleep(40 * time.Millisecond)
	began := time.Now()
	ask()
	took := time.Since(began)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	got := r.shots.lastMasks()
	places := map[float64]bool{}
	for _, m := range got {
		places[m.X] = true
	}
	if took < oniOverlayWait || took > oniOverlayWait+time.Second || len(places) < 3 {
		t.Fatalf("a restless presenter: the capture took %v with masks %+v", took, got)
	}
}

func nearly(c color.Color, v uint32) bool {
	r, g, b, _ := c.RGBA()
	for _, ch := range []uint32{r >> 8, g >> 8, b >> 8} {
		if ch+12 < v || ch > v+12 {
			return false
		}
	}
	return true
}

func TestOniHostedEmberRouteNeedsTheApp(t *testing.T) {
	r := serveWire(t, true, llm.Plan{Provider: protocol.ProviderEmber, Model: "eyes", Vision: true, Paid: true})
	surface, _ := r.presenting(t)
	cli := dialSocket(t, protocol.ClientCLI)
	perchOn(t, cli, "0x1")
	surface.frames(2)
	if err := cli.call(protocol.MOniInspect, protocol.Empty{}, nil); !forbidden(err) {
		t.Fatalf("the CLI used the account's model: %v", err)
	}
	if r.grim.count() != 0 {
		t.Fatal("a refused hosted question captured the screen")
	}
	app := r.dialApp(t)
	consent := inspectFor(t, app)
	if consent.Destination != "hosted" || consent.Route != protocol.ProviderEmber+"/eyes" {
		t.Fatalf("consent = %+v", consent)
	}
	r.model.mu.Lock()
	r.model.planErr = &llm.NoRoute{Reason: "Nothing here reads pictures.", NoVision: true}
	r.model.mu.Unlock()
	if code, _ := reasonOf(app.call(protocol.MOniInspect, protocol.Empty{}, nil)); code != protocol.CodeNoVision {
		t.Fatalf("no picture model: %s", code)
	}
}

func TestOniInspectOffCapturesNothing(t *testing.T) {
	r := serveWire(t, false, localEyes)
	surface, _ := r.presenting(t)
	cli := dialSocket(t, protocol.ClientCLI)
	perchOn(t, cli, "0x1")
	surface.frames(2)
	err := cli.call(protocol.MOniAsk, protocol.OniAskParams{Text: "What is this?"}, nil)
	if code, reason := reasonOf(err); code != protocol.CodeDisabled || reason != feature.ReasonFeatureOff {
		t.Fatalf("ask with looking off: %v", err)
	}
	if r.grim.count() != 0 || r.shots.count() != 0 || len(r.model.sent()) != 0 {
		t.Fatal("looking off still captured or sent")
	}
}

func TestOniNewCallsAreForbiddenToAgentsTeamsSurfacesAndObservers(t *testing.T) {
	r := serveWire(t, true, localEyes)
	surface, watcher := r.presenting(t)
	second := dialSocket(t, protocol.ClientSurface)
	if second.open(protocol.OniRendererNative).Presenter {
		t.Fatal("a second surface became presenter")
	}
	bar := dialSocket(t, protocol.ClientBar)
	hub := dialSocket(t, protocol.ClientHub)
	params := map[string]any{
		protocol.MOniPerch:         protocol.OniPerchParams{Window: "0x1"},
		protocol.MOniDetach:        protocol.OniDetachParams{},
		protocol.MOniInspect:       protocol.Empty{},
		protocol.MOniAsk:           protocol.OniAskParams{Text: "What is this?"},
		protocol.MOniConsentAnswer: protocol.OniConsentAnswerParams{ID: "c-1", Answer: "yes"},
		protocol.MOniOverlay:       protocol.OniOverlayParams{Output: "DP-1", Rects: []protocol.OniRect{}},
	}
	for method, p := range params {
		for name, peer := range map[string]*socketPeer{"a non-presenter surface": second, "an observer": watcher, "the bar": bar, "the hub": hub} {
			if err := peer.call(method, p, nil); !forbidden(err) {
				t.Errorf("%s called %s: %v", name, method, err)
			}
		}
		for _, info := range []protocol.SessionInfo{{ID: "ask-1", Kind: protocol.KindAsk}, {ID: "agent-1", Kind: protocol.KindAgent, Team: "t-1"}} {
			if err := r.d.ToolHost(info).Call(context.Background(), method, p, nil); !forbidden(err) {
				t.Errorf("an agent's tools (team %q) called %s: %v", info.Team, method, err)
			}
		}
		if err := agentGate(method); !forbidden(err) {
			t.Errorf("a team session's socket may call %s", method)
		}
	}
	for _, method := range []string{protocol.MOniInspect, protocol.MOniAsk, protocol.MOniConsentAnswer} {
		if err := surface.call(method, params[method], nil); !forbidden(err) {
			t.Errorf("the presenter called %s: %v", method, err)
		}
	}
	cli := dialSocket(t, protocol.ClientCLI)
	if err := cli.call(protocol.MOniOverlay, params[protocol.MOniOverlay], nil); !forbidden(err) {
		t.Errorf("a CLI reported the overlay: %v", err)
	}
	if r.grim.count() != 0 || len(r.model.sent()) != 0 || r.detects.Load() != 0 {
		t.Fatal("a forbidden call reached the desktop, the screen or the model")
	}
	if _, ok := r.d.oni.service.Load().Perched(); ok {
		t.Fatal("a forbidden call perched Oni")
	}
}

func TestOniNewCallsAreRefusedOnATeamSocket(t *testing.T) {
	isolateDaemon(t)
	shortRuntime(t)
	crew := &boundTeams{handled: make(chan string, 8)}
	d := New(refuseRealDesktop(t, Deps{Teams: crew}))
	team, err := d.manager.create(protocol.SessionNewParams{Kind: protocol.KindAgent, Cwd: filepath.Join(paths.Home(), "Teams", "one"), Team: "team-1", TeamTask: "task-1", Member: "member-1"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.agentSandbox(team); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(d.closeAgentSockets)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, err := client.Dial(ctx, client.Options{Client: protocol.ClientCLI, Socket: paths.AgentSocket(team.ID)})
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	for _, method := range []string{protocol.MOniPerch, protocol.MOniDetach, protocol.MOniInspect, protocol.MOniAsk, protocol.MOniConsentAnswer, protocol.MOniOverlay} {
		if err := conn.Call(ctx, method, protocol.Empty{}, nil); !forbidden(err) {
			t.Errorf("a team socket called %s: %v", method, err)
		}
	}
}

func TestOniPerchRunsNothingAtRestAndKeepsNoOneAwake(t *testing.T) {
	r := serveWire(t, true, localEyes)
	surface, watcher := r.presenting(t)
	cli := dialSocket(t, protocol.ClientCLI)
	capabilities := func() json.RawMessage {
		var status struct {
			Capabilities json.RawMessage `json:"capabilities"`
		}
		if err := cli.call(protocol.MOniStatus, protocol.Empty{}, &status); err != nil {
			t.Fatal(err)
		}
		return status.Capabilities
	}
	before := capabilities()
	// nothing is read, watched, polled or armed before a perch
	if r.detects.Load() != 0 || r.grim.count() != 0 || r.clock.Armed() != 0 {
		t.Fatal("Oni read the desktop or armed a timer with no perch")
	}
	if w, g := r.desk.state(); w != 0 || g != "" {
		t.Fatal("a watch or the poll runs with no perch")
	}
	perchOn(t, cli, "0x1")
	surface.frames(2)
	r.desk.await(t, 1, "0x1")
	if !bytes.Equal(capabilities(), before) {
		t.Fatal("perching changed the capabilities")
	}
	// a perched Oni asleep keeps the window watch but no geometry poll;
	// waking turns the poll back on and catches up with the window
	r.d.oniRest(true)
	r.desk.await(t, 1, "")
	asleep := desktopWindows()
	asleep.Windows[0].Box = &window.Box{X: 120, Y: 80, W: 800, H: 600}
	r.desk.mu.Lock()
	r.desk.snap = asleep
	r.desk.mu.Unlock()
	r.d.oniRest(false)
	r.desk.await(t, 1, "0x1")
	if p := surface.frames(1)[0].(*feature.PerchFrame); p.Box.X != 120 {
		t.Fatalf("waking caught up as %+v", p)
	}
	cli.conn.Close()
	watcher.conn.Close()
	deadline := time.Now().Add(5 * time.Second)
	for !r.d.Idle() {
		if time.Now().After(deadline) {
			t.Fatal("a perched presenter keeps the daemon up")
		}
		time.Sleep(10 * time.Millisecond)
	}
	r.d.mu.RLock()
	timer := r.d.idle
	r.d.mu.RUnlock()
	moved := desktopWindows()
	moved.Windows[0].Box = &window.Box{X: 300, Y: 80, W: 800, H: 600}
	r.desk.set(moved)
	surface.frames(1)
	r.d.mu.RLock()
	reset := r.d.idle != timer
	r.d.mu.RUnlock()
	if reset || !r.d.Idle() {
		t.Fatal("following the window kept the daemon up or reset its clock")
	}
	// the presenter leaving ends the watch and the poll
	surface.conn.Close()
	r.desk.await(t, 0, "")
	if r.clock.Armed() != 0 {
		t.Fatal("a timer is left armed")
	}
	again := dialSocket(t, protocol.ClientCLI)
	var status struct {
		Capabilities json.RawMessage `json:"capabilities"`
	}
	if err := again.call(protocol.MOniStatus, protocol.Empty{}, &status); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(status.Capabilities, before) {
		t.Fatalf("capabilities changed:\n%s\n%s", status.Capabilities, before)
	}
}

// A perch on a tiled window polls nothing: the layout moves it only with an
// event the watch reads. The poll runs exactly while the perched window
// floats, following it as it floats or tiles in place.
func TestOniGeometryPollRunsOnlyWhileThePerchedWindowFloats(t *testing.T) {
	r := serveWire(t, true, localEyes)
	surface, _ := r.presenting(t)
	cli := dialSocket(t, protocol.ClientCLI)
	tiled := desktopWindows()
	tiled.Windows[0].Floating = false
	r.desk.mu.Lock()
	r.desk.snap = cloneSnapshot(tiled)
	r.desk.mu.Unlock()
	perchOn(t, cli, "0x1")
	surface.frames(2)
	r.desk.await(t, 1, "")
	// the same window floats where it is: the poll starts
	floating := cloneSnapshot(tiled)
	floating.Windows[0].Floating = true
	r.desk.set(floating)
	r.desk.await(t, 1, "0x1")
	// and tiles again in place: the poll stops, the watch stays
	r.desk.set(tiled)
	r.desk.await(t, 1, "")
	// asleep and awake on a tiled window: still nothing polled
	r.d.oniRest(true)
	r.d.oniRest(false)
	r.desk.await(t, 1, "")
}
