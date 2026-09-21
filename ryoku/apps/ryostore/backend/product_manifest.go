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
	// Upstream is the https:// home of the project a product comes from
	// (a registry entry always names one); Discord is an optional invite to
	// that project's community. Both surface as links on the detail page.
	Upstream string `json:"upstream"`
	Discord  string `json:"discord,omitempty"`
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
	Schema      int           `json:"schema"`
	ID          string        `json:"id"`
	Category    string        `json:"category"`
	Version     string        `json:"version"`
	Destination string        `json:"destination"`
	Files       []ProductFile `json:"files"`
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
	if cache == nil {
		return ProductManifest{}, fmt.Errorf("%s/%s: manifest cache is nil", category, entry.ID)
	}
	if err := validateProductEntry(category, entry); err != nil {
		return ProductManifest{}, err
	}

	rel := path.Join(entry.Path, entry.Manifest)
	raw, _, err := cache.Fetch(ctx, rel, true)
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
		if !productColorPattern.MatchString(value) {
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
	return nil
}

func productUpdateAvailable(installedVersion, availableVersion string) bool {
	return installedVersion != "" && availableVersion != "" && installedVersion != availableVersion
}

func validProductCategory(category string) bool {
	switch category {
	case "rices", "lockscreens", "barstyles", "fastfetch", "plugins", "bundles", "decors",
		"launcher-images", "fastfetch-emblems", "ryotunes-skins":
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
