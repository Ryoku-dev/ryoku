package doctor

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"ryoku-cli/internal/ryotunesrelease"
	"ryoku-cli/internal/sys"

	i18n "ryoku-i18n"
)

// ryotunesSocketUnit provides native daemon socket activation. The launcher can
// start it on demand; enabling it also makes cold session activation available.
const ryotunesSocketUnit = "ryotunesd.socket"

// Ryotunes is part of the Ryoku desktop (ryoku-desktop depends on it) but is
// delivered on its own GitHub release channel rather than built in the [ryoku]
// repo: the repo imports and re-signs the official checksummed epoch=1 build,
// and a box also tracks it directly (internal/ryotunesrelease). Two things keep
// a box from opening the packaged native client: a wrapper or a locally built
// copy in ~/.local/bin that shadows /usr/bin/ryotunes on PATH (a dev deploy laid
// both before the package existed), and a managed box -- a dev checkout
// (ResolveRepo) or one with the packaged ryoku-desktop -- simply missing the
// package -- which the reconcile installs from the official release (never the
// stale repo copy), so it lands the current build even before a repo re-import
// has propagated.
func reconcileRyotunes(checkOnly bool) recResult {
	var problems, fixes []string
	// Deliver-once like the shipped apps: while the package is here, keep it in
	// the provisioned ledger, so a later deletion reads as the user's choice
	// rather than a gap to heal. recordProvisioned is a no-op when recorded,
	// and a check-only run never writes.
	if !checkOnly && sys.PkgInstalled("ryotunes") {
		recordProvisioned("ryotunes")
	}

	bin := filepath.Join(sys.Home(), ".local", "bin", "ryotunes")
	stale := staleUserRyotunes(bin)
	if stale != "" {
		problems = append(problems, i18n.Tf("%s in ~/.local/bin shadows the packaged app", stale))
		fixes = append(fixes, "rm -f ~/.local/bin/ryotunes ~/.local/share/applications/ryotunes.desktop")
	}
	// A box that runs the Ryoku desktop is expected to have Ryotunes: a dev
	// checkout (all Ryoku managed by `ryoku deploy`, so ryoku-desktop is not a
	// pacman package) or a packaged install (ryoku-desktop present). Either way,
	// if the app is absent the reconcile installs the current official build --
	// unless the user deleted it: ryotunes is a deliver-once app like the rest
	// of the shipped set, so the provisioned ledger is honoured here too.
	managedDesktop := sys.ResolveRepo() != "" || sys.PkgInstalled("ryoku-desktop")
	desktopMissingRyotunes := managedDesktop && !sys.PkgInstalled("ryotunes") && !removedByUser("ryotunes")
	if desktopMissingRyotunes {
		problems = append(problems, i18n.T("the ryotunes package is not installed"))
		fixes = append(fixes, "ryoku update")
	}
	socketMissing := sys.PkgInstalled("ryotunes") && !ryotunesSocketEnabled()
	if socketMissing {
		problems = append(problems, i18n.T("the ryotunesd socket is not enabled for session activation"))
		fixes = append(fixes, "systemctl --user enable --now ryotunesd.socket")
	}
	if len(problems) == 0 {
		if _, err := sys.RunOut("pacman", "-Qoq", "/usr/bin/ryotunes"); err != nil && sys.Exists("/usr/bin/ryotunes") {
			return warnRes(i18n.T("/usr/bin/ryotunes is not owned by the ryotunes package")).
				withFix("sudo pacman -S --overwrite /usr/bin/ryotunes ryotunes")
		}
		if note, ok := ryotunesUpdateNote(); ok {
			return note
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
		// Install from the official GitHub release, verified and re-checked
		// against its own pacman metadata, rather than `pacman -S` from the
		// [ryoku] repo: it delivers the current native build on any box (dev
		// checkout or packaged) without depending on the repo being configured
		// or a re-import having propagated. Ryotunes' own channel, like Upgrade.
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		_, err := ryotunesrelease.Ensure(ctx)
		cancel()
		if err != nil {
			return failRes(i18n.T("could not install ryotunes: %v"), err).withFix("ryoku update")
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
