package surface

import (
	"context"
	"image"
	"math"
	"slices"
	"sort"
	"time"

	i18n "ryoku-i18n"

	"ryoku-rashin/internal/oni/feature"
	"ryoku-rashin/internal/oni/placement"
	"ryoku-rashin/internal/oni/render"
	"ryoku-rashin/internal/oni/spec"
	"ryoku-rashin/internal/protocol"
)

// What Hands' pill and handoff card offer. Stop and Not now end the task;
// Resume goes on from a pause. There is no yes: a handed-off control is
// the person's to press, never Hands'.
const (
	handsStop     = protocol.OniHandsStop
	handsResume   = protocol.OniHandsResume
	handsNotNow   = "not-now"
	noticeDismiss = "dismiss-notice"
)

// handsInactive is why the Hands parts came down on hide lines: the frame
// that ended the task. handsApart is why the glow did: the task works on
// Hyakki's separate desktop, not on the person's screens.
const (
	handsInactive = "inactive"
	handsApart    = "apart"
)

// handsFeature says whether a card is one of Hands' handoffs: from the
// browser tab, a live window or Hyakki's separate desktop.
func handsFeature(id string) bool {
	return id == feature.HandsBrowser || id == feature.HandsApp || id == feature.HandsDesktop
}

// handsHost is the host's half of Hands: the running task's frame, the
// handoff card it waits on, the pill and the card beside the character,
// and the glow round every output. Nothing here runs while no task runs:
// no surface, no buffer, no timer. While one runs the glow breathes on a
// timer at the spec's fps, and only while it works or waits unreduced.
type handsHost struct {
	hands     *feature.HandsFrame
	handsCard *feature.ConsentFrame
	// handsBegan is when the running task's first frame came
	handsBegan time.Time

	pill    handsPart
	handoff handsPart
	notice  handsPart

	// approvals are the approval cards waiting, oldest first; the first
	// shows in the card's place while no handoff card does, and
	// handoffApproval is the id of the one it shows, "" for none
	approvals       []feature.ApprovalFrame
	handoffApproval string

	glow glowHost
}

// handsPart is the pill or the handoff card: its fader and layout, the
// pointer over it, the action the button went down on and whether a press
// was sent for the layout on screen.
type handsPart struct {
	f    fader
	lay  render.Card
	at   *spec.Point
	down string
	sent bool
}

func (h *host) noticeFrame(f *feature.NoticeFrame) {
	if !h.shown || !h.haveOut {
		return
	}
	h.notice.lay = render.LayoutCard(render.CardContent{
		Kicker: i18n.T("Notice"), Text: f.Text,
		Pills: []render.CardPill{{Text: i18n.T("Dismiss"), Action: noticeDismiss}},
	}, h.sp)
	h.notice.at, h.notice.down, h.notice.sent = nil, "", false
	h.notice.f.dirty = true
	if h.clk.idle() {
		h.clk.halt()
	}
	h.kick()
}

func (h *host) clearNotice(reason string) {
	if h.notice.f.shown {
		h.be.hideOverlay(overlayNotice)
		h.led.Add(Entry{Ev: EvHide, Part: PartPill, Reason: reason, Output: h.out.ID})
	}
	h.notice = handsPart{}
}

// glowHost is the glow on every output: its strengths, buffers and what
// each output's surfaces show, the level they were painted at, when the
// breath started and its pacing timer.
type glowHost struct {
	outs    map[string]*glowOut
	level   float64
	start   time.Time
	timer   *time.Timer
	gen     int
	ticking bool
}

type glowOut struct {
	eg    *render.EdgeGlow
	imgs  [render.GlowStrips]*image.NRGBA
	level float64
	// shown: the surfaces hold pixels at level; dirty: they owe new ones
	shown, dirty bool
}

// --- frames ---

