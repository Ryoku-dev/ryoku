// ryostore is the Go data plane for the Ryostore app: it normalizes every
// product catalogue into one JSON contract and installs items without
// activating them. The Quickshell front end shells out to these subcommands.
//
//	ryostore catalog [--refresh] [--category <id>]   normalized catalogue, JSON
//	ryostore install <category> <id> [--from <dir>]  install-only, no activation
//	ryostore remove <category> <id>                  receipt-owned removal
//
// A full catalog answers from a disk snapshot so every launch after the first is
// instant; --refresh rebuilds it live. --category always probes live.
//
// The internal namespace holds calls the extras actuator and Settings make
// directly; later tasks register those subcommands under it.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func main() {
	scrubQuickshellCrashEnv()
	if err := dispatch(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "ryostore:", err)
		os.Exit(1)
	}
}

// dispatch routes one command. It returns an error for every bad invocation so
// the caller reports one useful line and exits nonzero.
func dispatch(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("no command; expected catalog, install, remove, search, check, open, settings, or internal")
	}
	switch args[0] {
	case "catalog":
		return runCatalog(os.Stdout, providers(), args[1:])
	case "install":
		return runInstall(providers(), args[1:])
	case "remove":
		return runRemove(providers(), args[1:])
	case "search":
		return runSearch(os.Stdout, providers(), args[1:])
	case "warm":
		return runWarm(args[1:])
	case "check":
		return runCheck(os.Stdout, providers())
	case "open":
		return runOpen(args[1:])
	case "settings":
		return runSettings(args[1:])
	case "internal":
		return runInternal(args[1:])
	default:
		return fmt.Errorf("unknown command %q; expected catalog, install, remove, search, check, warm, open, settings, or internal", args[0])
	}
}

// snapshotPath is the instant-launch catalogue snapshot.
func snapshotPath() string {
	return filepath.Join(extrasCacheDir(), "catalog.json")
}

