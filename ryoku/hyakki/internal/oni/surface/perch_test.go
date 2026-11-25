package surface

import (
	"bytes"
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

var perchBox = feature.Box{X: 600, Y: 300, W: 800, H: 500}

func perchOn(seq uint64, box feature.Box) *feature.PerchFrame {
	return &feature.PerchFrame{Seq: seq, Attached: true, Window: "w1", App: "Fixture", Title: "Notes",
		Corner: strand.CornerTopRight, Output: "winit", Box: &box}
}

func perchStrand(seq uint64) *feature.StrandTargetFrame {
	return &feature.StrandTargetFrame{Seq: seq, X: 0, Y: 0, W: 1, H: 1, Mode: feature.StrandModePerch}
}

// land steps the host 16 ms apart until its flight is over.
func land(t *testing.T, h *host, now time.Time) time.Time {
	t.Helper()
	for i := 0; h.fl != nil; i++ {
		if i > 600 {
			t.Fatal("the flight never landed")
		}
		now = now.Add(16 * time.Millisecond)
		h.step(now)
	}
	return now
}

// curl steps the host until the perch is done moving: the character by its
// corner, the thread curled and holding past its landing burst, the chip
// fully in.
func curl(t *testing.T, h *host, now time.Time) time.Time {
	t.Helper()
	for i := 0; i == 0 || h.fl != nil || h.strandF.Phase != strand.PhaseHold || h.chip.alpha < 1; i++ {
		if i > 900 {
			t.Fatalf("the perch never settled: flying=%v strand running=%v phase=%s chip alpha=%v want=%v shown=%v perch=%v",
				h.fl != nil, h.strandRun, h.strandF.Phase, h.chip.alpha, h.chip.want, h.shown, h.perch != nil)
		}
		now = now.Add(16 * time.Millisecond)
		h.step(now)
	}
	return drive(h, now, 90)
}

// rest steps the host until its clock stops; an awake character only
// stops under reduced motion.
func rest(t *testing.T, h *host, now time.Time) time.Time {
	t.Helper()
	for i := 0; i == 0 || !h.clk.stopped; i++ {
		if i > 900 {
			t.Fatal("the host never came to rest")
		}
		now = now.Add(16 * time.Millisecond)
		h.step(now)
	}
	return now
}

func seatOf(h *host, box feature.Box, corner string) image.Point {
	o := h.out
	pt := strand.PerchSeat(h.sp, strand.Rect{X: box.X, Y: box.Y, W: box.W, H: box.H}, corner, float64(h.extent().X),
		strand.Rect{W: float64(o.Rect.Dx()), H: float64(o.Rect.Dy())})
	return o.Rect.Min.Add(image.Pt(int(pt.X+0.5), int(pt.Y+0.5)))
}

// perchedHost is a linked host perched on w1 with its thread curled and
// everything settled; the overlay report it owes has gone out.
func perchedHost(t *testing.T) (*host, *fakeDisplay, *fakeDaemon, time.Time) {
	t.Helper()
	h, d, _, fd := linkedHost(t)
	send(t, h, perchOn(1, perchBox))
	send(t, h, perchStrand(1))
	now := land(t, h, time.Now())
	now = curl(t, h, now)
	d.drain(t, func() bool { return !h.ovBusy })
	return h, d, fd, now
}

func TestAPerchFliesToTheCornerThenCurls(t *testing.T) {
	h, d, buf, fd := linkedHost(t)
	send(t, h, perchOn(1, perchBox))
	send(t, h, perchStrand(1))
	if h.fl == nil {
		t.Fatal("the perch did not fly the character")
	}
	now := time.Now()
	for i := 0; h.fl != nil; i++ {
		if i > 600 {
			t.Fatal("the flight never landed")
		}
		if h.target != nil || len(ledgerLines(t, buf, EvStrandTarget)) != 0 {
			t.Fatal("the thread left before the character stood by the corner")
		}
		now = now.Add(16 * time.Millisecond)
		h.step(now)
	}
	if want := seatOf(h, perchBox, strand.CornerTopRight); h.origin != want {
		t.Fatalf("the character landed at %v, want the seat %v", h.origin, want)
	}
	targets := ledgerLines(t, buf, EvStrandTarget)
	if len(targets) != 1 || targets[0].Target.Mode != strand.ModePerch || targets[0].Target.Corner != strand.CornerTopRight {
		t.Fatalf("strand targets after landing = %+v, want one perch on the top-right corner", targets)
	}
	curl(t, h, now)
	curled := false
	for _, e := range ledgerLines(t, buf, EvStep) {
		if e.Strand != nil && e.Strand.Phase == strand.PhaseCurl {
			curled = true
		}
	}
	if !curled {
		t.Fatal("the thread never curled")
	}
	if got := arrivals(t, fd, 1); len(got) != 1 || got[0].Seq != 1 || got[0].Skipped != "" {
		t.Fatalf("arrivals = %+v, want the perch strand landed once", got)
	}
	perches := ledgerLines(t, buf, EvPerch)
	if len(perches) != 1 || perches[0].Perch == nil || perches[0].Perch.Window != "w1" || perches[0].Perch.Corner != strand.CornerTopRight {
		t.Fatalf("perch lines = %+v", perches)
	}
	chips := d.overlays[overlayChip]
	if len(chips) == 0 || !d.ovMapped[overlayChip] {
		t.Fatal("the chip never showed")
	}
	last := chips[len(chips)-1]
	if last.input != (image.Rectangle{Max: last.rect.Size()}) {
		t.Fatalf("the chip's input region %v is not the whole chip %v", last.input, last.rect.Size())
	}
	if h.chipText != "Fixture · Notes" {
		t.Fatalf("the chip says %q", h.chipText)
	}
}

// linkedOn is a linked host on outs with Oni placed on active.
func linkedOn(t *testing.T, active string, outs ...placement.Output) (*host, *fakeDisplay, *fakeDaemon) {
	t.Helper()
	h, d, _ := testHost(t, outs...)
	fd := newFakeDaemon(nil)
	t.Cleanup(func() { fd.Close() })
	h.link, h.linking = fd, false
	h.applyPlacement(&feature.PlacementFrame{Seq: 1, Size: "normal", Corner: "br", Active: active})
	if !h.shown || h.out.ID != active {
		t.Fatalf("the test host is not shown on %s", active)
	}
	return h, d, fd
}

func (d *fakeDaemon) perchCalls() []protocol.OniPerchParams {
	d.mu.Lock()
	defer d.mu.Unlock()
	return slices.Clone(d.perches)
}

func TestADropAsksForThePerchAtTheOutputLocalPoint(t *testing.T) {
	a := placement.Output{ID: "A", Rect: image.Rect(0, 0, 1920, 1080), Scale: 1}
	b := placement.Output{ID: "B", Rect: image.Rect(1920, 0, 3840, 1080), Scale: 1}
	h, d, fd := linkedOn(t, "B", a, b)
	under := protocol.OniRect{X: 300, Y: 200, W: 900, H: 600}
	fd.perchAt = func(p protocol.OniPerchParams) (protocol.OniPerchResult, error) {
		if p.Hover {
			return protocol.OniPerchResult{Window: "w2", App: "Fixture", Output: "B", Box: &under}, nil
		}
		return protocol.OniPerchResult{}, &protocol.Error{Code: protocol.CodeNotFound, Message: "gone", Reason: feature.PerchClosed}
	}
	now := time.Now()
	if len(d.overlays[overlayHighlight]) != 0 {
		t.Fatal("the highlight drew with no drag")
	}
	ext := h.extent()
	start := spec.Point{X: float64(h.origin.X + ext.X/2), Y: float64(h.origin.Y + ext.Y/2)}
	h.pointerMotion(start)
	h.button(true)
	end := spec.Point{X: start.X - 700, Y: start.Y - 400}
	for i := 1; i <= 7; i++ {
		h.pointerMotion(spec.Point{X: start.X - 100*float64(i), Y: start.Y - 400*float64(i)/7})
		now = now.Add(16 * time.Millisecond)
		h.step(now)
	}
	d.drain(t, func() bool { return h.hlBox != nil && !h.hoverBusy })
	now = now.Add(16 * time.Millisecond)
	h.step(now)
	hls := d.overlays[overlayHighlight]
	if len(hls) == 0 {
		t.Fatal("dragging over a window drew no highlight")
	}
	for _, p := range hls {
		if !p.input.Empty() {
			t.Fatalf("the highlight takes the pointer: input %v", p.input)
		}
		if p.output != "B" || !p.rect.Overlaps(image.Rect(300, 200, 1200, 800)) || p.rect.Dx() < 900 {
			t.Fatalf("the highlight at %v on %s is not round the window", p.rect, p.output)
		}
	}
	h.button(false)
	if d.ovMapped[overlayHighlight] || d.ovHidden[overlayHighlight] != 1 {
		t.Fatalf("the drop left the highlight: mapped=%v hidden=%d", d.ovMapped[overlayHighlight], d.ovHidden[overlayHighlight])
	}
	drawn := len(d.overlays[overlayHighlight])
	drive(h, now, 30)
	if len(d.overlays[overlayHighlight]) != drawn {
		t.Fatal("the highlight committed after the drag ended")
	}

	var drop *protocol.OniPerchParams
	deadline := time.Now().Add(3 * time.Second)
	for drop == nil && time.Now().Before(deadline) {
		for _, p := range fd.perchCalls() {
			if !p.Hover {
				drop = &p
			}
		}
		time.Sleep(2 * time.Millisecond)
	}
	if drop == nil {
		t.Fatal("the drop never asked for a perch")
	}
	if want := (protocol.OniPerchParams{Output: "B", X: end.X - 1920, Y: end.Y}); *drop != want {
		t.Fatalf("the drop asked %+v, want %+v (output-local)", *drop, want)
	}
	for _, p := range fd.perchCalls() {
		if p.Hover && p.Output != "B" {
			t.Fatalf("a hover asked about %s", p.Output)
		}
	}
	d.drain(t, func() bool { return slices.Contains(fd.Calls(), protocol.MOniPlace) })
	if h.perch != nil {
		t.Fatal("a refused drop perched")
	}
}

func (d *fakeDaemon) overlayCalls() []protocol.OniOverlayParams {
	d.mu.Lock()
	defer d.mu.Unlock()
	return slices.Clone(d.overlays)
}

func TestOverlayRectsAreReportedOnlyOnChange(t *testing.T) {
	h, d, fd, now := perchedHost(t)
	n := len(fd.overlayCalls())
	if n == 0 {
		t.Fatal("a perch reported no rects")
	}
	for range 5 {
		now = now.Add(16 * time.Millisecond)
		h.step(now)
	}
	d.drain(t, func() bool { return !h.ovBusy })
	if got := len(fd.overlayCalls()); got != n {
		t.Fatalf("an unchanged presenter reported %d more times", got-n)
	}
	moved := perchBox
	moved.X += 10
	send(t, h, perchOn(2, moved))
	now = curl(t, h, now)
	d.drain(t, func() bool { return !h.ovBusy && !h.ovPending })
	calls := fd.overlayCalls()
	if len(calls) <= n {
		t.Fatal("a window move reported no new rects")
	}
	last := calls[len(calls)-1]
	if last.Output != "winit" || len(last.Rects) == 0 {
		t.Fatalf("last report = %+v", last)
	}
	sprite := last.Rects[0]
	if at := h.origin.Sub(h.out.Rect.Min); sprite.X != float64(at.X) || sprite.Y != float64(at.Y) {
		t.Fatalf("the sprite rect %+v is not where the character is (%v)", sprite, at)
	}
	n = len(calls)
	drive(h, curl(t, h, now), 5)
	d.drain(t, func() bool { return !h.ovBusy })
	if got := len(fd.overlayCalls()); got != n {
		t.Fatalf("a settled perch reported %d more times", got-n)
	}
	send(t, h, &feature.PerchFrame{Seq: 3})
	d.drain(t, func() bool { return !h.ovBusy })
	calls = fd.overlayCalls()
	if last := calls[len(calls)-1]; len(last.Rects) != 0 {
		t.Fatalf("after the detach the rects are %+v, want cleared", last.Rects)
	}
}

func TestASettledPerchArmsNoTimer(t *testing.T) {
	h, _, _, now := perchedHost(t)
	// an awake character breathes until motion is reduced; the perch must
	// then add nothing of its own
	send(t, h, &feature.MotionFrame{Reduce: "on", Courtesy: true})
	rest(t, h, now)
	if h.perch == nil || h.chip.alpha != 1 {
		t.Fatalf("not perched and shown: perch=%v chip alpha=%v", h.perch, h.chip.alpha)
	}
	if !h.clk.stopped || h.clk.armed || h.clk.waiting || h.settle != nil || h.hoverBusy || h.strandRun {
		t.Fatalf("a settled perch keeps work: stopped=%v armed=%v waiting=%v settle=%v hover=%v strand=%v",
			h.clk.stopped, h.clk.armed, h.clk.waiting, h.settle != nil, h.hoverBusy, h.strandRun)
	}
}

func (d *fakeDaemon) detachCalls() []string {
	d.mu.Lock()
	defer d.mu.Unlock()
	return slices.Clone(d.detaches)
}

func waitDetach(t *testing.T, fd *fakeDaemon, reason string) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for !slices.Contains(fd.detachCalls(), reason) {
		if time.Now().After(deadline) {
			t.Fatalf("no oni.detach %q; got %v", reason, fd.detachCalls())
		}
		time.Sleep(2 * time.Millisecond)
	}
}

