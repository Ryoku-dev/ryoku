package host

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestHyakkiInheritedActivationIsUserLocalAndReversible(t *testing.T) {
	for _, fail := range []bool{false, true} {
		for _, locallyEnabled := range []bool{false, true} {
			t.Run(fmt.Sprintf("failure=%v/local=%v", fail, locallyEnabled), func(t *testing.T) {
				f := newHyakkiMigrationFixture(t)
				f.states["rashind.socket"] = hyakkiServiceState{true, true}
				root := filepath.Join(f.app.home(), "config", "systemd", "user")
				unit := filepath.Join(root, "rashind.socket")
				link := filepath.Join(root, "sockets.target.wants", "rashind.socket")
				if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil {
					t.Fatal(err)
				}
				before := []byte("[Socket]\nListenStream=%t/rashin/rashin.sock\n[Install]\nWantedBy=sockets.target\n")
				if err := os.WriteFile(unit, before, 0o640); err != nil {
					t.Fatal(err)
				}
				if locallyEnabled {
					if err := os.Symlink("../rashind.socket", link); err != nil {
						t.Fatal(err)
					}
				}
				if fail {
					f.migrationCode = 1
				}
				base := f.runner.answer
				f.runner.answer = func(c Command) Result {
					if c.Name == "systemctl" {
						if len(c.Args) > 0 && c.Args[0] == "--global" {
							if strings.Join(c.Args, " ") != "--global is-enabled rashind.socket" {
								t.Fatalf("migration changed another user's activation: %v", c.Args)
							}
							return Result{Output: "enabled\n"}
						}
						if len(c.Args) == 2 && c.Args[1] == "daemon-reload" {
							return Result{}
						}
						if c.Args[len(c.Args)-1] == "rashind.socket" {
							switch c.Args[1] {
							case "is-enabled":
								target, _ := os.Readlink(unit)
								state := f.states["rashind.socket"]
								state.enabled = target != "/dev/null"
								f.states["rashind.socket"] = state
								if !state.enabled {
									return Result{Code: 1}
								}
								return Result{}
							case "disable":
								if err := os.Remove(link); err != nil && !os.IsNotExist(err) {
									t.Fatal(err)
								}
								return Result{}
							case "enable":
								t.Fatal("inherited activation must not become a new explicit local enable")
							}
						}
					}
					if c.Name == "hyakki" {
						if target, err := os.Readlink(unit); err != nil || target != "/dev/null" {
							t.Fatalf("native migration preceded per-user suppression: %q, %v", target, err)
						}
					}
					return base(c)
				}
				err := f.app.migrateHyakkiQuiesced(f.snapshot(), nil, f.app.legacyHyakkiReleased, func(time.Duration) { t.Fatal("an idle predecessor must not wait") }, "")
				if (err != nil) != fail {
					t.Fatalf("migration failure=%v, error=%v", fail, err)
				}
				if fail {
					body, readErr := os.ReadFile(unit)
					info, statErr := os.Stat(unit)
					if readErr != nil || statErr != nil || !bytes.Equal(body, before) || info.Mode().Perm() != 0o640 {
						t.Fatalf("original user definition not restored: %q, %v, %v", body, readErr, statErr)
					}
					if target, linkErr := os.Readlink(link); locallyEnabled {
						if linkErr != nil || target != "../rashind.socket" {
							t.Fatalf("original local activation not restored: %q, %v", target, linkErr)
						}
					} else if !os.IsNotExist(linkErr) {
						t.Fatalf("rollback created an unwanted local enable: %q, %v", target, linkErr)
					}
					if state := f.states["rashind.socket"]; !state.enabled || !state.running {
						t.Fatalf("original inherited activation not restored: %+v", state)
					}
				} else {
					if target, err := os.Readlink(unit); err != nil || target != "/dev/null" {
						t.Fatalf("legacy activation was uncovered: %q, %v", target, err)
					}
					backups, err := filepath.Glob(filepath.Join(root, ".hyakki-retired-units-*", "rashind.socket"))
					if err != nil || len(backups) != 1 {
						t.Fatalf("original definition backup: %v, %v", backups, err)
					}
					body, err := os.ReadFile(backups[0])
					if err != nil || !bytes.Equal(body, before) {
						t.Fatalf("retired definition not preserved: %q, %v", body, err)
					}
				}
			})
		}
	}
}
