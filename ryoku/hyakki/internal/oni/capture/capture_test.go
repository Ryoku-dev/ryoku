package capture

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"hash/crc32"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"ryoku-rashin/internal/oni/feature"
	"ryoku-rashin/internal/oni/window"
)

// fakeGrim stands in for grim: it never touches a display. It checks every
// argv against the allowlist, records it, and answers with a PNG sized like
// the grab at the given scale.
type fakeGrim struct {
	t       *testing.T
	scale   int
	calls   [][]string
	failTop bool
	// returned keeps every buffer handed back so tests can see it zeroed.
	returned [][]byte
	// next, when set, is returned instead of a rendered PNG.
	next []byte
}

func (f *fakeGrim) run(ctx context.Context, name string, args ...string) ([]byte, error) {
	f.t.Helper()
	if !Allowed(name, args...) {
		f.t.Fatalf("argv outside the allowlist: %s %q", name, args)
	}
	f.calls = append(f.calls, append([]string{name}, args...))
	if f.next != nil {
		b := f.next
		f.next = nil
		f.returned = append(f.returned, b)
		return b, nil
	}
	w, h := 640, 480
	switch args[0] {
	case "-T":
		if f.failTop {
			return nil, errors.New("grim: compositor does not support toplevel capture")
		}
	case "-g":
		var x, y int
		if _, err := fmt.Sscanf(args[1], "%d,%d %dx%d", &x, &y, &w, &h); err != nil {
			f.t.Fatal(err)
		}
		w, h = w*f.scale, h*f.scale
	}
	b := pngOf(f.t, pattern(w, h))
	f.returned = append(f.returned, b)
	return b, nil
}

func pattern(w, h int) *image.RGBA {
	m := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			m.SetRGBA(x, y, color.RGBA{uint8(x), uint8(y), 90, 255})
		}
	}
	return m
}

func pngOf(t *testing.T, m image.Image) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := (&png.Encoder{CompressionLevel: png.BestSpeed}).Encode(&buf, m); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

var all = []Path{PathToplevel, PathNiriWindow, PathOutputCrop, PathPortal, PathX11}

// Two outputs side by side; the target is a focused tiled window on the right one.
func layout(windows ...window.Window) func(context.Context) (window.Snapshot, error) {
	return func(context.Context) (window.Snapshot, error) {
		return window.Snapshot{
			Outputs: []window.Output{
				{Name: "A", X: 0, Y: 0, W: 1920, H: 1080, Scale: 1},
				{Name: "B", X: 1920, Y: 0, W: 1280, H: 800, Scale: 2},
			},
			Windows: windows,
		}, nil
	}
}

func target() window.Window {
	return window.Window{ID: "7", App: "fixture", Title: "Fixture", Output: "B",
		Box: &window.Box{X: 100, Y: 50, W: 400, H: 300}, Focused: true, Visible: true}
}

func resolver(id string) Option {
	return Toplevel(func(context.Context, window.Window) (string, error) { return id, nil })
}

func TestProbeListsTheLadderInOrder(t *testing.T) {
	f := &fakeGrim{t: t, scale: 1}
	s := New(f.run, all, resolver("ext-1"), Layout(layout()))
	got := s.Probe(context.Background())
	var paths []Path
	var offered []bool
	for _, c := range got {
		paths = append(paths, c.Path)
		offered = append(offered, c.Offered)
		if c.Offered != (c.Reason == "") {
			t.Errorf("%s: offered %v with reason %q", c.Path, c.Offered, c.Reason)
		}
	}
	if !slices.Equal(paths, Ladder) {
		t.Fatalf("order %v, want %v", paths, Ladder)
	}
	if want := []bool{true, false, true, false, false}; !slices.Equal(offered, want) {
		t.Fatalf("offered %v, want %v", offered, want)
	}
	top, crop := got[0], got[2]
	if !top.ScopeExact || top.MasksNeeded || top.WholeOutputInMemory {
		t.Errorf("toplevel capability %+v", top)
	}
	if crop.ScopeExact || !crop.MasksNeeded || !crop.WholeOutputInMemory {
		t.Errorf("crop capability %+v", crop)
	}
	if len(f.calls) != 0 {
		t.Errorf("Probe started %v", f.calls)
	}

	bare := New(f.run, all).Probe(context.Background())
	if bare[0].Offered || bare[2].Offered {
		t.Errorf("toplevel and crop offered without a resolver or layout: %+v", bare)
	}
	hidden := New(f.run, []Path{PathOutputCrop}, resolver("ext-1"), Layout(layout())).Probe(context.Background())
	if hidden[0].Offered || hidden[0].Reason == "" || !hidden[2].Offered {
		t.Errorf("a path left out of the allowed set must not be offered: %+v", hidden)
	}
}

