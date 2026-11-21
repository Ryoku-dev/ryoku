package main

import (
	"encoding/json"
	"os/exec"
	"strings"

	wm "ryoku-wm"
)

var capsManifest = []wm.Capability{
	wm.CapWorkspaces,
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
	caps := wm.Caps{
		Name:           "umbriel",
		Version:        probeVersion(),
		Instance:       instanceHandle(),
		Supports:       capsManifest,
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

func probeVersion() string {
	out, err := exec.Command("umbriel", "--version").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
