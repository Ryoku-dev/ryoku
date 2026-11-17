package doctor

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"ryoku-cli/internal/host"
	"ryoku-cli/internal/ryotunesrelease"
	"ryoku-cli/internal/sys"

	i18n "ryoku-i18n"
)

// ryotunesSocketUnit provides native daemon socket activation. The launcher can
// start it on demand; enabling it also makes cold session activation available.
const ryotunesSocketUnit = "ryotunesd.socket"

var (
	ryotunesEnsure = ryotunesrelease.Ensure
	ryotunesOwner  = func(path string) (string, error) {
		return host.Default().PackageOwner(path)
	}
)

// Ryotunes is part of the Ryoku desktop and ships as a native repository package
// on every supported edition. Arch also follows the app's independently
// published package between repository imports. The reconcile removes stale
// user-local launchers, delivers the package once on managed desktops, and keeps
// socket activation ready where systemd user services are available.
func reconcileRyotunes(checkOnly bool) recResult {
	manager, err := doctorPackageManager()
	if err != nil || manager != host.Pacman && manager != host.XBPS && manager != host.DNF {
		return noteRes(i18n.T("Ryotunes package checks are not available on this host"))
	}
	var problems, fixes []string
	installed := doctorPackageInstalled("ryotunes")
	if !checkOnly && installed {
		recordProvisioned("ryotunes")
	}

	bin := filepath.Join(sys.Home(), ".local", "bin", "ryotunes")
	stale := staleUserRyotunes(bin)
	if stale != "" {
		problems = append(problems, i18n.Tf("%s in ~/.local/bin shadows the packaged app", stale))
		fixes = append(fixes, "rm -f ~/.local/bin/ryotunes ~/.local/share/applications/ryotunes.desktop")
	}
	installFix := doctorInstallAdvice("ryotunes")
	if manager == host.Pacman {
		installFix = "ryoku update"
	}
	managedDesktop := sys.ResolveRepo() != "" || doctorPackageInstalled("ryoku-desktop")
	desktopMissingRyotunes := managedDesktop && !installed && !removedByUser("ryotunes")
	if desktopMissingRyotunes {
		problems = append(problems, i18n.T("the ryotunes package is not installed"))
		fixes = append(fixes, installFix)
	}
	socketMissing := installed && manager != host.XBPS && !ryotunesSocketEnabled()
	if socketMissing {
		problems = append(problems, i18n.T("the ryotunesd socket is not enabled for session activation"))
		fixes = append(fixes, "systemctl --user enable --now ryotunesd.socket")
	}
	if len(problems) == 0 {
		if sys.Exists("/usr/bin/ryotunes") {
			if owner, err := ryotunesOwner("/usr/bin/ryotunes"); err != nil || owner != "ryotunes" {
				fix := doctorInstallAdvice("ryotunes")
				switch manager {
				case host.Pacman:
					fix = "sudo pacman -S --overwrite /usr/bin/ryotunes ryotunes"
				case host.DNF:
					fix = "sudo dnf reinstall -y ryotunes"
				case host.XBPS:
					fix = "sudo xbps-install -fy ryotunes"
				}
				return warnRes(i18n.T("/usr/bin/ryotunes is not owned by the ryotunes package")).withFix(fix)
			}
		}
		if manager == host.Pacman {
			if note, ok := ryotunesUpdateNote(); ok {
				return note
			}
		}
		return okRes(i18n.T("ryotunes is the packaged app"))
	}
	if checkOnly {
		return wouldRes("%s", strings.Join(problems, "; ")).withFix(strings.Join(fixes, " && "))
	}
	if stale != "" {
		appshare := sys.Xdg("XDG_DATA_HOME", ".local/share")
		for _, p := range []string{
			bin,
			filepath.Join(appshare, "applications", "ryotunes.desktop"),
			filepath.Join(appshare, "ryoku", "ryotunes.commit"),
			filepath.Join(appshare, "icons", "hicolor", "scalable", "apps", "ryotunes.svg"),
		} {
			_ = os.Remove(p)
		}
		icons, _ := filepath.Glob(filepath.Join(appshare, "icons", "hicolor", "*", "apps", "ryotunes.png"))
		for _, p := range icons {
			_ = os.Remove(p)
		}
	}
	if desktopMissingRyotunes {
		if manager == host.Pacman {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
			_, err = ryotunesEnsure(ctx)
			cancel()
		} else {
			err = sys.Sudo("ryoku-host", "pkg", "install", "ryotunes")
		}
		if err != nil {
			return failRes(i18n.T("could not install ryotunes: %v"), err).withFix(installFix)
		}
	}
	if socketMissing {
		// daemon-reload so a unit the package just delivered is known, then
		// enable --now: the socket binds in this session without a relogin.
		_ = exec.Command("systemctl", "--user", "daemon-reload").Run()
		if err := exec.Command("systemctl", "--user", "enable", "--now", ryotunesSocketUnit).Run(); err != nil {
			return failRes(i18n.T("could not enable %s: %v"), ryotunesSocketUnit, err).
				withFix("systemctl --user enable --now ryotunesd.socket")
		}
	}
	return fixedRes(i18n.T("ryotunes opens the packaged app (%s)"), strings.Join(problems, "; "))
}

// Release availability is advisory: doctor checks but never installs. A lookup
// failure remains visible in check/report modes without failing desktop health.
func ryotunesUpdateNote() (recResult, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	st, err := ryotunesrelease.Check(ctx)
	if errors.Is(err, ryotunesrelease.ErrNoRelease) {
		return noteRes(i18n.T("no Ryotunes 1.x release is published; keeping Ryotunes %s"), st.Installed), true
	}
	if err != nil {
		return noteRes(i18n.T("could not check Ryotunes releases: %v"), err), true
	}
	if !st.Available {
		return recResult{}, false
	}
	return noteRes(i18n.T("a newer Ryotunes (%s) is available; `ryoku update` installs it"), st.Latest).
		withFix("ryoku update"), true
}

func ryotunesSocketEnabled() bool {
	out, _ := exec.Command("systemctl", "--user", "is-enabled", ryotunesSocketUnit).Output()
	return strings.TrimSpace(string(out)) == "enabled"
}

// staleUserRyotunes names what ~/.local/bin/ryotunes is when it is not the
// user's own program: the Chromium wrapper (a script that opens
// music.youtube.com) or a build the dev deploy recorded a commit for.
func staleUserRyotunes(bin string) string {
	st, err := os.Stat(bin)
	if err != nil || st.IsDir() {
		return ""
	}
	head := make([]byte, 4096)
	if f, err := os.Open(bin); err == nil {
		n, _ := f.Read(head)
		f.Close()
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
