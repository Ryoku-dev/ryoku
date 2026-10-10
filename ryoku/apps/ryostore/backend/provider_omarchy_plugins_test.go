package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

func writeOmarchyShellStyle(t *testing.T, configHome, style string) {
	t.Helper()
	path := filepath.Join(configHome, "ryoku", "shell.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(map[string]any{"barStyle": style})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatal(err)
	}
}

func writeInstalledOmarchyPlugin(t *testing.T, configHome, dirName, id, version string) {
	t.Helper()
	dir := filepath.Join(configHome, "omarchy", "plugins", dirName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(map[string]any{"schemaVersion": 1, "id": id, "version": version})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "manifest.json"), raw, 0o644); err != nil {
		t.Fatal(err)
	}
}

func omarchyFixtureCatalogue() omarchyPluginCatalogue {
	return omarchyPluginCatalogue{
		GeneratedAt: "2026-10-07T17:05:51Z",
		Plugins: []omarchyPluginCatalogueItem{
			{
				ID: "clock.card", Name: "Clock Card", Description: "A clock for the bar.", Author: "Aki",
				Version: "2.0.0", Category: "Widgets", Tags: []string{"bar", "clock"}, Kind: "Bar widget",
				Initials: "CC", Accent: "#6750a4", Repo: "https://github.com/example/clock-card", InstallAvailable: true,
				License: "MIT", Stars: 12, PreviewImage: "assets/img/plugins/clock-detail.webp",
				PreviewThumbnail: "assets/img/plugins/clock-card.webp", VerificationStatus: "verified",
				ListedAt: "2026-10-01T00:00:00Z",
			},
			{
				ID: "notes.panel", Name: "Notes", Description: "A notes panel.", Author: "Ren",
				Version: "1.0.0", Category: "Productivity", Kind: "Panel", Repo: "https://github.com/example/notes-panel",
				InstallAvailable: true, License: "GPL-3.0", Stars: 30, PreviewImage: "https://cdn.example/notes.webp",
				VerificationStatus: "unverified", ListedAt: "2026-10-06T00:00:00Z",
			},
			{
				ID: "manual.suite", Name: "Manual Suite", Kind: "Suite", Repo: "https://github.com/example/manual",
				InstallAvailable: false, Stars: 100,
			},
		},
	}
}

func newOmarchyFixtureProvider(t *testing.T, serverURL, configHome string, run func(context.Context, string, ...string) ([]byte, error)) *omarchyPluginProvider {
	t.Helper()
	return &omarchyPluginProvider{
		client:     http.DefaultClient,
		catalogURL: serverURL,
		cacheDir:   filepath.Join(t.TempDir(), "cache"),
		configHome: configHome,
		now:        func() time.Time { return time.Date(2026, 10, 7, 18, 0, 0, 0, time.UTC) },
		run:        run,
	}
}

func TestOmarchyPluginProviderNormalizesInstalledStateAndFilters(t *testing.T) {
	configHome := t.TempDir()
	writeOmarchyShellStyle(t, configHome, "nomarchy")
	writeInstalledOmarchyPlugin(t, configHome, "checkout-name", "clock.card", "1.0.0")
	fixture := omarchyFixtureCatalogue()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("ETag", `"fixture-v1"`)
		_ = json.NewEncoder(w).Encode(fixture)
	}))
	defer server.Close()

	run := func(_ context.Context, name string, args ...string) ([]byte, error) {
		switch name + " " + strings.Join(args, " ") {
		case "omarchy plugin list --json":
			return []byte(`[{"id":"clock.card","enabled":true}]`), nil
		case "ryoku wm compat " + filepath.Join(configHome, "omarchy", "plugins", "checkout-name"):
			return []byte(`{"ok":false,"requires":["foreign-workspace-api"],"reason":"Needs an API the active window manager does not provide."}`), nil
		default:
			return nil, errors.New("unexpected command")
		}
	}
	provider := newOmarchyFixtureProvider(t, server.URL, configHome, run)
	items, state, err := provider.Load(context.Background(), true)
	if err != nil {
		t.Fatal(err)
	}
	if state.Offline {
		t.Fatal("live fixture unexpectedly reported offline")
	}
	if len(items) != 2 {
		t.Fatalf("installable filter returned %d items, want 2", len(items))
	}
	byID := itemsByID(items)
	clock := byID["clock.card"]
	if !clock.Installed || !clock.Enabled || !clock.UpdateAvailable || clock.InstalledVersion != "1.0.0" {
		t.Fatalf("installed state not normalized: %#v", clock)
	}
	if !clock.Unavailable || clock.UnavailableReason != "Needs an API the active window manager does not provide." {
		t.Fatalf("compatibility result not surfaced: %#v", clock)
	}
	if clock.Art != omarchyPluginSiteURL+"assets/img/plugins/clock-card.webp" {
		t.Fatalf("thumbnail URL = %q", clock.Art)
	}
	if !reflect.DeepEqual(clock.Screenshots, []string{omarchyPluginSiteURL + "assets/img/plugins/clock-detail.webp"}) {
		t.Fatalf("screenshots = %#v", clock.Screenshots)
	}
	if clock.Kind != "Bar widget" || clock.Summary != "Bar widget" || clock.License != "MIT" || clock.Stars != 12 || clock.VerificationStatus != "verified" || clock.Initials != "CC" || clock.Accent != "#6750a4" {
		t.Fatalf("marketplace metadata not normalized: %#v", clock)
	}
	if _, exists := byID["manual.suite"]; exists {
		t.Fatal("manual-only marketplace entry must not be listed")
	}
	if items[0].ID != "notes.panel" {
		t.Fatalf("popular order starts with %q, want notes.panel", items[0].ID)
	}
	newest := sortOmarchyPluginItems(items, "new")
	if newest[0].ID != "notes.panel" {
		t.Fatalf("new order starts with %q", newest[0].ID)
	}
	verified := sortOmarchyPluginItems(items, "verified")
	if verified[0].ID != "clock.card" {
		t.Fatalf("verified order starts with %q", verified[0].ID)
	}
}

