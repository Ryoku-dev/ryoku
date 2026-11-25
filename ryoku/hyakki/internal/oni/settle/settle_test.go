package settle

import (
	"errors"
	"math"
	"slices"
	"testing"
	"time"

	"ryoku-rashin/internal/oni/feature"
	"ryoku-rashin/internal/oni/spec"
)

func specConfig(t *testing.T) Config {
	t.Helper()
	sp, err := spec.Load()
	if err != nil {
		t.Fatal(err)
	}
	return FromSpec(sp)
}

// screen is a synthetic gray picture that reduces itself the way a
// capture.Shot does: region in fractions, each cell the mean of its pixels,
// nil once cleared, and every handed-out buffer held until Clear.
type screen struct {
	w, h int
	pix  []uint8
	held [][]uint8
}

func newScreen(w, h int, fill func(x, y int) uint8) *screen {
	s := &screen{w: w, h: h, pix: make([]uint8, w*h)}
	for y := range h {
		for x := range w {
			s.pix[y*w+x] = fill(x, y)
		}
	}
	return s
}

func (s *screen) clone() *screen {
	return &screen{w: s.w, h: s.h, pix: slices.Clone(s.pix)}
}

// paint sets the pixels of the rectangle x, y, w×h to v.
func (s *screen) paint(x, y, w, h int, v uint8) {
	for j := y; j < y+h; j++ {
		for i := x; i < x+w; i++ {
			s.pix[j*s.w+i] = v
		}
	}
}

func (s *screen) Gray(w, h int, region *feature.Box) []uint8 {
	if s.pix == nil || w <= 0 || h <= 0 {
		return nil
	}
	x0, y0, x1, y1 := 0, 0, s.w, s.h
	if region != nil {
		x0 = max(0, int(math.Round(region.X*float64(s.w))))
		y0 = max(0, int(math.Round(region.Y*float64(s.h))))
		x1 = min(s.w, int(math.Round((region.X+region.W)*float64(s.w))))
		y1 = min(s.h, int(math.Round((region.Y+region.H)*float64(s.h))))
	}
	rw, rh := x1-x0, y1-y0
	if rw < w || rh < h {
		return nil
	}
	out := make([]uint8, w*h)
	for j := range h {
		for i := range w {
			sum, n := 0, 0
			for y := y0 + j*rh/h; y < y0+(j+1)*rh/h; y++ {
				for x := x0 + i*rw/w; x < x0+(i+1)*rw/w; x++ {
					sum += int(s.pix[y*s.w+x])
					n++
				}
			}
			out[j*w+i] = uint8((sum + n/2) / n)
		}
	}
	s.held = append(s.held, out)
	return out
}

func (s *screen) Clear() {
	clear(s.pix)
	s.pix = nil
	for _, b := range s.held {
		clear(b)
	}
	s.held = nil
}

// texture is a busy light background, so a fingerprint is not trivially flat.
func texture(x, y int) uint8 { return uint8(180 + (x/7+y/5)%40) }

// shifted returns a copy of base whose cells differ from it by sum in total,
// spread as evenly as whole cells allow.
func shifted(base []uint8, sum int) []uint8 {
	out := slices.Clone(base)
	n := len(out)
	for i := range out {
		d := sum / n
		if i < sum%n {
			d++
		}
		out[i] -= uint8(d)
	}
	return out
}

func TestSpecNumbers(t *testing.T) {
	c := specConfig(t)
	want := Config{W: 64, H: 40, MoveDelta: 4.0, StillDelta: 1.5, Frame: 150 * time.Millisecond, Nap: 450 * time.Millisecond, MaxFrames: 20}
	if c != want {
		t.Fatalf("FromSpec = %+v, want %+v", c, want)
	}
}

