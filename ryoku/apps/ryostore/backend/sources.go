package main

// Store sources. The two official stores (ryoku-dev/ryostore and
// ryoku-dev/ryostore-revamped) are always present. A user may add more: a
// static tree with a ryostore.json index and one canonical registry per
// category, exactly the official product format. A store the user added may
// only serve the categories in userSourceCategories, every one of which installs
// through the hashed manifest transaction; its items are never official, and
// its receipts, files and transaction journals sit under their own sources/<id>/
// folders so two stores can list the same category/id without sharing state.

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"
)

const (
	officialExtrasSourceID   = "ryostore"
	officialRevampedSourceID = "ryostore-revamped"
	storeIndexName           = "ryostore.json"
	maxSourceNameRunes       = 48
	maxSourceDescription     = 280
	maxSourceURL             = 512
	maxSourceIDLength        = 32
)

// userSourceCategories are the categories a store the user added may serve, in
// rail order. Each installs only through the manifest+sha256 transaction and
// runs from a receipt-built index, so a second store's copy lives in its own
// folder. Everything else (profiles, bundles, installers, rices, colour
// schemes, Nautilus scripts, and the categories applied from a shared folder
// outside the store's own: lock screens, fastfetch, Ryotunes skins, Vesktop
// themes and the picture categories) comes only from the official stores.
var userSourceCategories = []string{
	"subshells", "barstyles", "docks", "launchers", "osds", "notifications", "sidebars", "plugins",
}

var userSourceIDPattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

// reservedSourceIDs name the built-in sources an item can carry.
var reservedSourceIDs = map[string]bool{
	officialExtrasSourceID: true, officialRevampedSourceID: true,
	"ryoku": true, "flathub": true, "omarchy": true, "official": true, "local": true,
}

func userSourceCategory(category string) bool {
	for _, allowed := range userSourceCategories {
		if allowed == category {
			return true
		}
	}
	return false
}

func validUserSourceID(id string) bool {
	return len(id) <= maxSourceIDLength && userSourceIDPattern.MatchString(id) && !reservedSourceIDs[id]
}

// validateProductIdentity checks a source/category/id triple before any path
// is built from it. An empty source is the official stores.
func validateProductIdentity(source, category, id string) error {
	if !validProductCategory(category) || !productIDPattern.MatchString(id) {
		return fmt.Errorf("invalid product identity %s/%s", category, id)
	}
	if source == "" {
		return nil
	}
	if !validUserSourceID(source) {
		return fmt.Errorf("invalid store source %q", source)
	}
	if !userSourceCategory(category) {
		return fmt.Errorf("%s cannot come from a store you added", category)
	}
	return nil
}

// userSourceStateDir holds one user store's receipts and its source.json.
func userSourceStateDir(source string) string {
	return filepath.Join(storeStateDir(), "sources", source)
}

// storeSource is one store the user added, as ryostore-sources.json records it.
type storeSource struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	URL     string `json:"url"`
	Enabled bool   `json:"enabled"`
	Added   string `json:"added"`
}

type sourcesConfig struct {
	Schema  int           `json:"schema"`
	Sources []storeSource `json:"sources"`
}

// storeIndex is a user store's ryostore.json.
type storeIndex struct {
	Schema      int      `json:"schema"`
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Homepage    string   `json:"homepage,omitempty"`
	Categories  []string `json:"categories"`
}

func sourcesConfigPath() string {
	return filepath.Join(configHome(), "ryoku", "ryostore-sources.json")
}

// userSourceCacheDir keys the cache by id and URL, so a source re-added at
// another address never serves the old address's bytes.
func userSourceCacheDir(source storeSource) string {
	sum := sha256.Sum256([]byte(source.URL))
	return filepath.Join(xdgCacheHome(), "ryoku", "ryostore-sources", source.ID+"-"+hex.EncodeToString(sum[:6]))
}

func newUserSourceCache(source storeSource) *Cache {
	cache := newSourceCache(source.URL, "", userSourceCacheDir(source))
	cache.source = source.ID
	return cache
}

