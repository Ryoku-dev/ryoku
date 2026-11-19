//go:build live

package services

import (
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"ryoget/internal/media"
	"ryoget/internal/plan"
	"ryoget/internal/ytdlp"
)

// ytBlockPlayer fails the native player call so the yt-dlp path runs for real.
type ytBlockPlayer struct{}

func (ytBlockPlayer) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.URL.Host == "www.youtube.com" && req.URL.Path == "/youtubei/v1/player" {
		return &http.Response{StatusCode: http.StatusForbidden, Body: io.NopCloser(strings.NewReader("")), Request: req}, nil
	}
	return http.DefaultTransport.RoundTrip(req)
}

func TestLiveYouTubeForcedFallback(t *testing.T) {
	env := &plan.Env{HTTP: &http.Client{Transport: ytBlockPlayer{}, Timeout: 60 * time.Second}, Cache: t.TempDir()}
	m, err := media.Resolve(context.Background(), "https://www.youtube.com/watch?v=aqz-KE-bpKQ", env, true)
	if err != nil {
		t.Fatal(err)
	}
	kinds := map[plan.PartKind]int{}
	for _, f := range m.Formats {
		kinds[f.Parts[0].Kind]++
	}
	t.Logf("via yt-dlp: %s | %s | %d formats %v", m.Title, m.Source.Name, len(m.Formats), kinds)
	pl, err := media.PlanFor(m, media.Options{Mode: "audio"})
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("audio plan: %s post=%s part=%s", pl.Name, pl.Post, pl.Parts[0].Kind)
	if pl.Parts[0].Kind == plan.PartHTTP {
		req, _ := http.NewRequest(http.MethodGet, pl.Parts[0].URL, nil)
		for k, v := range pl.Parts[0].Headers {
			req.Header[k] = v
		}
		req.Header.Set("Range", "bytes=0-65535")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		t.Logf("range on the yt-dlp URL outside yt-dlp: %s", resp.Status)
	}

	part := plan.Part{Kind: plan.PartYTDLP, URL: m.URL, Args: []string{"-f", "139"}}
	dst := filepath.Join(t.TempDir(), "a.part0.m4a")
	p := &ytLiveProgress{}
	if err := ytdlp.Fetch(context.Background(), env, part, dst, p); err != nil {
		t.Fatal(err)
	}
	fi, err := os.Stat(dst)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("yt-dlp fetched %d bytes, progress added %d of %d", fi.Size(), p.added, p.total)
}

type ytLiveProgress struct{ added, total int64 }

func (p *ytLiveProgress) SetTotal(n int64)       { p.total = n }
func (p *ytLiveProgress) Add(n int64)            { p.added += n }
func (p *ytLiveProgress) Mark(int, int, float64) {}
func (p *ytLiveProgress) Stage(string)           {}

func TestLiveYouTubePlaylist(t *testing.T) {
	env := &plan.Env{HTTP: &http.Client{Timeout: 30 * time.Second}, Cache: t.TempDir()}
	m, err := media.Resolve(context.Background(), "https://www.youtube.com/playlist?list=PLa1F2ddGya_-UvuAqHAksYnB0qL9yWDO6", env, false)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("%s | %s | %d formats | %d items | note %q | first item %+v", m.Title, m.URL, len(m.Formats), len(m.Items), m.Note, m.Items[0])
	if len(m.Formats) == 0 || len(m.Items) != 43 || m.Items[0].URL != m.URL {
		t.Fatal("playlist media incomplete")
	}
}
