package updater

import (
	"os"
	"path/filepath"
	"testing"
)

type accountFixture struct {
	home, share, state string
}

func newAccountFixture(t *testing.T) accountFixture {
	t.Helper()
	f := accountFixture{home: t.TempDir(), share: t.TempDir()}
	f.state = filepath.Join(f.home, ".local", "state")
	base := t.TempDir()
	writeFile(t, filepath.Join(base, "fastfetch/config.jsonc"), "{}\n")
	t.Setenv("RYOKU_CONFIG_BASE", base)
	t.Setenv("HOME", f.home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(f.home, ".config"))
	t.Setenv("XDG_STATE_HOME", f.state)
	t.Setenv("XDG_PICTURES_DIR", "")
	t.Setenv("RYOKU_SHARE_ROOT", f.share)
	writeFile(t, filepath.Join(f.share, "brand/fastfetch-emblem.png"), "emblem")
	writeFile(t, filepath.Join(f.share, "brand/logo-mark.png"), "mark")
	writeFile(t, filepath.Join(f.share, "ryodecors/placard.png"), "decor")
	writeFile(t, filepath.Join(f.share, "npmrc"), "prefix=${HOME}/.local\n")
	writeFile(t, filepath.Join(f.share, "rices/default/rice.json"), "{}\n")
	return f
}

func (f accountFixture) seeded() map[string]string {
	return map[string]string{
		filepath.Join(f.home, ".local/share/ryoku/assets/brand/fastfetch-emblem.png"): "emblem",
		filepath.Join(f.home, ".local/share/ryoku/assets/brand/logo-mark.png"):        "mark",
		filepath.Join(f.home, "Pictures/ryodecors/placard.png"):                       "decor",
		filepath.Join(f.home, ".npmrc"):                                               "prefix=${HOME}/.local\n",
	}
}

func readOrEmpty(path string) string {
	b, _ := os.ReadFile(path)
	return string(b)
}

// A second account's first login runs materialize on a home that holds only
// /etc/skel: it must end up with what the installer gave the first account.
func TestMaterializeSeedsANewAccount(t *testing.T) {
	f := newAccountFixture(t)
	for range 2 {
		if err := Materialize(); err != nil {
			t.Fatal(err)
		}
		for path, want := range f.seeded() {
			if got := readOrEmpty(path); got != want {
				t.Fatalf("%s = %q, want %q", path, got, want)
			}
		}
		if got := readOrEmpty(filepath.Join(f.state, "ryoku/default-rice-pending")); got != "default\n" {
			t.Fatalf("default rice marker = %q, want the default rice queued", got)
		}
	}
}

// An account already in use keeps every file it has and never gets the default
// rice pushed over its look, rice or not.
func TestMaterializeLeavesACustomisedAccountAlone(t *testing.T) {
	f := newAccountFixture(t)
	mine := map[string]string{}
	for path := range f.seeded() {
		mine[path] = "mine"
		writeFile(t, path, "mine")
	}
	writeFile(t, filepath.Join(f.home, ".config/ryoku/shell.json"), `{"barStyle":"kairos"}`)
	if err := Materialize(); err != nil {
		t.Fatal(err)
	}
	for path, want := range mine {
		if got := readOrEmpty(path); got != want {
			t.Fatalf("%s = %q, the user's file was replaced", path, got)
		}
	}
	if got := readOrEmpty(filepath.Join(f.home, ".config/ryoku/shell.json")); got != `{"barStyle":"kairos"}` {
		t.Fatalf("shell.json = %q", got)
	}
	if _, err := os.Lstat(filepath.Join(f.state, "ryoku/default-rice-pending")); !os.IsNotExist(err) {
		t.Fatalf("an account in use must not get the default rice queued (err=%v)", err)
	}
}

func TestPlanAccountSeedRice(t *testing.T) {
	for _, c := range []struct {
		name string
		s    accountSeedState
		want bool
	}{
		{"new account", accountSeedState{defaultRice: true}, true},
		{"no packaged default rice", accountSeedState{}, false},
		{"shell already ran", accountSeedState{defaultRice: true, shellStore: true}, false},
		{"a rice is active", accountSeedState{defaultRice: true, activeRice: true}, false},
		{"already queued", accountSeedState{defaultRice: true, ricePending: true}, false},
		{"root", accountSeedState{defaultRice: true, root: true}, false},
	} {
		if got := planAccountSeed(c.s).QueueRice; got != c.want {
			t.Errorf("%s: QueueRice = %v, want %v", c.name, got, c.want)
		}
	}
}
