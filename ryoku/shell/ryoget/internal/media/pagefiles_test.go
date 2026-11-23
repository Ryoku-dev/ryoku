package media

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"ryoget/internal/plan"
	"ryoget/internal/proto"
)

func serveFile(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func zipHandler(name string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/zip")
		w.Header().Set("Content-Disposition", `attachment; filename="`+name+`"`)
		http.ServeContent(w, r, "", timeZero(), strings.NewReader(strings.Repeat("z", 4096)))
	}
}

func pageSite(t *testing.T) *httptest.Server {
	h := http.NewServeMux()
	h.HandleFunc("/gameboy-rom-pokemon-blue-version/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "the hand-out answers script only", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "text/html")
		w.Write(serveFile(t, "romsgames-page.html"))
	})
	h.HandleFunc("/list", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<html><head><title>Mirror</title></head><body>
<a href="/files/tool-1.2.zip">Tool</a>
<a href="files/disc.iso#top">Disc image</a>
<a href="/files/tool-1.2.zip">Tool again</a>
<a href="/get?id=3" download>Get the manual</a>
<a href="/get?id=4" download="Setup Guide.pdf">guide</a>
<a href="/about.html">About</a>
<a href="mailto:me@example.org">Mail</a>
<a class="download-btn" href="/x">Styled</a>
<a title="click to download" href="/y">Titled</a>
<a href="#files">Jump</a>
</body></html>`)
	})
	h.HandleFunc("/one", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<html><head><title>Game page</title></head><body><a href="/roms/Mario%20%26%20Yoshi.gb">Download</a></body></html>`)
	})
	h.HandleFunc("/roms/", func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.Referer(), "/one") {
			http.Error(w, "bad hand off", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/octet-stream")
		http.ServeContent(w, r, "", timeZero(), strings.NewReader(strings.Repeat("r", 32768)))
	})
	h.HandleFunc("/form-get", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<html><head><title>Get</title></head><body><form id="search"><input name="q"></form>
<form id="dl" action="/fetch" method="get"><input type="hidden" name="id" value="7"><input type="hidden" name="t" value="">
<button type="submit" name="go" value="1">Download now</button></form></body></html>`)
	})
	h.HandleFunc("/fetch", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("id") != "7" || r.URL.Query().Get("go") != "1" || !r.URL.Query().Has("t") {
			http.Error(w, "missing fields", http.StatusBadRequest)
			return
		}
		zipHandler("Seven.zip")(w, r)
	})
	h.HandleFunc("/form-post", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<html><head><title>Post</title></head><body>
<form class="download-form" action="/hand-out" method="POST"><input type="hidden" name="file" value="x.7z"><input type="submit" value="Start"></form></body></html>`)
	})
	h.HandleFunc("/hand-out", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.FormValue("file") != "x.7z" {
			http.Error(w, "no", http.StatusBadRequest)
			return
		}
		http.Redirect(w, r, "/files/x.7z", http.StatusFound)
	})
	h.HandleFunc("/form-post-body", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<html><head><title>Post body</title></head><body>
<form id="download" action="/stream-out" method="post"><input type="hidden" name="k" value="v"></form></body></html>`)
	})
	h.HandleFunc("/stream-out", zipHandler("inline.zip"))
	h.HandleFunc("/form-typed", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<html><head><title>Typed</title></head><body>
<form id="download-form" action="/fetch" method="get"><input type="password" name="pw"><button>Download</button></form></body></html>`)
	})
	h.HandleFunc("/files/", zipHandler("from-path.zip"))
	h.HandleFunc("/plain", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<html><head><title>Plain</title></head><body><a href="/about.html">About</a>
<script>var u='/cdn-cgi/challenge-platform/scripts/jsd/main.js'</script></body></html>`)
	})
	h.HandleFunc("/cf/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=UTF-8")
		w.Write(serveFile(t, "cf-challenge.html"))
	})
	h.HandleFunc("/cf-header", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cf-Mitigated", "challenge")
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusForbidden)
		fmt.Fprint(w, "<html><body>wait</body></html>")
	})
	h.HandleFunc("/ddg/setup.exe", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusForbidden)
		w.Write(serveFile(t, "ddos-guard.html"))
	})
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return srv
}

func wireCode(t *testing.T, err error) *proto.Error {
	t.Helper()
	var pe *proto.Error
	if !errors.As(err, &pe) {
		t.Fatalf("err %v does not carry a wire code", err)
	}
	return pe
}

func TestPageROMSiteNeedsBrowser(t *testing.T) {
	srv := pageSite(t)
	m, err := Resolve(context.Background(), srv.URL+"/gameboy-rom-pokemon-blue-version/", &plan.Env{HTTP: srv.Client()}, false)
	if !errors.Is(err, ErrBrowserCheck) {
		t.Fatalf("err = %v", err)
	}
	pe := wireCode(t, err)
	if pe.Code != proto.CodeBrowserCheck || pe.Message != "This page needs a browser to hand out the file." {
		t.Fatalf("wire = %+v", pe)
	}
	if m == nil || m.Kind != plan.KindPage || !strings.HasPrefix(m.Title, "Pokemon: Blue Version") {
		t.Fatalf("page = %+v", m)
	}
}

func TestPageListsFiles(t *testing.T) {
	srv := pageSite(t)
	m, err := Resolve(context.Background(), srv.URL+"/list", &plan.Env{HTTP: srv.Client()}, false)
	if err != nil {
		t.Fatal(err)
	}
	want := []Item{
		{Title: "tool-1.2.zip", Kind: "file", URL: srv.URL + "/files/tool-1.2.zip", Referer: srv.URL + "/list"},
		{Title: "disc.iso", Kind: "file", URL: srv.URL + "/files/disc.iso", Referer: srv.URL + "/list"},
		{Title: "Get the manual", Kind: "file", URL: srv.URL + "/get?id=3", Referer: srv.URL + "/list"},
		{Title: "Setup Guide.pdf", Kind: "file", URL: srv.URL + "/get?id=4", Referer: srv.URL + "/list"},
	}
	if m.Kind != plan.KindMedia || m.Title != "Mirror" || len(m.Items) != len(want) {
		t.Fatalf("media = %+v", m)
	}
	for i, it := range m.Items {
		if it != want[i] {
			t.Fatalf("item %d = %+v, want %+v", i, it, want[i])
		}
	}
	// each file is a task of its own, resolved against its host
	if _, err := PlanFor(m, Options{}); !errors.Is(err, ErrUnsupported) {
		t.Fatalf("plan err = %v", err)
	}
}

func TestPageOneFileResolvesToIt(t *testing.T) {
	srv := pageSite(t)
	m, err := Resolve(context.Background(), srv.URL+"/one", &plan.Env{HTTP: srv.Client()}, false)
	if err != nil {
		t.Fatal(err)
	}
	if m.Kind != plan.KindFile || m.Name != "Mario & Yoshi.gb" || m.Size != 32768 || !m.Resumable ||
		m.URL != srv.URL+"/roms/Mario%20%26%20Yoshi.gb" || m.Headers["Referer"] != srv.URL+"/one" {
		t.Fatalf("file = %+v", m)
	}
}

func TestPageFollowsPlainForms(t *testing.T) {
	srv := pageSite(t)
	env := &plan.Env{HTTP: srv.Client()}
	m, err := Resolve(context.Background(), srv.URL+"/form-get", env, false)
	if err != nil {
		t.Fatal(err)
	}
	if m.Kind != plan.KindFile || m.Name != "Seven.zip" || !strings.Contains(m.URL, "/fetch?") || m.Headers["Referer"] != srv.URL+"/form-get" {
		t.Fatalf("get form = %+v", m)
	}
	m, err = Resolve(context.Background(), srv.URL+"/form-post", env, false)
	if err != nil {
		t.Fatal(err)
	}
	if m.Kind != plan.KindFile || m.URL != srv.URL+"/files/x.7z" || m.Name != "from-path.zip" {
		t.Fatalf("post form = %+v", m)
	}
	// the file came as the answer to the post, which only posting again would fetch
	if _, err := Resolve(context.Background(), srv.URL+"/form-post-body", env, false); !errors.Is(err, ErrBrowserCheck) {
		t.Fatalf("post answered with the file: %v", err)
	}
	if _, err := Resolve(context.Background(), srv.URL+"/form-typed", env, false); !errors.Is(err, ErrBrowserCheck) {
		t.Fatalf("typed form: %v", err)
	}
	m, err = Resolve(context.Background(), srv.URL+"/plain", env, false)
	if !errors.Is(err, ErrUnsupported) || m.Kind != plan.KindPage {
		t.Fatalf("plain page = %+v, %v", m, err)
	}
}

// a preview only reads: it names a post form's page as a file and leaves the post to the download
func TestPreviewNeverPostsAForm(t *testing.T) {
	var posts atomic.Int32
	h := http.NewServeMux()
	h.HandleFunc("/page", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<html><head><title>Post</title></head><body>
<form class="download-form" action="/hand-out" method="POST"><input type="hidden" name="file" value="x.7z"><input type="submit" value="Start"></form></body></html>`)
	})
	h.HandleFunc("/hand-out", func(w http.ResponseWriter, r *http.Request) {
		posts.Add(1)
		http.Redirect(w, r, "/files/x.7z", http.StatusFound)
	})
	h.HandleFunc("/files/", zipHandler("x.7z"))
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	env := &plan.Env{HTTP: srv.Client()}

	m, err := Resolve(Preview(context.Background()), srv.URL+"/page", env, false)
	if err != nil {
		t.Fatal(err)
	}
	if posts.Load() != 0 || m.Kind != plan.KindFile || m.URL != srv.URL+"/page" {
		t.Fatalf("preview posted %d times and gave %+v", posts.Load(), m)
	}
	m, err = Resolve(context.Background(), srv.URL+"/page", env, false)
	if err != nil {
		t.Fatal(err)
	}
	if posts.Load() != 1 || m.URL != srv.URL+"/files/x.7z" {
		t.Fatalf("download posted %d times and gave %+v", posts.Load(), m)
	}
}