func TestThresholds(t *testing.T) {
	c := specConfig(t)
	n := c.W * c.H
	base := make([]uint8, n)
	for i := range base {
		base[i] = uint8(100 + i%50)
	}
	print := func(whole, crop float64) Print {
		return Print{Whole: shifted(base, int(math.Round(whole*float64(n)))), Crop: shifted(base, int(math.Round(crop*float64(n))))}
	}
	ref := Print{Whole: base, Crop: base}
	for _, tc := range []struct {
		name         string
		whole, crop  float64
		moved, still bool
	}{
		{"whole at move", 4.0, 0, true, false},
		{"crop at move", 0, 4.0, true, false},
		{"both well past move", 30, 12, true, false},
		{"whole just under move", 3.9, 0, false, false},
		{"crop just under move", 0, 3.9, false, false},
		{"both at still", 1.5, 1.5, false, true},
		{"nothing", 0, 0, false, true},
		{"whole just over still", 1.6, 0, false, false},
		{"crop just over still", 1.5, 1.6, false, false},
		{"between", 2.5, 2.5, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ch := Delta(ref, print(tc.whole, tc.crop))
			if ch.Whole != tc.whole || ch.Crop != tc.crop {
				t.Fatalf("Delta = %+v, want whole %v crop %v", ch, tc.whole, tc.crop)
			}
			if got := c.Moved(ch); got != tc.moved {
				t.Errorf("Moved = %v, want %v", got, tc.moved)
			}
			if got := c.Still(ch); got != tc.still {
				t.Errorf("Still = %v, want %v", got, tc.still)
			}
		})
	}
}

func TestDeltaIsSymmetricAndCountsDarkerAndLighter(t *testing.T) {
	a := Print{Whole: []uint8{10, 200, 50, 50}, Crop: []uint8{0, 0, 0, 0}}
	b := Print{Whole: []uint8{20, 190, 50, 50}, Crop: []uint8{255, 0, 0, 0}}
	ab, ba := Delta(a, b), Delta(b, a)
	if ab != ba {
		t.Fatalf("Delta(a, b) = %+v, Delta(b, a) = %+v", ab, ba)
	}
	if ab.Whole != 5 || ab.Crop != 63.75 {
		t.Fatalf("Delta = %+v, want whole 5 crop 63.75", ab)
	}
}

// The screens below are 2560×1600, so the 64×40 whole fingerprint has
// 40×40-pixel cells and the crops line up with whole pixels.
const sw, sh = 2560, 1600

func TestEqualFramesAreStill(t *testing.T) {
	c := specConfig(t)
	crop := feature.Box{X: 0.4, Y: 0.4, W: 0.05, H: 0.05}
	a := newScreen(sw, sh, texture)
	b := a.clone()
	pa, err := c.Take(a, crop)
	if err != nil {
		t.Fatal(err)
	}
	pb, err := c.Take(b, crop)
	if err != nil {
		t.Fatal(err)
	}
	ch := Delta(pa, pb)
	if ch != (Change{}) || !c.Still(ch) || c.Moved(ch) {
		t.Fatalf("equal frames: %+v still %v moved %v", ch, c.Still(ch), c.Moved(ch))
	}
	if ch := Delta(pa, pa); !c.Still(ch) {
		t.Fatalf("a print against itself: %+v", ch)
	}
}

func TestSmallChangeInsideTheCropCountsOnlyOnTheCrop(t *testing.T) {
	c := specConfig(t)
	for _, tc := range []struct {
		name string
		crop feature.Box
		// the changed rectangle, in screen pixels, inside the crop
		x, y, w, h int
		cropMoved  bool
	}{
		// A 16×24 pointer over a 128×80 button.
		{"pointer on a button", feature.Box{X: 0.4, Y: 0.4, W: 0.05, H: 0.05}, 1024 + 50, 640 + 30, 16, 24, true},
		// A 2×24 caret in a 128×40 text field.
		{"caret in a field", feature.Box{X: 0.2, Y: 0.5, W: 0.05, H: 0.025}, 512 + 60, 800 + 8, 2, 24, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			before := newScreen(sw, sh, texture)
			after := before.clone()
			after.paint(tc.x, tc.y, tc.w, tc.h, 10)
			pb, err := c.Take(before, tc.crop)
			if err != nil {
				t.Fatal(err)
			}
			pa, err := c.Take(after, tc.crop)
			if err != nil {
				t.Fatal(err)
			}
			ch := Delta(pb, pa)
			if ch.Whole > c.StillDelta {
				t.Errorf("whole delta %v: a change this small must leave the whole picture still", ch.Whole)
			}
			if ch.Crop <= c.StillDelta {
				t.Errorf("crop delta %v: the crop must see the change", ch.Crop)
			}
			if c.Still(ch) {
				t.Errorf("Still(%+v) = true", ch)
			}
			if got := c.Moved(ch); got != tc.cropMoved {
				t.Errorf("Moved(%+v) = %v, want %v", ch, got, tc.cropMoved)
			}
		})
	}
}

