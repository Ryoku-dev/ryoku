package main

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type profilesFixture struct {
	provider profilesProvider
	entry    ProductEntry
}

func newProfilesFixture(t *testing.T) profilesFixture {
	t.Helper()
	home := t.TempDir()
	config := filepath.Join(home, "config")
	data := filepath.Join(home, "data")
	state := filepath.Join(home, "state")
	cacheDir := filepath.Join(home, "cache")
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", config)
	t.Setenv("XDG_DATA_HOME", data)
	t.Setenv("XDG_STATE_HOME", state)
	t.Setenv("XDG_CACHE_HOME", cacheDir)
	stubPackageManager(t, "pacman")

	root := filepath.Join(home, "catalogue")
	productDir := filepath.Join(root, profilesCategory, "hacker")
	if err := os.MkdirAll(productDir, 0o755); err != nil {
		t.Fatal(err)
	}
	payload := []byte("{\"schema\":1,\"id\":\"hacker\"}\n")
	payloadHash := fmt.Sprintf("%x", sha256.Sum256(payload))
	manifest := ProductManifest{
		Schema:      1,
		ID:          "hacker",
		Category:    profilesCategory,
		Version:     "1.0.0",
		Destination: "ryoku/profiles/hacker",
		Files: []ProductFile{{
			Source: "profile.json", Destination: "profile.json", SHA256: payloadHash,
			Mode: "0644", Size: int64(len(payload)), Install: true,
		}},
	}
	manifestRaw, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	manifestHash := fmt.Sprintf("%x", sha256.Sum256(manifestRaw))
	entry := ProductEntry{
		ID: "hacker", Name: "Hacker", Version: "1.0.0", Path: "profiles/hacker",
		Author: "Ryoku Team", Summary: "a separate account for security labs",
		Description: "Tools and a separate desktop for security study.",
		Tags:        []string{"ctf"}, Screenshots: []string{}, Accent: "#668877", Surface: "#101010",
		Preview: "assets/preview.webp", Manifest: "manifest.json", ManifestSHA256: manifestHash,
		Official: true, Upstream: "https://github.com/ryoku-dev/ryostore-revamped",
		PackageManagers: []string{"pacman"}, PackageManagersReason: "Profiles need Arch or CachyOS.",
	}
	unofficial := entry
	unofficial.Official = false
	unofficial.ID = "community"
	unofficial.Name = "Community"
	unofficial.Path = "profiles/community"
	unofficial.ManifestSHA256 = strings.Repeat("0", 64)
	registryRaw, err := json.Marshal(map[string]any{
		"schema":   1,
		"profiles": []ProductEntry{entry, unofficial},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(productDir, "profile.json"), payload, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(productDir, "manifest.json"), manifestRaw, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, profilesCategory, "registry.json"), registryRaw, 0o644); err != nil {
		t.Fatal(err)
	}
	cache := newSourceCache(root, "", filepath.Join(cacheDir, "profiles-test"))
	return profilesFixture{provider: profilesProvider{cache: cache}, entry: entry}
}

func TestProfilesProviderListsOfficialProfilesAndCachesStatusPerLoad(t *testing.T) {
	fixture := newProfilesFixture(t)
	calls := 0
	fixture.provider.status = func(context.Context) map[string]bool {
		calls++
		return map[string]bool{"hacker": true, "community": true}
	}
	items, _, err := fixture.provider.Load(context.Background(), false)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].ID != "hacker" {
		t.Fatalf("items = %#v, want the official Hacker profile only", items)
	}
	if !items[0].Active {
		t.Fatal("Hacker profile is not active for its existing account")
	}
	if calls != 1 {
		t.Fatalf("profile status calls = %d, want 1", calls)
	}
	if err := fixture.provider.Install(context.Background(), "community"); err == nil {
		t.Fatal("unofficial profile was installable")
	}
}

func TestProfilesProviderInstallsDefinitionToContractPathWithReceipt(t *testing.T) {
	fixture := newProfilesFixture(t)
	if err := fixture.provider.Install(context.Background(), fixture.entry.ID); err != nil {
		t.Fatal(err)
	}
	installed := filepath.Join(dataHome(), "ryoku", "profiles", "hacker", "profile.json")
	if raw, err := os.ReadFile(installed); err != nil || string(raw) != "{\"schema\":1,\"id\":\"hacker\"}\n" {
		t.Fatalf("installed profile = %q, %v", raw, err)
	}
	receipt, err := readReceipt(profilesCategory, "hacker")
	if err != nil {
		t.Fatal(err)
	}
	if receipt.Destination != "ryoku/profiles/hacker" || receipt.Category != profilesCategory {
		t.Fatalf("receipt = %#v", receipt)
	}
	if _, err := os.Stat(receiptPath(profilesCategory, "hacker")); err != nil {
		t.Fatalf("profile receipt: %v", err)
	}
}

func TestProvidersGateProfilesCategory(t *testing.T) {
	hasProfiles := func(providers []Provider) bool {
		_, ok := providerFor(providers, profilesCategory)
		return ok
	}
	if hasProfiles(providersWithProfileGate(func() bool { return false })) {
		t.Fatal("stable channel registered profiles")
	}
	if !hasProfiles(providersWithProfileGate(func() bool { return true })) {
		t.Fatal("unstable channel hid profiles")
	}
}

func TestProfileChannelGateHidesFailuresAndStableChannels(t *testing.T) {
	for name, test := range map[string]struct {
		body string
		want bool
	}{
		"unstable": {"printf '%s\\n' '{\"stable\":false}'\n", true},
		"stable":   {"printf '%s\\n' '{\"stable\":true}'\n", false},
		"invalid":  {"printf '%s\\n' 'not json'\n", false},
		"failure":  {"exit 1\n", false},
	} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			command := filepath.Join(dir, "ryoku")
			if err := os.WriteFile(command, []byte("#!/bin/sh\n"+test.body), 0o755); err != nil {
				t.Fatal(err)
			}
			t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
			if got := profileChannelEnabled(); got != test.want {
				t.Fatalf("profileChannelEnabled() = %v, want %v", got, test.want)
			}
		})
	}
}

func TestRevampedSourceHasItsOwnBaseAndCacheNamespace(t *testing.T) {
	root := filepath.Join(t.TempDir(), "revamped")
	t.Setenv("XDG_CACHE_HOME", filepath.Join(t.TempDir(), "cache"))
	t.Setenv("RYOSTORE_REVAMPED_BASE", root)
	cache := newRevampedCache()
	if cache.base != root {
		t.Fatalf("revamped base = %q, want %q", cache.base, root)
	}
	if local, ok := localBase(cache.base); !ok || local != root {
		t.Fatalf("localBase(%q) = %q, %v", cache.base, local, ok)
	}
	wantRoot := filepath.Join(xdgCacheHome(), "ryoku", "ryostore-revamped", "sources")
	if !strings.HasPrefix(cache.dir, wantRoot+string(filepath.Separator)) {
		t.Fatalf("revamped cache = %q, want source under %q", cache.dir, wantRoot)
	}
	if strings.HasPrefix(cache.dir, filepath.Join(xdgCacheHome(), "ryoku", "ryostore", "sources")+string(filepath.Separator)) {
		t.Fatalf("revamped cache reused the classic store namespace: %q", cache.dir)
	}
}
