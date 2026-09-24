package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"ryoku-cli/internal/doctor"
	"ryoku-cli/internal/sys"
	"ryoku-cli/internal/updater"

	i18n "ryoku-i18n"
	wm "ryoku-wm"
)

// cmdWm is the neutral front door to the window-manager provider: what is running
// (status), where its config lives (config), dispatch an action from a script
// (act), print the provider's session entry (session), and switch compositors as
// a reversible package op (use).
func cmdWm(args []string) {
	if len(args) == 0 {
		wmUsage()
		os.Exit(2)
	}
	switch args[0] {
	case "status":
		cmdWmStatus()
	case "caps":
		cmdWmCaps()
	case "act":
		cmdWmAct(args[1:])
	case "session":
		cmdWmSession()
	case "config":
		cmdWmConfig(args[1:])
	case "reset-paths":
		cmdWmResetPaths()
	case "use":
		cmdWmUse(args[1:])
	default:
		die("unknown wm command: %s", args[0])
	}
}

func wmUsage() {
	fmt.Print(i18n.T("Usage: ryoku wm <command>\n\n  status            print the detected provider, its capabilities and workspace model\n  caps              print the active provider's capability manifest (JSON)\n  config [name]     print a provider's config dir and the files it owns (JSON)\n  reset-paths       print the config files a factory reset clears (one path per line)\n  use <name>        preview and switch to another compositor (installs its package)\n  act <id> [args]   dispatch a window-manager action through the provider\n  session           print the provider's wayland-session desktop entry\n"))
}

func cmdWmStatus() {
	c := wm.Open()
	d := c.Detection()
	name := d.Name
	if name == "" {
		name = i18n.T("(none)")
	}
	fmt.Printf(i18n.T("Provider: %s\n"), name)
	fmt.Printf(i18n.T("Live: %t\n"), d.Live)
	fmt.Printf(i18n.T("Source: %s\n"), d.Source)
	caps, err := c.Caps()
	if err != nil {
		fmt.Printf(i18n.T("Capabilities: unavailable (%v)\n"), err)
		return
	}
	if caps.Version != "" {
		fmt.Printf(i18n.T("Version: %s\n"), caps.Version)
	}
	fmt.Printf(i18n.T("Workspace model: %s\n"), caps.WorkspaceModel)
	supports := make([]string, 0, len(caps.Supports))
	for _, capability := range caps.Supports {
		supports = append(supports, string(capability))
	}
	sort.Strings(supports)
	fmt.Printf(i18n.T("Capabilities: %s\n"), strings.Join(supports, ", "))
	fmt.Printf(i18n.T("Setting domains: %s\n"), strings.Join(caps.SettingDomains, ", "))
}

