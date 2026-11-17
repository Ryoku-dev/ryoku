package doctor

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCheckoutAssetsPreserveEditsOwnersAndSymlinks(t *testing.T) {
	repo, root := t.TempDir(), t.TempDir()
	bin := filepath.Join(root, "bin")
	writeFile(t, filepath.Join(bin, "sudo"), "#!/bin/sh\n[ \"$1\" = rm ] || exit 2\ncase \"$4\" in \"$ASSET_ROOT\"/*) exec /usr/bin/rm -f -- \"$4\";; *) exit 3;; esac\n")
	if err := os.Chmod(filepath.Join(bin, "sudo"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ASSET_ROOT", root)
	t.Setenv("PATH", bin+":"+os.Getenv("PATH"))
	oldOwner := pkgOwnsFile
	pkgOwnsFile = func(path string) bool { return filepath.Base(path) == "owned.svg" }
	t.Cleanup(func() { pkgOwnsFile = oldOwner })
	for _, name := range []string{"logo.svg", "edited.svg", "owned.svg", "linked.svg", "nested/dark.png"} {
		writeFile(t, filepath.Join(repo, "ryoku/assets/brand", name), "original")
		writeFile(t, filepath.Join(root, "usr/share/ryoku/brand", name), "original")
	}
	writeFile(t, filepath.Join(root, "usr/share/ryoku/brand/edited.svg"), "custom")
	linked := filepath.Join(root, "usr/share/ryoku/brand/linked.svg")
	if err := os.Remove(linked); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(repo, "ryoku/assets/brand/linked.svg"), linked); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(repo, "ryoku/apps/npm/npmrc"), "prefix=${HOME}/.local")
	writeFile(t, filepath.Join(root, "usr/share/ryoku/npmrc"), "prefix=${HOME}/.local")
	if result := reconcileCheckoutAssets(true, repo, root); result.status != recWouldFix {
		t.Fatalf("check: %+v", result)
	}
	logo := filepath.Join(root, "usr/share/ryoku/brand/logo.svg")
	if _, err := os.Stat(logo); err != nil {
		t.Fatal("check-only removed an asset")
	}
	if result := reconcileCheckoutAssets(false, repo, root); result.status != recFixed {
		t.Fatalf("apply: %+v", result)
	}
	for _, path := range []string{"usr/share/ryoku/brand/logo.svg", "usr/share/ryoku/brand/nested/dark.png", "usr/share/ryoku/npmrc"} {
		if _, err := os.Lstat(filepath.Join(root, path)); !os.IsNotExist(err) {
			t.Fatalf("unowned identical asset remains: %s (%v)", path, err)
		}
	}
	for _, name := range []string{"edited.svg", "owned.svg", "linked.svg"} {
		if _, err := os.Lstat(filepath.Join(root, "usr/share/ryoku/brand", name)); err != nil {
			t.Fatalf("protected asset removed: %s (%v)", name, err)
		}
	}
	if result := reconcileCheckoutAssets(false, repo, root); result.status != recOK {
		t.Fatalf("second apply: %+v", result)
	}
}
