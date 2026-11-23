package services

import (
	"errors"
	"strconv"
	"strings"
	"testing"

	"ryoget/internal/media"
	"ryoget/internal/plan"
	"ryoget/internal/proto"
)

func TestBandcampTrack(t *testing.T) {
	m, err := resolveFixture(t, "http://youtube-dl.bandcamp.com/track/youtube-dl-test-song?from=embed",
		route{url: "youtube-dl.bandcamp.com/track/youtube-dl-test-song", file: "bandcamp/track.html"})
	if err != nil {
		t.Fatal(err)
	}
	checkMedia(t, m, "bandcamp")
	f := findFormat(m, "mp3-128")
	if len(m.Formats) != 1 || f == nil || f.Kind != "audio" || f.HasVideo || f.Bitrate != 128000 || f.Ext != "mp3" ||
		!strings.HasPrefix(f.Parts[0].URL, "https://t4.bcbits.com/stream/") {
		t.Fatalf("formats = %+v", m.Formats)
	}
	if !strings.HasPrefix(m.Title, "youtube-dl test song") || !strings.HasPrefix(m.Author, "youtube-dl") ||
		m.Thumb != "https://f4.bcbits.com/img/a3216802731_5.jpg" || m.Duration < 9 || m.URL != "https://youtube-dl.bandcamp.com/track/youtube-dl-test-song" {
		t.Fatalf("track = %+v", m)
	}
	p, err := media.PlanFor(m, media.Options{Mode: "audio"})
	if err != nil || len(p.Parts) != 1 || p.Container != "mp3" {
		t.Fatalf("plan = %+v, %v", p, err)
	}
}

func TestBandcampAlbum(t *testing.T) {
	m, err := resolveFixture(t, "https://blazo.bandcamp.com/album/jazz-format-mixtape-vol-1",
		route{url: "blazo.bandcamp.com/album/jazz-format-mixtape-vol-1", file: "bandcamp/album.html"})
	if err != nil {
		t.Fatal(err)
	}
	checkMedia(t, m, "bandcamp")
	// the third track only plays after buying, so it is left out
	if m.Title != "Jazz Format Mixtape vol.1" || m.Author != "Blazo" || len(m.Formats) != 0 || len(m.Items) != 3 {
		t.Fatalf("album = %+v", m)
	}
	if it := m.Items[0]; it.Title != "Intro" || it.Kind != "audio" || it.URL != "https://blazo.bandcamp.com/track/intro" {
		t.Fatalf("first item = %+v", it)
	}
	if _, err := media.PlanFor(m, media.Options{}); !errors.Is(err, media.ErrUnsupported) {
		t.Fatalf("album tracks are pages of their own: %v", err)
	}
}

func TestBandcampChallengeAndScope(t *testing.T) {
	_, err := resolveFixture(t, "https://youtube-dl.bandcamp.com/track/x",
		route{url: "youtube-dl.bandcamp.com/track/x", file: "bandcamp/challenge.html"})
	var pe *proto.Error
	if !errors.Is(err, media.ErrBrowserCheck) || !errors.As(err, &pe) || pe.Code != proto.CodeBrowserCheck {
		t.Fatalf("challenge = %v", err)
	}
	for _, raw := range []string{"https://blazo.bandcamp.com/", "https://blazo.bandcamp.com/music", "https://bandcamp.com/discover"} {
		if s := media.Lookup(mustURL(t, raw)); s != nil && s.ID() == "bandcamp" {
			t.Fatalf("%s matched bandcamp", raw)
		}
	}
}

func TestBandcampPurchaseOnlyIsPrivate(t *testing.T) {
	const paid = `{&quot;id&quot;:1,&quot;title&quot;:&quot;Paid&quot;,&quot;title_link&quot;:&quot;/track/paid&quot;,&quot;file&quot;:null}`
	for name, c := range map[string]struct{ path, tralbum string }{
		"track without files":  {"/track/paid", `{&quot;item_type&quot;:&quot;track&quot;,&quot;artist&quot;:&quot;Blazo&quot;,&quot;current&quot;:{&quot;title&quot;:&quot;Paid&quot;},&quot;trackinfo&quot;:[` + paid + `]}`},
		"track without mp3":    {"/track/paid", `{&quot;item_type&quot;:&quot;track&quot;,&quot;artist&quot;:&quot;Blazo&quot;,&quot;current&quot;:{&quot;title&quot;:&quot;Paid&quot;},&quot;trackinfo&quot;:[{&quot;id&quot;:1,&quot;title&quot;:&quot;Paid&quot;,&quot;file&quot;:{&quot;flac&quot;:&quot;https://t4.bcbits.com/stream/x&quot;}}]}`},
		"album without tracks": {"/album/paid", `{&quot;item_type&quot;:&quot;album&quot;,&quot;artist&quot;:&quot;Blazo&quot;,&quot;current&quot;:{&quot;title&quot;:&quot;Paid&quot;},&quot;trackinfo&quot;:[` + paid + `]}`},
	} {
		_, err := resolveFixture(t, "https://blazo.bandcamp.com"+c.path,
			route{url: "blazo.bandcamp.com" + c.path, text: `<html><body><script data-tralbum="` + c.tralbum + `"></script></body></html>`})
		if !errors.Is(err, media.ErrPrivate) {
			t.Fatalf("%s: %v, want the private code", name, err)
		}
	}
}

