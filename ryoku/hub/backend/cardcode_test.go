package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"math"
	"os"
	"reflect"
	"strings"
	"testing"
	"unicode/utf8"
)

// drawCard paints a card the way an antialiasing renderer does at scale s: paper, the rulers'
// ticks with fractional edge pixels covered in part, and a coloured window
func drawCard(levels [][]int, s float64, paper, ink color.RGBA) *image.RGBA {
	return paintCard(levels, s, paper, ink, false)
}

// paintCard paints paper, the window and the ticks. With snap, every tick edge is rounded to
// a whole pixel the way Qt Quick draws a rectangle without antialiasing, so at a scale like
// 8/3 (the share still on a 133% display) a 2-unit tick comes out 5 or 6 pixels wide
func paintCard(levels [][]int, s float64, paper, ink color.RGBA, snap bool) *image.RGBA {
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
	// the part of pixel [p, p+1) a tick edge pair [lo, hi) covers
	cover := func(p int, lo, hi float64) float64 {
		if snap {
			if c := float64(p) + 0.5; c >= lo && c < hi {
				return 1
			}
			return 0
		}
		return math.Max(0, math.Min(float64(p+1), hi)-math.Max(float64(p), lo))
	}
	for r, y0 := range rulerY {
		for k, lv := range levels[r] {
			x0, x1 := rulerX+float64(k)*rulerPitch, rulerX+float64(k)*rulerPitch+rulerTick
			y1 := y0 + float64(lv+1)*rulerUnit
			for py := int(y0 * s); float64(py) < y1*s+1; py++ {
				for px := int(x0 * s); float64(px) < x1*s+1; px++ {
					if t := cover(px, x0*s, x1*s) * cover(py, y0*s, y1*s); t > 0 {
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

// Recompression strips metadata, so every field must survive in the rulers.
func TestCardCodeSurvivesAChatApp(t *testing.T) {
	want := codedDesign{kind: 1, threads: 16, ramGb: 46, refresh: 164, width: 2560,
		gpuClass: 2, distro: 1, wm: 2, cardID: [8]byte{0x3f, 0xa2, 0xc1, 4, 5, 6, 7, 8},
		name: "Nero 力", tagline: "Power and beauty.", epithet: "Ryoku", gallerySrc: "hawk", gpu: "RTX 4060"}
	themes := [][2]color.RGBA{
		{{18, 12, 0, 255}, {226, 214, 196, 255}},
		{{236, 230, 220, 255}, {40, 34, 28, 255}},
	}
	for kind := range 3 {
		want.kind = kind
		levels, err := encodeLevels(want)
		if err != nil {
			t.Fatal(err)
		}
		for _, s := range []float64{1, 1.45, 2} {
			for _, th := range themes {
				raw := jpegOf(t, drawCard(levels, s, th[0], th[1]), 72)
				img, _, err := image.Decode(bytes.NewReader(raw))
				if err != nil {
					t.Fatal(err)
				}
				_, got, err := readCard(img)
				if err != nil || got != want {
					t.Fatalf("kind %d scale %.2f: got %+v (%v), want %+v", kind, s, got, err, want)
				}
			}
		}
	}
}

// The share still is drawn at twice the display scale, so a 133% display hands over a card at
// 8/3 whose ticks the renderer snapped to 5 or 6 pixels; 120%, 110% and 175% snap too. Such a
// card has to read straight from a lossless upload and after a chat app re-encodes it.
func TestCardCodeReadsTicksSnappedToWholePixels(t *testing.T) {
	want := codedDesign{kind: 2, threads: 16, ramGb: 46, refresh: 164, width: 2560, gpuClass: 2, distro: 1, wm: 2,
		cardID: [8]byte{0x3a, 0xa4, 0xcc, 0x71, 0xd7, 0x6d, 0xfb, 0xff},
		name:   "nero", tagline: "For power and beauty", epithet: "Ryoku", gpu: "RTX 4060"}
	levels, err := encodeLevels(want)
	if err != nil {
		t.Fatal(err)
	}
	paper, ink := color.RGBA{24, 20, 16, 255}, color.RGBA{232, 222, 206, 255}
	for _, s := range []float64{2.0 * 4 / 3, 2 * 1.2, 2 * 1.1, 2 * 1.75} {
		card := paintCard(levels, s, paper, ink, true)
		reencoded, _, err := image.Decode(bytes.NewReader(jpegOf(t, card, 85)))
		if err != nil {
			t.Fatal(err)
		}
		for name, img := range map[string]image.Image{"lossless": card, "jpeg": reencoded} {
			if _, got, err := readCard(img); err != nil || got != want {
				t.Fatalf("scale %.3f %s: got %+v (%v), want %+v", s, name, got, err, want)
			}
		}
	}
}

// A card that decodes cleanly but was made by another version of the code says which side has
// to update, instead of blaming the picture.
func TestCardCodeNamesTheVersionThatMadeIt(t *testing.T) {
	body := (codedDesign{name: "friend", cardID: [8]byte{1, 2, 3, 4, 5, 6, 7, 8}}).bytes()
	for version, want := range map[byte]error{1: errOldCardCode, 2: errOldCardCode, 4: errNewCardCode, 0: errNoCardCode} {
		other := append([]byte{}, body...)
		other[0] = version<<4 | other[0]&0x0f
		levels, err := encodeFrame(other)
		if err != nil {
			t.Fatal(err)
		}
		card := drawCard(levels, 2, color.RGBA{18, 12, 0, 255}, color.RGBA{226, 214, 196, 255})
		if _, _, err := readCard(card); !errors.Is(err, want) {
			t.Fatalf("v%d card: %v, want %v", version, err, want)
		}
	}
}

func TestCardCodeBodyBudgetAndClipping(t *testing.T) {
	d := codedDesign{threads: 1000, ramGb: 1000, refresh: 2000, width: 20000, gpuClass: 9,
		name: strings.Repeat("n", 40), tagline: strings.Repeat("t", 40), epithet: strings.Repeat("e", 40),
		gallerySrc: strings.Repeat("g", 40), gpu: "NVIDIA GeForce " + strings.Repeat("x", 40) + " Laptop GPU Max-Q"}
	if n := len(d.bytes()); n != 78 || n+3 > codeBytes()-codeParity {
		t.Fatalf("body uses %d bytes, want 78 within room %d", n, codeBytes()-codeParity)
	}
	levels, err := encodeLevels(d)
	if err != nil {
		t.Fatal(err)
	}
	got, err := decodeLevels(levels)
	if err != nil {
		t.Fatal(err)
	}
	if got.threads != 255 || got.ramGb != 255 || got.refresh != 510 || got.width != 8160 || got.gpuClass != 5 ||
		len(got.name) != 14 || len(got.tagline) != 20 || len(got.epithet) != 7 || len(got.gallerySrc) != 10 || len(got.gpu) != 8 {
		t.Fatalf("clipping failed: %+v", got)
	}
	for _, n := range []int{7, 8, 10, 14, 20} {
		if s := clip(strings.Repeat("力", 20), n); len(s) > n || !utf8.ValidString(s) {
			t.Fatalf("clip split a character: %q", s)
		}
	}
	for _, gpu := range []string{"NVIDIA GeForce RTX 4060 Laptop GPU Max-Q", "AMD Radeon RX 7900", "Intel Arc A770"} {
		got := shortGPU(gpu)
		if strings.Contains(got, "NVIDIA") || strings.Contains(got, "Radeon") || strings.Contains(got, "Intel") || len(got) > 8 {
			t.Fatalf("short GPU = %q", got)
		}
	}
}

func setCardMachine(t *testing.T) [8]byte {
	t.Helper()
	// Binder tests must never reach team.ryoku.dev or a real device key.
	t.Setenv("RYOKU_TEAM_URL", "http://127.0.0.1:1")
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	id, err := ownCardID()
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func TestProfileCodeFlags(t *testing.T) {
	owner := setCardMachine(t)
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("USER", "fallback")
	if err := atomicWrite(profileConfigPath(), []byte(`{"hero":{"kind":"gallery","source":"hawk.png"},"text":{"name":"nero","tagline":"hello","epithets":["Ryoku"]}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		distro, wm string
		d, w       int
	}{
		{"arch", "hyprland", 1, 1}, {"cachyos", "niri", 2, 2}, {"fedora", "wayfire", 3, 3},
		{"void", "unknown", 4, 0}, {"nixos", "", 5, 0}, {"unknown", "", 0, 0},
	} {
		out := captureStdout(t, func() {
			if err := runProfile([]string{"code", "--threads", "16", "--ram", "46", "--refresh", "165", "--width", "2560",
				"--gpu-class", "2", "--gpu", "NVIDIA GeForce RTX 4060 Laptop GPU", "--distro", tc.distro, "--wm", tc.wm}); err != nil {
				t.Fatal(err)
			}
		})
		var code cardCode
		if err := json.Unmarshal([]byte(out), &code); err != nil {
			t.Fatal(err)
		}
		if code.X != rulerX || !reflect.DeepEqual(code.Ys, rulerY) || code.Pitch != rulerPitch || code.Tick != rulerTick || code.Unit != rulerUnit {
			t.Fatalf("changed ruler geometry: %+v", code)
		}
		got, err := decodeLevels(code.Levels)
		want := codedDesign{kind: 1, name: "nero", tagline: "hello", epithet: "Ryoku", gallerySrc: "hawk",
			threads: 16, ramGb: 46, refresh: 164, width: 2560, gpuClass: 2, gpu: "RTX 4060", distro: tc.d, wm: tc.w, cardID: owner}
		if err != nil || got != want {
			t.Fatalf("got %+v (%v), want %+v", got, err, want)
		}
	}
	if err := os.Remove(profileConfigPath()); err != nil {
		t.Fatal(err)
	}
	out := captureStdout(t, func() {
		if err := runProfile([]string{"code"}); err != nil {
			t.Fatal(err)
		}
	})
	var code cardCode
	if err := json.Unmarshal([]byte(out), &code); err != nil {
		t.Fatal(err)
	}
	got, err := decodeLevels(code.Levels)
	if want := (codedDesign{name: "fallback", cardID: owner}); err != nil || got != want {
		t.Fatalf("missing flags: %+v (%v), want %+v", got, err, want)
	}
	if err := runProfile([]string{"code", "--threads", "not-a-number"}); err == nil {
		t.Fatal("invalid numeric flag accepted")
	}
}

// A crop is not a card; a whole card shrunk below the code's resolution says it is too small,
// so the reader knows to open the full picture rather than ask for another.
func TestCardCodeRefusesWhatItCannotRead(t *testing.T) {
	levels, _ := encodeLevels(codedDesign{name: "x"})
	paper, ink := color.RGBA{18, 12, 0, 255}, color.RGBA{226, 214, 196, 255}
	cropped := drawCard(levels, 1, paper, ink).SubImage(image.Rect(0, 0, 630, 700))
	for img, want := range map[image.Image]error{cropped: errNoCardCode, drawCard(levels, 0.5, paper, ink): errSmallCardCode} {
		if _, _, err := readCard(img); !errors.Is(err, want) {
			t.Fatalf("refusal = %v, want %v", err, want)
		}
	}
}