// normalizeSourceURL accepts https://, http:// on the loopback only, and
// file:///absolute or a bare absolute path. No credentials, query or fragment.
func normalizeSourceURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", fmt.Errorf("a store address is needed")
	}
	if len(raw) > maxSourceURL || strings.ContainsAny(raw, "\x00\r\n\t ") {
		return "", fmt.Errorf("the store address is not valid")
	}
	if strings.HasPrefix(raw, "/") {
		raw = "file://" + raw
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("the store address is not valid: %v", err)
	}
	if parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" || parsed.Opaque != "" {
		return "", fmt.Errorf("the store address may not carry a user name, query or fragment")
	}
	switch parsed.Scheme {
	case "https":
		if parsed.Hostname() == "" {
			return "", fmt.Errorf("the store address has no host")
		}
	case "http":
		host := parsed.Hostname()
		if host != "localhost" {
			ip := net.ParseIP(host)
			if ip == nil || !ip.IsLoopback() {
				return "", fmt.Errorf("plain http is allowed only on this computer (127.0.0.1); use https")
			}
		}
	case "file":
		if parsed.Host != "" && parsed.Host != "localhost" {
			return "", fmt.Errorf("a file address must be file:///absolute/path")
		}
		clean := path.Clean(parsed.Path)
		if !strings.HasPrefix(clean, "/") || clean == "/" {
			return "", fmt.Errorf("a file address must name an absolute folder")
		}
		return "file://" + clean, nil
	default:
		return "", fmt.Errorf("the store address must start with https://")
	}
	parsed.Path = strings.TrimRight(path.Clean("/"+parsed.Path), "/")
	parsed.RawPath = ""
	return strings.TrimRight(parsed.String(), "/"), nil
}

func validSourceText(value string, maxRunes int, allowEmpty bool) bool {
	if value == "" {
		return allowEmpty
	}
	if !utf8.ValidString(value) || utf8.RuneCountInString(value) > maxRunes || strings.TrimSpace(value) != value {
		return false
	}
	for _, r := range value {
		if unicode.IsControl(r) {
			return false
		}
	}
	return true
}

func validSourceName(name string) error {
	if !validSourceText(name, maxSourceNameRunes, false) {
		return fmt.Errorf("a store name is 1 to %d characters with no control characters or edge spaces", maxSourceNameRunes)
	}
	return nil
}

// parseStoreIndex validates a ryostore.json strictly: known fields only,
// schema 1, a plain name, an https homepage, and categories a user store may
// serve. A store naming any other category is refused as a whole.
func parseStoreIndex(raw []byte) (storeIndex, error) {
	if err := strictJSONFields(raw, map[string]bool{
		"schema": true, "name": true, "description": true, "homepage": true, "categories": true,
	}); err != nil {
		return storeIndex{}, fmt.Errorf("%s: %w", storeIndexName, err)
	}
	var index storeIndex
	if err := decodeOneJSON(raw, &index); err != nil {
		return storeIndex{}, fmt.Errorf("%s: %w", storeIndexName, err)
	}
	if index.Schema != 1 {
		return storeIndex{}, fmt.Errorf("%s: unsupported schema %d", storeIndexName, index.Schema)
	}
	if err := validSourceName(index.Name); err != nil {
		return storeIndex{}, fmt.Errorf("%s: name: %w", storeIndexName, err)
	}
	if !validSourceText(index.Description, maxSourceDescription, true) {
		return storeIndex{}, fmt.Errorf("%s: description is over %d characters or holds control characters", storeIndexName, maxSourceDescription)
	}
	if index.Homepage != "" {
		parsed, err := url.Parse(index.Homepage)
		if err != nil || parsed.Scheme != "https" || parsed.Hostname() == "" || parsed.User != nil || len(index.Homepage) > maxSourceURL {
			return storeIndex{}, fmt.Errorf("%s: homepage must be an https:// address", storeIndexName)
		}
	}
	if len(index.Categories) == 0 {
		return storeIndex{}, fmt.Errorf("%s: categories is empty", storeIndexName)
	}
	seen := map[string]bool{}
	for _, category := range index.Categories {
		if seen[category] {
			return storeIndex{}, fmt.Errorf("%s: category %q is listed twice", storeIndexName, category)
		}
		seen[category] = true
		if !userSourceCategory(category) {
			return storeIndex{}, fmt.Errorf("%s: category %q cannot come from a store you add; allowed: %s",
				storeIndexName, category, strings.Join(userSourceCategories, ", "))
		}
	}
	return index, nil
}

