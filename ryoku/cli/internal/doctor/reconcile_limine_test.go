package doctor

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

// TestProbeFile verifies that probeFile correctly distinguishes readable files,
// genuinely missing files (ENOENT), and permission-denied files/directories (EACCES)
// without needing root privileges.
func TestProbeFile(t *testing.T) {
	dir := t.TempDir()

	// 1. Readable file
	readablePath := filepath.Join(dir, "readable.conf")
	content := "timeout: 3\ndefault_entry: 1\n"
	if err := os.WriteFile(readablePath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	gotContent, exists, readable := probeFile(readablePath)
	if !exists || !readable || gotContent != content {
		t.Errorf("readable file: got (%q, %v, %v), want (%q, true, true)", gotContent, exists, readable, content)
	}

	// 2. Genuinely missing file (ENOENT)
	missingPath := filepath.Join(dir, "nonexistent.conf")
	gotContent, exists, readable = probeFile(missingPath)
	if exists || readable || gotContent != "" {
		t.Errorf("missing file: got (%q, %v, %v), want (\"\", false, false)", gotContent, exists, readable)
	}

	// 3. Directory path (EISDIR, a non-ENOENT error that fails for both normal users and root)
	dirPath := filepath.Join(dir, "test_dir")
	if err := os.Mkdir(dirPath, 0o755); err != nil {
		t.Fatal(err)
	}
	gotContent, exists, readable = probeFile(dirPath)
	if !exists || readable || gotContent != "" {
		t.Errorf("directory probe: got (%q, %v, %v), want (\"\", true, false)", gotContent, exists, readable)
	}

	// 4. Permission-denied file (mode 0000)
	if os.Geteuid() == 0 {
		t.Log("skipping DAC permission-denied file test when running as root")
	} else {
		unreadableFile := filepath.Join(dir, "unreadable.conf")
		if err := os.WriteFile(unreadableFile, []byte("secret"), 0o000); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Chmod(unreadableFile, 0o600) })

		gotContent, exists, readable = probeFile(unreadableFile)
		if !exists || readable || gotContent != "" {
			t.Errorf("unreadable file: got (%q, %v, %v), want (\"\", true, false)", gotContent, exists, readable)
		}
	}

	// 5. Permission-denied parent directory (simulating /boot mode 0700 for non-root)
	if os.Geteuid() == 0 {
		t.Log("skipping DAC permission-denied directory test when running as root")
	} else {
		protectedDir := filepath.Join(dir, "protected_boot")
		if err := os.Mkdir(protectedDir, 0o700); err != nil {
			t.Fatal(err)
		}
		inProtected := filepath.Join(protectedDir, "limine.conf")
		if err := os.WriteFile(inProtected, []byte("timeout: 5\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		// Chmod directory to 0000 so traversal fails with EACCES
		if err := os.Chmod(protectedDir, 0o000); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Chmod(protectedDir, 0o700) })

		gotContent, exists, readable = probeFile(inProtected)
		if !exists || readable || gotContent != "" {
			t.Errorf("traversal-denied file: got (%q, %v, %v), want (\"\", true, false)", gotContent, exists, readable)
		}
	}
}

