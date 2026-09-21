// ryostore normalizes six product catalogues (lockscreens, plugins, bundles,
// rices, bar styles, and fastfetch styles) into one JSON contract the Quickshell
// app renders without per-category logic. These types are that contract; their
// field names and JSON tags match docs/store.md and are consumed unchanged by
// every later task.
package main

// Catalog is one probe of every registered provider: the categories with their
// derived counts and the flat item list, plus an offline flag raised whenever a
// source is serving cached data or failed to load.
type Catalog struct {
	GeneratedAt string `json:"generatedAt"`
	Revision    string `json:"revision,omitempty"`
	// WindowManager is the provider this catalogue was built against, so a
	// snapshot records which desktop its per-item availability answers belong
	// to. Empty when no provider could be detected.
	WindowManager string     `json:"windowManager,omitempty"`
	Offline       bool       `json:"offline"`
	Categories    []Category `json:"categories"`
	Items         []Item     `json:"items"`
}

// Category is one product kind. Count and InstalledCount are derived by
// BuildCatalog once its provider settles; Offline, CachedAt, and Error carry the
// source's fetch state so the UI can draw an honest running head.
type Category struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	Group          string `json:"group"`
	Description    string `json:"description"`
	Count          int    `json:"count"`
	InstalledCount int    `json:"installedCount"`
	Offline        bool   `json:"offline,omitempty"`
	CachedAt       string `json:"cachedAt,omitempty"`
	Error          string `json:"error,omitempty"`
}

// Item is one specimen. Its state is explicit: Installed for an owned item,
// Active or Enabled for the refinement a worn rice or running plugin adds,
// InstalledCount/TotalCount for a partial bundle, UpdateAvailable when a newer
// store-managed version exists, and DownloadPaused when the source has paused
// new downloads (the item stays listed and removable). Unavailable is the same
// shape for a different cause: the product names a window manager other than the
// running one, so it is listed and removable but never offered as installable.
// Metadata carries category-specific facts and stays nil for a provider that has
// none.
type Item struct {
	ID                  string `json:"id"`
	Category            string `json:"category"`
	Name                string `json:"name"`
	Summary             string `json:"summary,omitempty"`
	Description         string `json:"description,omitempty"`
	Art                 string `json:"art,omitempty"`
	ArtRaw              string `json:"artRaw,omitempty"`
	Author              string `json:"author,omitempty"`
	Version             string `json:"version,omitempty"`
	Manifest            string `json:"manifest,omitempty"`
	ManifestSHA256      string `json:"manifestSha256,omitempty"`
	InstalledVersion    string `json:"installedVersion,omitempty"`
	Compatibility       string `json:"compatibility,omitempty"`
	DownloadPauseReason string `json:"downloadPauseReason,omitempty"`
	Accent              string `json:"accent,omitempty"`
	Surface             string `json:"surface,omitempty"`
	// Upstream and Discord carry the product's provenance links from the
	// registry entry: the project's home (always present) and an optional
	// community invite. The app renders them as icon actions on the detail page.
	Upstream        string   `json:"upstream,omitempty"`
	Discord         string   `json:"discord,omitempty"`
	Screenshots     []string `json:"screenshots,omitempty"`
	Tags            []string `json:"tags,omitempty"`
	Installed       bool     `json:"installed"`
	Active          bool     `json:"active"`
	Enabled         bool     `json:"enabled"`
	InstalledCount  int      `json:"installedCount"`
	TotalCount      int      `json:"totalCount"`
	UpdateAvailable bool     `json:"updateAvailable"`
	DownloadPaused  bool     `json:"downloadPaused"`
	// RequiredWindowManager is the window manager the product was authored for,
	// empty when it runs on any; Unavailable says it is not the running one and
	// UnavailableReason carries the catalogue's human note, if it wrote one.
	RequiredWindowManager string         `json:"requiredWindowManager,omitempty"`
	Unavailable           bool           `json:"unavailable"`
	UnavailableReason     string         `json:"unavailableReason,omitempty"`
	HasSettings           bool           `json:"hasSettings,omitempty"`
	Metadata              map[string]any `json:"metadata,omitempty"`
}

// SourceState is a provider's fetch outcome: whether it fell back to cached data
// (Offline) and the timestamp of that cache (CachedAt). BuildCatalog folds it
// onto the item's Category, and Cache.Fetch returns it for a single fetch.
type SourceState struct {
	Offline  bool   `json:"offline,omitempty"`
	CachedAt string `json:"cachedAt,omitempty"`
}
