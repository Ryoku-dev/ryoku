package doctor

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ryoku-cli/internal/apprelease"
	"ryoku-cli/internal/host"
)

func TestRyotunesUsesNativeRepositoryOutsidePacman(t *testing.T) {
	for _, manager := range []host.PackageManager{host.DNF, host.XBPS} {
		t.Run(string(manager), func(t *testing.T) {
			oldManager := doctorPackageManager
			oldInstalled := doctorPackageInstalled
			oldAdvice := doctorInstallAdvice
			oldEnsure := ryotunesEnsure
			t.Cleanup(func() {
				doctorPackageManager = oldManager
				doctorPackageInstalled = oldInstalled
				doctorInstallAdvice = oldAdvice
				ryotunesEnsure = oldEnsure
			})
			isolateProvisioned(t)

			t.Setenv("HOME", t.TempDir())
			bin := t.TempDir()
			sudoLog := filepath.Join(bin, "sudo.log")
			if err := os.WriteFile(filepath.Join(bin, "sudo"), []byte("#!/bin/sh\nprintf '%s\\n' \"$*\" >\"$RYOTUNES_SUDO_LOG\"\n"), 0o755); err != nil {
				t.Fatal(err)
			}
			t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
			t.Setenv("RYOTUNES_SUDO_LOG", sudoLog)
			t.Setenv("XDG_STATE_HOME", t.TempDir())
			doctorPackageManager = func() (host.PackageManager, error) { return manager, nil }
			doctorPackageInstalled = func(name string) bool { return name == "ryoku-desktop" }
			doctorInstallAdvice = func(names ...string) string {
				return fmt.Sprintf("native-%s-install %s", manager, strings.Join(names, " "))
			}
			ryotunesEnsure = func(context.Context) (apprelease.Status, error) {
				t.Fatal("native repository package used the standalone pacman release lane")
				return apprelease.Status{}, nil
			}

			result := reconcileRyotunes(false)
			got, err := os.ReadFile(sudoLog)
			if err != nil {
				t.Fatal(err)
			}
			if command := strings.TrimSpace(string(got)); command != "ryoku-host pkg install ryotunes" {
				t.Fatalf("sudo command = %q", command)
			}
			if result.status != recFixed {
				t.Fatalf("status = %s, detail = %q", result.status.label(), result.detail)
			}
			if strings.Contains(result.remedy+result.detail, "pacman") {
				t.Fatalf("native result mentions pacman: %#v", result)
			}
		})
	}
}

