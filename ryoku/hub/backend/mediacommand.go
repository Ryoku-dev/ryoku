package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"image"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func mediaPreview(raw []byte) (map[string]any, error) {
	media, err := inspectMedia(context.Background(), raw, false)
	if err != nil {
		return nil, err
	}
	img, err := decodePictureLocal(media.Poster)
	if err != nil {
		return nil, err
	}
	w, h := img.Bounds().Dx(), img.Bounds().Dy()
	if w > 512 || h > 512 {
		scale := max(w, h)
		nw, nh := max(1, w*512/scale), max(1, h*512/scale)
		thumb := image.NewNRGBA(image.Rect(0, 0, nw, nh))
		for y := range nh {
			for x := range nw {
				thumb.Set(x, y, img.At(img.Bounds().Min.X+x*w/nw, img.Bounds().Min.Y+y*h/nh))
			}
		}
		media.Poster, err = pngBytes(thumb)
		if err != nil {
			return nil, err
		}
	}
	base, err := os.UserCacheDir()
	if err != nil {
		return nil, err
	}
	base = filepath.Join(base, "ryoku", "card-previews-v1")
	if err = os.MkdirAll(base, 0700); err != nil {
		return nil, err
	}
	if st, e := os.Lstat(base); e != nil || !st.IsDir() || st.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("unsafe card preview cache")
	}
	sum := sha256.Sum256(media.Poster)
	digest := hex.EncodeToString(sum[:])
	target := filepath.Join(base, digest+".png")
	// Only this job's freshly encoded poster is accepted, never an uploader's
	// claimed-clean path. Replacing an object is atomic and cannot follow a link.
	if err = atomicWrite(target, media.Poster, 0600); err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(base)
	if err != nil {
		return nil, err
	}
	type item struct {
		name string
		time int64
		size int64
	}
	items := []item{}
	var total int64
	for _, entry := range entries {
		name := entry.Name()
		if len(name) != 68 || !strings.HasSuffix(name, ".png") {
			return nil, fmt.Errorf("unexpected card preview cache entry")
		}
		if _, e := hex.DecodeString(name[:64]); e != nil {
			return nil, fmt.Errorf("invalid card preview cache entry")
		}
		st, e := entry.Info()
		if e != nil {
			return nil, e
		}
		if !st.Mode().IsRegular() {
			return nil, fmt.Errorf("unsafe card preview cache entry")
		}
		items = append(items, item{name, st.ModTime().UnixNano(), st.Size()})
		total += st.Size()
	}
	sort.Slice(items, func(i, j int) bool { return items[i].time < items[j].time })
	count := len(items)
	for _, item := range items {
		if count <= 64 && total <= 16<<20 {
			break
		}
		if item.name == digest+".png" {
			continue
		}
		if err = os.Remove(filepath.Join(base, item.name)); err != nil {
			return nil, err
		}
		count--
		total -= item.size
	}
	sourceSum := sha256.Sum256(raw)
	return map[string]any{"path": target, "sha": digest, "sourceSha": hex.EncodeToString(sourceSum[:]), "width": media.Width, "height": media.Height, "frames": media.Frames, "extension": media.Extension}, nil
}

