package youtube

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync/atomic"
	"testing"

	"ryoget/internal/media"
	"ryoget/internal/plan"
)

func TestParseURL(t *testing.T) {
	cases := []struct {
		in, video, list string
	}{
		{"https://www.youtube.com/watch?v=aqz-KE-bpKQ", "aqz-KE-bpKQ", ""},
		{"https://youtube.com/watch?v=aqz-KE-bpKQ&list=PLa1F2ddGya_-UvuAqHAksYnB0qL9yWDO6&index=2", "aqz-KE-bpKQ", "PLa1F2ddGya_-UvuAqHAksYnB0qL9yWDO6"},
		{"https://youtu.be/aqz-KE-bpKQ?t=10", "aqz-KE-bpKQ", ""},
		{"https://www.youtube.com/shorts/2Gq8pXZ3a1Q", "2Gq8pXZ3a1Q", ""},
		{"https://m.youtube.com/live/aqz-KE-bpKQ?feature=share", "aqz-KE-bpKQ", ""},
		{"https://www.youtube-nocookie.com/embed/aqz-KE-bpKQ", "aqz-KE-bpKQ", ""},
		{"https://music.youtube.com/watch?v=aqz-KE-bpKQ&si=x", "aqz-KE-bpKQ", ""},
		{"https://www.youtube.com/playlist?list=PLa1F2ddGya_-UvuAqHAksYnB0qL9yWDO6", "", "PLa1F2ddGya_-UvuAqHAksYnB0qL9yWDO6"},
		{"https://www.youtube.com/@BlenderStudio", "", ""},
		{"https://www.youtube.com/watch?v=short", "", ""},
		{"https://example.com/watch?v=aqz-KE-bpKQ", "", ""},
	}
	for _, c := range cases {
		u, _ := url.Parse(c.in)
		v, l := ParseURL(u)
		if v != c.video || l != c.list {
			t.Errorf("ParseURL(%s) = %q, %q; want %q, %q", c.in, v, l, c.video, c.list)
		}
	}
}

func TestParseVisitor(t *testing.T) {
	data, err := os.ReadFile("testdata/swjs_data.txt")
	if err != nil {
		t.Fatal(err)
	}
	if got := ParseVisitor(data); got != "CgtGSVhUVVJFVklTSVQo2aH31wYyCgoCVVMSBBoAID5IAQ%3D%3D" {
		t.Fatalf("visitor = %q", got)
	}
	if got := ParseVisitor([]byte(`ytcfg.set({"VISITOR_DATA":"CgtBQkNERUZHSElKSyiAgICA"});`)); got != "CgtBQkNERUZHSElKSyiAgICA" {
		t.Fatalf("ytcfg visitor = %q", got)
	}
	if got := ParseVisitor([]byte("nothing here")); got != "" {
		t.Fatalf("visitor from junk = %q", got)
	}
}

type fakeYT struct {
	srv       *httptest.Server
	visitors  atomic.Int32
	players   atomic.Int32
	browses   atomic.Int32
	botFirst  bool
	lastBody  atomic.Value
	lastHdr   atomic.Value
	playerDoc string
}

func newFakeYT(t *testing.T) *fakeYT {
	f := &fakeYT{playerDoc: "testdata/player_ok.json"}
	mux := http.NewServeMux()
	mux.HandleFunc("/sw.js_data", func(w http.ResponseWriter, r *http.Request) {
		f.visitors.Add(1)
		http.ServeFile(w, r, "testdata/swjs_data.txt")
	})
	mux.HandleFunc("/youtubei/v1/player", func(w http.ResponseWriter, r *http.Request) {
		n := f.players.Add(1)
		body, _ := io.ReadAll(r.Body)
		f.lastBody.Store(string(body))
		f.lastHdr.Store(r.Header.Clone())
		if f.botFirst && n == 1 {
			io.WriteString(w, `{"playabilityStatus":{"status":"LOGIN_REQUIRED","reason":"Sign in to confirm you’re not a bot"}}`)
			return
		}
		http.ServeFile(w, r, f.playerDoc)
	})
	mux.HandleFunc("/youtubei/v1/search", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "testdata/search.json")
	})
	mux.HandleFunc("/youtubei/v1/next", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "testdata/next_playlist.json")
	})
	mux.HandleFunc("/youtubei/v1/browse", func(w http.ResponseWriter, r *http.Request) {
		f.browses.Add(1)
		body, _ := io.ReadAll(r.Body)
		switch b := string(body); {
		case strings.Contains(b, `"browseId":"VLRD`):
			http.Error(w, "bad request", http.StatusBadRequest)
		case strings.Contains(b, `"browseId":"VLPLendless"`), strings.Contains(b, `"continuation":"ENDLESS`):
			w.Write(endlessPage(int(f.browses.Load())))
		case strings.Contains(b, `"continuation":"PAGE2TOKEN"`):
			http.ServeFile(w, r, "testdata/browse_continuation.json")
		default:
			http.ServeFile(w, r, "testdata/browse_playlist.json")
		}
	})
	f.srv = httptest.NewServer(mux)
	t.Cleanup(f.srv.Close)
	return f
}

