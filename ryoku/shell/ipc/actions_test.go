package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeCommandStub(t *testing.T, dir, name, body string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+body+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
}

func useSystemdRuntime(t *testing.T, path string) {
	t.Helper()
	old := systemdRuntimeDir
	systemdRuntimeDir = path
	t.Cleanup(func() { systemdRuntimeDir = old })
}

func TestSystemdInitRequiresRuntimeDirectory(t *testing.T) {
	dir := t.TempDir()
	useSystemdRuntime(t, filepath.Join(dir, "missing"))
	if systemdInit() {
		t.Fatal("missing runtime path reported systemd")
	}
	if err := os.WriteFile(systemdRuntimeDir, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if systemdInit() {
		t.Fatal("runtime file reported systemd")
	}
	if err := os.Remove(systemdRuntimeDir); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(systemdRuntimeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if !systemdInit() {
		t.Fatal("runtime directory did not report systemd")
	}
}

func TestUserServiceSystemdCommandsStayUnchanged(t *testing.T) {
	dir := t.TempDir()
	runtimeDir := filepath.Join(dir, "systemd")
	if err := os.Mkdir(runtimeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	useSystemdRuntime(t, runtimeDir)
	logPath := filepath.Join(dir, "calls")
	bin := filepath.Join(dir, "bin")
	if err := os.Mkdir(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	writeCommandStub(t, bin, "systemctl", `printf '%s\n' "$*" >>"$CALL_LOG"`)
	t.Setenv("CALL_LOG", logPath)
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))

	if !userServiceActive("voxtype.service") {
		t.Fatal("active systemd service reported inactive")
	}
	if err := stopUserService("guard.service"); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	want := "--user is-active --quiet voxtype.service\n--user stop guard.service\n"
	if string(got) != want {
		t.Fatalf("systemctl calls = %q, want %q", got, want)
	}
}

func TestUserServiceRunitCommandsAndMissingService(t *testing.T) {
	dir := t.TempDir()
	useSystemdRuntime(t, filepath.Join(dir, "no-systemd"))
	home := filepath.Join(dir, "home")
	service := filepath.Join(home, ".config", "service", "voxtype")
	if err := os.MkdirAll(service, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	logPath := filepath.Join(dir, "calls")
	bin := filepath.Join(dir, "bin")
	if err := os.Mkdir(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	writeCommandStub(t, bin, "sv", `printf '%s\n' "$*" >>"$CALL_LOG"`)
	t.Setenv("CALL_LOG", logPath)
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))

	if !userServiceActive("voxtype.service") {
		t.Fatal("active runit service reported inactive")
	}
	if err := stopUserService("voxtype.service"); err != nil {
		t.Fatal(err)
	}
	if userServiceActive("absent.service") {
		t.Fatal("missing runit service reported active")
	}
	if err := stopUserService("absent.service"); err != nil {
		t.Fatalf("missing runit service stop returned %v", err)
	}
	got, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(got)), "\n")
	want := []string{"check " + service, "down " + service}
	if len(lines) != len(want) {
		t.Fatalf("sv calls = %q, want %q", lines, want)
	}
	for i := range want {
		if lines[i] != want[i] {
			t.Errorf("sv call %d = %q, want %q", i, lines[i], want[i])
		}
	}
}
