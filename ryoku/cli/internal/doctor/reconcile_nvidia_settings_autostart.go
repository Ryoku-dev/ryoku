package doctor

import (
	"os"
	"path/filepath"
	"strings"

	"ryoku-cli/internal/sys"

	i18n "ryoku-i18n"
)

// ---- reconciler: the nvidia-settings login autostart -------------------------
//
// The nvidia-settings package ships an XDG autostart entry that runs
// `nvidia-settings -l` to reload ~/.nvidia-settings-rc. Without that file it
// exits 1 on every login, so the session shows a failed unit, and on Wayland the
// X settings it would load only reach Xwayland anyway. A user-level Hidden=true
// copy turns it off. It is written once and ledgered: a user who deletes it gets
// the package's autostart back for good, and a saved rc file keeps it running.

const (
	nvidiaSettingsAutostart = "nvidia-settings-user.desktop"
	hiddenAutostartEntry    = "[Desktop Entry]\nHidden=true\n"
)

func autostartOverrideLedger() string {
	return filepath.Join(sys.StateDir(), "autostart-overrides")
}

// autostartDirs lists the XDG autostart directories, the user's first, then
// every XDG_CONFIG_DIRS entry.
func autostartDirs() (user string, system []string) {
	user = filepath.Join(sys.ConfigHome(), "autostart")
	confDirs := os.Getenv("XDG_CONFIG_DIRS")
	if confDirs == "" {
		confDirs = "/etc/xdg"
	}
	for _, d := range strings.Split(confDirs, ":") {
		if d != "" {
			system = append(system, filepath.Join(d, "autostart"))
		}
	}
	return user, system
}

func reconcileNvidiaSettingsAutostart(checkOnly bool) recResult {
	user, system := autostartDirs()
	dst := filepath.Join(user, nvidiaSettingsAutostart)
	ledgerPath := autostartOverrideLedger()
	switch {
	case firstEntry(system, nvidiaSettingsAutostart) == "":
		return okRes(i18n.T("nvidia-settings is not set to autostart"))
	case sys.Exists(dst), readLedger(ledgerPath)[nvidiaSettingsAutostart]:
		return okRes(i18n.T("the nvidia-settings autostart follows the user's choice"))
	case sys.Exists(filepath.Join(sys.Home(), ".nvidia-settings-rc")):
		return okRes(i18n.T("nvidia-settings loads the saved ~/.nvidia-settings-rc at login"))
	}
	if checkOnly {
		return wouldRes(i18n.T("nvidia-settings fails at every login: there is no ~/.nvidia-settings-rc to load")).
			withFix(i18n.T("ryoku doctor turns off the nvidia-settings autostart"))
	}
	if err := os.MkdirAll(user, 0o755); err != nil {
		return failRes(i18n.T("could not create %s: %v"), tildeOf(user), err)
	}
	if err := os.WriteFile(dst, []byte(hiddenAutostartEntry), 0o644); err != nil {
		return failRes(i18n.T("could not write %s: %v"), tildeOf(dst), err)
	}
	if err := os.MkdirAll(filepath.Dir(ledgerPath), 0o755); err != nil {
		return failRes(i18n.T("could not create %s: %v"), tildeOf(filepath.Dir(ledgerPath)), err)
	}
	f, err := os.OpenFile(ledgerPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return failRes(i18n.T("could not record %s: %v"), tildeOf(ledgerPath), err)
	}
	defer f.Close()
	if _, err := f.WriteString(nvidiaSettingsAutostart + "\n"); err != nil {
		return failRes(i18n.T("could not record %s: %v"), tildeOf(ledgerPath), err)
	}
	return fixedRes(i18n.T("turned off the nvidia-settings autostart, which had no settings to load"))
}
