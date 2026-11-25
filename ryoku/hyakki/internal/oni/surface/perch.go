package surface

import (
	"context"
	"errors"
	"image"
	"math"
	"slices"
	"sort"
	"time"

	"ryoku-rashin/internal/oni/feature"
	"ryoku-rashin/internal/oni/placement"
	"ryoku-rashin/internal/oni/render"
	"ryoku-rashin/internal/oni/spec"
	"ryoku-rashin/internal/oni/strand"
	"ryoku-rashin/internal/protocol"
)

// Detach reasons the surface records: the daemon's closed set, plus daemon
// for a detach the daemon sent.
const (
	detachUser    = feature.PerchUser
	detachDragged = feature.PerchDraggedOff
	detachHidden  = feature.PerchHidden
	detachOff     = feature.PerchOniOff
	detachLost    = feature.PerchPresenterLost
	detachOutput  = feature.PerchOutputGone
	detachDaemon  = "daemon"

	perchMove     = "move"
	perchReattach = "reattach"

	highlightNone   = "none"
	highlightDrop   = "drop"
	highlightHidden = "hidden"
)

// perched is the window the daemon perched the character on: its frame in
// its output's local logical px and the top corner the character sits by.
type perched struct {
	seq                uint64
	window, app, title string
	corner, output     string
	box                strand.Rect
}

func (p *perched) info() *PerchInfo {
	return &PerchInfo{Window: p.window, Corner: p.corner, Box: p.box}
}

// chipText names the perched window: its app and title, or whichever it
// has, or its id.
func (p *perched) chipText() string {
	switch {
	case p.app != "" && p.title != "":
		return p.app + " · " + p.title
	case p.title != "":
		return p.title
	case p.app != "":
		return p.app
	}
	return p.window
}

// fader is one perch overlay surface fading in or out with the host's
// steps: alpha moves to 1 while want, else to 0, and the surface is
// unmapped (its buffer freed) at 0.
type fader struct {
	alpha float64
	want  bool
	dirty bool // new pixels are owed
	shown bool // the surface is mapped with a buffer
	img   *image.NRGBA
	rect  image.Rectangle // where it is, output-local logical px
}

// perchHost is the host's half of a perch: the drag highlight and its
// hover calls, the chip, the strand held for the flight, the reattach
// wait and the overlay rects the daemon was told.
type perchHost struct {
	perch *perched
	// heldPerch is a perch strand that waits for the flight to the seat;
	// perchT is the perch strand the engine took, kept to curl again.
	heldPerch *strand.Target
	perchT    *strand.Target
	// reperch curls the thread again when the flight lands
	reperch   bool
	settle    *time.Timer
	settleGen int
	// homeWait: the window held still while the thread was still winding
	// in, so the flight waits for it to be home
	homeWait bool

	hoverBusy bool
	hoverNext *spec.Point // output-local, the latest pointer to hit-test
	hoverOut  string
	hlBox     *strand.Rect
	hl        fader

	chip        fader
	chipLay     render.Chip
	chipText    string
	chipAt      *spec.Point
	chipPressed bool

	// ovSent is what the daemon was told, by output; ovBusy a report in
	// flight and ovPending another owed after it
	ovSent    map[string][]protocol.OniRect
	ovBusy    bool
	ovPending bool
	ovBuf     [feature.MaxOverlayRects]protocol.OniRect
	// ovMerge is scratch for squeezing the strand's rects into the room
	// the other parts leave
	ovMerge []image.Rectangle
	// strandMask masks the committed strand
	strandMask strandMask
}

// --- perch frames ---

