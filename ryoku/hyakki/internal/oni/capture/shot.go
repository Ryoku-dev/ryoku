package capture

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"math"
	"sync"

	xdraw "golang.org/x/image/draw"

	"ryoku-rashin/internal/oni/feature"
	"ryoku-rashin/internal/oni/window"
)

// MaxPixels bounds a decoded picture: an 8K screen fits, a crafted PNG that
// claims more is refused before a single pixel is allocated.
const MaxPixels = 7680 * 4320

// Encodings the screen-ask contract fixes: the picture sent with a pointing
// ask, and the smaller one sent with an answer-mode ask.
const (
	AskLongSide    = 1568
	AskQuality     = 80
	AnswerLongSide = 1280
	AnswerQuality  = 78
)

// Shot is one picture in memory. Every buffer it hands out, the JPEG bytes
// and thumbnails included, is zeroed by Clear. masked are the pixel
// rectangles painted white over the presenter's own pixels.
type Shot struct {
	mu     sync.Mutex
	img    *image.RGBA
	held   [][]byte
	masked []image.Rectangle
}

// Size is the picture's pixel size; zero after Clear.
func (s *Shot) Size() image.Point {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.img == nil {
		return image.Point{}
	}
	return s.img.Rect.Size()
}

// JPEG encodes the picture with its long side at most longSide pixels, never
// upscaled, at the given quality.
func (s *Shot) JPEG(longSide, quality int) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.img == nil {
		return nil, ErrCleared
	}
	scaled := s.scaled(longSide)
	var buf bytes.Buffer
	buf.Grow(len(scaled.Pix) / 8)
	err := jpeg.Encode(&buf, scaled, &jpeg.Options{Quality: quality})
	if scaled != s.img {
		zero(scaled.Pix)
	}
	out := buf.Bytes()
	if err != nil {
		zero(out)
		return nil, err
	}
	s.held = append(s.held, out)
	return out, nil
}

// Thumb returns the picture with its long side at most maxPx, for the preview
// the person sees before anything is sent. It is nil after Clear.
func (s *Shot) Thumb(maxPx int) image.Image {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.img == nil {
		return nil
	}
	t := s.scaled(maxPx)
	if t == s.img {
		t = &image.RGBA{Pix: append([]byte(nil), s.img.Pix...), Stride: s.img.Stride, Rect: s.img.Rect}
	}
	s.held = append(s.held, t.Pix)
	return t
}

// PNG encodes the whole picture losslessly at its own size, for a reader
// that needs every pixel, such as the text reader. The bytes are held and
// zeroed by Clear; growing copies are zeroed as they are left behind.
func (s *Shot) PNG() ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.img == nil {
		return nil, ErrCleared
	}
	out := &bounded{max: MaxStdout, stop: func() {}}
	if err := (&png.Encoder{CompressionLevel: png.BestSpeed}).Encode(out, s.img); err != nil {
		out.clear()
		return nil, err
	}
	s.held = append(s.held, out.buf[:cap(out.buf)])
	return out.buf, nil
}

// MaxGray bounds the cells one Gray call may ask for.
const MaxGray = 1 << 20

