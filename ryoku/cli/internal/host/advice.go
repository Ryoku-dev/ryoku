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
	table, err := a.xbpsTable()
	if err != nil {
		parts := make([]string, 0, len(names))
		for _, name := range names {
			parts = append(parts, fmt.Sprintf("%s is not packaged for this system", name))
		}
		return strings.Join(parts, "; ")
	}
	translated, unavailable := translatePackages(table, names)
	parts := make([]string, 0, len(unavailable)+1)
	if len(translated) > 0 {
		parts = append(parts, "sudo xbps-install -S "+strings.Join(translated, " "))
	}
	for _, name := range unavailable {
		parts = append(parts, fmt.Sprintf("%s is not packaged for this system", name))
	}
	return strings.Join(parts, "; ")
}
