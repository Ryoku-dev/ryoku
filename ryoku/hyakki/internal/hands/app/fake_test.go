package app

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"ryoku-rashin/internal/hands/input"
	"ryoku-rashin/internal/oni/hands/discipline"
	"ryoku-rashin/internal/oni/screenask"
	"ryoku-rashin/internal/oni/window"
)

const (
	selfApp = "dev.ryoku.Hyakki"
	tolPx   = 4
)

// The desk: two outputs side by side, a floating editor (the target), a
// tiled browser with a box on the second output (a Hyprland-style placed
// tile) and a tiled terminal niri does not place.
func desk() window.Snapshot {
	return window.Snapshot{
		Outputs: []window.Output{
			{Name: "DP-1", X: 0, Y: 0, W: 1920, H: 1080, Scale: 1},
			{Name: "DP-2", X: 1920, Y: 0, W: 2560, H: 1440, Scale: 1},
		},
		Windows: []window.Window{
			{ID: "0x1", App: "org.gnome.TextEditor", Title: "Notes", Output: "DP-1", PID: 100,
				Box: &window.Box{X: 100, Y: 100, W: 800, H: 600}, Floating: true, Focused: true, Visible: true},
			{ID: "0x2", App: "org.mozilla.firefox", Title: "Start", Output: "DP-2", PID: 200,
				Box: &window.Box{X: 0, Y: 0, W: 1280, H: 1440}, Visible: true, FocusOrder: 1},
			{ID: "7", App: "kitty", Title: "shell", Output: "DP-2", PID: 300,
				Unplaced: "tiled", Visible: true, FocusOrder: 2},
		},
	}
}

func win(s *window.Snapshot, id window.ID) *window.Window {
	for i := range s.Windows {
		if s.Windows[i].ID == id {
			return &s.Windows[i]
		}
	}
	panic("no window " + string(id))
}

// focus gives id the focus and takes it from every other window.
func focus(s *window.Snapshot, id window.ID) {
	for i := range s.Windows {
		s.Windows[i].Focused = s.Windows[i].ID == id
	}
}

// fakeWindows serves snap; before the n-th read (from 1) it applies
// change[n], so a test can make the n-th check see something else.
type fakeWindows struct {
	mu     sync.Mutex
	snap   window.Snapshot
	reads  int
	change map[int]func(*window.Snapshot)
	fail   map[int]bool
}

func (f *fakeWindows) Snapshot(ctx context.Context) (window.Snapshot, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.reads++
	if c := f.change[f.reads]; c != nil {
		c(&f.snap)
	}
	if f.fail[f.reads] {
		return window.Snapshot{}, errors.New("compositor gone")
	}
	return clone(f.snap), nil
}

func (f *fakeWindows) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.reads
}

func (f *fakeWindows) edit(fn func(*window.Snapshot)) {
	f.mu.Lock()
	defer f.mu.Unlock()
	fn(&f.snap)
}

func clone(s window.Snapshot) window.Snapshot {
	out := window.Snapshot{Outputs: slices.Clone(s.Outputs), Windows: slices.Clone(s.Windows)}
	for i := range out.Windows {
		if b := out.Windows[i].Box; b != nil {
			box := *b
			out.Windows[i].Box = &box
		}
	}
	return out
}

// call is one keyboard tool run.
type call struct {
	env  []string
	name string
	args []string
}

// fakeKeyboard records every keyboard tool run and never runs anything.
// fail makes the n-th run (from 1) fail with an error that echoes its
// arguments, as a tool's stderr might; hook runs during the n-th run.
type fakeKeyboard struct {
	mu    sync.Mutex
	calls []call
	fail  map[int]bool
	hook  map[int]func()
}

func (k *fakeKeyboard) runner(env []string) input.Runner {
	return func(ctx context.Context, name string, args ...string) error {
		k.mu.Lock()
		k.calls = append(k.calls, call{env: slices.Clone(env), name: name, args: slices.Clone(args)})
		n := len(k.calls)
		hook, fail := k.hook[n], k.fail[n]
		k.mu.Unlock()
		if hook != nil {
			hook()
		}
		if fail {
			return fmt.Errorf("the tool choked on %q", strings.Join(args, " "))
		}
		return nil
	}
}

func (k *fakeKeyboard) runs() []call {
	k.mu.Lock()
	defer k.mu.Unlock()
	return slices.Clone(k.calls)
}

// typed is the text of every typing run, in order.
func (k *fakeKeyboard) typed() []string {
	var out []string
	for _, c := range k.runs() {
		if len(c.args) == 4 && c.args[2] == "--" {
			out = append(out, c.args[3])
		}
	}
	return out
}

// fakePointer records what a click did.
type fakePointer struct {
	mu      sync.Mutex
	sockets []string
	ops     []string
	onMove  func()
	dialErr error
}

func (p *fakePointer) dial(socket string) (input.Pointer, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.sockets = append(p.sockets, socket)
	if p.dialErr != nil {
		return nil, p.dialErr
	}
	return pointerConn{p}, nil
}

func (p *fakePointer) record(op string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.ops = append(p.ops, op)
}

func (p *fakePointer) log() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return slices.Clone(p.ops)
}

func (p *fakePointer) dials() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.sockets)
}

type pointerConn struct{ p *fakePointer }

