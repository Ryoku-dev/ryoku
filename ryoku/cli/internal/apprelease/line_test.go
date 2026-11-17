package apprelease

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
)

// listRelease is one entry of a fake GitHub release list.
type listRelease struct {
	tag        string
	draft      bool
	prerelease bool
	sourceOnly bool // a release with the source tarball and no package
}

// listFixture serves a release list (and every release's assets) the way the
// GitHub API does, and counts hits on /releases/latest, which the prerelease
// line must never read.
type listFixture struct {
	releases   []listRelease
	latestHits int32
	listHits   int32
	pages      int32
}

func (f *listFixture) server() *httptest.Server {
	payload := []byte("fake ryotunes 2 package")
	sum := sha256.Sum256(payload)
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/releases/latest"):
			atomic.AddInt32(&f.latestHits, 1)
			http.NotFound(w, r)
		case strings.HasSuffix(r.URL.Path, "/releases"):
			atomic.AddInt32(&f.listHits, 1)
			if r.URL.Query().Get("per_page") != "100" {
				t := fmt.Sprintf("per_page %q", r.URL.Query().Get("per_page"))
				http.Error(w, t, http.StatusBadRequest)
				return
			}
			page, _ := strconv.Atoi(r.URL.Query().Get("page"))
			if page < 1 {
				http.Error(w, "no page", http.StatusBadRequest)
				return
			}
			atomic.AddInt32(&f.pages, 1)
			from, to := (page-1)*100, page*100
			from, to = min(from, len(f.releases)), min(to, len(f.releases))
			list := make([]map[string]any, 0, to-from)
			for _, rel := range f.releases[from:to] {
				version := strings.TrimPrefix(rel.tag, "v")
				pkg := "ryotunes-" + version + "-1-x86_64.pkg.tar.zst"
				assets := []map[string]string{{"name": pkg}, {"name": pkg + ".sha256"}}
				if rel.sourceOnly {
					tarball := "ryotunes-" + version + ".tar.gz"
					assets = []map[string]string{{"name": tarball}, {"name": tarball + ".sha256"}}
				}
				list = append(list, map[string]any{
					"tag_name": rel.tag, "draft": rel.draft, "prerelease": rel.prerelease, "assets": assets,
				})
			}
			_ = json.NewEncoder(w).Encode(list)
		case strings.HasSuffix(r.URL.Path, ".sha256"):
			name := strings.TrimSuffix(filepath.Base(r.URL.Path), ".sha256")
			fmt.Fprintf(w, "%s  %s\n", hex.EncodeToString(sum[:]), name)
		case strings.HasSuffix(r.URL.Path, ".pkg.tar.zst"):
			_, _ = w.Write(payload)
		default:
			http.NotFound(w, r)
		}
	}))
}

func lineClient(t *testing.T, srv *httptest.Server, app App, installed string, rec *recorder) *Client {
	t.Helper()
	f := &fixture{repo: app.Repo, pkgAsset: "ryotunes-0.0.0-1-x86_64.pkg.tar.zst"}
	c := newClient(t, srv, f, installed, pkgMeta{}, nil, rec)
	c.app = app
	c.inspect = func(_ context.Context, path string) (pkgMeta, error) {
		rest := strings.TrimSuffix(strings.TrimPrefix(filepath.Base(path), "ryotunes-"), "-x86_64.pkg.tar.zst")
		return pkgMeta{Name: "ryotunes", Version: "1:" + rest, Arch: wantArch}, nil
	}
	return c
}

func hyperionFixture() *listFixture {
	return &listFixture{releases: []listRelease{
		{tag: "v1.2.6"},
		{tag: "v3.0.0", prerelease: true},
		{tag: "v2.1.0", draft: true},
		{tag: "v2.0.10", prerelease: true},
		{tag: "v2.0.9", prerelease: true},
		{tag: "v2.0.0"},
		{tag: "v2.5.1", sourceOnly: true},
		{tag: "v2.5.0", sourceOnly: true},
	}}
}

