package host

import (
	"fmt"
	"strings"
)

// InstallAdvice returns an install command for the package names this host can
// provide and names anything that has no package on this host.
func (a *App) InstallAdvice(names ...string) string {
	if len(names) == 0 {
		return ""
	}
	manager, err := a.PackageManager()
	if err != nil {
		return ""
	}
	if manager == Pacman {
		return "sudo pacman -S " + strings.Join(names, " ")
	}
	if manager == DNF {
		table, err := a.fedoraTable()
		if err != nil {
			return unavailableAdvice(names)
		}
		return translatedInstallAdvice("sudo "+a.dnfCommand()+" install", table, names)
	}
	table, err := a.xbpsTable()
	if err != nil {
		return unavailableAdvice(names)
	}
	return translatedInstallAdvice("sudo xbps-install -S", table, names)
}

func translatedInstallAdvice(command string, table packageTable, names []string) string {
	translated, unavailable := translatePackages(table, names)
	parts := make([]string, 0, len(unavailable)+1)
	if len(translated) > 0 {
		parts = append(parts, command+" "+strings.Join(translated, " "))
	}
	for _, name := range unavailable {
		parts = append(parts, fmt.Sprintf("%s is not packaged for this system", name))
	}
	return strings.Join(parts, "; ")
}

func unavailableAdvice(names []string) string {
	parts := make([]string, 0, len(names))
	for _, name := range names {
		parts = append(parts, fmt.Sprintf("%s is not packaged for this system", name))
	}
	return strings.Join(parts, "; ")
}
