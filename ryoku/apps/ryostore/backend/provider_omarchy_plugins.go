package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	omarchyPluginCategoryID = "omarchy-plugins"
	omarchyPluginCatalogURL = "https://plugins.omarchy.org/catalog.json"
	omarchyPluginSiteURL    = "https://plugins.omarchy.org/"
	omarchyPluginCacheTTL   = 30 * time.Minute
	omarchyPluginMaxBody    = 32 << 20
)

type omarchyPluginProvider struct {
	client     *http.Client
	catalogURL string
	cacheDir   string
	configHome string
	now        func() time.Time
	run        func(context.Context, string, ...string) ([]byte, error)
}

type omarchyPluginCatalogue struct {
	GeneratedAt string                       `json:"generatedAt"`
	Plugins     []omarchyPluginCatalogueItem `json:"plugins"`
}

type omarchyPluginCatalogueItem struct {
	ID                 string   `json:"id"`
	Name               string   `json:"name"`
	Description        string   `json:"description"`
	Author             string   `json:"author"`
	Version            string   `json:"version"`
	Category           string   `json:"category"`
	Tags               []string `json:"tags"`
	Kind               string   `json:"kind"`
	Initials           string   `json:"initials"`
	Accent             string   `json:"accent"`
	Repo               string   `json:"repo"`
	InstallAvailable   bool     `json:"installAvailable"`
	License            string   `json:"license"`
	Stars              int      `json:"stars"`
	PreviewImage       string   `json:"previewImage"`
	PreviewThumbnail   string   `json:"previewThumbnail"`
	VerificationStatus string   `json:"verificationStatus"`
	ListedAt           string   `json:"listedAt"`
}

type omarchyPluginCacheMetadata struct {
	ETag         string    `json:"etag,omitempty"`
	LastModified string    `json:"lastModified,omitempty"`
	FetchedAt    time.Time `json:"fetchedAt"`
}

type installedOmarchyPlugin struct {
	Version string
	Dir     string
}

type omarchyPluginListRow struct {
	ID      string `json:"id"`
	Enabled bool   `json:"enabled"`
}

type omarchyCompatResult struct {
	OK       bool     `json:"ok"`
	Requires []string `json:"requires"`
	Reason   string   `json:"reason"`
}

func newOmarchyPluginProvider() *omarchyPluginProvider {
	return &omarchyPluginProvider{
		client:     &http.Client{Timeout: 45 * time.Second},
		catalogURL: omarchyPluginCatalogURL,
		cacheDir:   filepath.Join(extrasCacheDir(), omarchyPluginCategoryID),
		configHome: configHome(),
		now:        time.Now,
		run:        runOmarchyPluginCommand,
	}
}

func (*omarchyPluginProvider) Category() Category {
	return Category{
		ID:          omarchyPluginCategoryID,
		Name:        "Omarchy plugins",
		Group:       "EXTEND",
		Description: "Marketplace plugins for the Nomarchy shell.",
	}
}

func (p *omarchyPluginProvider) Load(ctx context.Context, refresh bool) ([]Item, SourceState, error) {
	if !p.nomarchyActive() {
		return nil, SourceState{}, nil
	}
	catalogue, state, err := p.loadCatalogue(ctx, refresh)
	if err != nil {
		return nil, state, err
	}
	installed := readInstalledOmarchyPlugins(filepath.Join(p.configHome, "omarchy", "plugins"))
	enabled := p.readEnabled(ctx)
	items := make([]Item, 0, len(catalogue.Plugins))
	for _, entry := range catalogue.Plugins {
		if !entry.InstallAvailable || !validOmarchyPluginID(entry.ID) || !safeOmarchyRepo(entry.Repo) {
			continue
		}
		item := normalizeOmarchyPlugin(entry)
		if local, ok := installed[entry.ID]; ok {
			item.Installed = true
			item.InstalledVersion = local.Version
			item.Enabled = enabled[entry.ID]
			item.UpdateAvailable = local.Version != "" && entry.Version != "" && local.Version != entry.Version
			p.applyCompatibility(ctx, &item, local.Dir)
		}
		items = append(items, item)
	}
	return sortOmarchyPluginItems(items, "popular"), state, nil
}

