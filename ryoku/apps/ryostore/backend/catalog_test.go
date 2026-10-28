package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// fakeProvider stands in for a real catalogue source so BuildCatalog can be
// exercised without network or disk: it returns canned items, source state, and
// an optional error.
type fakeProvider struct {
	category Category
	items    []Item
	state    SourceState
	err      error
}

func (f fakeProvider) Category() Category { return f.category }
func (f fakeProvider) Load(context.Context, bool) ([]Item, SourceState, error) {
	return f.items, f.state, f.err
}
func (f fakeProvider) Install(context.Context, string) error { return nil }

func TestBuildCatalogIsolatesProviderFailure(t *testing.T) {
	providers := []Provider{
		fakeProvider{category: Category{ID: "plugins", Name: "Plugins", Group: "extend"}, items: []Item{{ID: "market", Category: "plugins", Installed: true}}},
		fakeProvider{category: Category{ID: "locks", Name: "Lockscreens", Group: "wear"}, err: errors.New("offline")},
	}
	got := BuildCatalog(context.Background(), providers, false)
	if len(got.Items) != 1 || got.Items[0].ID != "market" {
		t.Fatalf("items = %#v", got.Items)
	}
	if got.Categories[0].InstalledCount != 1 {
		t.Fatalf("installed = %d", got.Categories[0].InstalledCount)
	}
	if got.Categories[1].Error != "offline" {
		t.Fatalf("error = %q", got.Categories[1].Error)
	}
}

// TestBuildCatalogCountsAndOffline feeds one active, one enabled, one partially
// installed bundle, and one available item through a single offline source, and
// asserts the derived category counts and the catalogue-wide offline flag.
func TestBuildCatalogCountsAndOffline(t *testing.T) {
	providers := []Provider{
		fakeProvider{
			category: Category{ID: "rices", Name: "Rices", Group: "wear"},
			items: []Item{
				{ID: "worn", Category: "rices", Installed: true, Active: true},
				{ID: "running", Category: "rices", Installed: true, Enabled: true},
				{ID: "starter", Category: "rices", InstalledCount: 2, TotalCount: 3},
				{ID: "browse", Category: "rices"},
			},
			state: SourceState{Offline: true, CachedAt: "2026-07-01T00:00:00Z"},
		},
	}
	got := BuildCatalog(context.Background(), providers, false)
	if len(got.Items) != 4 {
		t.Fatalf("items = %d, want 4", len(got.Items))
	}
	cat := got.Categories[0]
	if cat.Count != 4 {
		t.Fatalf("count = %d, want 4", cat.Count)
	}
	if cat.InstalledCount != 3 {
		t.Fatalf("installedCount = %d, want 3 (active, enabled, partial bundle)", cat.InstalledCount)
	}
	if !cat.Offline || cat.CachedAt != "2026-07-01T00:00:00Z" {
		t.Fatalf("offline state not carried onto category: %+v", cat)
	}
	if got.Offline {
		t.Fatalf("a source serving cached items must not flag the store offline: %+v", got)
	}
}

func TestBuildCatalogOfflineOnlyWithoutRemoteItems(t *testing.T) {
	// Cached remote items keep the store usable even when their source is down.
	serving := BuildCatalog(context.Background(), []Provider{fakeProvider{
		category: Category{ID: "rices", Name: "Rices", Group: "wear"},
		items:    []Item{{ID: "cached", Category: "rices"}},
		state:    SourceState{Offline: true},
	}}, false)
	if serving.Offline {
		t.Fatalf("serving cached items must keep the store online: %+v", serving)
	}

	// Built-in bars are local inventory, not a remote catalogue or cache. A
	// failed remote source alongside them must still report the store offline.
	builtinsOnly := BuildCatalog(context.Background(), []Provider{
		fakeProvider{
			category: Category{ID: "barstyles", Name: "Bars", Group: "wear"},
			items: []Item{{
				ID: "orpheus", Category: "barstyles",
				Metadata: map[string]any{"core": true},
			}},
		},
		fakeProvider{
			category: Category{ID: "rices", Name: "Rices", Group: "wear"},
			err:      errors.New("source unreachable"),
		},
	}, false)
	if !builtinsOnly.Offline {
		t.Fatalf("built-ins without a remote catalogue must report offline: %+v", builtinsOnly)
	}
}

