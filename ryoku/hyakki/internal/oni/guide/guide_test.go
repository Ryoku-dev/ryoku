package guide

import (
	"errors"
	"fmt"
	"math"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"ryoku-rashin/internal/oni/feature"
	"ryoku-rashin/internal/oni/keep"
	"ryoku-rashin/internal/oni/screenask"
	"ryoku-rashin/internal/oni/spec"
)

var t0 = time.Date(2026, 11, 23, 12, 0, 0, 0, time.UTC)

// rig plays the daemon: it hands events to the machine, keeps the latest
// token of each kind the machine asked for, and checks every frame shown.
type rig struct {
	t   *testing.T
	m   *Machine
	at  time.Time
	cfg Config

	read, look, point, settle, seq uint64
	reads, looks, points           int
	shows                          []feature.GuideFrame
	all                            []Effect
}

func config(t *testing.T) Config {
	t.Helper()
	s, err := spec.Load()
	if err != nil {
		t.Fatal(err)
	}
	return FromSpec(s)
}

func newRig(t *testing.T, keepOn bool) *rig {
	t.Helper()
	r := &rig{t: t, at: t0, cfg: config(t)}
	r.cfg.Keep = keepOn
	r.m = New(r.cfg, func() time.Time { return r.at })
	return r
}

func (r *rig) do(ev Event) []Effect {
	r.t.Helper()
	eff := r.m.Handle(ev)
	for _, e := range eff {
		switch e := e.(type) {
		case ReadScreen:
			r.read = e.Token
			r.reads++
		case Look:
			r.look = e.Token
			r.looks++
		case Point:
			r.point = e.Token
			r.points++
		case StartSettle:
			r.settle = e.Token
		case Show:
			if _, err := feature.EncodeFrame(&e.Frame); err != nil {
				r.t.Fatalf("after %T the machine showed an invalid frame %+v: %v", ev, e.Frame, err)
			}
			if n := len(r.shows); n > 0 && e.Frame.Seq <= r.shows[n-1].Seq {
				r.t.Fatalf("frame seq %d after %d", e.Frame.Seq, r.shows[n-1].Seq)
			}
			r.shows = append(r.shows, e.Frame)
		}
	}
	r.all = append(r.all, eff...)
	return eff
}

func (r *rig) last() feature.GuideFrame {
	r.t.Helper()
	if len(r.shows) == 0 {
		r.t.Fatal("no frame shown")
	}
	return r.shows[len(r.shows)-1]
}

func box(x float64) feature.Box { return feature.Box{X: x, Y: 0.1, W: 0.06, H: 0.04} }

// pointing is a parsed step reply pointing at label.
func pointing(say, label string, at float64, sure bool) screenask.Pointing {
	b := box(at)
	conf := 0.9
	if !sure {
		conf = 0.5
	}
	return screenask.Pointing{Kind: screenask.KindBox, Box: &b, Label: label, Conf: conf, Sure: sure, Verified: sure, Say: say}
}

func (r *rig) reply(p screenask.Pointing) []Effect { return r.do(Read{Token: r.read, Pointing: p}) }

func (r *rig) pointed() []Effect {
	r.seq++
	return r.do(Pointed{Token: r.point, Seq: r.seq})
}

func (r *rig) arrived() []Effect { return r.do(Arrived{Seq: r.seq}) }

func (r *rig) land() { r.pointed(); r.arrived() }

func (r *rig) settled(changed bool) []Effect {
	return r.do(Settled{Token: r.settle, Changed: changed})
}

// step answers the pending read with a trusted step on label, lands it and
// sees the window change after Next.
func (r *rig) step(say, label string, at float64, sure bool) {
	r.reply(pointing(say, label, at, sure))
	r.land()
	r.do(Advance{By: ByNext})
	r.settled(true)
}

func (r *rig) start() []Effect { return r.do(Start{Goal: "save this file", App: "notes"}) }

func kinds(eff []Effect) string {
	var out []string
	for _, e := range eff {
		out = append(out, strings.TrimPrefix(fmt.Sprintf("%T", e), "guide."))
	}
	return strings.Join(out, " ")
}

func find[T Effect](eff []Effect) (T, bool) {
	for _, e := range eff {
		if v, ok := e.(T); ok {
			return v, true
		}
	}
	var zero T
	return zero, false
}

func all[T Effect](eff []Effect) []T {
	var out []T
	for _, e := range eff {
		if v, ok := e.(T); ok {
			out = append(out, v)
		}
	}
	return out
}

func ended(t *testing.T, r *rig) End {
	t.Helper()
	e, ok := find[End](r.all)
	if !ok {
		t.Fatalf("the guide did not end: %s", kinds(r.all))
	}
	return e
}

var setups = []struct {
	name, phase string
	keep        bool
	run         func(r *rig)
}{
	{"idle", feature.GuideIdle, false, func(r *rig) {}},
	{"reading", feature.GuideReading, false, func(r *rig) { r.start() }},
	{"reading, its picture asked about", feature.GuideReading, false, func(r *rig) {
		r.start()
		r.do(Asking{Token: r.read})
	}},
	{"reading a kept guide", feature.GuideReading, false, func(r *rig) {
		r.do(Start{Goal: "save this file", App: "notes", Entry: kept()})
	}},
	{"reading the settle", feature.GuideReading, false, func(r *rig) {
		r.start()
		r.reply(pointing("Open the File menu.", "File", 0.1, true))
		r.land()
		r.do(Advance{By: ByNext})
	}},
	{"pointing, strand not sent", feature.GuidePointing, false, func(r *rig) {
		r.start()
		r.reply(pointing("Open the File menu.", "File", 0.1, true))
	}},
	{"pointing, strand flying", feature.GuidePointing, false, func(r *rig) {
		r.start()
		r.reply(pointing("Open the File menu.", "File", 0.1, true))
		r.pointed()
	}},
	{"waiting", feature.GuideWaiting, false, func(r *rig) {
		r.start()
		r.reply(pointing("Open the File menu.", "File", 0.1, true))
		r.land()
	}},
	{"explaining an unchanged step", feature.GuideExplaining, false, func(r *rig) {
		r.start()
		r.reply(pointing("Open the File menu.", "File", 0.1, true))
		r.land()
		r.do(Advance{By: ByNext})
		r.settled(false)
		r.land()
		r.do(Advance{By: ByNext})
		r.settled(false)
	}},
	{"explaining a refused read", feature.GuideExplaining, false, func(r *rig) {
		r.start()
		r.do(Read{Token: r.read, Err: &screenask.Refusal{Code: screenask.CodeLowConfidence, Line: "not sure"}})
	}},
	{"keeping", feature.GuideKeeping, true, func(r *rig) {
		r.start()
		r.step("Open the File menu.", "File", 0.1, true)
		r.reply(screenask.Pointing{Kind: screenask.KindNone, Done: true, Say: "All saved."})
	}},
	{"done", feature.GuideDone, false, func(r *rig) {
		r.start()
		r.step("Open the File menu.", "File", 0.1, true)
		r.reply(screenask.Pointing{Kind: screenask.KindNone, Done: true, Say: "All saved."})
	}},
	{"winding", feature.GuideWinding, false, func(r *rig) {
		r.start()
		r.reply(pointing("Open the File menu.", "File", 0.1, true))
		r.land()
		r.do(Stop{Why: "dismissed"})
	}},
}

