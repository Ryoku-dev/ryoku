package surface

import (
	"bytes"
	"encoding/json"
	"image"
	"slices"
	"strings"
	"testing"
	"time"

	"ryoku-rashin/internal/oni/feature"
	"ryoku-rashin/internal/oni/placement"
	"ryoku-rashin/internal/oni/spec"
	"ryoku-rashin/internal/oni/strand"
	"ryoku-rashin/internal/protocol"
)

const stepText = "Press Save in the File menu."

func pointing(seq, target uint64) *feature.GuideFrame {
	return &feature.GuideFrame{Seq: seq, Phase: feature.GuidePointing, Step: 2, Text: stepText, Target: target,
		Plan:    &feature.GuidePlan{Done: []string{"Open the File menu"}, Current: "Press Save", Next: []string{"Name the file", "Pick a folder"}},
		Actions: []string{feature.GuideActionNext, feature.GuideActionStop, feature.GuideActionCircle}}
}

// guideTarget is a small control near the top left of the output.
func guideTarget(seq uint64) *feature.StrandTargetFrame {
	return &feature.StrandTargetFrame{Seq: seq, X: 0.3, Y: 0.2, W: 0.04, H: 0.03}
}

// showCard steps the host until the card is fully in and the thread, if
// out, has landed.
func showCard(t *testing.T, h *host, now time.Time) time.Time {
	t.Helper()
	for i := 0; i == 0 || h.card.alpha < 1 || (h.target != nil && h.landed != h.target.Seq); i++ {
		if i > 900 {
			t.Fatalf("the card never settled: alpha=%v want=%v landed=%d", h.card.alpha, h.card.want, h.landed)
		}
		now = now.Add(16 * time.Millisecond)
		h.step(now)
	}
	return now
}

func pillCentre(t *testing.T, h *host, action string) spec.Point {
	t.Helper()
	for _, p := range h.cardLay.Pills {
		if p.Action == action {
			return spec.Point{X: p.Rect.X + p.Rect.W/2, Y: p.Rect.Y + p.Rect.H/2}
		}
	}
	t.Fatalf("the card offers no %q pill: %+v", action, h.cardLay.Pills)
	return spec.Point{}
}

func press(t *testing.T, h *host, action string) {
	t.Helper()
	p := pillCentre(t, h, action)
	h.cardPointer(&p)
	h.cardButton(true)
	h.cardButton(false)
}

func pillActions(h *host) []string {
	var out []string
	for _, p := range h.cardLay.Pills {
		out = append(out, p.Action)
	}
	return out
}

func (d *fakeDaemon) guideCalls() (guides, skips []string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	return slices.Clone(d.guides), slices.Clone(d.skips)
}

func (d *fakeDaemon) pickCalls() []protocol.OniPickParams {
	d.mu.Lock()
	defer d.mu.Unlock()
	return slices.Clone(d.picks)
}

// settleCalls waits until want holds of the fake's calls, then a little
// longer so an extra call would show.
func settleCalls(t *testing.T, fd *fakeDaemon, want func() bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for !want() {
		if time.Now().After(deadline) {
			g, s := fd.guideCalls()
			t.Fatalf("calls never got there: guides=%v skips=%v picks=%+v", g, s, fd.pickCalls())
		}
		time.Sleep(2 * time.Millisecond)
	}
	time.Sleep(50 * time.Millisecond)
}

func overlapsTarget(h *host, r image.Rectangle) bool {
	b := h.target.Box
	return r.Overlaps(image.Rect(int(b.X), int(b.Y), int(b.X+b.W+0.999), int(b.Y+b.H+0.999)))
}

