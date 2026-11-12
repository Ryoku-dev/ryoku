package main

import (
	"fmt"
	"os/exec"
	"strings"
	"time"
)

var storeSections = map[string]struct{}{
	"discover": {}, "library": {}, "rices": {}, "lockscreens": {}, "colorschemes": {},
	"subshells": {}, "barstyles": {}, "docks": {}, "launchers": {}, "osds": {}, "notifications": {}, "sidebars": {},
	"fastfetch": {}, "plugins": {}, "omarchy-plugins": {}, "bundles": {}, "decors": {},
	"launcher-images": {}, "fastfetch-emblems": {}, "ryotunes-skins": {},
	"vesktop-themes": {}, "flatpak": {}, "profiles": {},
}

func storeSection(section string) bool {
	_, ok := storeSections[section]
	return ok
}

// settingsSection maps a Store category to the Settings page, or "page/row",
// that applies or manages it. The lockscreen page only edits config, not a
// store install, and the flat-image categories (decors, launcher images) have
// no page at all, so none of them appear here and none shows an "open in
// settings" action.
func settingsSection(category string) (string, bool) {
	section, ok := map[string]string{
		"plugins":       "add-ons",
		"bundles":       "add-ons",
		"barstyles":     "bar",
		"subshells":     "bar",
		"launchers":     "launcher",
		"osds":          "notifications",
		"notifications": "notifications",
		"sidebars":      "quick-settings/panel-design",
		"fastfetch":     "terminal",
		"profiles":      "users",
	}[category]
	return section, ok
}

// itemRows prefixes the Add-ons row that opens one item's own page
var itemRows = map[string]string{"plugins": "add-on:", "bundles": "bundle:"}

// rices are worn from the wallpaper picker's Rices collection, not Settings
var ryogamiRoutes = map[string][]string{
	"rices": {"wallpaper", "collection", "rices"},
}

// editorSection maps a Store category the desktop editor wears to its Stage
// Editor catalogue: a dock is picked in Edit desktop -> Dock, not in the Hub.
func editorSection(category string) (string, bool) {
	section, ok := map[string]string{"docks": "dock"}[category]
	return section, ok
}

func hasSettingsSection(category string) bool {
	_, hub := settingsSection(category)
	_, editor := editorSection(category)
	_, ryogami := ryogamiRoutes[category]
	return hub || editor || ryogami
}

func runOpen(args []string) error {
	if len(args) != 1 || !storeSection(args[0]) {
		return fmt.Errorf("open needs a valid section")
	}
	return openConfig("ryostore", "/tmp/ryostore.lock", args[0])
}

// the shell daemon owns the Hub's single instance, so every route goes through it
func runSettings(args []string) error {
	if len(args) < 1 || len(args) > 2 {
		return fmt.Errorf("settings needs <category> [id]")
	}
	if args[0] == profilesCategory {
		if len(args) != 2 || !productIDPattern.MatchString(args[1]) {
			return fmt.Errorf("settings profiles needs a valid profile id")
		}
		return ryokuShell("hub", "open", "users", args[1])
	}
	if section, ok := editorSection(args[0]); ok {
		// an empty monitor means the focused one
		return ryokuShell("ipc", "call", "desktop", "editSection", section, "")
	}
	if route, ok := ryogamiRoutes[args[0]]; ok {
		return runReporting("ryogami", route...)
	}
	section, ok := settingsSection(args[0])
	if !ok {
		return fmt.Errorf("unknown settings category %q", args[0])
	}
	if prefix, ok := itemRows[args[0]]; ok && len(args) == 2 {
		if !productIDPattern.MatchString(args[1]) {
			return fmt.Errorf("settings %s needs a valid id", args[0])
		}
		section += "/" + prefix + args[1]
	}
	return ryokuShell("hub", "open", section)
}

func ryokuShell(args ...string) error { return runReporting("ryoku-shell", args...) }

// runReporting runs a hand-off command and reports its refusal to the caller
func runReporting(name string, args ...string) error {
	out, err := exec.Command(name, args...).CombinedOutput()
	if err != nil {
		if msg := strings.TrimSpace(string(out)); msg != "" {
			return fmt.Errorf("%s %s: %s", name, strings.Join(args, " "), msg)
		}
		return fmt.Errorf("%s %s: %w", name, strings.Join(args, " "), err)
	}
	return nil
}

func openConfig(config, lock, section string) error {
	start := exec.Command("flock", "-n", "-o", lock, "qs", "-c", config)
	if err := start.Start(); err != nil {
		return fmt.Errorf("start %s: %w", config, err)
	}
	_ = start.Process.Release()

	if err := retryConfigCall(config, "nav", "open", section); err != nil {
		return fmt.Errorf("open %s section %q: %w", config, section, err)
	}
	return nil
}

func retryConfigCall(config string, args ...string) error {
	var last error
	command := append([]string{"-c", config, "ipc", "call"}, args...)
	for range 30 {
		call := exec.Command("qs", command...)
		if err := call.Run(); err == nil {
			return nil
		} else {
			last = err
		}
		time.Sleep(100 * time.Millisecond)
	}
	return last
}
