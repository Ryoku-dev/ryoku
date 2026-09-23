package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// The apply path: publish the frame the shell QML renders, persist the
// per-output state for restore, bump the catalog's apply count, trigger the
// matugen palette, and broadcast the applied event the picker listens for.

// applyWallpaper handles static and video applies, dispatching on the
// video_engine knob: "ryogami" (the C player + READY handshake, default) or
// "in_shell" (the clip publishes as videoPath and the shell decodes it).
func (d *daemon) applyWallpaper(wpType, path, mode string, outputs []string, mute map[string]bool, volume map[string]int) error {
	if path == "" {
		return fmt.Errorf("missing 'path' parameter")
	}
	if _, err := os.Stat(path); err != nil {
		return fmt.Errorf("wallpaper not readable: %v", err)
	}
	fit := contentFit()
	isVideo := wpType == "video"
	paint := path
	prefs := wallPrefs()
	if isVideo {
		if still := liveStill(path, d.config().videoFrame()); still != "" {
			paint = still
		}
	}

	if isVideo && prefs.Engine == "in_shell" {
		if d.video.Playing() {
			d.video.Stop()
		}
		name := filepath.Base(path)
		key := strings.TrimSuffix(name, filepath.Ext(name))
		clip := path
		if !prefs.Enabled {
			clip = ""
		} else if entry, ok := d.store.get(keyFor(d.store, name, key)); ok && entry.VideoFile != "" && entry.VideoFile != path {
			// Animated image formats are transcoded to mp4 at scan time: the
			// player cannot advance those frames from the original.
			clip = entry.VideoFile
			if mp4Still := liveStill(entry.VideoFile, d.config().videoFrame()); mp4Still != "" {
				paint = mp4Still
			}
		} else if prefs.Transcode {
			if capped := transcodeCachePath(path, prefs.TransFps, prefs.TransWidth); capped != "" && fileExists(capped) {
				clip = capped
				if mp4Still := liveStill(capped, d.config().videoFrame()); mp4Still != "" {
					paint = mp4Still
				}
			} else {
				clip = ""
				d.transcodeAsync(path, outputs, prefs)
			}
		}
		clipAudio := frameAudio(outputs, mute, volume)
		d.surface.show(paint, fit, d.transitionFor(mode), false, true, videoClip{path: clip, mute: clipAudio.mute, volume: clipAudio.volume})
		d.setCurrent(name)
		d.saveOutputs(outputs, wpType, path, mute, volume)
		d.store.mutate(keyFor(d.store, name, key), func(e *Entry) { e.ApplyCount++ })
		d.broadcast("ryogami.wall.applied", map[string]interface{}{
			"type": wpType, "name": name, "path": path, "we_id": "", "key": key,
		})
		return nil
	}

	live := isVideo
	if !isVideo && d.video.Playing() {
		d.video.Stop()
	}
	// A reveal transition is an image operation: the clip's still gets one
	// too, so a switch onto or off a video animates like any other. Only a
	// video without a still falls back to a bare cut, with the live flag
	// telling the painter to yield immediately.
	frameLive := live && paint == path
	var tr interface{}
	if !frameLive {
		if picked := d.transitionFor(mode); picked != nil {
			tr = picked
		}
	}
	seq := d.paintSeq.Add(1)
	if live {
		// The player's READY/exit handshake swaps the painter between the
		// clip's still and yielding to the video surface below it. The yield
		// waits out the reveal so the transition is never cut short, and the
		// sequence guard drops a flip the user has already switched past.
		revealUntil := time.Now()
		if p, okT := tr.(*pickedTransition); okT {
			dur := p.DurationMs
			if dur <= 0 {
				dur = transitionDurationMs
			}
			revealUntil = revealUntil.Add(time.Duration(dur+150) * time.Millisecond)
		}
		repaint := func(l bool) {
			if l {
				if wait := time.Until(revealUntil); wait > 0 {
					time.Sleep(wait)
				}
			}
			if d.paintSeq.Load() != seq {
				return
			}
			d.repaintOutputs(outputs, paint, fit, l)
		}
		d.video.Play(outputs, path, liveFit(fit), d.config().ResourceTier, repaint)
	}
	if len(outputs) == 0 || contains(outputs, "*") {
		d.surface.show(paint, fit, tr, frameLive, live, videoClip{})
	} else {
		for _, out := range outputs {
			d.surface.showOutput(out, paint, fit, tr, frameLive, live, videoClip{})
		}
	}
	name := filepath.Base(path)
	d.setCurrent(name)
	d.saveOutputs(outputs, wpType, path, mute, volume)
	key := strings.TrimSuffix(name, filepath.Ext(name))
	d.store.mutate(keyFor(d.store, name, key), func(e *Entry) { e.ApplyCount++ })
	// The palette follows through ryoku-shell: its ryogami bridge watches the
	// frame and drives the matugen pipeline (the enriched template context the
	// deployed templates need), so the daemon never execs matugen itself.
	d.broadcast("ryogami.wall.applied", map[string]interface{}{
		"type": wpType, "name": name, "path": path, "we_id": "", "key": key,
	})
	return nil
}

