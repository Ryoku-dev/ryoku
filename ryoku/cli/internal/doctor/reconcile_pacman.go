package doctor

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"ryoku-cli/internal/sys"

	i18n "ryoku-i18n"
)

// Ryoku's pacman progress bar. The installer sets this in the target's
// pacman.conf (installation/backend/lib/mirrors.sh); this reconciler delivers it
// to a box installed before it shipped. Purely cosmetic: pacman draws the
// transfer bar as Pac-Man eating pellets instead of a row of hashes.
const pacmanCandyDirective = "ILoveCandy"

const pacmanCandyMigration = "pacman-ilovecandy"

// A commented-out directive, as CachyOS-derived configs ship it.
var pacmanCandyCommented = regexp.MustCompile(`^#+[[:space:]]*` + pacmanCandyDirective + `[[:space:]]*$`)

func pacmanCandyMarker() string {
	return filepath.Join(sys.StateDir(), "migrations", pacmanCandyMigration)
}

// enableILoveCandy activates the directive inside [options]: it uncomments a
// commented line in place when the config ships one, else inserts the directive
// under the [options] header. ok is false when there is no [options] section to
// put it in; guessing a spot in a config pacman is about to parse is worse than
// reporting it. changed is false when the directive is already active.
func enableILoveCandy(conf []byte) (out []byte, changed, ok bool) {
	lines := strings.Split(string(conf), "\n")
	section, header, commented := "", -1, -1
	for i, l := range lines {
		t := strings.TrimSpace(l)
		if strings.HasPrefix(t, "[") && strings.HasSuffix(t, "]") {
			section = t
			if section == "[options]" && header < 0 {
				header = i
			}
			continue
		}
		if section != "[options]" {
			continue
		}
		if t == pacmanCandyDirective {
			return conf, false, true
		}
		if commented < 0 && pacmanCandyCommented.MatchString(t) {
			commented = i
		}
	}
	if header < 0 {
		return nil, false, false
	}
	if commented >= 0 {
		lines[commented] = pacmanCandyDirective
	} else {
		lines = append(lines[:header+1], append([]string{pacmanCandyDirective}, lines[header+1:]...)...)
	}
	return []byte(strings.Join(lines, "\n")), true, true
}

// reconcilePacmanCandy seeds the ILoveCandy default into /etc/pacman.conf once,
// for a box installed before the installer set it. The marker makes the seed
// one-shot, so deleting the line is a decision that sticks: /etc/pacman.conf is
// the user's file (pacman tracks it as a backup file) and nothing in Ryoku
// rewrites it wholesale.
func reconcilePacmanCandy(checkOnly bool) recResult {
	if !hasPacman() {
		return okRes(i18n.T("not a pacman host; there is no pacman progress bar to configure"))
	}
	const conf = "/etc/pacman.conf"
	marker := pacmanCandyMarker()
	if sys.Exists(marker) {
		return okRes(i18n.T("pacman progress bar already seeded (yours to change)"))
	}

	live, err := os.ReadFile(conf)
	if err != nil {
		return warnRes(i18n.T("could not read %s: %v"), conf, err).
			withFix(i18n.T("fix the file permissions, then run `ryoku doctor`"))
	}
	next, changed, ok := enableILoveCandy(live)
	if !ok {
		return warnRes(i18n.T("%s has no [options] section to hold the progress-bar default"), conf).
			withFix(i18n.T("repair %s (`pacman-conf` parses it), then run `ryoku doctor`"), conf)
	}
	if !changed {
		if checkOnly {
			return okRes(i18n.T("pacman draws Ryoku's candy progress bar"))
		}
		if err := markMigration(marker); err != nil {
			return failRes(i18n.T("could not record the pacman progress-bar seed: %v"), err)
		}
		return okRes(i18n.T("pacman draws Ryoku's candy progress bar"))
	}
	if checkOnly {
		return wouldRes(i18n.T("pacman still draws the stock hash progress bar")).
			withFix(i18n.T("ryoku doctor  (adds ILoveCandy under [options] in %s)"), conf)
	}
	if err := writeRootFile(conf, string(next), "0644"); err != nil {
		return failRes(i18n.T("could not add the progress-bar default to %s: %v"), conf, err).
			withFix("sudo sed -i '/^\\[options\\]/a %s' %s", pacmanCandyDirective, conf)
	}
	if err := markMigration(marker); err != nil {
		return failRes(i18n.T("set the pacman progress bar, but its seed marker could not be written: %v"), err).
			withFix(i18n.T("run `ryoku doctor` again"))
	}
	return fixedRes(i18n.T("set pacman's candy progress bar in %s (delete the ILoveCandy line to go back; it is not re-added)"), conf)
}
