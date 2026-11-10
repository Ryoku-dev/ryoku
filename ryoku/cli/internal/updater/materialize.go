package updater

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"ryoku-cli/internal/sys"
	i18n "ryoku-i18n"
	wm "ryoku-wm"
	"sort"
	"strings"
)

const wirePlumberPolicyRel = "wireplumber/wireplumber.conf.d/51-ryoku-bluetooth.conf"

// Ryoku's drop-ins share wireplumber.conf.d with the user's own, so only the
// NN-ryoku-*.conf ones are ours to converge.
const wpDropInDir = "wireplumber/wireplumber.conf.d"

var ryokuDropIn = regexp.MustCompile(`^[0-9]+-ryoku-[^/]*\.conf$`)

type iniProjection struct {
	section string
	key     string
}

var iconThemeProjections = map[string]iniProjection{
	"gtk-3.0/settings.ini": {"Settings", "gtk-icon-theme-name"},
	"gtk-4.0/settings.ini": {"Settings", "gtk-icon-theme-name"},
	"qt6ct/qt6ct.conf":     {"Appearance", "icon_theme"},
}

// stripINIProjection removes the one line Ryoku derives from icon-theme state.
// Comparing the remaining bytes keeps that projection from looking like a
// whole-file user fork while preserving every unrelated edit.
func stripINIProjection(body []byte, projection iniProjection) []byte {
	lines := bytes.SplitAfter(body, []byte{'\n'})
	out := make([]byte, 0, len(body))
	section := ""
	removed := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(string(line))
		if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
			section = strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(trimmed, "["), "]"))
		}
		if !removed && section == projection.section &&
			!strings.HasPrefix(trimmed, "#") && !strings.HasPrefix(trimmed, ";") {
			if key, _, ok := strings.Cut(trimmed, "="); ok && strings.TrimSpace(key) == projection.key {
				removed = true
				continue
			}
		}
		out = append(out, line...)
	}
	return out
}

// A picked icon theme is durable state under ~/.config/ryoku, while its GTK
// and Qt projections are managed files refreshed during materialize. Reapply
// after the base and user overlay land, then hash projected files without the
// derived key so a later Hub choice never looks like a hand-edited fork.
var reapplyIconTheme = func(configHome string) error {
	if !sys.Exists(filepath.Join(configHome, "ryoku", "icon-theme")) {
		return nil
	}
	return exec.Command("ryoku-hub", "desktop", "icon-theme", "apply").Run()
}

// generatedSeed: base files seeded once on a fresh install, then never
// clobbered or pruned by an update. The machine owns them after first boot.
// Two kinds qualify: per-machine files the runtime regenerates (monitors.lua,
// gpu.lua; Matugen's app palette siblings) and user-owned config the package
// only seeds a starting point for (keyboard.lua; hypr/user.lua;
// fastfetch/config.jsonc; the Starship layout selected in the Hub; the Vencord
// settings of Vesktop and Equibop, which enable the shipped Ryoku Discord theme
// until the client writes its own). Slash-separated paths, relative to the
// config base. Most are also in sys.LiveOwnedConfig so the overlay never re-lays
// a frozen copy over a file edited in place; ghostty/config is the exception --
// it is a seed the user may instead fork through the overlay, so it stays
// overlay-able (not live-owned).
var generatedSeed = generatedSeedSet()

func generatedSeedSet() map[string]bool {
	seed := map[string]bool{
		"fastfetch/config.jsonc":         true,
		"fastfetch/ryoku-colors.json":    true,
		"starship.toml":                  true,
		"starship/ryoku-colors.toml":     true,
		"kitty/current-theme.conf":       true,
		"ghostty/config":                 true,
		"ghostty/ryoku-colors":           true,
		"vesktop/settings/settings.json": true,
		"equibop/settings/settings.json": true,
	}
	// Every provider's per-machine files are seeded and kept regardless of which
	// compositor is running, so an update under one never prunes another's. The
	// names come from the provider because they are its own config format.
	for _, name := range wm.Providers() {
		for _, rel := range wm.ConfigSeeds(name) {
			seed[rel] = true
		}
	}
	return seed
}

// nvim is seeded like ghostty: Ryoku lays its LazyVim starting point once, then
// the config is the user's. Their edits, plugins and LazyVim's own state under
// ~/.config/nvim then survive every update instead of being reset each time.
// isSeed folds the per-path seeds and the whole nvim tree into one test.
func isSeed(rel string) bool {
	return generatedSeed[rel] || strings.HasPrefix(rel, "nvim/")
}

