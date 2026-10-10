package main

import (
	"fmt"
	"math/rand/v2"
	"path/filepath"
	"sort"
	"strings"
)

// dispatchVerb answers one plain command line, keeping the grammar the shell
// daemon, the keybinds, and the CLI already speak: `wallpaper <mode> [--screen
// <name>] [arg]` and `depth <set|clear> ...`.
func (d *daemon) dispatchVerb(line string) string {
	switch strings.Fields(line)[0] {
	case "wallpaper":
		return d.wallpaperVerb(line)
	case "depth":
		return d.depthVerb(line)
	default:
		return fmt.Sprintf("err unknown command: %s", strings.Fields(line)[0])
	}
}

func (d *daemon) wallpaperVerb(line string) string {
	if !d.config().wallpapersEnabled() {
		return "err wallpaper: wallpapers module is disabled"
	}
	rest := strings.TrimSpace(strings.TrimPrefix(line, "wallpaper"))
	rest, screen := extractScreen(rest)
	rest, workspace := extractFlag(rest, "--workspace")
	mode := ""
	if f := strings.Fields(rest); len(f) > 0 {
		mode = f[0]
	}
	var outputs []string
	if screen != "" {
		outputs = []string{screen}
	}
	switch mode {
	case "set", "assign":
		path := strings.TrimSpace(strings.TrimPrefix(rest, mode))
		if path == "" && mode == "set" {
			return "err wallpaper: set requires a path"
		}
		if workspace || mode == "assign" {
			params := map[string]interface{}{"output": screen}
			if path != "" {
				params["path"] = path
				params["type"] = typeOf(path)
			}
			if _, err := d.assignWorkspace(params); err != nil {
				return "err wallpaper: " + err.Error()
			}
			return "ok"
		}
		if err := d.applyWallpaper(typeOf(path), path, "set", outputs, nil, nil); err != nil {
			return "err wallpaper: " + err.Error()
		}
		return "ok"
	case "random":
		if pick := d.pickRandom(nil, false); pick != "" {
			if workspace {
				if _, err := d.assignWorkspace(map[string]interface{}{"output": screen, "path": pick, "type": typeOf(pick)}); err != nil {
					return "err wallpaper: " + err.Error()
				}
			} else if err := d.applyWallpaper(typeOf(pick), pick, "set", outputs, nil, nil); err != nil {
				return "err wallpaper: " + err.Error()
			}
			return "ok"
		}
		return "err wallpaper: no wallpapers available"
	case "next":
		if pick := d.pickNext(); pick != "" {
			if workspace {
				if _, err := d.assignWorkspace(map[string]interface{}{"output": screen, "path": pick, "type": typeOf(pick)}); err != nil {
					return "err wallpaper: " + err.Error()
				}
			} else if err := d.applyWallpaper(typeOf(pick), pick, "set", outputs, nil, nil); err != nil {
				return "err wallpaper: " + err.Error()
			}
			return "ok"
		}
		return "err wallpaper: no wallpapers available"
	case "unassign":
		if _, err := d.unassignWorkspace(map[string]interface{}{"output": screen}); err != nil {
			return "err wallpaper: " + err.Error()
		}
		return "ok"
	case "repaint":
		d.surface.republish()
		return "ok"
	case "restore":
		d.restoreOutputs()
		return "ok"
	case "ui":
		// Resident picker: flip its surface over the event hub; a dead or
		// not-yet-started instance cold-launches already visible instead.
		if d.ui.ensure() {
			d.broadcast("ryogami.wall.toggle", map[string]interface{}{})
		}
		return "ok"
	case "settings":
		tab := ""
		if f := strings.Fields(rest); len(f) > 1 {
			tab = f[1]
		}
		d.openSettings(tab)
		return "ok"
	case "resource":
		f := strings.Fields(rest)
		if len(f) < 2 || (f[1] != "low" && f[1] != "medium" && f[1] != "high") {
			return "err wallpaper: resource expects low|medium|high"
		}
		d.cfgMu.Lock()
		d.cfg.ResourceTier = f[1]
		d.cfgMu.Unlock()
		persistResourceTier(f[1])
		return "ok"
	case "live-reload":
		// Relaunch the current clip after a settings change; a still just
		// republishes so nothing reveals.
		if d.video.Playing() {
			d.restoreOutputs()
		} else {
			d.surface.republish()
		}
		return "ok"
	case "":
		return "err wallpaper: missing mode"
	default:
		return fmt.Sprintf("err wallpaper: unknown mode: %s", mode)
	}
}

