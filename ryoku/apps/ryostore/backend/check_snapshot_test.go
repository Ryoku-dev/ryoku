package main

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"
)

// TestRunCheckRefreshesTheLaunchSnapshot proves the startup probe keeps its live
// build as the launch snapshot. Installed state is local and never part of the
// revision, so without this a part installed while the store was closed (from
// Settings or a terminal) stayed frozen as absent on the next open.
func TestRunCheckRefreshesTheLaunchSnapshot(t *testing.T) {
	t.Setenv("RYOKU_EXTRAS_BASE", "")
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	providers := func(installed bool) []Provider {
		count := 0
		if installed {
			count = 1
		}
		return []Provider{fakeProvider{
			category: Category{ID: "bundles", Name: "Bundles", Count: 1},
			items: []Item{{
				ID:             "demo",
				Category:       "bundles",
				Name:           "Demo",
				Installed:      installed,
				InstalledCount: count,
				TotalCount:     1,
			}},
		}}
	}
	snapshot := func() Item {
		data, err := os.ReadFile(filepath.Join(extrasCacheDir(), "catalog.json"))
		if err != nil {
			t.Fatalf("launch snapshot not written: %v", err)
		}
		var cat Catalog
		if err := json.Unmarshal(data, &cat); err != nil {
			t.Fatalf("decode snapshot: %v", err)
		}
		if len(cat.Items) != 1 {
			t.Fatalf("snapshot items = %d, want 1", len(cat.Items))
		}
		return cat.Items[0]
	}

	if err := runCheck(io.Discard, providers(false)); err != nil {
		t.Fatalf("runCheck before install: %v", err)
	}
	if got := snapshot(); got.Installed || got.InstalledCount != 0 {
		t.Fatalf("snapshot before install = %+v, want absent", got)
	}

	if err := runCheck(io.Discard, providers(true)); err != nil {
		t.Fatalf("runCheck after install: %v", err)
	}
	if got := snapshot(); !got.Installed || got.InstalledCount != 1 {
		t.Fatalf("snapshot after install = %+v, want installed", got)
	}
}
