package migration

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/unix"
	"ryoku-rashin/internal/paths"
)

// Restore reconstructs the exact legacy objects from a private, completed
// snapshot. Hyakki destinations are deliberately retained: restoring must never
// erase conversations, keys or model downloads created since the migration.
// A differing legacy object is a conflict, not permission to overwrite it.
func Restore(backupDir string) error {
	backup, err := filepath.Abs(backupDir)
	if err != nil {
		return err
	}
	m, err := readBackup(backup)
	if err != nil {
		return err
	}
	guard, err := serializeMigration()
	if err != nil {
		return err
	}
	defer guard.Close()
	locks, err := lockApplications(nil, true)
	if err != nil {
		return err
	}
	defer closeLocks(locks)
	if err := verifySnapshot(backup, m); err != nil {
		return err
	}
	if err := recoveryOwnership(m); err != nil {
		return err
	}
	// Preflight every original before restoring even the first one.
	for _, e := range m.Entries {
		if err := checkAncestors(e.Source); err != nil {
			return err
		}
		if _, err := os.Lstat(e.Source); errors.Is(err, fs.ErrNotExist) {
			continue
		} else if err != nil {
			return err
		}
		matches, err := originalMatches(e)
		if err != nil {
			return err
		}
		if !matches {
			return fmt.Errorf("restore would overwrite newer legacy content at %s; preserve or relocate it explicitly first; Hyakki data was not removed", e.Source)
		}
	}
	for i, e := range m.Entries {
		if _, err := os.Lstat(e.Source); err == nil {
			continue
		} else if !errors.Is(err, fs.ErrNotExist) {
			return err
		}
		if err := restoreOriginal(backup, i, e); err != nil {
			return err
		}
	}
	return nil
}

func readBackup(backup string) (*backupManifest, error) {
	base := filepath.Clean(paths.MigrationBackups())
	if !filepath.IsAbs(backup) || filepath.Clean(backup) != backup || filepath.Dir(backup) != base || !strings.HasPrefix(filepath.Base(backup), "rashin-to-hyakki-") {
		return nil, fmt.Errorf("restore requires an original private Rashin-to-Hyakki backup under %s", base)
	}
	if err := checkAncestors(filepath.Join(backup, "manifest.json")); err != nil {
		return nil, err
	}
	for _, dir := range []string{base, backup, filepath.Join(backup, "objects")} {
		if err := privateObject(dir, true); err != nil {
			return nil, err
		}
	}
	manifest := filepath.Join(backup, "manifest.json")
	if err := privateObject(manifest, false); err != nil {
		return nil, err
	}
	f, err := os.Open(manifest)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	decoder := json.NewDecoder(io.LimitReader(f, 1<<20))
	decoder.DisallowUnknownFields()
	var m backupManifest
	if err := decoder.Decode(&m); err != nil {
		return nil, fmt.Errorf("invalid migration backup manifest: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return nil, fmt.Errorf("unexpected trailing migration manifest data")
	}
	if m.Version != migrationVersion || m.UID != os.Getuid() || len(m.Entries) == 0 || len(m.Entries) > len(relocationCandidates()) {
		return nil, fmt.Errorf("migration backup provenance does not match this user/layout")
	}
	if m.Status != "prepared" && m.Status != "committed" && m.Status != "rolledback" {
		return nil, fmt.Errorf("invalid migration transaction status")
	}
	allowed := make(map[Move]bool)
	for _, move := range relocationCandidates() {
		allowed[move] = true
	}
	for _, e := range m.Entries {
		if !allowed[e.Move] || len(e.Digest) != 64 {
			return nil, fmt.Errorf("migration backup contains an unexpected or duplicate path")
		}
		delete(allowed, e.Move)
		if e.Phase != "" && e.Phase != "installing" && e.Phase != "installed" {
			return nil, fmt.Errorf("invalid migration object phase")
		}
		if m.Status == "committed" && e.Phase != "installed" {
			return nil, fmt.Errorf("incomplete committed migration manifest")
		}
		if e.Symlink != "" {
			if !e.KeepSource || e.Source != paths.LegacyRashinConfig() {
				return nil, fmt.Errorf("invalid configuration symlink provenance")
			}
		} else if e.KeepSource != retainedMemorySource(e.Source) {
			return nil, fmt.Errorf("invalid retained-source provenance")
		}
	}
	return &m, nil
}

func verifySnapshot(backup string, m *backupManifest) error {
	for i, e := range m.Entries {
		root := snapshotPath(backup, i)
		info, err := os.Lstat(root)
		if err != nil {
			return fmt.Errorf("incomplete migration snapshot: %w", err)
		}
		// Top-level symlinks were never snapshotted, except the config is
		// materialized as bytes. Reject a replacement before walking it.
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("unsafe migration snapshot object")
		}
		if err := inspectOwnedObject(root); err != nil {
			return err
		}
		digest, err := objectDigest(root)
		if err != nil {
			return err
		}
		if digest != e.Digest {
			return fmt.Errorf("migration snapshot integrity mismatch for %s; no original content was overwritten", e.Source)
		}
	}
	return nil
}

