package main

import (
	"os"
	"testing"
)

// The package's tests pin the optional-program lookup, so a runner missing an
// optional app cannot change what the fixtures expect, and an apply
// publishes the greeter's numlock into a private folder instead of the real
// /var/lib/ryoku.
func TestMain(m *testing.M) {
	hasProgram = func(string) bool { return true }
	dir, err := os.MkdirTemp("", "ryoku-greeter-")
	if err != nil {
		panic(err)
	}
	os.Setenv("RYOKU_GREETER_STATE_DIR", dir)
	os.Unsetenv("RYOKU_GREETER_NUMLOCK_FILE")
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}
