package tts

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"ryoku-rashin/internal/oni/sound"
)

func flat(n int, v float32) []float32 {
	pcm := make([]float32, n)
	for i := range pcm {
		pcm[i] = v
	}
	return pcm
}

func cached(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, e := range entries {
		out = append(out, e.Name())
	}
	return out
}

// Building the voice creates nothing and starts nothing: no cache folder,
// no goroutine, no child process, until a line is spoken.
func TestNothingAtRest(t *testing.T) {
	home := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", home)
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	before := runtime.NumGoroutine()
	e := &fakeEngine{name: System, level: 0.5}
	c := NewCache("", 0, 0)
	if c.dir != filepath.Join(home, "hyakki", "oni", "voice") {
		t.Fatalf("cache folder %s", c.dir)
	}
	if _, ok := c.Get(e, "On it."); ok {
		t.Fatal("an empty cache hit")
	}
	NewSpeaker(Options{Engines: []Engine{e}, SpeakReplies: true, Cache: c, Open: refuseSink(t)})
	if _, err := NewSystem(SystemOptions{Path: executable(t, "espeak-ng")}); err != nil {
		t.Fatal(err)
	}
	if _, err := FindKokoro(KokoroOptions{Binary: executable(t, KokoroBinary)}); err == nil {
		t.Fatal("found a Kokoro pack in an empty data folder")
	}
	if got := cached(t, home); len(got) > 0 {
		t.Fatalf("created %q at rest", got)
	}
	if n := runtime.NumGoroutine(); n > before {
		t.Fatalf("%d goroutines at rest", n-before)
	}
	if len(e.rendered()) > 0 {
		t.Fatal("rendered at rest")
	}
}

func TestCacheBounds(t *testing.T) {
	e := &fakeEngine{name: System}
	other := &fakeEngine{name: Kokoro}
	dir := filepath.Join(t.TempDir(), "voice")
	c := NewCache(dir, 3, 0)
	put := func(line string, pcm []float32) {
		t.Helper()
		if err := c.Put(e, line, pcm); err != nil {
			t.Fatal(err)
		}
		time.Sleep(5 * time.Millisecond)
	}
	for i := range 3 {
		put("Line "+strconv.Itoa(i)+".", flat(100, float32(i)))
	}
	if info, err := os.Stat(dir); err != nil || info.Mode().Perm() != 0o700 {
		t.Fatalf("cache folder: %v %v", info, err)
	}
	if pcm, ok := c.Get(e, "Line 0."); !ok || !slices.Equal(pcm, flat(100, 0)) {
		t.Fatalf("Line 0 = %v %v", len(pcm), ok)
	}
	if _, ok := c.Get(other, "Line 0."); ok {
		t.Fatal("another engine's line hit")
	}
	time.Sleep(5 * time.Millisecond)
	put("Line 3.", flat(100, 3))
	put("Line 4.", flat(100, 4))
	if n := len(cached(t, dir)); n != 3 {
		t.Fatalf("%d entries, bound 3", n)
	}
	for line, want := range map[string]bool{"Line 0.": true, "Line 1.": false, "Line 2.": false, "Line 3.": true, "Line 4.": true} {
		if _, ok := c.Get(e, line); ok != want {
			t.Fatalf("%s cached %v, want %v (least recently used goes first)", line, ok, want)
		}
	}

	bytes := NewCache(filepath.Join(t.TempDir(), "voice"), 100, 4000)
	for i := range 3 {
		if err := bytes.Put(e, "Byte "+strconv.Itoa(i)+".", flat(1000, 1)); err != nil {
			t.Fatal(err)
		}
		time.Sleep(5 * time.Millisecond)
	}
	if n := len(cached(t, bytes.dir)); n != 1 {
		t.Fatalf("%d entries of 4000 bytes under a 4000-byte bound", n)
	}
	if _, ok := bytes.Get(e, "Byte 2."); !ok {
		t.Fatal("the newest entry was evicted")
	}
	_ = bytes.Put(e, "Too big.", flat(1001, 1))
	if _, ok := bytes.Get(e, "Too big."); ok {
		t.Fatal("an entry larger than the cache was stored")
	}
	long := strings.Repeat("word ", CacheMaxRunes/5+1)
	_ = c.Put(e, long, flat(10, 1))
	if _, ok := c.Get(e, long); ok {
		t.Fatal("a long line was cached")
	}
	p := c.path(e, "Line 4.")
	if err := os.WriteFile(p, []byte{1, 2, 3}, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, ok := c.Get(e, "Line 4."); ok {
		t.Fatal("a damaged entry hit")
	}
	if _, err := os.Stat(p); !os.IsNotExist(err) {
		t.Fatalf("the damaged entry stayed: %v", err)
	}
}

func TestSpeakerUsesTheCache(t *testing.T) {
	e := &fakeEngine{name: System, level: 0.5}
	c := NewCache(filepath.Join(t.TempDir(), "voice"), 0, 0)
	r := newReports()
	s := NewSpeaker(Options{
		Engines: []Engine{e}, SpeakReplies: true, Cache: c, Report: r.fn,
		Open: func() (sound.Sink, error) {
			return sinkFunc(func([]float32, int) (func(), error) { return func() {}, nil }), nil
		},
	})
	if err := s.Prerender(context.Background(), []string{"On it.", "Got it. Working."}); err != nil {
		t.Fatal(err)
	}
	if n := len(cached(t, c.dir)); n != 3 {
		t.Fatalf("prerendered %d sentences, want 3", n)
	}
	s.Say(1, "Got it.")
	got := r.until(t)
	if !got[0].Cached || got[0].Engine != System {
		t.Fatalf("a prerendered line: %+v", got[0])
	}
	if n := len(e.rendered()); n != 3 {
		t.Fatalf("the engine ran %d times, want 3", n)
	}
}