func TestTEDTalk(t *testing.T) {
	m, err := resolveFixture(t, "https://embed.ted.com/talks/dan_dennett_the_illusion_of_consciousness",
		route{url: "www.ted.com/talks/dan_dennett_the_illusion_of_consciousness", file: "ted/talk.html"},
		route{url: "hls.ted.com/project_masters/854/manifest.m3u8", query: "intro_master_id=9294", file: "ted/master.m3u8"})
	if err != nil {
		t.Fatal(err)
	}
	checkMedia(t, m, "ted")
	if m.Title != "The illusion of consciousness" || m.Author != "Dan Dennett" || m.Duration != 1294 ||
		!strings.HasPrefix(m.Thumb, "https://pe.tedcdn.com/images/ted/") || m.URL != "https://www.ted.com/talks/dan_dennett_the_illusion_of_consciousness" {
		t.Fatalf("talk = %+v", m)
	}
	def := findFormat(m, m.Default)
	if def.Height != 480 || len(def.Parts) != 2 || def.Parts[0].Kind != plan.PartHLS || def.Parts[1].Role != "audio" {
		t.Fatalf("default = %+v", def)
	}
	mp4 := findFormat(m, "h264-1200k")
	if mp4 == nil || mp4.Parts[0].Kind != plan.PartHTTP || !strings.HasSuffix(mp4.Parts[0].URL, "-1200k.mp4") {
		t.Fatalf("mp4 fallback = %+v", mp4)
	}
	p, err := media.PlanFor(m, media.Options{})
	if err != nil || p.Post != plan.PostMerge || len(p.Parts) != 2 {
		t.Fatalf("plan = %+v, %v", p, err)
	}
	_, err = resolveFixture(t, "https://www.ted.com/talks/nothing",
		route{url: "www.ted.com/talks/nothing", text: `<script id="__NEXT_DATA__" type="application/json">{"props":{"pageProps":{"videoData":{"title":"x","videoPlayerData":{"resources":null}}}}}</script>`})
	if !errors.Is(err, media.ErrUnavailable) {
		t.Fatalf("no media: %v", err)
	}
	if media.Lookup(mustURL(t, "https://www.ted.com/playlists/1/x")) != nil {
		t.Fatal("playlists are not talks")
	}
}

const odyseeProxy = "api.na-backend.odysee.com/api/v1/proxy"

func TestOdyseeVideo(t *testing.T) {
	m, err := resolveFixture(t, "https://odysee.com/@BrodieRobertson:5/apple-is-tracking-everything-you-do-on:e",
		route{method: "POST", url: odyseeProxy, body: `"method":"resolve"`, file: "odysee/resolve.json"},
		route{method: "POST", url: odyseeProxy, body: `"uri":"lbry://apple-is-tracking-everything-you-do-on#e6b46b4ae2bb25c67915dad71ac5d202e47dba25"`, file: "odysee/get.json"},
		route{url: "player.odycdn.com/v6/streams/e6b46b4ae2bb25c67915dad71ac5d202e47dba25/baac0c/master.m3u8", file: "odysee/master.m3u8"})
	if err != nil {
		t.Fatal(err)
	}
	checkMedia(t, m, "odysee")
	if m.Title != "Apple Is Tracking EVERYTHING You Do On Big Sur" || !strings.HasPrefix(m.Author, "Brodie Robertson") || m.Duration != 745 ||
		m.Thumb == "" || m.URL != "https://odysee.com/@BrodieRobertson:5/apple-is-tracking-everything-you-do-on:e" {
		t.Fatalf("video = %+v", m)
	}
	orig := findFormat(m, "original")
	if m.Default != "original" || orig.Height != 1080 || orig.Size != 79159050 || orig.Ext != "mp4" ||
		orig.Parts[0].URL != "https://player.odycdn.com/v6/streams/e6b46b4ae2bb25c67915dad71ac5d202e47dba25/baac0c.mp4" {
		t.Fatalf("original = %+v", orig)
	}
	if len(m.Formats) != 5 {
		t.Fatalf("formats = %d, want the original and four HLS heights", len(m.Formats))
	}
	p, err := media.PlanFor(m, media.Options{Quality: "720"})
	if err != nil || p.Parts[0].Kind != plan.PartHLS {
		t.Fatalf("720p plan = %+v, %v", p, err)
	}
}