// handsFrame takes Hands' frame: an active one shows or updates the pill,
// the card and the glow; the inactive one takes every part down at once,
// in the same call, with no fade.
func (h *host) handsFrame(f *feature.HandsFrame) {
	e := Entry{Ev: EvHands, Phase: f.Phase, Step: f.Step, Reason: f.Pause, Scope: f.Scope, Output: f.Output}
	if !f.Active {
		e.Reason, e.End = f.Reason, f.End
	}
	h.led.Add(e)
	if !f.Active {
		h.endHands(handsInactive, f.End)
		return
	}
	started := h.hands == nil
	g := *f
	h.hands = &g
	if f.Phase != protocol.OniHandsStateWaiting {
		h.handsCard = nil
	}
	if started {
		h.handsBegan = time.Now()
		h.glow.start = h.handsBegan
		h.cue("hands-start")
	}
	h.layoutHands()
	h.syncGlow(time.Now())
}

// handoffFrame keeps the handoff card the waiting task shows.
func (h *host) handoffFrame(f *feature.ConsentFrame) {
	if h.hands == nil || h.hands.Phase != protocol.OniHandsStateWaiting {
		return
	}
	c := *f
	h.handsCard = &c
	h.layoutHands()
	h.syncGlow(time.Now())
}

// handoffAnswered drops the card an answer settled.
func (h *host) handoffAnswered(id string) {
	if h.handsCard == nil || h.handsCard.ID != id {
		return
	}
	h.handsCard = nil
	h.layoutHands()
}

// endHands takes the pill, the card and the glow down at once and forgets
// the task; the daemon ends it when its presenter goes, so a lost link does
// the same. end is how the task ended (protocol.OniHandsEnds), "" when
// unknown: a task done after at least the spec's handsDoneMinS sounds
// done-notes, any other end hands-stop. An approval card showing in the
// card's place is not the task's and stays; one waiting behind the task's
// handoff card comes into its place.
func (h *host) endHands(reason, end string) {
	was := h.hands != nil
	h.hands, h.handsCard = nil, nil
	type slot struct {
		part  *handsPart
		which overlay
		name  string
	}
	parts := []slot{{&h.pill, overlayPill, PartPill}}
	if h.handoffApproval == "" {
		parts = append(parts, slot{&h.handoff, overlayHandoff, PartHandoff})
	}
	for _, p := range parts {
		p.part.lay, p.part.at, p.part.down, p.part.sent = render.Card{}, nil, "", false
		p.part.f.want, p.part.f.alpha = false, 0
		if p.part.f.shown {
			h.be.hideOverlay(p.which)
			h.led.Add(Entry{Ev: EvHide, Part: p.name, Reason: reason, Output: h.out.ID})
		}
		p.part.f.shown, p.part.f.dirty, p.part.f.img, p.part.f.rect = false, false, nil, image.Rectangle{}
	}
	h.dropGlow(reason)
	if was {
		took := time.Since(h.handsBegan).Seconds() * h.timeScale()
		if end == protocol.OniHandsEndDone && took >= h.sp.Sound.Cues.HandsDoneMinS {
			h.cue("done-notes")
		} else {
			h.cue("hands-stop")
		}
	}
	if len(h.approvals) > 0 {
		h.layoutHands()
	}
	h.reportOverlay()
}

// cue plays one of the spec's cues through the sound gate and records it.
func (h *host) cue(id string) {
	if h.player == nil {
		return
	}
	played, err := h.player.PlayCue(time.Now(), id)
	reason := soundPlayed
	switch {
	case err != nil:
		reason = soundError
	case played == "":
		reason = soundGated
	}
	h.led.Add(Entry{Ev: EvSound, Event: id, Motif: played, Reason: reason})
}

// --- pill and card ---

