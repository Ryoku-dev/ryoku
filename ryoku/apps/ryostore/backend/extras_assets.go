// Guest asset primitives: the install/remove operations Ryostore performs on
// the machine for a plugin, a bundle script installer, or a Nautilus pack. They
// write to the runtime's own data directories (never the browse cache) and are
// reached through the `internal install-guest`/`remove-guest`/`installer`
// commands the extras actuator calls. Every write lands atomically and, for a
// plugin, manifest-last inside a staging directory renamed into place only once
// complete, so an aborted fetch never leaves a half-installed guest the runtime
// would try to mount. Removal is symlink-safe: a dev guest symlinked into a
// checkout is unlinked, never recursed into.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
)

var renamePath = os.Rename

func validComponent(name string) bool {
	return name != "" && name != "." && !strings.HasPrefix(name, ".ryostore-") && filepath.IsLocal(name) && filepath.Clean(name) == name && filepath.Base(name) == name
}

func validLocalPath(name string) bool {
	if name == "" || name == "." || !filepath.IsLocal(name) || filepath.Clean(name) != name {
		return false
	}
	for _, part := range strings.Split(name, string(filepath.Separator)) {
		if part == "" || part == "." || strings.HasPrefix(part, ".ryostore-") {
			return false
		}
	}
	return true
}

func rejectSymlinkPath(root, rel string) error {
	current := root
	for _, part := range strings.Split(rel, string(filepath.Separator)) {
		current = filepath.Join(current, part)
		fi, err := os.Lstat(current)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return err
		}
		if fi.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("%s is a symlink", current)
		}
	}
	return nil
}

func lockTree(dst string) (func(), error) {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return nil, err
	}
	path := filepath.Join(filepath.Dir(dst), ".ryostore-lock-"+filepath.Base(dst))
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX); err != nil {
		file.Close()
		return nil, err
	}
	return func() {
		_ = syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
		_ = file.Close()
	}, nil
}

func backupTreePath(dst string) string {
	return filepath.Join(filepath.Dir(dst), ".ryostore-backup-"+filepath.Base(dst))
}

func journalTreePath(dst string) string {
	return backupTreePath(dst) + ".journal"
}