func (c pointerConn) Move(x, y, w, h int) error {
	c.p.record(fmt.Sprintf("move %d,%d of %dx%d", x, y, w, h))
	if c.p.onMove != nil {
		c.p.onMove()
	}
	return nil
}

func (c pointerConn) Button(code uint32, down bool) error {
	c.p.record(fmt.Sprintf("button %#x %v", code, down))
	return nil
}

func (c pointerConn) Scroll(dx, dy int) error {
	c.p.record("scroll")
	return nil
}

func (c pointerConn) Close() { c.p.record("close") }

// fakeShooter hands out a fixed picture and records the masks and clears.
type fakeShooter struct {
	mu      sync.Mutex
	err     error
	masks   [][]window.Box
	targets []window.ID
	shots   int
	clears  int
}

func (s *fakeShooter) Shoot(ctx context.Context, w window.Window, masks []window.Box) ([]byte, func(), error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.targets = append(s.targets, w.ID)
	s.masks = append(s.masks, slices.Clone(masks))
	if s.err != nil {
		return nil, nil, s.err
	}
	s.shots++
	return []byte("png"), func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		s.clears++
	}, nil
}

// fakeOCR answers each read with the next entry of reads.
type fakeOCR struct {
	mu    sync.Mutex
	reads []ocrRead
	n     int
}

type ocrRead struct {
	words []string
	err   error
}

func (o *fakeOCR) Words(ctx context.Context, png []byte) ([]screenask.Word, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	r := ocrRead{err: errors.New("no reads left")}
	if o.n < len(o.reads) {
		r = o.reads[o.n]
	}
	o.n++
	if r.err != nil {
		return nil, r.err
	}
	var out []screenask.Word
	for _, w := range r.words {
		out = append(out, screenask.Word{Text: w})
	}
	return out, nil
}

// fakeKeys is the key-edge reader: press calls the driver's onKey.
type fakeKeys struct {
	mu      sync.Mutex
	onKey   func()
	starts  int
	stops   int
	refused error
}

func (k *fakeKeys) start(onKey func()) (func(), error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	if k.refused != nil {
		return nil, k.refused
	}
	k.starts++
	k.onKey = onKey
	return func() {
		k.mu.Lock()
		defer k.mu.Unlock()
		k.stops++
	}, nil
}

func (k *fakeKeys) press() {
	k.mu.Lock()
	f := k.onKey
	k.mu.Unlock()
	f()
}

// rig is one driver with every fake.
type rig struct {
	t       *testing.T
	d       *Driver
	win     *fakeWindows
	kb      *fakeKeyboard
	ptr     *fakePointer
	shots   *fakeShooter
	ocr     *fakeOCR
	keys    *fakeKeys
	logs    *bytes.Buffer
	pauses  []discipline.Pause
	pauseMu sync.Mutex
	display Display
}

func limits(chunk int) Limits {
	return Limits{GeometryTolPx: tolPx, Verify: time.Second, Chunk: time.Second, ChunkRunes: chunk, MaxTypeRunes: 200, Readback: time.Second}
}

func newRig(t *testing.T, chunk int) *rig {
	t.Helper()
	r := &rig{t: t, win: &fakeWindows{snap: desk()}, kb: &fakeKeyboard{}, ptr: &fakePointer{},
		shots: &fakeShooter{}, ocr: &fakeOCR{}, keys: &fakeKeys{}, logs: &bytes.Buffer{},
		display: Display{RuntimeDir: "/run/user/1000", Name: "wayland-1"}}
	d, err := New(Config{
		Display:  r.display,
		Windows:  r.win,
		Pictures: r.shots,
		OCR:      r.ocr,
		Masks: func(output string) []window.Box {
			return []window.Box{{X: 0, Y: 0, W: 1920, H: 130}, {X: 0, Y: 950, W: 1920, H: 130}}
		},
		Keys:   r.keys.start,
		Self:   []string{selfApp},
		Limits: limits(chunk),
		Log:    slog.New(slog.NewTextHandler(r.logs, nil)),
		OnPause: func(p discipline.Pause) {
			r.pauseMu.Lock()
			defer r.pauseMu.Unlock()
			r.pauses = append(r.pauses, p)
		},
		Run:  r.kb.runner,
		Dial: r.ptr.dial,
	})
	if err != nil {
		t.Fatal(err)
	}
	r.d = d
	return r
}

// pick selects id and starts counting reads afresh.
func (r *rig) pick(id window.ID) {
	r.t.Helper()
	if _, err := r.d.Select(context.Background(), id); err != nil {
		r.t.Fatalf("select %s: %v", id, err)
	}
	r.win.mu.Lock()
	r.win.reads = 0
	r.win.mu.Unlock()
}

func (r *rig) told() []discipline.Pause {
	r.pauseMu.Lock()
	defer r.pauseMu.Unlock()
	return slices.Clone(r.pauses)
}

func pausedFor(t *testing.T, err error, want discipline.Pause) {
	t.Helper()
	var p *Paused
	if !errors.As(err, &p) || p.Reason != want {
		t.Fatalf("err = %v, want a pause for %q", err, want)
	}
}

func refusedFor(t *testing.T, err error, want string) {
	t.Helper()
	var r *Refused
	if !errors.As(err, &r) || r.Why != want {
		t.Fatalf("err = %v, want a refusal for %q", err, want)
	}
}
