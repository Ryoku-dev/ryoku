package surface

import (
	"bytes"
	"encoding/json"
	"image"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/design"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"

	"ryoku-rashin/internal/oni/feature"
	"ryoku-rashin/internal/oni/placement"
	"ryoku-rashin/internal/oni/render"
	"ryoku-rashin/internal/oni/render/desktoplook"
	"ryoku-rashin/internal/oni/sound"
	"ryoku-rashin/internal/oni/spec"
	"ryoku-rashin/internal/protocol"
)

const handoffLine = "Your turn. Press Send when it looks right."

func handsFrameAt(phase string, step int) *feature.HandsFrame {
	f := &feature.HandsFrame{Stage: protocol.OniHandsBrowser, Active: true, Reason: feature.ReasonAllowed, Scope: protocol.OniHandsScopeBrowser, Phase: phase, Step: step, Max: 15}
	if phase == protocol.OniHandsStatePaused {
		f.Pause = protocol.OniHandsPauseFocus
	}
	return f
}

func handsOff() *feature.HandsFrame {
	return &feature.HandsFrame{Stage: protocol.OniHandsBrowser, Reason: feature.ReasonAllowed}
}

// handsHostOn is a linked host on two outputs, the character on A, wearing
// the desktop's dark signature look in design d.
func handsHostOn(t *testing.T, d design.Style) (*host, *fakeDisplay, *fakeDaemon, *bytes.Buffer) {
	t.Helper()
	a := placement.Output{ID: "A", Rect: image.Rect(0, 0, 1920, 1080), Scale: 1}
	b := placement.Output{ID: "B", Rect: image.Rect(1920, 0, 3200, 720), Scale: 1}
	h, disp, fd := linkedOn(t, "A", a, b)
	h.look = desktoplook.FromRoles(palette.Signature(true))
	h.look.Design = d
	return h, disp, fd, h.led.w.(*bytes.Buffer)
}

// settleHands steps the host until the pill and the card are fully in.
func settleHands(t *testing.T, h *host, now time.Time) time.Time {
	t.Helper()
	for i := 0; i == 0 || (h.pill.f.want && h.pill.f.alpha < 1) || (h.handoff.f.want && h.handoff.f.alpha < 1) || (h.notice.f.want && h.notice.f.alpha < 1); i++ {
		if i > 900 {
			t.Fatalf("the pill never settled: pill %v/%v card %v/%v", h.pill.f.alpha, h.pill.f.want, h.handoff.f.alpha, h.handoff.f.want)
		}
		now = now.Add(16 * time.Millisecond)
		h.step(now)
	}
	return now
}

func pressHands(t *testing.T, h *host, which overlay, action string) {
	t.Helper()
	part := h.handsPart(which)
	for _, p := range part.lay.Pills {
		if p.Action == action {
			at := spec.Point{X: p.Rect.X + p.Rect.W/2, Y: p.Rect.Y + p.Rect.H/2}
			h.handsPointer(which, &at)
			h.handsButton(which, true)
			h.handsButton(which, false)
			return
		}
	}
	t.Fatalf("no %q on %v: %+v", action, which, part.lay.Pills)
}

func partActions(p *handsPart) []string {
	var out []string
	for _, pl := range p.lay.Pills {
		out = append(out, pl.Action)
	}
	return out
}

