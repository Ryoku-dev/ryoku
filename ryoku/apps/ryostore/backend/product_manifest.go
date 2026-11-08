package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"path"
	"regexp"
	"strings"
)

const (
	maxProductFiles    = 2048
	maxProductFileSize = int64(256 << 20)
	maxProductSize     = int64(512 << 20)
)

var (
	productIDPattern    = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)
	productHashPattern  = regexp.MustCompile(`^[0-9a-f]{64}$`)
	productColorPattern = regexp.MustCompile(`^#[0-9A-Fa-f]{6}$`)
)

type ProductProvides struct {
	Dock           []string `json:"dock,omitempty"`
	Launcher       []string `json:"launcher,omitempty"`
	DesktopWidgets []string `json:"desktopWidgets,omitempty"`
}

type ShellSettings struct {
	IPC       string   `json:"ipc" toml:"ipc"`
	Call      string   `json:"call" toml:"call"`
	Args      []string `json:"args" toml:"args"`
	HubEditor string   `json:"hubEditor" toml:"hubEditor"`
}

type ShellDockContribution struct {
	ID    string `json:"id" toml:"id"`
	Name  string `json:"name" toml:"name"`
	Entry string `json:"entry" toml:"entry"`
}

type ShellLauncherContribution struct {
	ID          string `json:"id" toml:"id"`
	Name        string `json:"name" toml:"name"`
	Description string `json:"description" toml:"description"`
	Entry       string `json:"entry" toml:"entry"`
	Preview     string `json:"preview" toml:"preview"`
}

type ShellDesktopWidgetContribution struct {
	ID            string           `json:"id" toml:"id"`
	Widget        string           `json:"widget" toml:"widget"`
	Options       string           `json:"options,omitempty" toml:"options"`
	Roster        string           `json:"roster" toml:"roster"`
	RosterRecords []map[string]any `json:"rosterRecords" toml:"-"`
}

type ShellProvides struct {
	Dock           []ShellDockContribution          `json:"dock"`
	Launcher       []ShellLauncherContribution      `json:"launcher"`
	DesktopWidgets []ShellDesktopWidgetContribution `json:"desktopWidgets"`
}

type ShellLifecycle struct {
	Activate   string `json:"activate,omitempty" toml:"activate"`
	Deactivate string `json:"deactivate,omitempty" toml:"deactivate"`
	Palette    string `json:"palette,omitempty" toml:"palette"`
}

type ShellRuntime struct {
	Check   string `json:"check,omitempty" toml:"check"`
	Install string `json:"install,omitempty" toml:"install"`
	Start   string `json:"start,omitempty" toml:"start"`
}

type ShellManifest struct {
	Entry     string   `json:"entry"`
	FrameHost string   `json:"frameHost"`
	Owns      []string `json:"owns"`
	// OwnsSetting maps an owned surface to the dotted shell.json key that
	// switches the item's own version of it on or off.
	OwnsSetting map[string]string `json:"ownsSetting"`
	Modules     []string          `json:"modules"`
	Settings    ShellSettings     `json:"settings"`
	Provides    ShellProvides     `json:"provides"`
	Lifecycle   ShellLifecycle    `json:"lifecycle"`
	Runtime     ShellRuntime      `json:"runtime"`
	// Env is the item's [shell.env], kept unexpanded; the daemon expands it.
	Env map[string]string `json:"env"`
}

// shellOwnedSurfaces are the Ryoku surfaces an item may draw itself while active.
var shellOwnedSurfaces = map[string]bool{
	"notifications": true, "notificationSound": true, "osd": true, "polkit": true,
	"dock": true, "widgets": true, "cheatsheet": true, "windowClose": true,
}

func shellCategory(category string) bool {
	return category == "subshells" || category == "barstyles"
}

func strictJSONFields(data []byte, allowed map[string]bool) error {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}
	for name := range fields {
		if !allowed[name] {
			return fmt.Errorf("unknown field %q", name)
		}
	}
	return nil
}

