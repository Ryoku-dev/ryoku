package main

import (
	"fmt"
	"os/exec"
	"time"
)

var storeSections = map[string]struct{}{
	"discover": {}, "library": {}, "rices": {}, "lockscreens": {},
	"subshells": {}, "barstyles": {}, "docks": {}, "launchers": {}, "osds": {}, "notifications": {}, "sidebars": {},
	"fastfetch": {}, "plugins": {}, "bundles": {}, "decors": {},
	"launcher-images": {}, "fastfetch-emblems": {}, "ryotunes-skins": {},
	"vesktop-themes": {}, "flatpak": {},
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

func runSettings(args []string) error {
	if len(args) < 1 || len(args) > 2 {
		return fmt.Errorf("settings needs <category> [id]")
	}
	if section, ok := editorSection(args[0]); ok {
		return openEditorSection(section)
	}
	section, ok := settingsSection(args[0])
	if !ok {
		return fmt.Errorf("unknown settings category %q", args[0])
	}
	return openConfig("hub", "/tmp/ryoku-hub.lock", section)
}

// openEditorSection opens the Stage Editor on one catalogue of the focused
// monitor. The shell is resident, so there is nothing to start first.
func openEditorSection(section string) error {
	if err := exec.Command("qs", "-c", "shell", "ipc", "call", "desktop", "editSection", section, "").Run(); err != nil {
		return fmt.Errorf("open the desktop editor's %s section: %w", section, err)
	}
	return nil
}

func openConfig(config, lock, section string) error {
	start := exec.Command("flock", "-n", "-o", lock, "qs", "-c", config)
	if err := start.Start(); err != nil {
		return fmt.Errorf("start %s: %w", config, err)
	}
	_ = start.Process.Release()

	var last error
	for attempt := 0; attempt < 30; attempt++ {
		call := exec.Command("qs", "-c", config, "ipc", "call", "nav", "open", section)
		if err := call.Run(); err == nil {
			return nil
		} else {
			last = err
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("open %s section %q: %w", config, section, last)
}