// pauseLine is what the pill says for a pause in scope: Hyakki's own
// words, naming the rule, never the page or the window's text. A key the
// person pressed is only ever "a key": the reader never says which.
func pauseLine(scope, pause string) string {
	tab := scope == "" || scope == protocol.OniHandsScopeBrowser
	switch pause {
	case protocol.OniHandsPauseFocus:
		return i18n.T("Paused: another window took focus")
	case protocol.OniHandsPauseWindow:
		return i18n.T("Paused: the window can't be told apart")
	case protocol.OniHandsPauseNavigation:
		return i18n.T("Paused: the tab left the page")
	case protocol.OniHandsPauseHidden:
		if tab {
			return i18n.T("Paused: the tab is out of view")
		}
		return i18n.T("Paused: the window is out of view")
	case protocol.OniHandsPauseCovered:
		return i18n.T("Paused: Hands can't be seen")
	case protocol.OniHandsPauseDialog:
		return i18n.T("Paused: the page opened a dialog")
	case protocol.OniHandsPausePopup:
		return i18n.T("Paused: the page opened a popup")
	case protocol.OniHandsPauseDownload:
		return i18n.T("Paused: the page started a download")
	case protocol.OniHandsPauseOutput:
		if scope == protocol.OniHandsScopeWindow {
			return i18n.T("Paused: the window went to another screen")
		}
		return i18n.T("Paused: your screens changed")
	case protocol.OniHandsPauseTitle:
		return i18n.T("Paused: the window's title changed")
	case protocol.OniHandsPauseGeometry:
		return i18n.T("Paused: the window moved or changed size")
	case protocol.OniHandsPauseSelf:
		if scope == protocol.OniHandsScopeWindow {
			return i18n.T("Paused: Hyakki's own window took focus")
		}
		return i18n.T("Paused: you went to Hyakki's window")
	case protocol.OniHandsPausePrivate:
		return i18n.T("Paused: a private window is in front")
	case protocol.OniHandsPauseKey:
		return i18n.T("Paused: you pressed a key")
	case protocol.OniHandsPausePresenter:
		return i18n.T("Paused: Oni was not on screen")
	}
	return i18n.T("Hands is paused")
}

// scopeLine is the pill's line over its row: where Hands works. The tab
// says nothing, as it always has; a live window is named by its app's id,
// never its title; Hyakki's separate desktop is said to be apart from
// the person's, since the glow does not go round theirs.
func scopeLine(f *feature.HandsFrame) string {
	switch f.Scope {
	case protocol.OniHandsScopeWindow:
		if f.App != "" {
			return i18n.Tf("In %s", f.App)
		}
		return i18n.T("In the window you picked")
	case protocol.OniHandsScopeNested, protocol.OniHandsScopeDesktop:
		return i18n.T("On Hyakki's separate desktop")
	}
	return ""
}

// pillContent is what the pill says and offers for f: where it works, its
// state, Stop always, Resume only while paused.
func pillContent(f *feature.HandsFrame) (string, string, []render.CardPill) {
	scope := scopeLine(f)
	stop := render.CardPill{Action: handsStop, Text: i18n.T("Stop")}
	switch f.Phase {
	case protocol.OniHandsStateWaiting:
		return scope, i18n.T("Hands is waiting for you"), []render.CardPill{stop}
	case protocol.OniHandsStatePaused:
		return scope, pauseLine(f.Scope, f.Pause), []render.CardPill{stop, {Action: handsResume, Text: i18n.T("Resume"), Primary: true}}
	}
	if f.Step > 0 {
		return scope, i18n.Tf("Hands is working, step %d of %d", f.Step, f.Max), []render.CardPill{stop}
	}
	return scope, i18n.T("Hands is starting"), []render.CardPill{stop}
}

// handoffContent is the card for c: the control it names in the daemon's
// line, how to go on in the scope the task works in, and Not now. It never
// offers a yes.
func handoffContent(c *feature.ConsentFrame, scope string) render.CardContent {
	hint := i18n.T("Press it yourself. Hands never presses it for you.")
	if scope == protocol.OniHandsScopeNested || scope == protocol.OniHandsScopeDesktop {
		hint = i18n.T("Hands never presses it there. Stop to skip it.")
	}
	return render.CardContent{
		Kicker: i18n.T("Your turn"),
		Text:   c.Question,
		Hint:   hint,
		Pills:  []render.CardPill{{Action: handsNotNow, Text: i18n.T("Not now")}},
	}
}

