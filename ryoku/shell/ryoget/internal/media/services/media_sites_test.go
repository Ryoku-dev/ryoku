package services

import (
	"errors"
	"strings"
	"testing"

	"ryoget/internal/media"
	"ryoget/internal/plan"
)

func TestSoundCloudTrack(t *testing.T) {
	m, err := resolveFixture(t, "https://m.soundcloud.com/the80m/the-following?in=x",
		route{url: "soundcloud.com/the80m/the-following", file: "soundcloud/track.html"},
		route{url: "api-v2.soundcloud.com/media/soundcloud:tracks:343609555/d3b1d913-bca3-4cb5-98a8-1eee2c83eee8/stream/hls",
			query: "client_id=QhVEWrrJKvW4uSU5u5EoI0FTXYUGzy5k", file: "soundcloud/stream-160.json"},
		route{url: "api-v2.soundcloud.com/media/soundcloud:tracks:343609555/45c5285f-2127-4db0-a530-195e2806a2f2/stream/hls",
			query: "track_authorization=", file: "soundcloud/stream-96.json"})
	if err != nil {
		t.Fatal(err)
	}
	checkMedia(t, m, "soundcloud")
	if len(m.Formats) != 2 || m.Default != "aac_160k-hls" || m.Author != "80M" || m.Duration < 397 {
		t.Fatalf("track = %+v", m)
	}
	f := findFormat(m, m.Default)
	if f.Kind != "audio" || f.HasVideo || f.Bitrate != 160000 || f.Parts[0].Kind != plan.PartHLS || !strings.Contains(f.Parts[0].URL, "playlist.m3u8") {
		t.Fatalf("default = %+v", f)
	}
	if !strings.Contains(m.Thumb, "t500x500") {
		t.Fatalf("thumb = %s", m.Thumb)
	}
	p, err := media.PlanFor(m, media.Options{Mode: "audio"})
	if err != nil || len(p.Parts) != 1 {
		t.Fatalf("plan = %+v, %v", p, err)
	}
}

func TestSoundCloudSet(t *testing.T) {
	m, err := resolveFixture(t, "https://soundcloud.com/lofistudymusic/sets/study-music-lofi-study-music",
		route{url: "soundcloud.com/lofistudymusic/sets/study-music-lofi-study-music", file: "soundcloud/set.html"},
		route{url: "api-v2.soundcloud.com/tracks", query: "ids=", file: "soundcloud/tracks.json"})
	if err != nil {
		t.Fatal(err)
	}
	checkMedia(t, m, "soundcloud")
	if len(m.Items) != 30 || len(m.Formats) != 0 || m.Items[29].Kind != "audio" || !strings.HasPrefix(m.Items[29].URL, "https://soundcloud.com/") {
		t.Fatalf("set: %d items, last %+v", len(m.Items), m.Items[len(m.Items)-1])
	}
}

func TestSoundCloudClientIDFromBundle(t *testing.T) {
	page := `<script crossorigin src="https://a-v2.sndcdn.com/assets/0-abc.js"></script>
<script>window.__sc_hydration = [{"hydratable":"sound","data":{"id":1,"title":"T","permalink_url":"https://soundcloud.com/a/t","duration":1000,
"media":{"transcodings":[{"url":"https://api-v2.soundcloud.com/media/soundcloud:tracks:1/p/stream/progressive","preset":"mp3_1_0","format":{"protocol":"progressive","mime_type":"audio/mpeg"}}]}}}];</script>`
	m, err := resolveFixture(t, "https://soundcloud.com/a/t",
		route{url: "soundcloud.com/a/t", text: page},
		route{url: "a-v2.sndcdn.com/assets/0-abc.js", text: `x={client_id:"AbCdEfGhIjKlMnOpQrStUvWx12345678"}`},
		route{url: "api-v2.soundcloud.com/media/soundcloud:tracks:1/p/stream/progressive", query: "client_id=AbCdEfGhIjKlMnOpQrStUvWx12345678",
			text: `{"url":"https://cf-media.sndcdn.com/t.128.mp3?Policy=x"}`})
	if err != nil {
		t.Fatal(err)
	}
	f := m.Formats[0]
	if f.Ext != "mp3" || f.Parts[0].Kind != plan.PartHTTP || f.Bitrate != 128000 {
		t.Fatalf("format = %+v", f)
	}
	_, err = resolveFixture(t, "https://soundcloud.com/a/gone", route{url: "soundcloud.com/a/gone", status: 404})
	if !errors.Is(err, media.ErrUnavailable) {
		t.Fatalf("missing: %v", err)
	}
}

