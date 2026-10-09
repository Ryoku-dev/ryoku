package doctor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestReconcileBootSpaceInaccessible verifies that inaccessible kernel-image
// directories cannot produce the false-success message
// "no kernel images on /boot to size against", and instead warn with sudo fix.
func TestReconcileBootSpaceInaccessible(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("skipping permission-denied test when running as root (root bypasses DAC permissions)")
	}
	dir := t.TempDir()

	origRoots, origPkg := bootImageRoots, liminePkgInstalled
	t.Cleanup(func() {
		bootImageRoots, liminePkgInstalled = origRoots, origPkg
	})
	liminePkgInstalled = func(string) bool { return true }

	// Create a protected directory that cannot be read (mode 0000)
	protectedBoot := filepath.Join(dir, "protected_boot")
	if err := os.Mkdir(protectedBoot, 0o700); err != nil {
		t.Fatal(err)
	}
	// Place a kernel image inside it
	img := filepath.Join(protectedBoot, "initramfs-linux.img")
	if err := os.WriteFile(img, []byte("fake image content"), 0o600); err != nil {
		t.Fatal(err)
	}
	// Restrict permissions to 0000 so ReadDir fails with EACCES
	if err := os.Chmod(protectedBoot, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(protectedBoot, 0o700) })

	bootImageRoots = []string{protectedBoot}

	st := measureBootImages()
	if !st.inaccessible {
		t.Fatal("measureBootImages: inaccessible should be true when directory traversal fails with EACCES")
	}

	res := reconcileBootSpace(true)
	if res.status != recWarn {
		t.Fatalf("reconcileBootSpace on inaccessible directory: got status %d, want recWarn (%d)", res.status, recWarn)
	}
	if strings.Contains(res.detail, "no kernel images on /boot to size against") {
		t.Fatalf("reconcileBootSpace emitted false-success message: %s", res.detail)
	}
	if !strings.Contains(res.detail, "cannot inspect kernel images on /boot to verify headroom") {
		t.Errorf("expected warning about inspecting kernel images, got: %s", res.detail)
	}
	if res.remedy != "sudo ryoku doctor --check" {
		t.Errorf("expected remedy 'sudo ryoku doctor --check', got %q", res.remedy)
	}
}

// TestReconcileBootSpaceEmptyOrMissing verifies that genuinely missing or empty
// directories (with no permission errors) still report "no kernel images on /boot to size against".
func TestReconcileBootSpaceEmptyOrMissing(t *testing.T) {
	dir := t.TempDir()

	origRoots, origPkg, origFree := bootImageRoots, liminePkgInstalled, bootFreeBytes
	t.Cleanup(func() {
		bootImageRoots, liminePkgInstalled, bootFreeBytes = origRoots, origPkg, origFree
	})
	liminePkgInstalled = func(string) bool { return true }
	bootFreeBytes = func(string) (uint64, bool) {
		return 100 * 1024 * 1024, true
	}

	emptyBoot := filepath.Join(dir, "empty_boot")
	if err := os.Mkdir(emptyBoot, 0o755); err != nil {
		t.Fatal(err)
	}
	missingDir := filepath.Join(dir, "missing_dir")

	bootImageRoots = []string{missingDir, emptyBoot}

	st := measureBootImages()
	if st.inaccessible {
		t.Fatal("measureBootImages: inaccessible should be false for genuinely empty/missing directories")
	}
	if st.largest != 0 {
		t.Fatalf("measureBootImages: largest should be 0, got %d", st.largest)
	}

	res := reconcileBootSpace(true)
	if res.status != recOK {
		t.Fatalf("expected recOK for empty boot, got status %d", res.status)
	}
	if !strings.Contains(res.detail, "no kernel images on /boot to size against") {
		t.Errorf("expected 'no kernel images on /boot to size against', got %q", res.detail)
	}
}