// generatedRels are the config files a provider's apply authors, across every
// provider, so the lay never writes them: the file's own provider owns its
// bytes. wayfire's generated path is a shipped file too (its baseline is
// compose's first layer, read from the packaged base directly), which is what
// makes this set necessary rather than theoretical.
var generatedRels = generatedRelSet()

func generatedRelSet() map[string]bool {
	out := map[string]bool{}
	for _, name := range wm.Providers() {
		dir := wm.ConfigDir(name)
		for _, rel := range wm.GeneratedConfig(name) {
			if d, _, ok := strings.Cut(rel, "/"); ok && d == dir {
				out[rel] = true
			}
		}
	}
	return out
}

// providerConfigDirs is the set of ~/.config subdirs the window-manager
// providers own, asked of the seam so no compositor is named here.
func providerConfigDirs() map[string]bool {
	out := map[string]bool{}
	for _, name := range wm.Providers() {
		if dir := wm.ConfigDir(name); dir != "" {
			out[dir] = true
		}
	}
	return out
}

func isProviderConfigDir(dir string) bool { return providerConfigDirs()[dir] }

// keepsSwitchedAwayTree reports whether rel belongs to a provider config dir this
// base ships nothing under: that compositor is switched away from, and its tree
// is kept whole so switching back restores the desktop instead of a default one.
func keepsSwitchedAwayTree(rel string, shipped map[string]bool) bool {
	dir, _, ok := strings.Cut(rel, "/")
	if !ok || !isProviderConfigDir(dir) {
		return false
	}
	return !shipped[dir]
}

// applyProvider renders one provider's generated config from the neutral
// store; a var so tests stub the provider binary away.
var applyProvider = func(name, store string) error {
	_, err := wm.OpenNamed(name).Apply(store)
	return err
}

// providerInstalled reports whether a laid-down tree still has its provider
// binary. Switched-away trees stay in place after their variant is removed, but
// without the binary there is nothing to render. A var so tests stub the lookup.
var providerInstalled = func(name string) bool {
	return wm.OpenNamed(name).Available()
}

// applyGenerated completes every laid-down provider tree. Packages ship the
// static half, while each provider owns files rendered from the neutral store.
// A missing generated file can make the target reject its config at login, and
// a whole-file render composed from an older shipped baseline can keep an
// otherwise complete tree on obsolete defaults. Materialize is shared by login
// bootstrap, switching and updates, so it repairs both states here.
//
// Render marks are provider-declared through the seam. They also distinguish a
// managed user_edits mirror from a whole-file fork: a fork is never replaced
// merely because the shipped baseline changed. Provider failures remain best
// effort, so an unavailable binary leaves its tree untouched instead of failing
// the complete materialize.
//
// A missing neutral store is not a reason to skip the first render. The
// installer only seeds desktop.json for a non-default keyboard, and providers
// render defaults from an unreadable store path. An unmarked user_edits copy
// still has an owner, so without a store it is left alone rather than replaced
// with provider defaults.
func applyGenerated(configHome string) {
	store := filepath.Join(configHome, "ryoku", "desktop.json")
	hasStore := sys.Exists(store)
	base := sys.BaseConfigDir()
	for _, name := range wm.Providers() {
		dir := wm.ConfigDir(name)
		if dir == "" || !sys.Exists(filepath.Join(configHome, dir)) {
			continue // tree not laid down: nothing to complete
		}
		if !providerInstalled(name) {
			continue // switched-away tree has no renderer
		}
		missing := false
		authored := false
		for _, rel := range wm.GeneratedConfig(name) {
			if d, _, ok := strings.Cut(rel, "/"); !ok || d != dir {
				// A provider render in user_edits is its managed mirror. Anything
				// else is a fork whose bytes materialize must not replace.
				path := filepath.Join(configHome, rel)
				if sys.Exists(path) {
					body, err := os.ReadFile(path)
					if err != nil || !wm.GeneratedConfigIsRender(name, body) {
						authored = true
					}
				}
				continue
			}
			if !sys.Exists(filepath.Join(configHome, rel)) {
				missing = true
			}
		}
		stale := wm.GeneratedConfigStale(name, base, configHome)
		if !missing && !stale {
			continue
		}
		if authored && (!hasStore || stale) {
			continue
		}
		if err := applyProvider(name, store); err != nil {
			fmt.Printf(i18n.T("note: could not render %s's generated config (%v); run `ryoku doctor` inside a %s session\n"), name, err, name)
		}
	}
}