func TestChallengePages(t *testing.T) {
	srv := pageSite(t)
	env := &plan.Env{HTTP: srv.Client()}
	for _, p := range []string{"/cf/file.zip", "/cf-header", "/ddg/setup.exe"} {
		_, err := Resolve(context.Background(), srv.URL+p, env, false)
		if !errors.Is(err, ErrBrowserCheck) || wireCode(t, err).Code != proto.CodeBrowserCheck {
			t.Fatalf("%s: %v", p, err)
		}
	}
	withFakeFallback(t)
	m, err := Resolve(context.Background(), srv.URL+"/cf-header", env, true)
	if err != nil || m.Title != "via yt-dlp" || fallbackCalls.Load() != 1 {
		t.Fatalf("yt-dlp first: %+v, %v", m, err)
	}
	if _, err := Resolve(context.Background(), srv.URL+"/cf/nothing.zip", env, true); !errors.Is(err, ErrBrowserCheck) || fallbackCalls.Load() != 2 {
		t.Fatalf("yt-dlp knows nothing: %v", err)
	}
}

func TestChallengeMarkersNeedASmallPage(t *testing.T) {
	big := append([]byte("<title>Just a moment...</title>"), make([]byte, challengeLimit)...)
	if ChallengePage(http.Header{}, big) {
		t.Fatal("a huge page is not an interstitial")
	}
	if !ChallengePage(http.Header{"Cf-Mitigated": {" Challenge "}}, nil) {
		t.Fatal("header alone marks a challenge")
	}
}