func TestTheCardShowsTheStepBesideItsTargetNeverOverIt(t *testing.T) {
	h, d, buf, fd := linkedHost(t)
	send(t, h, guideTarget(1))
	send(t, h, pointing(1, 1))
	now := showCard(t, h, time.Now())
	cards := d.overlays[overlayCard]
	if len(cards) == 0 || !d.ovMapped[overlayCard] {
		t.Fatal("the card never showed")
	}
	for i, c := range cards {
		if overlapsTarget(h, c.rect) {
			t.Fatalf("card presentation %d at %v covers the target %+v", i, c.rect, h.target.Box)
		}
		if c.input != (image.Rectangle{Max: c.rect.Size()}) {
			t.Fatalf("the card's input region %v is not its whole face %v", c.input, c.rect.Size())
		}
	}
	if got := pillActions(h); !slices.Equal(got, []string{feature.GuideActionCircle, feature.GuideActionStop, feature.GuideActionNext}) {
		t.Fatalf("pills = %v", got)
	}
	if n := len(h.cardLay.Rows); n != cardItemsMax {
		t.Fatalf("the checklist shows %d rows, want %d", n, cardItemsMax)
	}
	if h.cardLay.Rows[0].Label.Text != "Open the File menu" || h.cardLay.Rows[1].Label.Text != "Press Save" || h.cardLay.Rows[2].Label.Text != "Name the file" {
		t.Fatalf("rows = %+v", h.cardLay.Rows)
	}
	// the card's pixels are masked out of captures like the chip's
	d.drain(t, func() bool { return !h.ovBusy })
	calls := fd.overlayCalls()
	if len(calls) == 0 {
		t.Fatal("the card was never reported for the capture masks")
	}
	last := calls[len(calls)-1]
	r := h.card.rect
	if !slices.Contains(last.Rects, protocol.OniRect{X: float64(r.Min.X), Y: float64(r.Min.Y), W: float64(r.Dx()), H: float64(r.Dy())}) {
		t.Fatalf("the reported rects %+v miss the card %v", last.Rects, r)
	}
	// its words stay off the ledger
	if strings.Contains(buf.String(), "File menu") || strings.Contains(buf.String(), "Press Save") {
		t.Fatal("the ledger carries the guide's words")
	}
	lines := ledgerLines(t, buf, EvGuide)
	if len(lines) != 1 || lines[0].Phase != feature.GuidePointing || lines[0].Step != 2 || lines[0].Seq != 1 {
		t.Fatalf("guide lines = %+v", lines)
	}
	// a stale frame changes nothing
	send(t, h, &feature.GuideFrame{Seq: 1, Phase: feature.GuideIdle})
	if h.guide == nil {
		t.Fatal("a stale idle frame took the card down")
	}
	// idle takes it down and the masks are cleared
	send(t, h, &feature.GuideFrame{Seq: 2, Phase: feature.GuideIdle})
	for i := 0; d.ovMapped[overlayCard]; i++ {
		if i > 300 {
			t.Fatal("the card never went away")
		}
		now = now.Add(16 * time.Millisecond)
		h.step(now)
	}
	d.drain(t, func() bool { return !h.ovBusy && !h.ovPending })
	calls = fd.overlayCalls()
	if last := calls[len(calls)-1]; len(last.Rects) != 0 {
		t.Fatalf("after the guide the rects are %+v, want cleared", last.Rects)
	}
}

func TestACardWithoutATargetSitsByTheCharacter(t *testing.T) {
	h, d, _, _ := linkedHost(t)
	send(t, h, &feature.GuideFrame{Seq: 1, Phase: feature.GuideReading, Step: 1, Actions: []string{feature.GuideActionStop}})
	showCard(t, h, time.Now())
	r := d.overlays[overlayCard][len(d.overlays[overlayCard])-1].rect
	out := image.Rectangle{Max: h.out.Rect.Size()}
	if !r.In(out) {
		t.Fatalf("the card at %v is off the output %v", r, out)
	}
	at := h.origin.Sub(h.out.Rect.Min)
	char := image.Rectangle{Min: at, Max: at.Add(h.extent())}
	if r.Overlaps(char) {
		t.Fatalf("the card at %v covers the character at %v", r, char)
	}
	if got := pillActions(h); !slices.Equal(got, []string{feature.GuideActionStop}) {
		t.Fatalf("a reading card offers %v", got)
	}
}

