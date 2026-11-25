package audio

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/godbus/dbus/v5"

	"ryoku-rashin/internal/reflex/bus"
)

func TestAllowedIsExact(t *testing.T) {
	allowed := [][]string{
		{"get-sink-volume", "@DEFAULT_SINK@"},
		{"set-sink-volume", "@DEFAULT_SINK@", "+5%"},
		{"set-sink-volume", "@DEFAULT_SINK@", "-5%"},
		{"set-sink-volume", "@DEFAULT_SINK@", "100%"},
		{"set-sink-mute", "@DEFAULT_SINK@", "1"},
		{"set-sink-mute", "@DEFAULT_SINK@", "0"},
		{"set-sink-mute", "@DEFAULT_SINK@", "toggle"},
	}
	for _, a := range allowed {
		if !Allowed("pactl", a...) {
			t.Errorf("refused pactl %q", a)
		}
	}
	refused := []struct {
		name string
		args []string
	}{
		{"pactl", []string{"set-sink-volume", "@DEFAULT_SINK@", "+50%"}},
		{"pactl", []string{"set-sink-volume", "@DEFAULT_SINK@", "150%"}},
		{"pactl", []string{"set-sink-volume", "alsa_output.pci", "+5%"}},
		{"pactl", []string{"set-source-mute", "@DEFAULT_SOURCE@", "1"}},
		{"pactl", []string{"set-default-sink", "x"}},
		{"pactl", []string{"load-module", "module-null-sink"}},
		{"pactl", []string{"set-sink-mute", "@DEFAULT_SINK@"}},
		{"pactl", []string{"get-sink-volume", "@DEFAULT_SINK@", "extra"}},
		{"pactl", nil},
		{"wpctl", []string{"set-volume", "@DEFAULT_AUDIO_SINK@", "5%+"}},
		{"amixer", []string{"set", "Master", "5%+"}},
		{"sh", []string{"-c", "pactl set-sink-volume @DEFAULT_SINK@ +5%"}},
	}
	for _, r := range refused {
		if Allowed(r.name, r.args...) {
			t.Errorf("allowed %s %q", r.name, r.args)
		}
	}
}

type fakePactl struct {
	volume string
	calls  [][]string
}

func (f *fakePactl) run(_ context.Context, name string, args ...string) ([]byte, error) {
	f.calls = append(f.calls, append([]string{name}, args...))
	if slices.Equal(args, argvGetVolume) {
		return []byte(f.volume), nil
	}
	return nil, nil
}

func vol(left, right string) string {
	return "Volume: front-left: 1 / " + left + "% / -1 dB,   front-right: 1 / " + right + "% / -1 dB\n        balance 0.00\n"
}

func TestVolumeSteps(t *testing.T) {
	ctx := context.Background()
	cases := []struct {
		name   string
		volume string
		do     func(*Volume) error
		last   string
		err    error
	}{
		{"up from half", vol("50", "48"), (*Volume).upCtx, "+5%", nil},
		{"up near full clamps to 100", vol("97", "98"), (*Volume).upCtx, "100%", nil},
		{"up at full changes nothing", vol("100", "100"), (*Volume).upCtx, "get-sink-volume", ErrAtFull},
		{"down", vol("50", "50"), (*Volume).downCtx, "-5%", nil},
		{"mute on", "", func(v *Volume) error { return v.Mute(ctx, MuteOn) }, "1", nil},
		{"mute off", "", func(v *Volume) error { return v.Mute(ctx, MuteOff) }, "0", nil},
		{"mute toggle", "", func(v *Volume) error { return v.Mute(ctx, MuteToggle) }, "toggle", nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := &fakePactl{volume: c.volume}
			err := c.do(NewVolume(f.run))
			if !errors.Is(err, c.err) || (c.err == nil && err != nil) {
				t.Fatalf("err = %v, want %v", err, c.err)
			}
			last := f.calls[len(f.calls)-1]
			if last[len(last)-1] != c.last && !(c.last == "get-sink-volume" && slices.Equal(last[1:], argvGetVolume)) {
				t.Fatalf("calls %q, last should end %q", f.calls, c.last)
			}
			for _, call := range f.calls {
				if !Allowed(call[0], call[1:]...) || !slices.Contains(call, "@DEFAULT_SINK@") {
					t.Fatalf("ran %q", call)
				}
			}
		})
	}
	f := &fakePactl{volume: "garbage"}
	if err := NewVolume(f.run).Up(ctx); err == nil || len(f.calls) != 1 {
		t.Fatalf("an unreadable level must not change the volume: %v %q", err, f.calls)
	}
}

func (v *Volume) upCtx() error   { return v.Up(context.Background()) }
func (v *Volume) downCtx() error { return v.Down(context.Background()) }

type call struct {
	dest, path, method string
	args               []any
}

type fakeBus struct {
	names  []string
	status map[string]string // missing: Get fails
	calls  []call
	closed int
}

func (f *fakeBus) dial(context.Context) (bus.Conn, error) { return f, nil }

