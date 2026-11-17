package ryotunesrelease

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// relInfo is the slice of a GitHub release this client needs: the tag it was cut
// from, the pacman version parsed off the package asset's filename, and the two
// asset filenames (the package and its checksum sidecar). Download URLs are not
// stored: they are rebuilt from the trusted download base, the repo, the tag and
// the asset name, so a compromised or overridden API can never redirect the
// install to bytes off some other host.
type relInfo struct {
	Tag      string `json:"tag"`
	Version  string `json:"version"`
	PkgAsset string `json:"pkgAsset"`
	ShaAsset string `json:"shaAsset"`
}

// asset is one release asset. Only its name is read; browser_download_url is
// deliberately ignored (see relInfo): trusting the API's own URL would let a
// stubbed or hijacked API point the download anywhere.
type asset struct {
	Name string `json:"name"`
}

// ghRelease is the subset of GitHub's release payload we read.
type ghRelease struct {
	TagName    string  `json:"tag_name"`
	Draft      bool    `json:"draft"`
	Prerelease bool    `json:"prerelease"`
	Assets     []asset `json:"assets"`
}

// tagRe is the only tag shape this lane follows: v1.Y.Z. Ryotunes 2 and the
// retired 2.x tags share the repo and must never reach a v1 box; anything else
// (a moving dev tag, a malformed name) is refused before URL construction.
var tagRe = regexp.MustCompile(`^v1\.[0-9]+\.[0-9]+$`)

// ErrNoRelease means the repo lists no stable 1.x release, so the installed
// build stays as it is.
var ErrNoRelease = errors.New("no Ryotunes 1.x release is published")

// The release list is newest-created first; a few pages reach past a long run
// of 2.x releases without walking the whole history.
const (
	releasePageSize = 100
	maxReleasePages = 10
)

// pkgRelRe is a valid, positive pkgrel: a plain integer with no leading zero.
// Ryotunes pins pkgrel to 1 and only ever bumps it (2, 3, ...) for a rebuild, so
// a decimal, zero, empty, or otherwise non-integer pkgrel is not something this
// channel publishes and is refused before it can reach a filename or URL.
var pkgRelRe = regexp.MustCompile(`^[1-9][0-9]*$`)

// pkgVer is the pacman pkgver a validated vX.Y.Z tag stands for (the tag with its
// leading "v" dropped). Callers must have matched tagRe first.
func pkgVer(tag string) string { return strings.TrimPrefix(tag, "v") }

// pkgAssetName is the one legal package-asset filename for a release: every
// component is fixed -- the "ryotunes-" prefix, the tag's own X.Y.Z pkgver, an
// integer pkgrel, the x86_64 arch and the .pkg.tar.zst suffix. Because the name
// is rebuilt from these pieces, nothing else (a slash, backslash, "..", a query
// or fragment, an unrelated version) can ever appear in the string that feeds
// URL and path construction.
func pkgAssetName(pkgver, pkgrel string) string {
	return "ryotunes-" + pkgver + "-" + pkgrel + "-" + wantArch + ".pkg.tar.zst"
}

// resolveAsset picks the single package asset a validated tag legitimately
// carries. A candidate name must be exactly ryotunes-<tag X.Y.Z>-<pkgrel>-x86_64.pkg.tar.zst
// for a positive integer pkgrel; anything else (a mismatched version, a bad
// pkgrel, a traversal or separator smuggled into the name) is ignored. It
// returns the (epochless) asset name and the pacman version, which carries the
// fixed epoch the package is published under (wantEpoch:pkgver-pkgrel), and
// refuses a
// release that offers none, or more than one, such asset -- ambiguity is treated
// as untrustworthy rather than "pick one".
func resolveAsset(tag string, assets []asset) (name, version string, err error) {
	pv := pkgVer(tag)
	prefix := "ryotunes-" + pv + "-"
	suffix := "-" + wantArch + ".pkg.tar.zst"
	var gotName, gotRel string
	for _, a := range assets {
		rest, ok := strings.CutPrefix(a.Name, prefix)
		if !ok {
			continue
		}
		rel, ok := strings.CutSuffix(rest, suffix)
		if !ok || !pkgRelRe.MatchString(rel) {
			continue
		}
		if gotName != "" {
			return "", "", fmt.Errorf("ryotunes release %s offers more than one %s package asset", tag, wantArch)
		}
		gotName, gotRel = a.Name, rel
	}
	if gotName == "" {
		return "", "", fmt.Errorf("ryotunes release %s has no %s package asset for version %s", tag, wantArch, pv)
	}
	return gotName, wantEpoch + ":" + pv + "-" + gotRel, nil
}

