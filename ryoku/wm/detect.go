package wm

import (
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// The only place allowed to care which compositor is running. It resolves an
// identity into a provider name once; every later decision reads Caps.

// Provider names name a binary (ryoku-wm-<name>) and a settings domain
// (wm.<name>.*), so they are part of the on-disk contract.
const (
	ProviderHyprland = "hyprland"
	ProviderNiri     = "niri"
)

type Detection struct {
	Name string
	// Live means the compositor is running for this user, so actions and watch
	// are meaningful. False means the box is configured for it and only caps
	// and apply are safe: doctor runs in a chroot right after install, where
	// the config tree exists but no compositor does.
	Live bool
	// Source records what answered, for doctor output and bug reports.
	Source string
}

// Presence of an instance handle beats XDG_CURRENT_DESKTOP, which a user or a
// greeter can set to anything.
//
// The handle must still ANSWER, not merely be set. A terminal, a tmux server or
// a service started under one compositor keeps that compositor's handle in its
// environment for as long as it lives, so after a switch a stale handle would
// name the compositor that is gone: every later action would be dispatched at a
// dead socket and silently fail. socket resolves the handle to the socket that
// proves the session is alive.
var envProviders = []struct {
	env    string
	name   string
	socket func(handle string) []string
}{
	{"HYPRLAND_INSTANCE_SIGNATURE", ProviderHyprland, hyprlandSockets},
	// niri exports the socket path itself.
	{"NIRI_SOCKET", ProviderNiri, func(h string) []string { return []string{h} }},
}

// Hyprland's socket dir survives the instance that made it, so the path
// existing is not proof; only a dial is.
func hyprlandSockets(sig string) []string {
	var out []string
	if rt := os.Getenv("XDG_RUNTIME_DIR"); rt != "" {
		out = append(out, filepath.Join(rt, "hypr", sig, ".socket.sock"))
	}
	return append(out, filepath.Join("/tmp", "hypr", sig, ".socket.sock"))
}

// handleAlive dials the handle's socket. A refused or missing socket means the
// session it names has exited.
func handleAlive(sockets []string) bool {
	for _, p := range sockets {
		if p == "" {
			continue
		}
		c, err := net.DialTimeout("unix", p, 200*time.Millisecond)
		if err == nil {
			_ = c.Close()
			return true
		}
	}
	return false
}

// Every compositor-scoped delivery allowlist derives from this, so the mapping
// lives here once instead of in materialize, user_edits and doctor.
var configDirs = map[string]string{
	ProviderHyprland: "hypr",
	ProviderNiri:     "niri",
}

func ConfigDir(name string) string { return configDirs[name] }

// LeafScriptsDir is where a provider keeps the standalone programs its config
// and autostart call by bare name (ryoku-monitor, ryoku-workspace), as a path
// relative to the repository root. The repo payload dir is named by the provider
// itself; a provider with no such scripts (niri's are compositor actions or
// `spawn ryoku-shell`) has no dir, and the caller treats its absence as "ships
// none". This is the one definition of that layout; deploy.sh and the switch
// both resolve scripts through it so a new provider's dir is named in one place.
func LeafScriptsDir(name string) string {
	if name == "" {
		return ""
	}
	return "ryoku/" + name + "/scripts"
}

// configEntries is the file each provider's config tree is read from: the one
// whose absence means the compositor boots its own defaults instead of Ryoku's
// tree (no keybinds, no autostart). Named per provider because it is the
// provider's own file, in its own format.
var configEntries = map[string]string{
	ProviderHyprland: "hyprland.lua",
	ProviderNiri:     "config.kdl",
}

// ConfigEntry returns the entry point of a provider's config tree as a path
// relative to ~/.config, or "" for an unknown provider.
func ConfigEntry(name string) string {
	dir, leaf := configDirs[name], configEntries[name]
	if dir == "" || leaf == "" {
		return ""
	}
	return dir + "/" + leaf
}

// configSeeds are the per-machine files under a provider's config dir that are
// seeded once and then owned by the machine: the runtime rewrites them (display
// and GPU pins) or the user edits them in place. Delivery reads this for EVERY
// provider, not just the active one, so an update under one compositor never
// prunes another's. Naming them per provider matters because the file names are
// the compositor's own, not a shared shape.
var configSeeds = map[string][]string{
	ProviderHyprland: {"monitors.lua", "gpu.lua", "keyboard.lua", "user.lua"},
	// niri seeds its hand-edit file too, unlike Hyprland: config.kdl includes
	// monitors_user.kdl by name and a missing include is a hard config error,
	// so the file has to exist from first boot. Being a seed is also what stops
	// an update re-laying it over a user's edits.
	ProviderNiri: {"monitors.kdl", "gpu.kdl", "keyboard.kdl", "user.kdl", "monitors_user.kdl"},
}

// ConfigSeeds returns the seeded, machine-owned files for a provider, as paths
// relative to ~/.config. Empty for an unknown provider.
func ConfigSeeds(name string) []string {
	dir := configDirs[name]
	if dir == "" {
		return nil
	}
	out := make([]string, 0, len(configSeeds[name]))
	for _, leaf := range configSeeds[name] {
		out = append(out, dir+"/"+leaf)
	}
	return out
}

// ConfigUserOwned are the files a user edits in place under a provider's config
// dir. The overlay must never lay a frozen copy over one, or an update would
// silently wipe an edit made after it was captured.
func ConfigUserOwned(name string) []string {
	seeds := ConfigSeeds(name)
	dir := configDirs[name]
	if dir == "" {
		return nil
	}
	// Hyprland's monitors_user.lua is optional, so it is hand-created rather
	// than seeded; niri's equivalent is already in its seed list above.
	if name == ProviderHyprland {
		return append(seeds, dir+"/monitors_user.lua")
	}
	return seeds
}

// configFiles are the hand-edit escape hatches a user owns under a provider's
// config dir, as paths relative to ~/.config, most useful first. The Hub offers
// these to open, and a factory reset clears them, because they are the user's
// own config rather than the machine state the seeds hold. The raw-config
// include is first, so a caller that wants one file wants this one.
var configFiles = map[string][]string{
	ProviderHyprland: {"hypr/user.lua", "hypr/monitors_user.lua", "hypr/modules"},
	ProviderNiri:     {"niri/user.kdl", "niri/monitors_user.kdl"},
}

// generatedConfig are the files a provider's apply authors from the store, as
// paths relative to ~/.config: the generated config and the user_edits overlay
// copy the updater re-lays. A pure function of the store, so clearing the store
// clears these, and a reset that restores pure defaults removes them.
var generatedConfig = map[string][]string{
	ProviderHyprland: {"hypr/settings.lua", "hypr/rebinds.lua", "ryoku/user_edits/hypr/settings.lua", "ryoku/user_edits/hypr/rebinds.lua"},
	ProviderNiri:     {"niri/settings.kdl", "niri/rebinds.kdl", "ryoku/user_edits/niri/settings.kdl", "ryoku/user_edits/niri/rebinds.kdl"},
}

// ConfigFiles are a provider's user-editable config paths (the hand-edit escape
// hatches the Hub offers), relative to ~/.config. Empty for an unknown provider.
func ConfigFiles(name string) []string {
	return append([]string(nil), configFiles[name]...)
}

// GeneratedConfig are the config files a provider's apply authors, relative to
// ~/.config. Empty for an unknown provider.
func GeneratedConfig(name string) []string {
	return append([]string(nil), generatedConfig[name]...)
}

// ResetPaths are the config files a factory reset removes for a provider, as
// paths relative to ~/.config: its generated config and its hand-edit files. It
// never names the per-machine seeds (the display, GPU and keyboard pins), which
// a reset keeps, since those are not among these. Pure Go, so recovery can ask
// for the set with no compositor running and no provider binary built, which is
// what lets the rescue clear a box whose desktop is down.
func ResetPaths(name string) []string {
	return append(GeneratedConfig(name), ConfigFiles(name)...)
}

// Providers is stable order, so generated config and installer prompts do not
// reshuffle between runs.
func Providers() []string { return []string{ProviderHyprland, ProviderNiri} }

// Detect resolves the active provider: RYOKU_WM so a developer can drive one
// without that session, then a live session handle, then XDG_CURRENT_DESKTOP,
// then a config tree.
func Detect() Detection {
	if forced := strings.TrimSpace(os.Getenv("RYOKU_WM")); forced != "" {
		return Detection{Name: forced, Live: liveFor(forced), Source: "RYOKU_WM"}
	}
	for _, p := range envProviders {
		if h := os.Getenv(p.env); h != "" && handleAlive(p.socket(h)) {
			return Detection{Name: p.name, Live: true, Source: p.env}
		}
	}
	// Colon-separated and case-inconsistent across greeters, so match a
	// component rather than the whole string.
	desktop := strings.ToLower(os.Getenv("XDG_CURRENT_DESKTOP"))
	for _, name := range Providers() {
		for _, part := range strings.Split(desktop, ":") {
			if strings.TrimSpace(part) == name {
				return Detection{Name: name, Live: true, Source: "XDG_CURRENT_DESKTOP"}
			}
		}
	}
	home, err := os.UserHomeDir()
	if err == nil {
		for _, name := range Providers() {
			dir := configDirs[name]
			if dir == "" {
				continue
			}
			if _, err := os.Stat(filepath.Join(home, ".config", dir)); err == nil {
				return Detection{Name: name, Live: false, Source: "config tree"}
			}
		}
	}
	return Detection{}
}

// liveFor stops a forced provider claiming liveness it cannot back.
func liveFor(name string) bool {
	for _, p := range envProviders {
		if p.name != name {
			continue
		}
		if h := os.Getenv(p.env); h != "" && handleAlive(p.socket(h)) {
			return true
		}
	}
	return false
}
