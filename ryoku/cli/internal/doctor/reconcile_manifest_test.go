package doctor

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ryoku-cli/internal/ryokumanifest"
	"ryoku-cli/internal/sys"
	"ryoku-cli/internal/updater"
)

func TestManifestConvergedCheckAndApply(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(home, ".local/state"))
	t.Setenv("RYOKU_REPO", "")
	t.Setenv("RYOKU_CHANNEL", "")
	t.Setenv("RYOKU_HOST_PKGMGR", "pacman")
	t.Setenv("RYOKU_OS_RELEASE", filepath.Join(home, "os-release"))
	oldConf, oldInstalled, oldPacman := sys.PacmanConf, updater.PacmanInstalled, hasPacman
	t.Cleanup(func() {
		sys.PacmanConf, updater.PacmanInstalled, hasPacman = oldConf, oldInstalled, oldPacman
	})
	sys.PacmanConf = filepath.Join(home, "pacman.conf")
	if err := os.WriteFile(sys.PacmanConf, []byte("[ryoku]\nServer = "+sys.ChannelServer("stable")+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	hasPacman = func() bool { return true }
	installed := map[string]bool{"base": true, "yay": true}
	updater.PacmanInstalled = func() (map[string]bool, error) { return installed, nil }
	bin := filepath.Join(home, "bin")
	if err := os.MkdirAll(bin, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bin, "pacman"), []byte("#!/bin/sh\n[ \"$1\" = -T ] || exit 2\nshift\nfor n; do [ \"$n\" = yay ] || { echo \"$n\"; exit 127; }; done\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	manifest := ryokumanifest.Manifest{Schema: ryokumanifest.Schema, Release: "v1.0.9", Base: []string{"base"}, AUR: []string{"yay-bin"}}
	body, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	cache := filepath.Join(sys.StateDir(), "release-cache")
	if err := os.MkdirAll(cache, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cache, "manifest-stable.json"), body, 0o600); err != nil {
		t.Fatal(err)
	}
	if got := reconcileManifest(true); got.status != recWouldFix {
		t.Fatalf("first baseline check = %+v", got)
	}
	if got := reconcileManifest(false); got.status != recOK {
		t.Fatalf("record baseline = %+v", got)
	}
	for _, check := range []bool{true, false, true} {
		got := reconcileManifest(check)
		t.Logf("fake HOME doctor check=%t: %s: %s", check, got.status.label(), got.detail)
		if got.status != recOK || got.remedy != "" || !strings.Contains(got.detail, "yay-bin") {
			t.Fatalf("converged = %+v", got)
		}
	}
	delete(installed, "base")
	if err := updater.SaveApplied(manifest, []string{"yay-bin"}); err != nil {
		t.Fatal(err)
	}
	if got := reconcileManifest(true); got.status != recWouldFix || got.remedy == "" {
		t.Fatalf("missing package = %+v", got)
	}
	if err := updater.SaveApplied(manifest, []string{"base", "yay-bin"}); err != nil {
		t.Fatal(err)
	}
	if got := reconcileManifest(true); got.status != recOK {
		t.Fatalf("deliberate user removal = %+v", got)
	}
}
