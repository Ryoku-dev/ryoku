package main

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"strings"
)

// config mirrors the Rust daemon's ~/.config/ryoku/ryogami.json: only the keys
// the Go daemon consumes are modeled; unknown keys pass through untouched on
// the persist path because the file is rewritten by targeted patches only.
type config struct {
	Matugen struct {
		Mode       string  `json:"mode"`
		SchemeType string  `json:"schemeType"`
		ColorIndex uint32  `json:"colorIndex"`
		VideoFrame float64 `json:"videoFrame"`
	} `json:"matugen"`
	ResourceTier     string `json:"resource_tier"`
	RestoreOnStartup *bool  `json:"restoreOnStartup"`
	Features         struct {
		Wallpapers *bool `json:"wallpapers"`
		Matugen    *bool `json:"matugen"`
	} `json:"features"`
	Paths struct {
		Wallpaper      string `json:"wallpaper"`
		VideoWallpaper string `json:"videoWallpaper"`
		Cache          string `json:"cache"`
	} `json:"paths"`
}

func home() string {
	h, err := os.UserHomeDir()
	if err != nil {
		return "/"
	}
	return h
}

// stateHome resolves XDG_STATE_HOME (~/.local/state fallback): where Ryoku's
// cross-process state files live, including the legacy ryoku-wallpaper path the
// wallpaper readers (rice capture, the overview backdrop, the Super+W on-air
// dot, the shell's palette bridge) watch to learn what is on screen.
func stateHome() string {
	if d := os.Getenv("XDG_STATE_HOME"); d != "" {
		return d
	}
	return filepath.Join(home(), ".local", "state")
}

func ryokuConfigDir() string {
	if d := os.Getenv("XDG_CONFIG_HOME"); d != "" {
		return filepath.Join(d, "ryoku")
	}
	return filepath.Join(home(), ".config", "ryoku")
}

func configPath() string { return filepath.Join(ryokuConfigDir(), "ryogami.json") }

func loadConfig() config {
	var c config
	loadJSON(configPath(), &c)
	if c.Matugen.Mode == "" {
		c.Matugen.Mode = "dark"
	}
	if c.Matugen.SchemeType == "" {
		c.Matugen.SchemeType = "scheme-tonal-spot"
	}
	if c.ResourceTier == "" {
		c.ResourceTier = "medium"
	}
	return c
}

func resolvePath(p string) string {
	if p == "" {
		return ""
	}
	if strings.HasPrefix(p, "~/") {
		return filepath.Join(home(), p[2:])
	}
	return p
}

func (c config) wallpaperDir() string {
	if p := resolvePath(c.Paths.Wallpaper); p != "" {
		return p
	}
	return filepath.Join(home(), "Pictures", "Wallpapers")
}

func (c config) videoDir() string {
	if p := resolvePath(c.Paths.VideoWallpaper); p != "" {
		return p
	}
	// Ryoku keeps clips in ~/Pictures/livewalls (the switcher and the old live
	// stack both scanned it); fall back to the shared wallpaper dir only when
	// that folder is absent, mirroring upstream's shared-dir default.
	if live := filepath.Join(home(), "Pictures", "livewalls"); dirExists(live) {
		return live
	}
	return c.wallpaperDir()
}

func dirExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.IsDir()
}

func (c config) cacheDir() string {
	if p := resolvePath(c.Paths.Cache); p != "" {
		return p
	}
	if d := os.Getenv("XDG_CACHE_HOME"); d != "" {
		return filepath.Join(d, "ryogami")
	}
	return filepath.Join(home(), ".cache", "ryogami")
}

func (c config) wallpapersEnabled() bool {
	return c.Features.Wallpapers == nil || *c.Features.Wallpapers
}

func (c config) matugenEnabled() bool {
	return c.Features.Matugen == nil || *c.Features.Matugen
}

func (c config) restoreEnabled() bool {
	return c.RestoreOnStartup == nil || *c.RestoreOnStartup
}

// videoFrame is the second of a clip liveStill samples for the frame the shell
// paints and matugen reads. Defaults to 1s; the picker's palette-frame slider
// scrubs it.
func (c config) videoFrame() float64 {
	if c.Matugen.VideoFrame <= 0 {
		return 1
	}
	return c.Matugen.VideoFrame
}

// persistVideoFrame patches only matugen.videoFrame so hand-edited keys survive.
func persistVideoFrame(sec float64) {
	raw := map[string]json.RawMessage{}
	loadJSON(configPath(), &raw)
	mat := map[string]interface{}{}
	if m, ok := raw["matugen"]; ok {
		_ = json.Unmarshal(m, &mat)
	}
	mat["videoFrame"] = sec
	b, err := json.Marshal(mat)
	if err != nil {
		return
	}
	raw["matugen"] = b
	_ = os.MkdirAll(ryokuConfigDir(), 0o755)
	out, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		return
	}
	saveRaw(configPath(), out)
}

