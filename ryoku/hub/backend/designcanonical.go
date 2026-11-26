package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// Validate the complete existing data-only schema, canonicalize every asset
// (including unused assets), then rebuild references from typed values only.
func canonicalDesignPack(p designPack, materialize bool) (designPack, error) {
	checked := p
	checked.root = ""
	spec, err := designValidate(checked)
	if err != nil {
		return p, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), mediaDeadline)
	defer cancel()
	files := map[string][]byte{}
	names := map[string]string{}
	var pixels int64
	var total int64
	for _, name := range p.names {
		if name == "design.json" {
			continue
		}
		media, e := inspectMedia(ctx, p.files[name], false)
		if e != nil {
			return p, fmt.Errorf("%s: %w", name, e)
		}
		pixels += int64(media.Width) * int64(media.Height) * int64(media.Frames)
		if pixels > designPixelLimit {
			return p, fmt.Errorf("pack exceeds composited-pixel budget")
		}
		cleanName := name
		data := media.Data
		if media.Extension == "jpg" {
			// Still JPEG input becomes PNG, avoiding generation loss on every reopen.
			sum := sha256.Sum256([]byte(name))
			cleanName = "still-" + hex.EncodeToString(sum[:8]) + ".png"
			data = media.Poster
		}
		if _, exists := files[cleanName]; exists {
			return p, fmt.Errorf("canonical image filename collision")
		}
		files[cleanName] = data
		names[name] = cleanName
		total += int64(len(data))
	}
	spec.Badge.Emblem = names[spec.Badge.Emblem]
	if spec.Card.Background != "" {
		spec.Card.Background = names[spec.Card.Background]
	}
	if spec.Card.Frame != "" {
		spec.Card.Frame = names[spec.Card.Frame]
	}
	if spec.Card.Glow != "" {
		spec.Card.Glow = names[spec.Card.Glow]
	}
	for i := range spec.Card.Art {
		spec.Card.Art[i].Image = names[spec.Card.Art[i].Image]
	}
	normalized, err := json.Marshal(spec)
	if err != nil {
		return p, err
	}
	files["design.json"] = normalized
	total += int64(len(normalized))
	if total > designTotalLimit {
		return p, fmt.Errorf("canonical design exceeds 24 MB")
	}
	p.files = files
	p.bytes = total
	p.names = make([]string, 0, len(files))
	for name := range files {
		p.names = append(p.names, name)
	}
	// Stable ordering is also the cache digest's inventory ordering.
	sort.Strings(p.names)
	if _, err = designValidate(p); err != nil {
		return p, err
	}
	if !materialize {
		return p, nil
	}
	base, err := os.UserCacheDir()
	if err != nil {
		return p, err
	}
	base = filepath.Join(base, "ryoku", "card-media-v1")
	if err = os.MkdirAll(base, 0700); err != nil {
		return p, err
	}
	if st, e := os.Lstat(base); e != nil || !st.IsDir() || st.Mode()&os.ModeSymlink != 0 {
		return p, fmt.Errorf("unsafe canonical media cache")
	}
	hash := sha256.New()
	hash.Write([]byte("ryoku-local-design-canonical-v1\x00"))
	for _, name := range p.names {
		hash.Write([]byte(name))
		hash.Write([]byte{0})
		sum := sha256.Sum256(p.files[name])
		hash.Write(sum[:])
	}
	target := filepath.Join(base, hex.EncodeToString(hash.Sum(nil)))
	if st, e := os.Lstat(target); e == nil {
		if !st.IsDir() || st.Mode()&os.ModeSymlink != 0 {
			return p, fmt.Errorf("unsafe canonical design cache")
		}
		entries, e := os.ReadDir(target)
		if e != nil {
			return p, e
		}
		if len(entries) != len(p.names) {
			return p, fmt.Errorf("canonical design cache differs")
		}
		for _, name := range p.names {
			b, e := readBoundedFile(filepath.Join(target, name), int64(len(p.files[name])))
			if e != nil || !bytes.Equal(b, p.files[name]) {
				return p, fmt.Errorf("canonical design cache differs")
			}
		}
	} else if !os.IsNotExist(e) {
		return p, e
	} else {
		tmp, e := os.MkdirTemp(base, ".canonical-")
		if e != nil {
			return p, e
		}
		defer os.RemoveAll(tmp)
		for _, name := range p.names {
			if e = os.WriteFile(filepath.Join(tmp, name), p.files[name], 0600); e != nil {
				return p, e
			}
		}
		if e = os.Rename(tmp, target); e != nil {
			return p, e
		}
	}
	if err = pruneCanonicalDesigns(base, target); err != nil {
		return p, err
	}
	p.root = target
	return p, nil
}

// Cache eviction applies only to this backend's exact digest directories and
// data-only asset names; unknown entries are refused, not recursively removed.
func pruneCanonicalDesigns(base, keep string) error {
	entries, err := os.ReadDir(base)
	if err != nil {
		return err
	}
	type cached struct {
		path     string
		bytes    int64
		modified int64
	}
	cache := []cached{}
	var total int64
	for _, entry := range entries {
		if len(entry.Name()) != 64 {
			return fmt.Errorf("unexpected canonical cache entry")
		}
		if _, e := hex.DecodeString(entry.Name()); e != nil {
			return fmt.Errorf("unexpected canonical cache digest")
		}
		st, e := entry.Info()
		if e != nil {
			return e
		}
		if !st.IsDir() || st.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("unsafe canonical cache folder")
		}
		path := filepath.Join(base, entry.Name())
		files, e := os.ReadDir(path)
		if e != nil {
			return e
		}
		var size int64
		for _, file := range files {
			if !designNameOK(file.Name()) {
				return fmt.Errorf("unexpected canonical cache file")
			}
			info, e := file.Info()
			if e != nil {
				return e
			}
			if !info.Mode().IsRegular() {
				return fmt.Errorf("unsafe canonical cache file")
			}
			size += info.Size()
		}
		cache = append(cache, cached{path, size, st.ModTime().UnixNano()})
		total += size
	}
	sort.Slice(cache, func(i, j int) bool { return cache[i].modified < cache[j].modified })
	count := len(cache)
	for _, entry := range cache {
		if count <= 8 && total <= 64<<20 {
			break
		}
		if entry.path == keep {
			continue
		}
		if err = os.RemoveAll(entry.path); err != nil {
			return err
		}
		count--
		total -= entry.bytes
	}
	return nil
}
