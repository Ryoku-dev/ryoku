package httpdl

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

var multiExt = []string{".tar.gz", ".tar.xz", ".tar.bz2", ".tar.zst", ".tar.lz4"}

func splitExt(name string) (string, string) {
	lower := strings.ToLower(name)
	for _, e := range multiExt {
		if strings.HasSuffix(lower, e) && len(name) > len(e) {
			return name[:len(name)-len(e)], name[len(name)-len(e):]
		}
	}
	ext := filepath.Ext(name)
	if ext == name {
		return name, ""
	}
	return name[:len(name)-len(ext)], ext
}

// Numbered returns name for n <= 1, else "base (n).ext".
func Numbered(name string, n int) string {
	if n <= 1 {
		return name
	}
	base, ext := splitExt(name)
	return base + " (" + strconv.Itoa(n) + ")" + ext
}

func exists(p string) bool {
	_, err := os.Lstat(p)
	return err == nil
}

// Unique picks a name in dir that neither a file, an unfinished .part nor a
// caller-held name (taken) already uses.
func Unique(dir, name string, taken func(string) bool) string {
	for n := 1; ; n++ {
		c := Numbered(name, n)
		p := filepath.Join(dir, c)
		if exists(p) || exists(p+".part") || (taken != nil && taken(c)) {
			continue
		}
		return c
	}
}

// Publish moves src to dst without ever replacing an existing file: a hard link
// fails on a taken name, so a file that appeared meanwhile gets "name (2)".
func Publish(src, dst string) (string, error) {
	dir, name := filepath.Split(dst)
	for n := 1; n < 10000; n++ {
		cand := filepath.Join(dir, Numbered(name, n))
		err := os.Link(src, cand)
		if err == nil {
			_ = os.Remove(src)
			return cand, nil
		}
		if errors.Is(err, fs.ErrExist) {
			continue
		}
		// filesystems without hard links: check, then rename
		if exists(cand) {
			continue
		}
		if err := os.Rename(src, cand); err != nil {
			return "", err
		}
		return cand, nil
	}
	return "", fs.ErrExist
}
