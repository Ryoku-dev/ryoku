package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"image"
	"image/color"
	"image/png"
	"math/rand/v2"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

const clipFakeTSV = "level\tpage_num\tblock_num\tpar_num\tline_num\tword_num\tleft\ttop\twidth\theight\tconf\ttext\n" +
	"4\t1\t1\t1\t1\t0\t0\t0\t10\t10\t-1\t\n" +
	"5\t1\t1\t1\t1\t1\t0\t0\t10\t10\t96.5\tInvoice\n" +
	"5\t1\t1\t1\t1\t2\t0\t0\t10\t10\t91\t#4471\n" +
	"5\t1\t1\t1\t2\t1\t0\t0\t10\t10\t88\tDue\n" +
	"5\t1\t1\t1\t2\t2\t0\t0\t10\t10\t31\tgarbIe\n" +
	"5\t1\t1\t1\t2\t3\t0\t0\t10\t10\t90\tFriday\n" +
	"5\t1\t2\t1\t1\t1\t0\t0\t10\t10\t95\t|\n" +
	"5\t1\t2\t1\t1\t2\t0\t0\t10\t10\t93\t駅\n"

type clipReadCall struct {
	name string
	args []string
}

// clipReadFake stands in for tesseract, magick and zbarimg and records every command
type clipReadFake struct {
	mu    sync.Mutex
	calls []clipReadCall
	codes string
}

func useClipReadFake(t *testing.T) *clipReadFake {
	t.Helper()
	f := &clipReadFake{}
	run, look, window := runClipRead, clipReadLookPath, clipReadWindow
	t.Cleanup(func() { runClipRead, clipReadLookPath, clipReadWindow = run, look, window })
	clipReadLookPath = func(name string) (string, error) { return "/usr/bin/" + name, nil }
	runClipRead = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		f.mu.Lock()
		f.calls = append(f.calls, clipReadCall{name, append([]string(nil), args...)})
		codes := f.codes
		f.mu.Unlock()
		switch name {
		case "tesseract":
			if len(args) > 0 && args[0] == "--list-langs" {
				return []byte("List of available languages in \"/usr/share/tessdata/\" (2):\neng\nosd\n"), nil
			}
			return []byte(clipFakeTSV), nil
		case "magick":
			return nil, os.WriteFile(args[len(args)-1], []byte("enlarged"), 0o600)
		case "zbarimg":
			if codes == "" {
				return nil, errors.New("no code")
			}
			return []byte(`<barcodes xmlns="http://zbar.sourceforge.net/2008/barcode"><source><image><symbol type="QR-Code"><data>` + codes + `</data></symbol></image></source></barcodes>`), nil
		}
		return nil, errors.New("unexpected command " + name)
	}
	return f
}

// reads lists the files tesseract was asked to read, in order
func (f *clipReadFake) reads() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []string
	for _, c := range f.calls {
		if c.name == "tesseract" && len(c.args) > 0 && c.args[0] != "--list-langs" {
			out = append(out, c.args[0])
		}
	}
	return out
}

func (f *clipReadFake) ran(name string) []clipReadCall {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []clipReadCall
	for _, c := range f.calls {
		if c.name == name {
			out = append(out, c)
		}
	}
	return out
}

func newReadingClip(t *testing.T, on bool) *clipState {
	t.Helper()
	dir := t.TempDir()
	s := &clipState{topic: newStateTopic(), stateDir: dir, dataDir: filepath.Join(dir, "data"), cacheDir: filepath.Join(dir, "cache")}
	s.readOn = func() bool { return on }
	for _, d := range []string{s.dataDir, s.cacheDir} {
		if err := os.MkdirAll(d, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	return s
}

// screenPNG draws a flat page with a few dark strokes, the way a screenshot looks
func screenPNG(t *testing.T, w, h int, seed int64) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	bg := color.RGBA{uint8(240 - seed%40), 240, 245, 255}
	for i := 0; i < len(img.Pix); i += 4 {
		img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3] = bg.R, bg.G, bg.B, bg.A
	}
	for y := h / 8; y < h; y += h / 8 {
		for x := w / 10; x < w*9/10; x++ {
			img.Set(x, y, color.RGBA{20, 20, 30, 255})
		}
	}
	return encodePNG(t, img)
}