// readSourcesConfig reads ryostore-sources.json. A missing file is no sources;
// a malformed one is an error, so a hand edit never silently drops a store.
func readSourcesConfig() (sourcesConfig, error) {
	config := sourcesConfig{Schema: 1, Sources: []storeSource{}}
	raw, err := os.ReadFile(sourcesConfigPath())
	if os.IsNotExist(err) {
		return config, nil
	}
	if err != nil {
		return config, err
	}
	if err := strictJSONFields(raw, map[string]bool{"schema": true, "sources": true}); err != nil {
		return config, fmt.Errorf("ryostore-sources.json: %w", err)
	}
	var fields struct {
		Sources []json.RawMessage `json:"sources"`
	}
	if err := json.Unmarshal(raw, &fields); err != nil {
		return config, fmt.Errorf("ryostore-sources.json: %w", err)
	}
	for _, source := range fields.Sources {
		if err := strictJSONFields(source, map[string]bool{"id": true, "name": true, "url": true, "enabled": true, "added": true}); err != nil {
			return config, fmt.Errorf("ryostore-sources.json: source: %w", err)
		}
	}
	if err := decodeOneJSON(raw, &config); err != nil {
		return config, fmt.Errorf("ryostore-sources.json: %w", err)
	}
	if err := validateSourcesConfig(config); err != nil {
		return config, err
	}
	if config.Sources == nil {
		config.Sources = []storeSource{}
	}
	return config, nil
}

func validateSourcesConfig(config sourcesConfig) error {
	if config.Schema != 1 {
		return fmt.Errorf("ryostore-sources.json: unsupported schema %d", config.Schema)
	}
	ids := map[string]bool{}
	urls := map[string]bool{}
	for _, source := range config.Sources {
		label := "ryostore-sources.json: source " + source.ID
		if !validUserSourceID(source.ID) {
			return fmt.Errorf("ryostore-sources.json: invalid source id %q", source.ID)
		}
		if ids[source.ID] {
			return fmt.Errorf("%s: duplicate id", label)
		}
		ids[source.ID] = true
		if err := validSourceName(source.Name); err != nil {
			return fmt.Errorf("%s: %w", label, err)
		}
		normalized, err := normalizeSourceURL(source.URL)
		if err != nil {
			return fmt.Errorf("%s: %w", label, err)
		}
		if normalized != source.URL {
			return fmt.Errorf("%s: address is not in its normal form %q", label, normalized)
		}
		if urls[source.URL] {
			return fmt.Errorf("%s: duplicate address", label)
		}
		urls[source.URL] = true
		if _, err := time.Parse(time.RFC3339, source.Added); err != nil {
			return fmt.Errorf("%s: added is not an RFC 3339 time", label)
		}
	}
	return nil
}

func writeSourcesConfig(config sourcesConfig) error {
	if err := validateSourcesConfig(config); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}
	return atomicWrite(sourcesConfigPath(), append(raw, '\n'), 0o644)
}

func findSource(config sourcesConfig, id string) (storeSource, int, bool) {
	for index, source := range config.Sources {
		if source.ID == id {
			return source, index, true
		}
	}
	return storeSource{}, -1, false
}

// sourceIDFor slugs a name into an id no built-in or existing source holds.
func sourceIDFor(name string, taken map[string]bool) string {
	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(name) {
		if r < unicode.MaxASCII && (unicode.IsLetter(r) || unicode.IsDigit(r)) {
			b.WriteRune(r)
			dash = false
		} else if !dash && b.Len() > 0 {
			b.WriteByte('-')
			dash = true
		}
	}
	base := strings.Trim(b.String(), "-")
	if len(base) > maxSourceIDLength-3 {
		base = strings.Trim(base[:maxSourceIDLength-3], "-")
	}
	if base == "" {
		base = "store"
	}
	candidate := base
	for n := 2; !validUserSourceID(candidate) || taken[candidate]; n++ {
		candidate = fmt.Sprintf("%s-%d", base, n)
	}
	return candidate
}

// enabledSourcesKey identifies the set of enabled user stores, so the launch
// snapshot of one set is never served for another. Empty with none.
func enabledSourcesKey(config sourcesConfig) string {
	rows := []string{}
	for _, source := range config.Sources {
		if source.Enabled {
			rows = append(rows, source.ID+"\x00"+source.URL)
		}
	}
	if len(rows) == 0 {
		return ""
	}
	sort.Strings(rows)
	sum := sha256.Sum256([]byte(strings.Join(rows, "\n")))
	return hex.EncodeToString(sum[:8])
}

// sourceRecord is store/sources/<id>/source.json: the name and address a user
// store had when something was installed from it, so its installs stay
// labelled after the user removes the store.
type sourceRecord struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	URL  string `json:"url"`
}