// encodeSnapshot renders a catalogue for the store: previews already in the
// cache become local file:// paths (no downloads, so the catalogue stays
// instant); the detached `warm` command fills the rest, so the next open renders
// from disk with no network image loads to abort at close.
func encodeSnapshot(cat Catalog) ([]byte, error) {
	rewriteCachedAssets(cat.Items)
	data, err := json.Marshal(cat)
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

// writeSnapshot stores an encoded catalogue as the launch snapshot. Best effort:
// a failed write only costs the next launch its instant render.
func writeSnapshot(data []byte) {
	_ = atomicWrite(snapshotPath(), data, 0o644)
}

func runCatalog(w io.Writer, provs []Provider, args []string) error {
	refresh := false
	category := ""
	haveCategory := false
	rest := args
	for len(rest) > 0 {
		switch rest[0] {
		case "--refresh":
			refresh = true
			rest = rest[1:]
		case "--category":
			if len(rest) < 2 {
				return fmt.Errorf("--category needs a category id")
			}
			category = rest[1]
			haveCategory = true
			rest = rest[2:]
		default:
			return fmt.Errorf("unknown catalog flag %q", rest[0])
		}
	}
	ctx := context.Background()
	if haveCategory {
		if category == "" {
			return fmt.Errorf("--category needs a non-empty id")
		}
		p, ok := providerFor(provs, category)
		if !ok {
			return fmt.Errorf("unknown category %q", category)
		}
		// A single-category probe is a subset of the store, never the snapshot.
		cat := BuildCatalog(ctx, []Provider{p}, refresh)
		cat.Revision = catalogRevision(cat)
		return json.NewEncoder(w).Encode(cat)
	}
	// The full catalogue is snapshotted so a launch is instant and works offline;
	// the store's refresh button (--refresh) rebuilds it live and rewrites it.
	snapshot := snapshotPath()
	if !refresh {
		// A healthy snapshot answers instantly. One flagged offline came from a
		// transient source failure; serving it again pins the store to "offline"
		// forever, so fall through to a live rebuild that self-heals once the
		// source is reachable. A rebuild that still fails falls back to the same
		// cache, so this never does worse than serving the snapshot. A snapshot
		// built under another window manager is stale the same way: it answers
		// which products run here, and after a compositor switch that answer is
		// the old one. One whose built-in styles read differently predates this
		// binary.
		if data, err := os.ReadFile(snapshot); err == nil && len(data) > 0 && !snapshotOffline(data) && !snapshotForeignWindowManager(data) && !snapshotStaleBuiltins(data) && !snapshotProviderMismatch(data, provs) && !snapshotFlatpakRemoteMismatch(data, provs) {
			_, err := w.Write(data)
			return err
		}
	}
	cat := BuildCatalog(ctx, provs, refresh)
	cat.Revision = catalogRevision(cat)
	data, err := encodeSnapshot(cat)
	if err != nil {
		return err
	}
	// Only snapshot a catalogue that produced items, so a failed first fetch is
	// retried live next launch instead of caching an empty store.
	if len(cat.Items) > 0 {
		writeSnapshot(data)
		// A freshly built catalogue is the baseline the user is now looking at,
		// so acknowledge it: the refresh dot stays clear until upstream advances.
		writeSeenRevision(cat.Revision)
	}
	_, err = w.Write(data)
	return err
}

func snapshotProviderMismatch(data []byte, provs []Provider) bool {
	var s struct {
		Categories []struct {
			ID string `json:"id"`
		} `json:"categories"`
	}
	if json.Unmarshal(data, &s) != nil || len(s.Categories) != len(provs) {
		return true
	}
	for i, p := range provs {
		if s.Categories[i].ID != p.Category().ID {
			return true
		}
	}
	return false
}

func snapshotFlatpakRemoteMismatch(data []byte, provs []Provider) bool {
	var p flatpakProvider
	found := false
	for _, provider := range provs {
		if flatpak, ok := provider.(flatpakProvider); ok {
			p, found = flatpak, true
			break
		}
	}
	if !found {
		return false
	}
	var snapshot struct {
		Items []struct {
			Category          string         `json:"category"`
			Installed         bool           `json:"installed"`
			UnavailableReason string         `json:"unavailableReason"`
			Metadata          map[string]any `json:"metadata"`
		} `json:"items"`
	}
	if json.Unmarshal(data, &snapshot) != nil {
		return true
	}
	present := p.present()
	systemReason := "Flatpak is not installed on this system."
	checked := map[string]string{}
	if present {
		checked = p.remoteReasons(context.Background(), nil)
		_, systemReason = newFlatpakInstallation(checked)
	}
	for _, item := range snapshot.Items {
		if item.Category != flatpakCategory {
			continue
		}
		if !item.Installed {
			if item.UnavailableReason != systemReason {
				return true
			}
			continue
		}
		installation, ok := item.Metadata["installation"].(string)
		if !ok || !present {
			return true
		}
		if reason, ok := checked[installation]; ok {
			if reason != "" {
				return true
			}
		} else {
			checked[installation] = p.flathubRemoteReason(context.Background(), installation)
			if checked[installation] != "" {
				return true
			}
		}
	}
	return false
}

// snapshotOffline reports whether a cached catalogue was written while a source
// was failing, so a stale offline snapshot is retried live instead of pinning
// the store offline on every launch.
func snapshotOffline(data []byte) bool {
	var s struct {
		Offline bool `json:"offline"`
	}
	return json.Unmarshal(data, &s) == nil && s.Offline
}

// snapshotForeignWindowManager reports whether a cached catalogue was built
// against a window manager other than the running one. Its per-item availability
// answers belong to the desktop it was built on, so after a compositor switch the
// snapshot is rebuilt live instead of served. An unrecorded or undetectable
// manager is never foreign: nothing is rebuilt on a guess.
func snapshotForeignWindowManager(data []byte) bool {
	var s struct {
		WindowManager string `json:"windowManager"`
	}
	if json.Unmarshal(data, &s) != nil {
		return false
	}
	running := runningWindowManager()
	if s.WindowManager == "" || running == "" {
		return false
	}
	return !strings.EqualFold(s.WindowManager, running)
}

// runWarm is the detached background asset populator the store launches after a
// catalogue load: it pulls every preview and screenshot in the current snapshot
// into the cache and rewrites the snapshot to local file:// paths, so the next
// open renders entirely from disk with no network image loads. Single-flight (a
// non-blocking lock) and idempotent, so repeated launches converge without
// duplicate downloads.
func runWarm(_ []string) error {
	unlock, ok := warmLock()
	if !ok {
		return nil
	}
	defer unlock()
	snapshot := filepath.Join(extrasCacheDir(), "catalog.json")
	data, err := os.ReadFile(snapshot)
	if err != nil {
		return nil
	}
	var cat Catalog
	if err := json.Unmarshal(data, &cat); err != nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), warmBudget)
	defer cancel()
	warmAssets(ctx, assetHTTPClient(), cat.Items)
	out, err := json.Marshal(cat)
	if err != nil {
		return err
	}
	return atomicWrite(snapshot, append(out, '\n'), 0o644)
}

