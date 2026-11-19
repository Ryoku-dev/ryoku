package main

import (
	"os"
	"reflect"
	"strings"
	"testing"
)

// parseBarEdge is the whole grammar of `bar <edge|all> <toggle|reveal|hide>`; a
// wrong edge or action must not reach the frame, and a missing one must not be
// silently accepted as a default.
func TestParseBarEdge(t *testing.T) {
	for _, edge := range []string{"top", "bottom", "left", "right", "all"} {
		for _, action := range []string{"toggle", "reveal", "hide"} {
			e, a, ok := parseBarEdge([]string{edge, action})
			if !ok || e != edge || a != action {
				t.Errorf("parseBarEdge(%s %s) = (%q,%q,%v), want (%q,%q,true)", edge, action, e, a, ok, edge, action)
			}
		}
	}
	for _, args := range [][]string{
		{},
		{"left"},
		{"left", "toggle", "extra"},
		{"sideways", "toggle"},
		{"left", "sideways"},
		{"toggle", "left"},
	} {
		if _, _, ok := parseBarEdge(args); ok {
			t.Errorf("parseBarEdge(%v) accepted, want rejection", args)
		}
	}
}

// The bar's wheel has to land where the key lands, so the volume verbs run the
// same helper the media keys run, and mute stays a direct sink toggle.
func TestAudioCmd(t *testing.T) {
	cases := map[string]struct {
		name string
		argv []string
	}{
		"up":   {"ryoku-volume", []string{"up"}},
		"down": {"ryoku-volume", []string{"down"}},
		"mute": {"wpctl", []string{"set-mute", "@DEFAULT_AUDIO_SINK@", "toggle"}},
	}
	for sub, want := range cases {
		name, argv, ok := audioCmd(sub)
		if !ok || name != want.name || !reflect.DeepEqual(argv, want.argv) {
			t.Errorf("audioCmd(%q) = (%q,%v,%v), want (%q,%v,true)", sub, name, argv, ok, want.name, want.argv)
		}
	}
	if _, _, ok := audioCmd("sideways"); ok {
		t.Errorf("audioCmd(sideways) accepted, want rejection")
	}
}

// The brightness verb hands the helper its word, so the helper's own setting and
// clamping are the only ones in play.
func TestBrightnessArgv(t *testing.T) {
	for sub, want := range map[string][]string{"up": {"up"}, "down": {"down"}} {
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
		"bar":                             "err bar",
		"bar left":                        "err bar",
		"bar sideways toggle":             "err bar",
		"bar left sideways":               "err bar",
		"audio":                           "err audio",
		"audio sideways":                  "err audio",
		"brightness":                      "err brightness",
		"brightness sideways":             "err brightness",
		"hub":                             "err hub",
		"hub bogus":                       "err hub",
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
