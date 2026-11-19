package services

import (
	"errors"
	"net/http"
	"strings"
	"testing"

	"ryoget/internal/media"
	"ryoget/internal/plan"
)

func TestBilibiliParts(t *testing.T) {
	m, err := resolveFixture(t, "https://www.bilibili.com/video/BV1bK411W797?p=2&spm_id_from=x",
		route{url: "www.bilibili.com/video/BV1bK411W797/", query: "p=2", file: "bilibili/parts.html"},
		route{url: "api.bilibili.com/x/player/playurl", query: "cid=191559297", file: "bilibili/playurl.json"})
	if err != nil {
		t.Fatal(err)
	}
	checkMedia(t, m, "bilibili")
	if len(m.Items) != 23 || m.Items[1].URL != "https://www.bilibili.com/video/BV1bK411W797/?p=2" || !strings.HasPrefix(m.Thumb, "https://") {
		t.Fatalf("items %d, %+v", len(m.Items), m.Items[1])
	}
	if m.Default != "dash-32-avc" || m.Headers["Referer"] != "https://www.bilibili.com/" {
		t.Fatalf("default %s headers %v", m.Default, m.Headers)
	}
	d := findFormat(m, m.Default)
	if d.Height != 480 || len(d.Parts) != 2 || d.Parts[1].Role != "audio" {
		t.Fatalf("default = %+v", d)
	}
	p, err := media.PlanFor(m, media.Options{})
	if err != nil || p.Post != plan.PostMerge || p.Parts[0].Headers.Get("Referer") == "" {
		t.Fatalf("plan = %+v, %v", p, err)
	}
	_, err = resolveFixture(t, "https://www.bilibili.com/video/BV1bK411W797",
		route{url: "www.bilibili.com/video/BV1bK411W797/", file: "bilibili/parts.html"},
		route{url: "api.bilibili.com/x/player/playurl", text: `{"code":-404,"message":"啥都木有"}`})
	if !errors.Is(err, media.ErrUnavailable) {
		t.Fatalf("missing: %v", err)
	}
}

func TestLoomFiles(t *testing.T) {
	m, err := resolveFixture(t, "https://www.loom.com/share/c43a642f815f4378b6f80a889bb73d8d?sid=1",
		route{method: "POST", url: "www.loom.com/graphql", body: "c43a642f815f4378b6f80a889bb73d8d", file: "loom/meta-mp4.json"},
		route{method: "POST", url: "www.loom.com/api/campaigns/sessions/c43a642f815f4378b6f80a889bb73d8d/transcoded-url", file: "loom/transcoded.json"},
		route{method: "POST", url: "www.loom.com/api/campaigns/sessions/c43a642f815f4378b6f80a889bb73d8d/raw-url", file: "loom/raw-webm.json"})
	if err != nil {
		t.Fatal(err)
	}
	checkMedia(t, m, "loom")
	if len(m.Formats) != 2 || m.Formats[1].Ext != "webm" || m.Title != "Lilah Nielsen Intro Video" || m.Duration != 20 {
		t.Fatalf("loom = %+v", m)
	}
}

func TestLoomHLS(t *testing.T) {
	m, err := resolveFixture(t, "https://www.loom.com/embed/43d05f362f734614a2e81b4694a3a523",
		route{method: "POST", url: "www.loom.com/graphql", file: "loom/meta-hls.json"},
		route{method: "POST", url: "www.loom.com/api/campaigns/sessions/43d05f362f734614a2e81b4694a3a523/transcoded-url", status: 204},
		route{method: "POST", url: "www.loom.com/api/campaigns/sessions/43d05f362f734614a2e81b4694a3a523/raw-url", file: "loom/raw-hls.json"},
		route{url: "luna.loom.com/id/43d05f362f734614a2e81b4694a3a523/rev/8edb6ff5103625e5f83b27b550c50199bcd4dca18d82b72e43dab38c84c70e793/resource/hls/playlist-split.m3u8",
			file: "loom/master.m3u8"})
	if err != nil {
		t.Fatal(err)
	}
	checkMedia(t, m, "loom")
	d := findFormat(m, m.Default)
	if len(d.Parts) != 2 || !strings.HasPrefix(d.Parts[0].Headers.Get("Cookie"), "CloudFront-Policy=") || !strings.Contains(d.Parts[1].Headers.Get("Cookie"), "CloudFront-Key-Pair-Id=") {
		t.Fatalf("parts = %+v", d.Parts)
	}
	_, err = resolveFixture(t, "https://www.loom.com/share/00000000000000000000000000000000",
		route{method: "POST", url: "www.loom.com/graphql", text: `{"data":{"getVideo":{"__typename":"PrivateVideo","id":"x","status":403}}}`})
	if !errors.Is(err, media.ErrPrivate) {
		t.Fatalf("private: %v", err)
	}
}

