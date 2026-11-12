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

// fakeTools puts each named shell script alone on PATH, so the command under
// test runs these and nothing on the machine
func fakeTools(t *testing.T, scripts map[string]string) {
	t.Helper()
	bin := t.TempDir()
	for name, body := range scripts {
		if err := os.WriteFile(filepath.Join(bin, name), []byte("#!/bin/sh\n"+body+"\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", bin)
}

const modprobeRel, modprobeBefore = "etc/modprobe.d/ryoku-kvmfr.conf", "options kvmfr static_size_mb=64\n"

// enableWithUnwritableHook runs a real enable into a fresh /etc holding only a
// modprobe file and a read-only libvirt hooks dir; the hook is the last file
// written, so every other write lands before it fails
func enableWithUnwritableHook(t *testing.T) (etc string, err error) {
	t.Helper()
	if os.Geteuid() == 0 {
		t.Skip("root ignores directory permissions")
	}
	etc = t.TempDir()
	t.Setenv("RYOKU_ETC_ROOT", etc)
	fakeTools(t, map[string]string{"ryoku-host": "exit 0", "modinfo": "exit 0"})
	seed(t, filepath.Join(etc, modprobeRel), modprobeBefore)
	hooks := filepath.Join(etc, "etc/libvirt/hooks")
	mustMkdir(t, hooks)
	if err := os.Chmod(hooks, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(hooks, 0o755) })
	return etc, applyPlan("enable", "ryoku-test-nobody", "/usr/bin/ryoku-hub", false)
}

// an enable that fails partway puts back every /etc file it had written
func TestApplyEnableFailureLeavesNoTrace(t *testing.T) {
	etc, err := enableWithUnwritableHook(t)
	if err == nil {
		t.Fatal("enable must fail when the libvirt hook cannot be written")
	}
	if got := readFile(filepath.Join(etc, modprobeRel)); got != modprobeBefore {
		t.Errorf("/%s = %q, want it put back to %q", modprobeRel, got, modprobeBefore)
	}
	for _, f := range managedFiles("ryoku-test-nobody", "/usr/bin/ryoku-hub") {
		if f.rel != modprobeRel && fileExists(filepath.Join(etc, f.rel)) {
			t.Errorf("a failed enable left /%s behind", f.rel)
		}
	}
}

// the rollback also removes the directories the enable made, and only those
func TestApplyEnableFailureRemovesCreatedDirs(t *testing.T) {
	etc, err := enableWithUnwritableHook(t)
	if err == nil {
		t.Fatal("enable must fail when the libvirt hook cannot be written")
	}
	for _, dir := range []string{"etc/modules-load.d", "etc/udev", "etc/polkit-1"} {
		if fileExists(filepath.Join(etc, dir)) {
			t.Errorf("a failed enable left the /%s it created", dir)
		}
	}
	for _, dir := range []string{"etc/modprobe.d", "etc/libvirt/hooks"} {
		if !fileExists(filepath.Join(etc, dir)) {
			t.Errorf("the rollback removed /%s, which was there before", dir)
		}
	}
}

// steps after the writes that do not take are each named, with the files left
// in place said plainly, instead of reporting done
func TestApplyEnableReportsFailedSteps(t *testing.T) {
	etc := t.TempDir()
	t.Setenv("RYOKU_ETC_ROOT", etc)
	calls := filepath.Join(t.TempDir(), "calls")
	t.Setenv("CALLS", calls)
	fakeTools(t, map[string]string{
		"ryoku-host": `echo "ryoku-host $*" >> "$CALLS"; case "$*" in svc*) exit 1 ;; esac`,
		"modinfo":    "exit 0",
		"gpasswd":    `echo "gpasswd $*" >> "$CALLS"`,
		"udevadm":    `echo "udevadm $*" >> "$CALLS"`,
		"virsh":      `echo "virsh $*" >> "$CALLS"; echo "network not found"; exit 1`,
	})
	err := applyPlan("enable", "ryoku-test-nobody", "/usr/bin/ryoku-hub", false)
	if err == nil {
		t.Fatal("enable must fail when libvirtd and the default network do not take")
	}
	for _, want := range []string{"libvirtd.socket", "default network", "are in place"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error must name %q, got %v", want, err)
		}
	}
	for _, want := range []string{"gpasswd -a ryoku-test-nobody libvirt", "gpasswd -a ryoku-test-nobody kvm", "udevadm control --reload-rules"} {
		if !strings.Contains(readFile(calls), want) {
			t.Errorf("a failed step stopped %q; calls: %q", want, readFile(calls))
		}
	}
	for _, f := range managedFiles("ryoku-test-nobody", "/usr/bin/ryoku-hub") {
		if !fileExists(filepath.Join(etc, f.rel)) {
			t.Errorf("/%s is missing, but the error says the files are in place", f.rel)
		}
	}
}

