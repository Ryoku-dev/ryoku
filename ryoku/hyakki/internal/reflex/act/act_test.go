package act

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"ryoku-rashin/internal/oni/reflex"
	"ryoku-rashin/internal/oni/window"
	"ryoku-rashin/internal/reflex/audio"
	"ryoku-rashin/internal/reflex/browser"
	"ryoku-rashin/internal/reflex/launch"
	"ryoku-rashin/internal/reflex/shot"
	"ryoku-rashin/internal/reflex/wm"
)

type rig struct {
	spawns [][]string
	runs   [][]string
	now    time.Time
}

func (r *rig) spawn(argv []string) (<-chan error, error) {
	r.spawns = append(r.spawns, slices.Clone(argv))
	return make(chan error), nil
}

func (r *rig) run(_ context.Context, name string, args ...string) ([]byte, error) {
	r.runs = append(r.runs, append([]string{name}, args...))
	if name == "pactl" && args[0] == "get-sink-volume" {
		return []byte("Volume: front-left: 1 /  40% / -1 dB"), nil
	}
	if name == "ryoku" {
		return nil, errors.New("no seam")
	}
	return nil, nil
}

func newRig(t *testing.T) (*rig, Executor) {
	t.Helper()
	dir := t.TempDir()
	for id, body := range map[string]string{
		"dev.ryoku.OniNotes.desktop":   "[Desktop Entry]\nType=Application\nName=Lindqvist Notes\nExec=notes-app %F\n",
		"dev.ryoku.OniBrowser.desktop": "[Desktop Entry]\nType=Application\nName=Lab Browser\nExec=tabs-app %u\nCategories=WebBrowser;\n",
	} {
		if err := os.WriteFile(filepath.Join(dir, id), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	r := &rig{now: time.Unix(5000, 0)}
	cat := launch.Scan(launch.ScanOptions{Dirs: []string{dir}, Max: 200})
	l := launch.New(cat, launch.Options{Spawn: r.spawn, Systemd: func() bool { return false }, Now: func() time.Time { return r.now }})
	b, err := browser.New(l, "dev.ryoku.OniBrowser.desktop", "")
	if err != nil {
		t.Fatal(err)
	}
	x := New(Deps{
		Launcher: l, Browser: b, Volume: audio.NewVolume(r.run),
		WM:   wm.New([]string{window.EnvHyprlandInstance + "=x"}, r.run),
		Shot: shot.New(shot.Options{LookPath: func(string) (string, error) { return "", errors.New("none") }}),
	})
	return r, x
}

func TestLaunchAndCooldownOutcomes(t *testing.T) {
	ctx := context.Background()
	r, x := newRig(t)
	out, err := x.Do(ctx, reflex.Action{Intent: reflex.OpenApp, Arg: "Lindqvist Notes", Confidence: 1})
	if err != nil || !out.Done || out.Line != "Opening Lindqvist Notes." || out.Launch == nil || out.Launch.Scope != launch.ScopeFallback {
		t.Fatalf("launch: %+v %v", out, err)
	}
	out, err = x.Do(ctx, reflex.Action{Intent: reflex.OpenApp, Arg: "Lindqvist Notes", Confidence: 1})
	if !errors.Is(err, launch.ErrCooldown) || out.Done || out.Line != LineCooldown {
		t.Fatalf("a launch inside the cooldown: %+v %v", out, err)
	}
	if out.Line == LineFailed || errors.Is(err, ErrUnsupported) {
		t.Fatal("a cooldown refusal must be its own answer, not a failure")
	}
	out, err = x.Do(ctx, reflex.Action{Intent: reflex.OpenApp, Arg: "Nothing Like It"})
	if !errors.Is(err, launch.ErrNotFound) || out.Line != LineNoApp {
		t.Fatalf("unknown app: %+v %v", out, err)
	}
	out, err = x.Do(ctx, reflex.Action{Intent: reflex.NewTab})
	if err != nil || out.Line != LineNewTab {
		t.Fatalf("new tab: %+v %v", out, err)
	}
	want := [][]string{{"notes-app"}, {"tabs-app", "about:blank"}}
	if !slices.EqualFunc(r.spawns, want, slices.Equal) {
		t.Fatalf("started %q, want %q", r.spawns, want)
	}
}

func TestUnsupportedVerbsAreHonest(t *testing.T) {
	ctx := context.Background()
	r, x := newRig(t)
	for _, in := range []reflex.Intent{reflex.SnapLeft, reflex.SnapRight, reflex.Maximize, reflex.Minimize} {
		out, err := x.Do(ctx, reflex.Action{Intent: in})
		if !errors.Is(err, ErrUnsupported) || out.Done || out.Line != LineCantHere {
			t.Errorf("%s: %+v %v", in, out, err)
		}
	}
	out, err := x.Do(ctx, reflex.Action{Intent: reflex.Screenshot})
	if !errors.Is(err, shot.ErrUnavailable) || out.Done || out.Line != LineNoShotTool {
		t.Fatalf("screenshot without a tool: %+v %v", out, err)
	}
	for _, in := range []reflex.Intent{"close_tab", "next_tab", "type_text", "send_message"} {
		out, err := x.Do(ctx, reflex.Action{Intent: in})
		if !errors.Is(err, ErrUnsupported) || out.Done || out.Line != LineCantHere {
			t.Errorf("%s: %+v %v", in, out, err)
		}
	}
	for _, in := range []reflex.Intent{reflex.RememberNote, reflex.SetTimer, reflex.SelfMove, reflex.SettingFlip, reflex.RunRoutine} {
		if _, err := x.Do(ctx, reflex.Action{Intent: in}); !errors.Is(err, ErrNotExecutor) {
			t.Errorf("%s: %v, want ErrNotExecutor", in, err)
		}
	}
	for _, c := range r.runs {
		if c[0] != "ryoku" || !slices.Equal(c[1:], []string{"wm", "caps"}) {
			t.Fatalf("an unsupported verb ran %q", c)
		}
	}
	if len(r.spawns) != 0 {
		t.Fatalf("started %q", r.spawns)
	}

	none := New(Deps{})
	for _, info := range reflex.Registry() {
		out, err := none.Do(ctx, reflex.Action{Intent: info.ID})
		if errors.Is(err, ErrNotExecutor) {
			continue
		}
		if err == nil || out.Done {
			t.Errorf("%s with no executors claimed success: %+v", info.ID, out)
		}
	}
	for in, ok := range none.Caps() {
		if ok {
			t.Errorf("%s offered with no executors", in)
		}
	}
}

func TestCaps(t *testing.T) {
	_, x := newRig(t)
	caps := x.Caps()
	want := map[reflex.Intent]bool{
		reflex.OpenApp: true, reflex.OpenSite: true, reflex.WebSearch: true, reflex.NewTab: true,
		reflex.SnapLeft: false, reflex.SnapRight: false, reflex.Maximize: false, reflex.Minimize: false,
		reflex.VolumeUp: true, reflex.VolumeDown: true, reflex.Mute: true, reflex.PlayPause: false, reflex.Screenshot: false,
	}
	for in, w := range want {
		if caps[in] != w {
			t.Errorf("%s: %v, want %v", in, caps[in], w)
		}
	}
	if len(caps) != len(want) {
		t.Errorf("caps cover %d intents, want %d", len(caps), len(want))
	}
}

func TestVolumeOutcome(t *testing.T) {
	r, x := newRig(t)
	out, err := x.Do(context.Background(), reflex.Action{Intent: reflex.VolumeUp})
	if err != nil || out.Line != LineVolumeUp {
		t.Fatalf("%+v %v", out, err)
	}
	if last := r.runs[len(r.runs)-1]; !slices.Equal(last, []string{"pactl", "set-sink-volume", "@DEFAULT_SINK@", "+5%"}) {
		t.Fatalf("ran %q", last)
	}
	out, _ = x.Do(context.Background(), reflex.Action{Intent: reflex.Mute, Arg: reflex.MuteOn})
	if out.Line != LineMuted {
		t.Fatalf("%+v", out)
	}
}

// Every line Oni may say keeps its first sentence to five words and names
// no product.
func TestLinesAreShort(t *testing.T) {
	lines := []string{
		LineCantHere, LineFailed, LineCooldown, LineNoApp, LineNoBrowser, LineBadAddress, LineNewTab,
		LineOpenSite, LineSearch, LineSnapLeft, LineSnapRight, LineMaximize, LineNoWindow, LineVolumeUp,
		LineVolumeDown, LineVolumeFull, LineMuted, LineUnmuted, LineMuteToggled, LinePlaying, LinePaused,
		LineToggled, LineNoPlayer, LineNothingOn, LineAlreadyOn, LinePickArea, LineShotSaved, LineNoShotTool,
		LineOpeningIt, opening("Lindqvist Notes"), opening("A Very Long App Name Here"),
	}
	for _, l := range lines {
		first, _, _ := strings.Cut(l, ".")
		if n := len(strings.Fields(first)); n == 0 || n > 5 {
			t.Errorf("%q: first sentence has %d words", l, n)
		}
	}
	if opening("A Very Long App Name Here") != LineOpeningIt {
		t.Fatal("a long name falls back to a short line")
	}
}