func TestChangeOutsideTheCropLeavesTheCropStill(t *testing.T) {
	c := specConfig(t)
	crop := feature.Box{X: 0.4, Y: 0.4, W: 0.05, H: 0.05}
	before := newScreen(sw, sh, texture)
	after := before.clone()
	after.paint(0, 0, 640, 400, 10) // a sixteenth of the screen, far from the crop
	pb, err := c.Take(before, crop)
	if err != nil {
		t.Fatal(err)
	}
	pa, err := c.Take(after, crop)
	if err != nil {
		t.Fatal(err)
	}
	ch := Delta(pb, pa)
	if ch.Crop != 0 {
		t.Errorf("crop delta %v, want 0", ch.Crop)
	}
	if !c.Moved(ch) {
		t.Errorf("Moved(%+v) = false: the whole picture changed", ch)
	}
}

func TestTakeOfAClearedPictureIsAnError(t *testing.T) {
	c := specConfig(t)
	s := newScreen(sw, sh, texture)
	s.Clear()
	p, err := c.Take(s, feature.Box{X: 0.1, Y: 0.1, W: 0.2, H: 0.2})
	if !errors.Is(err, ErrNoPicture) {
		t.Fatalf("Take of a cleared picture: err %v", err)
	}
	if p.Whole != nil || p.Crop != nil {
		t.Fatalf("Take of a cleared picture returned cells")
	}
}

func TestTakeRefusesABadCropOrSize(t *testing.T) {
	c := specConfig(t)
	s := newScreen(sw, sh, texture)
	for _, tc := range []struct {
		name string
		c    Config
		crop feature.Box
		want error
	}{
		{"zero width", c, feature.Box{X: 0.1, Y: 0.1, H: 0.2}, ErrBadCrop},
		{"negative height", c, feature.Box{X: 0.1, Y: 0.1, W: 0.2, H: -0.2}, ErrBadCrop},
		{"NaN", c, feature.Box{X: math.NaN(), Y: 0.1, W: 0.2, H: 0.2}, ErrBadCrop},
		{"infinite", c, feature.Box{X: 0.1, Y: 0.1, W: math.Inf(1), H: 0.2}, ErrBadCrop},
		{"off the picture", c, feature.Box{X: 1.5, Y: 1.5, W: 0.2, H: 0.2}, ErrBadCrop},
		{"zero fingerprint", Config{}, feature.Box{X: 0.1, Y: 0.1, W: 0.2, H: 0.2}, ErrBadSize},
		{"huge fingerprint", Config{W: 1 << 11, H: 1 << 10}, feature.Box{X: 0.1, Y: 0.1, W: 0.2, H: 0.2}, ErrBadSize},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := tc.c.Take(s, tc.crop); !errors.Is(err, tc.want) {
				t.Fatalf("err %v, want %v", err, tc.want)
			}
		})
	}
}

// wrongSize hands back fingerprints of a size nobody asked for.
type wrongSize struct{}

func (wrongSize) Gray(w, h int, _ *feature.Box) []uint8 { return make([]uint8, w*h-1) }

func TestTakeRefusesAFingerprintOfTheWrongSize(t *testing.T) {
	if _, err := specConfig(t).Take(wrongSize{}, feature.Box{W: 1, H: 1}); !errors.Is(err, ErrBadSize) {
		t.Fatalf("err %v, want %v", err, ErrBadSize)
	}
}

func TestPrintOutlivesThePictureAndLeavesNoCellsInIt(t *testing.T) {
	c := specConfig(t)
	s := newScreen(sw, sh, texture)
	p, err := c.Take(s, feature.Box{X: 0.4, Y: 0.4, W: 0.05, H: 0.05})
	if err != nil {
		t.Fatal(err)
	}
	if len(s.held) != 2 {
		t.Fatalf("Take asked for %d fingerprints, want 2", len(s.held))
	}
	for i, b := range s.held {
		if slices.ContainsFunc(b, func(v uint8) bool { return v != 0 }) {
			t.Errorf("buffer %d the picture handed over was not zeroed", i)
		}
	}
	whole, crop := slices.Clone(p.Whole), slices.Clone(p.Crop)
	s.Clear()
	if !slices.Equal(p.Whole, whole) || !slices.Equal(p.Crop, crop) {
		t.Fatal("clearing the picture changed the Print")
	}
	if slices.Index(whole, 0) >= 0 {
		t.Fatal("the whole fingerprint of a light picture holds a black cell")
	}
	cells := p.Whole
	p.Clear()
	if p.Whole != nil || p.Crop != nil || slices.ContainsFunc(cells, func(v uint8) bool { return v != 0 }) {
		t.Fatal("Print.Clear left cells behind")
	}
}