// the events, each answering whatever the rig last saw asked for
var events = []struct {
	name string
	ev   func(r *rig) Event
}{
	{"start", func(r *rig) Event { return Start{Goal: "export it", App: "notes"} }},
	{"read", func(r *rig) Event {
		return Read{Token: r.read, Pointing: pointing("Click Save.", "Save", 0.3, true)}
	}},
	{"looked", func(r *rig) Event { return Looked{Token: r.look, Words: words("File Save", 0.1)} }},
	{"pointed", func(r *rig) Event { return Pointed{Token: r.point, Seq: r.seq + 1} }},
	{"arrived", func(r *rig) Event { return Arrived{Seq: r.seq} }},
	{"cancelled", func(r *rig) Event { return Cancelled{Seq: r.seq, Why: "superseded"} }},
	{"next", func(r *rig) Event { return Advance{By: ByNext} }},
	{"circle", func(r *rig) Event { return Circle{} }},
	{"settled", func(r *rig) Event { return Settled{Token: r.settle, Changed: true} }},
	{"stop", func(r *rig) Event { return Stop{Why: "dismissed"} }},
	{"deadline", func(r *rig) Event { return Deadline{Token: max(r.read, r.look)} }},
	{"asking", func(r *rig) Event { return Asking{Token: r.read} }},
	{"allowed", func(r *rig) Event { return Allowed{Token: r.read} }},
	{"keep", func(r *rig) Event { return KeepAnswer{} }},
}

// ignored: the event changes nothing and does nothing
const ignored = "-"

// The outcome of every event in every phase, "phase: effects" or ignored.
// An event answers the newest token or strand the rig saw asked for, so in
// a phase that no longer waits on it the event is a stale reply.
var outcomes = map[string]map[string]string{
	"idle": {
		"start": "reading: Show ReadScreen", "read": ignored, "looked": ignored, "pointed": ignored,
		"arrived": ignored, "cancelled": ignored, "next": ignored, "circle": ignored,
		"settled": ignored, "stop": ignored, "deadline": ignored, "keep": ignored,
		"asking": ignored, "allowed": ignored,
	},
	"reading": {
		"start": ignored, "read": "pointing: Point", "looked": ignored, "pointed": ignored,
		"arrived": ignored, "cancelled": ignored, "next": ignored, "circle": ignored,
		"settled": ignored, "stop": "winding: CancelRead Show End", "deadline": "done: CancelRead Show End", "keep": ignored,
		"asking": "reading: SetDeadline", "allowed": ignored,
	},
	// the deadline is held until the yes, which gives the read a whole
	// timeout again
	"reading, its picture asked about": {
		"start": ignored, "read": "pointing: Point", "looked": ignored, "pointed": ignored,
		"arrived": ignored, "cancelled": ignored, "next": ignored, "circle": ignored,
		"settled": ignored, "stop": "winding: CancelRead Show End", "deadline": ignored, "keep": ignored,
		"asking": ignored, "allowed": "reading: SetDeadline",
	},
	"reading a kept guide": {
		"start": ignored, "read": ignored, "looked": "pointing: Point", "pointed": ignored,
		"arrived": ignored, "cancelled": ignored, "next": ignored, "circle": ignored,
		"settled": ignored, "stop": "winding: CancelRead Show End", "deadline": "done: CancelRead Show End", "keep": ignored,
		"asking": ignored, "allowed": ignored,
	},
	"reading the settle": {
		"start": ignored, "read": ignored, "looked": ignored, "pointed": ignored,
		"arrived": ignored, "cancelled": ignored, "next": ignored, "circle": ignored,
		"settled": "reading: Retract Show ReadScreen", "stop": "winding: Retract Show End", "deadline": ignored, "keep": ignored,
		"asking": ignored, "allowed": ignored,
	},
	// next is held until the strand lands; a stop retracts the strand once
	// its seq comes back (TestAStrandSentAfterAStopIsRetracted)
	"pointing, strand not sent": {
		"start": ignored, "read": ignored, "looked": ignored, "pointed": "pointing: Show",
		"arrived": ignored, "cancelled": ignored, "next": "pointing: ", "circle": ignored,
		"settled": ignored, "stop": "winding: Show End", "deadline": ignored, "keep": ignored,
		"asking": ignored, "allowed": ignored,
	},
	"pointing, strand flying": {
		"start": ignored, "read": ignored, "looked": ignored, "pointed": ignored,
		"arrived": "waiting: Show", "cancelled": "done: Show End", "next": "pointing: ", "circle": ignored,
		"settled": ignored, "stop": "winding: Retract Show End", "deadline": ignored, "keep": ignored,
		"asking": ignored, "allowed": ignored,
	},
	"waiting": {
		"start": ignored, "read": ignored, "looked": ignored, "pointed": ignored,
		"arrived": ignored, "cancelled": ignored, "next": "reading: Show StartSettle", "circle": "pointing: Point",
		"settled": ignored, "stop": "winding: Retract Show End", "deadline": ignored, "keep": ignored,
		"asking": ignored, "allowed": ignored,
	},
	// next moves past the unchanged step without trusting it
	"explaining an unchanged step": {
		"start": ignored, "read": ignored, "looked": ignored, "pointed": ignored,
		"arrived": ignored, "cancelled": ignored, "next": "reading: Retract Show ReadScreen", "circle": "pointing: Point",
		"settled": ignored, "stop": "winding: Retract Show End", "deadline": ignored, "keep": ignored,
		"asking": ignored, "allowed": ignored,
	},
	// next reads the same step again
	"explaining a refused read": {
		"start": ignored, "read": ignored, "looked": ignored, "pointed": ignored,
		"arrived": ignored, "cancelled": ignored, "next": "reading: Show ReadScreen", "circle": ignored,
		"settled": ignored, "stop": "winding: Show End", "deadline": ignored, "keep": ignored,
		"asking": ignored, "allowed": ignored,
	},
	"keeping": {
		"start": ignored, "read": ignored, "looked": ignored, "pointed": ignored,
		"arrived": ignored, "cancelled": ignored, "next": ignored, "circle": ignored,
		"settled": ignored, "stop": "done: Show End", "deadline": ignored, "keep": "done: Show End",
		"asking": ignored, "allowed": ignored,
	},
	"done": {
		"start": ignored, "read": ignored, "looked": ignored, "pointed": ignored,
		"arrived": ignored, "cancelled": ignored, "next": ignored, "circle": ignored,
		"settled": ignored, "stop": ignored, "deadline": ignored, "keep": ignored,
		"asking": ignored, "allowed": ignored,
	},
	"winding": {
		"start": ignored, "read": ignored, "looked": ignored, "pointed": ignored,
		"arrived": ignored, "cancelled": ignored, "next": ignored, "circle": ignored,
		"settled": ignored, "stop": ignored, "deadline": ignored, "keep": ignored,
		"asking": ignored, "allowed": ignored,
	},
}