func TestRyoportUsesNativeRepositoryOutsidePacman(t *testing.T) {
	oldManager := doctorPackageManager
	oldInstalled := doctorPackageInstalled
	oldEnsure := ryoportEnsure
	t.Cleanup(func() {
		doctorPackageManager = oldManager
		doctorPackageInstalled = oldInstalled
		ryoportEnsure = oldEnsure
	})
	isolateProvisioned(t)
	t.Setenv("HOME", t.TempDir())
	bin := t.TempDir()
	log := filepath.Join(bin, "sudo.log")
	if err := os.WriteFile(filepath.Join(bin, "sudo"), []byte("#!/bin/sh\nprintf '%s\\n' \"$*\" >\"$RYOPORT_SUDO_LOG\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("RYOPORT_SUDO_LOG", log)
	doctorPackageManager = func() (host.PackageManager, error) { return host.DNF, nil }
	doctorPackageInstalled = func(name string) bool { return name == "ryoku-desktop" }
	ryoportEnsure = func(context.Context) (apprelease.Status, error) {
		t.Fatal("native repository package used the standalone pacman release lane")
		return apprelease.Status{}, nil
	}

	result := reconcileRyoport(false)
	got, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	if command := strings.TrimSpace(string(got)); command != "ryoku-host pkg install ryoport" {
		t.Fatalf("sudo command = %q", command)
	}
	if result.status != recFixed {
		t.Fatalf("status = %s, detail = %q", result.status.label(), result.detail)
	}
}

func TestRyotunesNativeRepositoryCheckOnlyUsesHostAdvice(t *testing.T) {
	oldManager := doctorPackageManager
	oldInstalled := doctorPackageInstalled
	oldInstall := doctorPackageInstall
	oldAdvice := doctorInstallAdvice
	t.Cleanup(func() {
		doctorPackageManager = oldManager
		doctorPackageInstalled = oldInstalled
		doctorPackageInstall = oldInstall
		doctorInstallAdvice = oldAdvice
	})
	isolateProvisioned(t)

	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	doctorPackageManager = func() (host.PackageManager, error) { return host.DNF, nil }
	doctorPackageInstalled = func(name string) bool { return name == "ryoku-desktop" }
	doctorPackageInstall = func(...string) int {
		t.Fatal("check-only reconciliation installed a package")
		return host.ExitFailure
	}
	doctorInstallAdvice = func(...string) string { return "sudo dnf install -y ryotunes" }

	result := reconcileRyotunes(true)
	if result.status != recWouldFix {
		t.Fatalf("status = %s, want todo", result.status.label())
	}
	if result.remedy != "sudo dnf install -y ryotunes" {
		t.Fatalf("remedy = %q", result.remedy)
	}
}

func TestRyoportLegacyHiddenEntryRecordsRemoval(t *testing.T) {
	oldManager := doctorPackageManager
	oldInstalled := doctorPackageInstalled
	oldEnsure := ryoportEnsure
	t.Cleanup(func() {
		doctorPackageManager = oldManager
		doctorPackageInstalled = oldInstalled
		ryoportEnsure = oldEnsure
	})
	isolateProvisioned(t)
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, ".local/share"))
	desktop := filepath.Join(home, ".local/share/applications", legacyRyoportDesktopFile)
	if err := os.MkdirAll(filepath.Dir(desktop), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(desktop, []byte("[Desktop Entry]\nName=Ryoport\n# installer opt-out\nHidden=true\nExec=ryoport\n\n[Desktop Action Open]\nHidden=false\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	doctorPackageManager = func() (host.PackageManager, error) { return host.Pacman, nil }
	doctorPackageInstalled = func(name string) bool { return name == "ryoku-desktop" }
	ryoportEnsure = func(context.Context) (apprelease.Status, error) {
		t.Fatal("legacy deselection reinstalled Ryoport")
		return apprelease.Status{}, nil
	}

	result := reconcileRyoport(false)
	if result.status != recFixed {
		t.Fatalf("status = %s, detail = %q", result.status.label(), result.detail)
	}
	if !provisioned()["ryoport"] {
		t.Fatal("legacy deselection was not recorded in the provisioned ledger")
	}
	if _, err := os.Stat(desktop); !os.IsNotExist(err) {
		t.Fatalf("legacy hide entry still exists: %v", err)
	}
}

func TestRyoportLegacyNoDisplayEntryRecordsRemoval(t *testing.T) {
	oldManager := doctorPackageManager
	oldInstalled := doctorPackageInstalled
	oldEnsure := ryoportEnsure
	t.Cleanup(func() {
		doctorPackageManager = oldManager
		doctorPackageInstalled = oldInstalled
		ryoportEnsure = oldEnsure
	})
	isolateProvisioned(t)
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, ".local/share"))
	desktop := filepath.Join(home, ".local/share/applications", legacyRyoportDesktopFile)
	if err := os.MkdirAll(filepath.Dir(desktop), 0o755); err != nil {
		t.Fatal(err)
	}
	override, err := host.NoDisplayDesktopEntry([]byte("[Desktop Entry]\nName=Ryoport\nExec=ryoport\nActions=Open;\n\n[Desktop Action Open]\nName=Open\nExec=ryoport\n"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(desktop, override, 0o644); err != nil {
		t.Fatal(err)
	}
	doctorPackageManager = func() (host.PackageManager, error) { return host.Pacman, nil }
	doctorPackageInstalled = func(name string) bool { return name == "ryoku-desktop" }
	ryoportEnsure = func(context.Context) (apprelease.Status, error) {
		t.Fatal("legacy deselection reinstalled Ryoport")
		return apprelease.Status{}, nil
	}

	result := reconcileRyoport(false)
	if result.status != recFixed {
		t.Fatalf("status = %s, detail = %q", result.status.label(), result.detail)
	}
	if !provisioned()["ryoport"] {
		t.Fatal("legacy deselection was not recorded in the provisioned ledger")
	}
	if _, err := os.Stat(desktop); !os.IsNotExist(err) {
		t.Fatalf("legacy hide entry still exists: %v", err)
	}
}

func TestRyoportLegacyEntryWithoutHideKeyIsNotRemoval(t *testing.T) {
	oldManager := doctorPackageManager
	oldInstalled := doctorPackageInstalled
	oldEnsure := ryoportEnsure
	t.Cleanup(func() {
		doctorPackageManager = oldManager
		doctorPackageInstalled = oldInstalled
		ryoportEnsure = oldEnsure
	})
	isolateProvisioned(t)
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, ".local/share"))
	desktop := filepath.Join(home, ".local/share/applications", legacyRyoportDesktopFile)
	if err := os.MkdirAll(filepath.Dir(desktop), 0o755); err != nil {
		t.Fatal(err)
	}
	body := []byte("[Desktop Entry]\nName=Ryoport\nExec=ryoport\nActions=Open;\n\n[Desktop Action Open]\nName=Open\nExec=ryoport\n")
	if err := os.WriteFile(desktop, body, 0o644); err != nil {
		t.Fatal(err)
	}
	doctorPackageManager = func() (host.PackageManager, error) { return host.Pacman, nil }
	doctorPackageInstalled = func(name string) bool { return name == "ryoku-desktop" }
	called := false
	ryoportEnsure = func(context.Context) (apprelease.Status, error) {
		called = true
		return apprelease.Status{Installed: "1.0.0-1", Latest: "1.0.0-1", Updated: true}, nil
	}

	result := reconcileRyoport(false)
	if result.status != recFixed || !called {
		t.Fatalf("result = %#v, release lane called = %v", result, called)
	}
	if got, err := os.ReadFile(desktop); err != nil || string(got) != string(body) {
		t.Fatalf("user desktop entry changed: body = %q, error = %v", got, err)
	}
}

func TestRyoportMissingManagedPackageUsesReleaseLane(t *testing.T) {
	oldManager := doctorPackageManager
	oldInstalled := doctorPackageInstalled
	oldEnsure := ryoportEnsure
	t.Cleanup(func() {
		doctorPackageManager = oldManager
		doctorPackageInstalled = oldInstalled
		ryoportEnsure = oldEnsure
	})
	isolateProvisioned(t)
	t.Setenv("HOME", t.TempDir())
	doctorPackageManager = func() (host.PackageManager, error) { return host.Pacman, nil }
	doctorPackageInstalled = func(name string) bool { return name == "ryoku-desktop" }
	called := false
	ryoportEnsure = func(context.Context) (apprelease.Status, error) {
		called = true
		return apprelease.Status{Installed: "1.0.0-1", Latest: "1.0.0-1", Updated: true}, nil
	}

	result := reconcileRyoport(false)
	if result.status != recFixed || !called {
		t.Fatalf("result = %#v, release lane called = %v", result, called)
	}
	if !provisioned()["ryoport"] {
		t.Fatal("installed Ryoport was not recorded in the provisioned ledger")
	}
}
