package doctor

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"ryoku-cli/internal/host"
)

func kernelHeadersTree(t *testing.T, kernels map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for version, pkgbase := range kernels {
		dir := filepath.Join(root, version)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if pkgbase != "" {
			if err := os.WriteFile(filepath.Join(dir, "pkgbase"), []byte(pkgbase+"\n"), 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
	return root
}

func withKernelHeaderTestState(t *testing.T, kernels map[string]string, installed map[string]bool) {
	t.Helper()
	oldDir := kernelHeadersModulesDir
	oldReadDir, oldReadFile := kernelHeadersReadDir, kernelHeadersReadFile
	oldInstalled := kernelHeadersPackageInstalled
	oldHasPacman := hasPacman
	oldInstall, oldAdvice := doctorPackageInstall, doctorInstallAdvice
	kernelHeadersModulesDir = kernelHeadersTree(t, kernels)
	kernelHeadersReadDir, kernelHeadersReadFile = os.ReadDir, os.ReadFile
	kernelHeadersPackageInstalled = func(name string) bool { return installed[name] }
	hasPacman = func() bool { return true }
	doctorPackageInstall = func(...string) int { return host.ExitOK }
	doctorInstallAdvice = func(names ...string) string { return "install " + strings.Join(names, " ") }
	t.Cleanup(func() {
		kernelHeadersModulesDir = oldDir
		kernelHeadersReadDir, kernelHeadersReadFile = oldReadDir, oldReadFile
		kernelHeadersPackageInstalled = oldInstalled
		hasPacman = oldHasPacman
		doctorPackageInstall, doctorInstallAdvice = oldInstall, oldAdvice
	})
}

func TestInstalledKernelHeaderPackagesUsesInstalledPkgbases(t *testing.T) {
	installed := map[string]bool{
		"linux":                  true,
		"linux-cachyos-bore-lto": true,
	}
	withKernelHeaderTestState(t, map[string]string{
		"7.2.11-arch1-1":         "linux",
		"7.3.0-cachyos-bore-lto": "linux-cachyos-bore-lto",
		"stale":                  "linux-zen",
		"module-only":            "",
		"invalid":                "linux bad",
	}, installed)

	got, err := installedKernelHeaderPackages()
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"linux-cachyos-bore-lto-headers", "linux-headers"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("headers = %v, want %v", got, want)
	}
}

func TestReconcileKernelHeadersInstallsOnlyMissingMatches(t *testing.T) {
	installed := map[string]bool{
		"linux":                          true,
		"linux-headers":                  true,
		"linux-cachyos-bore-lto":         true,
		"linux-cachyos-bore-lto-headers": false,
	}
	withKernelHeaderTestState(t, map[string]string{
		"7.2.11-arch1-1":         "linux",
		"7.3.0-cachyos-bore-lto": "linux-cachyos-bore-lto",
	}, installed)
	var installedNow []string
	doctorPackageInstall = func(names ...string) int {
		installedNow = append(installedNow, names...)
		return host.ExitOK
	}

	got := reconcileKernelHeaders(false)
	if got.status != recFixed {
		t.Fatalf("result = %+v, want fixed", got)
	}
	want := []string{"linux-cachyos-bore-lto-headers"}
	if !reflect.DeepEqual(installedNow, want) {
		t.Fatalf("installed = %v, want %v", installedNow, want)
	}
}

func TestReconcileKernelHeadersCheckOnlyDoesNotInstall(t *testing.T) {
	installed := map[string]bool{"linux-cachyos": true}
	withKernelHeaderTestState(t, map[string]string{"7.3.0-cachyos": "linux-cachyos"}, installed)
	doctorPackageInstall = func(names ...string) int {
		t.Fatalf("check-only installed %v", names)
		return host.ExitOK
	}

	got := reconcileKernelHeaders(true)
	if got.status != recWouldFix || !strings.Contains(got.detail, "linux-cachyos-headers") ||
		got.remedy != "install linux-cachyos-headers" {
		t.Fatalf("result = %+v", got)
	}
}