// ProductEntry is the common registry envelope shared by every extras category.
type ProductEntry struct {
	ID                  string   `json:"id"`
	Name                string   `json:"name"`
	Version             string   `json:"version"`
	Path                string   `json:"path"`
	Author              string   `json:"author"`
	Summary             string   `json:"summary"`
	Description         string   `json:"description"`
	Tags                []string `json:"tags"`
	Screenshots         []string `json:"screenshots"`
	Accent              string   `json:"accent"`
	Surface             string   `json:"surface"`
	Preview             string   `json:"preview"`
	PreviewRaw          string   `json:"previewRaw,omitempty"`
	Manifest            string   `json:"manifest"`
	ManifestSHA256      string   `json:"manifestSha256"`
	Official            bool     `json:"official,omitempty"`
	Tagline             string   `json:"tagline,omitempty"`
	Icon                string   `json:"icon,omitempty"`
	Hosts               []string `json:"hosts,omitempty"`
	LastUpdated         string   `json:"lastUpdated,omitempty"`
	DownloadPaused      bool     `json:"downloadPaused,omitempty"`
	DownloadPauseReason string   `json:"downloadPauseReason,omitempty"`
	// WindowManager names the window manager the product is written for, using
	// the provider name `ryoku wm use <name>` takes. Empty means any.
	WindowManager       string `json:"windowManager,omitempty"`
	WindowManagerReason string `json:"windowManagerReason,omitempty"`
	// PackageManagers limits the product to the editions whose package manager
	// `ryoku-host pkgmgr` names (pacman, dnf, xbps). Empty means every edition.
	PackageManagers       []string `json:"packageManagers,omitempty"`
	PackageManagersReason string   `json:"packageManagersReason,omitempty"`
	// Upstream is the https:// home of the project a product comes from
	// (a registry entry always names one); Discord is an optional invite to
	// that project's community. Both surface as links on the detail page.
	Upstream string `json:"upstream"`
	Discord  string `json:"discord,omitempty"`
	// Provides lists the contribution ids a subshell or bar style offers;
	// RequiresQmlModules is accepted from the registry and not used here.
	Provides           *ProductProvides `json:"provides,omitempty"`
	RequiresQmlModules []string         `json:"requiresQmlModules,omitempty"`
	// optional, measured against Orpheus by the catalogue
	Metadata *ProductMetadata `json:"metadata,omitempty"`
}

func (entry *ProductEntry) UnmarshalJSON(data []byte) error {
	allowed := map[string]bool{
		"id": true, "name": true, "version": true, "path": true, "author": true,
		"summary": true, "description": true, "tags": true, "screenshots": true,
		"accent": true, "surface": true, "preview": true, "previewRaw": true,
		"manifest": true, "manifestSha256": true, "official": true, "tagline": true,
		"icon": true, "hosts": true, "lastUpdated": true, "downloadPaused": true,
		"downloadPauseReason": true, "windowManager": true, "windowManagerReason": true,
		"packageManagers": true, "packageManagersReason": true, "upstream": true,
		"discord": true, "provides": true, "requiresQmlModules": true, "metadata": true,
	}
	if err := strictJSONFields(data, allowed); err != nil {
		return err
	}
	type plain ProductEntry
	var decoded plain
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*entry = ProductEntry(decoded)
	return nil
}

// ProductFile is one manifest-owned source and its installed destination.
type ProductFile struct {
	Source      string `json:"source"`
	Destination string `json:"destination"`
	SHA256      string `json:"sha256"`
	Mode        string `json:"mode"`
	Size        int64  `json:"size"`
	Install     bool   `json:"install"`
}

// ProductManifest is the complete, versioned file contract for one product.
type ProductManifest struct {
	Schema      int            `json:"schema"`
	ID          string         `json:"id"`
	Category    string         `json:"category"`
	Version     string         `json:"version"`
	Destination string         `json:"destination"`
	Files       []ProductFile  `json:"files"`
	Shell       *ShellManifest `json:"shell,omitempty"`
}

func (manifest *ProductManifest) UnmarshalJSON(data []byte) error {
	if err := strictJSONFields(data, map[string]bool{
		"schema": true, "id": true, "category": true, "version": true,
		"destination": true, "files": true, "shell": true,
	}); err != nil {
		return err
	}
	type plain ProductManifest
	var decoded plain
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*manifest = ProductManifest(decoded)
	return nil
}

