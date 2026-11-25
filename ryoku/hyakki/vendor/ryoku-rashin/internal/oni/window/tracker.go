package window

import "ryoku-rashin/internal/oni/spec"

// Change kinds Tracker.Update reports.
const (
	ChangeMoved      = "moved"
	ChangeResized    = "resized"
	ChangeFocus      = "focus"
	ChangeOutput     = "output"
	ChangeHidden     = "hidden"
	ChangeFullscreen = "fullscreen"
	ChangeClosed     = "closed"
)

// Change is one difference between two snapshots. For focus, Window is the
// newly focused window (zero when nothing has focus); for every other kind it
// is the tracked window as it is now (as it was last, for closed). A moved
// change whose Window.Box is nil means the window can no longer be placed.
type Change struct {
	Kind   string
	Window Window
}

// Tracker follows one window across snapshots. The zero value tracks
// nothing and is ready to use.
type Tracker struct {
	id        ID
	last      Window
	have      bool
	focused   ID
	seenFocus bool
}

// Track follows id from the next Update on; an empty id stops tracking.
func (t *Tracker) Track(id ID) {
	t.id, t.have, t.last = id, false, Window{}
}

// Update compares s with the previous snapshot. The first snapshot after
// Track is the baseline and reports nothing for the tracked window unless it
// is already gone, which reports closed. Focus changes are reported for any
// window, so a perch that follows focus can see where focus went.
func (t *Tracker) Update(s Snapshot) []Change {
	var out []Change
	if t.id != "" {
		cur, ok := find(s, t.id)
		switch {
		case !ok:
			out = append(out, Change{Kind: ChangeClosed, Window: t.last})
			if !t.have {
				out[len(out)-1].Window = Window{ID: t.id}
			}
			t.id, t.have, t.last = "", false, Window{}
		case !t.have:
			t.last, t.have = cur, true
		default:
			out = append(out, diff(t.last, cur)...)
			t.last = cur
		}
	}
	focused := Window{}
	for _, w := range s.Windows {
		if w.Focused {
			focused = w
			break
		}
	}
	if t.seenFocus && focused.ID != t.focused {
		out = append(out, Change{Kind: ChangeFocus, Window: focused})
	}
	t.focused, t.seenFocus = focused.ID, true
	return out
}

func diff(was, now Window) []Change {
	var out []Change
	add := func(kind string) { out = append(out, Change{Kind: kind, Window: now}) }
	if was.Output != now.Output {
		add(ChangeOutput)
	}
	shown := func(w Window) bool { return w.Visible && !w.Minimized }
	if shown(was) && !shown(now) {
		add(ChangeHidden)
	}
	if was.Fullscreen != now.Fullscreen {
		add(ChangeFullscreen)
	}
	switch {
	case (was.Box == nil) != (now.Box == nil):
		add(ChangeMoved)
	case was.Box != nil:
		if was.Box.X != now.Box.X || was.Box.Y != now.Box.Y {
			add(ChangeMoved)
		}
		if was.Box.W != now.Box.W || was.Box.H != now.Box.H {
			add(ChangeResized)
		}
	}
	return out
}

func find(s Snapshot, id ID) (Window, bool) {
	for _, w := range s.Windows {
		if w.ID == id {
			return w, true
		}
	}
	return Window{}, false
}

// Global is w's box in the global logical layout, false when w cannot be
// placed or its output is gone.
func Global(w Window, outputs []Output) (Box, bool) {
	if w.Box == nil {
		return Box{}, false
	}
	o, ok := outputByName(outputs, w.Output)
	if !ok {
		return Box{}, false
	}
	return Box{X: w.Box.X + float64(o.X), Y: w.Box.Y + float64(o.Y), W: w.Box.W, H: w.Box.H}, true
}

// HitTest finds the shown, placed window under a global logical point:
// floating windows over tiled ones, then the most recently focused.
func (t *Tracker) HitTest(global spec.Point, s Snapshot) (Window, bool) {
	var best Window
	found := false
	for _, w := range s.Windows {
		if !w.Visible || w.Minimized {
			continue
		}
		b, ok := Global(w, s.Outputs)
		if !ok || global.X < b.X || global.Y < b.Y || global.X >= b.X+b.W || global.Y >= b.Y+b.H {
			continue
		}
		if !found || above(w, best) {
			best, found = w, true
		}
	}
	return best, found
}

func above(a, b Window) bool {
	if a.Floating != b.Floating {
		return a.Floating
	}
	return a.FocusOrder < b.FocusOrder
}
