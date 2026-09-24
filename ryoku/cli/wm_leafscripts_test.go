package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// checkoutRepo makes a throw-away git work tree that ResolveRepo will accept,
// with a hyprland scripts dir holding two leaf scripts and one non-ryoku file.
func checkoutRepo(t *testing.T) string {
	t.Helper()
	repo := t.TempDir()
	if out, err := exec.Command("git", "-C", repo, "init").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, out)
	}
	scripts := filepath.Join(repo, "ryoku", "hyprland", "scripts")
	if err := os.MkdirAll(scripts, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"ryoku-monitor", "ryoku-workspace", "magick-policy"} {
		if err := os.WriteFile(filepath.Join(scripts, name), []byte("#!"+name+"\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return repo
}

func TestSyncLeafScriptsLaysTarget(t *testing.T) {
	repo := checkoutRepo(t)
	home := t.TempDir()
	t.Setenv("RYOKU_REPO", repo)
	t.Setenv("HOME", home)

	if err := syncLeafScripts("hyprland"); err != nil {
		t.Fatalf("syncLeafScripts: %v", err)
	}
	bindir := filepath.Join(home, ".local", "bin")
	for _, name := range []string{"ryoku-monitor", "ryoku-workspace"} {
		st, err := os.Stat(filepath.Join(bindir, name))
		if err != nil {
			t.Fatalf("%s not laid: %v", name, err)
		}
		if st.Mode().Perm()&0o111 == 0 {
			t.Errorf("%s is not executable", name)
		}
	}
	// The glob is ryoku-*, matching deploy.sh; a provider's other payload files
	// are not bare-name executables and must not land on PATH.
	if _, err := os.Stat(filepath.Join(bindir, "magick-policy")); err == nil {
		t.Error("non-ryoku file was laid into the bindir")
	}
}

// A provider that ships no scripts (niri) is a no-op, not an error: the switch
// must not fail just because the target has no leaf scripts to lay.
func TestSyncLeafScriptsNoopWithoutScripts(t *testing.T) {
	repo := checkoutRepo(t)
	home := t.TempDir()
	t.Setenv("RYOKU_REPO", repo)
	t.Setenv("HOME", home)

	if err := syncLeafScripts("niri"); err != nil {
		t.Fatalf("niri sync: %v", err)
	}
	if _, err := os.Stat(filepath.Join(home, ".local", "bin", "ryoku-monitor")); err == nil {
		t.Error("niri laid hyprland's scripts")
	}
}

// On a packaged box there is no checkout to copy from; the scripts ship in
// /usr/bin and a ~/.local/bin copy would shadow them, which is the drift the
// dev-residue doctor heals. The switch must lay nothing.
func TestSyncLeafScriptsSkipsPackagedBox(t *testing.T) {
	home := t.TempDir()
	t.Setenv("RYOKU_REPO", "")
	t.Setenv("HOME", home)
	// ResolveRepo falls back to ~/ryoku-arch only when a channel is recorded;
	// an empty state dir keeps this a packaged box.
	t.Setenv("XDG_STATE_HOME", filepath.Join(home, ".local/state"))

	if err := syncLeafScripts("hyprland"); err != nil {
		t.Fatalf("packaged sync: %v", err)
	}
	if _, err := os.Stat(filepath.Join(home, ".local", "bin")); err == nil {
		t.Error("a packaged box got a ~/.local/bin shadow script")
	}
}

// A stale copy is the actual defect: the June-19 ryoku-monitor had no `apply`
// verb, so every Hub display change failed silently and login recomputed the
// scale. Laying the target's scripts must overwrite what is already there.
func TestSyncLeafScriptsOverwritesStale(t *testing.T) {
	repo := checkoutRepo(t)
	home := t.TempDir()
	t.Setenv("RYOKU_REPO", repo)
	t.Setenv("HOME", home)

	stale := filepath.Join(home, ".local", "bin", "ryoku-monitor")
	if err := os.MkdirAll(filepath.Dir(stale), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(stale, []byte("#!/bin/sh\necho ancient\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := syncLeafScripts("hyprland"); err != nil {
		t.Fatalf("syncLeafScripts: %v", err)
	}
	got, err := os.ReadFile(stale)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) == "#!/bin/sh\necho ancient\n" {
		t.Error("the stale copy survived; the next session still calls it")
	}
}
