package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestManagedFilesContent(t *testing.T) {
	files := managedFiles("nero", "/usr/bin/ryoku-hub")
	var hook *managedFile
	for i := range files {
		if files[i].rel == "etc/libvirt/hooks/qemu" {
			hook = &files[i]
		}
	}
	if hook == nil {
		t.Fatal("no libvirt hook in managed files")
	}
	if !strings.Contains(hook.content, "ryoku") {
		t.Error("hook must contain the ryoku marker (hookInstalled checks for it)")
	}
	if hook.mode != 0o755 {
		t.Errorf("hook mode = %o, want 0755", hook.mode)
	}
	if !containsFile(files, "etc/udev/rules.d/99-ryoku-kvmfr.rules", `OWNER="nero"`) {
		t.Error("kvmfr udev rule must own the device to the invoking user")
	}
}

func TestWriteManagedIdempotent(t *testing.T) {
	root := t.TempDir()
	f := managedFile{"etc/modules-load.d/ryoku-kvmfr.conf", "kvmfr\n", 0o644, true}
	if err := writeManaged(root, f); err != nil {
		t.Fatal(err)
	}
	if err := writeManaged(root, f); err != nil {
		t.Fatal(err) // same content = no-op, not an error
	}
	b, _ := os.ReadFile(filepath.Join(root, f.rel))
	if string(b) != "kvmfr\n" {
		t.Errorf("content = %q", b)
	}
}

func TestApplyPlanDryRunWritesNothing(t *testing.T) {
	root := t.TempDir()
	t.Setenv("RYOKU_ETC_ROOT", root)
	if err := applyPlan("enable", "nero", "/usr/bin/ryoku-hub", true); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "etc/libvirt/hooks/qemu")); err == nil {
		t.Error("a dry-run must not write any file")
	}
}
func TestSnapshotWritersSkipUnsupportedHost(t *testing.T) {
	bin := t.TempDir()
	marker := filepath.Join(bin, "snapper-called")
	if err := os.WriteFile(filepath.Join(bin, "ryoku-host"), []byte("#!/bin/sh\nexit 5\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bin, "snapper"), []byte("#!/bin/sh\nprintf called >> "+marker+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))

	snapshot("gpu change")
	snapshotBestEffort("import")
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("snapper ran on an unsupported host (stat err %v)", err)
	}
}

func containsFile(files []managedFile, rel, want string) bool {
	for _, f := range files {
		if f.rel == rel {
			return strings.Contains(f.content, want)
		}
	}
	return false
}

func TestMissingPkgs(t *testing.T) {
	have := map[string]bool{"looking-glass": true}
	got := missingPkgs([]string{"looking-glass", "looking-glass-module-dkms"}, func(p string) bool { return have[p] })
	if len(got) != 1 || got[0] != "looking-glass-module-dkms" {
		t.Errorf("missingPkgs = %v, want [looking-glass-module-dkms]", got)
	}
	if none := missingPkgs([]string{"a", "b"}, func(string) bool { return true }); len(none) != 0 {
		t.Errorf("all installed should be empty, got %v", none)
	}
}
