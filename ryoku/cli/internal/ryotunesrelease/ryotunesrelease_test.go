package ryotunesrelease

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// fixture is a stand-in for the Ryotunes GitHub releases: one server that answers
// the paginated release list and the two asset downloads of f.tag, with knobs
// for the error shapes the client must handle.
type fixture struct {
	repo        string
	tag         string
	pkgAsset    string
	assets      []string // exact asset-name list to serve; nil => [pkgAsset, pkgAsset+".sha256"]
	listing     []fakeRelease
	pkgBytes    []byte
	shaOverride string // when set, the sha256 sidecar hex is this instead of the real one
	draft       bool
	prerelease  bool
	redirectPkg bool  // package download 302s to an untrusted host
	hits        int32 // total requests served
}

// fakeRelease is one entry of the release list, newest-created first.
type fakeRelease struct {
	tag        string
	assets     []string
	draft      bool
	prerelease bool
}

func (f *fixture) assetList() []string {
	if f.assets != nil {
		return f.assets
	}
	return []string{f.pkgAsset, f.pkgAsset + ".sha256"}
}

// releaseList is the full listing; nil listing => the single release f describes.
func (f *fixture) releaseList() []fakeRelease {
	if f.listing != nil {
		return f.listing
	}
	return []fakeRelease{{tag: f.tag, assets: f.assetList(), draft: f.draft, prerelease: f.prerelease}}
}

// publishedRelease is a release entry carrying a well-formed package, sidecar and
// source tarball, whatever its major version.
func publishedRelease(version string) fakeRelease {
	pkg := "ryotunes-" + version + "-1-x86_64.pkg.tar.zst"
	return fakeRelease{tag: "v" + version, assets: []string{pkg, pkg + ".sha256", "ryotunes-" + version + ".tar.gz"}}
}

func (f *fixture) server() *httptest.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&f.hits, 1)
		download := "/" + f.repo + "/releases/download/" + f.tag + "/"
		switch r.URL.Path {
		case "/repos/" + f.repo + "/releases":
			perPage, _ := strconv.Atoi(r.URL.Query().Get("per_page"))
			page, _ := strconv.Atoi(r.URL.Query().Get("page"))
			if perPage < 1 {
				perPage = 30
			}
			if page < 1 {
				page = 1
			}
			all := f.releaseList()
			out := make([]map[string]any, 0)
			for i := (page - 1) * perPage; i < len(all) && i < page*perPage; i++ {
				assets := make([]map[string]string, 0)
				for _, name := range all[i].assets {
					assets = append(assets, map[string]string{"name": name})
				}
				out = append(out, map[string]any{
					"tag_name":   all[i].tag,
					"draft":      all[i].draft,
					"prerelease": all[i].prerelease,
					"assets":     assets,
				})
			}
			_ = json.NewEncoder(w).Encode(out)
		case download + f.pkgAsset + ".sha256":
			sum := f.shaOverride
			if sum == "" {
				d := sha256.Sum256(f.pkgBytes)
				sum = hex.EncodeToString(d[:])
			}
			fmt.Fprintf(w, "%s  %s\n", sum, f.pkgAsset)
		case download + f.pkgAsset:
			if f.redirectPkg {
				http.Redirect(w, r, "http://untrusted.example/evil.pkg.tar.zst", http.StatusFound)
				return
			}
			_, _ = w.Write(f.pkgBytes)
		default:
			http.NotFound(w, r)
		}
	})
	return httptest.NewServer(mux)
}

