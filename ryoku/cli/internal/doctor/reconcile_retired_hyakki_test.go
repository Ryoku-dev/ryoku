package doctor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"ryoku-cli/internal/host"
)

type retiredHyakkiBox struct {
	home      string
	installed map[string]bool
	active    map[string]bool
	removed   []string
	stopped   []string
	killed    []string
}

func newRetiredHyakkiBox(t *testing.T) *retiredHyakkiBox {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(home, ".local", "state"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, ".local", "share"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(home, ".cache"))
	box := &retiredHyakkiBox{home: home, installed: map[string]bool{}, active: map[string]bool{}}
	oldService, oldKill, oldInstalled, oldRemove, oldNow, oldVault :=
		retiredHyakkiService, retiredHyakkiKill, retiredHyakkiInstalled, retiredHyakkiRemove, retiredHyakkiNow, retiredHyakkiVaultLeft
	t.Cleanup(func() {
		retiredHyakkiService, retiredHyakkiKill, retiredHyakkiInstalled, retiredHyakkiRemove, retiredHyakkiNow, retiredHyakkiVaultLeft =
			oldService, oldKill, oldInstalled, oldRemove, oldNow, oldVault
	})
	retiredHyakkiService = func(args []string) int {
		switch {
		case len(args) == 3 && (args[1] == "is-active" || args[1] == "is-enabled"):
			if box.active[args[2]] {
				return host.ExitOK
			}
			return host.ExitFalse
		case len(args) == 4 && args[1] == "disable":
			box.stopped = append(box.stopped, args[3])
			delete(box.active, args[3])
		}
		return host.ExitOK
	}
	retiredHyakkiKill = func(name string) { box.killed = append(box.killed, name) }
	retiredHyakkiInstalled = func(pkg string) bool { return box.installed[pkg] }
	retiredHyakkiRemove = func(pkg string) error {
		box.removed = append(box.removed, pkg)
		delete(box.installed, pkg)
		return nil
	}
	retiredHyakkiNow = func() time.Time { return time.Date(2026, 11, 26, 9, 30, 0, 0, time.UTC) }
	retiredHyakkiVaultLeft = func() bool { return false }
	return box
}