func (p *omarchyPluginProvider) Install(ctx context.Context, id string) error {
	if !p.nomarchyActive() {
		return fmt.Errorf("Omarchy plugins are available only while Nomarchy is active")
	}
	entry, err := p.catalogueEntry(ctx, id)
	if err != nil {
		return err
	}
	installed := readInstalledOmarchyPlugins(filepath.Join(p.configHome, "omarchy", "plugins"))
	if _, ok := installed[id]; ok {
		return p.runChecked(ctx, "omarchy", "plugin", "update", id, "--yes")
	}
	if !safeOmarchyRepo(entry.Repo) {
		return fmt.Errorf("plugin %q has an invalid repository URL", id)
	}
	return p.runChecked(ctx, "omarchy", "plugin", "add", entry.Repo, "--yes")
}

func (p *omarchyPluginProvider) Remove(ctx context.Context, id string) error {
	if !validOmarchyPluginID(id) {
		return fmt.Errorf("invalid plugin id %q", id)
	}
	if !p.nomarchyActive() {
		return fmt.Errorf("Omarchy plugins are available only while Nomarchy is active")
	}
	return p.runChecked(ctx, "omarchy", "plugin", "remove", id, "--yes")
}

func (p *omarchyPluginProvider) runChecked(ctx context.Context, name string, args ...string) error {
	out, err := p.run(ctx, name, args...)
	if err == nil {
		return nil
	}
	message := strings.TrimSpace(string(out))
	if message == "" {
		return err
	}
	return errors.New(message)
}

func (p *omarchyPluginProvider) catalogueEntry(ctx context.Context, id string) (omarchyPluginCatalogueItem, error) {
	if !validOmarchyPluginID(id) {
		return omarchyPluginCatalogueItem{}, fmt.Errorf("invalid plugin id %q", id)
	}
	catalogue, _, err := p.loadCatalogue(ctx, false)
	if err != nil {
		return omarchyPluginCatalogueItem{}, err
	}
	for _, entry := range catalogue.Plugins {
		if entry.ID == id && entry.InstallAvailable {
			return entry, nil
		}
	}
	return omarchyPluginCatalogueItem{}, fmt.Errorf("plugin %q is not installable", id)
}

func (p *omarchyPluginProvider) nomarchyActive() bool {
	raw, err := os.ReadFile(filepath.Join(p.configHome, "ryoku", "shell.json"))
	if err != nil {
		return false
	}
	var settings struct {
		BarStyle string `json:"barStyle"`
	}
	return json.Unmarshal(raw, &settings) == nil && settings.BarStyle == "nomarchy"
}

func (p *omarchyPluginProvider) cachePaths() (string, string) {
	return filepath.Join(p.cacheDir, "catalog.json"), filepath.Join(p.cacheDir, "http.json")
}

func (p *omarchyPluginProvider) loadCatalogue(ctx context.Context, refresh bool) (omarchyPluginCatalogue, SourceState, error) {
	catalogPath, metadataPath := p.cachePaths()
	cached, cachedAt, cacheErr := readOmarchyCatalogueCache(catalogPath)
	metadata := readOmarchyPluginCacheMetadata(metadataPath)
	if !refresh && cacheErr == nil && !metadata.FetchedAt.IsZero() && p.now().Sub(metadata.FetchedAt) < omarchyPluginCacheTTL {
		return cached, SourceState{}, nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.catalogURL, nil)
	if err != nil {
		return omarchyPluginCatalogue{}, SourceState{}, err
	}
	if metadata.ETag != "" {
		req.Header.Set("If-None-Match", metadata.ETag)
	}
	if metadata.LastModified != "" {
		req.Header.Set("If-Modified-Since", metadata.LastModified)
	}
	res, err := p.client.Do(req)
	if err != nil {
		return cachedOmarchyFallback(cached, cachedAt, cacheErr, err)
	}
	defer res.Body.Close()
	if res.StatusCode == http.StatusNotModified {
		if cacheErr != nil {
			return omarchyPluginCatalogue{}, SourceState{}, fmt.Errorf("marketplace returned not modified without a cached catalogue")
		}
		metadata.FetchedAt = p.now().UTC()
		writeOmarchyPluginCacheMetadata(metadataPath, metadata)
		return cached, SourceState{}, nil
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return cachedOmarchyFallback(cached, cachedAt, cacheErr, fmt.Errorf("marketplace catalogue: HTTP %d", res.StatusCode))
	}
	if res.ContentLength > omarchyPluginMaxBody {
		return cachedOmarchyFallback(cached, cachedAt, cacheErr, fmt.Errorf("marketplace catalogue exceeds %d bytes", omarchyPluginMaxBody))
	}
	raw, err := io.ReadAll(io.LimitReader(res.Body, omarchyPluginMaxBody+1))
	if err != nil {
		return cachedOmarchyFallback(cached, cachedAt, cacheErr, err)
	}
	if len(raw) > omarchyPluginMaxBody {
		return cachedOmarchyFallback(cached, cachedAt, cacheErr, fmt.Errorf("marketplace catalogue exceeds %d bytes", omarchyPluginMaxBody))
	}
	catalogue, err := decodeOmarchyPluginCatalogue(raw)
	if err != nil {
		return cachedOmarchyFallback(cached, cachedAt, cacheErr, err)
	}
	if err := atomicWrite(catalogPath, raw, 0o644); err != nil {
		return omarchyPluginCatalogue{}, SourceState{}, fmt.Errorf("cache marketplace catalogue: %w", err)
	}
	writeOmarchyPluginCacheMetadata(metadataPath, omarchyPluginCacheMetadata{
		ETag:         res.Header.Get("ETag"),
		LastModified: res.Header.Get("Last-Modified"),
		FetchedAt:    p.now().UTC(),
	})
	return catalogue, SourceState{}, nil
}