// cmpVer is a deterministic version comparator for the test fixtures, so the
// tests do not depend on the system vercmp binary. It compares dot/dash-split
// numeric fields, which is all the fixtures need.
func cmpVer(a, b string) (int, error) {
	// Epoch dominates pacman ordering: split a leading "N:" and compare epochs
	// first, so 1:1.0.5-1 outranks 2.5.1-1 exactly as pacman's vercmp would.
	epoch := func(s string) (int, string) {
		if i := strings.IndexByte(s, ':'); i >= 0 {
			e, _ := strconv.Atoi(s[:i])
			return e, s[i+1:]
		}
		return 0, s
	}
	ea, ra := epoch(a)
	eb, rb := epoch(b)
	if ea != eb {
		if ea < eb {
			return -1, nil
		}
		return 1, nil
	}
	split := func(s string) []string {
		return strings.FieldsFunc(s, func(r rune) bool { return r == '.' || r == '-' })
	}
	fa, fb := split(ra), split(rb)
	for i := 0; i < len(fa) && i < len(fb); i++ {
		na, _ := strconv.Atoi(fa[i])
		nb, _ := strconv.Atoi(fb[i])
		if na != nb {
			if na < nb {
				return -1, nil
			}
			return 1, nil
		}
	}
	switch {
	case len(fa) < len(fb):
		return -1, nil
	case len(fa) > len(fb):
		return 1, nil
	}
	return 0, nil
}

// recorder captures what the injected installer was handed.
type recorder struct {
	installs int32
}

// newClient wires a Client to the fixture server with stubbed system deps, so no
// real pacman, sudo, network or package on disk is touched.
func newClient(t *testing.T, srv *httptest.Server, f *fixture, installed string, meta pkgMeta, installErr error, rec *recorder) *Client {
	t.Helper()
	return &Client{
		Repo:             f.repo,
		APIBase:          srv.URL,
		DownloadBase:     srv.URL,
		HTTP:             defaultHTTPClient(), // production redirect policy on purpose
		now:              time.Now,
		cacheDir:         t.TempDir(),
		checkTTL:         time.Hour,
		installedVersion: func(string) string { return installed },
		inspect: func(context.Context, string) (pkgMeta, error) {
			if meta.Name == "" {
				meta = pkgMeta{
					Name:    pkgName,
					Version: wantEpoch + ":" + strings.TrimSuffix(strings.TrimPrefix(f.pkgAsset, "ryotunes-"), "-"+wantArch+".pkg.tar.zst"),
					Arch:    wantArch,
				}
			}
			return meta, nil
		},
		install: func(_ context.Context, _ string, _ []byte) error {
			atomic.AddInt32(&rec.installs, 1)
			return installErr
		},
		vercmp: cmpVer,
	}
}

func stableFixture() *fixture {
	pkg := "ryotunes-1.2.5-1-x86_64.pkg.tar.zst"
	return &fixture{
		repo:     "ryoku-dev/ryotunes",
		tag:      "v1.2.5",
		pkgAsset: pkg,
		pkgBytes: []byte("fake ryotunes package payload"),
	}
}

// Check must not touch the network, or install anything, when Ryotunes is not
// installed: Ryoku tracks an app the box has, it does not resurrect a removed one.
func TestCheckSkipsUninstalled(t *testing.T) {
	f := stableFixture()
	srv := f.server()
	defer srv.Close()
	var rec recorder
	c := newClient(t, srv, f, "", pkgMeta{}, nil, &rec)

	st, err := c.Check(context.Background())
	if err != nil {
		t.Fatalf("Check: unexpected error: %v", err)
	}
	if st != (Status{}) {
		t.Fatalf("Check: want zero Status for uninstalled, got %+v", st)
	}
	if n := atomic.LoadInt32(&f.hits); n != 0 {
		t.Fatalf("Check hit the network %d time(s) while uninstalled", n)
	}
}

// Check reports a strictly-newer published build as available.
func TestCheckReportsAvailable(t *testing.T) {
	f := stableFixture()
	srv := f.server()
	defer srv.Close()
	var rec recorder
	c := newClient(t, srv, f, "1:1.2.4-1", pkgMeta{}, nil, &rec)

	st, err := c.Check(context.Background())
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	want := Status{Installed: "1:1.2.4-1", Latest: "1:1.2.5-1", Available: true}
	if st != want {
		t.Fatalf("Check: got %+v, want %+v", st, want)
	}
}

