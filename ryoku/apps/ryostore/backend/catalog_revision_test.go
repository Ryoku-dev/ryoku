package main

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestCatalogRevisionIgnoresOrderAndVolatileState(t *testing.T) {
	items := []Item{
		{Category: "rices", ID: "a", Version: "1", ManifestSHA256: "x"},
		{Category: "plugins", ID: "b", Version: "2", ManifestSHA256: "y"},
	}
	base := catalogRevision(Catalog{Items: items})

	reordered := catalogRevision(Catalog{Items: []Item{items[1], items[0]}})
	if reordered != base {
		t.Fatalf("item order must not change revision: %s vs %s", reordered, base)
	}

	volatile := catalogRevision(Catalog{
		GeneratedAt: "2099-01-01T00:00:00Z",
		Offline:     true,
		Items: []Item{
			{Category: "rices", ID: "a", Version: "1", ManifestSHA256: "x", Installed: true, Active: true},
			{Category: "plugins", ID: "b", Version: "2", ManifestSHA256: "y", UpdateAvailable: true},
		},
	})
	if volatile != base {
		t.Fatal("generatedAt, offline, and install state must not change revision")
	}
}

// A pause or resume is catalogue state the user acts on -- it blocks installs and
// shows as Under construction -- so it must move the revision. Otherwise a store
// that cached the paused catalogue keeps serving it: no refresh dot, and no way
// to reach the resume.
func TestCatalogRevisionTracksDownloadPause(t *testing.T) {
	active := catalogRevision(Catalog{Items: []Item{
		{Category: "barstyles", ID: "ricelin", Version: "1.0.0", ManifestSHA256: "x"},
	}})
	paused := catalogRevision(Catalog{Items: []Item{
		{Category: "barstyles", ID: "ricelin", Version: "1.0.0", ManifestSHA256: "x",
			DownloadPaused: true, DownloadPauseReason: "Has known issues. The developer is working on fixes."},
	}})
	if paused == active {
		t.Fatal("pausing an item must change the revision")
	}
	if catalogRevision(Catalog{Items: []Item{
		{Category: "barstyles", ID: "ricelin", Version: "1.0.0", ManifestSHA256: "x", DownloadPaused: true},
	}}) == paused {
		t.Fatal("a changed pause reason must change the revision")
	}
	// The reason alone, without the gate, is not part of what the catalogue offers.
	if catalogRevision(Catalog{Items: []Item{
		{Category: "barstyles", ID: "ricelin", Version: "1.0.0", ManifestSHA256: "x",
			DownloadPauseReason: "Has known issues. The developer is working on fixes."},
	}}) != active {
		t.Fatal("a reason without the pause flag must not change the revision")
	}
}

func TestCatalogRevisionChangesOnRealContentChange(t *testing.T) {
	base := catalogRevision(Catalog{Items: []Item{
		{Category: "rices", ID: "a", Version: "1", ManifestSHA256: "x"},
	}})
	cases := map[string]Catalog{
		"version bump":    {Items: []Item{{Category: "rices", ID: "a", Version: "2", ManifestSHA256: "x"}}},
		"manifest digest": {Items: []Item{{Category: "rices", ID: "a", Version: "1", ManifestSHA256: "z"}}},
		"new item": {Items: []Item{
			{Category: "rices", ID: "a", Version: "1", ManifestSHA256: "x"},
			{Category: "decors", ID: "c", Version: "1"},
		}},
	}
	for name, cat := range cases {
		if catalogRevision(cat) == base {
			t.Fatalf("%s must change the revision", name)
		}
	}
}

// Provenance links are display-only, but they are the whole point of a catalogue
// update that changes nothing else: an item that gains (or renames) its upstream,
// or adds a community invite, must move the revision so a warm cache offers the
// refresh instead of serving a snapshot with no links to show.
func TestCatalogRevisionTracksProvenance(t *testing.T) {
	const home = "https://github.com/neur0map/MJ-widgets"
	bare := catalogRevision(Catalog{Items: []Item{
		{Category: "plugins", ID: "awe-clock", Version: "1.0.0", ManifestSHA256: "x"},
	}})
	upstream := catalogRevision(Catalog{Items: []Item{
		{Category: "plugins", ID: "awe-clock", Version: "1.0.0", ManifestSHA256: "x", Upstream: home},
	}})
	if upstream == bare {
		t.Fatal("an item gaining an upstream must change the revision")
	}
	if catalogRevision(Catalog{Items: []Item{
		{Category: "plugins", ID: "awe-clock", Version: "1.0.0", ManifestSHA256: "x",
			Upstream: "https://github.com/other/MJ-widgets"},
	}}) == upstream {
		t.Fatal("a changed upstream must change the revision")
	}
	if catalogRevision(Catalog{Items: []Item{
		{Category: "plugins", ID: "awe-clock", Version: "1.0.0", ManifestSHA256: "x", Upstream: home,
			Discord: "https://discord.gg/8KjBmUEyKA"},
	}}) == upstream {
		t.Fatal("an item gaining a discord invite must change the revision")
	}
}

func TestSeenRevisionRoundTrip(t *testing.T) {
	t.Setenv("RYOKU_EXTRAS_BASE", "")
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	if got := readSeenRevision(); got != "" {
		t.Fatalf("no seen revision expected initially, got %q", got)
	}
	writeSeenRevision("deadbeef")
	if got := readSeenRevision(); got != "deadbeef" {
		t.Fatalf("seen revision = %q, want deadbeef", got)
	}
}

func TestRunCheckFlagsUpdateOnlyPastSeen(t *testing.T) {
	t.Setenv("RYOKU_EXTRAS_BASE", "")
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	provsV1 := []Provider{fakeProvider{
		category: Category{ID: "rices", Name: "Rices"},
		items:    []Item{{ID: "a", Category: "rices", Version: "1"}},
	}}
	decode := func(provs []Provider) struct {
		Revision        string `json:"revision"`
		UpdateAvailable bool   `json:"updateAvailable"`
		Offline         bool   `json:"offline"`
	} {
		var buf bytes.Buffer
		if err := runCheck(&buf, provs); err != nil {
			t.Fatalf("runCheck: %v", err)
		}
		var out struct {
			Revision        string `json:"revision"`
			UpdateAvailable bool   `json:"updateAvailable"`
			Offline         bool   `json:"offline"`
		}
		if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
			t.Fatalf("decode: %v", err)
		}
		return out
	}

	// No baseline yet: never claim an update the user has never acknowledged.
	first := decode(provsV1)
	if first.UpdateAvailable {
		t.Fatal("no update should be flagged before any revision is seen")
	}

	// Acknowledge the current revision: check now reports no update.
	writeSeenRevision(first.Revision)
	if decode(provsV1).UpdateAvailable {
		t.Fatal("no update when the live revision equals the seen revision")
	}

	// Upstream advances past the seen revision: the dot lights.
	provsV2 := []Provider{fakeProvider{
		category: Category{ID: "rices", Name: "Rices"},
		items:    []Item{{ID: "a", Category: "rices", Version: "2"}},
	}}
	if !decode(provsV2).UpdateAvailable {
		t.Fatal("update must be flagged when the revision advances past seen")
	}
}
