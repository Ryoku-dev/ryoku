package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	wm "ryoku-wm"
)

// two providers, a private config, and a host seam that reports the target's
// package installed when asked to
func pendingFixture(t *testing.T, targetInstalled bool) (string, string) {
	t.Helper()
	providers := wm.Providers()
	if len(providers) < 2 {
		t.Fatalf("need two window-manager providers, got %d", len(providers))
	}
	active, target := providers[0], providers[1]
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	bin := t.TempDir()
	host := "#!/bin/sh\nexit 1\n"
	if targetInstalled {
		host = "#!/bin/sh\n[ \"$1:$2:$3\" = \"pkg:installed:" + wmPackage(target) + "\" ] && exit 0\nexit 1\n"
	}
	if err := os.WriteFile(filepath.Join(bin, "ryoku-host"), []byte(host), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	return active, target
}

func TestWMPendingWaitsForReadyTarget(t *testing.T) {
	active, target := pendingFixture(t, false)
	t.Setenv("RYOKU_WM", active)
	if err := setWMSwitch(target); err != nil {
		t.Fatal(err)
	}
	if p := wmPending(); p != nil {
		t.Fatalf("pending before the target is ready = %#v", p)
	}
	if loadConfig().UI.WMSwitch != target {
		t.Fatal("an install in progress must keep the remembered switch")
	}
}

func TestWMPendingInstalledTarget(t *testing.T) {
	active, target := pendingFixture(t, true)
	t.Setenv("RYOKU_WM", active)
	if err := setWMSwitch(target); err != nil {
		t.Fatal(err)
	}
	p := wmPending()
	if p == nil || p.Target != target || len(p.Args) != 1 || p.Args[0] != strings.ToUpper(target[:1])+target[1:] {
		t.Fatalf("pending = %#v", p)
	}
}

func TestWMPendingClearsOnceRunning(t *testing.T) {
	_, target := pendingFixture(t, true)
	t.Setenv("RYOKU_WM", target)
	if err := setWMSwitch(target); err != nil {
		t.Fatal(err)
	}
	if p := wmPending(); p != nil {
		t.Fatalf("pending while the target runs = %#v", p)
	}
	if loadConfig().UI.WMSwitch != "" {
		t.Fatal("the stale switch was not cleared")
	}
}

func TestWMPendingCommand(t *testing.T) {
	_, target := pendingFixture(t, false)
	if err := wmPendingCmd([]string{"set", "no-such-wm"}); err == nil {
		t.Fatal("an unknown window manager was accepted")
	}
	if err := wmPendingCmd([]string{"set", target}); err != nil {
		t.Fatal(err)
	}
	if err := wmPendingCmd([]string{"clear"}); err != nil {
		t.Fatal(err)
	}
	if loadConfig().UI.WMSwitch != "" {
		t.Fatal("clear left the switch")
	}
	out := captureStdout(t, func() { _ = wmPendingCmd(nil) })
	if strings.TrimSpace(out) != "null" {
		t.Fatalf("nothing pending printed %q", out)
	}
}
