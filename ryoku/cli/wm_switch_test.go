package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"ryoku-cli/internal/updater"
)

func installFakeWmHost(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	logPath := filepath.Join(dir, "calls")
	body := `#!/bin/sh
printf '%s\n' "$*" >> "$RYOKU_HOST_LOG"
case "$2" in
  available)
    [ "$RYOKU_HOST_AVAILABLE" = "1" ]
    ;;
  why)
    if [ -n "$RYOKU_HOST_REASON" ]; then
      printf '%s\n' "$RYOKU_HOST_REASON"
      exit 0
    fi
    exit 3
    ;;
  installed)
    exit 0
    ;;
  *)
    exit 64
    ;;
esac
`
	if err := os.WriteFile(filepath.Join(dir, "ryoku-host"), []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("RYOKU_HOST_LOG", logPath)
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return logPath
}

func TestTargetPackageAvailabilityReportsHostReason(t *testing.T) {
	logPath := installFakeWmHost(t)
	reason := "Ryoku's Hyprland needs GCC 15 and Void still ships GCC 14."
	t.Setenv("RYOKU_HOST_AVAILABLE", "0")
	t.Setenv("RYOKU_HOST_REASON", reason)

	err := targetPackageAvailability("hyprland", "ryoku-desktop-hyprland")
	if err == nil {
		t.Fatal("unavailable target was accepted")
	}
	if !strings.Contains(err.Error(), reason) {
		t.Fatalf("refusal = %q, want host reason %q", err, reason)
	}
	calls, readErr := os.ReadFile(logPath)
	if readErr != nil {
		t.Fatal(readErr)
	}
	want := "pkg available ryoku-desktop-hyprland\npkg why ryoku-desktop-hyprland\n"
	if string(calls) != want {
		t.Fatalf("host calls = %q, want %q", calls, want)
	}
}

func TestWmSwitchPackageChecksAndInstallUseHostSeam(t *testing.T) {
	logPath := installFakeWmHost(t)
	t.Setenv("RYOKU_HOST_AVAILABLE", "1")

	if err := targetPackageAvailability("niri", "ryoku-desktop-niri"); err != nil {
		t.Fatalf("available target refused: %v", err)
	}
	if !packageInstalled("ryoku-desktop-current") {
		t.Fatal("host-installed package was not detected")
	}
	calls, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	wantCalls := "pkg available ryoku-desktop-niri\npkg installed ryoku-desktop-current\n"
	if string(calls) != wantCalls {
		t.Fatalf("host calls = %q, want %q", calls, wantCalls)
	}
	wantInstall := []string{
		"ryoku-host", "pkg", "install", "--overwrite", updater.RyokuOverwriteGlob, "ryoku-desktop-niri",
	}
	if got := wmSwitchInstallArgs("ryoku-desktop-niri"); !reflect.DeepEqual(got, wantInstall) {
		t.Fatalf("switch install = %q, want %q", got, wantInstall)
	}
}
