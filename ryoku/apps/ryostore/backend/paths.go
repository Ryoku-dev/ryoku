package main

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// The catalogue lives in the Ryostore repo (ryoku-dev/ryostore), fetched from
// raw GitHub. RYOSTORE_BASE overrides the base for a fork or a local tree under
// test; the former RYOKU_EXTRAS_BASE is still honoured for one release so a dev
// with it already exported keeps working.
const defaultExtrasBase = "https://raw.githubusercontent.com/ryoku-dev/ryostore/main"

const defaultRevampedBase = "https://raw.githubusercontent.com/ryoku-dev/ryostore-revamped/main"

// extrasBaseOverride returns the environment source override, preferring the
// current RYOSTORE_BASE and falling back to the legacy RYOKU_EXTRAS_BASE.
func extrasBaseOverride() string {
	if b := os.Getenv("RYOSTORE_BASE"); b != "" {
		return b
	}
	return os.Getenv("RYOKU_EXTRAS_BASE")
}

func extrasBase() string {
	if b := extrasBaseOverride(); b != "" {
		return strings.TrimRight(b, "/")
	}
	if b := configuredExtrasBase(); b != "" {
		return strings.TrimRight(b, "/")
	}
	return defaultExtrasBase
}

func revampedBaseOverride() string {
	return os.Getenv("RYOSTORE_REVAMPED_BASE")
}

func revampedBase() string {
	if b := revampedBaseOverride(); b != "" {
		return strings.TrimRight(b, "/")
	}
	if b := configuredRevampedBase(); b != "" {
		return strings.TrimRight(b, "/")
	}
	return defaultRevampedBase
}

// configuredExtrasBase reads a persistent source override from
// ${XDG_CONFIG_HOME:-~/.config}/ryoku/ryostore-base (one line: a base URL or a
// file:// path). It lets a dev checkout, fork, or mirror be selected without a
// session env var, which a long-running compositor may never have inherited.
func configuredExtrasBase() string {
	b, err := os.ReadFile(filepath.Join(configHome(), "ryoku", "ryostore-base"))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

func configuredRevampedBase() string {
	b, err := os.ReadFile(filepath.Join(configHome(), "ryoku", "ryostore-revamped-base"))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

// localBase reports whether a catalogue base is a file:// URL or plain local
// path and returns its root. Local bases let a checkout under test serve the
// catalogue straight off disk, with no network or CDN.
func localBase(base string) (string, bool) {
	if strings.HasPrefix(base, "file://") {
		return strings.TrimPrefix(base, "file://"), true
	}
	if base != "" && !strings.Contains(base, "://") {
		return filepath.Clean(base), true
	}
	return "", false
}

func xdgCacheHome() string {
	if b := os.Getenv("XDG_CACHE_HOME"); b != "" {
		return b
	}
	return filepath.Join(os.Getenv("HOME"), ".cache")
}

// extrasCacheDir is where fetched registries and assets are cached so the
// catalogue still renders offline: ${XDG_CACHE_HOME:-~/.cache}/ryoku/ryostore.
// An install that predates the ryostore rename keeps a cache under the old
// ryoku/extras path; `ryoku doctor` renames it across (reconcileRyostoreCache)
// so the archive survives the upgrade with no re-download. An explicit source
// override (RYOSTORE_BASE / ryostore-base) caches under a stable per-source
// subdirectory so a fork or local tree never serves its bytes as the default
// source's archive.
func extrasCacheDir() string {
	root := filepath.Join(xdgCacheHome(), "ryoku", "ryostore")
	if extrasBase() == defaultExtrasBase {
		return root
	}
	sum := sha256.Sum256([]byte(extrasBase()))
	return filepath.Join(root, "sources", hex.EncodeToString(sum[:8]))
}

func revampedCacheDir() string {
	root := filepath.Join(xdgCacheHome(), "ryoku", "ryostore-revamped")
	if revampedBase() == defaultRevampedBase {
		return root
	}
	sum := sha256.Sum256([]byte(revampedBase()))
	return filepath.Join(root, "sources", hex.EncodeToString(sum[:8]))
}

func currentRyokuVersion() string {
	out, err := exec.Command("ryoku", "version").Output()
	if err != nil {
		return ""
	}
	return strings.TrimPrefix(strings.TrimSpace(string(out)), "v")
}