// keyFor resolves the store key for an applied file: entries under subfolders
// carry the relative name, so a basename-derived key needs a fallback search.
func keyFor(s *store, name, key string) string {
	if _, okKey := s.get(key); okKey {
		return key
	}
	for _, e := range s.list(false) {
		if filepath.Base(e.Name) == name {
			return e.Key
		}
	}
	return key
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

// saveOutputs persists {output: {type, path, mute, volume}} to
// cacheDir/outputs.json for the startup restore, mirroring the Rust daemon: a
// broadcast apply clears the map to a single "*" entry, a per-output apply
// removes "*". Audio is the clip's effective value: the per-output apply map
// when it names the key, else the wall-ui global default (so a missing key
// stays muted at the configured volume instead of unmuting).
func (d *daemon) saveOutputs(outputs []string, wpType, path string, mute map[string]bool, volume map[string]int) {
	cacheDir := d.config().cacheDir()
	state := map[string]map[string]interface{}{}
	loadJSON(filepath.Join(cacheDir, "outputs.json"), &state)
	keys := outputs
	if len(keys) == 0 || contains(keys, "*") {
		keys = []string{"*"}
		state = map[string]map[string]interface{}{}
	} else {
		delete(state, "*")
	}
	def := wallAudioDefaults()
	for _, k := range keys {
		m, vol := effectiveAudio(k, mute, volume, def)
		state[k] = map[string]interface{}{"type": wpType, "path": path, "mute": m, "volume": vol}
	}
	_ = os.MkdirAll(cacheDir, 0o755)
	saveJSON(filepath.Join(cacheDir, "outputs.json"), state)
	syncWallState(state)
}

// frameAudio resolves the single broadcast in-shell frame's audio: the first
// named output's effective value, or the global default for a broadcast apply
// (no outputs) or one whose maps name none of them.
func frameAudio(outputs []string, mute map[string]bool, volume map[string]int) wallAudio {
	key := "*"
	if len(outputs) > 0 && !contains(outputs, "*") {
		key = outputs[0]
	}
	m, vol := effectiveAudio(key, mute, volume, wallAudioDefaults())
	return wallAudio{mute: m, volume: vol}
}

// wallStatePath is the legacy ~/.local/state/ryoku-wallpaper file: the absolute
// path of the current default (broadcast) wallpaper, newline-terminated.
// Ryogami owns wallpaper apply now, but rice capture, the overview backdrop, the
// Super+W on-air dot, and the shell's palette bridge still read this file to
// learn what is on screen. The old Go/C backend wrote it on every apply; when
// ryogami took the wallpaper over the write was dropped, so those readers saw a
// stale wallpaper (a saved rice and the overview showed the wrong wall). This
// restores the write.
func wallStatePath() string { return filepath.Join(stateHome(), "ryoku-wallpaper") }

// defaultWallpaperFrom picks the broadcast wallpaper from the persisted outputs
// map: the "*" entry when present, else the first per-output entry by sorted key
// so the single-path legacy file names one stable wallpaper even in a per-output
// setup. The stored path is the real file (a clip's own path for a live wall),
// which is what the readers want to bundle or repaint.
func defaultWallpaperFrom(state map[string]map[string]interface{}) string {
	if e, ok := state["*"]; ok {
		if p, _ := e["path"].(string); p != "" {
			return p
		}
	}
	keys := make([]string, 0, len(state))
	for k := range state {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if p, _ := state[k]["path"].(string); p != "" {
			return p
		}
	}
	return ""
}

// syncWallState mirrors the persisted default wallpaper into the legacy state
// file, so every apply and the startup restore keep the wallpaper readers honest.
func syncWallState(state map[string]map[string]interface{}) {
	dir := stateHome()
	_ = os.MkdirAll(dir, 0o755)
	_ = os.WriteFile(filepath.Join(dir, "ryoku-wallpaper"), []byte(defaultWallpaperFrom(state)+"\n"), 0o644)
}

// legacyWallStatePath: the shell's own choice from before Ryogami owned the
// wallpaper. Nothing writes it now.
func legacyWallStatePath() string { return filepath.Join(stateHome(), "ryoku-wallpaper.json") }

// migrateLegacyOutputs seeds outputs.json from the pre-split state once,
// only while no wallpaper is stored.
func (d *daemon) migrateLegacyOutputs() {
	cacheDir := d.config().cacheDir()
	cur := map[string]map[string]interface{}{}
	loadJSON(filepath.Join(cacheDir, "outputs.json"), &cur)
	for _, e := range cur {
		if p, _ := e["path"].(string); p != "" {
			return
		}
	}
	var legacy struct {
		Default string            `json:"default"`
		Outputs map[string]string `json:"outputs"`
	}
	loadJSON(legacyWallStatePath(), &legacy)
	state := map[string]map[string]interface{}{}
	def := wallAudioDefaults()
	if legacy.Default != "" {
		state["*"] = map[string]interface{}{"type": typeOf(legacy.Default), "path": legacy.Default, "mute": def.mute, "volume": def.volume}
	} else {
		for name, p := range legacy.Outputs {
			if p != "" {
				state[name] = map[string]interface{}{"type": typeOf(p), "path": p, "mute": def.mute, "volume": def.volume}
			}
		}
	}
	if len(state) == 0 {
		return
	}
	_ = os.MkdirAll(cacheDir, 0o755)
	saveJSON(filepath.Join(cacheDir, "outputs.json"), state)
	syncWallState(state)
	fmt.Fprintln(os.Stderr, "ryogami: migrated the pre-split wallpaper choice into outputs.json")
}

// webpHealRev marks the catalog revision that stopped forcing every webp onto
// the video path. An older daemon transcoded a single-frame webp to a looping
// clip and kept it decoding on the GPU, so a still image cost video-level
// power. Bump it if the classifier changes again so a healed box re-checks.
const webpHealRev = "1"

// healAnimatedWebp reclassifies webps the old classifier promoted to video. A
// single-frame webp becomes a still again -- painted once instead of decoded on
// a loop -- and the stored wallpaper choice drops the video type so a restore
// paints it as an image. The catalog and outputs.json both carry the stale
// type, and neither the rescan (warm-returns an unchanged mtime) nor the
// restore (reads outputs.json directly) fixes it on its own, so this runs once
// per revision before the first restore.
func (d *daemon) healAnimatedWebp() {
	if v, okKey := d.store.stateGet("webpHealRev"); okKey && v == webpHealRev {
		return
	}
	cfg := d.config()
	healed := 0
	for _, e := range d.store.list(false) {
		if e.Type != "video" || !strings.EqualFold(filepath.Ext(e.Name), ".webp") {
			continue
		}
		src := filepath.Join(cfg.wallpaperDir(), e.Name)
		if !fileExists(src) || isAnimatedImage(src) {
			continue
		}
		d.store.mutate(e.Key, func(en *Entry) {
			en.Type = "static"
			en.VideoFile = ""
		})
		healed++
	}
	// The stored choice carries the type a restore paints from; a webp recorded
	// as video would still boot into the player even after the catalog heals.
	cacheDir := cfg.cacheDir()
	outputs := map[string]map[string]interface{}{}
	loadJSON(filepath.Join(cacheDir, "outputs.json"), &outputs)
	changed := false
	for _, e := range outputs {
		p, _ := e["path"].(string)
		if e["type"] == "video" && strings.EqualFold(filepath.Ext(p), ".webp") &&
			fileExists(p) && !isAnimatedImage(p) {
			e["type"] = "static"
			changed = true
		}
	}
	if changed {
		saveJSON(filepath.Join(cacheDir, "outputs.json"), outputs)
		syncWallState(outputs)
	}
	if healed > 0 || changed {
		fmt.Fprintf(os.Stderr, "ryogami: reclassified %d static webp(s) off the video path\n", healed)
	}
	rev := webpHealRev
	d.store.stateSet("webpHealRev", &rev)
}

// restoreOutputs republishes the stored wallpaper; the caller retries while
// applied < want, since the file or the outputs can lag at login.
func (d *daemon) restoreOutputs() (want, applied int) {
	d.restoreMu.Lock()
	defer d.restoreMu.Unlock()
	cacheDir := d.config().cacheDir()
	state := map[string]map[string]interface{}{}
	loadJSON(filepath.Join(cacheDir, "outputs.json"), &state)
	for _, e := range state {
		if p, _ := e["path"].(string); p != "" {
			want++
		}
	}
	fit := contentFit()
	restored := ""
	restore := func(out string, e map[string]interface{}) {
		p, _ := e["path"].(string)
		if p == "" || !fileExists(p) {
			return
		}
		live := e["type"] == "video"
		paint := p
		prefs := wallPrefs()
		// The clip's poster frame, resolved here the same way the apply path does
		// it. Without this the stored .mp4 is published as the frame's image, the
		// shell's Image cannot decode it, and the desktop paints nothing (a grey
		// wallpaper) until the next manual apply. The engine-specific branches
		// below override paint with their own frame when they have one.
		if live {
			if still := liveStill(p, d.config().videoFrame()); still != "" {
				paint = still
			}
		}
		if live && prefs.Engine == "in_shell" {
			name := filepath.Base(p)
			key := strings.TrimSuffix(name, filepath.Ext(name))
			clip := p
			if !prefs.Enabled {
				clip = ""
			} else if entry, ok := d.store.get(keyFor(d.store, name, key)); ok && entry.VideoFile != "" && entry.VideoFile != p {
				clip = entry.VideoFile
				if mp4Still := liveStill(entry.VideoFile, d.config().videoFrame()); mp4Still != "" {
					paint = mp4Still
				}
			} else if prefs.Transcode {
				if capped := transcodeCachePath(p, prefs.TransFps, prefs.TransWidth); capped != "" && fileExists(capped) {
					clip = capped
					if mp4Still := liveStill(capped, d.config().videoFrame()); mp4Still != "" {
						paint = mp4Still
					}
				} else {
					clip = ""
					d.transcodeAsync(p, []string{out}, prefs)
				}
			}
			m, vol := entryAudio(e, wallAudioDefaults())
			if out == "*" {
				d.surface.show(paint, fit, nil, false, true, videoClip{path: clip, mute: m, volume: vol})
			} else {
				d.surface.showOutput(out, paint, fit, nil, false, true, videoClip{path: clip, mute: m, volume: vol})
			}
			restored = filepath.Base(p)
			applied++
			return
		}

		if live {
			var outs []string
			if out != "*" {
				outs = []string{out}
			}
			seq := d.paintSeq.Add(1)
			repaint := func(l bool) {
				if d.paintSeq.Load() != seq {
					return
				}
				d.repaintOutputs(outs, paint, fit, l)
			}
			d.video.Play(outs, p, liveFit(fit), d.config().ResourceTier, repaint)
		}
		frameLive := live && paint == p
		if out == "*" {
			d.surface.show(paint, fit, nil, frameLive, live, videoClip{})
		} else {
			d.surface.showOutput(out, paint, fit, nil, frameLive, live, videoClip{})
		}
		restored = filepath.Base(p)
		applied++
	}
	if e, okAll := state["*"]; okAll {
		restore("*", e)
	} else {
		for out, e := range state {
			restore(out, e)
		}
	}
	if restored != "" {
		d.setCurrent(restored)
		fmt.Fprintf(os.Stderr, "ryogami: auto-restored wallpaper: %s\n", restored)
	}
	syncWallState(state)
	return want, applied
}

// defaultWallpaper is the fallback the startup restore paints when nothing is
// recorded: the first static image in the configured wallpaper directory, in
// name order so the choice is deterministic. A box that never set a wallpaper
// through Ryogami (a fresh install, or one cut over from awww) would otherwise
// sit on the empty grey frame. Videos and animated formats (typeOf "video",
// e.g. .gif) are skipped so the fallback never lands on the live player, which
// the box may not be able to decode on every GPU.
func (d *daemon) defaultWallpaper() string {
	dir := d.config().wallpaperDir()
	ents, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	var names []string
	for _, e := range ents {
		if e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(e.Name()), "."))
		if imageExts[ext] && typeOf(e.Name()) == "static" {
			names = append(names, e.Name())
		}
	}
	if len(names) == 0 {
		return ""
	}
	sort.Strings(names)
	return filepath.Join(dir, names[0])
}