// TestReconcileBootSpaceUnknownFreeSpaceWarns verifies that when free space measurement
// itself fails, reconcileBootSpace warns instead of claiming verified headroom or
// emitting false low-space warnings.
func TestReconcileBootSpaceUnknownFreeSpaceWarns(t *testing.T) {
	dir := t.TempDir()

	origRoots, origPkg, origFree := bootImageRoots, liminePkgInstalled, bootFreeBytes
	t.Cleanup(func() {
		bootImageRoots, liminePkgInstalled, bootFreeBytes = origRoots, origPkg, origFree
	})
	liminePkgInstalled = func(string) bool { return true }
	bootFreeBytes = func(string) (uint64, bool) {
		return 0, false // measurement failed
	}

	bootDir := filepath.Join(dir, "boot")
	if err := os.Mkdir(bootDir, 0o755); err != nil {
		t.Fatal(err)
	}
	img := filepath.Join(bootDir, "initramfs-linux.img")
	if err := os.WriteFile(img, make([]byte, 1024), 0o644); err != nil {
		t.Fatal(err)
	}
	bootImageRoots = []string{bootDir}

	res := reconcileBootSpace(true)
	if res.status != recWarn {
		t.Fatalf("expected recWarn when free space measurement fails, got status %d", res.status)
	}
	if !strings.Contains(res.detail, "cannot measure free space on /boot to verify headroom") {
		t.Errorf("unexpected detail: %s", res.detail)
	}
	if res.remedy != "sudo ryoku doctor --check" {
		t.Errorf("expected remedy 'sudo ryoku doctor --check', got %q", res.remedy)
	}
}

// TestReconcileBootSpaceReadableImages verifies that readable kernel images are properly sized
// and produce deterministic recOK when headroom is sufficient.
func TestReconcileBootSpaceReadableImages(t *testing.T) {
	dir := t.TempDir()

	origRoots, origPkg, origFree := bootImageRoots, liminePkgInstalled, bootFreeBytes
	t.Cleanup(func() {
		bootImageRoots, liminePkgInstalled, bootFreeBytes = origRoots, origPkg, origFree
	})
	liminePkgInstalled = func(string) bool { return true }
	bootFreeBytes = func(string) (uint64, bool) {
		return 100 * 1024 * 1024, true // 100 MiB free, plenty for 1 MiB image
	}

	bootDir := filepath.Join(dir, "boot")
	if err := os.Mkdir(bootDir, 0o755); err != nil {
		t.Fatal(err)
	}
	img := filepath.Join(bootDir, "initramfs-linux.img")
	imgData := make([]byte, 1024*1024) // 1 MiB
	if err := os.WriteFile(img, imgData, 0o644); err != nil {
		t.Fatal(err)
	}

	bootImageRoots = []string{bootDir}

	st := measureBootImages()
	if st.inaccessible {
		t.Fatal("inaccessible should be false for readable directory")
	}
	if st.largest != 1024*1024 {
		t.Fatalf("expected largest to be 1 MiB (%d), got %d", 1024*1024, st.largest)
	}

	res := reconcileBootSpace(true)
	if res.status != recOK {
		t.Fatalf("expected recOK when headroom is sufficient, got status %d (%s)", res.status, res.detail)
	}
	if !strings.Contains(res.detail, "room for a 1 MiB image rebuild") {
		t.Errorf("unexpected detail: %s", res.detail)
	}
}

// TestReconcileBootSpaceInsufficientHeadroom verifies that low headroom deterministically warns.
func TestReconcileBootSpaceInsufficientHeadroom(t *testing.T) {
	dir := t.TempDir()

	origRoots, origPkg, origFree := bootImageRoots, liminePkgInstalled, bootFreeBytes
	t.Cleanup(func() {
		bootImageRoots, liminePkgInstalled, bootFreeBytes = origRoots, origPkg, origFree
	})
	liminePkgInstalled = func(string) bool { return true }
	bootFreeBytes = func(string) (uint64, bool) {
		return 1 * 1024 * 1024, true // 1 MiB free, less than 2 * 1 MiB needed
	}

	bootDir := filepath.Join(dir, "boot")
	if err := os.Mkdir(bootDir, 0o755); err != nil {
		t.Fatal(err)
	}
	img := filepath.Join(bootDir, "initramfs-linux.img")
	imgData := make([]byte, 1024*1024) // 1 MiB
	if err := os.WriteFile(img, imgData, 0o644); err != nil {
		t.Fatal(err)
	}

	bootImageRoots = []string{bootDir}

	res := reconcileBootSpace(true)
	if res.status != recWarn {
		t.Fatalf("expected recWarn for insufficient headroom, got status %d", res.status)
	}
	if !strings.Contains(res.detail, "the next kernel update builds its image and cannot copy it in") {
		t.Errorf("unexpected detail: %s", res.detail)
	}
}
