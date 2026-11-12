package main

import (
	"context"
	"os"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	wm "ryoku-wm"
)

// The audio verb steps volume by the reference literal (5% of the [0,1] range),
// clamps the rise at 1.0, and toggles mute; those argv are the behaviour, so
// pin them.
func TestAudioArgv(t *testing.T) {
	cases := map[string][]string{
		"up":   {"set-volume", "-l", "1", "@DEFAULT_AUDIO_SINK@", "5%+"},
		"down": {"set-volume", "@DEFAULT_AUDIO_SINK@", "5%-"},
		"mute": {"set-mute", "@DEFAULT_AUDIO_SINK@", "toggle"},
	}
	for sub, want := range cases {
		got, ok := audioArgv(sub)
		if !ok || !reflect.DeepEqual(got, want) {
			t.Errorf("audioArgv(%q) = (%v,%v), want (%v,true)", sub, got, ok, want)
		}
	}
	if _, ok := audioArgv("sideways"); ok {
		t.Errorf("audioArgv(sideways) accepted, want rejection")
	}
}

// The brightness verb steps by the reference literal (5 of the [0,100] range).
func TestBrightnessArgv(t *testing.T) {
	cases := map[string][]string{
		"up":   {"+5"},
		"down": {"-5"},
	}
	for sub, want := range cases {
		got, ok := brightnessArgv(sub)
		if !ok || !reflect.DeepEqual(got, want) {
			t.Errorf("brightnessArgv(%q) = (%v,%v), want (%v,true)", sub, got, ok, want)
		}
	}
	if _, ok := brightnessArgv("sideways"); ok {
		t.Errorf("brightnessArgv(sideways) accepted, want rejection")
	}
}

// menuID accepts only `menu <id>` for an id in the current frame menu catalog;
// retired sidebar/power menu ids, the old bar-prefixed form, and unknown ids
// must miss.
func TestMenuID(t *testing.T) {
	if id, ok := menuID("menu theme"); !ok || id != "theme" {
		t.Errorf("menuID(menu theme) = (%q,%v), want (theme,true)", id, ok)
	}
	// #page suffix: the full id (base#page) is returned for QML's deep-link.
	if id, ok := menuID("menu wallpaper#gallery"); !ok || id != "wallpaper#gallery" {
		t.Errorf("menuID(menu wallpaper#gallery) = (%q,%v), want (wallpaper#gallery,true)", id, ok)
	}
	for _, cmd := range []string{"menu", "menu bogus", "menu clock", "menu clipboard", "menu recording", "menu system", "menu quick-settings", "menu quick-settings#clipboard", "menu clock extra", "bar clock", "clipboard", "menu bogus#page"} {
		if _, ok := menuID(cmd); ok {
			t.Errorf("menuID(%q) accepted, want rejection", cmd)
		}
	}
}

// Every new control verb rejects a missing or malformed argument with a
// verb-scoped error rather than opening the wrong thing or a bare "unknown
// command"; these paths return before any IPC or subprocess.
func TestDispatchControlErrors(t *testing.T) {
	d := &daemon{}
	cases := map[string]string{
		"menu":                            "err menu",
		"menu bogus":                      "err menu",
		"audio":                           "err audio",
		"audio sideways":                  "err audio",
		"brightness":                      "err brightness",
		"brightness sideways":             "err brightness",
		"hub":                             "err hub",
		"hub bogus":                       "err hub",
		"hub open display users":          "err hub",
		"hub close now":                   "err hub",
		"hub open users a b":              "err hub",
		"lock-quiesce now":                "err lock-quiesce",
		"sleep-ready":                     "err sleep-ready",
		"sleep-ready now":                 "err sleep-ready",
		"suspend":                         "err suspend",
		"suspend now":                     "err suspend",
		"suspend transaction":             "err suspend",
		"suspend-cancel":                  "err suspend-cancel",
		"suspend-cancel transaction":      "err suspend-cancel",
		"suspend-cancel transaction bad/": "err suspend-cancel",
	}
	for cmd, prefix := range cases {
		got := d.dispatch(cmd)
		if !strings.HasPrefix(got, prefix) {
			t.Errorf("dispatch(%q) = %q, want prefix %q", cmd, got, prefix)
		}
	}
}

