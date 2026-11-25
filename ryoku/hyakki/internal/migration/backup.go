package migration

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"syscall"

	"golang.org/x/sys/unix"
	"ryoku-rashin/internal/paths"
)

type backupEntry struct {
	Move
	Digest     string `json:"digest"`
	Symlink    string `json:"symlink,omitempty"`
	KeepSource bool   `json:"keep_source,omitempty"`
	Phase      string `json:"phase,omitempty"`
}

type backupManifest struct {
	Version int           `json:"version"`
	UID     int           `json:"uid"`
	Status  string        `json:"status"`
	Entries []backupEntry `json:"entries"`
}

type backupPointer struct {
	Version   int    `json:"version"`
	Directory string `json:"directory"`
}

func snapshotPath(backup string, index int) string {
	return filepath.Join(backup, "objects", strconv.Itoa(index))
}

func createBackup(entries []backupEntry) (string, *backupManifest, error) {
	base := paths.MigrationBackups()
	if err := checkAncestors(filepath.Join(base, "sentinel")); err != nil {
		return "", nil, err
	}
	if err := mkdirDurable(base, 0o700); err != nil {
		return "", nil, err
	}
	if err := privateObject(base, true); err != nil {
		return "", nil, err
	}
	backup, err := os.MkdirTemp(base, "rashin-to-hyakki-")
	if err != nil {
		return "", nil, err
	}
	ok := false
	defer func() {
		if !ok {
			_ = os.RemoveAll(backup)
		}
	}()
	if err := os.Mkdir(filepath.Join(backup, "objects"), 0o700); err != nil {
		return "", nil, err
	}
	m := &backupManifest{Version: migrationVersion, UID: os.Getuid(), Status: "prepared", Entries: entries}
	for i := range m.Entries {
		e := &m.Entries[i]
		dest := snapshotPath(backup, i)
		if e.Symlink != "" {
			// keep the dotfiles target in place and snapshot only its config bytes
			f, err := os.Open(e.Source)
			if err != nil {
				return "", nil, err
			}
			info, err := f.Stat()
			if err == nil && info.Mode().IsRegular() {
				err = copyRegular(f, info, dest)
			} else if err == nil {
				err = fmt.Errorf("configuration target changed type during backup")
			}
			_ = f.Close()
			if err != nil {
				return "", nil, err
			}
		} else if err := copyObject(e.Source, dest); err != nil {
			return "", nil, fmt.Errorf("snapshot %s: %w", e.Source, err)
		}
		digest, err := objectDigest(dest)
		if err != nil {
			return "", nil, err
		}
		e.Digest = digest
	}
	if err := syncDirectory(filepath.Join(backup, "objects")); err != nil {
		return "", nil, err
	}
	// A manifest is written only after every complete object has been synced.
	if err := writeManifest(backup, m); err != nil {
		return "", nil, err
	}
	if err := syncDirectory(base); err != nil {
		return "", nil, err
	}
	ok = true
	return backup, m, nil
}

func copyObject(source, dest string) error {
	info, err := os.Lstat(source)
	if err != nil {
		return err
	}
	switch {
	case info.Mode()&os.ModeSymlink != 0:
		target, err := os.Readlink(source)
		if err != nil {
			return err
		}
		return os.Symlink(target, dest)
	case info.IsDir():
		if err := os.Mkdir(dest, 0o700); err != nil {
			return err
		}
		children, err := os.ReadDir(source)
		if err != nil {
			return err
		}
		for _, child := range children {
			if err := copyObject(filepath.Join(source, child.Name()), filepath.Join(dest, child.Name())); err != nil {
				return err
			}
		}
		if err := os.Chmod(dest, info.Mode()&(os.ModePerm|os.ModeSetuid|os.ModeSetgid|os.ModeSticky)); err != nil {
			return err
		}
		if err := os.Chtimes(dest, info.ModTime(), info.ModTime()); err != nil {
			return err
		}
		return syncDirectory(dest)
	case info.Mode().IsRegular():
		fd, err := unix.Open(source, unix.O_RDONLY|unix.O_CLOEXEC|unix.O_NOFOLLOW|unix.O_NONBLOCK, 0)
		if err != nil {
			return err
		}
		f := os.NewFile(uintptr(fd), source)
		defer f.Close()
		actual, err := f.Stat()
		if err != nil {
			return err
		}
		if !actual.Mode().IsRegular() {
			return fmt.Errorf("file changed type during snapshot: %s", source)
		}
		return copyRegular(f, actual, dest)
	default:
		return fmt.Errorf("cannot snapshot special file: %s", source)
	}
}