// photoPNG is grain: almost no two neighbouring pixels agree
func photoPNG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	r := rand.New(rand.NewPCG(7, 7))
	for i := range img.Pix {
		img.Pix[i] = uint8(r.Uint32())
	}
	for i := 3; i < len(img.Pix); i += 4 {
		img.Pix[i] = 255
	}
	return encodePNG(t, img)
}

func encodePNG(t *testing.T, img image.Image) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func clipWait(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(2 * time.Millisecond)
	}
}

func clipEntryAt(s *clipState, i int) clipEntry {
	s.mu.Lock()
	defer s.mu.Unlock()
	return *s.entries[i]
}

func clipIsReading(s *clipState) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.reading
}

// clipReadingFrame is the reading count of the last frame, -1 when the frame has none
func clipReadingFrame(t *testing.T, s *clipState) int {
	t.Helper()
	s.topic.mu.Lock()
	last := s.topic.last
	s.topic.mu.Unlock()
	var frame struct {
		Reading *int `json:"reading"`
	}
	if err := json.Unmarshal(last, &frame); err != nil {
		t.Fatalf("decode frame %s: %v", last, err)
	}
	if frame.Reading == nil {
		return -1
	}
	return *frame.Reading
}

// A closed clipboard reads nothing: pictures copied before the palette wakes the
// reader wait, newest first once it does, a picture copied while it is awake is
// read straight away, and once the window passes it stops and later copies wait.
func TestClipReaderRunsOnlyWhileAwake(t *testing.T) {
	fake := useClipReadFake(t)
	clipReadWindow = time.Minute
	s := newReadingClip(t, true)
	s.ingest("image/png", screenPNG(t, 900, 600, 1))
	s.ingest("image/png", screenPNG(t, 900, 600, 2))
	older, newer := clipEntryAt(s, 1), clipEntryAt(s, 0)
	time.Sleep(50 * time.Millisecond)
	if got := fake.reads(); len(got) != 0 {
		t.Fatalf("read %v before the clipboard was in use", got)
	}

	s.wake()
	clipWait(t, "both pictures read", func() bool { return clipEntryAt(s, 1).Read && clipEntryAt(s, 0).Read })
	reads := fake.reads()
	if len(reads) != 2 || reads[0] != s.dataPath(newer.hash) || reads[1] != s.dataPath(older.hash) {
		t.Fatalf("read order = %v, want the newest picture first", reads)
	}
	if got := clipEntryAt(s, 0).text; got != "Invoice #4471\nDue Friday\n駅" {
		t.Errorf("read text = %q", got)
	}

	s.ingest("image/png", screenPNG(t, 900, 600, 3))
	clipWait(t, "a picture copied while awake to be read", func() bool { return clipEntryAt(s, 0).Read })

	clipReadWindow = 50 * time.Millisecond
	s.wake()
	clipWait(t, "the reader to stop after its window", func() bool { return !clipIsReading(s) })
	if n := clipReadingFrame(t, s); n != -1 {
		t.Errorf("frame still says reading %d after the reader stopped", n)
	}
	s.ingest("image/png", screenPNG(t, 900, 600, 4))
	time.Sleep(50 * time.Millisecond)
	if clipEntryAt(s, 0).Read || len(fake.reads()) != 3 {
		t.Errorf("a picture copied after the window was read: %v", fake.reads())
	}
}