func (file *ProductFile) UnmarshalJSON(data []byte) error {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}
	allowed := map[string]bool{
		"source": true, "destination": true, "sha256": true,
		"mode": true, "size": true, "install": true,
	}
	for name := range fields {
		if !allowed[name] {
			return fmt.Errorf("unknown field %q", name)
		}
	}
	for name := range allowed {
		value, ok := fields[name]
		if !ok {
			return fmt.Errorf("missing field %q", name)
		}
		if bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			return fmt.Errorf("field %q must not be null", name)
		}
	}
	type plainProductFile ProductFile
	var decoded plainProductFile
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*file = ProductFile(decoded)
	return nil
}

func loadProductManifest(ctx context.Context, cache *Cache, category string, entry ProductEntry) (ProductManifest, error) {
	return loadProductManifestFrom(ctx, cache, category, entry, true)
}

// loadProductManifestFrom reads the manifest live when refresh is set, else it
// may answer from the process memo; the registry hash check applies either way.
func loadProductManifestFrom(ctx context.Context, cache *Cache, category string, entry ProductEntry, refresh bool) (ProductManifest, error) {
	if cache == nil {
		return ProductManifest{}, fmt.Errorf("%s/%s: manifest cache is nil", category, entry.ID)
	}
	if err := validateProductEntry(category, entry); err != nil {
		return ProductManifest{}, err
	}

	rel := path.Join(entry.Path, entry.Manifest)
	raw, _, err := cache.Fetch(ctx, rel, refresh)
	if err != nil {
		return ProductManifest{}, fmt.Errorf("%s/%s: fetch manifest: %w", category, entry.ID, err)
	}
	actualHash := fmt.Sprintf("%x", sha256.Sum256(raw))
	if actualHash != entry.ManifestSHA256 {
		return ProductManifest{}, fmt.Errorf("%s/%s: manifest hash mismatch", category, entry.ID)
	}

	var manifest ProductManifest
	decoder := json.NewDecoder(bytes.NewReader(raw))
	if err := decoder.Decode(&manifest); err != nil {
		return ProductManifest{}, fmt.Errorf("%s/%s: decode manifest: %w", category, entry.ID, err)
	}
	var trailing json.RawMessage
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			err = fmt.Errorf("multiple JSON values")
		}
		return ProductManifest{}, fmt.Errorf("%s/%s: decode manifest: %w", category, entry.ID, err)
	}
	if err := validateProductManifest(category, entry, manifest); err != nil {
		return ProductManifest{}, err
	}
	return manifest, nil
}

func validateProductEntry(category string, entry ProductEntry) error {
	label := category + "/" + entry.ID
	if !validProductCategory(category) {
		return fmt.Errorf("%s: invalid category", label)
	}
	if !productIDPattern.MatchString(entry.ID) {
		return fmt.Errorf("%s: invalid product id", label)
	}
	for name, value := range map[string]string{
		"name": entry.Name, "version": entry.Version, "author": entry.Author,
		"summary": entry.Summary, "description": entry.Description,
	} {
		if value == "" {
			return fmt.Errorf("%s: missing %s", label, name)
		}
	}
	for name, value := range map[string]string{
		"path": entry.Path, "preview": entry.Preview, "manifest": entry.Manifest,
	} {
		if !validProductPath(value) {
			return fmt.Errorf("%s: invalid %s %q", label, name, value)
		}
	}
	if entry.Tags == nil {
		return fmt.Errorf("%s: missing tags", label)
	}
	if entry.Screenshots == nil {
		return fmt.Errorf("%s: missing screenshots", label)
	}
	for name, value := range map[string]string{
		"accent": entry.Accent, "surface": entry.Surface,
	} {
		if value != "" && !productColorPattern.MatchString(value) {
			return fmt.Errorf("%s: invalid %s %q", label, name, value)
		}
		if value == "" && !shellCategory(category) {
			return fmt.Errorf("%s: invalid %s %q", label, name, value)
		}
	}
	for _, screenshot := range entry.Screenshots {
		if !validProductPath(screenshot) {
			return fmt.Errorf("%s: invalid screenshot %q", label, screenshot)
		}
	}
	for _, tag := range entry.Tags {
		if tag == "" {
			return fmt.Errorf("%s: empty tag", label)
		}
	}
	if entry.Provides != nil {
		for kind, ids := range map[string][]string{
			"dock": entry.Provides.Dock, "launcher": entry.Provides.Launcher,
			"desktopWidgets": entry.Provides.DesktopWidgets,
		} {
			seen := map[string]bool{}
			for _, id := range ids {
				if !productIDPattern.MatchString(id) || seen[id] {
					return fmt.Errorf("%s: invalid %s contribution %q", label, kind, id)
				}
				seen[id] = true
			}
		}
	}
	for _, module := range entry.RequiresQmlModules {
		if module == "" || strings.ContainsAny(module, "/\\\x00") {
			return fmt.Errorf("%s: invalid required QML module %q", label, module)
		}
	}
	if err := validateProductMetadata(category, entry.Metadata); err != nil {
		return fmt.Errorf("%s: %w", label, err)
	}
	if !productHashPattern.MatchString(entry.ManifestSHA256) {
		return fmt.Errorf("%s: invalid manifest SHA-256", label)
	}
	return nil
}

