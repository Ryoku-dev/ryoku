package surface

import (
	"image"
	"math"

	"ryoku-rashin/internal/oni/render"
)

// setMicMark follows the voice phase: the spec says which phases keep the
// microphone open between presses, and while one does the mic mark shows
// beside the character. Nothing the user does hides it; with Oni itself
// hidden the surface shows the mark alone.
func (h *host) setMicMark(phase string) {
	mark := false
	if phase != "" {
		_, mark, _ = h.sp.VoiceState(phase)
	}
	if mark == h.micMark {
		return
	}
	h.micMark = mark
	reason := "off"
	if mark {
		reason = "on"
	}
	h.led.Add(Entry{Ev: EvMicMark, Reason: reason})
	h.dirty = true
	h.kick()
	h.syncMarkOnly()
}

// syncMarkOnly shows the mark alone while Oni is not shown and the
// microphone is open, and takes it down once either changes.
func (h *host) syncMarkOnly() {
	switch {
	case !h.shown && h.micMark && h.haveOut && h.char != nil:
		h.markOnly = true
		h.commitMarkOnly()
	case h.markOnly:
		h.markOnly = false
		if !h.shown {
			h.be.hide()
			h.led.Add(Entry{Ev: EvHide, Reason: "mic-mark", Output: h.out.ID})
		}
	}
}

// commitMarkOnly presents the mark where the character would be, with no
// input region: it is a notice, not a control.
func (h *host) commitMarkOnly() {
	if !h.markOnly || !h.haveOut || h.char == nil {
		return
	}
	origin, _, ext := h.sprite()
	if ext.X <= 0 || ext.Y <= 0 {
		return
	}
	dpr := h.be.scale(h.out.ID)
	phys := image.Pt(int(math.Ceil(float64(ext.X)*dpr)), int(math.Ceil(float64(ext.Y)*dpr)))
	if h.img == nil || h.img.Bounds().Size() != phys {
		h.img = image.NewNRGBA(image.Rectangle{Max: phys})
	}
	clear(h.img.Pix)
	render.PaintMicMark(h.img, render.MicMarkRect(ext), h.look, dpr)
	local := origin.Sub(h.out.Rect.Min)
	at, committed, err := h.be.present(presentation{output: h.out.ID, origin: local, size: ext, img: h.img})
	if err != nil {
		h.led.Add(Entry{Ev: EvCommit, Reason: "error", Output: h.out.ID})
		return
	}
	if committed {
		h.led.Add(Entry{Ev: EvCommit, Rect: rectOf(image.Rectangle{Min: at, Max: at.Add(ext)}), Output: h.out.ID, Scale: dpr, Mark: true, Reason: "mic-mark"})
	}
}

// paintMicMark draws the mark over the character just drawn into h.img.
func (h *host) paintMicMark(ext image.Point, dpr float64) bool {
	if !h.micMark {
		return false
	}
	render.PaintMicMark(h.img, render.MicMarkRect(ext), h.look, dpr)
	return true
}
