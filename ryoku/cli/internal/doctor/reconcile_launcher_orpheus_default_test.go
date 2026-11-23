package doctor

import (
	"os"
	"testing"
)

func TestReconcileLauncherOrpheusDefaultMovesHeroOnce(t *testing.T) {
	configRoot, _ := launcherMigrationRoots(t)
	path := writeLauncherConfig(t, configRoot, `{"variant":"hero","radius":21,"pinnedApps":["firefox"]}`, 0o600)

	if res := reconcileLauncherOrpheusDefault(false); res.status != recFixed {
		t.Fatalf("old default = %s (%s), want fixed", res.status.label(), res.detail)
	}
	got := readTestStore(t, path)
	if got["variant"] != "orpheus" || got["radius"] != 21.0 {
		t.Fatalf("launcher.json = %v, want orpheus with its radius kept", got)
	}
	if pinned, _ := got["pinnedApps"].([]any); len(pinned) != 1 || pinned[0] != "firefox" {
		t.Fatalf("pinned apps changed: %v", got["pinnedApps"])
	}
	if info, err := os.Stat(path); err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("mode not kept: %v %v", info, err)
	}
	if _, err := os.Stat(launcherOrpheusDefaultMarker()); err != nil {
		t.Fatalf("migration not marked: %v", err)
	}

	// picking Hero again after the move has to stick
	writeLauncherConfig(t, configRoot, `{"variant":"hero"}`, 0o600)
	if res := reconcileLauncherOrpheusDefault(false); res.status != recOK {
		t.Fatalf("second run = %s (%s), want ok", res.status.label(), res.detail)
	}
	if got := readTestStore(t, path); got["variant"] != "hero" {
		t.Fatalf("a later choice was overwritten: %v", got)
	}
}

func TestReconcileLauncherOrpheusDefaultKeepsChosenLaunchers(t *testing.T) {
	for _, raw := range []string{`{"variant":"main"}`, `{"variant":"kairos"}`, `{"variant":"Hero"}`, `{"radius":16}`, `{"variant":7}`} {
		t.Run(raw, func(t *testing.T) {
			configRoot, _ := launcherMigrationRoots(t)
			path := writeLauncherConfig(t, configRoot, raw, 0o644)
			if res := reconcileLauncherOrpheusDefault(false); res.status != recOK {
				t.Fatalf("chosen launcher = %s (%s), want ok", res.status.label(), res.detail)
			}
			after, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if string(after) != raw {
				t.Fatalf("a chosen launcher was rewritten: %s", after)
			}
			if _, err := os.Stat(launcherOrpheusDefaultMarker()); err != nil {
				t.Fatalf("nothing to move should still mark: %v", err)
			}
		})
	}
}

func TestReconcileLauncherOrpheusDefaultCheckOnlyWritesNothing(t *testing.T) {
	configRoot, _ := launcherMigrationRoots(t)
	in := `{"variant":"hero"}`
	path := writeLauncherConfig(t, configRoot, in, 0o644)

	if res := reconcileLauncherOrpheusDefault(true); res.status != recWouldFix {
		t.Fatalf("check-only = %s (%s), want would", res.status.label(), res.detail)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != in {
		t.Fatalf("check-only rewrote launcher.json: %s", after)
	}
	if _, err := os.Stat(launcherOrpheusDefaultMarker()); !os.IsNotExist(err) {
		t.Fatalf("check-only wrote the migration marker: %v", err)
	}
}

func TestReconcileLauncherOrpheusDefaultLeavesMalformedFile(t *testing.T) {
	for _, in := range []string{`{"variant":`, `["hero"]`} {
		t.Run(in, func(t *testing.T) {
			configRoot, _ := launcherMigrationRoots(t)
			path := writeLauncherConfig(t, configRoot, in, 0o644)

			if res := reconcileLauncherOrpheusDefault(false); res.status != recWarn {
				t.Fatalf("malformed = %s (%s), want warn", res.status.label(), res.detail)
			}
			after, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if string(after) != in {
				t.Fatalf("malformed launcher.json was rewritten: %s", after)
			}
			if _, err := os.Stat(launcherOrpheusDefaultMarker()); !os.IsNotExist(err) {
				t.Fatalf("a failed migration was marked done: %v", err)
			}
		})
	}
}

func TestReconcileLauncherOrpheusDefaultMissingFileMarks(t *testing.T) {
	launcherMigrationRoots(t)
	if res := reconcileLauncherOrpheusDefault(false); res.status != recOK {
		t.Fatalf("missing file = %s (%s), want ok", res.status.label(), res.detail)
	}
	if _, err := os.Stat(launcherOrpheusDefaultMarker()); err != nil {
		t.Fatalf("missing file should mark: %v", err)
	}
}
