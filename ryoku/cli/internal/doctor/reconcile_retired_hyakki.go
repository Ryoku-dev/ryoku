package doctor

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"ryoku-cli/internal/host"
	"ryoku-cli/internal/sys"
	"ryoku-cli/internal/vault"

	i18n "ryoku-i18n"
)

// ---- reconciler: Hyakki and Rashin leave the desktop ---------------------------

// Hyakki, Oni and Rashin left Ryoku for their own repository. The update that
// delivers that stops their services, removes their packages and moves what
// they left in the home into one folder, so nothing the user made is deleted.
// It runs once: a marker records the move, so a Hyakki the user installs again
// on purpose later is never touched.

var retiredHyakkiUnits = []string{
	"oni.socket", "oni.service", "oni-routines.timer", "oni-routines.service",
	"hyakki-usage.timer", "hyakki-usage.service",
	"rashind.socket", "rashind.service", "rashind-routines.timer", "rashind-routines.service",
	"ryoku-rashin.service",
}

var retiredHyakkiProcesses = []string{"oni", "oni-surface", "hyakki-app", "hyakki", "rashind", "rashin-app", "rashin"}

var retiredHyakkiPackages = []string{"hyakki", "ryoku-rashin"}

// Seams: the live box's answers, replaced in tests.
var (
	retiredHyakkiService = func(args []string) int { return host.Default().Service(args) }
	retiredHyakkiKill    = func(name string) {
		_ = exec.Command("pkill", "-x", "-u", strconv.Itoa(os.Getuid()), name).Run()
	}
	retiredHyakkiInstalled = func(pkg string) bool { return doctorPackageInstalled(pkg) }
	retiredHyakkiRemove    = func(pkg string) error { return removeRetiredApp(pkg) }
	retiredHyakkiNow       = time.Now
	retiredHyakkiVaultLeft = vault.LegacyPresent
)

func retiredHyakkiMarker() string {
	return filepath.Join(sys.Xdg("XDG_STATE_HOME", ".local/state"), "ryoku", "hyakki-retired")
}

// retiredHyakkiPaths names everything Hyakki and Rashin keep in the home: their
// settings, sessions, memory, models and caches, and what a checkout deploy
// installed (binaries, units, runit services, desktop entry, icon, licenses).
func retiredHyakkiPaths() []string {
	home := sys.Home()
	config := sys.ConfigHome()
	state := sys.Xdg("XDG_STATE_HOME", ".local/state")
	data := sys.Xdg("XDG_DATA_HOME", ".local/share")
	cache := sys.Xdg("XDG_CACHE_HOME", ".cache")
	paths := []string{
		filepath.Join(config, "hyakki"),
		filepath.Join(state, "hyakki"),
		filepath.Join(state, "hyakki-migration-backups"),
		filepath.Join(data, "hyakki"),
		filepath.Join(cache, "hyakki"),
		filepath.Join(config, "ryoku", "rashin.json"),
		filepath.Join(state, "ryoku", "rashin"),
		filepath.Join(state, "ryoku", "rashin-app.json"),
		filepath.Join(cache, "ryoku", "rashin"),
		filepath.Join(data, "applications", "dev.ryoku.Hyakki.desktop"),
		filepath.Join(data, "metainfo", "dev.ryoku.Hyakki.metainfo.xml"),
		filepath.Join(data, "icons", "hicolor", "scalable", "apps", "dev.ryoku.Hyakki.svg"),
		filepath.Join(data, "licenses", "hyakki"),
		filepath.Join(data, "doc", "hyakki"),
	}
	// An unmigrated Rashin 1 vault is the Ryoku vault's to take in.
	if !retiredHyakkiVaultLeft() {
		paths = append(paths, filepath.Join(data, "ryoku", "rashin"))
	}
	for _, name := range retiredHyakkiProcesses {
		paths = append(paths, filepath.Join(home, ".local", "bin", name))
	}
	for _, unit := range retiredHyakkiUnits {
		units := filepath.Join(config, "systemd", "user")
		paths = append(paths, filepath.Join(units, unit), filepath.Join(units, unit+".d"))
		paths = append(paths, filepath.Join(config, "service", strings.TrimSuffix(unit, filepath.Ext(unit))))
	}
	var present []string
	seen := map[string]bool{}
	for _, path := range paths {
		if seen[path] {
			continue
		}
		seen[path] = true
		if _, err := os.Lstat(path); err == nil {
			present = append(present, path)
		}
	}
	return present
}