// runCheck probes the live registries (no asset downloads, no snapshot rewrite)
// and reports whether the catalogue has advanced past the last acknowledged
// revision, so the store lights its refresh dot only on a real ryostore
// change. It never writes the seen revision: only viewing the fresh catalogue
// (runCatalog) acknowledges it.
func runCheck(w io.Writer, provs []Provider) error {
	cat := BuildCatalog(context.Background(), provs, true)
	cat.Revision = catalogRevision(cat)
	// The probe already built the catalogue live, so keep that build as the
	// launch snapshot. Installed state is local and never part of the revision,
	// so a part installed outside the store (Settings, a terminal) would
	// otherwise stay frozen as absent in the snapshot served on the next open.
	if data, err := encodeSnapshot(cat); err == nil && len(cat.Items) > 0 {
		writeSnapshot(data)
	}
	revision := cat.Revision
	seen := readSeenRevision()
	out := struct {
		Revision        string `json:"revision"`
		UpdateAvailable bool   `json:"updateAvailable"`
		Offline         bool   `json:"offline"`
	}{
		Revision:        revision,
		UpdateAvailable: seen != "" && !cat.Offline && revision != seen,
		Offline:         cat.Offline,
	}
	return json.NewEncoder(w).Encode(out)
}

func runInstall(provs []Provider, args []string) error {
	dither := false
	from := ""
	var only []string
	rest := make([]string, 0, len(args))
	i := 0
	for i < len(args) {
		a := args[i]
		i++
		switch a {
		case "--dither":
			dither = true
		case "--from":
			if i < len(args) {
				from = args[i]
				i++
			}
		case "--only":
			if i < len(args) {
				for _, n := range strings.Split(args[i], ",") {
					if n = strings.TrimSpace(n); n != "" {
						only = append(only, n)
					}
				}
				i++
			}
		default:
			rest = append(rest, a)
		}
	}
	if len(rest) != 2 {
		return fmt.Errorf("install needs <category> <id>")
	}
	category, id := rest[0], rest[1]
	p, ok := providerFor(provs, category)
	if !ok {
		return fmt.Errorf("unknown category %q", category)
	}
	ctx := withProgress(context.Background(), newProgressReporter(os.Stderr))
	finish := func(err error) error {
		if err == nil {
			reportProgressNow(ctx, "apply", 1, 1)
		}
		return err
	}
	if from != "" {
		li, ok := p.(localInstaller)
		if !ok {
			return fmt.Errorf("category %q does not support --from", category)
		}
		return finish(li.InstallFrom(ctx, id, from))
	}
	if len(only) > 0 {
		if ci, ok := p.(componentInstaller); ok {
			return finish(ci.InstallComponents(ctx, id, only))
		}
	}
	if dither {
		if vi, ok := p.(variantInstaller); ok {
			return finish(vi.InstallVariant(ctx, id, true))
		}
	}
	return finish(p.Install(ctx, id))
}

