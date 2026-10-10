package wm

import (
	"net"
	"os"
	"os/exec"
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
	ProviderWayfire  = "wayfire"
)

// RetiredCompositorArtifacts names the state a removed provider left on
// packaged systems. It is not a selectable provider; update and doctor use it
// only to move old installations onto a supported variant and remove residue.
type RetiredCompositorArtifacts struct {
	VariantPackage string
	Packages       []string
	ConfigDir      string
	ProviderBinary string
	SessionHandle  string
}

func RetiredCompositor() RetiredCompositorArtifacts {
	name := "mango"
	return RetiredCompositorArtifacts{
		VariantPackage: "ryoku-desktop-" + name,
		Packages:       []string{name + "wm"},
		ConfigDir:      name,
		ProviderBinary: "ryoku-wm-" + name,
		SessionHandle:  strings.ToUpper(name) + "_INSTANCE_SIGNATURE",
	}
}

// RetiredCompositorLive is deliberately conservative: a process carrying the
// retired session handle may still belong to that session, so cleanup waits
// until the user is in a supported one.
func RetiredCompositorLive() bool {
	return strings.TrimSpace(os.Getenv(RetiredCompositor().SessionHandle)) != ""
}

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
	// wayfire's ipc plugin exports the socket path itself.
	{"WAYFIRE_SOCKET", ProviderWayfire, func(h string) []string { return []string{h} }},
}

// SessionHandles names the environment variables a compositor session carries
// as its live IPC handle. Consumers that clean or clear the session environment
// (the login cutover, tests) derive the names here instead of repeating them:
// the seam owns which handles prove which session.
func SessionHandles() []string {
	out := make([]string, 0, len(envProviders))
	for _, p := range envProviders {
		out = append(out, p.env)
	}
	return out
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
	ProviderWayfire:  "wayfire",
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
	ProviderWayfire:  "wayfire.ini",
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

// gpuPinFile is the file each provider's ryoku-gpu writer owns inside the
// config tree: the drop-in the compositor reads for its render-device order.
// Empty for a provider that picks its own device (niri), for which a mode
// write has no effect and the installer skips it.
var gpuPinFile = map[string]string{
	ProviderHyprland: "gpu.lua",
}

// GpuPinFile returns a provider's render-pin file inside its config dir, or
// "" when the compositor has no ryoku-gpu writer.
func GpuPinFile(name string) string { return gpuPinFile[name] }

// GpuPinStore is the canonical policy store every mode write lands in and
// every reader (the Hub GPU page, doctor, the login persist) audits: the
// Hyprland drop-in, because the policy predates the seam and the other
// providers mirror it into their own dialect.
func GpuPinStore() string { return configDirs[ProviderHyprland] + "/gpu.lua" }

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
	// wayfire has no include mechanism, so apply composes wayfire.ini from
	// the shipped defaults, the store and these files, user.ini last.
	ProviderWayfire: {"monitors.ini", "keyboard.ini", "user.ini"},
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
	ProviderWayfire:  {"wayfire/user.ini", "wayfire/monitors.ini"},
}

// generatedConfig are the files a provider's apply authors from the store, as
// paths relative to ~/.config: the generated config and the user_edits overlay
// copy the updater re-lays. A pure function of the store, so clearing the store
// clears these, and a reset that restores pure defaults removes them.
var generatedConfig = map[string][]string{
	ProviderHyprland: {"hypr/settings.lua", "hypr/rebinds.lua", "ryoku/user_edits/hypr/settings.lua", "ryoku/user_edits/hypr/rebinds.lua"},
	ProviderNiri:     {"niri/settings.kdl", "niri/rebinds.kdl", "ryoku/user_edits/niri/settings.kdl", "ryoku/user_edits/niri/rebinds.kdl"},
	ProviderWayfire:  {"wayfire/wayfire.ini", "ryoku/user_edits/wayfire/wayfire.ini"},
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
func Providers() []string {
	return []string{ProviderHyprland, ProviderNiri, ProviderWayfire}
}

// compositorBins is the executable each provider's session runs. Named in the
// seam because nothing outside it may know a compositor's shape; its absence
// means the greeter would offer a session that cannot start.
var compositorBins = map[string]string{
	ProviderHyprland: "Hyprland",
	ProviderNiri:     "niri",
	ProviderWayfire:  "wayfire",
}

// sessionEntryDirs are the directories a greeter lists wayland-session entries
// from: SDDM's configured SessionDir first, then the XDG data dirs every
// greeter scans. A var so a test can point the scan at a sandbox tree.
var sessionEntryDirs = func() []string {
	dirs := []string{"/usr/local/share/wayland-sessions", "/usr/share/wayland-sessions"}
	for _, d := range filepath.SplitList(os.Getenv("XDG_DATA_DIRS")) {
		if d != "" {
			dirs = append(dirs, filepath.Join(d, "wayland-sessions"))
		}
	}
	return dirs
}

// SessionGap names what stops this provider from being entered at the greeter
// right now, "" when nothing does: "compositor" when its binary is not
// installed, "entry" when no greeter directory holds its session file. The
// switch asks before promising "log out and pick it": a provider deployed
// from a checkout has its binary and its config tree but may still have no
// compositor and no greeter entry, and the promise is then false.
func SessionGap(name string) string {
	if _, err := exec.LookPath(compositorBins[name]); err != nil {
		return "compositor"
	}
	entry := name + ".desktop"
	for _, d := range sessionEntryDirs() {
		if _, err := os.Stat(filepath.Join(d, entry)); err == nil {
			return ""
		}
	}
	return "entry"
}

// SessionReady reports whether picking this provider at the greeter now would
// start a session: its compositor is installed and a greeter lists its entry.
func SessionReady(name string) bool { return SessionGap(name) == "" }

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