func validateProductManifest(category string, entry ProductEntry, manifest ProductManifest) error {
	label := category + "/" + entry.ID
	if !validProductCategory(category) {
		return fmt.Errorf("%s: invalid category", label)
	}
	if !productIDPattern.MatchString(entry.ID) || entry.Version == "" {
		return fmt.Errorf("%s: invalid registry identity", label)
	}
	if manifest.Schema != 1 {
		return fmt.Errorf("%s: unsupported manifest schema %d", label, manifest.Schema)
	}
	if manifest.ID != entry.ID {
		return fmt.Errorf("%s: manifest id is %q", label, manifest.ID)
	}
	if manifest.Category != category {
		return fmt.Errorf("%s: manifest category is %q", label, manifest.Category)
	}
	if manifest.Version != entry.Version {
		return fmt.Errorf("%s: manifest version is %q", label, manifest.Version)
	}
	if !validProductPath(manifest.Destination) {
		return fmt.Errorf("%s: invalid manifest destination %q", label, manifest.Destination)
	}
	if len(manifest.Files) == 0 || len(manifest.Files) > maxProductFiles {
		return fmt.Errorf("%s: manifest has %d files", label, len(manifest.Files))
	}
	if shellCategory(category) {
		if manifest.Shell == nil {
			return fmt.Errorf("%s: manifest has no shell block", label)
		}
		if err := validateShellManifest(label, *manifest.Shell); err != nil {
			return err
		}
	} else if manifest.Shell != nil {
		return fmt.Errorf("%s: non-shell manifest has a shell block", label)
	}

	sources := make(map[string]struct{}, len(manifest.Files))
	destinations := make(map[string]struct{}, len(manifest.Files))
	var total int64
	for index, file := range manifest.Files {
		fileLabel := fmt.Sprintf("%s: files[%d]", label, index)
		if !validProductPath(file.Source) {
			return fmt.Errorf("%s: invalid source %q", fileLabel, file.Source)
		}
		if !validProductPath(file.Destination) {
			return fmt.Errorf("%s: invalid destination %q", fileLabel, file.Destination)
		}
		if _, exists := sources[file.Source]; exists {
			return fmt.Errorf("%s: duplicate source %q", fileLabel, file.Source)
		}
		for existing := range destinations {
			if existing == file.Destination {
				return fmt.Errorf("%s: duplicate destination %q", fileLabel, file.Destination)
			}
			if strings.HasPrefix(existing, file.Destination+"/") || strings.HasPrefix(file.Destination, existing+"/") {
				return fmt.Errorf("%s: destination collision %q", fileLabel, file.Destination)
			}
		}
		if file.Mode != "0644" && file.Mode != "0755" {
			return fmt.Errorf("%s: invalid mode %q", fileLabel, file.Mode)
		}
		if file.Size < 0 || file.Size > maxProductFileSize {
			return fmt.Errorf("%s: invalid size %d", fileLabel, file.Size)
		}
		if !productHashPattern.MatchString(file.SHA256) {
			return fmt.Errorf("%s: invalid SHA-256", fileLabel)
		}
		if total > maxProductSize-file.Size {
			return fmt.Errorf("%s: product exceeds %d bytes", label, maxProductSize)
		}
		total += file.Size
		sources[file.Source] = struct{}{}
		destinations[file.Destination] = struct{}{}
	}
	if category == vesktopThemesCategory {
		_, err := vesktopThemeManifestFile(manifest.Files)
		return err
	}
	return nil
}