// links on a redirected page resolve against where it ended, the pasted link stays the media's
func TestPageLinksFollowRedirect(t *testing.T) {
	h := http.NewServeMux()
	h.HandleFunc("/pub/linux", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/pub/linux/", http.StatusMovedPermanently)
	})
	h.HandleFunc("/pub/linux/{$}", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<html><head><title>Index of /pub/linux</title></head><body><a href="a.zip">a.zip</a> <a href="b.zip">b.zip</a></body></html>`)
	})
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	m, err := Resolve(context.Background(), srv.URL+"/pub/linux", &plan.Env{HTTP: srv.Client()}, false)
	if err != nil {
		t.Fatal(err)
	}
	if m.URL != srv.URL+"/pub/linux" || len(m.Items) != 2 || m.Items[0].URL != srv.URL+"/pub/linux/a.zip" ||
		m.Items[0].Referer != srv.URL+"/pub/linux/" {
		t.Fatalf("index = %+v", m)
	}
}

func TestPageLinksFollowBaseHref(t *testing.T) {
	h := http.NewServeMux()
	h.HandleFunc("/page", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<html><head><base href="/cdn/files/"><title>Files</title></head><body><a href="a.zip">a</a> <a href="b.zip">b</a></body></html>`)
	})
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	m, err := Resolve(context.Background(), srv.URL+"/page", &plan.Env{HTTP: srv.Client()}, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Items) != 2 || m.Items[0].URL != srv.URL+"/cdn/files/a.zip" || m.Items[1].URL != srv.URL+"/cdn/files/b.zip" ||
		m.Items[0].Referer != srv.URL+"/page" {
		t.Fatalf("items = %+v", m.Items)
	}
}