// Offline with a stale cache, Check falls back to the last-known release rather
// than erroring; offline with no cache, it errors instead of claiming current.
func TestCheckOfflineCacheThenError(t *testing.T) {
	f := stableFixture()
	srv := f.server()
	var rec recorder
	c := newClient(t, srv, f, "1:1.2.4-1", pkgMeta{}, nil, &rec)

	if _, err := c.Check(context.Background()); err != nil {
		t.Fatalf("warm Check: %v", err)
	}
	srv.Close() // go offline

	// Cache is now stale (advance the clock past the TTL); Check should serve the
	// stale entry rather than fail.
	c.now = func() time.Time { return time.Now().Add(2 * time.Hour) }
	st, err := c.Check(context.Background())
	if err != nil {
		t.Fatalf("offline Check with cache: unexpected error: %v", err)
	}
	if st.Latest != "1:1.2.5-1" {
		t.Fatalf("offline Check: want cached Latest 1:1.2.5-1, got %q", st.Latest)
	}

	// A fresh client with no cache and no server must surface the error, never a
	// false "up to date".
	c2 := newClient(t, srv, f, "1:1.2.4-1", pkgMeta{}, nil, &rec)
	st2, err := c2.Check(context.Background())
	if err == nil {
		t.Fatalf("offline Check without cache: want error, got Status %+v", st2)
	}
	if st2.Available {
		t.Fatalf("offline Check without cache: must not report Available")
	}
}

// A poisoned on-disk cache (an asset name with path traversal) is rejected by the
// cache's strict re-validation; offline, that leaves nothing usable, so Check
// errors rather than trusting the tampered entry.
func TestCheckRejectsPoisonedCache(t *testing.T) {
	f := stableFixture()
	srv := f.server()
	srv.Close() // force reliance on cache
	var rec recorder
	c := newClient(t, srv, f, "1:1.2.4-1", pkgMeta{}, nil, &rec)

	poison := relInfo{
		Tag:      "v1.2.5",
		Version:  "1:1.2.5-1",
		PkgAsset: "ryotunes-1.2.5-1-x86_64.pkg.tar.zst/../../etc/evil",
		ShaAsset: "ryotunes-1.2.5-1-x86_64.pkg.tar.zst.sha256",
	}
	b, _ := json.Marshal(poison)
	if err := os.MkdirAll(c.cacheDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(c.cacheDir, "latest.json"), b, 0o644); err != nil {
		t.Fatal(err)
	}

	st, err := c.Check(context.Background())
	if err == nil {
		t.Fatalf("Check: want error on poisoned cache, got Status %+v", st)
	}
	if strings.Contains(st.Latest, "..") || strings.Contains(st.Latest, "/") {
		t.Fatalf("Check: leaked a poisoned value: %+v", st)
	}
}

// Discovery refuses any release whose package asset does not name exactly the
// tag's version with a positive integer pkgrel and the x86_64 suffix -- version
// mismatch, path traversal, bad pkgrel, wrong arch, or an ambiguous pair.
func TestDiscoveryRejectsMalformedAssets(t *testing.T) {
	cases := map[string][]string{
		"tag/version mismatch": {"ryotunes-9.9.9-1-x86_64.pkg.tar.zst", "ryotunes-9.9.9-1-x86_64.pkg.tar.zst.sha256"},
		"path traversal":       {"ryotunes-1.2.5-../../evil-x86_64.pkg.tar.zst", "ryotunes-1.2.5-../../evil-x86_64.pkg.tar.zst.sha256"},
		"zero pkgrel":          {"ryotunes-1.2.5-0-x86_64.pkg.tar.zst", "ryotunes-1.2.5-0-x86_64.pkg.tar.zst.sha256"},
		"nonnumeric pkgrel":    {"ryotunes-1.2.5-1a-x86_64.pkg.tar.zst", "ryotunes-1.2.5-1a-x86_64.pkg.tar.zst.sha256"},
		"wrong arch":           {"ryotunes-1.2.5-1-aarch64.pkg.tar.zst", "ryotunes-1.2.5-1-aarch64.pkg.tar.zst.sha256"},
		"ambiguous pkgrel":     {"ryotunes-1.2.5-1-x86_64.pkg.tar.zst", "ryotunes-1.2.5-2-x86_64.pkg.tar.zst", "ryotunes-1.2.5-1-x86_64.pkg.tar.zst.sha256"},
	}
	for name, assets := range cases {
		t.Run(name, func(t *testing.T) {
			f := stableFixture()
			f.assets = assets
			srv := f.server()
			defer srv.Close()
			var rec recorder
			c := newClient(t, srv, f, "1:1.2.4-1", pkgMeta{}, nil, &rec)

			st, err := c.Check(context.Background())
			if err == nil {
				t.Fatalf("Check: want rejection, got Status %+v", st)
			}
			if st.Available || st.Latest != "" {
				t.Fatalf("Check: must not report a malformed release as available (%+v)", st)
			}
		})
	}
}