// Materialize lays the Ryoku-owned base configs into the user's ~/.config,
// declaratively: every file the package ships under baseConfigDir() is
// copied over (clobbering the previous Ryoku copy), files we shipped before
// but no longer ship are removed, anything the package never shipped (user
// files: hypr/monitors_user.lua, kitty/user.conf, ...) is left alone. Per-machine
// generated seeds (generatedSeed, e.g. hypr/monitors.lua) are the exception:
// seeded only when absent, never clobbered, so an update keeps the user's
// runtime-written display and GPU config.
//
// Production replacement for deploy.sh's per-user config copy. Base lives at
// /usr/share/ryoku/config on an installed system; on a dev checkout
// RYOKU_CONFIG_BASE points at ryoku/<...> via `ryoku deploy`.
//
// The set of Ryoku-owned paths is the manifest. Recorded in the state file so
// the next run can prune files dropped from a release without guessing.
func Materialize() error {
	base := sys.BaseConfigDir()
	dest := sys.ConfigHome()
	state := materializeStatePath()
	wirePlumberBefore, _ := os.ReadFile(filepath.Join(dest, wirePlumberPolicyRel))

	info, err := os.Stat(base)
	if err != nil || !info.IsDir() {
		if os.Getenv("RYOKU_CONFIG_BASE") != "" {
			return fmt.Errorf(i18n.T("base config dir not found: %s (RYOKU_CONFIG_BASE points at a missing dir)"), base)
		}
		return fmt.Errorf(i18n.T("base config dir not found: %s\n"+
			"  `ryoku materialize` applies a packaged install's config; on a dev checkout run `ryoku deploy` instead"), base)
	}

	// ~/.config/ryoku is where the shell's JSON stores live (shell.json,
	// launcher.json, hypr.json). The package ships no file under it, so the
	// walk below never creates it, and the shell's QML self-seed cannot make
	// parent directories: guarantee it here, at install and on every update.
	_ = os.MkdirAll(filepath.Join(dest, "ryoku"), 0o755)

	current, err := walkRel(base)
	if err != nil {
		return fmt.Errorf(i18n.T("scan %s: %w"), base, err)
	}

	// Lay down every shipped file, except seeds: those copy only when absent
	// (fresh install) and never get overwritten, so an update leaves the
	// user's display, GPU pin, and keyboard layout alone. Only clobbered
	// files enter the manifest, so a later prune can never remove a seed either.
	//
	// A shipped file whose live bytes match neither what the last update laid
	// nor what this one ships was edited by hand; it becomes a fork under the
	// overlay instead of being thrown away.
	laidHashes := readManifestHashes(state)
	iconThemeState := sys.Exists(filepath.Join(dest, "ryoku", "icon-theme"))
	overlaid := map[string]bool{}
	if rels, err := sys.UserEditFiles(); err == nil {
		for _, rel := range rels {
			overlaid[rel] = true
		}
	}
	managed := make([]string, 0, len(current))
	hashes := make(map[string]string, len(current))
	var kept []string
	for _, rel := range current {
		dst := filepath.Join(dest, rel)
		if isSeed(rel) {
			// PathPresent, not Exists: a seed slot the user filled with a symlink
			// into their dotfiles is theirs, so never lay the shipped default over
			// it -- not even when the link dangles because its repo is not mounted
			// yet at this point in boot. Exists follows the link and would see the
			// missing target as an empty slot, clobbering the symlink.
			if !sys.PathPresent(dst) {
				if err := sys.CopyFile(filepath.Join(base, rel), dst); err != nil {
					return fmt.Errorf(i18n.T("seed %s: %w"), rel, err)
				}
			}
			continue
		}
		if generatedRels[rel] {
			// The provider's own output, not shipped content: applyGenerated
			// below reads this path as "the tree still needs its render", and
			// laying the shipped bytes would forge that signal. The first
			// login would then read the raw baseline -- no store rows, no
			// seeds, no keybinds -- and every later materialize would clobber
			// a composed file back to it.
			continue
		}
		shipped, err := os.ReadFile(filepath.Join(base, rel))
		if err != nil {
			return fmt.Errorf(i18n.T("read %s: %w"), rel, err)
		}
		hashes[rel] = hashBytes(shipped)
		if forkable(rel) && !overlaid[rel] {
			if laid, ok := laidHashes[rel]; ok {
				if live, err := os.ReadFile(dst); err == nil {
					h := hashBytes(live)
					untouched := h == laid || h == hashes[rel]
					if !untouched && iconThemeState {
						if projection, ok := iconThemeProjections[rel]; ok {
							strippedLive := hashBytes(stripINIProjection(live, projection))
							strippedShipped := hashBytes(stripINIProjection(shipped, projection))
							untouched = strippedLive == laid || strippedLive == strippedShipped
						}
					}
					if !untouched {
						if err := sys.CopyFile(dst, filepath.Join(sys.UserEditsDir(), rel)); err != nil {
							return fmt.Errorf(i18n.T("keep your edit of %s: %w"), rel, err)
						}
						kept = append(kept, rel)
					}
				}
			}
		}
		if err := sys.CopyFile(filepath.Join(base, rel), dst); err != nil {
			return fmt.Errorf(i18n.T("copy %s: %w"), rel, err)
		}
		managed = append(managed, rel)
	}

	// A provider's live tree can be all generated state: wayfire ships no
	// static file under wayfire/ at all, its baseline being read from this base
	// at compose time. Laying files alone would leave ~/.config/wayfire absent
	// and applyGenerated would read "no tree laid down" and skip the render,
	// so the tree directory itself is part of the lay.
	for _, name := range wm.Providers() {
		if dir := wm.ConfigDir(name); dir != "" && sys.Exists(filepath.Join(base, dir)) {
			if err := os.MkdirAll(filepath.Join(dest, dir), 0o755); err != nil {
				return fmt.Errorf(i18n.T("lay the %s config dir: %w"), name, err)
			}
		}
	}

	// Prune files this release no longer ships (in the previous manifest,
	// absent now). Never touches paths outside the manifest, i.e. user files.
	previous := readManifest(state)
	curSet := make(map[string]bool, len(current))
	for _, p := range current {
		curSet[p] = true
	}

	// ~/.config/quickshell is wholly Ryoku-owned (user plugins live under
	// ~/.local/share/ryoku), so converge it against the shipped tree directly:
	// stale QML from releases this box's manifest never recorded (a lost state
	// file, an old deploy.sh run) would otherwise load beside the new tree
	// forever. Everything else is mixed with user files and stays manifest-pruned.
	if local, err := walkRel(filepath.Join(dest, "quickshell")); err == nil {
		for _, rel := range local {
			full := "quickshell/" + rel
			if curSet[full] || generatedSeed[full] {
				continue
			}
			_ = os.Remove(filepath.Join(dest, full))
			pruneEmptyParents(dest, filepath.Dir(full))
		}
	}
	wpConfigPruned := false

	// Withdrawn policy goes even without a manifest entry: deploy.sh copies the
	// files but never writes the manifest, so a box configured that way would
	// keep a dropped override forever.
	if local, err := walkRel(filepath.Join(dest, wpDropInDir)); err == nil {
		for _, rel := range local {
			full := wpDropInDir + "/" + rel
			if curSet[full] || !ryokuDropIn.MatchString(rel) {
				continue
			}
			_ = os.Remove(filepath.Join(dest, full))
			wpConfigPruned = true
			pruneEmptyParents(dest, filepath.Dir(full))
		}
	}

	// retiredKeep: a path Ryoku used to lay into ~/.config and deliberately
	// stopped shipping. The manifest prune would delete the user's copy, which
	// for mimeapps.list means throwing away every default app they picked, the
	// exact damage that moving Ryoku's map to the site layer
	// (/usr/local/share/applications/mimeapps.list) exists to stop. `ryoku doctor`
	// strips the stale Ryoku lines out of it instead.
	retiredKeep := map[string]bool{"mimeapps.list": true}

	// A compositor the machine switched away from still owns its config tree. Its
	// variant package is gone, so its files leave the base and the manifest prune
	// would delete them, and a switch back then boots an autogenerated config with
	// no keybinds and no shell autostart. Delivery reads EVERY provider's tree, so
	// keep whole any provider config dir this base does not ship; a file dropped
	// within a tree the box still ships prunes as before.
	shippedProvider := map[string]bool{}
	for _, rel := range current {
		if dir, _, ok := strings.Cut(rel, "/"); ok && isProviderConfigDir(dir) {
			shippedProvider[dir] = true
		}
	}
	for _, rel := range previous {
		if curSet[rel] || retiredKeep[rel] || keepsSwitchedAwayTree(rel, shippedProvider) {
			continue
		}
		if strings.HasPrefix(rel, "wireplumber/") {
			wpConfigPruned = true
		}
		_ = os.Remove(filepath.Join(dest, rel))
		pruneEmptyParents(dest, filepath.Dir(rel))
	}

	if err := overlayUserEdits(dest); err != nil {
		return err
	}
	_ = reapplyIconTheme(dest)
	for _, rel := range managed {
		if live, err := os.ReadFile(filepath.Join(dest, rel)); err == nil {
			if projection, ok := iconThemeProjections[rel]; ok && iconThemeState {
				live = stripINIProjection(live, projection)
			}
			hashes[rel] = hashBytes(live)
		}
	}
	if err := writeManifest(state, managed, hashes); err != nil {
		return fmt.Errorf(i18n.T("record manifest: %w"), err)
	}
	wirePlumberAfter, _ := os.ReadFile(filepath.Join(dest, wirePlumberPolicyRel))
	if wpConfigPruned || !bytes.Equal(wirePlumberBefore, wirePlumberAfter) {
		_ = service("--user", "try-restart", "wireplumber.service")
	}
	fmt.Printf(i18n.T("materialized %d files -> %s\n"), len(managed), dest)
	if len(kept) > 0 {
		fmt.Printf(i18n.T("kept your edits to %d shipped file(s) as forks under %s (a fork wins over updates; delete it to take Ryoku's version again):\n"), len(kept), sys.UserEditsDir())
		for _, rel := range kept {
			fmt.Printf("  %s\n", rel)
		}
	}
	applyGenerated(dest)
	seedAccount()
	return nil
}