func (b *retiredHyakkiBox) write(t *testing.T, rel, body string) {
	t.Helper()
	path := filepath.Join(b.home, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestRetiredHyakkiMovesEverythingAsideOnce(t *testing.T) {
	box := newRetiredHyakkiBox(t)
	box.installed["hyakki"] = true
	box.active["oni.socket"] = true
	box.write(t, ".config/hyakki/hyakki.json", `{"enabled":true}`)
	box.write(t, ".local/state/hyakki/sessions/a/engine.jsonl", "turn\n")
	box.write(t, ".cache/hyakki/index/x", "cache")
	box.write(t, ".local/share/hyakki/oni/voice/models/m.bin", "model")
	box.write(t, ".config/ryoku/rashin.json", "{}")
	box.write(t, ".local/share/ryoku/rashin/memory.md", "remember")
	box.write(t, ".local/bin/oni", "#!/bin/sh\n")
	box.write(t, ".config/systemd/user/oni.service", "[Service]\n")
	box.write(t, ".config/systemd/user/oni.service.d/ryoku-dev-bin.conf", "[Service]\n")
	box.write(t, ".config/service/oni-routines/run", "#!/bin/sh\n")
	box.write(t, ".config/ryoku/shell.json", "{}")

	if r := reconcileRetiredHyakki(true); r.status != recWouldFix {
		t.Fatalf("check: status=%s detail=%q, want would-fix", r.status.label(), r.detail)
	}
	if _, err := os.Stat(filepath.Join(box.home, ".config/hyakki/hyakki.json")); err != nil {
		t.Fatalf("check-only moved data: %v", err)
	}

	r := reconcileRetiredHyakki(false)
	if r.status != recFixed {
		t.Fatalf("fix: status=%s detail=%q, want fixed", r.status.label(), r.detail)
	}
	archive := filepath.Join(box.home, ".local/share/ryoku/retired/hyakki-rashin-20261126-093000")
	for _, rel := range []string{
		".config/hyakki/hyakki.json",
		".local/state/hyakki/sessions/a/engine.jsonl",
		".cache/hyakki/index/x",
		".local/share/hyakki/oni/voice/models/m.bin",
		".config/ryoku/rashin.json",
		".local/share/ryoku/rashin/memory.md",
		".local/bin/oni",
		".config/systemd/user/oni.service",
		".config/systemd/user/oni.service.d/ryoku-dev-bin.conf",
		".config/service/oni-routines/run",
	} {
		if _, err := os.Stat(filepath.Join(archive, rel)); err != nil {
			t.Errorf("%s was not kept in the archive: %v", rel, err)
		}
		if _, err := os.Lstat(filepath.Join(box.home, rel)); !os.IsNotExist(err) {
			t.Errorf("%s is still in the home: %v", rel, err)
		}
	}
	if _, err := os.Stat(filepath.Join(archive, "README.txt")); err != nil {
		t.Errorf("archive has no README: %v", err)
	}
	if _, err := os.Stat(filepath.Join(box.home, ".config/ryoku/shell.json")); err != nil {
		t.Errorf("an unrelated Ryoku file was moved: %v", err)
	}
	if strings.Join(box.removed, ",") != "hyakki" {
		t.Errorf("removed packages = %v, want hyakki", box.removed)
	}
	if strings.Join(box.stopped, ",") != "oni.socket" {
		t.Errorf("stopped units = %v, want oni.socket", box.stopped)
	}
	if len(box.killed) == 0 {
		t.Error("left-over processes were not stopped")
	}

	// A Hyakki installed again on purpose afterwards is left alone.
	box.installed["hyakki"] = true
	box.write(t, ".config/hyakki/hyakki.json", `{"enabled":true}`)
	box.removed = nil
	if r := reconcileRetiredHyakki(false); r.status != recOK {
		t.Fatalf("second run: status=%s detail=%q, want ok", r.status.label(), r.detail)
	}
	if len(box.removed) != 0 {
		t.Errorf("second run removed %v", box.removed)
	}
	if _, err := os.Stat(filepath.Join(box.home, ".config/hyakki/hyakki.json")); err != nil {
		t.Errorf("second run moved a new install: %v", err)
	}
}

func TestRetiredHyakkiLeavesAnUnmigratedVault(t *testing.T) {
	box := newRetiredHyakkiBox(t)
	retiredHyakkiVaultLeft = func() bool { return true }
	box.write(t, ".local/share/ryoku/rashin/AGENTS.md", "vault")
	box.write(t, ".config/hyakki/hyakki.json", "{}")
	if r := reconcileRetiredHyakki(false); r.status != recFixed {
		t.Fatalf("status=%s detail=%q, want fixed", r.status.label(), r.detail)
	}
	if _, err := os.Stat(filepath.Join(box.home, ".local/share/ryoku/rashin/AGENTS.md")); err != nil {
		t.Errorf("the Rashin 1 vault left before the vault took it in: %v", err)
	}
}

func TestRetiredHyakkiNothingLeftIsQuiet(t *testing.T) {
	newRetiredHyakkiBox(t)
	if r := reconcileRetiredHyakki(true); r.status != recOK {
		t.Fatalf("check: status=%s, want ok", r.status.label())
	}
	if r := reconcileRetiredHyakki(false); r.status != recOK {
		t.Fatalf("fix: status=%s, want ok", r.status.label())
	}
	if _, err := os.Stat(retiredHyakkiMarker()); err != nil {
		t.Fatalf("marker not written: %v", err)
	}
}

func TestMoveRetiredCopiesWhenRenameCannot(t *testing.T) {
	src := filepath.Join(t.TempDir(), "a")
	if err := os.MkdirAll(filepath.Join(src, "b"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "b", "f"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("b/f", filepath.Join(src, "l")); err != nil {
		t.Fatal(err)
	}
	dst := filepath.Join(t.TempDir(), "kept", "a")
	if err := os.MkdirAll(filepath.Dir(dst), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := copyRetired(src, dst); err != nil {
		t.Fatal(err)
	}
	if body, err := os.ReadFile(filepath.Join(dst, "b", "f")); err != nil || string(body) != "x" {
		t.Fatalf("copied file = %q, %v", body, err)
	}
	if link, err := os.Readlink(filepath.Join(dst, "l")); err != nil || link != "b/f" {
		t.Fatalf("copied link = %q, %v", link, err)
	}
}