// Gray reduces the picture, or the part of it region names, to w×h luma
// cells, row-major (index y*w+x). region is in fractions of the picture, as
// screen-ask boxes are, and is clamped to it; nil is the whole picture. Each
// cell is the area-weighted mean of the pixels it covers, with BT.601 luma
// rounded to 0..255. The cells are held and zeroed by Clear. Gray returns
// nil after Clear, for a size outside 1..MaxGray cells, or for a region that
// is not finite or covers no pixel.
func (s *Shot) Gray(w, h int, region *feature.Box) []uint8 {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.img == nil || w <= 0 || h <= 0 || w > MaxGray/h {
		return nil
	}
	x0, y0, x1, y1, ok := s.area(region)
	if !ok {
		return nil
	}
	cols := spans(x0, x1, w)
	sum := make([]float64, w*h)
	row := make([]float64, w)
	for j, rs := range spans(y0, y1, h) {
		for _, ry := range rs {
			// row holds, per cell column, the weighted luma of source row ry
			off := ry.at * s.img.Stride
			for i, cs := range cols {
				var v float64
				for _, cx := range cs {
					p := s.img.Pix[off+cx.at*4:]
					v += cx.weight * float64(299*int(p[0])+587*int(p[1])+114*int(p[2]))
				}
				row[i] = v
			}
			for i := range w {
				sum[j*w+i] += ry.weight * row[i]
			}
		}
	}
	area := (x1 - x0) / float64(w) * (y1 - y0) / float64(h) * 1000
	out := make([]uint8, w*h)
	for i, v := range sum {
		out[i] = uint8(min(max(math.Round(v/area), 0), 255))
	}
	clear(sum)
	clear(row)
	s.held = append(s.held, out)
	return out
}

// Masked marks the cells of the w×h reduction Gray makes of region that a
// painted mask touches: those cells hold Oni's own white, not the window,
// so a comparison of two pictures leaves them out. It is nil when nothing
// was masked, after Clear, and where Gray gives nil.
func (s *Shot) Masked(w, h int, region *feature.Box) []bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.img == nil || len(s.masked) == 0 || w <= 0 || h <= 0 || w > MaxGray/h {
		return nil
	}
	x0, y0, x1, y1, ok := s.area(region)
	if !ok {
		return nil
	}
	sx, sy := (x1-x0)/float64(w), (y1-y0)/float64(h)
	out := make([]bool, w*h)
	for _, r := range s.masked {
		// the cells whose span overlaps r, by the same split Gray uses
		i0 := max(int(math.Floor((float64(r.Min.X)-x0)/sx)), 0)
		i1 := min(int(math.Ceil((float64(r.Max.X)-x0)/sx)), w)
		j0 := max(int(math.Floor((float64(r.Min.Y)-y0)/sy)), 0)
		j1 := min(int(math.Ceil((float64(r.Max.Y)-y0)/sy)), h)
		for j := j0; j < j1; j++ {
			for i := i0; i < i1; i++ {
				out[j*w+i] = true
			}
		}
	}
	return out
}

// area is region of the picture in pixels, clamped to it, as Gray reduces
// it; ok is false for a region that is not finite or covers no pixel. The
// caller holds s.mu.
func (s *Shot) area(region *feature.Box) (x0, y0, x1, y1 float64, ok bool) {
	b := s.img.Rect
	pw, ph := float64(b.Dx()), float64(b.Dy())
	x0, y0, x1, y1 = 0, 0, pw, ph
	if region != nil {
		for _, v := range []float64{region.X, region.Y, region.W, region.H} {
			if math.IsNaN(v) || math.IsInf(v, 0) {
				return 0, 0, 0, 0, false
			}
		}
		x0 = min(max(region.X*pw, 0), pw)
		y0 = min(max(region.Y*ph, 0), ph)
		x1 = min(max((region.X+region.W)*pw, 0), pw)
		y1 = min(max((region.Y+region.H)*ph, 0), ph)
	}
	return x0, y0, x1, y1, x1 > x0 && y1 > y0
}

// tap is one source pixel's share of a cell: its index from the picture's
// origin and the length of it the cell covers.
type tap struct {
	at     int
	weight float64
}

// spans splits [lo, hi) into n equal cells and lists, for each, the whole
// pixels it touches and how much of each.
func spans(lo, hi float64, n int) [][]tap {
	out := make([][]tap, n)
	step := (hi - lo) / float64(n)
	for i := range n {
		a, z := lo+float64(i)*step, lo+float64(i+1)*step
		if i == n-1 {
			z = hi
		}
		for px := int(math.Floor(a)); float64(px) < z; px++ {
			if wt := min(float64(px+1), z) - max(float64(px), a); wt > 0 {
				out[i] = append(out[i], tap{px, wt})
			}
		}
	}
	return out
}