// applyDefaultWallpaper paints the shipped default in "init" mode (no reveal,
// straight onto the fresh backdrop) and persists it, so the desktop is never
// left blank and the next login restores this choice instead of re-picking.
func (d *daemon) applyDefaultWallpaper() {
	pick := d.defaultWallpaper()
	if pick == "" {
		return
	}
	if err := d.applyWallpaper("static", pick, "init", nil, nil, nil); err != nil {
		fmt.Fprintf(os.Stderr, "ryogami: default wallpaper %s: %v\n", pick, err)
	}
}

func fileExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.Mode().IsRegular()
}

// entryAudio resolves a persisted outputs.json entry's clip audio, falling back
// to the wall-ui global default for a key the entry predates (an old outputs.json
// with no volume, or a mute its writer never set).
func entryAudio(e map[string]interface{}, def wallAudio) (bool, int) {
	m := def.mute
	if v, ok := e["mute"].(bool); ok {
		m = v
	}
	vol := def.volume
	if v, ok := e["volume"].(float64); ok {
		vol = clampVolume(int(v))
	}
	return m, vol
}

// outputsState answers wall.outputs from the persisted map, echoing each entry's
// mute flag and volume the picker's monitor popup reads (audio routing itself is
// the shell's domain, so this is echoed state, not a mixer control).
func (d *daemon) outputsState() map[string]interface{} {
	state := map[string]map[string]interface{}{}
	loadJSON(filepath.Join(d.config().cacheDir(), "outputs.json"), &state)
	def := wallAudioDefaults()
	out := map[string]interface{}{}
	for k, e := range state {
		m, vol := entryAudio(e, def)
		entry := map[string]interface{}{"type": e["type"], "mute": m, "volume": vol}
		if p, okPath := e["path"].(string); okPath {
			entry["path"] = p
		}
		out[k] = entry
	}
	return out
}