// validRelInfo reports whether rel is internally consistent under the strict
// naming rules, so a cache entry -- which lives on disk and could have been
// tampered with -- is re-checked exactly like a freshly fetched release before
// it is ever trusted to build a download URL.
func validRelInfo(rel relInfo) bool {
	if !tagRe.MatchString(rel.Tag) {
		return false
	}
	pv := pkgVer(rel.Tag)
	rel_, ok := strings.CutPrefix(rel.Version, wantEpoch+":"+pv+"-")
	if !ok || !pkgRelRe.MatchString(rel_) {
		return false
	}
	if rel.PkgAsset != pkgAssetName(pv, rel_) {
		return false
	}
	return rel.ShaAsset == rel.PkgAsset+".sha256"
}

// latestRelease resolves the newest published stable Ryotunes 1.x release. When
// fresh is false it serves a cached lookup within checkCacheTTL and, if the
// network fails, falls back to the last cached result (so a doctor run offline
// still reports the last-known latest). When fresh is true it always hits the
// network and never falls back to a stale cache: an install decision is made
// only against what GitHub serves right now. ErrNoRelease is never papered over
// with the cache: GitHub answered, it just lists nothing this lane may follow.
func (c *Client) latestRelease(ctx context.Context, fresh bool) (relInfo, error) {
	cachePath := filepath.Join(c.cacheDir, "latest.json")

	if !fresh {
		if rel, ok := c.readCache(cachePath, true); ok {
			return rel, nil
		}
	}

	rel, err := c.fetchLatest(ctx)
	if err != nil {
		if !fresh && !errors.Is(err, ErrNoRelease) {
			// Fall back to whatever we last saw, ignoring its age: a stale
			// "latest" beats pretending we could not tell. Only Check does this.
			if rel, ok := c.readCache(cachePath, false); ok {
				return rel, nil
			}
		}
		return relInfo{}, err
	}

	c.writeCache(cachePath, rel)
	return rel, nil
}

// readCache returns the cached release, re-validated under the same strict rules
// as a fresh fetch (a cache file is on-disk state that could have been tampered
// with, so it is never trusted blindly). When freshOnly is set it is returned
// only while still within checkCacheTTL; otherwise any valid cached entry is
// accepted (the offline fallback). The read is bounded: the cache is a few short
// fields, so a bloated file is treated as corrupt.
func (c *Client) readCache(path string, freshOnly bool) (relInfo, bool) {
	st, err := os.Stat(path)
	if err != nil {
		return relInfo{}, false
	}
	if freshOnly && c.now().Sub(st.ModTime()) >= c.checkTTL {
		return relInfo{}, false
	}
	f, err := os.Open(path)
	if err != nil {
		return relInfo{}, false
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, maxCacheBytes))
	if err != nil {
		return relInfo{}, false
	}
	var rel relInfo
	if json.Unmarshal(b, &rel) != nil || !validRelInfo(rel) {
		return relInfo{}, false
	}
	return rel, true
}

// writeCache persists rel for the next Check. Best-effort: a cache we cannot
// write only costs a future lookup, it never fails the current one.
func (c *Client) writeCache(path string, rel relInfo) {
	b, err := json.Marshal(rel)
	if err != nil {
		return
	}
	if os.MkdirAll(filepath.Dir(path), 0o755) != nil {
		return
	}
	_ = os.WriteFile(path, b, 0o644)
}

