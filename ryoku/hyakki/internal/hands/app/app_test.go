package app

import (
	"context"
	"errors"
	"fmt"
	"go/parser"
	"go/token"
	"math"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"

	"ryoku-rashin/internal/hands/input"
	"ryoku-rashin/internal/oni/hands/discipline"
	"ryoku-rashin/internal/oni/window"
)

const line = "seven ravens fly" // four chunks of five: "seven", " rave", "ns fl", "y"

// pauses are the ways a window can stop being the one picked, each with the
// reason it must pause for.
var pauses = []struct {
	name   string
	reason discipline.Pause
	change func(*window.Snapshot)
}{
	{"title change", discipline.PauseTitle, func(s *window.Snapshot) { win(s, "0x1").Title = "Notes (edited elsewhere)" }},
	{"geometry beyond tolerance", discipline.PauseGeometry, func(s *window.Snapshot) { win(s, "0x1").Box.W += tolPx + 1 }},
	{"moved beyond tolerance", discipline.PauseGeometry, func(s *window.Snapshot) { win(s, "0x1").Box.X -= tolPx + 1 }},
	{"focus to another window", discipline.PauseFocus, func(s *window.Snapshot) { focus(s, "0x2") }},
	{"no focus at all", discipline.PauseFocus, func(s *window.Snapshot) { focus(s, "") }},
	{"hidden", discipline.PauseHidden, func(s *window.Snapshot) { win(s, "0x1").Visible = false }},
	{"minimized", discipline.PauseHidden, func(s *window.Snapshot) { win(s, "0x1").Minimized = true }},
	{"output change", discipline.PauseOutput, func(s *window.Snapshot) { win(s, "0x1").Output = "DP-2" }},
	{"private focus", discipline.PausePrivate, func(s *window.Snapshot) {
		s.Windows = append(s.Windows, window.Window{ID: "0x9", App: "org.keepassxc.KeePassXC", Title: "Vault", Output: "DP-2", PID: 900, Visible: true})
		focus(s, "0x9")
	}},
	{"self focus", discipline.PauseSelf, func(s *window.Snapshot) {
		s.Windows = append(s.Windows, window.Window{ID: "0x8", App: selfApp, Title: "Hyakki", Output: "DP-2", PID: 800, Visible: true})
		focus(s, "0x8")
	}},
	{"closed", discipline.PauseClosed, func(s *window.Snapshot) {
		s.Windows = slices.DeleteFunc(s.Windows, func(w window.Window) bool { return w.ID == "0x1" })
	}},
	{"covered", discipline.PauseCovered, func(s *window.Snapshot) {
		s.Windows = append(s.Windows, window.Window{ID: "0x7", App: "org.gnome.Calculator", Output: "DP-1", PID: 700,
			Box: &window.Box{X: 500, Y: 500, W: 300, H: 300}, Floating: true, Visible: true})
	}},
}

func TestEveryCheckFailureStopsTypingAtAChunkBoundary(t *testing.T) {
	chunks := input.Chunks(line, 5)
	if len(chunks) != 4 {
		t.Fatalf("chunks %q", chunks)
	}
	cases := append(slices.Clone(pauses), struct {
		name   string
		reason discipline.Pause
		change func(*window.Snapshot)
	}{"unreadable windows", discipline.PauseWindow, nil})
	for _, tc := range cases {
		// checks run before and after each chunk: 2 per chunk
		for k := 1; k <= 2*len(chunks); k++ {
			t.Run(fmt.Sprintf("%s at check %d", tc.name, k), func(t *testing.T) {
				r := newRig(t, 5)
				r.pick("0x1")
				if tc.change != nil {
					r.win.change = map[int]func(*window.Snapshot){k: tc.change}
				} else {
					r.win.fail = map[int]bool{k: true}
				}
				res, err := r.d.Type(context.Background(), line)
				pausedFor(t, err, tc.reason)
				runs := k / 2 // a failed check before chunk n+1 or after chunk n leaves n typed
				if got := r.kb.typed(); !slices.Equal(got, chunks[:runs]) {
					t.Fatalf("typed %q, want %q", got, chunks[:runs])
				}
				landed := utf8.RuneCountInString(strings.Join(chunks[:runs], ""))
				if res.Landed != landed || res.Rest != strings.Join(chunks[runs:], "") || res.Unsure != 0 {
					t.Fatalf("result %+v (rest %q), want landed %d", res, res.Rest, landed)
				}
				if r.win.count() != k {
					t.Fatalf("%d checks ran after the failing one", r.win.count()-k)
				}
				if got := r.told(); !slices.Equal(got, []discipline.Pause{tc.reason}) {
					t.Fatalf("OnPause heard %v", got)
				}
				// paused: nothing is typed, nothing is read, until Resume
				if _, err := r.d.Type(context.Background(), "more"); !errors.As(err, new(*Paused)) {
					t.Fatalf("a paused driver typed: %v", err)
				}
				if r.win.count() != k || len(r.kb.runs()) != runs {
					t.Fatal("a paused driver read the windows or ran the tool")
				}
			})
		}
	}
}

