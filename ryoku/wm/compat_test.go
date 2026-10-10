package wm

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestCompatibilityDetectsForeignAPIs(t *testing.T) {
	dir := t.TempDir()
	writeCompatFixture(t, dir, "Main.qml", "import Quickshell.Hyprland\n")
	writeCompatFixture(t, dir, "reload.sh", "#!/bin/sh\nhyprctl reload\n")
	writeCompatFixture(t, dir, "events.py", "sock = runtime / '.socket2.sock'\nsig = 'HYPRLAND_INSTANCE_SIGNATURE'\n")

	got, err := CompatibilityFor(dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	want := []ForeignAPI{ForeignAPIHyprctl, ForeignAPIHyprlandIPC, ForeignAPIQuickshellHyprland}
	if !reflect.DeepEqual(got.Requires, want) {
		t.Fatalf("requires = %v, want %v", got.Requires, want)
	}
	if got.OK {
		t.Fatal("plugin requiring unavailable interfaces was accepted")
	}
	wantReason := "running window manager does not provide: hyprctl, hyprland-ipc, quickshell-hyprland"
	if got.Reason != wantReason {
		t.Fatalf("reason = %q, want %q", got.Reason, wantReason)
	}
}

func TestCompatibilityDetectsEachIPCPattern(t *testing.T) {
	for _, source := range []string{
		`const signature = "HYPRLAND_INSTANCE_SIGNATURE";`,
		`const commandSocket = ".socket.sock";`,
		`const eventSocket = ".socket2.sock";`,
	} {
		dir := t.TempDir()
		writeCompatFixture(t, dir, "ipc.js", source)
		got, err := CompatibilityFor(dir, nil)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(got.Requires, []ForeignAPI{ForeignAPIHyprlandIPC}) {
			t.Fatalf("source %q requires %v", source, got.Requires)
		}
	}
}

func TestCompatibilityAllowsServedAPIs(t *testing.T) {
	dir := t.TempDir()
	writeCompatFixture(t, dir, "Main.qml", "import Quickshell.Hyprland\n")

	got, err := CompatibilityFor(dir, []ForeignAPI{ForeignAPIQuickshellHyprland})
	if err != nil {
		t.Fatal(err)
	}
	if !got.OK || got.Reason != "" {
		t.Fatalf("served interface was blocked: %+v", got)
	}
	if !reflect.DeepEqual(got.Requires, []ForeignAPI{ForeignAPIQuickshellHyprland}) {
		t.Fatalf("requires = %v", got.Requires)
	}
	raw, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != `{"ok":true,"requires":["quickshell-hyprland"],"reason":""}` {
		t.Fatalf("JSON contract = %s", raw)
	}
}

func TestCompatibilityReasonNamesOnlyMissingAPIs(t *testing.T) {
	dir := t.TempDir()
	writeCompatFixture(t, dir, "Main.qml", "import Quickshell.Hyprland\n")
	writeCompatFixture(t, dir, "reload.sh", "hyprctl reload\n")

	got, err := CompatibilityFor(dir, []ForeignAPI{ForeignAPIHyprctl})
	if err != nil {
		t.Fatal(err)
	}
	if got.OK || got.Reason != "running window manager does not provide: quickshell-hyprland" {
		t.Fatalf("partial compatibility = %+v", got)
	}
}

func TestCompatibilityScansExecutableWithoutKnownExtension(t *testing.T) {
	dir := t.TempDir()
	path := writeCompatFixture(t, dir, "reload", "#!/bin/sh\nhyprctl reload\n")
	if err := os.Chmod(path, 0o755); err != nil {
		t.Fatal(err)
	}

	got, err := CompatibilityFor(dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.Requires, []ForeignAPI{ForeignAPIHyprctl}) {
		t.Fatalf("requires = %v", got.Requires)
	}
}

func TestCompatibilityIgnoresUnloadedFileKinds(t *testing.T) {
	dir := t.TempDir()
	writeCompatFixture(t, dir, "notes.txt", "hyprctl reload\n")

	got, err := CompatibilityFor(dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !got.OK || len(got.Requires) != 0 {
		t.Fatalf("non-source file affected compatibility: %+v", got)
	}
	raw, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != `{"ok":true,"requires":[],"reason":""}` {
		t.Fatalf("empty JSON contract = %s", raw)
	}
}

func TestCompatibilityRejectsSymlinkEscape(t *testing.T) {
	dir := t.TempDir()
	outside := filepath.Join(t.TempDir(), "outside.qml")
	if err := os.WriteFile(outside, []byte("import Quickshell.Hyprland\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(dir, "Main.qml")); err != nil {
		t.Fatal(err)
	}

	_, err := CompatibilityFor(dir, nil)
	if err == nil || !strings.Contains(err.Error(), "symlink escapes plugin directory") {
		t.Fatalf("symlink escape error = %v", err)
	}
}

func TestCompatibilityFollowsInternalSymlinkOnce(t *testing.T) {
	dir := t.TempDir()
	target := writeCompatFixture(t, dir, "source.qml", "import Quickshell.Hyprland\n")
	if err := os.Symlink(target, filepath.Join(dir, "Main.qml")); err != nil {
		t.Fatal(err)
	}

	got, err := CompatibilityFor(dir, []ForeignAPI{ForeignAPIQuickshellHyprland})
	if err != nil {
		t.Fatal(err)
	}
	if !got.OK || !reflect.DeepEqual(got.Requires, []ForeignAPI{ForeignAPIQuickshellHyprland}) {
		t.Fatalf("internal symlink scan = %+v", got)
	}
}

func TestCompatibilityRejectsOversizedSource(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Main.qml")
	if err := os.WriteFile(path, make([]byte, compatMaxFileBytes+1), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := CompatibilityFor(dir, nil)
	if err == nil || !strings.Contains(err.Error(), "source exceeds") {
		t.Fatalf("oversized source error = %v", err)
	}
}

func writeCompatFixture(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}