func recordUserSource(source string) error {
	if source == "" {
		return nil
	}
	config, err := readSourcesConfig()
	if err != nil {
		return err
	}
	configured, _, ok := findSource(config, source)
	if !ok {
		return fmt.Errorf("store source %q is not configured", source)
	}
	raw, err := json.Marshal(sourceRecord{ID: configured.ID, Name: configured.Name, URL: configured.URL})
	if err != nil {
		return err
	}
	return atomicWrite(filepath.Join(userSourceStateDir(source), "source.json"), append(raw, '\n'), 0o600)
}

func readSourceRecord(source string) (sourceRecord, bool) {
	raw, err := os.ReadFile(filepath.Join(userSourceStateDir(source), "source.json"))
	if err != nil {
		return sourceRecord{}, false
	}
	var record sourceRecord
	if decodeOneJSON(raw, &record) != nil || record.ID != source || validSourceName(record.Name) != nil {
		return sourceRecord{}, false
	}
	return record, true
}

// sourceDisplayName names a source for the user: the official stores as
// Ryostore, a user store by its configured or last recorded name.
func sourceDisplayName(source string) string {
	if source == "" {
		return "Ryostore"
	}
	if config, err := readSourcesConfig(); err == nil {
		if configured, _, ok := findSource(config, source); ok {
			return configured.Name
		}
	}
	if record, ok := readSourceRecord(source); ok {
		return record.Name
	}
	return source
}

// stateSourceIDs lists the user stores that have a state folder, whether or
// not they are still configured.
func stateSourceIDs() []string {
	entries, err := os.ReadDir(filepath.Join(storeStateDir(), "sources"))
	if err != nil {
		return nil
	}
	ids := []string{}
	for _, entry := range entries {
		if entry.IsDir() && validUserSourceID(entry.Name()) {
			ids = append(ids, entry.Name())
		}
	}
	return ids
}

func receiptExists(source, category, id string) bool {
	if validateProductIdentity(source, category, id) != nil {
		return false
	}
	_, err := os.Lstat(sourceReceiptPath(source, category, id))
	return err == nil
}

// productOwners lists every source holding a receipt for category/id, the
// official stores ("") first.
func productOwners(category, id string) []string {
	owners := []string{}
	if receiptExists("", category, id) {
		owners = append(owners, "")
	}
	if !userSourceCategory(category) {
		return owners
	}
	for _, source := range stateSourceIDs() {
		if receiptExists(source, category, id) {
			owners = append(owners, source)
		}
	}
	return owners
}

// otherProductOwner is the collision rule: a source other than source that
// already holds a receipt for category/id, found or not.
func otherProductOwner(source, category, id string) (string, bool) {
	for _, owner := range productOwners(category, id) {
		if owner != source {
			return owner, true
		}
	}
	return "", false
}

// installedReceipts reads every receipt of category across the official stores
// and every user store, official first. An unreadable receipt is reported in
// problems and left out.
func installedReceipts(category string) ([]Receipt, []string, error) {
	sources := []string{""}
	if userSourceCategory(category) {
		sources = append(sources, stateSourceIDs()...)
	}
	receipts := []Receipt{}
	problems := []string{}
	for _, source := range sources {
		directory := filepath.Join(storeStateDir(), category)
		if source != "" {
			directory = filepath.Join(userSourceStateDir(source), category)
		}
		entries, err := os.ReadDir(directory)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, nil, err
		}
		for _, entry := range entries {
			if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
				continue
			}
			id := strings.TrimSuffix(entry.Name(), ".json")
			if !productIDPattern.MatchString(id) {
				continue
			}
			receipt, err := readSourceReceipt(source, category, id)
			if err != nil {
				label := category + "/" + id
				if source != "" {
					label = source + ":" + label
				}
				problems = append(problems, fmt.Sprintf("%s: %v", label, err))
				continue
			}
			receipts = append(receipts, receipt)
		}
	}
	return receipts, problems, nil
}

func countSourceReceipts(source string) int {
	count := 0
	for _, category := range userSourceCategories {
		entries, err := os.ReadDir(filepath.Join(userSourceStateDir(source), category))
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if !entry.IsDir() && filepath.Ext(entry.Name()) == ".json" {
				count++
			}
		}
	}
	return count
}

