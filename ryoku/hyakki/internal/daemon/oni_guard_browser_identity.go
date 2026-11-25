package daemon

import (
	"strings"

	"ryoku-rashin/internal/reflex/launch"
)

// guardBrowserIdentity protects whole browser windows independently of feed
// words. Installed applications use the existing desktop-entry category/MIME
// classifier; the small fallback covers common browser window identities even
// without an entry. Hands' deliberately unique class is always a browser too.
// Only its authorized target may be closed, by the separate pipe-backed path.
func guardBrowserIdentity(app string, catalog *launch.Catalog) bool {
	id := strings.TrimSuffix(strings.TrimSpace(app), ".desktop")
	if catalog != nil {
		if entry, ok := catalog.ByID(id + ".desktop"); ok && entry.Browser() {
			return true
		}
		if entry, ok := catalog.Lookup(id); ok && entry.Browser() {
			return true
		}
	}
	id = strings.ToLower(id)
	if strings.HasPrefix(id, "hyakki-hands-") {
		return true
	}
	if at := strings.LastIndexByte(id, '.'); at >= 0 {
		id = id[at+1:]
	}
	switch id {
	case "browser", "firefox", "firefox-bin", "chromium", "chromium-browser", "chrome",
		"google-chrome", "google-chrome-stable", "google-chrome-beta", "google-chrome-unstable",
		"brave", "brave-browser", "vivaldi", "vivaldi-stable", "opera", "epiphany", "librewolf", "zen", "zen-browser":
		return true
	}
	return false
}
