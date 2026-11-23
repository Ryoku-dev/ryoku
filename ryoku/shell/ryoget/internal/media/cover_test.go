package media

import (
	"testing"

	"ryoget/internal/plan"
)

func coverMedia(thumb, codec string) *Media {
	return &Media{Kind: plan.KindMedia, Title: "Song", Thumb: thumb, Headers: map[string]string{"User-Agent": "UA", "Cookie": "secret"},
		Formats: []Format{
			{ID: "v", Kind: "video", Codec: "avc1", Height: 720, HasVideo: true, Parts: []plan.Part{{Kind: plan.PartHTTP, URL: "https://cdn/v.mp4", Ext: "mp4"}}},
			{ID: "a", Kind: "audio", Codec: codec, Bitrate: 128000, HasAudio: true, Parts: []plan.Part{{Kind: plan.PartHTTP, URL: "https://cdn/a", Ext: "m4a", Role: "audio"}}},
		}}
}

func TestCoverArtOnAudioOnly(t *testing.T) {
	m := coverMedia("https://img/cover.webp", "mp4a.40.2")
	p, err := PlanFor(m, Options{Mode: "audio", CoverArt: true})
	if err != nil {
		t.Fatal(err)
	}
	last := p.Parts[len(p.Parts)-1]
	if p.Post != plan.PostAudio || p.Container != "m4a" || len(p.Parts) != 2 || last.Role != "cover" ||
		last.URL != "https://img/cover.webp" || last.Ext != "cover" || last.Headers.Get("User-Agent") != "UA" || last.Headers.Get("Cookie") != "" {
		t.Fatalf("plan = %+v", p)
	}
	p, err = PlanFor(m, Options{Mode: "audio", AudioFormat: "opus", CoverArt: true})
	if err != nil || p.Post != plan.PostTranscode || p.Parts[len(p.Parts)-1].Role != "cover" {
		t.Fatalf("opus = %+v, %v", p, err)
	}
	for name, c := range map[string]struct {
		m    *Media
		opts Options
	}{
		"setting off":    {m, Options{Mode: "audio"}},
		"video":          {m, Options{CoverArt: true}},
		"no thumbnail":   {coverMedia("", "mp4a.40.2"), Options{Mode: "audio", CoverArt: true}},
		"inline picture": {coverMedia("data:image/png;base64,AA", "mp4a.40.2"), Options{Mode: "audio", CoverArt: true}},
		"unknown codec":  {coverMedia("https://img/c.jpg", "weird"), Options{Mode: "audio", CoverArt: true}},
	} {
		p, err := PlanFor(c.m, c.opts)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		for _, part := range p.Parts {
			if part.Role == "cover" {
				t.Fatalf("%s: plan carries a cover: %+v", name, p)
			}
		}
	}
}