func (h *host) layoutHands() {
	pill, card := render.Card{}, render.Card{}
	h.handoffApproval = ""
	if f := h.hands; f != nil {
		scope, text, pills := pillContent(f)
		pill = render.LayoutPill(scope, text, pills, h.sp)
		if c := h.handsCard; c != nil {
			card = render.LayoutCard(handoffContent(c, f.Scope), h.sp)
		}
	}
	if card.Empty() && len(h.approvals) > 0 {
		a := h.approvals[0]
		card = render.LayoutCard(approvalContent(&a), h.sp)
		h.handoffApproval = a.ID
	}
	for _, p := range []struct {
		part *handsPart
		lay  render.Card
	}{{&h.pill, pill}, {&h.handoff, card}} {
		if !sameCard(p.part.lay, p.lay) {
			p.part.sent = false
		}
		p.part.lay, p.part.down = p.lay, ""
		p.part.f.dirty = true
	}
	h.kick()
}

// sameCard: the same words and actions in the same places.
func sameCard(a, b render.Card) bool {
	if a.Size != b.Size || len(a.Lines) != len(b.Lines) || len(a.Pills) != len(b.Pills) || a.Kicker.Text != b.Kicker.Text {
		return false
	}
	for i := range a.Lines {
		if a.Lines[i].Text != b.Lines[i].Text {
			return false
		}
	}
	for i := range a.Pills {
		if a.Pills[i].Action != b.Pills[i].Action || a.Pills[i].Rect != b.Pills[i].Rect {
			return false
		}
	}
	return true
}

// handsWanted: the part has something to show and the character is on
// screen to show it beside.
func (h *host) handsWanted(p *handsPart) bool {
	return !p.lay.Empty() && h.shown && h.haveOut
}

// handsRects stacks the task pill, approval card and independent notice
// beside the character, inside the output's seat margin.
func (h *host) handsRects() (pill, card, notice image.Rectangle) {
	if !h.haveOut || h.char == nil {
		return image.Rectangle{}, image.Rectangle{}, image.Rectangle{}
	}
	size := func(k render.Card) image.Point {
		if k.Empty() {
			return image.Point{}
		}
		return image.Pt(int(math.Ceil(k.Size.X)), int(math.Ceil(k.Size.Y)))
	}
	sizes := [3]image.Point{size(h.pill.lay), size(h.handoff.lay), size(h.notice.lay)}
	at := h.origin.Sub(h.out.Rect.Min)
	ext := h.extent()
	gap := int(math.Round(h.sp.Perch.Chip.GapPx))
	m := int(math.Round(h.sp.Perch.Seat.MarginPx))
	w, ht := h.out.Rect.Dx(), h.out.Rect.Dy()
	below, above := at.Y+ext.Y+gap, at.Y-gap
	var obstacles [2]image.Rectangle
	if h.chip.want {
		obstacles[0] = h.chipRect()
	}
	if h.card.want {
		obstacles[1] = h.cardRect()
	}
	for _, obstacle := range obstacles {
		if !obstacle.Empty() {
			below = max(below, obstacle.Max.Y+gap)
			above = min(above, obstacle.Min.Y-gap)
		}
	}
	stack := 0
	for _, sz := range sizes {
		if sz.Y > 0 {
			if stack > 0 {
				stack += gap
			}
			stack += sz.Y
		}
	}
	place := func(sz image.Point, y int) image.Rectangle {
		if sz == (image.Point{}) {
			return image.Rectangle{}
		}
		x := at.X + ext.X/2 - sz.X/2
		x = max(m, min(x, w-m-sz.X))
		y = max(m, min(y, ht-m-sz.Y))
		return image.Rectangle{Min: image.Pt(x, y), Max: image.Pt(x, y).Add(sz)}
	}
	down := below+stack <= ht-m
	y := below
	if !down {
		y = above
	}
	var rects [3]image.Rectangle
	for i, sz := range sizes {
		if sz.Y == 0 {
			continue
		}
		if !down {
			y -= sz.Y
		}
		rects[i] = place(sz, y)
		if down {
			y += sz.Y + gap
		} else {
			y -= gap
		}
	}
	return rects[0], rects[1], rects[2]
}

