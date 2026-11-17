package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// sourceEntry is one row of `ryostore sources list --json`. Status is ok or
// error for a store that was read; disabled for a store the user turned off;
// removed for a store the user removed that still owns installs.
type sourceEntry struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	URL         string   `json:"url"`
	Official    bool     `json:"official"`
	Enabled     bool     `json:"enabled"`
	Status      string   `json:"status"`
	Error       string   `json:"error,omitempty"`
	Description string   `json:"description,omitempty"`
	Homepage    string   `json:"homepage,omitempty"`
	Added       string   `json:"added,omitempty"`
	Items       *int     `json:"items,omitempty"`
	Categories  []string `json:"categories,omitempty"`
	Installed   int      `json:"installed"`
	Note        string   `json:"note,omitempty"`
}

const sourceProbeTimeout = 15 * time.Second

func runSources(w io.Writer, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("sources needs list, add, remove, enable or disable")
	}
	ctx, cancel := context.WithTimeout(context.Background(), sourceProbeTimeout)
	defer cancel()
	switch args[0] {
	case "list":
		if len(args) > 2 || (len(args) == 2 && args[1] != "--json") {
			return fmt.Errorf("sources list takes only --json")
		}
		return encodeSourcesJSON(w, struct {
			Schema  int           `json:"schema"`
			Sources []sourceEntry `json:"sources"`
		}{1, listSources(ctx)})
	case "add":
		url, name := "", ""
		for i := 1; i < len(args); i++ {
			switch {
			case args[i] == "--name" && i+1 < len(args):
				name = args[i+1]
				i++
			case url == "" && !strings.HasPrefix(args[i], "--"):
				url = args[i]
			default:
				return fmt.Errorf("sources add needs <url> [--name N]")
			}
		}
		entry, err := addSource(ctx, url, name, time.Now())
		if err != nil {
			return err
		}
		return encodeSourcesJSON(w, entry)
	case "remove":
		if len(args) != 2 {
			return fmt.Errorf("sources remove needs <id>")
		}
		installed, err := removeSource(args[1])
		if err != nil {
			return err
		}
		return encodeSourcesJSON(w, struct {
			ID        string `json:"id"`
			Removed   bool   `json:"removed"`
			Installed int    `json:"installed"`
			Note      string `json:"note,omitempty"`
		}{args[1], true, installed, removedNote(installed)})
	case "enable", "disable":
		if len(args) != 2 {
			return fmt.Errorf("sources %s needs <id>", args[0])
		}
		source, err := setSourceEnabled(args[1], args[0] == "enable")
		if err != nil {
			return err
		}
		return encodeSourcesJSON(w, describeUserSource(ctx, source))
	default:
		return fmt.Errorf("unknown sources command %q; expected list, add, remove, enable or disable", args[0])
	}
}

func encodeSourcesJSON(w io.Writer, value any) error {
	return json.NewEncoder(w).Encode(value)
}

func removedNote(installed int) string {
	switch installed {
	case 0:
		return ""
	case 1:
		return "Its installed item keeps working but gets no updates."
	default:
		return fmt.Sprintf("Its %d installed items keep working but get no updates.", installed)
	}
}

// listSources describes the two official stores, every store the user added,
// and every removed store that still owns installs. Stores are read in
// parallel within one deadline.
func listSources(ctx context.Context) []sourceEntry {
	config, configErr := readSourcesConfig()
	entries := make([]sourceEntry, 2+len(config.Sources))
	var wg sync.WaitGroup
	wg.Add(len(entries))
	go func() {
		defer wg.Done()
		entries[0] = describeOfficialSource(ctx, officialExtrasSourceID, "Ryostore", extrasBase(), "plugins/registry.json", officialExtrasCategories())
	}()
	go func() {
		defer wg.Done()
		entries[1] = describeOfficialSource(ctx, officialRevampedSourceID, "Ryostore shells", revampedBase(), "barstyles/registry.json", []string{"subshells", "barstyles", profilesCategory})
	}()
	for i, source := range config.Sources {
		go func() {
			defer wg.Done()
			entries[2+i] = describeUserSource(ctx, source)
		}()
	}
	wg.Wait()
	if configErr != nil {
		entries = append(entries, sourceEntry{
			ID: "config", Name: "ryostore-sources.json", Status: "error", Error: configErr.Error(),
		})
	}
	configured := map[string]bool{}
	for _, source := range config.Sources {
		configured[source.ID] = true
	}
	for _, id := range stateSourceIDs() {
		installed := countSourceReceipts(id)
		if configured[id] || installed == 0 {
			continue
		}
		entry := sourceEntry{ID: id, Name: id, Status: "removed", Installed: installed, Note: removedNote(installed)}
		if record, ok := readSourceRecord(id); ok {
			entry.Name, entry.URL = record.Name, record.URL
		}
		entries = append(entries, entry)
	}
	return entries
}

