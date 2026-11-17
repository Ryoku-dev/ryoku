package main

// The Stand card's cutout: the figure cut from the reader's own picture, kept beside it as
// hero-cutout.png so the card can stand it in front of its art. Only a PNG whose ground is
// really clear qualifies, and it must keep the picture's shape so the figure lands where it
// stood in the picture.

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/png"
	"math"
	"path/filepath"
)

const (
	profileCutoutName = "hero-cutout.png"
	maxCutoutBytes    = 8 << 20
	minCutoutSide     = 64
	maxCutoutSide     = 4096
)

var (
	errCutoutNeedsBadge = errors.New("a cutout comes with a team badge")
	// only a card style draws the cutout, and a style is its owner's own look
	errCutoutNotYours = errors.New("a cutout comes with a card style of your own; make your own look through a team design")
	errCutoutNotPNG   = errors.New("a cutout is a PNG with a clear ground")
	errCutoutNoGround = errors.New("this picture has no clear ground; a cutout keeps only the figure")
)

type savedCutout struct {
	Cutout string `json:"cutout"`
	Width  int    `json:"width,omitempty"`
	Height int    `json:"height,omitempty"`
}

// cutoutImage checks a cutout's bytes and returns its pixels re-encoded as a bare PNG, so no
// chunk but the picture itself is kept or shared.
func cutoutImage(raw []byte) ([]byte, image.Rectangle, error) {
	if len(raw) > maxCutoutBytes {
		return nil, image.Rectangle{}, fmt.Errorf("this cutout is %s; a cutout can be up to %s", sizeText(int64(len(raw))), sizeText(maxCutoutBytes))
	}
	if kind, err := sniffPicture(raw); err != nil || kind.ext != "png" {
		return nil, image.Rectangle{}, errCutoutNotPNG
	}
	config, err := png.DecodeConfig(bytes.NewReader(raw))
	if err != nil {
		return nil, image.Rectangle{}, errCutoutNotPNG
	}
	if config.Width < minCutoutSide || config.Height < minCutoutSide || config.Width > maxCutoutSide || config.Height > maxCutoutSide {
		return nil, image.Rectangle{}, fmt.Errorf("a cutout is %d to %d pixels on each side; this one is %dx%d", minCutoutSide, maxCutoutSide, config.Width, config.Height)
	}
	img, err := decodePicture(raw)
	if err != nil {
		return nil, image.Rectangle{}, errCutoutNotPNG
	}
	if !cutoutHasClearGround(img) {
		return nil, image.Rectangle{}, errCutoutNoGround
	}
	var clean bytes.Buffer
	if err := png.Encode(&clean, img); err != nil {
		return nil, image.Rectangle{}, err
	}
	return clean.Bytes(), img.Bounds(), nil
}

// cutoutHasClearGround asks for a real cut: at least 2% of the pixels clear and at least 2%
// solid, so neither a plain picture nor an empty sheet passes for a figure.
func cutoutHasClearGround(img image.Image) bool {
	if opaque, ok := img.(interface{ Opaque() bool }); ok && opaque.Opaque() {
		return false
	}
	b := img.Bounds()
	need := (b.Dx()*b.Dy() + 49) / 50
	clearPixels, solidPixels := 0, 0
	count := func(alpha uint8) {
		if alpha < 16 {
			clearPixels++
		} else if alpha > 240 {
			solidPixels++
		}
	}
	if n, ok := img.(*image.NRGBA); ok {
		for y := range b.Dy() {
			at := n.PixOffset(b.Min.X, b.Min.Y+y)
			row := n.Pix[at : at+b.Dx()*4]
			for i := 3; i < len(row); i += 4 {
				count(row[i])
			}
		}
	} else {
		for y := b.Min.Y; y < b.Max.Y; y++ {
			for x := b.Min.X; x < b.Max.X; x++ {
				_, _, _, a := img.At(x, y).RGBA()
				count(uint8(a >> 8))
			}
		}
	}
	return clearPixels >= need && solidPixels >= need
}

// cutoutHeroSize is the custom picture's size, read off the same still a .ryoprofile carries.
func cutoutHeroSize() (int, int, error) {
	raw, err := readProfileFile(profileConfigPath(), maxProfileBytes)
	var model profileModel
	if err == nil {
		err = json.Unmarshal(raw, &model)
	}
	if err != nil || model.Hero.Kind != "custom" || model.Hero.Source == "" {
		return 0, 0, errors.New("choose your own picture first: the cutout is cut from it")
	}
	still, err := heroStill(model.Hero.Source)
	if err != nil {
		return 0, 0, fmt.Errorf("your picture: %w", err)
	}
	config, err := png.DecodeConfig(bytes.NewReader(still))
	if err != nil {
		return 0, 0, fmt.Errorf("your picture: %w", err)
	}
	return config.Width, config.Height, nil
}

// saveCutout validates the file at src and keeps it as the card's cutout. profile.json is
// left to Settings, which turns hero.cutout on once this succeeds.
func saveCutout(src string) (savedCutout, error) {
	b, held, _, _ := ownBadge(false)
	if !held {
		return savedCutout{}, errCutoutNeedsBadge
	}
	if len(stylesOwnedBy(b.Login)) == 0 {
		return savedCutout{}, errCutoutNotYours
	}
	heroW, heroH, err := cutoutHeroSize()
	if err != nil {
		return savedCutout{}, err
	}
	raw, err := readChosenFile(src, maxCutoutBytes, "a cutout")
	if err != nil {
		return savedCutout{}, err
	}
	clean, bounds, err := cutoutImage(raw)
	if err != nil {
		return savedCutout{}, err
	}
	w, h := bounds.Dx(), bounds.Dy()
	if math.Abs(float64(w*heroH)/float64(h*heroW)-1) > 0.02 {
		return savedCutout{}, fmt.Errorf("the cutout needs your picture's shape (%dx%d); this one is %dx%d", heroW, heroH, w, h)
	}
	if err := atomicWrite(filepath.Join(profileHeroDir(), profileCutoutName), clean, 0o644); err != nil {
		return savedCutout{}, err
	}
	return savedCutout{Cutout: profileCutoutName, Width: w, Height: h}, nil
}

// runCutout keeps a cutout or, with --clear, removes it; clearing needs no badge, since a
// reader whose badge has gone must still be able to take theirs away.
func runCutout(args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("profile cutout needs the cutout's path or --clear")
	}
	if args[0] == "--clear" {
		if err := removeIfPresent(filepath.Join(profileHeroDir(), profileCutoutName)); err != nil {
			return err
		}
		return printJSON(savedCutout{})
	}
	saved, err := saveCutout(args[0])
	if err != nil {
		return err
	}
	return printJSON(saved)
}

// packedCutout is a .ryoprofile's cutout re-checked as if chosen here, or nil when it isn't one;
// the badge and shape rules stay with the machine that cut it.
func packedCutout(packed *ryoHero) []byte {
	if packed == nil || packed.Name != profileCutoutName || len(packed.B64) > base64.StdEncoding.EncodedLen(maxCutoutBytes) {
		return nil
	}
	raw, err := base64.StdEncoding.DecodeString(packed.B64)
	if err != nil {
		return nil
	}
	clean, _, err := cutoutImage(raw)
	if err != nil {
		return nil
	}
	return clean
}