func TestAMoveWithinToleranceDoesNotPause(t *testing.T) {
	r := newRig(t, 5)
	r.pick("0x1")
	r.win.change = map[int]func(*window.Snapshot){3: func(s *window.Snapshot) {
		b := win(s, "0x1").Box
		b.X, b.W = b.X+tolPx, b.W-tolPx
	}}
	res, err := r.d.Type(context.Background(), line)
	if err != nil || res.Landed != utf8.RuneCountInString(line) || res.Rest != "" {
		t.Fatalf("%+v, %v", res, err)
	}
	if got := strings.Join(r.kb.typed(), ""); got != line || r.win.count() != 8 {
		t.Fatalf("typed %q with %d checks", got, r.win.count())
	}
}

func TestAUserKeyPauses(t *testing.T) {
	r := newRig(t, 5)
	r.pick("0x1")
	// the person types while the second chunk goes in
	r.kb.hook = map[int]func(){2: r.keys.press}
	res, err := r.d.Type(context.Background(), line)
	pausedFor(t, err, discipline.PauseKey)
	if res.Landed != 10 || len(r.kb.runs()) != 2 || r.d.Paused() != discipline.PauseKey {
		t.Fatalf("%v after %d runs, paused %q", res, len(r.kb.runs()), r.d.Paused())
	}
	if got := r.told(); !slices.Equal(got, []discipline.Pause{discipline.PauseKey}) {
		t.Fatalf("OnPause heard %v", got)
	}
	// more keys while paused, then Resume: those keys no longer count
	r.keys.press()
	r.keys.press()
	r.d.Resume()
	if _, err := r.d.Type(context.Background(), res.Rest); err != nil {
		t.Fatalf("after Resume: %v", err)
	}
	if got := strings.Join(r.kb.typed(), ""); got != line {
		t.Fatalf("typed %q", got)
	}
	// a key between actions pauses the next one before it reads anything
	reads, runs := r.win.count(), len(r.kb.runs())
	r.keys.press()
	pausedFor(t, r.d.Press(context.Background(), "Tab"), discipline.PauseKey)
	if r.win.count() != reads || len(r.kb.runs()) != runs {
		t.Fatal("a paused press read the windows or ran the tool")
	}
}

func TestNoKeyReaderNoTask(t *testing.T) {
	r := newRig(t, 5)
	r.keys.refused = errors.New("keys: keyboards unavailable: permission")
	if _, err := r.d.Select(context.Background(), "0x1"); !errors.Is(err, ErrNoKeys) {
		t.Fatalf("select without a key reader: %v", err)
	}
	if _, err := r.d.Type(context.Background(), "hello"); !errors.Is(err, ErrNotSelected) || len(r.kb.runs()) != 0 {
		t.Fatalf("typed with no pick: %v", err)
	}
}

func TestSelectRefusesWhatHandsNeverDrives(t *testing.T) {
	for name, tc := range map[string]struct {
		change func(*window.Snapshot)
		id     window.ID
		why    discipline.Reason
	}{
		"missing": {nil, "0x5", discipline.ReasonMissing},
		"self": {func(s *window.Snapshot) {
			s.Windows = append(s.Windows, window.Window{ID: "0x8", App: selfApp, Output: "DP-1", PID: 8, Visible: true})
		}, "0x8", discipline.ReasonSelf},
		"private": {func(s *window.Snapshot) {
			s.Windows = append(s.Windows, window.Window{ID: "0x9", App: "polkit-gnome-authentication-agent-1", Output: "DP-1", PID: 9, Visible: true})
		}, "0x9", discipline.ReasonPrivate},
		"hidden":   {func(s *window.Snapshot) { win(s, "0x1").Minimized = true }, "0x1", discipline.ReasonHidden},
		"unpinned": {func(s *window.Snapshot) { win(s, "0x1").PID = 0; win(s, "0x2").App = win(s, "0x1").App }, "0x1", discipline.ReasonUnpinned},
	} {
		t.Run(name, func(t *testing.T) {
			r := newRig(t, 5)
			if tc.change != nil {
				r.win.edit(tc.change)
			}
			_, err := r.d.Select(context.Background(), tc.id)
			refusedFor(t, err, string(tc.why))
			if r.keys.starts != 0 {
				t.Fatal("a refused pick started the key reader")
			}
		})
	}
}