func (f *fakeYT) client(t *testing.T) *Client {
	c := New(f.srv.Client(), t.TempDir())
	c.base = f.srv.URL
	return c
}

func TestPlayerRequestAndMedia(t *testing.T) {
	f := newFakeYT(t)
	c := f.client(t)
	m, err := c.Media(context.Background(), "aqz-KE-bpKQ")
	if err != nil {
		t.Fatal(err)
	}
	var body map[string]any
	if err := json.Unmarshal([]byte(f.lastBody.Load().(string)), &body); err != nil {
		t.Fatal(err)
	}
	client := body["context"].(map[string]any)["client"].(map[string]any)
	if client["clientName"] != "VISIONOS" || client["clientVersion"] != "1.02" || client["deviceModel"] != "RealityDevice17,1" {
		t.Fatalf("player context = %v", client)
	}
	if client["visitorData"] != "CgtGSVhUVVJFVklTSVQo2aH31wYyCgoCVVMSBBoAID5IAQ%3D%3D" || body["videoId"] != "aqz-KE-bpKQ" {
		t.Fatalf("player body = %v", body)
	}
	hdr := f.lastHdr.Load().(http.Header)
	if hdr.Get("X-YouTube-Client-Name") != "101" || hdr.Get("X-Goog-Visitor-Id") == "" {
		t.Fatalf("player headers = %v", hdr)
	}

	if m.Title != "Big Buck Bunny 60fps 4K - Official Blender Foundation Short Film" || m.Author != "Blender" || m.Duration != 635 {
		t.Fatalf("metadata = %q %q %v", m.Title, m.Author, m.Duration)
	}
	if m.Thumb != "https://i.ytimg.com/vi/aqz-KE-bpKQ/hq720.jpg" {
		t.Fatalf("thumb = %q", m.Thumb)
	}
	ids := map[string]media.Format{}
	for _, mf := range m.Formats {
		if _, dup := ids[mf.ID]; dup {
			t.Fatalf("duplicate format %s", mf.ID)
		}
		ids[mf.ID] = mf
		if strings.Contains(mf.Parts[0].URL, "drc=1") {
			t.Fatalf("DRC format %s kept", mf.ID)
		}
	}
	v := ids["299"]
	if v.Kind != "video" || !v.HasVideo || v.HasAudio || v.Height != 1080 || v.FPS != 60 || v.Ext != "mp4" || v.Size != 257619653 {
		t.Fatalf("299 = %+v", v)
	}
	a := ids["140"]
	if a.Kind != "audio" || a.HasVideo || !a.HasAudio || a.Ext != "m4a" || a.Label != "129 kbps" {
		t.Fatalf("140 = %+v", a)
	}
	if o := ids["251"]; o.Ext != "webm" || o.Codec != "opus" {
		t.Fatalf("251 = %+v", o)
	}
	p := a.Parts[0]
	if p.Kind != plan.PartHTTP || p.Headers.Get("User-Agent") != visionOS.ua || p.Size != a.Size {
		t.Fatalf("140 part = %+v", p)
	}
	if !m.Formats[0].HasVideo || m.Formats[0].Height != 2160 {
		t.Fatalf("formats not best first: %+v", m.Formats[0])
	}

	pl, err := media.PlanFor(m, media.Options{Mode: "auto", Quality: "1080"})
	if err != nil {
		t.Fatal(err)
	}
	if pl.Post != plan.PostMerge || pl.Container != "mp4" || len(pl.Parts) != 2 {
		t.Fatalf("plan = %+v", pl)
	}
}

