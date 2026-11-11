package doctor

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"ryoku-cli/internal/sys"
	"strings"

	i18n "ryoku-i18n"
)

var installerPacmanHookNames = []string{
	"05-snap-pac-pre.hook",
	"80-limine-efi-deploy.hook",
	"zz-snap-pac-post.hook",
	"60-mkinitcpio-remove.hook",
	"90-mkinitcpio-install.hook",
}

var (
	installerPacmanHooksDir       = "/etc/pacman.d/hooks"
	installerPacmanHookLstat      = os.Lstat
	installerPacmanHookReadlink   = os.Readlink
	installerPacmanHookRemove     = removeRootFiles
	installerPacmanHookRestore    = func(from, to string) error { return sys.Sudo("mv", "-f", "--", from, to) }
	installerPacmanHooksActive    = installerPacmanHooksInUse
	installerPacmanHookBootRepair = reconcileLimineKernelImages
)

type installerPacmanHookArtifact struct {
	name    string
	path    string
	present bool
	masked  bool
	parked  bool
}

func installerPacmanHookExists(path string) bool {
	_, err := installerPacmanHookLstat(path)
	return err == nil
}

func installerCommand(cmdline []byte) bool {
	args := bytes.Split(cmdline, []byte{0})
	for i := 0; i < len(args) && i < 2; i++ {
		if filepath.Base(string(args[i])) == "ryoku-install" {
			return true
		}
	}
	return false
}

func installerPacmanHooksInUse() bool {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return false
	}
	for _, entry := range entries {
		cmdline, err := os.ReadFile(filepath.Join("/proc", entry.Name(), "cmdline"))
		if err == nil && installerCommand(cmdline) {
			return true
		}
	}
	return false
}

func installerPacmanHookArtifacts() []installerPacmanHookArtifact {
	var out []installerPacmanHookArtifact
	for _, name := range installerPacmanHookNames {
		path := filepath.Join(installerPacmanHooksDir, name)
		present := installerPacmanHookExists(path)
		target, err := installerPacmanHookReadlink(path)
		masked := err == nil && target == "/dev/null"
		parked := installerPacmanHookExists(path + ".ryoku-off")
		if masked || parked {
			out = append(out, installerPacmanHookArtifact{
				name: name, path: path, present: present, masked: masked, parked: parked,
			})
		}
	}
	return out
}

func restoreInstallerPacmanHook(artifact installerPacmanHookArtifact) error {
	parked := artifact.path + ".ryoku-off"
	switch {
	case artifact.parked && (artifact.masked || !artifact.present):
		return installerPacmanHookRestore(parked, artifact.path)
	case artifact.parked:
		return installerPacmanHookRemove(parked)
	case artifact.masked:
		return installerPacmanHookRemove(artifact.path)
	default:
		return nil
	}
}

func installerPacmanHookNamesOf(artifacts []installerPacmanHookArtifact) string {
	names := make([]string, 0, len(artifacts))
	for _, artifact := range artifacts {
		names = append(names, artifact.name)
	}
	return strings.Join(names, ", ")
}

func reconcileInstallerPacmanHooks(checkOnly bool) recResult {
	if !hasPacman() {
		return okRes(i18n.T("this host does not use pacman hooks"))
	}
	artifacts := installerPacmanHookArtifacts()
	if len(artifacts) == 0 {
		return okRes(i18n.T("installer pacman hook overrides are absent"))
	}
	names := installerPacmanHookNamesOf(artifacts)
	if installerPacmanHooksActive() {
		return noteRes(i18n.T("the installer is still using temporary pacman hook overrides: %s"), names)
	}
	if checkOnly {
		return wouldRes(i18n.T("temporary installer masks still disable kernel, bootloader, or snapshot hooks: %s"), names).
			withFix(i18n.T("ryoku doctor restores the packaged hooks and rebuilds stale boot images"))
	}
	for _, artifact := range artifacts {
		if err := restoreInstallerPacmanHook(artifact); err != nil {
			return failRes(i18n.T("could not restore pacman hook %s: %v"), artifact.name, err).
				withFix(i18n.T("remove its /dev/null mask and .ryoku-off file, then rerun ryoku doctor"))
		}
	}
	if left := installerPacmanHookArtifacts(); len(left) > 0 {
		return failRes(i18n.T("temporary installer hook overrides remain after repair: %s"), installerPacmanHookNamesOf(left))
	}

	detail := fmt.Sprintf(i18n.T("restored pacman hooks disabled by the installer: %s"), names)
	boot := installerPacmanHookBootRepair(false)
	switch boot.status {
	case recFixed:
		return fixedRes("%s; %s", detail, boot.detail)
	case recWarn, recFailed:
		return recResult{status: boot.status, detail: detail + "; " + boot.detail, remedy: boot.remedy}
	default:
		return fixedRes("%s", detail)
	}
}
