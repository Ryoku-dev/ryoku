package target

import (
	"context"
	"errors"
	"testing"

	"ryoku-rashin/internal/oni/window"
)

// Every window.Provider is a Source, so the daemon passes its provider as is.
var _ Source = window.Provider(nil)

type fakeSource struct {
	snaps []window.Snapshot
	err   error
	reads int
}

func (f *fakeSource) Snapshot(context.Context) (window.Snapshot, error) {
	f.reads++
	if f.err != nil {
		return window.Snapshot{}, f.err
	}
	i := min(f.reads-1, len(f.snaps)-1)
	return f.snaps[i], nil
}

func editor() window.Window {
	return window.Window{ID: "0x5a1e", App: "org.gnome.TextEditor", Title: "notes.txt", Output: "eDP-1",
		Focused: true, Visible: true, PID: 4120}
}

func term() window.Window {
	return window.Window{ID: "0x77aa", App: "foot", Title: "foot", Output: "eDP-1", Visible: true, PID: 5003, FocusOrder: 1}
}

func snap(ws ...window.Window) window.Snapshot {
	return window.Snapshot{Outputs: []window.Output{{Name: "eDP-1", W: 1920, H: 1200}, {Name: "DP-1", X: 1920, W: 2560, H: 1440}}, Windows: ws}
}

func TestCaptureRecordsTheFocusedWindow(t *testing.T) {
	src := &fakeSource{snaps: []window.Snapshot{snap(term(), editor())}}
	got, err := Capture(context.Background(), src)
	if err != nil {
		t.Fatal(err)
	}
	want := Target{Window: "0x5a1e", App: "org.gnome.TextEditor", Title: "notes.txt", Output: "eDP-1", PID: 4120, Seq: got.Seq}
	if got != want || got.Seq == 0 {
		t.Fatalf("captured %+v, want %+v", got, want)
	}
	again, err := Capture(context.Background(), src)
	if err != nil {
		t.Fatal(err)
	}
	if again.Seq <= got.Seq {
		t.Fatalf("second capture seq %d not after %d", again.Seq, got.Seq)
	}
	if err := got.Verify(context.Background(), src); err != nil {
		t.Fatalf("unchanged window refused: %v", err)
	}
}

func TestCaptureRefusesWithoutOneShownFocusedWindow(t *testing.T) {
	hidden := editor()
	hidden.Minimized = true
	second := term()
	second.Focused = true
	noID := editor()
	noID.ID = ""
	boom := errors.New("compositor gone")
	cases := []struct {
		name string
		src  Source
		want Reason
	}{
		{"no source", nil, ReasonUnavailable},
		{"read fails", &fakeSource{err: boom}, ReasonUnavailable},
		{"nothing focused", &fakeSource{snaps: []window.Snapshot{snap(term())}}, ReasonUnavailable},
		{"two focused", &fakeSource{snaps: []window.Snapshot{snap(editor(), second)}}, ReasonUnavailable},
		{"focused without id", &fakeSource{snaps: []window.Snapshot{snap(noID)}}, ReasonUnavailable},
		{"focused but minimized", &fakeSource{snaps: []window.Snapshot{snap(hidden)}}, ReasonHidden},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := Capture(context.Background(), c.src)
			if ReasonOf(err) != c.want {
				t.Fatalf("reason %q (%v), want %q", ReasonOf(err), err, c.want)
			}
			if got != (Target{}) {
				t.Fatalf("refused capture returned %+v", got)
			}
		})
	}
	_, err := Capture(context.Background(), &fakeSource{err: boom})
	if !errors.Is(err, boom) {
		t.Fatalf("read failure not wrapped: %v", err)
	}
}

func TestVerifyRefusesEveryChangedTarget(t *testing.T) {
	start := snap(term(), editor())
	tg, err := Capture(context.Background(), &fakeSource{snaps: []window.Snapshot{start}})
	if err != nil {
		t.Fatal(err)
	}
	with := func(f func(e, o *window.Window)) window.Snapshot {
		e, o := editor(), term()
		f(&e, &o)
		return snap(o, e)
	}
	cases := []struct {
		name string
		now  window.Snapshot
		err  error
		want Reason
	}{
		{"unchanged", start, nil, ""},
		{"retitled", with(func(e, _ *window.Window) { e.Title = "notes.txt — modified" }), nil, ""},
		{"closed", snap(term()), nil, ReasonClosed},
		{"id reused by another app", with(func(e, _ *window.Window) { e.App = "firefox"; e.PID = 4120 }), nil, ReasonClosed},
		{"id reused by another process", with(func(e, _ *window.Window) { e.PID = 9999 }), nil, ReasonClosed},
		{"focus moved", with(func(e, o *window.Window) { e.Focused, o.Focused = false, true }), nil, ReasonUnfocused},
		{"focus nowhere", with(func(e, _ *window.Window) { e.Focused = false }), nil, ReasonUnfocused},
		{"moved output", with(func(e, _ *window.Window) { e.Output = "DP-1" }), nil, ReasonMovedOutput},
		{"minimized", with(func(e, _ *window.Window) { e.Minimized = true }), nil, ReasonHidden},
		{"workspace hidden", with(func(e, _ *window.Window) { e.Visible = false }), nil, ReasonHidden},
		{"two focused", with(func(_, o *window.Window) { o.Focused = true }), nil, ReasonUnavailable},
		{"read fails", window.Snapshot{}, errors.New("socket closed"), ReasonUnavailable},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			src := &fakeSource{snaps: []window.Snapshot{c.now}, err: c.err}
			err := tg.Verify(context.Background(), src)
			if ReasonOf(err) != c.want {
				t.Fatalf("reason %q (%v), want %q", ReasonOf(err), err, c.want)
			}
			if src.reads != 1 {
				t.Fatalf("verify read the windows %d times, want once", src.reads)
			}
			var r *Refusal
			if c.want != "" && (!errors.As(err, &r) || r.Message() == "") {
				t.Fatalf("refusal %v has no user copy", err)
			}
		})
	}
}

func TestZeroTargetNeverVerifies(t *testing.T) {
	src := &fakeSource{snaps: []window.Snapshot{snap(editor())}}
	if got := ReasonOf(Target{}.Verify(context.Background(), src)); got != ReasonUnavailable {
		t.Fatalf("zero target: %q", got)
	}
	// A target with a window id but no capture sequence was not made by
	// Capture.
	forged := Target{Window: "0x5a1e", App: "org.gnome.TextEditor", Output: "eDP-1"}
	if got := ReasonOf(forged.Verify(context.Background(), src)); got != ReasonUnavailable {
		t.Fatalf("uncaptured target: %q", got)
	}
	if src.reads != 0 {
		t.Fatalf("a target that cannot match read the windows %d times", src.reads)
	}
	tg, err := Capture(context.Background(), src)
	if err != nil {
		t.Fatal(err)
	}
	if got := ReasonOf(tg.Verify(context.Background(), nil)); got != ReasonUnavailable {
		t.Fatalf("nil source: %q", got)
	}
}

func TestReasonOf(t *testing.T) {
	if ReasonOf(nil) != "" {
		t.Fatal("nil error has a reason")
	}
	if ReasonOf(errors.New("other")) != ReasonUnavailable {
		t.Fatal("a foreign error must read as unavailable")
	}
	wrapped := errors.Join(errors.New("ctx"), &Refusal{Reason: ReasonMovedOutput})
	if ReasonOf(wrapped) != ReasonMovedOutput {
		t.Fatal("wrapped refusal lost its reason")
	}
}
