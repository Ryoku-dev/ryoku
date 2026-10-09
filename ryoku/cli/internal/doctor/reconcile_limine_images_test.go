package doctor

import (
	"os"
	"path/filepath"
	"ryoku-cli/internal/host"
	"strings"
	"syscall"
	"testing"
	"time"
)

// Which kernel the countdown autoboots is decided from the box, never from the
// entry names: the install records the kernel it was built around, and the
// kernel this session booted is the next-best statement. No brand is preferred,
// so a plain install that added a second kernel keeps its menu order.
func TestPickKernelPath(t *testing.T) {
	both := []string{"Ryoku Linux/linux", "Ryoku Linux/linux-cachyos"}
	for _, c := range []struct {
		name   string
		paths  []string
		prefer []string
		want   string
	}{
		{"recorded kernel wins", both, []string{"linux-cachyos", "linux"}, "Ryoku Linux/linux-cachyos"},
		{"recorded stock kernel is honoured even beside a cachyos entry", both, []string{"linux"}, "Ryoku Linux/linux"},
		{"running kernel when nothing is recorded", both, []string{"", "linux-cachyos"}, "Ryoku Linux/linux-cachyos"},
		{"box says nothing: menu order, no brand preference", both, nil, "Ryoku Linux/linux"},
		{"a preferred kernel with no entry falls through", both, []string{"linux-lts"}, "Ryoku Linux/linux"},
		{"no kernel entries at all", nil, []string{"linux"}, ""},
	} {
		t.Run(c.name, func(t *testing.T) {
			if got := pickKernelPath(c.paths, c.prefer); got != c.want {
				t.Errorf("pickKernelPath = %q, want %q", got, c.want)
			}
		})
	}

	conf := "/Ryoku Linux\n  //linux\n  //linux-cachyos\n     //Snapshots\n"
	if got := limineFirstKernelPath(conf); got != "Ryoku Linux/linux" {
		t.Errorf("first kernel path = %q, want the stock linux kernel", got)
	}
	if got := limineDefaultKernelPath("/Ryoku Linux\n    protocol: linux\n"); got != "" {
		t.Errorf("flat menu should have no kernel path, got %q", got)
	}
}

// The autoboot reconciler must only fix a default_entry that cannot boot (a bare
// numeric index that loops on the collapsed layout, or none at all) and never
// reset a deliberate one; remember_last_entry is seeded only when missing.
func TestLimineEnsureAutobootPreservesUserChoices(t *testing.T) {
	conf := "timeout: 3\ndefault_entry: Ryoku Linux/linux\nremember_last_entry: no\n\n/Ryoku Linux\n  //linux\n  //linux-cachyos\n"
	got, changed := limineEnsureAutoboot(conf)
	if changed {
		t.Errorf("a valid user default_entry + remember must be left untouched:\n%s", got)
	}
	if !strings.Contains(got, "default_entry: Ryoku Linux/linux") {
		t.Errorf("user default_entry was reset:\n%s", got)
	}
	if !strings.Contains(got, "remember_last_entry: no") {
		t.Errorf("user remember_last_entry was reset:\n%s", got)
	}

	num := "timeout: 3\ndefault_entry: 2\n\n/Ryoku Linux\n  //linux\n  //linux-cachyos\n"
	fixed, ch := limineEnsureAutoboot(num)
	if !ch {
		t.Fatal("a numeric default on the nested layout loops and must be repointed")
	}
	// Which kernel it lands on comes from the box (TestPickKernelPath covers
	// that); here it only has to stop being an index that loops.
	if !strings.Contains(fixed, "default_entry: Ryoku Linux/linux") {
		t.Errorf("numeric default not repointed at a kernel entry:\n%s", fixed)
	}
	if !strings.Contains(fixed, "remember_last_entry: yes") {
		t.Errorf("remember_last_entry not seeded:\n%s", fixed)
	}
	if again, ch2 := limineEnsureAutoboot(fixed); ch2 || again != fixed {
		t.Error("limineEnsureAutoboot is not idempotent after the fix")
	}
}

