package main

import (
	"encoding/json"
	"os/exec"
	"strings"

	wm "ryoku-wm"
)

var capsManifest = []wm.Capability{
	wm.CapWorkspaces,
	wm.CapFocusHistory,
	wm.CapWindowGeometry,
	wm.CapWindowFloat,
	wm.CapOutputPower,
	wm.CapMonitorConfig,
	wm.CapKeyboardLayoutSwitch,
	wm.CapTouchpadToggle,
	wm.CapSessionExit,
	// Umbriel draws its own overview over the scrolling layout, including live
	// window cards, and publishes an open/closed event for it. The shell should
	// stand aside rather than draw its own Hyprland-style overview.
	wm.CapNativeOverview,
	wm.CapOverviewState,
}

func runCaps() error {
	supports := append([]wm.Capability(nil), capsManifest...)
	if outputsSupportVrr() {
		supports = append(supports, wm.CapOutputVrr)
	}
	caps := wm.Caps{
		Name:           "umbriel",
		Version:        probeVersion(),
		Instance:       instanceHandle(),
		Supports:       supports,
		WorkspaceModel: wm.WorkspaceModelDynamic,
		SettingDomains: []string{"desktop", "wm.umbriel"},
		ConfigFiles:    wm.ConfigFiles("umbriel"),
		GeneratedFiles: wm.GeneratedConfig("umbriel"),
		PortalBackend:  "umbriel",
		Packages: []string{
			"ryoku-desktop-umbriel", "umbriel", "xorg-xwayland",
			"xdg-desktop-portal-umbriel", "wlsunset",
		},
	}
	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(caps)
}

// outputsSupportVrr checks Umbriel's own state. adaptive_sync is null when the
// output cannot use variable refresh, so the setting is claimed only when an
// output actually reports it.
func outputsSupportVrr() bool {
	raw, err := query("outputs")
	if err != nil {
		return false
	}
	var outputs []struct {
		AdaptiveSync json.RawMessage `json:"adaptive_sync"`
	}
	if json.Unmarshal(raw, &outputs) != nil {
		return false
	}
	for _, o := range outputs {
		value := strings.TrimSpace(string(o.AdaptiveSync))
		if value != "" && value != "null" {
			return true
		}
	}
	return false
}

func probeVersion() string {
	out, err := exec.Command("umbriel", "--version").Output()
	if err != nil {
		return ""
	}
	f := strings.Fields(string(out))
	if len(f) < 3 {
		return ""
	}
	return f[2]
}