// Clear zeroes the picture and every buffer handed out from it. It is safe
// to call more than once.
func (s *Shot) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.img != nil {
		zero(s.img.Pix)
		s.img = nil
	}
	for _, b := range s.held {
		zero(b)
	}
	s.held = nil
	s.masked = nil
}

// scaled returns s.img itself when it already fits, else a fresh copy whose
// long side is longSide. The caller holds s.mu.
func (s *Shot) scaled(longSide int) *image.RGBA {
	b := s.img.Rect
	w, h := b.Dx(), b.Dy()
	long := max(w, h)
	if longSide <= 0 || long <= longSide {
		return s.img
	}
	f := float64(longSide) / float64(long)
	nw := max(1, int(math.Round(float64(w)*f)))
	nh := max(1, int(math.Round(float64(h)*f)))
	dst := image.NewRGBA(image.Rect(0, 0, nw, nh))
	xdraw.CatmullRom.Scale(dst, dst.Rect, s.img, b, xdraw.Src, nil)
	return dst
}

// mask paints every mask white. region is the grabbed area and masks are
// in the same output-local logical pixels; the picture may be at a higher
// scale, so each mask is mapped and grown to whole pixels.
func (s *Shot) mask(region window.Box, masks []window.Box) {
	if len(masks) == 0 || region.W <= 0 || region.H <= 0 {
		return
	}
	b := s.img.Rect
	sx := float64(b.Dx()) / region.W
	sy := float64(b.Dy()) / region.H
	white := image.NewUniform(color.RGBA{255, 255, 255, 255})
	for _, m := range masks {
		m.X -= region.X
		m.Y -= region.Y
		r := image.Rect(
			int(math.Floor(m.X*sx)), int(math.Floor(m.Y*sy)),
			int(math.Ceil((m.X+m.W)*sx)), int(math.Ceil((m.Y+m.H)*sy)),
		).Add(b.Min).Intersect(b)
		if !r.Empty() {
			xdraw.Draw(s.img, r, white, image.Point{}, xdraw.Src)
			s.masked = append(s.masked, r.Sub(b.Min))
		}
	}
}

// FromPNG is the picture in raw, a PNG some other path took (the Hands
// tab's own viewport), with the same bounds and clearing as a capture. raw
// is zeroed.
func FromPNG(raw []byte) (*Shot, error) { return decode(raw) }

// decode reads one PNG, refusing it before allocation when it claims more
// than MaxPixels, and zeroes raw afterwards.
func decode(raw []byte) (*Shot, error) {
	defer zero(raw)
	cfg, err := png.DecodeConfig(bytes.NewReader(raw))
	if err != nil {
		return nil, ErrBadImage
	}
	if cfg.Width <= 0 || cfg.Height <= 0 {
		return nil, ErrBadImage
	}
	if cfg.Width > MaxPixels/cfg.Height {
		return nil, ErrTooLarge
	}
	src, err := png.Decode(bytes.NewReader(raw))
	if err != nil {
		return nil, ErrBadImage
	}
	if rgba, ok := src.(*image.RGBA); ok {
		return &Shot{img: rgba}, nil
	}
	dst := image.NewRGBA(image.Rect(0, 0, cfg.Width, cfg.Height))
	xdraw.Draw(dst, dst.Rect, src, src.Bounds().Min, xdraw.Src)
	zeroImage(src)
	return &Shot{img: dst}, nil
}

func zeroImage(m image.Image) {
	switch p := m.(type) {
	case *image.NRGBA:
		zero(p.Pix)
	case *image.RGBA64:
		zero(p.Pix)
	case *image.NRGBA64:
		zero(p.Pix)
	case *image.Gray:
		zero(p.Pix)
	case *image.Gray16:
		zero(p.Pix)
	case *image.Paletted:
		zero(p.Pix)
	}
}

func zero(b []byte) { clear(b) }
