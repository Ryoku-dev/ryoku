package surface

import (
	"context"
	"image"
	"math"
	"slices"
	"time"

	i18n "ryoku-i18n"

	"ryoku-rashin/internal/oni/feature"
	"ryoku-rashin/internal/oni/render"
	"ryoku-rashin/internal/oni/spec"
	"ryoku-rashin/internal/oni/strand"
	"ryoku-rashin/internal/protocol"
)

// askCallTimeout bounds a call that may run a whole read of the window
// before it answers: a guide's next step or a pick's question.
const askCallTimeout = 3 * time.Minute

// Why the pick overlay's surfaces changed, on EvPick lines.
const (
	pickArmed   = "armed"
	pickPoint   = "point"
	pickCancel  = "cancel"
	pickDaemon  = "daemon"
	pickLost    = "lost"
	pickOutputs = "outputs"
)

// cardItemsMax is the most checklist rows the card shows: the step now and
// what is around it.
const cardItemsMax = 3

// guideHost is the host's half of a guide and of select-and-ask: the guide
// frame the card shows, the keep offer or hint its Skip lets go, the card
// overlay and its pointer, and the pick overlay's surfaces. Nothing here
// runs a timer or polls: the card changes only with a frame or a press, and
// the pick surfaces exist only while the daemon has armed them.
type guideHost struct {
	guide    *feature.GuideFrame
	guideSeq uint64
	// offer is the keep offer or hint the card's Skip answers
	offer *feature.ConsentFrame

	card     fader
	cardLay  render.Card
	cardAt   *spec.Point
	cardDown string // the pill the button went down on
	cardSent uint64 // the guide seq a pill press already answered

	pickSeq   uint64
	pickArmed bool
	pickOuts  []string
}

// --- guide frames ---

func (h *host) guideFrame(f *feature.GuideFrame) {
	if f.Seq <= h.guideSeq {
		return
	}
	h.guideSeq = f.Seq
	h.led.Add(Entry{Ev: EvGuide, Seq: f.Seq, Phase: f.Phase, Step: f.Step, Actions: f.Actions})
	switch f.Phase {
	case feature.GuideIdle, feature.GuideWinding:
		h.guide = nil
	default:
		g := *f
		g.Actions = slices.Clone(f.Actions)
		h.guide = &g
	}
	if f.Phase != feature.GuideKeeping && f.Phase != feature.GuideNudging {
		h.offer = nil
	}
	h.layoutCard()
}

// offerFrame keeps a keep offer or a hint for the card's Skip; other
// consent questions are the app's and the CLI's.
func (h *host) offerFrame(f *feature.ConsentFrame) {
	if f.Feature != feature.GuideKeep && f.Feature != feature.GuideHints {
		return
	}
	o := *f
	h.offer = &o
	h.layoutCard()
}

// offerAnswered drops the offer an answer settled.
func (h *host) offerAnswered(id string) {
	if h.offer == nil || h.offer.ID != id {
		return
	}
	h.offer = nil
	h.layoutCard()
}

// dropGuide forgets the guide and the pick here, as a lost link does: the
// daemon ends both when its presenter goes, and a daemon that comes back
// counts its guide and pick seqs from 1 again.
func (h *host) dropGuide(reason string) {
	h.disarmPick(reason)
	h.guideSeq, h.pickSeq, h.cardSent = 0, 0, 0
	if h.guide == nil && h.offer == nil {
		return
	}
	h.guide, h.offer = nil, nil
	h.layoutCard()
}

func (h *host) layoutCard() {
	h.cardLay = render.Card{}
	if g := h.guide; g != nil {
		h.cardLay = render.LayoutCard(h.cardContent(g), h.sp)
	}
	h.cardDown = ""
	h.card.dirty = true
	h.kick()
}

// cardContent is what the card says for g: its kicker (the step when the
// daemon named none), its line (the keep question while keeping), a hint
// on how to go on, the checklist and the pills. The presenter offers Next,
// Stop, Circle and Skip; Keep is the app's, the CLI's or a spoken yes, so it
// is never a pill.
func (h *host) cardContent(g *feature.GuideFrame) render.CardContent {
	c := render.CardContent{Kicker: g.Kicker, Text: g.Text, Items: planItems(g.Plan)}
	if c.Kicker == "" && g.Step > 0 {
		c.Kicker = i18n.Tf("Step %d", g.Step)
	}
	offered := func(a string) bool { return slices.Contains(g.Actions, a) }
	switch g.Phase {
	case feature.GuideReading:
		c.Hint = i18n.T("Reading the window…")
	case feature.GuidePointing, feature.GuideWaiting:
		if offered(feature.GuideActionNext) {
			c.Hint = i18n.T("Click it yourself, or press Next")
		}
	case feature.GuideKeeping:
		if h.offer != nil {
			c.Text = h.offer.Question
		}
		c.Hint = i18n.T("To keep it, say yes or use the app.")
	}
	if offered(feature.GuideActionCircle) {
		c.Pills = append(c.Pills, render.CardPill{Action: feature.GuideActionCircle, Text: i18n.T("Circle it")})
	}
	if offered(feature.GuideActionStop) {
		c.Pills = append(c.Pills, render.CardPill{Action: feature.GuideActionStop, Text: i18n.T("Stop")})
	}
	if offered(feature.GuideActionSkip) && h.offer != nil {
		c.Pills = append(c.Pills, render.CardPill{Action: feature.GuideActionSkip, Text: i18n.T("Skip")})
	}
	if offered(feature.GuideActionNext) {
		label := i18n.T("Next")
		if g.Phase == feature.GuideNudging {
			label = i18n.T("Show me")
		}
		c.Pills = append(c.Pills, render.CardPill{Action: feature.GuideActionNext, Text: label, Primary: true})
	}
	return c
}

