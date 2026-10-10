package wm

import (
	"os"
	"path/filepath"
)

// PublishGreeterNumlock records the session's effective numlock state where
// the login greeter can read it. The greeter runs as another user before any
// session exists, so it cannot see desktop.json; without the hand-off it
// forces the keypad on for everyone, and the LED state it sets at login
// carries into the session and its lock screen, garbling passwords typed on a
// compact keyboard whose letter keys the keypad overlays (issue #321). The
// greeter and the session then agree: login behaves exactly the way the
// session boots. Before the first apply (a fresh install's first login, where
// a PIN must reach the field as digits) there is no hand-off and the greeter
// keeps its keypad-on default. Best-effort: the file is created by tmpfiles
// on a packaged box, and a checkout without it simply keeps that default.
func PublishGreeterNumlock(on bool) {
	value := "off\n"
	if on {
		value = "on\n"
	}
	_ = os.WriteFile(GreeterNumlockPath(), []byte(value), 0o666)
}

// GreeterNumlockPath is the hand-off file: RYOKU_GREETER_NUMLOCK_FILE for
// tests and nested sessions, the shared state dir otherwise.
func GreeterNumlockPath() string {
	if p := os.Getenv("RYOKU_GREETER_NUMLOCK_FILE"); p != "" {
		return p
	}
	return filepath.Join(greeterStateDir(), "greeter-numlock")
}

func greeterStateDir() string {
	if st := os.Getenv("RYOKU_GREETER_STATE_DIR"); st != "" {
		return st
	}
	return "/var/lib/ryoku"
}