// officialItemSource stamps an item from a built-in provider with the source
// it really comes from.
func officialItemSource(item Item) (string, string) {
	if isBuiltinCatalogItem(item) {
		return "ryoku", "Ryoku"
	}
	switch item.Category {
	case "subshells", "barstyles", profilesCategory:
		return officialRevampedSourceID, "Ryostore shells"
	case flatpakCategory:
		return "flathub", "Flathub"
	case "omarchy-plugins":
		return "omarchy", "Omarchy marketplace"
	}
	return officialExtrasSourceID, "Ryostore"
}

// userCategoryProvider is the provider that serves one allowed category from
// a user store's cache.
func userCategoryProvider(category string, cache *Cache) (Provider, bool) {
	if !userSourceCategory(category) {
		return nil, false
	}
	switch category {
	case "plugins":
		return pluginProvider{cache: cache}, true
	case "subshells", "barstyles":
		return newShellItemProvider(category, cache), true
	}
	product, ok := shellQMLProductFor(category)
	if !ok {
		return nil, false
	}
	return newShellQMLProvider(product, cache), true
}

// loadedSource is one user store with its fetched index, or why it failed.
type loadedSource struct {
	source storeSource
	cache  *Cache
	index  storeIndex
	err    error
}

func loadUserSource(ctx context.Context, source storeSource, refresh bool) loadedSource {
	loaded := loadedSource{source: source, cache: newUserSourceCache(source)}
	raw, _, err := loaded.cache.Fetch(ctx, storeIndexName, refresh)
	if err != nil {
		loaded.err = err
		return loaded
	}
	loaded.index, loaded.err = parseStoreIndex(raw)
	return loaded
}

func loadUserSources(ctx context.Context, sources []storeSource, refresh bool) []loadedSource {
	loaded := make([]loadedSource, len(sources))
	var wg sync.WaitGroup
	for i, source := range sources {
		wg.Add(1)
		go func() {
			defer wg.Done()
			loaded[i] = loadUserSource(ctx, source, refresh)
		}()
	}
	wg.Wait()
	return loaded
}

// sourcePart is one user store's provider for a category.
type sourcePart struct {
	source   storeSource
	provider Provider
}

// sourcedProvider joins one official category with the user stores serving it.
// Install and Remove stay the official provider's; `ryostore install|remove
// --source` routes a user store's copy.
type sourcedProvider struct {
	Provider
	parts []sourcePart
}

func (p sourcedProvider) Load(ctx context.Context, refresh bool) ([]Item, SourceState, error) {
	type result struct {
		items []Item
		state SourceState
		err   error
	}
	results := make([]result, len(p.parts)+1)
	var wg sync.WaitGroup
	for i := range results {
		wg.Add(1)
		go func() {
			defer wg.Done()
			provider := p.Provider
			if i > 0 {
				provider = p.parts[i-1].provider
			}
			items, state, err := provider.Load(ctx, refresh)
			results[i] = result{items, state, err}
		}()
	}
	wg.Wait()

	category := p.Category().ID
	items := append([]Item{}, results[0].items...)
	loaded := map[string]bool{}
	for i, part := range p.parts {
		r := results[i+1]
		if r.err != nil {
			continue
		}
		loaded[part.source.ID] = true
		for _, item := range r.items {
			item.Source, item.SourceName, item.Official = part.source.ID, part.source.Name, false
			items = append(items, item)
		}
	}
	// Installs from a store that is removed, turned off or out of reach stay
	// listed and removable; they get no updates.
	receipts, _, _ := installedReceipts(category)
	for _, receipt := range receipts {
		if receipt.Source == "" || loaded[receipt.Source] {
			continue
		}
		name := receipt.Name
		if name == "" {
			name = receipt.ID
		}
		items = append(items, Item{
			ID: receipt.ID, Category: category, Name: name, Summary: receipt.Summary,
			Source: receipt.Source, SourceName: sourceDisplayName(receipt.Source), Official: false,
			Version: receipt.Version, InstalledVersion: receipt.Version, Installed: true,
			Metadata: map[string]any{"sourceUnavailable": true},
		})
	}
	markInstalledElsewhere(items)
	return items, results[0].state, results[0].err
}

// markInstalledElsewhere tells an item that another source's copy of the same
// category/id is installed, which is why installing it is refused.
func markInstalledElsewhere(items []Item) {
	owner := map[string]Item{}
	for _, item := range items {
		if item.Installed {
			owner[item.Category+"/"+item.ID] = item
		}
	}
	for i := range items {
		if items[i].Installed {
			continue
		}
		if other, ok := owner[items[i].Category+"/"+items[i].ID]; ok && other.Source != items[i].Source {
			items[i].InstalledFrom = other.SourceName
			if items[i].InstalledFrom == "" {
				items[i].InstalledFrom = "Ryostore"
			}
		}
	}
}

