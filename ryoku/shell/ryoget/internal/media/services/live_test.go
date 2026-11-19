//go:build live

package services

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"ryoget/internal/media"
	"ryoget/internal/plan"
)

type liveCase struct {
	name   string
	source string
	url    string
	items  int // at least this many items
	kind   plan.Kind
}

// Real public posts; each one is fetched from the live site.
var liveCases = []liveCase{
	{"x-video", "x", "https://x.com/historyinmemes/status/1790637656616943991", 0, plan.KindMedia},
	{"x-multi", "x", "https://twitter.com/CTVJLaidlaw/status/1600649710662213632", 2, plan.KindMedia},
	{"x-mixed", "x", "https://x.com/oshtru/status/1577855540407197696", 2, plan.KindMedia},
	{"instagram-video", "instagram", "https://www.instagram.com/p/aye83DjauH/", 0, plan.KindMedia},
	{"instagram-reel", "instagram", "https://www.instagram.com/reel/Cf0q4O4tt9f/", 0, plan.KindMedia},
	{"instagram-carousel", "instagram", "https://www.instagram.com/p/DbjY8EVEskh/", 2, plan.KindMedia},
	{"instagram-photo", "instagram", "https://www.instagram.com/p/CQfJwGkgZio/", 0, plan.KindFile},
	{"tiktok-video", "tiktok", "https://www.tiktok.com/@patroxofficial/video/6742501081818877190", 0, plan.KindMedia},
	{"bluesky-video", "bluesky", "https://bsky.app/profile/bsky.app/post/3l3vgf77uco2g", 0, plan.KindMedia},
	{"bluesky-images", "bluesky", "https://bsky.app/profile/bsky.app/post/3myx7vphq3s2j", 4, plan.KindMedia},
	{"streamable", "streamable", "https://streamable.com/dnd1", 0, plan.KindMedia},
	{"twitch-clip", "twitch", "https://clips.twitch.tv/FaintLightGullWholeWheat", 0, plan.KindMedia},
	{"soundcloud-track", "soundcloud", "https://soundcloud.com/the80m/the-following", 0, plan.KindMedia},
	{"soundcloud-set", "soundcloud", "https://soundcloud.com/lofistudymusic/sets/study-music-lofi-study-music", 20, plan.KindMedia},
	{"vimeo", "vimeo", "https://vimeo.com/76979871", 0, plan.KindMedia},
	{"dailymotion", "dailymotion", "https://www.dailymotion.com/video/x5kesuj", 0, plan.KindMedia},
	{"pinterest-video", "pinterest", "https://www.pinterest.com/pin/664281013778109217/", 0, plan.KindMedia},
	{"pinterest-story", "pinterest", "https://www.pinterest.com/pin/1084663891475263837/", 0, plan.KindMedia},
	{"tumblr-video", "tumblr", "https://maskofthedragon.tumblr.com/post/626907179849564160/mona-talking-in-english", 0, plan.KindMedia},
	{"tumblr-images", "tumblr", "https://staff.tumblr.com/post/828009069026721792/premium-just-got-better", 4, plan.KindMedia},
	{"reddit-video", "reddit", "https://v.redd.it/zv89llsvexdz", 0, plan.KindMedia},
	{"bilibili", "bilibili", "https://www.bilibili.com/video/BV13x41117TL", 0, plan.KindMedia},
	{"bilibili-parts", "bilibili", "https://www.bilibili.com/video/BV1bK411W797?p=2", 20, plan.KindMedia},
	{"loom-mp4", "loom", "https://www.loom.com/share/c43a642f815f4378b6f80a889bb73d8d", 0, plan.KindMedia},
	{"loom-hls", "loom", "https://www.loom.com/share/43d05f362f734614a2e81b4694a3a523", 0, plan.KindMedia},
	{"okru", "okru", "https://ok.ru/video/20079905452", 0, plan.KindMedia},
	{"rutube", "rutube", "https://rutube.ru/video/3eac3b4561676c17df9132a9a1e62e3e/", 0, plan.KindMedia},
	{"vk", "vk", "https://vk.com/video-77521_162222515", 0, plan.KindMedia},
	{"facebook", "facebook", "https://www.facebook.com/cnn/videos/10155529876156509/", 0, plan.KindMedia},
	{"twitch-vod", "twitch", "https://www.twitch.tv/videos/635475444", 0, plan.KindMedia},
}

// recorder saves every live response under $RYOGET_RECORD for building
// fixtures.
type recorder struct {
	dir  string
	next http.RoundTripper
	n    atomic.Int32
}