func cachedOmarchyFallback(cached omarchyPluginCatalogue, cachedAt string, cacheErr, liveErr error) (omarchyPluginCatalogue, SourceState, error) {
	if cacheErr == nil {
		return cached, SourceState{Offline: true, CachedAt: cachedAt}, nil
	}
	return omarchyPluginCatalogue{}, SourceState{}, liveErr
}

func readOmarchyCatalogueCache(path string) (omarchyPluginCatalogue, string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return omarchyPluginCatalogue{}, "", err
	}
	catalogue, err := decodeOmarchyPluginCatalogue(raw)
	if err != nil {
		return omarchyPluginCatalogue{}, "", err
	}
	cachedAt := ""
	if info, statErr := os.Stat(path); statErr == nil {
		cachedAt = info.ModTime().UTC().Format(time.RFC3339)
	}
	return catalogue, cachedAt, nil
}

func decodeOmarchyPluginCatalogue(raw []byte) (omarchyPluginCatalogue, error) {
	var catalogue omarchyPluginCatalogue
	if err := decodeOneJSON(raw, &catalogue); err != nil {
		return omarchyPluginCatalogue{}, fmt.Errorf("decode marketplace catalogue: %w", err)
	}
	if catalogue.Plugins == nil {
		return omarchyPluginCatalogue{}, fmt.Errorf("decode marketplace catalogue: plugins array is missing")
	}
	return catalogue, nil
}

func readOmarchyPluginCacheMetadata(path string) omarchyPluginCacheMetadata {
	raw, err := os.ReadFile(path)
	if err != nil {
		return omarchyPluginCacheMetadata{}
	}
	var metadata omarchyPluginCacheMetadata
	if decodeOneJSON(raw, &metadata) != nil {
		return omarchyPluginCacheMetadata{}
	}
	return metadata
}

func writeOmarchyPluginCacheMetadata(path string, metadata omarchyPluginCacheMetadata) {
	raw, err := json.Marshal(metadata)
	if err == nil {
		_ = atomicWrite(path, append(raw, '\n'), 0o644)
	}
}

func normalizeOmarchyPlugin(entry omarchyPluginCatalogueItem) Item {
	art := resolveOmarchyMarketplaceAsset(entry.PreviewThumbnail)
	if art == "" {
		art = resolveOmarchyMarketplaceAsset(entry.PreviewImage)
	}
	var screenshots []string
	if preview := resolveOmarchyMarketplaceAsset(entry.PreviewImage); preview != "" {
		screenshots = []string{preview}
	}
	metadata := map[string]any{}
	if entry.Category != "" {
		metadata["marketplaceCategory"] = entry.Category
	}
	if len(metadata) == 0 {
		metadata = nil
	}
	return Item{
		ID:                 entry.ID,
		Category:           omarchyPluginCategoryID,
		Name:               entry.Name,
		Summary:            entry.Kind,
		Description:        entry.Description,
		Art:                art,
		Initials:           entry.Initials,
		Accent:             entry.Accent,
		Author:             entry.Author,
		Version:            entry.Version,
		Compatibility:      "Checked before enabling",
		Upstream:           entry.Repo,
		Screenshots:        screenshots,
		Tags:               append([]string(nil), entry.Tags...),
		Kind:               entry.Kind,
		License:            entry.License,
		Stars:              entry.Stars,
		VerificationStatus: entry.VerificationStatus,
		ListedAt:           entry.ListedAt,
		Metadata:           metadata,
	}
}

