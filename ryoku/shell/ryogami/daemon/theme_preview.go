package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"
)

const previewCommandTimeout = 10 * time.Second

var previewColorGenerator = matugenPreviewColors

func matugenPreviewColors(img string) (map[string]string, error) {
	if img == "" {
		return nil, fmt.Errorf("no image to preview")
	}
	ctx, cancel := context.WithTimeout(context.Background(), previewCommandTimeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, "ryoku-shell", "matugen-preview", img).Output()
	if ctx.Err() != nil {
		return nil, fmt.Errorf("matugen-preview timed out after %s", previewCommandTimeout)
	}
	if err != nil {
		return nil, fmt.Errorf("matugen-preview: %w", err)
	}
	var doc struct {
		Colors map[string]string `json:"colors"`
	}
	if json.Unmarshal(out, &doc) != nil || len(doc.Colors) == 0 {
		return nil, fmt.Errorf("matugen-preview returned no colours")
	}
	return doc.Colors, nil
}

type cachedPreview struct {
	mtime  int64
	colors map[string]string
}

var (
	previewMu       sync.Mutex
	previewRenderMu sync.Mutex
	previewCache    = map[string]cachedPreview{}
)

// Cached per key and mtime so a repeated hover is instant.
func (d *daemon) previewColorsFor(key string) (map[string]string, error) {
	entry, ok := d.store.get(key)
	if !ok {
		return nil, fmt.Errorf("unknown wallpaper %q", key)
	}
	img := entry.Thumb
	if img == "" {
		img = entry.Path
	}
	previewMu.Lock()
	if c, ok := previewCache[key]; ok && c.mtime == entry.Mtime {
		colors := c.colors
		previewMu.Unlock()
		return colors, nil
	}
	previewMu.Unlock()

	previewRenderMu.Lock()
	defer previewRenderMu.Unlock()
	previewMu.Lock()
	if c, ok := previewCache[key]; ok && c.mtime == entry.Mtime {
		colors := c.colors
		previewMu.Unlock()
		return colors, nil
	}
	previewMu.Unlock()

	colors, err := previewColorGenerator(img)
	if err != nil {
		return nil, err
	}
	previewMu.Lock()
	previewCache[key] = cachedPreview{mtime: entry.Mtime, colors: colors}
	previewMu.Unlock()
	return colors, nil
}

func (d *daemon) palettePreview(p map[string]interface{}) (map[string]interface{}, error) {
	key := strParam(p, "key", "")
	if key == "" {
		return nil, fmt.Errorf("palette.preview requires a key")
	}
	colors, err := d.previewColorsFor(key)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{"palette": nineFromColors(colors)}, nil
}

// One preview at a time; the saved palette returns on hover end, hide, disconnect or timeout.
type hoverPreviewState struct {
	mu       sync.Mutex
	active   bool
	hadFile  bool
	saved    []byte
	debounce *time.Timer
	hold     *time.Timer
	epoch    uint64
}

var hoverPreview = &hoverPreviewState{}

const (
	hoverDebounce = 120 * time.Millisecond
	hoverMaxHold  = 12 * time.Second
)

// Debounced so a fast sweep only paints the card it settles on.
func (h *hoverPreviewState) hover(d *daemon, key string) {
	if key == "" {
		h.end()
		return
	}
	h.mu.Lock()
	h.epoch++
	epoch := h.epoch
	if h.debounce != nil {
		h.debounce.Stop()
	}
	h.debounce = time.AfterFunc(hoverDebounce, func() { h.settle(d, key, epoch) })
	h.mu.Unlock()
}

// The generator runs outside the lock so a slow decode never stalls the socket.
func (h *hoverPreviewState) settle(d *daemon, key string, epoch uint64) {
	h.mu.Lock()
	stale := epoch != h.epoch
	h.mu.Unlock()
	if stale {
		return
	}
	colors, err := d.previewColorsFor(key)
	if err != nil {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if epoch != h.epoch {
		return
	}
	if !h.active {
		h.saved, h.hadFile = readColorsFile()
		h.active = true
	}
	writeColorsFile(colors)
	if h.hold != nil {
		h.hold.Stop()
	}
	h.hold = time.AfterFunc(hoverMaxHold, h.end)
	themeReloadNudge()
}

func (h *hoverPreviewState) end() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.epoch++
	if h.debounce != nil {
		h.debounce.Stop()
		h.debounce = nil
	}
	if h.hold != nil {
		h.hold.Stop()
		h.hold = nil
	}
	if !h.active {
		return
	}
	h.active = false
	if h.hadFile {
		_ = os.WriteFile(themeColorsPath(), h.saved, 0o644)
	} else {
		_ = os.Remove(themeColorsPath())
	}
	h.saved = nil
	themeReloadNudge()
}

func readColorsFile() ([]byte, bool) {
	b, err := os.ReadFile(themeColorsPath())
	if err != nil {
		return nil, false
	}
	return b, true
}

// Written in place, not renamed, so the shell's file watch fires reliably.
func writeColorsFile(colors map[string]string) {
	path := themeColorsPath()
	_ = os.MkdirAll(filepath.Dir(path), 0o755)
	b, err := json.MarshalIndent(colors, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(path, b, 0o644)
}

func themeReloadNudge() {
	go func() { _ = exec.Command("qs", "-c", "shell", "ipc", "call", "theme", "reload").Run() }()
}

func (d *daemon) paletteHover(p map[string]interface{}) (map[string]interface{}, error) {
	key := strParam(p, "key", "")
	if key == "" {
		hoverPreview.end()
		return map[string]interface{}{"ok": true}, nil
	}
	if !d.settingBool("shell.hoverPreview") {
		return map[string]interface{}{"ok": true, "previewing": false}, nil
	}
	hoverPreview.hover(d, key)
	return map[string]interface{}{"ok": true, "previewing": true}, nil
}

func (d *daemon) restoreHoverPreview() { hoverPreview.end() }

func init() {
	watchSetting("shell.hoverPreview", func(d *daemon, _ string, v interface{}) {
		if on, ok := v.(bool); ok && !on {
			hoverPreview.end()
		}
	})
}
