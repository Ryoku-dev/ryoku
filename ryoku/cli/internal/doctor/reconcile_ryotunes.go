package doctor

import (
	"bufio"
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"ryoku-cli/internal/apprelease"
	"ryoku-cli/internal/host"
	"ryoku-cli/internal/sys"

	i18n "ryoku-i18n"
)

const (
	ryotunesSocketUnit       = "ryotunesd.socket"
	legacyRyoportDesktopFile = "ryovm.desktop"
)

var (
	ryotunesEnsure  = apprelease.Ryotunes.Ensure
	ryoportEnsure   = apprelease.Ryoport.Ensure
	appPackageOwner = func(path string) (string, error) {
		return host.Default().PackageOwner(path)
	}
)

type appReconcileSpec struct {
	app        apprelease.App
	ensure     func(context.Context) (apprelease.Status, error)
	socketUnit string
	staleUser  func() (string, func())
	legacyDrop func() (bool, string, func())
}

func reconcileRyotunes(checkOnly bool) recResult {
	bin := filepath.Join(sys.Home(), ".local", "bin", "ryotunes")
	return reconcileIndependentApp(appReconcileSpec{
		app: apprelease.Ryotunes, ensure: ryotunesEnsure, socketUnit: ryotunesSocketUnit,
		staleUser: func() (string, func()) {
			stale := staleUserRyotunes(bin)
			if stale == "" {
				return "", nil
			}
			return i18n.Tf("%s in ~/.local/bin shadows the packaged app", stale), func() {
				removeStaleUserRyotunes(bin)
			}
		},
	}, checkOnly)
}

func reconcileRyoport(checkOnly bool) recResult {
	return reconcileIndependentApp(appReconcileSpec{
		app: apprelease.Ryoport, ensure: ryoportEnsure,
		legacyDrop: func() (bool, string, func()) {
			path := filepath.Join(sys.Xdg("XDG_DATA_HOME", ".local/share"), "applications", legacyRyoportDesktopFile)
			data, err := os.ReadFile(path)
			if err != nil || !ryoportDesktopOptedOut(data) {
				return false, "", nil
			}
			return true, i18n.T("the old Ryoport launcher opt-out is still recorded"), func() { _ = os.Remove(path) }
		},
	}, checkOnly)
}

func ryoportDesktopOptedOut(data []byte) bool {
	scanner := bufio.NewScanner(bytes.NewReader(data))
	inDesktopEntry := false
	hidden, noDisplay := false, false
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			inDesktopEntry = line == "[Desktop Entry]"
			continue
		}
		if !inDesktopEntry || line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		switch strings.TrimSpace(key) {
		case "Hidden":
			hidden = strings.TrimSpace(value) == "true"
		case "NoDisplay":
			noDisplay = strings.TrimSpace(value) == "true"
		}
	}
	if scanner.Err() != nil {
		return false
	}
	return hidden || noDisplay
}