func resolveOmarchyMarketplaceAsset(asset string) string {
	asset = strings.TrimSpace(asset)
	if asset == "" {
		return ""
	}
	ref, err := url.Parse(asset)
	if err != nil {
		return ""
	}
	base, _ := url.Parse(omarchyPluginSiteURL)
	return base.ResolveReference(ref).String()
}

func readInstalledOmarchyPlugins(root string) map[string]installedOmarchyPlugin {
	installed := map[string]installedOmarchyPlugin{}
	entries, err := os.ReadDir(root)
	if err != nil {
		return installed
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		dir := filepath.Join(root, entry.Name())
		raw, err := os.ReadFile(filepath.Join(dir, "manifest.json"))
		if err != nil {
			continue
		}
		var manifest struct {
			ID      string `json:"id"`
			Version string `json:"version"`
		}
		if decodeOneJSON(raw, &manifest) != nil || !validOmarchyPluginID(manifest.ID) {
			continue
		}
		installed[manifest.ID] = installedOmarchyPlugin{Version: manifest.Version, Dir: dir}
	}
	return installed
}

func (p *omarchyPluginProvider) readEnabled(ctx context.Context) map[string]bool {
	enabled := map[string]bool{}
	out, err := p.run(ctx, "omarchy", "plugin", "list", "--json")
	if err != nil {
		return enabled
	}
	var rows []omarchyPluginListRow
	if decodeOneJSON(out, &rows) != nil {
		return enabled
	}
	for _, row := range rows {
		if validOmarchyPluginID(row.ID) {
			enabled[row.ID] = row.Enabled
		}
	}
	return enabled
}

func (p *omarchyPluginProvider) applyCompatibility(ctx context.Context, item *Item, dir string) {
	out, err := p.run(ctx, "ryoku", "wm", "compat", dir)
	if err != nil {
		return
	}
	var result omarchyCompatResult
	if decodeOneJSON(out, &result) != nil {
		return
	}
	if item.Metadata == nil {
		item.Metadata = map[string]any{}
	}
	if len(result.Requires) > 0 {
		item.Metadata["requires"] = result.Requires
	}
	if result.OK {
		item.Compatibility = "Compatible"
		return
	}
	item.Compatibility = "Unavailable"
	item.Unavailable = true
	item.UnavailableReason = strings.TrimSpace(result.Reason)
	if item.UnavailableReason == "" {
		item.UnavailableReason = "This plugin cannot use the active window manager APIs."
	}
}

func validOmarchyPluginID(id string) bool {
	if id == "" || strings.Contains(id, "..") {
		return false
	}
	for i, r := range id {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || (i > 0 && (r == '.' || r == '_' || r == '-')) {
			continue
		}
		return false
	}
	return true
}

func safeOmarchyRepo(raw string) bool {
	u, err := url.Parse(strings.TrimSpace(raw))
	return err == nil && u.Scheme == "https" && u.Host != "" && u.User == nil
}

func runOmarchyPluginCommand(ctx context.Context, name string, args ...string) ([]byte, error) {
	return exec.CommandContext(ctx, name, args...).CombinedOutput()
}

func sortOmarchyPluginItems(items []Item, mode string) []Item {
	out := append([]Item(nil), items...)
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		switch mode {
		case "new":
			if a.ListedAt != b.ListedAt {
				return a.ListedAt > b.ListedAt
			}
		case "verified":
			av := strings.EqualFold(a.VerificationStatus, "verified")
			bv := strings.EqualFold(b.VerificationStatus, "verified")
			if av != bv {
				return av
			}
		}
		if a.Stars != b.Stars {
			return a.Stars > b.Stars
		}
		return strings.ToLower(a.Name) < strings.ToLower(b.Name)
	})
	return out
}
