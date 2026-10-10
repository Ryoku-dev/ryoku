package doctor

import (
	"errors"
	"os"
	"strings"
	"testing"

	"ryoku-cli/internal/host"
	"ryoku-cli/internal/updater"
)

const fedoraSnapshotsReason = "Snapshots are not available on Fedora: it boots through GRUB, which Ryoku's snapshot boot menu does not drive."

func stubDoctorDNF(t *testing.T) {
	t.Helper()
	oldManager := doctorPackageManager
	doctorPackageManager = func() (host.PackageManager, error) { return host.DNF, nil }
	t.Setenv("RYOKU_HOST_PKGMGR", "dnf")
	t.Cleanup(func() { doctorPackageManager = oldManager })
}

func assertNoForeignPackageAdvice(t *testing.T, result recResult) {
	t.Helper()
	text := strings.ToLower(result.detail + " " + result.remedy)
	for _, forbidden := range []string{"pacman", "aur", "limine", "xbps"} {
		if strings.Contains(text, forbidden) {
			t.Errorf("Fedora result mentions %q: %+v", forbidden, result)
		}
	}
}

func TestFedoraSnapshotRowExplainsUnsupportedBootStack(t *testing.T) {
	stubDoctorDNF(t)
	oldSnapshots := doctorSnapshots
	doctorSnapshots = func() (bool, string) { return false, fedoraSnapshotsReason }
	t.Cleanup(func() { doctorSnapshots = oldSnapshots })

	result := reconcileSnapper(true)
	if result.status != recNote || result.detail != fedoraSnapshotsReason {
		t.Fatalf("snapshot result = %+v", result)
	}
	assertNoForeignPackageAdvice(t, result)
}

func TestFedoraManifestUsesNativePackageLane(t *testing.T) {
	plan := updater.Plan{
		Install: []string{"base-package"},
		AUR:     []string{"community-tool"},
	}
	got := planSummary(host.DNF, &plan)
	if got != "install base-package, community-tool" {
		t.Fatalf("manifest plan = %q", got)
	}
}

func TestFedoraManifestSkipsUnavailableNativePackages(t *testing.T) {
	oldAvailability := doctorPackageAvailability
	doctorPackageAvailability = func(pkg string) int {
		if pkg == "blesh" || pkg == "songrec" {
			return host.ExitNotProvided
		}
		return host.ExitOK
	}
	t.Cleanup(func() { doctorPackageAvailability = oldAvailability })

	supported, unavailable := supportedManifestPackages(host.DNF, []string{"blesh", "quickshell", "songrec"})
	if got := strings.Join(supported, ","); got != "quickshell" {
		t.Fatalf("supported packages = %q", got)
	}
	if got := strings.Join(unavailable, ","); got != "blesh,songrec" {
		t.Fatalf("unavailable packages = %q", got)
	}
}

func TestFedoraPackageChannelReadsRPMRepo(t *testing.T) {
	stubDoctorDNF(t)
	oldInstalled := doctorPackageInstalled
	doctorPackageInstalled = func(name string) bool { return name == "ryoku-desktop" }
	t.Cleanup(func() { doctorPackageInstalled = oldInstalled })

	path := t.TempDir() + "/ryoku.repo"
	t.Setenv("RYOKU_DNF_REPO_CONFIG", path)
	if err := writeTestFile(path, dnfRepoConfig("stable")); err != nil {
		t.Fatal(err)
	}

	result := reconcileRyokuChannel(true)
	if result.status != recOK || !strings.Contains(result.detail, "stable packages") {
		t.Fatalf("channel result = %+v", result)
	}
	assertNoForeignPackageAdvice(t, result)
}

func TestFedoraPackageChannelRestoresSignedStableRepo(t *testing.T) {
	stubDoctorDNF(t)
	oldInstalled := doctorPackageInstalled
	oldWrite, oldRefresh := writeDNFRepoConfig, refreshDNFRepoMetadata
	doctorPackageInstalled = func(name string) bool { return name == "ryoku-desktop" }
	t.Cleanup(func() {
		doctorPackageInstalled = oldInstalled
		writeDNFRepoConfig, refreshDNFRepoMetadata = oldWrite, oldRefresh
	})

	path := t.TempDir() + "/ryoku.repo"
	t.Setenv("RYOKU_DNF_REPO_CONFIG", path)
	t.Setenv("RYOKU_RELEASE_BASE", "https://repo.example")
	var gotPath, gotConfig string
	refreshed := false
	writeDNFRepoConfig = func(path, contents string) error {
		gotPath, gotConfig = path, contents
		return nil
	}
	refreshDNFRepoMetadata = func() error {
		refreshed = true
		return nil
	}

	result := reconcileRyokuChannel(false)
	if result.status != recFixed || !refreshed {
		t.Fatalf("channel repair result = %+v refreshed=%v", result, refreshed)
	}
	const wantConfig = "[ryoku]\nname=Ryoku\nbaseurl=https://repo.example/x86_64\nenabled=1\ngpgcheck=1\nrepo_gpgcheck=1\ngpgkey=file:///etc/pki/rpm-gpg/RPM-GPG-KEY-ryoku\n"
	if gotPath != path || gotConfig != wantConfig {
		t.Fatalf("repo config = path %q body %q", gotPath, gotConfig)
	}
	assertNoForeignPackageAdvice(t, result)
}

