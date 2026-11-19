package media

import (
	"errors"
	"strings"
	"testing"

	"ryoget/internal/plan"
)

func httpFmt(id, kind, ext, codec string, h, fps, br int, v, a bool) Format {
	return Format{ID: id, Kind: kind, Ext: ext, Codec: codec, Height: h, FPS: fps, Bitrate: br, HasVideo: v, HasAudio: a, Size: 100,
		Parts: []plan.Part{{Kind: plan.PartHTTP, URL: "https://cdn.test/" + id, Ext: ext, Size: 100}}}
}

func ytLike() *Media {
	return &Media{Kind: plan.KindMedia, Title: "Big Buck Bunny: 4K / 60fps", URL: "https://www.youtube.com/watch?v=x",
		Headers: map[string]string{"Referer": "https://www.youtube.com/"},
		Formats: []Format{
			httpFmt("315", "video", "webm", "vp9", 2160, 60, 26000000, true, false),
			httpFmt("401", "video", "mp4", "av01.0.13M.08", 2160, 60, 17000000, true, false),
			httpFmt("299", "video", "mp4", "avc1.64002A", 1080, 60, 6900000, true, false),
			httpFmt("303", "video", "webm", "vp9", 1080, 60, 4600000, true, false),
			httpFmt("136", "video", "mp4", "avc1.4d401f", 720, 30, 2000000, true, false),
			httpFmt("140", "audio", "m4a", "mp4a.40.2", 0, 0, 130000, false, true),
			httpFmt("251", "audio", "webm", "opus", 0, 0, 143000, false, true),
			httpFmt("18", "video", "mp4", "avc1.42001E,mp4a.40.2", 360, 30, 500000, true, true),
		}}
}

func ids(pl *plan.Plan) string {
	var out []string
	for _, p := range pl.Parts {
		out = append(out, strings.TrimPrefix(p.URL, "https://cdn.test/")+":"+p.Role)
	}
	return strings.Join(out, " ")
}

func TestPlanFor(t *testing.T) {
	cases := []struct {
		name  string
		media *Media
		opts  Options
		parts string
		post  plan.Post
		box   string
	}{
		{"auto best goes tall in webm", ytLike(), Options{}, "315:video 251:audio", plan.PostMerge, "webm"},
		{"auto 1080 prefers h264 in mp4", ytLike(), Options{Mode: "auto", Quality: "1080"}, "299:video 140:audio", plan.PostMerge, "mp4"},
		{"auto 720p", ytLike(), Options{Quality: "720"}, "136:video 140:audio", plan.PostMerge, "mp4"},
		{"quality below everything takes lowest", ytLike(), Options{Quality: "144"}, "18:", plan.PostNone, "mp4"},
		{"audio best copies aac to m4a", ytLike(), Options{Mode: "audio"}, "251:", plan.PostAudio, "opus"},
		{"audio mp3 transcodes", ytLike(), Options{Mode: "audio", AudioFormat: "mp3"}, "251:", plan.PostTranscode, "mp3"},
		{"audio m4a matches without transcode", ytLike(), Options{Mode: "audio", AudioFormat: "m4a"}, "140:", plan.PostAudio, "m4a"},
		{"mute takes video only", ytLike(), Options{Mode: "mute", Quality: "1080"}, "299:", plan.PostNone, "mp4"},
		{"explicit video pairs audio", ytLike(), Options{Format: "303"}, "303:video 251:audio", plan.PostMerge, "webm"},
		{"explicit pair mixed goes mkv", ytLike(), Options{Format: "299+251"}, "299:video 251:audio", plan.PostMerge, "mkv"},
		{"explicit progressive", ytLike(), Options{Format: "18"}, "18:", plan.PostNone, "mp4"},
		{"explicit progressive mute", ytLike(), Options{Format: "18", Mode: "mute"}, "18:", plan.PostMute, "mp4"},
		{"explicit audio in auto", ytLike(), Options{Format: "140"}, "140:", plan.PostAudio, "m4a"},
		{"progressive only mute", &Media{Kind: plan.KindMedia, Title: "t", Formats: []Format{httpFmt("p", "video", "mp4", "avc1,mp4a.40.2", 720, 30, 1, true, true)}},
			Options{Mode: "mute"}, "p:", plan.PostMute, "mp4"},
		{"progressive beats shorter split", &Media{Kind: plan.KindMedia, Title: "t", Formats: []Format{
			httpFmt("p", "video", "mp4", "avc1,mp4a.40.2", 720, 30, 1, true, true),
			httpFmt("v", "video", "mp4", "avc1", 480, 30, 1, true, false),
			httpFmt("a", "audio", "m4a", "mp4a.40.2", 0, 0, 1, false, true)}},
			Options{}, "p:", plan.PostNone, "mp4"},
		{"audio only media in auto", &Media{Kind: plan.KindMedia, Title: "song", Formats: []Format{
			httpFmt("mp3", "audio", "mp3", "mp3", 0, 0, 128000, false, true)}},
			Options{}, "mp3:", plan.PostAudio, "mp3"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			pl, err := PlanFor(c.media, c.opts)
			if err != nil {
				t.Fatal(err)
			}
			if got := ids(pl); got != c.parts || pl.Post != c.post || pl.Container != c.box {
				t.Fatalf("got %q %q %q; want %q %q %q", got, pl.Post, pl.Container, c.parts, c.post, c.box)
			}
			if !strings.HasSuffix(pl.Name, "."+c.box) || strings.Contains(pl.Name, "/") {
				t.Fatalf("name %q", pl.Name)
			}
			for _, p := range pl.Parts {
				if c.media.Headers != nil && p.Headers.Get("Referer") == "" {
					t.Fatalf("media headers lost on %s", p.URL)
				}
			}
		})
	}
	pl, _ := PlanFor(ytLike(), Options{})
	if pl.Name != "Big Buck Bunny: 4K - 60fps.webm" {
		t.Fatalf("name = %q", pl.Name)
	}
}

