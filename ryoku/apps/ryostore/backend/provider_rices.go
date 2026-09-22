// The rice provider owns remote catalogue browsing and install-only downloads.
// Ryoku Settings keeps activation, editing, import, export, and publishing.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"ryostore/compat"
)

type riceRegistryEntry struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Author      string   `json:"author,omitempty"`
	Blurb       string   `json:"blurb,omitempty"`
	Tags        []string `json:"tags,omitempty"`
	CreatedWith string   `json:"createdWith,omitempty"`
	Color       string   `json:"color,omitempty"`
	Manifest    string   `json:"manifest,omitempty"`
	Preview     string   `json:"preview,omitempty"`
	Screenshots []string `json:"screenshots,omitempty"`
	Palette     string   `json:"palette,omitempty"`
	Wallpaper   string   `json:"wallpaper,omitempty"`
	Hero        string   `json:"hero,omitempty"`
	Accent      string   `json:"accent,omitempty"`
	Surface     string   `json:"surface,omitempty"`
	Rounding    int      `json:"rounding,omitempty"`
}

type riceRegistry struct {
	Version int                 `json:"version"`
	Rices   []riceRegistryEntry `json:"rices"`
}

type riceManifest struct {
	Schema int    `json:"schema"`
	Slug   string `json:"slug"`
	Color  struct {
		Palette string `json:"palette"`
	} `json:"color"`
	Assets struct {
		Wallpaper string `json:"wallpaper"`
		Hero      string `json:"hero"`
	} `json:"assets"`
}

type riceProvider struct {
	cache          *Cache
	downloadClient *http.Client
	base           string
	ricesDir       string
	activePath     string
	runningVersion func() string
}

func newRiceProvider(cache *Cache) riceProvider {
	ricesDir := filepath.Join(configHome(), "ryoku", "rices")
	return riceProvider{
		cache:          cache,
		downloadClient: &http.Client{Timeout: 5 * time.Minute},
		base:           cache.base,
		ricesDir:       ricesDir,
		activePath:     filepath.Join(ricesDir, ".active"),
		runningVersion: currentRyokuVersion,
	}
}

func (riceProvider) Category() Category {
	return Category{
		ID:          "rices",
		Name:        "Rices",
		Group:       "wear",
		Description: "Complete desktop looks ready to install and refine.",
	}
}

func (p riceProvider) Load(ctx context.Context, refresh bool) ([]Item, SourceState, error) {
	raw, state, err := p.cache.Fetch(ctx, "rices/registry.json", refresh)
	if err != nil {
		return nil, state, err
	}
	var registry riceRegistry
	if err := json.Unmarshal(raw, &registry); err != nil {
		return nil, state, fmt.Errorf("parse rice registry: %w", err)
	}
	active := ""
	if rawActive, err := os.ReadFile(p.activePath); err == nil {
		active = strings.TrimSpace(string(rawActive))
	}
	items := make([]Item, 0, len(registry.Rices))
	for _, entry := range registry.Rices {
		if !validComponent(entry.ID) {
			return nil, state, fmt.Errorf("invalid rice id %q", entry.ID)
		}
		installed := isRegularFile(filepath.Join(p.ricesDir, entry.ID, "rice.json"))
		metadata := map[string]any{}
		if entry.Color != "" {
			metadata["color"] = entry.Color
		}
		if entry.Rounding != 0 {
			metadata["rounding"] = entry.Rounding
		}
		if entry.Accent != "" {
			metadata["accent"] = entry.Accent
		}
		if entry.Surface != "" {
			metadata["surface"] = entry.Surface
		}
		if len(metadata) == 0 {
			metadata = nil
		}
		items = append(items, Item{
			ID:            entry.ID,
			Category:      "rices",
			Name:          entry.Name,
			Summary:       entry.Blurb,
			Description:   entry.Blurb,
			Art:           resolveAsset(p.base, "rices/"+entry.ID, entry.Preview),
			Author:        entry.Author,
			Version:       entry.CreatedWith,
			Compatibility: compat.Rice(entry.CreatedWith, p.runningVersion()),
			Screenshots:   resolveAssets(p.base, "rices/"+entry.ID, entry.Screenshots),
			Tags:          entry.Tags,
			Installed:     installed,
			Active:        installed && entry.ID == active,
			Metadata:      metadata,
		})
	}
	return items, state, nil
}