func TestVimeo(t *testing.T) {
	m, err := resolveFixture(t, "https://vimeo.com/channels/staffpicks/76979871",
		route{url: "player.vimeo.com/video/76979871/config", file: "vimeo/config.json"},
		route{url: "vod-adaptive-ak.vimeocdn.com/fixture/v2/playlist/av/primary/playlist.m3u8", file: "vimeo/master.m3u8"})
	if err != nil {
		t.Fatal(err)
	}
	checkMedia(t, m, "vimeo")
	d := findFormat(m, m.Default)
	if d.Height != 720 || len(d.Parts) != 2 || d.Parts[1].Role != "audio" || m.Thumb == "" || m.Duration != 62 {
		t.Fatalf("vimeo = %+v default %+v", m, d)
	}
	_, err = resolveFixture(t, "https://player.vimeo.com/video/1234567",
		route{url: "player.vimeo.com/video/1234567/config", status: 403, text: `{"message":"private"}`})
	if !errors.Is(err, media.ErrPrivate) {
		t.Fatalf("private: %v", err)
	}
	_, err = resolveFixture(t, "https://vimeo.com/7654321/abcdef1234",
		route{url: "player.vimeo.com/video/7654321/config", query: "h=abcdef1234", status: 404})
	if !errors.Is(err, media.ErrUnavailable) {
		t.Fatalf("missing: %v", err)
	}
}

func TestDailymotion(t *testing.T) {
	m, err := resolveFixture(t, "https://dai.ly/x5kesuj",
		route{url: "www.dailymotion.com/player/metadata/video/x5kesuj", file: "dailymotion/x5kesuj.json"},
		route{url: "cdndirector.dailymotion.com/cdn/manifest/video/x5kesuj.m3u8", file: "dailymotion/master.m3u8"})
	if err != nil {
		t.Fatal(err)
	}
	checkMedia(t, m, "dailymotion")
	if len(m.Formats) != 2 || findFormat(m, m.Default).Height != 480 || m.Author != "Deadline" || !strings.Contains(m.Thumb, "x1080") ||
		m.Formats[0].Parts[0].Headers.Get("Referer") != "https://www.dailymotion.com/" {
		t.Fatalf("dailymotion = %+v", m)
	}
	_, err = resolveFixture(t, "https://www.dailymotion.com/video/x0000zz",
		route{url: "www.dailymotion.com/player/metadata/video/x0000zz",
			text: `{"error":{"code":"404","message":"Can't find object video","type":"not_found"},"is_password_protected":false}`})
	if !errors.Is(err, media.ErrUnavailable) {
		t.Fatalf("missing: %v", err)
	}
}

func TestPinterestVideo(t *testing.T) {
	m, err := resolveFixture(t, "https://www.pinterest.ca/pin/origami--664281013778109217/",
		route{url: "www.pinterest.com/resource/PinResource/get/", query: "664281013778109217", file: "pinterest/video.json"},
		route{url: "v1.pinimg.com/videos/mc/hls/12/83/f0/1283f06c1c8fa040011cd7231f33f069.m3u8", file: "pinterest/video.m3u8"})
	if err != nil {
		t.Fatal(err)
	}
	checkMedia(t, m, "pinterest")
	if m.Default != "v_720p" || m.Title != "Origami" || m.Duration < 57 || len(m.Formats) < 3 {
		t.Fatalf("pin = %+v", m)
	}
}

