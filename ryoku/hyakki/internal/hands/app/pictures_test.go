package app

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"slices"
	"testing"

	"ryoku-rashin/internal/oni/capture"
	"ryoku-rashin/internal/oni/window"
)

// The real capture service behind Pictures paints the glow strips out of the
// crop the read-back reads, and only them. The screen tool is a fake that
// hands back a black screen region.
func TestPicturesPaintTheGlowStripsOut(t *testing.T) {
	var grabs [][]string
	run := func(ctx context.Context, name string, args ...string) ([]byte, error) {
		grabs = append(grabs, append([]string{name}, args...))
		img := image.NewRGBA(image.Rect(0, 0, 800, 600))
		for i := 3; i < len(img.Pix); i += 4 {
			img.Pix[i] = 255
		}
		var b bytes.Buffer
		if err := png.Encode(&b, img); err != nil {
			t.Fatal(err)
		}
		return b.Bytes(), nil
	}
	s := desk()
	svc := capture.New(run, []capture.Path{capture.PathOutputCrop},
		capture.Layout(func(context.Context) (window.Snapshot, error) { return s, nil }))
	target := *win(&s, "0x1") // 800×600 at 100,100 on DP-1
	// the top glow strip runs across the output and over the window's top
	// 30 px; the bottom one sits wholly outside the window
	strips := []window.Box{{X: 0, Y: 0, W: 1920, H: 130}, {X: 0, Y: 950, W: 1920, H: 130}}
	raw, clear, err := Pictures(svc).Shoot(context.Background(), target, strips)
	if err != nil {
		t.Fatal(err)
	}
	if len(grabs) != 1 || !slices.Equal(grabs[0], []string{"grim", "-g", "100,100 800x600", "-t", "png", "-l", "1", "-"}) {
		t.Fatalf("grabs %q", grabs)
	}
	img, err := png.Decode(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	white := color.RGBA{255, 255, 255, 255}
	for _, p := range []image.Point{{0, 0}, {799, 29}, {400, 15}} {
		if c := color.RGBAModel.Convert(img.At(p.X, p.Y)); c != white {
			t.Fatalf("%v under the strip is %v", p, c)
		}
	}
	for _, p := range []image.Point{{0, 30}, {400, 300}, {799, 599}} {
		if c := color.RGBAModel.Convert(img.At(p.X, p.Y)); c == white {
			t.Fatalf("%v outside the strips was painted", p)
		}
	}
	clear()
	for _, b := range raw[:cap(raw)] {
		if b != 0 {
			t.Fatal("clear left the picture's bytes")
		}
	}
}