func (p riceProvider) Install(ctx context.Context, id string) error {
	if !validComponent(id) {
		return fmt.Errorf("bad rice id %q", id)
	}
	raw, _, err := p.cache.Fetch(ctx, "rices/registry.json", false)
	if err != nil {
		return err
	}
	var registry riceRegistry
	if err := json.Unmarshal(raw, &registry); err != nil {
		return fmt.Errorf("parse rice registry: %w", err)
	}
	var entry *riceRegistryEntry
	for i := range registry.Rices {
		if registry.Rices[i].ID == id {
			entry = &registry.Rices[i]
			break
		}
	}
	if entry == nil {
		return fmt.Errorf("rice %q is not in the store", id)
	}
	// A rice's manifest lives at rices/<id>/rice.json. The registry may name it
	// as a bare filename ("rice.json", the published convention) or omit it; both
	// resolve under the rice's own directory. A value that already carries a path
	// is honoured as-is (a repo-root path), so an explicit override still works.
	manifestRel := entry.Manifest
	if manifestRel == "" {
		manifestRel = "rice.json"
	}
	if !strings.Contains(manifestRel, "/") {
		manifestRel = filepath.ToSlash(filepath.Join("rices", id, manifestRel))
	}
	manifestRaw, err := p.fetchSmall(ctx, p.assetURL(manifestRel))
	if err != nil {
		return fmt.Errorf("fetch manifest: %w", err)
	}
	var manifest riceManifest
	if err := json.Unmarshal(manifestRaw, &manifest); err != nil {
		return fmt.Errorf("parse manifest: %w", err)
	}
	if manifest.Slug != id {
		return fmt.Errorf("manifest slug %q does not match rice %q", manifest.Slug, id)
	}

	type asset struct {
		name     string
		remote   string
		local    string
		required bool
	}
	assets := []asset{}
	// The palette is part of a rice's substance: a manifest that names one with
	// no source, or a palette that will not download, fails the install. The
	// wallpaper and hero are display art: a missing file degrades to the
	// apply-time default (the live wallpaper, no hero) rather than refusing the
	// whole rice, so a rice whose art was never published still installs.
	for _, candidate := range []asset{
		{name: "palette", remote: entry.Palette, local: manifest.Color.Palette, required: true},
		{name: "wallpaper", remote: entry.Wallpaper, local: manifest.Assets.Wallpaper},
		{name: "hero", remote: entry.Hero, local: manifest.Assets.Hero},
	} {
		if candidate.local == "" {
			continue
		}
		if !validLocalPath(candidate.local) {
			return fmt.Errorf("invalid %s path %q", candidate.name, candidate.local)
		}
		if candidate.remote == "" {
			if !candidate.required {
				continue
			}
			return fmt.Errorf("manifest requires %s %q but the registry has no source", candidate.name, candidate.local)
		}
		assets = append(assets, candidate)
	}

	dst := filepath.Join(p.ricesDir, id)
	unlock, err := lockTree(dst)
	if err != nil {
		return err
	}
	defer unlock()
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	stage, err := os.MkdirTemp(filepath.Dir(dst), ".ryostore-stage-"+id+"-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(stage)
	for _, asset := range assets {
		if err := p.download(ctx, p.assetURL(asset.remote), filepath.Join(stage, asset.local)); err != nil {
			if !asset.required {
				continue
			}
			return fmt.Errorf("download %s: %w", asset.name, err)
		}
	}
	for _, name := range riceBundledAssets(manifestRaw) {
		remote := path.Join("rices", id, name)
		// Bundled decor/brand art is optional: a fetch failure degrades to the
		// apply-time default rather than failing the whole install.
		if err := p.download(ctx, p.assetURL(remote), filepath.Join(stage, name)); err != nil {
			continue
		}
	}
	if err := atomicWrite(filepath.Join(stage, "rice.json"), manifestRaw, 0o644); err != nil {
		return err
	}
	return replaceTree(stage, dst, nil)
}

var riceAssetRef = regexp.MustCompile(`rice://([A-Za-z0-9._-]+)`)

// riceBundledAssets returns the unique rice://<name> assets a manifest
// references (look.decor[*].src, layers.brand.markImage, etc). Names that would
// escape the rice directory are skipped so a hostile manifest cannot traverse.
func riceBundledAssets(manifest []byte) []string {
	seen := map[string]bool{}
	names := []string{}
	for _, m := range riceAssetRef.FindAllSubmatch(manifest, -1) {
		name := string(m[1])
		if seen[name] || !validLocalPath(name) {
			continue
		}
		seen[name] = true
		names = append(names, name)
	}
	return names
}

func (p riceProvider) assetURL(path string) string {
	if path == "" || strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		return path
	}
	return strings.TrimRight(p.base, "/") + "/" + strings.TrimLeft(path, "/")
}

func (p riceProvider) assetURLs(paths []string) []string {
	if len(paths) == 0 {
		return nil
	}
	out := make([]string, 0, len(paths))
	for _, path := range paths {
		if url := p.assetURL(path); url != "" {
			out = append(out, url)
		}
	}
	return out
}

func (p riceProvider) fetchSmall(ctx context.Context, url string) ([]byte, error) {
	// A local (file://) base serves the tree straight off disk, the same as the
	// catalogue fetch: read the file rather than sending it through the HTTP
	// client, which rejects the file:// scheme.
	if src, ok := localBase(url); ok {
		body, err := os.ReadFile(src)
		if err != nil {
			return nil, err
		}
		if len(body) > maxBody {
			return nil, fmt.Errorf("%s: response exceeds %d bytes", url, maxBody)
		}
		return body, nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := p.downloadClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s: %s", url, resp.Status)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBody+1))
	if err != nil {
		return nil, err
	}
	if len(body) > maxBody {
		return nil, fmt.Errorf("%s: response exceeds %d bytes", url, maxBody)
	}
	return body, nil
}

func (p riceProvider) download(ctx context.Context, url, dst string) error {
	if src, ok := localBase(url); ok {
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return err
		}
		in, err := os.Open(src)
		if err != nil {
			return err
		}
		defer in.Close()
		out, err := os.OpenFile(dst, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
		if err != nil {
			return err
		}
		if _, err := io.Copy(out, in); err != nil {
			out.Close()
			return err
		}
		return out.Close()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := p.downloadClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s: %s", url, resp.Status)
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	file, err := os.OpenFile(dst, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(file, resp.Body)
	closeErr := file.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}

func isRegularFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}