func TestProgramNoticeNeedsNoStrandAndLeavesNoHiddenReplay(t *testing.T) {
	h, display, fd, ledger := handsHostOn(t, design.Lithic)
	const text = "Local fixture: The local lantern check is ready"
	h.clk.halt()
	h.clk.last = time.Now().Add(-time.Hour)
	send(t, h, &feature.NoticeFrame{Text: text})
	if h.notice.f.alpha >= 1 {
		t.Fatal("a notice after a long idle skipped its fade")
	}
	now := settleHands(t, h, time.Now())
	if !display.ovMapped[overlayNotice] || strings.Join(labels(h.notice.lay.Lines), " ") != text {
		t.Fatalf("notice is not fully readable: mapped=%v, words=%q", display.ovMapped[overlayNotice], labels(h.notice.lay.Lines))
	}
	if h.perch != nil || h.target != nil || h.hands != nil {
		t.Fatal("a plain notice acquired perch, strand or task authority")
	}
	box := h.notice.f.rect
	mask := protocol.OniRect{X: float64(box.Min.X), Y: float64(box.Min.Y), W: float64(box.Dx()), H: float64(box.Dy())}
	if !slices.Contains(h.overlayRects(), mask) {
		t.Fatal("the visible notice is missing from capture masks")
	}
	if strings.Contains(ledger.String(), text) {
		t.Fatal("the notice's words entered the surface ledger")
	}
	pressHands(t, h, overlayNotice, noticeDismiss)
	if display.ovMapped[overlayNotice] || !h.notice.lay.Empty() || len(fd.handsCalls()) != 0 {
		t.Fatal("dismissal left a notice or invoked Hands")
	}
	send(t, h, &feature.NoticeFrame{Text: text})
	now = settleHands(t, h, now)
	h.daemonHidden = true
	h.updateVisible()
	h.daemonHidden = false
	h.updateVisible()
	settleHands(t, h, now)
	if display.ovMapped[overlayNotice] || !h.notice.lay.Empty() {
		t.Fatal("a notice returned after Oni was hidden")
	}
	send(t, h, &feature.NoticeFrame{Text: text})
	settleHands(t, h, now)
	h.retried = true
	h.lost(fd)
	if display.ovMapped[overlayNotice] || !h.notice.lay.Empty() {
		t.Fatal("a notice survived its daemon connection")
	}
}

func TestProgramNoticeDoesNotReplaceHandsOrItsApproval(t *testing.T) {
	h, display, _, _ := handsHostOn(t, design.Lithic)
	send(t, h, handsFrameAt(protocol.OniHandsStateWaiting, 2))
	send(t, h, feature.HandoffCard(feature.HandsBrowser, "h-notice", handoffLine, time.Minute))
	send(t, h, &feature.NoticeFrame{Text: "Local fixture: Build finished"})
	settleHands(t, h, time.Now())
	if !display.ovMapped[overlayPill] || !display.ovMapped[overlayHandoff] || !display.ovMapped[overlayNotice] {
		t.Fatal("the notice replaced the task or its approval")
	}
	rects := []image.Rectangle{h.pill.f.rect, h.handoff.f.rect, h.notice.f.rect}
	for i, a := range rects {
		for _, b := range rects[i+1:] {
			if a.Overlaps(b) {
				t.Fatalf("native panels overlap: %v and %v", a, b)
			}
		}
	}
	pressHands(t, h, overlayNotice, noticeDismiss)
	if !display.ovMapped[overlayPill] || !display.ovMapped[overlayHandoff] || display.ovMapped[overlayNotice] {
		t.Fatal("dismissing the notice affected the task or its approval")
	}
}

func (d *fakeDaemon) handsCalls() []string {
	d.mu.Lock()
	defer d.mu.Unlock()
	return slices.Clone(d.hands)
}

func glowsOn(d *fakeDisplay, output string) []glowPresentation {
	var out []glowPresentation
	for _, g := range d.glows {
		if g.output == output {
			out = append(out, g)
		}
	}
	return out
}

// strength is the alpha the glow painted at an output's top edge.
func strength(g glowPresentation) uint8 {
	img := g.imgs[render.GlowTop]
	return img.NRGBAAt(img.Bounds().Dx()/2, 0).A
}