// A draft/prerelease returned by a stubbed endpoint is refused.
func TestDiscoveryRejectsPrerelease(t *testing.T) {
	f := stableFixture()
	f.prerelease = true
	srv := f.server()
	defer srv.Close()
	var rec recorder
	c := newClient(t, srv, f, "1:1.2.4-1", pkgMeta{}, nil, &rec)

	if st, err := c.Check(context.Background()); err == nil {
		t.Fatalf("Check: want prerelease rejection, got %+v", st)
	}
}

// A checksum that does not match the downloaded bytes aborts the upgrade before
// pacman runs.
func TestUpgradeChecksumMismatch(t *testing.T) {
	f := stableFixture()
	f.shaOverride = strings.Repeat("00", sha256.Size) // wrong digest
	srv := f.server()
	defer srv.Close()
	var rec recorder
	c := newClient(t, srv, f, "1:1.2.4-1", pkgMeta{}, nil, &rec)

	st, err := c.Upgrade(context.Background())
	if err == nil {
		t.Fatalf("Upgrade: want checksum error, got Status %+v", st)
	}
	if !strings.Contains(err.Error(), "sha256") {
		t.Fatalf("Upgrade: want sha256 error, got %v", err)
	}
	if st.Updated {
		t.Fatalf("Upgrade: Updated must be false on checksum mismatch")
	}
	if n := atomic.LoadInt32(&rec.installs); n != 0 {
		t.Fatalf("Upgrade: installer ran %d time(s) despite checksum mismatch", n)
	}
}

// A package whose own metadata is not Ryotunes/x86_64/expected-version is refused
// before pacman runs, even when its checksum is correct.
func TestUpgradeWrongPackageIdentity(t *testing.T) {
	cases := map[string]pkgMeta{
		"wrong name":    {Name: "notryotunes", Version: "1:1.2.5-1", Arch: wantArch},
		"wrong arch":    {Name: pkgName, Version: "1:1.2.5-1", Arch: "aarch64"},
		"wrong version": {Name: pkgName, Version: "1:9.9.9-1", Arch: wantArch},
	}
	for name, meta := range cases {
		t.Run(name, func(t *testing.T) {
			f := stableFixture()
			srv := f.server()
			defer srv.Close()
			var rec recorder
			c := newClient(t, srv, f, "1:1.2.4-1", meta, nil, &rec)

			st, err := c.Upgrade(context.Background())
			if err == nil {
				t.Fatalf("Upgrade: want identity error, got Status %+v", st)
			}
			if st.Updated || atomic.LoadInt32(&rec.installs) != 0 {
				t.Fatalf("Upgrade: installed despite wrong identity (%+v)", st)
			}
		})
	}
}

// Upgrade never installs a build that is not strictly newer than what is
// installed: no download, no pacman, Updated false.
func TestUpgradeNoDowngrade(t *testing.T) {
	for _, installed := range []string{"1:1.2.5-1", "1:1.2.6-1"} {
		t.Run("installed_"+installed, func(t *testing.T) {
			f := stableFixture() // latest is 1:1.2.5-1
			srv := f.server()
			defer srv.Close()
			var rec recorder
			c := newClient(t, srv, f, installed, pkgMeta{}, nil, &rec)

			st, err := c.Upgrade(context.Background())
			if err != nil {
				t.Fatalf("Upgrade: %v", err)
			}
			if st.Updated {
				t.Fatalf("Upgrade: Updated true for non-newer release (installed %s)", installed)
			}
			if n := atomic.LoadInt32(&rec.installs); n != 0 {
				t.Fatalf("Upgrade: installer ran %d time(s) for a non-upgrade", n)
			}
		})
	}
}