func TestPlanHLS(t *testing.T) {
	hlsFmt := func(id string, h int, parts ...plan.Part) Format {
		return Format{ID: id, Kind: "video", Height: h, Codec: "avc1.64001f,mp4a.40.2", HasVideo: true, HasAudio: true, Parts: parts}
	}
	m := &Media{Kind: plan.KindMedia, Title: "stream", Formats: []Format{
		hlsFmt("hls-0", 1080, plan.Part{Kind: plan.PartHLS, URL: "v1080", Role: "video"}, plan.Part{Kind: plan.PartHLS, URL: "aud", Role: "audio"}),
		hlsFmt("hls-1", 720, plan.Part{Kind: plan.PartHLS, URL: "v720"}),
	}}
	pl, err := PlanFor(m, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if ids(pl) != "v1080:video aud:audio" || pl.Post != plan.PostMerge || pl.Container != "mp4" {
		t.Fatalf("hls master plan = %s %s %s", ids(pl), pl.Post, pl.Container)
	}
	pl, _ = PlanFor(m, Options{Quality: "720"})
	if ids(pl) != "v720:" || pl.Post != plan.PostRemux || pl.Name != "stream.mp4" {
		t.Fatalf("hls single plan = %s %s %s", ids(pl), pl.Post, pl.Name)
	}
	pl, _ = PlanFor(m, Options{Mode: "audio"})
	if ids(pl) != "aud:audio" || pl.Post != plan.PostAudio || pl.Container != "m4a" {
		t.Fatalf("hls audio plan = %s %s %s", ids(pl), pl.Post, pl.Container)
	}
	pl, _ = PlanFor(m, Options{Mode: "mute"})
	if ids(pl) != "v1080:video" || pl.Post != plan.PostMute {
		t.Fatalf("hls mute plan = %s %s", ids(pl), pl.Post)
	}
}

func TestPlanYTDLP(t *testing.T) {
	yf := func(id, kind, ext, codec string, h int, v, a bool) Format {
		return Format{ID: id, Kind: kind, Ext: ext, Codec: codec, Height: h, HasVideo: v, HasAudio: a,
			Parts: []plan.Part{{Kind: plan.PartYTDLP, URL: "https://site.test/watch/1", Args: []string{"-f", id}}}}
	}
	m := &Media{Kind: plan.KindMedia, Title: "clip", URL: "https://site.test/watch/1", Formats: []Format{
		yf("dash-v", "video", "mp4", "avc1", 1080, true, false),
		httpFmt("http-a", "audio", "m4a", "mp4a.40.2", 0, 0, 128000, false, true),
	}}
	pl, err := PlanFor(m, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(pl.Parts) != 1 || pl.Parts[0].Kind != plan.PartYTDLP || strings.Join(pl.Parts[0].Args, " ") != "-f dash-v+http-a --merge-output-format mp4" ||
		pl.Post != plan.PostNone || pl.Parts[0].URL != "https://site.test/watch/1" {
		t.Fatalf("ytdlp plan = %+v", pl)
	}
	pl, _ = PlanFor(m, Options{Mode: "mute"})
	if strings.Join(pl.Parts[0].Args, " ") != "-f dash-v" || pl.Post != plan.PostNone {
		t.Fatalf("ytdlp mute plan = %+v", pl)
	}
}

func TestPlanFileAndRefusals(t *testing.T) {
	pl, err := PlanFor(&Media{Kind: plan.KindFile, URL: "https://x.test/a.iso", Name: "a.iso", Size: 42,
		Headers: map[string]string{"Cookie": "k=v"}}, Options{Mode: "audio"})
	if err != nil {
		t.Fatal(err)
	}
	if pl.Name != "a.iso" || pl.Container != "iso" || len(pl.Parts) != 1 || pl.Parts[0].Role != "file" ||
		pl.Parts[0].Size != 42 || pl.Parts[0].Headers.Get("Cookie") != "k=v" || pl.Post != plan.PostNone {
		t.Fatalf("file plan = %+v", pl)
	}
	for _, k := range []plan.Kind{plan.KindTorrent, plan.KindPage} {
		if _, err := PlanFor(&Media{Kind: k}, Options{}); err == nil {
			t.Fatalf("%s planned", k)
		}
	}
	if _, err := PlanFor(ytLike(), Options{Format: "nope"}); err == nil {
		t.Fatal("unknown format planned")
	}
}

func TestSafeName(t *testing.T) {
	cases := map[string]string{
		"a/b\\c":             "a-b-c.mp4",
		"  ..hidden.  ":      "hidden.mp4",
		"tab\there\nnewline": "tab here newline.mp4",
		"":                   "download.mp4",
	}
	for in, want := range cases {
		if got := SafeName(in, "mp4"); got != want {
			t.Errorf("SafeName(%q) = %q; want %q", in, got, want)
		}
	}
	long := strings.Repeat("ü", 200)
	got := SafeName(long, "webm")
	if len(got) > nameLimit || !strings.HasSuffix(got, "ü.webm") {
		t.Fatalf("long name %d bytes: %q", len(got), got[len(got)-10:])
	}
}

func TestPlanItems(t *testing.T) {
	photos := &Media{Kind: plan.KindMedia, Title: "A post", URL: "https://x.com/u/status/1",
		Headers: map[string]string{"Referer": "https://x.com/"},
		Items: []Item{
			{Kind: "photo", URL: "https://pbs.twimg.com/media/AAA?format=jpg&name=orig"},
			{Kind: "photo", URL: "https://cdn.bsky.app/img/feed_fullsize/plain/did/cid@png"},
			{Kind: "photo", URL: "https://64.media.tumblr.com/x/s2048x3072/pic.webp"},
		}}
	pl, err := PlanFor(photos, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if pl.Name != "A post" || pl.Container != "" || pl.Post != plan.PostNone || len(pl.Parts) != 3 {
		t.Fatalf("folder plan = %+v", pl)
	}
	want := []string{"A post 01.jpg", "A post 02.png", "A post 03.webp"}
	for i, p := range pl.Parts {
		if p.Role != "item" || p.Name != want[i] || p.Kind != plan.PartHTTP || p.Headers.Get("Referer") == "" {
			t.Fatalf("part %d = %+v", i, p)
		}
	}
	pl, err = PlanFor(photos, Options{Items: []int{1}})
	if err != nil || len(pl.Parts) != 1 || pl.Parts[0].Role != "file" || pl.Name != "A post 02.png" || pl.Container != "png" {
		t.Fatalf("single item plan = %+v, %v", pl, err)
	}
	if _, err := PlanFor(photos, Options{Items: []int{7}}); err == nil {
		t.Fatal("out of range item planned")
	}

	mixed := &Media{Kind: plan.KindMedia, Title: "mixed", URL: "https://x.com/u/status/2",
		Formats: []Format{httpFmt("v", "video", "mp4", "avc1,mp4a.40.2", 720, 30, 1, true, true)},
		Items: []Item{
			{Kind: "photo", URL: "https://pbs.twimg.com/media/BBB.jpg"},
			{Kind: "video", URL: "https://x.com/u/status/2/video/1"},
		}}
	pl, _ = PlanFor(mixed, Options{})
	if ids(pl) != "v:" {
		t.Fatalf("mixed default plans formats: %s", ids(pl))
	}
	pl, _ = PlanFor(mixed, Options{Items: []int{1}})
	if ids(pl) != "v:" {
		t.Fatalf("first video selection plans formats: %s", ids(pl))
	}
	if _, err := PlanFor(mixed, Options{Items: []int{0, 1}}); !errors.Is(err, ErrUnsupported) {
		t.Fatalf("page item err = %v", err)
	}

	list := &Media{Kind: plan.KindMedia, Title: "v2", URL: "https://www.youtube.com/watch?v=b",
		Formats: []Format{httpFmt("v", "video", "mp4", "avc1,mp4a.40.2", 720, 30, 1, true, true)},
		Items: []Item{
			{Kind: "video", URL: "https://www.youtube.com/watch?v=a"},
			{Kind: "video", URL: "https://www.youtube.com/watch?v=b"},
		}}
	if pl, err := PlanFor(list, Options{Items: []int{1}}); err != nil || ids(pl) != "v:" {
		t.Fatalf("item at media URL plans formats: %v", err)
	}
	if _, err := PlanFor(list, Options{Items: []int{0}}); !errors.Is(err, ErrUnsupported) {
		t.Fatalf("other playlist entry err = %v", err)
	}
}