// planItems is the checklist the card shows from p: the step now, then one
// step ahead, the latest step done, and more ahead while there is room, at
// most cardItemsMax rows in the order done, now, ahead.
func planItems(p *feature.GuidePlan) []render.CardItem {
	if p == nil {
		return nil
	}
	room := cardItemsMax
	if p.Current != "" {
		room--
	}
	next, done := 0, 0
	if room > 0 && len(p.Next) > 0 {
		next, room = 1, room-1
	}
	if room > 0 && len(p.Done) > 0 {
		done, room = 1, room-1
	}
	for room > 0 && next < len(p.Next) {
		next, room = next+1, room-1
	}
	for room > 0 && done < len(p.Done) {
		done, room = done+1, room-1
	}
	var items []render.CardItem
	for _, s := range p.Done[len(p.Done)-done:] {
		items = append(items, render.CardItem{Mark: render.CardDone, Text: s})
	}
	if p.Current != "" {
		items = append(items, render.CardItem{Mark: render.CardNow, Text: p.Current})
	}
	for _, s := range p.Next[:next] {
		items = append(items, render.CardItem{Mark: render.CardNext, Text: s})
	}
	return items
}

// --- card overlay ---

// cardWanted: a guide has something to show and the character is on
// screen to show it beside.
func (h *host) cardWanted() bool {
	return h.guide != nil && !h.cardLay.Empty() && h.shown && h.haveOut
}

// cardGuard is the box the card must keep clear of, output-local: the
// strand's target (for a ring or a pen, the ring round it), if one is out.
func (h *host) cardGuard() (strand.Rect, bool) {
	t := h.target
	if t == nil || t.Mode == strand.ModePerch {
		return strand.Rect{}, false
	}
	box := t.Box
	if t.Mode != strand.ModePoint {
		g := h.sp.Strand.Lasso.GapPx
		box = strand.Rect{X: box.X - g, Y: box.Y - g, W: box.W + 2*g, H: box.H + 2*g}
	}
	return box, true
}

// cardRect is where the card goes, output-local. With a target out it sits
// beside the target as the bubble does (strand.BubbleRect), by the landed
// tip or, before the thread lands, on the side facing the character, so it
// never covers the target. With none it sits by the character like the
// chip: under it (under the chip when the chip shows), or over it when
// under would cross the seat margin, kept that margin inside the output.
func (h *host) cardRect() image.Rectangle {
	if h.cardLay.Empty() || !h.haveOut || h.char == nil {
		return image.Rectangle{}
	}
	size := image.Pt(int(math.Ceil(h.cardLay.Size.X)), int(math.Ceil(h.cardLay.Size.Y)))
	at := h.origin.Sub(h.out.Rect.Min)
	ext := h.extent()
	w, ht := h.out.Rect.Dx(), h.out.Rect.Dy()
	if box, ok := h.cardGuard(); ok {
		tip := spec.Point{X: float64(at.X) + float64(ext.X)/2, Y: float64(at.Y) + float64(ext.Y)/2}
		var approach spec.Point
		if h.landed == h.target.Seq {
			tip, approach = h.strandF.Tip, h.strandF.Approach()
		}
		r := strand.BubbleRect(box, tip, approach, spec.Point{X: float64(size.X), Y: float64(size.Y)}, strand.Rect{W: float64(w), H: float64(ht)}, h.sp)
		tl := image.Pt(int(math.Round(r.X)), int(math.Round(r.Y)))
		return image.Rectangle{Min: tl, Max: tl.Add(size)}
	}
	gap := int(math.Round(h.sp.Perch.Chip.GapPx))
	m := int(math.Round(h.sp.Perch.Seat.MarginPx))
	below, above := at.Y+ext.Y+gap, at.Y-gap-size.Y
	if h.chip.want {
		if cr := h.chipRect(); !cr.Empty() {
			if cr.Min.Y >= at.Y+ext.Y {
				below = cr.Max.Y + gap
			} else {
				above = cr.Min.Y - gap - size.Y
			}
		}
	}
	y := below
	if y+size.Y > ht-m {
		y = above
	}
	x := at.X + ext.X/2 - size.X/2
	x = max(m, min(x, w-m-size.X))
	y = max(m, min(y, ht-m-size.Y))
	return image.Rectangle{Min: image.Pt(x, y), Max: image.Pt(x, y).Add(size)}
}