// a Hub that isn't a bus client (older build, or its bus never connected) still gets found by the qs probe
func TestHubOpenUsersProfile(t *testing.T) {
	calls := fakeQs(t, func(context.Context, int) ([]byte, []byte, error) { return nil, nil, nil })
	launches, _ := fakeHubLaunch(t, nil)
	d := &daemon{wmc: wm.OpenNamed("does-not-exist")}
	if got := d.dispatch("hub open users hacker"); got != "ok" {
		t.Fatalf("dispatch = %q, want ok", got)
	}
	want := [][]string{
		{"-c", "hub", "ipc", "call", "nav", "section"},
		{"-c", "hub", "ipc", "call", "users", "fromProfile", "hacker"},
	}
	hubWaitFor(t, func() bool { return len(calls()) >= len(want) })
	if got := calls(); !reflect.DeepEqual(got, want) {
		t.Fatalf("hub qs calls = %q, want %q", got, want)
	}
	if n := launches(); n != 0 {
		t.Fatalf("a running Hub was launched again (%d)", n)
	}
}

func TestHubOpenWarmBusSendsOneCmd(t *testing.T) {
	calls := fakeQs(t, func(context.Context, int) ([]byte, []byte, error) { return nil, nil, nil })
	launches, _ := fakeHubLaunch(t, nil)
	fb := fakeBusClient(t, "hub", 4242, "nav", "users", "hub")
	d := &daemon{wmc: wm.OpenNamed("does-not-exist")}
	for _, tc := range []struct {
		line string
		want busCmd
	}{
		{"hub open displays/night-light", busCmd{Target: "nav", Fn: "open", Args: []string{"displays/night-light"}}},
		// an old section name passes through for the Hub's alias table
		{"hub open gpu", busCmd{Target: "nav", Fn: "open", Args: []string{"gpu"}}},
		{"hub open users hacker", busCmd{Target: "users", Fn: "fromProfile", Args: []string{"hacker"}}},
	} {
		before := len(fb.cmds())
		if got := d.dispatch(tc.line); got != "ok" {
			t.Fatalf("dispatch %q = %q, want ok", tc.line, got)
		}
		hubWaitFor(t, func() bool { return len(fb.cmds()) > before })
		time.Sleep(50 * time.Millisecond)
		cmds := fb.cmds()[before:]
		if len(cmds) != 1 || !sameBusCmd(cmds[0], tc.want) {
			t.Fatalf("%s: bus cmds = %+v, want one %+v", tc.line, cmds, tc.want)
		}
	}
	if got := calls(); len(got) != 0 {
		t.Fatalf("warm open ran qs: %q", got)
	}
	if n := launches(); n != 0 {
		t.Fatalf("warm open launched the Hub (%d)", n)
	}
}

// a cold open names its page in the launch itself, so the Hub never builds Home first
// and nothing waits on the bus for it
func TestHubOpenColdPassesStartPage(t *testing.T) {
	calls := fakeQs(t, func(context.Context, int) ([]byte, []byte, error) {
		return nil, []byte("No running instances for this config.\n"), nil
	})
	registered := make(chan *fakeBus, 1)
	launches, starts := fakeHubLaunch(t, func() {
		registered <- fakeBusClient(t, "hub", 4343, "nav", "users", "hub")
	})
	d := &daemon{wmc: wm.OpenNamed("does-not-exist")}
	if got := d.dispatch("hub open displays"); got != "ok" {
		t.Fatalf("dispatch = %q, want ok", got)
	}
	var fb *fakeBus
	select {
	case fb = <-registered:
	case <-time.After(2 * time.Second):
		t.Fatal("the cold open never launched the Hub")
	}
	time.Sleep(100 * time.Millisecond)
	if got := starts(); !reflect.DeepEqual(got, []string{"displays"}) {
		t.Fatalf("launch start pages = %q, want [displays]", got)
	}
	if cmds := fb.cmds(); len(cmds) != 0 {
		t.Fatalf("bus cmds = %+v, want none", cmds)
	}
	probe := [][]string{{"-c", "hub", "ipc", "call", "nav", "section"}}
	if got := calls(); !reflect.DeepEqual(got, probe) {
		t.Fatalf("qs calls = %q, want only the probe %q", got, probe)
	}
	if n := launches(); n != 1 {
		t.Fatalf("launches = %d, want 1", n)
	}
}

