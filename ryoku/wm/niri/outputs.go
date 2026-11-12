package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	wm "ryoku-wm"
)

// outputs is the write half of the display seam for niri: it renders the neutral
// output layout the display editor built into monitors.kdl. niri watches its
// config, so the write both applies live and persists to the next login; there
// is no imperative step and no reload to trigger. monitors.kdl is a seeded,
// machine-owned include that config.kdl requires, so it is written straight into
// the niri dir rather than the user_edits overlay, matching the Hyprland monitors
// drop-in. monitors_user.kdl loads after it, so a hand pin still wins.

const monitorsHeader = "// Written by the display tooling from the neutral output layout: one block per\n" +
	"// connected screen with its mode, scale, position, rotation and VRR resolved.\n" +
	"// The provider watch temporarily enables a connected panel if every output is off.\n" +
	"// niri watches this file, so a write applies live and persists to the next\n" +
	"// login. Hand edits are lost on the next apply; put durable pins in\n" +
	"// monitors_user.kdl, which loads after this file and wins.\n\n"

func runOutputs(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("outputs: missing layout path")
	}
	raw, err := os.ReadFile(args[0])
	if err != nil {
		return err
	}
	var layout []wm.OutputLayout
	if err := json.Unmarshal(raw, &layout); err != nil {
		return fmt.Errorf("outputs: %w", err)
	}

	rep := wm.ApplyReport{Provider: wm.ProviderNiri, ReloadNeeded: false}
	rep.Unhonored = outputsUnhonored(layout)

	path := filepath.Join(niriConfigDir(), "monitors.kdl")
	if err := atomicWrite(path, monitorsKdl(layout), 0o644); err != nil {
		return err
	}
	rep.Written = []string{path}
	return encodeReport(rep)
}

// monitorsKdl renders one output block per layout entry. A disabled output is a
// bare `off`; an enabled one carries only the lines it needs, so an omitted mode
// or scale leaves niri to choose. Position is always written for an enabled
// output, since the editor lays every screen on one canvas.
func monitorsKdl(layout []wm.OutputLayout) []byte {
	var b strings.Builder
	b.WriteString(monitorsHeader)
	for _, o := range layout {
		if o.Name == "" {
			continue
		}
		b.WriteString("output ")
		b.WriteString(kdlStr(o.Name))
		b.WriteString(" {\n")
		if !o.Enabled {
			b.WriteString("    off\n")
		} else {
			if o.Mode != "" {
				b.WriteString("    mode " + kdlStr(o.Mode) + "\n")
			}
			if o.Scale > 0 {
				b.WriteString("    scale " + kdlNum(o.Scale) + "\n")
			}
			b.WriteString(fmt.Sprintf("    position x=%d y=%d\n", o.X, o.Y))
			if t := waylandTransformToNiri(o.Transform); t != "" {
				b.WriteString("    transform " + kdlStr(t) + "\n")
			}
			if o.VRR {
				b.WriteString("    variable-refresh-rate\n")
			}
		}
		b.WriteString("}\n")
	}
	return []byte(b.String())
}

// fallbackOutput returns the connected output to enable when every output is
// off. The built-in panel is the safest first choice after undocking; otherwise
// the provider uses the first output from niri's name-sorted snapshot.
func fallbackOutput(outputs []wm.Output) string {
	for _, output := range outputs {
		if !output.Disabled {
			return ""
		}
	}
	for _, output := range outputs {
		if output.Disabled && isInternalOutput(output.Name) {
			return output.Name
		}
	}
	for _, output := range outputs {
		if output.Disabled {
			return output.Name
		}
	}
	return ""
}

func outputFallbackMarker() string {
	runtimeDir := os.Getenv("XDG_RUNTIME_DIR")
	if runtimeDir == "" {
		return ""
	}
	return filepath.Join(runtimeDir, "ryoku", "niri-output-fallback")
}

