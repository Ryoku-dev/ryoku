package main

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

const designZipLimit int64 = 8 << 20
const designTotalLimit int64 = 24 << 20
const designJSONLimit int64 = 64 << 10

var designFilePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,31}\.(png|webp|jpg|jpeg)$`)

type designPack struct {
	files map[string][]byte
	names []string
	bytes int64
	root  string
}

func designNameOK(name string) bool {
	return name == "design.json" || designFilePattern.MatchString(name)
}

func designReadZip(raw []byte, root string) (designPack, error) {
	p := designPack{files: make(map[string][]byte), names: []string{}, root: root}
	if int64(len(raw)) > designZipLimit {
		return p, fmt.Errorf("zip exceeds 8 MB")
	}
	zr, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		return p, err
	}
	if len(zr.File) > 24 {
		return p, fmt.Errorf("pack exceeds 24 files")
	}
	seen := map[string]bool{}
	for _, f := range zr.File {
		lower := strings.ToLower(f.Name)
		if seen[lower] {
			return p, fmt.Errorf("duplicate filename %q", f.Name)
		}
		seen[lower] = true
		if !designNameOK(f.Name) || !f.Mode().IsRegular() {
			return p, fmt.Errorf("invalid pack entry %q", f.Name)
		}
		if f.Method != zip.Store && f.Method != zip.Deflate {
			return p, fmt.Errorf("unsupported zip method")
		}
		n := f.UncompressedSize64
		if n > uint64(designTotalLimit) || f.CompressedSize64 > uint64(len(raw)) || (n > 0 && (f.CompressedSize64 == 0 || n > f.CompressedSize64*50)) {
			return p, fmt.Errorf("entry exceeds size or compression ratio limit")
		}
		if f.Name == "design.json" && n > uint64(designJSONLimit) {
			return p, fmt.Errorf("design.json exceeds 64 KB")
		}
		if p.bytes+int64(n) > designTotalLimit {
			return p, fmt.Errorf("pack exceeds 24 MB")
		}
		r, e := f.Open()
		if e != nil {
			return p, e
		}
		b, e := readBounded(r, int64(n))
		closeErr := r.Close()
		if e != nil {
			return p, e
		}
		if closeErr != nil {
			return p, closeErr
		}
		if uint64(len(b)) != n {
			return p, fmt.Errorf("zip size mismatch")
		}
		p.files[f.Name] = b
		p.names = append(p.names, f.Name)
		p.bytes += int64(n)
	}
	sort.Strings(p.names)
	return p, nil
}

func designReadPack(path string) (designPack, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return designPack{}, err
	}
	st, err := os.Lstat(abs)
	if err != nil {
		return designPack{}, err
	}
	if !st.IsDir() {
		if !st.Mode().IsRegular() {
			return designPack{}, fmt.Errorf("pack must be a directory or regular zip")
		}
		raw, e := readBoundedFile(abs, designZipLimit)
		if e != nil {
			return designPack{}, e
		}
		return designReadZip(raw, "")
	}
	p := designPack{files: make(map[string][]byte), names: []string{}, root: abs}
	entries, err := os.ReadDir(abs)
	if err != nil {
		return p, err
	}
	if len(entries) > 24 {
		return p, fmt.Errorf("pack exceeds 24 files")
	}
	seen := map[string]bool{}
	for _, entry := range entries {
		name := entry.Name()
		lower := strings.ToLower(name)
		if seen[lower] {
			return p, fmt.Errorf("duplicate filename")
		}
		seen[lower] = true
		if !designNameOK(name) {
			return p, fmt.Errorf("invalid filename %q", name)
		}
		info, e := entry.Info()
		if e != nil {
			return p, e
		}
		if !info.Mode().IsRegular() {
			return p, fmt.Errorf("entry is not a regular file")
		}
		limit := designTotalLimit - p.bytes
		if name == "design.json" && limit > designJSONLimit {
			limit = designJSONLimit
		}
		// Open without following symlinks, including a replacement between ReadDir and Open.
		f, e := os.OpenFile(filepath.Join(abs, name), os.O_RDONLY|designNoFollow, 0)
		if e != nil {
			return p, e
		}
		actual, e := f.Stat()
		if e != nil {
			f.Close()
			return p, e
		}
		if !actual.Mode().IsRegular() || actual.Size() > limit {
			f.Close()
			return p, fmt.Errorf("invalid entry size or kind")
		}
		b, e := readBounded(f, actual.Size())
		f.Close()
		if e != nil {
			return p, e
		}
		if int64(len(b)) != actual.Size() {
			return p, fmt.Errorf("file size changed")
		}
		p.files[name] = b
		p.names = append(p.names, name)
		p.bytes += int64(len(b))
	}
	sort.Strings(p.names)
	return p, nil
}

func designZip(p designPack) ([]byte, error) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, name := range p.names {
		// Store avoids producing packs which fail the same ratio limit on highly compressible art.
		h := &zip.FileHeader{Name: name, Method: zip.Store}
		h.SetMode(0600)
		h.SetModTime(time.Date(1980, 1, 1, 0, 0, 0, 0, time.UTC))
		w, err := zw.CreateHeader(h)
		if err != nil {
			return nil, err
		}
		if _, err = io.Copy(w, bytes.NewReader(p.files[name])); err != nil {
			return nil, err
		}
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	if int64(buf.Len()) > designZipLimit {
		return nil, fmt.Errorf("packed zip exceeds 8 MB")
	}
	return buf.Bytes(), nil
}
