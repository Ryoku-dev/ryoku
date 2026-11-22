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

// sameContent compares in fixed chunks, so weighing a multi-gigabyte clip
// against its library copy never holds either file in memory.
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
		if na != nb || !bytes.Equal(ba[:na], bb[:nb]) {
			return false, nil
		}
		if errors.Is(erra, io.EOF) || errors.Is(erra, io.ErrUnexpectedEOF) {
			return errors.Is(errb, io.EOF) || errors.Is(errb, io.ErrUnexpectedEOF), nil
		}
		if erra != nil {
			return false, erra
		}
		if errb != nil {
			return false, errb
		}
	}
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
	// FAT, exFAT and some FUSE mounts have no hard links. Exclusive create
	// still refuses a taken name; the file is just visible while it fills.
	out, err := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return err
	}
	if err := copyInto(out, tmp.Name()); err != nil {
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
