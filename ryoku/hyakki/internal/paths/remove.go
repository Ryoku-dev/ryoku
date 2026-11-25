package paths

import (
	"io/fs"
	"os"
	"path/filepath"
)

// RemoveWritableTree removes a scratch tree whose tools may have made
// directories read-only (Go's module cache does this). It never follows links.
func RemoveWritableTree(root string) error {
	if err := os.RemoveAll(root); err == nil {
		return nil
	}
	if err := filepathWalkWritable(root); err != nil {
		return err
	}
	return os.RemoveAll(root)
}
func filepathWalkWritable(root string) error {
	return filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		return os.Chmod(path, info.Mode().Perm()|0o700)
	})
}