// depthVerb keeps the surface bridge with the shell daemon's depth worker:
// `set` carries a JSON body so paths with spaces survive, `clear` wipes every
// slot's cutout.
func (d *daemon) depthVerb(line string) string {
	rest := strings.TrimSpace(strings.TrimPrefix(line, "depth"))
	if rest == "clear" {
		d.surface.clearDepth()
		return "ok"
	}
	if body, okSet := strings.CutPrefix(rest, "set "); okSet {
		var req struct {
			Screen string `json:"screen"`
			Source string `json:"source"`
			Out    string `json:"out"`
			Rev    int64  `json:"rev"`
		}
		if err := unmarshalStrict(body, &req); err != nil {
			return fmt.Sprintf("err depth set: %v", err)
		}
		d.surface.setDepth(req.Screen, req.Source, req.Out, req.Rev)
		return "ok"
	}
	return "err depth expects set|clear"
}

// extractScreen recovers `--screen <name>` from anywhere in the argument tail,
// leaving the rest verbatim so a path may contain spaces.
func extractScreen(rest string) (string, string) {
	i := strings.Index(rest, "--screen ")
	if i < 0 {
		return rest, ""
	}
	after := rest[i+len("--screen "):]
	end := strings.IndexByte(after, ' ')
	if end < 0 {
		end = len(after)
	}
	screen := strings.TrimSpace(after[:end])
	out := strings.TrimSpace(strings.TrimSpace(rest[:i]) + " " + strings.TrimSpace(after[end:]))
	return out, screen
}

func extractFlag(rest, flag string) (string, bool) {
	for start := 0; start <= len(rest)-len(flag); {
		i := strings.Index(rest[start:], flag)
		if i < 0 {
			break
		}
		i += start
		leftOK := i == 0 || rest[i-1] == ' ' || rest[i-1] == '\t'
		end := i + len(flag)
		rightOK := end == len(rest) || rest[end] == ' ' || rest[end] == '\t'
		if leftOK && rightOK {
			return strings.TrimSpace(rest[:i] + rest[end:]), true
		}
		start = i + len(flag)
	}
	return rest, false
}

func typeOf(path string) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".mp4", ".webm", ".mkv", ".avi", ".mov", ".gif":
		return "video"
	}
	return "static"
}

// pickRandom returns a random source path different from the current one.
func (d *daemon) pickRandom(types []string, favouritesOnly bool) string {
	candidates := d.candidatePaths(types, favouritesOnly)
	current := d.currentName()
	filtered := candidates[:0]
	for _, c := range candidates {
		if filepath.Base(c) != current {
			filtered = append(filtered, c)
		}
	}
	if len(filtered) == 0 {
		filtered = candidates
	}
	if len(filtered) == 0 {
		return ""
	}
	return filtered[rand.IntN(len(filtered))]
}

// pickNext advances to the file after the current one in name order, wrapping.
func (d *daemon) pickNext() string {
	candidates := d.candidatePaths(nil, false)
	if len(candidates) == 0 {
		return ""
	}
	sort.Strings(candidates)
	current := d.currentName()
	for i, c := range candidates {
		if filepath.Base(c) == current {
			return candidates[(i+1)%len(candidates)]
		}
	}
	return candidates[0]
}

func (d *daemon) candidatePaths(types []string, favouritesOnly bool) []string {
	cfg := d.config()
	var out []string
	for _, e := range d.store.list(favouritesOnly) {
		if len(types) > 0 && !contains(types, e.Type) {
			continue
		}
		p := e.VideoFile
		if p == "" {
			p = filepath.Join(cfg.wallpaperDir(), e.Name)
		}
		out = append(out, p)
	}
	return out
}

func (d *daemon) randomPick(types []string, favouritesOnly bool) {
	if pick := d.pickRandom(types, favouritesOnly); pick != "" {
		_ = d.applyWallpaperReason("random", typeOf(pick), pick, "set", nil, nil, nil)
	}
}