func TestEveryPhaseAndEventHasAnOutcome(t *testing.T) {
	phases := map[string]bool{}
	for _, s := range setups {
		phases[s.phase] = true
		row, ok := outcomes[s.name]
		if !ok {
			t.Fatalf("no outcomes for %q", s.name)
		}
		if len(row) != len(events) {
			t.Fatalf("%q states %d outcomes for %d events", s.name, len(row), len(events))
		}
		for _, e := range events {
			want, ok := row[e.name]
			if !ok {
				t.Fatalf("%q states no outcome for %q", s.name, e.name)
			}
			t.Run(s.name+"/"+e.name, func(t *testing.T) {
				r := newRig(t, s.keep)
				s.run(r)
				if got := r.m.Phase(); got != s.phase {
					t.Fatalf("setup reached %q, want %q", got, s.phase)
				}
				eff := r.do(e.ev(r))
				got := r.m.Phase() + ": " + kinds(eff)
				if want == ignored {
					if len(eff) != 0 || r.m.Phase() != s.phase {
						t.Fatalf("got %q, want the event ignored", got)
					}
					return
				}
				if got != want {
					t.Fatalf("got %q, want %q", got, want)
				}
			})
		}
	}
	for _, p := range feature.GuidePhases {
		if p != feature.GuideNudging && !phases[p] {
			t.Errorf("no setup reaches phase %q", p)
		}
	}
}

func TestStaleRepliesChangeNothing(t *testing.T) {
	r := newRig(t, false)
	r.start()
	first := r.read
	// an unsure step left unchanged is read again
	r.reply(pointing("Open the File menu.", "File", 0.1, false))
	r.land()
	firstSeq := r.seq
	r.do(Advance{By: ByNext})
	firstSettle := r.settle
	if eff := r.settled(false); kinds(eff) != "Show ReadScreen" {
		t.Fatalf("an unsure unchanged step did %q, want a second read", kinds(eff))
	}
	for name, ev := range map[string]Event{
		"the first read's reply": Read{Token: first, Pointing: pointing("Click Save.", "Save", 0.3, true)},
		"the first strand":       Arrived{Seq: firstSeq},
		"the first settle":       Settled{Token: firstSettle, Changed: true},
		"a deadline of old":      Deadline{Token: first},
		"an unknown look":        Looked{Token: r.read, Words: words("File", 0.1)},
	} {
		if eff := r.do(ev); len(eff) != 0 || r.m.Phase() != feature.GuideReading {
			t.Errorf("%s did %q in %s", name, kinds(eff), r.m.Phase())
		}
	}
	r.reply(pointing("Open the File menu.", "File", 0.1, false))
	r.pointed()
	if eff := r.do(Arrived{Seq: firstSeq}); len(eff) != 0 {
		t.Errorf("the old strand's arrival did %q", kinds(eff))
	}
	if eff := r.do(Cancelled{Seq: firstSeq, Why: "superseded"}); len(eff) != 0 {
		t.Errorf("the old strand's cancel did %q", kinds(eff))
	}
	if eff := r.arrived(); kinds(eff) != "Show" || r.m.Phase() != feature.GuideWaiting {
		t.Fatalf("the current arrival did %q in %s", kinds(eff), r.m.Phase())
	}
	if eff := r.arrived(); len(eff) != 0 {
		t.Errorf("a repeated arrival did %q", kinds(eff))
	}
}