// If the installed state changes during the download (the app is removed, or
// upgraded past the candidate), the install is abandoned rather than resurrecting
// or downgrading it.
func TestUpgradeAbortsOnInstalledChange(t *testing.T) {
	for name, second := range map[string]string{"removed": "", "upgraded past": "1:1.2.9-1"} {
		t.Run(name, func(t *testing.T) {
			f := stableFixture()
			srv := f.server()
			defer srv.Close()
			var rec recorder
			c := newClient(t, srv, f, "1:1.2.4-1", pkgMeta{}, nil, &rec)

			// First read (upgrade decision) sees the old version; the read just
			// before install sees the changed state.
			var calls int32
			c.installedVersion = func(string) string {
				if atomic.AddInt32(&calls, 1) == 1 {
					return "1:1.2.4-1"
				}
				return second
			}

			st, err := c.Upgrade(context.Background())
			if err == nil {
				t.Fatalf("Upgrade: want abort on state change, got Status %+v", st)
			}
			if st.Updated || atomic.LoadInt32(&rec.installs) != 0 {
				t.Fatalf("Upgrade: installed despite state change (%+v)", st)
			}
		})
	}
}

// A download that redirects off GitHub to another origin is refused; the upgrade
// fails rather than installing bytes from an untrusted host.
func TestUpgradeRefusesUntrustedRedirect(t *testing.T) {
	f := stableFixture()
	f.redirectPkg = true
	srv := f.server()
	defer srv.Close()
	var rec recorder
	c := newClient(t, srv, f, "1:1.2.4-1", pkgMeta{}, nil, &rec)

	st, err := c.Upgrade(context.Background())
	if err == nil {
		t.Fatalf("Upgrade: want redirect error, got Status %+v", st)
	}
	if st.Updated || atomic.LoadInt32(&rec.installs) != 0 {
		t.Fatalf("Upgrade: installed across an untrusted redirect (%+v)", st)
	}
}

func v1LineFixture(listing []fakeRelease) *fixture {
	f := stableFixture()
	f.tag = "v1.2.7"
	f.pkgAsset = "ryotunes-1.2.7-1-x86_64.pkg.tar.zst"
	f.listing = listing
	return f
}

// A Ryotunes 2 release that GitHub marks latest, the retired 2.x tags, drafts and
// prereleases never reach a v1 box: the highest stable 1.x release is installed.
func TestDiscoveryFollowsOnlyV1Line(t *testing.T) {
	draft := publishedRelease("1.3.0")
	draft.draft = true
	pre := publishedRelease("1.2.9")
	pre.prerelease = true
	f := v1LineFixture([]fakeRelease{
		publishedRelease("2.0.0"),
		draft,
		pre,
		publishedRelease("1.2.6"),
		publishedRelease("1.2.7"),
		{tag: "v2.5.1", assets: []string{"ryotunes-2.5.1.tar.gz", "ryotunes-2.5.1.tar.gz.sha256"}},
	})
	srv := f.server()
	defer srv.Close()
	var rec recorder
	c := newClient(t, srv, f, "1:1.2.6-1", pkgMeta{}, nil, &rec)

	st, err := c.Check(context.Background())
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if want := (Status{Installed: "1:1.2.6-1", Latest: "1:1.2.7-1", Available: true}); st != want {
		t.Fatalf("Check: got %+v, want %+v", st, want)
	}
	st, err = c.Upgrade(context.Background())
	if err != nil {
		t.Fatalf("Upgrade: %v", err)
	}
	if want := (Status{Installed: "1:1.2.7-1", Latest: "1:1.2.7-1", Updated: true}); st != want {
		t.Fatalf("Upgrade: got %+v, want %+v", st, want)
	}
	if n := atomic.LoadInt32(&rec.installs); n != 1 {
		t.Fatalf("Upgrade: installer ran %d time(s), want 1", n)
	}
}

