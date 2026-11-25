//go:build ocrengine

// Real-reader checks run with -tags ocrengine where tesseract and its eng
// data are installed. Synthetic pictures reach tesseract on stdin only.
package ocr

import (
	"bytes"
	"context"
	"image"
	"image/png"
	"math"
	"os"
	"strings"
	"testing"
	"unicode"

	"github.com/go-text/typesetting/font"
	ot "github.com/go-text/typesetting/font/opentype"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/vector"
)

type placed struct {
	text string
	x, y float32 // pen start on the baseline, px
}

// ink draws text at px onto a w×h coverage raster and returns it.
func ink(t *testing.T, face *font.Face, w, h int, px float32, words ...placed) *image.Alpha {
	t.Helper()
	k := px / float32(face.Upem())
	z := vector.NewRasterizer(w, h)
	for _, p := range words {
		pen := p.x
		for _, r := range p.text {
			gid, ok := face.NominalGlyph(r)
			if !ok {
				t.Fatalf("no glyph for %q", r)
			}
			outline, _ := face.GlyphDataOutline(gid)
			at := func(q ot.SegmentPoint) (float32, float32) { return pen + q.X*k, p.y - q.Y*k }
			open := false
			for _, s := range outline.Segments {
				switch s.Op {
				case ot.SegmentOpMoveTo:
					if open {
						z.ClosePath()
					}
					z.MoveTo(at(s.Args[0]))
					open = true
				case ot.SegmentOpLineTo:
					z.LineTo(at(s.Args[0]))
				case ot.SegmentOpQuadTo:
					bx, by := at(s.Args[0])
					cx, cy := at(s.Args[1])
					z.QuadTo(bx, by, cx, cy)
				case ot.SegmentOpCubeTo:
					bx, by := at(s.Args[0])
					cx, cy := at(s.Args[1])
					dx, dy := at(s.Args[2])
					z.CubeTo(bx, by, cx, cy, dx, dy)
				}
			}
			if open {
				z.ClosePath()
			}
			pen += face.HorizontalAdvance(gid) * k
		}
	}
	a := image.NewAlpha(image.Rect(0, 0, w, h))
	z.Draw(a, a.Bounds(), image.Opaque, image.Point{})
	return a
}

// bounds is the box of the pixels at least half covered: what a reader
// that thresholds the picture sees as ink.
func bounds(a *image.Alpha) image.Rectangle {
	var r image.Rectangle
	for y := range a.Rect.Dy() {
		for x := range a.Rect.Dx() {
			if a.AlphaAt(x, y).A >= 128 {
				r = r.Union(image.Rect(x, y, x+1, y+1))
			}
		}
	}
	return r
}

func TestRealTesseractFindsDrawnWords(t *testing.T) {
	checkRealTesseractWords(t, 640, 240, 26, false, []placed{
		{"File", 30, 60},
		{"Save", 160, 60},
		{"Export", 320, 60},
		{"Settings", 40, 170},
		{"Account", 300, 170},
	})
}

func TestRealTesseractReadsSmallWordsAtCaptureEdges(t *testing.T) {
	checkRealTesseractWords(t, 640, 240, 13, true, []placed{
		{"File", 0, 13},
		{"Save", 160, 11},
		{"Export", 320, 11},
		{"Settings", 40, 233},
		{"Account", 590, 170},
	})
}

func TestRealTesseractRetainsDistantButton(t *testing.T) {
	if !Available() {
		t.Skip("tesseract is not installed")
	}
	pic, err := os.ReadFile("testdata/nested-typed.png")
	if err != nil {
		t.Fatal(err)
	}
	// The private GTK capture has a small button far below the text being
	// typed; enlarging it must not make that visible control disappear.
	words, err := New(Exec([]string{"PATH=/usr/bin:/bin", "LANG=C.UTF-8",
		"TMPDIR=" + t.TempDir(), "XDG_RUNTIME_DIR=" + t.TempDir(), "OMP_THREAD_LIMIT=1"}), "eng").Words(context.Background(), pic)
	if err != nil {
		t.Fatal(err)
	}
	for _, word := range words {
		if word.Text == "Mark" && word.Box.X > 0.48 && word.Box.X < 0.52 &&
			word.Box.Y > 0.95 && word.Box.Y+word.Box.H <= 1 {
			return
		}
	}
	t.Fatalf("visible bottom button was not read at its position: %+v", words)
}