func TestStopMidReadCancelsTheReadAndDropsTheLateReply(t *testing.T) {
	r := newRig(t, false)
	r.start()
	r.step("Open the File menu.", "File", 0.1, true)
	pending := r.read
	eff := r.do(Stop{Why: "dismissed"})
	if kinds(eff) != "CancelRead Show End" {
		t.Fatalf("stop mid-read did %q", kinds(eff))
	}
	if c, _ := find[CancelRead](eff); c.Token != pending {
		t.Fatalf("cancelled read %d, want %d", c.Token, pending)
	}
	if e := ended(t, r); e.Reason != EndStopped || e.Why != "dismissed" || e.Line != lineStopped {
		t.Fatalf("end %+v", e)
	}
	if f := r.last(); f.Phase != feature.GuideWinding {
		t.Fatalf("frame %+v, want winding", f)
	}
	if eff := r.reply(pointing("Click Save.", "Save", 0.3, true)); len(eff) != 0 {
		t.Fatalf("the late reply did %q", kinds(eff))
	}
	if eff := r.do(Deadline{Token: pending}); len(eff) != 0 {
		t.Fatalf("the late deadline did %q", kinds(eff))
	}
	if r.points != 1 || r.m.Phase() != feature.GuideWinding {
		t.Fatalf("points %d, phase %s", r.points, r.m.Phase())
	}
}

func TestAStrandSentAfterAStopIsRetracted(t *testing.T) {
	r := newRig(t, false)
	r.start()
	r.reply(pointing("Open the File menu.", "File", 0.1, true))
	r.do(Stop{Why: "detached"})
	r.seq = 7
	eff := r.do(Pointed{Token: r.point, Seq: 7})
	if got, _ := find[Retract](eff); kinds(eff) != "Retract" || got.Seq != 7 {
		t.Fatalf("the late strand did %q", kinds(eff))
	}
	if eff := r.do(Pointed{Token: r.point, Seq: 7}); len(eff) != 0 {
		t.Fatalf("a second answer did %q", kinds(eff))
	}
}

func TestStepCeilingEndsWithTheLine(t *testing.T) {
	r := newRig(t, true)
	r.start()
	for i := 1; i <= r.cfg.MaxSteps; i++ {
		if r.m.Phase() != feature.GuideReading {
			t.Fatalf("step %d starts in %s", i, r.m.Phase())
		}
		r.step(fmt.Sprintf("Click button %d.", i), fmt.Sprintf("Button %d", i), 0.05*float64(i%10), true)
	}
	e := ended(t, r)
	if e.Reason != EndCeiling || e.Line != lineCeiling {
		t.Fatalf("end %+v", e)
	}
	if r.reads != r.cfg.MaxSteps {
		t.Fatalf("%d reads for a ceiling of %d", r.reads, r.cfg.MaxSteps)
	}
	if f := r.last(); f.Phase != feature.GuideDone || f.Text != lineCeiling || f.Step != r.cfg.MaxSteps {
		t.Fatalf("last frame %+v", f)
	}
	if _, ok := find[OfferKeep](r.all); ok {
		t.Fatal("a guide cut at the ceiling was offered for keeping")
	}
	if rs := all[Retract](r.all); len(rs) != r.cfg.MaxSteps {
		t.Fatalf("%d retracts for %d strands", len(rs), r.cfg.MaxSteps)
	}
}

func TestReadDeadlineEndsWithTheTimeoutLine(t *testing.T) {
	r := newRig(t, false)
	eff := r.start()
	rs, _ := find[ReadScreen](eff)
	if want := t0.Add(75 * time.Second); !rs.Until.Equal(want) {
		t.Fatalf("read until %v, want %v", rs.Until, want)
	}
	eff = r.do(Deadline{Token: rs.Token})
	if kinds(eff) != "CancelRead Show End" {
		t.Fatalf("deadline did %q", kinds(eff))
	}
	if e := ended(t, r); e.Reason != EndTimeout || e.Line != lineTimeout {
		t.Fatalf("end %+v", e)
	}
	if f := r.last(); f.Phase != feature.GuideDone || f.Text != lineTimeout {
		t.Fatalf("frame %+v", f)
	}

	// a reply that comes back after its deadline, before the timer fired
	r = newRig(t, false)
	r.start()
	r.at = t0.Add(76 * time.Second)
	eff = r.reply(pointing("Open the File menu.", "File", 0.1, true))
	if kinds(eff) != "Show End" || r.points != 0 {
		t.Fatalf("a reply past its deadline did %q", kinds(eff))
	}
	if e := ended(t, r); e.Reason != EndTimeout {
		t.Fatalf("end %+v", e)
	}

	// one in time is used, and the next read has its own deadline
	r = newRig(t, false)
	r.start()
	r.at = t0.Add(74 * time.Second)
	r.step("Open the File menu.", "File", 0.1, true)
	rs, _ = find[ReadScreen](r.all[len(r.all)-1:])
	if want := r.at.Add(75 * time.Second); !rs.Until.Equal(want) {
		t.Fatalf("second read until %v, want %v", rs.Until, want)
	}
}

// A read whose picture waits on the user's yes holds its deadline: the
// consent's own expiry bounds that wait, and a yes, however slow, gives the
// read a whole timeout from then. A read already late when it asks ends.
func TestASlowYesGivesTheReadAWholeTimeout(t *testing.T) {
	r := newRig(t, false)
	r.start()
	r.at = t0.Add(10 * time.Second)
	eff := r.do(Asking{Token: r.read})
	if sd, ok := find[SetDeadline](eff); !ok || len(eff) != 1 || sd.Token != r.read || !sd.Until.IsZero() {
		t.Fatalf("asking did %q %+v", kinds(eff), eff)
	}
	r.at = t0.Add(100 * time.Second)
	if eff := r.do(Deadline{Token: r.read}); len(eff) != 0 {
		t.Fatalf("a deadline while asking did %q", kinds(eff))
	}
	eff = r.do(Allowed{Token: r.read})
	if sd, ok := find[SetDeadline](eff); !ok || len(eff) != 1 || !sd.Until.Equal(t0.Add(175*time.Second)) {
		t.Fatalf("the yes at 100 s did %q %+v", kinds(eff), eff)
	}
	if eff := r.do(Allowed{Token: r.read}); len(eff) != 0 {
		t.Fatalf("a second yes did %q", kinds(eff))
	}
	r.at = t0.Add(174 * time.Second)
	if eff := r.reply(pointing("Open the File menu.", "File", 0.1, true)); kinds(eff) != "Point" {
		t.Fatalf("a reply 74 s after the yes did %q", kinds(eff))
	}

	// the read after the yes still ends at its own deadline
	r = newRig(t, false)
	r.start()
	r.do(Asking{Token: r.read})
	r.at = t0.Add(100 * time.Second)
	r.do(Allowed{Token: r.read})
	r.at = t0.Add(176 * time.Second)
	if eff := r.reply(pointing("Open the File menu.", "File", 0.1, true)); kinds(eff) != "Show End" || ended(t, r).Reason != EndTimeout {
		t.Fatalf("a reply 76 s after the yes did %q", kinds(eff))
	}
	r = newRig(t, false)
	r.start()
	r.do(Asking{Token: r.read})
	r.at = t0.Add(100 * time.Second)
	r.do(Allowed{Token: r.read})
	r.at = t0.Add(175 * time.Second)
	if eff := r.do(Deadline{Token: r.read}); kinds(eff) != "CancelRead Show End" || ended(t, r).Reason != EndTimeout {
		t.Fatalf("the deadline after the yes did %q", kinds(eff))
	}

	// capturing and reading the words before asking took the whole timeout
	r = newRig(t, false)
	r.start()
	r.at = t0.Add(76 * time.Second)
	if eff := r.do(Asking{Token: r.read}); kinds(eff) != "CancelRead Show End" || ended(t, r).Reason != EndTimeout {
		t.Fatalf("asking past the deadline did %q", kinds(eff))
	}
}

