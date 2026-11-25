package discipline

import (
	"testing"

	"ryoku-rashin/internal/oni/window"
)

// nested is the nested desktop Hyakki started for a task, with a file
// manager focused in it.
func nested() window.Snapshot {
	return window.Snapshot{
		Outputs: []window.Output{{Name: "headless-1", W: 1280, H: 800, Scale: 1}},
		Windows: []window.Window{{ID: "3", App: "org.gnome.Nautilus", PID: 10, Output: "headless-1", Focused: true, Visible: true}},
	}
}

func TestCaptureDesktopNeedsAnOutput(t *testing.T) {
	if _, why := CaptureDesktop(window.Snapshot{}); why != ReasonMissing {
		t.Fatalf("CaptureDesktop with no output = %q, want %q", why, ReasonMissing)
	}
	if _, why := CaptureDesktop(nested()); why != Accepted {
		t.Fatalf("CaptureDesktop = %q", why)
	}
}

func TestDesktopAwayRule(t *testing.T) {
	cases := []struct {
		name string
		edit func(sc *Scene)
		want Pause
	}{
		{name: "nothing changed", want: OK},
		{name: "the person works in another live window", edit: func(sc *Scene) {
			at(sc.Live, "0xa").Focused = false
			at(sc.Live, "0xb").Focused = true
		}, want: OK},
		{name: "nothing has focus in the nested desktop", edit: func(sc *Scene) { sc.Nested.Windows[0].Focused = false }, want: OK},
		{name: "the presenter is gone", edit: func(sc *Scene) { sc.Presenter = false }, want: PausePresenter},
		{name: "the presenter is gone and an output changed", edit: func(sc *Scene) {
			sc.Presenter = false
			sc.Nested.Outputs[0].W = 1920
		}, want: PausePresenter},
		{name: "the live desktop cannot be read", edit: func(sc *Scene) { sc.Live = nil }, want: PauseWindow},
		{name: "the person focused Hyakki's own window", edit: func(sc *Scene) {
			at(sc.Live, "0xa").Focused = false
			w := at(sc.Live, "0xb")
			w.Focused, w.App = true, hyakki
		}, want: PauseSelf},
		{name: "the nested desktop cannot be read", edit: func(sc *Scene) { sc.Nested = nil }, want: PauseWindow},
		{name: "a nested output resized", edit: func(sc *Scene) { sc.Nested.Outputs[0].H = 1080 }, want: PauseOutput},
		{name: "a nested output rescaled", edit: func(sc *Scene) { sc.Nested.Outputs[0].Scale = 2 }, want: PauseOutput},
		{name: "a nested output was added", edit: func(sc *Scene) {
			sc.Nested.Outputs = append(sc.Nested.Outputs, window.Output{Name: "headless-2", W: 1280, H: 800, Scale: 1})
		}, want: PauseOutput},
		{name: "the nested output is gone", edit: func(sc *Scene) { sc.Nested.Outputs = nil }, want: PauseOutput},
		{name: "two nested windows report focus", edit: func(sc *Scene) {
			sc.Nested.Windows = append(sc.Nested.Windows, window.Window{ID: "4", App: "foot", Output: "headless-1", Focused: true, Visible: true})
		}, want: PauseWindow},
		{name: "a password manager has focus in the nested desktop", edit: func(sc *Scene) {
			sc.Nested.Windows[0].App = "org.keepassxc.KeePassXC"
		}, want: PausePrivate},
		{name: "a polkit prompt has focus in the nested desktop", edit: func(sc *Scene) {
			sc.Nested.Windows[0].App = "lxqt-policykit-agent"
		}, want: PausePrivate},
		{name: "Hyakki's own window has focus in the nested desktop", edit: func(sc *Scene) {
			sc.Nested.Windows[0].App = hyakki
		}, want: PauseSelf},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			d, _ := CaptureDesktop(nested())
			live, inner := scene(), nested()
			sc := Scene{Presenter: true, Live: &live, Nested: &inner}
			if c.edit != nil {
				c.edit(&sc)
			}
			if got := d.Check(sc, self); got != c.want {
				t.Fatalf("Check = %q, want %q", got, c.want)
			}
		})
	}
}

func TestDesktopOutputsCompareInAnyOrder(t *testing.T) {
	two := nested()
	two.Outputs = append(two.Outputs, window.Output{Name: "headless-2", X: 1280, W: 1280, H: 800, Scale: 1})
	d, _ := CaptureDesktop(two)
	live, inner := scene(), two
	inner.Outputs = []window.Output{two.Outputs[1], two.Outputs[0]}
	if got := d.Check(Scene{Presenter: true, Live: &live, Nested: &inner}, self); got != OK {
		t.Fatalf("the same outputs listed in another order check as %q", got)
	}
	// the capture keeps its own copy
	two.Outputs[0].W = 1
	inner.Outputs = []window.Output{{Name: "headless-1", W: 1280, H: 800, Scale: 1}, {Name: "headless-2", X: 1280, W: 1280, H: 800, Scale: 1}}
	if got := d.Check(Scene{Presenter: true, Live: &live, Nested: &inner}, self); got != OK {
		t.Fatalf("changing the captured snapshot changed the desktop: %q", got)
	}
}

func TestTheZeroDesktopNeverPasses(t *testing.T) {
	live, inner := scene(), nested()
	if got := (Desktop{}).Check(Scene{Presenter: true, Live: &live, Nested: &inner}, self); got != PauseOutput {
		t.Fatalf("the zero desktop checks as %q", got)
	}
}