// With no task, Hands costs nothing: no glow surface, no pill, no timer and
// no commit, whatever inactive frames come.
func TestHandsAtRestCostsNothing(t *testing.T) {
	h, d, fd, buf := handsHostOn(t, design.Lithic)
	now := drive(h, time.Now(), 3)
	presented, overlays := len(d.presented), len(d.overlays[overlayPill])+len(d.overlays[overlayHandoff])
	for _, f := range []*feature.HandsFrame{
		{Stage: protocol.OniHandsOff, Reason: feature.ReasonFeatureOff},
		{Stage: protocol.OniHandsBrowser, Reason: feature.ReasonAllowed},
		handsOff(),
	} {
		send(t, h, f)
	}
	drive(h, now, 3)
	if len(d.glows) != 0 || len(d.glowOn) != 0 {
		t.Fatalf("idle Hands made glow surfaces: %d commits on %v", len(d.glows), d.glowOn)
	}
	if n := len(d.overlays[overlayPill]) + len(d.overlays[overlayHandoff]); n != overlays {
		t.Fatalf("idle Hands committed the pill or the card %d times", n-overlays)
	}
	if h.glow.timer != nil || h.glow.ticking {
		t.Fatal("idle Hands armed the glow's timer")
	}
	if len(d.presented) > presented+3 {
		t.Fatalf("inactive frames committed the sprite %d times", len(d.presented)-presented)
	}
	if slices.Contains(fd.Calls(), protocol.MOniHands) {
		t.Fatal("idle Hands called the daemon")
	}
	for _, e := range ledgerLines(t, buf, EvSound) {
		if strings.HasPrefix(e.Event, "hands-") {
			t.Fatalf("idle Hands sounded %s", e.Event)
		}
	}
}