func TestToplevelIsPreferredAndExact(t *testing.T) {
	f := &fakeGrim{t: t, scale: 2}
	s := New(f.run, all, resolver("ext-1"), Layout(layout()))
	shot, d, err := s.Capture(context.Background(), target(), []window.Box{{X: 100, Y: 50, W: 50, H: 50}})
	if err != nil {
		t.Fatal(err)
	}
	defer shot.Clear()
	if d.Path != PathToplevel || !d.ScopeExact || d.Text == "" {
		t.Fatalf("disclosure %+v", d)
	}
	if len(f.calls) != 1 || f.calls[0][1] != "-T" || f.calls[0][2] != "ext-1" {
		t.Fatalf("calls %q", f.calls)
	}
	// The toplevel picture holds only the window; no mask is painted on it.
	if c := shot.img.RGBAAt(0, 0); c == (color.RGBA{255, 255, 255, 255}) {
		t.Fatalf("toplevel picture was masked")
	}
}

func TestHidingToplevelFallsBackToTheCropAndSaysSo(t *testing.T) {
	f := &fakeGrim{t: t, scale: 2}
	top := New(f.run, all, resolver("ext-1"), Layout(layout()))
	crop := New(f.run, []Path{PathOutputCrop, PathPortal, PathX11}, resolver("ext-1"), Layout(layout()))

	s1, d1, err := top.Capture(context.Background(), target(), nil)
	if err != nil {
		t.Fatal(err)
	}
	s1.Clear()
	f.calls = nil
	s2, d2, err := crop.Capture(context.Background(), target(), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer s2.Clear()
	if d2.Path != PathOutputCrop || d2.ScopeExact {
		t.Fatalf("crop disclosure %+v", d2)
	}
	if d1.Text == d2.Text {
		t.Fatalf("both paths disclose %q", d1.Text)
	}
	if !strings.Contains(d2.Text, "region") || !strings.Contains(d2.Text, "Anything drawn over") {
		t.Errorf("crop disclosure does not name the region cost: %q", d2.Text)
	}
	// Output B starts at x=1920: the region is in layout coordinates.
	want := []string{"grim", "-g", "2020,50 400x300", "-t", "png", "-l", "1", "-"}
	if len(f.calls) != 1 || !slices.Equal(f.calls[0], want) {
		t.Fatalf("calls %q, want %q", f.calls, want)
	}
	if got := s2.Size(); got != image.Pt(800, 600) {
		t.Fatalf("size %v", got)
	}
}

func TestToplevelFailureFallsToTheCrop(t *testing.T) {
	f := &fakeGrim{t: t, scale: 1, failTop: true}
	s := New(f.run, all, resolver("ext-1"), Layout(layout()))
	shot, d, err := s.Capture(context.Background(), target(), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer shot.Clear()
	if d.Path != PathOutputCrop || len(f.calls) != 2 {
		t.Fatalf("disclosure %+v calls %q", d, f.calls)
	}

	// An identifier grim could read as a flag never reaches grim.
	f = &fakeGrim{t: t, scale: 1}
	s = New(f.run, all, resolver("-o HDMI-1"), Layout(layout()))
	shot, d, err = s.Capture(context.Background(), target(), nil)
	if err != nil {
		t.Fatal(err)
	}
	shot.Clear()
	if d.Path != PathOutputCrop || len(f.calls) != 1 || f.calls[0][1] != "-g" {
		t.Fatalf("disclosure %+v calls %q", d, f.calls)
	}
}

func TestTheCropRefusesWhatItCannotScope(t *testing.T) {
	floating := func(w window.Window) window.Window { w.Floating = true; return w }
	other := window.Window{ID: "9", Output: "B", Visible: true, Floating: true,
		Box: &window.Box{X: 450, Y: 300, W: 200, H: 200}}
	cases := []struct {
		name   string
		w      window.Window
		others []window.Window
		want   error
	}{
		{"unfocused", func() window.Window { w := target(); w.Focused = false; return w }(), nil, ErrNotFocused},
		{"floating over it", target(), []window.Window{other}, ErrOccluded},
		{"fullscreen on its output", target(), []window.Window{{ID: "3", Output: "B", Visible: true, Fullscreen: true}}, ErrOccluded},
		{"no geometry", func() window.Window { w := target(); w.Box = nil; return w }(), nil, ErrNoGeometry},
		{"minimized", func() window.Window { w := target(); w.Minimized = true; return w }(), nil, ErrHidden},
		{"not visible", func() window.Window { w := target(); w.Visible = false; return w }(), nil, ErrHidden},
		{"unknown output", func() window.Window { w := target(); w.Output = "C"; return w }(), nil, ErrOffOutput},
		{"off its output", func() window.Window { w := target(); w.Box = &window.Box{X: 2000, Y: 0, W: 100, H: 100}; return w }(), nil, ErrOffOutput},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := &fakeGrim{t: t, scale: 1}
			s := New(f.run, []Path{PathOutputCrop}, Layout(layout(append([]window.Window{c.w}, c.others...)...)))
			shot, _, err := s.Capture(context.Background(), c.w, nil)
			if !errors.Is(err, c.want) || shot != nil {
				t.Fatalf("err %v, want %v", err, c.want)
			}
			if len(f.calls) != 0 {
				t.Fatalf("grim ran: %q", f.calls)
			}
		})
	}

	// Not covered: a floating window elsewhere, one on another output, and a
	// floating target above another floating window.
	f := &fakeGrim{t: t, scale: 1}
	apart := other
	apart.Box = &window.Box{X: 900, Y: 600, W: 100, H: 100}
	elsewhere := other
	elsewhere.Output = "A"
	under := other
	tgt := floating(target())
	s := New(f.run, []Path{PathOutputCrop}, Layout(layout(tgt, apart, elsewhere, under)))
	shot, _, err := s.Capture(context.Background(), tgt, nil)
	if err != nil {
		t.Fatal(err)
	}
	shot.Clear()
}

