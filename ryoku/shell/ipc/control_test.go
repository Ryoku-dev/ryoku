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

// A Hub that is not a bus client (an older build, or one whose bus never
// connected) is still found by the qs probe and driven through qs ipc:
// `hub open users <profile-id>` lands it on that profile's Users page.
func TestHubOpenUsersProfile(t *testing.T) {
	calls := fakeQs(t, func(context.Context, int) ([]byte, []byte, error) { return nil, nil, nil })
	launches := fakeHubLaunch(t, nil)
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

// A Hub on the bus takes an open as exactly one cmd: no qs probe, no qs call
// and no second instance.
func TestHubOpenWarmBusSendsOneCmd(t *testing.T) {
	calls := fakeQs(t, func(context.Context, int) ([]byte, []byte, error) { return nil, nil, nil })
	launches := fakeHubLaunch(t, nil)
	fb := fakeBusClient(t, "hub", 4242, "nav", "users", "hub")
	d := &daemon{wmc: wm.OpenNamed("does-not-exist")}
	for _, tc := range []struct {
		line string
		want busCmd
	}{
		{"hub open appearance", busCmd{Target: "nav", Fn: "open", Args: []string{"appearance"}}},
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

// A cold open starts the Hub once, waits for it to serve nav on the bus, and
// only then sends the navigation; the qs probe is the only qs it runs.
func TestHubOpenColdStartsOnceThenSendsOnBus(t *testing.T) {
	calls := fakeQs(t, func(context.Context, int) ([]byte, []byte, error) {
		return nil, []byte("No running instances for this config.\n"), nil
	})
	prevWait := hubStartWait
	hubStartWait = 3 * time.Second
	t.Cleanup(func() { hubStartWait = prevWait })
	registered := make(chan *fakeBus, 1)
	launches := fakeHubLaunch(t, func() {
		time.Sleep(100 * time.Millisecond)
		registered <- fakeBusClient(t, "hub", 4343, "nav", "users", "hub")
	})
	d := &daemon{wmc: wm.OpenNamed("does-not-exist")}
	if got := d.dispatch("hub open appearance"); got != "ok" {
		t.Fatalf("dispatch = %q, want ok", got)
	}
	var fb *fakeBus
	select {
	case fb = <-registered:
	case <-time.After(2 * time.Second):
		t.Fatal("the cold open never launched the Hub")
	}
	hubWaitFor(t, func() bool { return len(fb.cmds()) >= 1 })
	time.Sleep(50 * time.Millisecond)
	want := busCmd{Target: "nav", Fn: "open", Args: []string{"appearance"}}
	if cmds := fb.cmds(); len(cmds) != 1 || !sameBusCmd(cmds[0], want) {
		t.Fatalf("bus cmds = %+v, want one %+v", cmds, want)
	}
	probe := [][]string{{"-c", "hub", "ipc", "call", "nav", "section"}}
	if got := calls(); !reflect.DeepEqual(got, probe) {
		t.Fatalf("qs calls = %q, want only the probe %q", got, probe)
	}
	if n := launches(); n != 1 {
		t.Fatalf("launches = %d, want 1", n)
	}
}

// `hub close` reaches a bus Hub as one cmd.
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

// fakeHubLaunch replaces the flock qs start: it counts launches, runs start
// (the fake Hub coming up) in the background, and blocks the returned wait
// until the test ends.
func fakeHubLaunch(t *testing.T, start func()) func() int {
	t.Helper()
	prev := hubLaunch
	done := make(chan struct{})
	var mu sync.Mutex
	n := 0
	hubLaunch = func() (func() error, error) {
		mu.Lock()
		n++
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
	return func() int {
		mu.Lock()
		defer mu.Unlock()
		return n
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
