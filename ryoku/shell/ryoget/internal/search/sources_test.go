package search

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"ryoget/internal/media"
	"ryoget/internal/youtube"
)

func fakeYouTube(_ context.Context, _ *http.Client, query, _ string, _ int) ([]youtube.Video, error) {
	if query == "rate" {
		return nil, media.ErrRateLimited
	}
	return []youtube.Video{
		{ID: "abc123def45", Title: "Fixing NVIDIA flicker on Wayland", URL: "https://www.youtube.com/watch?v=abc123def45",
			Channel: "Linux Cast", Duration: 252, DurationText: "4:12", Views: 120000, Published: "1 year ago",
			Thumb: "https://i.ytimg.com/vi/abc123def45/hqdefault.jpg"},
		{ID: "zzz", Title: "Live", URL: "https://www.youtube.com/watch?v=zzz", Views: -1, Live: true},
	}, nil
}

func run(t *testing.T, e *Service, id, text, lang string) ([]Item, bool, error) {
	t.Helper()
	s := e.byID[id]
	if s == nil {
		t.Fatalf("source %s not registered", id)
	}
	return s.search(context.Background(), e, newQuery(text, lang))
}

func TestDuckDuckGoRegion(t *testing.T) {
	for lang, want := range map[string]string{"en": "wt-wt", "de": "de-de", "pt-br": "br-pt", "pt": "pt-pt", "xx": "wt-wt"} {
		if got := ddgRegion(lang); got != want {
			t.Errorf("ddgRegion(%q) = %q, want %q", lang, got, want)
		}
	}
}

func TestWikipedia(t *testing.T) {
	f := newFixtures(t).standard()
	f.set("de.wikipedia.org/w/rest.php/v1/search/page", route{file: "wikipedia.json"})
	e := testService(t, f, "os-release-arch")
	items, more, err := run(t, e, "wikipedia", "wayland", "de_DE")
	if err != nil {
		t.Fatal(err)
	}
	if f.count("de.wikipedia.org/w/rest.php/v1/search/page") != 1 {
		t.Fatal("the UI language did not pick the German Wikipedia")
	}
	if len(items) != 10 || !more {
		t.Fatalf("len=%d more=%v", len(items), more)
	}
	it := items[0]
	if it.Key != "de:Wayland_(protocol)" || it.Snippet != "Display system intended to replace X11" || it.Kind != KindArticle {
		t.Errorf("item = %+v", it)
	}
	if !strings.Contains(it.thumbURL, "/120px-") || !strings.HasPrefix(it.thumbURL, "https://") {
		t.Errorf("thumb = %q", it.thumbURL)
	}
	if strings.Contains(it.Meta["excerpt"].(string), "<span") {
		t.Errorf("excerpt kept markup: %q", it.Meta["excerpt"])
	}
}

func TestGitHub(t *testing.T) {
	e := testService(t, newFixtures(t).standard(), "os-release-arch")
	items, more, err := run(t, e, "github", "wayland compositor", "en")
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 6 || !more || items[0].Title != "swaywm/sway" || items[0].Key != "swaywm/sway" {
		t.Fatalf("items=%d more=%v first=%+v", len(items), more, items[0])
	}
	if items[0].Meta["stars"].(int64) < 1000 || items[0].Meta["license"] != "MIT" || items[0].Date == "" {
		t.Errorf("meta = %+v", items[0].Meta)
	}
	if !strings.HasSuffix(items[0].thumbURL, "&s=64") {
		t.Errorf("avatar = %q", items[0].thumbURL)
	}
}

func TestGitHubRateLimitPausesUntilReset(t *testing.T) {
	f := newFixtures(t)
	f.set("api.github.com/search/repositories", route{status: 403, body: `{"message":"API rate limit exceeded"}`,
		header: map[string]string{"X-RateLimit-Remaining": "0", "X-RateLimit-Reset": "4102444800", "X-RateLimit-Resource": "search"}})
	e := testService(t, f, "os-release-arch")
	_, _, err := run(t, e, "github", "x", "en")
	p, ok := isPaused(err)
	if !ok || p.until.UTC().Year() != 2100 {
		t.Fatalf("err = %v, want paused until the reset in 2100", err)
	}
	if until := e.gov.paused("github"); until.UTC().Year() != 2100 {
		t.Fatalf("engine not paused: %v", until)
	}
	if until := e.gov.paused("github-api"); !until.IsZero() {
		t.Error("repository details keep their own budget")
	}
}