func TestBotGateRefreshesVisitorOnce(t *testing.T) {
	f := newFakeYT(t)
	f.botFirst = true
	c := f.client(t)
	if _, err := c.Media(context.Background(), "aqz-KE-bpKQ"); err != nil {
		t.Fatal(err)
	}
	if f.players.Load() != 2 || f.visitors.Load() != 2 {
		t.Fatalf("players %d visitors %d; want 2 and 2", f.players.Load(), f.visitors.Load())
	}

	f2 := newFakeYT(t)
	f2.playerDoc = "testdata/player_bot.json"
	c2 := f2.client(t)
	_, err := c2.Media(context.Background(), "aqz-KE-bpKQ")
	if !errors.Is(err, ErrBotGate) || !errors.Is(err, media.ErrRateLimited) {
		t.Fatalf("err = %v", err)
	}
	if f2.players.Load() != 2 {
		t.Fatalf("players = %d", f2.players.Load())
	}
	// cooldown: a second gate does not refetch the visitor right away
	c2.Media(context.Background(), "aqz-KE-bpKQ")
	if f2.visitors.Load() != 2 {
		t.Fatalf("visitors = %d", f2.visitors.Load())
	}
}

func TestPlayabilityKinds(t *testing.T) {
	cases := []struct {
		status, reason string
		private        bool
		want           []error
	}{
		{"ERROR", "Video unavailable", false, []error{media.ErrUnavailable, ErrGone}},
		{"LOGIN_REQUIRED", "This video is private", false, []error{media.ErrPrivate}},
		{"UNPLAYABLE", "This video is not available", false, []error{media.ErrUnavailable}},
		{"LOGIN_REQUIRED", "Sign in to confirm your age", false, []error{media.ErrUnavailable}},
	}
	for _, c := range cases {
		p := &Player{}
		p.PlayabilityStatus.Status, p.PlayabilityStatus.Reason = c.status, c.reason
		p.VideoDetails.IsPrivate = c.private
		err := p.check()
		for _, w := range c.want {
			if !errors.Is(err, w) {
				t.Errorf("%s/%s: %v is not %v", c.status, c.reason, err, w)
			}
		}
		if err.Error() != c.reason {
			t.Errorf("message = %q", err.Error())
		}
	}
}

func TestNoPlainURLs(t *testing.T) {
	p := &Player{}
	p.PlayabilityStatus.Status = "OK"
	p.VideoDetails.VideoID = "aqz-KE-bpKQ"
	p.StreamingData.AdaptiveFormats = []streamFormat{{Itag: 140, SignatureCipher: "s=x&url=y", MimeType: `audio/mp4; codecs="mp4a.40.2"`}}
	if _, err := p.Media(); !errors.Is(err, ErrNoFormats) {
		t.Fatalf("err = %v", err)
	}
	p.VideoDetails.IsLive = true
	if _, err := p.Media(); !errors.Is(err, media.ErrUnsupported) {
		t.Fatalf("live err = %v", err)
	}
}

func TestDubbedKeepsOriginalTrack(t *testing.T) {
	p := &Player{}
	p.PlayabilityStatus.Status = "OK"
	p.VideoDetails.VideoID = "aqz-KE-bpKQ"
	track := func(id, name string, def bool) *struct {
		DisplayName    string `json:"displayName"`
		ID             string `json:"id"`
		AudioIsDefault bool   `json:"audioIsDefault"`
	} {
		return &struct {
			DisplayName    string `json:"displayName"`
			ID             string `json:"id"`
			AudioIsDefault bool   `json:"audioIsDefault"`
		}{name, id, def}
	}
	p.StreamingData.AdaptiveFormats = []streamFormat{
		{Itag: 140, URL: "https://x/de", MimeType: `audio/mp4; codecs="mp4a.40.2"`, AudioTrack: track("de.3", "German", true)},
		{Itag: 141, URL: "https://x/en", MimeType: `audio/mp4; codecs="mp4a.40.2"`, AudioTrack: track("en.4", "English original", false)},
	}
	m, err := p.Media()
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Formats) != 1 || m.Formats[0].Parts[0].URL != "https://x/en" {
		t.Fatalf("formats = %+v", m.Formats)
	}
}

