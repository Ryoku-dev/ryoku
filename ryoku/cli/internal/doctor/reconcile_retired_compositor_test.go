package doctor

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"ryoku-cli/internal/host"

	wm "ryoku-wm"
)

func stubRetiredCompositor(t *testing.T, installed map[string]bool) (*[][]string, string) {
	t.Helper()
	oldInstalled := retiredCompositorPackageInstalled
	oldRun := retiredCompositorRun
	oldRemoveAll := retiredCompositorRemoveAll
	oldRemove := retiredCompositorRemove
	oldHome := retiredCompositorHome
	oldManager := doctorPackageManager
	t.Cleanup(func() {
		retiredCompositorPackageInstalled = oldInstalled
		retiredCompositorRun = oldRun
		retiredCompositorRemoveAll = oldRemoveAll
		retiredCompositorRemove = oldRemove
		retiredCompositorHome = oldHome
		doctorPackageManager = oldManager
	})

	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("RYOKU_WM", wm.ProviderNiri)
	for _, legacy := range wm.RetiredCompositors() {
		t.Setenv(legacy.SessionHandle, "")
	}
	retiredCompositorHome = func() (string, error) { return home, nil }
	doctorPackageManager = func() (host.PackageManager, error) { return host.Pacman, nil }
	retiredCompositorPackageInstalled = func(name string) bool { return installed[name] }
	calls := &[][]string{}
	retiredCompositorRun = func(args ...string) error {
		*calls = append(*calls, append([]string(nil), args...))
		if len(args) >= 4 && args[1] == "-Rdd" {
			installed[args[len(args)-1]] = false
		}
		if len(args) >= 4 && args[1] == "-Rns" {
			for _, pkg := range args[3:] {
				installed[pkg] = false
			}
		}
		if len(args) >= 2 && args[1] == "-S" {
			installed[args[len(args)-1]] = true
		}
		return nil
	}
	return calls, home
}

