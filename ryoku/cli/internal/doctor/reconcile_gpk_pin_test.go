package doctor

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"ryoku-cli/internal/host"
)

func gpkPinCheckout(t *testing.T, pkgver string) string {
	t.Helper()
	repo := t.TempDir()
	if out, err := exec.Command("git", "-C", repo, "init", "-q").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v %s", err, out)
	}
	dir := filepath.Join(repo, "release", "packages", "gpk")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "PKGBUILD"), []byte("pkgname=gpk\npkgver="+pkgver+"\npkgrel=1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return repo
}

func TestReconcileGpkPin(t *testing.T) {
	oldInstalled := gpkInstalledVersion
	t.Cleanup(func() { gpkInstalledVersion = oldInstalled })
	t.Setenv("RYOKU_REPO", gpkPinCheckout(t, "0.7.0"))

	cases := []struct {
		manager   host.PackageManager
		installed string
		status    recStatus
		fix       string
	}{
		{host.Pacman, "0.6.10-1", recWarn, "ryoku deploy"},
		{host.Pacman, "", recWarn, "ryoku deploy"},
		{host.DNF, "0:0.6.10-1.fc44", recWarn, "ryoku update"},
		{host.XBPS, "0.6.10_1", recWarn, "ryoku update"},
		{host.Pacman, "0.7.0-1", recOK, ""},
		{host.Pacman, "0.7.1-1", recOK, ""},
		{host.Pacman, "0.10.0-1", recOK, ""},
	}
	for _, c := range cases {
		gpkInstalledVersion = func() (host.PackageManager, string) { return c.manager, c.installed }
		res := reconcileGpkPin(true)
		if res.status != c.status || !strings.HasPrefix(res.remedy, c.fix) {
			t.Errorf("%s %q: status %v remedy %q detail %q", c.manager, c.installed, res.status, res.remedy, res.detail)
		}
	}
}

// A packaged box gets gpk with ryoku-desktop and is never measured here.
func TestReconcileGpkPinPackagedBox(t *testing.T) {
	oldInstalled := gpkInstalledVersion
	t.Cleanup(func() { gpkInstalledVersion = oldInstalled })
	t.Setenv("RYOKU_REPO", "")
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	gpkInstalledVersion = func() (host.PackageManager, string) {
		t.Fatal("packaged box queried gpk")
		return "", ""
	}
	if res := reconcileGpkPin(true); res.status != recOK {
		t.Fatalf("packaged box: %+v", res)
	}
}

func TestUpstreamVersion(t *testing.T) {
	for in, want := range map[string]string{"1:0.6.10-1": "0.6.10", "0.6.10-1.fc44": "0.6.10", "0.6.10_1": "0.6.10", "0.7.0": "0.7.0", "": ""} {
		if got := upstreamVersion(in); got != want {
			t.Errorf("upstreamVersion(%q) = %q, want %q", in, got, want)
		}
	}
}