func TestFedoraMetadataQueryAcceptsRepositoryKeys(t *testing.T) {
	argsPath := t.TempDir() + "/dnf-args"
	t.Setenv("DNF_ARGS", argsPath)
	script := `printf '%s\n' "$@" > "$DNF_ARGS"; echo ryoku-desktop`
	stubPath(t, map[string]string{"dnf": script, "dnf5": script})

	if _, err := checkDNFRepoMetadata(); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(argsPath)
	if err != nil {
		t.Fatal(err)
	}
	const want = "repoquery\n-y\n--repo=ryoku\n--available\nryoku-desktop\n"
	if string(raw) != want {
		t.Fatalf("dnf metadata args = %q, want %q", raw, want)
	}
}

func TestFedoraMetadataRefreshAcceptsRepositoryKeys(t *testing.T) {
	argsPath := t.TempDir() + "/sudo-args"
	t.Setenv("DNF_ARGS", argsPath)
	stubPath(t, map[string]string{
		"dnf5": ":",
		"sudo": `printf '%s\n' "$@" > "$DNF_ARGS"`,
	})

	if err := refreshDNFRepoMetadata(); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(argsPath)
	if err != nil {
		t.Fatal(err)
	}
	const want = "dnf5\nmakecache\n-y\n--refresh\n--repo=ryoku\n"
	if string(raw) != want {
		t.Fatalf("dnf metadata refresh args = %q, want %q", raw, want)
	}
}

func TestFedoraPackageDatabaseRefreshesBrokenMetadata(t *testing.T) {
	stubDoctorDNF(t)
	oldInstalled := doctorPackageInstalled
	oldCheck, oldRefresh := checkDNFRepoMetadata, refreshDNFRepoMetadata
	doctorPackageInstalled = func(name string) bool { return name == "ryoku-desktop" }
	t.Cleanup(func() {
		doctorPackageInstalled = oldInstalled
		checkDNFRepoMetadata, refreshDNFRepoMetadata = oldCheck, oldRefresh
	})

	path := t.TempDir() + "/ryoku.repo"
	t.Setenv("RYOKU_DNF_REPO_CONFIG", path)
	if err := writeTestFile(path, dnfRepoConfig("stable")); err != nil {
		t.Fatal(err)
	}

	healthy := false
	checkDNFRepoMetadata = func() (string, error) {
		if healthy {
			return "ryoku-desktop", nil
		}
		return "repository metadata is unavailable", errors.New("metadata load failed")
	}
	refreshDNFRepoMetadata = func() error {
		healthy = true
		return nil
	}

	check := reconcileRyokuSyncDB(true)
	if check.status != recWouldFix || !strings.Contains(check.remedy, "makecache -y --refresh --repo=ryoku") {
		t.Fatalf("check result = %+v", check)
	}
	assertNoForeignPackageAdvice(t, check)

	applied := reconcileRyokuSyncDB(false)
	if applied.status != recFixed || !healthy {
		t.Fatalf("apply result = %+v healthy=%v", applied, healthy)
	}
	assertNoForeignPackageAdvice(t, applied)
}

func TestFedoraPackageDatabaseReportsRefreshFailure(t *testing.T) {
	stubDoctorDNF(t)
	oldInstalled := doctorPackageInstalled
	oldCheck, oldRefresh := checkDNFRepoMetadata, refreshDNFRepoMetadata
	doctorPackageInstalled = func(name string) bool { return name == "ryoku-desktop" }
	checkDNFRepoMetadata = func() (string, error) {
		return "repository metadata is unavailable", errors.New("metadata load failed")
	}
	refreshDNFRepoMetadata = func() error { return errors.New("network unavailable") }
	t.Cleanup(func() {
		doctorPackageInstalled = oldInstalled
		checkDNFRepoMetadata, refreshDNFRepoMetadata = oldCheck, oldRefresh
	})

	path := t.TempDir() + "/ryoku.repo"
	t.Setenv("RYOKU_DNF_REPO_CONFIG", path)
	if err := writeTestFile(path, dnfRepoConfig("stable")); err != nil {
		t.Fatal(err)
	}

	result := reconcileRyokuSyncDB(false)
	if result.status != recFailed || !strings.Contains(result.detail, "network unavailable") ||
		!strings.Contains(result.remedy, "makecache -y --refresh --repo=ryoku") {
		t.Fatalf("refresh failure result = %+v", result)
	}
	assertNoForeignPackageAdvice(t, result)
}