// officialExtrasCategories are the categories the extras store serves.
func officialExtrasCategories() []string {
	categories := []string{}
	for _, provider := range providersWithProfileGate(nil) {
		switch id := provider.Category().ID; id {
		case "subshells", "barstyles", flatpakCategory, "omarchy-plugins":
		default:
			categories = append(categories, id)
		}
	}
	return categories
}

// describeOfficialSource reads one registry of an official store to tell
// whether it answers now.
func describeOfficialSource(ctx context.Context, id, name, base, probe string, categories []string) sourceEntry {
	entry := sourceEntry{
		ID: id, Name: name, URL: base, Official: true, Enabled: true, Status: "ok",
		Categories: categories, Note: "Official: reviewed by Ryoku. Always on.",
	}
	for _, category := range categories {
		entries, err := os.ReadDir(filepath.Join(storeStateDir(), category))
		if err != nil {
			continue
		}
		for _, file := range entries {
			if !file.IsDir() && filepath.Ext(file.Name()) == ".json" {
				entry.Installed++
			}
		}
	}
	if _, err := getFrom(ctx, &http.Client{Timeout: cacheTimeout}, base, probe); err != nil {
		entry.Status, entry.Error = "error", err.Error()
	}
	return entry
}

// describeUserSource reads a store the user added: its index and every
// registry it lists, each validated, and counts the items it serves.
func describeUserSource(ctx context.Context, source storeSource) sourceEntry {
	installed := countSourceReceipts(source.ID)
	entry := sourceEntry{
		ID: source.ID, Name: source.Name, URL: source.URL, Enabled: source.Enabled,
		Added: source.Added, Installed: installed,
	}
	if !source.Enabled {
		entry.Status = "disabled"
		if installed > 0 {
			entry.Note = "Turned off. " + removedNote(installed)
		}
		return entry
	}
	loaded := loadUserSource(ctx, source, true)
	describeLoadedSource(ctx, &entry, loaded)
	return entry
}

func describeLoadedSource(ctx context.Context, entry *sourceEntry, loaded loadedSource) {
	if loaded.err != nil {
		entry.Status, entry.Error = "error", loaded.err.Error()
		return
	}
	entry.Description, entry.Homepage = loaded.index.Description, loaded.index.Homepage
	entry.Categories = loaded.index.Categories
	count := 0
	for _, category := range loaded.index.Categories {
		products, _, err := loadProductRegistry(ctx, loaded.cache, category, true)
		if err != nil {
			entry.Status, entry.Error = "error", fmt.Sprintf("%s: %v", category, err)
			return
		}
		count += len(products)
	}
	entry.Status = "ok"
	entry.Items = &count
}