// commitCard puts the card where it belongs, with new pixels only when they
// changed; it takes the pointer over its whole face.
func (h *host) commitCard() {
	r := h.cardRect()
	h.commitOverlay(overlayCard, &h.card, PartCard, r, image.Rectangle{Max: r.Size()}, func(img *image.NRGBA, dpr float64) {
		render.PaintCard(img, h.cardLay, h.sp, h.look, dpr, h.card.alpha, h.cardDown)
	})
}

// hideCard takes the card's surface down at once, for a hide.
func (h *host) hideCard() {
	h.card.alpha = 0
	h.commitOverlay(overlayCard, &h.card, PartCard, image.Rectangle{}, image.Rectangle{}, nil)
	h.cardAt, h.cardDown = nil, ""
}

func (h *host) cardPointer(p *spec.Point) {
	h.cardAt = p
	if p == nil && h.cardDown != "" {
		h.cardDown = ""
		h.card.dirty = true
		h.kick()
	}
}

// cardButton presses a pill on the way down and sends it on the way up over
// the same pill; letting go anywhere else sends nothing.
func (h *host) cardButton(pressed bool) {
	on := ""
	if h.cardAt != nil && h.card.shown {
		on = h.cardLay.PillAt(*h.cardAt)
	}
	if pressed {
		if on != h.cardDown {
			h.cardDown = on
			h.card.dirty = true
			h.kick()
		}
		return
	}
	down := h.cardDown
	if down == "" {
		return
	}
	h.cardDown = ""
	h.card.dirty = true
	h.kick()
	if down == on {
		h.cardPress(down)
	}
}

// cardPress sends the one call a pill makes: oni.guide next, stop or
// circle, or oni.skip with the offer's id. One press per guide frame: the
// next frame brings the pills back, and a refused call frees them again.
func (h *host) cardPress(action string) {
	g, l := h.guide, h.link
	if g == nil || l == nil || h.cardSent == g.Seq || !slices.Contains(g.Actions, action) {
		return
	}
	var send func(ctx context.Context) error
	switch action {
	case feature.GuideActionNext, feature.GuideActionStop, feature.GuideActionCircle:
		send = func(ctx context.Context) error { return l.Guide(ctx, action) }
	case feature.GuideActionSkip:
		o := h.offer
		if o == nil {
			return
		}
		id := o.ID
		send = func(ctx context.Context) error { return l.Skip(ctx, id) }
	default:
		return
	}
	seq := g.Seq
	h.cardSent = seq
	h.led.Add(Entry{Ev: EvCardPress, Seq: seq, Reason: action})
	go func() {
		ctx, cancel := context.WithTimeout(h.ctx, askCallTimeout)
		defer cancel()
		if err := send(ctx); err != nil {
			h.be.post(func() {
				if h.cardSent == seq {
					h.cardSent = 0
				}
			})
		}
	}()
}

// --- pick overlay ---

func (h *host) pickFrame(f *feature.PickFrame) {
	if f.Seq <= h.pickSeq {
		return
	}
	h.pickSeq = f.Seq
	if f.Armed {
		h.pickArmed = true
		h.syncPick(pickArmed)
		return
	}
	h.disarmPick(pickDaemon)
}

// syncPick maps a pick surface on every output while armed and none
// otherwise, telling the display only when the set changes.
func (h *host) syncPick(reason string) {
	var outs []string
	if h.pickArmed && h.be != nil {
		for _, o := range h.be.outputs() {
			outs = append(outs, o.ID)
		}
	}
	if slices.Equal(outs, h.pickOuts) {
		return
	}
	h.pickOuts = outs
	h.be.setPick(outs)
	h.led.Add(Entry{Ev: EvPick, Reason: reason, Outputs: outs})
}

func (h *host) disarmPick(reason string) {
	h.pickArmed = false
	h.syncPick(reason)
}

// pickClick sends the one click oni.pick waits for and takes the overlay
// down at once; the spot goes to the daemon, never to the ledger.
func (h *host) pickClick(output string, p spec.Point) {
	if !h.pickArmed || !slices.Contains(h.pickOuts, output) {
		return
	}
	h.disarmPick(pickPoint)
	l := h.link
	if l == nil {
		return
	}
	params := protocol.OniPickParams{Action: protocol.OniPickPoint, Output: output, X: max(0, p.X), Y: max(0, p.Y)}
	go func() {
		ctx, cancel := context.WithTimeout(h.ctx, askCallTimeout)
		defer cancel()
		l.Pick(ctx, params)
	}()
}

// pickCancel takes the overlay down and lets the daemon's pick go: a layer
// surface never hears Esc, so a right click is the way out.
func (h *host) pickCancel() {
	if !h.pickArmed {
		return
	}
	h.disarmPick(pickCancel)
	if l := h.link; l != nil {
		h.call(func(ctx context.Context) error {
			return l.Pick(ctx, protocol.OniPickParams{Action: protocol.OniPickCancel})
		})
	}
}