// The glow is on every output while a task runs, breathes between the
// spec's low and high at its fps while working, at half strength while a
// card waits, holds still while paused and under reduced motion, and is
// gone with the pill and the card in the call that takes the inactive
// frame, with no fade.
func TestTheGlowBreathesOnlyWhileActiveAndGoesAtOnce(t *testing.T) {
	h, d, _, buf := handsHostOn(t, design.Lithic)
	g := h.sp.Hands.Glow
	send(t, h, handsFrameAt(protocol.OniHandsStateWorking, 1))
	start := h.glow.start
	for _, id := range []string{"A", "B"} {
		gs := glowsOn(d, id)
		if len(gs) != 1 || !d.glowOn[id] {
			t.Fatalf("output %s: %d glow commits, mapped %v", id, len(gs), d.glowOn[id])
		}
		o := placement.Output{ID: id}
		for _, out := range d.outs {
			if out.ID == id {
				o = out
			}
		}
		want := render.NewEdgeGlow(o.Rect.Size(), g.StripPx, design.Lithic, glowDPR)
		if gs[0].rects != want.Rects {
			t.Fatalf("output %s: strips %v, want %v", id, gs[0].rects, want.Rects)
		}
		for i, img := range gs[0].imgs {
			if sz := want.DeviceSize(i); img == nil || img.Bounds().Size() != sz {
				t.Fatalf("output %s strip %d: buffer %v, want %v at the glow's scale", id, i, img.Bounds(), sz)
			}
		}
	}
	if !h.glow.ticking || h.glow.timer == nil {
		t.Fatal("a working task's glow is not breathing")
	}
	low := uint8(min(255, 0.5+g.Low*255*0.99))
	if s := strength(glowsOn(d, "A")[0]); s < low-3 || s > low+3 {
		t.Fatalf("the breath starts at %d, want about the spec's low %v", s, g.Low)
	}
	// half a period on: high
	h.syncGlow(start.Add(time.Duration(g.PeriodS * float64(time.Second) / 2)))
	if s := strength(glowsOn(d, "A")[1]); s < 248 {
		t.Fatalf("half a breath on the glow is %d, want the spec's high", s)
	}
	// the timer paces it at the spec's fps
	before := len(d.glows)
	at := time.Now()
	d.drain(t, func() bool { return len(d.glows) > before })
	if el := time.Since(at); el < g.Frame()-5*time.Millisecond {
		t.Fatalf("the next breath came after %v, sooner than one frame %v", el, g.Frame())
	}
	commits := ledgerLines(t, buf, EvCommit)
	if last := commits[len(commits)-1]; last.Part != PartGlow || last.FPS != g.FPS || last.Level == nil || len(last.Rects) != render.GlowStrips {
		t.Fatalf("glow commit line = %+v", last)
	}

	// a card waits: half strength
	send(t, h, handsFrameAt(protocol.OniHandsStateWaiting, 2))
	for range 8 {
		h.syncGlow(start.Add(time.Duration(g.PeriodS * float64(time.Second) / 2)))
		if s := strength(d.glows[len(d.glows)-1]); float64(s) > 255*g.High*g.WaitingFactor+2 {
			t.Fatalf("while a card waits the glow is %d, over half", s)
		}
	}

	// paused: still, at low, no timer and no more commits
	send(t, h, handsFrameAt(protocol.OniHandsStatePaused, 2))
	if h.glow.ticking || h.glow.timer != nil {
		t.Fatal("a paused task's glow still breathes")
	}
	n := len(d.glows)
	for i := range 5 {
		h.syncGlow(time.Now().Add(time.Duration(i) * time.Second))
	}
	if len(d.glows) != n {
		t.Fatalf("a paused glow committed %d more times", len(d.glows)-n)
	}

	// reduced motion: still at high, one commit per output
	send(t, h, handsFrameAt(protocol.OniHandsStateWorking, 3))
	send(t, h, &feature.MotionFrame{Reduce: "on", Courtesy: true})
	if h.glow.ticking || h.glow.timer != nil {
		t.Fatal("under reduced motion the glow still breathes")
	}
	n = len(d.glows)
	for i := range 5 {
		h.syncGlow(time.Now().Add(time.Duration(i) * 700 * time.Millisecond))
	}
	if len(d.glows) != n {
		t.Fatalf("a reduced glow committed %d more times", len(d.glows)-n)
	}
	if s := strength(d.glows[n-1]); s < 248 {
		t.Fatalf("the reduced glow is %d, want the spec's high", s)
	}
	now := settleHands(t, h, time.Now())

	// the inactive frame takes everything down in the same call
	buf.Reset()
	send(t, h, handsOff())
	if len(d.glowOn) != 0 || h.glow.timer != nil || h.glow.outs != nil {
		t.Fatalf("after the task the glow is on %v", d.glowOn)
	}
	if d.ovMapped[overlayPill] || d.ovMapped[overlayHandoff] {
		t.Fatal("after the task the pill or the card is still mapped")
	}
	var seq []string
	for _, line := range strings.Split(strings.TrimSpace(buf.String()), "\n") {
		var e Entry
		if err := json.Unmarshal([]byte(line), &e); err != nil {
			t.Fatal(err)
		}
		seq = append(seq, e.Ev+":"+e.Part+":"+e.Output)
	}
	want := []string{EvFrame + "::", EvHands + "::", EvHide + ":" + PartPill + ":A", EvHide + ":" + PartGlow + ":A", EvHide + ":" + PartGlow + ":B", EvSound + "::"}
	if !slices.Equal(seq[:len(want)], want) {
		t.Fatalf("ending the task logged %q, want %q first", seq, want)
	}
	before = len(d.glows)
	drive(h, now, 30)
	if len(d.glows) != before || d.ovMapped[overlayPill] {
		t.Fatal("something of Hands came back after the task")
	}
	sounds := ledgerLines(t, buf, EvSound)
	if len(sounds) != 1 || sounds[0].Event != "hands-stop" {
		t.Fatalf("ending sounded %+v, want hands-stop through the gate", sounds)
	}
}

