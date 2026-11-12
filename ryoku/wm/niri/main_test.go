package main

import (
	"os"
	"testing"
)

// The package's tests run against a box that has every optional program, so a
// runner without Rashin cannot change what the fixtures expect, and an apply
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