func reconcileIndependentApp(spec appReconcileSpec, checkOnly bool) recResult {
	manager, err := doctorPackageManager()
	if err != nil || manager != host.Pacman && manager != host.XBPS && manager != host.DNF {
		return noteRes(i18n.T("%s package checks are not available on this host"), spec.app.DisplayName)
	}

	var problems, fixes []string
	var apply []func()
	installed := doctorPackageInstalled(spec.app.Name)
	if !checkOnly && installed {
		recordProvisioned(spec.app.Name)
	}
	legacyRemoved := false
	if spec.legacyDrop != nil {
		found, problem, fix := spec.legacyDrop()
		legacyRemoved = found
		if legacyRemoved {
			problems = append(problems, problem)
			fixes = append(fixes, "ryoku doctor")
			if fix != nil {
				apply = append(apply, fix)
			}
		}
	}
	if spec.staleUser != nil {
		if problem, fix := spec.staleUser(); problem != "" {
			problems = append(problems, problem)
			fixes = append(fixes, "ryoku doctor")
			if fix != nil {
				apply = append(apply, fix)
			}
		}
	}

	installFix := doctorInstallAdvice(spec.app.Name)
	if manager == host.Pacman {
		installFix = "ryoku update"
	}
	managedDesktop := sys.ResolveRepo() != "" || doctorPackageInstalled("ryoku-desktop")
	removed := removedByUser(spec.app.Name) || legacyRemoved
	missing := managedDesktop && !installed && !removed
	if missing {
		problems = append(problems, i18n.Tf("the %s package is not installed", spec.app.Name))
		fixes = append(fixes, installFix)
	}
	socketMissing := installed && spec.socketUnit != "" && manager != host.XBPS && !userSocketEnabled(spec.socketUnit)
	if socketMissing {
		problems = append(problems, i18n.Tf("the %s socket is not enabled for session activation", spec.socketUnit))
		fixes = append(fixes, "systemctl --user enable --now "+spec.socketUnit)
	}

	if len(problems) == 0 {
		if sys.Exists(spec.app.Binary) {
			if owner, err := appPackageOwner(spec.app.Binary); err != nil || owner != spec.app.Name {
				return warnRes(i18n.T("%s is not owned by the %s package"), spec.app.Binary, spec.app.Name).
					withFix(reinstallAppAdvice(manager, spec.app))
			}
		}
		if manager == host.Pacman {
			if note, ok := appUpdateNote(spec.app); ok {
				return note
			}
		}
		return okRes(i18n.T("%s is the packaged app"), spec.app.Name)
	}
	if checkOnly {
		return wouldRes("%s", strings.Join(problems, "; ")).withFix(strings.Join(fixes, " && "))
	}
	if legacyRemoved {
		recordProvisioned(spec.app.Name)
	}
	for _, fix := range apply {
		fix()
	}
	if missing {
		if manager == host.Pacman {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
			_, err = spec.ensure(ctx)
			cancel()
		} else {
			err = sys.Sudo("ryoku-host", "pkg", "install", spec.app.Name)
		}
		if err != nil {
			return failRes(i18n.T("could not install %s: %v"), spec.app.Name, err).withFix(installFix)
		}
		recordProvisioned(spec.app.Name)
	}
	if socketMissing {
		_ = exec.Command("systemctl", "--user", "daemon-reload").Run()
		if err := exec.Command("systemctl", "--user", "enable", "--now", spec.socketUnit).Run(); err != nil {
			return failRes(i18n.T("could not enable %s: %v"), spec.socketUnit, err).
				withFix("systemctl --user enable --now " + spec.socketUnit)
		}
	}
	return fixedRes(i18n.T("%s opens the packaged app (%s)"), spec.app.Name, strings.Join(problems, "; "))
}

func reinstallAppAdvice(manager host.PackageManager, app apprelease.App) string {
	switch manager {
	case host.Pacman:
		return "sudo pacman -S --overwrite " + app.Binary + " " + app.Name
	case host.DNF:
		return "sudo dnf reinstall -y " + app.Name
	case host.XBPS:
		return "sudo xbps-install -fy " + app.Name
	default:
		return doctorInstallAdvice(app.Name)
	}
}

func appUpdateNote(app apprelease.App) (recResult, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	st, err := app.Check(ctx)
	if err != nil {
		return noteRes(i18n.T("could not check %s releases: %v"), app.DisplayName, err), true
	}
	if !st.Available {
		return recResult{}, false
	}
	return noteRes(i18n.T("a newer %s (%s) is available; `ryoku update` installs it"), app.DisplayName, st.Latest).
		withFix("ryoku update"), true
}

func userSocketEnabled(unit string) bool {
	out, _ := exec.Command("systemctl", "--user", "is-enabled", unit).Output()
	return strings.TrimSpace(string(out)) == "enabled"
}

func removeStaleUserRyotunes(bin string) {
	appshare := sys.Xdg("XDG_DATA_HOME", ".local/share")
	for _, path := range []string{
		bin,
		filepath.Join(appshare, "applications", "ryotunes.desktop"),
		filepath.Join(appshare, "ryoku", "ryotunes.commit"),
		filepath.Join(appshare, "icons", "hicolor", "scalable", "apps", "ryotunes.svg"),
	} {
		_ = os.Remove(path)
	}
	icons, _ := filepath.Glob(filepath.Join(appshare, "icons", "hicolor", "*", "apps", "ryotunes.png"))
	for _, path := range icons {
		_ = os.Remove(path)
	}
}

func staleUserRyotunes(bin string) string {
	st, err := os.Stat(bin)
	if err != nil || st.IsDir() {
		return ""
	}
	head := make([]byte, 4096)
	if file, err := os.Open(bin); err == nil {
		n, _ := file.Read(head)
		file.Close()
		head = head[:n]
	}
	if strings.HasPrefix(string(head), "#!") && strings.Contains(string(head), "music.youtube.com") {
		return "the Chromium YouTube Music wrapper"
	}
	if sys.Exists(filepath.Join(sys.Xdg("XDG_DATA_HOME", ".local/share"), "ryoku", "ryotunes.commit")) {
		return "a locally built ryotunes"
	}
	return ""
}