// setAudio persists an audio change for the addressed outputs (all when none are
// given) to outputs.json, then republishes the live frame so a running in-shell
// clip mutes or changes volume at once. A nil mute or volume leaves that field.
func (d *daemon) setAudio(mute *bool, volume *int, outputs []string) {
	cacheDir := d.config().cacheDir()
	state := map[string]map[string]interface{}{}
	loadJSON(filepath.Join(cacheDir, "outputs.json"), &state)
	for k, e := range state {
		if len(outputs) > 0 && !contains(outputs, k) {
			continue
		}
		if mute != nil {
			e["mute"] = *mute
		}
		if volume != nil {
			e["volume"] = *volume
		}
		state[k] = e
	}
	saveJSON(filepath.Join(cacheDir, "outputs.json"), state)
	d.surface.setAudio(mute, volume, outputs)
}

// deleteWallpaper removes the source file and the catalog row, then tells
// every client the file is gone.
func (d *daemon) deleteWallpaper(key string) error {
	e, okKey := d.store.remove(key)
	if !okKey {
		return fmt.Errorf("unknown wallpaper: %s", key)
	}
	src := e.VideoFile
	if src == "" {
		src = filepath.Join(d.config().wallpaperDir(), e.Name)
	}
	if err := os.Remove(src); err != nil && !os.IsNotExist(err) {
		return err
	}
	for _, t := range []string{e.Thumb, e.ThumbSm} {
		if t != "" {
			_ = os.Remove(t)
		}
	}
	d.broadcast("ryogami.wall.file_removed", map[string]interface{}{"name": e.Name, "type": e.Type})
	return nil
}