func TestPillsSendOnlyTheCallsThePresenterMayMake(t *testing.T) {
	h, _, _, fd := linkedHost(t)
	send(t, h, guideTarget(1))
	send(t, h, pointing(1, 1))
	now := showCard(t, h, time.Now())
	press(t, h, feature.GuideActionNext)
	// a second press on the same frame sends nothing more
	press(t, h, feature.GuideActionStop)
	settleCalls(t, fd, func() bool { g, _ := fd.guideCalls(); return len(g) == 1 })
	if g, _ := fd.guideCalls(); !slices.Equal(g, []string{protocol.OniGuideNext}) {
		t.Fatalf("guide calls = %v", g)
	}
	// letting go off the pill sends nothing
	send(t, h, pointing(2, 1))
	now = showCard(t, h, now)
	p := pillCentre(t, h, feature.GuideActionStop)
	h.cardPointer(&p)
	h.cardButton(true)
	h.cardPointer(&spec.Point{X: 1, Y: 1})
	h.cardButton(false)
	press(t, h, feature.GuideActionCircle)
	settleCalls(t, fd, func() bool { g, _ := fd.guideCalls(); return len(g) == 2 })
	send(t, h, pointing(3, 1))
	now = showCard(t, h, now)
	press(t, h, feature.GuideActionStop)
	settleCalls(t, fd, func() bool { g, _ := fd.guideCalls(); return len(g) == 3 })
	if g, _ := fd.guideCalls(); !slices.Equal(g, []string{protocol.OniGuideNext, protocol.OniGuideCircle, protocol.OniGuideStop}) {
		t.Fatalf("guide calls = %v", g)
	}

	// a finished guide offers to keep it: Skip is a pill, Keep never is
	send(t, h, &feature.GuideFrame{Seq: 4, Phase: feature.GuideKeeping, Step: 2, Actions: []string{feature.GuideActionKeep, feature.GuideActionSkip}})
	if got := pillActions(h); len(got) != 0 {
		t.Fatalf("with no offer yet the card offers %v", got)
	}
	send(t, h, feature.KeepOffer("keep-7", "Keep these steps for another time?"))
	now = showCard(t, h, now)
	if got := pillActions(h); !slices.Equal(got, []string{feature.GuideActionSkip}) {
		t.Fatalf("a keep offer's pills = %v, want Skip alone", got)
	}
	if h.cardLay.Lines[0].Text != "Keep these steps for another time?" {
		t.Fatalf("the keep card says %+v", h.cardLay.Lines)
	}
	press(t, h, feature.GuideActionSkip)
	settleCalls(t, fd, func() bool { _, s := fd.guideCalls(); return len(s) == 1 })
	if _, s := fd.guideCalls(); !slices.Equal(s, []string{"keep-7"}) {
		t.Fatalf("skip calls = %v", s)
	}
	// an answered offer takes Skip away
	send(t, h, &feature.GuideFrame{Seq: 5, Phase: feature.GuideKeeping, Step: 2, Actions: []string{feature.GuideActionKeep, feature.GuideActionSkip}})
	send(t, h, &feature.ConsentAnswerFrame{ID: "keep-7", Answer: "no"})
	if got := pillActions(h); len(got) != 0 {
		t.Fatalf("an answered offer still offers %v", got)
	}
	// a hint: Next takes it up, Skip lets it go by the hint's id
	send(t, h, &feature.GuideFrame{Seq: 6, Phase: feature.GuideNudging, Kicker: "Kept from before", Text: "Save this file the way you did before?", Actions: []string{feature.GuideActionNext, feature.GuideActionSkip}})
	send(t, h, feature.HintOffer("hint-3", "Save this file the way you did before?"))
	showCard(t, h, now)
	press(t, h, feature.GuideActionSkip)
	settleCalls(t, fd, func() bool { _, s := fd.guideCalls(); return len(s) == 2 })
	if _, s := fd.guideCalls(); s[1] != "hint-3" {
		t.Fatalf("skip calls = %v", s)
	}
	for _, m := range fd.Calls() {
		switch m {
		case protocol.MOniGuide, protocol.MOniSkip, protocol.MOniOverlay, protocol.MOniStrandArrived:
		default:
			t.Errorf("the card made a %s call", m)
		}
	}
}

