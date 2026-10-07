package doctor

import (
	"os"
	"path/filepath"
	"testing"
)

func writeFile(t *testing.T, p, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readFile(t *testing.T, p string) string {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func residueFixture(t *testing.T) residueLayout {
	root := t.TempDir()
	l := residueLayout{
		localBin:   filepath.Join(root, "home", ".local", "bin"),
		configHome: filepath.Join(root, "home", ".config"),
		baseDir:    filepath.Join(root, "base"),
		usrUnits:   filepath.Join(root, "usr-units"),
	}
	l.editsDir = filepath.Join(l.configHome, "ryoku", "user_edits")
	return l
}

var packagedNames = func(name string) bool {
	switch name {
	case "ryoku-shell", "ryoku-qylock-activate", "ryoku-rashin", "ryogami":
		return true
	}
	return false
}

// A box that went dev -> packaged: the overlay holds a fork of the deploy's
// rewritten shell unit, materialize keeps laying it, and the home binary it
// names is already gone. The heal must drop the fork and put the shipped unit
// back, or the shell never starts after the next login.
func TestDevUnitsForkAndLiveCopyReturnToTheShippedUnit(t *testing.T) {
	l := residueFixture(t)
	shipped := "[Service]\nExecStartPre=/usr/bin/ryoku-qylock-activate\nExecStart=/usr/bin/ryoku-shell daemon\n"
	dev := "[Service]\nExecStartPre=" + l.localBin + "/ryoku-qylock-activate\nExecStart=" + l.localBin + "/ryoku-shell daemon\n"
	writeFile(t, filepath.Join(l.baseDir, "systemd/user/ryoku-shell.service"), shipped)
	fork := filepath.Join(l.editsDir, "systemd/user/ryoku-shell.service")
	writeFile(t, fork, dev)
	live := filepath.Join(l.userUnits(), "ryoku-shell.service")
	writeFile(t, live, dev)

	units, failed := l.applyDevUnits(l.planDevUnits(packagedNames))
	if len(failed) > 0 {
		t.Fatalf("failed: %v", failed)
	}
	if _, err := os.Stat(fork); !os.IsNotExist(err) {
		t.Fatalf("the fork survived: %v", err)
	}
	if got := readFile(t, live); got != shipped {
		t.Fatalf("live unit = %q, want the shipped unit", got)
	}
	if len(units) != 1 || units[0] != "ryoku-shell.service" {
		t.Fatalf("touched units = %v", units)
	}
	if again := l.planDevUnits(packagedNames); len(again) != 0 {
		t.Fatalf("a healed box still plans %v", again)
	}
}

// Whole units written by deploy shadow the packaged units even when their
// resolved binaries live outside ~/.local/bin.
func TestDevUnitsHomeUnitYieldsToPackagedUnit(t *testing.T) {
	cases := []struct {
		name, unit, homeExec, packagedExec string
		enabled                            bool
	}{
		{
			name:         "rashin stays enabled",
			unit:         "ryoku-rashin.service",
			homeExec:     "/home/dev/bin/ryoku-rashin serve --if-enabled",
			packagedExec: "/usr/bin/ryoku-rashin serve --if-enabled",
			enabled:      true,
		},
		{
			name:         "prowl shadow is removed",
			unit:         "ryoku-prowl.service",
			homeExec:     "/home/dev/bin/prowl gateway serve --port 8788",
			packagedExec: "/usr/bin/prowl gateway serve --port 8788",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			l := residueFixture(t)
			home := filepath.Join(l.userUnits(), c.unit)
			writeFile(t, home, "[Service]\nExecStart="+c.homeExec+"\n")
			pkg := filepath.Join(l.usrUnits, c.unit)
			writeFile(t, pkg, "[Service]\nExecStart="+c.packagedExec+"\n")

			var link string
			if c.enabled {
				link = filepath.Join(l.userUnits(), "default.target.wants", c.unit)
				if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(home, link); err != nil {
					t.Fatal(err)
				}
			}

			units, failed := l.applyDevUnits(l.planDevUnits(packagedNames))
			if len(failed) > 0 {
				t.Fatalf("failed: %v", failed)
			}
			if _, err := os.Lstat(home); !os.IsNotExist(err) {
				t.Fatalf("the home unit survived: %v", err)
			}
			if c.enabled {
				if target, err := os.Readlink(link); err != nil || target != pkg {
					t.Fatalf("wants link -> %q (%v), want %q", target, err, pkg)
				}
			}
			if len(units) != 1 || units[0] != c.unit {
				t.Fatalf("touched units = %v, want [%s]", units, c.unit)
			}
			if again := l.planDevUnits(packagedNames); len(again) != 0 {
				t.Fatalf("a healed box still plans %v", again)
			}
		})
	}
}

func TestDevUnitsDropInIsRemoved(t *testing.T) {
	l := residueFixture(t)
	dropIn := filepath.Join(l.userUnits(), "ryogami.service.d", devBinDropIn)
	writeFile(t, dropIn, "[Service]\nExecStart=\nExecStart="+l.localBin+"/ryogami\n")
	keep := filepath.Join(l.userUnits(), "ryoku-shell.service.d", "media.conf")
	writeFile(t, keep, "[Service]\nEnvironment=QT_MEDIA_BACKEND=ffmpeg\n")

	units, failed := l.applyDevUnits(l.planDevUnits(packagedNames))
	if len(failed) > 0 {
		t.Fatalf("failed: %v", failed)
	}
	if _, err := os.Stat(dropIn); !os.IsNotExist(err) {
		t.Fatalf("the dev drop-in survived: %v", err)
	}
	if _, err := os.Stat(keep); err != nil {
		t.Fatalf("an unrelated drop-in went too: %v", err)
	}
	if len(units) != 1 || units[0] != "ryogami.service" {
		t.Fatalf("touched units = %v", units)
	}
}

// A user's own unit running their own ~/.local/bin tool, and one naming a
// packaged binary with nothing packaged to fall back on, are theirs to keep.
func TestDevUnitsLeavesUnitsWithNothingPackagedToReturnTo(t *testing.T) {
	l := residueFixture(t)
	writeFile(t, filepath.Join(l.userUnits(), "mika.service"), "[Service]\nExecStart="+l.localBin+"/mika\n")
	writeFile(t, filepath.Join(l.userUnits(), "custom-shell.service"), "[Service]\nExecStart="+l.localBin+"/ryoku-shell daemon\n")
	if heals := l.planDevUnits(packagedNames); len(heals) != 0 {
		t.Fatalf("planned %v for units with no packaged counterpart", heals)
	}
}

func TestHomeRyokuWrapperMatchesOnlyTheHomeCLI(t *testing.T) {
	cfg := t.TempDir()
	p := filepath.Join(cfg, "fish", "functions", "ryoku.fish")
	writeFile(t, p, "function ryoku\n    command $HOME/.local/bin/ryoku-shell $argv\nend\n")
	if got := homeRyokuWrapper(cfg); got != "" {
		t.Fatalf("a wrapper for another binary matched: %q", got)
	}
	writeFile(t, p, "function ryoku\n    command $HOME/.local/bin/ryoku $argv\nend\n")
	if got := homeRyokuWrapper(cfg); got != p {
		t.Fatalf("homeRyokuWrapper = %q, want %q", got, p)
	}
}