func TestOKRu(t *testing.T) {
	m, err := resolveFixture(t, "https://ok.ru/video/20079905452",
		route{url: "ok.ru/videoembed/20079905452", file: "okru/videoembed.html"})
	if err != nil {
		t.Fatal(err)
	}
	checkMedia(t, m, "okru")
	if len(m.Formats) != 5 || m.Default != "hd" || findFormat(m, "hd").Height != 720 || m.Headers["User-Agent"] != browserUA || m.Duration != 100 {
		t.Fatalf("ok = %+v", m)
	}
	if !strings.HasPrefix(m.Title, "Культура") {
		t.Fatalf("title = %q", m.Title)
	}
	_, err = resolveFixture(t, "https://ok.ru/video/1", route{url: "ok.ru/videoembed/1", text: `<div class="vp_video_stub_txt">Video has been blocked</div>`})
	if !errors.Is(err, media.ErrUnavailable) {
		t.Fatalf("missing: %v", err)
	}
}

func TestRutube(t *testing.T) {
	m, err := resolveFixture(t, "https://rutube.ru/play/embed/3eac3b4561676c17df9132a9a1e62e3e",
		route{url: "rutube.ru/api/play/options/3eac3b4561676c17df9132a9a1e62e3e/", file: "rutube/options.json"},
		route{url: "bl.rutube.ru/route/3eac3b4561676c17df9132a9a1e62e3e.m3u8", file: "rutube/master.m3u8"})
	if err != nil {
		t.Fatal(err)
	}
	checkMedia(t, m, "rutube")
	if m.Duration != 81 || m.Author != "NTDRussian" || m.Formats[0].Parts[0].Kind != plan.PartHLS {
		t.Fatalf("rutube = %+v", m)
	}
	_, err = resolveFixture(t, "https://rutube.ru/video/00000000000000000000000000000000/",
		route{url: "rutube.ru/api/play/options/00000000000000000000000000000000/", status: 404, text: `{"detail":{"name":"Not found","description":"Видео не найдено"}}`})
	if !errors.Is(err, media.ErrUnavailable) || !strings.Contains(err.Error(), "Видео") {
		t.Fatalf("missing: %v", err)
	}
}

func TestVK(t *testing.T) {
	m, err := resolveFixture(t, "https://vk.com/videos-77521?z=video-77521_162222515%2Fclub77521",
		route{method: "POST", url: "vk.com/al_video.php", body: "video=-77521_162222515", file: "vk/al_video.json",
			header: map[string]string{"Content-Type": "application/json; charset=windows-1251"}})
	if err != nil {
		t.Fatal(err)
	}
	checkMedia(t, m, "vk")
	if m.Title != "ProtivoGunz - Хуёвая песня" || m.Author != "Noize MC" || m.Duration != 195 || m.Default != "mp4-240" {
		t.Fatalf("vk = %+v", m)
	}
	_, err = resolveFixture(t, "https://vk.com/video1_2",
		route{method: "POST", url: "vk.com/al_video.php", text: `{"payload":["3",["login"]]}`})
	if !errors.Is(err, media.ErrPrivate) {
		t.Fatalf("login: %v", err)
	}
	_, err = resolveFixture(t, "https://vk.com/video1_3",
		route{method: "POST", url: "vk.com/al_video.php", text: `{"payload":["8",["\"<b>Access denied</b>\""]]}`})
	if !errors.Is(err, media.ErrUnavailable) {
		t.Fatalf("denied: %v", err)
	}
}

func TestFacebook(t *testing.T) {
	m, err := resolveFixture(t, "https://www.facebook.com/cnn/videos/10155529876156509/",
		route{url: "www.facebook.com/cnn/videos/10155529876156509/", file: "facebook/video.html"})
	if err != nil {
		t.Fatal(err)
	}
	checkMedia(t, m, "facebook")
	if len(m.Formats) != 2 || m.Default != "hd" || m.Duration < 40 || strings.Contains(m.Title, "views") || !strings.HasPrefix(m.Title, "She survived") {
		t.Fatalf("fb = %+v", m)
	}
	if strings.Contains(m.Formats[0].Parts[0].URL, `\/`) {
		t.Fatalf("url not unescaped: %s", m.Formats[0].Parts[0].URL)
	}
	_, err = resolveFixture(t, "https://fb.watch/abc/",
		route{url: "fb.watch/abc/", status: http.StatusFound, header: map[string]string{"Location": "https://www.facebook.com/watch/?v=42424242"}},
		route{url: "www.facebook.com/watch/", query: "v=42424242", text: `<form id="login_form"></form>`})
	if !errors.Is(err, media.ErrPrivate) {
		t.Fatalf("login wall: %v", err)
	}
}
