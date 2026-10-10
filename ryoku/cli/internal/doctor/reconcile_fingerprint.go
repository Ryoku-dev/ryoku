package doctor

import (
	"os"
	"strings"

	"ryoku-cli/internal/host"
	"ryoku-cli/internal/sys"

	i18n "ryoku-i18n"
)

// ---- reconciler: fingerprint unlock module -----------------------------------
//
// The in-session lock (qylock) and the SDDM greeter authenticate through a PAM
// stack that loads pam_fprintd_grosshack.so -- a fork of pam_fprintd that starts
// fprintd-verify at the beginning of the conversation, so the sensor scans while
// the password field is live and the first success (touch OR type) wins. The
// lock's PAM service (ryoku/lockscreen/.../assets/pam/ryoku-lock) names that
// module unconditionally, and the Hub's Lockscreen page only offers the sddm/sudo
// fingerprint toggles when it is present.
//
// The module is AUR-only (pam-fprint-grosshack) and ships in aur.packages, but
// that set is installed once at post-install and never revisited by
// `ryoku update`, so a box that predates the entry -- or that skipped the AUR
// step -- has the whole fingerprint-unlock feature wired up with the module
// missing: fprintd's own enroll/verify (Ryoku Settings) still works, but the
// lock and greeter silently do nothing on a touch, and PAM logs a "cannot open
// pam_fprintd_grosshack.so / faulty module" line on every unlock attempt.
//
// Converge it on a box with a fingerprint reader when the host package lane
// provides the module. Report-only in checkOnly. Silent on a machine with no
// reader, so a desktop is never made to build a package it cannot use.

// fingerprintModulePath is where the grosshack PAM module lands, the exact path
// the lock's PAM stack and the Hub probe. A var so a test can point it elsewhere.
var fingerprintModulePath = "/usr/lib/security/pam_fprintd_grosshack.so"

// fingerprintReaderPresent reports whether fprintd sees a fingerprint device. A
// var seam so planFingerprintModule stays a pure function testable without
// hardware. fprintd-list enumerates the reader(s) for a user and prints each
// device's object path; no reader (or no fprintd) yields neither, i.e. false.
var fingerprintReaderPresent = func() bool {
	if !sys.Has("fprintd-list") {
		return false
	}
	user := os.Getenv("USER")
	if user == "" {
		return false
	}
	out, err := sys.RunOut("fprintd-list", user)
	if err != nil {
		return false
	}
	return strings.Contains(out, "/net/reactivated/Fprint/Device")
}

var (
	fingerprintProviderAvailable = func() bool {
		app := host.Default()
		manager, err := app.PackageManager()
		if err != nil {
			return false
		}
		if manager == host.Pacman {
			return true
		}
		return app.Package([]string{"available", "pam-fprint-grosshack"}) == host.ExitOK
	}
	fingerprintInstall = func() int {
		manager, _ := host.Default().PackageManager()
		if manager == host.DNF {
			return host.Default().Package([]string{"install", "pam-fprint-grosshack"})
		}
		return host.Default().Package([]string{"install", "--aur", "pam-fprint-grosshack"})
	}
	fingerprintInstallAdvice = func() string {
		return host.Default().InstallAdvice("pam-fprint-grosshack")
	}
	fingerprintFailureAdvice = func() string {
		manager, _ := host.Default().PackageManager()
		if manager == host.Pacman {
			return "ryoku-pkg-aur-add pam-fprint-grosshack, then sudo ryoku doctor"
		}
		return host.Default().InstallAdvice("pam-fprint-grosshack")
	}
)

// planFingerprintModule decides the result from the two facts it depends on: is
// there a reader, and is the module already installed. pure, so every branch is
// unit-testable without fprintd or an AUR helper.
func planFingerprintModule(readerPresent, moduleInstalled, checkOnly bool) (recResult, bool) {
	switch {
	case !readerPresent:
		return okRes(i18n.T("no fingerprint reader; the unlock module is not needed")), false
	case moduleInstalled:
		return okRes(i18n.T("fingerprint unlock module (pam-fprint-grosshack) present")), false
	case checkOnly:
		return wouldRes(i18n.T("install the fingerprint unlock module so touch-to-unlock works at the lock and login screens")).
			withFix(i18n.T("ryoku doctor installs pam-fprint-grosshack (AUR)")), false
	default:
		return recResult{}, true // caller performs the install
	}
}

func reconcileFingerprintModule(checkOnly bool) recResult {
	readerPresent := fingerprintReaderPresent()
	moduleInstalled := sys.Exists(fingerprintModulePath)
	if readerPresent && !moduleInstalled && !fingerprintProviderAvailable() {
		return noteRes(i18n.T("fingerprint unlock module is not packaged for this system")).
			withFix(fingerprintInstallAdvice())
	}
	res, install := planFingerprintModule(readerPresent, moduleInstalled, checkOnly)
	if dnfHost() && res.status == recWouldFix {
		res.remedy = i18n.T("ryoku doctor installs pam-fprint-grosshack from the configured Fedora repositories")
	}
	if !install {
		return res
	}
	if code := fingerprintInstall(); code != host.ExitOK {
		return failRes(i18n.T("could not install the fingerprint unlock module (exit %d)"), code).
			withFix(fingerprintFailureAdvice())
	}
	if !sys.Exists(fingerprintModulePath) {
		return failRes(i18n.T("pam-fprint-grosshack installed but %s is missing"), fingerprintModulePath).
			withFix(fingerprintFailureAdvice())
	}
	return fixedRes(i18n.T("installed the fingerprint unlock module (pam-fprint-grosshack); touch-to-unlock now works at the lock and login screens"))
}