// A page full of newer 2.x releases does not hide the 1.x line.
func TestDiscoveryPagesPastV2Releases(t *testing.T) {
	var listing []fakeRelease
	for i := range releasePageSize {
		listing = append(listing, publishedRelease(fmt.Sprintf("2.0.%d", releasePageSize-i)))
	}
	f := v1LineFixture(append(listing, publishedRelease("1.2.7"), publishedRelease("1.2.6")))
	srv := f.server()
	defer srv.Close()
	var rec recorder
	c := newClient(t, srv, f, "1:1.2.6-1", pkgMeta{}, nil, &rec)

	st, err := c.Check(context.Background())
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if st.Latest != "1:1.2.7-1" || !st.Available {
		t.Fatalf("Check: got %+v, want Latest 1:1.2.7-1 available", st)
	}
}

// A cache left by an older client that followed GitHub's latest release can hold
// a Ryotunes 2 entry; offline, it is refused rather than reported as available.
func TestCheckRefusesCachedV2Release(t *testing.T) {
	f := stableFixture()
	srv := f.server()
	srv.Close()
	var rec recorder
	c := newClient(t, srv, f, "1:1.2.6-1", pkgMeta{}, nil, &rec)
	b, _ := json.Marshal(relInfo{
		Tag:      "v2.0.0",
		Version:  "1:2.0.0-1",
		PkgAsset: "ryotunes-2.0.0-1-x86_64.pkg.tar.zst",
		ShaAsset: "ryotunes-2.0.0-1-x86_64.pkg.tar.zst.sha256",
	})
	if err := os.MkdirAll(c.cacheDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(c.cacheDir, "latest.json"), b, 0o644); err != nil {
		t.Fatal(err)
	}

	st, err := c.Check(context.Background())
	if err == nil || st.Available || st.Latest != "" {
		t.Fatalf("Check: want an error and no 2.x Latest, got %+v, %v", st, err)
	}
}

// With only 2.x releases published the installed build stays and the caller gets
// ErrNoRelease, never a v2 install or a cached guess.
func TestDiscoveryWithoutV1ReleaseKeepsInstalled(t *testing.T) {
	f := v1LineFixture([]fakeRelease{
		publishedRelease("2.0.0"),
		{tag: "v2.5.1", assets: []string{"ryotunes-2.5.1.tar.gz", "ryotunes-2.5.1.tar.gz.sha256"}},
	})
	srv := f.server()
	defer srv.Close()
	var rec recorder
	c := newClient(t, srv, f, "1:1.2.6-1", pkgMeta{}, nil, &rec)

	for name, run := range map[string]func(context.Context) (Status, error){
		"check":   c.Check,
		"upgrade": c.Upgrade,
		"ensure":  c.Ensure,
	} {
		st, err := run(context.Background())
		if !errors.Is(err, ErrNoRelease) {
			t.Fatalf("%s: want ErrNoRelease, got %v", name, err)
		}
		if want := (Status{Installed: "1:1.2.6-1"}); st != want {
			t.Fatalf("%s: got %+v, want %+v", name, st, want)
		}
	}
	if n := atomic.LoadInt32(&rec.installs); n != 0 {
		t.Fatalf("installer ran %d time(s) with no 1.x release", n)
	}
}

// The v1 package must upgrade the retired higher-numbered v2 build because its
// pacman epoch takes precedence over the semantic version.
func TestUpgradeInstallsNewer(t *testing.T) {
	f := stableFixture()
	f.tag = "v1.0.5"
	f.pkgAsset = "ryotunes-1.0.5-1-x86_64.pkg.tar.zst"
	srv := f.server()
	defer srv.Close()
	var rec recorder
	c := newClient(t, srv, f, "2.5.1-1", pkgMeta{}, nil, &rec)

	st, err := c.Upgrade(context.Background())
	if err != nil {
		t.Fatalf("Upgrade: %v", err)
	}
	want := Status{Installed: "1:1.0.5-1", Latest: "1:1.0.5-1", Updated: true}
	if st != want {
		t.Fatalf("Upgrade: got %+v, want %+v", st, want)
	}
	if n := atomic.LoadInt32(&rec.installs); n != 1 {
		t.Fatalf("Upgrade: installer ran %d time(s), want 1", n)
	}
}