func (h *host) perchFrame(f *feature.PerchFrame) {
	if !f.Attached || f.Box == nil {
		if h.dropPerch(detachDaemon) {
			h.reposition(true)
		}
		return
	}
	if p := h.perch; p != nil && f.Seq <= p.seq {
		return
	}
	np := &perched{seq: f.Seq, window: f.Window, app: f.App, title: f.Title, corner: f.Corner, output: f.Output,
		box: strand.Rect{X: f.Box.X, Y: f.Box.Y, W: f.Box.W, H: f.Box.H}}
	if np.corner != strand.CornerTopLeft && np.corner != strand.CornerTopRight {
		np.corner = h.nearerCorner(np.box)
	}
	old := h.perch
	h.perch = np
	if t := np.chipText(); t != h.chipText {
		h.chipText, h.chipLay = t, render.LayoutChip(t, h.sp)
		h.chip.dirty = true
	}
	if old == nil || old.window != np.window {
		h.led.Add(Entry{Ev: EvPerch, Seq: np.seq, Output: np.output, Perch: np.info()})
		h.cancelSettle()
		h.heldPerch, h.perchT, h.reperch = nil, nil, false
		h.reposition(true)
		return
	}
	h.movePerch(old, np)
}

// nearerCorner is the top corner of box the character would perch by from
// where it stands, when the daemon named none.
func (h *host) nearerCorner(box strand.Rect) string {
	w, ht := float64(h.out.Rect.Dx()), float64(h.out.Rect.Dy())
	var from spec.Point
	if h.char != nil && h.haveOut {
		from = h.anchor()
	}
	return strand.PerchCorner(h.sp, box, from, spec.Point{X: w / 2, Y: ht / 2}, strand.Rect{W: w, H: ht})
}

// movePerch follows the perched window to its new frame: a short move
// carries the character and the curl with it, a long one waits for the
// window to hold still, then flies and curls again.
func (h *host) movePerch(old, np *perched) {
	o, ok := h.perchOutput(np)
	if !ok {
		return
	}
	from, to := h.seat(o, old), h.seat(o, np)
	switch {
	case old.output != np.output:
		// the daemon sends the perch strand again for the new output
		h.led.Add(Entry{Ev: EvPerch, Seq: np.seq, Output: np.output, Perch: np.info(), Reason: perchMove})
		h.cancelSettle()
		h.reposition(false)
	case h.reduced():
		h.led.Add(Entry{Ev: EvPerch, Seq: np.seq, Output: np.output, Perch: np.info(), Reason: perchMove})
		h.cancelSettle()
		h.reposition(false)
		h.curlAgain()
		h.kick()
	case h.settle == nil && strand.PerchJumps(h.sp, ptOf(from), ptOf(to)):
		h.led.Add(Entry{Ev: EvPerch, Seq: np.seq, Output: np.output, Perch: np.info(), Reason: perchMove})
		h.reposition(false)
		if h.strd.Reframe(np.box) {
			h.led.Add(Entry{Ev: EvStrandReframe, Seq: h.topSeq, Box: &np.box})
			h.followTarget(np.box)
			h.strandRun = true
			h.kick()
		}
	default:
		h.led.Add(Entry{Ev: EvPerch, Seq: np.seq, Output: np.output, Perch: np.info(), Reason: perchReattach})
		if h.settle == nil && h.target != nil && h.strd.Out() {
			h.strd.Retract(h.target.Seq)
			h.led.Add(Entry{Ev: EvStrandRetract, Seq: h.target.Seq, Reason: perchReattach})
			h.bubble = nil
			h.strandRun = true
			h.kick()
		}
		h.armSettle()
	}
}

func ptOf(p image.Point) spec.Point { return spec.Point{X: float64(p.X), Y: float64(p.Y)} }

// followTarget keeps the host's copy of the perch strand on the frame.
func (h *host) followTarget(box strand.Rect) {
	if h.perchT != nil {
		h.perchT.Box, h.perchT.Frame = box, nil
	}
	if h.target != nil && h.target.Mode == strand.ModePerch {
		h.target.Box, h.target.Frame = box, nil
	}
}

// curlAgain has the engine reach and curl the perch strand on the perch's
// frame again, if it took one. It may run inside a step, so it does not
// step; a caller outside one kicks.
func (h *host) curlAgain() {
	p := h.perch
	if p == nil || h.perchT == nil {
		return
	}
	if h.strd.Reperch(p.box) {
		h.led.Add(Entry{Ev: EvStrandReperch, Seq: h.perchT.Seq, Box: &p.box})
		h.followTarget(p.box)
		t := *h.perchT
		h.target = &t
		h.strandRun = true
	}
}

