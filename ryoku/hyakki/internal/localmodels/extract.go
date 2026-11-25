package localmodels

import (
	"archive/tar"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"syscall"

	i18n "ryoku-i18n"
)

const noFollow = syscall.O_NOFOLLOW

// Bounds on an archive Rashin unpacks: the pinned llama.cpp tarball is about
// 100 MB in 63 entries, so anything far past that is not it.
const (
	extractMaxBytes   = 2 << 30
	extractMaxEntries = 4096
)

// extractTarGz unpacks a gzipped tar into dest, which must not exist. A
// leading top folder is stripped. Absolute names, parent steps, special
// files and hard links are refused, and so is a symlink unless its target is
// relative and only goes down from its own folder: the tarball's
// libfoo.so.1 -> libfoo.so.1.0.0 links stay, nothing can point outside
// dest. Files are never written through a symlink.
func extractTarGz(archive, dest, top string) (err error) {
	f, err := os.Open(archive)
	if err != nil {
		return err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gz.Close()
	if err := os.Mkdir(dest, 0o755); err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = os.RemoveAll(dest)
		}
	}()
	tr := tar.NewReader(gz)
	var written int64
	for entries := 0; ; entries++ {
		h, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		if entries >= extractMaxEntries {
			return errors.New(i18n.T("The archive has more entries than llama.cpp ships."))
		}
		name, ok := entryName(h.Name, top)
		if !ok {
			return unsafeEntry(h.Name)
		}
		if name == "" {
			continue
		}
		target := filepath.Join(dest, filepath.FromSlash(name))
		switch h.Typeflag {
		case tar.TypeDir:
			if err := mkdirIn(dest, name); err != nil {
				return err
			}
		case tar.TypeReg:
			if h.Size < 0 || written+h.Size > extractMaxBytes {
				return errors.New(i18n.T("The archive is larger than llama.cpp ships."))
			}
			written += h.Size
			if err := mkdirIn(dest, path.Dir(name)); err != nil {
				return err
			}
			if err := writeEntry(target, tr, h.Size, os.FileMode(h.Mode)&0o755|0o600); err != nil {
				return err
			}
		case tar.TypeSymlink:
			if !downLink(h.Linkname) {
				return unsafeEntry(h.Name + " -> " + h.Linkname)
			}
			if err := mkdirIn(dest, path.Dir(name)); err != nil {
				return err
			}
			if err := os.Symlink(h.Linkname, target); err != nil {
				return err
			}
		default:
			return unsafeEntry(h.Name)
		}
	}
}

// entryName cleans an archive name and strips the top folder; "" is the top
// folder itself.
func entryName(name, top string) (string, bool) {
	name = strings.TrimPrefix(name, "./")
	if name == "" || strings.HasPrefix(name, "/") || strings.ContainsAny(name, "\\\x00") {
		return "", false
	}
	for _, part := range strings.Split(strings.TrimSuffix(name, "/"), "/") {
		if part == ".." {
			return "", false
		}
	}
	name = path.Clean(name)
	if top != "" {
		if name == top {
			return "", true
		}
		if !strings.HasPrefix(name, top+"/") {
			return "", false
		}
		name = strings.TrimPrefix(name, top+"/")
	}
	if name == "." {
		return "", true
	}
	return name, true
}

// downLink reports a symlink target that stays inside the link's own folder
// tree: relative, no parent step.
func downLink(target string) bool {
	if target == "" || strings.HasPrefix(target, "/") || strings.ContainsAny(target, "\\\x00") {
		return false
	}
	for _, part := range strings.Split(target, "/") {
		if part == ".." {
			return false
		}
	}
	return true
}

// mkdirIn makes dest/rel, refusing any step that is not a real folder.
func mkdirIn(dest, rel string) error {
	if rel == "." || rel == "" {
		return nil
	}
	dir := dest
	for _, part := range strings.Split(rel, "/") {
		dir = filepath.Join(dir, part)
		info, err := os.Lstat(dir)
		switch {
		case errors.Is(err, os.ErrNotExist):
			if err := os.Mkdir(dir, 0o755); err != nil {
				return err
			}
		case err != nil:
			return err
		case !info.IsDir():
			return unsafeEntry(rel)
		}
	}
	return nil
}

func writeEntry(target string, r io.Reader, size int64, mode os.FileMode) error {
	// O_NOFOLLOW: a symlink earlier in the archive cannot redirect the write
	out, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY|noFollow, mode)
	if err != nil {
		return err
	}
	n, err := io.Copy(out, io.LimitReader(r, size))
	if err == nil && n != size {
		err = io.ErrUnexpectedEOF
	}
	if cerr := out.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return err
	}
	return os.Chmod(target, mode)
}

func unsafeEntry(name string) error {
	return fmt.Errorf("%s: %s", i18n.T("The archive has an entry Hyakki does not unpack"), name)
}