func TestThePickOverlayIsMappedOnlyWhileArmed(t *testing.T) {
	a := placement.Output{ID: "A", Rect: image.Rect(0, 0, 1920, 1080), Scale: 1}
	b := placement.Output{ID: "B", Rect: image.Rect(1920, 0, 3840, 1080), Scale: 1}
	h, d, fd := linkedOn(t, "A", a, b)
	buf := h.led.w.(*bytes.Buffer)
	if len(d.picks) != 0 {
		t.Fatalf("pick surfaces at rest: %v", d.picks)
	}
	// a click with nothing armed goes nowhere
	h.pickClick("A", spec.Point{X: 5, Y: 5})
	send(t, h, &feature.PickFrame{Seq: 1, Armed: true})
	if !slices.Equal(d.picked, []string{"A", "B"}) {
		t.Fatalf("armed, the pick surfaces are on %v, want every output", d.picked)
	}
	h.pickClick("B", spec.Point{X: 812.5, Y: 431.75})
	if len(d.picked) != 0 {
		t.Fatalf("after the click the pick surfaces are still on %v", d.picked)
	}
	// the overlay is gone, so a second click sends nothing
	h.pickClick("B", spec.Point{X: 9, Y: 9})
	settleCalls(t, fd, func() bool { return len(fd.pickCalls()) == 1 })
	if got := fd.pickCalls(); got[0] != (protocol.OniPickParams{Action: protocol.OniPickPoint, Output: "B", X: 812.5, Y: 431.75}) {
		t.Fatalf("the click sent %+v", got[0])
	}
	// a stale frame arms nothing
	send(t, h, &feature.PickFrame{Seq: 1, Armed: true})
	if len(d.picked) != 0 {
		t.Fatal("a stale pick frame armed the overlay")
	}
	// a right click cancels
	send(t, h, &feature.PickFrame{Seq: 2, Armed: true})
	h.pickCancel()
	if len(d.picked) != 0 {
		t.Fatal("the cancel left the overlay mapped")
	}
	settleCalls(t, fd, func() bool { return len(fd.pickCalls()) == 2 })
	if got := fd.pickCalls(); got[1] != (protocol.OniPickParams{Action: protocol.OniPickCancel}) {
		t.Fatalf("the cancel sent %+v", got[1])
	}
	// the daemon's disarm takes it down and sends nothing back
	send(t, h, &feature.PickFrame{Seq: 3, Armed: true})
	send(t, h, &feature.PickFrame{Seq: 4, Armed: false})
	if len(d.picked) != 0 {
		t.Fatal("the daemon's disarm left the overlay mapped")
	}
	// an output that comes while armed gets its surface; a lost link
	// disarms
	send(t, h, &feature.PickFrame{Seq: 5, Armed: true})
	c := placement.Output{ID: "C", Rect: image.Rect(3840, 0, 5760, 1080), Scale: 1}
	d.outs = append(d.outs, c)
	h.outputsChanged()
	if !slices.Equal(d.picked, []string{"A", "B", "C"}) {
		t.Fatalf("a new output while armed: surfaces on %v", d.picked)
	}
	fd.Close()
	// a second loss goes dormant instead of redialling
	h.retried = true
	h.lost(fd)
	if len(d.picked) != 0 {
		t.Fatal("the lost link left the overlay mapped")
	}
	settleCalls(t, fd, func() bool { return true })
	if got := fd.pickCalls(); len(got) != 2 {
		t.Fatalf("pick calls = %+v, want the click and the cancel only", got)
	}
	for _, e := range ledgerLines(t, buf, EvPick) {
		if e.Rect != nil || e.Box != nil || e.FX != nil || e.FY != nil || e.Input != nil {
			t.Fatalf("a pick line carries a position: %+v", e)
		}
	}
	if spotInLedger(t, buf.Bytes(), 812.5, 431.75) {
		t.Fatal("the click's spot reached the ledger")
	}
	reasons := []string{}
	for _, e := range ledgerLines(t, buf, EvPick) {
		reasons = append(reasons, e.Reason)
	}
	if want := []string{pickArmed, pickPoint, pickArmed, pickCancel, pickArmed, pickDaemon, pickArmed, pickOutputs, pickLost}; !slices.Equal(reasons, want) {
		t.Fatalf("pick lines = %v, want %v", reasons, want)
	}
}