func recoverTree(dst string) error {
	backup := backupTreePath(dst)
	journal := journalTreePath(dst)
	_, backupErr := os.Lstat(backup)
	_, journalErr := os.Lstat(journal)
	if os.IsNotExist(backupErr) {
		if journalErr == nil {
			return os.Remove(journal)
		}
		if journalErr != nil && !os.IsNotExist(journalErr) {
			return journalErr
		}
		return nil
	}
	if backupErr != nil {
		return backupErr
	}
	if journalErr != nil {
		if os.IsNotExist(journalErr) {
			return fmt.Errorf("reserved backup path %s is not a Ryostore journal", backup)
		}
		return journalErr
	}
	if _, err := os.Lstat(dst); err == nil {
		if err := os.RemoveAll(backup); err != nil {
			return err
		}
		return os.Remove(journal)
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := renamePath(backup, dst); err != nil {
		return err
	}
	return os.Remove(journal)
}

func replaceTree(stage, dst string, finalize func() error) error {
	if err := recoverTree(dst); err != nil {
		return fmt.Errorf("recover prior install: %w", err)
	}
	backup := backupTreePath(dst)
	journal := journalTreePath(dst)
	hadPrior := false
	if _, err := os.Lstat(dst); err == nil {
		if err := atomicWrite(journal, []byte(filepath.Base(dst)), 0o600); err != nil {
			return err
		}
		if err := renamePath(dst, backup); err != nil {
			_ = os.Remove(journal)
			return err
		}
		hadPrior = true
	} else if !os.IsNotExist(err) {
		return err
	}

	restore := func(cause error) error {
		_ = os.RemoveAll(dst)
		if hadPrior {
			if err := renamePath(backup, dst); err != nil {
				return fmt.Errorf("%w; restore prior install: %v", cause, err)
			}
			_ = os.Remove(journal)
		}
		return cause
	}
	if err := renamePath(stage, dst); err != nil {
		return restore(err)
	}
	if finalize != nil {
		if err := finalize(); err != nil {
			return restore(err)
		}
	}
	if hadPrior {
		if err := os.RemoveAll(backup); err != nil {
			return err
		}
		return os.Remove(journal)
	}
	return nil
}

func dataHome() string {
	if b := os.Getenv("XDG_DATA_HOME"); b != "" {
		return b
	}
	return filepath.Join(os.Getenv("HOME"), ".local", "share")
}

// atomicWrite writes b to path via a same-directory temp file and rename, so a
// reader never sees a half-written file.
func atomicWrite(path string, b []byte, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".tmp-*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	if _, err := f.Write(b); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if err := f.Chmod(mode); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, path)
}

// installGuest routes an internal install-guest call to the primitive for its
// kind. Both kinds place the guest without activating it.
func installGuest(kind, id string) error {
	switch kind {
	case "plugins":
		_, err := ensurePlugin(id)
		return err
	case "nautilus":
		_, err := ensureNautilusPack(id)
		return err
	default:
		return fmt.Errorf("unknown guest kind %q", kind)
	}
}

// removeGuest routes an internal remove-guest call to the primitive for its
// kind. Both removals are symlink-safe.
//
// A Store-installed plugin owns a receipt, so it has to go out the way it came
// in: the product transaction, which drops the receipt, the index row and the
// cached view along with the files. Deleting only the data dir leaves the Store
// certain the plugin is still installed, so the card stays badged INSTALLED and
// the user can never reinstall it. A dev plugin (RYOSTORE_PLUGINS_DIR, or a
// symlink into a checkout) has no receipt and keeps the plain directory unlink.
func removeGuest(kind, id string) error {
	switch kind {
	case "plugins":
		if _, err := readReceipt("plugins", id); err == nil {
			return removeProduct(context.Background(), "plugins", id)
		}
		return removePlugin(id)
	case "nautilus":
		return removeNautilusPack(id)
	default:
		return fmt.Errorf("unknown guest kind %q", kind)
	}
}

// pluginDataDir is where an installed plugin's source lives; the shell runtime
// and Ryoku Settings both read it. Mirrors plugin_dir() in ryostore-install.
func pluginDataDir(id string) string {
	return filepath.Join(dataHome(), "ryoku", "plugins", id)
}

// ensurePlugin pulls a plugin's full source tree from the catalogue
// (plugins/<id>/) into a staging directory and renames it into the data dir only
// once every manifest-declared file has arrived. The manifest is read to know
// which files to grab (entryPoints + commands + files); README and the preview
// are cosmetic and skip on a miss. It seeds the plugin's preset block into
// plugins.json but never enables it: installing must not activate placement.
func ensurePlugin(id string) (string, error) {
	if !validComponent(id) {
		return "", fmt.Errorf("invalid plugin id %q", id)
	}
	dst := pluginDataDir(id)
	unlock, err := lockTree(dst)
	if err != nil {
		return "", err
	}
	defer unlock()
	if err := recoverTree(dst); err != nil {
		return "", fmt.Errorf("recover plugin %q: %w", id, err)
	}
	_, priorErr := os.ReadFile(filepath.Join(dst, "manifest.json"))
	priorInstalled := priorErr == nil
	if priorErr != nil && !os.IsNotExist(priorErr) {
		return "", priorErr
	}
	c := newCache()
	ctx := context.Background()
	rel := "plugins/" + id
	manRaw, err := c.get(ctx, rel+"/manifest.json")
	if err != nil {
		return "", fmt.Errorf("plugin %q not found in the catalogue: %w", id, err)
	}
	var man struct {
		EntryPoints map[string]string `json:"entryPoints"`
		Commands    []string          `json:"commands"`
		Files       []string          `json:"files"`
	}
	if err := json.Unmarshal(manRaw, &man); err != nil {
		return "", fmt.Errorf("plugin %q manifest: %w", id, err)
	}

	optional := map[string]bool{"README.md": true, "assets/preview.gif": true}
	files := []string{"README.md", "assets/preview.gif"}
	for _, f := range man.EntryPoints {
		files = append(files, f)
	}
	files = append(files, man.Commands...)
	files = append(files, man.Files...)
	for _, f := range files {
		if !validLocalPath(f) {
			return "", fmt.Errorf("plugin %q has invalid file path %q", id, f)
		}
	}

	stage, err := os.MkdirTemp(filepath.Dir(dst), ".stage-*")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(stage)

	for _, f := range files {
		b, err := c.get(ctx, rel+"/"+f)
		if err != nil {
			if optional[f] {
				continue
			}
			return "", fmt.Errorf("plugin %q: could not fetch %s: %w", id, f, err)
		}
		mode := os.FileMode(0o644)
		if strings.HasPrefix(f, "bin/") {
			mode = 0o755
		}
		if err := atomicWrite(filepath.Join(stage, f), b, mode); err != nil {
			return "", err
		}
	}
	if err := atomicWrite(filepath.Join(stage, "manifest.json"), manRaw, 0o644); err != nil {
		return "", err
	}

	if !priorInstalled {
		if err := exec.Command("ryoku-plugins-place", id, "enabled", "false").Run(); err != nil {
			return "", fmt.Errorf("disable plugin placement before install: %w", err)
		}
	}
	if err := replaceTree(stage, dst, func() error {
		if err := exec.Command("ryoku-plugins-place", id, "seed").Run(); err != nil {
			return fmt.Errorf("seed plugin settings: %w", err)
		}
		return nil
	}); err != nil {
		return "", err
	}
	return dst, nil
}

// removePlugin removes only the installed source tree. Placement and settings
// survive bundle removal; Settings explicitly forgets them for a full uninstall.
// Symlink-safe: a dev source link is unlinked, never traversed.
func removePlugin(id string) error {
	if !validComponent(id) {
		return fmt.Errorf("invalid plugin id %q", id)
	}
	dir := pluginDataDir(id)
	unlock, err := lockTree(dir)
	if err != nil {
		return err
	}
	defer unlock()
	if err := recoverTree(dir); err != nil {
		return fmt.Errorf("recover plugin %q: %w", id, err)
	}
	fi, err := os.Lstat(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if fi.Mode()&os.ModeSymlink != 0 {
		return os.Remove(dir)
	}
	return os.RemoveAll(dir)
}

// ensureInstaller pulls a fresh copy of a script installer into the cache and
// returns its path, falling back to the cached copy when the source is offline.
// A bundle publishes its installer under its own directory
// (bundles/<id>/installers/<name>.sh, as its bundle.json declares); the
// catalogue root (installers/<name>.sh) remains only for legacy entries, so
// resolve through the owning bundle's registry entry first and fall back to
// the root.
func ensureInstaller(name string) (string, error) {
	if !validComponent(name) {
		return "", fmt.Errorf("invalid installer name %q", name)
	}
	cache := newCache()
	rels := []string{}
	if raw, _, err := cache.Fetch(context.Background(), "bundles/registry.json", false); err == nil {
		var reg registry
		if err := json.Unmarshal(raw, &reg); err == nil {
			for _, entry := range reg.Bundles {
				declares := false
				for _, comp := range entry.Components {
					if comp.Type == "script" && comp.Name == name {
						declares = true
						break
					}
				}
				if !declares {
					continue
				}
				bundlePath := entry.Path
				if bundlePath == "" {
					bundlePath = "bundles/" + entry.ID
				}
				if validLocalPath(bundlePath) {
					rels = append(rels, bundlePath+"/installers/"+name+".sh")
					break
				}
			}
		}
	}
	rels = append(rels, "installers/"+name+".sh")
	var lastErr error
	for _, rel := range rels {
		if _, _, err := cache.Fetch(context.Background(), rel, true); err != nil {
			lastErr = err
			continue
		}
		return filepath.Join(extrasCacheDir(), rel), nil
	}
	return "", fmt.Errorf("installer %q not found in the catalogue: %w", name, lastErr)
}

// ensureBundleManifest pulls the selected bundle's bundle.json into the cache on
// demand and returns its path. Browsing carries inline components in the
// registry and never warms per-bundle definitions, so install and remove fetch
// the full item list (sources, upstreams, requires) only for the one bundle the
// user acted on.
func ensureBundleManifest(id string) (string, error) {
	if !validComponent(id) {
		return "", fmt.Errorf("invalid bundle id %q", id)
	}
	cache := newCache()
	raw, _, err := cache.Fetch(context.Background(), "bundles/registry.json", false)
	if err != nil {
		return "", fmt.Errorf("bundle catalogue: %w", err)
	}
	var reg registry
	if err := json.Unmarshal(raw, &reg); err != nil {
		return "", fmt.Errorf("bundle catalogue: %w", err)
	}
	relPath := ""
	for _, entry := range reg.Bundles {
		if entry.ID == id {
			relPath = entry.Path
			if relPath == "" {
				relPath = "bundles/" + id
			}
			break
		}
	}
	if relPath == "" {
		return "", fmt.Errorf("unknown bundle %q", id)
	}
	if !validLocalPath(relPath) {
		return "", fmt.Errorf("bundle %q has invalid path %q", id, relPath)
	}
	rel := relPath + "/bundle.json"
	if _, _, err := cache.Fetch(context.Background(), rel, true); err != nil {
		return "", fmt.Errorf("bundle %q definition not found: %w", id, err)
	}
	return filepath.Join(extrasCacheDir(), filepath.FromSlash(rel)), nil
}

type nautilusPack struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Path   string `json:"path,omitempty"`
	Subdir string `json:"subdir,omitempty"`
}

type nautilusRegistry struct {
	Version int            `json:"version"`
	Packs   []nautilusPack `json:"packs"`
}

// nautilusScriptsDir is where Nautilus reads user scripts. nautilusTrackDir is
// our per-pack record of what we installed, so removal is exact.
func nautilusScriptsDir() string { return filepath.Join(dataHome(), "nautilus", "scripts") }
func nautilusTrackDir(id string) string {
	return filepath.Join(dataHome(), "ryoku", "nautilus", id)
}

// ensureNautilusPack fetches a pack's scripts into the Nautilus scripts dir
// under its subdir (0755, live-rescanned), recording the file list for a clean
// removal. The scripts ARE the pack, so a failed fetch aborts rather than
// landing a pack with missing right-click actions that still reports installed.
func ensureNautilusPack(id string) (string, error) {
	if !validComponent(id) {
		return "", fmt.Errorf("invalid nautilus pack id %q", id)
	}
	track := nautilusTrackDir(id)
	idUnlock, err := lockTree(track)
	if err != nil {
		return "", err
	}
	defer idUnlock()
	if fi, err := os.Lstat(track); err == nil && fi.Mode()&os.ModeSymlink != 0 {
		return "", fmt.Errorf("%s is a symlink", track)
	} else if err != nil && !os.IsNotExist(err) {
		return "", err
	}
	oldSubdir := ""
	var unlock func()
	if oldRaw, err := os.ReadFile(filepath.Join(track, "manifest.json")); err == nil {
		var old struct {
			Subdir string `json:"subdir"`
		}
		if err := json.Unmarshal(oldRaw, &old); err != nil {
			return "", fmt.Errorf("nautilus tracking manifest: %w", err)
		}
		if !validLocalPath(old.Subdir) {
			return "", fmt.Errorf("invalid tracked nautilus subdir %q", old.Subdir)
		}
		oldSubdir = old.Subdir
		oldRoot := filepath.Join(nautilusScriptsDir(), oldSubdir)
		var err error
		unlock, err = lockTree(oldRoot)
		if err != nil {
			return "", err
		}
		defer unlock()
		if err := recoverTree(oldRoot); err != nil {
			return "", fmt.Errorf("recover nautilus pack %q: %w", id, err)
		}
	} else if !os.IsNotExist(err) {
		return "", err
	}
	c := newCache()
	ctx := context.Background()
	raw, err := c.get(ctx, "nautilus/registry.json")
	if err != nil {
		return "", fmt.Errorf("nautilus catalogue not found: %w", err)
	}
	var reg nautilusRegistry
	if err := json.Unmarshal(raw, &reg); err != nil {
		return "", fmt.Errorf("nautilus/registry.json: %w", err)
	}
	var pk *nautilusPack
	for i := range reg.Packs {
		if reg.Packs[i].ID == id {
			pk = &reg.Packs[i]
			break
		}
	}
	if pk == nil {
		return "", fmt.Errorf("nautilus pack %q not in the catalogue", id)
	}
	path := pk.Path
	if path == "" {
		path = "nautilus/" + id
	}
	if !validLocalPath(path) {
		return "", fmt.Errorf("nautilus pack %q has invalid source path %q", id, path)
	}
	manRaw, err := c.get(ctx, path+"/manifest.json")
	if err != nil {
		return "", fmt.Errorf("nautilus pack %q manifest: %w", id, err)
	}
	var man struct {
		Subdir  string   `json:"subdir"`
		Scripts []string `json:"scripts"`
	}
	if err := json.Unmarshal(manRaw, &man); err != nil {
		return "", fmt.Errorf("nautilus pack %q manifest: %w", id, err)
	}
	subdir := man.Subdir
	if subdir == "" {
		subdir = pk.Subdir
	}
	if subdir == "" {
		subdir = pk.Name
	}
	if subdir == "" {
		subdir = id
	}
	if !validLocalPath(subdir) {
		return "", fmt.Errorf("nautilus pack %q has invalid subdir %q", id, subdir)
	}
	for _, script := range man.Scripts {
		if !validLocalPath(script) {
			return "", fmt.Errorf("nautilus pack %q has invalid script path %q", id, script)
		}
	}

	scriptsRoot := nautilusScriptsDir()
	if err := rejectSymlinkPath(scriptsRoot, subdir); err != nil {
		return "", err
	}
	root := filepath.Join(scriptsRoot, subdir)
	if oldSubdir != "" && oldSubdir != subdir {
		return "", fmt.Errorf("nautilus pack %q cannot change subdir from %q to %q", id, oldSubdir, subdir)
	}
	if unlock == nil {
		var err error
		unlock, err = lockTree(root)
		if err != nil {
			return "", err
		}
		defer unlock()
		if err := recoverTree(root); err != nil {
			return "", fmt.Errorf("recover nautilus pack %q: %w", id, err)
		}
	}
	if err := os.MkdirAll(filepath.Dir(root), 0o755); err != nil {
		return "", err
	}
	stage, err := os.MkdirTemp(filepath.Dir(root), ".stage-*")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(stage)
	for _, script := range man.Scripts {
		b, err := c.get(ctx, path+"/scripts/"+script)
		if err != nil {
			return "", fmt.Errorf("nautilus pack %q: could not fetch %s: %w", id, script, err)
		}
		if err := atomicWrite(filepath.Join(stage, script), b, 0o755); err != nil {
			return "", err
		}
	}

	rec, err := json.Marshal(map[string]any{"id": id, "subdir": subdir, "scripts": man.Scripts})
	if err != nil {
		return "", err
	}
	// The tracking tree is prepared before publication; its manifest is renamed
	// last by the replace transaction below.
	if fi, err := os.Lstat(track); err == nil && fi.Mode()&os.ModeSymlink != 0 {
		return "", fmt.Errorf("%s is a symlink", track)
	} else if err != nil && !os.IsNotExist(err) {
		return "", err
	}
	if err := os.MkdirAll(track, 0o755); err != nil {
		return "", err
	}
	pending, err := os.CreateTemp(track, ".manifest-*")
	if err != nil {
		return "", err
	}
	pendingName := pending.Name()
	defer os.Remove(pendingName)
	if _, err := pending.Write(rec); err != nil {
		pending.Close()
		return "", err
	}
	if err := pending.Chmod(0o644); err != nil {
		pending.Close()
		return "", err
	}
	if err := pending.Close(); err != nil {
		return "", err
	}

	if err := replaceTree(stage, root, func() error {
		return renamePath(pendingName, filepath.Join(track, "manifest.json"))
	}); err != nil {
		return "", err
	}
	return root, nil
}

// removeNautilusPack deletes a pack's installed scripts (its whole subdir) and
// the tracking record. No-op if never installed.
func removeNautilusPack(id string) error {
	if !validComponent(id) {
		return fmt.Errorf("invalid nautilus pack id %q", id)
	}
	track := nautilusTrackDir(id)
	idUnlock, err := lockTree(track)
	if err != nil {
		return err
	}
	defer idUnlock()
	if fi, err := os.Lstat(track); err == nil && fi.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("%s is a symlink", track)
	} else if err != nil && !os.IsNotExist(err) {
		return err
	}
	readSubdir := func() (string, error) {
		b, err := os.ReadFile(filepath.Join(track, "manifest.json"))
		if err != nil {
			return "", err
		}
		var rec struct {
			Subdir string `json:"subdir"`
		}
		if err := json.Unmarshal(b, &rec); err != nil {
			return "", fmt.Errorf("nautilus tracking manifest: %w", err)
		}
		if !validLocalPath(rec.Subdir) {
			return "", fmt.Errorf("invalid tracked nautilus subdir %q", rec.Subdir)
		}
		return rec.Subdir, nil
	}
	subdir, err := readSubdir()
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	root := filepath.Join(nautilusScriptsDir(), subdir)
	rootUnlock, err := lockTree(root)
	if err != nil {
		return err
	}
	defer rootUnlock()
	if err := recoverTree(root); err != nil {
		return fmt.Errorf("recover nautilus pack %q: %w", id, err)
	}
	subdir, err = readSubdir()
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	root = filepath.Join(nautilusScriptsDir(), subdir)
	if err := rejectSymlinkPath(nautilusScriptsDir(), subdir); err != nil {
		return err
	}
	if err := os.RemoveAll(root); err != nil {
		return err
	}
	return os.RemoveAll(track)
}
