package main

import (
	"encoding/json"
	"sync"
)

// The in-shell wallpaper surface: a default entry plus per-output overrides,
// published as one `{default, outputs}` frame on the `wallpaper` topic. Ryoku's
// shell QML renders this frame directly; the entry keys are parsed verbatim by
// modules/wallpaper/WallpaperFrame.qml. Mute and Volume (0-100) are the
// in-shell clip's audio, meaningful only for a VideoPath frame.

type frameEntry struct {
	Path      string `json:"path"`
	Revision  int64  `json:"revision"`
	Fit       string `json:"fit"`
	Live      bool   `json:"live"`
	Video     bool   `json:"video,omitempty"`
	VideoPath string `json:"videoPath,omitempty"`
	// Paused freezes an in-shell clip in place without reloading the image.
	Paused bool `json:"paused,omitempty"`
	// Preview marks a paint wall.preview published but the daemon has not
	// committed: no assignment, count, or applied event backs it, so
	// downstream readers must not treat the entry as the wallpaper.
	Preview    bool        `json:"preview,omitempty"`
	Mute       bool        `json:"mute"`
	Volume     int         `json:"volume"`
	Transition interface{} `json:"transition"`
	Depth      string      `json:"depth"`
	DepthRev   int64       `json:"depthRev"`
}

type wallFrame struct {
	Default frameEntry            `json:"default"`
	Outputs map[string]frameEntry `json:"outputs"`
}

type wallSurface struct {
	mu      sync.Mutex
	seq     int64
	def     frameEntry
	outputs map[string]frameEntry
	topic   *stateTopic
}

func newWallSurface() *wallSurface {
	return &wallSurface{outputs: map[string]frameEntry{}, topic: newStateTopic()}
}

func (w *wallSurface) publishLocked() {
	f := wallFrame{Default: w.def, Outputs: map[string]frameEntry{}}
	for k, v := range w.outputs {
		f.Outputs[k] = v
	}
	if fitOverrides != nil {
		for out, fit := range fitOverrides() {
			e, ok := f.Outputs[out]
			if !ok {
				e = w.def
			}
			e.Fit = fit
			f.Outputs[out] = e
		}
	}
	b, err := json.Marshal(f)
	if err != nil {
		return
	}
	w.topic.publish(string(b))
}

// publishCurrent emits the current (initially empty) frame so a subscriber that
// connects before the first set still sees a defined frame.
func (w *wallSurface) publishCurrent() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.publishLocked()
}

func fresh(rev int64, pic, fit string, tr interface{}) frameEntry {
	// A fresh wallpaper needs a fresh cutout; the shell daemon's depth worker
	// regenerates it and hands it back over `depth set`.
	return frameEntry{Path: pic, Revision: rev, Fit: fit, Transition: tr}
}

// videoClip is the in-shell video part of a frame: the clip path plus the audio
// the shell's QtMultimedia player applies. The zero value (empty path) marks a
// still frame with no clip and no audio.
type videoClip struct {
	path   string
	mute   bool
	volume int
}

// show is the broadcast set: replace the default and clear every override.
// live is the ryogami-live yield flag; isVideo marks a frame whose path is a
// video's still; clip.path, when set, is the in-shell clip (video_engine
// "in_shell") and carries its audio.
func (w *wallSurface) show(pic, fit string, tr interface{}, live, isVideo bool, clip videoClip) {
	w.showEntry(pic, fit, tr, live, isVideo, false, clip)
}

func (w *wallSurface) showPreview(pic, fit string, tr interface{}, live, isVideo bool, clip videoClip) {
	w.showEntry(pic, fit, tr, live, isVideo, true, clip)
}

func commitMatchingPreview(e *frameEntry, pic, fit string, live, isVideo, preview bool, clip videoClip) bool {
	if preview || !e.Preview || e.Path != pic || live || clip.path != "" {
		return false
	}
	e.Fit = fit
	e.Live = false
	e.Video = isVideo
	e.VideoPath = ""
	e.Preview = false
	e.Mute = clip.mute
	e.Volume = clip.volume
	e.Transition = nil
	return true
}

func (w *wallSurface) showEntry(pic, fit string, tr interface{}, live, isVideo, preview bool, clip videoClip) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if commitMatchingPreview(&w.def, pic, fit, live, isVideo, preview, clip) {
		w.outputs = map[string]frameEntry{}
		w.publishLocked()
		return
	}
	w.seq++
	w.def = fresh(w.seq, pic, fit, tr)
	w.def.Live = live
	w.def.Video = isVideo
	w.def.VideoPath = clip.path
	w.def.Preview = preview
	w.def.Mute = clip.mute
	w.def.Volume = clip.volume
	w.outputs = map[string]frameEntry{}
	w.publishLocked()
}

// showOutput writes one per-output override, leaving the rest intact.
func (w *wallSurface) showOutput(name, pic, fit string, tr interface{}, live, isVideo bool, clip videoClip) {
	w.showOutputEntry(name, pic, fit, tr, live, isVideo, false, clip)
}