// The hyperion line reads the release list, never /latest, and takes the
// highest v2.x.y that carries a package: prereleases count, drafts, other
// majors and the source-only 2.5 tags from before the epoch do not, and
// versions order numerically (2.0.10 after 2.0.9).
func TestHyperionLineTakesNewestV2IncludingPrereleases(t *testing.T) {
	f := hyperionFixture()
	srv := f.server()
	defer srv.Close()
	var rec recorder
	c := lineClient(t, srv, Ryotunes.OnLine(HyperionLine), "1:1.2.6-1", &rec)

	st, err := c.Check(context.Background())
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	want := Status{Installed: "1:1.2.6-1", Latest: "1:2.0.10-1", Available: true}
	if st != want {
		t.Fatalf("Check = %+v, want %+v", st, want)
	}
	if f.latestHits != 0 || f.listHits != 1 {
		t.Fatalf("latest hits %d, list hits %d; want 0 and 1", f.latestHits, f.listHits)
	}
}

// Ensure on the hyperion line installs the prerelease, hears the plan first, and
// keeps its cache apart from the stable line's.
func TestHyperionLineEnsureInstallsPrerelease(t *testing.T) {
	f := hyperionFixture()
	srv := f.server()
	defer srv.Close()
	var rec recorder
	c := lineClient(t, srv, Ryotunes.OnLine(HyperionLine), "", &rec)
	var heard []string
	c.resolved = func(installed, latest string) { heard = append(heard, installed+"->"+latest) }

	st, err := c.Ensure(context.Background())
	if err != nil {
		t.Fatalf("Ensure: %v", err)
	}
	if !st.Updated || st.Installed != "1:2.0.10-1" || atomic.LoadInt32(&rec.installs) != 1 {
		t.Fatalf("Ensure = %+v, installs %d", st, rec.installs)
	}
	if len(heard) != 1 || heard[0] != "->1:2.0.10-1" {
		t.Fatalf("resolved hook heard %v", heard)
	}
	if _, err := os.Stat(filepath.Join(c.cacheDir, "latest-hyperion.json")); err != nil {
		t.Fatalf("hyperion cache: %v", err)
	}
	if _, err := os.Stat(filepath.Join(c.cacheDir, "latest.json")); !os.IsNotExist(err) {
		t.Fatalf("the stable cache was written by the hyperion line: %v", err)
	}
}

// Only forward: a box already past the newest v2 release keeps what it has.
func TestHyperionLineNeverDowngrades(t *testing.T) {
	f := hyperionFixture()
	srv := f.server()
	defer srv.Close()
	var rec recorder
	c := lineClient(t, srv, Ryotunes.OnLine(HyperionLine), "1:2.1.0-1", &rec)

	st, err := c.Upgrade(context.Background())
	if err != nil {
		t.Fatalf("Upgrade: %v", err)
	}
	if st.Updated || atomic.LoadInt32(&rec.installs) != 0 {
		t.Fatalf("Upgrade moved backwards: %+v", st)
	}
}

// A list with no v2 release is an error, not a silent fall back to 1.x.
func TestHyperionLineWithoutV2Release(t *testing.T) {
	f := &listFixture{releases: []listRelease{{tag: "v1.2.6"}, {tag: "v2.0.0", draft: true}, {tag: "v2.5.1", sourceOnly: true}}}
	srv := f.server()
	defer srv.Close()
	var rec recorder
	c := lineClient(t, srv, Ryotunes.OnLine(HyperionLine), "1:1.2.6-1", &rec)

	if st, err := c.Check(context.Background()); err == nil {
		t.Fatalf("Check without a v2 release = %+v, want an error", st)
	}
}

