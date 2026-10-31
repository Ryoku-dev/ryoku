package main

import (
	"bufio"
	"bytes"
	"net"
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
	body := []byte("XDG_SESSION_ID=9\x00HYPRLAND_INSTANCE_SIGNATURE=session-nine\x00SECRET=nope\x00")
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
	if got, want := output.String(), "HYPRLAND_INSTANCE_SIGNATURE=session-nine\x00"; got != want {
		t.Fatalf("environment output = %q, want %q", got, want)
	}
}

func TestRunCtlRebindsStaleInstanceSignature(t *testing.T) {
	runtimeDir := t.TempDir()
	t.Setenv("XDG_RUNTIME_DIR", runtimeDir)
	t.Setenv("HYPRLAND_INSTANCE_SIGNATURE", "dead-instance")

	socketDir := filepath.Join(runtimeDir, "hypr", "live-instance")
	if err := os.MkdirAll(socketDir, 0o700); err != nil {
		t.Fatal(err)
	}
	listener, err := net.Listen("unix", filepath.Join(socketDir, ".socket.sock"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })

	bin := t.TempDir()
	hyprctl := filepath.Join(bin, "hyprctl")
	if err := os.WriteFile(hyprctl, []byte("#!/bin/sh\nprintf '%s\\n' \"$HYPRLAND_INSTANCE_SIGNATURE\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)

	out, err := runCtl("version", "-j")
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(out), "live-instance\n"; got != want {
		t.Fatalf("hyprctl signature = %q, want %q", got, want)
	}
}