// perchOutput is the output the perch is on, if the display has it.
func (h *host) perchOutput(p *perched) (placement.Output, bool) {
	for _, o := range h.be.outputs() {
		if o.ID == p.output {
			return o, true
		}
	}
	return placement.Output{}, false
}

// seat is where the character's frame sits by p on o, global px.
func (h *host) seat(o placement.Output, p *perched) image.Point {
	screen := strand.Rect{W: float64(o.Rect.Dx()), H: float64(o.Rect.Dy())}
	pt := strand.PerchSeat(h.sp, p.box, p.corner, float64(h.extent().X), screen)
	return o.Rect.Min.Add(image.Pt(int(math.Round(pt.X)), int(math.Round(pt.Y))))
}

// flightDone runs when the character stands where it was going: a perch
// strand held for the flight goes to the engine, a reattach curls again.
func (h *host) flightDone() {
	if t := h.heldPerch; t != nil {
		h.heldPerch = nil
		h.giveTarget(*t)
	}
	if h.reperch {
		h.reperch = false
		h.curlAgain()
	}
}

func (h *host) armSettle() {
	h.cancelSettle()
	gen := h.settleGen
	scale := h.lab.TimeScale
	if !(scale > 0) {
		scale = 1
	}
	d := time.Duration(h.sp.Perch.Reattach.SettleMs * float64(time.Millisecond) / scale)
	h.settle = time.AfterFunc(d, func() {
		h.be.post(func() {
			if gen == h.settleGen {
				h.settle = nil
				h.settled()
			}
		})
	})
}

func (h *host) cancelSettle() {
	h.settleGen++
	h.homeWait = false
	if h.settle != nil {
		h.settle.Stop()
		h.settle = nil
	}
}

// settled flies the character to the seat of a window that held still,
// and curls the thread again when it lands. A thread still winding in
// is let finish first: the flight starts once it is home (strandHome).
func (h *host) settled() {
	if h.perch == nil {
		return
	}
	if h.strd.Out() {
		h.homeWait = true
		return
	}
	h.reperch = true
	h.reposition(true)
}

// strandHome runs inside a step when the thread has come home: a reattach
// that waited for it flies now, after the step.
func (h *host) strandHome() {
	if !h.homeWait {
		return
	}
	h.homeWait = false
	gen := h.settleGen
	h.be.post(func() {
		if gen == h.settleGen && h.settle == nil {
			h.settled()
		}
	})
}

// dropPerch forgets the perch here; it reports whether there was one. The
// chip fades out with the host's steps; the daemon is told elsewhere, and
// the caller moves the character.
func (h *host) dropPerch(reason string) bool {
	if h.perch == nil {
		return false
	}
	h.led.Add(Entry{Ev: EvDetach, Reason: reason, Output: h.out.ID})
	h.perch, h.heldPerch, h.perchT, h.reperch = nil, nil, nil, false
	h.cancelSettle()
	h.chip.want = false
	h.chipAt, h.chipPressed = nil, false
	h.reportOverlay()
	return true
}

// detach takes the perch down for one of the presenter's own reasons and
// tells the daemon.
func (h *host) detach(reason string) {
	if h.perch == nil {
		return
	}
	h.dropPerch(reason)
	if l := h.link; l != nil {
		h.call(func(ctx context.Context) error { return l.Detach(ctx, reason) })
	}
}

// tearPerch takes everything perch-shaped off the screen at once, for a
// hide or a quit: the drag highlight, the chip and the perch itself.
func (h *host) tearPerch(reason string) {
	h.hideHighlight(highlightHidden)
	h.hoverNext = nil
	h.dropPerch(reason)
	h.chip.alpha = 0
	h.commitOverlay(overlayChip, &h.chip, PartChip, image.Rectangle{}, image.Rectangle{}, nil)
}

