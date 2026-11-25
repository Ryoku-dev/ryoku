package host

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestRepoSetReplacesOnlyInstalledLegacyHyakki(t *testing.T) {
	for _, manager := range []PackageManager{Pacman, DNF, XBPS} {
		for _, state := range []struct {
			name                     string
			legacy, served, excluded bool
			wantError                bool
		}{
			{name: "installed", legacy: true, served: true},
			{name: "removed", served: true},
			{name: "never-installed"},
			{name: "missing-replacement", legacy: true, wantError: true},
			{name: "explicit-exclusion", legacy: true, served: true, excluded: true},
		} {
			t.Run(string(manager)+"/"+state.name, func(t *testing.T) {
				version, want := "1.0-1", "ryoku/hyakki"
				listing := "ryoku ryoku-rashin 99.0-1\n"
				switch manager {
				case DNF:
					version, want = "0:1.0-1.fc44", "hyakki-0:1.0-1.fc44"
					listing = "ryoku-rashin\t0:99.0-1.fc44\n"
				case XBPS:
					version, want = "1.0_1", "hyakki"
					listing = "[-] ryoku-rashin-99.0_1 Retired app\n"
				}
				if state.served {
					switch manager {
					case Pacman:
						listing += "ryoku hyakki " + version + "\n"
					case DNF:
						listing += "hyakki\t" + version + "\n"
					case XBPS:
						listing += "[-] hyakki-" + version + " Native app\n"
					}
				}
				runner := &fakeRunner{paths: map[string]bool{"dnf5": true}, answer: func(command Command) Result {
					joined := argv(command)
					if joined == "pacman -Sl ryoku" || strings.Contains(joined, "repoquery") || strings.Contains(joined, "--repository /repo -Rs") {
						return Result{Output: listing}
					}
					if command.Name == "pacman" || command.Name == "rpm" || command.Name == "xbps-query" {
						name := command.Args[len(command.Args)-1]
						if name != "ryoku-rashin" || !state.legacy {
							return Result{Code: 1}
						}
						switch manager {
						case Pacman:
							return Result{Output: name + " 99.0-1\n"}
						case DNF:
							return Result{Output: "0:99.0-1.fc44\n"}
						default:
							return Result{Output: name + "-99.0_1\n"}
						}
					}
					t.Fatalf("unexpected host command: %s", joined)
					return Result{Code: 1}
				}}
				app, _, _ := testApp(runner, map[string]string{"RYOKU_HOST_PKGMGR": string(manager)})
				app.cfg.UID = 1000
				if manager == XBPS {
					dir := t.TempDir()
					if err := os.WriteFile(filepath.Join(dir, "20-ryoku.conf"), []byte("repository=/repo\n"), 0o644); err != nil {
						t.Fatal(err)
					}
					app.cfg.XBPSConfigDir = dir
					app.cfg.XBPSShippedConfig = filepath.Join(dir, "missing")
				}
				for _, allowDowngrade := range []bool{false, true} {
					set, held, err := app.RepoInstalledSet(allowDowngrade, map[string]bool{"hyakki": state.excluded})
					if (err != nil) != state.wantError {
						t.Fatalf("set=%v held=%d error=%v, want error=%v", set, held, err, state.wantError)
					}
					if state.wantError {
						continue
					}
					if held != 0 {
						t.Fatalf("a renamed package compared unrelated release versions: held=%d", held)
					}
					if state.legacy && state.served && !state.excluded {
						if !reflect.DeepEqual(set, []string{want}) {
							t.Fatalf("replacement targets=%v, want only %q", set, want)
						}
					} else if len(set) != 0 {
						t.Fatalf("removed, excluded or retired app was scheduled: %v", set)
					}
				}
			})
		}
	}
}
