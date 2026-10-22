// Shell QML products are folders of QML the shell runs in place: the app
// launchers Super+Space opens and the dock designs every bar style shares. The
// shell ships its own of each; a store product installs into
// dataHome/ryoku/<category>/<id>, runs from a content-addressed view, and joins
// the shell and its settings surface through a <category>.json index the
// running shell follows. Install never wears one: the settings surface saves the
// choice, and removing the worn product moves that choice back to the shell's
// default inside the same transaction (product_selection.go).
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type shellQMLProduct struct {
	category Category
	// noun names one product in errors: "launcher", "dock".
	noun string
	// entry is the root the shell loads; preview is the optional Settings
	// preview beside it, "" for a category that shows none.
	entry   string
	preview string
	// builtins are the ids the shell ships. A store product never takes one:
	// the shell would always run its own.
	builtins map[string]bool
	views    productViews
}

var (
	launcherProducts = shellQMLProduct{
		category: Category{
			ID:          "launchers",
			Name:        "App launchers",
			Group:       "wear",
			Description: "Community launchers for Super+Space. Install here, pick one in Settings -> App Launcher.",
		},
		noun:    "launcher",
		entry:   "Main.qml",
		preview: "Preview.qml",
		// The shell's modules/launcher/catalog.json.
		builtins: map[string]bool{
			"main": true, "hero": true, "okshell": true, "kairos": true, "spotlight": true, "python": true,
		},
		views: launcherViews,
	}
	dockProducts = shellQMLProduct{
		category: Category{
			ID:          "docks",
			Name:        "Docks",
			Group:       "wear",
			Description: "Community docks every bar style shares. Install here, pick one in Edit desktop -> Dock.",
		},
		noun:  "dock",
		entry: "Dock.qml",
		// The shell's services/Dock.qml builtinDesigns.
		builtins: map[string]bool{
			"ryoku": true, "python": true, "shima": true, "ii": true, "caelestia": true, "none": true,
		},
		views: dockViews,
	}
	// The overlays every bar style shares. Each ships one design of its own,
	// "ryoku", drawn by the shell's OSD, popup and Controls hosts.
	osdProducts = shellQMLProduct{
		category: Category{
			ID:          "osds",
			Name:        "On-screen displays",
			Group:       "wear",
			Description: "Community volume, brightness and keyboard pop-ups. Install here, pick one in Settings -> Overlays.",
		},
		noun:     "OSD design",
		entry:    "Osd.qml",
		builtins: map[string]bool{"ryoku": true},
		views:    osdViews,
	}
	notificationProducts = shellQMLProduct{
		category: Category{
			ID:          "notifications",
			Name:        "Notifications",
			Group:       "wear",
			Description: "Community notification cards. Install here, pick one in Settings -> Overlays.",
		},
		noun:     "notification design",
		entry:    "Card.qml",
		builtins: map[string]bool{"ryoku": true},
		views:    notificationViews,
	}
	sidebarProducts = shellQMLProduct{
		category: Category{
			ID:          "sidebars",
			Name:        "Sidebars",
			Group:       "wear",
			Description: "Community designs for the Super+Esc panel. Install here, pick one in Settings -> Overlays.",
		},
		noun:     "sidebar design",
		entry:    "Sidebar.qml",
		builtins: map[string]bool{"ryoku": true},
		views:    sidebarViews,
	}
)

func shellQMLProductFor(category string) (shellQMLProduct, bool) {
	for _, product := range []shellQMLProduct{launcherProducts, dockProducts, osdProducts, notificationProducts, sidebarProducts} {
		if product.category.ID == category {
			return product, true
		}
	}
	return shellQMLProduct{}, false
}

// shellQMLIndexRow is one runnable store product: its entry, and its preview
// when it ships one, both inside View.
type shellQMLIndexRow struct {
	ID      string `json:"id"`
	Version string `json:"version"`
	Preview string `json:"preview"`
	View    string `json:"view"`
}

type shellQMLProvider struct {
	product shellQMLProduct
	cache   *Cache
}

func newShellQMLProvider(product shellQMLProduct, cache *Cache) shellQMLProvider {
	if cache == nil {
		cache = newCache()
	}
	return shellQMLProvider{product: product, cache: cache}
}

func (p shellQMLProvider) Category() Category {
	return p.product.category
}