// persistResourceTier patches only the resource_tier key so hand-edited keys in
// ryogami.json survive (the file is user-facing config, not daemon state).
func persistResourceTier(tier string) {
	raw := map[string]json.RawMessage{}
	loadJSON(configPath(), &raw)
	b, err := json.Marshal(tier)
	if err != nil {
		return
	}
	raw["resource_tier"] = b
	_ = os.MkdirAll(ryokuConfigDir(), 0o755)
	out, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		return
	}
	saveRaw(configPath(), out)
}

func saveRaw(path string, b []byte) {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return
	}
	_ = os.Rename(tmp, path)
}

// contentFit reads the shell's wallpaper fill mode from ryoku's shell.json,
// mirroring the Rust daemon: the shell owns that preference, the daemon only
// echoes it into published frames.
func contentFit() string {
	const def = "Cover"
	var shell struct {
		Wallpaper struct {
			Fit string `json:"content_fit"`
		} `json:"wallpaper"`
	}
	loadJSON(filepath.Join(ryokuConfigDir(), "shell.json"), &shell)
	switch shell.Wallpaper.Fit {
	case "Contain", "Cover", "Fill", "ScaleDown", "Center", "Tile":
		return shell.Wallpaper.Fit
	}
	return def
}

// wallPrefs are the video-engine knobs the shell owns in shell.json: the
// engine (ryogami C player or in-shell), the master switch, the bite-sized
// transcode, and the fps/width caps.
type wallTune struct {
	Engine     string `json:"video_engine"`
	Enabled    bool   `json:"video_enabled"`
	Transcode  bool   `json:"video_transcode"`
	TransFps   int    `json:"video_transcode_fps"`
	TransWidth int    `json:"video_transcode_width"`
}

func wallPrefs() wallTune {
	var shell struct {
		Wallpaper struct {
			Engine     *string `json:"video_engine"`
			Enabled    *bool   `json:"video_enabled"`
			Transcode  *bool   `json:"video_transcode"`
			TransFps   *int    `json:"video_transcode_fps"`
			TransWidth *int    `json:"video_transcode_width"`
		} `json:"wallpaper"`
	}
	p := wallTune{Engine: "ryogami", Enabled: true, TransFps: 24, TransWidth: 1920}
	loadJSON(filepath.Join(ryokuConfigDir(), "shell.json"), &shell)
	if shell.Wallpaper.Engine != nil {
		if e := *shell.Wallpaper.Engine; e == "in_shell" {
			p.Engine = "in_shell"
		}
	}
	if shell.Wallpaper.Enabled != nil {
		p.Enabled = *shell.Wallpaper.Enabled
	}
	if shell.Wallpaper.Transcode != nil {
		p.Transcode = *shell.Wallpaper.Transcode
	}
	if shell.Wallpaper.TransFps != nil && *shell.Wallpaper.TransFps > 0 {
		p.TransFps = *shell.Wallpaper.TransFps
	}
	if shell.Wallpaper.TransWidth != nil && *shell.Wallpaper.TransWidth > 0 {
		p.TransWidth = *shell.Wallpaper.TransWidth
	}
	return p
}

// wallAudio is the wall-ui's global video-audio default for a clip: whether it
// plays muted and at what volume (0-100). It is the fallback for any output a
// wall.apply audio map does not name.
type wallAudio struct {
	mute   bool
	volume int
}

// ryogamiWallConfigDir resolves the wall-ui config directory the way its
// Config.qml does: RYOGAMI_WALL_CONFIG when set, else $XDG_CONFIG_HOME (or
// ~/.config) + /ryogami-wall.
func ryogamiWallConfigDir() string {
	if d := os.Getenv("RYOGAMI_WALL_CONFIG"); d != "" {
		return d
	}
	if d := os.Getenv("XDG_CONFIG_HOME"); d != "" {
		return filepath.Join(d, "ryogami-wall")
	}
	return filepath.Join(home(), ".config", "ryogami-wall")
}

// wallAudioDefaults reads ~/.config/ryogami-wall/config.json for the picker's
// global audio knobs, mirroring wall-ui's Config.qml: wallpaperMute is true
// unless the key is exactly false, and wallpaperVolume defaults to 100, rounded
// and clamped to 0-100. An absent file yields muted at full volume.
func wallAudioDefaults() wallAudio {
	a := wallAudio{mute: true, volume: 100}
	var data struct {
		Mute   *bool    `json:"wallpaperMute"`
		Volume *float64 `json:"wallpaperVolume"`
	}
	loadJSON(filepath.Join(ryogamiWallConfigDir(), "config.json"), &data)
	if data.Mute != nil {
		a.mute = *data.Mute
	}
	if data.Volume != nil {
		a.volume = clampVolume(int(math.Round(*data.Volume)))
	}
	return a
}

// effectiveAudio resolves one output's clip audio: the explicit per-output value
// from a wall.apply audio map when the key is present, else the global default.
func effectiveAudio(key string, mute map[string]bool, volume map[string]int, def wallAudio) (bool, int) {
	m := def.mute
	if v, ok := mute[key]; ok {
		m = v
	}
	vol := def.volume
	if v, ok := volume[key]; ok {
		vol = clampVolume(v)
	}
	return m, vol
}

func clampVolume(v int) int {
	if v < 0 {
		return 0
	}
	if v > 100 {
		return 100
	}
	return v
}
