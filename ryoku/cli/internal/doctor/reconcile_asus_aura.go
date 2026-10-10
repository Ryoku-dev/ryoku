package doctor

import (
	"fmt"
	"os/exec"

	"ryoku-cli/internal/host"

	i18n "ryoku-i18n"
)

type asusAuraStatus struct {
	supported bool
	installed bool
	tlp       bool
	running   bool
}

var (
	readAsusAuraStatus   = probeAsusAuraStatus
	asusPackageInstalled = func(name string) bool {
		return host.Default().Package([]string{"installed", name}) == host.ExitOK
	}
	asusProviderAvailable = func() bool {
		app := host.Default()
		manager, err := app.PackageManager()
		if err != nil {
			return false
		}
		if manager == host.Pacman {
			return true
		}
		return app.Package([]string{"available", "asusctl"}) == host.ExitOK
	}
	asusInstallAdvice = func() string { return host.Default().InstallAdvice("asusctl") }
	asusService       = func(args []string) int { return host.Default().Service(args) }
	installAsusAura   = func() error {
		if code := host.Default().Package([]string{"install", "asusctl"}); code != host.ExitOK {
			return fmt.Errorf("package install exited %d", code)
		}
		return nil
	}
	startAsusAura = func() error {
		if code := asusService([]string{"--system", "start", "asusd"}); code != host.ExitOK {
			return fmt.Errorf("service start exited %d", code)
		}
		return nil
	}
	asusServiceRunning = func() bool {
		return asusService([]string{"--system", "is-active", "asusd"}) == host.ExitOK
	}
	asusServiceAdvice = func() string {
		init, err := host.Default().Init()
		if err == nil && init == host.Systemd {
			return "sudo systemctl start asusd.service"
		}
		return "sudo ryoku-host svc --system start asusd"
	}
)

func probeAsusAuraStatus() asusAuraStatus {
	st := asusAuraStatus{supported: exec.Command("ryoku-hw-asus-aura").Run() == nil}
	if !st.supported {
		return st
	}
	st.installed = asusPackageInstalled("asusctl")
	st.tlp = asusPackageInstalled("tlp")
	st.running = asusServiceRunning()
	return st
}

func reconcileAsusAura(checkOnly bool) recResult {
	st := readAsusAuraStatus()
	if !st.supported {
		return okRes(i18n.T("this machine has no supported ASUS Aura laptop controller"))
	}
	if !st.installed && !asusProviderAvailable() {
		return noteRes(i18n.T("ASUS Aura keyboard provider is not packaged for this system")).
			withFix(asusInstallAdvice())
	}
	if !st.installed && st.tlp {
		return warnRes(i18n.T("ASUS Aura keyboard support needs asusctl, which conflicts with the installed TLP power stack")).
			withFix(i18n.T("choose TLP or ASUS Aura control; remove TLP before installing asusctl"))
	}
	if !st.installed {
		if removedByUser("asusctl") {
			return okRes(i18n.T("asusctl was removed by hand; leaving the Aura keyboard unmanaged"))
		}
		if checkOnly {
			return wouldRes(i18n.T("ASUS Aura keyboard provider is missing")).
				withFix(i18n.T("ryoku doctor installs asusctl and starts asusd"))
		}
		if err := installAsusAura(); err != nil {
			return failRes(i18n.T("could not install the ASUS Aura provider: %v"), err).
				withFix(asusInstallAdvice())
		}
		recordProvisioned("asusctl")
	}
	if st.installed && st.running {
		return okRes(i18n.T("ASUS Aura keyboard provider is installed and running"))
	}
	if checkOnly {
		return wouldRes(i18n.T("asusd is installed but not running, so the Aura keyboard is absent from Appearance")).
			withFix(i18n.T("ryoku doctor starts asusd"))
	}
	if err := startAsusAura(); err != nil {
		return failRes(i18n.T("could not start the ASUS Aura provider: %v"), err).
			withFix(asusServiceAdvice())
	}
	if st.installed {
		return fixedRes(i18n.T("started asusd; the ASUS Aura keyboard is available in Appearance"))
	}
	return fixedRes(i18n.T("installed asusctl and started asusd; the ASUS Aura keyboard is available in Appearance"))
}
