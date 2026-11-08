package apprelease

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
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

var (
	tagRe        = regexp.MustCompile(`^v[0-9]+\.[0-9]+\.[0-9]+$`)
	ryoportTagRe = regexp.MustCompile(`^v1\.[0-9]+\.[0-9]$`)
	pkgRelRe     = regexp.MustCompile(`^[1-9][0-9]*$`)
)

func validTag(app App, tag string) bool {
	if app.Name == Ryoport.Name {
		return ryoportTagRe.MatchString(tag)
	}
	return tagRe.MatchString(tag)
}

func validPkgrel(app App, pkgrel string) bool {
	return pkgRelRe.MatchString(pkgrel) && (app.Name != Ryoport.Name || pkgrel == "1")
}

// pkgVer is the pacman pkgver a validated vX.Y.Z tag stands for (the tag with its
// leading "v" dropped). Callers must have matched tagRe first.
func pkgVer(tag string) string { return strings.TrimPrefix(tag, "v") }

func pkgAssetName(app App, pkgver, pkgrel string) string {
	return app.Name + "-" + pkgver + "-" + pkgrel + "-" + wantArch + ".pkg.tar.zst"
}

func packageVersion(app App, pkgver, pkgrel string) string {
	version := pkgver + "-" + pkgrel
	if app.Epoch != "" {
		return app.Epoch + ":" + version
	}
	return version
}

// resolveAsset picks the single package asset matching the app and release tag.
func resolveAsset(app App, tag string, assets []asset) (name, version string, err error) {
	if !validTag(app, tag) {
		return "", "", fmt.Errorf("%s release tag %q does not match its stable tag contract", app.Name, tag)
	}
	pv := pkgVer(tag)
	prefix := app.Name + "-" + pv + "-"
	suffix := "-" + wantArch + ".pkg.tar.zst"
	var gotName, gotRel string
	for _, a := range assets {
		rest, ok := strings.CutPrefix(a.Name, prefix)
		if !ok {
			continue
		}
		rel, ok := strings.CutSuffix(rest, suffix)
		if !ok || !validPkgrel(app, rel) {
			continue
		}
		if gotName != "" {
			return "", "", fmt.Errorf("%s release %s offers more than one %s package asset", app.Name, tag, wantArch)
		}
		gotName, gotRel = a.Name, rel
	}
	if gotName == "" {
		return "", "", fmt.Errorf("%s release %s has no %s package asset for version %s", app.Name, tag, wantArch, pv)
	}
	return gotName, packageVersion(app, pv, gotRel), nil
}

// validRelInfo reports whether rel is internally consistent under the strict
// naming rules, so a cache entry -- which lives on disk and could have been
// tampered with -- is re-checked exactly like a freshly fetched release before
// it is ever trusted to build a download URL.
func validRelInfo(app App, rel relInfo) bool {
	if !validTag(app, rel.Tag) {
		return false
	}
	pv := pkgVer(rel.Tag)
	versionPrefix := pv + "-"
	if app.Epoch != "" {
		versionPrefix = app.Epoch + ":" + versionPrefix
	}
	rel_, ok := strings.CutPrefix(rel.Version, versionPrefix)
	if !ok || !validPkgrel(app, rel_) {
		return false
	}
	if rel.PkgAsset != pkgAssetName(app, pv, rel_) {
		return false
	}
	return rel.ShaAsset == rel.PkgAsset+".sha256"
}

// latestRelease resolves the newest stable app release, using the one-hour
// check cache unless fresh is requested.
func (c *Client) latestRelease(ctx context.Context, fresh bool) (relInfo, error) {
	cachePath := filepath.Join(c.cacheDir, "latest.json")

	if !fresh {
		if rel, ok := c.readCache(cachePath, true); ok {
			return rel, nil
		}
	}

	rel, err := c.fetchLatest(ctx)
	if err != nil {
		if !fresh {
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
	if json.Unmarshal(b, &rel) != nil || !validRelInfo(c.app, rel) {
		return relInfo{}, false
	}
	return rel, true
}

// writeCache atomically persists rel for the next check. Failure only costs a
// future lookup.
func (c *Client) writeCache(path string, rel relInfo) {
	b, err := json.Marshal(rel)
	if err != nil || os.MkdirAll(filepath.Dir(path), 0o755) != nil {
		return
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".latest-*.json")
	if err != nil {
		return
	}
	tmp := file.Name()
	defer os.Remove(tmp)
	if file.Chmod(0o644) != nil {
		_ = file.Close()
		return
	}
	if _, err := file.Write(b); err != nil {
		_ = file.Close()
		return
	}
	if file.Sync() != nil || file.Close() != nil {
		_ = file.Close()
		return
	}
	_ = os.Rename(tmp, path)
}

// fetchLatest asks the GitHub API for the newest stable app release.
func (c *Client) fetchLatest(ctx context.Context) (relInfo, error) {
	ctx, cancel := context.WithTimeout(ctx, metadataTimeout)
	defer cancel()

	url := strings.TrimSuffix(c.APIBase, "/") + "/repos/" + c.Repo + "/releases/latest"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return relInfo{}, err
	}
	// GitHub 403s an API request with no User-Agent; the media type pins the
	// response shape.
	req.Header.Set("User-Agent", "ryoku-cli")
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return relInfo{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return relInfo{}, fmt.Errorf("%s release lookup: GitHub returned %s", c.app.Name, resp.Status)
	}

	var gr ghRelease
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxMetaBytes)).Decode(&gr); err != nil {
		return relInfo{}, fmt.Errorf("%s release lookup: %w", c.app.Name, err)
	}
	if gr.Draft || gr.Prerelease {
		return relInfo{}, fmt.Errorf("%s release %q is a draft/prerelease, not a stable build", c.app.Name, gr.TagName)
	}
	if !validTag(c.app, gr.TagName) {
		return relInfo{}, fmt.Errorf("%s release tag %q does not match its stable tag contract", c.app.Name, gr.TagName)
	}

	name, ver, err := resolveAsset(c.app, gr.TagName, gr.Assets)
	if err != nil {
		return relInfo{}, err
	}
	sha := name + ".sha256"
	if !hasAsset(gr.Assets, sha) {
		return relInfo{}, fmt.Errorf("%s release %s is missing the checksum asset %s", c.app.Name, gr.TagName, sha)
	}
	rel := relInfo{Tag: gr.TagName, Version: ver, PkgAsset: name, ShaAsset: sha}
	if !validRelInfo(c.app, rel) {
		return relInfo{}, fmt.Errorf("%s release %s produced an inconsistent asset set", c.app.Name, gr.TagName)
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