func TestHubCloseOverBus(t *testing.T) {
	calls := fakeQs(t, func(context.Context, int) ([]byte, []byte, error) { return nil, nil, nil })
	fb := fakeBusClient(t, "hub", 4444, "nav", "users", "hub")
	d := &daemon{wmc: wm.OpenNamed("does-not-exist")}
	if got := d.dispatch("hub close"); got != "ok" {
		t.Fatalf("dispatch = %q, want ok", got)
	}
	hubWaitFor(t, func() bool { return len(fb.cmds()) >= 1 })
	want := busCmd{Target: "hub", Fn: "close"}
	if cmds := fb.cmds(); len(cmds) != 1 || !sameBusCmd(cmds[0], want) {
		t.Fatalf("bus cmds = %+v, want one %+v", cmds, want)
	}
	if got := calls(); len(got) != 0 {
		t.Fatalf("close ran qs: %q", got)
	}
}

// two cold opens race for the one Hub; the open that loses the lock hands its page to the
// instance the other started, instead of dropping it
func TestHubOpenLosingColdOpenHandsItsPageOver(t *testing.T) {
	fakeQs(t, func(context.Context, int) ([]byte, []byte, error) {
		return nil, []byte("No running instances for this config.\n"), nil
	})
	prev := hubLaunch
	release := make(chan struct{})
	registered := make(chan *fakeBus, 1)
	var mu sync.Mutex
	winner := ""
	hubLaunch = func(_, start string) (func() error, error) {
		mu.Lock()
		first := winner == ""
		if first {
			winner = start
		}
		mu.Unlock()
		if !first {
			return func() error { return errHubStarting }, nil
		}
		go func() {
			time.Sleep(100 * time.Millisecond)
			registered <- fakeBusClient(t, "hub", 4545, "nav", "users", "hub")
		}()
		return func() error { <-release; return nil }, nil
	}
	t.Cleanup(func() {
		close(release)
		hubLaunch = prev
	})
	d := &daemon{wmc: wm.OpenNamed("does-not-exist")}
	for _, line := range []string{"hub open about", "hub open power"} {
		if got := d.dispatch(line); got != "ok" {
			t.Fatalf("dispatch %q = %q, want ok", line, got)
		}
	}
	var fb *fakeBus
	select {
	case fb = <-registered:
	case <-time.After(2 * time.Second):
		t.Fatal("neither open launched the Hub")
	}
	hubWaitFor(t, func() bool { return len(fb.cmds()) >= 1 })
	time.Sleep(50 * time.Millisecond)
	mu.Lock()
	loser := map[string]string{"about": "power", "power": "about"}[winner]
	mu.Unlock()
	want := busCmd{Target: "nav", Fn: "open", Args: []string{loser}}
	if cmds := fb.cmds(); len(cmds) != 1 || !sameBusCmd(cmds[0], want) {
		t.Fatalf("bus cmds = %+v, want one %+v (the page of the open that lost the lock)", cmds, want)
	}
}

// with the daemon down, `hub open` still opens Settings on the page
func TestHubOpenWithoutTheDaemonLaunchesOnThePage(t *testing.T) {
	fakeQs(t, func(context.Context, int) ([]byte, []byte, error) {
		return nil, []byte("No running instances for this config.\n"), nil
	})
	launches, starts := fakeHubLaunch(t, nil)
	if err := sendCommand("hub open displays"); err != nil {
		t.Fatalf("hub open with the daemon down: %v", err)
	}
	if n := launches(); n != 1 {
		t.Fatalf("launches = %d, want 1", n)
	}
	if got := starts(); !reflect.DeepEqual(got, []string{"displays"}) {
		t.Fatalf("launch start pages = %q, want [displays]", got)
	}
}

// with the daemon down and Settings running, `hub open` navigates it over qs ipc
func TestHubOpenWithoutTheDaemonNavigatesARunningHub(t *testing.T) {
	calls := fakeQs(t, func(context.Context, int) ([]byte, []byte, error) { return nil, nil, nil })
	launches, _ := fakeHubLaunch(t, nil)
	if err := sendCommand("hub open power"); err != nil {
		t.Fatalf("hub open with the daemon down: %v", err)
	}
	want := [][]string{
		{"-c", "hub", "ipc", "call", "nav", "section"},
		{"-c", "hub", "ipc", "call", "nav", "open", "power"},
	}
	if got := calls(); !reflect.DeepEqual(got, want) {
		t.Fatalf("qs calls = %q, want %q", got, want)
	}
	if n := launches(); n != 0 {
		t.Fatalf("a running Hub was launched again (%d)", n)
	}
}