func copyRegular(source *os.File, info fs.FileInfo, dest string) error {
	f, err := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	// FICLONE preserves big artifacts without allocating another full copy on
	// CoW filesystems. Real IO/disk errors are not disguised as unsupported CoW.
	err = unix.IoctlFileClone(int(f.Fd()), int(source.Fd()))
	if err != nil {
		if !errors.Is(err, unix.EOPNOTSUPP) && !errors.Is(err, unix.ENOTTY) && !errors.Is(err, unix.EXDEV) && !errors.Is(err, unix.EINVAL) && !errors.Is(err, unix.ENOSYS) {
			return fmt.Errorf("clone snapshot (check free space and filesystem health): %w", err)
		}
		if _, err := io.Copy(f, source); err != nil {
			return fmt.Errorf("copy snapshot (check free space and filesystem health): %w", err)
		}
	}
	if err := f.Chmod(info.Mode() & (os.ModePerm | os.ModeSetuid | os.ModeSetgid | os.ModeSticky)); err != nil {
		return err
	}
	if err := os.Chtimes(dest, info.ModTime(), info.ModTime()); err != nil {
		return err
	}
	return f.Sync()
}

func mkdirDurable(path string, mode fs.FileMode) error {
	var missing []string
	for dir := filepath.Clean(path); ; dir = filepath.Dir(dir) {
		info, err := os.Lstat(dir)
		if err == nil {
			if !info.IsDir() {
				return fmt.Errorf("migration directory is not a real directory: %s", dir)
			}
			break
		}
		if !errors.Is(err, fs.ErrNotExist) {
			return err
		}
		missing = append(missing, dir)
		if dir == filepath.Dir(dir) {
			return err
		}
	}
	if err := os.MkdirAll(path, mode); err != nil {
		return err
	}
	for _, dir := range missing {
		if err := syncDirectory(dir); err != nil {
			return err
		}
	}
	if len(missing) != 0 {
		return syncDirectory(filepath.Dir(missing[len(missing)-1]))
	}
	return nil
}

func objectDigest(root string) (string, error) {
	h := sha256.New()
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		// JSON framing avoids ambiguous names, symlink targets or boundaries.
		size := info.Size()
		if info.IsDir() {
			size = 0
		} // Directory allocation is filesystem-specific.
		if err := json.NewEncoder(h).Encode([]any{rel, uint32(info.Mode()), size}); err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			target, err := os.Readlink(path)
			if err != nil {
				return err
			}
			return json.NewEncoder(h).Encode(target)
		}
		if info.IsDir() {
			return nil
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("special file in snapshot: %s", path)
		}
		fd, err := unix.Open(path, unix.O_RDONLY|unix.O_CLOEXEC|unix.O_NOFOLLOW|unix.O_NONBLOCK, 0)
		if err != nil {
			return err
		}
		f := os.NewFile(uintptr(fd), path)
		_, err = io.Copy(h, f)
		closeErr := f.Close()
		if err != nil {
			return err
		}
		return closeErr
	})
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func privateObject(path string, directory bool) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || int(stat.Uid) != os.Getuid() || info.Mode().Perm()&0o077 != 0 || info.Mode()&os.ModeSymlink != 0 || (directory && !info.IsDir()) || (!directory && !info.Mode().IsRegular()) {
		return fmt.Errorf("migration backup must be private, owned by this user and not symlinked: %s", path)
	}
	return nil
}

func writeManifest(backup string, m *backupManifest) error {
	return atomicJSON(filepath.Join(backup, "manifest.json"), m)
}

func atomicJSON(path string, value any) error {
	if err := checkAncestors(path); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".migration-")
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	if err := json.NewEncoder(f).Encode(value); err != nil {
		f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		return err
	}
	return syncDirectory(filepath.Dir(path))
}

func syncDirectory(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return f.Sync()
}

func writePointer(name, backup string) error {
	return atomicJSON(filepath.Join(paths.MigrationBackups(), name), backupPointer{Version: migrationVersion, Directory: filepath.Base(backup)})
}

func loadPointer(name string) (string, error) {
	base := paths.MigrationBackups()
	if err := checkAncestors(filepath.Join(base, name)); err != nil {
		return "", err
	}
	if err := privateObject(base, true); err != nil {
		return "", err
	}
	path := filepath.Join(base, name)
	if err := privateObject(path, false); err != nil {
		return "", err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	var p backupPointer
	if err := json.Unmarshal(data, &p); err != nil {
		return "", fmt.Errorf("invalid migration pointer: %w", err)
	}
	if p.Version != migrationVersion || filepath.Base(p.Directory) != p.Directory || p.Directory == "." || p.Directory == "" {
		return "", fmt.Errorf("invalid migration backup pointer")
	}
	backup := filepath.Join(base, p.Directory)
	m, err := readBackup(backup)
	if err != nil {
		return "", err
	}
	if name == "complete.json" && m.Status != "committed" {
		return "", fmt.Errorf("success marker refers to an incomplete migration backup")
	}
	return backup, nil
}

func pendingBackup() (string, error) {
	_, err := os.Lstat(filepath.Join(paths.MigrationBackups(), "active.json"))
	if errors.Is(err, fs.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return loadPointer("active.json")
}