func TestNoPathMeansNoGrab(t *testing.T) {
	f := &fakeGrim{t: t, scale: 1}
	for _, allowed := range [][]Path{nil, {PathNiriWindow, PathPortal, PathX11}} {
		s := New(f.run, allowed, resolver("ext-1"), Layout(layout()))
		if _, _, err := s.Capture(context.Background(), target(), nil); !errors.Is(err, ErrNoPath) {
			t.Fatalf("allowed %v: err %v", allowed, err)
		}
	}
	if len(f.calls) != 0 {
		t.Fatalf("grim ran: %q", f.calls)
	}
}

func TestMasksAreExactlyWhite(t *testing.T) {
	f := &fakeGrim{t: t, scale: 2}
	s := New(f.run, []Path{PathOutputCrop}, Layout(layout()))
	// The window sits at (100,50) on output B, grabbed at scale 2.
	mask := window.Box{X: 150, Y: 80, W: 40.5, H: 30}
	outside := window.Box{X: 900, Y: 700, W: 50, H: 50}
	shot, _, err := s.Capture(context.Background(), target(), []window.Box{mask, outside})
	if err != nil {
		t.Fatal(err)
	}
	defer shot.Clear()
	in := image.Rect(100, 60, 181, 120) // (150-100)*2 .. ceil((190.5-100)*2), (80-50)*2 .. (110-50)*2
	white := color.RGBA{255, 255, 255, 255}
	b := shot.img.Rect
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			got := shot.img.RGBAAt(x, y)
			if image.Pt(x, y).In(in) {
				if got != white {
					t.Fatalf("(%d,%d) inside the mask is %v", x, y, got)
				}
			} else if want := (color.RGBA{uint8(x), uint8(y), 90, 255}); got != want {
				t.Fatalf("(%d,%d) outside the mask is %v, want %v", x, y, got, want)
			}
		}
	}
}

