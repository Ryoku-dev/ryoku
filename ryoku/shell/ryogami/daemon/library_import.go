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
	if scannerLists(cfg, src, videoExts[ext]) {
		return src, nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	// The scanner skips dotfiles, so a hidden source gets a visible name.
	base := strings.TrimLeft(filepath.Base(src), ".")
	stem := strings.TrimSuffix(base, filepath.Ext(base))
	if stem == "" {
		stem = "wallpaper"
	}
	for n := 0; ; n++ {
		name := stem + filepath.Ext(base)
		if n > 0 {
			name = fmt.Sprintf("%s-%d%s", stem, n, filepath.Ext(base))
		}
		dest := filepath.Join(dir, name)
		same, err := sameContent(src, dest)
		switch {
		case same:
			return dest, nil
		case errors.Is(err, os.ErrNotExist):
			err = copyNew(src, dest)
			if errors.Is(err, os.ErrExist) {
				// Another import claimed the name first: weigh it like any
				// existing file, reusing it if identical, else the next name.
				n--
				continue
			}
			if err != nil {
				return "", err
			}
			return dest, nil
		case err != nil:
			return "", err
		}
	}
}

var linkFile = os.Link

const compareChunk = 64 << 10

// scannerLists mirrors collectMedia's filters: a still is listed from the
// wallpaper dir, a clip from either dir, never from a hidden path or the cache,
// and only as a regular file, not a symlink.
func scannerLists(cfg config, path string, video bool) bool {
	if info, err := os.Lstat(path); err != nil || !info.Mode().IsRegular() {
		return false
	}
	if within(cfg.cacheDir(), path) != "" {
		return false
	}
	roots := []string{cfg.wallpaperDir()}
	if video {
		roots = append(roots, cfg.videoDir())
	}
	for _, root := range roots {
		rel := within(root, path)
		if rel != "" && walkReaches(root, rel) {
			return true
		}
	}
	return false
}

// walkReaches reports whether WalkDir from root descends to rel: no hidden
// part on the way, and no folder on the way that is a symlink it won't follow.
func walkReaches(root, rel string) bool {
	parts := strings.Split(rel, string(filepath.Separator))
	dir := root
	for i, part := range parts {
		if strings.HasPrefix(part, ".") {
			return false
		}
		if i == len(parts)-1 {
			break
		}
		dir = filepath.Join(dir, part)
		if info, err := os.Lstat(dir); err != nil || !info.IsDir() {
			return false
		}
	}
	return true
}

// within answers path relative to root, or "" when path lies outside it.
func within(root, path string) string {
	if root == "" {
		return ""
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return ""
	}
	rel, err := filepath.Rel(abs, path)
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return ""
	}
	return rel
}

// sameContent compares in fixed chunks, so weighing a multi-gigabyte clip
// against its library copy never holds either file in memory.
func sameContent(a, b string) (bool, error) {
	bi, err := os.Lstat(b)
	if err != nil {
		return false, err
	}
	if !bi.Mode().IsRegular() {
		// A symlink or folder holding the name is never listed, so never reused.
		return false, nil
	}
	ai, err := os.Stat(a)
	if err != nil {
		return false, err
	}
	if ai.Size() != bi.Size() {
		return false, nil
	}
	fa, err := os.Open(a)
	if err != nil {
		return false, err
	}
	defer fa.Close()
	fb, err := os.Open(b)
	if err != nil {
		return false, err
	}
	defer fb.Close()
	ba, bb := make([]byte, compareChunk), make([]byte, compareChunk)
	for {
		na, erra := io.ReadFull(fa, ba)
		nb, errb := io.ReadFull(fb, bb)
		endA, endB := atEnd(erra), atEnd(errb)
		if erra != nil && !endA {
			return false, erra
		}
		if errb != nil && !endB {
			return false, errb
		}
		if na != nb || !bytes.Equal(ba[:na], bb[:nb]) {
			return false, nil
		}
		if endA || endB {
			return endA && endB, nil
		}
	}
}

func atEnd(err error) bool {
	return errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF)
}

// copyNew writes through a hidden temp name the library walk ignores, then
// links it into place, so the watcher never catalogues a half-written file. A
// name taken in the meantime fails with os.ErrExist and is never replaced.
func copyNew(src, dest string) error {
	tmp, err := os.CreateTemp(filepath.Dir(dest), ".import-*.part")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if err := tmp.Chmod(0o644); err != nil {
		tmp.Close()
		return err
	}
	if err := copyInto(tmp, src); err != nil {
		return err
	}
	err = linkFile(tmp.Name(), dest)
	if err == nil || errors.Is(err, os.ErrExist) {
		return err
	}
	// FAT, exFAT and some FUSE mounts have no hard links. Claim the name with
	// an exclusive empty placeholder, then rename the finished copy over our
	// own placeholder: a taken name is still refused, and the picture is only
	// ever visible complete.
	claim, err := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return err
	}
	claim.Close()
	if err := os.Rename(tmp.Name(), dest); err != nil {
		os.Remove(dest)
		return err
	}
	return nil
}

// copyInto copies src into w and closes w.
func copyInto(w *os.File, src string) error {
	in, err := os.Open(src)
	if err != nil {
		w.Close()
		return err
	}
	defer in.Close()
	if _, err := io.Copy(w, in); err != nil {
		w.Close()
		return err
	}
	return w.Close()
}