func reconcileRetiredHyakki(checkOnly bool) recResult {
	if sys.Exists(retiredHyakkiMarker()) {
		return okRes(i18n.T("Hyakki and Rashin were already moved aside"))
	}
	paths := retiredHyakkiPaths()
	var packages []string
	for _, pkg := range retiredHyakkiPackages {
		if retiredHyakkiInstalled(pkg) {
			packages = append(packages, pkg)
		}
	}
	var units []string
	for _, unit := range retiredHyakkiUnits {
		if retiredHyakkiService([]string{"--user", "is-active", unit}) == host.ExitOK ||
			retiredHyakkiService([]string{"--user", "is-enabled", unit}) == host.ExitOK {
			units = append(units, unit)
		}
	}
	if len(paths) == 0 && len(packages) == 0 && len(units) == 0 {
		if checkOnly {
			return okRes(i18n.T("nothing of Hyakki or Rashin is left"))
		}
		if err := writeRetiredHyakkiMarker(""); err != nil {
			return failRes(i18n.T("could not record that Hyakki and Rashin are gone: %v"), err)
		}
		return okRes(i18n.T("nothing of Hyakki or Rashin is left"))
	}
	if checkOnly {
		return wouldRes(i18n.T("Hyakki or Rashin is still here (%d services, %d packages, %d files)"),
			len(units), len(packages), len(paths)).
			withFix(i18n.T("ryoku doctor stops them, removes the packages and moves their data aside"))
	}

	for _, unit := range units {
		retiredHyakkiService([]string{"--user", "disable", "--now", unit})
	}
	for _, name := range retiredHyakkiProcesses {
		retiredHyakkiKill(name)
	}

	var failed []string
	for _, pkg := range packages {
		if err := retiredHyakkiRemove(pkg); err != nil {
			failed = append(failed, pkg)
		}
	}

	archive := ""
	if len(paths) > 0 {
		archive = filepath.Join(sys.Xdg("XDG_DATA_HOME", ".local/share"), "ryoku", "retired",
			"hyakki-rashin-"+retiredHyakkiNow().Format("20060102-150405"))
		if err := os.MkdirAll(archive, 0o700); err != nil {
			return failRes(i18n.T("could not make %s: %v"), archive, err)
		}
		home := sys.Home()
		for _, path := range paths {
			rel, err := filepath.Rel(home, path)
			if err != nil || strings.HasPrefix(rel, "..") {
				rel = strings.TrimPrefix(path, string(filepath.Separator))
			}
			if err := moveRetired(path, filepath.Join(archive, rel)); err != nil {
				failed = append(failed, path)
			}
		}
		if err := os.WriteFile(filepath.Join(archive, "README.txt"), []byte(retiredHyakkiReadme), 0o644); err != nil {
			failed = append(failed, filepath.Join(archive, "README.txt"))
		}
	}
	if len(units) > 0 {
		retiredHyakkiService([]string{"--user", "daemon-reload"})
	}
	if len(failed) > 0 {
		return failRes(i18n.T("could not move aside: %s"), strings.Join(failed, ", ")).
			withFix(i18n.T("run ryoku doctor again; nothing was deleted"))
	}
	if err := writeRetiredHyakkiMarker(archive); err != nil {
		return failRes(i18n.T("could not record that Hyakki and Rashin are gone: %v"), err)
	}
	if archive == "" {
		return fixedRes(i18n.T("stopped and removed Hyakki and Rashin"))
	}
	return fixedRes(i18n.T("stopped and removed Hyakki and Rashin; their data is kept in %s"), archive)
}

func writeRetiredHyakkiMarker(archive string) error {
	marker := retiredHyakkiMarker()
	if err := os.MkdirAll(filepath.Dir(marker), 0o755); err != nil {
		return err
	}
	return os.WriteFile(marker, []byte(archive+"\n"), 0o644)
}

// moveRetired renames when it can and copies across filesystems, removing the
// source only after the whole copy landed.
func moveRetired(source, destination string) error {
	if err := os.MkdirAll(filepath.Dir(destination), 0o700); err != nil {
		return err
	}
	err := os.Rename(source, destination)
	if err == nil || !errors.Is(err, syscall.EXDEV) {
		return err
	}
	if err := copyRetired(source, destination); err != nil {
		return err
	}
	return os.RemoveAll(source)
}

func copyRetired(source, destination string) error {
	return filepath.WalkDir(source, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		target := filepath.Join(destination, rel)
		info, err := os.Lstat(path)
		if err != nil {
			return err
		}
		switch {
		case info.IsDir():
			return os.MkdirAll(target, info.Mode().Perm())
		case info.Mode()&os.ModeSymlink != 0:
			link, err := os.Readlink(path)
			if err != nil {
				return err
			}
			return os.Symlink(link, target)
		case info.Mode().IsRegular():
			return copyRetiredFile(path, target, info.Mode().Perm())
		default:
			return nil
		}
	})
}

func copyRetiredFile(source, destination string, mode os.FileMode) error {
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(destination, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return fmt.Errorf("copy %s: %w", source, err)
	}
	return out.Close()
}

const retiredHyakkiReadme = `Hyakki, Oni and Rashin no longer ship with Ryoku.

This folder holds everything they left in your home when Ryoku removed them:
settings, sessions, memory, models, caches and any locally built copies. Each
item sits at the same path it had under your home directory, so moving it back
restores it. Nothing in here is read by Ryoku, and Ryoku will not touch it again.
`