// The pill offers Stop, and Resume only while paused; the handoff card
// names the control in the daemon's line, says to press it yourself and
// offers Not now and no yes. Each press sends one oni.hands call.
func TestThePillAndTheHandoffCardSendOnlyStopAndResume(t *testing.T) {
	h, d, fd, buf := handsHostOn(t, design.Lithic)
	send(t, h, handsFrameAt(protocol.OniHandsStateWorking, 1))
	now := settleHands(t, h, time.Now())
	if got := partActions(&h.pill); !slices.Equal(got, []string{handsStop}) {
		t.Fatalf("a working pill offers %v", got)
	}
	pills := d.overlays[overlayPill]
	if len(pills) == 0 || !d.ovMapped[overlayPill] {
		t.Fatal("the pill never showed")
	}
	last := pills[len(pills)-1]
	if last.input != (image.Rectangle{Max: last.rect.Size()}) {
		t.Fatalf("the pill's input region %v is not its face %v", last.input, last.rect.Size())
	}
	at := h.origin.Sub(h.out.Rect.Min)
	char := image.Rectangle{Min: at, Max: at.Add(h.extent())}
	if last.rect.Overlaps(char) || !last.rect.In(image.Rectangle{Max: h.out.Rect.Size()}) {
		t.Fatalf("the pill at %v covers the character %v or leaves the output", last.rect, char)
	}

	// paused: Resume joins Stop, and a press sends resume once
	send(t, h, handsFrameAt(protocol.OniHandsStatePaused, 1))
	now = settleHands(t, h, now)
	if got := partActions(&h.pill); !slices.Equal(got, []string{handsStop, handsResume}) {
		t.Fatalf("a paused pill offers %v", got)
	}
	pressHands(t, h, overlayPill, handsResume)
	pressHands(t, h, overlayPill, handsStop)
	settleCalls(t, fd, func() bool { return len(fd.handsCalls()) == 1 })
	if got := fd.handsCalls(); !slices.Equal(got, []string{protocol.OniHandsResume}) {
		t.Fatalf("hands calls = %v", got)
	}

	// waiting on the person: the card with the control's name and Not now
	send(t, h, handsFrameAt(protocol.OniHandsStateWaiting, 2))
	send(t, h, feature.HandoffCard(feature.HandsBrowser, "h-2", handoffLine, time.Minute))
	now = settleHands(t, h, now)
	if !d.ovMapped[overlayHandoff] {
		t.Fatal("the handoff card never showed")
	}
	k := h.handoff.lay
	if k.Kicker.Text != "Your turn" || len(k.Lines) == 0 || !strings.Contains(strings.Join(labels(k.Lines), " "), "Send") {
		t.Fatalf("the card says %q / %q", k.Kicker.Text, labels(k.Lines))
	}
	if !strings.Contains(k.Hint.Text, "yourself") {
		t.Fatalf("the card's guidance is %q", k.Hint.Text)
	}
	if got := partActions(&h.handoff); !slices.Equal(got, []string{handsNotNow}) {
		t.Fatalf("the card offers %v, want Not now alone", got)
	}
	pr, cr := h.pill.f.rect, h.handoff.f.rect
	if pr.Overlaps(cr) || cr.Overlaps(char) {
		t.Fatalf("the card %v overlaps the pill %v or the character %v", cr, pr, char)
	}
	// the card's words stay off the ledger; its actions' places are on it
	if strings.Contains(buf.String(), "Send") || strings.Contains(buf.String(), "yourself") {
		t.Fatal("the ledger carries the card's words")
	}
	var buttons []ButtonInfo
	for _, e := range ledgerLines(t, buf, EvCommit) {
		if e.Part == PartHandoff {
			buttons = e.Buttons
		}
	}
	if len(buttons) != 1 || buttons[0].Action != handsNotNow || !(buttons[0].X >= float64(cr.Min.X)) {
		t.Fatalf("the card's commit line names buttons %+v", buttons)
	}
	// Resume is not on offer while waiting, even pressed by hand
	h.handsPress(&h.pill, handsResume)
	pressHands(t, h, overlayHandoff, handsNotNow)
	settleCalls(t, fd, func() bool { return len(fd.handsCalls()) == 2 })
	if got := fd.handsCalls(); !slices.Equal(got, []string{protocol.OniHandsResume, protocol.OniHandsStop}) {
		t.Fatalf("hands calls = %v", got)
	}
	if got := ledgerLines(t, buf, EvHandsPress); len(got) != 2 || got[1].Reason != handsNotNow {
		t.Fatalf("press lines = %+v", got)
	}

	// the person pressed the control: the card goes, the pill stays
	send(t, h, handsFrameAt(protocol.OniHandsStateWorking, 3))
	settleHands(t, h, now)
	if d.ovMapped[overlayHandoff] || !d.ovMapped[overlayPill] {
		t.Fatalf("after the handoff: card mapped %v, pill mapped %v", d.ovMapped[overlayHandoff], d.ovMapped[overlayPill])
	}
	for _, m := range fd.Calls() {
		switch m {
		case protocol.MOniHands, protocol.MOniOverlay:
		default:
			t.Errorf("Hands' parts made a %s call", m)
		}
	}
}