// TestGatherLimineLayoutStateDistinguishesEACCESFromENOENT checks that
// gatherLimineLayoutState marks espConfExists=true and espConfReadable=false
// when /boot/limine.conf is permission-denied, but leaves both false when missing.
func TestGatherLimineLayoutStateDistinguishesEACCESFromENOENT(t *testing.T) {
	dir := t.TempDir()

	origESP, origShadow, origTool, origLegacy, origPkg := limineESPConf, limineShadow, limineToolEFI, limineLegacyEFI, liminePkgInstalled
	t.Cleanup(func() {
		limineESPConf, limineShadow, limineToolEFI, limineLegacyEFI, liminePkgInstalled = origESP, origShadow, origTool, origLegacy, origPkg
	})
	liminePkgInstalled = func(string) bool { return true }
	limineShadow = filepath.Join(dir, "shadow_missing.conf")
	limineToolEFI = filepath.Join(dir, "tool_missing.efi")
	limineLegacyEFI = filepath.Join(dir, "legacy_missing.efi")

	// Case A: Missing file (ENOENT)
	limineESPConf = filepath.Join(dir, "missing.conf")
	stMissing := gatherLimineLayoutState()
	if stMissing.espConfExists || stMissing.espConfReadable {
		t.Errorf("missing config: got exists=%v, readable=%v; want false, false",
			stMissing.espConfExists, stMissing.espConfReadable)
	}
	outcome, _ := planLimineLayout(stMissing)
	if outcome != limineLayoutSkip {
		t.Errorf("missing config plan: got outcome %d, want limineLayoutSkip (%d)", outcome, limineLayoutSkip)
	}

	// Case B: Permission denied (EACCES) via mode 0000 file
	if os.Geteuid() == 0 {
		t.Log("skipping permission-denied layout test when running as root (root bypasses DAC permissions)")
	} else {
		unreadable := filepath.Join(dir, "unreadable_limine.conf")
		if err := os.WriteFile(unreadable, []byte("timeout: 3\n"), 0o000); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Chmod(unreadable, 0o600) })

		limineESPConf = unreadable
		stDenied := gatherLimineLayoutState()
		if !stDenied.espConfExists || stDenied.espConfReadable {
			t.Errorf("permission-denied config: got exists=%v, readable=%v; want true, false",
				stDenied.espConfExists, stDenied.espConfReadable)
		}
		outcome, _ := planLimineLayout(stDenied)
		if outcome != limineLayoutUnreadable {
			t.Errorf("permission-denied config plan: got outcome %d, want limineLayoutUnreadable (%d)", outcome, limineLayoutUnreadable)
		}

		res := reconcileLimineLayout(true)
		if res.status != recWarn {
			t.Errorf("reconcileLimineLayout on unreadable config: got status %d, want recWarn (%d)", res.status, recWarn)
		}
		if !strings.Contains(res.detail, "cannot read the limine config under /boot") {
			t.Errorf("unexpected detail: %s", res.detail)
		}
		if res.remedy != "sudo ryoku doctor --check" {
			t.Errorf("expected remedy 'sudo ryoku doctor --check', got %q", res.remedy)
		}
	}

	// Case C: Readable file
	readable := filepath.Join(dir, "readable_limine.conf")
	if err := os.WriteFile(readable, []byte("timeout: 3\ndefault_entry: 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	limineESPConf = readable
	stReadable := gatherLimineLayoutState()
	if !stReadable.espConfExists || !stReadable.espConfReadable || stReadable.espConf == "" {
		t.Errorf("readable config: got exists=%v, readable=%v, content=%q; want true, true, non-empty",
			stReadable.espConfExists, stReadable.espConfReadable, stReadable.espConf)
	}
}

// TestReconcileLimineBootEntryPermissions verifies that reconcileLimineBootEntry
// distinguishes permission-denied EFI binary from a genuinely missing one.
func TestReconcileLimineBootEntryPermissions(t *testing.T) {
	dir := t.TempDir()

	origTool, origPkg := limineToolEFI, liminePkgInstalled
	t.Cleanup(func() {
		limineToolEFI, liminePkgInstalled = origTool, origPkg
	})
	liminePkgInstalled = func(string) bool { return true }

	// Genuinely missing: skips with okRes ("not a limine-managed boot on this box")
	limineToolEFI = filepath.Join(dir, "missing.efi")
	resMissing := reconcileLimineBootEntry(true)
	if resMissing.status != recOK || !strings.Contains(resMissing.detail, "not a limine-managed boot") {
		t.Errorf("missing EFI binary: got %+v, want recOK (not limine-managed)", resMissing)
	}

	// Permission denied: warns instead of falsely reporting not a limine boot
	if os.Geteuid() == 0 {
		t.Log("skipping permission-denied EFI binary test when running as root (root bypasses DAC permissions)")
	} else {
		protectedDir := filepath.Join(dir, "protected_efi")
		if err := os.Mkdir(protectedDir, 0o700); err != nil {
			t.Fatal(err)
		}
		efiFile := filepath.Join(protectedDir, "limine_x64.efi")
		if err := os.WriteFile(efiFile, []byte("efi"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(protectedDir, 0o000); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Chmod(protectedDir, 0o700) })

		limineToolEFI = efiFile
		resDenied := reconcileLimineBootEntry(true)
		if resDenied.status != recWarn {
			t.Errorf("permission-denied EFI binary: got status %d, want recWarn (%d)", resDenied.status, recWarn)
		}
		if !strings.Contains(resDenied.detail, "cannot inspect") {
			t.Errorf("unexpected detail: %s", resDenied.detail)
		}
		if resDenied.remedy != "sudo ryoku doctor --check" {
			t.Errorf("expected remedy 'sudo ryoku doctor --check', got %q", resDenied.remedy)
		}
	}
}

// TestLimineDependentChecksStandAsideOnUnreadableConf tests that dependent Limine
// reconcilers do not report false-success or misleading findings when limineESPConf is unreadable.
func TestLimineDependentChecksStandAsideOnUnreadableConf(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("skipping permission-dependent test when running as root (root bypasses DAC permissions)")
	}
	dir := t.TempDir()

	origESP, origPkg, origDef := limineESPConf, liminePkgInstalled, limineDefaultsPath
	t.Cleanup(func() {
		limineESPConf, liminePkgInstalled, limineDefaultsPath = origESP, origPkg, origDef
	})
	liminePkgInstalled = func(string) bool { return true }

	defaultsFile := filepath.Join(dir, "default_limine")
	if err := os.WriteFile(defaultsFile, []byte("ENABLE_UKI=yes\nTARGET_OS_NAME=\"Ryoku\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	limineDefaultsPath = defaultsFile

	unreadable := filepath.Join(dir, "limine.conf")
	if err := os.WriteFile(unreadable, []byte("timeout: 3\n"), 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(unreadable, 0o600) })
	limineESPConf = unreadable

	wantStandAside := "no readable " + unreadable + "; the layout reconciler owns that"

	// reconcileLimineUKITree
	if res := reconcileLimineUKITree(true); res.status != recNote || !strings.Contains(res.detail, "the layout reconciler owns that") {
		t.Errorf("reconcileLimineUKITree on unreadable: got %+v, want recNote with detail containing %q", res, wantStandAside)
	}

	// reconcileLimineKernelImages
	if res := reconcileLimineKernelImages(true); res.status != recNote || !strings.Contains(res.detail, "the layout reconciler owns that") {
		t.Errorf("reconcileLimineKernelImages on unreadable: got %+v, want recNote with detail containing %q", res, wantStandAside)
	}

	// reconcileLimineDeadEntries
	if res := reconcileLimineDeadEntries(true); res.status != recNote || !strings.Contains(res.detail, "the layout reconciler owns that") {
		t.Errorf("reconcileLimineDeadEntries on unreadable: got %+v, want recNote with detail containing %q", res, wantStandAside)
	}

	// reconcileLimineOSName
	if res := reconcileLimineOSName(true); res.status != recNote || !strings.Contains(res.detail, "the layout reconciler owns that") {
		t.Errorf("reconcileLimineOSName on unreadable: got %+v, want recNote with detail containing %q", res, wantStandAside)
	}

	// reconcileLimineAutoboot
	if res := reconcileLimineAutoboot(true); res.status != recNote || !strings.Contains(res.detail, "the layout reconciler owns that") {
		t.Errorf("reconcileLimineAutoboot on unreadable: got %+v, want recNote with detail containing %q", res, wantStandAside)
	}
}

// TestGatherLimineLayoutStateInaccessibleEFIBinaries verifies that when EFI binaries
// cannot be inspected due to access or I/O errors, planLimineLayout yields limineLayoutUnreadable
// instead of silently assuming they are absent.
func TestGatherLimineLayoutStateInaccessibleEFIBinaries(t *testing.T) {
	dir := t.TempDir()

	origESP, origShadow, origTool, origLegacy, origPkg, origProbe := limineESPConf, limineShadow, limineToolEFI, limineLegacyEFI, liminePkgInstalled, liminePathProbe
	t.Cleanup(func() {
		limineESPConf, limineShadow, limineToolEFI, limineLegacyEFI, liminePkgInstalled, liminePathProbe = origESP, origShadow, origTool, origLegacy, origPkg, origProbe
	})
	liminePkgInstalled = func(string) bool { return true }

	confPath := filepath.Join(dir, "limine.conf")
	if err := os.WriteFile(confPath, []byte("timeout: 3\ndefault_entry: 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	limineESPConf = confPath
	limineShadow = filepath.Join(dir, "shadow_missing.conf")
	limineToolEFI = "/boot/EFI/limine/limine_x64.efi"
	limineLegacyEFI = "/boot/EFI/limine/limine.efi"

	// Simulate EIO error on legacy EFI binary
	liminePathProbe = func(p string) (bool, error) {
		switch p {
		case limineLegacyEFI:
			return false, syscall.EIO
		case limineToolEFI:
			return true, nil
		default:
			return false, nil
		}
	}

	st := gatherLimineLayoutState()
	if !st.legacyEFIInaccessible {
		t.Errorf("expected legacyEFIInaccessible=true on EIO error, got false")
	}

	outcome, _ := planLimineLayout(st)
	if outcome != limineLayoutUnreadable {
		t.Errorf("planLimineLayout = %d, want limineLayoutUnreadable (%d)", outcome, limineLayoutUnreadable)
	}

	res := reconcileLimineLayout(true)
	if res.status != recWarn {
		t.Errorf("reconcileLimineLayout = %d, want recWarn (%d)", res.status, recWarn)
	}
	if !strings.Contains(res.detail, "cannot inspect limine bootloader binaries under /boot") {
		t.Errorf("expected warning mentioning bootloader binaries, got: %s", res.detail)
	}
	if res.remedy != "sudo ryoku doctor --check" {
		t.Errorf("expected remedy 'sudo ryoku doctor --check', got %q", res.remedy)
	}
}

// TestPlanLimineLayoutInaccessibleEFIWithMissingConfigsNotSkipped verifies that
// an inaccessible EFI binary with genuinely absent configs produces limineLayoutUnreadable,
// NOT limineLayoutSkip.
func TestPlanLimineLayoutInaccessibleEFIWithMissingConfigsNotSkipped(t *testing.T) {
	stTool := limineLayoutState{
		limineInstalled:     true,
		espConfExists:       false,
		shadowExists:        false,
		toolEFIInaccessible: true,
	}
	outcome, _ := planLimineLayout(stTool)
	if outcome != limineLayoutUnreadable {
		t.Fatalf("outcome = %d, want limineLayoutUnreadable (%d); inaccessible tool EFI must not skip even when configs are absent", outcome, limineLayoutUnreadable)
	}

	stLegacy := limineLayoutState{
		limineInstalled:       true,
		espConfExists:         false,
		shadowExists:          false,
		legacyEFIInaccessible: true,
	}
	outcome, _ = planLimineLayout(stLegacy)
	if outcome != limineLayoutUnreadable {
		t.Fatalf("outcome = %d, want limineLayoutUnreadable (%d); inaccessible legacy EFI must not skip even when configs are absent", outcome, limineLayoutUnreadable)
	}
}

// TestReconcileLimineDefaultsUnreadable verifies that when /etc/default/limine is
// unreadable (e.g. permission denied), dependent checks (reconcileLimineUKITree and
// reconcileLimineOSName) warn instead of silently assuming the file or settings are absent.
func TestReconcileLimineDefaultsUnreadable(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("skipping DAC permission-denied test when running as root (root bypasses DAC permissions)")
	}
	dir := t.TempDir()

	origESP, origPkg, origDef := limineESPConf, liminePkgInstalled, limineDefaultsPath
	t.Cleanup(func() {
		limineESPConf, liminePkgInstalled, limineDefaultsPath = origESP, origPkg, origDef
	})
	liminePkgInstalled = func(string) bool { return true }

	// Create unreadable defaults file (mode 0000)
	unreadableDef := filepath.Join(dir, "default_limine")
	if err := os.WriteFile(unreadableDef, []byte("ENABLE_UKI=yes\nTARGET_OS_NAME=\"Ryoku\"\n"), 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(unreadableDef, 0o600) })
	limineDefaultsPath = unreadableDef

	// Readable esp config
	readableESP := filepath.Join(dir, "limine.conf")
	if err := os.WriteFile(readableESP, []byte("timeout: 3\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	limineESPConf = readableESP

	// reconcileLimineUKITree must warn about unreadable defaults, NOT return okRes
	resUKI := reconcileLimineUKITree(true)
	if resUKI.status != recWarn {
		t.Errorf("reconcileLimineUKITree on unreadable defaults: got status %d, want recWarn (%d)", resUKI.status, recWarn)
	}
	if !strings.Contains(resUKI.detail, "cannot read") || !strings.Contains(resUKI.detail, unreadableDef) {
		t.Errorf("unexpected detail: %s", resUKI.detail)
	}
	if resUKI.remedy != "sudo ryoku doctor --check" {
		t.Errorf("expected remedy 'sudo ryoku doctor --check', got %q", resUKI.remedy)
	}

	// reconcileLimineOSName must warn about unreadable defaults, NOT return okRes
	resOS := reconcileLimineOSName(true)
	if resOS.status != recWarn {
		t.Errorf("reconcileLimineOSName on unreadable defaults: got status %d, want recWarn (%d)", resOS.status, recWarn)
	}
	if !strings.Contains(resOS.detail, "cannot read") || !strings.Contains(resOS.detail, unreadableDef) {
		t.Errorf("unexpected detail: %s", resOS.detail)
	}
	if resOS.remedy != "sudo ryoku doctor --check" {
		t.Errorf("expected remedy 'sudo ryoku doctor --check', got %q", resOS.remedy)
	}
}