// catalogProviders joins the user stores into the official providers for a
// catalogue build. With no user store configured and none installed from, it
// returns provs unchanged.
func catalogProviders(ctx context.Context, provs []Provider, refresh bool) []Provider {
	config, err := readSourcesConfig()
	if err != nil {
		fmt.Fprintln(os.Stderr, "ryostore: store sources ignored:", err)
		config = sourcesConfig{Schema: 1}
	}
	enabled := []storeSource{}
	for _, source := range config.Sources {
		if source.Enabled {
			enabled = append(enabled, source)
		}
	}
	if len(enabled) == 0 && len(stateSourceIDs()) == 0 {
		return provs
	}
	parts := map[string][]sourcePart{}
	for _, loaded := range loadUserSources(ctx, enabled, refresh) {
		if loaded.err != nil {
			fmt.Fprintf(os.Stderr, "ryostore: store %s: %v\n", loaded.source.ID, loaded.err)
			continue
		}
		for _, category := range loaded.index.Categories {
			if provider, ok := userCategoryProvider(category, loaded.cache); ok {
				parts[category] = append(parts[category], sourcePart{source: loaded.source, provider: provider})
			}
		}
	}
	wrapped := make([]Provider, len(provs))
	for i, provider := range provs {
		wrapped[i] = provider
		if userSourceCategory(provider.Category().ID) {
			wrapped[i] = sourcedProvider{Provider: provider, parts: parts[provider.Category().ID]}
		}
	}
	return wrapped
}

// userSourceInstallProvider resolves the provider that installs category from
// one enabled user store, reading its index live.
func userSourceInstallProvider(ctx context.Context, sourceID, category string) (Provider, error) {
	config, err := readSourcesConfig()
	if err != nil {
		return nil, err
	}
	source, _, ok := findSource(config, sourceID)
	if !ok {
		return nil, fmt.Errorf("no store source %q", sourceID)
	}
	if !source.Enabled {
		return nil, fmt.Errorf("the %s store is turned off", source.Name)
	}
	if !userSourceCategory(category) {
		return nil, fmt.Errorf("%s cannot come from a store you added", category)
	}
	loaded := loadUserSource(ctx, source, true)
	if loaded.err != nil {
		return nil, fmt.Errorf("store %s: %w", source.Name, loaded.err)
	}
	listed := false
	for _, served := range loaded.index.Categories {
		listed = listed || served == category
	}
	if !listed {
		return nil, fmt.Errorf("the %s store does not serve %s", source.Name, category)
	}
	provider, _ := userCategoryProvider(category, loaded.cache)
	return provider, nil
}

// removeUserSourceProduct removes one user store's install. It needs no
// network and works for a store that is turned off or removed.
func removeUserSourceProduct(ctx context.Context, source, category, id string) error {
	if err := removeSourceProduct(ctx, source, category, id); err != nil {
		return err
	}
	if err := forgetRemovedSource(source); err != nil {
		return err
	}
	if !shellCategory(category) {
		return nil
	}
	if err := removeShellItemArtifacts(category, id); err != nil {
		return err
	}
	if _, err := runShellsSync(false); err != nil {
		return fmt.Errorf("%s/%s is removed, but the shells index was not rebuilt: %w", category, id, err)
	}
	return nil
}

// forgetRemovedSource drops the folders of a store the user removed once its
// last install is gone, so nothing of it stays behind.
func forgetRemovedSource(source string) error {
	unlock, err := lockTree(storeTransactionLockPath())
	if err != nil {
		return err
	}
	defer unlock()
	if config, err := readSourcesConfig(); err != nil {
		return nil
	} else if _, _, configured := findSource(config, source); configured {
		return nil
	}
	if countSourceReceipts(source) > 0 {
		return nil
	}
	journals := filepath.Join(storeTransactionsDir(), "sources", source)
	if files, _ := filepath.Glob(filepath.Join(journals, "*", "*.json")); len(files) > 0 {
		return nil
	}
	for _, dir := range []string{userSourceStateDir(source), filepath.Join(dataHome(), "ryoku", "sources", source), journals} {
		if err := os.RemoveAll(dir); err != nil {
			return err
		}
	}
	return nil
}