// addSource reads and validates a store before recording it: the index, then
// every registry it lists. A store that fails any check is not added.
func addSource(ctx context.Context, rawURL, name string, now time.Time) (sourceEntry, error) {
	address, err := normalizeSourceURL(rawURL)
	if err != nil {
		return sourceEntry{}, err
	}
	if address == extrasBase() || address == revampedBase() || address == defaultExtrasBase || address == defaultRevampedBase {
		return sourceEntry{}, fmt.Errorf("that is an official store; it is always on")
	}
	if name != "" {
		if err := validSourceName(name); err != nil {
			return sourceEntry{}, err
		}
	}
	scratch, err := os.MkdirTemp("", "ryostore-source-")
	if err != nil {
		return sourceEntry{}, err
	}
	defer os.RemoveAll(scratch)
	probe := newSourceCache(address, "", scratch)
	loaded := loadedSource{cache: probe}
	raw, _, err := probe.Fetch(ctx, storeIndexName, true)
	if err != nil {
		return sourceEntry{}, fmt.Errorf("no store at %s: %w", address, err)
	}
	if loaded.index, err = parseStoreIndex(raw); err != nil {
		return sourceEntry{}, err
	}
	entry := sourceEntry{}
	describeLoadedSource(ctx, &entry, loaded)
	if entry.Status != "ok" {
		return sourceEntry{}, fmt.Errorf("the store at %s is not valid: %s", address, entry.Error)
	}
	if name == "" {
		name = loaded.index.Name
	}

	unlock, err := lockConfigFile(sourcesConfigPath())
	if err != nil {
		return sourceEntry{}, err
	}
	defer unlock()
	config, err := readSourcesConfig()
	if err != nil {
		return sourceEntry{}, err
	}
	taken := map[string]bool{}
	for _, existing := range config.Sources {
		if existing.URL == address {
			return sourceEntry{}, fmt.Errorf("%s is already added as %s", address, existing.Name)
		}
		taken[existing.ID] = true
	}
	// A removed store's installs keep their id, so a new store never takes it.
	for _, id := range stateSourceIDs() {
		taken[id] = true
	}
	source := storeSource{
		ID: sourceIDFor(name, taken), Name: name, URL: address, Enabled: true,
		Added: now.UTC().Format(time.RFC3339),
	}
	config.Sources = append(config.Sources, source)
	if err := writeSourcesConfig(config); err != nil {
		return sourceEntry{}, err
	}
	dropCatalogSnapshots()
	entry.ID, entry.Name, entry.URL, entry.Enabled, entry.Added = source.ID, source.Name, source.URL, true, source.Added
	return entry, nil
}

func removeSource(id string) (int, error) {
	if reservedSourceIDs[id] {
		return 0, fmt.Errorf("the %s source is official and cannot be removed", id)
	}
	unlock, err := lockConfigFile(sourcesConfigPath())
	if err != nil {
		return 0, err
	}
	defer unlock()
	config, err := readSourcesConfig()
	if err != nil {
		return 0, err
	}
	source, index, ok := findSource(config, id)
	if !ok {
		return 0, fmt.Errorf("no store source %q", id)
	}
	config.Sources = append(config.Sources[:index], config.Sources[index+1:]...)
	if err := writeSourcesConfig(config); err != nil {
		return 0, err
	}
	_ = os.RemoveAll(userSourceCacheDir(source))
	if err := forgetRemovedSource(id); err != nil {
		return 0, err
	}
	dropCatalogSnapshots()
	return countSourceReceipts(id), nil
}

func setSourceEnabled(id string, enabled bool) (storeSource, error) {
	if reservedSourceIDs[id] {
		return storeSource{}, fmt.Errorf("the %s source is official and always on", id)
	}
	unlock, err := lockConfigFile(sourcesConfigPath())
	if err != nil {
		return storeSource{}, err
	}
	defer unlock()
	config, err := readSourcesConfig()
	if err != nil {
		return storeSource{}, err
	}
	_, index, ok := findSource(config, id)
	if !ok {
		return storeSource{}, fmt.Errorf("no store source %q", id)
	}
	config.Sources[index].Enabled = enabled
	if err := writeSourcesConfig(config); err != nil {
		return storeSource{}, err
	}
	dropCatalogSnapshots()
	return config.Sources[index], nil
}

// dropCatalogSnapshots forgets every launch snapshot after the source set
// changes, so the next open rebuilds with the stores and installs as they are.
func dropCatalogSnapshots() {
	matches, _ := filepath.Glob(filepath.Join(extrasCacheDir(), "catalog*.json"))
	for _, match := range matches {
		_ = os.Remove(match)
	}
}
