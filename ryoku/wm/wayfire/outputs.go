package main

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	wm "ryoku-wm"
)

// outputs is the write half of the display seam for wayfire: it renders the
// neutral output layout the display editor built into monitors.ini, a seed layer
// apply composes but never writes, so the display tooling owns it outright.
// wayfire watches its config, so the write both applies live and persists to the
// next login; there is no imperative step and no reload to trigger. user.ini's
// [output:...] sections load after this file and win, so a hand pin that must
// survive the next display apply goes there instead.

const monitorsHeader = "# Written by the display tooling from the neutral output layout: one section\n" +
	"# per connected screen with its mode, position, transform, scale and VRR\n" +
	"# resolved. wayfire watches this file, so a write applies live and persists to\n" +
	"# the next login. The display tooling owns this file; hand edits are lost on\n" +
	"# the next apply. Put durable pins in user.ini, whose [output:...] sections\n" +
	"# load after this one and win.\n\n"

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

	rep := wm.ApplyReport{Provider: wm.ProviderWayfire, ReloadNeeded: false}
	rep.Unhonored = outputsUnhonored(layout)

	path := filepath.Join(wayfireConfigDir(), "monitors.ini")
	if err := atomicWrite(path, monitorsIni(layout), 0o644); err != nil {
		return err
	}
	rep.Written = []string{path}
	return encodeReport(rep)
}

// monitorsIni renders one output section per layout entry. A disabled output is
// `mode = off`; a mirrored one is `mode = mirror <name>` and carries nothing
// else, since a clone has no layout of its own; an enabled one carries only the
// lines it needs, so an omitted mode or scale leaves wayfire to choose.
// Position is always written for an enabled output, since the editor lays every
// screen on one canvas.
func monitorsIni(layout []wm.OutputLayout) []byte {
	var b strings.Builder
	b.WriteString(monitorsHeader)
	for _, o := range layout {
		if o.Name == "" {
			continue
		}
		fmt.Fprintf(&b, "[output:%s]\n", o.Name)
		if !o.Enabled {
			b.WriteString("mode = off\n\n")
			continue
		}
		if o.Mirror != "" && o.Mirror != "none" {
			fmt.Fprintf(&b, "mode = mirror %s\n\n", o.Mirror)
			continue
		}
		if o.Mode != "" {
			fmt.Fprintf(&b, "mode = %s\n", wayfireMode(o.Mode))
		}
		fmt.Fprintf(&b, "position = %d,%d\n", o.X, o.Y)
		if o.Scale > 0 {
			fmt.Fprintf(&b, "scale = %g\n", o.Scale)
		}
		if t := wayfireTransform(o.Transform); t != "" {
			fmt.Fprintf(&b, "transform = %s\n", t)
		}
		if o.VRR {
			b.WriteString("vrr = true\n")
		}
		b.WriteString("\n")
	}
	return []byte(b.String())
}

// wayfireMode spells a neutral "WxH@Hz" the way wayfire's config wants it, in
// millihertz: the neutral store carries the panel rate as the human Hz it
// prints, while wayfire parses "@60000". A rate already in mHz passes through,
// so a profile saved from wayfire's own state replays unchanged.
func wayfireMode(mode string) string {
	at := strings.LastIndex(mode, "@")
	if at < 0 {
		return mode
	}
	rate, err := strconv.ParseFloat(mode[at+1:], 64)
	if err != nil {
		return mode
	}
	if rate < 1000 {
		rate = math.Round(rate * 1000)
	}
	return mode[:at] + "@" + strconv.FormatFloat(rate, 'f', -1, 64)
}

// wayfireTransform maps the neutral wayland transform integer onto wayfire's
// config token. Wayfire spells the flipped rotations "90_flipped", not the
// protocol's "flipped-90", and an unknown token is rejected back to normal, so
// the vocabulary is pinned by test. 0 returns empty so the caller omits the line.
func wayfireTransform(t int) string {
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
		return "90_flipped"
	case 6:
		return "180_flipped"
	case 7:
		return "270_flipped"
	}
	return ""
}

// transformFromWayfire maps a wayfire config token back to the neutral wayland
// transform integer, so the state read sees what this writer wrote; a numeric
// token parses directly and anything unknown reads as normal.
func transformFromWayfire(s string) int {
	switch s {
	case "normal":
		return 0
	case "90":
		return 1
	case "180":
		return 2
	case "270":
		return 3
	case "flipped":
		return 4
	case "90_flipped":
		return 5
	case "180_flipped":
		return 6
	case "270_flipped":
		return 7
	}
	if n, err := strconv.Atoi(s); err == nil && n >= 0 && n <= 7 {
		return n
	}
	return 0
}

// outputsUnhonored names each requested detail wayfire cannot express. The
// layout path covers mode (mirroring included), position, scale, transform and
// VRR; the losses are the HDR / wide-gamut colour pipeline and its SDR
// brightness, reported per leaf so a profile carried over from another
// compositor says what it could not keep, rather than the whole apply failing.
// No non-advertised mode check: wayfire's IPC never advertises the mode list.
func outputsUnhonored(layout []wm.OutputLayout) []wm.Unhonored {
	var unh []wm.Unhonored
	for _, o := range layout {
		if !o.Enabled {
			continue
		}
		if o.ColorMode != "" && o.ColorMode != "srgb" {
			unh = append(unh, wm.Unhonored{
				Key:    "displays." + o.Name + ".colorMode",
				Reason: "wayfire has no HDR or wide-gamut colour pipeline",
			})
		}
		if o.SdrBrightness > 0 && o.SdrBrightness != 1 {
			unh = append(unh, wm.Unhonored{
				Key:    "displays." + o.Name + ".sdrBrightness",
				Reason: "wayfire has no HDR SDR-brightness control",
			})
		}
	}
	return unh
}