func TestStackOverflow(t *testing.T) {
	f := newFixtures(t).standard()
	e := testService(t, f, "os-release-arch")
	items, _, err := run(t, e, "stackoverflow", "nvidia wayland", "en")
	if err != nil {
		t.Fatal(err)
	}
	if len(items) == 0 || items[0].Key != "70869849" || items[0].URL != "https://stackoverflow.com/questions/70869849" {
		t.Fatalf("first = %+v", items[0])
	}
	if strings.Contains(items[0].Title, "&#") || strings.Contains(items[0].Snippet, "<span") {
		t.Errorf("entities or markup leaked: %q / %q", items[0].Title, items[0].Snippet)
	}
	if items[0].Meta["accepted"] != true || items[0].Meta["score"].(int64) != 3 {
		t.Errorf("meta = %+v", items[0].Meta)
	}
	if f.count("api.stackexchange.com/2.3/similar") != 0 {
		t.Error("similar called although excerpts answered")
	}
}

func TestStackOverflowFallsBackToSimilar(t *testing.T) {
	f := newFixtures(t)
	f.set("api.stackexchange.com/2.3/search/excerpts", route{body: `{"items":[],"has_more":false,"quota_max":300,"quota_remaining":200}`})
	f.set("api.stackexchange.com/2.3/similar", route{file: "stackoverflow_similar.json"})
	e := testService(t, f, "os-release-arch")
	items, _, err := run(t, e, "stackoverflow", "nvidia wayland flicker", "en")
	if err != nil || len(items) == 0 {
		t.Fatalf("err=%v items=%d", err, len(items))
	}
	if got := f.last("api.stackexchange.com/2.3/similar").URL.Query().Get("title"); got != "nvidia wayland flicker" {
		t.Errorf("similar title = %q", got)
	}
}

func TestStackOverflowThrottle(t *testing.T) {
	f := newFixtures(t)
	f.set("api.stackexchange.com/2.3/search/excerpts", route{status: 400,
		body: `{"error_id":502,"error_message":"too many requests from this IP, more requests available in 79 seconds","error_name":"throttle_violation"}`})
	e := testService(t, f, "os-release-arch")
	if _, _, err := run(t, e, "stackoverflow", "x", "en"); err == nil {
		t.Fatal("throttle not reported")
	} else if _, ok := isPaused(err); !ok {
		t.Fatalf("err = %v, want paused", err)
	}
	if d := e.gov.paused("stackexchange").Sub(e.now()); d < 70e9 || d > 80e9 {
		t.Errorf("paused for %v, want about 79 s", d)
	}
}

func TestCodeSources(t *testing.T) {
	e := testService(t, newFixtures(t).standard(), "os-release-arch")
	mdn, more, err := run(t, e, "mdn", "array map", "en")
	if err != nil || len(mdn) != 6 || !more {
		t.Fatalf("mdn: %v %d %v", err, len(mdn), more)
	}
	if mdn[0].URL != "https://developer.mozilla.org/en-US/docs/Web/JavaScript/Reference/Global_Objects/Array/map" || mdn[0].Kind != KindDoc {
		t.Errorf("mdn first = %+v", mdn[0])
	}
	if got := e.byID["mdn"]; got == nil || mdnLocale("pt-br") != "pt-BR" || mdnLocale("fi") != "en-US" {
		t.Error("mdn locale mapping")
	}

	npm, _, err := run(t, e, "npm", "wayland", "en")
	if err != nil || len(npm) != 6 || npm[0].Title != "is-wayland" || npm[0].URL == "" {
		t.Fatalf("npm: %v %+v", err, npm)
	}
	if _, ok := npm[0].Meta["weeklyDownloads"]; !ok {
		t.Errorf("npm meta = %+v", npm[0].Meta)
	}

	crates, _, err := run(t, e, "crates", "wayland", "en")
	if err != nil || len(crates) != 6 {
		t.Fatalf("crates: %v %d", err, len(crates))
	}
	if crates[0].Title != "wayland" || crates[0].URL != "https://crates.io/crates/wayland" {
		t.Errorf("crates first = %+v", crates[0])
	}
}

func TestArchSourcesOnlyOnArch(t *testing.T) {
	f := newFixtures(t).standard()
	if e := testService(t, f, "os-release-fedora"); e.byID["aur"] != nil || e.byID["archpkg"] != nil || e.byID["archwiki"] == nil {
		t.Fatal("Arch package sources must stay off non-Arch editions; the wiki stays")
	}
	if e := testService(t, f, "os-release-arch"); e.byID["aur"] == nil || e.byID["archpkg"] == nil {
		t.Fatal("ID_LIKE=arch must enable the Arch package sources")
	}
}