// The quickshell tree is the shell itself; a stale fork there breaks it.
func forkable(rel string) bool {
	return !strings.HasPrefix(rel, "quickshell/") && !sys.IsLiveOwnedConfig(rel)
}

func hashBytes(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// walkRel: every regular file under root, as slash-separated paths relative
// to root, sorted.
func walkRel(root string) ([]string, error) {
	var rels []string
	err := filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.Type().IsRegular() {
			return nil
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		rels = append(rels, filepath.ToSlash(rel))
		return nil
	})
	sort.Strings(rels)
	return rels, err
}

func pruneEmptyParents(root, rel string) {
	for rel != "." && rel != "/" && rel != "" {
		dir := filepath.Join(root, rel)
		entries, err := os.ReadDir(dir)
		if err != nil || len(entries) > 0 {
			return
		}
		if err := os.Remove(dir); err != nil {
			return
		}
		rel = filepath.Dir(rel)
	}
}

// Manifest lines are "path<TAB>sha256"; an older line without a hash still
// prunes, it just cannot tell a hand edit apart.
func readManifest(path string) []string {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var out []string
	for _, line := range strings.Split(string(b), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			rel, _, _ := strings.Cut(line, "\t")
			out = append(out, rel)
		}
	}
	return out
}

func readManifestHashes(path string) map[string]string {
	out := map[string]string{}
	b, err := os.ReadFile(path)
	if err != nil {
		return out
	}
	for _, line := range strings.Split(string(b), "\n") {
		if rel, h, ok := strings.Cut(strings.TrimSpace(line), "\t"); ok && rel != "" && h != "" {
			out[rel] = h
		}
	}
	return out
}