func TestUnchangedOnceRepointsATrustedStepThenExplains(t *testing.T) {
	r := newRig(t, false)
	r.start()
	r.reply(pointing("Open the File menu.", "File", 0.1, true))
	r.land()
	if f := r.last(); f.Kicker != "Step 1" || f.Text != "Open the File menu." {
		t.Fatalf("first frame %+v", f)
	}
	r.do(Advance{By: ByNext})
	eff := r.settled(false)
	p, ok := find[Point](eff)
	if kinds(eff) != "Point" || !ok || p.Box != box(0.1) || p.Step != 1 {
		t.Fatalf("unchanged once did %q", kinds(eff))
	}
	r.pointed()
	if f := r.last(); f.Phase != feature.GuidePointing || f.Kicker != "Step 1 · again" || f.Target != r.seq {
		t.Fatalf("re-point frame %+v", f)
	}
	r.arrived()
	r.do(Advance{By: ByNext})
	eff = r.settled(false)
	if kinds(eff) != "Show" || r.m.Phase() != feature.GuideExplaining {
		t.Fatalf("unchanged twice did %q in %s", kinds(eff), r.m.Phase())
	}
	f := r.last()
	if f.Text != lineUnchanged || f.Target != r.seq || !slices.Contains(f.Actions, feature.GuideActionCircle) {
		t.Fatalf("explain frame %+v", f)
	}
	if r.reads != 1 {
		t.Fatalf("%d reads, want the one", r.reads)
	}
	// the window changing now still moves on, trusted
	r.do(Advance{By: ByChange})
	if eff := r.settled(true); kinds(eff) != "Retract Show ReadScreen" {
		t.Fatalf("a late change did %q", kinds(eff))
	}
}

func TestUnchangedOnceRereadsAnUntrustedStep(t *testing.T) {
	r := newRig(t, false)
	r.start()
	r.reply(pointing("Open the File menu.", "File", 0.1, false))
	r.land()
	if f := r.last(); f.Kicker != kickerGuess {
		t.Fatalf("an unsure step's kicker %q", f.Kicker)
	}
	r.do(Advance{By: ByNext})
	eff := r.settled(false)
	rs, ok := find[ReadScreen](eff)
	if !ok || !rs.Again || len(rs.Done) != 0 {
		t.Fatalf("unchanged once did %q (%+v), want the step read again", kinds(eff), rs)
	}
	r.reply(pointing("Open the File menu.", "File", 0.12, false))
	r.land()
	for _, f := range r.shows {
		if strings.Contains(f.Kicker, "again") {
			t.Fatalf("an untrusted step was shown as %q", f.Kicker)
		}
	}
	r.do(Advance{By: ByNext})
	if r.settled(false); r.m.Phase() != feature.GuideExplaining || r.last().Text != lineUnchanged {
		t.Fatalf("unchanged twice left %s, frame %+v", r.m.Phase(), r.last())
	}
}

func TestAnAdvanceBeforeTheStrandLandsWaitsForIt(t *testing.T) {
	r := newRig(t, false)
	r.start()
	r.reply(pointing("Open the File menu.", "File", 0.1, true))
	r.pointed()
	if eff := r.do(Advance{By: ByVoice}); len(eff) != 0 {
		t.Fatalf("an early advance did %q", kinds(eff))
	}
	if eff := r.arrived(); kinds(eff) != "Show StartSettle" {
		t.Fatalf("the arrival after an advance did %q", kinds(eff))
	}
	if eff := r.do(Advance{By: "click"}); len(eff) != 0 {
		t.Fatalf("an unknown advance did %q", kinds(eff))
	}
}

func TestDoneStepsReachTheNextReadAndTheChecklist(t *testing.T) {
	r := newRig(t, false)
	r.start()
	b := box(0.1)
	p := screenask.Pointing{Kind: screenask.KindBox, Box: &b, Label: "File", Conf: 0.9, Sure: true, Verified: true,
		Say: "Open the File menu.", Plan: []screenask.PlanItem{{Text: "Click Save", Control: "Save"}}}
	r.reply(p)
	r.pointed()
	f := r.last()
	if f.Plan == nil || f.Plan.Current != "Open the File menu." || !reflect.DeepEqual(f.Plan.Next, []string{"Click Save"}) || f.Plan.Done != nil {
		t.Fatalf("checklist %+v", f.Plan)
	}
	r.arrived()
	r.do(Advance{By: ByNext})
	eff := r.settled(true)
	rs, _ := find[ReadScreen](eff)
	if !reflect.DeepEqual(rs.Done, []string{"Open the File menu."}) || rs.Again || rs.Goal != "save this file" {
		t.Fatalf("second read %+v", rs)
	}
	if f := r.last(); f.Step != 2 || f.Plan == nil || !reflect.DeepEqual(f.Plan.Done, []string{"Open the File menu."}) {
		t.Fatalf("reading frame %+v", f)
	}
}

