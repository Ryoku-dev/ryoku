package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// importToLibrary copies a picture or clip from anywhere into the library dir
// the picker scans, so a wallpaper set from a file manager is still there to
// pick later. A file already in the library is used where it is, an identical
// copy is reused, and an existing file is never overwritten.
func (d *daemon) importToLibrary(src string) (string, error) {
	src, err := filepath.Abs(src)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(src)
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("%s is not a file", src)
	}
	cfg := d.config()
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(src), "."))
	var dir string
	switch {
	case videoExts[ext]:
		dir = cfg.videoDir()
	case imageExts[ext]:
		dir = cfg.wallpaperDir()
	default:
		return "", fmt.Errorf("%s is not an image or video the library shows", filepath.Base(src))
	}
	for _, root := range []string{cfg.wallpaperDir(), cfg.videoDir()} {
		if within(root, src) {
			return src, nil
		}
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	stem := strings.TrimSuffix(filepath.Base(src), filepath.Ext(src))
	for n := 0; ; n++ {
		name := filepath.Base(src)
		if n > 0 {
			name = fmt.Sprintf("%s-%d%s", stem, n, filepath.Ext(src))
		}
		dest := filepath.Join(dir, name)
		same, err := sameContent(src, dest)
		switch {
		case same:
			return dest, nil
		case errors.Is(err, os.ErrNotExist):
			return dest, copyNew(src, dest)
		case err != nil:
			return "", err
		}
	}
}

func within(root, path string) bool {
	if root == "" {
		return false
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(abs, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func sameContent(a, b string) (bool, error) {
	bi, err := os.Stat(b)
	if err != nil {
		return false, err
	}
	ai, err := os.Stat(a)
	if err != nil {
		return false, err
	}
	if ai.Size() != bi.Size() {
		return false, nil
	}
	ab, err := os.ReadFile(a)
	if err != nil {
		return false, err
	}
	bb, err := os.ReadFile(b)
	if err != nil {
		return false, err
	}
	return bytes.Equal(ab, bb), nil
}

// copyNew writes through a hidden temp name the library walk ignores, then
// links it into place, so the watcher never catalogues a half-written file and
// a name taken in the meantime is refused rather than replaced.
func copyNew(src, dest string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	tmp, err := os.CreateTemp(filepath.Dir(dest), ".import-*.part")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if err := tmp.Chmod(0o644); err != nil {
		tmp.Close()
		return err
	}
	if _, err := io.Copy(tmp, in); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	err = os.Link(tmp.Name(), dest)
	if err == nil || errors.Is(err, os.ErrExist) {
		return err
	}
	// FAT, exFAT and some FUSE mounts have no hard links.
	if _, statErr := os.Lstat(dest); !errors.Is(statErr, os.ErrNotExist) {
		return fmt.Errorf("%s appeared while importing", dest)
	}
	return os.Rename(tmp.Name(), dest)
}