// The frame counts the pictures still to read while the reader is awake, so the
// palette can say it is busy, and carries read on each picture that has been.
func TestClipReaderFrameCounts(t *testing.T) {
	useClipReadFake(t)
	clipReadWindow = time.Minute
	s := newReadingClip(t, true)
	s.ingest("image/png", screenPNG(t, 900, 600, 1))
	s.ingest("text/plain", []byte("not a picture"))
	if n := clipReadingFrame(t, s); n != -1 {
		t.Fatalf("frame says reading %d before any wake", n)
	}
	s.wake()
	clipWait(t, "the picture to be read", func() bool { return clipEntryAt(s, 1).Read })
	clipWait(t, "the frame to settle at zero", func() bool { return clipReadingFrame(t, s) == 0 })
	s.mu.Lock()
	s.readUntil = time.Now()
	s.mu.Unlock()
	poke(s.readKick)
	clipWait(t, "the reader to stop", func() bool { return !clipIsReading(s) })
	frame := clipFrame(t, s)
	if frame[1]["read"] != true || frame[0]["read"] != nil {
		t.Errorf("read flags = %v / %v, want true on the picture only", frame[1]["read"], frame[0]["read"])
	}
	if _, ok := frame[1]["text"]; ok {
		t.Error("the frame carries read text, which only the words call should hand out")
	}
}

// Screens are read at full size, small screens enlarged first, photos only from
// their thumbnail, and every picture is checked for a QR code.
func TestClipReaderGate(t *testing.T) {
	fake := useClipReadFake(t)
	fake.codes = "https://example.org/menu"
	clipReadWindow = time.Minute
	s := newReadingClip(t, true)
	s.ingest("image/png", photoPNG(t, 1600, 1000))
	s.ingest("image/png", screenPNG(t, 700, 300, 1))
	s.ingest("image/png", screenPNG(t, 1600, 1000, 2))
	screen, small, photo := clipEntryAt(s, 0), clipEntryAt(s, 1), clipEntryAt(s, 2)
	s.wake()
	clipWait(t, "all three read", func() bool { return clipEntryAt(s, 2).Read })

	reads := fake.reads()
	enlarged := filepath.Join(s.cacheDir, "ocr-"+strings.TrimSuffix(filepath.Base(s.dataPath(small.hash)), ".bin")+".png")
	want := []string{s.dataPath(screen.hash), enlarged, photo.ThumbPath}
	if len(reads) != 3 || reads[0] != want[0] || reads[1] != want[1] || reads[2] != want[2] {
		t.Fatalf("tesseract read %v, want %v", reads, want)
	}
	if mk := fake.ran("magick"); len(mk) != 1 || mk[0].args[0] != s.dataPath(small.hash) || mk[0].args[2] != "200%" {
		t.Errorf("magick ran %v, want one 200%% enlargement of the small screen", mk)
	}
	if _, err := os.Stat(enlarged); !os.IsNotExist(err) {
		t.Errorf("the enlarged copy outlived its read: %v", err)
	}
	if zb := fake.ran("zbarimg"); len(zb) != 3 {
		t.Errorf("zbarimg ran %d times, want once per picture", len(zb))
	}
	if got := clipEntryAt(s, 2).text; !strings.HasSuffix(got, "\nhttps://example.org/menu") {
		t.Errorf("photo text = %q, want the QR code's text after the words", got)
	}
}

func TestClipReadWordsKeepsConfidentWords(t *testing.T) {
	if got := clipReadWords([]byte(clipFakeTSV)); got != "Invoice #4471\nDue Friday\n駅" {
		t.Errorf("clipReadWords = %q", got)
	}
	if got := clipReadWords(nil); got != "" {
		t.Errorf("clipReadWords(nil) = %q", got)
	}
}