// cmdWmCaps prints the active provider's capability manifest as indented JSON,
// so a script (ryoku-cmd-nightlight) reads nightLightProcess and the rest
// through one verb instead of probing the provider a second way.
func cmdWmCaps() {
	caps, err := wm.Open().Caps()
	if err != nil {
		die("%v", err)
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(caps); err != nil {
		die("%v", err)
	}
}

// cmdWmAct dispatches an action and maps the two seam errors to distinct exit
// codes so a caller (a keybind, lock.sh) can tell "no compositor" from "this
// compositor cannot". Quiet on success: it runs on the idle and lock paths.
func cmdWmAct(args []string) {
	if len(args) == 0 {
		die("usage: ryoku wm act <action> [args...]")
	}
	// ActOutput, not Act: an action that answers with a value (input.touchpad
	// status prints on|off) must reach the caller's stdout, while the silent
	// actions still print nothing because their output is empty.
	out, err := wm.Open().ActOutput(wm.Action(args[0]), args[1:]...)
	if err == nil {
		if out != "" {
			fmt.Fprintln(os.Stdout, out)
		}
		return
	}
	fmt.Fprintf(os.Stderr, "ryoku: %v\n", err)
	switch {
	case errors.Is(err, wm.ErrNoProvider):
		os.Exit(3)
	case errors.Is(err, wm.ErrUnsupported):
		os.Exit(4)
	}
	os.Exit(1)
}

// cmdWmSession writes the provider's wayland-session desktop entry verbatim, and
// nothing else, so sddm/setup can redirect it straight into a file.
func cmdWmSession() {
	out, err := wm.Open().Session()
	if err != nil {
		die("%v", err)
	}
	os.Stdout.Write(out)
}

// cmdWmConfig prints where a provider's config lives and which files belong to
// whom, as JSON for a script to read. The mapping already exists once in the
// seam (ryoku/wm/detect.go); exposing it here is what keeps deploy.sh and any
// other tool from keeping a second copy of a per-compositor file list.
func cmdWmConfig(args []string) {
	name := wm.Detect().Name
	if len(args) > 0 && strings.TrimSpace(args[0]) != "" {
		name = args[0]
	}
	if name == "" {
		die(i18n.T("no window manager detected; name one: %s"), strings.Join(wm.Providers(), ", "))
	}
	if !knownProvider(name) {
		die("unknown compositor %q; known: %s", name, strings.Join(wm.Providers(), ", "))
	}
	// Generated files come from the provider, which is the only thing that knows
	// what its apply authors; empty when that provider is not installed.
	generated := []string{}
	if caps, err := wm.OpenNamed(name).Caps(); err == nil {
		generated = caps.GeneratedFiles
	}
	payload := struct {
		Name      string   `json:"name"`
		Dir       string   `json:"dir"`
		Seeds     []string `json:"seeds"`
		UserOwned []string `json:"userOwned"`
		Generated []string `json:"generated"`
	}{
		Name:      name,
		Dir:       wm.ConfigDir(name),
		Seeds:     wm.ConfigSeeds(name),
		UserOwned: wm.ConfigUserOwned(name),
		Generated: generated,
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(payload); err != nil {
		die("%v", err)
	}
}

// cmdWmResetPaths prints, one path per line relative to ~/.config, the config
// files a factory reset clears for every provider the tree carries: each one's
// generated config and hand-edit files, never the per-machine seeds a reset
// keeps. Recovery reads this instead of hardcoding a compositor's file names, so
// the list stays in the seam. Pure Go on purpose: it answers with no compositor
// running and no provider binary built, which is what lets the rescue clear a
// box whose desktop is down.
func cmdWmResetPaths() {
	seen := map[string]bool{}
	var paths []string
	for _, name := range wm.Providers() {
		for _, rel := range wm.ResetPaths(name) {
			if rel == "" || seen[rel] {
				continue
			}
			seen[rel] = true
			paths = append(paths, rel)
		}
	}
	sort.Strings(paths)
	for _, rel := range paths {
		fmt.Println(rel)
	}
}

func cmdWmUse(args []string) {
	keepPrevious := true
	var rest []string
	for _, a := range args {
		switch a {
		case "--remove-previous":
			keepPrevious = false
		case "--keep-previous":
			keepPrevious = true
		default:
			rest = append(rest, a)
		}
	}
	if len(rest) == 0 {
		die("usage: ryoku wm use <name> [--keep-previous|--remove-previous]")
	}
	name := rest[0]
	if !knownProvider(name) {
		die("unknown compositor %q; known: %s", name, strings.Join(wm.Providers(), ", "))
	}
	store := filepath.Join(sys.ConfigHome(), "ryoku", "desktop.json")
	active := wm.Detect().Name
	if name == active {
		fmt.Printf(i18n.T("%s is already the compositor.\n"), name)
		return
	}

	// A dry run, not an apply: asking the target what it cannot honour must not
	// author its config as a side effect. Best-effort, since the provider may
	// not be installed yet.
	report, applyErr := wm.OpenNamed(name).DryRun(store)
	printWmSwitchPreview(name, active, store, report, applyErr)

	// The keep-or-remove tradeoff in the terms that are actually true: what
	// leaving the active compositor reclaims, or that there is nothing to remove.
	printWmPreviousChoice(active, name, keepPrevious)

	pkg := "ryoku-desktop-" + name
	// A checkout box runs deployed trees, so switching to one installs nothing;
	// picking the session at the greeter is the whole move. A package box installs
	// the target first, as a plain pacman transaction (no SNAP_PAC_SKIP) so
	// snap-pac snapshots it and `ryoku rollback` can undo the switch.
	if deployedProvider(name) {
		// A checkout box runs deployed trees, so the switch installs no package,
		// but the leaf scripts (ryoku-monitor and friends) the target's config
		// and autostart call by bare name live in ~/.local/bin, and deploy.sh
		// lays only the LIVE provider's. Switching to a compositor whose scripts
		// an earlier deploy deleted would leave the next session's bare-name
		// calls falling through PATH to whatever stale copy sits there (or to
		// nothing). Lay the target's current scripts before declaring it ready.
		if err := syncLeafScripts(name); err != nil {
			die(i18n.T("switched to %s but could not lay its desktop scripts (%v); run `ryoku deploy` before logging out"), name, err)
		}
		fmt.Printf(i18n.T("%s is ready. Log out and pick %s at the greeter.\n"), name, name)
	} else {
		if !packageAvailable(pkg) {
			die(i18n.T("cannot switch to %s yet: the %s package is not available on this channel"), name, pkg)
		}
		// The variants are not exclusive, so the install leaves the outgoing
		// compositor in place and "keep" means what it says. A box whose packages
		// predate that still declares the shared virtual as a conflict and pacman
		// refuses the install under --noconfirm; only then drop it first.
		if err := sys.Sudo("pacman", "-S", "--needed", "--noconfirm", pkg); err != nil {
			out := "ryoku-desktop-" + active
			if active == "" || active == name || !packageInstalled(out) {
				die(i18n.T("could not install %s: %v"), pkg, err)
			}
			if err := sys.Sudo("pacman", "-Rdd", "--noconfirm", out); err != nil {
				die(i18n.T("could not install %s, and could not remove %s first: %v"), pkg, out, err)
			}
			if err := sys.Sudo("pacman", "-S", "--needed", "--noconfirm", pkg); err != nil {
				die(i18n.T("could not install %s after removing %s: %v"), pkg, out, err)
			}
		}
		fmt.Printf(i18n.T("Installed %s; %s is the compositor at the next login.\n"), pkg, name)
	}

	// The switch is not complete until the target's config is laid down. A package
	// install writes nothing into ~/.config, so a switch that stopped at the
	// package left the next login on a bare compositor (no keybinds, no shell, a
	// grey desktop) until an update happened to materialize it. Runs after the
	// install, so the base carries the target's tree, and before any removal, so a
	// refusal there still leaves a working desktop. A checkout box deploys its own
	// trees and has no packaged base to lay.
	if sys.Exists(sys.BaseConfigDir()) {
		if err := laySwitchConfig(); err != nil {
			die(i18n.T("installed %s but could not lay its config down (%v); run `ryoku materialize` before logging out"), pkg, err)
		}
		fmt.Printf(i18n.T("Laid down the %s config; log out and pick %s at the greeter.\n"), name, name)
	}

	// Removing the outgoing compositor's packages is a second transaction on
	// purpose: the switch is complete once the target is ready, so a refusal or
	// failure here leaves a working desktop rather than a half-switched one. It
	// runs whether the target came from a package or a checkout, because the old
	// compositor is a pacman package set either way.
	if !keepPrevious && active != "" && active != name {
		removePreviousCompositor(active, name)
	}
}

// laySwitchConfig brings the target desktop up as a desktop: materialize lays
// the shipped tree, the doctor reconciles the rest (session target, portal
// routing, wallpaper). Running `ryoku doctor` by hand was the switch's own gap.
func laySwitchConfig() error {
	if err := updater.Materialize(); err != nil {
		return err
	}
	if err := doctor.Run(nil); err != nil {
		// The tree is down, so the desktop comes up; a reconciler that could not
		// finish (a privileged fix with no terminal) is worth saying, not failing.
		fmt.Printf(i18n.T("  Some post-switch checks did not finish (%v); run `ryoku doctor` after logging in.\n"), err)
	}
	return nil
}

// deployedProvider reports whether a provider is usable without its package:
// its binary answers caps and its config tree exists, which is what a checkout
// deploy leaves behind. Both must hold, since a provider with no config tree
// would start a bare compositor.
func deployedProvider(name string) bool {
	if _, err := wm.OpenNamed(name).Caps(); err != nil {
		return false
	}
	dir := wm.ConfigDir(name)
	if dir == "" {
		return false
	}
	_, err := os.Stat(filepath.Join(sys.ConfigHome(), dir))
	return err == nil
}

// syncLeafScripts lays a checkout box's target-compositor leaf scripts into
// ~/.local/bin so the next session's bare-name calls (ryoku-monitor from the
// display seam, ryoku-workspace from a keybind) resolve to the current copy
// instead of falling through PATH to a stale one. It is additive on purpose: the
// running session is still the outgoing compositor, so deleting its scripts here
// would break its keybinds before logout; deploy.sh and the package own pruning.
// It runs only on a genuine checkout (ResolveRepo): a packaged box ships these in
// /usr/bin, and dropping a copy in ~/.local/bin would shadow it, which is exactly
// the drift the dev-residue doctor heals. A provider with no scripts dir (niri)
// ships none and is a no-op.
func syncLeafScripts(name string) error {
	repo := sys.ResolveRepo()
	if repo == "" {
		return nil
	}
	src := filepath.Join(repo, wm.LeafScriptsDir(name))
	if !sys.Exists(src) {
		return nil
	}
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	bindir := filepath.Join(sys.Home(), ".local", "bin")
	for _, e := range entries {
		if e.IsDir() || !strings.HasPrefix(e.Name(), "ryoku-") {
			continue
		}
		if err := sys.CopyFile(filepath.Join(src, e.Name()), filepath.Join(bindir, e.Name())); err != nil {
			return fmt.Errorf("lay %s: %w", e.Name(), err)
		}
	}
	return nil
}

// printWmPreviousChoice states the tradeoff in the terms that are actually true:
// how many packages leaving the active compositor reclaims and how much space,
// or that nothing of it is installed to remove. The settings surviving either
// way is what makes a removal safe, and is stated by the preview above.
func printWmPreviousChoice(active, incoming string, keep bool) {
	if active == "" || active == incoming {
		return
	}
	rs, err := wm.Reclaim(active, incoming)
	if err != nil || !rs.Removable {
		fmt.Printf(i18n.T("  Nothing to remove: no installed %s packages can be reclaimed, so switching back costs nothing.\n"), active)
		return
	}
	if keep {
		fmt.Printf(i18n.T("  Keeping %s: switch back with no download, at the cost of %d packages (%s) staying on disk.\n"), active, rs.Count, humanSize(rs.Size))
		return
	}
	fmt.Printf(i18n.T("  Removing %s: frees %d packages (%s) and drops its session entry; switching back later reinstalls them.\n"), active, rs.Count, humanSize(rs.Size))
}

// removePreviousCompositor removes the outgoing compositor's packages after a
// switch to incoming, in one pacman transaction over exactly the reviewed set,
// leaving its config tree and its wm.<name>.* settings alone so a switch back
// restores the desktop rather than a default one. It refuses rather than remove
// anything pacman's own plan no longer agrees with, so a switch can never
// cascade past what the preview showed.
func removePreviousCompositor(active, incoming string) {
	rs, err := wm.Reclaim(active, incoming)
	if err != nil {
		fmt.Printf(i18n.T("Switched, but %s could not be measured for removal: %v\n"), active, err)
		return
	}
	if !rs.Removable {
		return // nothing installed to reclaim
	}
	if err := wm.VerifyRemoval(rs); err != nil {
		fmt.Printf(i18n.T("Switched, but %s was kept: %v\n"), active, err)
		return
	}
	rmArgs := append([]string{"pacman", "-Rns", "--noconfirm"}, rs.Targets...)
	if err := sys.Sudo(rmArgs...); err != nil {
		fmt.Printf(i18n.T("Switched, but %s could not be removed: %v\n"), active, err)
		return
	}
	fmt.Printf(i18n.T("Removed %s: reclaimed %d packages (%s).\n"), active, rs.Count, humanSize(rs.Size))
}

// humanSize names a byte count the way pacman's own removal summary does, so a
// reclaimed size reads the same in `ryoku wm use` as in pacman.
func humanSize(n int64) string {
	if n >= 1<<30 {
		return fmt.Sprintf("%.2f GiB", float64(n)/(1<<30))
	}
	return fmt.Sprintf("%.2f MiB", float64(n)/(1<<20))
}

func knownProvider(name string) bool {
	for _, p := range wm.Providers() {
		if p == name {
			return true
		}
	}
	return false
}

func printWmSwitchPreview(name, active, store string, report wm.ApplyReport, applyErr error) {
	fmt.Printf(i18n.T("Preview: switch to %s\n"), name)
	if kb := storeKeybindCount(store); kb > 0 {
		fmt.Printf(i18n.T("  Keybinds: %d carry over unchanged (compositor-neutral).\n"), kb)
	}
	fmt.Print(i18n.T("  Settings: every desktop.* setting carries over; the target honours what it can.\n"))
	switch {
	case applyErr != nil:
		fmt.Printf(i18n.T("  Unavailable features: install %s to preview the exact list.\n"), "ryoku-desktop-"+name)
	case len(report.Unhonored) == 0:
		fmt.Printf(i18n.T("  Unavailable features: none; %s honours every current setting.\n"), name)
	default:
		fmt.Printf(i18n.T("  Unavailable on %s:\n"), name)
		for _, u := range report.Unhonored {
			fmt.Printf("    - %s: %s\n", u.Key, u.Reason)
		}
	}
	if active != "" && active != name {
		fmt.Printf(i18n.T("  Your wm.%s.* settings stay in the store and return if you switch back.\n"), active)
	}
}

// storeKeybindCount counts the neutral keybinds in the store for the honest
// carry-over line; zero when the store or the field is absent.
func storeKeybindCount(store string) int {
	raw, err := os.ReadFile(store)
	if err != nil {
		return 0
	}
	var doc struct {
		Desktop struct {
			Keybinds       json.RawMessage `json:"keybinds"`
			KeybindRebinds json.RawMessage `json:"keybindRebinds"`
		} `json:"desktop"`
	}
	if json.Unmarshal(raw, &doc) != nil {
		return 0
	}
	return rawLen(doc.Desktop.Keybinds) + rawLen(doc.Desktop.KeybindRebinds)
}

// rawLen counts entries in a JSON array or object, or 0 for anything else.
func rawLen(raw json.RawMessage) int {
	if len(raw) == 0 {
		return 0
	}
	var arr []json.RawMessage
	if json.Unmarshal(raw, &arr) == nil {
		return len(arr)
	}
	var obj map[string]json.RawMessage
	if json.Unmarshal(raw, &obj) == nil {
		return len(obj)
	}
	return 0
}

func packageAvailable(pkg string) bool {
	_, err := sys.RunOut("pacman", "-Si", pkg)
	return err == nil
}

// packageInstalled reports whether pkg is installed here, which decides whether
// a switch has an outgoing variant package to drop.
func packageInstalled(pkg string) bool {
	_, err := sys.RunOut("pacman", "-Qq", pkg)
	return err == nil
}
