package main

import (
	"bytes"
	"encoding/json"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"testing"
)

// drawCard paints a card the way the card does at scale s: paper, the rulers' ticks
// with fractional edges covered the way a renderer blends them, and a coloured window
func drawCard(levels [][]int, s float64, paper, ink color.RGBA) *image.RGBA {
	w, h := int(math.Round(cardW*s)), int(math.Round(cardH*s))
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.Set(x, y, paper)
		}
	}
	for y := windowRect.Min.Y; float64(y) < float64(windowRect.Max.Y); y++ {
		for x := windowRect.Min.X; x < windowRect.Max.X; x++ {
			for py := int(float64(y) * s); py < int(float64(y+1)*s); py++ {
				for px := int(float64(x) * s); px < int(float64(x+1)*s); px++ {
					img.Set(px, py, color.RGBA{200, 30, 60, 255})
				}
			}
		}
	}
	blend := func(a, b uint8, t float64) uint8 { return uint8(float64(a)*(1-t) + float64(b)*t) }
	for r, y0 := range rulerY {
		for k, lv := range levels[r] {
			x0, x1 := rulerX+float64(k)*rulerPitch, rulerX+float64(k)*rulerPitch+rulerTick
			y1 := y0 + float64(lv+1)*rulerUnit
			for py := int(y0 * s); float64(py) < y1*s+1; py++ {
				for px := int(x0 * s); float64(px) < x1*s+1; px++ {
					cx := math.Max(0, math.Min(float64(px+1), x1*s)-math.Max(float64(px), x0*s))
					cy := math.Max(0, math.Min(float64(py+1), y1*s)-math.Max(float64(py), y0*s))
					t := cx * cy
					if t > 0 {
						img.Set(px, py, color.RGBA{blend(paper.R, ink.R, t), blend(paper.G, ink.G, t), blend(paper.B, ink.B, t), 255})
					}
				}
			}
		}
	}
	return img
}

func jpegOf(t *testing.T, img image.Image, q int) []byte {
	t.Helper()
	var b bytes.Buffer
	if err := jpeg.Encode(&b, img, &jpeg.Options{Quality: q}); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

// A card shared through a chat app reaches the other side resized and recompressed,
// its PNG chunks gone: the design must still come back from the pixels, at the sizes
// chat apps send and on a dark or a light card.
func TestCardCodeSurvivesAChatApp(t *testing.T) {
	want := codedDesign{kind: 1, left: true, cardHome: true, epithets: true, specs: true,
		name: "Nero 力", tagline: "For the sake of power and beauty.", epithet: "Ryoku", gallerySrc: "hawk"}
	levels, err := encodeLevels(want)
	if err != nil {
		t.Fatal(err)
	}
	themes := [][2]color.RGBA{
		{{18, 12, 0, 255}, {226, 214, 196, 255}},
		{{236, 230, 220, 255}, {40, 34, 28, 255}},
	}
	for _, s := range []float64{1, 1.45, 2} {
		for _, th := range themes {
			raw := jpegOf(t, drawCard(levels, s, th[0], th[1]), 72)
			env, err := envelopeFromPixels(raw)
			if err != nil {
				t.Fatalf("scale %.2f: %v", s, err)
			}
			var e ryoEnvelope
			if err := json.Unmarshal(env, &e); err != nil {
				t.Fatal(err)
			}
			var p struct {
				HeroSide, HomeView string
				Hero               struct{ Kind, Source string }
				Text               struct {
					Name, Tagline string
					Epithets      []string
				}
			}
			if err := json.Unmarshal(e.Profile, &p); err != nil {
				t.Fatal(err)
			}
			if p.Text.Name != want.name || p.Text.Tagline != want.tagline || p.Hero.Source != "hawk.png" ||
				p.HeroSide != "left" || p.HomeView != "card" || len(p.Text.Epithets) != 1 {
				t.Fatalf("scale %.2f: got %+v", s, p)
			}
		}
	}
}

// A card with the reader's own picture brings the picture back from its window.
func TestCardCodeLiftsTheOwnPicture(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	levels, err := encodeLevels(codedDesign{kind: 2, name: "nero"})
	if err != nil {
		t.Fatal(err)
	}
	raw := jpegOf(t, drawCard(levels, 1.45, color.RGBA{18, 12, 0, 255}, color.RGBA{226, 214, 196, 255}), 80)
	src := filepath.Join(t.TempDir(), "shared.jpg")
	if err := os.WriteFile(src, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := importProfile(src); err != nil {
		t.Fatalf("importProfile: %v", err)
	}
	f, err := os.Open(filepath.Join(profileHeroDir(), "hero.png"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	pic, err := png.Decode(f)
	if err != nil {
		t.Fatal(err)
	}
	r, _, _, _ := pic.At(pic.Bounds().Dx()/2, pic.Bounds().Dy()/2).RGBA()
	if dx := pic.Bounds().Dx(); dx < 780 || dx > 830 || r>>8 < 150 {
		t.Fatalf("lifted %v, red %d: not the window", pic.Bounds(), r>>8)
	}
}

// A card cut down or drawn too small to read is refused rather than misread.
func TestCardCodeRefusesWhatItCannotRead(t *testing.T) {
	levels, _ := encodeLevels(codedDesign{name: "x"})
	card := drawCard(levels, 1, color.RGBA{18, 12, 0, 255}, color.RGBA{226, 214, 196, 255})
	cropped := card.SubImage(image.Rect(0, 0, 630, 700))
	for _, img := range []image.Image{cropped, drawCard(levels, 0.5, color.RGBA{18, 12, 0, 255}, color.RGBA{226, 214, 196, 255})} {
		if _, err := envelopeFromPixels(jpegOf(t, img, 80)); err == nil {
			t.Fatal("misread a card it should refuse")
		}
	}
}