// --- drag, hover, drop ---

// hover asks the daemon which window is under the dragged character's
// pointer: at most one call in flight, the latest point next.
func (h *host) hover(p spec.Point) {
	if h.link == nil || !h.haveOut {
		return
	}
	local := spec.Point{X: p.X - float64(h.out.Rect.Min.X), Y: p.Y - float64(h.out.Rect.Min.Y)}
	h.hoverNext, h.hoverOut = &local, h.out.ID
	if !h.hoverBusy {
		h.sendHover()
	}
}

func (h *host) sendHover() {
	p, out, l := *h.hoverNext, h.hoverOut, h.link
	h.hoverNext, h.hoverBusy = nil, true
	go func() {
		ctx, cancel := context.WithTimeout(h.ctx, callTimeout)
		defer cancel()
		res, err := l.Perch(ctx, protocol.OniPerchParams{Output: out, X: p.X, Y: p.Y, Hover: true})
		h.be.post(func() { h.hovered(l, out, res, err) })
	}()
}

func (h *host) hovered(l Link, out string, res protocol.OniPerchResult, err error) {
	h.hoverBusy = false
	if l != h.link || !h.grip.dragging {
		h.hoverNext = nil
		return
	}
	if err == nil && res.Window != "" && res.Box != nil && res.Output == out && out == h.out.ID {
		box := strand.Rect{X: res.Box.X, Y: res.Box.Y, W: res.Box.W, H: res.Box.H}
		if h.hlBox == nil || *h.hlBox != box || !h.hl.want {
			h.led.Add(Entry{Ev: EvHighlight, Output: out, Box: &box})
		}
		h.hlBox, h.hl.want = &box, true
		h.hl.dirty = true
	} else if h.hl.want {
		h.hl.want = false
		h.led.Add(Entry{Ev: EvHighlight, Output: out, Reason: highlightNone})
	}
	if h.hoverNext != nil {
		h.sendHover()
	}
	h.kick()
}

// hideHighlight takes the drag highlight down at once.
func (h *host) hideHighlight(reason string) {
	if h.hlBox == nil && !h.hl.shown {
		return
	}
	if h.hl.want || h.hl.shown {
		h.led.Add(Entry{Ev: EvHighlight, Output: h.out.ID, Reason: reason})
	}
	h.hlBox, h.hl.want, h.hl.alpha = nil, false, 0
	h.commitOverlay(overlayHighlight, &h.hl, PartHighlight, image.Rectangle{}, image.Rectangle{}, nil)
}

// dropOn asks the daemon to perch on the window under the pointer where the
// drag ended. A refusal (the empty desktop, a window it cannot place)
// leaves the character where it was dropped, as a plain drop.
func (h *host) dropOn(fr placement.Saved) {
	out := h.out.ID
	if h.link == nil || h.pointer == nil {
		h.placeAt(out, fr)
		return
	}
	pt := spec.Point{X: h.pointer.X - float64(h.out.Rect.Min.X), Y: h.pointer.Y - float64(h.out.Rect.Min.Y)}
	l := h.link
	go func() {
		ctx, cancel := context.WithTimeout(h.ctx, callTimeout)
		defer cancel()
		res, err := l.Perch(ctx, protocol.OniPerchParams{Output: out, X: pt.X, Y: pt.Y})
		h.be.post(func() { h.droppedOn(l, out, fr, res, err) })
	}()
}

func (h *host) droppedOn(l Link, out string, fr placement.Saved, res protocol.OniPerchResult, err error) {
	if err == nil {
		h.call(func(ctx context.Context) error { return l.Interact(ctx, "drop") })
		return
	}
	reason := "refused"
	if pe := (*protocol.Error)(nil); errors.As(err, &pe) && pe.Reason != "" {
		reason = pe.Reason
	}
	h.led.Add(Entry{Ev: EvPerch, Output: out, Reason: reason})
	if l != h.link {
		return
	}
	h.placeAt(out, fr)
}

