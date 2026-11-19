package services

import (
	"errors"
	"strings"
	"testing"

	"ryoget/internal/media"
	"ryoget/internal/plan"
)

func igRoutes(post string) []route {
	return []route{
		{url: "www.instagram.com/", file: "instagram/home.html", header: map[string]string{"Set-Cookie": "csrftoken=abc; Path=/"}},
		{method: "POST", url: "www.instagram.com/api/graphql", body: "doc_id=" + igDocID, file: post},
	}
}

func TestInstagramPK(t *testing.T) {
	if pk, _ := igPK("aye83DjauH"); pk != "482584233761418119" {
		t.Fatalf("pk = %s", pk)
	}
}

func TestInstagramReel(t *testing.T) {
	m, err := resolveFixture(t, "https://www.instagram.com/reel/Cf0q4O4tt9f/?igsh=x", igRoutes("instagram/reel.json")...)
	if err != nil {
		t.Fatal(err)
	}
	checkMedia(t, m, "instagram")
	if m.Headers["Referer"] != "https://www.instagram.com/" || m.Duration < 50 {
		t.Fatalf("headers %v duration %v", m.Headers, m.Duration)
	}
	d := findFormat(m, m.Default)
	if d.Height != 1920 || len(d.Parts) != 2 || d.Parts[0].Role != "video" || d.Parts[1].Role != "audio" {
		t.Fatalf("default = %+v", d)
	}
	var audio int
	for _, f := range m.Formats {
		if f.Kind == "audio" {
			audio++
		}
	}
	if audio != 1 || !strings.HasPrefix(m.Formats[0].ID, "mp4-") || !m.Formats[0].HasAudio {
		t.Fatalf("formats = %+v", m.Formats)
	}
	p, err := media.PlanFor(m, media.Options{})
	if err != nil || p.Post != plan.PostMerge {
		t.Fatalf("plan = %+v, %v", p, err)
	}
}

func TestInstagramCarousel(t *testing.T) {
	m, err := resolveFixture(t, "https://www.instagram.com/p/DbjY8EVEskh/", igRoutes("instagram/carousel.json")...)
	if err != nil {
		t.Fatal(err)
	}
	checkMedia(t, m, "instagram")
	if len(m.Items) < 2 || len(m.Formats) != 0 || m.Items[0].Kind != "photo" || !strings.Contains(m.Items[0].URL, "cdninstagram.com") {
		t.Fatalf("items = %+v formats %d", m.Items, len(m.Formats))
	}
	m, err = resolveFixture(t, "https://www.instagram.com/p/DbjY8EVEskh/?img_index=2", igRoutes("instagram/carousel.json")...)
	if err != nil || m.Kind != plan.KindFile {
		t.Fatalf("img_index=2: %+v, %v", m, err)
	}
}

func TestInstagramErrors(t *testing.T) {
	routes := []route{
		{url: "www.instagram.com/", file: "instagram/home.html"},
		{method: "POST", url: "www.instagram.com/api/graphql", text: `{"data":{"xig_polaris_media":{"__typename":"XIGPolarisImageMedia","if_not_gated_logged_out":null}}}`},
	}
	if _, err := resolveFixture(t, "https://www.instagram.com/p/DbjY8EVEskh/", routes...); !errors.Is(err, media.ErrPrivate) {
		t.Fatalf("gated: %v", err)
	}
	routes[1].text = `{"data":{"xig_polaris_media":null}}`
	if _, err := resolveFixture(t, "https://www.instagram.com/p/DbjY8EVEskh/", routes...); !errors.Is(err, media.ErrUnavailable) {
		t.Fatalf("missing: %v", err)
	}
	routes[1].text = `<!DOCTYPE html><html></html>`
	if _, err := resolveFixture(t, "https://www.instagram.com/p/DbjY8EVEskh/", routes...); !errors.Is(err, media.ErrRateLimited) {
		t.Fatalf("html: %v", err)
	}
	if _, err := resolveFixture(t, "https://www.instagram.com/stories/someone/123/"); !errors.Is(err, media.ErrPrivate) {
		t.Fatalf("story: %v", err)
	}
}