func TestCourtesyHideTearsThePerchDown(t *testing.T) {
	h, d, fd, _ := perchedHost(t)
	buf := h.led.w.(*bytes.Buffer)
	d.cover = true
	h.coveredChanged()
	if h.perch != nil || d.ovMapped[overlayChip] || h.chip.shown {
		t.Fatalf("the hide left the perch: perch=%v chip mapped=%v", h.perch, d.ovMapped[overlayChip])
	}
	waitDetach(t, fd, feature.PerchHidden)
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	hid := -1
	for i, line := range lines {
		if strings.Contains(line, `"ev":"hide"`) && !strings.Contains(line, `"part"`) {
			hid = i
		}
	}
	if hid < 0 {
		t.Fatal("no hide line")
	}
	for _, line := range lines[hid:] {
		if strings.Contains(line, `"ev":"commit"`) {
			t.Fatalf("a commit after the hide: %s", line)
		}
	}
	if !strings.Contains(strings.Join(lines[hid:], "\n"), `"ev":"detach","reason":"hidden"`) {
		t.Fatal("no detach line for the hide")
	}
}

func TestOniOffTearsThePerchDown(t *testing.T) {
	h, d, _, _ := perchedHost(t)
	send(t, h, &feature.StateFrame{State: "idle", Reason: "oni-off"})
	if h.perch != nil || d.ovMapped[overlayChip] || h.hl.shown {
		t.Fatalf("Oni off left the perch: perch=%v chip mapped=%v", h.perch, d.ovMapped[overlayChip])
	}
	if !h.quitting {
		t.Fatal("Oni off did not quit")
	}
}

