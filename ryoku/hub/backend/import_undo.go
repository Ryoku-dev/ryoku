package main

// Undo a config import: restore every file the import touched from its backup,
// or remove it when the backup is empty (the import created it). The manifest is
// the pure restore truth, so this reverts the marked blocks and the ingested
// desktop.json entries, then re-authors the compositor config from it. A failed
// restore or apply puts the imported files back and re-applies them, so the
// import stays whole and undoable.

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

func undoImport(ts string) (undoResult, error) {
	base := backupsDir()
	if ts == "" {
		latest, err := latestImport(base)
		if err != nil {
			return undoResult{}, err
		}
		ts = latest
	}
	dir := filepath.Join(base, ts)
	mb, err := os.ReadFile(filepath.Join(dir, "manifest.json"))
	if err != nil {
		return undoResult{}, fmt.Errorf("no import to undo for %q: %w", ts, err)
	}
	var man importManifest
	if err := json.Unmarshal(mb, &man); err != nil {
		return undoResult{}, fmt.Errorf("parse manifest: %w", err)
	}

	paths := make([]string, 0, len(man.Files))
	for _, f := range man.Files {
		paths = append(paths, f.Path)
	}
	imported, err := snapshotFiles(paths...)
	if err != nil {
		return undoResult{}, err
	}
	if err := restoreImported(man); err != nil {
		if rerr := imported.restore(); rerr != nil {
			return undoResult{}, errors.Join(err, rerr)
		}
		return undoResult{}, err
	}
	if err := regenerateFromStore(); err != nil {
		if rerr := imported.restore(); rerr != nil {
			return undoResult{}, errors.Join(err, rerr)
		}
		_ = regenerateFromStore()
		return undoResult{}, err
	}
	return undoResult{Ts: man.Ts, Restored: relPaths(paths)}, nil
}

// restoreImported writes every manifest file back from its backup, removing the
// ones the import created.
func restoreImported(man importManifest) error {
	for _, f := range man.Files {
		if f.Backup == "" {
			if err := removeIfPresent(f.Path); err != nil {
				return fmt.Errorf("remove %s: %w", f.Path, err)
			}
			continue
		}
		data, err := os.ReadFile(f.Backup)
		if err != nil {
			return fmt.Errorf("read backup %s: %w", f.Backup, err)
		}
		if err := atomicWrite(f.Path, data, 0o644); err != nil {
			return err
		}
	}
	return nil
}

// regenerateFromStore matches the emitted config to the store, as it is a pure
// function of it: re-author when there is a store, and clear what apply authored
// when there is not, rather than emitting defaults the user never had.
func regenerateFromStore() error {
	if _, err := os.Stat(desktopStorePath()); err == nil {
		return applyDesktop()
	}
	return clearGeneratedConfig()
}

// clearGeneratedConfig removes what the provider's apply authors. Used when a
// restore leaves no store: the emitted config is a pure function of the store,
// so an absent store means absent config, not defaults.
func clearGeneratedConfig() error {
	caps, err := freshCaps()
	if err != nil {
		return nil // no caps means there is nothing to clear
	}
	base := configHome()
	var errs []error
	for _, rel := range caps.GeneratedFiles {
		errs = append(errs, removeIfPresent(filepath.Join(base, rel)))
	}
	return errors.Join(errs...)
}

// latestImport returns the newest import timestamp that still has a manifest.
// The 20060102-150405 stamp sorts chronologically, so lexical max is newest.
func latestImport(base string) (string, error) {
	entries, err := os.ReadDir(base)
	if err != nil {
		return "", fmt.Errorf("no imports to undo")
	}
	var stamps []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if _, err := os.Stat(filepath.Join(base, e.Name(), "manifest.json")); err == nil {
			stamps = append(stamps, e.Name())
		}
	}
	if len(stamps) == 0 {
		return "", fmt.Errorf("no imports to undo")
	}
	sort.Strings(stamps)
	return stamps[len(stamps)-1], nil
}
