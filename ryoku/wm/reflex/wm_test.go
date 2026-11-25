package wm

import (
	"context"
	"errors"
	"slices"
	"testing"
)

func TestAllowedIsExact(t *testing.T) {
	allowed := [][]string{
		{"ryoku", "wm", "caps"},
		{"ryoku", "wm", "state"},
		{"ryoku", "wm", "act", "window.place", "0xabc", "0", "0", "960", "1080", "DP-1"},
		{"niri", "msg", "action", "maximize-column"},
	}
	for _, a := range allowed {
		if !Allowed(a[0], a[1:]...) {
			t.Errorf("refused %q", a)
		}
	}
	refused := [][]string{
		{"ryoku", "wm", "act", "window.close", "0xabc"},
		{"ryoku", "wm", "act", "window.focus", "0xabc"},
		{"ryoku", "wm", "act", "session.exit"},
		{"ryoku", "wm", "act", "window.place", "0xabc", "-1", "0", "960", "1080", "DP-1"},
		{"ryoku", "wm", "act", "window.place", "0xabc", "0", "0", "960", "1080"},
		{"ryoku", "wm", "act", "window.place", "-rf", "0", "0", "960", "1080", "DP-1"},
		{"ryoku", "wm", "act", "window.place", "0xabc", "0", "0", "960", "1080", "DP 1"},
		{"ryoku", "wm", "act", "window.place", "0xabc", "00", "0", "960", "1080", "DP-1"},
		{"ryoku", "wm", "act", "window.place", "0xabc", "0", "0", "999999", "1080", "DP-1"},
		{"ryoku", "wm", "watch"},
		{"niri", "msg", "action", "close-window"},
		{"niri", "msg", "action", "spawn", "--", "x"},
		{"hyprctl", "dispatch", "hl.dsp.window.fullscreen()"},
	}
	for _, a := range refused {
		if Allowed(a[0], a[1:]...) {
			t.Errorf("allowed %q", a)
		}
	}
}

type fakeWM struct {
	caps, state string
	capsErr     error
	calls       [][]string
}

func (f *fakeWM) run(_ context.Context, name string, args ...string) ([]byte, error) {
	f.calls = append(f.calls, append([]string{name}, args...))
	switch {
	case slices.Equal(args, []string{"wm", "caps"}):
		return []byte(f.caps), f.capsErr
	case slices.Equal(args, []string{"wm", "state"}):
		return []byte(f.state), nil
	}
	return nil, nil
}

const (
	fullCaps   = `{"name":"niri","supports":["windowFloat","focusHistory","workspaces"]}`
	noFocus    = `{"name":"hyprland","supports":["windowFloat"]}`
	twoOutputs = `{"outputs":[{"name":"DP-1","width":1921,"height":1080},{"name":"HDMI-A-1","width":1280,"height":720}],
	"workspaces":[{"id":"3","output":"HDMI-A-1"}],
	"windows":[{"id":"w2","workspace":"1","output":"DP-1","focusOrder":1},{"id":"w9","workspace":"3","focusOrder":0}]}`
)

func TestSupportsIsHonestPerProvider(t *testing.T) {
	ctx := context.Background()
	cases := []struct {
		name string
		env  []string
		f    *fakeWM
		want map[Verb]bool
	}{
		{"seam on niri", []string{"NIRI_SOCKET=/run/niri.sock"}, &fakeWM{caps: fullCaps},
			map[Verb]bool{SnapLeft: true, SnapRight: true, Maximize: true, Minimize: false}},
		{"seam without focus history", []string{"HYPRLAND_INSTANCE_SIGNATURE=x"}, &fakeWM{caps: noFocus},
			map[Verb]bool{SnapLeft: false, SnapRight: false, Maximize: false, Minimize: false}},
		{"bare niri", []string{"NIRI_SOCKET=/run/niri.sock"}, &fakeWM{capsErr: errors.New("no seam")},
			map[Verb]bool{SnapLeft: false, SnapRight: false, Maximize: true, Minimize: false}},
		{"bare Hyprland", []string{"HYPRLAND_INSTANCE_SIGNATURE=x"}, &fakeWM{capsErr: errors.New("no seam")},
			map[Verb]bool{SnapLeft: false, SnapRight: false, Maximize: false, Minimize: false}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := New(c.env, c.f.run).Supports(ctx)
			for v, want := range c.want {
				if got[v] != want {
					t.Errorf("%s: %v, want %v", v, got[v], want)
				}
			}
		})
	}
}