// Masked names exactly the cells a reduction of the picture takes from
// painted-out pixels, for the whole picture and a crop, and nothing for a
// picture with no mask or after Clear.
func TestMaskedNamesTheCellsAMaskTouches(t *testing.T) {
	f := &fakeGrim{t: t, scale: 2}
	s := New(f.run, []Path{PathOutputCrop}, Layout(layout()))
	plain, _, err := s.Capture(context.Background(), target(), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer plain.Clear()
	if m := plain.Masked(8, 6, nil); m != nil {
		t.Fatalf("a picture with no mask names cells %v", m)
	}
	mask := window.Box{X: 150, Y: 80, W: 40.5, H: 30}
	shot, _, err := s.Capture(context.Background(), target(), []window.Box{mask})
	if err != nil {
		t.Fatal(err)
	}
	in := image.Rect(100, 60, 181, 120) // the painted pixels, as in TestMasksAreExactlyWhite
	b := shot.img.Rect
	pw, ph := float64(b.Dx()), float64(b.Dy())
	for _, c := range []struct {
		name   string
		region *feature.Box
	}{
		{"whole", nil},
		{"crop", &feature.Box{X: 0.1, Y: 0.05, W: 0.3, H: 0.4}},
	} {
		const w, h = 16, 12
		x0, y0, x1, y1 := 0.0, 0.0, pw, ph
		if r := c.region; r != nil {
			x0, y0, x1, y1 = r.X*pw, r.Y*ph, (r.X+r.W)*pw, (r.Y+r.H)*ph
		}
		got := shot.Masked(w, h, c.region)
		if len(got) != w*h {
			t.Fatalf("%s: %d cells", c.name, len(got))
		}
		marked := 0
		for j := range h {
			for i := range w {
				cx0, cx1 := x0+float64(i)*(x1-x0)/w, x0+float64(i+1)*(x1-x0)/w
				cy0, cy1 := y0+float64(j)*(y1-y0)/h, y0+float64(j+1)*(y1-y0)/h
				want := cx0 < float64(in.Max.X) && cx1 > float64(in.Min.X) && cy0 < float64(in.Max.Y) && cy1 > float64(in.Min.Y)
				if got[j*w+i] != want {
					t.Fatalf("%s: cell (%d,%d) masked %v, want %v", c.name, i, j, got[j*w+i], want)
				}
				if want {
					marked++
				}
			}
		}
		if marked == 0 {
			t.Fatalf("%s: the mask touched no cell", c.name)
		}
	}
	shot.Clear()
	if m := shot.Masked(16, 12, nil); m != nil {
		t.Fatal("a cleared picture still names masked cells")
	}
}

func TestClearZeroesEveryBuffer(t *testing.T) {
	f := &fakeGrim{t: t, scale: 1}
	s := New(f.run, []Path{PathOutputCrop}, Layout(layout()))
	shot, _, err := s.Capture(context.Background(), target(), nil)
	if err != nil {
		t.Fatal(err)
	}
	pix := shot.img.Pix
	big, err := shot.JPEG(AskLongSide, AskQuality)
	if err != nil {
		t.Fatal(err)
	}
	small, err := shot.JPEG(200, AnswerQuality)
	if err != nil {
		t.Fatal(err)
	}
	thumb := shot.Thumb(120).(*image.RGBA)
	full := shot.Thumb(0).(*image.RGBA)
	for _, b := range f.returned {
		if !allZero(b) {
			t.Fatal("grim's PNG bytes were kept after decoding")
		}
	}
	shot.Clear()
	shot.Clear()
	for name, b := range map[string][]byte{"pixels": pix, "jpeg": big, "small jpeg": small, "thumb": thumb.Pix, "full thumb": full.Pix} {
		if len(b) == 0 || !allZero(b) {
			t.Errorf("%s not zeroed (%d bytes)", name, len(b))
		}
	}
	if _, err := shot.JPEG(AskLongSide, AskQuality); !errors.Is(err, ErrCleared) {
		t.Errorf("JPEG after Clear: %v", err)
	}
	if shot.Thumb(100) != nil || shot.Size() != (image.Point{}) {
		t.Error("a cleared shot still hands out pixels")
	}
}

func allZero(b []byte) bool {
	for _, v := range b {
		if v != 0 {
			return false
		}
	}
	return true
}

func TestDecodeBombIsRefusedBeforeAllocation(t *testing.T) {
	small := pngOf(t, pattern(4, 4))
	for _, c := range []struct {
		name string
		raw  []byte
		want error
	}{
		{"ten gigapixels", withSize(small, 100000, 100000), ErrTooLarge},
		{"one column past 8K", withSize(small, 7681, 4320), ErrTooLarge},
		{"one row past 8K", withSize(small, 7680, 4321), ErrTooLarge},
		{"negative when signed", withSize(small, 1<<31, 1), ErrBadImage},
		{"garbage", []byte("not a picture"), ErrBadImage},
	} {
		f := &fakeGrim{t: t, scale: 1, next: c.raw}
		s := New(f.run, []Path{PathOutputCrop}, Layout(layout()))
		if shot, _, err := s.Capture(context.Background(), target(), nil); !errors.Is(err, c.want) || shot != nil {
			t.Errorf("%s: err %v, want %v", c.name, err, c.want)
		}
		if !allZero(c.raw) {
			t.Errorf("%s: refused bytes were kept", c.name)
		}
	}
}

// withSize rewrites a PNG's IHDR to claim another size, keeping its CRC valid.
func withSize(p []byte, w, h uint32) []byte {
	out := append([]byte(nil), p...)
	// signature(8) length(4) "IHDR"(4) data(13) crc(4)
	binary.BigEndian.PutUint32(out[16:], w)
	binary.BigEndian.PutUint32(out[20:], h)
	binary.BigEndian.PutUint32(out[29:], crc32.ChecksumIEEE(out[12:29]))
	return out
}

func TestJPEGSizesAndQualities(t *testing.T) {
	f := &fakeGrim{t: t, scale: 1, next: pngOf(t, pattern(3136, 1764))}
	s := New(f.run, []Path{PathOutputCrop}, Layout(layout()))
	shot, _, err := s.Capture(context.Background(), target(), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer shot.Clear()
	for _, c := range []struct {
		side, quality int
		want          image.Point
		dqt           [2]byte // the first two luminance entries, scaled by quality
	}{
		{AskLongSide, AskQuality, image.Pt(1568, 882), [2]byte{6, 4}},
		{AnswerLongSide, AnswerQuality, image.Pt(1280, 720), [2]byte{7, 5}},
		{4000, 80, image.Pt(3136, 1764), [2]byte{6, 4}},
	} {
		b, err := shot.JPEG(c.side, c.quality)
		if err != nil {
			t.Fatal(err)
		}
		cfg, err := jpeg.DecodeConfig(bytes.NewReader(b))
		if err != nil {
			t.Fatal(err)
		}
		if got := image.Pt(cfg.Width, cfg.Height); got != c.want {
			t.Errorf("JPEG(%d,%d) is %v, want %v", c.side, c.quality, got, c.want)
		}
		if got := luminanceTable(t, b); got != c.dqt {
			t.Errorf("JPEG(%d,%d) quantisation starts %v, want %v", c.side, c.quality, got, c.dqt)
		}
	}
	if th := shot.Thumb(320); th.Bounds().Size() != image.Pt(320, 180) {
		t.Errorf("thumb %v", th.Bounds())
	}
}

// luminanceTable returns the first two entries of quantisation table 0.
func luminanceTable(t *testing.T, b []byte) [2]byte {
	t.Helper()
	for i := 2; i+4 < len(b); {
		if b[i] != 0xff {
			t.Fatal("JPEG marker stream broken")
		}
		marker, n := b[i+1], int(binary.BigEndian.Uint16(b[i+2:]))
		if marker == 0xdb && b[i+4]&0x0f == 0 {
			return [2]byte{b[i+5], b[i+6]}
		}
		i += 2 + n
	}
	t.Fatal("no quantisation table")
	return [2]byte{}
}

func TestArgvAllowlist(t *testing.T) {
	ok := [][]string{
		{"grim", "-T", "ext-1", "-t", "png", "-l", "1", "-"},
		{"grim", "-g", "-10,20 30x40", "-t", "png", "-l", "1", "-"},
	}
	bad := [][]string{
		{"grim", "-t", "png", "-l", "1", "-"},
		{"grim", "-o", "HDMI-1", "-t", "png", "-l", "1", "-"},
		{"grim", "-g", "0,0 0x40", "-t", "png", "-l", "1", "-"},
		{"grim", "-g", "0,0 10x10 ", "-t", "png", "-l", "1", "-"},
		{"grim", "-T", "-c", "-t", "png", "-l", "1", "-"},
		{"grim", "-T", "a b", "-t", "png", "-l", "1", "-"},
		{"grim", "-T", "ext-1", "-t", "png", "-l", "1", "/tmp/shot.png"},
		{"grim", "-T", "ext-1", "-c", "png", "-l", "1", "-"},
		{"niri", "msg", "action", "screenshot-window", "-t", "png", "-"},
		{"grimshot", "-T", "ext-1", "-t", "png", "-l", "1", "-"},
	}
	for _, a := range ok {
		if !Allowed(a[0], a[1:]...) {
			t.Errorf("refused %q", a)
		}
	}
	for _, a := range bad {
		if Allowed(a[0], a[1:]...) {
			t.Errorf("allowed %q", a)
		}
	}
}

// Every capture path, every encoding and Clear leave no file behind in the
// temporary directory, the runtime directory or the working directory.
func TestNothingIsWrittenToDisk(t *testing.T) {
	tmp, run := t.TempDir(), t.TempDir()
	t.Setenv("TMPDIR", tmp)
	t.Setenv("XDG_RUNTIME_DIR", run)
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	before := tree(t, cwd)

	f := &fakeGrim{t: t, scale: 2, failTop: true}
	for _, s := range []Service{
		New(f.run, all, resolver("ext-1"), Layout(layout())),
		New(f.run, []Path{PathOutputCrop}, Layout(layout())),
	} {
		s.Probe(context.Background())
		shot, _, err := s.Capture(context.Background(), target(), []window.Box{{X: 120, Y: 60, W: 30, H: 30}})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := shot.JPEG(AskLongSide, AskQuality); err != nil {
			t.Fatal(err)
		}
		if _, err := shot.JPEG(AnswerLongSide, AnswerQuality); err != nil {
			t.Fatal(err)
		}
		shot.Thumb(160)
		shot.Clear()
	}
	f.failTop = false
	shot, _, err := New(f.run, all, resolver("ext-1"), Layout(layout())).Capture(context.Background(), target(), nil)
	if err != nil {
		t.Fatal(err)
	}
	shot.Clear()

	for _, dir := range []string{tmp, run} {
		if got := tree(t, dir); len(got) != 0 {
			t.Errorf("%s gained %v", dir, got)
		}
	}
	if after := tree(t, cwd); !slices.Equal(before, after) {
		t.Errorf("working directory changed: %v -> %v", before, after)
	}
}

func tree(t *testing.T, root string) []string {
	t.Helper()
	var out []string
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if p != root {
			out = append(out, p)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func TestExecRefusesBeforeStarting(t *testing.T) {
	run := Exec([]string{"PATH=/nonexistent"})
	for _, argv := range [][]string{
		{"sh", "-c", "touch /tmp/x"},
		{"grim", "-o", "HDMI-1", "-"},
		{"grim", "/tmp/shot.png"},
	} {
		if _, err := run(context.Background(), argv[0], argv[1:]...); !errors.Is(err, ErrNotAllowed) {
			t.Errorf("%q: %v", argv, err)
		}
	}
}

func TestStdoutIsBoundedAndZeroed(t *testing.T) {
	stopped := false
	b := &bounded{max: 10, stop: func() { stopped = true }}
	if n, err := b.Write([]byte("secret")); err != nil || n != 6 {
		t.Fatal(n, err)
	}
	held := b.buf[:cap(b.buf)]
	if _, err := b.Write([]byte("pixels")); !errors.Is(err, ErrTooLarge) || !b.over || !stopped {
		t.Fatalf("over the bound: err %v over %v stopped %v", err, b.over, stopped)
	}
	b.clear()
	if !allZero(held) || b.buf != nil {
		t.Fatal("bounded stdout kept its bytes")
	}
}