// Ensure installs the latest build when Ryotunes is absent -- the install path
// (a fresh box, or a reconcile after the user removed it), unlike Upgrade which
// leaves a removed app removed -- and is a no-op when the box is already current.
func TestEnsure(t *testing.T) {
	t.Run("absent installs", func(t *testing.T) {
		f := stableFixture()
		srv := f.server()
		defer srv.Close()
		var rec recorder
		c := newClient(t, srv, f, "", pkgMeta{}, nil, &rec) // not installed

		st, err := c.Ensure(context.Background())
		if err != nil {
			t.Fatalf("Ensure: %v", err)
		}
		want := Status{Installed: "1:1.2.5-1", Latest: "1:1.2.5-1", Updated: true}
		if st != want {
			t.Fatalf("Ensure: got %+v, want %+v", st, want)
		}
		if n := atomic.LoadInt32(&rec.installs); n != 1 {
			t.Fatalf("Ensure: installer ran %d time(s), want 1", n)
		}
	})
	t.Run("current is a no-op", func(t *testing.T) {
		f := stableFixture()
		srv := f.server()
		defer srv.Close()
		var rec recorder
		c := newClient(t, srv, f, "1:1.2.5-1", pkgMeta{}, nil, &rec)

		st, err := c.Ensure(context.Background())
		if err != nil {
			t.Fatalf("Ensure: %v", err)
		}
		if st.Updated {
			t.Fatalf("Ensure: Updated true for an already-current box")
		}
		if n := atomic.LoadInt32(&rec.installs); n != 0 {
			t.Fatalf("Ensure: installer ran %d time(s) for a current box", n)
		}
	})
}

// isTrustedGitHubHost admits only the concrete GitHub hosts a release download
// traverses, and rejects look-alikes and unrelated *.githubusercontent.com hosts.
func TestTrustedHostAllowlist(t *testing.T) {
	ok := []string{"github.com", "API.GitHub.com", "objects.githubusercontent.com", "release-assets.githubusercontent.com"}
	bad := []string{"raw.githubusercontent.com", "evil.com", "github.com.evil.com", "githubusercontent.com", "notgithub.com"}
	for _, h := range ok {
		if !isTrustedGitHubHost(h) {
			t.Errorf("host %q should be trusted", h)
		}
	}
	for _, h := range bad {
		if isTrustedGitHubHost(h) {
			t.Errorf("host %q should NOT be trusted", h)
		}
	}
}

// verifyStagedDigest is the root-side re-check gate: it accepts a matching
// sha256sum line and rejects a mismatch or a malformed digest.
func TestVerifyStagedDigest(t *testing.T) {
	payload := []byte("staged bytes")
	sum := sha256.Sum256(payload)
	line := hex.EncodeToString(sum[:]) + "  /var/tmp/ryotunes-release.XXXX/ryotunes-2.5.0-1-x86_64.pkg.tar.zst\n"

	if err := verifyStagedDigest(line, sum[:]); err != nil {
		t.Fatalf("verifyStagedDigest: matching digest rejected: %v", err)
	}
	other := sha256.Sum256([]byte("different"))
	if err := verifyStagedDigest(line, other[:]); err == nil {
		t.Fatalf("verifyStagedDigest: mismatch accepted")
	}
	if err := verifyStagedDigest("", sum[:]); err == nil {
		t.Fatalf("verifyStagedDigest: empty output accepted")
	}
	if err := verifyStagedDigest("nothex  file\n", sum[:]); err == nil {
		t.Fatalf("verifyStagedDigest: malformed hex accepted")
	}
}

func TestPackageOperationsAreUnavailableWithoutPacman(t *testing.T) {
	for _, manager := range []string{"dnf", "xbps"} {
		t.Run(manager, func(t *testing.T) {
			t.Setenv("RYOKU_HOST_PKGMGR", manager)
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			for name, operation := range map[string]func(context.Context) (Status, error){
				"check":   Check,
				"upgrade": Upgrade,
				"ensure":  Ensure,
			} {
				t.Run(name, func(t *testing.T) {
					status, err := operation(ctx)
					if err != nil {
						t.Fatalf("%s returned %v", name, err)
					}
					if status != (Status{}) {
						t.Fatalf("%s status = %+v", name, status)
					}
				})
			}
		})
	}
}
