package main

import (
	"bufio"
	"bytes"
	"strings"
	"testing"
)

// The entry is the seam's detection fallback: a greeter builds
// XDG_CURRENT_DESKTOP from DesktopNames, and detect matches its "wayfire"
// component. Pin the bytes so a tidy-up cannot silently detach detection or
// point the greeter at a binary that does not exist.
func TestRunSessionPrintsTheGreeterEntry(t *testing.T) {
	var output bytes.Buffer
	old := stdout
	stdout = bufio.NewWriter(&output)
	t.Cleanup(func() { stdout = old })

	if err := runSession(); err != nil {
		t.Fatal(err)
	}
	if err := stdout.Flush(); err != nil {
		t.Fatal(err)
	}
	got := output.String()
	for _, want := range []string{
		"[Desktop Entry]",
		"Exec=wayfire",
		"TryExec=wayfire",
		"DesktopNames=Wayfire;wlroots",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("session entry lacks %q:\n%s", want, got)
		}
	}
}