// an item resolved on its own still sends the page it came from, which hotlink guards ask for
func TestResolveSendsContextReferer(t *testing.T) {
	const ref = "https://site.example/page"
	h := http.NewServeMux()
	h.HandleFunc("/item.zip", func(w http.ResponseWriter, r *http.Request) {
		if r.Referer() != ref {
			http.Error(w, "no hotlinking", http.StatusForbidden)
			return
		}
		zipHandler("item.zip")(w, r)
	})
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	env := &plan.Env{HTTP: srv.Client()}
	if _, err := Resolve(context.Background(), srv.URL+"/item.zip", env, false); !errors.Is(err, ErrPrivate) {
		t.Fatalf("without referer: %v", err)
	}
	m, err := Resolve(WithReferer(context.Background(), ref), srv.URL+"/item.zip", env, false)
	if err != nil {
		t.Fatal(err)
	}
	if m.Kind != plan.KindFile || m.Name != "item.zip" || m.Headers["Referer"] != ref {
		t.Fatalf("file = %+v", m)
	}
}

func TestCrossHostRefererIsOriginOnly(t *testing.T) {
	var pageOrigin string
	files := http.NewServeMux()
	files.HandleFunc("/f/", func(w http.ResponseWriter, r *http.Request) {
		if r.Referer() != pageOrigin {
			http.Error(w, "referer "+r.Referer(), http.StatusForbidden)
			return
		}
		zipHandler("disc.iso")(w, r)
	})
	fileSrv := httptest.NewServer(files)
	t.Cleanup(fileSrv.Close)
	pages := http.NewServeMux()
	pages.HandleFunc("/one", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `<html><head><title>One</title></head><body><a href="%s/f/disc.iso">Disc</a></body></html>`, fileSrv.URL)
	})
	pages.HandleFunc("/two", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `<html><head><title>Two</title></head><body><a href="%[1]s/f/a.iso">A</a> <a href="%[1]s/f/b.iso">B</a></body></html>`, fileSrv.URL)
	})
	pageSrv := httptest.NewServer(pages)
	t.Cleanup(pageSrv.Close)
	pageOrigin = pageSrv.URL + "/"
	env := &plan.Env{HTTP: pageSrv.Client()}

	m, err := Resolve(context.Background(), pageSrv.URL+"/one", env, false)
	if err != nil {
		t.Fatal(err)
	}
	if m.Kind != plan.KindFile || m.Name != "disc.iso" || m.Headers["Referer"] != pageOrigin {
		t.Fatalf("file = %+v", m)
	}
	m, err = Resolve(context.Background(), pageSrv.URL+"/two", env, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Items) != 2 || m.Items[0].Referer != pageOrigin || m.Items[1].Referer != pageOrigin {
		t.Fatalf("items = %+v", m.Items)
	}
	page, _ := url.Parse("https://a.example/p?q=1")
	if r := refererFor(page, &url.URL{Scheme: "http", Host: "b.example", Path: "/f.zip"}); r != "" {
		t.Fatalf("https to http sent %q", r)
	}
	if r := refererFor(page, &url.URL{Scheme: "https", Host: "a.example", Path: "/f.zip"}); r != "https://a.example/p?q=1" {
		t.Fatalf("same origin sent %q", r)
	}
}