func fakeHubLaunch(t *testing.T, start func()) (func() int, func() []string) {
	t.Helper()
	prev := hubLaunch
	done := make(chan struct{})
	var mu sync.Mutex
	n := 0
	var pages []string
	hubLaunch = func(_, page string) (func() error, error) {
		mu.Lock()
		n++
		pages = append(pages, page)
		mu.Unlock()
		if start != nil {
			go start()
		}
		return func() error { <-done; return nil }, nil
	}
	t.Cleanup(func() {
		close(done)
		hubLaunch = prev
	})
	launches := func() int {
		mu.Lock()
		defer mu.Unlock()
		return n
	}
	starts := func() []string {
		mu.Lock()
		defer mu.Unlock()
		return append([]string(nil), pages...)
	}
	return launches, starts
}

func TestHubScreenScales(t *testing.T) {
	cases := []struct {
		name     string
		settings map[string]any
		want     string
	}{
		{"no displays", map[string]any{}, ""},
		{"no scales", map[string]any{"displays": map[string]any{"bar": map[string]any{}}}, ""},
		{"sorted, unscaled skipped", map[string]any{"displays": map[string]any{"ui_scale": map[string]any{
			"HDMI-A-1": 1.25, "eDP-1": 1.0, "DP-2": 0.8,
		}}}, "DP-2=0.8;HDMI-A-1=1.25"},
		{"clamped like the shell", map[string]any{"displays": map[string]any{"ui_scale": map[string]any{
			"DP-1": 3.0, "DP-2": 0.2,
		}}}, "DP-1=2;DP-2=0.5"},
		{"bad entries dropped", map[string]any{"displays": map[string]any{"ui_scale": map[string]any{
			"DP-1": "1.5", "DP-2": -1.0, "a=b": 1.5, "c;d": 1.5, "DP-3": 1.5,
		}}}, "DP-3=1.5"},
	}
	for _, c := range cases {
		if got := hubScreenScales(c.settings); got != c.want {
			t.Errorf("%s: hubScreenScales = %q, want %q", c.name, got, c.want)
		}
	}
}

func sameBusCmd(got, want busCmd) bool {
	if got.Target != want.Target || got.Fn != want.Fn || got.Prop != want.Prop || len(got.Args) != len(want.Args) {
		return false
	}
	for i := range want.Args {
		if got.Args[i] != want.Args[i] {
			return false
		}
	}
	return true
}

func hubWaitFor(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for !cond() && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
}

// lock status is the reference check: it prints locked or unlocked and, per the
// exit-0-either-way contract, never fails. With no lock marker it is unlocked; a
// marker left behind by a killed locker is stale and cleared, so a crash does
// not report a locked screen that is really open.
func TestLockStatus(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())
	t.Setenv("XDG_SESSION_ID", "test")
	oldRunning, oldProof := lockProcessRunning, lockProofValid
	lockProcessRunning = func() bool { return false }
	lockProofValid = func() bool { return false }
	defer func() {
		lockProcessRunning, lockProofValid = oldRunning, oldProof
	}()
	d := &daemon{}
	if got := d.dispatch("lock status"); got != "unlocked" {
		t.Fatalf("dispatch(lock status) with no marker = %q, want unlocked", got)
	}
	marker := lockMarker()
	if err := os.WriteFile(marker, []byte{}, 0o600); err != nil {
		t.Fatal(err)
	}
	if got := d.dispatch("lock status"); got != "unlocked" {
		t.Fatalf("dispatch(lock status) with a stale marker = %q, want unlocked", got)
	}
	if _, err := os.Stat(marker); err == nil {
		t.Errorf("stale lock marker not cleared")
	}
}

func TestLockQuiesceRejectsNewLaunches(t *testing.T) {
	d := &daemon{}
	if got := d.dispatch("lock-quiesce"); got != "ok" {
		t.Fatalf("lock-quiesce = %q, want ok", got)
	}
	if got := d.dispatch("lock"); got != "err lock: generation cutover is in progress" {
		t.Fatalf("lock after quiesce = %q", got)
	}
}

func TestLockSessionRejectsAnotherLogin1Session(t *testing.T) {
	t.Setenv("XDG_SESSION_ID", "session-a")
	d := &daemon{}
	const want = "err lock: requested login1 session is not owned by this daemon"
	if got := d.dispatch("lock session session-b"); got != want {
		t.Fatalf("dispatch(lock session session-b) = %q, want %q", got, want)
	}
}

func TestUnlockPrepareRejectsAnotherLogin1Session(t *testing.T) {
	t.Setenv("XDG_SESSION_ID", "session-a")
	d := &daemon{}
	const want = "err unlock-prepare: requested login1 session is not owned by this daemon"
	if got := d.dispatch("unlock-prepare session session-b"); got != want {
		t.Fatalf("dispatch(unlock-prepare session session-b) = %q, want %q", got, want)
	}
}