// importWallpaper copies a file into the wallpaper dir and rescans, so the new
// entry flows to clients through the cached event.
func (d *daemon) importWallpaper(src string) error {
	if !fileExists(src) {
		return fmt.Errorf("source not readable: %s", src)
	}
	dst := filepath.Join(d.config().wallpaperDir(), filepath.Base(src))
	b, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	if err := os.WriteFile(dst, b, 0o644); err != nil {
		return err
	}
	now := time.Now()
	_ = os.Chtimes(dst, now, now)
	go d.rescan(true)
	return nil
}

// marshalable sanity check for events carrying Entry values.
var _ = json.Marshal

// repaintOutputs republishes paint on the apply's output set with the given
// live flag and no transition: the READY/exit handshake's frame swaps are
// cuts, never reveals.
func (d *daemon) repaintOutputs(outputs []string, paint, fit string, live bool) {
	if len(outputs) == 0 || contains(outputs, "*") {
		d.surface.show(paint, fit, nil, live, true, videoClip{})
		return
	}
	for _, out := range outputs {
		d.surface.showOutput(out, paint, fit, nil, live, true, videoClip{})
	}
}

// transcodeAsync builds the in-shell transcode cache off the hot path, then
// re-applies the same wallpaper so the frame points at the bite-sized cache.
func (d *daemon) transcodeAsync(path string, outputs []string, prefs wallTune) {
	go func() {
		capped := ensureVideoTranscode(path, prefs.TransFps, prefs.TransWidth)
		if capped == "" {
			return
		}
		_ = d.applyWallpaper("video", path, "live-reload", outputs, nil, nil)
	}()
}