func TestArchWikiDropsOtherLanguages(t *testing.T) {
	e := testService(t, newFixtures(t).standard(), "os-release-arch")
	items, more, err := run(t, e, "archwiki", "nvidia wayland", "en")
	if err != nil || !more {
		t.Fatal(err, more)
	}
	for _, it := range items {
		if m := parenSuffix.FindStringSubmatch(it.Title); m != nil && archLangs[m[1]] != "" {
			t.Errorf("translated page kept: %q", it.Title)
		}
		if strings.Contains(it.Snippet, "[[") || strings.Contains(it.Snippet, "<span") {
			t.Errorf("snippet markup: %q", it.Snippet)
		}
	}
	pt, _, _ := run(t, e, "archwiki", "nvidia wayland other", "pt-BR")
	if len(pt) == 0 || pt[0].Title != "Wayland (Português)" {
		t.Errorf("Portuguese reader should see the Portuguese page first, got %+v", pt)
	}
}

func TestArchPackagesFilterAndRank(t *testing.T) {
	f := newFixtures(t).standard()
	e := testService(t, f, "os-release-arch")
	items, _, err := run(t, e, "archpkg", "wayland clipboard", "en")
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].Title != "cliphist" || items[0].URL != "https://archlinux.org/packages/extra/x86_64/cliphist/" {
		t.Fatalf("items = %+v", items)
	}
	if q := f.last("archlinux.org/packages/search/json/").URL.Query().Get("q"); q != "clipboard" {
		t.Errorf("searched %q, want the longest word", q)
	}
}

func TestAUR(t *testing.T) {
	f := newFixtures(t)
	f.set("aur.archlinux.org/rpc/v5/search/hyprland", route{file: "aur.json"})
	f.set("aur.archlinux.org/rpc/v5/search/ab", route{body: `{"error":"Too many package results.","resultcount":0,"results":[],"type":"error","version":5}`})
	e := testService(t, f, "os-release-arch")
	items, _, err := run(t, e, "aur", "hyprland", "en")
	if err != nil || len(items) == 0 {
		t.Fatal(err, items)
	}
	if items[0].Title != "ashell" {
		t.Errorf("most popular first, got %q", items[0].Title)
	}
	if items, _, err := run(t, e, "aur", "ab", "en"); err != nil || len(items) != 0 {
		t.Errorf("too many results is an empty answer, got %v %v", err, items)
	}
}

func TestFlathubAndHackerNews(t *testing.T) {
	f := newFixtures(t).standard()
	e := testService(t, f, "os-release-arch")
	fh, more, err := run(t, e, "flathub", "firefox", "de")
	if err != nil || len(fh) != 5 || !more {
		t.Fatalf("flathub: %v %d %v", err, len(fh), more)
	}
	if fh[0].URL != "https://flathub.org/apps/org.mozilla.firefox" || !strings.HasPrefix(fh[0].thumbURL, "https://dl.flathub.org/") || !strings.HasPrefix(fh[0].Date, "2026-") {
		t.Errorf("flathub first = %+v", fh[0])
	}
	body := f.last("flathub.org/api/v2/search")
	if body == nil || body.Method != "POST" {
		t.Error("flathub search must POST")
	}

	hn, _, err := run(t, e, "hackernews", "wayland", "en")
	if err != nil || len(hn) != 6 {
		t.Fatalf("hn: %v %d", err, len(hn))
	}
	if hn[0].URL != "https://github.com/fyzugx/DEsk" || hn[0].Meta["discussion"] != "https://news.ycombinator.com/item?id=50103338" || hn[0].Meta["points"].(int64) != 704 {
		t.Errorf("hn first = %+v", hn[0])
	}
}

func TestYouTube(t *testing.T) {
	e := testService(t, newFixtures(t), "os-release-arch")
	items, _, err := run(t, e, "youtube", "nvidia flicker", "en")
	if err != nil || len(items) != 2 {
		t.Fatal(err, items)
	}
	if items[0].Kind != KindVideo || items[0].Meta["durationText"] != "4:12" || items[0].thumbURL == "" {
		t.Errorf("first = %+v", items[0])
	}
	if _, _, err := run(t, e, "youtube", "rate", "en"); err == nil {
		t.Error("rate limit not reported")
	} else if _, ok := isPaused(err); !ok {
		t.Errorf("rate limit should pause YouTube: %v", err)
	}
}
