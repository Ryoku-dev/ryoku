//go:build live

package youtube

import (
	"context"
	"net/http"
	"net/url"
	"testing"
	"time"

	"ryoget/internal/media"
)

func liveClient(t *testing.T) *Client {
	return New(&http.Client{Timeout: 30 * time.Second}, t.TempDir())
}

func logMedia(t *testing.T, m *media.Media) {
	t.Logf("%s | %s | %.0fs | %s | %d formats", m.Title, m.Author, m.Duration, m.Thumb, len(m.Formats))
	for _, f := range m.Formats {
		t.Logf("  %-4s %-5s %-8s %-6s %-24s %9d B", f.ID, f.Kind, f.Label, f.Ext, f.Codec, f.Size)
	}
}

// probeRange checks a stream URL answers a byte range outside our resolver.
func probeRange(t *testing.T, f media.Format) {
	p := f.Parts[0]
	req, _ := http.NewRequest(http.MethodGet, p.URL, nil)
	for k, v := range p.Headers {
		req.Header[k] = v
	}
	req.Header.Set("Range", "bytes=0-65535")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusPartialContent {
		t.Fatalf("range on format %s: %s", f.ID, resp.Status)
	}
}

func TestLiveBlenderMovie(t *testing.T) {
	c := liveClient(t)
	m, err := c.Media(context.Background(), "aqz-KE-bpKQ")
	if err != nil {
		t.Fatal(err)
	}
	logMedia(t, m)
	pl, err := media.PlanFor(m, media.Options{Quality: "1080"})
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("plan 1080: %s post=%s parts=%d", pl.Name, pl.Post, len(pl.Parts))
	for _, f := range m.Formats {
		if f.ID == "140" || f.ID == "299" {
			probeRange(t, f)
		}
	}
}

func TestLiveShort(t *testing.T) {
	c := liveClient(t)
	vs, err := c.Search(context.Background(), "blender animation #shorts", "en", 20)
	if err != nil {
		t.Fatal(err)
	}
	id := ""
	for _, v := range vs {
		if v.Duration > 0 && v.Duration <= 60 {
			id = v.ID
			break
		}
	}
	if id == "" {
		t.Skip("search found no short")
	}
	u, _ := url.Parse("https://www.youtube.com/shorts/" + id)
	vid, _ := ParseURL(u)
	m, err := c.Media(context.Background(), vid)
	if err != nil {
		t.Fatalf("short %s: %v", id, err)
	}
	logMedia(t, m)
}

func TestLiveSearch(t *testing.T) {
	vs, err := Search(context.Background(), nil, "blender open movie", "en", 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(vs) == 0 {
		t.Fatal("no results")
	}
	for _, v := range vs {
		t.Logf("%s | %s | %s | %s | %d views | %s", v.ID, v.Title, v.Channel, v.DurationText, v.Views, v.Published)
	}
}

func TestLivePlaylist(t *testing.T) {
	big, err := liveClient(t).Playlist(context.Background(), "UUSMOQeBJ2RAnuFungnQOxLg")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("%s: %d videos, %d listed, first %s, last %s %q", big.Title, big.Count, len(big.Entries), big.First, big.Entries[len(big.Entries)-1].ID, big.Entries[len(big.Entries)-1].Title)
	if len(big.Entries) != PlaylistCap {
		t.Fatalf("listed %d", len(big.Entries))
	}
	pl, err := liveClient(t).Playlist(context.Background(), "PLa1F2ddGya_-UvuAqHAksYnB0qL9yWDO6")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("%+v", pl)
}