func (r *recorder) RoundTrip(req *http.Request) (*http.Response, error) {
	var reqBody []byte
	if req.Body != nil {
		reqBody, _ = io.ReadAll(req.Body)
		req.Body = io.NopCloser(bytes.NewReader(reqBody))
	}
	resp, err := r.next.RoundTrip(req)
	if err != nil {
		return nil, err
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	resp.Body = io.NopCloser(bytes.NewReader(body))
	n := r.n.Add(1)
	name := fmt.Sprintf("%02d-%s", n, strings.NewReplacer("/", "_", ":", "_").Replace(req.URL.Host+req.URL.Path))
	if len(name) > 120 {
		name = name[:120]
	}
	meta := fmt.Sprintf("%s %s\nstatus %d\nreqbody %s\n", req.Method, req.URL.String(), resp.StatusCode, reqBody)
	for k, v := range resp.Header {
		meta += k + ": " + strings.Join(v, " | ") + "\n"
	}
	os.WriteFile(filepath.Join(r.dir, name+".meta"), []byte(meta), 0o644)
	os.WriteFile(filepath.Join(r.dir, name+".body"), body, 0o644)
	return resp, nil
}

func liveEnv(t *testing.T, name string) *plan.Env {
	var rt http.RoundTripper = http.DefaultTransport
	if dir := os.Getenv("RYOGET_RECORD"); dir != "" {
		d := filepath.Join(dir, name)
		os.MkdirAll(d, 0o755)
		rt = &recorder{dir: d, next: rt}
	}
	return &plan.Env{HTTP: &http.Client{Transport: rt, Timeout: 45 * time.Second}}
}

func TestLiveServices(t *testing.T) {
	only := os.Getenv("RYOGET_LIVE")
	for _, c := range liveCases {
		if only != "" && !strings.Contains(","+only+",", ","+c.source+",") && !strings.Contains(","+only+",", ","+c.name+",") {
			continue
		}
		t.Run(c.name, func(t *testing.T) {
			u, err := url.Parse(c.url)
			if err != nil {
				t.Fatal(err)
			}
			s := media.Lookup(u)
			if s == nil || s.ID() != c.source {
				t.Fatalf("lookup = %v, want %s", s, c.source)
			}
			env := liveEnv(t, c.name)
			m, err := s.Resolve(t.Context(), u, env)
			if err != nil {
				t.Fatalf("resolve: %v", err)
			}
			checkMedia(t, m, c.source)
			if m.Kind != c.kind {
				t.Fatalf("kind = %s, want %s", m.Kind, c.kind)
			}
			if len(m.Items) < c.items {
				t.Fatalf("items = %d, want at least %d", len(m.Items), c.items)
			}
			var seen []string
			for _, f := range m.Formats {
				seen = append(seen, fmt.Sprintf("%s:%s:%dp:%dk:v%v/a%v:%s", f.ID, f.Kind, f.Height, f.Bitrate/1000, b2i(f.HasVideo), b2i(f.HasAudio), f.Parts[0].Kind))
			}
			t.Logf("%q by %q, %.0fs, thumb %v, items %d, default %s", m.Title, m.Author, m.Duration, m.Thumb != "", len(m.Items), m.Default)
			t.Logf("formats: %s", strings.Join(seen, " "))
			if m.Kind == plan.KindFile {
				probeLive(t, env, plan.Part{Kind: plan.PartHTTP, URL: m.URL}, m.Headers)
				return
			}
			if f := findFormat(m, m.Default); f != nil {
				for _, p := range f.Parts {
					probeLive(t, env, p, m.Headers)
				}
			}
		})
	}
}

func b2i(b bool) int {
	if b {
		return 1
	}
	return 0
}

// probeLive proves the CDN serves a part with the headers the service gave.
func probeLive(t *testing.T, env *plan.Env, p plan.Part, base map[string]string) {
	t.Helper()
	req, _ := http.NewRequestWithContext(t.Context(), http.MethodGet, p.URL, nil)
	req.Header.Set("User-Agent", browserUA)
	for k, v := range base {
		req.Header.Set(k, v)
	}
	for k, v := range p.Headers {
		req.Header[k] = v
	}
	if p.Kind == plan.PartHTTP {
		req.Header.Set("Range", "bytes=0-1023")
	}
	resp, err := env.HTTP.Do(req)
	if err != nil {
		t.Fatalf("probe %s: %v", p.URL, err)
	}
	defer resp.Body.Close()
	head, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
	if resp.StatusCode != 200 && resp.StatusCode != 206 {
		t.Fatalf("probe %s part %s: HTTP %d", p.Kind, p.URL, resp.StatusCode)
	}
	if p.Kind == plan.PartHLS && !bytes.HasPrefix(bytes.TrimSpace(head), []byte("#EXTM3U")) {
		t.Fatalf("probe %s: not a playlist", p.URL)
	}
	t.Logf("part %s %s ok (HTTP %d)", p.Kind, p.Role, resp.StatusCode)
}