func (f *fakeBus) Call(_ context.Context, dest, path, method string, args []any, reply ...any) error {
	f.calls = append(f.calls, call{dest, path, method, args})
	switch method {
	case listNames:
		*reply[0].(*[]string) = f.names
	case propsGet:
		s, ok := f.status[dest]
		if !ok {
			return errors.New("no such property")
		}
		*reply[0].(*dbus.Variant) = dbus.MakeVariant(s)
	}
	return nil
}

func (f *fakeBus) Close() error { f.closed++; return nil }

func (f *fakeBus) flipped() []string {
	var out []string
	for _, c := range f.calls {
		if strings.HasSuffix(c.method, ".PlayPause") {
			out = append(out, c.dest)
		}
	}
	return out
}

func TestMediaFlipsOnePlayerWithPlayPause(t *testing.T) {
	const a, b = "org.mpris.MediaPlayer2.alpha", "org.mpris.MediaPlayer2.beta"
	cases := []struct {
		name    string
		status  map[string]string
		verb    MediaVerb
		did     MediaDid
		err     error
		flipped []string
	}{
		{"pause the playing one", map[string]string{a: "Paused", b: "Playing"}, MediaPause, DidPause, nil, []string{b}},
		{"pause with nothing playing", map[string]string{a: "Paused", b: "Stopped"}, MediaPause, DidNothing, ErrNothingPlaying, nil},
		{"play the paused one", map[string]string{a: "Stopped", b: "Paused"}, MediaPlay, DidPlay, nil, []string{b}},
		{"play while playing", map[string]string{a: "Playing"}, MediaPlay, DidNothing, ErrAlreadyPlaying, nil},
		{"toggle pauses first", map[string]string{a: "Paused", b: "Playing"}, MediaToggle, DidPause, nil, []string{b}},
		{"toggle plays a paused one", map[string]string{a: "Paused", b: "Stopped"}, MediaToggle, DidPlay, nil, []string{a}},
		{"no status anywhere flips the first", map[string]string{}, MediaPause, DidToggle, nil, []string{a}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := &fakeBus{names: []string{"org.freedesktop.Notifications", b, a, ":1.42"}, status: c.status}
			did, err := NewMedia(f.dial).Do(context.Background(), c.verb)
			if did != c.did || !errors.Is(err, c.err) || (c.err == nil && err != nil) {
				t.Fatalf("did %v err %v, want %v %v", did, err, c.did, c.err)
			}
			if !slices.Equal(f.flipped(), c.flipped) {
				t.Fatalf("PlayPause on %q, want %q", f.flipped(), c.flipped)
			}
			if f.closed != 1 {
				t.Fatalf("the connection must be closed after the action, closed %d", f.closed)
			}
			for _, cl := range f.calls {
				if !AllowedCall(cl.dest, cl.path, cl.method, cl.args) {
					t.Fatalf("made a call off the allowlist: %+v", cl)
				}
			}
		})
	}
	f := &fakeBus{names: []string{"org.freedesktop.Notifications"}}
	if _, err := NewMedia(f.dial).Do(context.Background(), MediaToggle); !errors.Is(err, ErrNoPlayer) {
		t.Fatalf("no player: %v", err)
	}
}

func TestMediaAllowedCallIsExact(t *testing.T) {
	const p = "org.mpris.MediaPlayer2.alpha"
	ok := []call{
		{busName, busPath, listNames, nil},
		{p, playerPath, propsGet, []any{playerIface, statusProp}},
		{p, playerPath, playerIface + ".PlayPause", nil},
	}
	for _, c := range ok {
		if !AllowedCall(c.dest, c.path, c.method, c.args) {
			t.Errorf("refused %+v", c)
		}
	}
	bad := []call{
		{p, playerPath, playerIface + ".Play", nil},
		{p, playerPath, playerIface + ".Pause", nil},
		{p, playerPath, playerIface + ".Next", nil},
		{p, playerPath, playerIface + ".OpenUri", []any{"file:///x"}},
		{p, playerPath, "org.mpris.MediaPlayer2.Quit", nil},
		{p, playerPath, "org.freedesktop.DBus.Properties.Set", []any{playerIface, "Volume", 1.0}},
		{p, playerPath, propsGet, []any{playerIface, "Metadata"}},
		{p, "/other", playerIface + ".PlayPause", nil},
		{p, playerPath, playerIface + ".PlayPause", []any{"x"}},
		{"org.mpris.MediaPlayer2.", playerPath, playerIface + ".PlayPause", nil},
		{"org.mpris.MediaPlayer2.a/b", playerPath, playerIface + ".PlayPause", nil},
		{"org.freedesktop.Notifications", "/org/freedesktop/Notifications", "org.freedesktop.Notifications.Notify", nil},
		{busName, busPath, listNames, []any{"x"}},
		{busName, busPath, "org.freedesktop.DBus.RequestName", []any{"x", uint32(0)}},
	}
	for _, c := range bad {
		if AllowedCall(c.dest, c.path, c.method, c.args) {
			t.Errorf("allowed %+v", c)
		}
	}
}