type countingProvider struct {
	fakeProvider
	loads *int32
}

func (c countingProvider) Load(ctx context.Context, refresh bool) ([]Item, SourceState, error) {
	atomic.AddInt32(c.loads, 1)
	return c.fakeProvider.Load(ctx, refresh)
}

type failWriter struct{}

func (failWriter) Write([]byte) (int, error) { return 0, errors.New("write failed") }

// TestRunCatalogCategoryResolvesBeforeLoad proves --category resolves its
// provider before any source loads: a valid category loads only its own
// provider, an unknown or empty category loads none and errors, and no filter
// loads all.
func TestRunCatalogCategoryResolvesBeforeLoad(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	var pluginLoads, riceLoads int32
	provs := []Provider{
		countingProvider{fakeProvider{category: Category{ID: "plugins", Name: "Plugins"}, items: []Item{{ID: "market", Category: "plugins"}}}, &pluginLoads},
		countingProvider{fakeProvider{category: Category{ID: "rices", Name: "Rices"}, items: []Item{{ID: "nord", Category: "rices"}}}, &riceLoads},
	}
	reset := func() { atomic.StoreInt32(&pluginLoads, 0); atomic.StoreInt32(&riceLoads, 0) }
	loads := func() (int32, int32) { return atomic.LoadInt32(&pluginLoads), atomic.LoadInt32(&riceLoads) }

	var buf bytes.Buffer
	if err := runCatalog(&buf, provs, []string{"--category", "plugins"}); err != nil {
		t.Fatalf("catalog --category plugins: %v", err)
	}
	if p, r := loads(); p != 1 || r != 0 {
		t.Fatalf("valid category loads: plugins=%d rices=%d, want 1/0", p, r)
	}
	var got Catalog
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got.Categories) != 1 || got.Categories[0].ID != "plugins" || len(got.Items) != 1 {
		t.Fatalf("filtered catalogue = %+v", got)
	}

	reset()
	if err := runCatalog(io.Discard, provs, []string{"--category", "bundles"}); err == nil {
		t.Fatal("unknown category must error")
	}
	if p, r := loads(); p != 0 || r != 0 {
		t.Fatalf("unknown category must load nothing: plugins=%d rices=%d", p, r)
	}

	reset()
	if err := runCatalog(io.Discard, provs, []string{"--category", ""}); err == nil {
		t.Fatal("explicit empty category id must error")
	}
	if p, r := loads(); p != 0 || r != 0 {
		t.Fatalf("empty category must load nothing: plugins=%d rices=%d", p, r)
	}

	reset()
	if err := runCatalog(io.Discard, provs, nil); err != nil {
		t.Fatalf("catalog: %v", err)
	}
	if p, r := loads(); p != 1 || r != 1 {
		t.Fatalf("no filter loads all: plugins=%d rices=%d, want 1/1", p, r)
	}
}

// TestRunCatalogPropagatesWriteError proves a failed JSON write is surfaced
// rather than swallowed into a successful exit.
func TestRunCatalogPropagatesWriteError(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	if err := runCatalog(failWriter{}, nil, nil); err == nil || !strings.Contains(err.Error(), "write failed") {
		t.Fatalf("runCatalog write error = %v, want it propagated", err)
	}
}

