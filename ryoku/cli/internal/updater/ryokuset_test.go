package updater

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
)

// The set decides what `ryoku update` is allowed to touch, so every property
// here is load-bearing: only packages the [ryoku] repo serves, only ones the
// box actually has, and every target repo-qualified so pacman resolves it from
// our repo even for a name that also exists in core/extra (an unqualified
// target would silently install the Arch build, or fail to downgrade onto a
// frozen release).
func TestRyokuSet(t *testing.T) {
	for _, c := range []struct {
		name            string
		repo, installed []string
		want            []string
	}{
		{
			name:      "installed and served, repo-qualified and sorted",
			repo:      []string{"ryoku-shell", "ryoku-desktop", "ryotunes"},
			installed: []string{"linux", "ryoku-desktop", "ryoku-shell", "firefox"},
			want:      []string{"ryoku/ryoku-desktop", "ryoku/ryoku-shell"},
		},
		{
			name:      "a package the repo serves but the box lacks is not installed by an update",
			repo:      []string{"ryoku-desktop", "asusctl"},
			installed: []string{"ryoku-desktop"},
			want:      []string{"ryoku/ryoku-desktop"},
		},
		{
			name:      "a kernel is never in the set, because the repo never serves one",
			repo:      []string{"ryoku-desktop"},
			installed: []string{"linux", "linux-cachyos", "linux-firmware", "ryoku-desktop"},
			want:      []string{"ryoku/ryoku-desktop"},
		},
		{
			name:      "nothing served: empty, never a bare pacman target",
			repo:      nil,
			installed: []string{"ryoku-desktop"},
			want:      []string{},
		},
		{
			name:      "duplicate repo lines collapse",
			repo:      []string{"ryoku-desktop", "ryoku-desktop"},
			installed: []string{"ryoku-desktop"},
			want:      []string{"ryoku/ryoku-desktop"},
		},
		{
			name:      "an external-release package the repo serves is never in the update set (no downgrade of a newer external build)",
			repo:      []string{"ryoku-desktop", "ryotunes"},
			installed: []string{"ryoku-desktop", "ryotunes"},
			want:      []string{"ryoku/ryoku-desktop"},
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			got := ryokuSet(c.repo, c.installed)
			if !reflect.DeepEqual(got, c.want) {
				t.Errorf("ryokuSet = %v, want %v", got, c.want)
			}
		})
	}
}

// The update lane must never move a package DOWN. Two real shapes make that
// load-bearing: a distro repo ahead of our vendored copy (CachyOS serving
// limine-snapper-sync 1.32.0 while [ryoku] holds 1.31.0 -- re-issuing
// ryoku/<name> flip-flops the package inside one run and writes a .pacnew
// every time), and a split official package (asusctl and rog-control-center)
// whose pinned dep a forced downgrade breaks, failing the whole transaction.
func TestDropOlderServes(t *testing.T) {
	// versions maps a package name to its (installed, served) pair.
	serve := func(versions map[string][2]string) func(string, ...string) (string, error) {
		return func(_ string, args ...string) (string, error) {
			if len(args) < 2 {
				return "", fmt.Errorf("unexpected pacman call: %v", args)
			}
			var name string
			var idx int
			switch args[0] {
			case "-Qi":
				name, idx = args[1], 0
			case "-Si":
				name, idx = strings.TrimPrefix(args[1], "ryoku/"), 1
			default:
				return "", fmt.Errorf("unexpected pacman call: %v", args)
			}
			v, ok := versions[name]
			if !ok || v[idx] == "" {
				return "", fmt.Errorf("package not found: %s", name)
			}
			if idx == 0 {
				return fmt.Sprintf("Name            : %s\nVersion         : %s\n", name, v[0]), nil
			}
			return fmt.Sprintf("Repository      : ryoku\nName            : %s\nVersion         : %s\n", name, v[1]), nil
		}
	}
	// vercmp stand-in on the fixed-width versions the fake serves: correct for
	// every comparison the tests below make.
	compare := func(a, b string) int {
		return strings.Compare(a, b)
	}

	set := []string{"ryoku/asusctl", "ryoku/limine-snapper-sync", "ryoku/ryoku-desktop"}

	t.Run("a newer installed version is held back", func(t *testing.T) {
		runPacman = serve(map[string][2]string{
			"limine-snapper-sync": {"1.32.0-1", "1.31.0-1"}, // the box is ahead of [ryoku]
			"asusctl":             {"6.4.0-1", "6.4.0-1"},
			"ryoku-desktop":       {"0.74.0-1", "0.74.0-1"},
		})
		vercmp = compare
		got := dropOlderServes(set)
		want := []string{"ryoku/asusctl", "ryoku/ryoku-desktop"}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("dropOlderServes = %v, want %v", got, want)
		}
	})

	t.Run("a box behind the repo stays in the set", func(t *testing.T) {
		runPacman = serve(map[string][2]string{
			"limine-snapper-sync": {"1.30.0-1", "1.31.0-1"},
			"asusctl":             {"6.4.0-1", "6.5.0-1"},
			"ryoku-desktop":       {"0.73.0-1", "0.74.0-1"},
		})
		vercmp = compare
		got := dropOlderServes(set)
		if !reflect.DeepEqual(got, set) {
			t.Errorf("dropOlderServes = %v, want the whole set", got)
		}
	})

	t.Run("an unanswerable query keeps the target", func(t *testing.T) {
		runPacman = serve(map[string][2]string{})
		vercmp = compare
		got := dropOlderServes(set)
		if !reflect.DeepEqual(got, set) {
			t.Errorf("dropOlderServes = %v, want the whole set (never skip on a failed query)", got)
		}
	})
}

func TestVersionField(t *testing.T) {
	out := "Repository      : ryoku\nName            : gpk\nVersion         : 0.5.3-1\nDescription     : a: b\n"
	if got := versionField(out); got != "0.5.3-1" {
		t.Errorf("versionField = %q, want 0.5.3-1", got)
	}
	if got := versionField("Name : x\n"); got != "" {
		t.Errorf("versionField = %q, want empty", got)
	}
}