func TestWindowsListsOnlyPickableAndFlagsKeyboardOnly(t *testing.T) {
	r := newRig(t, 5)
	r.win.edit(func(s *window.Snapshot) {
		s.Windows = append(s.Windows,
			window.Window{ID: "0x8", App: selfApp, Output: "DP-1", PID: 8, Visible: true},
			window.Window{ID: "0x9", App: "com.bitwarden.desktop", Output: "DP-1", PID: 9, Visible: true},
			window.Window{ID: "0xa", App: "org.gnome.Maps", Output: "DP-1", PID: 10, Minimized: true})
	})
	got, err := r.d.Windows(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	want := []Cand{
		{ID: "0x1", App: "org.gnome.TextEditor", Title: "Notes", Clickable: true},
		{ID: "0x2", App: "org.mozilla.firefox", Title: "Start", Clickable: true},
		{ID: "7", App: "kitty", Title: "shell", Clickable: false},
	}
	if !slices.Equal(got, want) {
		t.Fatalf("windows %+v", got)
	}
}

func TestTypingRefusesWhatTheRulesRefuse(t *testing.T) {
	for text, why := range map[string]string{
		"line one\nline two":          "submit",
		"tab\there":                   "control",
		"sk-proj-abcdefghijklmnop123": "secret",
		"   ":                         "empty",
		strings.Repeat("word ", 41):   "long",
	} {
		r := newRig(t, 5)
		r.pick("0x1")
		res, err := r.d.Type(context.Background(), text)
		refusedFor(t, err, why)
		if res.Rest != text || len(r.kb.runs()) != 0 || r.win.count() != 0 {
			t.Fatalf("%q: %+v, %d runs, %d reads", why, res, len(r.kb.runs()), r.win.count())
		}
	}
}

func TestAToolFailureLeavesTheChunkUnsure(t *testing.T) {
	r := newRig(t, 5)
	r.pick("0x1")
	r.kb.fail = map[int]bool{2: true}
	res, err := r.d.Type(context.Background(), line)
	if !errors.Is(err, ErrKeyboard) || res.Landed != 5 || res.Unsure != 5 || res.Rest != line[5:] {
		t.Fatalf("%+v (rest %q), %v", res, res.Rest, err)
	}
}

func TestPressesOnlyTheSafeSet(t *testing.T) {
	for _, chord := range []string{"Tab", "shift+Tab", "Left", "Right", "Up", "Down", "Home", "End", "PageUp", "PageDown", "BackSpace", "ctrl+z"} {
		r := newRig(t, 5)
		r.pick("0x1")
		if err := r.d.Press(context.Background(), chord); err != nil {
			t.Fatalf("%s: %v", chord, err)
		}
		runs := r.kb.runs()
		if len(runs) != 1 || runs[0].name != input.Tool || !input.Allowed(runs[0].name, runs[0].args...) || r.win.count() != 2 {
			t.Fatalf("%s: runs %v, %d checks", chord, runs, r.win.count())
		}
	}
	for chord, why := range map[string]string{
		"Return": "submit", "ctrl+Return": "submit", "ctrl+v": "paste", "shift+Insert": "paste",
		"super+l": "system-key", "ctrl+alt+F2": "system-key", "alt+F4": "risky-key", "ctrl+w": "risky-key",
		"ctrl+q": "risky-key", "Delete": "risky-key", "Escape": "risky-key", "F5": "risky-key", "nonsense+key": "system-key",
	} {
		r := newRig(t, 5)
		r.pick("0x1")
		refusedFor(t, r.d.Press(context.Background(), chord), why)
		if len(r.kb.runs()) != 0 || r.win.count() != 0 {
			t.Fatalf("%s ran or read", chord)
		}
	}
}

func TestPressIsCheckedBeforeAndAfter(t *testing.T) {
	r := newRig(t, 5)
	r.pick("0x1")
	r.win.change = map[int]func(*window.Snapshot){1: pauses[0].change}
	pausedFor(t, r.d.Press(context.Background(), "Tab"), discipline.PauseTitle)
	if len(r.kb.runs()) != 0 {
		t.Fatal("pressed after a failed check")
	}
	r = newRig(t, 5)
	r.pick("0x1")
	r.win.change = map[int]func(*window.Snapshot){2: func(s *window.Snapshot) { focus(s, "0x2") }}
	pausedFor(t, r.d.Press(context.Background(), "Tab"), discipline.PauseFocus)
	if len(r.kb.runs()) != 1 {
		t.Fatal("the press did not run before the after-check")
	}
}

func TestClickLandsInsideAFloatingOrPlacedWindow(t *testing.T) {
	r := newRig(t, 5)
	r.pick("0x1")
	if err := r.d.Click(context.Background(), 0.5, 0.5); err != nil {
		t.Fatal(err)
	}
	// Every cursor position stays in the picked window; the click is at
	// the requested point in the complete multi-output coordinate space.
	var lastX, lastY, presses, releases int
	for _, op := range r.ptr.log() {
		if strings.HasPrefix(op, "move ") {
			var width, height int
			if _, err := fmt.Sscanf(op, "move %d,%d of %dx%d", &lastX, &lastY, &width, &height); err != nil {
				t.Fatal(err)
			}
			if lastX < 100 || lastX >= 900 || lastY < 100 || lastY >= 700 || width != 4480 || height != 1440 {
				t.Fatalf("cursor left the picked window: %s", op)
			}
		}
		if op == "button 0x110 true" {
			presses++
			if lastX != 500 || lastY != 400 {
				t.Fatalf("pressed away from the requested point: %d,%d", lastX, lastY)
			}
		}
		if op == "button 0x110 false" {
			releases++
		}
	}
	if presses != 1 || releases != 1 {
		t.Fatalf("not one complete click: %q", r.ptr.log())
	}

	// a Hyprland-style tile has a box, so it is placed
	r = newRig(t, 5)
	r.win.edit(func(s *window.Snapshot) { focus(s, "0x2") })
	r.pick("0x2")
	if err := r.d.Click(context.Background(), 0.25, 0.5); err != nil {
		t.Fatal(err)
	}
	if got := r.ptr.log(); !slices.Contains(got, "move 2240,720 of 4480x1440") {
		t.Fatalf("requested second-output point not reached: %q", got)
	}
}

func TestClickRefusals(t *testing.T) {
	// a tiled niri window has no box: keys only
	r := newRig(t, 5)
	r.win.edit(func(s *window.Snapshot) { focus(s, "7") })
	r.pick("7")
	refusedFor(t, r.d.Click(context.Background(), 0.5, 0.5), "unplaced")
	if r.ptr.dials() != 0 || r.win.count() != 0 {
		t.Fatal("a click on an unplaced window reached the pointer or the windows")
	}
	if _, err := r.d.Type(context.Background(), "ok"); err != nil {
		t.Fatalf("typing into a tiled window: %v", err)
	}

	for _, p := range [][2]float64{{1, 0.5}, {0.5, 1}, {-0.01, 0.5}, {0.5, 1.5}, {math.NaN(), 0.5}, {0.5, math.Inf(1)}} {
		r := newRig(t, 5)
		r.pick("0x1")
		refusedFor(t, r.d.Click(context.Background(), p[0], p[1]), "region")
		if r.ptr.dials() != 0 {
			t.Fatalf("%v reached the pointer", p)
		}
	}

	// a window that may sit over the point
	r = newRig(t, 5)
	r.win.edit(func(s *window.Snapshot) {
		s.Windows = append(s.Windows, window.Window{ID: "0x6", App: "org.gnome.Nautilus", Output: "DP-1", PID: 600,
			Box: &window.Box{X: 400, Y: 300, W: 200, H: 200}, Visible: true})
	})
	r.pick("0x1")
	refusedFor(t, r.d.Click(context.Background(), 0.5, 0.5), "covered")
	if r.ptr.dials() != 0 {
		t.Fatal("a covered point reached the pointer")
	}

	// the box hangs off its output: the point must be on screen
	r = newRig(t, 5)
	r.win.edit(func(s *window.Snapshot) { win(s, "0x1").Box = &window.Box{X: 1500, Y: 100, W: 800, H: 600} })
	r.pick("0x1")
	refusedFor(t, r.d.Click(context.Background(), 0.9, 0.5), "region")
	if r.ptr.dials() != 0 {
		t.Fatal("an off-screen point reached the pointer")
	}
}

func TestClickChecksBeforeDiallingAndAfterTheMove(t *testing.T) {
	r := newRig(t, 5)
	r.pick("0x1")
	r.win.change = map[int]func(*window.Snapshot){1: pauses[1].change}
	pausedFor(t, r.d.Click(context.Background(), 0.5, 0.5), discipline.PauseGeometry)
	if r.ptr.dials() != 0 {
		t.Fatal("the pointer was opened before the check passed")
	}

	// the move hands focus away: no press
	r = newRig(t, 5)
	r.pick("0x1")
	r.ptr.onMove = func() { r.win.edit(func(s *window.Snapshot) { focus(s, "0x2") }) }
	pausedFor(t, r.d.Click(context.Background(), 0.5, 0.5), discipline.PauseFocus)
	for _, op := range r.ptr.log() {
		if strings.HasPrefix(op, "button ") {
			t.Fatalf("pressed after focus was lost: %q", r.ptr.log())
		}
	}

	// A person's key during motion stops the click before any press.
	r = newRig(t, 5)
	r.pick("0x1")
	r.ptr.onMove = func() { r.keys.press() }
	pausedFor(t, r.d.Click(context.Background(), 0.5, 0.5), discipline.PauseKey)
	for _, op := range r.ptr.log() {
		if strings.HasPrefix(op, "button ") {
			t.Fatalf("pressed after the person's key: %q", r.ptr.log())
		}
	}
}

func TestClickApproachStaysInsideWindowAtEveryCorner(t *testing.T) {
	for _, point := range [][2]float64{{0, 0}, {0, 0.999}, {0.999, 0}, {0.999, 0.999}} {
		r := newRig(t, 5)
		r.pick("0x1")
		if err := r.d.Click(context.Background(), point[0], point[1]); err != nil {
			t.Fatal(err)
		}
		for _, op := range r.ptr.log() {
			if !strings.HasPrefix(op, "move ") {
				continue
			}
			var x, y, width, height int
			if _, err := fmt.Sscanf(op, "move %d,%d of %dx%d", &x, &y, &width, &height); err != nil {
				t.Fatal(err)
			}
			if x < 100 || x >= 900 || y < 100 || y >= 700 {
				t.Fatalf("point %v moved outside the picked window: %s", point, op)
			}
		}
	}
}

func TestClickRefusesWhenNoSafeApproachFits(t *testing.T) {
	r := newRig(t, 5)
	r.win.edit(func(s *window.Snapshot) {
		win(s, "0x1").Box = &window.Box{X: 100, Y: 100, W: 1, H: 1}
	})
	r.pick("0x1")
	refusedFor(t, r.d.Click(context.Background(), 0.5, 0.5), "region")
	if r.ptr.dials() != 0 {
		t.Fatal("a click without a safe approach reached the pointer")
	}
}

func TestReadBack(t *testing.T) {
	r := newRig(t, 5)
	r.pick("0x1")
	r.ocr.reads = []ocrRead{
		{words: []string{"Seven", "ravens", "fly."}},
		{words: []string{"lorem", "ipsum"}},
	}
	if v, err := r.d.ReadBack(context.Background(), line); v != Yes || err != nil {
		t.Fatalf("all words present: %v, %v", v, err)
	}
	if v, err := r.d.ReadBack(context.Background(), line); v != No || err != nil {
		t.Fatalf("none present: %v, %v", v, err)
	}
	// both pictures were of the target with Oni's strips masked, then zeroed
	if r.shots.shots != 2 || r.shots.clears != 2 || !slices.Equal(r.shots.targets, []window.ID{"0x1", "0x1"}) {
		t.Fatalf("shots %d, clears %d, targets %v", r.shots.shots, r.shots.clears, r.shots.targets)
	}
	for _, m := range r.shots.masks {
		if len(m) != 2 || m[0].H != 130 {
			t.Fatalf("masks %v", m)
		}
	}
	// checks ran before and after each picture
	if r.win.count() != 4 {
		t.Fatalf("%d checks", r.win.count())
	}
}

func TestReadBackWaitsForRenderedTextWithoutTypingAgain(t *testing.T) {
	r := newRig(t, 5)
	r.pick("0x1")
	if _, err := r.d.Type(context.Background(), line); err != nil {
		t.Fatal(err)
	}
	inputs := len(r.kb.runs())
	r.ocr.reads = []ocrRead{
		{words: []string{"Seven", "raven"}},
		{words: []string{"Seven", "ravens", "fly."}},
	}
	if v, err := r.d.ReadBack(context.Background(), line); v != Yes || err != nil {
		t.Fatalf("the target finished rendering within readback: %v, %v", v, err)
	}
	if len(r.kb.runs()) != inputs {
		t.Fatal("readback typed the text again")
	}
}

func TestPauseWhileWaitingForRenderedTextCountsNoVerdict(t *testing.T) {
	r := newRig(t, 5)
	r.pick("0x1")
	r.ocr.reads = []ocrRead{
		{words: []string{"Seven", "raven"}},
		{words: []string{"Seven", "ravens", "fly."}},
	}
	r.win.change = map[int]func(*window.Snapshot){3: pauses[0].change}
	v, err := r.d.ReadBack(context.Background(), line)
	pausedFor(t, err, discipline.PauseTitle)
	if v != Unknown {
		t.Fatalf("paused readback passed: %v", v)
	}
	r.win.edit(func(s *window.Snapshot) { win(s, "0x1").Title = "Notes" })
	r.d.Resume()
	r.ocr.reads = nil
	if v, err := r.d.ReadBack(context.Background(), line); v != Unknown || err != nil {
		t.Fatalf("a paused partial frame counted toward ending the task: %v, %v", v, err)
	}
}

func TestUnknownNeverPassesAndASecondUnknownEnds(t *testing.T) {
	for name, second := range map[string]func(r *rig){
		"reader fails":  func(r *rig) { r.ocr.reads[1] = ocrRead{err: errors.New("ocr: no text reader installed")} },
		"nothing read":  func(r *rig) { r.ocr.reads[1] = ocrRead{} },
		"picture fails": func(r *rig) { r.shots.err = errors.New("That window is behind another one.") },
		"partial":       func(r *rig) { r.ocr.reads[1] = ocrRead{words: []string{"seven"}} },
	} {
		t.Run(name, func(t *testing.T) {
			r := newRig(t, 5)
			r.pick("0x1")
			r.ocr.reads = []ocrRead{{words: []string{"seven", "crows"}}, {}}
			second(r)
			if v, err := r.d.ReadBack(context.Background(), line); v != Unknown || err != nil {
				t.Fatalf("first: %v, %v", v, err)
			}
			v, err := r.d.ReadBack(context.Background(), line)
			if v != Unknown || !errors.Is(err, ErrEnded) {
				t.Fatalf("second: %v, %v", v, err)
			}
			if r.keys.stops != 1 {
				t.Fatal("the key reader still runs after the task ended")
			}
			if _, err := r.d.Type(context.Background(), "more"); !errors.Is(err, ErrEnded) || len(r.kb.runs()) != 0 {
				t.Fatalf("typed after the end: %v", err)
			}
		})
	}
	// text with no readable word cannot be read back
	r := newRig(t, 5)
	r.pick("0x1")
	if v, err := r.d.ReadBack(context.Background(), "-- ..."); v != Unknown || err != nil || r.shots.shots != 0 {
		t.Fatalf("%v, %v", v, err)
	}
}

func TestAPauseAroundThePictureCountsNoVerdict(t *testing.T) {
	r := newRig(t, 5)
	r.pick("0x1")
	r.ocr.reads = []ocrRead{{}, {}}
	r.win.change = map[int]func(*window.Snapshot){2: pauses[0].change}
	v, err := r.d.ReadBack(context.Background(), line)
	pausedFor(t, err, discipline.PauseTitle)
	if v != Unknown || r.shots.clears != 1 || r.ocr.n != 0 {
		t.Fatalf("%v, clears %d, reads %d", v, r.shots.clears, r.ocr.n)
	}
	r.win.edit(func(s *window.Snapshot) { win(s, "0x1").Title = "Notes" })
	r.d.Resume()
	if v, err := r.d.ReadBack(context.Background(), line); v != Unknown || err != nil {
		t.Fatalf("one Unknown after a pause ended the task: %v, %v", v, err)
	}
}

func TestLookIsMaskedAndChecked(t *testing.T) {
	r := newRig(t, 5)
	r.pick("0x1")
	r.ocr.reads = []ocrRead{{words: []string{"Notes"}}}
	f, err := r.d.Look(context.Background())
	if err != nil || !f.Read || len(f.Words) != 1 || string(f.PNG) != "png" {
		t.Fatalf("%+v, %v", f, err)
	}
	f.Clear()
	if r.shots.clears != 1 || len(r.shots.masks[0]) != 2 || r.win.count() != 2 {
		t.Fatalf("clears %d, masks %v, checks %d", r.shots.clears, r.shots.masks, r.win.count())
	}
}

// Typed text and read-back words reach no error and no log line.
func TestTypedTextNeverReachesErrorsOrLogs(t *testing.T) {
	const secretish = "violet harbor lanterns"
	words := append(strings.Fields(secretish), input.Chunks(secretish, 8)...)
	check := func(t *testing.T, r *rig, what string, err error, res Typed) {
		t.Helper()
		out := r.logs.String()
		if err != nil {
			out += err.Error()
		}
		out += fmt.Sprintf("%v %+v %#v %s", res, res, res, res)
		for _, w := range words {
			if strings.Contains(out, w) {
				t.Fatalf("%s: %q reached %q", what, w, out)
			}
		}
	}
	// a tool failing with the text in its stderr
	r := newRig(t, 8)
	r.pick("0x1")
	r.kb.fail = map[int]bool{2: true}
	res, err := r.d.Type(context.Background(), secretish)
	if !errors.Is(err, ErrKeyboard) {
		t.Fatalf("tool failure: %v", err)
	}
	check(t, r, "tool failure", err, res)
	// a pause mid-text
	r = newRig(t, 8)
	r.pick("0x1")
	r.win.change = map[int]func(*window.Snapshot){3: pauses[0].change}
	res, err = r.d.Type(context.Background(), secretish)
	check(t, r, "pause", err, res)
	// a refusal
	r = newRig(t, 8)
	r.pick("0x1")
	res, err = r.d.Type(context.Background(), secretish+"\n")
	check(t, r, "refusal", err, res)
	// a full typing and its read-back
	r = newRig(t, 8)
	r.pick("0x1")
	r.ocr.reads = []ocrRead{{words: words}}
	res, err = r.d.Type(context.Background(), secretish)
	if err != nil || res.Rest != "" {
		t.Fatalf("%v, %v", res, err)
	}
	if v, err := r.d.ReadBack(context.Background(), secretish); v != Yes || err != nil {
		t.Fatalf("read-back %v, %v", v, err)
	}
	check(t, r, "typing and read-back", nil, res)
	if !strings.Contains(r.logs.String(), "landed=") || !strings.Contains(r.logs.String(), "verdict=yes") {
		t.Fatalf("the log holds no counts: %s", r.logs.String())
	}
}

// Nothing but the keyboard tool's allowlisted argv is ever run, and the
// package has no way to start anything else.
func TestNoExecutableIsLaunched(t *testing.T) {
	r := newRig(t, 5)
	r.pick("0x1")
	r.ocr.reads = []ocrRead{{words: strings.Fields(line)}}
	r.d.Type(context.Background(), line)
	r.d.Press(context.Background(), "ctrl+z")
	r.d.Click(context.Background(), 0.5, 0.5)
	r.d.ReadBack(context.Background(), line)
	r.d.Type(context.Background(), "/usr/bin/evil --now")
	for _, c := range r.kb.runs() {
		if c.name != input.Tool || !input.Allowed(c.name, c.args...) {
			t.Fatalf("ran %s %q", c.name, c.args)
		}
	}
	files, _ := filepath.Glob("*.go")
	scanned := 0
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		scanned++
		ast, err := parser.ParseFile(token.NewFileSet(), f, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatal(err)
		}
		for _, imp := range ast.Imports {
			p, _ := strconv.Unquote(imp.Path.Value)
			if p == "os" || p == "os/exec" || p == "syscall" || strings.HasSuffix(p, "/hands/nest") || strings.Contains(p, "/reflex") {
				t.Errorf("%s imports %s", f, p)
			}
		}
	}
	if scanned == 0 {
		t.Fatal("no source scanned")
	}
}