func labels(ls []render.Label) []string {
	var out []string
	for _, l := range ls {
		out = append(out, l.Text)
	}
	return out
}

// The pill, the card and the glow are masked out of captures: the
// character's output reports them with the sprite, the other output its
// strips, and the task's end clears both.
func TestHandsPartsAreMaskedOnEveryOutput(t *testing.T) {
	h, d, fd, _ := handsHostOn(t, design.Lithic)
	send(t, h, handsFrameAt(protocol.OniHandsStateWaiting, 1))
	send(t, h, feature.HandoffCard(feature.HandsBrowser, "h-1", handoffLine, time.Minute))
	settleHands(t, h, time.Now())
	d.drain(t, func() bool { _, _, due := h.overlayDue(); return !h.ovBusy && !due })
	latest := map[string][]protocol.OniRect{}
	for _, c := range fd.overlayCalls() {
		latest[c.Output] = c.Rects
	}
	rect := func(r image.Rectangle) protocol.OniRect {
		return protocol.OniRect{X: float64(r.Min.X), Y: float64(r.Min.Y), W: float64(r.Dx()), H: float64(r.Dy())}
	}
	for _, r := range []image.Rectangle{h.pill.f.rect, h.handoff.f.rect} {
		if !slices.Contains(latest["A"], rect(r)) {
			t.Fatalf("A's masks %+v miss %v", latest["A"], r)
		}
	}
	for _, id := range []string{"A", "B"} {
		for _, r := range h.glowRects(id) {
			if !slices.Contains(latest[id], rect(r)) {
				t.Fatalf("%s's masks %+v miss the strip %v", id, latest[id], r)
			}
		}
		if len(latest[id]) > feature.MaxOverlayRects {
			t.Fatalf("%s reports %d rects", id, len(latest[id]))
		}
	}
	if len(latest["B"]) != render.GlowStrips {
		t.Fatalf("B reports %+v, want its four strips", latest["B"])
	}
	send(t, h, handsOff())
	d.drain(t, func() bool { _, _, due := h.overlayDue(); return !h.ovBusy && !due })
	for _, c := range fd.overlayCalls() {
		latest[c.Output] = c.Rects
	}
	if len(latest["A"]) != 0 || len(latest["B"]) != 0 {
		t.Fatalf("after the task the masks are %+v", latest)
	}
}

// Drawn offscreen, the pill and the card wear the desktop's shape, Lithic
// cut or rounded, and the glow wears the look's thinking accent.
func TestHandsPartsWearTheDesktopsShapeAndColours(t *testing.T) {
	for _, dsg := range []design.Style{design.Lithic, design.Material} {
		h, d, _, _ := handsHostOn(t, dsg)
		send(t, h, handsFrameAt(protocol.OniHandsStateWaiting, 1))
		send(t, h, feature.HandoffCard(feature.HandsBrowser, "h-1", handoffLine, time.Minute))
		settleHands(t, h, time.Now())
		ch := h.sp.Perch.Chip
		for _, which := range []overlay{overlayPill, overlayHandoff} {
			ps := d.overlays[which]
			var img *image.NRGBA
			for _, p := range ps {
				if p.img != nil {
					img = p.img
				}
			}
			if img == nil {
				t.Fatalf("%s: overlay %d never drew", dsg, which)
			}
			lay := h.handsPart(which).lay
			corner := float64(dsg.Corner(float32(ch.RadiusPx+ch.PadYPx), float32(lay.Size.X), float32(lay.Size.Y)))
			at := int(0.36 * corner)
			inside := img.NRGBAAt(at, at).A > 0
			if dsg == design.Lithic && inside {
				t.Errorf("lithic: overlay %d's corner is rounded", which)
			}
			if dsg != design.Lithic && !inside {
				t.Errorf("%s: overlay %d's corner is cut", dsg, which)
			}
			if mid := img.NRGBAAt(int(lay.Pad/2), img.Bounds().Dy()/2); mid != h.look.White {
				t.Errorf("%s: overlay %d's plate is %v, want the look's White", dsg, which, mid)
			}
		}
		g := d.glows[len(d.glows)-1].imgs[render.GlowTop]
		px := g.NRGBAAt(g.Bounds().Dx()/2, 0)
		if want := h.look.Accent["thinking"]; px.R != want.R || px.G != want.G || px.B != want.B {
			t.Errorf("%s: the glow is %v, want the thinking accent %v", dsg, px, want)
		}
	}
}