func originalMatches(e backupEntry) (bool, error) {
	if e.Symlink != "" {
		target, err := os.Readlink(e.Source)
		if errors.Is(err, fs.ErrNotExist) {
			return false, nil
		}
		if err != nil {
			return false, nil
		}
		return target == e.Symlink, nil
	}
	digest, err := objectDigest(e.Source)
	if err != nil {
		return false, err
	}
	return digest == e.Digest, nil
}

func restoreOriginal(backup string, index int, e backupEntry) error {
	if err := checkAncestors(e.Source); err != nil {
		return err
	}
	parent := filepath.Dir(e.Source)
	if err := mkdirDurable(parent, 0o700); err != nil {
		return err
	}
	stage, err := os.MkdirTemp(parent, ".hyakki-restore-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(stage)
	object := filepath.Join(stage, "object")
	if e.Symlink != "" {
		if err := os.Symlink(e.Symlink, object); err != nil {
			return err
		}
	} else if err := copyObject(snapshotPath(backup, index), object); err != nil {
		return err
	}
	// Unlike os.Rename, NOREPLACE cannot overwrite a concurrently-created
	// original even when both objects happen to be regular files.
	if err := unix.Renameat2(unix.AT_FDCWD, object, unix.AT_FDCWD, e.Source, unix.RENAME_NOREPLACE); err != nil {
		return fmt.Errorf("install restored original without overwriting %s: %w", e.Source, err)
	}
	return syncDirectory(parent)
}

func rollback(backup string, m *backupManifest) error {
	if err := verifySnapshot(backup, m); err != nil {
		return err
	}
	if err := recoveryOwnership(m); err != nil {
		return err
	}
	// Restore original bytes before retaining attempted destinations. Nothing
	// gets deleted: changed/partial originals also receive a recovery copy.
	for i, e := range m.Entries {
		if err := checkAncestors(e.Source); err != nil {
			return err
		}
		if _, err := os.Lstat(e.Source); err == nil {
			matches, err := originalMatches(e)
			if err != nil {
				return err
			}
			if matches {
				continue
			}
			if e.KeepSource {
				return fmt.Errorf("retained source changed during migration; preserve it explicitly before retrying: %s", e.Source)
			}
			if err := retainRecovery(backup, "original", i, e.Source); err != nil {
				return err
			}
		} else if !errors.Is(err, fs.ErrNotExist) {
			return err
		}
		if err := restoreOriginal(backup, i, e); err != nil {
			return err
		}
	}
	// Reverse order handles app.json nested under the state destination.
	for i := len(m.Entries) - 1; i >= 0; i-- {
		e := m.Entries[i]
		if e.Phase == "" {
			continue
		}
		if err := checkAncestors(e.Destination); err != nil {
			return err
		}
		if _, err := os.Lstat(e.Destination); errors.Is(err, fs.ErrNotExist) {
			continue
		} else if err != nil {
			return err
		}
		if err := retainRecovery(backup, "destination", i, e.Destination); err != nil {
			return err
		}
	}
	m.Status = "rolledback"
	if err := writeManifest(backup, m); err != nil {
		return err
	}
	if err := os.Remove(filepath.Join(paths.MigrationBackups(), "active.json")); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return syncDirectory(paths.MigrationBackups())
}

func recoveryOwnership(m *backupManifest) error {
	// PostgreSQL can outlive both app locks, including after an interrupted move
	for _, e := range m.Entries {
		for _, path := range []string{e.Source, e.Destination} {
			if err := checkAncestors(path); err != nil {
				return err
			}
			info, err := os.Lstat(path)
			if errors.Is(err, fs.ErrNotExist) {
				continue
			}
			if err != nil {
				return err
			}
			if info.Mode()&os.ModeSymlink != 0 {
				if path == e.Source && e.KeepSource {
					continue
				}
				return fmt.Errorf("recovery root became a symlink: %s; preserve it explicitly before retrying", path)
			}
			if info.IsDir() {
				if err := inspectOwnedObject(path); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func retainRecovery(backup, kind string, index int, source string) error {
	root := filepath.Join(backup, "recovered")
	if err := mkdirDurable(root, 0o700); err != nil {
		return err
	}
	dir, err := os.MkdirTemp(root, fmt.Sprintf("%s-%d-", kind, index))
	if err != nil {
		return err
	}
	if err := syncDirectory(dir); err != nil {
		return err
	}
	if err := syncDirectory(root); err != nil {
		return err
	}
	dest := filepath.Join(dir, "object")
	if err := os.Rename(source, dest); err == nil {
		if err := syncDirectory(dir); err != nil {
			return err
		}
		return syncDirectory(filepath.Dir(source))
	} else if !errors.Is(err, unix.EXDEV) {
		return err
	}
	// Cross-device recovery remains lossless. Only remove the original after
	// a complete, synced copy; an interrupted retry keeps both copies.
	if err := copyObject(source, dest); err != nil {
		return err
	}
	if err := syncDirectory(dir); err != nil {
		return err
	}
	if err := os.RemoveAll(source); err != nil {
		return err
	}
	return syncDirectory(filepath.Dir(source))
}