func TestOmarchyPluginProviderHiddenOutsideNomarchy(t *testing.T) {
	configHome := t.TempDir()
	writeOmarchyShellStyle(t, configHome, "another-style")
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { requests++ }))
	defer server.Close()
	provider := newOmarchyFixtureProvider(t, server.URL, configHome, func(context.Context, string, ...string) ([]byte, error) {
		t.Fatal("inactive provider must not run plugin commands")
		return nil, nil
	})
	items, _, err := provider.Load(context.Background(), true)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 0 || requests != 0 {
		t.Fatalf("inactive provider returned %d items after %d requests", len(items), requests)
	}
}

func TestOmarchyPluginProviderUsesConditionalCacheAndOfflineFallback(t *testing.T) {
	configHome := t.TempDir()
	writeOmarchyShellStyle(t, configHome, "nomarchy")
	fixture := omarchyFixtureCatalogue()
	var mu sync.Mutex
	requests := 0
	fail := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		requests++
		if fail {
			http.Error(w, "offline", http.StatusServiceUnavailable)
			return
		}
		if r.Header.Get("If-None-Match") == `"fixture-v1"` {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.Header().Set("ETag", `"fixture-v1"`)
		_ = json.NewEncoder(w).Encode(fixture)
	}))
	defer server.Close()
	provider := newOmarchyFixtureProvider(t, server.URL, configHome, func(_ context.Context, name string, args ...string) ([]byte, error) {
		if name == "omarchy" && reflect.DeepEqual(args, []string{"plugin", "list", "--json"}) {
			return []byte(`[]`), nil
		}
		return nil, errors.New("unexpected command")
	})
	if _, _, err := provider.Load(context.Background(), true); err != nil {
		t.Fatal(err)
	}
	if _, state, err := provider.Load(context.Background(), true); err != nil || state.Offline {
		t.Fatalf("conditional refresh: state=%#v err=%v", state, err)
	}
	mu.Lock()
	fail = true
	mu.Unlock()
	items, state, err := provider.Load(context.Background(), true)
	if err != nil {
		t.Fatal(err)
	}
	if !state.Offline || state.CachedAt == "" || len(items) != 2 {
		t.Fatalf("offline fallback: items=%d state=%#v", len(items), state)
	}
	mu.Lock()
	gotRequests := requests
	mu.Unlock()
	if gotRequests != 3 {
		t.Fatalf("requests = %d, want 3", gotRequests)
	}
}

func TestOmarchyPluginProviderDispatchesInstallUpdateAndRemove(t *testing.T) {
	configHome := t.TempDir()
	writeOmarchyShellStyle(t, configHome, "nomarchy")
	fixture := omarchyFixtureCatalogue()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(fixture)
	}))
	defer server.Close()
	var commands [][]string
	run := func(_ context.Context, name string, args ...string) ([]byte, error) {
		commands = append(commands, append([]string{name}, args...))
		return nil, nil
	}
	provider := newOmarchyFixtureProvider(t, server.URL, configHome, run)
	if err := provider.Install(context.Background(), "clock.card"); err != nil {
		t.Fatal(err)
	}
	writeInstalledOmarchyPlugin(t, configHome, "clock.card", "clock.card", "1.0.0")
	if err := provider.Install(context.Background(), "clock.card"); err != nil {
		t.Fatal(err)
	}
	if err := provider.Remove(context.Background(), "clock.card"); err != nil {
		t.Fatal(err)
	}
	want := [][]string{
		{"omarchy", "plugin", "add", "https://github.com/example/clock-card", "--yes"},
		{"omarchy", "plugin", "update", "clock.card", "--yes"},
		{"omarchy", "plugin", "remove", "clock.card", "--yes"},
	}
	if !reflect.DeepEqual(commands, want) {
		t.Fatalf("commands = %#v, want %#v", commands, want)
	}
}