func TestTikTokVideo(t *testing.T) {
	m, err := resolveFixture(t, "https://vm.tiktok.com/ZMabc/",
		route{url: "vm.tiktok.com/ZMabc/", status: 301, header: map[string]string{"Location": "https://www.tiktok.com/@patroxofficial/video/6742501081818877190?_r=1"}},
		route{url: "www.tiktok.com/@i/video/6742501081818877190", file: "tiktok/video.html",
			header: map[string]string{"Set-Cookie": "tt_chain_token=chain; Path=/; Domain=.tiktok.com"}})
	if err != nil {
		t.Fatal(err)
	}
	checkMedia(t, m, "tiktok")
	if m.Headers["Cookie"] != "tt_chain_token=chain" || m.Headers["Referer"] != "https://www.tiktok.com/" {
		t.Fatalf("headers = %v", m.Headers)
	}
	if len(m.Formats) != 3 || m.Default != "normal_540_0" || m.Formats[0].Size != 2748647 || m.Author != "patroX" || m.Duration != 27 {
		t.Fatalf("media = %+v", m)
	}
}

func TestTikTokPhotoAndErrors(t *testing.T) {
	m, err := resolveFixture(t, "https://www.tiktok.com/@someone/photo/7300000000000000001",
		route{url: "www.tiktok.com/@i/video/7300000000000000001", file: "tiktok/photo.html"})
	if err != nil {
		t.Fatal(err)
	}
	checkMedia(t, m, "tiktok")
	if len(m.Items) != 3 || m.Items[2].URL != "https://p16-sign.tiktokcdn-us.com/photo-3.jpeg" || m.Formats[0].Kind != "audio" {
		t.Fatalf("photo post = %+v", m)
	}
	_, err = resolveFixture(t, "https://www.tiktok.com/@x/video/7139980461132074283",
		route{url: "www.tiktok.com/@i/video/7139980461132074283", file: "tiktok/deleted.html"})
	if !errors.Is(err, media.ErrUnavailable) {
		t.Fatalf("deleted: %v", err)
	}
	_, err = resolveFixture(t, "https://www.tiktok.com/@x/video/7139980461132074283",
		route{url: "www.tiktok.com/@i/video/7139980461132074283", text: "<html>verify</html>"})
	if !errors.Is(err, media.ErrRateLimited) {
		t.Fatalf("challenge: %v", err)
	}
}

const bskyMaster = "video.bsky.app/watch/did%3Aplc%3Az72i7hdynmk6r22z27h6tvur/bafkreih7qpo7ef7hhtm7ehi7paimlieddrn5argpq4eayz6teez2qbsr7m/"

func TestBlueskyVideo(t *testing.T) {
	m, err := resolveFixture(t, "https://bsky.app/profile/bsky.app/post/3l3vgf77uco2g",
		route{url: "public.api.bsky.app/xrpc/app.bsky.feed.getPostThread", query: "3l3vgf77uco2g", file: "bluesky/video.json"},
		route{url: "video.bsky.app/watch/did:plc:z72i7hdynmk6r22z27h6tvur/bafkreih7qpo7ef7hhtm7ehi7paimlieddrn5argpq4eayz6teez2qbsr7m/playlist.m3u8", file: "bluesky/master.m3u8"},
		route{url: "video.bsky.app/watch/did:plc:z72i7hdynmk6r22z27h6tvur/bafkreih7qpo7ef7hhtm7ehi7paimlieddrn5argpq4eayz6teez2qbsr7m/720p/video.m3u8", file: "bluesky/720p.m3u8"},
		route{url: "video.bsky.app/watch/did:plc:z72i7hdynmk6r22z27h6tvur/bafkreih7qpo7ef7hhtm7ehi7paimlieddrn5argpq4eayz6teez2qbsr7m/360p/video.m3u8", file: "bluesky/720p.m3u8"})
	if err != nil {
		t.Fatal(err)
	}
	checkMedia(t, m, "bluesky")
	if len(m.Formats) < 2 || m.Formats[0].Parts[0].Kind != plan.PartHLS || findFormat(m, m.Default).Height != 720 {
		t.Fatalf("formats = %+v", m.Formats)
	}
}