// a forum page quoting a challenge script is far bigger than any interstitial
func TestBigPageQuotingAChallengeIsNotOne(t *testing.T) {
	page := `<html><head><title>Forum</title></head><body><pre>window._cf_chl_opt = {}</pre><p>` +
		strings.Repeat("x", 300<<10) + `</p></body></html>`
	if ChallengePage(http.Header{}, []byte(page)) {
		t.Fatal("a 300 KiB page is not an interstitial")
	}
	h := http.NewServeMux()
	h.HandleFunc("/thread", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, page)
	})
	h.HandleFunc("/closed", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusForbidden)
		fmt.Fprint(w, page)
	})
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	env := &plan.Env{HTTP: srv.Client()}
	if _, err := Resolve(context.Background(), srv.URL+"/thread", env, false); !errors.Is(err, ErrUnsupported) || errors.Is(err, ErrBrowserCheck) {
		t.Fatalf("thread: %v", err)
	}
	if _, err := Resolve(context.Background(), srv.URL+"/closed", env, false); !errors.Is(err, ErrPrivate) {
		t.Fatalf("closed thread: %v", err)
	}
}

func TestGetFormAnsweredWithAPageNeedsBrowser(t *testing.T) {
	h := http.NewServeMux()
	h.HandleFunc("/page", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<html><head><title>Get</title></head><body>
<form id="download" action="/next" method="get"><input type="hidden" name="id" value="1"><button>Download</button></form></body></html>`)
	})
	h.HandleFunc("/next", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<html><head><title>Almost there</title></head><body><script>start()</script></body></html>`)
	})
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	m, err := Resolve(context.Background(), srv.URL+"/page", &plan.Env{HTTP: srv.Client()}, false)
	if !errors.Is(err, ErrBrowserCheck) || wireCode(t, err).Code != proto.CodeBrowserCheck {
		t.Fatalf("err = %v", err)
	}
	if m == nil || m.Kind != plan.KindPage || m.Title != "Get" {
		t.Fatalf("page = %+v", m)
	}
}

// a post sent multipart or as plain text is not what followForm sends, a get ignores the encoding
func TestFormWithOtherEncodingNeedsBrowser(t *testing.T) {
	base, _ := url.Parse("https://files.example/game")
	for _, c := range []struct {
		attrs   string
		browser bool
	}{
		{`method="post" enctype="multipart/form-data"`, true},
		{`method="post" enctype="text/plain"`, true},
		{`method="post" enctype="application/x-www-form-urlencoded"`, false},
		{`method="post"`, false},
		{`method="get" enctype="multipart/form-data"`, false},
	} {
		f, ok := downloadForm(base, []byte(`<form id="download" action="/out" `+c.attrs+`><input type="hidden" name="k" value="v"></form>`))
		if !ok || f.browser != c.browser {
			t.Fatalf("%s: form = %+v", c.attrs, f)
		}
	}
}

func TestFormSkipsDisabledControls(t *testing.T) {
	base, _ := url.Parse("https://files.example/game")
	f, ok := downloadForm(base, []byte(`<form id="download" action="/out">
<input name="q" disabled><input type="hidden" name="a" value="1" disabled="disabled"><select name="s" disabled></select>
<input type="hidden" name="b" value="2" title="not disabled"><button disabled>Download</button></form>`))
	if !ok || f.browser || f.values.Encode() != "b=2" {
		t.Fatalf("form = %+v", f)
	}
}
