package doctor

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"ryoku-cli/internal/sys"

	i18n "ryoku-i18n"
)

// Quickshell's QML disk cache keeps a compiled unit for every QML path it ever
// loaded and never drops one, so deploys, item runtimes and retired shells grow
// it without bound. Pruning is safe while the shell runs: a loaded unit stays
// mapped, and Qt recompiles any entry it finds missing.

const qmlCacheMaxIdle = 30 * 24 * time.Hour

func qmlCacheDir() string {
	return filepath.Join(sys.Xdg("XDG_CACHE_HOME", ".cache"), "quickshell", "qmlcache")
}

type qmlCacheEntry struct {
	path string
	size uint64
}

type qmlCacheScan struct {
	files     int
	size      uint64
	stale     []qmlCacheEntry
	staleSize uint64
}

// scanQMLCache sizes the cache and lists the files last used before cutoff.
// Entries that vanish or cannot be read mid-walk are skipped, since the shell
// may be writing the cache at the same time.
func scanQMLCache(dir string, cutoff time.Time) (qmlCacheScan, error) {
	var s qmlCacheScan
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			if p == dir {
				return err
			}
			return nil
		}
		if !d.Type().IsRegular() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		n := uint64(info.Size())
		s.files++
		s.size += n
		if qmlCacheLastUse(info).Before(cutoff) {
			s.stale = append(s.stale, qmlCacheEntry{p, n})
			s.staleSize += n
		}
		return nil
	})
	return s, err
}

// qmlCacheLastUse is the later of atime and mtime: on a noatime mount atime only
// records creation, and a rewritten unit is in use even if never read since.
func qmlCacheLastUse(info fs.FileInfo) time.Time {
	last := info.ModTime()
	if st, ok := info.Sys().(*syscall.Stat_t); ok {
		if at := time.Unix(st.Atim.Unix()); at.After(last) {
			last = at
		}
	}
	return last
}

func reconcileQMLCache(checkOnly bool) recResult {
	dir := qmlCacheDir()
	scan, err := scanQMLCache(dir, time.Now().Add(-qmlCacheMaxIdle))
	if errors.Is(err, fs.ErrNotExist) {
		return okRes(i18n.T("no QML cache yet"))
	}
	if err != nil {
		return warnRes(i18n.T("could not read %s: %v"), dir, err)
	}
	if len(scan.stale) == 0 {
		return okRes(i18n.T("QML cache is %s in %d files, none unused for 30 days"), humanSize(scan.size), scan.files)
	}
	if checkOnly {
		return wouldRes(i18n.T("QML cache is %s; %d files (%s) unused for 30 days"),
			humanSize(scan.size), len(scan.stale), humanSize(scan.staleSize)).
			withFix(i18n.T("ryoku doctor removes them; Qt recompiles any it needs"))
	}
	removed := 0
	var freed uint64
	for _, e := range scan.stale {
		err := os.Remove(e.path)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return failRes(i18n.T("removed %d QML cache files, then could not remove %s: %v"), removed, e.path, err)
		}
		removed++
		freed += e.size
	}
	return fixedRes(i18n.T("removed %d QML cache files (%s) unused for 30 days; %s remain"),
		removed, humanSize(freed), humanSize(scan.size-freed))
}