func TestSearch(t *testing.T) {
	f := newFakeYT(t)
	c := f.client(t)
	vs, err := c.Search(context.Background(), "blender open movie", "en", 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(vs) != 2 {
		t.Fatalf("got %d videos", len(vs))
	}
	v := vs[0]
	if v.ID != "_cMxraX_5RE" || v.Title != "Sprite Fright - Blender Open Movie" || v.Channel != "Blender Studio" ||
		v.Duration != 630 || v.DurationText != "10:30" || v.Views != 9562158 || v.Published != "5 years ago" ||
		v.URL != "https://www.youtube.com/watch?v=_cMxraX_5RE" || !strings.HasPrefix(v.Thumb, "https://i.ytimg.com/") || v.Snippet == "" {
		t.Fatalf("video = %+v", v)
	}
	if vs[1].ID != "WhWc3b3KhnY" {
		t.Fatalf("second = %+v", vs[1])
	}
	all, _ := c.Search(context.Background(), "blender", "en", 0)
	if len(all) != 3 {
		t.Fatalf("unlimited got %d", len(all))
	}
}

func TestPlaylistBrowse(t *testing.T) {
	f := newFakeYT(t)
	c := f.client(t)
	pl, err := c.Playlist(context.Background(), "UUSMOQeBJ2RAnuFungnQOxLg")
	if err != nil {
		t.Fatal(err)
	}
	ids := make([]string, len(pl.Entries))
	for i, e := range pl.Entries {
		ids[i] = e.ID
		if e.Title == "" || !strings.HasPrefix(e.Thumb, "https://i.ytimg.com/") {
			t.Fatalf("entry %d = %+v", i, e)
		}
	}
	if strings.Join(ids, " ") != "ljj8nhLYt6A xJmovF_l1fY pvUy69I_Xuk 6rjcePddRlg SqJ_rtaPqg4" {
		t.Fatalf("entries = %v", ids)
	}
	if pl.First != "ljj8nhLYt6A" || pl.Count != 1596 || pl.Title != "Uploads from Blender" || f.browses.Load() != 2 {
		t.Fatalf("playlist = %+v after %d browses", pl, f.browses.Load())
	}
	if pl.Entries[0].Duration != 17 {
		t.Fatalf("duration = %v", pl.Entries[0].Duration)
	}
}

func TestPlaylistNextFallback(t *testing.T) {
	f := newFakeYT(t)
	pl, err := f.client(t).Playlist(context.Background(), "RDaqz-KE-bpKQ")
	if err != nil {
		t.Fatal(err)
	}
	if pl.First != "MF1qEhBSfq4" || pl.Count != 43 || pl.Title != "Blender Fundamentals 2.8" || len(pl.Entries) != 2 {
		t.Fatalf("playlist = %+v", pl)
	}
}

// endlessPage is a browse answer of 50 fresh videos and another continuation.
func endlessPage(n int) []byte {
	var items []string
	for i := range 50 {
		id := fmt.Sprintf("e%04d%06d", n, i)
		items = append(items, fmt.Sprintf(`{"lockupViewModel":{"contentId":%q,"contentType":"LOCKUP_CONTENT_TYPE_VIDEO","metadata":{"lockupMetadataViewModel":{"title":{"content":"v %s"}}}}}`, id, id))
	}
	items = append(items, fmt.Sprintf(`{"continuationItemViewModel":{"continuationCommand":{"innertubeCommand":{"continuationCommand":{"token":"ENDLESS%d"}}}}}`, n))
	list := strings.Join(items, ",")
	if n == 1 {
		return []byte(`{"contents":{"sectionListRenderer":{"contents":[` + list + `]}}}`)
	}
	return []byte(`{"onResponseReceivedActions":[{"appendContinuationItemsAction":{"continuationItems":[` + list + `]}}]}`)
}

func TestPlaylistCap(t *testing.T) {
	f := newFakeYT(t)
	pl, err := f.client(t).Playlist(context.Background(), "PLendless")
	if err != nil {
		t.Fatal(err)
	}
	if len(pl.Entries) != PlaylistCap || f.browses.Load() != 4 {
		t.Fatalf("%d entries after %d browses", len(pl.Entries), f.browses.Load())
	}
}

func TestVisitorPersists(t *testing.T) {
	f := newFakeYT(t)
	dir := t.TempDir()
	c := New(f.srv.Client(), dir)
	c.base = f.srv.URL
	c.Visitor(context.Background())
	c2 := New(f.srv.Client(), dir)
	c2.base = f.srv.URL
	if v := c2.Visitor(context.Background()); v == "" || f.visitors.Load() != 1 {
		t.Fatalf("visitor %q after %d fetches", v, f.visitors.Load())
	}
}

func TestClockAndViews(t *testing.T) {
	if parseClock("1:02:03") != 3723 || parseClock("4:12") != 252 || parseClock("LIVE") != 0 {
		t.Fatal("parseClock")
	}
	if parseViews("1,234 views") != 1234 || parseViews("No views") != 0 || parseViews("") != -1 {
		t.Fatal("parseViews")
	}
}
