package doctor

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"ryoku-cli/internal/host"
)

func TestHyakkiDoctorNeverRetiresAnUnmigratedLegacyOwner(t *testing.T) {
	for _, state := range []struct {
		installed bool
		marked    bool
	}{{true, false}, {true, true}, {false, false}} {
		t.Run(fmt.Sprintf("installed=%v/marked=%v", state.installed, state.marked), func(t *testing.T) {
			config, bin := t.TempDir(), t.TempDir()
			t.Setenv("XDG_CONFIG_HOME", config)
			t.Setenv("XDG_STATE_HOME", t.TempDir())
			t.Setenv("PATH", bin)
			if state.installed {
				if err := os.WriteFile(filepath.Join(bin, "hyakki"), []byte("#!/bin/sh\nexit 99\n"), 0o755); err != nil {
					t.Fatal(err)
				}
			}
			unit := filepath.Join(config, "systemd", "user", "rashind.service")
			before := []byte("[Service]\nExecStart=/owned/legacy-worker\nRestart=no\n")
			if err := os.MkdirAll(filepath.Dir(unit), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(unit, before, 0o640); err != nil {
				t.Fatal(err)
			}
			oldInit, oldService, oldMigration := hyakkiInit, hyakkiService, hyakkiMigration
			oldRunning, oldVersion := oniRunning, reportPackageVersion
			t.Cleanup(func() {
				hyakkiInit, hyakkiService, hyakkiMigration = oldInit, oldService, oldMigration
				oniRunning, reportPackageVersion = oldRunning, oldVersion
			})
			hyakkiInit = func() (host.InitSystem, error) { return host.Systemd, nil }
			oniRunning = func() bool { return false }
			reportPackageVersion = func(string) (string, bool) { return "guard", true }
			if state.marked {
				marker := hyakkiMigrationMarker()
				if err := os.MkdirAll(filepath.Dir(marker), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(marker, []byte("done\n"), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			var mutations [][]string
			hyakkiService = func(args []string) int {
				if args[1] == "is-active" || args[1] == "is-enabled" {
					if args[len(args)-1] == "rashind.service" {
						return host.ExitOK
					}
					return host.ExitAbsent
				}
				mutations = append(mutations, append([]string(nil), args...))
				return host.ExitOK
			}
			hyakkiMigration = func() error { return fmt.Errorf("legacy owner still has unsaved work") }
			result := reconcileHyakkiDaemon(false)
			want := recOK
			if state.installed {
				want = recFailed
			}
			if result.status != want {
				t.Errorf("busy legacy owner: status=%v, want %v", result.status, want)
			}
			if len(mutations) != 0 {
				t.Errorf("unmigrated owner received supervisor mutations: %v", mutations)
			}
			got, err := os.ReadFile(unit)
			info, statErr := os.Stat(unit)
			if err != nil || statErr != nil || !bytes.Equal(got, before) || info.Mode().Perm() != 0o640 {
				t.Fatalf("legacy unit was not preserved: %q, read=%v, stat=%v", got, err, statErr)
			}
		})
	}
}

func TestHyakkiRetirementKeepsInheritedActivationSuppressed(t *testing.T) {
	config := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", config)
	unit := filepath.Join(config, "systemd", "user", "rashind.socket")
	if err := os.MkdirAll(filepath.Dir(unit), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("/dev/null", unit); err != nil {
		t.Fatal(err)
	}
	oldService := hyakkiService
	t.Cleanup(func() { hyakkiService = oldService })
	hyakkiService = func([]string) int { return host.ExitAbsent }
	if legacyAppResidue(host.Systemd) {
		t.Fatal("a suppressed predecessor became a recurring migration finding")
	}
	if err := removeLegacyAppUnits(host.Systemd); err != nil {
		t.Fatal(err)
	}
	if target, err := os.Readlink(unit); err != nil || target != "/dev/null" {
		t.Fatalf("retirement uncovered global activation: %q, %v", target, err)
	}
}
