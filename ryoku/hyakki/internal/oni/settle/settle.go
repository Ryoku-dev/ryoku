// Package settle tells whether the screen moved after a guide step was
// advanced. It keeps two coarse gray fingerprints of a picture, the whole
// picture and the crop round the pointed target, and compares two of them by
// their mean absolute difference: a fingerprint that changed by at least the
// spec's moveDelta has moved, one that changed by at most stillDelta is
// still, and anything between is neither. The crop lets a change the size of
// a pointer or a text caret show on the target while the whole picture stays
// still.
//
// It is pure: no capture, no clock, no goroutine. The caller takes the
// pictures, one frame each, and decides how many frames to watch.
package settle

import (
	"errors"
	"math"
	"slices"
	"time"

	"ryoku-rashin/internal/oni/feature"
	"ryoku-rashin/internal/oni/spec"
)

// Gray is a picture that can be reduced to a w×h gray fingerprint:
// row-major luma 0–255, each cell the mean of the pixels it covers. region,
// in fractions of the picture, picks the part to reduce; nil is the whole
// picture. A cleared picture, or a region covering none of it, gives nil.
// *capture.Shot is one.
type Gray interface {
	Gray(w, h int, region *feature.Box) []uint8
}

// Masker is a Gray that knows which cells of the same reduction hold the
// presenter's own pixels painted out rather than the window: nil when none
// do. *capture.Shot is one. Oni's strand and card move between the picture a
// step was read from and the frames after Next, so those cells are no sign
// that the window changed.
type Masker interface {
	Masked(w, h int, region *feature.Box) []bool
}

// Config holds the watcher's numbers; FromSpec fills it from the spec.
type Config struct {
	W, H       int           // fingerprint size in cells
	MoveDelta  float64       // a fingerprint whose delta reaches this has moved
	StillDelta float64       // a fingerprint whose delta is at most this is still
	Frame      time.Duration // the time between two fingerprints
	Nap        time.Duration // the wait used when no fingerprint can be taken
	MaxFrames  int           // the most fingerprints taken after one advance
}

func ms(v float64) time.Duration { return time.Duration(v * float64(time.Millisecond)) }

// FromSpec returns the watcher numbers from the spec's guide.settle section.
func FromSpec(sp *spec.OniSpec) Config {
	s := sp.Guide.Settle
	return Config{
		W:          s.WidthPx,
		H:          s.HeightPx,
		MoveDelta:  s.MoveDelta,
		StillDelta: s.StillDelta,
		Frame:      ms(s.FrameMs),
		Nap:        ms(s.NapMs),
		MaxFrames:  s.MaxFrames,
	}
}

// maxCells bounds a fingerprint, as capture bounds a gray reduction.
const maxCells = 1 << 20

var (
	// ErrNoPicture: the picture was cleared, or never held one.
	ErrNoPicture = errors.New("settle: no picture to take")
	// ErrBadCrop: the crop is not a finite box of positive size inside the
	// picture.
	ErrBadCrop = errors.New("settle: the crop covers no part of the picture")
	// ErrBadSize: the fingerprint size is not positive and bounded, or the
	// picture gave a fingerprint of another size.
	ErrBadSize = errors.New("settle: the fingerprint has the wrong size")
)

// Print is one picture's fingerprints: Whole over the whole picture, Crop
// over the crop. Each holds W*H cells. WholeOut and CropOut, when not nil,
// mark the cells that held the presenter's own pixels; Delta leaves them out.
type Print struct {
	Whole, Crop       []uint8
	WholeOut, CropOut []bool
}

// Clear zeroes both fingerprints.
func (p *Print) Clear() {
	clear(p.Whole)
	clear(p.Crop)
	p.Whole, p.Crop, p.WholeOut, p.CropOut = nil, nil, nil, nil
}

// Clone is a copy of p that shares no buffer with it.
func (p Print) Clone() Print {
	return Print{Whole: slices.Clone(p.Whole), Crop: slices.Clone(p.Crop), WholeOut: slices.Clone(p.WholeOut), CropOut: slices.Clone(p.CropOut)}
}

// Take fingerprints g and the crop of it. The cells are copied out and the
// buffers g handed over are zeroed, so the Print outlives the picture and no
// reduced pixels are left behind in it.
func (c Config) Take(g Gray, crop feature.Box) (Print, error) {
	n := c.W * c.H
	if c.W <= 0 || c.H <= 0 || n > maxCells {
		return Print{}, ErrBadSize
	}
	for _, v := range []float64{crop.X, crop.Y, crop.W, crop.H} {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return Print{}, ErrBadCrop
		}
	}
	if crop.W <= 0 || crop.H <= 0 {
		return Print{}, ErrBadCrop
	}
	whole := g.Gray(c.W, c.H, nil)
	defer clear(whole)
	if whole == nil {
		return Print{}, ErrNoPicture
	}
	part := g.Gray(c.W, c.H, &crop)
	defer clear(part)
	if part == nil {
		return Print{}, ErrBadCrop
	}
	if len(whole) != n || len(part) != n {
		return Print{}, ErrBadSize
	}
	p := Print{Whole: append([]uint8(nil), whole...), Crop: append([]uint8(nil), part...)}
	if m, ok := g.(Masker); ok {
		if out := m.Masked(c.W, c.H, nil); len(out) == n {
			p.WholeOut = out
		}
		if out := m.Masked(c.W, c.H, &crop); len(out) == n {
			p.CropOut = out
		}
	}
	return p, nil
}

// Change is how far one Print moved from another: the mean absolute
// difference of their cells, 0–255, for the whole picture and the crop.
type Change struct {
	Whole, Crop float64
}

// Delta compares two Prints over the cells neither marks as the
// presenter's own; with no such cell left a fingerprint did not move.
// Prints of different or zero size cannot be compared and count as fully
// changed.
func Delta(a, b Print) Change {
	return Change{Whole: mean(a.Whole, b.Whole, a.WholeOut, b.WholeOut), Crop: mean(a.Crop, b.Crop, a.CropOut, b.CropOut)}
}

func mean(a, b []uint8, aOut, bOut []bool) float64 {
	if len(a) == 0 || len(a) != len(b) {
		return math.Inf(1)
	}
	sum, n := 0, 0
	for i, v := range a {
		if i < len(aOut) && aOut[i] || i < len(bOut) && bOut[i] {
			continue
		}
		d := int(v) - int(b[i])
		if d < 0 {
			d = -d
		}
		sum += d
		n++
	}
	if n == 0 {
		return 0
	}
	return float64(sum) / float64(n)
}

// Moved says whether either fingerprint changed by at least MoveDelta.
func (c Config) Moved(ch Change) bool { return ch.Whole >= c.MoveDelta || ch.Crop >= c.MoveDelta }

// Still says whether both fingerprints changed by at most StillDelta.
func (c Config) Still(ch Change) bool { return ch.Whole <= c.StillDelta && ch.Crop <= c.StillDelta }