// placeAt saves fr as the character's spot on out and tells the daemon.
func (h *host) placeAt(out string, fr placement.Saved) {
	h.place.Corner, h.place.Active = "", out
	if h.place.Outputs == nil {
		h.place.Outputs = map[string]placement.Saved{}
	}
	h.place.Outputs[out] = fr
	h.withLink(func(l Link) {
		h.call(func(ctx context.Context) error {
			return l.Place(ctx, protocol.OniPlaceParams{Output: out, FX: fr.FX, FY: fr.FY})
		})
		h.call(func(ctx context.Context) error { return l.Interact(ctx, "drop") })
	})
	h.led.Add(Entry{Ev: EvPlace, Output: out, FX: new(fr.FX), FY: new(fr.FY)})
}

// --- chip input ---

func (h *host) chipPointer(p *spec.Point) {
	h.chipAt = p
	if p == nil {
		h.chipPressed = false
	}
}

func (h *host) chipButton(pressed bool) {
	on := h.chipAt != nil && h.chip.shown && inRect(*h.chipAt, h.chipLay.Detach)
	if pressed {
		h.chipPressed = on
		return
	}
	if h.chipPressed && on {
		h.chipPressed = false
		h.detach(detachUser)
		h.reposition(true)
		return
	}
	h.chipPressed = false
}

func inRect(p spec.Point, r strand.Rect) bool {
	return p.X >= r.X && p.X < r.X+r.W && p.Y >= r.Y && p.Y < r.Y+r.H
}

// --- overlay surfaces ---

// stepFaders moves the overlays toward where they want to be; it reports
// whether any is still on its way.
func (h *host) stepFaders(dt time.Duration) bool {
	h.chip.want = h.perch != nil && h.fl == nil && h.shown
	h.card.want = h.cardWanted()
	h.pill.f.want = h.handsWanted(&h.pill)
	h.handoff.f.want = h.handsWanted(&h.handoff)
	h.notice.f.want = h.handsWanted(&h.notice)
	hl, ch := h.sp.Perch.Highlight, h.sp.Perch.Chip
	a := h.fade(&h.hl, dt, hl.FadeInS, hl.FadeOutS)
	b := h.fade(&h.chip, dt, ch.FadeS, ch.FadeS)
	c := h.fade(&h.card, dt, ch.FadeS, ch.FadeS)
	d := h.fade(&h.pill.f, dt, ch.FadeS, ch.FadeS)
	e := h.fade(&h.handoff.f, dt, ch.FadeS, ch.FadeS)
	f := h.fade(&h.notice.f, dt, ch.FadeS, ch.FadeS)
	return a || b || c || d || e || f
}

func (h *host) fade(f *fader, dt time.Duration, in, out float64) bool {
	target, dur := 0.0, out
	if f.want {
		target, dur = 1, in
	}
	if f.alpha == target {
		return false
	}
	if h.reduced() || !(dur > 0) {
		f.alpha = target
	} else if step := dt.Seconds() / dur; target > f.alpha {
		f.alpha = min(target, f.alpha+step)
	} else {
		f.alpha = max(target, f.alpha-step)
	}
	f.dirty = true
	return f.alpha != target
}

// commitOverlays puts the highlight, the chip, the guide card and Hands'
// pill and card where they belong, with new pixels only when they changed.
func (h *host) commitOverlays() {
	if !h.haveOut {
		return
	}
	var hlRect image.Rectangle
	if b := h.hlBox; b != nil {
		m := render.HighlightMargin(h.sp)
		hlRect = image.Rect(int(math.Floor(b.X-m)), int(math.Floor(b.Y-m)), int(math.Ceil(b.X+b.W+m)), int(math.Ceil(b.Y+b.H+m)))
	}
	h.commitOverlay(overlayHighlight, &h.hl, PartHighlight, hlRect, image.Rectangle{}, func(img *image.NRGBA, dpr float64) {
		render.PaintHighlight(img, hlRect.Min, *h.hlBox, h.sp, h.look, dpr, h.hl.alpha)
	})
	chipRect := h.chipRect()
	h.commitOverlay(overlayChip, &h.chip, PartChip, chipRect, image.Rectangle{Max: chipRect.Size()}, func(img *image.NRGBA, dpr float64) {
		render.PaintChip(img, h.chipLay, h.sp, h.look, dpr, h.chip.alpha)
	})
	h.commitCard()
	h.commitHands()
}