// TestRunCatalogSnapshotServesThenRefresh proves the full catalogue is
// snapshotted: the first launch builds and caches it, a later launch answers
// from the snapshot without touching any provider, and --refresh rebuilds live.
func TestRunCatalogSnapshotServesThenRefresh(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	var loads int32
	provs := []Provider{
		countingProvider{fakeProvider{category: Category{ID: "rices", Name: "Rices"}, items: []Item{{ID: "nord", Category: "rices"}}}, &loads},
	}

	var first bytes.Buffer
	if err := runCatalog(&first, provs, nil); err != nil {
		t.Fatalf("first launch: %v", err)
	}
	if got := atomic.LoadInt32(&loads); got != 1 {
		t.Fatalf("first launch loads = %d, want 1", got)
	}

	var second bytes.Buffer
	if err := runCatalog(&second, provs, nil); err != nil {
		t.Fatalf("cached launch: %v", err)
	}
	if got := atomic.LoadInt32(&loads); got != 1 {
		t.Fatalf("cached launch must not load providers: loads = %d, want 1", got)
	}
	if second.String() != first.String() {
		t.Fatalf("cached launch output drifted from the snapshot")
	}

	if err := runCatalog(io.Discard, provs, []string{"--refresh"}); err != nil {
		t.Fatalf("refresh: %v", err)
	}
	if got := atomic.LoadInt32(&loads); got != 2 {
		t.Fatalf("refresh must rebuild live: loads = %d, want 2", got)
	}
}

func TestRunCatalogRebuildsWhenProviderAdded(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	var originalLoads, addedLoads int32
	original := countingProvider{
		fakeProvider{category: Category{ID: "rices"}, items: []Item{{ID: "demo", Category: "rices"}}},
		&originalLoads,
	}
	if err := runCatalog(io.Discard, []Provider{original}, nil); err != nil {
		t.Fatal(err)
	}
	added := countingProvider{
		fakeProvider{category: Category{ID: "flatpak"}, items: []Item{{ID: "org.example.App", Category: "flatpak"}}},
		&addedLoads,
	}
	var out bytes.Buffer
	if err := runCatalog(&out, []Provider{original, added}, nil); err != nil {
		t.Fatal(err)
	}
	var cat Catalog
	if err := json.Unmarshal(out.Bytes(), &cat); err != nil {
		t.Fatal(err)
	}
	if len(cat.Categories) != 2 || originalLoads != 2 || addedLoads != 1 {
		t.Fatalf("new provider was hidden by old snapshot: categories=%d, loads=%d/%d", len(cat.Categories), originalLoads, addedLoads)
	}
}

// TestRunCatalogDoesNotSnapshotEmpty proves a catalogue with no items is never
// cached, so a launch after a failed fetch probes live again instead of pinning
// an empty store.
func TestRunCatalogDoesNotSnapshotEmpty(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	var loads int32
	provs := []Provider{
		countingProvider{fakeProvider{category: Category{ID: "rices", Name: "Rices"}, err: errors.New("offline")}, &loads},
	}
	if err := runCatalog(io.Discard, provs, nil); err != nil {
		t.Fatalf("first: %v", err)
	}
	if err := runCatalog(io.Discard, provs, nil); err != nil {
		t.Fatalf("second: %v", err)
	}
	if got := atomic.LoadInt32(&loads); got != 2 {
		t.Fatalf("empty catalogue must not be cached: loads = %d, want 2", got)
	}
}

// TestRunCatalogRebuildsOfflineSnapshot proves a snapshot written while a source
// was failing is not served forever: the next non-refresh launch rebuilds live
// and comes back online once the source is reachable, so the store self-heals
// instead of pinning "offline" on every launch until a manual refresh.
func TestRunCatalogRebuildsOfflineSnapshot(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	// A snapshot flagged offline (however it arose) must not be served forever:
	// the next non-refresh launch rebuilds live and comes back online, so the
	// store self-heals instead of pinning "offline" until a manual refresh.
	snapshot := filepath.Join(extrasCacheDir(), "catalog.json")
	if err := os.MkdirAll(filepath.Dir(snapshot), 0o755); err != nil {
		t.Fatal(err)
	}
	seed := []byte(`{"offline":true,"categories":[],"items":[{"id":"demo","category":"rices"}]}` + "\n")
	if err := os.WriteFile(snapshot, seed, 0o644); err != nil {
		t.Fatal(err)
	}
	var loads int32
	healthy := []Provider{countingProvider{
		fakeProvider{category: Category{ID: "rices", Name: "Rices", Group: "wear"}, items: []Item{{ID: "demo", Category: "rices"}}},
		&loads,
	}}
	var buf bytes.Buffer
	if err := runCatalog(&buf, healthy, nil); err != nil {
		t.Fatalf("relaunch: %v", err)
	}
	if got := atomic.LoadInt32(&loads); got != 1 {
		t.Fatalf("offline snapshot served instead of rebuilt: loads = %d, want 1", got)
	}
	var cat Catalog
	if err := json.Unmarshal(buf.Bytes(), &cat); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if cat.Offline {
		t.Fatalf("store stayed offline after a healthy relaunch")
	}
}

