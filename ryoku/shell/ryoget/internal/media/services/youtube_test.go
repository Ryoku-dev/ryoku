package services

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync/atomic"
	"testing"

	"ryoget/internal/media"
	"ryoget/internal/plan"
	"ryoget/internal/youtube"
)

// ytTransport answers innertube from the youtube package's fixtures.
type ytTransport struct {
	player  string
	players atomic.Int32
	nexts   atomic.Int32
	browses atomic.Int32
}

func (y *ytTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	var body []byte
	if req.Body != nil {
		body, _ = io.ReadAll(req.Body)
	}
	file := ""
	switch {
	case req.URL.Host != "www.youtube.com":
		return &http.Response{StatusCode: 502, Body: io.NopCloser(strings.NewReader("")), Request: req}, nil
	case req.URL.Path == "/sw.js_data":
		file = "swjs_data.txt"
	case req.URL.Path == "/youtubei/v1/player":
		y.players.Add(1)
		file = y.player
	case req.URL.Path == "/youtubei/v1/browse" && strings.Contains(string(body), "PAGE2TOKEN"):
		file = "browse_continuation.json"
	case req.URL.Path == "/youtubei/v1/browse":
		y.browses.Add(1)
		file = "browse_playlist.json"
	case req.URL.Path == "/youtubei/v1/next":
		y.nexts.Add(1)
		file = "next_playlist.json"
	}
	data := []byte(file)
	if !strings.HasPrefix(file, "{") {
		var err error
		if data, err = os.ReadFile("../../youtube/testdata/" + file); file == "" || err != nil {
			return &http.Response{StatusCode: 404, Body: io.NopCloser(strings.NewReader("")), Request: req}, nil
		}
	}
	return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": {"application/json"}},
		Body: io.NopCloser(strings.NewReader(string(data))), Request: req}, nil
}

func ytEnv(t *testing.T, player string) (*plan.Env, *ytTransport) {
	tr := &ytTransport{player: player}
	return &plan.Env{HTTP: &http.Client{Transport: tr}, Cache: t.TempDir()}, tr
}

func ytStubFallback(t *testing.T, err error) *atomic.Int32 {
	var calls atomic.Int32
	old := ytResolveFallback
	ytResolveFallback = func(ctx context.Context, env *plan.Env, raw string) (*media.Media, error) {
		calls.Add(1)
		if err != nil {
			return nil, err
		}
		return &media.Media{Kind: plan.KindMedia, Title: "from yt-dlp " + raw, Source: media.Source{ID: "ytdlp", Name: "Youtube"}}, nil
	}
	t.Cleanup(func() { ytResolveFallback = old })
	return &calls
}

func ytResolve(env *plan.Env, raw string, allow bool) (*media.Media, error) {
	u, _ := url.Parse(raw)
	return ytService{}.Resolve(media.WithYTDLP(context.Background(), allow), u, env)
}

func TestYouTubeNative(t *testing.T) {
	env, _ := ytEnv(t, "player_ok.json")
	calls := ytStubFallback(t, nil)
	m, err := ytResolve(env, "https://youtu.be/aqz-KE-bpKQ", true)
	if err != nil {
		t.Fatal(err)
	}
	if m.Source.ID != "youtube" || m.URL != "https://www.youtube.com/watch?v=aqz-KE-bpKQ" || len(m.Formats) == 0 || calls.Load() != 0 {
		t.Fatalf("media = %+v", m)
	}
	m, err = ytResolve(env, "https://music.youtube.com/watch?v=aqz-KE-bpKQ", true)
	if err != nil || m.Source.Name != "YouTube Music" || !strings.HasPrefix(m.URL, "https://music.youtube.com/") {
		t.Fatalf("music = %+v, %v", m, err)
	}
}

