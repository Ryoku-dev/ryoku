package chats

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// Count counts imported Markdown chats, without reading their contents or
// following symlinks. The caller serializes this with Import and ForgetAll.
func Count(ctx context.Context, root string) (int, error) {
	n := 0
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			if path == root && errors.Is(err, fs.ErrNotExist) {
				return nil
			}
			return err
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if entry.Type().IsRegular() && strings.HasSuffix(entry.Name(), ".md") {
			n++
		}
		return nil
	})
	return n, err
}

// ForgetAll removes only Hyakki's import root, through its parent's rooted
// filesystem. A symlink at root is removed, never followed. Export files the
// user originally chose, and any vault, are outside this root and untouched.
// The caller serializes this with Import and removes the associated index.
func ForgetAll(root string) error {
	parent, err := os.OpenRoot(filepath.Dir(root))
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	defer parent.Close()
	return parent.RemoveAll(filepath.Base(root))
}