// chipRect is where the chip goes, output-local: centred under the
// character's frame, or over it when under would cross the seat margin,
// and kept that margin inside the output.
func (h *host) chipRect() image.Rectangle {
	if h.chipText == "" || h.char == nil {
		return image.Rectangle{}
	}
	ext := h.extent()
	at := h.origin.Sub(h.out.Rect.Min)
	size := image.Pt(int(math.Ceil(h.chipLay.Size.X)), int(math.Ceil(h.chipLay.Size.Y)))
	gap := int(math.Round(h.sp.Perch.Chip.GapPx))
	m := int(math.Round(h.sp.Perch.Seat.MarginPx))
	w, ht := h.out.Rect.Dx(), h.out.Rect.Dy()
	x := at.X + ext.X/2 - size.X/2
	y := at.Y + ext.Y + gap
	if y+size.Y > ht-m {
		y = at.Y - gap - size.Y
	}
	x = max(m, min(x, w-m-size.X))
	return image.Rectangle{Min: image.Pt(x, y), Max: image.Pt(x, y).Add(size)}
}

// commitOverlay shows f at rect (output-local) with input region input,
// painting new pixels when it owes them, or takes it down at alpha 0.
func (h *host) commitOverlay(which overlay, f *fader, part string, rect, input image.Rectangle, paint func(*image.NRGBA, float64)) {
	if !(f.alpha > 0) || rect.Empty() || paint == nil {
		if f.shown {
			h.be.hideOverlay(which)
			h.led.Add(Entry{Ev: EvHide, Part: part, Output: h.out.ID})
		}
		f.shown, f.dirty, f.img, f.rect = false, false, nil, image.Rectangle{}
		return
	}
	if !f.dirty && f.shown && rect == f.rect {
		return
	}
	dpr := h.be.scale(h.out.ID)
	var img *image.NRGBA
	if f.dirty || !f.shown || rect.Size() != f.rect.Size() {
		phys := image.Pt(int(math.Ceil(float64(rect.Dx())*dpr)), int(math.Ceil(float64(rect.Dy())*dpr)))
		if f.img == nil || f.img.Bounds().Size() != phys {
			f.img = image.NewNRGBA(image.Rectangle{Max: phys})
		}
		paint(f.img, dpr)
		img = f.img
	}
	committed, err := h.be.presentOverlay(which, overlayPresentation{output: h.out.ID, rect: rect, img: img, input: input})
	if err != nil {
		f.dirty = false
		h.led.Add(Entry{Ev: EvCommit, Part: part, Reason: "error", Output: h.out.ID})
		return
	}
	if !committed {
		return
	}
	f.rect, f.shown = rect, true
	if img != nil {
		f.dirty = false
		h.led.Add(Entry{Ev: EvCommit, Part: part, Rect: rectOf(rect), Output: h.out.ID, Scale: dpr, Buttons: h.partButtons(which, rect)})
	}
}

// lostOverlays forgets surfaces the display took with the sprite.
func (h *host) lostOverlays() {
	for _, f := range []*fader{&h.hl, &h.chip, &h.card, &h.pill.f, &h.handoff.f, &h.notice.f} {
		f.dirty = f.dirty || f.shown
		f.shown, f.rect = false, image.Rectangle{}
	}
}

// --- overlay report ---

