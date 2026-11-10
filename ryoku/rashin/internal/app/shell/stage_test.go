package shell

import (
	"image"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"gioui.org/f32"
	"gioui.org/io/input"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/anim"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"

	"ryoku-rashin/internal/app/desktop"
	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/state"
)

// stage is the whole app against the demo daemon with a real input router and
// no window: a test points, presses and types the way a person does, and
// finds controls by the name a screen reader would say.
type stage struct {
	t       *testing.T
	runtime *runtime
	shell   *uiShell
	router  input.Router
	ops     op.Ops
	size    image.Point
	now     time.Time
	dirty   atomic.Bool

	// programs the app asked the desktop to start; none is started
	launched []string
	// chooser answers the system file chooser; the machine has none while it is nil
	chooser func(desktop.ChooseRequest) []string
	// moving is a stage with motion on: flakes over a working session never stop, so it cannot wait for stillness
	moving bool
}

func newStage(t *testing.T, size image.Point) *stage {
	t.Helper()
	anim.SetReduce(true)
	t.Cleanup(func() { anim.SetReduce(false) })
	return openStage(t, size, false)
}

// newMovingStage is the app with its motion on. A frame is 16 ms, so a page
// arrives over several of them, and what a page asks for in its first frame
// has to survive until it is fully there.
func newMovingStage(t *testing.T, size image.Point) *stage {
	t.Helper()
	anim.SetReduce(false)
	return openStage(t, size, true)
}

func openStage(t *testing.T, size image.Point, moving bool) *stage {
	t.Helper()
	root := t.TempDir()
	for name, directory := range map[string]string{
		"HOME": "home", "XDG_CONFIG_HOME": "config", "XDG_STATE_HOME": "state", "XDG_CACHE_HOME": "cache", "XDG_RUNTIME_DIR": "run",
	} {
		t.Setenv(name, filepath.Join(root, directory))
	}
	r, err := startRuntime(Options{Demo: true}, palette.Dark)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(r.Close)
	s := &stage{t: t, runtime: r, size: size, now: storyboardEpoch, moving: moving}
	s.shell = newShell(r, Options{Preferences: state.AppPreferences{}.Normalize()})
	s.shell.desktop = env.Desktop{
		Launch: func(name string, args []string, done func(error)) {
			s.launched = append(s.launched, strings.Join(append([]string{name}, args...), " "))
			if done != nil {
				done(nil)
			}
		},
		Choose: func(request desktop.ChooseRequest, done func([]string, error)) {
			if s.chooser == nil {
				done(nil, desktop.ErrNoChooser)
				return
			}
			done(s.chooser(request), nil)
		},
	}
	t.Cleanup(r.hub.Add(func() { s.dirty.Store(true) }))
	deadline := time.Now().Add(5 * time.Second)
	for r.store.Snapshot().Connection != state.Ready {
		if time.Now().After(deadline) {
			t.Fatal("the demo daemon never answered")
		}
		s.frame()
		time.Sleep(2 * time.Millisecond)
	}
	s.settle()
	return s
}

func (s *stage) frame() {
	s.ops.Reset()
	gtx := layout.Context{
		Ops: &s.ops, Now: s.now, Source: s.router.Source(), Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Exact(s.size),
	}
	s.shell.posts.Drain()
	s.shell.Frame(gtx)
	s.router.Frame(&s.ops)
	s.now = s.now.Add(16 * time.Millisecond)
}

// settle draws until the daemon's answers stop arriving and nothing asks for another frame
func (s *stage) settle() {
	s.t.Helper()
	if s.moving {
		// three quarters of a second of frames, longer than any one motion, with time for the daemon to answer
		for range 45 {
			time.Sleep(time.Millisecond)
			s.frame()
		}
		return
	}
	deadline := time.Now().Add(5 * time.Second)
	for quiet := 0; quiet < 6; {
		if time.Now().After(deadline) {
			s.t.Fatal("the app never came to rest")
		}
		time.Sleep(3 * time.Millisecond)
		// a wake that is still ahead is a caret waiting to blink, not work left to do
		at, wake := s.router.WakeupTime()
		if !s.dirty.Swap(false) && (!wake || at.After(s.now)) {
			quiet++
			continue
		}
		quiet = 0
		s.frame()
	}
	s.frame()
}

func (s *stage) open(route string) {
	s.t.Helper()
	s.shell.openStoryboardRoute(route)
	s.frame()
	s.settle()
}