func TestRealTesseractReadsTextBesideCaret(t *testing.T) {
	if !Available() {
		t.Skip("tesseract is not installed")
	}
	for _, file := range []string{"caret-on.png", "caret-fading.png", "caret-off.png"} {
		t.Run(file, func(t *testing.T) {
			pic, err := os.ReadFile("testdata/" + file)
			if err != nil {
				t.Fatal(err)
			}
			words, err := New(Exec([]string{"PATH=/usr/bin:/bin", "LANG=C.UTF-8",
				"TMPDIR=" + t.TempDir(), "XDG_RUNTIME_DIR=" + t.TempDir(), "OMP_THREAD_LIMIT=1"}), "eng").Words(context.Background(), pic)
			if err != nil {
				t.Fatal(err)
			}
			found := make(map[string]bool)
			for _, word := range words {
				for _, field := range strings.Fields(word.Text) {
					token := strings.Map(func(r rune) rune {
						if unicode.IsLetter(r) || unicode.IsDigit(r) {
							return unicode.ToLower(r)
						}
						return -1
					}, field)
					found[token] = true
				}
			}
			for _, want := range []string{"hello", "lantern", "mark"} {
				if !found[want] {
					t.Errorf("visible word %q was not readable beside the caret: %+v", want, words)
				}
			}
		})
	}
}

func checkRealTesseractWords(t *testing.T, w, h int, px float32, dark bool, words []placed) {
	t.Helper()
	if !Available() {
		t.Skip("tesseract is not installed")
	}
	face, err := font.ParseTTF(bytes.NewReader(goregular.TTF))
	if err != nil {
		t.Fatal(err)
	}
	cover := ink(t, face, w, h, px, words...)
	pic := image.NewRGBA(image.Rect(0, 0, w, h))
	for i, c := range cover.Pix {
		v := 255 - c
		if dark {
			v = c
		}
		pic.Pix[4*i], pic.Pix[4*i+1], pic.Pix[4*i+2], pic.Pix[4*i+3] = v, v, v, 255
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, pic); err != nil {
		t.Fatal(err)
	}

	tmp, run := t.TempDir(), t.TempDir()
	env := []string{"PATH=/usr/bin:/bin", "LANG=C.UTF-8", "TMPDIR=" + tmp, "XDG_RUNTIME_DIR=" + run, "OMP_THREAD_LIMIT=1"}
	original := bytes.Clone(buf.Bytes())
	got, err := New(Exec(env), "eng").Words(context.Background(), buf.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(buf.Bytes(), original) {
		t.Fatal("recognition destroyed the caller's original picture")
	}
	for _, want := range words {
		truth := bounds(ink(t, face, w, h, px, want))
		found := false
		for _, g := range got {
			if g.Text != want.text {
				continue
			}
			found = true
			fw, fh := float64(w), float64(h)
			edges := [4][2]float64{
				{g.Box.X * fw, float64(truth.Min.X)},
				{g.Box.Y * fh, float64(truth.Min.Y)},
				{(g.Box.X + g.Box.W) * fw, float64(truth.Max.X)},
				{(g.Box.Y + g.Box.H) * fh, float64(truth.Max.Y)},
			}
			for _, e := range edges {
				if math.Abs(e[0]-e[1]) > 1+1e-9 {
					t.Errorf("%q at %+v (px %.2f,%.2f %.2fx%.2f), ink %v", want.text, g.Box,
						g.Box.X*fw, g.Box.Y*fh, g.Box.W*fw, g.Box.H*fh, truth)
					break
				}
			}
		}
		if !found {
			t.Errorf("%q not read; read %+v", want.text, got)
		}
	}
	for _, dir := range []string{tmp, run} {
		if left := tree(t, dir); len(left) != 0 {
			t.Errorf("%s gained %v", dir, left)
		}
	}
}