func TestFedoraBootGuardUsesSystemdUnit(t *testing.T) {
	stubDoctorDNF(t)
	oldInstalled := doctorPackageInstalled
	oldCheckout, oldShipped := bootGuardCheckout, bootGuardShipped
	oldWritable, oldService := bootGuardWritable, doctorService
	doctorPackageInstalled = func(name string) bool { return name == "ryoku-desktop" }
	bootGuardCheckout = func() bool { return false }
	bootGuardShipped = func(manager host.PackageManager) bool { return manager == host.DNF }
	bootGuardWritable = func() bool { return false }
	doctorService = func(args ...string) int { return host.ExitFalse }
	t.Cleanup(func() {
		doctorPackageInstalled = oldInstalled
		bootGuardCheckout, bootGuardShipped = oldCheckout, oldShipped
		bootGuardWritable, doctorService = oldWritable, oldService
	})

	result := reconcileBootGuard(true)
	if result.status != recWouldFix || !strings.Contains(result.remedy, "systemctl enable ryoku-boot-guard.service") {
		t.Fatalf("boot guard result = %+v", result)
	}
	assertNoForeignPackageAdvice(t, result)
}

func TestFedoraPendingConfigAndOrphanAdvice(t *testing.T) {
	stubDoctorDNF(t)
	oldFind, oldOrphans := doctorFindPendingConfig, doctorOrphans
	doctorFindPendingConfig = func(pattern string) []string {
		if pattern == "*.rpmnew" {
			return []string{"/etc/example.conf.rpmnew"}
		}
		return nil
	}
	doctorOrphans = func() ([]string, error) { return []string{"unused-package"}, nil }
	t.Cleanup(func() {
		doctorFindPendingConfig, doctorOrphans = oldFind, oldOrphans
	})

	pending := reconcilePacnew(true)
	if pending.status != recWarn || !strings.Contains(pending.detail, ".rpmnew") {
		t.Fatalf("pending result = %+v", pending)
	}
	assertNoForeignPackageAdvice(t, pending)

	orphans := reconcileOrphans(true)
	if orphans.status != recNote || !strings.Contains(orphans.remedy, "autoremove") {
		t.Fatalf("orphan result = %+v", orphans)
	}
	assertNoForeignPackageAdvice(t, orphans)
}

func TestFedoraNvidiaStaysOnNouveauWithManualRPMFusionAdvice(t *testing.T) {
	stubDoctorDNF(t)
	oldManager, oldPCI := nvidiaPackageManager, nvidiaPCIOutput
	nvidiaPackageManager = func() host.PackageManager { return host.DNF }
	nvidiaPCIOutput = func() string { return "NVIDIA Corporation AD107M" }
	t.Cleanup(func() {
		nvidiaPackageManager, nvidiaPCIOutput = oldManager, oldPCI
	})

	if pkg := nvidia580Package(); pkg != "akmod-nvidia" {
		t.Fatalf("Fedora NVIDIA package = %q", pkg)
	}
	result := reconcileNvidiaModeset(true)
	if result.status != recNote || !strings.Contains(result.detail, "Nouveau") ||
		!strings.Contains(result.remedy, "RPM Fusion") || !strings.Contains(result.remedy, "akmod-nvidia") ||
		!strings.Contains(result.remedy, "dracut --regenerate-all --force") {
		t.Fatalf("NVIDIA result = %+v", result)
	}
	assertNoForeignPackageAdvice(t, result)
}

func TestFedoraBootReconcilersSkipWithoutForeignBootAdvice(t *testing.T) {
	stubDoctorDNF(t)
	for name, reconcile := range map[string]func(bool) recResult{
		"boot space":        reconcileBootSpace,
		"GPU image trim":    reconcileInitramfsGPUTrim,
		"console key image": reconcileInitramfsConsoleKeys,
		"boot menu":         reconcileLimineLayout,
	} {
		t.Run(name, func(t *testing.T) {
			result := reconcile(true)
			if result.status != recOK {
				t.Fatalf("result = %+v", result)
			}
			assertNoForeignPackageAdvice(t, result)
		})
	}
}

