package discipline

import (
	"math"
	"testing"

	"ryoku-rashin/internal/oni/window"
)

const hyakki = "dev.ryoku.Hyakki"

var self = []string{hyakki}

// scene is a live desktop with the window the person picks ("0xa", focused
// and floating) and an ordinary tiled terminal behind it.
func scene() window.Snapshot {
	return window.Snapshot{
		Outputs: []window.Output{{Name: "DP-1", W: 2560, H: 1440, Scale: 1}, {Name: "HDMI-A-1", X: 2560, W: 1920, H: 1080, Scale: 1}},
		Windows: []window.Window{
			{
				ID: "0xa", App: "org.gnome.TextEditor", PID: 100, Output: "DP-1", Title: "notes",
				Box: &window.Box{X: 100, Y: 100, W: 800, H: 600}, Focused: true, Floating: true, Visible: true,
			},
			{
				ID: "0xb", App: "kitty", PID: 200, Output: "DP-1", Title: "shell",
				Box: &window.Box{X: 0, Y: 0, W: 1280, H: 1440}, Visible: true, FocusOrder: 1,
			},
		},
	}
}

func at(s *window.Snapshot, id window.ID) *window.Window {
	for k := range s.Windows {
		if s.Windows[k].ID == id {
			return &s.Windows[k]
		}
	}
	panic("no window " + string(id))
}

func picked(t *testing.T) Baseline {
	t.Helper()
	b, why := Capture(scene(), "0xa", self)
	if why != Accepted {
		t.Fatalf("the ordinary pick was refused: %q", why)
	}
	return b
}

func TestCaptureRefusesWhatHandsMustNotDrive(t *testing.T) {
	cases := []struct {
		name string
		id   window.ID
		edit func(*window.Snapshot)
		want Reason
	}{
		{"an ordinary window", "0xa", nil, Accepted},
		{"a window without focus: the person picks from Hyakki's own window", "0xb", nil, Accepted},
		{"a tiled window nobody can place", "0xb", func(s *window.Snapshot) { at(s, "0xb").Box = nil }, Accepted},
		{"no id", "", nil, ReasonMissing},
		{"an id no window has", "0xz", nil, ReasonMissing},
		{"Hyakki's own window", "0xa", func(s *window.Snapshot) { at(s, "0xa").App = hyakki }, ReasonSelf},
		{"Hyakki's own window in another case", "0xa", func(s *window.Snapshot) { at(s, "0xa").App = "DEV.RYOKU.HYAKKI" }, ReasonSelf},
		{"a password manager", "0xa", func(s *window.Snapshot) { at(s, "0xa").App = "org.keepassxc.KeePassXC" }, ReasonPrivate},
		{"a polkit agent", "0xa", func(s *window.Snapshot) { at(s, "0xa").App = "polkit-gnome-authentication-agent-1" }, ReasonPrivate},
		{"a lock screen", "0xa", func(s *window.Snapshot) { at(s, "0xa").App = "hyprlock" }, ReasonPrivate},
		{"a credential prompt known only by its title", "0xa", func(s *window.Snapshot) {
			w := at(s, "0xa")
			w.App, w.Title = "", "Authentication Required"
		}, ReasonPrivate},
		{"a minimized window", "0xa", func(s *window.Snapshot) { at(s, "0xa").Minimized = true }, ReasonHidden},
		{"a window on a hidden workspace", "0xa", func(s *window.Snapshot) { at(s, "0xa").Visible = false }, ReasonHidden},
		{"no process id and an app id another window shares", "0xa", func(s *window.Snapshot) {
			at(s, "0xa").PID = 0
			at(s, "0xb").App = "org.gnome.TextEditor"
		}, ReasonUnpinned},
		{"no process id and no app id", "0xa", func(s *window.Snapshot) {
			w := at(s, "0xa")
			w.PID, w.App = 0, ""
		}, ReasonUnpinned},
		{"no process id but an app id of its own", "0xa", func(s *window.Snapshot) { at(s, "0xa").PID = 0 }, Accepted},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := scene()
			if c.edit != nil {
				c.edit(&s)
			}
			b, why := Capture(s, c.id, self)
			if why != c.want {
				t.Fatalf("Capture = %q, want %q", why, c.want)
			}
			if why != Accepted && b != (Baseline{}) {
				t.Errorf("a refused pick still recorded %+v", b)
			}
			if why == Accepted && b.ID != c.id {
				t.Errorf("recorded %q, want %q", b.ID, c.id)
			}
		})
	}
}

