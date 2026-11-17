package host

import (
	"strconv"
	"strings"
)

// InstalledSize reports how much disk an installed package takes, in bytes, as
// the package manager records it. It is false when the package is absent or the
// manager gives no figure.
func (a *App) InstalledSize(name string) (int64, bool) {
	manager, err := a.PackageManager()
	if err != nil {
		return 0, false
	}
	var result Result
	switch manager {
	case Pacman:
		result = a.cfg.Runner.Run(Command{Name: "pacman", Args: []string{"-Qi", "--", name}, Env: []string{"LC_ALL=C"}})
		if result.Code != 0 {
			return 0, false
		}
		for _, line := range strings.Split(result.Output, "\n") {
			key, value, ok := strings.Cut(line, ":")
			if ok && strings.TrimSpace(key) == "Installed Size" {
				return parseSize(value)
			}
		}
		return 0, false
	case DNF:
		result = a.query("rpm", "-q", "--qf", "%{SIZE}", "--", name)
	case XBPS:
		result = a.query("xbps-query", "-p", "installed_size", name)
	}
	if result.Code != 0 {
		return 0, false
	}
	value := strings.TrimSpace(result.Output)
	value = strings.TrimSpace(strings.TrimPrefix(value, "installed_size:"))
	return parseSize(value)
}

// parseSize reads a byte count in the forms package managers print: plain
// bytes, pacman's "12.34 MiB" and XBPS's "12MB" (both binary multiples).
func parseSize(value string) (int64, bool) {
	value = strings.TrimSpace(value)
	end := 0
	for end < len(value) && (value[end] >= '0' && value[end] <= '9' || value[end] == '.') {
		end++
	}
	number, err := strconv.ParseFloat(value[:end], 64)
	if err != nil || number < 0 {
		return 0, false
	}
	unit := strings.TrimSuffix(strings.ToUpper(strings.TrimSpace(value[end:])), "IB")
	unit = strings.TrimSuffix(unit, "B")
	scale := map[string]float64{"": 1, "K": 1 << 10, "M": 1 << 20, "G": 1 << 30, "T": 1 << 40}[unit]
	if scale == 0 {
		return 0, false
	}
	return int64(number * scale), true
}