func TestFedoraShippedAppQueryAcceptsRepositoryKeys(t *testing.T) {
	stubDoctorDNF(t)
	argsPath := t.TempDir() + "/dnf-args"
	t.Setenv("DNF_ARGS", argsPath)
	script := `printf '%s\n' "$@" > "$DNF_ARGS"`
	stubPath(t, map[string]string{"dnf": script, "dnf5": script})

	if !appInstalledAsDep("example-app") {
		t.Fatal("an app absent from --userinstalled should be treated as automatic")
	}
	raw, err := os.ReadFile(argsPath)
	if err != nil {
		t.Fatal(err)
	}
	const want = "repoquery\n-y\n--userinstalled\n--qf\n%{name}\n\nexample-app\n"
	if string(raw) != want {
		t.Fatalf("dnf repoquery args = %q, want %q", raw, want)
	}
}

func TestFedoraShippedAppQueryIgnoresDNF4BlankRows(t *testing.T) {
	stubDoctorDNF(t)
	script := `printf 'example-app\n\n'`
	stubPath(t, map[string]string{"dnf": script, "dnf5": script})

	if appInstalledAsDep("example-app") {
		t.Fatal("a user-installed app must not be classified as automatic")
	}
}

func TestFedoraShippedAppsSkipUnavailablePackages(t *testing.T) {
	stubDoctorDNF(t)
	isolateProvisioned(t)
	oldInstalled, oldDep := appInstalled, appInstalledAsDep
	oldInstall, oldExplicit := installShippedApps, markAppsExplicit
	oldAvailability := doctorPackageAvailability
	appInstalled = func(string) bool { return true }
	appInstalledAsDep = func(string) bool { return false }
	doctorPackageAvailability = func(pkg string) int {
		switch pkg {
		case "blesh", "waifu2x-ncnn-vulkan", "songrec":
			return host.ExitNotProvided
		default:
			return host.ExitOK
		}
	}
	installShippedApps = func(pkgs []string) { t.Fatalf("installed unavailable packages: %v", pkgs) }
	markAppsExplicit = func(pkgs []string) { t.Fatalf("changed ownership unexpectedly: %v", pkgs) }
	for _, app := range shippedApps() {
		recordProvisioned(app.pkg)
	}
	t.Cleanup(func() {
		appInstalled, appInstalledAsDep = oldInstalled, oldDep
		installShippedApps, markAppsExplicit = oldInstall, oldExplicit
		doctorPackageAvailability = oldAvailability
	})

	result := reconcileShippedApps(true)
	if result.status != recNote || strings.Contains(result.detail, "would install") {
		t.Fatalf("unavailable shipped-app result = %+v", result)
	}
	for _, pkg := range []string{"blesh", "waifu2x-ncnn-vulkan", "songrec"} {
		if !strings.Contains(result.detail, pkg) {
			t.Errorf("unavailable result does not name %s: %s", pkg, result.detail)
		}
	}
}

func TestFedoraShippedAndRetiredAppsUseNativePackageAdvice(t *testing.T) {
	stubDoctorDNF(t)
	isolateProvisioned(t)
	oldInstalled, oldDep := appInstalled, appInstalledAsDep
	oldInstall, oldExplicit := installShippedApps, markAppsExplicit
	oldAvailability := doctorPackageAvailability
	oldRetiredInstalled, oldRetiredRemove := retiredAppInstalled, removeRetiredApp
	appInstalled = func(string) bool { return false }
	appInstalledAsDep = func(string) bool { return false }
	doctorPackageAvailability = func(string) int { return host.ExitOK }
	installShippedApps = func(pkgs []string) { t.Fatalf("check mode installed %v", pkgs) }
	markAppsExplicit = func(pkgs []string) { t.Fatalf("check mode changed ownership of %v", pkgs) }
	retiredAppInstalled = func(pkg string) bool { return pkg == "ryomotion" }
	removeRetiredApp = func(string) error { t.Fatal("check mode removed a package"); return nil }
	t.Cleanup(func() {
		appInstalled, appInstalledAsDep = oldInstalled, oldDep
		installShippedApps, markAppsExplicit = oldInstall, oldExplicit
		doctorPackageAvailability = oldAvailability
		retiredAppInstalled, removeRetiredApp = oldRetiredInstalled, oldRetiredRemove
	})

	shipped := reconcileShippedApps(true)
	if shipped.status != recWouldFix || !strings.Contains(shipped.detail, "would install") {
		t.Fatalf("shipped apps result = %+v", shipped)
	}
	assertNoForeignPackageAdvice(t, shipped)

	retired := reconcileRetiredApps(true)
	if retired.status != recWouldFix || !strings.Contains(retired.remedy, "remove -y ryomotion") {
		t.Fatalf("retired app result = %+v", retired)
	}
	assertNoForeignPackageAdvice(t, retired)
}

func writeTestFile(path, body string) error {
	return os.WriteFile(path, []byte(body), 0o644)
}