func TestCaptureHoldsItsOwnCopyOfTheBox(t *testing.T) {
	s := scene()
	b, _ := Capture(s, "0xa", self)
	at(&s, "0xa").Box.X = 900
	if got := b.Check(s, self, 2); got != PauseGeometry {
		t.Fatalf("a box moved after the pick checks as %q, want %q", got, PauseGeometry)
	}
}

func TestCheckPausesOnEveryChange(t *testing.T) {
	cases := []struct {
		name string
		tol  int
		edit func(*window.Snapshot)
		want Pause
	}{
		{"nothing changed", 2, nil, OK},
		{"the window behind changed its title", 2, func(s *window.Snapshot) { at(s, "0xb").Title = "vim" }, OK},

		// stricter than dictation: a title change is never allowed
		{"the title changed", 2, func(s *window.Snapshot) { at(s, "0xa").Title = "notes *" }, PauseTitle},

		{"moved one pixel, inside the tolerance", 2, func(s *window.Snapshot) { at(s, "0xa").Box.X++ }, OK},
		{"moved exactly the tolerance", 2, func(s *window.Snapshot) { at(s, "0xa").Box.Y -= 2 }, OK},
		{"moved past the tolerance", 2, func(s *window.Snapshot) { at(s, "0xa").Box.X += 3 }, PauseGeometry},
		{"grew past the tolerance", 2, func(s *window.Snapshot) { at(s, "0xa").Box.W += 3 }, PauseGeometry},
		{"shrank past the tolerance", 2, func(s *window.Snapshot) { at(s, "0xa").Box.H -= 2.5 }, PauseGeometry},
		{"moved one pixel with no tolerance", 0, func(s *window.Snapshot) { at(s, "0xa").Box.X++ }, PauseGeometry},
		{"a negative tolerance counts as none", -5, func(s *window.Snapshot) { at(s, "0xa").Box.X++ }, PauseGeometry},
		{"a box that reads as NaN", 2, func(s *window.Snapshot) { at(s, "0xa").Box.X = math.NaN() }, PauseGeometry},
		{"the box can no longer be placed", 2, func(s *window.Snapshot) { at(s, "0xa").Box = nil }, PauseGeometry},
		{"floating became tiled", 2, func(s *window.Snapshot) { at(s, "0xa").Floating = false }, PauseGeometry},
		{"went fullscreen", 2, func(s *window.Snapshot) { at(s, "0xa").Fullscreen = true }, PauseGeometry},

		{"closed", 2, func(s *window.Snapshot) { s.Windows = s.Windows[1:] }, PauseClosed},
		{"its id reused by another app", 2, func(s *window.Snapshot) { at(s, "0xa").App = "org.mozilla.firefox" }, PauseClosed},
		{"its id reused by another process of the same app", 2, func(s *window.Snapshot) { at(s, "0xa").PID = 101 }, PauseClosed},
		{"its process id no longer read", 2, func(s *window.Snapshot) { at(s, "0xa").PID = 0 }, PauseClosed},

		{"moved to another output", 2, func(s *window.Snapshot) { at(s, "0xa").Output = "HDMI-A-1" }, PauseOutput},
		{"minimized", 2, func(s *window.Snapshot) { at(s, "0xa").Minimized = true }, PauseHidden},
		{"its workspace was switched away", 2, func(s *window.Snapshot) { at(s, "0xa").Visible = false }, PauseHidden},

		{"another window took focus", 2, func(s *window.Snapshot) {
			at(s, "0xa").Focused = false
			at(s, "0xb").Focused = true
		}, PauseFocus},
		{"no window has focus, as under a lock screen", 2, func(s *window.Snapshot) { at(s, "0xa").Focused = false }, PauseFocus},
		{"a second window also reports focus", 2, func(s *window.Snapshot) { at(s, "0xb").Focused = true }, PauseWindow},
		{"Hyakki's own window took focus", 2, func(s *window.Snapshot) {
			at(s, "0xa").Focused = false
			w := at(s, "0xb")
			w.Focused, w.App = true, hyakki
		}, PauseSelf},
		{"a polkit prompt took focus", 2, func(s *window.Snapshot) {
			at(s, "0xa").Focused = false
			s.Windows = append(s.Windows, window.Window{
				ID: "0xc", App: "org.kde.polkit-kde-authentication-agent-1", PID: 300, Output: "DP-1",
				Focused: true, Floating: true, Visible: true,
			})
		}, PausePrivate},

		{"a floating window over it", 2, func(s *window.Snapshot) { s.Windows = append(s.Windows, floating("0xc", "mpv", 500, 400)) }, PauseCovered},
		{"a floating window whose edge only touches it", 2, func(s *window.Snapshot) { s.Windows = append(s.Windows, floating("0xc", "mpv", 900, 100)) }, OK},
		{"a floating window at the same place on another output", 2, func(s *window.Snapshot) {
			w := floating("0xc", "mpv", 500, 400)
			w.Output = "HDMI-A-1"
			s.Windows = append(s.Windows, w)
		}, OK},
		{"a minimized floating window over it", 2, func(s *window.Snapshot) {
			w := floating("0xc", "mpv", 500, 400)
			w.Minimized = true
			s.Windows = append(s.Windows, w)
		}, OK},
		{"a floating window nobody can place", 2, func(s *window.Snapshot) {
			w := floating("0xc", "mpv", 0, 0)
			w.Box = nil
			s.Windows = append(s.Windows, w)
		}, PauseCovered},
		{"another window fullscreen on its output", 2, func(s *window.Snapshot) { at(s, "0xb").Fullscreen = true }, PauseCovered},
		{"Hyakki's own window over it", 2, func(s *window.Snapshot) { s.Windows = append(s.Windows, floating("0xc", hyakki, 500, 400)) }, PauseSelf},
		{"a password manager over it", 2, func(s *window.Snapshot) {
			s.Windows = append(s.Windows, floating("0xc", "com.bitwarden.desktop", 500, 400))
		}, PausePrivate},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			b := picked(t)
			s := scene()
			if c.edit != nil {
				c.edit(&s)
			}
			if got := b.Check(s, self, c.tol); got != c.want {
				t.Fatalf("Check = %q, want %q", got, c.want)
			}
		})
	}
}

