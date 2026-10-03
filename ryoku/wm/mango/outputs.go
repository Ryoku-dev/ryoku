package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	wm "ryoku-wm"
)

// outputs is the write half of the display seam for mango: it renders the
// neutral output layout the display editor built into monitors.conf as
// monitorrule lines. mango has no output watcher, so the write persists for the
// next login and reload_config re-applies it for the running session (verified
// live: reload re-reads monitor rules). monitors.conf is a seeded,
// machine-owned file the entry sources, written straight into the mango dir
// rather than the user_edits overlay, matching the Hyprland monitors drop-in;
// monitors_user.conf loads after it, so a hand pin still wins.

const monitorsHeader = "# Written by the display tooling from the neutral output layout: one rule per\n" +
	"# output. Regenerated on every display change; a forced mode, scale or\n" +
	"# position set here beats autoscale. Hand pins belong in\n" +
	"# monitors_user.conf, which the entry file loads after this one and wins.\n\n"

func runOutputs(args []string) error {
	layoutPath := ""
	for _, a := range args {
		if layoutPath == "" {
			layoutPath = a
		}
	}
	if layoutPath == "" {
		return fmt.Errorf("outputs: missing layout path")
	}
	b, err := os.ReadFile(layoutPath)
	if err != nil {
		return fmt.Errorf("outputs: %w", err)
	}
	var layout []wm.OutputLayout
	if err := json.Unmarshal(b, &layout); err != nil {
		return fmt.Errorf("outputs: bad layout: %w", err)
	}

	rep := wm.ApplyReport{Provider: wm.ProviderMango}
	rep.Unhonored = outputsUnhonored(layout)

	path := filepath.Join(mangoConfigDir(), "monitors.conf")
	if err := atomicWrite(path, monitorsConf(layout), 0o644); err != nil {
		return err
	}
	rep.Written = []string{path}
	// The reload rides this verb: monitors.conf is written and the running
	// session re-reads it here and now, so the caller has nothing left to
	// trigger and ReloadNeeded stays false.
	rep.ReloadNeeded = false
	if live() {
		if err := dispatch("reload_config"); err != nil {
			return err
		}
	}
	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(rep)
}

// monitorsConf renders one monitorrule per layout entry. A disabled output is
// the rule's disable:1; an enabled one carries only the fields it needs, so an
// omitted mode or scale leaves mango to choose. Position is always written for
// an enabled output, since the editor lays every screen on one canvas — and
// clamped non-negative: mango's own docs warn that negative coordinates break
// XWayland click routing (a known upstream issue), and the display editor's
// canvas starts at 0,0.
func monitorsConf(layout []wm.OutputLayout) []byte {
	var b strings.Builder
	b.WriteString(monitorsHeader)
	for _, o := range layout {
		rule := "monitorrule=name:^" + o.Name + "$,"
		var parts []string
		if !o.Enabled {
			parts = append(parts, "disable:1")
			b.WriteString(rule + strings.Join(parts, ",") + "\n")
			continue
		}
		x, y := o.X, o.Y
		if x < 0 {
			x = 0
		}
		if y < 0 {
			y = 0
		}
		if w, h, ok := modeSize(o.Mode); ok {
			parts = append(parts, "width:"+strconv.Itoa(w), "height:"+strconv.Itoa(h))
		}
		if hz, ok := modeHz(o.Mode); ok {
			parts = append(parts, "refresh:"+confNum(hz))
		}
		parts = append(parts, "x:"+strconv.Itoa(x), "y:"+strconv.Itoa(y))
		if o.Scale != 0 {
			parts = append(parts, "scale:"+confNum(o.Scale))
		}
		if o.Transform != 0 {
			parts = append(parts, "rr:"+strconv.Itoa(o.Transform))
		}
		if o.VRR {
			parts = append(parts, "vrr:1")
		}
		b.WriteString(rule + strings.Join(parts, ",") + "\n")
	}
	return []byte(b.String())
}

// modeSize splits the editor's "WxH@Hz" into the size part.
func modeSize(mode string) (int, int, bool) {
	base, _, _ := strings.Cut(mode, "@")
	var w, h int
	if _, err := fmt.Sscanf(base, "%dx%d", &w, &h); err == nil && w > 0 && h > 0 {
		return w, h, true
	}
	return 0, 0, false
}

// modeHz splits the editor's "WxH@Hz" into the refresh part.
func modeHz(mode string) (float64, bool) {
	i := strings.IndexByte(mode, '@')
	if i < 0 {
		return 0, false
	}
	f, err := strconv.ParseFloat(mode[i+1:], 64)
	if err != nil || f <= 0 {
		return 0, false
	}
	return f, true
}

// outputsUnhonored names each requested detail mango cannot express. mango
// covers mode, position, scale, transform and VRR through monitorrule; the
// losses are output mirroring, the HDR pipeline (the wl-only Vulkan branch,
// not the scenefx build mango links) and a forced non-advertised mode, which mango
// can write (custom:1) but the docs warn may black-screen, so it is reported
// as a risk the user sees rather than applied silently.
func outputsUnhonored(layout []wm.OutputLayout) []wm.Unhonored {
	advertised := advertisedModes()
	var out []wm.Unhonored
	for _, o := range layout {
		if o.Mirror != "" {
			out = append(out, wm.Unhonored{
				Key:    "outputs." + o.Name + ".mirror",
				Reason: "mango cannot clone one output onto another.",
			})
		}
		if o.ColorMode != "" && o.ColorMode != "srgb" {
			out = append(out, wm.Unhonored{
				Key:    "outputs." + o.Name + ".colorMode",
				Reason: "mango's HDR pipeline lives in a separate build that Ryoku does not ship.",
			})
		}
		if o.Mode != "" && advertised != nil {
			if modes, ok := advertised[o.Name]; ok && !modes[o.Mode] {
				out = append(out, wm.Unhonored{
					Key:    "outputs." + o.Name + ".mode",
					Reason: "a forced mode the panel does not advertise can black-screen mango, so it is left unset.",
				})
			}
		}
	}
	return out
}

// advertisedModes maps each live output to the modes it advertises, via
// wlr-randr over the session's display. nil means the client is unavailable;
// a successful empty response stays a non-nil empty map.
func advertisedModes() map[string]map[string]bool {
	randr := readRandr()
	if randr.state == randrUnavailable {
		return nil
	}
	return parseAdvertisedModes(randr.text)
}

// parseAdvertisedModes reads wlr-randr's `NAME "make"` headers and mode rows
// into the same exact "WxH@Hz" spelling the editor uses.
func parseAdvertisedModes(text string) map[string]map[string]bool {
	out := map[string]map[string]bool{}
	name := ""
	inModes := false
	for _, raw := range strings.Split(text, "\n") {
		if raw == "" {
			continue
		}
		if !strings.HasPrefix(raw, " ") && !strings.HasPrefix(raw, "\t") {
			fields := strings.Fields(raw)
			if len(fields) > 0 {
				name = fields[0]
				if out[name] == nil {
					out[name] = map[string]bool{}
				}
			}
			inModes = false
			continue
		}
		trimmed := strings.TrimSpace(raw)
		if trimmed == "Modes:" {
			inModes = true
			continue
		}
		if inModes && name != "" {
			if mode, ok := parseRandrMode(trimmed); ok {
				out[name][mode] = true
			}
		}
	}
	return out
}