func TestALastStepFinishesWithoutAnotherRead(t *testing.T) {
	r := newRig(t, false)
	r.start()
	p := pointing("Click Save.", "Save", 0.3, true)
	p.Done = true
	r.reply(p)
	r.land()
	r.do(Advance{By: ByNext})
	eff := r.settled(true)
	if kinds(eff) != "Retract Show End" || r.reads != 1 {
		t.Fatalf("the last step did %q after %d reads", kinds(eff), r.reads)
	}
	if e := ended(t, r); e.Reason != EndDone || e.Line != lineDone {
		t.Fatalf("end %+v", e)
	}
}

// A read the route may not take ends the guide refused, showing the line
// that says why; a replay that misses falls through to such a read.
func TestABarredReadEndsRefusedWithItsLine(t *testing.T) {
	const line = "Only the app may use that route."
	r := newRig(t, false)
	r.start()
	eff := r.do(Read{Token: r.read, Err: &Barred{Line: line}})
	if kinds(eff) != "Show End" || r.points != 0 {
		t.Fatalf("a barred read did %q", kinds(eff))
	}
	if e := ended(t, r); e.Reason != EndRefused || e.Line != line {
		t.Fatalf("end %+v", e)
	}
	if f := r.last(); f.Phase != feature.GuideDone || f.Text != line {
		t.Fatalf("frame %+v", f)
	}
}

func TestRefusalsAndFailures(t *testing.T) {
	for _, tc := range []struct {
		name   string
		err    error
		reason string
		line   string
	}{
		{"private field", &screenask.Refusal{Code: screenask.CodeSensitive, Line: screenask.RefusalLine}, EndRefused, screenask.RefusalLine},
		{"action", fmt.Errorf("wrapped: %w", &screenask.Refusal{Code: screenask.CodeAction, Line: "I only point."}), EndRefused, "I only point."},
		{"model failure", errors.New("route down"), EndFailed, lineFailed},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := newRig(t, false)
			r.start()
			r.step("Open the File menu.", "File", 0.1, true)
			eff := r.do(Read{Token: r.read, Err: tc.err})
			if kinds(eff) != "Show End" || r.points != 1 {
				t.Fatalf("did %q", kinds(eff))
			}
			if e := ended(t, r); e.Reason != tc.reason || e.Line != tc.line {
				t.Fatalf("end %+v", e)
			}
		})
	}
	r := newRig(t, false)
	if eff := r.do(Start{Goal: "  ", App: "notes"}); kinds(eff) != "Show End" {
		t.Fatalf("a blank goal did %q", kinds(eff))
	}
	if e := ended(t, r); e.Reason != EndFailed || e.Line != lineNoGoal {
		t.Fatalf("end %+v", e)
	}
	r = newRig(t, false)
	r.start()
	r.reply(pointing("Open the File menu.", "File", 0.1, true))
	if eff := r.do(Pointed{Token: r.point, Err: errors.New("no presenter")}); kinds(eff) != "Show End" {
		t.Fatalf("a strand that could not go did %q", kinds(eff))
	}
	if e := ended(t, r); e.Reason != EndLost {
		t.Fatalf("end %+v", e)
	}
}

// runs a guide to its finish and reports whether a keep was offered
func TestKeepIsOfferedOnlyForAWhollyTrustedFinish(t *testing.T) {
	done := screenask.Pointing{Kind: screenask.KindNone, Done: true, Say: "All saved."}
	r := newRig(t, true)
	r.at = t0.Add(time.Minute)
	r.start()
	r.step("Open the File menu.", "File", 0.1, true)
	r.step("Click Save.", "Save", 0.3, true)
	r.reply(done)
	offer, ok := find[OfferKeep](r.all)
	if !ok || r.m.Phase() != feature.GuideKeeping {
		t.Fatalf("a trusted finish offered nothing: %s", kinds(r.all))
	}
	want := keep.Entry{
		Key: keep.KeyFor("notes", "save this file"), App: "notes", Goal: "save this file",
		Steps: []keep.Step{
			{Text: "Open the File menu.", Label: "File", Box: box(0.1), Mode: feature.StrandModeLasso, Trusted: true},
			{Text: "Click Save.", Label: "Save", Box: box(0.3), Mode: feature.StrandModeLasso, Trusted: true},
		},
		Kept: r.at, Used: r.at,
	}
	if !reflect.DeepEqual(offer.Entry, want) {
		t.Fatalf("offered %+v\nwant %+v", offer.Entry, want)
	}
	if f := r.last(); f.Phase != feature.GuideKeeping || !reflect.DeepEqual(f.Actions, []string{feature.GuideActionKeep, feature.GuideActionSkip}) {
		t.Fatalf("keeping frame %+v", f)
	}
	r.do(KeepAnswer{Keep: true})
	if e := ended(t, r); e.Reason != EndDone || e.Line != lineKept {
		t.Fatalf("end %+v", e)
	}

	withheld := map[string]func(r *rig){
		"an untrusted step": func(r *rig) {
			r.step("Open the File menu.", "File", 0.1, false)
			r.reply(done)
		},
		"a refused read": func(r *rig) {
			r.do(Read{Token: r.read, Err: &screenask.Refusal{Code: screenask.CodeLowConfidence, Line: "not sure"}})
			r.do(Advance{By: ByNext})
			r.step("Open the File menu.", "File", 0.1, true)
			r.reply(done)
		},
		"a reply with no point": func(r *rig) {
			r.reply(screenask.Pointing{Kind: screenask.KindNone, Say: "I see no menu."})
			r.do(Advance{By: ByNext})
			r.step("Open the File menu.", "File", 0.1, true)
			r.reply(done)
		},
		"a step forced past an unchanged window": func(r *rig) {
			r.reply(pointing("Open the File menu.", "File", 0.1, true))
			r.land()
			r.do(Advance{By: ByNext})
			r.settled(false)
			r.land()
			r.do(Advance{By: ByNext})
			r.settled(false)
			r.do(Advance{By: ByNext})
			r.reply(done)
		},
		"a step with no label": func(r *rig) {
			r.step("Click there.", "", 0.1, true)
			r.reply(done)
		},
		"a replay that missed": func(r *rig) {
			r.m, r.shows, r.all = New(r.cfg, func() time.Time { return r.at }), nil, nil
			r.do(Start{Goal: "save this file", App: "notes", Entry: kept()})
			r.do(Looked{Token: r.look, Words: words("Export", 0.1)})
			r.step("Open the File menu.", "File", 0.1, true)
			r.step("Click Save.", "Save", 0.3, true)
			r.reply(done)
		},
		"a guide stopped before its end": func(r *rig) {
			r.step("Open the File menu.", "File", 0.1, true)
			r.do(Stop{})
		},
		"a timed-out read": func(r *rig) {
			r.step("Open the File menu.", "File", 0.1, true)
			r.do(Deadline{Token: r.read})
		},
	}
	for name, run := range withheld {
		t.Run(name, func(t *testing.T) {
			r := newRig(t, true)
			r.start()
			run(r)
			if _, ok := find[OfferKeep](r.all); ok {
				t.Fatalf("offered a keep after %s", name)
			}
			ended(t, r)
		})
	}
	for name, mk := range map[string]func(t *testing.T) *rig{
		"keep off": func(t *testing.T) *rig { r := newRig(t, false); r.start(); return r },
		"no app": func(t *testing.T) *rig {
			r := newRig(t, true)
			r.do(Start{Goal: "save this file"})
			return r
		},
	} {
		t.Run(name, func(t *testing.T) {
			r := mk(t)
			r.step("Open the File menu.", "File", 0.1, true)
			r.reply(done)
			if _, ok := find[OfferKeep](r.all); ok {
				t.Fatalf("offered a keep with %s", name)
			}
			if e := ended(t, r); e.Line != "All saved." {
				t.Fatalf("end %+v", e)
			}
		})
	}
}

