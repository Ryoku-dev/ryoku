package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// A badge holder may wear their own card design on this machine before nero approves it. The pack
// is copied beside profile.json and checked again, with the limits of a submitted pack, every time
// it is read. It stays on this machine: the card record, the team service and friends' binders
// never carry it, so friends see the classic card until an approved design takes its place.

var errWearNeedsBadge = errors.New("wearing a design needs an active badge on this machine")

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
	spec, e := designWearable(p)
	if e != nil {
		return nil, e
	}
	return &badgeDesign{Dir: dir, Name: spec.Name, Worn: true, Spec: spec}, nil
}

// designWear keeps a checked design folder or zip as this machine's worn design, replacing the
// one worn before. It needs this machine's own active badge, and an approved design already on
// the card leaves nothing to wear.
func designWear(path string) error {
	b, held, published, list := ownBadge(false)
	if !held {
		return errWearNeedsBadge
	}
	if _, approved := teamApproval(list, b.Login); published && approved {
		return errors.New("your approved design is already on your card")
	}
	p, e := designReadPack(path)
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
