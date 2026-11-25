package capture

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/png"
	"math"
	"slices"
	"testing"

	"ryoku-rashin/internal/oni/feature"
)

// quadrants is a 4×4 picture with a flat colour in each 2×2 quarter.
func quadrants() *Shot {
	m := image.NewRGBA(image.Rect(0, 0, 4, 4))
	for y := range 4 {
		for x := range 4 {
			c := map[[2]bool]color.RGBA{
				{false, false}: {0, 0, 0, 255},
				{true, false}:  {255, 255, 255, 255},
				{false, true}:  {255, 0, 0, 255},
				{true, true}:   {0, 0, 255, 255},
			}[[2]bool{x >= 2, y >= 2}]
			m.SetRGBA(x, y, c)
		}
	}
	return &Shot{img: m}
}

func TestPNGIsTheWholePictureLossless(t *testing.T) {
	shot := &Shot{img: pattern(37, 23)}
	b, err := shot.PNG()
	if err != nil {
		t.Fatal(err)
	}
	m, err := png.Decode(bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	if m.Bounds() != shot.img.Rect {
		t.Fatalf("bounds %v, want %v", m.Bounds(), shot.img.Rect)
	}
	for y := range 23 {
		for x := range 37 {
			if got, want := color.RGBAModel.Convert(m.At(x, y)), shot.img.At(x, y); got != want {
				t.Fatalf("pixel %d,%d is %v, want %v", x, y, got, want)
			}
		}
	}
}

func TestGrayAveragesEachCell(t *testing.T) {
	// BT.601 luma ×1000 of each quarter's colour
	const black, white, red, blue = 0.0, 255000.0, 76245.0, 29070.0
	l := func(v float64) uint8 { return uint8(math.Round(v / 1000)) }
	cases := []struct {
		name   string
		w, h   int
		region *feature.Box
		want   []uint8
	}{
		{"quarters", 2, 2, nil, []uint8{l(black), l(white), l(red), l(blue)}},
		{"one cell", 1, 1, nil, []uint8{l((black + white + red + blue) / 4)}},
		{"right half", 1, 2, &feature.Box{X: 0.5, Y: 0, W: 0.5, H: 1}, []uint8{l(white), l(blue)}},
		{"bottom left quarter, grown", 2, 2, &feature.Box{X: 0, Y: 0.5, W: 0.5, H: 0.5}, []uint8{l(red), l(red), l(red), l(red)}},
		{"clamped to the picture", 1, 1, &feature.Box{X: 0.5, Y: -1, W: 2, H: 1.5}, []uint8{l(white)}},
		// three cells over four pixels: the middle one is two thirds of px 1
		// (red) and two thirds of px 2 (blue)
		{"cells split pixels", 3, 1, &feature.Box{X: 0, Y: 0.5, W: 1, H: 0.5}, []uint8{l(red), l((red + blue) / 2), l(blue)}},
	}
	for _, c := range cases {
		got := quadrants().Gray(c.w, c.h, c.region)
		if !slices.Equal(got, c.want) {
			t.Errorf("%s: %v, want %v", c.name, got, c.want)
		}
	}
}

func TestGrayRefusesWhatItCannotReduce(t *testing.T) {
	shot := quadrants()
	nan := math.NaN()
	for name, c := range map[string]struct {
		w, h   int
		region *feature.Box
	}{
		"no width":        {0, 4, nil},
		"negative height": {4, -1, nil},
		"too many cells":  {MaxGray, 2, nil},
		"empty region":    {2, 2, &feature.Box{X: 0.2, Y: 0.2, W: 0, H: 0.5}},
		"region outside":  {2, 2, &feature.Box{X: 1.2, Y: 0, W: 0.5, H: 0.5}},
		"region not real": {2, 2, &feature.Box{X: nan, Y: 0, W: 0.5, H: 0.5}},
		"region infinite": {2, 2, &feature.Box{X: 0, Y: 0, W: math.Inf(1), H: 0.5}},
	} {
		if got := shot.Gray(c.w, c.h, c.region); got != nil {
			t.Errorf("%s: %v", name, got)
		}
	}
}

func TestPNGAndGrayAreGoneAfterClear(t *testing.T) {
	shot := &Shot{img: pattern(64, 40)}
	b, err := shot.PNG()
	if err != nil {
		t.Fatal(err)
	}
	g := shot.Gray(16, 10, nil)
	crop := shot.Gray(8, 8, &feature.Box{X: 0.25, Y: 0.25, W: 0.5, H: 0.5})
	if allZero(b) || allZero(g) || allZero(crop) {
		t.Fatal("the buffers were empty before Clear")
	}
	shot.Clear()
	for name, buf := range map[string][]byte{"png": b, "gray": g, "crop": crop} {
		if !allZero(buf) {
			t.Errorf("%s not zeroed", name)
		}
	}
	if b, err := shot.PNG(); b != nil || !errors.Is(err, ErrCleared) {
		t.Errorf("PNG after Clear gave %d bytes, %v", len(b), err)
	}
	if g := shot.Gray(16, 10, nil); g != nil {
		t.Errorf("Gray after Clear gave %v", g)
	}
}