func TestYouTubePlaylistItems(t *testing.T) {
	env, tr := ytEnv(t, "player_ok.json")
	ytStubFallback(t, nil)
	m, err := ytResolve(env, "https://www.youtube.com/playlist?list=UUSMOQeBJ2RAnuFungnQOxLg", true)
	if err != nil {
		t.Fatal(err)
	}
	if tr.browses.Load() != 1 || len(m.Items) != 5 || m.Note != "1596 videos, the first 5 listed" || len(m.Formats) == 0 {
		t.Fatalf("items %d note %q formats %d after %d browses", len(m.Items), m.Note, len(m.Formats), tr.browses.Load())
	}
	first := m.Items[0]
	if first.Kind != "video" || first.URL != "https://www.youtube.com/watch?v=ljj8nhLYt6A" || first.Title == "" || first.Thumb == "" ||
		m.URL != first.URL {
		t.Fatalf("first item %+v, media url %s", first, m.URL)
	}
	pl, err := media.PlanFor(m, media.Options{})
	if err != nil || pl.Post != plan.PostMerge {
		t.Fatalf("default plan = %+v, %v", pl, err)
	}
	if pl, err = media.PlanFor(m, media.Options{Items: []int{0}}); err != nil || pl.Post != plan.PostMerge {
		t.Fatalf("first item plan = %+v, %v", pl, err)
	}
	if _, err = media.PlanFor(m, media.Options{Items: []int{0, 3}}); !errors.Is(err, media.ErrUnsupported) {
		t.Fatalf("whole playlist plan err = %v (engine expands page items)", err)
	}

	m, err = ytResolve(env, "https://www.youtube.com/watch?v=aqz-KE-bpKQ&list=UUSMOQeBJ2RAnuFungnQOxLg", true)
	if err != nil || len(m.Items) != 5 || m.URL != "https://www.youtube.com/watch?v=aqz-KE-bpKQ" {
		t.Fatalf("watch in list = %+v, %v", m, err)
	}
	before := tr.browses.Load()
	m, err = ytResolve(env, "https://music.youtube.com/watch?v=aqz-KE-bpKQ&list=RDAMVMaqz-KE-bpKQ", true)
	if err != nil || len(m.Items) != 0 || tr.browses.Load() != before || !strings.HasPrefix(m.URL, "https://music.youtube.com/") {
		t.Fatalf("mix = %+v, %v", m, err)
	}
}

func TestYouTubeFallback(t *testing.T) {
	env, tr := ytEnv(t, "player_bot.json")
	calls := ytStubFallback(t, nil)
	m, err := ytResolve(env, "https://www.youtube.com/shorts/aqz-KE-bpKQ", true)
	if err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 1 || m.Source.ID != "youtube" || m.Title != "from yt-dlp https://www.youtube.com/watch?v=aqz-KE-bpKQ" {
		t.Fatalf("fallback media = %+v after %d calls", m, calls.Load())
	}
	if tr.players.Load() != 2 {
		t.Fatalf("native tried %d times; want a visitor refresh and one retry", tr.players.Load())
	}

	env2, _ := ytEnv(t, "player_bot.json")
	_, err = ytResolve(env2, "https://www.youtube.com/watch?v=aqz-KE-bpKQ", false)
	if !errors.Is(err, youtube.ErrBotGate) || calls.Load() != 1 {
		t.Fatalf("not allowed: %v, %d calls", err, calls.Load())
	}
}

func TestYouTubeFallbackKeepsNativeReasonWithoutYTDLP(t *testing.T) {
	env, _ := ytEnv(t, "player_bot.json")
	ytStubFallback(t, media.ErrUnsupported)
	_, err := ytResolve(env, "https://www.youtube.com/watch?v=aqz-KE-bpKQ", true)
	if !errors.Is(err, media.ErrRateLimited) {
		t.Fatalf("err = %v", err)
	}
}

func TestYouTubeGoneSkipsFallback(t *testing.T) {
	env, _ := ytEnv(t, `{"playabilityStatus":{"status":"ERROR","reason":"Video unavailable"}}`)
	calls := ytStubFallback(t, nil)
	_, err := ytResolve(env, "https://www.youtube.com/watch?v=aqz-KE-bpKQ", true)
	if !errors.Is(err, youtube.ErrGone) || calls.Load() != 0 {
		t.Fatalf("err = %v, %d calls", err, calls.Load())
	}
	// the generic chain must not try yt-dlp again either
	_, err = media.Resolve(context.Background(), "https://www.youtube.com/@BlenderStudio", env, true)
	if !errors.Is(err, media.ErrUnsupported) || calls.Load() != 0 {
		t.Fatalf("channel err = %v, %d calls", err, calls.Load())
	}
}