// The reset-on-update bug: the reconcilers rewrote /boot/limine.conf and clobbered
// the user's edits. A merge must keep every value the box already carries -- the
// timeout, colours, wallpaper, default_entry, remember flag -- and only Ryoku's
// boot identity and the snapshot-safety flag stay forced.
func TestMergeLimineConfPreservesUserEdits(t *testing.T) {
	base := "timeout: 20\n" +
		"default_entry: Ryoku/linux-cachyos\n" +
		"remember_last_entry: no\n" +
		"interface_branding_color: 00FF00\n" +
		"term_background: 123456\n" +
		"wallpaper: boot():/my-wall.png\n" +
		"\n" +
		"/+Ryoku\n  //linux\n  //linux-cachyos\n"
	got := mergeLimineConf(base, "")
	for _, want := range []string{
		"timeout: 20",
		"default_entry: Ryoku/linux-cachyos",
		"remember_last_entry: no",
		"interface_branding_color: 00FF00",
		"term_background: 123456",
		"wallpaper: boot():/my-wall.png",
		"interface_branding: Ryoku Bootloader",
		"hash_mismatch_panic: no",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("merge dropped or reset %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "interface_branding_color: C75D2B") {
		t.Errorf("merge reset the user's accent colour to the shipped default:\n%s", got)
	}
	for _, k := range []string{"timeout:", "default_entry:", "remember_last_entry:", "term_background:", "wallpaper:", "interface_branding_color:", "interface_branding:"} {
		if n := strings.Count(got, "\n"+k) + boolToInt(strings.HasPrefix(got, k)); n != 1 {
			t.Errorf("global %q appears %d times, want 1:\n%s", k, n, got)
		}
	}
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func TestPlanLimineKernelImages(t *testing.T) {
	installed := map[string]installedKernel{
		"linux":         {version: "7.2-arch"},
		"linux-cachyos": {version: "7.2-cachy"},
	}
	entries := []limineBootKernel{
		{name: "linux", version: "7.2-arch", image: "/boot/EFI/Linux/ryoku_linux.efi", imageExists: true},
		{name: "linux-cachyos", version: "7.1-cachy", image: "/boot/EFI/Linux/ryoku_linux-cachyos.efi", imageExists: true}, // version drifted
		{name: "linux-lts", version: "6.6", image: "/boot/EFI/Linux/ryoku_linux-lts.efi", imageExists: true},               // not installed
	}
	stale, stray := planLimineKernelImages(installed, entries)
	if len(stale) != 1 || stale[0] != "linux-cachyos" {
		t.Errorf("stale = %v, want [linux-cachyos] (its entry names a version no longer installed)", stale)
	}
	if len(stray) != 1 || stray[0] != "linux-lts" {
		t.Errorf("stray = %v, want [linux-lts] (no such kernel installed)", stray)
	}

	// missing image and missing entry are both stale.
	entries2 := []limineBootKernel{
		{name: "linux", version: "7.2-arch", image: "/boot/EFI/Linux/ryoku_linux.efi", imageExists: false},
	}
	stale2, _ := planLimineKernelImages(installed, entries2)
	if len(stale2) != 2 || stale2[0] != "linux" || stale2[1] != "linux-cachyos" {
		t.Errorf("stale = %v, want [linux linux-cachyos] (missing image + kernel with no entry)", stale2)
	}
}

func TestGatherLimineBootKernels(t *testing.T) {
	esp := t.TempDir()
	if err := os.MkdirAll(filepath.Join(esp, "EFI", "Linux"), 0o755); err != nil {
		t.Fatal(err)
	}
	mkuki := func(name string, mod time.Time) {
		p := filepath.Join(esp, "EFI", "Linux", name)
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(p, mod, mod); err != nil {
			t.Fatal(err)
		}
	}
	now := time.Now()
	mkuki("ryoku_linux.efi", now)                           // newer than its kernel
	mkuki("ryoku_linux-cachyos.efi", now.Add(-2*time.Hour)) // older than its kernel

	installed := map[string]installedKernel{
		"linux":         {version: "7.2-arch", vmlinuz: now.Add(-time.Hour)},
		"linux-cachyos": {version: "7.2-cachy", vmlinuz: now},
	}
	conf := "/Ryoku Linux\n" +
		"  //linux\n" +
		"  comment: Kernel version: 7.2-arch\n" +
		"  protocol: efi\n" +
		"  path: boot():/EFI/Linux/ryoku_linux.efi#abc\n" +
		"  //linux-cachyos\n" +
		"  comment: Kernel version: 7.2-cachy\n" +
		"  protocol: efi\n" +
		"  path: boot():/EFI/Linux/ryoku_linux-cachyos.efi#def\n" +
		"     //Snapshots\n" +
		"     ///2026-01-01\n" +
		"     ////linux\n" +
		"     path: boot():/history/old.efi\n" +
		"/EFI fallback\n" +
		"  path: boot():/EFI/BOOT/BOOTX64.EFI\n"

	got := gatherLimineBootKernels(conf, esp, installed)
	if len(got) != 2 {
		t.Fatalf("gathered %d kernel entries, want 2 (linux, linux-cachyos), skipping Snapshots and the EFI fallback: %+v", len(got), got)
	}
	byName := map[string]limineBootKernel{}
	for _, e := range got {
		byName[e.name] = e
	}
	if e := byName["linux"]; e.version != "7.2-arch" || !e.imageExists || e.imageOlder {
		t.Errorf("linux entry = %+v, want version 7.2-arch, image present, not older", e)
	}
	if e := byName["linux-cachyos"]; e.version != "7.2-cachy" || !e.imageExists || !e.imageOlder {
		t.Errorf("cachyos entry = %+v, want version 7.2-cachy, image present, older than kernel", e)
	}

	stale, stray := planLimineKernelImages(installed, got)
	if len(stale) != 1 || stale[0] != "linux-cachyos" || len(stray) != 0 {
		t.Errorf("plan = stale %v stray %v, want stale [linux-cachyos] (stale image), no strays", stale, stray)
	}
}

func TestVoidKernelEntriesUseSeriesAndDracutImage(t *testing.T) {
	for version, want := range map[string]string{
		"6.12.58_1":    "linux6.12",
		"6.6.91_1":     "linux6.6",
		"not-a-kernel": "",
	} {
		if got := voidKernelEntryName(version); got != want {
			t.Errorf("voidKernelEntryName(%q) = %q, want %q", version, got, want)
		}
	}

	esp := t.TempDir()
	image := filepath.Join(esp, "initramfs-6.12.58_1.img")
	if err := os.WriteFile(image, []byte("dracut"), 0o644); err != nil {
		t.Fatal(err)
	}
	installed := map[string]installedKernel{"linux6.12": {version: "6.12.58_1"}}
	conf := "/Ryoku Linux\n" +
		"  //linux6.12\n" +
		"  comment: Kernel version: 6.12.58_1\n" +
		"  protocol: linux\n" +
		"  path: boot():/vmlinuz-6.12.58_1\n" +
		"  module_path: boot():/initramfs-6.12.58_1.img\n"
	got := gatherLimineBootKernels(conf, esp, installed)
	if len(got) != 1 || got[0].name != "linux6.12" || got[0].version != "6.12.58_1" ||
		got[0].image != image || !got[0].imageExists {
		t.Fatalf("Void kernel entry = %+v", got)
	}
}

func TestVoidLimineSnapshotDefaults(t *testing.T) {
	source := `TARGET_OS_NAME="Old"
ESP_PATH="/boot"
ENABLE_UKI=yes
MAX_SNAPSHOT_ENTRIES=10
SNAPSHOT_FORMAT_CHOICE=5
`
	got := limineSnapshotDefaults(source, "Ryoku Linux")
	want := "TARGET_OS_NAME=\"Ryoku Linux\"\nESP_PATH=\"/boot\"\nMAX_SNAPSHOT_ENTRIES=10\nSNAPSHOT_FORMAT_CHOICE=5\n"
	if got != want {
		t.Fatalf("snapshot defaults = %q, want %q", got, want)
	}
	if strings.Contains(got, "ENABLE_UKI") {
		t.Fatal("Void snapshot defaults retained an Arch UKI key")
	}
	if got := setLimineOSName("ESP_PATH=\"/boot\"\n", "Ryoku Linux"); !strings.Contains(got, `TARGET_OS_NAME="Ryoku Linux"`) {
		t.Fatalf("missing TARGET_OS_NAME was not inserted: %q", got)
	}
}

func TestVoidLimineSpecificChecksExplainDracutLayout(t *testing.T) {
	t.Setenv("RYOKU_HOST_PKGMGR", "xbps")
	oldManager := doctorPackageManager
	doctorPackageManager = func() (host.PackageManager, error) { return host.XBPS, nil }
	t.Cleanup(func() { doctorPackageManager = oldManager })

	for name, result := range map[string]recResult{
		"UKI tree":     reconcileLimineUKITree(true),
		"GPU trim":     reconcileInitramfsGPUTrim(true),
		"console keys": reconcileInitramfsConsoleKeys(true),
	} {
		if result.status != recOK || !strings.Contains(result.detail, "kernel plus a dracut initramfs image") {
			t.Errorf("%s = %#v, want a plain dracut-layout note", name, result)
		}
	}
}

func TestLimineToolingRequiresRyokuManagerBinary(t *testing.T) {
	stubPacmanHost(t, true)
	bin := t.TempDir()
	t.Setenv("PATH", bin)
	if limineManagedBoot() {
		t.Fatal("Limine package alone was treated as a Ryoku-managed boot")
	}
	tool := filepath.Join(bin, "limine-entry-tool")
	if err := os.WriteFile(tool, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if !limineManagedBoot() {
		t.Fatal("Ryoku Limine manager binary was not recognized")
	}
}

func TestLimineToolingUsesVoidKernelHookOnXBPS(t *testing.T) {
	t.Setenv("RYOKU_HOST_PKGMGR", "xbps")
	oldManager := doctorPackageManager
	oldInstalled := doctorPackageInstalled
	doctorPackageManager = func() (host.PackageManager, error) { return host.XBPS, nil }
	doctorPackageInstalled = func(name string) bool { return name == "limine" }
	t.Cleanup(func() {
		doctorPackageManager = oldManager
		doctorPackageInstalled = oldInstalled
	})
	if !limineManagedBoot() {
		t.Fatal("Void's packaged Limine kernel hook was not recognized")
	}
}

func TestLimineToolingIsNotManagedOnNonPacmanHost(t *testing.T) {
	stubPacmanHost(t, false)
	if limineManagedBoot() {
		t.Fatal("non-pacman host was treated as Ryoku-managed Limine")
	}
}

func TestLimineReconcilersAreNeutralWithoutRyokuTooling(t *testing.T) {
	old := limineManagedBoot
	limineManagedBoot = func() bool { return false }
	t.Cleanup(func() { limineManagedBoot = old })

	checks := []struct {
		name string
		run  func() recResult
	}{
		{"layout", func() recResult { return reconcileLimineLayout(false) }},
		{"boot entry", func() recResult { return reconcileLimineBootEntry(false) }},
		{"UKI tree", func() recResult { return reconcileLimineUKITree(false) }},
		{"OS name", func() recResult { return reconcileLimineOSName(false) }},
		{"autoboot", func() recResult { return reconcileLimineAutoboot(false) }},
		{"kernel images", func() recResult { return reconcileLimineKernelImages(false) }},
		{"dead entries", func() recResult { return reconcileLimineDeadEntries(false) }},
		{"alongside entry", func() recResult { return reconcileAlongsideBootEntry(false) }},
		{"boot space", func() recResult { return reconcileBootSpace(false) }},
		{"boot read-write", func() recResult { return reconcileBootRW(false) }},
	}
	for _, check := range checks {
		t.Run(check.name, func(t *testing.T) {
			if got := check.run(); got.status != recOK {
				t.Fatalf("result = %#v, want neutral ok", got)
			}
		})
	}
}

// TestGatherLimineBootKernelsInaccessibleImageNotStale tests that an inaccessible image
// (EACCES or EIO) is marked imageInaccessible and NOT marked stale by planLimineKernelImages.
func TestGatherLimineBootKernelsInaccessibleImageNotStale(t *testing.T) {
	origStat := limineKernelImageStat
	t.Cleanup(func() { limineKernelImageStat = origStat })

	limineKernelImageStat = func(p string) (os.FileInfo, error) {
		switch p {
		case "/boot/EFI/Linux/ryoku_linux.efi":
			return nil, os.ErrPermission // EACCES
		case "/boot/EFI/Linux/ryoku_linux-cachyos.efi":
			return nil, syscall.EIO // EIO
		default:
			return nil, os.ErrNotExist // ENOENT
		}
	}

	installed := map[string]installedKernel{
		"linux":         {version: "7.2-arch"},
		"linux-cachyos": {version: "7.2-cachy"},
		"linux-lts":     {version: "6.6"},
	}

	conf := "/Ryoku Linux\n" +
		"  //linux\n" +
		"  comment: Kernel version: 7.2-arch\n" +
		"  protocol: efi\n" +
		"  path: boot():/EFI/Linux/ryoku_linux.efi\n" +
		"  //linux-cachyos\n" +
		"  comment: Kernel version: 7.2-cachy\n" +
		"  protocol: efi\n" +
		"  path: boot():/EFI/Linux/ryoku_linux-cachyos.efi\n" +
		"  //linux-lts\n" +
		"  comment: Kernel version: 6.6\n" +
		"  protocol: efi\n" +
		"  path: boot():/EFI/Linux/ryoku_linux-lts.efi\n"

	got := gatherLimineBootKernels(conf, "/boot", installed)
	if len(got) != 3 {
		t.Fatalf("gathered %d kernel entries, want 3", len(got))
	}

	byName := map[string]limineBootKernel{}
	for _, e := range got {
		byName[e.name] = e
	}

	// linux had EACCES
	if e := byName["linux"]; !e.imageInaccessible || e.imageExists {
		t.Errorf("linux entry = %+v, want imageInaccessible=true, imageExists=false", e)
	}

	// linux-cachyos had EIO
	if e := byName["linux-cachyos"]; !e.imageInaccessible || e.imageExists {
		t.Errorf("linux-cachyos entry = %+v, want imageInaccessible=true, imageExists=false", e)
	}

	// linux-lts had genuine ENOENT
	if e := byName["linux-lts"]; e.imageInaccessible || e.imageExists {
		t.Errorf("linux-lts entry = %+v, want imageInaccessible=false, imageExists=false", e)
	}

	stale, stray := planLimineKernelImages(installed, got)
	// Only linux-lts is stale (missing image). The inaccessible ones must NOT be considered stale!
	if len(stale) != 1 || stale[0] != "linux-lts" {
		t.Errorf("stale = %v, want exactly [linux-lts] (inaccessible images must not be marked stale)", stale)
	}
	if len(stray) != 0 {
		t.Errorf("stray = %v, want none", stray)
	}
}

// TestReconcileLimineKernelImagesInaccessiblePreventsRebuild verifies that when an image
// cannot be inspected due to permission or I/O errors, reconcileLimineKernelImages warns
// and does NOT attempt to rebuild initramfs or prune stray images, even in apply mode.
func TestReconcileLimineKernelImagesInaccessiblePreventsRebuild(t *testing.T) {
	stubLimineBootGates(t)
	dir := t.TempDir()

	origESP, origPkg, origStat, origKernels, origRebuild, origPrune := limineESPConf, liminePkgInstalled, limineKernelImageStat, limineInstalledKernels, limineRebuildInitramfs, liminePruneStrayImages
	t.Cleanup(func() {
		limineESPConf, liminePkgInstalled, limineKernelImageStat, limineInstalledKernels, limineRebuildInitramfs, liminePruneStrayImages = origESP, origPkg, origStat, origKernels, origRebuild, origPrune
	})
	liminePkgInstalled = func(string) bool { return true }
	limineInstalledKernels = func() (map[string]installedKernel, []incompleteKernel, error) {
		return map[string]installedKernel{
			"linux": {version: "7.2-arch"},
		}, nil, nil
	}

	var rebuildCalled, pruneCalled bool
	limineRebuildInitramfs = func() error {
		rebuildCalled = true
		return nil
	}
	liminePruneStrayImages = func([]string) error {
		pruneCalled = true
		return nil
	}

	confPath := filepath.Join(dir, "limine.conf")
	conf := "/Ryoku Linux\n" +
		"  //linux\n" +
		"  comment: Kernel version: 7.2-arch\n" +
		"  protocol: efi\n" +
		"  path: boot():/EFI/Linux/ryoku_linux.efi\n"
	if err := os.WriteFile(confPath, []byte(conf), 0o644); err != nil {
		t.Fatal(err)
	}
	limineESPConf = confPath

	limineKernelImageStat = func(p string) (os.FileInfo, error) {
		return nil, os.ErrPermission // EACCES
	}

	// In apply mode (checkOnly = false). Must NOT rebuild or prune on uncertain state.
	res := reconcileLimineKernelImages(false)
	if res.status != recWarn {
		t.Fatalf("got status %d, want recWarn (%d)", res.status, recWarn)
	}
	if !strings.Contains(res.detail, "cannot inspect kernel boot images on /boot") || !strings.Contains(res.detail, "linux") {
		t.Errorf("unexpected detail: %s", res.detail)
	}
	if res.remedy != "sudo ryoku doctor --check" {
		t.Errorf("expected remedy 'sudo ryoku doctor --check', got %q", res.remedy)
	}
	if rebuildCalled {
		t.Error("rebuildInitramfs was called despite inaccessible image")
	}
	if pruneCalled {
		t.Error("pruneLimineStrayImages was called despite inaccessible image")
	}
}

// TestInstalledKernelVersions verifies that installedKernelVersions discovers valid
// kernels while recording incomplete trees (missing pkgbase or missing vmlinuz).
func TestInstalledKernelVersions(t *testing.T) {
	dir := t.TempDir()

	origDir := limineModulesDir
	t.Cleanup(func() { limineModulesDir = origDir })
	limineModulesDir = dir

	// 1. Valid kernel: has pkgbase and vmlinuz
	kGood := filepath.Join(dir, "6.12.1-arch1-1")
	if err := os.Mkdir(kGood, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(kGood, "pkgbase"), []byte("linux\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(kGood, "vmlinuz"), []byte("kernel"), 0o644); err != nil {
		t.Fatal(err)
	}

	// 2. Tree missing pkgbase
	kNoPkgbase := filepath.Join(dir, "6.12.1-cachyos")
	if err := os.Mkdir(kNoPkgbase, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(kNoPkgbase, "vmlinuz"), []byte("kernel"), 0o644); err != nil {
		t.Fatal(err)
	}

	// 3. Tree missing vmlinuz
	kNoVmlinuz := filepath.Join(dir, "6.11.0-zen")
	if err := os.Mkdir(kNoVmlinuz, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(kNoVmlinuz, "pkgbase"), []byte("linux-zen\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, inc, err := installedKernelVersions()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || got["linux"].version != "6.12.1-arch1-1" {
		t.Errorf("got %v, want exactly 1 valid kernel (linux: 6.12.1-arch1-1)", got)
	}
	if len(inc) != 2 {
		t.Fatalf("got %d incomplete trees, want 2: %+v", len(inc), inc)
	}
}

// TestInstalledKernelVersionsInaccessibleMetadata verifies that non-ENOENT errors
// (EACCES, EIO) reading pkgbase or stating vmlinuz return an error instead of silently skipping.
func TestInstalledKernelVersionsInaccessibleMetadata(t *testing.T) {
	dir := t.TempDir()

	origDir, origRead, origStat := limineModulesDir, limineReadFile, limineKernelImageStat
	t.Cleanup(func() {
		limineModulesDir, limineReadFile, limineKernelImageStat = origDir, origRead, origStat
	})
	limineModulesDir = dir

	k1 := filepath.Join(dir, "6.12-arch")
	if err := os.Mkdir(k1, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(k1, "pkgbase"), []byte("linux\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(k1, "vmlinuz"), []byte("kernel"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Case A: ReadFile on pkgbase fails with EACCES
	limineReadFile = func(p string) ([]byte, error) {
		return nil, os.ErrPermission
	}
	if _, _, err := installedKernelVersions(); err == nil {
		t.Error("expected error when pkgbase is unreadable, got nil")
	}

	// Case B: Stat on vmlinuz fails with EIO
	limineReadFile = os.ReadFile
	limineKernelImageStat = func(p string) (os.FileInfo, error) {
		if strings.HasSuffix(p, "vmlinuz") {
			return nil, syscall.EIO
		}
		return os.Stat(p)
	}
	if _, _, err := installedKernelVersions(); err == nil {
		t.Error("expected error when vmlinuz stat fails with EIO, got nil")
	}
}

// TestInstalledKernelVersionsUnavailableModulesDir verifies that a missing or unreadable
// /usr/lib/modules directory returns an error and does not falsely report okRes.
func TestInstalledKernelVersionsUnavailableModulesDir(t *testing.T) {
	dir := t.TempDir()

	origDir, origESP, origPkg := limineModulesDir, limineESPConf, liminePkgInstalled
	t.Cleanup(func() {
		limineModulesDir, limineESPConf, liminePkgInstalled = origDir, origESP, origPkg
	})
	liminePkgInstalled = func(string) bool { return true }
	limineModulesDir = filepath.Join(dir, "nonexistent-modules")

	confPath := filepath.Join(dir, "limine.conf")
	if err := os.WriteFile(confPath, []byte("/Ryoku Linux\n  //linux\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	limineESPConf = confPath

	// 1. installedKernelVersions directly returns an error
	_, _, err := installedKernelVersions()
	if err == nil {
		t.Fatal("expected error for unavailable modules dir, got nil")
	}

	// 2. reconcileLimineKernelImages warns and does not report false success
	res := reconcileLimineKernelImages(true)
	if res.status != recWarn {
		t.Fatalf("got status %d, want recWarn (%d)", res.status, recWarn)
	}
	if !strings.Contains(res.detail, "cannot inspect installed kernel metadata") {
		t.Errorf("unexpected detail: %s", res.detail)
	}
	if res.remedy != "sudo ryoku doctor --check" {
		t.Errorf("expected remedy 'sudo ryoku doctor --check', got %q", res.remedy)
	}
}

// TestReconcileLimineKernelImagesIncompleteDiscoveryPreventsPruneAndRebuild verifies that
// when installed kernel discovery encounters an error, reconcileLimineKernelImages warns
// and executes NEITHER rebuildInitramfs NOR pruneLimineStrayImages.
func TestReconcileLimineKernelImagesIncompleteDiscoveryPreventsPruneAndRebuild(t *testing.T) {
	stubLimineBootGates(t)
	dir := t.TempDir()

	origESP, origPkg, origKernels, origRebuild, origPrune := limineESPConf, liminePkgInstalled, limineInstalledKernels, limineRebuildInitramfs, liminePruneStrayImages
	t.Cleanup(func() {
		limineESPConf, liminePkgInstalled, limineInstalledKernels, limineRebuildInitramfs, liminePruneStrayImages = origESP, origPkg, origKernels, origRebuild, origPrune
	})
	liminePkgInstalled = func(string) bool { return true }

	limineInstalledKernels = func() (map[string]installedKernel, []incompleteKernel, error) {
		return nil, nil, os.ErrPermission
	}

	var rebuildCalled, pruneCalled bool
	limineRebuildInitramfs = func() error {
		rebuildCalled = true
		return nil
	}
	liminePruneStrayImages = func([]string) error {
		pruneCalled = true
		return nil
	}

	confPath := filepath.Join(dir, "limine.conf")
	conf := "/Ryoku Linux\n" +
		"  //linux\n" +
		"  comment: Kernel version: 7.2-arch\n" +
		"  protocol: efi\n" +
		"  path: boot():/EFI/Linux/ryoku_linux.efi\n" +
		"  //linux-cachyos\n" +
		"  comment: Kernel version: 7.2-cachy\n" +
		"  protocol: efi\n" +
		"  path: boot():/EFI/Linux/ryoku_linux-cachyos.efi\n"
	if err := os.WriteFile(confPath, []byte(conf), 0o644); err != nil {
		t.Fatal(err)
	}
	limineESPConf = confPath

	resCheck := reconcileLimineKernelImages(true)
	if resCheck.status != recWarn {
		t.Fatalf("check mode: got status %d, want recWarn (%d)", resCheck.status, recWarn)
	}
	if !strings.Contains(resCheck.detail, "cannot inspect installed kernel metadata") {
		t.Errorf("check mode: unexpected detail: %s", resCheck.detail)
	}

	resApply := reconcileLimineKernelImages(false)
	if resApply.status != recWarn {
		t.Fatalf("apply mode: got status %d, want recWarn (%d)", resApply.status, recWarn)
	}
	if rebuildCalled {
		t.Error("apply mode: rebuildInitramfs was called despite incomplete kernel discovery")
	}
	if pruneCalled {
		t.Error("apply mode: pruneLimineStrayImages was called despite incomplete kernel discovery")
	}
}

// TestIncompleteKernelMetadataBlocksDestructivePruning tests missing pkgbase,
// missing vmlinuz, and empty pkgbase, asserting no mutations occur under uncertainty.
func TestIncompleteKernelMetadataBlocksDestructivePruning(t *testing.T) {
	stubLimineBootGates(t)
	for _, tc := range []struct {
		name       string
		incomplete []incompleteKernel
	}{
		{
			name:       "missing pkgbase",
			incomplete: []incompleteKernel{{dir: "6.12.1-cachyos", reason: "missing pkgbase"}},
		},
		{
			name:       "missing vmlinuz",
			incomplete: []incompleteKernel{{dir: "6.12.1-cachyos", pkgbase: "linux-cachyos", reason: "missing vmlinuz"}},
		},
		{
			name:       "empty pkgbase",
			incomplete: []incompleteKernel{{dir: "6.12.1-cachyos", reason: "empty pkgbase"}},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()

			origESP, origPkg, origKernels, origStat, origRebuild, origPrune := limineESPConf, liminePkgInstalled, limineInstalledKernels, limineKernelImageStat, limineRebuildInitramfs, liminePruneStrayImages
			t.Cleanup(func() {
				limineESPConf, liminePkgInstalled, limineInstalledKernels, limineKernelImageStat, limineRebuildInitramfs, liminePruneStrayImages = origESP, origPkg, origKernels, origStat, origRebuild, origPrune
			})
			goodImg := filepath.Join(dir, "ryoku_linux.efi")
			if err := os.WriteFile(goodImg, []byte("img"), 0o644); err != nil {
				t.Fatal(err)
			}
			limineKernelImageStat = func(p string) (os.FileInfo, error) {
				if strings.HasSuffix(p, "ryoku_linux.efi") {
					return os.Stat(goodImg)
				}
				return nil, os.ErrNotExist
			}
			limineInstalledKernels = func() (map[string]installedKernel, []incompleteKernel, error) {
				return map[string]installedKernel{
					"linux": {version: "7.2-arch"},
				}, tc.incomplete, nil
			}

			var rebuildCalled, pruneCalled bool
			limineRebuildInitramfs = func() error {
				rebuildCalled = true
				return nil
			}
			liminePruneStrayImages = func([]string) error {
				pruneCalled = true
				return nil
			}

			confPath := filepath.Join(dir, "limine.conf")
			conf := "/Ryoku Linux\n" +
				"  //linux\n" +
				"  comment: Kernel version: 7.2-arch\n" +
				"  protocol: efi\n" +
				"  path: boot():/EFI/Linux/ryoku_linux.efi\n" +
				"  //linux-cachyos\n" +
				"  comment: Kernel version: 7.2-cachy\n" +
				"  protocol: efi\n" +
				"  path: boot():/EFI/Linux/ryoku_linux-cachyos.efi\n"
			if err := os.WriteFile(confPath, []byte(conf), 0o644); err != nil {
				t.Fatal(err)
			}
			limineESPConf = confPath

			// Check mode: must be recWarn, never recWouldFix or recOK
			resCheck := reconcileLimineKernelImages(true)
			if resCheck.status != recWarn {
				t.Fatalf("check mode: got status %d, want recWarn (%d)", resCheck.status, recWarn)
			}
			if !strings.Contains(resCheck.detail, "cannot verify installed kernel metadata") || !strings.Contains(resCheck.detail, "incomplete module tree") {
				t.Errorf("check mode: expected incomplete module tree warning, got: %s", resCheck.detail)
			}
			if resCheck.remedy != "sudo ryoku doctor --check" {
				t.Errorf("check mode: expected remedy 'sudo ryoku doctor --check', got %q", resCheck.remedy)
			}

			// Apply mode: must be recWarn, zero rebuild and zero prune
			resApply := reconcileLimineKernelImages(false)
			if resApply.status != recWarn {
				t.Fatalf("apply mode: got status %d, want recWarn (%d)", resApply.status, recWarn)
			}
			if !strings.Contains(resApply.detail, "cannot verify installed kernel metadata") || !strings.Contains(resApply.detail, "incomplete module tree") {
				t.Errorf("apply mode: unexpected detail: %s", resApply.detail)
			}
			if pruneCalled {
				t.Errorf("apply mode: pruneLimineStrayImages called for incomplete kernel: %s", tc.name)
			}
			if rebuildCalled {
				t.Errorf("apply mode: rebuildInitramfs called: %s", tc.name)
			}
		})
	}
}

// TestReconcileLimineKernelImagesMixedIncompleteAndRemovedKernels verifies that
// when partial discovery occurs (e.g. valid, incomplete, and apparently removed kernels),
// the reconciler returns recWarn and guarantees ZERO rebuild and ZERO prune calls in apply mode.
func TestReconcileLimineKernelImagesMixedIncompleteAndRemovedKernels(t *testing.T) {
	stubLimineBootGates(t)
	dir := t.TempDir()

	origESP, origPkg, origKernels, origStat, origRebuild, origPrune := limineESPConf, liminePkgInstalled, limineInstalledKernels, limineKernelImageStat, limineRebuildInitramfs, liminePruneStrayImages
	t.Cleanup(func() {
		limineESPConf, liminePkgInstalled, limineInstalledKernels, limineKernelImageStat, limineRebuildInitramfs, liminePruneStrayImages = origESP, origPkg, origKernels, origStat, origRebuild, origPrune
	})
	liminePkgInstalled = func(string) bool { return true }
	limineKernelImageStat = func(string) (os.FileInfo, error) { return nil, os.ErrNotExist }

	// linux is installed & complete.
	// linux-cachyos is incomplete (e.g. missing vmlinuz).
	// linux-lts has no directory at all (apparently removed).
	limineInstalledKernels = func() (map[string]installedKernel, []incompleteKernel, error) {
		return map[string]installedKernel{
			"linux": {version: "7.2-arch"},
		}, []incompleteKernel{
			{dir: "6.12.1-cachyos", pkgbase: "linux-cachyos", reason: "missing vmlinuz"},
		}, nil
	}

	var prunedTargets []string
	var rebuildCalled bool
	liminePruneStrayImages = func(names []string) error {
		prunedTargets = append(prunedTargets, names...)
		return nil
	}
	limineRebuildInitramfs = func() error {
		rebuildCalled = true
		return nil
	}

	confPath := filepath.Join(dir, "limine.conf")
	conf := "/Ryoku Linux\n" +
		"  //linux\n" +
		"  comment: Kernel version: 7.2-arch\n" +
		"  protocol: efi\n" +
		"  path: boot():/EFI/Linux/ryoku_linux.efi\n" +
		"  //linux-cachyos\n" +
		"  comment: Kernel version: 7.2-cachy\n" +
		"  protocol: efi\n" +
		"  path: boot():/EFI/Linux/ryoku_linux-cachyos.efi\n" +
		"  //linux-lts\n" +
		"  comment: Kernel version: 6.6\n" +
		"  protocol: efi\n" +
		"  path: boot():/EFI/Linux/ryoku_linux-lts.efi\n"
	if err := os.WriteFile(confPath, []byte(conf), 0o644); err != nil {
		t.Fatal(err)
	}
	limineESPConf = confPath

	// Check mode: returns recWarn, never recWouldFix or recOK
	resCheck := reconcileLimineKernelImages(true)
	if resCheck.status != recWarn {
		t.Fatalf("check mode: got status %d, want recWarn (%d)", resCheck.status, recWarn)
	}

	// Apply mode: returns recWarn and guarantees ZERO rebuild and ZERO prune calls
	resApply := reconcileLimineKernelImages(false)
	if resApply.status != recWarn {
		t.Fatalf("apply mode: got status %d, want recWarn (%d)", resApply.status, recWarn)
	}
	if len(prunedTargets) > 0 {
		t.Fatalf("apply mode: pruned targets %v, want NONE when discovery is partial", prunedTargets)
	}
	if rebuildCalled {
		t.Fatal("apply mode: rebuildInitramfs was called when discovery is partial")
	}
}

// TestReconcileLimineKernelImagesCompleteDiscoveryPrunesStrayAndRebuilds verifies that
// when discovery is complete, genuine missing-image and obsolete-entry handling is preserved.
func TestReconcileLimineKernelImagesCompleteDiscoveryPrunesStrayAndRebuilds(t *testing.T) {
	stubLimineBootGates(t)
	dir := t.TempDir()

	origESP, origPkg, origKernels, origStat, origRebuild, origPrune := limineESPConf, liminePkgInstalled, limineInstalledKernels, limineKernelImageStat, limineRebuildInitramfs, liminePruneStrayImages
	t.Cleanup(func() {
		limineESPConf, liminePkgInstalled, limineInstalledKernels, limineKernelImageStat, limineRebuildInitramfs, liminePruneStrayImages = origESP, origPkg, origKernels, origStat, origRebuild, origPrune
	})
	liminePkgInstalled = func(string) bool { return true }

	// linux is installed & complete, but its image is older than vmlinuz
	now := time.Now()
	goodImg := filepath.Join(dir, "ryoku_linux.efi")
	if err := os.WriteFile(goodImg, []byte("img"), 0o644); err != nil {
		t.Fatal(err)
	}
	limineKernelImageStat = func(p string) (os.FileInfo, error) {
		if strings.HasSuffix(p, "ryoku_linux.efi") {
			return os.Stat(goodImg)
		}
		return nil, os.ErrNotExist
	}
	limineInstalledKernels = func() (map[string]installedKernel, []incompleteKernel, error) {
		return map[string]installedKernel{
			"linux": {version: "7.2-arch", vmlinuz: now.Add(time.Hour)},
		}, nil, nil
	}

	var prunedTargets []string
	var rebuildCalled bool
	liminePruneStrayImages = func(names []string) error {
		prunedTargets = append(prunedTargets, names...)
		return nil
	}
	limineRebuildInitramfs = func() error {
		rebuildCalled = true
		return nil
	}

	confPath := filepath.Join(dir, "limine.conf")
	conf := "/Ryoku Linux\n" +
		"  //linux\n" +
		"  comment: Kernel version: 7.2-arch\n" +
		"  protocol: efi\n" +
		"  path: boot():/EFI/Linux/ryoku_linux.efi\n" +
		"  //linux-lts\n" +
		"  comment: Kernel version: 6.6\n" +
		"  protocol: efi\n" +
		"  path: boot():/EFI/Linux/ryoku_linux-lts.efi\n"
	if err := os.WriteFile(confPath, []byte(conf), 0o644); err != nil {
		t.Fatal(err)
	}
	limineESPConf = confPath

	// Check mode: wouldRes with both stale and stray identified
	resCheck := reconcileLimineKernelImages(true)
	if resCheck.status != recWouldFix {
		t.Fatalf("check mode: got status %d, want recWouldFix (%d)", resCheck.status, recWouldFix)
	}
	if !strings.Contains(resCheck.detail, "linux") || !strings.Contains(resCheck.detail, "linux-lts") {
		t.Errorf("check mode: unexpected detail: %s", resCheck.detail)
	}

	// Apply mode: rebuilds linux, prunes linux-lts
	resApply := reconcileLimineKernelImages(false)
	if resApply.status != recFixed {
		t.Fatalf("apply mode: got status %d, want recFixed (%d)", resApply.status, recFixed)
	}
	if !rebuildCalled {
		t.Error("apply mode: rebuildInitramfs was not called for stale image")
	}
	if len(prunedTargets) != 1 || prunedTargets[0] != "linux-lts" {
		t.Fatalf("apply mode: prunedTargets = %v, want [linux-lts]", prunedTargets)
	}
}

// TestInstalledKernelVersionsSymlinkValid verifies that a valid directory symlink
// containing pkgbase and vmlinuz is discovered correctly.
func TestInstalledKernelVersionsSymlinkValid(t *testing.T) {
	dir := t.TempDir()
	origDir := limineModulesDir
	t.Cleanup(func() { limineModulesDir = origDir })
	limineModulesDir = dir

	realTarget := t.TempDir()
	if err := os.WriteFile(filepath.Join(realTarget, "pkgbase"), []byte("linux\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(realTarget, "vmlinuz"), []byte("kernel"), 0o644); err != nil {
		t.Fatal(err)
	}

	linkPath := filepath.Join(dir, "6.12.1-arch1-1")
	if err := os.Symlink(realTarget, linkPath); err != nil {
		t.Fatal(err)
	}

	got, inc, err := installedKernelVersions()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(inc) != 0 {
		t.Fatalf("unexpected incomplete trees: %+v", inc)
	}
	if len(got) != 1 || got["linux"].version != "6.12.1-arch1-1" {
		t.Errorf("got %v, want exactly 1 valid kernel (linux: 6.12.1-arch1-1)", got)
	}
	if got["linux"].vmlinuz.IsZero() {
		t.Error("expected non-zero vmlinuz mtime")
	}
}

// TestInstalledKernelVersionsSymlinkBroken verifies that a broken directory symlink
// causes discovery to fail rather than silently succeed.
func TestInstalledKernelVersionsSymlinkBroken(t *testing.T) {
	dir := t.TempDir()
	origDir := limineModulesDir
	t.Cleanup(func() { limineModulesDir = origDir })
	limineModulesDir = dir

	brokenLink := filepath.Join(dir, "broken-kernel")
	if err := os.Symlink(filepath.Join(dir, "nonexistent-target"), brokenLink); err != nil {
		t.Fatal(err)
	}

	got, inc, err := installedKernelVersions()
	if err == nil {
		t.Fatalf("expected error for broken symlink, but discovery succeeded with got=%v inc=%v", got, inc)
	}
}

// TestInstalledKernelVersionsIgnoresRegularFilesAndFileSymlinks verifies that
// regular files and non-directory symlinks are ignored and do not create false kernel entries.
func TestInstalledKernelVersionsIgnoresRegularFilesAndFileSymlinks(t *testing.T) {
	dir := t.TempDir()
	origDir := limineModulesDir
	t.Cleanup(func() { limineModulesDir = origDir })
	limineModulesDir = dir

	// Valid kernel directory
	kGood := filepath.Join(dir, "6.12.1-arch1-1")
	if err := os.Mkdir(kGood, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(kGood, "pkgbase"), []byte("linux\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(kGood, "vmlinuz"), []byte("kernel"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Unrelated regular file
	regularFile := filepath.Join(dir, "modules.dep")
	if err := os.WriteFile(regularFile, []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Symlink to a regular file
	fileLink := filepath.Join(dir, "modules.alias")
	if err := os.Symlink(regularFile, fileLink); err != nil {
		t.Fatal(err)
	}

	got, inc, err := installedKernelVersions()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(inc) != 0 {
		t.Fatalf("unexpected incomplete trees: %+v", inc)
	}
	if len(got) != 1 || got["linux"].version != "6.12.1-arch1-1" {
		t.Errorf("got %v, want exactly 1 valid kernel (linux: 6.12.1-arch1-1)", got)
	}
}

// TestReconcileLimineKernelImagesMixedRealAndBrokenSymlink verifies that when a modules
// directory contains both a real kernel and a broken symlink, apply mode warns and invokes
// neither rebuildInitramfs nor pruneLimineStrayImages.
func TestReconcileLimineKernelImagesMixedRealAndBrokenSymlink(t *testing.T) {
	stubLimineBootGates(t)
	dir := t.TempDir()

	origDir, origESP, origPkg, origStat, origRebuild, origPrune := limineModulesDir, limineESPConf, liminePkgInstalled, limineKernelImageStat, limineRebuildInitramfs, liminePruneStrayImages
	t.Cleanup(func() {
		limineModulesDir, limineESPConf, liminePkgInstalled, limineKernelImageStat, limineRebuildInitramfs, liminePruneStrayImages = origDir, origESP, origPkg, origStat, origRebuild, origPrune
	})
	liminePkgInstalled = func(string) bool { return true }

	modDir := filepath.Join(dir, "modules")
	if err := os.Mkdir(modDir, 0o755); err != nil {
		t.Fatal(err)
	}
	limineModulesDir = modDir

	// Real kernel
	kGood := filepath.Join(modDir, "6.12.1-arch1-1")
	if err := os.Mkdir(kGood, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(kGood, "pkgbase"), []byte("linux\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(kGood, "vmlinuz"), []byte("kernel"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Broken symlink
	if err := os.Symlink(filepath.Join(dir, "nonexistent-target"), filepath.Join(modDir, "broken-kernel")); err != nil {
		t.Fatal(err)
	}

	confPath := filepath.Join(dir, "limine.conf")
	conf := "/Ryoku Linux\n" +
		"  //linux\n" +
		"  comment: Kernel version: 6.12.1-arch1-1\n" +
		"  protocol: efi\n" +
		"  path: boot():/EFI/Linux/ryoku_linux.efi\n" +
		"  //linux-lts\n" +
		"  comment: Kernel version: 6.6\n" +
		"  protocol: efi\n" +
		"  path: boot():/EFI/Linux/ryoku_linux-lts.efi\n"
	if err := os.WriteFile(confPath, []byte(conf), 0o644); err != nil {
		t.Fatal(err)
	}
	limineESPConf = confPath

	var rebuildCalled, pruneCalled bool
	limineRebuildInitramfs = func() error {
		rebuildCalled = true
		return nil
	}
	liminePruneStrayImages = func([]string) error {
		pruneCalled = true
		return nil
	}

	// Check mode
	resCheck := reconcileLimineKernelImages(true)
	if resCheck.status != recWarn {
		t.Fatalf("check mode: got status %d, want recWarn (%d)", resCheck.status, recWarn)
	}
	if !strings.Contains(resCheck.detail, "cannot inspect installed kernel metadata") {
		t.Errorf("check mode: unexpected detail: %s", resCheck.detail)
	}

	// Apply mode: must warn and invoke ZERO mutating operations
	resApply := reconcileLimineKernelImages(false)
	if resApply.status != recWarn {
		t.Fatalf("apply mode: got status %d, want recWarn (%d)", resApply.status, recWarn)
	}
	if !strings.Contains(resApply.detail, "cannot inspect installed kernel metadata") {
		t.Errorf("apply mode: unexpected detail: %s", resApply.detail)
	}
	if rebuildCalled {
		t.Error("apply mode: rebuildInitramfs was called despite broken symlink")
	}
	if pruneCalled {
		t.Error("apply mode: pruneLimineStrayImages was called despite broken symlink")
	}
}