func validateShellManifest(label string, shell ShellManifest) error {
	if !validProductPath(shell.Entry) {
		return fmt.Errorf("%s: invalid shell entry %q", label, shell.Entry)
	}
	if shell.FrameHost != "each" && shell.FrameHost != "primary" {
		return fmt.Errorf("%s: invalid shell frame host %q", label, shell.FrameHost)
	}
	owned := map[string]bool{}
	for _, surface := range shell.Owns {
		if !shellOwnedSurfaces[surface] || owned[surface] {
			return fmt.Errorf("%s: invalid owned surface %q", label, surface)
		}
		owned[surface] = true
	}
	for surface, key := range shell.OwnsSetting {
		if !owned[surface] || key == "" || strings.ContainsAny(key, " \x00") {
			return fmt.Errorf("%s: invalid owns setting %q", label, surface)
		}
	}
	for _, module := range shell.Modules {
		if !validProductPath(module) || strings.Contains(module, "/") {
			return fmt.Errorf("%s: invalid shell module %q", label, module)
		}
	}
	for _, value := range append(append([]string{}, shell.Settings.Args...), shell.Settings.IPC, shell.Settings.Call, shell.Settings.HubEditor) {
		if strings.ContainsRune(value, '\x00') {
			return fmt.Errorf("%s: invalid shell setting", label)
		}
	}
	seen := map[string]bool{}
	for _, dock := range shell.Provides.Dock {
		if !productIDPattern.MatchString(dock.ID) || dock.Name == "" || !validProductPath(dock.Entry) || seen["dock/"+dock.ID] {
			return fmt.Errorf("%s: invalid dock contribution %q", label, dock.ID)
		}
		seen["dock/"+dock.ID] = true
	}
	for _, launcher := range shell.Provides.Launcher {
		if !productIDPattern.MatchString(launcher.ID) || launcher.Name == "" || !validProductPath(launcher.Entry) || seen["launcher/"+launcher.ID] {
			return fmt.Errorf("%s: invalid launcher contribution %q", label, launcher.ID)
		}
		if launcher.Preview != "" && !validProductPath(launcher.Preview) {
			return fmt.Errorf("%s: invalid launcher preview %q", label, launcher.Preview)
		}
		seen["launcher/"+launcher.ID] = true
	}
	for _, widgets := range shell.Provides.DesktopWidgets {
		if !productIDPattern.MatchString(widgets.ID) || !validProductPath(widgets.Widget) || seen["desktopWidgets/"+widgets.ID] {
			return fmt.Errorf("%s: invalid desktop widget contribution %q", label, widgets.ID)
		}
		if widgets.Options != "" && !validProductPath(widgets.Options) {
			return fmt.Errorf("%s: invalid desktop widget options %q", label, widgets.Options)
		}
		if widgets.Roster != "" && !validProductPath(widgets.Roster) {
			return fmt.Errorf("%s: invalid desktop widget roster %q", label, widgets.Roster)
		}
		seen["desktopWidgets/"+widgets.ID] = true
	}
	for name, command := range map[string]string{
		"activate": shell.Lifecycle.Activate, "deactivate": shell.Lifecycle.Deactivate,
		"palette": shell.Lifecycle.Palette, "check": shell.Runtime.Check,
		"install": shell.Runtime.Install, "start": shell.Runtime.Start,
	} {
		if command != "" && !validProductPath(command) {
			return fmt.Errorf("%s: invalid %s command %q", label, name, command)
		}
	}
	return nil
}

func productUpdateAvailable(installedVersion, availableVersion string) bool {
	return installedVersion != "" && availableVersion != "" && installedVersion != availableVersion
}

func validProductCategory(category string) bool {
	switch category {
	case "rices", "lockscreens", "subshells", "barstyles", "launchers", "docks", "osds", "notifications", "sidebars", "fastfetch",
		"plugins", "bundles", "profiles", "decors", "launcher-images", "fastfetch-emblems", "ryotunes-skins", vesktopThemesCategory:
		return true
	default:
		return false
	}
}

func validProductPath(value string) bool {
	if value == "" || value == "." || strings.ContainsAny(value, "\\\x00") || strings.HasPrefix(value, "/") || path.Clean(value) != value {
		return false
	}
	for _, component := range strings.Split(value, "/") {
		if component == "" || component == "." || component == ".." || strings.HasPrefix(component, ".ryostore-") {
			return false
		}
	}
	return true
}