// a disable that cannot remove one file still removes the rest and runs every
// step, then names what failed instead of reporting done
func TestApplyDisableReportsEveryFailure(t *testing.T) {
	etc := t.TempDir()
	t.Setenv("RYOKU_ETC_ROOT", etc)
	calls := filepath.Join(t.TempDir(), "calls")
	t.Setenv("CALLS", calls)
	fakeTools(t, map[string]string{
		"gpasswd": `echo "gpasswd $*" >> "$CALLS"`,
		"udevadm": `echo "udevadm $*" >> "$CALLS"; exit 1`,
	})
	files := managedFiles("ryoku-test-nobody", "/usr/bin/ryoku-hub")
	stuck := "etc/polkit-1/rules.d/50-ryoku-libvirt.rules"
	for _, f := range files {
		if f.rel == stuck {
			seed(t, filepath.Join(etc, f.rel, "keep"), "x") // a non-empty dir: remove fails even as root
			continue
		}
		seed(t, filepath.Join(etc, f.rel), f.content)
	}

	err := applyPlan("disable", "ryoku-test-nobody", "/usr/bin/ryoku-hub", false)
	if err == nil || !strings.Contains(err.Error(), stuck) || !strings.Contains(err.Error(), "udevadm") {
		t.Fatalf("disable must name every failure, got %v", err)
	}
	for _, f := range files {
		if f.rel != stuck && fileExists(filepath.Join(etc, f.rel)) {
			t.Errorf("one failed removal stopped the rest: /%s left", f.rel)
		}
	}
	if !strings.Contains(readFile(calls), "udevadm control --reload-rules") {
		t.Errorf("one failed removal stopped the udev reload; calls: %q", readFile(calls))
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

func TestPassthroughAvailabilityUsesAURGateOnArch(t *testing.T) {
	bin := t.TempDir()
	host := filepath.Join(bin, "ryoku-host")
	script := `#!/bin/sh
if [ "$1" = pkgmgr ]; then
	printf 'pacman\n'
	exit 0
fi
if [ "$1" = capabilities ]; then
	printf '%s\n' '{"aur":{"supported":true,"reason":""}}'
	exit 0
fi
if [ "$1" = pkg ] && [ "$2" = available ] && [ "$3" = --aur ]; then
	exit 0
fi
if [ "$1" = pkg ] && [ "$2" = available ] && { [ "$3" = looking-glass ] || [ "$3" = looking-glass-module-dkms ]; }; then
	exit 3
fi
exit 0
`
	if err := os.WriteFile(host, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)

	got := passthroughAvailability()
	if !got.Available || got.Package != "" || got.Reason != "" {
		t.Fatalf("availability = %#v, want available", got)
	}
}

func TestPassthroughAvailabilityReportsVoidPackageReason(t *testing.T) {
	bin := t.TempDir()
	host := filepath.Join(bin, "ryoku-host")
	script := `#!/bin/sh
if [ "$1" = pkgmgr ]; then
	printf 'xbps\n'
	exit 0
fi
if [ "$1" = capabilities ]; then
	printf '%s\n' '{"aur":{"supported":false,"reason":"The AUR is an Arch Linux service; Void installs come from XBPS."}}'
	exit 0
fi
if [ "$1" = pkg ] && [ "$2" = available ] && [ "$3" = looking-glass ]; then
	exit 5
fi
if [ "$1" = pkg ] && [ "$2" = why ] && [ "$3" = looking-glass ]; then
	printf '%s\n' "Looking Glass is not packaged for this system."
	exit 0
fi
exit 0
`
	if err := os.WriteFile(host, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)

	got := passthroughAvailability()
	if got.Available {
		t.Fatal("passthrough reported available")
	}
	if got.Package != "looking-glass" {
		t.Fatalf("package = %q, want looking-glass", got.Package)
	}
	if got.Reason != "Looking Glass is not packaged for this system." {
		t.Fatalf("reason = %q", got.Reason)
	}
	if err := runGpuApply([]string{"enable", "--dry-run"}); err == nil || err.Error() != got.Reason {
		t.Fatalf("enable error = %v, want host reason %q", err, got.Reason)
	}
}