// fetchLatest lists the repo's releases and turns the newest stable 1.x one into
// a relInfo, validating the tag shape and locating the x86_64 package asset and
// its checksum sidecar. GitHub's "latest" release is not used: it follows
// whatever was published last, which can be a Ryotunes 2 build. The scan stops at
// the first page holding a 1.x release, or at the end of the list.
func (c *Client) fetchLatest(ctx context.Context) (relInfo, error) {
	ctx, cancel := context.WithTimeout(ctx, metadataTimeout)
	defer cancel()

	for page := 1; page <= maxReleasePages; page++ {
		list, err := c.fetchReleasePage(ctx, page)
		if err != nil {
			return relInfo{}, err
		}
		if gr, ok := newestV1(list); ok {
			return releaseInfo(gr)
		}
		if len(list) < releasePageSize {
			break
		}
	}
	return relInfo{}, ErrNoRelease
}

func (c *Client) fetchReleasePage(ctx context.Context, page int) ([]ghRelease, error) {
	url := strings.TrimSuffix(c.APIBase, "/") + "/repos/" + c.Repo +
		"/releases?per_page=" + strconv.Itoa(releasePageSize) + "&page=" + strconv.Itoa(page)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	// GitHub 403s an API request with no User-Agent; the media type pins the
	// response shape.
	req.Header.Set("User-Agent", "ryoku-cli")
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ryotunes release lookup: GitHub returned %s", resp.Status)
	}

	var list []ghRelease
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxMetaBytes)).Decode(&list); err != nil {
		return nil, fmt.Errorf("ryotunes release lookup: %w", err)
	}
	return list, nil
}

// newestV1 picks the highest-versioned stable 1.x release by version, not by
// list order, skipping drafts, prereleases and every other tag shape.
func newestV1(list []ghRelease) (ghRelease, bool) {
	var best ghRelease
	var bestKey []int
	for _, gr := range list {
		if gr.Draft || gr.Prerelease || !tagRe.MatchString(gr.TagName) {
			continue
		}
		key, ok := tagKey(gr.TagName)
		if ok && (bestKey == nil || slices.Compare(bestKey, key) < 0) {
			best, bestKey = gr, key
		}
	}
	return best, bestKey != nil
}

// tagKey is the numeric X.Y.Z of a tag that already matched tagRe.
func tagKey(tag string) ([]int, bool) {
	parts := strings.Split(pkgVer(tag), ".")
	key := make([]int, len(parts))
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil {
			return nil, false
		}
		key[i] = n
	}
	return key, true
}

// releaseInfo validates the chosen release's assets under the strict naming
// rules and returns what the installer needs.
func releaseInfo(gr ghRelease) (relInfo, error) {
	name, ver, err := resolveAsset(gr.TagName, gr.Assets)
	if err != nil {
		return relInfo{}, err
	}
	sha := name + ".sha256"
	if !hasAsset(gr.Assets, sha) {
		return relInfo{}, fmt.Errorf("ryotunes release %s is missing the checksum asset %s", gr.TagName, sha)
	}
	rel := relInfo{Tag: gr.TagName, Version: ver, PkgAsset: name, ShaAsset: sha}
	if !validRelInfo(rel) {
		// resolveAsset already enforces the rules; this is a belt-and-braces guard
		// so nothing inconsistent can ever leave this function.
		return relInfo{}, fmt.Errorf("ryotunes release %s produced an inconsistent asset set", gr.TagName)
	}
	return rel, nil
}

// hasAsset reports whether an asset with exactly this name is present.
func hasAsset(assets []asset, name string) bool {
	for _, a := range assets {
		if a.Name == name {
			return true
		}
	}
	return false
}

// assetURL is the download URL for one asset, built only from the trusted
// download base, the repo, the tag and the asset name. It never comes from the
// API payload, so the origin of an install can only ever be the release path of
// the configured repo on the configured (GitHub) host.
func (c *Client) assetURL(rel relInfo, name string) string {
	return strings.TrimSuffix(c.DownloadBase, "/") + "/" + c.Repo + "/releases/download/" + rel.Tag + "/" + name
}