// every event carries the stage's time, which is what tells a second press
// on the same spot from a new one
func (s *stage) queue(events ...pointer.Event) {
	for _, event := range events {
		event.Source, event.PointerID, event.Time = pointer.Mouse, 1, s.now.Sub(storyboardEpoch)
		s.router.Queue(event)
		s.frame()
	}
}

// a press and its release land in their own frames, as they do from a hand,
// and a hand takes a moment to get there
func (s *stage) clickAt(at image.Point) {
	s.t.Helper()
	s.now = s.now.Add(300 * time.Millisecond)
	position := f32.Pt(float32(at.X), float32(at.Y))
	s.queue(
		pointer.Event{Kind: pointer.Move, Position: position},
		pointer.Event{Kind: pointer.Press, Buttons: pointer.ButtonPrimary, Position: position},
		pointer.Event{Kind: pointer.Release, Position: position},
	)
	s.settle()
}

// drag presses at from and lets go at to, with the pointer seen on the way
func (s *stage) drag(from, to image.Point) {
	s.t.Helper()
	s.now = s.now.Add(300 * time.Millisecond)
	start, end := f32.Pt(float32(from.X), float32(from.Y)), f32.Pt(float32(to.X), float32(to.Y))
	s.queue(
		pointer.Event{Kind: pointer.Move, Position: start},
		pointer.Event{Kind: pointer.Press, Buttons: pointer.ButtonPrimary, Position: start},
		pointer.Event{Kind: pointer.Move, Buttons: pointer.ButtonPrimary, Position: start.Add(end).Mul(.5)},
		pointer.Event{Kind: pointer.Move, Buttons: pointer.ButtonPrimary, Position: end},
		pointer.Event{Kind: pointer.Release, Position: end},
	)
	s.settle()
}

func (s *stage) press(name key.Name, modifiers key.Modifiers) {
	s.t.Helper()
	s.router.Queue(key.Event{Name: name, Modifiers: modifiers, State: key.Press})
	s.frame()
	s.settle()
}

func (s *stage) typeText(text string) {
	s.t.Helper()
	s.router.Queue(key.EditEvent{Text: text})
	s.frame()
	s.settle()
}

// typeOver replaces the first runes of the focused field, as typing over a selection does
func (s *stage) typeOver(runes int, text string) {
	s.t.Helper()
	s.router.Queue(key.EditEvent{Range: key.Range{End: runes}, Text: text})
	s.frame()
	s.settle()
}

// await draws until the daemon's answer has made something true
func (s *stage) await(what string, ready func() bool) {
	s.t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for !ready() {
		if time.Now().After(deadline) {
			s.t.Fatalf("never happened: %s", what)
		}
		time.Sleep(2 * time.Millisecond)
		s.frame()
	}
	s.settle()
}

func (s *stage) nodes() []input.SemanticNode { return s.router.AppendSemantics(nil) }

// named finds the control a screen reader would call name. Text labels the
// area it is drawn in as well, so of several matches the smallest is the control.
func (s *stage) named(name string) (input.SemanticNode, bool) {
	var found input.SemanticNode
	ok := false
	for _, node := range s.nodes() {
		if node.Desc.Label != name && node.Desc.Description != name {
			continue
		}
		size, best := node.Desc.Bounds.Size(), found.Desc.Bounds.Size()
		if !ok || size.X*size.Y < best.X*best.Y {
			found, ok = node, true
		}
	}
	return found, ok
}

func (s *stage) click(name string) {
	s.t.Helper()
	node, ok := s.named(name)
	if !ok {
		s.t.Fatalf("nothing on screen is called %q; there is: %s", name, strings.Join(s.names(), ", "))
	}
	at := centre(node.Desc.Bounds)
	if !at.In(image.Rectangle{Max: s.size}) {
		s.t.Fatalf("%q is drawn at %v, outside the window", name, node.Desc.Bounds)
	}
	s.clickAt(at)
}

func (s *stage) names() []string {
	var names []string
	for _, node := range s.nodes() {
		if name := nodeName(node); name != "" {
			names = append(names, name)
		}
	}
	return names
}

func (s *stage) clipboard() string {
	_, content, _ := s.router.WriteClipboard()
	return string(content)
}

func centre(bounds image.Rectangle) image.Point {
	return image.Pt((bounds.Min.X+bounds.Max.X)/2, (bounds.Min.Y+bounds.Max.Y)/2)
}

func nodeName(node input.SemanticNode) string {
	if node.Desc.Label != "" {
		return node.Desc.Label
	}
	return node.Desc.Description
}
