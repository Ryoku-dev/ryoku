package doctor

import (
	"os"
	"path/filepath"
	"strings"

	"ryoku-cli/internal/host"
	"ryoku-cli/internal/sys"

	i18n "ryoku-i18n"
	wm "ryoku-wm"
)

var (
	retiredCompositorPackageInstalled = sys.PkgInstalled
	retiredCompositorRun              = func(args ...string) error { return sys.Sudo(args...) }
	retiredCompositorRemoveAll        = os.RemoveAll
	retiredCompositorRemove           = os.Remove
	retiredCompositorHome             = os.UserHomeDir
)

func retiredCompositorTarget() string {
	name := wm.Detect().Name
	for _, provider := range wm.Providers() {
		if provider == name {
			return name
		}
	}
	for _, provider := range wm.Providers() {
		if retiredCompositorPackageInstalled("ryoku-desktop-" + provider) {
			return provider
		}
	}
	return wm.Providers()[0]
}

func retiredCompositorArtifactsPresent(legacy wm.RetiredCompositorArtifacts) bool {
	if retiredCompositorPackageInstalled(legacy.VariantPackage) {
		return true
	}
	for _, pkg := range legacy.Packages {
		if retiredCompositorPackageInstalled(pkg) {
			return true
		}
	}
	home, err := retiredCompositorHome()
	if err != nil {
		return false
	}
	for _, path := range []string{
		filepath.Join(sys.ConfigHome(), legacy.ConfigDir),
		filepath.Join(home, ".local", "bin", legacy.ProviderBinary),
	} {
		if sys.PathPresent(path) {
			return true
		}
	}
	return false
}

func retiredCompositorFix(legacy wm.RetiredCompositorArtifacts, target string, live bool) string {
	targetPkg := "ryoku-desktop-" + target
	if live || !retiredCompositorPackageInstalled(legacy.VariantPackage) {
		return "sudo pacman -S --needed --noconfirm " + targetPkg
	}
	parts := []string{
		"sudo pacman -Sw --needed --noconfirm " + targetPkg,
		"sudo pacman -Rdd --noconfirm " + legacy.VariantPackage,
		"sudo pacman -S --needed --noconfirm " + targetPkg,
	}
	if len(legacy.Packages) > 0 {
		parts = append(parts, "sudo pacman -Rns --noconfirm "+strings.Join(legacy.Packages, " "))
	}
	return strings.Join(parts, " && ")
}

func installReplacementVariant(legacy wm.RetiredCompositorArtifacts, target string, live bool) error {
	targetPkg := "ryoku-desktop-" + target
	if retiredCompositorPackageInstalled(targetPkg) {
		return nil
	}
	if live || !retiredCompositorPackageInstalled(legacy.VariantPackage) {
		return retiredCompositorRun("pacman", "-S", "--needed", "--noconfirm", targetPkg)
	}
	if err := retiredCompositorRun("pacman", "-Sw", "--needed", "--noconfirm", targetPkg); err != nil {
		return err
	}
	if err := retiredCompositorRun("pacman", "-Rdd", "--noconfirm", legacy.VariantPackage); err != nil {
		return err
	}
	return retiredCompositorRun("pacman", "-S", "--needed", "--noconfirm", targetPkg)
}

func removeRetiredCompositor(legacy wm.RetiredCompositorArtifacts) error {
	if retiredCompositorPackageInstalled(legacy.VariantPackage) {
		if err := retiredCompositorRun("pacman", "-Rdd", "--noconfirm", legacy.VariantPackage); err != nil {
			return err
		}
	}
	var installed []string
	for _, pkg := range legacy.Packages {
		if retiredCompositorPackageInstalled(pkg) {
			installed = append(installed, pkg)
		}
	}
	if len(installed) > 0 {
		args := []string{"pacman", "-Rns", "--noconfirm"}
		if err := retiredCompositorRun(append(args, installed...)...); err != nil {
			return err
		}
	}
	home, err := retiredCompositorHome()
	if err != nil {
		return err
	}
	if err := retiredCompositorRemoveAll(filepath.Join(sys.ConfigHome(), legacy.ConfigDir)); err != nil {
		return err
	}
	binary := filepath.Join(home, ".local", "bin", legacy.ProviderBinary)
	if err := retiredCompositorRemove(binary); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// reconcileRetiredCompositor first makes a supported session bootable, then
// removes the retired stack only after the user has left that session.
func reconcileRetiredCompositor(checkOnly bool) recResult {
	manager, err := doctorPackageManager()
	if err != nil || manager != host.Pacman {
		return okRes(i18n.T("retired compositor package migration is not applicable on this host"))
	}
	target := retiredCompositorTarget()
	var result recResult
	for _, legacy := range wm.RetiredCompositors() {
		if !retiredCompositorArtifactsPresent(legacy) {
			continue
		}
		live := wm.RetiredCompositorLive(legacy)
		fix := retiredCompositorFix(legacy, target, live)
		if checkOnly {
			return wouldRes(i18n.T("retired compositor artifacts need migration to %s"), target).withFix(fix)
		}
		if err := installReplacementVariant(legacy, target, live); err != nil {
			return failRes(i18n.T("could not install the %s desktop variant: %v"), target, err).withFix(fix)
		}
		if live {
			return warnRes(i18n.T("installed the %s desktop variant; log into it before the retired compositor is removed"), target)
		}
		if err := removeRetiredCompositor(legacy); err != nil {
			return failRes(i18n.T("could not remove the retired compositor: %v"), err).withFix(fix)
		}
		result = fixedRes(i18n.T("migrated to the %s desktop variant and removed the retired compositor"), target)
	}
	if result.status == recFixed {
		return result
	}
	return okRes(i18n.T("no retired compositor artifacts present"))
}
