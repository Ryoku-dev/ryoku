package doctor

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"ryoku-cli/internal/sys"

	i18n "ryoku-i18n"
)

const hermesAgentInstallDir = ".hermes/hermes-agent"

func reconcileHermesAgent(_ bool) recResult {
	version, installed := installedHermesAgentVersion()
	if !installed {
		return okRes(i18n.T("Hermes agent not installed"))
	}

	result := noteRes(i18n.T("Hermes agent is installed; its version could not be read"))
	if version != "" {
		result = noteRes(i18n.T("Hermes agent %s is installed; Ryoku setup leaves existing installs untouched"), version)
	}
	return result.withFix(i18n.T("update Hermes through its own installer or checkout, then run `ryoku-rashin setup` to refresh its Ryoku wiring"))
}

func installedHermesAgentVersion() (string, bool) {
	if bin := findHermesAgentBinary(); bin != "" {
		return probeHermesAgentVersion(bin), true
	}
	info, err := os.Stat(filepath.Join(sys.Home(), hermesAgentInstallDir))
	return "", err == nil && info.IsDir()
}

func findHermesAgentBinary() string {
	if bin, err := exec.LookPath("hermes"); err == nil {
		return bin
	}
	for _, bin := range []string{
		filepath.Join(sys.Home(), ".hermes", "bin", "hermes"),
		filepath.Join(sys.Home(), ".local", "bin", "hermes"),
	} {
		if info, err := os.Stat(bin); err == nil && !info.IsDir() {
			return bin
		}
	}
	return ""
}

func probeHermesAgentVersion(bin string) string {
	out := runBoundedProbe(exec.Command(bin, "--version"), 2*time.Second, func(out string) bool {
		return strings.Contains(out, "\n")
	})
	line, _, _ := strings.Cut(strings.TrimSpace(out), "\n")
	for _, field := range strings.Fields(line) {
		if version := normalizeHermesAgentVersion(strings.Trim(field, "(),;[]")); version != "" {
			return version
		}
	}
	return ""
}

func normalizeHermesAgentVersion(value string) string {
	version := strings.TrimPrefix(strings.TrimSpace(value), "v")
	if version == "" || version[0] < '0' || version[0] > '9' || !strings.Contains(version, ".") {
		return ""
	}
	for _, r := range version {
		if (r >= '0' && r <= '9') || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') ||
			r == '.' || r == '-' || r == '+' {
			continue
		}
		return ""
	}
	return version
}