// TestDispatchErrors covers the argument and category errors the CLI must
// surface: every one returns a non-nil error carrying a useful phrase.
func TestDispatchErrors(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want string
	}{
		{"no command", nil, "no command"},
		{"unknown command", []string{"wibble"}, "unknown command"},
		{"install too few args", []string{"install", "plugins"}, "install needs"},
		{"install unknown category", []string{"install", "nope", "x"}, "unknown category"},
		{"catalog unknown flag", []string{"catalog", "--wat"}, "unknown catalog flag"},
		{"catalog category needs value", []string{"catalog", "--category"}, "needs"},
		{"catalog unknown category", []string{"catalog", "--category", "nope"}, "unknown category"},
		{"catalog empty category", []string{"catalog", "--category", ""}, "non-empty"},
		{"internal needs subcommand", []string{"internal"}, "internal needs"},
		{"internal unknown subcommand", []string{"internal", "frobnicate"}, "unknown internal command"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := dispatch(tc.args)
			if err == nil {
				t.Fatalf("args %v: want an error", tc.args)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("args %v: error %q, want substring %q", tc.args, err, tc.want)
			}
		})
	}
}

func assertKeys(t *testing.T, obj map[string]any, what string, keys ...string) {
	t.Helper()
	for _, k := range keys {
		if _, ok := obj[k]; !ok {
			t.Fatalf("%s JSON missing key %q", what, k)
		}
	}
}

// TestCatalogJSONContract locks the serialized shape the QML boundary and every
// later provider consume: each Category and Item lower-camel key is present, and
// an empty catalogue renders non-null [] arrays rather than null.
func TestCatalogJSONContract(t *testing.T) {
	cat := Catalog{
		GeneratedAt: "2026-07-29T00:00:00Z",
		Offline:     true,
		Categories: []Category{{
			ID: "plugins", Name: "Plugins", Group: "extend", Description: "d",
			Count: 2, InstalledCount: 1, Offline: true, CachedAt: "2026-07-01T00:00:00Z", Error: "boom",
		}},
		Items: []Item{{
			ID: "market", Category: "plugins", Name: "Market", Summary: "s", Description: "d",
			Art: "a.png", Author: "me", Version: "1.2.3", Compatibility: "ryoku>=1",
			Screenshots: []string{"s.png"}, Tags: []string{"tag"},
			Installed: true, Active: true, Enabled: true,
			InstalledCount: 1, TotalCount: 3, UpdateAvailable: true,
			Metadata: map[string]any{"slug": "market"},
		}},
	}
	b, err := json.Marshal(cat)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	assertKeys(t, m, "catalog", "generatedAt", "offline", "categories", "items")
	assertKeys(t, m["categories"].([]any)[0].(map[string]any), "category",
		"id", "name", "group", "description", "count", "installedCount", "offline", "cachedAt", "error")
	assertKeys(t, m["items"].([]any)[0].(map[string]any), "item",
		"id", "category", "name", "summary", "description", "art", "author", "version",
		"compatibility", "screenshots", "tags", "installed", "active", "enabled",
		"installedCount", "totalCount", "updateAvailable", "metadata")

	empty, err := json.Marshal(BuildCatalog(context.Background(), nil, false))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(empty), `"categories":[]`) || !strings.Contains(string(empty), `"items":[]`) {
		t.Fatalf("empty catalogue must render [] not null: %s", empty)
	}
}