// commitHands puts the pill and the card where they belong, with new
// pixels only when they changed; each takes the pointer over its face.
func (h *host) commitHands() {
	pr, cr, nr := h.handsRects()
	h.commitOverlay(overlayPill, &h.pill.f, PartPill, pr, image.Rectangle{Max: pr.Size()}, func(img *image.NRGBA, dpr float64) {
		render.PaintCard(img, h.pill.lay, h.sp, h.look, dpr, h.pill.f.alpha, h.pill.down)
	})
	h.commitOverlay(overlayHandoff, &h.handoff.f, h.handoffName(), cr, image.Rectangle{Max: cr.Size()}, func(img *image.NRGBA, dpr float64) {
		render.PaintCard(img, h.handoff.lay, h.sp, h.look, dpr, h.handoff.f.alpha, h.handoff.down)
	})
	h.commitOverlay(overlayNotice, &h.notice.f, PartPill, nr, image.Rectangle{Max: nr.Size()}, func(img *image.NRGBA, dpr float64) {
		render.PaintCard(img, h.notice.lay, h.sp, h.look, dpr, h.notice.f.alpha, h.notice.down)
	})
}

// hideHands takes the pill and the card's surfaces down at once, for a
// hide; the glow is the task's and stays.
func (h *host) hideHands() {
	for _, p := range []struct {
		part  *handsPart
		which overlay
		name  string
	}{{&h.pill, overlayPill, PartPill}, {&h.handoff, overlayHandoff, h.handoffName()}, {&h.notice, overlayNotice, PartPill}} {
		p.part.f.alpha = 0
		h.commitOverlay(p.which, &p.part.f, p.name, image.Rectangle{}, image.Rectangle{}, nil)
		p.part.at, p.part.down = nil, ""
	}
}

// partButtons are the actions of the pill or the card on a commit line,
// output-local logical px, so the lab can press them.
func (h *host) partButtons(which overlay, rect image.Rectangle) []ButtonInfo {
	var lay render.Card
	switch which {
	case overlayPill:
		lay = h.pill.lay
	case overlayHandoff:
		lay = h.handoff.lay
	case overlayNotice:
		lay = h.notice.lay
	default:
		return nil
	}
	out := make([]ButtonInfo, 0, len(lay.Pills))
	for _, p := range lay.Pills {
		out = append(out, ButtonInfo{Action: p.Action, X: float64(rect.Min.X) + p.Rect.X, Y: float64(rect.Min.Y) + p.Rect.Y, W: p.Rect.W, H: p.Rect.H})
	}
	return out
}

func (h *host) handsPart(which overlay) *handsPart {
	switch which {
	case overlayPill:
		return &h.pill
	case overlayHandoff:
		return &h.handoff
	case overlayNotice:
		return &h.notice
	}
	return nil
}

func (h *host) handsPointer(which overlay, p *spec.Point) {
	part := h.handsPart(which)
	if part == nil {
		return
	}
	part.at = p
	if p == nil && part.down != "" {
		part.down = ""
		part.f.dirty = true
		h.kick()
	}
}

// handsButton presses an action on the way down and sends it on the way up
// over the same action; letting go anywhere else sends nothing.
func (h *host) handsButton(which overlay, pressed bool) {
	part := h.handsPart(which)
	if part == nil {
		return
	}
	on := ""
	if part.at != nil && part.f.shown {
		on = part.lay.PillAt(*part.at)
	}
	if pressed {
		if on != part.down {
			part.down = on
			part.f.dirty = true
			h.kick()
		}
		return
	}
	down := part.down
	if down == "" {
		return
	}
	part.down = ""
	part.f.dirty = true
	h.kick()
	if down == on {
		h.handsPress(part, down)
	}
}