// Revalidate legacy on-disk media before presentation too: an old accepted hero
// is not proof that its original bytes have crossed the new decoder boundary.
func currentCardMedia() (map[string]any, error) {
	raw, err := readProfileFile(profileConfigPath(), maxProfileBytes)
	if err != nil {
		return nil, err
	}
	var profile profileModel
	if err = json.Unmarshal(raw, &profile); err != nil {
		return nil, err
	}
	if profile.Hero.Kind != "custom" || profile.Hero.Source == "" {
		return map[string]any{}, nil
	}
	source := profile.Hero.Source
	legacyStill := source == "hero.bmp" || source == "hero.avif"
	if !legacyStill && !profilePictureName.MatchString(source) && !profileMovingName.MatchString(source) {
		return nil, fmt.Errorf("unsafe custom picture filename")
	}
	raw, err = readPictureFile(filepath.Join(profileHeroDir(), source))
	if err != nil {
		return nil, err
	}
	kind, err := sniffPicture(raw)
	if err != nil {
		return nil, err
	}
	ext := filepath.Ext(source)
	videoName := ext == ".mp4" || ext == ".webm" || ext == ".mkv"
	if kind.video != videoName {
		return nil, fmt.Errorf("custom picture kind does not match its filename")
	}
	if kind.video && !badgeHolder() {
		return nil, errMovingNeedsBadge
	}
	media, err := inspectMedia(context.Background(), raw, kind.video)
	if err != nil {
		return nil, err
	}
	base, err := os.UserCacheDir()
	if err != nil {
		return nil, err
	}
	base = filepath.Join(base, "ryoku", "card-current-media-v1")
	if err = os.MkdirAll(base, 0700); err != nil {
		return nil, err
	}
	if st, e := os.Lstat(base); e != nil || !st.IsDir() || st.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("unsafe current media cache")
	}
	file, poster := filepath.Join(base, "current."+media.Extension), filepath.Join(base, "poster.png")
	if err = atomicWrite(file, media.Data, 0600); err != nil {
		return nil, err
	}
	if err = atomicWrite(poster, media.Poster, 0600); err != nil {
		return nil, err
	}
	result := map[string]any{"source": source, "file": file, "poster": poster, "frames": media.Frames, "extension": media.Extension}
	digest := sha256.Sum256(media.Data)
	result["digest"] = hex.EncodeToString(digest[:])
	if profile.Hero.Cutout {
		raw, e := readPictureFile(filepath.Join(profileHeroDir(), profileCutoutName))
		if e != nil {
			return nil, e
		}
		clean, _, e := cutoutImage(raw)
		if e != nil {
			return nil, e
		}
		path := filepath.Join(base, "cutout.png")
		if e = atomicWrite(path, clean, 0600); e != nil {
			return nil, e
		}
		result["cutout"] = path
	}
	// Keep only the current canonical stream/poster/cutout, never a growing
	// collection of prior originals or animations.
	entries, err := os.ReadDir(base)
	if err != nil {
		return nil, err
	}
	allowed := map[string]bool{"current.png": true, "current.jpg": true, "current.gif": true, "current.webp": true, "current.webm": true, "poster.png": true, "cutout.png": true}
	for _, entry := range entries {
		if !allowed[entry.Name()] || entry.IsDir() {
			return nil, fmt.Errorf("unexpected current media cache entry")
		}
		p := filepath.Join(base, entry.Name())
		if p != file && p != poster && !(profile.Hero.Cutout && entry.Name() == "cutout.png") {
			if err = os.Remove(p); err != nil {
				return nil, err
			}
		}
	}
	return result, nil
}

func runCardMedia(args []string) error {
	if len(args) == 1 && args[0] == "current" {
		result, err := currentCardMedia()
		if err != nil {
			return err
		}
		return printJSON(result)
	}
	if len(args) < 2 {
		return fmt.Errorf("usage: media preview <image> | inspect <image> <output-dir>")
	}
	raw, err := readChosenFile(args[1], maxPictureBytes, "a picture")
	if err != nil {
		return err
	}
	switch args[0] {
	case "preview":
		if len(args) != 2 {
			return fmt.Errorf("usage: media preview <image>")
		}
		result, e := mediaPreview(raw)
		if e != nil {
			return e
		}
		return printJSON(result)
	case "inspect":
		if len(args) != 3 {
			return fmt.Errorf("usage: media inspect <image> <output-dir>")
		}
		media, e := inspectMedia(context.Background(), raw, false)
		if e != nil {
			return e
		}
		// The explicit smoke/export destination must be new; no live files replaced.
		if e = os.Mkdir(args[2], 0700); e != nil {
			return e
		}
		if e = atomicWrite(filepath.Join(args[2], "canonical."+media.Extension), media.Data, 0600); e != nil {
			return e
		}
		if e = atomicWrite(filepath.Join(args[2], "poster.png"), media.Poster, 0600); e != nil {
			return e
		}
		return printJSON(map[string]any{"extension": media.Extension, "width": media.Width, "height": media.Height, "frames": media.Frames, "durations": media.Durations})
	}
	return fmt.Errorf("unknown media command")
}
