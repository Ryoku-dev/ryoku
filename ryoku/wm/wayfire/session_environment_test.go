package main

import (
	"bufio"
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestRunEnvironmentExportsOnlyProviderHandle(t *testing.T) {
	root := t.TempDir()
	pidDir := filepath.Join(root, "42")
	if err := os.Mkdir(pidDir, 0o700); err != nil {
		t.Fatal(err)
	}
	body := []byte("XDG_SESSION_ID=9\x00WAYFIRE_SOCKET=/run/user/1000/wayfire-wayland-1-.socket\x00SECRET=nope\x00")
	if err := os.WriteFile(filepath.Join(pidDir, "environ"), body, 0o600); err != nil {
		t.Fatal(err)
	}

	oldRoot, oldOut := sessionEnvironmentRoot, stdout
	sessionEnvironmentRoot = root
	var output bytes.Buffer
	stdout = bufio.NewWriter(&output)
	t.Cleanup(func() {
		sessionEnvironmentRoot = oldRoot
		stdout = oldOut
	})

	if err := runEnvironment([]string{"42"}); err != nil {
		t.Fatal(err)
	}
	if err := stdout.Flush(); err != nil {
		t.Fatal(err)
	}
	if got, want := output.String(), "WAYFIRE_SOCKET=/run/user/1000/wayfire-wayland-1-.socket\x00"; got != want {
		t.Fatalf("environment output = %q, want %q", got, want)
	}
}

func TestRunEnvironmentArgumentErrors(t *testing.T) {
	if err := runEnvironment(nil); err == nil {
		t.Error("no arguments must fail")
	}
	if err := runEnvironment([]string{"pid"}); err == nil {
		t.Error("a non-numeric pid must fail")
	}
}