func TestTheChipsActionDetaches(t *testing.T) {
	h, _, fd, _ := perchedHost(t)
	dt := h.chipLay.Detach
	h.chipPointer(&spec.Point{X: dt.X + dt.W/2, Y: dt.Y + dt.H/2})
	h.chipButton(true)
	h.chipButton(false)
	if h.perch != nil {
		t.Fatal("the detach action left the perch")
	}
	if h.fl == nil {
		t.Fatal("the character did not fly home")
	}
	waitDetach(t, fd, feature.PerchUser)
}

func TestALongWindowMoveWaitsThenCurlsAgain(t *testing.T) {
	h, d, _, now := perchedHost(t)
	buf := h.led.w.(*bytes.Buffer)
	moved := perchBox
	moved.X -= 400
	send(t, h, perchOn(2, moved))
	if h.settle == nil {
		t.Fatal("a long move did not wait for the window to settle")
	}
	if r := ledgerLines(t, buf, EvStrandRetract); len(r) != 1 || r[0].Seq != 1 || r[0].Reason != perchReattach {
		t.Fatal("the thread did not wind in for the move")
	}
	d.drain(t, func() bool { return h.settle == nil })
	now = home(t, h, d, now)
	if h.fl == nil {
		t.Fatal("the settled window did not fly the character")
	}
	now = land(t, h, now)
	if want := seatOf(h, moved, strand.CornerTopRight); h.origin != want {
		t.Fatalf("landed at %v, want the new seat %v", h.origin, want)
	}
	if r := ledgerLines(t, buf, EvStrandReperch); len(r) != 1 || r[0].Seq != 1 || r[0].Box == nil || r[0].Box.X != moved.X {
		t.Fatal("the thread did not curl again after the flight")
	}
	curl(t, h, now)
}