func TestBlueskyImagesAndMissing(t *testing.T) {
	m, err := resolveFixture(t, "https://bsky.app/profile/bsky.app/post/3myx7vphq3s2j",
		route{url: "public.api.bsky.app/xrpc/app.bsky.feed.getPostThread", file: "bluesky/images.json"})
	if err != nil {
		t.Fatal(err)
	}
	checkMedia(t, m, "bluesky")
	if len(m.Items) != 4 || !strings.Contains(m.Items[0].URL, "feed_fullsize") || len(m.Formats) != 0 {
		t.Fatalf("items = %+v", m.Items)
	}
	_, err = resolveFixture(t, "https://bsky.app/profile/bsky.app/post/nope",
		route{url: "public.api.bsky.app/xrpc/app.bsky.feed.getPostThread", status: 400, text: `{"error":"NotFound","message":"Post not found"}`})
	if !errors.Is(err, media.ErrUnavailable) {
		t.Fatalf("missing: %v", err)
	}
}

func TestStreamable(t *testing.T) {
	m, err := resolveFixture(t, "https://streamable.com/e/dnd1",
		route{url: "api.streamable.com/videos/dnd1", file: "streamable/dnd1.json"})
	if err != nil {
		t.Fatal(err)
	}
	checkMedia(t, m, "streamable")
	if m.Default != "mp4" || findFormat(m, "mp4").Size != 19286494 || !strings.HasPrefix(m.Thumb, "https://") {
		t.Fatalf("media = %+v", m)
	}
	_, err = resolveFixture(t, "https://streamable.com/zzzz", route{url: "api.streamable.com/videos/zzzz", status: 404})
	if !errors.Is(err, media.ErrUnavailable) {
		t.Fatalf("missing: %v", err)
	}
}

func TestTwitchClip(t *testing.T) {
	m, err := resolveFixture(t, "https://www.twitch.tv/ea/clip/FaintLightGullWholeWheat?filter=clips",
		route{method: "POST", url: "gql.twitch.tv/gql", body: "FaintLightGullWholeWheat", file: "twitch/clip.json"})
	if err != nil {
		t.Fatal(err)
	}
	checkMedia(t, m, "twitch")
	p := m.Formats[0].Parts[0]
	if !strings.Contains(p.URL, "index.mp4?sig=") || !strings.Contains(p.URL, "&token=") || m.Author != "EA" {
		t.Fatalf("clip = %+v", m)
	}
	_, err = resolveFixture(t, "https://clips.twitch.tv/NoSuchClip",
		route{method: "POST", url: "gql.twitch.tv/gql", text: `{"data":{"clip":null}}`})
	if !errors.Is(err, media.ErrUnavailable) {
		t.Fatalf("missing: %v", err)
	}
	if _, err := resolveFixture(t, "https://www.twitch.tv/riotgames"); !errors.Is(err, media.ErrUnsupported) {
		t.Fatalf("channel: %v", err)
	}
}

func TestTwitchVOD(t *testing.T) {
	m, err := resolveFixture(t, "https://www.twitch.tv/videos/635475444",
		route{method: "POST", url: "gql.twitch.tv/gql", body: "videoPlaybackAccessToken", file: "twitch/vod.json"},
		route{url: "usher.ttvnw.net/vod/635475444.m3u8", query: "sig=", file: "twitch/usher.m3u8"},
		route{url: "d2nvs31859zcd8.cloudfront.net/2baa3119c3dfe03c433a_riotgames_38359486704_1466356187/chunked/index-dvr.m3u8", file: "twitch/chunked.m3u8"})
	if err != nil {
		t.Fatal(err)
	}
	checkMedia(t, m, "twitch")
	if m.Duration != 11643 || m.Author != "Riot Games" || findFormat(m, m.Default).Height != 1080 {
		t.Fatalf("vod = %+v", m)
	}
	var audioOnly bool
	for _, f := range m.Formats {
		audioOnly = audioOnly || (f.Kind == "audio" && !f.HasVideo)
	}
	if !audioOnly {
		t.Fatal("no audio-only rendition")
	}
	_, err = resolveFixture(t, "https://www.twitch.tv/videos/1",
		route{method: "POST", url: "gql.twitch.tv/gql", text: `{"data":{"video":{"title":"x"},"videoPlaybackAccessToken":{"value":"{\"authorization\":{\"forbidden\":true}}","signature":"s"}}}`})
	if !errors.Is(err, media.ErrPrivate) {
		t.Fatalf("sub-only: %v", err)
	}
}