// handsPress sends the one call an action makes: oni.hands stop for Stop
// and Not now, resume for Resume while paused. One press per layout: the
// next frame brings the actions back, and a refused call frees them again.
func (h *host) handsPress(part *handsPart, action string) {
	if part == &h.notice && action == noticeDismiss {
		h.clearNotice("dismissed")
		h.kick()
		h.reportOverlay()
		return
	}
	if part == &h.handoff && h.handoffApproval != "" {
		h.approvalPress(part, action)
		return
	}
	f, l := h.hands, h.link
	if f == nil || l == nil || part.sent {
		return
	}
	call := handsStop
	switch action {
	case handsStop, handsNotNow:
	case handsResume:
		if f.Phase != protocol.OniHandsStatePaused {
			return
		}
		call = handsResume
	default:
		return
	}
	part.sent = true
	h.led.Add(Entry{Ev: EvHandsPress, Reason: action})
	go func() {
		ctx, cancel := context.WithTimeout(h.ctx, callTimeout)
		defer cancel()
		if err := l.Hands(ctx, call); err != nil {
			h.be.post(func() { part.sent = false })
		}
	}()
}

// --- glow ---

// glowLevel is the glow's strength at now and whether it is breathing:
// between the spec's low and high over its period while the task works,
// scaled by the waiting factor while a card waits, still at high under
// reduced motion and at low while paused.
func (h *host) glowLevel(now time.Time) (float64, bool) {
	g := h.sp.Hands.Glow
	f := h.hands
	if f == nil {
		return 0, false
	}
	if f.Phase == protocol.OniHandsStatePaused {
		return g.Low, false
	}
	level, breathing := g.High, false
	if !h.reduced() {
		t := now.Sub(h.glow.start).Seconds() * h.timeScale()
		level = g.Low + (g.High-g.Low)*(0.5-0.5*math.Cos(2*math.Pi*t/g.PeriodS))
		breathing = true
	}
	if f.Phase == protocol.OniHandsStateWaiting {
		level *= g.WaitingFactor
	}
	return level, breathing
}

func (h *host) timeScale() float64 {
	if s := h.lab.TimeScale; s > 0 {
		return s
	}
	return 1
}

// syncGlow shows the glow where the task acts while it runs (glowOutputs),
// painted anew when its strength, the outputs or the look changed, and
// keeps the breath paced while it breathes; with no task, or one on
// Hyakki's separate desktop, it takes the glow down and arms nothing.
func (h *host) syncGlow(now time.Time) {
	if h.hands == nil || h.be == nil {
		h.dropGlow(handsInactive)
		return
	}
	outs := h.glowOutputs()
	if len(outs) == 0 {
		h.dropGlow(handsApart)
		h.reportOverlay()
		return
	}
	level, breathing := h.glowLevel(now)
	gl := &h.glow
	if gl.outs == nil {
		gl.outs = map[string]*glowOut{}
	}
	keep := make([]string, 0, len(outs))
	px := h.sp.Hands.Glow.StripPx
	for _, o := range outs {
		keep = append(keep, o.ID)
		scale := h.be.glowScale(o.ID)
		g := gl.outs[o.ID]
		if g == nil {
			g = &glowOut{}
			gl.outs[o.ID] = g
		}
		if size := o.Rect.Size(); !g.eg.Fits(size, px, h.look.Design, scale) {
			g.eg = render.NewEdgeGlow(size, px, h.look.Design, scale)
			for i := range g.imgs {
				g.imgs[i] = nil
				if sz := g.eg.DeviceSize(i); sz.X > 0 && sz.Y > 0 {
					g.imgs[i] = image.NewNRGBA(image.Rectangle{Max: sz})
				}
			}
			g.dirty = true
		}
		if !g.dirty && g.shown && g.level == level {
			continue
		}
		var p glowPresentation
		p.output, p.rects = o.ID, g.eg.Rects
		for i, img := range g.imgs {
			if img != nil {
				g.eg.Paint(i, img, h.look, level)
				p.imgs[i] = img
			}
		}
		committed, err := h.be.presentGlow(p)
		if err != nil {
			g.dirty = false
			h.led.Add(Entry{Ev: EvCommit, Part: PartGlow, Reason: "error", Output: o.ID})
			continue
		}
		if !committed {
			g.dirty = true
			continue
		}
		g.shown, g.dirty, g.level = true, false, level
		e := Entry{Ev: EvCommit, Part: PartGlow, Output: o.ID, Scale: scale, FPS: h.sp.Hands.Glow.FPS, Level: new(level)}
		for _, r := range p.rects {
			if !r.Empty() {
				e.Rects = append(e.Rects, rectInfo(r))
			}
		}
		h.led.Add(e)
	}
	for id := range gl.outs {
		if !slices.Contains(keep, id) {
			delete(gl.outs, id)
		}
	}
	h.be.hideGlow(keep)
	gl.level = level
	h.paceGlow(breathing)
	h.reportOverlay()
}