func writeManifest(path string, rels []string, hashes map[string]string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	lines := make([]string, 0, len(rels))
	for _, rel := range rels {
		lines = append(lines, rel+"\t"+hashes[rel])
	}
	return os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o644)
}

// overlayUserEdits lays the user's override tree over the freshly materialized
// base: a regular file under ~/.config/ryoku/user_edits wins at the mirrored
// ~/.config path. Runs last, after the prune and the quickshell converge, so a
// fork is the final word and nothing sweeps it, while every base fix was still
// laid underneath first. An absent or empty overlay is a no-op.
func overlayUserEdits(dest string) error {
	rels, err := sys.UserEditFiles()
	if err != nil {
		return fmt.Errorf(i18n.T("scan overlay: %w"), err)
	}
	root := sys.UserEditsDir()
	for _, rel := range rels {
		// live-owned user files (hypr/user.lua, monitors_user.lua, kitty/user.conf)
		// are edited in place, never overlaid. A stale overlay copy from the retired
		// adopt step would otherwise re-lay a frozen snapshot over the live file
		// every update and wipe later hand edits; doctor migrates any such copy out.
		if sys.IsLiveOwnedConfig(rel) {
			continue
		}
		if err := sys.CopyFile(filepath.Join(root, rel), filepath.Join(dest, rel)); err != nil {
			return fmt.Errorf(i18n.T("overlay %s: %w"), rel, err)
		}
	}
	return nil
}
