package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"unicode/utf8"
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

// Recompression strips metadata, so every field must survive in the rulers.
func TestCardCodeSurvivesAChatApp(t *testing.T) {
	want := codedDesign{kind: 1, threads: 16, ramGb: 46, refresh: 164, width: 2560,
		gpuClass: 2, distro: 1, wm: 2, owner: [3]byte{0x3f, 0xa2, 0xc1},
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
				read, err := readLevels(img)
				if err != nil {
					t.Fatalf("kind %d scale %.2f: %v", kind, s, err)
				}
				got, err := decodeLevels(read)
				if err != nil || got != want {
					t.Fatalf("kind %d scale %.2f: got %+v (%v), want %+v", kind, s, got, err, want)
				}
			}
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
		len(got.name) != 14 || len(got.tagline) != 22 || len(got.epithet) != 8 || len(got.gallerySrc) != 10 || len(got.gpu) != 10 {
		t.Fatalf("clipping failed: %+v", got)
	}
	for _, n := range []int{8, 10, 14, 22} {
		if s := clip(strings.Repeat("力", 20), n); len(s) > n || !utf8.ValidString(s) {
			t.Fatalf("clip split a character: %q", s)
		}
	}
	for _, gpu := range []string{"NVIDIA GeForce RTX 4060 Laptop GPU Max-Q", "AMD Radeon RX 7900", "Intel Arc A770"} {
		got := shortGPU(gpu)
		if strings.Contains(got, "NVIDIA") || strings.Contains(got, "Radeon") || strings.Contains(got, "Intel") || len(got) > 10 {
			t.Fatalf("short GPU = %q", got)
		}
	}
	old := d.bytes()
	old[0] = 1 << 4
	if _, err := parseDesign(old); !errors.Is(err, errNoCardCode) {
		t.Fatalf("v1 accepted: %v", err)
	}
}

func setCardMachine(t *testing.T) [3]byte {
	t.Helper()
	old := machineIDPath
	machineIDPath = filepath.Join(t.TempDir(), "machine-id")
	t.Cleanup(func() { machineIDPath = old })
	const machineID = "test-binder-machine"
	if err := os.WriteFile(machineIDPath, []byte("  "+machineID+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256([]byte(machineID))
	return [3]byte{sum[0], sum[1], sum[2]}
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
			threads: 16, ramGb: 46, refresh: 164, width: 2560, gpuClass: 2, gpu: "RTX 4060", distro: tc.d, wm: tc.w, owner: owner}
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
	if want := (codedDesign{name: "fallback", owner: owner}); err != nil || got != want {
		t.Fatalf("missing flags: %+v (%v), want %+v", got, err, want)
	}
	if err := runProfile([]string{"code", "--threads", "not-a-number"}); err == nil {
		t.Fatal("invalid numeric flag accepted")
	}
}

func TestCardCodeRefusesWhatItCannotRead(t *testing.T) {
	levels, _ := encodeLevels(codedDesign{name: "x"})
	card := drawCard(levels, 1, color.RGBA{18, 12, 0, 255}, color.RGBA{226, 214, 196, 255})
	cropped := card.SubImage(image.Rect(0, 0, 630, 700))
	for _, img := range []image.Image{cropped, drawCard(levels, 0.5, color.RGBA{18, 12, 0, 255}, color.RGBA{226, 214, 196, 255})} {
		if _, err := readLevels(img); !errors.Is(err, errNoCardCode) {
			t.Fatalf("misread a card it should refuse: %v", err)
		}
	}
}
