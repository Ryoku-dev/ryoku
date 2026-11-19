package ytdlp

import (
	"context"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"golang.org/x/time/rate"

	"ryoget/internal/media"
	"ryoget/internal/plan"
)

func stubEnv(t *testing.T, fixture string) *plan.Env {
	t.Helper()
	stub, _ := filepath.Abs("testdata/yt-dlp-stub.sh")
	if fixture != "" {
		abs, _ := filepath.Abs(filepath.Join("testdata", fixture))
		t.Setenv("STUB_JSON", abs)
	}
	return &plan.Env{YTDLP: stub}
}

func TestResolveFormats(t *testing.T) {
	env := stubEnv(t, "video.json")
	args := filepath.Join(t.TempDir(), "args")
	t.Setenv("STUB_ARGS", args)
	m, err := Resolve(context.Background(), env, "https://vimeo.test/123")
	if err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(args)
	for _, want := range []string{"-J", "--no-playlist", "--ignore-config", "--\nhttps://vimeo.test/123"} {
		if !strings.Contains(string(got), want) {
			t.Fatalf("args %q lack %q", got, want)
		}
	}
	if m.Title != "A Test Clip" || m.Author != "Someone" || m.Duration != 12.5 || m.Source.Name != "Vimeo" || m.URL != "https://vimeo.test/123" {
		t.Fatalf("media = %+v", m)
	}
	byID := map[string]media.Format{}
	for _, f := range m.Formats {
		byID[f.ID] = f
	}
	if len(byID) != 4 {
		t.Fatalf("formats = %v (storyboard and DRM must go)", byID)
	}
	h := byID["http-1080"]
	if h.Parts[0].Kind != plan.PartHTTP || h.Parts[0].Headers.Get("User-Agent") != "UA-test" || h.Size != 1234567 ||
		!h.HasVideo || h.HasAudio || h.Label != "1080p60" {
		t.Fatalf("http format = %+v", h)
	}
	if p := byID["hls-720"].Parts[0]; p.Kind != plan.PartHLS || p.URL != "https://cdn.test/720.m3u8" || !byID["hls-720"].HasAudio {
		t.Fatalf("hls format = %+v", byID["hls-720"])
	}
	if p := byID["dash-1080"].Parts[0]; p.Kind != plan.PartYTDLP || strings.Join(p.Args, " ") != "-f dash-1080" || p.URL != "https://vimeo.test/123" {
		t.Fatalf("dash format = %+v", p)
	}
	if a := byID["audio-hi"]; a.Kind != "audio" || a.Bitrate != 128400 || a.Label != "128 kbps" || a.Size != 200000 {
		t.Fatalf("audio format = %+v", a)
	}

	pl, err := media.PlanFor(m, media.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if pl.Post != plan.PostMerge || len(pl.Parts) != 2 || pl.Parts[0].URL != "https://cdn.test/1080.mp4" {
		t.Fatalf("plan = %+v", pl)
	}
}

func TestResolvePlaylistTakesFirst(t *testing.T) {
	m, err := Resolve(context.Background(), stubEnv(t, "playlist.json"), "https://www.youtube.com/playlist?list=PL1")
	if err != nil {
		t.Fatal(err)
	}
	if m.Title != "First" || len(m.Formats) != 1 || m.Note == "" || !strings.Contains(m.Note, "7") {
		t.Fatalf("media = %+v", m)
	}
}

func TestResolveErrors(t *testing.T) {
	env := stubEnv(t, "video.json")
	if _, err := Resolve(context.Background(), env, "https://site.test/fail"); !errors.Is(err, media.ErrPrivate) || !strings.Contains(err.Error(), "Private video") {
		t.Fatalf("private err = %v", err)
	}
	if _, err := Resolve(context.Background(), env, "https://site.test/nope"); !errors.Is(err, media.ErrUnsupported) {
		t.Fatalf("unsupported err = %v", err)
	}
	if _, err := Resolve(context.Background(), &plan.Env{YTDLP: "/nonexistent/yt-dlp"}, "https://x.test"); !errors.Is(err, media.ErrUnsupported) {
		t.Fatalf("missing binary err = %v", err)
	}
}

func TestFallbackRegistered(t *testing.T) {
	u, _ := url.Parse("https://unknown-site.test/page")
	fb := media.Fallback()
	if fb == nil {
		t.Fatal("no fallback registered")
	}
	m, err := fb(context.Background(), u, stubEnv(t, "video.json"))
	if err != nil || m.Title != "A Test Clip" {
		t.Fatalf("fallback = %+v, %v", m, err)
	}
}

type progress struct {
	mu      sync.Mutex
	total   int64
	added   int64
	resumed int64
	marks   map[int]float64
	stages  []string
}

func (p *progress) SetTotal(n int64) { p.mu.Lock(); p.total = n; p.mu.Unlock() }
func (p *progress) Add(n int64)      { p.mu.Lock(); p.added += n; p.mu.Unlock() }
func (p *progress) Resumed(n int64)  { p.mu.Lock(); p.resumed += n; p.mu.Unlock() }
func (p *progress) Stage(s string)   { p.mu.Lock(); p.stages = append(p.stages, s); p.mu.Unlock() }
func (p *progress) Mark(i, count int, fill float64) {
	p.mu.Lock()
	if p.marks == nil {
		p.marks = map[int]float64{}
	}
	p.marks[i] = fill
	p.mu.Unlock()
}

func TestFetch(t *testing.T) {
	env := stubEnv(t, "")
	env.FFmpeg = "/usr/bin/ffmpeg"
	env.Limit = rate.NewLimiter(rate.Limit(512<<10), 256<<10)
	args := filepath.Join(t.TempDir(), "args")
	t.Setenv("STUB_ARGS", args)
	dst := filepath.Join(t.TempDir(), "100% done.part0.mp4")
	p := &progress{}
	part := plan.Part{Kind: plan.PartYTDLP, URL: "https://site.test/v", Args: []string{"-f", "a+b", "--merge-output-format", "mp4"},
		Headers: map[string][]string{"Referer": {"https://site.test/"}}}
	if err := Fetch(context.Background(), env, part, dst, p); err != nil {
		t.Fatal(err)
	}
	if b, err := os.ReadFile(dst); err != nil || string(b) != "merged" {
		t.Fatalf("dst = %q, %v", b, err)
	}
	got, _ := os.ReadFile(args)
	for _, want := range []string{"--newline", "--ffmpeg-location\n/usr/bin/ffmpeg", "--limit-rate\n524288", "--add-headers\nReferer:https://site.test/",
		"-f\na+b\n--merge-output-format\nmp4", "-o\n" + strings.ReplaceAll(dst, "%", "%%"), "--\nhttps://site.test/v"} {
		if !strings.Contains(string(got), want) {
			t.Fatalf("args lack %q:\n%s", want, got)
		}
	}
	if p.added != 1500 || p.total != 1500 || p.resumed != 0 {
		t.Fatalf("progress added %d total %d resumed %d", p.added, p.total, p.resumed)
	}
	if len(p.marks) != buckets || p.marks[buckets-1] != 1 {
		t.Fatalf("marks = %v", p.marks)
	}
}

func TestFetchCancel(t *testing.T) {
	env := stubEnv(t, "")
	t.Setenv("STUB_SLOW", "1")
	dst := filepath.Join(t.TempDir(), "x.mp4")
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- Fetch(ctx, env, plan.Part{URL: "https://site.test/v"}, dst, &progress{}) }()
	time.Sleep(300 * time.Millisecond)
	start := time.Now()
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("err = %v", err)
		}
	case <-time.After(15 * time.Second):
		t.Fatal("fetch ignored cancel")
	}
	if time.Since(start) > 8*time.Second {
		t.Fatalf("cancel took %v", time.Since(start))
	}
	if _, err := os.Stat(dst + ".interrupted"); err != nil {
		t.Fatal("yt-dlp did not get SIGINT")
	}
}

func TestFetchError(t *testing.T) {
	err := Fetch(context.Background(), stubEnv(t, ""), plan.Part{URL: "https://site.test/fail"}, filepath.Join(t.TempDir(), "x"), &progress{})
	if !errors.Is(err, media.ErrPrivate) {
		t.Fatalf("err = %v", err)
	}
}
