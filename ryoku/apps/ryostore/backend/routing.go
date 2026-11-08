package main

import (
	"fmt"
	"os/exec"
	"strings"
	"time"
)

var storeSections = map[string]struct{}{
	"discover": {}, "library": {}, "rices": {}, "lockscreens": {},
	"subshells": {}, "barstyles": {}, "docks": {}, "launchers": {}, "osds": {}, "notifications": {}, "sidebars": {},
	"fastfetch": {}, "plugins": {}, "bundles": {}, "decors": {},
	"launcher-images": {}, "fastfetch-emblems": {}, "ryotunes-skins": {},
	"vesktop-themes": {}, "flatpak": {}, "profiles": {},
}

func storeSection(section string) bool {
	_, ok := storeSections[section]
	return ok
}

// settingsSection maps a Store category to the Hub page that applies or manages
// it. The lockscreen page only edits config, not a store install, and the
// flat-image categories (decors, launcher images) have no page at all, so none
// of them appear here and none shows an "open in settings" action.
func settingsSection(category string) (string, bool) {
	section, ok := map[string]string{
		"rices":     "appearance",
		"plugins":   "addons",
		"bundles":   "addons",
		"barstyles": "bar-studio",
		"subshells": "subshells",
		"launchers": "launcher",
		// The overlays are worn on one Hub page.
		"osds":          "overlays",
		"notifications": "overlays",
		"sidebars":      "overlays",
		"fastfetch":     "fastfetch",
		"profiles":      "users",
	}[category]
	return section, ok
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
	return hub || editor
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
	section, ok := settingsSection(args[0])
	if !ok {
		return fmt.Errorf("unknown settings category %q", args[0])
	}
	return ryokuShell("hub", "open", section)
}

func ryokuShell(args ...string) error {
	out, err := exec.Command("ryoku-shell", args...).CombinedOutput()
	if err != nil {
		if msg := strings.TrimSpace(string(out)); msg != "" {
			return fmt.Errorf("ryoku-shell %s: %s", strings.Join(args, " "), msg)
		}
		return fmt.Errorf("ryoku-shell %s: %w", strings.Join(args, " "), err)
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