// kept is a two-step walkthrough of the notes fixture.
func kept() *keep.Entry {
	return &keep.Entry{
		Key: keep.KeyFor("notes", "save this file"), App: "notes", Goal: "save this file",
		Steps: []keep.Step{
			{Text: "Open the File menu.", Label: "File", Box: box(0.1), Mode: feature.StrandModeLasso},
			{Text: "Click Save.", Label: "Save", Box: box(0.3), Mode: feature.StrandModeLasso},
		},
		Kept: t0, Used: t0,
	}
}

// words is one row of read text starting at x, each word a button's
// padding from the next.
func words(text string, x float64) []screenask.Word {
	var out []screenask.Word
	for _, w := range strings.Fields(text) {
		width := 0.01 * float64(len([]rune(w)))
		out = append(out, screenask.Word{Text: w, Box: feature.Box{X: x, Y: 0.1, W: width, H: 0.02}})
		x += width + 0.05
	}
	return out
}

// A replay reads the window here and points with no model read. A label
// still inside its control as kept marks the whole control, as the model
// read it; a control that moved is marked where its label is now.
func TestReplayReadsNoModelUntilAMiss(t *testing.T) {
	r := newRig(t, true)
	eff := r.do(Start{Goal: "how can I save this file", App: "notes", Entry: kept()})
	if kinds(eff) != "Show Look" {
		t.Fatalf("a kept guide started with %q", kinds(eff))
	}
	if f := r.last(); !f.Replay || f.Kicker != kickerReplay {
		t.Fatalf("reading frame %+v", f)
	}
	// File where it was kept; Save moved right, out of its kept box
	screens := [][]screenask.Word{words("File Edit View", 0.12), words("Save Save As…", 0.52)}
	for i, ws := range screens {
		eff := r.do(Looked{Token: r.look, Words: ws})
		p, ok := find[Point](eff)
		if !ok {
			t.Fatalf("step %d did %q", i+1, kinds(eff))
		}
		want := ws[0].Box
		if i == 0 {
			want = kept().Steps[0].Box
		}
		if p.Box != want || p.Mode != feature.StrandModeLasso {
			t.Fatalf("step %d pointed at %+v, want %+v", i+1, p.Box, want)
		}
		r.land()
		if f := r.last(); !f.Replay || f.Kicker != kickerReplay || f.Text != kept().Steps[i].Text {
			t.Fatalf("step %d frame %+v", i+1, f)
		}
		r.do(Advance{By: ByNext})
		r.settled(true)
	}
	if r.reads != 0 || r.looks != 2 {
		t.Fatalf("%d model reads and %d looks, want 0 and 2", r.reads, r.looks)
	}
	if h, ok := find[Hit](r.all); !ok || h.Key != kept().Key {
		t.Fatalf("no hit: %s", kinds(r.all))
	}
	if _, ok := find[OfferKeep](r.all); ok {
		t.Fatal("a replay was offered for keeping again")
	}
	if e := ended(t, r); e.Reason != EndDone || e.Line != lineReplayDone {
		t.Fatalf("end %+v", e)
	}
	if f := r.last(); f.Phase != feature.GuideDone || !f.Replay || f.Text != lineReplayDone || f.Step != 2 {
		t.Fatalf("done frame %+v", f)
	}
}