// A poisoned hyperion cache naming a 1.x release fails the line's tag contract.
func TestHyperionCacheRejectsOtherMajor(t *testing.T) {
	app := Ryotunes.OnLine(HyperionLine)
	if validRelInfo(app, relInfo{Tag: "v1.2.6", Version: "1:1.2.6-1",
		PkgAsset: "ryotunes-1.2.6-1-x86_64.pkg.tar.zst", ShaAsset: "ryotunes-1.2.6-1-x86_64.pkg.tar.zst.sha256"}) {
		t.Fatal("a 1.x release passed the hyperion line's contract")
	}
	if !validRelInfo(Ryotunes, relInfo{Tag: "v1.2.6", Version: "1:1.2.6-1",
		PkgAsset: "ryotunes-1.2.6-1-x86_64.pkg.tar.zst", ShaAsset: "ryotunes-1.2.6-1-x86_64.pkg.tar.zst.sha256"}) {
		t.Fatal("the stable line rejected a 1.x release")
	}
}

// Only Ryotunes follows the hyperion line; every other line is stable.
func TestOnLine(t *testing.T) {
	cases := []struct {
		app  App
		line string
		want string
	}{
		{Ryotunes, HyperionLine, HyperionLine},
		{Ryotunes, "unstable-dev", ""},
		{Ryotunes, "main", ""},
		{Ryotunes, "", ""},
		{Ryoport, HyperionLine, ""},
		{Ryotunes.OnLine(HyperionLine), "", ""},
	}
	for _, c := range cases {
		got := c.app.OnLine(c.line)
		if got.Line != c.want || got.Prerelease() != (c.want != "") {
			t.Errorf("%s.OnLine(%q) line %q, want %q", c.app.Name, c.line, got.Line, c.want)
		}
		if err := got.validate(); err != nil {
			t.Errorf("%s.OnLine(%q): %v", c.app.Name, c.line, err)
		}
	}
	bad := Ryoport
	bad.Line = HyperionLine
	if bad.validate() == nil {
		t.Fatal("Ryoport on the hyperion line passed validation")
	}
}

// Options.Output takes the privileged commands' output instead of the terminal.
func TestPrivRunWritesToStreams(t *testing.T) {
	var out bytes.Buffer
	if err := privRun(context.Background(), streams{stdout: &out, stderr: &out}, true, "sh", "-c", "echo to-out; echo to-err >&2"); err != nil {
		t.Fatal(err)
	}
	if got := out.String(); !strings.Contains(got, "to-out") || !strings.Contains(got, "to-err") {
		t.Fatalf("streams got %q", got)
	}
}

// A line whose releases sit past the first page is still found, and the walk
// stops at the page that holds one.
func TestHyperionLinePagesPastOtherReleases(t *testing.T) {
	var releases []listRelease
	for i := 250; i > 0; i-- {
		releases = append(releases, listRelease{tag: fmt.Sprintf("v1.%d.0", i)})
	}
	releases = append(releases, listRelease{tag: "v2.0.1", prerelease: true}, listRelease{tag: "v2.0.0", prerelease: true})
	for i := 0; i < 120; i++ {
		releases = append(releases, listRelease{tag: fmt.Sprintf("v1.0.%d", i)})
	}
	f := &listFixture{releases: releases}
	srv := f.server()
	defer srv.Close()
	var rec recorder
	c := lineClient(t, srv, Ryotunes.OnLine(HyperionLine), "1:1.2.6-1", &rec)

	st, err := c.Check(context.Background())
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if st.Latest != "1:2.0.1-1" || f.pages != 3 {
		t.Fatalf("latest %q after %d pages, want 1:2.0.1-1 after 3", st.Latest, f.pages)
	}
}

// With no release on the line the walk ends at the list's end or the page cap.
func TestHyperionLinePageCap(t *testing.T) {
	var releases []listRelease
	for i := 0; i < (maxListPages+3)*100; i++ {
		releases = append(releases, listRelease{tag: fmt.Sprintf("v1.%d.0", i)})
	}
	f := &listFixture{releases: releases}
	srv := f.server()
	defer srv.Close()
	var rec recorder
	c := lineClient(t, srv, Ryotunes.OnLine(HyperionLine), "1:1.2.6-1", &rec)
	if _, err := c.Check(context.Background()); err == nil {
		t.Fatal("Check found a release on an empty line")
	}
	if f.pages != maxListPages {
		t.Fatalf("read %d pages, want the cap %d", f.pages, maxListPages)
	}
}
