package doctor

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"ryoku-cli/internal/host"
)

func stubPacmanHost(t *testing.T, available bool) {
	t.Helper()
	oldHasPacman := hasPacman
	oldManager := doctorPackageManager
	manager := host.XBPS
	if available {
		manager = host.Pacman
	}
	hasPacman = func() bool { return available }
	doctorPackageManager = func() (host.PackageManager, error) { return manager, nil }
	t.Setenv("RYOKU_HOST_PKGMGR", string(manager))
	t.Cleanup(func() {
		hasPacman = oldHasPacman
		doctorPackageManager = oldManager
	})
}

// enableILoveCandy must land the directive inside [options] and nowhere else:
// pacman only honors it there, and a config it cannot parse breaks every
// pacman invocation on the box.
func TestEnableILoveCandy(t *testing.T) {
	const stock = "[options]\nHoldPkg = pacman glibc\n\n[core]\nInclude = /etc/pacman.d/mirrorlist\n"
	for _, c := range []struct {
		name, conf, want string
		changed, ok      bool
	}{
		{
			name:    "inserted under the options header",
			conf:    stock,
			want:    "[options]\nILoveCandy\nHoldPkg = pacman glibc\n\n[core]\nInclude = /etc/pacman.d/mirrorlist\n",
			changed: true, ok: true,
		},
		{
			name:    "commented directive is uncommented in place",
			conf:    "[options]\nHoldPkg = pacman glibc\n#ILoveCandy\n\n[core]\nInclude = /etc/pacman.d/mirrorlist\n",
			want:    "[options]\nHoldPkg = pacman glibc\nILoveCandy\n\n[core]\nInclude = /etc/pacman.d/mirrorlist\n",
			changed: true, ok: true,
		},
		{
			name:    "already active is a no-op",
			conf:    "[options]\nILoveCandy\nHoldPkg = pacman glibc\n",
			want:    "[options]\nILoveCandy\nHoldPkg = pacman glibc\n",
			changed: false, ok: true,
		},
		{
			name:    "a directive in a repo section does not count as active",
			conf:    "[options]\nHoldPkg = pacman glibc\n\n[ryoku]\nILoveCandy\n",
			want:    "[options]\nILoveCandy\nHoldPkg = pacman glibc\n\n[ryoku]\nILoveCandy\n",
			changed: true, ok: true,
		},
		{
			name: "no options section: nothing is guessed",
			conf: "[core]\nInclude = /etc/pacman.d/mirrorlist\n",
			ok:   false,
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			out, changed, ok := enableILoveCandy([]byte(c.conf))
			if ok != c.ok || changed != c.changed {
				t.Fatalf("changed=%v ok=%v, want changed=%v ok=%v", changed, ok, c.changed, c.ok)
			}
			if !ok {
				return
			}
			if string(out) != c.want {
				t.Fatalf("got:\n%q\nwant:\n%q", out, c.want)
			}
		})
	}
}

// Applying the result twice must be stable: the reconciler runs on every
// `ryoku update`, and a second pass may not stack a duplicate directive.
func TestEnableILoveCandyIdempotent(t *testing.T) {
	once, _, ok := enableILoveCandy([]byte("[options]\nHoldPkg = pacman glibc\n"))
	if !ok {
		t.Fatal("first pass did not apply")
	}
	twice, changed, ok := enableILoveCandy(once)
	if !ok || changed {
		t.Fatalf("second pass changed=%v ok=%v, want changed=false ok=true", changed, ok)
	}
	if string(twice) != string(once) {
		t.Fatalf("second pass rewrote the config: %q", twice)
	}
}

func TestPacmanHostUsesHostPackageManager(t *testing.T) {
	t.Setenv("RYOKU_HOST_PKGMGR", "xbps")
	if hasPacman() {
		t.Fatal("xbps host reported pacman")
	}
}

func TestPacmanReconcilersAreNeutralOnOtherPackageManagers(t *testing.T) {
	stubPacmanHost(t, false)
	checks := []struct {
		name string
		run  func() recResult
	}{
		{"boot guard", func() recResult { return reconcileBootGuard(true) }},
		{"channel pin", func() recResult { return reconcileChannelPin(true) }},
		{"pacman progress", func() recResult { return reconcilePacmanCandy(true) }},
		{"multilib", func() recResult { return reconcileMultilibRepo(true) }},
	}
	for _, check := range checks {
		t.Run(check.name, func(t *testing.T) {
			if got := check.run(); got.status != recOK && got.status != recNote {
				t.Fatalf("result = %#v, want a neutral result", got)
			}
		})
	}
}

func TestConflictingFilesRespectSourceUpdates(t *testing.T) {
	for _, source := range []string{"checkout", "missing checkout", "packages"} {
		t.Run(source, func(t *testing.T) {
			stubPacmanHost(t, true)
			home := t.TempDir()
			t.Setenv("HOME", home)
			t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
			t.Setenv("XDG_STATE_HOME", filepath.Join(home, ".local", "state"))
			t.Setenv("RYOKU_REPO", "")
			bin := filepath.Join(home, "bin")
			engine := filepath.Join(home, "usr", "bin", "ryoku-dictate")
			t.Setenv("RYOKU_TEST_ENGINE", engine)
			writeFile(t, filepath.Join(bin, "pacman"), "#!/bin/sh\ncase \"$1\" in\n-Q) exit 0;;\n-Qo) exit 1;;\n*) exit 2;;\nesac\n")
			writeFile(t, filepath.Join(bin, "sudo"), "#!/bin/sh\n[ \"$#\" -eq 3 ] && [ \"$1\" = rm ] && [ \"$2\" = -f ] && [ \"$3\" = \"$RYOKU_TEST_ENGINE\" ] || exit 2\nexec /usr/bin/rm -f -- \"$3\"\n")
			for _, tool := range []string{"pacman", "sudo"} {
				if err := os.Chmod(filepath.Join(bin, tool), 0o755); err != nil {
					t.Fatal(err)
				}
			}
			t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
			if source != "packages" {
				repo := filepath.Join(home, "checkout")
				if source == "checkout" {
					if out, err := exec.Command("git", "init", "-q", repo).CombinedOutput(); err != nil {
						t.Fatalf("git init: %v: %s", err, out)
					}
				}
				writeFile(t, filepath.Join(home, ".local", "state", "ryoku", "repo"), repo)
			}
			oldGlobs := ryokuSystemGlobs
			ryokuSystemGlobs = []string{engine}
			t.Cleanup(func() { ryokuSystemGlobs = oldGlobs })
			for _, checkOnly := range []bool{true, false} {
				writeFile(t, engine, "installed voice engine")
				result := reconcileConflictingRyokuFiles(checkOnly)
				want := recOK
				if source != "checkout" {
					want = recWouldFix
					if !checkOnly {
						want = recFixed
					}
				}
				if result.status != want {
					t.Fatalf("checkOnly=%v: result=%+v, want %s", checkOnly, result, want.label())
				}
				_, err := os.Stat(engine)
				if source == "checkout" || checkOnly {
					if err != nil {
						t.Fatalf("checkOnly=%v: installed engine was removed: %v", checkOnly, err)
					}
				} else if !os.IsNotExist(err) {
					t.Fatalf("package-only cleanup left the unowned engine: %v", err)
				}
			}
		})
	}
}