func (p shellQMLProvider) Load(ctx context.Context, refresh bool) ([]Item, SourceState, error) {
	category := p.product.category.ID
	local, err := p.product.rebuildIndex()
	if err != nil {
		return nil, SourceState{}, err
	}
	rows := make(map[string]shellQMLIndexRow, len(local))
	for _, row := range local {
		rows[row.ID] = row
	}
	selection, _ := selectionFor(category)
	active := selection.current()

	entries, state, registryErr := loadProductRegistry(ctx, p.cache, category, refresh)
	if productRegistryAbsent(registryErr) {
		entries, registryErr = nil, nil
	}
	if registryErr != nil && !productRegistryUnavailable(registryErr) {
		return nil, state, registryErr
	}
	items := make([]Item, 0, len(entries)+len(local))
	seen := make(map[string]bool, len(entries))
	for _, entry := range entries {
		if p.product.builtins[entry.ID] {
			continue
		}
		item, err := productEntryItem(p.cache.base, category, entry)
		if err != nil {
			return nil, state, fmt.Errorf("%s/%s: installed state: %w", category, entry.ID, err)
		}
		if row, runnable := rows[entry.ID]; runnable {
			item.Active = item.Installed && entry.ID == active
			item.Metadata = p.product.metadata(row)
		}
		items = append(items, item)
		seen[entry.ID] = true
	}
	// A product that is installed but no longer listed, or whose source is out
	// of reach, stays usable and removable.
	for _, row := range local {
		if seen[row.ID] {
			continue
		}
		items = append(items, Item{
			ID: row.ID, Category: category, Name: row.ID,
			Version: row.Version, InstalledVersion: row.Version,
			Installed: true, Active: row.ID == active,
			Metadata: p.product.metadata(row),
		})
	}
	if registryErr != nil {
		state.Offline = true
	}
	return items, state, nil
}

func (s shellQMLProduct) metadata(row shellQMLIndexRow) map[string]any {
	return map[string]any{"entry": s.entry, "preview": row.Preview, "view": row.View}
}

func (p shellQMLProvider) Install(ctx context.Context, id string) error {
	if p.product.builtins[id] {
		return fmt.Errorf("the %s %s ships with the shell", id, p.product.noun)
	}
	entries, _, err := loadProductRegistry(ctx, p.cache, p.product.category.ID, false)
	if err != nil {
		return err
	}
	entry, err := findProductEntry(entries, id)
	if err != nil {
		return err
	}
	return installProduct(ctx, p.cache, p.product.category.ID, entry)
}

func (p shellQMLProvider) Remove(ctx context.Context, id string) error {
	if p.product.builtins[id] {
		return fmt.Errorf("the %s %s ships with the shell and is not removable", id, p.product.noun)
	}
	return removeProduct(ctx, p.product.category.ID, id)
}

func (s shellQMLProduct) indexPath() string {
	return filepath.Join(storeStateDir(), s.category.ID+".json")
}

func (s shellQMLProduct) rebuildIndex() ([]shellQMLIndexRow, error) {
	unlock, err := lockTree(storeTransactionLockPath())
	if err != nil {
		return nil, err
	}
	defer unlock()
	if err := recoverStoreTransactions(); err != nil {
		return nil, err
	}
	return s.rebuildIndexLocked()
}

func (s shellQMLProduct) writeIndexLocked() error {
	_, err := s.rebuildIndexLocked()
	return err
}

func (s shellQMLProduct) rebuildIndexLocked() ([]shellQMLIndexRow, error) {
	category := s.category.ID
	entries, err := os.ReadDir(filepath.Join(storeStateDir(), category))
	if os.IsNotExist(err) {
		entries = nil
	} else if err != nil {
		return nil, err
	}
	rows := make([]shellQMLIndexRow, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		id := strings.TrimSuffix(entry.Name(), ".json")
		// A broken receipt (bad name, unreadable, no entry, or drifted from
		// disk) is quarantined: it reads as installable again and a reinstall
		// heals it, while the rest of the index stays usable.
		if !productIDPattern.MatchString(id) || s.builtins[id] {
			continue
		}
		receipt, err := readReceipt(category, id)
		if err != nil || !receiptOwnsFile(receipt, s.entry) {
			continue
		}
		destination, _, err := productDestination(category, id)
		if err != nil {
			continue
		}
		if err := verifyInstalledReceipt(destination, receipt); err != nil {
			continue
		}
		row := shellQMLIndexRow{ID: id, Version: receipt.Version, View: s.views.path(id, receipt)}
		if s.preview != "" && receiptOwnsFile(receipt, s.preview) {
			row.Preview = s.preview
		}
		if err := s.views.ensure(id, row.View, receipt); err != nil {
			continue
		}
		rows = append(rows, row)
	}
	live := make(map[string]string, len(rows))
	for _, row := range rows {
		live[row.ID] = row.View
	}
	if err := s.views.prune(live); err != nil {
		return nil, err
	}
	raw, err := json.Marshal(rows)
	if err != nil {
		return nil, err
	}
	if err := atomicWrite(s.indexPath(), append(raw, '\n'), 0o600); err != nil {
		return nil, err
	}
	return rows, nil
}