func readOutputFallbackMarker() string {
	data, err := os.ReadFile(outputFallbackMarker())
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

func writeOutputFallbackMarker(name string) {
	path := outputFallbackMarker()
	if path == "" {
		return
	}
	if os.MkdirAll(filepath.Dir(path), 0o700) == nil {
		_ = os.WriteFile(path, []byte(name), 0o600)
	}
}

func clearOutputFallbackMarker() {
	if path := outputFallbackMarker(); path != "" {
		_ = os.Remove(path)
	}
}

// reconcileOutputFallback owns only the transient override it recorded. That
// lets a hand-written monitors_user.kdl keep winning while the temporary panel
// is turned off again as soon as another output is enabled.
func reconcileOutputFallback(outputs []wm.Output) bool {
	marker := readOutputFallbackMarker()
	if marker != "" {
		present, disabled, otherEnabled := false, false, false
		for _, output := range outputs {
			if output.Name == marker {
				present = true
				disabled = output.Disabled
			} else if !output.Disabled {
				otherEnabled = true
			}
		}
		if !present {
			clearOutputFallbackMarker()
		} else if otherEnabled {
			if disabled {
				clearOutputFallbackMarker()
				return false
			}
			if _, err := request(outputRequest(marker, "Off")); err == nil {
				clearOutputFallbackMarker()
				return true
			}
			return false
		} else if !disabled {
			return false
		}
	}

	name := fallbackOutput(outputs)
	if name == "" {
		return false
	}
	if _, err := request(outputRequest(name, "On")); err != nil {
		return false
	}
	writeOutputFallbackMarker(name)
	return true
}

// waylandTransformToNiri maps the neutral wayland transform integer onto niri's
// config token. 0 (normal) returns empty so the caller omits the line.
func waylandTransformToNiri(t int) string {
	switch t {
	case 1:
		return "90"
	case 2:
		return "180"
	case 3:
		return "270"
	case 4:
		return "flipped"
	case 5:
		return "flipped-90"
	case 6:
		return "flipped-180"
	case 7:
		return "flipped-270"
	}
	return ""
}

// outputsUnhonored names each requested detail niri cannot express. niri covers
// scale, position, transform and VRR; the losses are a forced (non-advertised)
// mode, output mirroring, and the HDR / wide-gamut colour pipeline. Each is
// reported per leaf so a profile carried over from another compositor says what
// it could not keep, rather than the whole apply failing.
func outputsUnhonored(layout []wm.OutputLayout) []wm.Unhonored {
	advertised := advertisedModes() // nil off a live session; skips the mode check
	var unh []wm.Unhonored
	for _, o := range layout {
		if !o.Enabled {
			continue
		}
		if o.Mirror != "" && o.Mirror != "none" {
			unh = append(unh, wm.Unhonored{
				Key:    "displays." + o.Name + ".mirror",
				Reason: "Niri can't show one display's picture on another.",
			})
		}
		if o.ColorMode != "" && o.ColorMode != "srgb" {
			unh = append(unh, wm.Unhonored{
				Key:    "displays." + o.Name + ".colorMode",
				Reason: "Niri has no HDR or wide colour mode.",
			})
		}
		if o.SdrBrightness > 0 && o.SdrBrightness != 1 {
			unh = append(unh, wm.Unhonored{
				Key:    "displays." + o.Name + ".sdrBrightness",
				Reason: "Niri has no brightness setting for everyday content in HDR.",
			})
		}
		if o.Mode != "" && advertised != nil {
			if modes, ok := advertised[o.Name]; ok && !modes[o.Mode] {
				unh = append(unh, wm.Unhonored{
					Key:    "displays." + o.Name + ".mode",
					Reason: fmt.Sprintf("Niri can't use %s because the display doesn't offer it, so it keeps its preferred mode.", o.Mode),
				})
			}
		}
	}
	return unh
}

// advertisedModes maps each live output to the modes it advertises, so a forced
// mode can be told from a real one. nil off a live session, which skips the check.
func advertisedModes() map[string]map[string]bool {
	ws, err := readWorkspaces()
	if err != nil {
		return nil
	}
	outs, err := readOutputs(ws, true)
	if err != nil {
		return nil
	}
	m := make(map[string]map[string]bool, len(outs))
	for _, o := range outs {
		set := make(map[string]bool, len(o.Modes))
		for _, mode := range o.Modes {
			set[mode] = true
		}
		m[o.Name] = set
	}
	return m
}