// blockingProvider parks in Load until released, so a test can require every
// provider to enter Load before any returns.
type blockingProvider struct {
	category Category
	item     Item
	started  *sync.WaitGroup
	release  chan struct{}
}

func (b blockingProvider) Category() Category { return b.category }
func (b blockingProvider) Load(context.Context, bool) ([]Item, SourceState, error) {
	b.started.Done()
	<-b.release
	return []Item{b.item}, SourceState{}, nil
}
func (b blockingProvider) Install(context.Context, string) error { return nil }

// TestBuildCatalogRunsProvidersConcurrently proves the fan-out is genuinely
// concurrent (a serialized loader would deadlock this) and that categories and
// items keep provider order.
func TestBuildCatalogRunsProvidersConcurrently(t *testing.T) {
	const n = 4
	var started sync.WaitGroup
	started.Add(n)
	release := make(chan struct{})
	provs := make([]Provider, n)
	for i := range n {
		id := fmt.Sprintf("cat%d", i)
		provs[i] = blockingProvider{
			category: Category{ID: id, Name: id},
			item:     Item{ID: fmt.Sprintf("item%d", i), Category: id},
			started:  &started,
			release:  release,
		}
	}

	done := make(chan Catalog, 1)
	go func() { done <- BuildCatalog(context.Background(), provs, false) }()

	allStarted := make(chan struct{})
	go func() { started.Wait(); close(allStarted) }()
	select {
	case <-allStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("providers did not all enter Load: fan-out is serialized")
	}
	close(release)

	var got Catalog
	select {
	case got = <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("BuildCatalog did not finish")
	}
	if len(got.Categories) != n || len(got.Items) != n {
		t.Fatalf("got %d categories, %d items, want %d each", len(got.Categories), len(got.Items), n)
	}
	for i := range n {
		if got.Categories[i].ID != fmt.Sprintf("cat%d", i) {
			t.Fatalf("category %d = %q, provider order not preserved", i, got.Categories[i].ID)
		}
		if got.Items[i].ID != fmt.Sprintf("item%d", i) {
			t.Fatalf("item %d = %q, provider order not preserved", i, got.Items[i].ID)
		}
	}
}

// TestCatalogJSONZeroStateContract proves the required state scalars serialize
// even at their zero value, so a consumer always sees an explicit false/0 rather
// than a missing key, while genuinely optional fields stay omitted. A stray
// omitempty on a required field would drop its key here even though the
// non-zero contract test stays green.
func TestCatalogJSONZeroStateContract(t *testing.T) {
	top, err := json.Marshal(Catalog{Categories: []Category{}, Items: []Item{}})
	if err != nil {
		t.Fatal(err)
	}
	var tm map[string]any
	if err := json.Unmarshal(top, &tm); err != nil {
		t.Fatal(err)
	}
	assertKeys(t, tm, "catalog", "generatedAt", "offline", "categories", "items")

	item, err := json.Marshal(Item{})
	if err != nil {
		t.Fatal(err)
	}
	var im map[string]any
	if err := json.Unmarshal(item, &im); err != nil {
		t.Fatal(err)
	}
	assertKeys(t, im, "item", "id", "category", "name",
		"installed", "active", "enabled", "updateAvailable", "installedCount", "totalCount")
	for _, k := range []string{"summary", "description", "art", "author", "version", "compatibility", "screenshots", "tags", "metadata"} {
		if _, ok := im[k]; ok {
			t.Fatalf("optional item key %q must be omitted at zero value, got %s", k, item)
		}
	}

	cat, err := json.Marshal(Category{})
	if err != nil {
		t.Fatal(err)
	}
	var cm map[string]any
	if err := json.Unmarshal(cat, &cm); err != nil {
		t.Fatal(err)
	}
	assertKeys(t, cm, "category", "id", "name", "group", "description", "count", "installedCount")
	for _, k := range []string{"offline", "cachedAt", "error"} {
		if _, ok := cm[k]; ok {
			t.Fatalf("optional category key %q must be omitted at zero value, got %s", k, cat)
		}
	}
}