func TestOdyseeURIs(t *testing.T) {
	for raw, want := range map[string]string{
		"https://odysee.com/@chan:5/some-video:e":         "lbry://@chan#5/some-video#e",
		"https://odysee.com/some-video:3f":                "lbry://some-video#3f",
		"https://odysee.com/$/embed/some-video/3f0e":      "lbry://some-video#3f0e",
		"https://odysee.com/$/embed/@chan:5/some-video:e": "lbry://@chan#5/some-video#e",
		"https://odysee.com/@chan:5/caf%C3%A9:1":          "lbry://@chan#5/café#1",
		"https://odysee.com/@chan:5":                      "",
		"https://odysee.com/$/following":                  "",
		"https://odysee.com/":                             "",
		"https://odysee.com/@chan:5/a:1/extra":            "",
	} {
		if got := lbryURI(mustURL(t, raw)); got != want {
			t.Errorf("%s: %q, want %q", raw, got, want)
		}
	}
	_, err := resolveFixture(t, "https://odysee.com/@chan:5")
	if !errors.Is(err, media.ErrUnsupported) {
		t.Fatalf("channel: %v", err)
	}
	_, err = resolveFixture(t, "https://odysee.com/gone:1",
		route{method: "POST", url: odyseeProxy, body: `"method":"resolve"`,
			text: `{"jsonrpc":"2.0","result":{"lbry://gone#1":{"error":{"name":"NOT_FOUND","text":"no claim"}}},"id":1}`})
	if !errors.Is(err, media.ErrUnavailable) {
		t.Fatalf("missing claim: %v", err)
	}
}

func TestRumbleVideo(t *testing.T) {
	m, err := resolveFixture(t, "https://rumble.com/vdmum1-moose-the-dog-helps-girls-dig-a-snow-fort.html?e9s=src_v1",
		route{url: "rumble.com/vdmum1-moose-the-dog-helps-girls-dig-a-snow-fort.html", file: "rumble/page.html"},
		route{url: "rumble.com/embedJS/u3/", query: "v=vb0ofn", file: "rumble/embed.json"})
	if err != nil {
		t.Fatal(err)
	}
	checkMedia(t, m, "rumble")
	if m.Title != "Winter-loving dog helps girls dig a snow fort" || m.Author != "LovingMontana" || m.Duration != 103 ||
		m.URL != "https://rumble.com/vdmum1-moose-the-dog-helps-girls-dig-a-snow-fort.html" || m.Thumb == "" {
		t.Fatalf("video = %+v", m)
	}
	def := findFormat(m, m.Default)
	if len(m.Formats) != 5 || m.Default != "mp4-1080" || def.Height != 1080 || def.Size != 34823447 || def.Bitrate != 2688000 || def.FPS != 30 {
		t.Fatalf("formats = %+v", m.Formats)
	}
	if w := findFormat(m, "webm-480"); w == nil || w.Ext != "webm" {
		t.Fatalf("webm = %+v", w)
	}
	m, err = resolveFixture(t, "https://rumble.com/embed/vb0ofn/?pub=4",
		route{url: "rumble.com/embedJS/u3/", query: "v=vb0ofn", file: "rumble/embed.json"})
	if err != nil || len(m.Formats) != 5 {
		t.Fatalf("embed = %+v, %v", m, err)
	}
	_, err = resolveFixture(t, "https://rumble.com/embed/vb0ofn/",
		route{url: "rumble.com/embedJS/u3/", status: 403, header: map[string]string{"Cf-Mitigated": "challenge"}, text: "<html></html>"})
	if !errors.Is(err, media.ErrBrowserCheck) {
		t.Fatalf("challenged: %v", err)
	}
	if _, err := resolveFixture(t, "https://rumble.com/c/LovingMontana"); !errors.Is(err, media.ErrUnsupported) {
		t.Fatalf("channel: %v", err)
	}
}

