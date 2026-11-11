package doctor

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func writeSeedFixture(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// accountSeedHome is a second account's home on a packaged box whose decor art
// comes from a dev checkout, so both source kinds are exercised.
func accountSeedHome(t *testing.T, pictures string) (home string, seeded map[string]string) {
	t.Helper()
	home, share, repo := t.TempDir(), t.TempDir(), t.TempDir()
	if out, err := exec.Command("git", "-C", repo, "init", "-q").CombinedOutput(); err != nil {
		t.Skipf("git unavailable: %v %s", err, out)
	}
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(home, ".local", "state"))
	t.Setenv("XDG_PICTURES_DIR", "")
	if pictures != "" {
		t.Setenv("XDG_PICTURES_DIR", filepath.Join(home, pictures))
	} else {
		pictures = "Pictures"
	}
	t.Setenv("RYOKU_REPO", repo)
	t.Setenv("RYOKU_SHARE_ROOT", share)
	t.Setenv("RYOKU_SYSTEM_RICES_DIR", "")
	writeSeedFixture(t, filepath.Join(repo, "ryoku/assets/ryodecors/a.png"), "decor-a")
	writeSeedFixture(t, filepath.Join(repo, "ryoku/assets/ryodecors/b.gif"), "decor-b")
	writeSeedFixture(t, filepath.Join(share, "brand/fastfetch-emblem.png"), "emblem")
	writeSeedFixture(t, filepath.Join(share, "npmrc"), "prefix=${HOME}/.local\n")
	writeSeedFixture(t, filepath.Join(share, "rices/default/rice.json"), "{}\n")
	return home, map[string]string{
		filepath.Join(home, pictures, "ryodecors/a.png"):                            "decor-a",
		filepath.Join(home, pictures, "ryodecors/b.gif"):                            "decor-b",
		filepath.Join(home, ".local/share/ryoku/assets/brand/fastfetch-emblem.png"): "emblem",
		filepath.Join(home, ".npmrc"):                                               "prefix=${HOME}/.local\n",
	}
}

func riceMarker(home string) string {
	b, _ := os.ReadFile(filepath.Join(home, ".local/state/ryoku/default-rice-pending"))
	return string(b)
}

func TestReconcileAccountSeedNewAccount(t *testing.T) {
	home, seeded := accountSeedHome(t, "")

	if r := reconcileAccountSeed(true); r.status != recWouldFix {
		t.Fatalf("check-only: status=%s detail=%q, want would-fix", r.status.label(), r.detail)
	}
	for path := range seeded {
		if _, err := os.Lstat(path); !os.IsNotExist(err) {
			t.Fatalf("check-only wrote %s", path)
		}
	}
	if riceMarker(home) != "" {
		t.Fatal("check-only queued the default rice")
	}

	if r := reconcileAccountSeed(false); r.status != recFixed {
		t.Fatalf("fix: status=%s detail=%q, want fixed", r.status.label(), r.detail)
	}
	for path, want := range seeded {
		if b, _ := os.ReadFile(path); string(b) != want {
			t.Fatalf("%s = %q, want %q", path, b, want)
		}
	}
	if got := riceMarker(home); got != "default\n" {
		t.Fatalf("default rice marker = %q", got)
	}

	mine := filepath.Join(home, "Pictures/ryodecors/mine.png")
	writeSeedFixture(t, mine, "mine")
	if r := reconcileAccountSeed(false); r.status != recOK {
		t.Fatalf("settled account: status=%s detail=%q, want ok", r.status.label(), r.detail)
	}
	if b, _ := os.ReadFile(mine); string(b) != "mine" {
		t.Fatal("a user's own decor file must survive")
	}
}

// A customised account keeps every byte, and an account whose shell has run is
// never handed the default rice, even with no rice applied (installs that
// predate shipped rices look exactly like that).
func TestReconcileAccountSeedLeavesACustomisedAccountAlone(t *testing.T) {
	home, seeded := accountSeedHome(t, "")
	for path := range seeded {
		writeSeedFixture(t, path, "mine")
	}
	store := filepath.Join(home, ".config/ryoku/shell.json")
	writeSeedFixture(t, store, `{"barStyle":"kairos"}`)

	if r := reconcileAccountSeed(false); r.status != recOK {
		t.Fatalf("status=%s detail=%q, want ok", r.status.label(), r.detail)
	}
	for path := range seeded {
		if b, _ := os.ReadFile(path); string(b) != "mine" {
			t.Fatalf("%s was replaced: %q", path, b)
		}
	}
	if b, _ := os.ReadFile(store); string(b) != `{"barStyle":"kairos"}` {
		t.Fatalf("shell.json = %q", b)
	}
	if riceMarker(home) != "" {
		t.Fatal("an account in use got the default rice queued")
	}
}

// A localized desktop keeps its pictures in ~/Imágenes: the decor art is seeded
// there and no stray ~/Pictures appears beside it.
func TestReconcileAccountSeedLocalizedPictures(t *testing.T) {
	home, _ := accountSeedHome(t, "Imágenes")
	if r := reconcileAccountSeed(false); r.status != recFixed {
		t.Fatalf("fix: status=%s detail=%q, want fixed", r.status.label(), r.detail)
	}
	if _, err := os.Stat(filepath.Join(home, "Imágenes", "ryodecors", "a.png")); err != nil {
		t.Fatalf("decor art must land in the XDG Pictures folder: %v", err)
	}
	if _, err := os.Stat(filepath.Join(home, "Pictures")); !os.IsNotExist(err) {
		t.Fatalf("a localized desktop must not grow ~/Pictures (stat err=%v)", err)
	}
}
