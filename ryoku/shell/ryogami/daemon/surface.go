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
	Paused     bool        `json:"paused,omitempty"`
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
	w.mu.Lock()
	defer w.mu.Unlock()
	w.seq++
	w.def = fresh(w.seq, pic, fit, tr)
	w.def.Live = live
	w.def.Video = isVideo
	w.def.VideoPath = clip.path
	w.def.Mute = clip.mute
	w.def.Volume = clip.volume
	w.outputs = map[string]frameEntry{}
	w.publishLocked()
}

// showOutput writes one per-output override, leaving the rest intact.
func (w *wallSurface) showOutput(name, pic, fit string, tr interface{}, live, isVideo bool, clip videoClip) {
	if name == "" {
		return
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	w.seq++
	e := fresh(w.seq, pic, fit, tr)
	e.Live = live
	e.Video = isVideo
	e.VideoPath = clip.path
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