// spotInLedger reports whether any number on any ledger line but its time
// stamp is one of the spot's coordinates. It reads the numbers, not the
// text: a stamp's digits (t 0.113812972) are no position.
func spotInLedger(t *testing.T, ledger []byte, spot ...float64) bool {
	t.Helper()
	var walk func(v any) bool
	walk = func(v any) bool {
		switch v := v.(type) {
		case float64:
			return slices.Contains(spot, v)
		case []any:
			return slices.ContainsFunc(v, walk)
		case map[string]any:
			for k, x := range v {
				if k != "t" && walk(x) {
					return true
				}
			}
		}
		return false
	}
	for _, line := range bytes.Split(bytes.TrimSpace(ledger), []byte("\n")) {
		var v map[string]any
		if err := json.Unmarshal(line, &v); err != nil {
			t.Fatalf("ledger line %q: %v", line, err)
		}
		if walk(v) {
			return true
		}
	}
	return false
}

// The spot check must not take a time stamp's digits for a position, the
// flake the substring check had, and must find a coordinate anywhere else.
func TestTheSpotCheckReadsNumbersNotDigits(t *testing.T) {
	start := time.Unix(1700000000, 0)
	stamps := []time.Time{start, start.Add(113812972 * time.Nanosecond)}
	var buf bytes.Buffer
	l := newLedger(&buf, "wayland-test", func() time.Time {
		now := stamps[0]
		if len(stamps) > 1 {
			stamps = stamps[1:]
		}
		return now
	})
	l.Add(Entry{Ev: EvPick, Reason: pickArmed, Outputs: []string{"A"}})
	if !strings.Contains(buf.String(), "812") {
		t.Fatalf("the stamp does not carry the digits the old check tripped on: %s", buf.String())
	}
	if spotInLedger(t, buf.Bytes(), 812.5, 431.75) {
		t.Fatal("a time stamp was taken for the spot")
	}
	l.Add(Entry{Ev: EvPick, Reason: pickPoint, Box: &strand.Rect{X: 812.5, Y: 431.75}})
	if !spotInLedger(t, buf.Bytes(), 812.5, 431.75) {
		t.Fatal("a line carrying the spot was missed")
	}
}

func TestAHeldCardAndStrandCostNothing(t *testing.T) {
	h, d, _, _ := linkedHost(t)
	send(t, h, &feature.MotionFrame{Reduce: "on", Courtesy: true})
	send(t, h, guideTarget(1))
	send(t, h, &feature.GuideFrame{Seq: 1, Phase: feature.GuideWaiting, Step: 1, Text: stepText, Target: 1,
		Actions: []string{feature.GuideActionNext, feature.GuideActionStop}})
	now := showCard(t, h, time.Now())
	rest(t, h, now)
	// the overlay report the card owed has come back
	d.drain(t, func() bool { return !h.ovBusy && !h.ovPending })
	if !d.ovMapped[overlayCard] || h.card.alpha != 1 || !h.strd.Out() {
		t.Fatalf("not held on screen: card mapped=%v alpha=%v strand out=%v", d.ovMapped[overlayCard], h.card.alpha, h.strd.Out())
	}
	if !h.clk.stopped || h.clk.armed || h.clk.waiting || h.strandRun {
		t.Fatalf("a held card keeps work: stopped=%v armed=%v waiting=%v strand=%v", h.clk.stopped, h.clk.armed, h.clk.waiting, h.strandRun)
	}
	d.mu.Lock()
	posted := d.posted
	d.mu.Unlock()
	sprites, strands, cards := len(d.presented), len(d.strands), len(d.overlays[overlayCard])
	time.Sleep(120 * time.Millisecond)
	d.mu.Lock()
	later := d.posted
	d.mu.Unlock()
	if later != posted || len(d.presented) != sprites || len(d.strands) != strands || len(d.overlays[overlayCard]) != cards {
		t.Fatalf("a held card did work: posted %d→%d, sprite %d→%d, strand %d→%d, card %d→%d",
			posted, later, sprites, len(d.presented), strands, len(d.strands), cards, len(d.overlays[overlayCard]))
	}
}
