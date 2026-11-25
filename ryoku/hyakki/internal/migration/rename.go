package migration

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"golang.org/x/sys/unix"
	"ryoku-rashin/internal/paths"
)

const migrationVersion = 1

// Ensure relocates only Rashin-owned content. It deliberately does nothing on a
// fresh install: even a status query must not create runtime or state directories.
func Ensure() error {
	// Fresh status requests do not even create the dedicated migration lock.
	pending, err := pendingBackup()
	if err != nil {
		return err
	}
	_, markerErr := os.Lstat(filepath.Join(paths.MigrationBackups(), "complete.json"))
	if markerErr == nil {
		_, err := loadPointer("complete.json")
		return err
	}
	if !errors.Is(markerErr, fs.ErrNotExist) {
		return markerErr
	}
	legacy := false
	for _, move := range relocationCandidates() {
		if _, err := os.Lstat(move.Source); err == nil {
			legacy = true
			break
		} else if !errors.Is(err, fs.ErrNotExist) {
			return err
		}
	}
	if pending == "" && !legacy {
		return nil
	}
	guard, err := serializeMigration()
	if err != nil {
		return err
	}
	defer guard.Close()
	base := paths.MigrationBackups()
	if err := checkAncestors(base); err != nil {
		return err
	}
	if _, err := os.Lstat(filepath.Join(base, "complete.json")); err == nil {
		_, err = loadPointer("complete.json")
		return err
	} else if !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	locks, err := lockApplications(nil, false)
	if err != nil {
		return err
	}
	defer func() { closeLocks(locks) }()

	pending, err = pendingBackup()
	if err != nil {
		return err
	}
	plan, err := relocationPlan()
	if err != nil && pending == "" {
		return err
	}
	if pending == "" && len(plan) == 0 {
		return nil
	}
	if pending != "" {
		locks, err = lockApplications(locks, true)
		if err != nil {
			return err
		}
		m, err := readBackup(pending)
		if err != nil {
			return err
		}
		if m.Status == "committed" {
			return finishMigration(pending)
		}
		if err := rollback(pending, m); err != nil {
			return fmt.Errorf("recover interrupted Hyakki migration: %w", err)
		}
		plan, err = relocationPlan()
		if err != nil {
			return err
		}
		if len(plan) == 0 {
			return nil
		}
	}
	backup, m, err := createBackup(plan)
	if err != nil {
		return fmt.Errorf("Hyakki migration backup failed; original content was not relocated: %w", err)
	}
	// Missing old locks are acquired after the full snapshot, then source
	// fingerprints catch a process that started while backup was in progress.
	locks, err = lockApplications(locks, true)
	if err != nil {
		_ = os.RemoveAll(backup)
		return err
	}
	for _, e := range m.Entries {
		matches, err := originalMatches(e)
		if err != nil {
			_ = os.RemoveAll(backup)
			return err
		}
		if !matches {
			_ = os.RemoveAll(backup)
			return fmt.Errorf("legacy content changed during backup; stop its writer and retry migration")
		}
	}
	if err := writePointer("active.json", backup); err != nil {
		return err
	}
	fail := func(cause error) error {
		if err := rollback(backup, m); err != nil {
			return fmt.Errorf("Hyakki migration failed (%v); rollback needs attention: %w; private backup: %s", cause, err, backup)
		}
		return fmt.Errorf("Hyakki migration rolled back: %w; private backup: %s", cause, backup)
	}
	moves := make([]Move, 0, len(m.Entries))
	for i := range m.Entries {
		e := &m.Entries[i]
		// Persist intent first. Recovery retains even partial/new destination
		// content rather than assuming it is safe to delete.
		if err := checkAncestors(e.Destination); err != nil {
			return fail(err)
		}
		if _, err := os.Lstat(e.Destination); !errors.Is(err, fs.ErrNotExist) {
			if err == nil {
				err = fmt.Errorf("destination appeared during migration: %s", e.Destination)
			}
			return fail(err)
		}
		if err := mkdirDurable(filepath.Dir(e.Destination), 0o700); err != nil {
			return fail(err)
		}
		e.Phase = "installing"
		if err := writeManifest(backup, m); err != nil {
			return fail(err)
		}
		// Copy from the completed snapshot, not from an external config
		// symlink target. Models use CoW clones where the filesystem supports it.
		if err := copyObject(snapshotPath(backup, i), e.Destination); err != nil {
			return fail(err)
		}
		if err := syncDirectory(filepath.Dir(e.Destination)); err != nil {
			return fail(err)
		}
		if !e.KeepSource {
			if err := os.RemoveAll(e.Source); err != nil {
				return fail(err)
			}
			if err := syncDirectory(filepath.Dir(e.Source)); err != nil {
				return fail(err)
			}
		}
		e.Phase = "installed"
		if err := writeManifest(backup, m); err != nil {
			return fail(err)
		}
		moves = append(moves, e.Move)
	}
	if err := rebaseSymlinks(moves); err != nil {
		return fail(err)
	}
	if err := RebaseReferences(moves); err != nil {
		return fail(err)
	}
	m.Status = "committed"
	if err := writeManifest(backup, m); err != nil {
		return fail(err)
	}
	return finishMigration(backup)
}