func TestAReplayMissRecordsItAndFallsThroughToAModelRead(t *testing.T) {
	r := newRig(t, false)
	r.do(Start{Goal: "how can I save this file", App: "notes", Entry: kept()})
	r.do(Looked{Token: r.look, Words: words("File", 0.1)})
	r.land()
	r.do(Advance{By: ByNext})
	r.settled(true)
	// only "Save As…" on the window: a kept "Save" is not it
	eff := r.do(Looked{Token: r.look, Words: []screenask.Word{
		{Text: "Save", Box: feature.Box{X: 0.3, Y: 0.1, W: 0.04, H: 0.02}},
		{Text: "As…", Box: feature.Box{X: 0.346, Y: 0.1, W: 0.03, H: 0.02}},
	}})
	if kinds(eff) != "Miss Show ReadScreen" {
		t.Fatalf("a miss did %q", kinds(eff))
	}
	if m, _ := find[Miss](eff); m.Key != kept().Key {
		t.Fatalf("missed %q", m.Key)
	}
	rs, _ := find[ReadScreen](eff)
	if rs.Goal != "how can I save this file" || !reflect.DeepEqual(rs.Done, []string{"Open the File menu."}) || rs.Again {
		t.Fatalf("fall-through read %+v", rs)
	}
	if f := r.last(); f.Replay || f.Step != 2 || f.Kicker != "Step 2" {
		t.Fatalf("fall-through frame %+v", f)
	}
	if _, ok := find[Point](eff); ok {
		t.Fatal("pointed at Save As")
	}
	if r.reads != 1 {
		t.Fatalf("%d reads", r.reads)
	}
	r.step("Click Save.", "Save", 0.5, true)
	r.reply(screenask.Pointing{Kind: screenask.KindNone, Done: true, Say: "Saved."})
	if _, ok := find[Hit](r.all); ok {
		t.Fatal("a missed replay counted as a hit")
	}
	if e := ended(t, r); e.Line != "Saved." {
		t.Fatalf("end %+v", e)
	}
	// a window that cannot be read is no miss of the entry
	r = newRig(t, false)
	r.do(Start{Goal: "save this file", App: "notes", Entry: kept()})
	eff = r.do(Looked{Token: r.look, Err: errors.New("no text reader")})
	if kinds(eff) != "Show ReadScreen" {
		t.Fatalf("an unreadable window did %q", kinds(eff))
	}
}

func TestAReplayRefusesAControlThatBecameAPrivateField(t *testing.T) {
	r := newRig(t, false)
	r.do(Start{Goal: "save this file", App: "notes", Entry: kept()})
	// the kept File, now under a caption naming a private field
	ws := []screenask.Word{
		{Text: "Card", Box: feature.Box{X: 0.05, Y: 0.07, W: 0.04, H: 0.02}},
		{Text: "number", Box: feature.Box{X: 0.096, Y: 0.07, W: 0.06, H: 0.02}},
		{Text: "File", Box: feature.Box{X: 0.1, Y: 0.1, W: 0.04, H: 0.02}},
	}
	eff := r.do(Looked{Token: r.look, Words: ws})
	if kinds(eff) != "Forget Show End" || r.points != 0 {
		t.Fatalf("a private field did %q", kinds(eff))
	}
	if e := ended(t, r); e.Reason != EndRefused || e.Line != screenask.RefusalLine {
		t.Fatalf("end %+v", e)
	}
}

func TestAKeptRegionMovesWithItsControl(t *testing.T) {
	e := kept()
	region := feature.Box{X: 0.05, Y: 0.05, W: 0.3, H: 0.2}
	e.Steps[0].Mode, e.Steps[0].Region = feature.StrandModeRegion, &region
	r := newRig(t, false)
	r.do(Start{Goal: "save this file", App: "notes", Entry: e})
	ws := words("File", 0.2)
	eff := r.do(Looked{Token: r.look, Words: ws})
	p, _ := find[Point](eff)
	near := func(a, b float64) bool { return math.Abs(a-b) < 1e-9 }
	if p.Mode != feature.StrandModeRegion || p.Region == nil || !near(p.Region.X, 0.15) || !near(p.Region.Y, 0.05) || p.Region.W != region.W {
		t.Fatalf("region point %+v (%+v)", p, p.Region)
	}
	r.land()
	r.do(Circle{})
	if p, _ := find[Point](r.all[len(r.all)-1:]); p.Mode != feature.StrandModeLasso || p.Region != nil {
		t.Fatalf("circle threw %+v", p)
	}
}

func TestFramesStayValidWhateverTheReplySays(t *testing.T) {
	r := newRig(t, false)
	r.start()
	long := strings.Repeat("Click\u200b the\tbutton\n", 40)
	b := box(0.1)
	r.reply(screenask.Pointing{Kind: screenask.KindBox, Box: &b, Label: "File", Conf: 0.9, Sure: true, Verified: true, Say: long,
		Plan: []screenask.PlanItem{{Text: "\u202e"}, {Text: "a"}, {Text: "b"}, {Text: "c"}, {Text: "d"}}})
	r.land()
	f := r.last()
	if len([]rune(f.Text)) > feature.MaxGuideText || strings.ContainsAny(f.Text, "\n\t\u200b") {
		t.Fatalf("text %q", f.Text)
	}
	if f.Plan == nil || len(f.Plan.Next) != r.cfg.PlanNext {
		t.Fatalf("plan %+v", f.Plan)
	}
	r.do(Advance{By: ByNext})
	r.settled(true)
	r.reply(screenask.Pointing{Kind: screenask.KindNone, Say: "\u200b \n"})
	if f := r.last(); f.Text != lineNotHere {
		t.Fatalf("a blank line became %q", f.Text)
	}
	r.do(Stop{})
	// every frame went through feature.EncodeFrame in r.do
	if len(r.shows) < 5 {
		t.Fatalf("only %d frames", len(r.shows))
	}
}

func TestFramesCountOnFromSeqBase(t *testing.T) {
	c := config(t)
	c.SeqBase = 40
	m := New(c, func() time.Time { return t0 })
	eff := m.Handle(Start{Goal: "save this file"})
	s, _ := find[Show](eff)
	if s.Frame.Seq != 41 || m.Seq() != 41 {
		t.Fatalf("first frame seq %d, Seq %d", s.Frame.Seq, m.Seq())
	}
	c.MaxSteps = 99
	if New(c, func() time.Time { return t0 }).cfg.MaxSteps != feature.MaxGuideSteps() {
		t.Fatal("a ceiling past the spec's was kept")
	}
}

func TestConfigComesFromTheSpec(t *testing.T) {
	c := config(t)
	if c.MaxSteps != 12 || c.ReadTimeout != 75*time.Second || c.PlanDone != 6 || c.PlanNext != 3 || c.Keep || c.SeqBase != 0 {
		t.Fatalf("config %+v", c)
	}
}