// home steps the host until its thread is home, then runs what that
// posted.
func home(t *testing.T, h *host, d *fakeDisplay, now time.Time) time.Time {
	t.Helper()
	for i := 0; h.strd.Out(); i++ {
		if i > 600 {
			t.Fatal("the thread never came home")
		}
		now = now.Add(16 * time.Millisecond)
		h.step(now)
	}
	d.drain(t, func() bool { return h.fl != nil })
	return now
}

// The spec's reattach: the thread winds home, and only once it is home
// (and the window held still) does the character fly to the new seat.
func TestALongMoveFliesOnlyOnceTheThreadIsHome(t *testing.T) {
	h, d, _, now := perchedHost(t)
	moved := perchBox
	moved.X -= 400
	send(t, h, perchOn(2, moved))
	// the window holds still before the rewind is done
	now = drive(h, now, 2)
	d.drain(t, func() bool { return h.settle == nil })
	if !h.strd.Out() {
		t.Fatal("the thread was home already; the test needs it still winding in")
	}
	if h.fl != nil {
		t.Fatal("the character flew while the thread was still out")
	}
	for i := 0; h.strd.Out(); i++ {
		if i > 600 {
			t.Fatal("the thread never came home")
		}
		if h.fl != nil {
			t.Fatalf("the character flew at step %d with the thread in phase %s", i, h.strandF.Phase)
		}
		now = now.Add(16 * time.Millisecond)
		h.step(now)
	}
	d.drain(t, func() bool { return h.fl != nil })
	if h.fl == nil {
		t.Fatal("the thread came home but the character never flew to the new seat")
	}
	land(t, h, now)
	if want := seatOf(h, moved, strand.CornerTopRight); h.origin != want {
		t.Fatalf("landed at %v, want the new seat %v", h.origin, want)
	}
}
