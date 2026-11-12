package main

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	wm "ryoku-wm"
)

// apply, defaults and session: the write half of the seam for Hyprland. apply
// renders the neutral store into settings.lua and rebinds.lua and reports what
// it could not express; defaults hands the Hub the
// baseline it overlays user values on; session prints the wayland-session entry
// the installer writes when the compositor package shipped none.

// runApply renders the neutral store into the Hyprland config and prints an
// ApplyReport. With --preview it pushes the state live and writes nothing;
// otherwise it writes the two generated modules and the caller reloads.
func runApply(args []string) error {
	preview := false
	storePath := ""
	for _, a := range args {
		if a == "--preview" {
			preview = true
			continue
		}
		if storePath == "" {
			storePath = a
		}
	}
	if storePath == "" {
		return fmt.Errorf("apply: missing store path")
	}
	o := loadStore(storePath)

	// --preview writes no config. It pushes what it can live, and it still
	// reports what it cannot honour: DryRun asks a provider that question
	// before a switch hands it the config, so an empty list here would tell a
	// user moving to Hyprland that nothing would be lost.
	if preview {
		pushEval(liveLua(o))
		rep := wm.ApplyReport{
			Provider:     wm.ProviderHyprland,
			Unhonored:    unhonored(storePath),
			ReloadNeeded: false,
		}
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(rep)
	}

	follow := borderFollowsPalette(o)
	if err := writeOverlayLua("settings.lua", []byte(genLua(o, follow))); err != nil {
		return err
	}
	if err := writeOverlayLua("rebinds.lua", renderRebinds(o)); err != nil {
		return err
	}
	wm.PublishGreeterNumlock(o.Input.NumlockByDefault)
	rep := wm.ApplyReport{
		Provider: wm.ProviderHyprland,
		Written: []string{
			filepath.Join(hyprConfigDir(), "settings.lua"),
			filepath.Join(hyprConfigDir(), "rebinds.lua"),
		},
		Unhonored:    unhonored(storePath),
		ReloadNeeded: true,
	}
	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(rep)
}

// hyprlandDesktopFields are the desktop.* top-level keys this provider turns into
// Hyprland config. A key present in the store but absent here has no expression
// on Hyprland and is reported unhonored rather than silently dropped.
var hyprlandDesktopFields = map[string]bool{
	"appearance": true, "input": true, "cursor": true, "env": true,
	"windowRules": true, "appOverrides": true, "autostart": true,
	"keybinds": true, "keybindRebinds": true, "unbinds": true, "apps": true,
	"keybindDisabled": true,
	"windows":         true,
}

// unhonored names every desktop.* key the store carries that Hyprland cannot
// express. Empty in practice today; the mechanism is what lets a future neutral
// key added for another compositor surface here instead of vanishing.
func unhonored(storePath string) []wm.Unhonored {
	ns, ok := readNeutralStore(storePath)
	if !ok {
		return nil
	}
	var out []wm.Unhonored
	for k := range ns.Desktop {
		if !hyprlandDesktopFields[k] {
			out = append(out, wm.Unhonored{
				Key:    "desktop." + k,
				Reason: "Hyprland has no setting for this.",
			})
		}
	}
	return out
}

// runDefaults prints the provider's default subtree of the neutral store. The
// keyboard baseline and the animatable inventory (every overridable leaf and the
// default bezier curves) are probed from the live compositor so the GUI need not
// fork hyprctl itself; the hardcoded defaults stand when nothing answers.
func runDefaults() error {
	o := defaultOverrides()
	liveKbDefaults(&o.Input)
	o.Anim.Items, o.Anim.Curves = probeAnimations()
	ns, err := splitStore(o)
	if err != nil {
		return err
	}
	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(ns)
}

// probeAnimations reads the compositor's animation tree: every overridable,
// non-internal leaf as an AnimItem at its baseline, and the default bezier curves.
// Empty slices when nothing answers, so the defaults payload always carries them.
func probeAnimations() ([]AnimItem, []AnimCurve) {
	items, curves := []AnimItem{}, []AnimCurve{}
	out, err := ctl("animations", "-j")
	if err != nil {
		return items, curves
	}
	var raw []json.RawMessage
	if json.Unmarshal(out, &raw) != nil || len(raw) < 2 {
		return items, curves
	}
	var anims []struct {
		Name       string  `json:"name"`
		Overridden bool    `json:"overridden"`
		Enabled    bool    `json:"enabled"`
		Speed      float64 `json:"speed"`
		Bezier     string  `json:"bezier"`
		Style      string  `json:"style"`
	}
	_ = json.Unmarshal(raw[0], &anims)
	for _, a := range anims {
		if !a.Overridden || strings.HasPrefix(a.Name, "__") {
			continue
		}
		items = append(items, AnimItem{Leaf: a.Name, Enabled: a.Enabled, Speed: a.Speed, Bezier: a.Bezier, Style: a.Style})
	}
	var cs []struct {
		Name string  `json:"name"`
		X0   float64 `json:"X0"`
		Y0   float64 `json:"Y0"`
		X1   float64 `json:"X1"`
		Y1   float64 `json:"Y1"`
	}
	_ = json.Unmarshal(raw[1], &cs)
	for _, c := range cs {
		curves = append(curves, AnimCurve{Name: c.Name, X0: c.X0, Y0: c.Y0, X1: c.X1, Y1: c.Y1})
	}
	return items, curves
}

// liveKbDefaults reads the running compositor's effective kb_* values so the
// unsaved baseline matches the real session; the hardcoded defaults stand when
// nothing answers.
func liveKbDefaults(in *Input) {
	opts := []struct {
		name string
		dst  *string
	}{
		{"input:kb_layout", &in.KbLayout},
		{"input:kb_variant", &in.KbVariant},
		{"input:kb_options", &in.KbOptions},
	}
	for _, o := range opts {
		b, err := ctl("getoption", o.name, "-j")
		if err != nil {
			continue
		}
		var v struct {
			Str string `json:"str"`
		}
		if json.Unmarshal(b, &v) != nil {
			continue
		}
		if s := strings.TrimSpace(v.Str); s != "" {
			*o.dst = s
		}
	}
}

// sessionEntry is the wayland-session .desktop the installer writes as a
// fallback when the Hyprland package shipped none. Byte-identical to the entry
// the setup script wrote before this moved behind the provider.
const sessionEntry = `[Desktop Entry]
Name=Hyprland
Comment=An intelligent dynamic tiling Wayland compositor
Exec=Hyprland
Type=Application`

// runSession prints the wayland-session entry. It must work with no live
// compositor: the installer writes the file inside a chroot.
func runSession() error {
	_, err := stdout.WriteString(sessionEntry + "\n")
	return err
}