// Off means nothing runs and nothing read is kept: a wake starts no reader, and
// text read before the switch went off is forgotten, on disk too.
func TestClipReaderOffForgets(t *testing.T) {
	fake := useClipReadFake(t)
	clipReadWindow = time.Minute
	on := true
	s := newReadingClip(t, true)
	s.readOn = func() bool { return on }
	s.ingest("image/png", screenPNG(t, 900, 600, 1))
	s.wake()
	clipWait(t, "the picture to be read", func() bool { return clipEntryAt(s, 0).Read })
	id := clipEntryAt(s, 0).ID
	if w := s.words(); w[jsonID(id)] == "" {
		t.Fatalf("words = %v, want the picture's text", w)
	}

	s.mu.Lock()
	on = false
	s.mu.Unlock()
	s.publish()
	if !clipIsReading(s) || clipReadingFrame(t, s) != -1 {
		t.Error("the frame still says reading while the reader winds down after the switch went off")
	}
	s.mu.Lock()
	s.readUntil = time.Now()
	s.mu.Unlock()
	poke(s.readKick)
	clipWait(t, "the reader to stop", func() bool { return !clipIsReading(s) })
	calls := len(fake.reads())
	s.wake()
	if clipIsReading(s) {
		t.Error("a wake started the reader while it is off")
	}
	if e := clipEntryAt(s, 0); e.Read || e.text != "" {
		t.Errorf("entry still holds read text %q while off", e.text)
	}
	if w := s.words(); len(w) != 0 {
		t.Errorf("words = %v while off, want none", w)
	}
	raw, err := os.ReadFile(s.indexPath())
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(raw, []byte("Invoice")) {
		t.Error("the index on disk still holds the forgotten text")
	}
	s.ingest("image/png", screenPNG(t, 900, 600, 2))
	time.Sleep(30 * time.Millisecond)
	if len(fake.reads()) != calls {
		t.Error("a picture was read while the switch is off")
	}
}

func jsonID(id uint64) string {
	b, _ := json.Marshal(id)
	return string(b)
}

// What was read survives a restart and a repeat copy of the same picture, and a
// restart with the switch off forgets it.
func TestClipReadTextPersists(t *testing.T) {
	useClipReadFake(t)
	clipReadWindow = time.Minute
	s := newReadingClip(t, true)
	data := screenPNG(t, 900, 600, 1)
	s.ingest("image/png", data)
	s.wake()
	clipWait(t, "the picture to be read", func() bool { return clipEntryAt(s, 0).Read })
	s.ingest("text/plain", []byte("something else"))
	s.ingest("image/png", data)
	if e := clipEntryAt(s, 0); !e.Read || e.text == "" {
		t.Fatalf("copying the picture again lost what was read: read=%v text=%q", e.Read, e.text)
	}

	again := &clipState{stateDir: s.stateDir, dataDir: s.dataDir, cacheDir: s.cacheDir, readOn: func() bool { return true }}
	again.load()
	if e := clipEntryAt(again, 0); !e.Read || e.text != "Invoice #4471\nDue Friday\n駅" {
		t.Errorf("after a restart read=%v text=%q", e.Read, e.text)
	}

	off := &clipState{stateDir: s.stateDir, dataDir: s.dataDir, cacheDir: s.cacheDir, readOn: func() bool { return false }}
	off.load()
	if e := clipEntryAt(off, 0); e.Read || e.text != "" {
		t.Errorf("a restart with the switch off kept read=%v text=%q", e.Read, e.text)
	}
	raw, err := os.ReadFile(s.indexPath())
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(raw, []byte("Invoice")) {
		t.Error("the index on disk still holds text after a restart with the switch off")
	}
}

// Without tesseract the reader gives up for the daemon's life instead of
// waking every minute to fail again, and leaves the pictures unread.
func TestClipReaderWithoutTesseract(t *testing.T) {
	fake := useClipReadFake(t)
	clipReadLookPath = func(name string) (string, error) { return "", errors.New(name + " not found") }
	clipReadWindow = time.Minute
	s := newReadingClip(t, true)
	s.ingest("image/png", screenPNG(t, 900, 600, 1))
	s.wake()
	clipWait(t, "the reader to give up", func() bool { return !clipIsReading(s) })
	s.wake()
	if clipIsReading(s) {
		t.Error("a later wake started the reader again without tesseract")
	}
	if clipEntryAt(s, 0).Read || len(fake.calls) != 0 {
		t.Errorf("ran %v without tesseract", fake.calls)
	}
}

// Reading must never compete with the desktop: every command runs at the
// lowest priority on one thread.
func TestClipReadRunsIdleOnOneThread(t *testing.T) {
	out, err := runClipRead(context.Background(), "sh", "-c", `echo "$OMP_THREAD_LIMIT $(cut -d' ' -f19 /proc/$$/stat)"`)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(string(out)); got != "1 19" {
		t.Errorf("thread limit and niceness = %q, want \"1 19\"", got)
	}
}
