package doctor

import (
	"fmt"
	"os/exec"

	"ryoku-cli/internal/host"
	"ryoku-cli/internal/sys"

	i18n "ryoku-i18n"
)

type qmkStatus struct {
	supported bool
	installed bool
}

var (
	readQMKStatus       = probeQMKStatus
	qmkPackageInstalled = func() bool {
		return host.Default().Package([]string{"installed", "qmk-hid"}) == host.ExitOK
	}
	qmkProviderAvailable = func() bool {
		app := host.Default()
		manager, err := app.PackageManager()
		if err != nil {
			return false
		}
		if manager == host.Pacman {
			return true
		}
		return app.Package([]string{"available", "qmk-hid"}) == host.ExitOK
	}
	qmkInstallAdvice = func() string { return host.Default().InstallAdvice("qmk-hid") }
	installQMK       = func() error {
		if code := host.Default().Package([]string{"install", "--aur", "qmk-hid"}); code != host.ExitOK {
			return fmt.Errorf("package install exited %d", code)
		}
		return nil
	}
	reloadQMKUdev = func() error {
		if err := sys.Sudo("udevadm", "control", "--reload"); err != nil {
			return err
		}
		return sys.Sudo("udevadm", "trigger", "--subsystem-match=hidraw")
	}
)

// probeQMKStatus asks the shared hardware detector whether a QMK/VIA keyboard is
// connected (by its 0xFF60 raw HID interface, so no qmk_hid needed), then checks
// whether the provider tool is already installed.
func probeQMKStatus() qmkStatus {
	st := qmkStatus{supported: exec.Command("ryoku-hw-qmk").Run() == nil}
	if !st.supported {
		return st
	}
	st.installed = qmkPackageInstalled()
	return st
}

// reconcileQMK installs qmk_hid when a VIA keyboard is present and leaves every
// other machine untouched. The uaccess udev rule and the detector ship with the
// desktop package, so once the tool is in place the shell's lighting provider
// drives the keyboard as the seat user; a udev reload applies the ACL to a board
// that was already plugged in when the tool arrived.
func reconcileQMK(checkOnly bool) recResult {
	st := readQMKStatus()
	if !st.supported {
		return okRes(i18n.T("this machine has no QMK/VIA keyboard for lighting"))
	}
	if st.installed {
		return okRes(i18n.T("QMK/VIA keyboard lighting provider is installed"))
	}
	if !qmkProviderAvailable() {
		return noteRes(i18n.T("QMK/VIA keyboard lighting provider is not packaged for this system")).
			withFix(qmkInstallAdvice())
	}
	if checkOnly {
		return wouldRes(i18n.T("QMK/VIA keyboard lighting provider is missing")).
			withFix(i18n.T("ryoku doctor installs qmk-hid so the keyboard follows the theme"))
	}
	if err := installQMK(); err != nil {
		return failRes(i18n.T("could not install the QMK lighting provider: %v"), err).
			withFix(qmkInstallAdvice())
	}
	_ = reloadQMKUdev()
	return fixedRes(i18n.T("installed qmk-hid; the QMK/VIA keyboard is available in Appearance"))
}