func relocationCandidates() []Move {
	moves := []Move{
		{Source: paths.LegacyRashinState(), Destination: paths.StateDir()},
		{Source: paths.LegacyRashinCache(), Destination: paths.CacheDir()},
		{Source: paths.LegacyRashinConfig(), Destination: paths.ConfigFile()},
		{Source: paths.LegacyRashinAppState(), Destination: paths.AppState()},
	}
	// The parent is also the shared vault. Never walk or move it wholesale.
	for _, name := range []string{"attachments", "skills", "models", "llama.cpp", "hindsight", "harness-homes", "decisions", "rules", "screen-home", "collections", "mcp-servers.json", "prompts.json", "voice.json", "words.txt"} {
		moves = append(moves, Move{Source: filepath.Join(paths.LegacyRashinData(), name), Destination: filepath.Join(paths.DataDir(), name)})
	}
	for _, vault := range []string{paths.VaultDir(), paths.LegacyVaultDir()} {
		moves = append(moves, Move{Source: filepath.Join(vault, "memory", "rashin"), Destination: filepath.Join(paths.DataDir(), "memory")})
	}
	return moves
}

func retainedMemorySource(source string) bool {
	return source == filepath.Join(paths.VaultDir(), "memory", "rashin") ||
		source == filepath.Join(paths.LegacyVaultDir(), "memory", "rashin")
}

func relocationPlan() ([]backupEntry, error) {
	var entries []backupEntry
	for _, move := range relocationCandidates() {
		info, err := os.Lstat(move.Source)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, err
		}
		if err := checkAncestors(move.Source); err != nil {
			return nil, err
		}
		if err := checkAncestors(move.Destination); err != nil {
			return nil, err
		}
		if overlaps(move.Source, paths.MigrationBackups()) || overlaps(move.Destination, paths.MigrationBackups()) || overlaps(move.Source, move.Destination) {
			return nil, fmt.Errorf("unsafe overlapping migration paths; correct XDG directory settings before retrying")
		}
		for _, vault := range []string{paths.LegacyRashinData(), paths.VaultDir()} {
			if (move.Source == paths.LegacyRashinState() || move.Source == paths.LegacyRashinCache()) && overlaps(move.Source, vault) {
				return nil, fmt.Errorf("legacy state/cache overlaps the shared Ryoku vault; separate XDG roots before migration")
			}
		}
		if _, err := os.Lstat(move.Destination); err == nil {
			return nil, fmt.Errorf("Hyakki destination already exists: %s; preserved it and legacy content at %s; reconcile these paths explicitly before migration", move.Destination, move.Source)
		} else if !errors.Is(err, fs.ErrNotExist) {
			return nil, err
		}
		e := backupEntry{Move: move}
		if info.Mode()&os.ModeSymlink != 0 {
			if move.Source != paths.LegacyRashinConfig() {
				return nil, fmt.Errorf("legacy app root is a symlink: %s; choose app-owned content explicitly instead of relocating an external location", move.Source)
			}
			e.Symlink, err = os.Readlink(move.Source)
			if err != nil {
				return nil, err
			}
			target, err := os.Stat(move.Source)
			if err != nil || !target.Mode().IsRegular() {
				return nil, fmt.Errorf("legacy configuration symlink must point to a readable regular file: %s", move.Source)
			}
			e.KeepSource = true
		} else if err := inspectOwnedObject(move.Source); err != nil {
			return nil, err
		}
		if retainedMemorySource(move.Source) {
			if !info.IsDir() {
				return nil, fmt.Errorf("legacy app memory must be a real directory: %s", move.Source)
			}
			e.KeepSource = true
		}
		entries = append(entries, e)
	}
	// App window state is a separate legacy file, but its new home is inside
	// the relocated state directory. Refuse a conflicting embedded app.json.
	for _, a := range entries {
		for _, b := range entries {
			if a.Source != b.Source && a.Destination == b.Destination {
				return nil, fmt.Errorf("multiple legacy sources target %s; reconcile them explicitly before migration", a.Destination)
			}
			if a.Source != b.Source && (overlaps(a.Source, b.Source) || overlaps(a.Source, b.Destination)) {
				return nil, fmt.Errorf("app-owned migration roots overlap; separate XDG roots before migration")
			}
			if a.Source == b.Source || !descendant(b.Destination, a.Destination) {
				continue
			}
			rel, _ := filepath.Rel(a.Destination, b.Destination)
			if _, err := os.Lstat(filepath.Join(a.Source, rel)); err == nil {
				return nil, fmt.Errorf("legacy state already contains %s; reconcile it with %s before migration", rel, b.Source)
			} else if !errors.Is(err, fs.ErrNotExist) {
				return nil, err
			}
		}
	}
	return entries, nil
}

