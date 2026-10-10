package host

import (
	"fmt"
	"os"
)

type InitSystem string

const (
	Systemd InitSystem = "systemd"
	Runit   InitSystem = "runit"
)

type PackageManager string

const (
	Pacman PackageManager = "pacman"
	XBPS   PackageManager = "xbps"
	DNF    PackageManager = "dnf"
)

func (a *App) Init() (InitSystem, error) {
	if override := a.getenv("RYOKU_HOST_INIT"); override != "" {
		switch InitSystem(override) {
		case Systemd, Runit:
			return InitSystem(override), nil
		default:
			return "", fmt.Errorf("invalid RYOKU_HOST_INIT %q", override)
		}
	}
	info, err := os.Stat(a.cfg.SystemdRuntimeDir)
	if err == nil && info.IsDir() {
		return Systemd, nil
	}
	return Runit, nil
}

func (a *App) PackageManager() (PackageManager, error) {
	if override := a.getenv("RYOKU_HOST_PKGMGR"); override != "" {
		switch PackageManager(override) {
		case Pacman, XBPS, DNF:
			return PackageManager(override), nil
		default:
			return "", fmt.Errorf("invalid RYOKU_HOST_PKGMGR %q", override)
		}
	}
	if a.cfg.Runner.LookPath("pacman") {
		return Pacman, nil
	}
	if a.cfg.Runner.LookPath("xbps-install") {
		return XBPS, nil
	}
	if a.cfg.Runner.LookPath("rpm") && (a.cfg.Runner.LookPath("dnf5") || a.cfg.Runner.LookPath("dnf")) {
		return DNF, nil
	}
	return "", fmt.Errorf("no supported package manager found")
}