func TestRumbleHLSOnly(t *testing.T) {
	const cdn = "hugh.cdn.rumble.cloud/live/v6hls1/"
	routes := []route{{url: "rumble.com/embedJS/u3/", query: "v=v6hls1", file: "rumble/hls.json"},
		{url: cdn + "master.m3u8", file: "rumble/master.m3u8"}}
	// every height answers too, so a pick that follows map order would show up as a single-height result
	for _, h := range []string{"360", "480", "720", "1080"} {
		routes = append(routes, route{url: cdn + h + ".m3u8", file: "rumble/media.m3u8"})
	}
	for i := range 8 {
		m, err := resolveFixture(t, "https://rumble.com/embed/v6hls1/", routes...)
		if err != nil {
			t.Fatal(err)
		}
		checkMedia(t, m, "rumble")
		if m.Title != `"Snow Day" at the cabin & more` || m.Author != "Cabin & Co" {
			t.Fatalf("title %q, author %q", m.Title, m.Author)
		}
		want := []int{1080, 720, 480, 360}
		if len(m.Formats) != len(want) {
			t.Fatalf("resolve %d: formats = %+v, want the master's four", i, m.Formats)
		}
		for j, f := range m.Formats {
			if f.Height != want[j] || f.Parts[0].Kind != plan.PartHLS || f.Parts[0].URL != "https://"+cdn+strconv.Itoa(want[j])+".m3u8" {
				t.Fatalf("resolve %d: format %d = %+v", i, j, f)
			}
		}
	}
}

func TestRumbleLiveGoesToYTDLP(t *testing.T) {
	_, err := resolveFixture(t, "https://rumble.com/embed/v6live1/",
		route{url: "rumble.com/embedJS/u3/", query: "v=v6live1", file: "rumble/live.json"})
	if !errors.Is(err, media.ErrUnsupported) {
		t.Fatalf("live: %v, want unsupported so yt-dlp records it", err)
	}
}

func TestLinkedInPost(t *testing.T) {
	const post = "https://www.linkedin.com/posts/mishalkhawaja_sendinblueviews-toronto-digitalmarketing-ugcPost-6850898786781339649-mM20"
	m, err := resolveFixture(t, post+"?utm_source=share",
		route{url: strings.TrimPrefix(post, "https://"), file: "linkedin/post.html"})
	if err != nil {
		t.Fatal(err)
	}
	checkMedia(t, m, "linkedin")
	if m.Author != "Mishal K." || !strings.HasPrefix(m.Title, "Sendinblue Toronto office") || !strings.HasPrefix(m.Thumb, "https://media.licdn.com/") || m.URL != post {
		t.Fatalf("post = %+v", m)
	}
	if len(m.Formats) != 3 || m.Default != "original" {
		t.Fatalf("formats = %+v, default %s", m.Formats, m.Default)
	}
	if f := findFormat(m, "mp4-720p"); f == nil || f.Height != 720 || f.FPS != 30 || f.Bitrate != 1002603 {
		t.Fatalf("720p = %+v", f)
	}
}

func TestLinkedInListingsRefused(t *testing.T) {
	for _, raw := range []string{"https://www.linkedin.com/feed/", "https://www.linkedin.com/in/someone/",
		"https://www.linkedin.com/company/acme/posts/", "https://linkedin.com/"} {
		u := mustURL(t, raw)
		s := media.Lookup(u)
		if s == nil || s.ID() != "linkedin" {
			t.Fatalf("%s: lookup %v", raw, s)
		}
		_, err := s.Resolve(t.Context(), u, fixtureEnv(t))
		if !errors.Is(err, media.ErrUnsupported) {
			t.Fatalf("%s: %v", raw, err)
		}
	}
	if media.Lookup(mustURL(t, "https://www.linkedin.com/pulse/some-article")) != nil {
		t.Fatal("articles go to the generic reader")
	}
	for _, raw := range []string{"https://www.linkedin.com/feed/update/urn:li:ugcPost:6850898786781339649/",
		"https://www.linkedin.com/embed/feed/update/urn:li:ugcPost:6850898786781339649"} {
		if !linkedinPost(mustURL(t, raw)) {
			t.Fatalf("%s is a post", raw)
		}
	}
}

func TestStatus999IsRateLimited(t *testing.T) {
	if err := statusErr(999); !errors.Is(err, media.ErrRateLimited) {
		t.Fatalf("999 = %v", err)
	}
}
