package host

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	i18n "ryoku-i18n"
)

const fingerprintPAMLine = "auth        sufficient    pam_fprintd_grosshack.so"

var fingerprintPAMTargets = []string{"sudo", "sddm", "polkit-1"}

type fingerprintStatus struct {
	Supported bool            `json:"supported"`
	Reason    string          `json:"reason,omitempty"`
	Targets   map[string]bool `json:"targets"`
}

func (a *App) Auth(args []string) int {
	if len(args) < 2 || args[0] != "fingerprint" {
		return ExitUsage
	}
	switch args[1] {
	case "status":
		if len(args) != 2 {
			return ExitUsage
		}
		return a.fingerprintStatus()
	case "set":
		if len(args) != 4 || !fingerprintTarget(args[2]) || (args[3] != "on" && args[3] != "off") {
			return ExitUsage
		}
		if a.cfg.UID != 0 {
			return commandExit(a.run("pkexec", "ryoku-host", "auth", "fingerprint", "set", args[2], args[3]))
		}
		return a.setFingerprintPAM(args[2], args[3] == "on")
	default:
		return ExitUsage
	}
}

func (a *App) fingerprintStatus() int {
	status := fingerprintStatus{Supported: a.fingerprintModulePresent(), Targets: map[string]bool{}}
	if !status.Supported {
		status.Reason = i18n.T("Fingerprint authentication integration is not installed on this edition.")
	}
	for _, target := range fingerprintPAMTargets {
		body, _, _, err := a.readPAM(target)
		status.Targets[target] = err == nil && strings.Contains(string(body), "pam_fprintd_grosshack")
	}
	if err := json.NewEncoder(a.cfg.Stdout).Encode(status); err != nil {
		return a.failf(i18n.T("encode fingerprint authentication status: %v"), err)
	}
	return ExitOK
}

func (a *App) setFingerprintPAM(target string, enabled bool) int {
	if !a.fingerprintModulePresent() {
		return a.failf("%s", i18n.T("Fingerprint authentication integration is not installed on this edition."))
	}
	path := filepath.Join(a.cfg.PAMDir, target)
	body, mode, vendor, err := a.readPAM(target)
	if err != nil {
		return a.failf(i18n.T("read %s: %v"), path, err)
	}
	if vendor && !enabled {
		// the vendor stack carries no fingerprint line, so there is nothing to undo
		fmt.Fprintln(a.cfg.Stdout, target)
		return ExitOK
	}
	if !vendor {
		if err := os.WriteFile(path+".ryoku-fp-bak", body, mode); err != nil {
			return a.failf(i18n.T("back up %s: %v"), path, err)
		}
	}

	lines := strings.Split(strings.TrimSuffix(string(body), "\n"), "\n")
	filtered := make([]string, 0, len(lines)+1)
	if enabled {
		filtered = append(filtered, fingerprintPAMLine)
	}
	for _, line := range lines {
		if !strings.Contains(line, "pam_fprintd_grosshack") {
			filtered = append(filtered, line)
		}
	}
	out := strings.Join(filtered, "\n") + "\n"
	// An /etc copy that is the vendor stack again once the line is gone goes
	// away, so the service follows the vendor file through package updates
	// again; a copy with local edits stays.
	if !enabled {
		v, err := os.ReadFile(filepath.Join(a.cfg.VendorPAMDir, target))
		if err == nil && strings.TrimSuffix(string(v), "\n") == strings.TrimSuffix(out, "\n") {
			if err := os.Remove(path); err != nil {
				return a.failf(i18n.T("remove %s: %v"), path, err)
			}
			fmt.Fprintln(a.cfg.Stdout, target)
			return ExitOK
		}
	}
	if err := os.WriteFile(path, []byte(out), mode); err != nil {
		return a.failf(i18n.T("write %s: %v"), path, err)
	}
	fmt.Fprintln(a.cfg.Stdout, target)
	return ExitOK
}

// readPAM returns a service's PAM stack and whether it came from the vendor
// directory. Newer packages (polkit 128) ship their stack only there, and PAM
// reads it whenever /etc has no file of that name, so a missing /etc file is
// not a missing service.
func (a *App) readPAM(target string) (body []byte, mode os.FileMode, vendor bool, err error) {
	path := filepath.Join(a.cfg.PAMDir, target)
	info, err := os.Stat(path)
	if err == nil {
		body, err = os.ReadFile(path)
		return body, info.Mode().Perm(), false, err
	}
	if !errors.Is(err, fs.ErrNotExist) {
		return nil, 0, false, err
	}
	vpath := filepath.Join(a.cfg.VendorPAMDir, target)
	vinfo, verr := os.Stat(vpath)
	if verr != nil {
		return nil, 0, false, err
	}
	body, err = os.ReadFile(vpath)
	return body, vinfo.Mode().Perm(), true, err
}

func (a *App) fingerprintModulePresent() bool {
	for _, path := range a.cfg.PAMModulePaths {
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			return true
		}
	}
	return false
}

func fingerprintTarget(target string) bool {
	for _, candidate := range fingerprintPAMTargets {
		if target == candidate {
			return true
		}
	}
	return false
}