// glowOutputs are the outputs the glow goes round: every one for the
// browser tab; for a live window, the output it is on, or every one while
// that output is not known here; none for Hyakki's separate desktop,
// which is not on the person's screens at all, so no glow there suggests
// their desktop is driven.
func (h *host) glowOutputs() []placement.Output {
	outs := h.be.outputs()
	switch h.hands.Scope {
	case protocol.OniHandsScopeNested, protocol.OniHandsScopeDesktop:
		return nil
	case protocol.OniHandsScopeWindow:
		for _, o := range outs {
			if o.ID == h.hands.Output {
				return []placement.Output{o}
			}
		}
	}
	return outs
}

// paceGlow arms the next breath one spec frame away while breathing, and
// stops the timer otherwise.
func (h *host) paceGlow(breathing bool) {
	gl := &h.glow
	gl.gen++
	if gl.timer != nil {
		gl.timer.Stop()
		gl.timer = nil
	}
	gl.ticking = breathing
	if !breathing {
		return
	}
	gen := gl.gen
	d := time.Duration(float64(h.sp.Hands.Glow.Frame()) / h.timeScale())
	gl.timer = time.AfterFunc(d, func() {
		h.be.post(func() {
			if gen == h.glow.gen {
				h.glow.timer = nil
				h.syncGlow(time.Now())
			}
		})
	})
}

// dropGlow destroys every glow surface and buffer and stops the breath.
func (h *host) dropGlow(reason string) {
	gl := &h.glow
	gl.gen++
	if gl.timer != nil {
		gl.timer.Stop()
		gl.timer = nil
	}
	gl.ticking = false
	if len(gl.outs) == 0 {
		return
	}
	ids := make([]string, 0, len(gl.outs))
	for id, g := range gl.outs {
		if g.shown {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	h.be.hideGlow(nil)
	for _, id := range ids {
		h.led.Add(Entry{Ev: EvHide, Part: PartGlow, Reason: reason, Output: id})
	}
	gl.outs = nil
}

// glowConfigured paints the glow on an output whose surfaces were just
// configured.
func (h *host) glowConfigured(output string) {
	if g := h.glow.outs[output]; g != nil {
		g.dirty = true
	}
	h.syncGlow(time.Now())
}

// glowRects are the glow's strips on output while they show, for the
// capture masks.
func (h *host) glowRects(output string) []image.Rectangle {
	g := h.glow.outs[output]
	if g == nil || !g.shown || g.eg == nil {
		return nil
	}
	var out []image.Rectangle
	for _, r := range g.eg.Rects {
		if !r.Empty() {
			out = append(out, r)
		}
	}
	return out
}
