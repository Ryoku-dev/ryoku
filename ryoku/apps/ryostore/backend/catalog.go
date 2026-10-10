package main

import (
	"context"
	"sync"
	"time"
)

// Provider is one product catalogue. Load fetches and normalizes the category's
// items, reporting its source state or an error that stays isolated to that
// category. Install and Remove change an item without activating or applying it.
// refresh bypasses any fresh in-process cache.
type Provider interface {
	Category() Category
	Load(ctx context.Context, refresh bool) ([]Item, SourceState, error)
	Install(ctx context.Context, id string) error
	Remove(ctx context.Context, id string) error
}

// providers is the ordered catalogue registry in rail order. Each later task
// appends its provider here; BuildCatalog preserves this order. All providers
// share one cache so a single probe fetches each source once.
func providers() []Provider {
	c := newCache()
	return []Provider{
		newLockProvider(c),
		newRiceProvider(c),
		newColorschemeProvider(c),
		newBarProvider(c),
		newFastfetchProvider(c),
		newRyotunesSkinsProvider(c),
		newVesktopThemesProvider(c),
		pluginProvider{cache: c},
		newOmarchyPluginProvider(),
		bundleProvider{cache: c, status: defaultBundleStatus, launch: launchBundleInstall},
		newDecorProvider(c),
		newLauncherImageProvider(c),
		newFastfetchEmblemProvider(c),
	}
}

// owned reports whether the item is present on the machine: fully installed, or
// a partially installed bundle. A category's InstalledCount tallies its owned
// items.
func (it *Item) owned() bool { return it.Installed || it.InstalledCount > 0 }

// BuildCatalog probes every provider concurrently and folds the results into one
// catalogue in provider order. Each goroutine writes only its own result slot,
// so a slow or failing source neither blocks nor aborts the others: its error
// lands in that one category and the rest still render. Counts are derived after
// all sources settle.
func BuildCatalog(ctx context.Context, provs []Provider, refresh bool) Catalog {
	type result struct {
		items []Item
		state SourceState
		err   error
	}
	results := make([]result, len(provs))
	var wg sync.WaitGroup
	for i, p := range provs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			items, state, err := p.Load(ctx, refresh)
			results[i] = result{items, state, err}
		}()
	}
	wg.Wait()

	cat := Catalog{
		GeneratedAt:   time.Now().UTC().Format(time.RFC3339),
		WindowManager: runningWindowManager(),
		Categories:    make([]Category, len(provs)),
		Items:         []Item{},
	}
	anyFailure := false
	for i, p := range provs {
		c := p.Category()
		r := results[i]
		c.Offline = r.state.Offline
		c.CachedAt = r.state.CachedAt
		if r.err != nil {
			c.Error = r.err.Error()
		} else {
			c.Count = len(r.items)
			settings := hasSettingsSection(c.ID)
			for j := range r.items {
				r.items[j].HasSettings = settings
				if r.items[j].owned() {
					c.InstalledCount++
				}
			}
			cat.Items = append(cat.Items, r.items...)
		}
		if c.Offline || c.Error != "" {
			anyFailure = true
		}
		cat.Categories[i] = c
	}
	// The store is "offline" to the user only when a source failed AND there is
	// nothing at all to show. Serving cached or partial data is normal
	// resilience, not an offline state, so a stale-but-reachable source (or one
	// category falling back to cache) never blanks the store or trips the banner.
	cat.Offline = anyFailure && len(cat.Items) == 0
	return cat
}

// providerFor returns the provider owning category id, or ok=false when none
// does, so an unknown category is rejected before any source loads.
func providerFor(provs []Provider, id string) (Provider, bool) {
	for _, p := range provs {
		if p.Category().ID == id {
			return p, true
		}
	}
	return nil, false
}