type searcher interface {
	Search(ctx context.Context, query string) ([]Item, error)
}

const searchTimeout = 20 * time.Second

func runSearch(w io.Writer, provs []Provider, args []string) error {
	if len(args) != 2 {
		return fmt.Errorf("search needs <category> <query>")
	}
	p, ok := providerFor(provs, args[0])
	if !ok {
		return fmt.Errorf("unknown category %q", args[0])
	}
	s, ok := p.(searcher)
	if !ok {
		return fmt.Errorf("category %q does not support search", args[0])
	}
	ctx, cancel := context.WithTimeout(context.Background(), searchTimeout)
	defer cancel()
	items, err := s.Search(ctx, args[1])
	if err != nil {
		return err
	}
	if items == nil {
		items = []Item{}
	}
	return json.NewEncoder(w).Encode(struct {
		Category string `json:"category"`
		Query    string `json:"query"`
		Items    []Item `json:"items"`
	}{args[0], args[1], items})
}

// localInstaller is a provider that can install a product from a local
// directory (the plugins provider, for `ryoku plugin add`).
type localInstaller interface {
	InstallFrom(ctx context.Context, id, dir string) error
}

// variantInstaller is a provider that offers install-time variants (the decors
// dither toggle). Providers that do not implement it ignore --dither.
type variantInstaller interface {
	InstallVariant(ctx context.Context, id string, dither bool) error
}

// componentInstaller is a provider that can install a named subset of a
// product's components (the bundles manual selection). Providers that do not
// implement it ignore --only.
type componentInstaller interface {
	InstallComponents(ctx context.Context, id string, only []string) error
}

func runRemove(provs []Provider, args []string) error {
	if len(args) != 2 {
		return fmt.Errorf("remove needs <category> <id>")
	}
	category, id := args[0], args[1]
	p, ok := providerFor(provs, category)
	if !ok {
		return fmt.Errorf("unknown category %q", category)
	}
	return p.Remove(context.Background(), id)
}

// runInternal namespaces commands the extras actuator and Settings call
// directly: the browse cache directory, this edition's bundle package table, an
// on-demand script installer path, and the guest install/remove primitives.
// These are not public UI commands.
func runInternal(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("internal needs a subcommand")
	}
	switch args[0] {
	case "cache":
		fmt.Println(extrasCacheDir())
		return nil
	case "package-table":
		// Empty on pacman hosts, whose package names are the catalogue's.
		if path := ensureBundlePackageTable(context.Background(), newCache(), false); path != "" {
			fmt.Println(path)
		}
		return nil
	case "installer":
		if len(args) < 2 {
			return fmt.Errorf("internal installer needs a name")
		}
		p, err := ensureInstaller(args[1])
		if err != nil {
			return err
		}
		fmt.Println(p)
		return nil
	case "bundle":
		if len(args) < 2 {
			return fmt.Errorf("internal bundle needs an id")
		}
		p, err := ensureBundleManifest(args[1])
		if err != nil {
			return err
		}
		fmt.Println(p)
		return nil
	case "install-guest":
		if len(args) < 3 {
			return fmt.Errorf("internal install-guest needs <kind> <id>")
		}
		return installGuest(args[1], args[2])
	case "remove-guest":
		if len(args) < 3 {
			return fmt.Errorf("internal remove-guest needs <kind> <id>")
		}
		return removeGuest(args[1], args[2])
	case "apply-fastfetch":
		if len(args) != 2 {
			return fmt.Errorf("internal apply-fastfetch needs <id>")
		}
		return applyFastfetchStyle(args[1])
	default:
		return fmt.Errorf("unknown internal command %q", args[0])
	}
}
