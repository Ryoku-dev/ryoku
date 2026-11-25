package feedclose

import (
	"context"
	"errors"
	"slices"
	"testing"
)

func TestAllowedIsExact(t *testing.T) {
	for _, a := range [][]string{
		{"ryoku", "wm", "caps"},
		{"ryoku", "wm", "state"},
		{"ryoku", "wm", "act", "window.close", "0xabc"},
		{"ryoku", "wm", "act", "window.close", "21"},
	} {
		if !Allowed(a[0], a[1:]...) {
			t.Errorf("refused %q", a)
		}
	}
	for _, a := range [][]string{
		{"ryoku", "wm", "act", "window.close"},
		{"ryoku", "wm", "act", "window.close", "-rf"},
		{"ryoku", "wm", "act", "window.close", "0xabc", "0xdef"},
		{"ryoku", "wm", "act", "window.close", "a b"},
		{"ryoku", "wm", "act", "window.focus", "0xabc"},
		{"ryoku", "wm", "act", "window.place", "0xabc", "0", "0", "960", "1080", "DP-1"},
		{"ryoku", "wm", "act", "session.exit"},
		{"ryoku", "wm", "watch"},
		{"unsafe-window-helper", "msg", "action", "close-window"},
		{"unsafe-window-helper", "msg", "action", "close-window", "--id", "7"},
		{"unsafe-window-helper", "dispatch", "hl.dsp.window.close()"},
		{"wtype", "-M", "ctrl", "-k", "w"},
		{"wtype", "-M", "ctrl", "-k", "q"},
	} {
		if Allowed(a[0], a[1:]...) {
			t.Errorf("allowed %q", a)
		}
	}
}

type fakeSeam struct {
	caps, state string
	capsErr     error
	calls       [][]string
}

func (f *fakeSeam) run(_ context.Context, name string, args ...string) ([]byte, error) {
	f.calls = append(f.calls, append([]string{name}, args...))
	switch {
	case slices.Equal(args, []string{"wm", "caps"}):
		return []byte(f.caps), f.capsErr
	case slices.Equal(args, []string{"wm", "state"}):
		return []byte(f.state), nil
	}
	return nil, nil
}

func (f *fakeSeam) closed() [][]string {
	var out [][]string
	for _, c := range f.calls {
		if len(c) > 3 && c[3] == "window.close" {
			out = append(out, c)
		}
	}
	return out
}

const (
	niriCaps  = `{"name":"niri","supports":["windowFloat"]}`
	feedState = `{"windows":[{"id":"21","appId":"tiktok","title":"For You"},{"id":"25","appId":"firefox","title":"Reddit"}]}`
)

func TestCloseActsOnlyOnTheSameWindowThroughTheSeam(t *testing.T) {
	ctx := context.Background()
	cases := []struct {
		name    string
		f       *fakeSeam
		id, app string
		want    error
		closes  bool
	}{
		{"same window", &fakeSeam{caps: niriCaps, state: feedState}, "21", "tiktok", nil, true},
		{"window gone", &fakeSeam{caps: niriCaps, state: `{"windows":[]}`}, "21", "tiktok", ErrChanged, false},
		{"id names another app", &fakeSeam{caps: niriCaps, state: feedState}, "25", "tiktok", ErrChanged, false},
		{"no seam", &fakeSeam{capsErr: errors.New("no ryoku")}, "21", "tiktok", ErrUnsupported, false},
		{"seam without a name", &fakeSeam{caps: `{}`, state: feedState}, "21", "tiktok", ErrUnsupported, false},
		{"bad id", &fakeSeam{caps: niriCaps, state: feedState}, "-21", "tiktok", ErrChanged, false},
	}
	for _, c := range cases {
		err := New(c.f.run).Close(ctx, c.id, c.app)
		if !errors.Is(err, c.want) {
			t.Errorf("%s: err %v, want %v", c.name, err, c.want)
		}
		if got := c.f.closed(); (len(got) == 1) != c.closes || len(got) > 1 {
			t.Errorf("%s: closes %q", c.name, got)
		}
		if c.closes && !slices.Equal(c.f.closed()[0], []string{"ryoku", "wm", "act", "window.close", c.id}) {
			t.Errorf("%s: closed with %q", c.name, c.f.closed()[0])
		}
	}
}

func TestSupportedNeedsANamedSeam(t *testing.T) {
	ctx := context.Background()
	if !New((&fakeSeam{caps: niriCaps}).run).Supported(ctx) {
		t.Error("a named seam is not supported")
	}
	if New((&fakeSeam{capsErr: errors.New("x")}).run).Supported(ctx) {
		t.Error("a missing seam is supported")
	}
	if New(nil).Supported(ctx) {
		t.Error("no runner is supported")
	}
}
