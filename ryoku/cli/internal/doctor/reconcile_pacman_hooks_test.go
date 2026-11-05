package doctor

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func stubInstallerPacmanHooks(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	oldDir := installerPacmanHooksDir
	oldLstat, oldReadlink := installerPacmanHookLstat, installerPacmanHookReadlink
	oldRemove, oldRestore := installerPacmanHookRemove, installerPacmanHookRestore
	oldActive, oldBoot := installerPacmanHooksActive, installerPacmanHookBootRepair
	oldHasPacman := hasPacman
	installerPacmanHooksDir = dir
	installerPacmanHookLstat = os.Lstat
	installerPacmanHookReadlink = os.Readlink
	installerPacmanHookRemove = func(paths ...string) error {
		for _, path := range paths {
			if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
				return err
			}
		}
		return nil
	}
	installerPacmanHookRestore = os.Rename
	installerPacmanHooksActive = func() bool { return false }
	installerPacmanHookBootRepair = func(bool) recResult { return okRes("boot images current") }
	hasPacman = func() bool { return true }
	t.Cleanup(func() {
		installerPacmanHooksDir = oldDir
		installerPacmanHookLstat, installerPacmanHookReadlink = oldLstat, oldReadlink
		installerPacmanHookRemove, installerPacmanHookRestore = oldRemove, oldRestore
		installerPacmanHooksActive, installerPacmanHookBootRepair = oldActive, oldBoot
		hasPacman = oldHasPacman
	})
	return dir
}

func writeInstallerMask(t *testing.T, dir, name string) {
	t.Helper()
	if err := os.Symlink("/dev/null", filepath.Join(dir, name)); err != nil {
		t.Fatal(err)
	}
}

func TestReconcileInstallerPacmanHooksRestoresEveryArtifact(t *testing.T) {
	dir := stubInstallerPacmanHooks(t)
	writeInstallerMask(t, dir, "05-snap-pac-pre.hook")
	writeInstallerMask(t, dir, "60-mkinitcpio-remove.hook")
	writeInstallerMask(t, dir, "90-mkinitcpio-install.hook")
	if err := os.WriteFile(filepath.Join(dir, "90-mkinitcpio-install.hook.ryoku-off"), []byte("parked hook\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "80-limine-efi-deploy.hook"), []byte("new package hook\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "80-limine-efi-deploy.hook.ryoku-off"), []byte("old package hook\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	bootRebuilt := 0
	installerPacmanHookBootRepair = func(checkOnly bool) recResult {
		if checkOnly {
			t.Fatal("repair path called the boot reconciler in check-only mode")
		}
		bootRebuilt++
		return fixedRes("rebuilt stale kernel image")
	}

	res := reconcileInstallerPacmanHooks(false)
	if res.status != recFixed || !strings.Contains(res.detail, "rebuilt stale kernel image") {
		t.Fatalf("repair result = %#v, want masks restored and stale image rebuilt", res)
	}
	if bootRebuilt != 1 {
		t.Fatalf("boot repair ran %d times, want once", bootRebuilt)
	}
	for _, name := range []string{"05-snap-pac-pre.hook", "60-mkinitcpio-remove.hook"} {
		if _, err := os.Lstat(filepath.Join(dir, name)); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("%s remains after repair", name)
		}
	}
	got, err := os.ReadFile(filepath.Join(dir, "90-mkinitcpio-install.hook"))
	if err != nil || string(got) != "parked hook\n" {
		t.Fatalf("parked mkinitcpio hook was not restored: %q, %v", got, err)
	}
	got, err = os.ReadFile(filepath.Join(dir, "80-limine-efi-deploy.hook"))
	if err != nil || string(got) != "new package hook\n" {
		t.Fatalf("current package hook was replaced: %q, %v", got, err)
	}
	for _, name := range []string{"80-limine-efi-deploy.hook.ryoku-off", "90-mkinitcpio-install.hook.ryoku-off"} {
		if _, err := os.Lstat(filepath.Join(dir, name)); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("%s remains after repair", name)
		}
	}
}

func TestReconcileInstallerPacmanHooksCheckAndInstallGuards(t *testing.T) {
	dir := stubInstallerPacmanHooks(t)
	writeInstallerMask(t, dir, "zz-snap-pac-post.hook")
	bootCalls := 0
	installerPacmanHookBootRepair = func(bool) recResult { bootCalls++; return okRes("unused") }

	if res := reconcileInstallerPacmanHooks(true); res.status != recWouldFix {
		t.Fatalf("check result = %#v, want would-fix", res)
	}
	if _, err := os.Lstat(filepath.Join(dir, "zz-snap-pac-post.hook")); err != nil {
		t.Fatalf("check-only removed the mask: %v", err)
	}
	installerPacmanHooksActive = func() bool { return true }
	if res := reconcileInstallerPacmanHooks(false); res.status != recNote {
		t.Fatalf("active-install result = %#v, want note", res)
	}
	if _, err := os.Lstat(filepath.Join(dir, "zz-snap-pac-post.hook")); err != nil {
		t.Fatalf("active-install guard removed the mask: %v", err)
	}
	if bootCalls != 0 {
		t.Fatalf("guarded paths ran boot repair %d times", bootCalls)
	}
}

func TestReconcileInstallerPacmanHooksIsNeutralOffPacman(t *testing.T) {
	dir := stubInstallerPacmanHooks(t)
	writeInstallerMask(t, dir, "05-snap-pac-pre.hook")
	hasPacman = func() bool { return false }
	installerPacmanHookRemove = func(...string) error { t.Fatal("non-pacman host changed a hook"); return nil }
	if res := reconcileInstallerPacmanHooks(false); res.status != recOK {
		t.Fatalf("non-pacman result = %#v, want ok", res)
	}
}

func TestInstallerCommandRecognizesDirectAndShellLaunches(t *testing.T) {
	for _, cmdline := range [][]byte{
		[]byte("/usr/local/bin/ryoku-install\x00"),
		[]byte("/usr/bin/bash\x00/usr/local/lib/ryoku/backend/ryoku-install\x00"),
	} {
		if !installerCommand(cmdline) {
			t.Fatalf("installerCommand(%q) = false", cmdline)
		}
	}
	if installerCommand([]byte("/usr/bin/ryoku\x00doctor\x00ryoku-install\x00")) {
		t.Fatal("an installer name outside argv[0:2] must not block doctor")
	}
}