func inspectOwnedObject(root string) error {
	return filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.Name() == ".git" {
			return fmt.Errorf("Git repository/worktree found in app content: %s; move or register it outside Rashin's app-owned roots before retrying", path)
		}
		if d.Name() == "postmaster.pid" {
			return fmt.Errorf("PostgreSQL ownership marker found at %s; stop Hindsight and its PostgreSQL child, then verify and remove a stale marker before migration", path)
		}
		if d.IsDir() {
			// Bare repositories have no .git marker.
			if regularAt(filepath.Join(path, "HEAD")) && directoryAt(filepath.Join(path, "objects")) && directoryAt(filepath.Join(path, "refs")) {
				return fmt.Errorf("bare Git repository found in app content: %s; relocate it explicitly before retrying", path)
			}
			return nil
		}
		if d.Type()&os.ModeSymlink != 0 {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("unsupported special file in app content: %s; stop its owner and remove the transient file before retrying", path)
		}
		return nil
	})
}

func regularAt(path string) bool   { i, err := os.Lstat(path); return err == nil && i.Mode().IsRegular() }
func directoryAt(path string) bool { i, err := os.Lstat(path); return err == nil && i.IsDir() }
func descendant(path, root string) bool {
	return strings.HasPrefix(path, root+string(filepath.Separator))
}
func overlaps(a, b string) bool { return a == b || descendant(a, b) || descendant(b, a) }

// No path component may redirect migration or backup writes through a symlink.
// A source's final component is treated separately for config symlinks.
func checkAncestors(path string) error {
	for dir := filepath.Dir(filepath.Clean(path)); ; dir = filepath.Dir(dir) {
		info, err := os.Lstat(dir)
		if err == nil && (!info.IsDir() || info.Mode()&os.ModeSymlink != 0) {
			return fmt.Errorf("unsafe migration path component: %s", dir)
		}
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			return err
		}
		if dir == filepath.Dir(dir) {
			return nil
		}
	}
}

func serializeMigration() (*os.File, error) {
	if err := checkAncestors(filepath.Join(paths.RuntimeDir(), "migration.lock")); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(paths.RuntimeDir(), 0o700); err != nil {
		return nil, err
	}
	fd, err := unix.Open(filepath.Join(paths.RuntimeDir(), "migration.lock"), unix.O_CREAT|unix.O_RDWR|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0o600)
	if err != nil {
		return nil, err
	}
	f := os.NewFile(uintptr(fd), "migration.lock")
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() {
		f.Close()
		return nil, fmt.Errorf("unsafe migration lock")
	}
	if err := syscall.Flock(fd, syscall.LOCK_EX); err != nil {
		f.Close()
		return nil, err
	}
	return f, nil
}

