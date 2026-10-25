package doctor

import (
	"os"
	"path/filepath"
	"testing"
)

func nvidiaAutostartBox(t *testing.T) (home, user string) {
	t.Helper()
	home = t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "config"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(home, "state"))
	xdg := filepath.Join(home, "etc", "xdg")
	t.Setenv("XDG_CONFIG_DIRS", xdg)
	system := filepath.Join(xdg, "autostart")
	if err := os.MkdirAll(system, 0o755); err != nil {
		t.Fatal(err)
	}
	entry := "[Desktop Entry]\nType=Application\nExec=nvidia-settings -l\n"
	if err := os.WriteFile(filepath.Join(system, nvidiaSettingsAutostart), []byte(entry), 0o644); err != nil {
		t.Fatal(err)
	}
	return home, filepath.Join(home, "config", "autostart")
}

func TestNvidiaSettingsAutostartHiddenOnce(t *testing.T) {
	_, user := nvidiaAutostartBox(t)
	dst := filepath.Join(user, nvidiaSettingsAutostart)

	if r := reconcileNvidiaSettingsAutostart(true); r.status != recWouldFix {
		t.Fatalf("check: status=%s want would-fix (detail=%s)", r.status.label(), r.detail)
	}
	if _, err := os.Stat(dst); !os.IsNotExist(err) {
		t.Fatal("check-only run wrote the override")
	}

	if r := reconcileNvidiaSettingsAutostart(false); r.status != recFixed {
		t.Fatalf("fix: status=%s want fixed (detail=%s)", r.status.label(), r.detail)
	}
	if b, err := os.ReadFile(dst); err != nil || string(b) != hiddenAutostartEntry {
		t.Fatalf("override = %q, %v; want %q", b, err, hiddenAutostartEntry)
	}

	if r := reconcileNvidiaSettingsAutostart(false); r.status != recOK {
		t.Fatalf("second run: status=%s want ok (detail=%s)", r.status.label(), r.detail)
	}

	// A user who deletes the override gets the package's autostart back.
	if err := os.Remove(dst); err != nil {
		t.Fatal(err)
	}
	if r := reconcileNvidiaSettingsAutostart(false); r.status != recOK {
		t.Fatalf("after removal: status=%s want ok (detail=%s)", r.status.label(), r.detail)
	}
	if _, err := os.Stat(dst); !os.IsNotExist(err) {
		t.Fatal("a removed override came back")
	}
}

func TestNvidiaSettingsAutostartLeavesOtherBoxesAlone(t *testing.T) {
	home, user := nvidiaAutostartBox(t)
	if err := os.WriteFile(filepath.Join(home, ".nvidia-settings-rc"), []byte("# saved\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if r := reconcileNvidiaSettingsAutostart(false); r.status != recOK {
		t.Fatalf("saved rc: status=%s want ok (detail=%s)", r.status.label(), r.detail)
	}
	if _, err := os.Stat(filepath.Join(user, nvidiaSettingsAutostart)); !os.IsNotExist(err) {
		t.Fatal("hid the autostart although it has settings to load")
	}

	_, user = nvidiaAutostartBox(t)
	own := "[Desktop Entry]\nType=Application\nExec=nvidia-settings -l -c :0\n"
	if err := os.MkdirAll(user, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(user, nvidiaSettingsAutostart), []byte(own), 0o644); err != nil {
		t.Fatal(err)
	}
	if r := reconcileNvidiaSettingsAutostart(false); r.status != recOK {
		t.Fatalf("user entry: status=%s want ok (detail=%s)", r.status.label(), r.detail)
	}
	if b, _ := os.ReadFile(filepath.Join(user, nvidiaSettingsAutostart)); string(b) != own {
		t.Fatalf("the user's own autostart entry was rewritten:\n%s", b)
	}

	_, user = nvidiaAutostartBox(t)
	if err := os.Remove(filepath.Join(os.Getenv("XDG_CONFIG_DIRS"), "autostart", nvidiaSettingsAutostart)); err != nil {
		t.Fatal(err)
	}
	if r := reconcileNvidiaSettingsAutostart(false); r.status != recOK {
		t.Fatalf("no package entry: status=%s want ok (detail=%s)", r.status.label(), r.detail)
	}
	if _, err := os.Stat(user); !os.IsNotExist(err) {
		t.Fatal("wrote an override without the package's autostart")
	}
}
