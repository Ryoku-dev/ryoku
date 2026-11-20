package main

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	wm "ryoku-wm"
)

var modePattern = regexp.MustCompile(`^[0-9]+x[0-9]+$`)

// outputs is the write half of the display seam for Umbriel. It renders the
// neutral layout from the Hub display editor into monitors.toml, the seeded
// machine-owned file included by config.toml. Umbriel watches the config, so a
// write applies live and persists to the next login; no reload call is needed.
func runOutputs(args []string) error {
	if len(args) < 1 {
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

	rep := wm.ApplyReport{Provider: "umbriel", ReloadNeeded: false}
	rep.Unhonored = outputsUnhonored(layout)
	dir := umbrielConfigDir()
	if dir == "" {
		return fmt.Errorf("outputs: no Umbriel config dir")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	path := filepath.Join(dir, "monitors.toml")
	if err := atomicWrite(path, monitorsToml(layout), 0o644); err != nil {
		return err
	}
	rep.Written = []string{"umbriel/monitors.toml"}
	return json.NewEncoder(stdout).Encode(rep)
}

func outputsUnhonored(layout []wm.OutputLayout) []wm.Unhonored {
	var out []wm.Unhonored
	seen := map[string]bool{}
	for _, o := range layout {
		if o.Mode != "" && normalizeMode(o.Mode) == "" {
			out = append(out, wm.Unhonored{Key: "output." + o.Name + ".mode", Reason: "Umbriel needs a mode shaped like WIDTHxHEIGHT or WIDTHxHEIGHT@HZ."})
		}
		if seen[o.Name] {
			out = append(out, wm.Unhonored{Key: "output." + o.Name, Reason: "Duplicate output names are written once."})
		}
		seen[o.Name] = true
		if o.Mirror != "" {
			out = append(out, wm.Unhonored{Key: "output." + o.Name + ".mirror", Reason: "Umbriel has no output mirroring control."})
		}
		if o.ColorMode != "" {
			out = append(out, wm.Unhonored{Key: "output." + o.Name + ".colorMode", Reason: "Colour mode is not mapped yet."})
		}
		if o.SdrBrightness > 0 {
			out = append(out, wm.Unhonored{Key: "output." + o.Name + ".sdrBrightness", Reason: "SDR brightness is not mapped yet."})
		}
		if o.VRR {
			out = append(out, wm.Unhonored{Key: "output." + o.Name + ".vrr", Reason: "Umbriel reports no adaptive sync for this output."})
		}
	}
	return out
}

func monitorsToml(layout []wm.OutputLayout) []byte {
	var b strings.Builder
	b.WriteString("# Written by the display tooling from the neutral output layout.\n")
	b.WriteString("# Umbriel watches this file, so saving applies live and persists to next login.\n")
	b.WriteString("# Hand edits are lost on the next apply; put durable pins in user.toml.\n\n")
	seen := map[string]bool{}
	for _, o := range layout {
		if o.Name == "" || seen[o.Name] {
			continue
		}
		seen[o.Name] = true
		fmt.Fprintf(&b, "[output.%s]\n", strconv.Quote(o.Name))
		fmt.Fprintf(&b, "enabled = %t\n", o.Enabled)
		if mode := normalizeMode(o.Mode); mode != "" {
			fmt.Fprintf(&b, "mode = %s\n", strconv.Quote(mode))
		}
		if o.Scale > 0 {
			fmt.Fprintf(&b, "scale = %s\n", tomlNum(o.Scale))
		}
		fmt.Fprintf(&b, "position = [%d, %d]\n", o.X, o.Y)
		if t := waylandTransformToUmbriel(o.Transform); t != "" {
			fmt.Fprintf(&b, "transform = %s\n", strconv.Quote(t))
		}
		b.WriteString("\n")
	}
	return []byte(b.String())
}

// The seam carries refresh in millihertz from Wayland; Umbriel's config schema
// takes a "WxH@Hz" mode string.
func normalizeMode(mode string) string {
	parts := strings.Split(mode, "@")
	if len(parts) > 2 {
		return ""
	}
	if !modePattern.MatchString(parts[0]) {
		return ""
	}
	if len(parts) == 1 {
		return parts[0]
	}
	hz, err := strconv.ParseFloat(parts[1], 64)
	if err != nil || hz <= 0 || math.IsNaN(hz) || math.IsInf(hz, 0) {
		return ""
	}
	if hz > 1000 {
		hz /= 1000
	}
	return fmt.Sprintf("%s@%g", parts[0], hz)
}

func waylandTransformToUmbriel(t int) string {
	switch t {
	case 1:
		return "90"
	case 2:
		return "180"
	case 3:
		return "270"
	default:
		return "normal"
	}
}

func tomlNum(v float64) string {
	return strconv.FormatFloat(v, 'f', -1, 64)
}

func atomicWrite(path string, body []byte, mode os.FileMode) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".umbriel-*.toml")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer os.Remove(name)
	if _, err := tmp.Write(body); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(mode); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(name, path)
}