// Losing the daemon ends the task here as it does there.
func TestALostLinkEndsHands(t *testing.T) {
	h, d, fd, _ := handsHostOn(t, design.Lithic)
	send(t, h, handsFrameAt(protocol.OniHandsStateWorking, 1))
	settleHands(t, h, time.Now())
	// a second loss goes dormant rather than redialling
	h.retried = true
	h.lost(fd)
	if h.hands != nil || len(d.glowOn) != 0 || h.glow.timer != nil || d.ovMapped[overlayPill] {
		t.Fatal("a lost link left Hands on screen")
	}
}

// A task done after at least the spec's handsDoneMinS sounds done-notes
// through the sound gate; a shorter one, and one stopped, away or failed
// however long it ran, sounds hands-stop.
func TestOnlyALongDoneTaskSoundsDoneNotes(t *testing.T) {
	sp, err := spec.Load()
	if err != nil {
		t.Fatal(err)
	}
	min := time.Duration(sp.Sound.Cues.HandsDoneMinS * float64(time.Second))
	cases := []struct {
		end  string
		took time.Duration
		want string
	}{
		{protocol.OniHandsEndDone, min + time.Second, "done-notes"},
		{protocol.OniHandsEndDone, min - time.Second, "hands-stop"},
		{protocol.OniHandsEndStopped, min + time.Second, "hands-stop"},
		{protocol.OniHandsEndAway, min + time.Second, "hands-stop"},
		{protocol.OniHandsEndFailed, min + time.Second, "hands-stop"},
		{"", min + time.Second, "hands-stop"},
	}
	for _, c := range cases {
		h, _, _, buf := handsHostOn(t, design.Lithic)
		pcm := filepath.Join(t.TempDir(), "sound.pcm")
		h.player = sound.NewPlayer(h.sp, nil, func() (sound.Sink, error) { return sound.FileSink(pcm), nil })
		h.player.Gate().Enable(true)
		h.player.Gate().Voice("idle")
		send(t, h, handsFrameAt(protocol.OniHandsStateWorking, 1))
		h.handsBegan = h.handsBegan.Add(-c.took)
		// past the gate's debounce after hands-start
		time.Sleep(time.Duration(h.sp.Sound.Synth.DebounceS*float64(time.Second)) + 20*time.Millisecond)
		buf.Reset()
		end := handsOff()
		end.End = c.end
		send(t, h, end)
		var got []string
		for _, e := range ledgerLines(t, buf, EvSound) {
			got = append(got, e.Event+":"+e.Reason)
		}
		if want := []string{c.want + ":" + soundPlayed}; !slices.Equal(got, want) {
			t.Errorf("end %q after %v sounded %q, want %q", c.end, c.took, got, want)
		}
		if lines := ledgerLines(t, buf, EvHands); len(lines) != 1 || lines[0].End != c.end {
			t.Errorf("end %q: hands lines %+v", c.end, lines)
		}
	}
	// a closed gate holds done-notes back like any sound
	h, _, _, buf := handsHostOn(t, design.Lithic)
	send(t, h, handsFrameAt(protocol.OniHandsStateWorking, 1))
	h.handsBegan = h.handsBegan.Add(-min - time.Second)
	buf.Reset()
	end := handsOff()
	end.End = protocol.OniHandsEndDone
	send(t, h, end)
	if got := ledgerLines(t, buf, EvSound); len(got) != 1 || got[0].Event != "done-notes" || got[0].Reason != soundGated {
		t.Fatalf("with sounds off the end sounded %+v", got)
	}
}