// overlayRects is what the presenter draws on its output while perched,
// while the guide card shows or while Hands runs, output-local logical px,
// for the daemon's capture masks: the sprite, the strand, the chip, the
// cards, the pill and the glow's strips there. The strand's rects merge
// down to the room the other parts leave in feature.MaxOverlayRects. It
// reuses the host's scratch, so a caller keeping the rects clones them.
func (h *host) overlayRects() []protocol.OniRect {
	if !h.haveOut {
		return nil
	}
	out := h.ovBuf[:0]
	add := func(r image.Rectangle) {
		if !r.Empty() {
			out = append(out, protocol.OniRect{X: float64(r.Min.X), Y: float64(r.Min.Y), W: float64(r.Dx()), H: float64(r.Dy())})
		}
	}
	glow := h.glowRects(h.out.ID)
	if h.shown && (h.perch != nil || h.card.shown || h.pill.f.shown || h.handoff.f.shown || h.notice.f.shown) {
		at := h.origin.Sub(h.out.Rect.Min)
		add(image.Rectangle{Min: at, Max: at.Add(h.extent())})
		parts := 0
		for _, f := range []*fader{&h.chip, &h.card, &h.pill.f, &h.handoff.f, &h.notice.f} {
			if f.shown && !f.rect.Empty() {
				parts++
			}
		}
		if h.strandShown {
			room := feature.MaxOverlayRects - len(out) - parts - len(glow)
			h.ovMerge = mergeRects(append(h.ovMerge[:0], h.strandMask.rects...), room)
			for _, r := range h.ovMerge {
				add(r)
			}
		}
		for _, f := range []*fader{&h.chip, &h.card, &h.pill.f, &h.handoff.f, &h.notice.f} {
			if f.shown {
				add(f.rect)
			}
		}
	}
	for _, r := range glow {
		add(r)
	}
	return out
}

// strandMaskPadPx is added round each stroke's painted half-width in a
// mask: the antialiased edge and the outward round to whole logical px.
const strandMaskPadPx = 2

// strandMaskMax is the strand's share of feature.MaxOverlayRects; the
// sprite, the chip and the card hold the rest.
const strandMaskMax = feature.MaxOverlayRects - 3

// strandMaskBreathPx is the room a held segment's envelope gains each time
// the breath carries it out, so a few steps take in its whole sway.
const strandMaskBreathPx = 4

// strandMask masks the strand in captures with at most limit rects that hug
// its drawn lines: each segment's round capsule, grown by the ink outline
// and strandMaskPadPx, gets a rect, then the pair of neighbours along the
// lines whose merge adds the least area merges until limit is met. The
// strand's bounding box would also cover the control it points at, which a
// settle watch must see change.
//
// A thread on the move gets fresh rects each frame, so one drawing back
// sheds its reach. A held thread still breathes with the character, so a
// segment's rect that leaves its envelope grows it by strandMaskBreathPx
// past the new rect, one for one, while the segments stay the same; once
// the envelope takes in the sway the mask, and so the report, holds still.
// The segments round the target stay put, so the room goes only where the
// thread sways.
type strandMask struct {
	rects []image.Rectangle
	// env is the segments' envelope, made for the held target held (0:
	// on the move); segs is scratch
	env, segs []image.Rectangle
	held      uint64
}

// cover sets the mask to r alone.
func (m *strandMask) cover(r image.Rectangle) {
	m.rects, m.env, m.held = append(m.rects[:0], r), m.env[:0], 0
}

// update masks frame f; shown says the mask before was f's strand's.
func (m *strandMask) update(f strand.Frame, outline float64, limit int, shown bool) {
	segs := m.segs[:0]
	for _, st := range f.Strokes {
		if !(st.Alpha > 0) || len(st.Pts) == 0 || !(st.Width > 0) {
			continue
		}
		r := st.Width/2 + outline + strandMaskPadPx
		for i, a := range st.Pts {
			b := a
			if i+1 < len(st.Pts) {
				b = st.Pts[i+1]
			} else if i > 0 {
				break
			}
			segs = append(segs, image.Rect(
				int(math.Floor(min(a.X, b.X)-r)), int(math.Floor(min(a.Y, b.Y)-r)),
				int(math.Ceil(max(a.X, b.X)+r)), int(math.Ceil(max(a.Y, b.Y)+r)),
			))
		}
	}
	held := f.Phase == strand.PhaseHold
	if held && shown && m.held == f.Seq && len(m.env) == len(segs) {
		grew := false
		for i, s := range segs {
			if !s.In(m.env[i]) {
				m.env[i], grew = m.env[i].Union(s.Inset(-strandMaskBreathPx)), true
			}
		}
		if !grew {
			m.segs = segs
			return
		}
		segs = append(segs[:0], m.env...)
	} else {
		m.env, m.held = m.env[:0], 0
		if held {
			m.env, m.held = append(m.env, segs...), f.Seq
		}
	}
	segs = mergeRects(segs, limit)
	m.rects, m.segs = append(m.rects[:0], segs...), segs
}