func floating(id window.ID, app string, x, y float64) window.Window {
	return window.Window{
		ID: id, App: app, PID: 300, Output: "DP-1", Box: &window.Box{X: x, Y: y, W: 400, H: 300},
		Floating: true, Visible: true, FocusOrder: 2,
	}
}

func TestATiledTargetNobodyCanPlace(t *testing.T) {
	s := scene()
	w := at(&s, "0xa")
	w.Box, w.Floating = nil, false
	b, why := Capture(s, "0xa", self)
	if why != Accepted {
		t.Fatalf("Capture = %q", why)
	}
	if got := b.Check(s, self, 2); got != OK {
		t.Fatalf("an unchanged unplaced target checks as %q", got)
	}
	// where the target sits is unknown, so any floating window may cover it
	s.Windows = append(s.Windows, floating("0xc", "mpv", 2000, 1200))
	if got := b.Check(s, self, 2); got != PauseCovered {
		t.Fatalf("a floating window beside an unplaced target checks as %q, want %q", got, PauseCovered)
	}
}

func TestATargetPinnedByItsAppIDAlone(t *testing.T) {
	s := scene()
	at(&s, "0xa").PID = 0
	b, why := Capture(s, "0xa", self)
	if why != Accepted {
		t.Fatalf("Capture = %q", why)
	}
	if got := b.Check(s, self, 2); got != OK {
		t.Fatalf("unchanged = %q", got)
	}
	s.Windows = append(s.Windows, window.Window{ID: "0xc", App: "org.gnome.TextEditor", Output: "DP-1", Visible: true})
	if got := b.Check(s, self, 2); got != PauseWindow {
		t.Fatalf("a second window of the same app checks as %q, want %q", got, PauseWindow)
	}
}

func TestTheZeroBaselineNeverPasses(t *testing.T) {
	if got := (Baseline{}).Check(scene(), self, 2); got != PauseClosed {
		t.Fatalf("the zero baseline checks as %q", got)
	}
}