func (w *wallSurface) showOutputPreview(name, pic, fit string, tr interface{}, live, isVideo bool, clip videoClip) {
	w.showOutputEntry(name, pic, fit, tr, live, isVideo, true, clip)
}

func (w *wallSurface) showOutputEntry(name, pic, fit string, tr interface{}, live, isVideo, preview bool, clip videoClip) {
	if name == "" {
		return
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if current, ok := w.outputs[name]; ok && commitMatchingPreview(&current, pic, fit, live, isVideo, preview, clip) {
		w.outputs[name] = current
		w.publishLocked()
		return
	}
	w.seq++
	e := fresh(w.seq, pic, fit, tr)
	e.Live = live
	e.Video = isVideo
	e.VideoPath = clip.path
	e.Preview = preview
	e.Mute = clip.mute
	e.Volume = clip.volume
	w.outputs[name] = e
	w.publishLocked()
}

// republish re-emits the frame with fresh revisions, busting the downstream
// image cache after a re-rendered source (theme change) without a reveal.
func (w *wallSurface) republish() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.seq++
	w.def.Revision = w.seq
	for k, e := range w.outputs {
		e.Revision = w.seq
		w.outputs[k] = e
	}
	w.publishLocked()
}

// setAudio changes the live frame's in-shell clip audio and republishes it, so a
// running clip mutes or changes volume at once. A nil field is left untouched.
// An empty outputs list (or "*") addresses the broadcast default and every
// override; named updates also reach the default that an output may inherit.
// The revision is deliberately left alone: only the audio changed, so the shell
// must not reload the image or restart the clip.
func (w *wallSurface) setAudio(mute *bool, volume *int, outputs []string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	set := func(e *frameEntry) {
		if mute != nil {
			e.Mute = *mute
		}
		if volume != nil {
			e.Volume = *volume
		}
	}
	set(&w.def)
	all := len(outputs) == 0 || contains(outputs, "*")
	for k, e := range w.outputs {
		if all || contains(outputs, k) {
			set(&e)
			w.outputs[k] = e
		}
	}
	w.publishLocked()
}

func (w *wallSurface) snapshot() wallFrame {
	w.mu.Lock()
	defer w.mu.Unlock()
	f := wallFrame{Default: w.def, Outputs: map[string]frameEntry{}}
	for k, v := range w.outputs {
		f.Outputs[k] = v
	}
	return f
}

func (w *wallSurface) restorePreview(saved wallFrame, outputs []string) {
	w.restorePreviewKeeping(saved, outputs, nil)
}

// keep holds the preview paint for the commit landing next, so it needs no second reveal
func (w *wallSurface) restorePreviewKeeping(saved wallFrame, outputs, keep []string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	all := len(outputs) == 0 || contains(outputs, "*")
	held := make(map[string]frameEntry, len(keep))
	for _, output := range keep {
		if entry, ok := w.outputs[output]; ok {
			held[output] = entry
		} else if all && output != "" && output != "*" {
			held[output] = w.def
		}
	}
	restore := func(e frameEntry) frameEntry {
		w.seq++
		e.Revision = w.seq
		e.Transition = nil
		e.Preview = false
		return e
	}
	if all {
		w.def = restore(saved.Default)
		w.outputs = make(map[string]frameEntry, len(saved.Outputs))
		for output, entry := range saved.Outputs {
			w.outputs[output] = restore(entry)
		}
	} else {
		for _, output := range outputs {
			if entry, ok := saved.Outputs[output]; ok {
				w.outputs[output] = restore(entry)
			} else {
				delete(w.outputs, output)
			}
		}
	}
	for output, entry := range held {
		w.outputs[output] = entry
	}
	w.publishLocked()
}

// setDepth publishes a slot's cutout unless a switch mid-generation already
// moved the slot to another wallpaper; rev is the cutout's mtime so a
// regenerated file at the same path still busts the image cache.
func (w *wallSurface) setDepth(slot, source, out string, rev int64) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if slot == "" {
		if w.def.Path != source {
			return
		}
		w.def.Depth = out
		w.def.DepthRev = rev
	} else {
		e, okSlot := w.outputs[slot]
		if !okSlot || e.Path != source {
			return
		}
		e.Depth = out
		e.DepthRev = rev
		w.outputs[slot] = e
	}
	w.publishLocked()
}

func (w *wallSurface) clearDepth() {
	w.mu.Lock()
	defer w.mu.Unlock()
	changed := false
	if w.def.Depth != "" {
		w.def.Depth = ""
		w.def.DepthRev = 0
		changed = true
	}
	for k, e := range w.outputs {
		if e.Depth != "" {
			e.Depth = ""
			e.DepthRev = 0
			w.outputs[k] = e
			changed = true
		}
	}
	if changed {
		w.publishLocked()
	}
}
