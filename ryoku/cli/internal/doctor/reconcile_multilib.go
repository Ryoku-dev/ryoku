package doctor

import (
	"os"
	"strings"

	"ryoku-cli/internal/sys"

	i18n "ryoku-i18n"
)

// reconcileMultilibRepo heals the update failure a box hits when 32-bit
// packages are installed but [multilib] is switched off in /etc/pacman.conf:
// pacman -Syu then aborts with "failed to prepare transaction (could not
// satisfy dependencies)", naming lib32 packages whose pinned dependency can no
// longer be resolved from a synced database. It happens when a bundle or the
// NVIDIA driver step enabled the section and something later rewrote the file
// (a .pacnew merge, a hand edit, a repo-management tool), or when lib32
// packages were installed on a box that has since lost the section.
//
// Nothing is installed or removed. When lib32 packages are present and the
// section is missing, the reconciler re-enables it (uncommenting the stock
// stanza pacman's config ships, else appending it) and syncs the database.
// When no lib32 package is installed, an absent section is correct.
func reconcileMultilibRepo(checkOnly bool) recResult {
	const conf = "/etc/pacman.conf"
	if !hasPacman() {
		return okRes(i18n.T("not a pacman box; the repository set is the installer's business"))
	}
	lib32, err := installedLib32()
	if err != nil {
		return warnRes(i18n.T("could not read the installed set: %v"), err)
	}
	if len(lib32) == 0 {
		return okRes(i18n.T("no 32-bit packages installed; [multilib] is not needed"))
	}
	body, err := os.ReadFile(conf)
	if err != nil {
		return warnRes(i18n.T("could not read %s: %v"), conf, err)
	}
	if hasMultilibSection(string(body)) {
		return okRes(i18n.T("%d 32-bit package(s) update from [multilib]"), len(lib32))
	}
	if checkOnly {
		return wouldRes(i18n.T("%d 32-bit package(s) are installed but [multilib] is disabled, so `pacman -Syu` fails"), len(lib32)).
			withFix(i18n.T("ryoku doctor re-enables [multilib] in %s"), conf)
	}
	out, ok := enableMultilibSection(string(body))
	if !ok {
		return failRes(i18n.T("%s has no [multilib] stanza to enable; add it by hand"), conf).
			withFix(i18n.T("append a [multilib] section with Include = /etc/pacman.d/mirrorlist"))
	}
	if err := writeRootFile(conf, out, "0644"); err != nil {
		return failRes(i18n.T("could not write %s: %v"), conf, err)
	}
	if err := sys.Run("sudo", "pacman", "-Sy", "multilib"); err != nil {
		return warnRes(i18n.T("re-enabled [multilib], but syncing it failed: %v"), err).
			withFix("sudo pacman -Sy multilib")
	}
	return fixedRes(i18n.Tf("re-enabled [multilib] in %s so the %d 32-bit package(s) can update", conf, len(lib32)))
}

// installedLib32 lists the installed packages whose names start with lib32-.
// The query is read-only and needs no root.
func installedLib32() ([]string, error) {
	out, err := sys.RunOut("pacman", "-Qq")
	if err != nil {
		return nil, err
	}
	var names []string
	for _, ln := range strings.Split(out, "\n") {
		if n := strings.TrimSpace(ln); strings.HasPrefix(n, "lib32-") {
			names = append(names, n)
		}
	}
	return names, nil
}

// hasMultilibSection reports whether the config carries an active [multilib]
// repository header. A commented "#[multilib]" does not count: pacman never
// reads it, which is exactly the broken state this reconciler heals.
func hasMultilibSection(conf string) bool {
	for _, ln := range strings.Split(conf, "\n") {
		if strings.TrimSpace(ln) == "[multilib]" {
			return true
		}
	}
	return false
}

// enableMultilibSection activates the [multilib] repository: it uncomments the
// stock commented stanza pacman's own config ships, else appends the stanza at
// the end of the file. ok is false only when the config is empty, where
// guessing a spot in a file pacman is about to parse is worse than reporting.
func enableMultilibSection(conf string) (string, bool) {
	lines := strings.Split(conf, "\n")
	for i, ln := range lines {
		if strings.TrimSpace(ln) != "#[multilib]" {
			continue
		}
		lines[i] = "[multilib]"
		for j := i + 1; j <= i+4 && j < len(lines); j++ {
			t := strings.TrimSpace(lines[j])
			if strings.HasPrefix(t, "#Include") {
				lines[j] = strings.TrimPrefix(t, "#")
				break
			}
			if t != "" && !strings.HasPrefix(t, "#") {
				break // a different section began; appending is safer than editing it
			}
		}
		return strings.Join(lines, "\n"), true
	}
	if strings.TrimSpace(conf) == "" {
		return "", false
	}
	return conf + "\n[multilib]\nInclude = /etc/pacman.d/mirrorlist\n", true
}