func TestPinterestStoryImageAndMissing(t *testing.T) {
	m, err := resolveFixture(t, "https://pin.it/abc",
		route{url: "pin.it/abc", status: 302, header: map[string]string{"Location": "https://api.pinterest.com/url_shortener/abc/redirect/"}},
		route{url: "api.pinterest.com/url_shortener/abc/redirect/", status: 302, header: map[string]string{"Location": "https://www.pinterest.com/pin/1084663891475263837/sent/"}},
		route{url: "www.pinterest.com/pin/1084663891475263837/sent/", text: "<html></html>"},
		route{url: "www.pinterest.com/resource/PinResource/get/", query: "1084663891475263837", file: "pinterest/story.json"},
		route{url: "v1.pinimg.com/videos/mc/hls/c8/67/44/c86744113f242e233ac4470b394eddf4.m3u8", file: "pinterest/story.m3u8"})
	if err != nil {
		t.Fatal(err)
	}
	checkMedia(t, m, "pinterest")
	for _, f := range m.Formats {
		if strings.Contains(f.Parts[0].URL, "_t1.mp4") {
			t.Fatalf("lower tier kept: %s", f.Parts[0].URL)
		}
	}
	if findFormat(m, "v_exp7") == nil || m.Default != "v_exp7" {
		t.Fatalf("story formats = %+v", m.Formats)
	}
	m, err = resolveFixture(t, "https://www.pinterest.com/pin/123456789012/",
		route{url: "www.pinterest.com/resource/PinResource/get/",
			text: `{"resource_response":{"status":"success","data":{"title":"Lake","domain":"Uploaded by user","images":{"orig":{"url":"https://i.pinimg.com/originals/aa/bb/cc/lake.jpg"}}}}}`})
	if err != nil || m.Kind != plan.KindFile || m.Name != "Lake.jpg" {
		t.Fatalf("image pin = %+v, %v", m, err)
	}
	_, err = resolveFixture(t, "https://www.pinterest.com/pin/1/",
		route{url: "www.pinterest.com/resource/PinResource/get/", text: `{"resource_response":{"status":"failure","data":null}}`})
	if !errors.Is(err, media.ErrUnavailable) {
		t.Fatalf("missing: %v", err)
	}
}

func tumblrRoutes(blog, id, file string) []route {
	return []route{
		{url: "www.tumblr.com/login", file: "tumblr/login.html"},
		{url: "www.tumblr.com/api/v2/blog/" + blog + "/posts/" + id + "/permalink", file: file},
	}
}

func TestTumblrVideo(t *testing.T) {
	m, err := resolveFixture(t, "https://maskofthedragon.tumblr.com/post/626907179849564160/mona-talking-in-english",
		tumblrRoutes("maskofthedragon", "626907179849564160", "tumblr/video.json")...)
	if err != nil {
		t.Fatal(err)
	}
	checkMedia(t, m, "tumblr")
	if len(m.Items) != 0 || len(m.Formats) != 1 || !strings.HasPrefix(m.Formats[0].Parts[0].URL, "https://va.media.tumblr.com/") || m.Author != "maskofthedragon" {
		t.Fatalf("tumblr video = %+v", m)
	}
}

func TestTumblrImagesAndMissing(t *testing.T) {
	m, err := resolveFixture(t, "https://www.tumblr.com/staff/828009069026721792/premium-just-got-better",
		tumblrRoutes("staff", "828009069026721792", "tumblr/images.json")...)
	if err != nil {
		t.Fatal(err)
	}
	checkMedia(t, m, "tumblr")
	if len(m.Items) != 4 || len(m.Formats) != 0 || m.Items[0].Kind != "photo" || !strings.Contains(m.Items[0].URL, "media.tumblr.com") {
		t.Fatalf("tumblr images = %+v", m.Items)
	}
	_, err = resolveFixture(t, "https://staff.tumblr.com/post/1",
		route{url: "www.tumblr.com/login", file: "tumblr/login.html"},
		route{url: "www.tumblr.com/api/v2/blog/staff/posts/1/permalink", status: 404, text: `{"meta":{"status":404}}`})
	if !errors.Is(err, media.ErrUnavailable) {
		t.Fatalf("missing: %v", err)
	}
	if _, err := resolveFixture(t, "https://staff.tumblr.com/archive"); !errors.Is(err, media.ErrUnsupported) {
		t.Fatalf("archive: %v", err)
	}
}

func TestRedditVideo(t *testing.T) {
	m, err := resolveFixture(t, "https://v.redd.it/zv89llsvexdz",
		route{url: "v.redd.it/zv89llsvexdz/HLSPlaylist.m3u8", file: "reddit/master.m3u8"})
	if err != nil {
		t.Fatal(err)
	}
	checkMedia(t, m, "reddit")
	d := findFormat(m, m.Default)
	if d.Height != 640 || len(d.Parts) != 2 || d.Parts[1].Role != "audio" {
		t.Fatalf("default = %+v", d)
	}
	_, err = resolveFixture(t, "https://v.redd.it/aaaaaaaaaaaa", route{url: "v.redd.it/aaaaaaaaaaaa/HLSPlaylist.m3u8", status: 404})
	if !errors.Is(err, media.ErrUnavailable) {
		t.Fatalf("missing: %v", err)
	}
	if s := media.Lookup(mustURL(t, "https://www.reddit.com/r/videos/comments/6rrwyj/x/")); s != nil && s.ID() == "reddit" {
		t.Fatal("reddit post pages are left to the generic chain")
	}
}