func TestPrintsThatCannotBeComparedCountAsMoved(t *testing.T) {
	c := specConfig(t)
	a := Print{Whole: []uint8{1, 2}, Crop: []uint8{1, 2}}
	for _, b := range []Print{{}, {Whole: []uint8{1}, Crop: []uint8{1}}} {
		ch := Delta(a, b)
		if !c.Moved(ch) || c.Still(ch) {
			t.Errorf("Delta(%v, %v) = %+v: moved %v still %v", a, b, ch, c.Moved(ch), c.Still(ch))
		}
	}
	if ch := Delta(Print{}, Print{}); !c.Moved(ch) {
		t.Errorf("two empty prints: %+v", ch)
	}
}

// painted is a screen that names the cells its painted-out rects touch, as
// capture.Shot does for the presenter's masks.
type painted struct {
	*screen
	out [][4]int
}

// mask paints x, y, w×h white as a capture paints Oni out, and keeps it.
func (p *painted) mask(x, y, w, h int) {
	p.paint(x, y, w, h, 255)
	p.out = append(p.out, [4]int{x, y, x + w, y + h})
}

func (p *painted) Masked(w, h int, region *feature.Box) []bool {
	if p.pix == nil || len(p.out) == 0 {
		return nil
	}
	x0, y0, x1, y1 := 0, 0, p.w, p.h
	if region != nil {
		x0 = max(0, int(math.Round(region.X*float64(p.w))))
		y0 = max(0, int(math.Round(region.Y*float64(p.h))))
		x1 = min(p.w, int(math.Round((region.X+region.W)*float64(p.w))))
		y1 = min(p.h, int(math.Round((region.Y+region.H)*float64(p.h))))
	}
	rw, rh := x1-x0, y1-y0
	out := make([]bool, w*h)
	for j := range h {
		for i := range w {
			cx0, cx1 := x0+i*rw/w, x0+(i+1)*rw/w
			cy0, cy1 := y0+j*rh/h, y0+(j+1)*rh/h
			for _, r := range p.out {
				if cx0 < r[2] && cx1 > r[0] && cy0 < r[3] && cy1 > r[1] {
					out[j*w+i] = true
				}
			}
		}
	}
	return out
}

// Oni's card and strand sit in different places in the picture a step was
// read from and in the frames after Next, each painted out. Those cells are
// no change of the window: the fingerprints stay still, even with the crop
// under the strand's box, while a real change elsewhere still moves them.
func TestPaintedOutPartsOfEitherPictureAreNoChange(t *testing.T) {
	c := specConfig(t)
	crop := feature.Box{X: 0.1, Y: 0.6, W: 0.08, H: 0.06}
	base := &painted{screen: newScreen(sw, sh, texture)}
	base.mask(1600, 200, 500, 300) // the card where it waited while the step was read
	frame := &painted{screen: newScreen(sw, sh, texture)}
	frame.mask(300, 300, 1500, 1000) // the strand's box out to the target, over the crop
	frame.mask(700, 1200, 500, 300)  // the card beside the target
	pb, err := c.Take(base, crop)
	if err != nil {
		t.Fatal(err)
	}
	pf, err := c.Take(frame, crop)
	if err != nil {
		t.Fatal(err)
	}
	ch := Delta(pb, pf)
	if !c.Still(ch) || c.Moved(ch) {
		t.Fatalf("only Oni's own parts moved, yet %+v", ch)
	}
	// unmasked, the same two pictures read as a change
	if plain := Delta(Print{Whole: pb.Whole, Crop: pb.Crop}, Print{Whole: pf.Whole, Crop: pf.Crop}); !c.Moved(plain) {
		t.Fatalf("the painted parts alone did not differ: %+v", plain)
	}
	// the window changing where neither picture is painted out is a change
	changed := &painted{screen: frame.clone(), out: slices.Clone(frame.out)}
	changed.paint(2200, 1300, 300, 250, 20)
	pc, err := c.Take(changed, crop)
	if err != nil {
		t.Fatal(err)
	}
	if ch := Delta(pb, pc); !c.Moved(ch) {
		t.Fatalf("a change of the window outside Oni's parts: %+v", ch)
	}
	// a clone shares nothing and compares the same
	cl := pb.Clone()
	cl.WholeOut[0] = !cl.WholeOut[0]
	if pb.WholeOut[0] == cl.WholeOut[0] {
		t.Fatal("Clone shares the masked cells")
	}
}