// mergeRects merges the pair of neighbours in segs whose merge adds the
// least area until at most limit are left (limit 0 or less: no bound), in
// place.
func mergeRects(segs []image.Rectangle, limit int) []image.Rectangle {
	area := func(r image.Rectangle) int { return r.Dx() * r.Dy() }
	for limit > 0 && len(segs) > limit {
		best, cost := 0, math.MaxInt
		for i := 0; i+1 < len(segs); i++ {
			if c := area(segs[i].Union(segs[i+1])) - area(segs[i]) - area(segs[i+1]); c < cost {
				best, cost = i, c
			}
		}
		segs[best] = segs[best].Union(segs[best+1])
		segs = slices.Delete(segs, best+1, best+2)
	}
	return segs
}

// reportOverlay tells the daemon the presenter's rects on each output when
// they changed, clearing an output it no longer draws on: one call in
// flight, the next one due after it.
func (h *host) reportOverlay() {
	l := h.link
	if l == nil {
		return
	}
	if h.ovBusy {
		h.ovPending = true
		return
	}
	out, rects, due := h.overlayDue()
	if !due {
		return
	}
	rects = slices.Clone(rects)
	if h.ovSent == nil {
		h.ovSent = map[string][]protocol.OniRect{}
	}
	if len(rects) == 0 {
		delete(h.ovSent, out)
	} else {
		h.ovSent[out] = rects
	}
	h.ovBusy = true
	e := Entry{Ev: EvOverlay, Output: out}
	for _, r := range rects {
		e.Rects = append(e.Rects, strand.Rect{X: r.X, Y: r.Y, W: r.W, H: r.H})
	}
	h.led.Add(e)
	go func() {
		ctx, cancel := context.WithTimeout(h.ctx, callTimeout)
		defer cancel()
		l.Overlay(ctx, out, rects)
		h.be.post(func() {
			if l != h.link {
				return
			}
			h.ovBusy, h.ovPending = false, false
			h.reportOverlay()
		})
	}()
}

// overlayDue is the first output whose rects differ from what the daemon
// was told: the presenter's own output, then the others the glow is on,
// then those told before and drawn on no more, each in name order.
func (h *host) overlayDue() (string, []protocol.OniRect, bool) {
	if h.haveOut {
		rects := h.overlayRects()
		if !slices.Equal(rects, h.ovSent[h.out.ID]) {
			return h.out.ID, rects, true
		}
	}
	ids := make([]string, 0, len(h.glow.outs)+len(h.ovSent))
	for id := range h.glow.outs {
		ids = append(ids, id)
	}
	for id := range h.ovSent {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range slices.Compact(ids) {
		if h.haveOut && id == h.out.ID {
			continue
		}
		var rects []protocol.OniRect
		for _, r := range h.glowRects(id) {
			rects = append(rects, protocol.OniRect{X: float64(r.Min.X), Y: float64(r.Min.Y), W: float64(r.Dx()), H: float64(r.Dy())})
		}
		if !slices.Equal(rects, h.ovSent[id]) {
			return id, rects, true
		}
	}
	return "", nil, false
}

// forgetOverlay drops what the daemon was told; a lost link took it.
func (h *host) forgetOverlay() {
	h.ovSent, h.ovBusy, h.ovPending = nil, false, false
}