type fakeNest struct{ dir, display string }

func (n fakeNest) Dir() string     { return n.dir }
func (n fakeNest) Display() string { return n.display }

// On the nested desktop, input reaches the nest's display and nothing of
// the live session, whatever the daemon's own environment holds.
func TestTheNestedDesktopGetsOnlyTheNestDisplay(t *testing.T) {
	for k, v := range map[string]string{
		"WAYLAND_DISPLAY": "wayland-live", "XDG_RUNTIME_DIR": "/run/user/1000", window.EnvNiriSocket: "/run/user/1000/niri.sock",
		window.EnvHyprlandInstance: "live", "DBUS_SESSION_BUS_ADDRESS": "unix:path=/run/user/1000/bus", "DISPLAY": ":0",
	} {
		t.Setenv(k, v)
	}
	nested := NestDisplay(fakeNest{dir: "/dev/shm/hk-nest-1", display: "wayland-1"})
	kb, ptr := &fakeKeyboard{}, &fakePointer{}
	win := &fakeWindows{snap: desk()}
	keys := &fakeKeys{}
	d, err := New(Config{Display: nested, Windows: win, Pictures: &fakeShooter{}, OCR: &fakeOCR{}, Keys: keys.start,
		Self: []string{selfApp}, Limits: limits(5), Run: kb.runner, Dial: ptr.dial})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.Select(context.Background(), "0x1"); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Type(context.Background(), "hello"); err != nil {
		t.Fatal(err)
	}
	if err := d.Click(context.Background(), 0.5, 0.5); err != nil {
		t.Fatal(err)
	}
	want, _ := input.Env("/dev/shm/hk-nest-1", "wayland-1")
	for _, c := range kb.runs() {
		if !slices.Equal(c.env, want) {
			t.Fatalf("env %q, want %q", c.env, want)
		}
		for _, e := range c.env {
			if strings.HasPrefix(e, "PATH=") {
				continue
			}
			if strings.Contains(e, "live") || strings.Contains(e, "/run/user/1000") || strings.HasPrefix(e, "DBUS") || strings.HasPrefix(e, "DISPLAY=") {
				t.Fatalf("live session in %q", c.env)
			}
		}
	}
	if !slices.Equal(ptr.sockets, []string{"/dev/shm/hk-nest-1/wayland-1"}) {
		t.Fatalf("pointer dialled %q", ptr.sockets)
	}
}

