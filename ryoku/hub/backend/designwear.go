package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// A local design is private creative work, not a badge or online entitlement.
// Wear reads only canonical assets and never makes the seal verified. An
// existing approved design retains its authority and takes precedence.

func designWornDir() string { return filepath.Join(profileHeroDir(), "design") }

// designWearable holds a worn pack to a submitted pack's limits: what fits a pack's 8 MB zip and
// passes its checks
func designWearable(p designPack) (designSpec, error) {
	if p.bytes > designZipLimit {
		return designSpec{}, fmt.Errorf("design exceeds 8 MB")
	}
	return designValidate(p)
}

// designWorn reads the privately worn design back through every check, or nil when there is none
func designWorn() (*badgeDesign, error) {
	dir := designWornDir()
	st, e := os.Lstat(dir)
	if os.IsNotExist(e) {
		return nil, nil
	}
	if e != nil {
		return nil, e
	}
	if !st.IsDir() {
		return nil, errors.New("the worn design is not a folder")
	}
	p, e := designReadPack(dir)
	if e != nil {
		return nil, e
	}
	p, e = canonicalDesignPack(p, true)
	if e != nil {
		return nil, e
	}
	spec, e := designWearable(p)
	if e != nil {
		return nil, e
	}
	return &badgeDesign{Dir: p.root, Name: spec.Name, Worn: true, Spec: spec}, nil
}

// designWear keeps a canonical data-only pack locally. No online entitlement
// is required; publication and badge authority remain separate server checks.
func designWear(path string) error {
	b, _, published, list := ownBadge(false)
	if _, approved := teamApproval(list, b.Login); published && approved {
		return errors.New("your approved design is already on your card")
	}
	p, e := designReadPack(path)
	if e != nil {
		return e
	}
	p, e = canonicalDesignPack(p, false)
	if e != nil {
		return e
	}
	if _, e = designWearable(p); e != nil {
		return e
	}
	// the same pack Save for Mika would make, so what is worn can also be submitted
	if _, e = designZip(p); e != nil {
		return e
	}
	root := profileHeroDir()
	if e = os.MkdirAll(root, 0o755); e != nil {
		return e
	}
	if st, e := os.Lstat(root); e != nil || !st.IsDir() {
		return errors.New("unsafe profile folder")
	}
	temp, e := os.MkdirTemp(root, ".design-")
	if e != nil {
		return e
	}
	defer os.RemoveAll(temp)
	for _, name := range p.names {
		if e = os.WriteFile(filepath.Join(temp, name), p.files[name], 0o600); e != nil {
			return e
		}
	}
	if e = designUnwear(); e != nil {
		return e
	}
	return os.Rename(temp, designWornDir())
}

// designUnwear stops wearing the private design; a link in its place is removed, never followed
func designUnwear() error {
	dir := designWornDir()
	if _, e := os.Lstat(dir); os.IsNotExist(e) {
		return nil
	} else if e != nil {
		return e
	}
	stale, e := os.MkdirTemp(filepath.Dir(dir), ".design-old-")
	if e != nil {
		return e
	}
	defer os.RemoveAll(stale)
	return os.Rename(dir, filepath.Join(stale, "design"))
}