func TestReconcileRetiredCompositorSwapsVariantBeforeCleanup(t *testing.T) {
	legacy := wm.RetiredCompositors()[0]
	target := "ryoku-desktop-" + wm.ProviderNiri
	installed := map[string]bool{legacy.VariantPackage: true, legacy.Packages[0]: true}
	calls, home := stubRetiredCompositor(t, installed)

	config := filepath.Join(home, ".config", legacy.ConfigDir)
	binary := filepath.Join(home, ".local", "bin", legacy.ProviderBinary)
	if err := os.MkdirAll(config, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(binary), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(binary, []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}

	if got := reconcileRetiredCompositor(false); got.status != recFixed {
		t.Fatalf("status = %v, detail = %q", got.status, got.detail)
	}
	want := [][]string{
		{"pacman", "-Sw", "--needed", "--noconfirm", target},
		{"pacman", "-Rdd", "--noconfirm", legacy.VariantPackage},
		{"pacman", "-S", "--needed", "--noconfirm", target},
		{"pacman", "-Rns", "--noconfirm", legacy.Packages[0]},
	}
	if !reflect.DeepEqual(*calls, want) {
		t.Fatalf("calls = %v, want %v", *calls, want)
	}
	if !installed[target] || installed[legacy.VariantPackage] || installed[legacy.Packages[0]] {
		t.Fatalf("installed packages after migration = %v", installed)
	}
	if _, err := os.Stat(config); !os.IsNotExist(err) {
		t.Fatalf("retired config remains: %v", err)
	}
	if _, err := os.Stat(binary); !os.IsNotExist(err) {
		t.Fatalf("retired provider remains: %v", err)
	}
}

func TestReconcileRetiredCompositorDefersRemovalForLiveSession(t *testing.T) {
	legacy := wm.RetiredCompositors()[0]
	target := "ryoku-desktop-" + wm.ProviderNiri
	installed := map[string]bool{legacy.VariantPackage: true, legacy.Packages[0]: true}
	calls, home := stubRetiredCompositor(t, installed)
	t.Setenv(legacy.SessionHandle, filepath.Join(home, "session.sock"))
	config := filepath.Join(home, ".config", legacy.ConfigDir)
	if err := os.MkdirAll(config, 0o755); err != nil {
		t.Fatal(err)
	}

	if got := reconcileRetiredCompositor(false); got.status != recWarn {
		t.Fatalf("status = %v, detail = %q", got.status, got.detail)
	}
	want := [][]string{{"pacman", "-S", "--needed", "--noconfirm", target}}
	if !reflect.DeepEqual(*calls, want) {
		t.Fatalf("calls = %v, want only replacement install %v", *calls, want)
	}
	if !installed[legacy.VariantPackage] || !installed[legacy.Packages[0]] {
		t.Fatalf("live session packages were removed: %v", installed)
	}
	if _, err := os.Stat(config); err != nil {
		t.Fatalf("live session config was removed: %v", err)
	}
}

func TestReconcileWayfireUsesInstalledSupportedVariant(t *testing.T) {
	legacy := wm.RetiredCompositors()[1]
	installed := map[string]bool{
		legacy.VariantPackage:                  true,
		legacy.Packages[0]:                     true,
		"ryoku-desktop-" + wm.ProviderHyprland: true,
		"ryoku-desktop-" + wm.ProviderNiri:     true,
	}
	calls, _ := stubRetiredCompositor(t, installed)
	t.Setenv("RYOKU_WM", legacy.ConfigDir)
	if got := reconcileRetiredCompositor(false); got.status != recFixed ||
		!strings.Contains(got.detail, wm.ProviderHyprland) {
		t.Fatalf("result = %+v, want installed Hyprland variant", got)
	}
	want := [][]string{
		{"pacman", "-Rdd", "--noconfirm", legacy.VariantPackage},
		{"pacman", "-Rns", "--noconfirm", legacy.Packages[0]},
	}
	if !reflect.DeepEqual(*calls, want) {
		t.Fatalf("calls = %v, want %v", *calls, want)
	}
	if !installed["ryoku-desktop-"+wm.ProviderNiri] {
		t.Fatal("other supported variant was removed")
	}
}

func TestReconcileWayfireInstallsNiriIfHyprlandAbsent(t *testing.T) {
	legacy := wm.RetiredCompositors()[1]
	installed := map[string]bool{legacy.VariantPackage: true}
	calls, _ := stubRetiredCompositor(t, installed)
	t.Setenv("RYOKU_WM", legacy.ConfigDir)
	installed["ryoku-desktop-"+wm.ProviderNiri] = true
	if got := reconcileRetiredCompositor(false); got.status != recFixed ||
		!strings.Contains(got.detail, wm.ProviderNiri) {
		t.Fatalf("result = %+v, want installed niri variant", got)
	}
	if len(*calls) != 1 || (*calls)[0][1] != "-Rdd" {
		t.Fatalf("existing niri variant must stay installed: %v", *calls)
	}
}

func TestReconcileWayfireKeepsLiveSessionUntilRelogin(t *testing.T) {
	legacy := wm.RetiredCompositors()[1]
	installed := map[string]bool{legacy.VariantPackage: true}
	calls, home := stubRetiredCompositor(t, installed)
	t.Setenv("RYOKU_WM", legacy.ConfigDir)
	t.Setenv(legacy.SessionHandle, filepath.Join(home, "session.sock"))
	if got := reconcileRetiredCompositor(false); got.status != recWarn {
		t.Fatalf("result = %+v, want deferred removal", got)
	}
	if !installed[legacy.VariantPackage] || !installed["ryoku-desktop-"+wm.ProviderHyprland] {
		t.Fatalf("live session lost or replacement missing: %v", installed)
	}
	if len(*calls) != 1 || (*calls)[0][1] != "-S" {
		t.Fatalf("live session must only install replacement: %v", *calls)
	}
}

func TestReconcileRetiredCompositorIsNeutralOnXBPS(t *testing.T) {
	oldManager := doctorPackageManager
	doctorPackageManager = func() (host.PackageManager, error) { return host.XBPS, nil }
	t.Cleanup(func() { doctorPackageManager = oldManager })

	if got := reconcileRetiredCompositor(false); got.status != recOK {
		t.Fatalf("XBPS result = %s %q, want neutral", got.status.label(), got.detail)
	}
}