func rebaseSymlinks(moves []Move) error {
	for _, move := range moves {
		if err := filepath.WalkDir(move.Destination, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.Type()&os.ModeSymlink == 0 {
				return nil
			}
			target, err := os.Readlink(path)
			if err != nil {
				return err
			}
			rebased := RebasePath(target, moves)
			if !filepath.IsAbs(target) {
				rel, err := filepath.Rel(move.Destination, path)
				if err != nil {
					return err
				}
				original := filepath.Join(move.Source, rel)
				intended := RebasePath(filepath.Clean(filepath.Join(filepath.Dir(original), target)), moves)
				// Keep internal relative links literal. A link escaping its
				// moved tree must retain its original external target, not
				// accidentally acquire a different target at the new depth.
				if filepath.Clean(filepath.Join(filepath.Dir(path), target)) != intended {
					rebased = intended
				}
			}
			if rebased == target {
				return nil
			}
			temp, err := os.CreateTemp(filepath.Dir(path), ".hyakki-link-")
			if err != nil {
				return err
			}
			name := temp.Name()
			if err := temp.Close(); err != nil {
				os.Remove(name)
				return err
			}
			if err := os.Remove(name); err != nil {
				return err
			}
			if err := os.Symlink(rebased, name); err != nil {
				return err
			}
			if err := os.Rename(name, path); err != nil {
				os.Remove(name)
				return err
			}
			return syncDirectory(filepath.Dir(path))
		}); err != nil {
			return err
		}
	}
	return nil
}

func closeLocks(locks []*os.File) {
	for _, f := range locks {
		_ = f.Close()
	}
}

func applicationLockPaths() []string {
	return []string{
		filepath.Join(paths.LegacyRashinRuntime(), "app.lock"),
		filepath.Join(paths.LegacyRashinRuntime(), "rashind.lock"),
		filepath.Join(paths.LegacyRashinRuntime(), "config.lock"),
		paths.AppLock(), paths.DaemonLock(), paths.ConfigLock(),
	}
}

func lockApplications(held []*os.File, create bool) ([]*os.File, error) {
	locks := held
	var missing []string
	fail := func(err error) ([]*os.File, error) { closeLocks(locks); return nil, err }
	open := func(path string, create bool) (*os.File, error) {
		flags := unix.O_RDWR | unix.O_CLOEXEC | unix.O_NOFOLLOW | unix.O_NONBLOCK
		if create {
			flags |= unix.O_CREAT | unix.O_EXCL
		}
		fd, err := unix.Open(path, flags, 0o600)
		if err != nil {
			return nil, err
		}
		f := os.NewFile(uintptr(fd), path)
		info, err := f.Stat()
		if err != nil || !info.Mode().IsRegular() {
			f.Close()
			return nil, fmt.Errorf("unsafe application lock: %s", path)
		}
		if err := syscall.Flock(fd, syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
			f.Close()
			return nil, fmt.Errorf("Rashin or Hyakki is using its data; quit the old app and stop its daemon before migration/restore (%s)", filepath.Base(path))
		}
		return f, nil
	}
	// Detect all existing owners before creating anything.
	for _, path := range applicationLockPaths() {
		alreadyHeld := false
		for _, f := range locks {
			if f.Name() == path {
				alreadyHeld = true
				break
			}
		}
		if alreadyHeld {
			continue
		}
		if err := checkAncestors(path); err != nil {
			return fail(err)
		}
		f, err := open(path, false)
		if errors.Is(err, fs.ErrNotExist) {
			missing = append(missing, path)
			continue
		}
		if err != nil {
			return fail(err)
		}
		locks = append(locks, f)
	}
	if !create {
		return locks, nil
	}
	for _, path := range missing {
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			return fail(err)
		}
		f, err := open(path, true)
		if errors.Is(err, fs.ErrExist) {
			f, err = open(path, false)
		}
		if err != nil {
			return fail(err)
		}
		locks = append(locks, f)
	}
	return locks, nil
}

func finishMigration(backup string) error {
	if err := writePointer("complete.json", backup); err != nil {
		return err
	}
	if err := os.Remove(filepath.Join(paths.MigrationBackups(), "active.json")); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return syncDirectory(paths.MigrationBackups())
}