func TestLiveDisplay(t *testing.T) {
	env := func(m map[string]string) func(string) string { return func(k string) string { return m[k] } }
	for _, tc := range []struct {
		env  map[string]string
		want Display
	}{
		{map[string]string{"XDG_RUNTIME_DIR": "/run/user/1000", "WAYLAND_DISPLAY": "wayland-1"}, Display{"/run/user/1000", "wayland-1"}},
		{map[string]string{"XDG_RUNTIME_DIR": "/run/user/1000"}, Display{"/run/user/1000", "wayland-0"}},
		{map[string]string{"WAYLAND_DISPLAY": "/run/user/1000/wayland-2"}, Display{"/run/user/1000", "wayland-2"}},
	} {
		d, err := LiveDisplay(env(tc.env))
		if err != nil || d != tc.want {
			t.Fatalf("%v: %v, %v", tc.env, d, err)
		}
	}
	for _, bad := range []map[string]string{
		{"WAYLAND_DISPLAY": "wayland-1"},
		{"XDG_RUNTIME_DIR": "run/user", "WAYLAND_DISPLAY": "wayland-1"},
		{"XDG_RUNTIME_DIR": "/run/user/1000", "WAYLAND_DISPLAY": "../wayland-1"},
	} {
		if _, err := LiveDisplay(env(bad)); !errors.Is(err, input.ErrEnv) {
			t.Fatalf("%v: %v", bad, err)
		}
	}
}

func TestNewRefusesAnIncompleteConfig(t *testing.T) {
	good := func() Config {
		return Config{Display: Display{"/run/user/1000", "wayland-1"}, Windows: &fakeWindows{}, Pictures: &fakeShooter{},
			OCR: &fakeOCR{}, Keys: (&fakeKeys{}).start, Limits: limits(5)}
	}
	if _, err := New(good()); err != nil {
		t.Fatal(err)
	}
	for name, spoil := range map[string]func(*Config){
		"no windows":      func(c *Config) { c.Windows = nil },
		"no pictures":     func(c *Config) { c.Pictures = nil },
		"no reader":       func(c *Config) { c.OCR = nil },
		"no keys":         func(c *Config) { c.Keys = nil },
		"chunk too big":   func(c *Config) { c.Limits.ChunkRunes = input.MaxChunk + 1 },
		"chunk zero":      func(c *Config) { c.Limits.ChunkRunes = 0 },
		"max below chunk": func(c *Config) { c.Limits.MaxTypeRunes = 2 },
		"no verify time":  func(c *Config) { c.Limits.Verify = 0 },
		"bad display":     func(c *Config) { c.Display.RuntimeDir = "relative" },
	} {
		c := good()
		spoil(&c)
		if _, err := New(c); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}
