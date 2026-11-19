package services

import (
	"errors"
	"strings"
	"testing"

	"ryoget/internal/media"
	"ryoget/internal/plan"
)

func TestXToken(t *testing.T) {
	// values X's embed computes in the browser for these posts
	for id, want := range map[string]string{
		"1600649710662213632": "3vol7tqmapv",
		"1790637656616943991": "4c9gcitu1vq",
		"665052190608723968":  "1m1bmpg2m2t",
	} {
		if got := xToken(id); got != want {
			t.Errorf("xToken(%s) = %q, want %q", id, got, want)
		}
	}
}

func xRoute(id, file string) route {
	return route{url: "cdn.syndication.twimg.com/tweet-result", query: "id=" + id, file: file}
}

func TestXMixedPost(t *testing.T) {
	m, err := resolveFixture(t, "https://x.com/oshtru/status/1577855540407197696?s=20",
		xRoute("1577855540407197696", "x/mixed.json"))
	if err != nil {
		t.Fatal(err)
	}
	checkMedia(t, m, "x")
	if len(m.Items) != 2 || m.Items[0].Kind != "photo" || m.Items[1].Kind != "video" {
		t.Fatalf("items = %+v", m.Items)
	}
	if !strings.HasSuffix(m.Items[0].URL, "?name=orig") || !strings.HasSuffix(m.Items[1].URL, "/status/1577855540407197696/video/2") {
		t.Fatalf("item urls = %q, %q", m.Items[0].URL, m.Items[1].URL)
	}
	if len(m.Formats) != 3 || m.Default != "mp4-2" || m.Duration != 30.03 {
		t.Fatalf("formats %d default %s duration %v", len(m.Formats), m.Default, m.Duration)
	}
	if f := findFormat(m, "mp4-2"); f.Height != 900 || f.Bitrate != 2176000 || !f.HasAudio {
		t.Fatalf("best = %+v", f)
	}
	if m.Author != "Oshtru" {
		t.Fatalf("author = %q", m.Author)
	}
}

func TestXItemSelection(t *testing.T) {
	r := xRoute("1577855540407197696", "x/mixed.json")
	m, err := resolveFixture(t, "https://x.com/oshtru/status/1577855540407197696/photo/1", r)
	if err != nil {
		t.Fatal(err)
	}
	if m.Kind != plan.KindFile || !strings.Contains(m.URL, "pbs.twimg.com/media/") || !strings.HasSuffix(m.Name, ".jpg") {
		t.Fatalf("photo item = %+v", m)
	}
	m, err = resolveFixture(t, "https://twitter.com/oshtru/status/1577855540407197696/video/2", r)
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Items) != 0 || len(m.Formats) != 3 {
		t.Fatalf("video item: items %d formats %d", len(m.Items), len(m.Formats))
	}
}

func TestXTwoVideos(t *testing.T) {
	m, err := resolveFixture(t, "https://t.co/abc123",
		route{url: "t.co/abc123", status: 301, header: map[string]string{"Location": "https://x.com/CTVJLaidlaw/status/1600649710662213632"}},
		xRoute("1600649710662213632", "x/multi.json"))
	if err != nil {
		t.Fatal(err)
	}
	checkMedia(t, m, "x")
	if len(m.Items) != 2 || m.Items[1].URL != "https://x.com/JocelynVLaidlaw/status/1600649710662213632/video/2" {
		t.Fatalf("items = %+v", m.Items)
	}
	if m.Formats[0].Parts[0].Kind != plan.PartHTTP {
		t.Fatalf("part = %+v", m.Formats[0].Parts[0])
	}
}

func TestXMissing(t *testing.T) {
	_, err := resolveFixture(t, "https://x.com/someone/status/1",
		route{url: "cdn.syndication.twimg.com/tweet-result", status: 404, text: ""})
	if !errors.Is(err, media.ErrUnavailable) {
		t.Fatalf("err = %v", err)
	}
	_, err = resolveFixture(t, "https://x.com/someone/likes")
	if !errors.Is(err, media.ErrUnsupported) {
		t.Fatalf("err = %v", err)
	}
	_, err = resolveFixture(t, "https://x.com/someone/status/2",
		route{url: "cdn.syndication.twimg.com/tweet-result", status: 429})
	if !errors.Is(err, media.ErrRateLimited) {
		t.Fatalf("err = %v", err)
	}
}