func TestUnsupportedVerbsRunNothingAndSaySo(t *testing.T) {
	ctx := context.Background()
	f := &fakeWM{capsErr: errors.New("no seam")}
	w := New([]string{"HYPRLAND_INSTANCE_SIGNATURE=x"}, f.run)
	for _, v := range []Verb{SnapLeft, SnapRight, Maximize, Minimize, Verb("close")} {
		if err := w.Do(ctx, v); !errors.Is(err, ErrUnsupported) {
			t.Errorf("%s: %v, want ErrUnsupported", v, err)
		}
	}
	for _, c := range f.calls {
		if !slices.Equal(c, []string{"ryoku", "wm", "caps"}) {
			t.Fatalf("an unsupported verb ran %q", c)
		}
	}
	full := &fakeWM{caps: fullCaps, state: twoOutputs}
	if err := New([]string{"NIRI_SOCKET=/s"}, full.run).Do(ctx, Minimize); !errors.Is(err, ErrUnsupported) || len(full.calls) != 0 {
		t.Fatalf("minimize is offered nowhere: %v %q", err, full.calls)
	}
}

func TestSnapPlacesTheFocusedWindowOnItsOutput(t *testing.T) {
	ctx := context.Background()
	for _, c := range []struct {
		verb Verb
		want []string
	}{
		{SnapLeft, []string{"ryoku", "wm", "act", "window.place", "w9", "0", "0", "640", "720", "HDMI-A-1"}},
		{SnapRight, []string{"ryoku", "wm", "act", "window.place", "w9", "640", "0", "640", "720", "HDMI-A-1"}},
	} {
		f := &fakeWM{caps: fullCaps, state: twoOutputs}
		if err := New(nil, f.run).Do(ctx, c.verb); err != nil {
			t.Fatal(err)
		}
		if last := f.calls[len(f.calls)-1]; !slices.Equal(last, c.want) {
			t.Fatalf("%s placed with %q, want %q", c.verb, last, c.want)
		}
	}
	odd := `{"outputs":[{"name":"DP-1","width":1921,"height":1080}],"windows":[{"id":"w1","output":"DP-1","focusOrder":0}]}`
	f := &fakeWM{caps: fullCaps, state: odd}
	if err := New(nil, f.run).Do(ctx, SnapRight); err != nil {
		t.Fatal(err)
	}
	if last := f.calls[len(f.calls)-1]; !slices.Equal(last[5:], []string{"960", "0", "961", "1080", "DP-1"}) {
		t.Fatalf("an odd width gives the right half the extra pixel: %q", last)
	}
	none := &fakeWM{caps: fullCaps, state: `{"outputs":[{"name":"DP-1","width":100,"height":100}],"windows":[]}`}
	if err := New(nil, none.run).Do(ctx, SnapLeft); !errors.Is(err, ErrNoWindow) {
		t.Fatalf("no focused window: %v", err)
	}
}

func TestMaximizeIsNirisOwnVerb(t *testing.T) {
	f := &fakeWM{}
	if err := New([]string{"NIRI_SOCKET=/s"}, f.run).Do(context.Background(), Maximize); err != nil {
		t.Fatal(err)
	}
	if !slices.EqualFunc(f.calls, [][]string{{"niri", "msg", "action", "maximize-column"}}, slices.Equal) {
		t.Fatalf("ran %q", f.calls)
	}
}
