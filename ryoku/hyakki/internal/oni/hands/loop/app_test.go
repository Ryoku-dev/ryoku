package loop

import (
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"ryoku-rashin/internal/oni/hands/consent"
	"ryoku-rashin/internal/oni/hands/proto"
)

const notes = "org.gnome.TextEditor"

// The regions the screen reader found in the fixture window.
var regions = []Control{
	{ID: "r1", Subject: consent.Subject{Name: "Next"}},
	{ID: "r2", Subject: consent.Subject{Name: "Send"}},
	{ID: "r3", Subject: consent.Subject{Name: "Password"}},
	{ID: "r4", Subject: consent.Subject{Name: "→"}},
}

func regionName(id string) string {
	for _, c := range regions {
		if c.ID == id {
			return c.Subject.Name
		}
	}
	return ""
}

// arig drives one machine outside a tab. Every Act it sees must answer a
// check that passed, carry at most one chunk, and be sorted safe by the
// app rules.
type arig struct {
	t      *testing.T
	m      *Machine
	now    time.Time
	scope  string
	app    string
	title  string
	read   uint64
	ask    uint64
	verify uint64
	when   string
	act    uint64
	check  uint64
	cardID string
	acts   []Act
	checks []Verify
	reads  []ReadBack
	ends   []End
	all    []Effect
}

func newARig(t *testing.T, scope string) *arig {
	t.Helper()
	r := &arig{t: t, now: time.Date(2026, 11, 23, 12, 0, 0, 0, time.UTC), scope: scope, app: notes, title: "notes.txt"}
	r.m = New(cfg(t), func() time.Time { return r.now })
	return r
}

func (r *arig) do(ev Event) []Effect {
	r.t.Helper()
	effs := r.m.Handle(ev)
	r.all = append(r.all, effs...)
	for _, e := range effs {
		switch e := e.(type) {
		case ReadPage:
			r.read = e.Token
		case Ask:
			r.ask = e.Token
		case Verify:
			r.verify, r.when = e.Token, e.When
			r.checks = append(r.checks, e)
		case Act:
			if v, passed := ev.(Verified); !passed || v.Pause != "" {
				r.t.Fatalf("Act %s answered %T, not a check that passed", e.Tool, ev)
			}
			if n := utf8.RuneCountInString(e.Args.Text); n > 40 {
				r.t.Fatalf("Act carries %d runes", n)
			}
			s := consent.AppSubject{App: r.app, Title: r.title, Near: regionName(e.Args.ID), Text: e.Args.Text, Keys: e.Args.Keys, Name: e.Args.Name}
			if c, why := consent.ClassifyApp(e.Tool, s, consent.ClaimNormal); c != consent.Safe {
				r.t.Fatalf("Act %s, which the rules sort %s (%s)", e.Tool, c, why)
			}
			r.act = e.Token
			r.acts = append(r.acts, e)
		case ReadBack:
			r.check = e.Token
			r.reads = append(r.reads, e)
		case ShowCard:
			r.cardID = e.ID
		case End:
			r.ends = append(r.ends, e)
		}
	}
	return effs
}

func (r *arig) start() []Effect {
	origin := r.app
	if proto.Nested(r.scope) {
		origin = ""
	}
	return r.do(Start{Goal: "write a note", Origin: origin, Scope: r.scope, By: ByButton})
}

func (r *arig) page(print string) []Effect {
	return r.do(PageRead{Token: r.read, Origin: r.app, Title: r.title, Controls: regions, Print: print})
}

func (r *arig) reply(text string) []Effect { return r.do(Reply{Token: r.ask, Text: text}) }
func (r *arig) pass() []Effect             { return r.do(Verified{Token: r.verify}) }
func (r *arig) fail(why string) []Effect   { return r.do(Verified{Token: r.verify, Pause: why}) }
func (r *arig) acted(res Result) []Effect  { return r.do(Acted{Token: r.act, Result: res}) }
func (r *arig) seen(s Seen) []Effect       { return r.do(ReadBackResult{Token: r.check, Seen: s}) }

func typeText(text string) string            { return call(proto.ToolType, map[string]any{"text": text}) }
func press(keys string) string               { return call(proto.ToolPress, map[string]any{"keys": keys}) }
func clickR(id string) string                { return call(proto.ToolClick, map[string]any{"id": id}) }
func showFile(i int) string                  { return call(proto.ToolShowFile, map[string]any{"index": i}) }
func findFiles(q string) string              { return call(proto.ToolFindFiles, map[string]any{"query": q}) }
func openApp(name string) string             { return call(proto.ToolOpenApp, map[string]any{"name": name}) }
func (r *arig) phase() string                { return r.m.Phase() }
func (r *arig) kindsOf(effs []Effect) string { return r.phase() + "|" + kinds(effs) }

func (r *arig) lastEnd() End {
	r.t.Helper()
	if len(r.ends) == 0 {
		r.t.Fatalf("no End; effects %s", kinds(r.all))
	}
	return r.ends[len(r.ends)-1]
}

// typeAll runs one type action through every chunk and its after-checks,
// up to the read-back.
func (r *arig) typeAll(text string) {
	r.t.Helper()
	r.reply(typeText(text))
	for r.phase() == PhaseVerifying {
		r.pass()
		if r.phase() == PhaseActing {
			r.acted(Result{})
		}
	}
	if r.phase() != PhaseChecking {
		r.t.Fatalf("typing ended in %s", r.phase())
	}
}

func TestEveryAppActionIsBracketedByChecks(t *testing.T) {
	text := strings.Repeat("abcdefghij", 9) + "é"
	r := newARig(t, proto.ScopeWindow)
	r.start()
	r.page("p1")
	if got := r.kindsOf(r.reply(typeText(text))); got != "verifying|Frame,Verify" {
		t.Fatalf("a type opens with %q, want the before-check", got)
	}
	if r.when != VerifyBefore {
		t.Fatalf("first check is %q", r.when)
	}
	var seq []string
	for r.phase() == PhaseVerifying {
		seq = append(seq, "verify:"+r.when)
		r.pass()
		if r.phase() == PhaseActing {
			seq = append(seq, "act")
			r.acted(Result{})
		}
	}
	want := []string{"verify:before", "act", "verify:after", "act", "verify:after", "act", "verify:after"}
	if !slices.Equal(seq, want) {
		t.Fatalf("sequence %v, want %v", seq, want)
	}
	var joined string
	for _, a := range r.acts {
		if a.Tool != proto.ToolType || strings.Contains(a.Line, "abc") {
			t.Fatalf("act %+v", a)
		}
		joined += a.Args.Text
	}
	if joined != text || len(r.acts) != 3 {
		t.Fatalf("chunks %d rejoin to %q", len(r.acts), joined)
	}
	if r.phase() != PhaseChecking || len(r.reads) != 1 || r.reads[0].Text != text {
		t.Fatalf("no read-back of the whole text: %s %+v", r.phase(), r.reads)
	}
	if got := r.kindsOf(r.seen(SeenYes)); got != "reading|Frame,ReadPage" {
		t.Fatalf("after a read-back yes: %q", got)
	}

	for _, step := range []string{press("Tab"), clickR("r1"), call(proto.ToolScroll, map[string]any{"dir": "down"})} {
		r.page(fmt.Sprintf("p-%d", len(r.acts)))
		before := len(r.acts)
		if got := r.kindsOf(r.reply(step)); got != "verifying|Frame,Verify" || r.when != VerifyBefore {
			t.Fatalf("%s opens with %q", step, got)
		}
		if got := r.kindsOf(r.pass()); got != "acting|Frame,Act" {
			t.Fatalf("%s after a passing check: %q", step, got)
		}
		if got := r.kindsOf(r.acted(Result{})); got != "verifying|Frame,Verify" || r.when != VerifyAfter {
			t.Fatalf("%s after acting: %q", step, got)
		}
		if got := r.kindsOf(r.pass()); got != "reading|Frame,ReadPage" {
			t.Fatalf("%s after the after-check: %q", step, got)
		}
		if len(r.acts) != before+1 {
			t.Fatalf("%s ran %d times", step, len(r.acts)-before)
		}
	}
	if r.acts[3].Args.Keys != "Tab" || r.acts[4].Line != "Clicking Next." {
		t.Fatalf("acts %+v", r.acts[3:])
	}
	f, _ := find[Frame](r.all)
	if f.Scope != proto.ScopeWindow {
		t.Fatalf("frame scope %q", f.Scope)
	}
}

// A check that fails pauses with its reason and sends no action, whether
// it fails before the first chunk or after one.
func TestAFailedCheckPausesWithItsReason(t *testing.T) {
	reasons := []string{"focus", "title", "geometry", "window", "hidden", "output", "covered", "self", "private", "key", "presenter"}
	for _, why := range reasons {
		for _, when := range []string{VerifyBefore, VerifyAfter} {
			t.Run(why+"/"+when, func(t *testing.T) {
				r := newARig(t, proto.ScopeWindow)
				r.start()
				r.page("p1")
				r.reply(typeText(strings.Repeat("x", 100)))
				if when == VerifyAfter {
					r.pass()
					r.acted(Result{})
				}
				acts := len(r.acts)
				effs := r.fail(why)
				if got := r.kindsOf(effs); got != "paused|Frame,Arm" {
					t.Fatalf("got %q", got)
				}
				if f, _ := find[Frame](effs); f.Pause != why || f.Phase != FramePaused {
					t.Fatalf("frame %+v", f)
				}
				// nothing in flight answers any more, and the button goes on
				// with a fresh look, never the rest of the text
				r.do(Acted{Token: r.act})
				r.do(Verified{Token: r.verify})
				if got := r.kindsOf(r.do(Resume{By: ByButton})); got != "reading|Frame,ReadPage" {
					t.Fatalf("resume: %q", got)
				}
				if len(r.acts) != acts {
					t.Fatalf("an action ran after the pause")
				}
				if when == VerifyAfter && !slices.ContainsFunc(r.m.history, func(h string) bool { return h == fmt.Sprintf(histTypeCut, 1, 3) }) {
					t.Fatalf("history %q lacks the cut", r.m.history)
				}
			})
		}
	}
}

// Any key the person presses, and any discipline signal, pauses a type
// mid-chunk: the chunk is cut, the rest is never typed, and the read-back
// never counts.
func TestAPauseMidChunkDropsTheRest(t *testing.T) {
	text := strings.Repeat("y", 120)
	r := newARig(t, proto.ScopeWindow)
	r.start()
	r.page("p1")
	r.reply(typeText(text))
	r.pass()
	r.acted(Result{})
	r.pass() // the second chunk is in flight
	inFlight := r.act
	effs := r.do(Away{Why: "key"})
	if got := r.kindsOf(effs); got != "paused|Cancel,Frame,Arm" {
		t.Fatalf("got %q", got)
	}
	if c, _ := find[Cancel](effs); c.Token != inFlight {
		t.Fatalf("cancelled %d, want the chunk %d", c.Token, inFlight)
	}
	if !slices.Contains(r.m.history, fmt.Sprintf(histTypeCut, 1, 3)) {
		t.Fatalf("history %q", r.m.history)
	}
	if r.m.typed != "" || len(r.m.chunks) != 0 {
		t.Fatal("the rest of the text is still held")
	}
	if got := r.kindsOf(r.do(Acted{Token: inFlight})); got != "paused|" {
		t.Fatalf("the cut chunk's late answer: %q", got)
	}
	for _, by := range []string{"chord", "wake", "open-mic", "cli", ""} {
		if got := r.kindsOf(r.do(Resume{By: by})); got != "paused|" {
			t.Fatalf("resume by %q: %q", by, got)
		}
	}
	r.do(Resume{By: ByButton})
	r.page("p2")
	if got := r.kindsOf(r.reply(done("Typed."))); got != "reading|Frame,ReadPage" {
		t.Fatalf("done after a cut type must be rejected, got %q", got)
	}
	for _, a := range r.all {
		if rb, isRB := a.(ReadBack); isRB {
			t.Fatalf("a read-back after the cut: %+v", rb)
		}
	}
	if len(r.acts) != 2 {
		t.Fatalf("%d chunks sent, want 2", len(r.acts))
	}
}

func TestAClosedTargetEndsTheTask(t *testing.T) {
	r := newARig(t, proto.ScopeWindow)
	r.start()
	r.page("p1")
	r.reply(press("Tab"))
	r.fail(PauseClosed)
	if e := r.lastEnd(); e.Reason != EndClosed || r.phase() != PhaseEnded {
		t.Fatalf("end %+v in %s", e, r.phase())
	}

	r = newARig(t, proto.ScopeWindow)
	r.start()
	r.page("p1")
	r.reply(press("Tab"))
	r.pass()
	if got := r.kindsOf(r.do(Away{Why: PauseClosed})); got != "ended|Cancel,End" {
		t.Fatalf("away closed: %q", got)
	}
	if r.lastEnd().Reason != EndClosed {
		t.Fatal("not closed")
	}
}

func TestClosedTargetEndsPausedAndWaitingTasks(t *testing.T) {
	for _, paused := range []bool{false, true} {
		t.Run(fmt.Sprintf("paused=%t", paused), func(t *testing.T) {
			r := newARig(t, proto.ScopeWindow)
			r.start()
			r.page("p1")
			r.reply(press("Return"))
			if r.phase() != PhaseWaiting {
				t.Fatalf("handoff phase = %s", r.phase())
			}
			if paused {
				r.do(Away{Why: PauseFocus})
			}
			r.do(Away{Why: PauseClosed})
			if r.phase() != PhaseEnded || r.lastEnd().Reason != EndClosed {
				t.Fatalf("closed target: phase %s, end %+v", r.phase(), r.ends)
			}
			if effs := r.do(Resume{By: ByButton}); len(effs) != 0 {
				t.Fatalf("a closed target resumed: %s", kinds(effs))
			}
		})
	}
}

// Read-back: Unknown never passes, and a second one ends the task.
func TestReadBackUnknownNeverPasses(t *testing.T) {
	r := newARig(t, proto.ScopeWindow)
	r.start()
	r.page("p1")
	r.typeAll("hello")
	if got := r.kindsOf(r.seen(SeenUnknown)); got != "reading|Frame,ReadPage" {
		t.Fatalf("first unknown: %q", got)
	}
	if !slices.Contains(r.m.history, histTypeUnknown) {
		t.Fatalf("history %q", r.m.history)
	}
	r.page("p2")
	if got := r.kindsOf(r.reply(done("Typed hello."))); got != "reading|Frame,ReadPage" {
		t.Fatalf("done after an unknown read-back: %q", got)
	}
	r.page("p3")
	r.typeAll("hello")
	r.seen(SeenUnknown)
	if e := r.lastEnd(); e.Reason != EndUnread || strings.Contains(e.Line, "hello") {
		t.Fatalf("second unknown: %+v", e)
	}

	// the zero value of a read-back is Unknown
	if (ReadBackResult{}).Seen != SeenUnknown {
		t.Fatal("the zero read-back must be unknown")
	}

	r = newARig(t, proto.ScopeWindow)
	r.start()
	r.page("p1")
	r.typeAll("hello")
	r.seen(SeenNo)
	r.page("p2")
	if got := r.kindsOf(r.reply(done("Typed hello."))); got != "reading|Frame,ReadPage" {
		t.Fatalf("done after a read-back no: %q", got)
	}
	r.page("p3")
	r.typeAll("hello")
	r.seen(SeenYes)
	r.page("p4")
	r.reply(done("Typed hello."))
	if e := r.lastEnd(); e.Reason != EndDone {
		t.Fatalf("done after a read-back yes: %+v", e)
	}
}

func TestAppHandoffsAndRefusalsRunNothing(t *testing.T) {
	cases := []struct {
		name   string
		reply  string
		card   bool
		reason string
	}{
		{"enter", press("Enter"), true, "submits"},
		{"ctrl+enter", press("ctrl+Return"), true, "submits"},
		{"delete", press("Delete"), true, "risky-key"},
		{"close", press("ctrl+w"), true, "risky-key"},
		{"alt+f4", press("alt+F4"), true, "risky-key"},
		{"multiline", typeText("dear Ada\nhello"), true, "submits"},
		{"send", clickR("r2"), true, "risky-label"},
		{"icon", clickR("r4"), true, "unreadable-label"},
		{"paste", press("ctrl+v"), false, "refused-key"},
		{"super", press("super+d"), false, "refused-key"},
		{"secret", typeText("4111 1111 1111 1111"), false, "secret-text"},
		{"password field", clickR("r3"), false, "private-field"},
		{"too long", typeText(strings.Repeat("z", 201)), false, "text-too-long"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := newARig(t, proto.ScopeWindow)
			r.start()
			r.page("p1")
			effs := r.reply(tc.reply)
			if card, shown := find[ShowCard](effs); shown != tc.card {
				t.Fatalf("card %v: %s", shown, kinds(effs))
			} else if shown && (strings.Contains(card.Line, "Ada") || strings.Contains(card.Line, "hello")) {
				t.Fatalf("the card holds typed words: %q", card.Line)
			} else if shown && string(card.Reason) != tc.reason {
				t.Fatalf("handoff reason %q, want %q", card.Reason, tc.reason)
			}
			if _, checked := find[Verify](effs); checked {
				t.Fatal("a check for a step that does not run")
			}
			if len(r.acts) != 0 {
				t.Fatal("an action ran")
			}
			if !tc.card {
				refusal, ok := find[Refused](effs)
				if !ok || refusal.Reason != tc.reason {
					t.Fatalf("refusal %+v, present %v, want %q", refusal, ok, tc.reason)
				}
			}
			for _, h := range r.m.history {
				if strings.Contains(h, "Ada") || strings.Contains(h, "4111") {
					t.Fatalf("history holds typed words: %q", h)
				}
			}
		})
	}
}

func TestPrivateWindowsAreNeverActedIn(t *testing.T) {
	r := newARig(t, proto.ScopeWindow)
	r.title = "Unlock: enter your password"
	r.start()
	r.page("p1")
	r.reply(press("Tab"))
	if refusal, ok := find[Refused](r.all); len(r.acts) != 0 || !ok || refusal.Reason != "private-field" {
		t.Fatalf("refusal %+v, present %v, acts %d", refusal, ok, len(r.acts))
	}

	r = newARig(t, proto.ScopeNested)
	r.app = "org.keepassxc.KeePassXC"
	r.start()
	r.page("p1")
	r.reply(typeText("hello"))
	if refusal, ok := find[Refused](r.all); len(r.acts) != 0 || !ok || refusal.Reason != "private-app" {
		t.Fatalf("refusal %+v, present %v, acts %d", refusal, ok, len(r.acts))
	}

	r = newARig(t, proto.ScopeNested)
	r.app = "dev.ryoku.Hyakki"
	r.start()
	r.page("p1")
	r.reply(clickR("r1"))
	if refusal, ok := find[Refused](r.all); len(r.acts) != 0 || !ok || refusal.Reason != "self-surface" {
		t.Fatalf("refusal %+v, present %v, acts %d", refusal, ok, len(r.acts))
	}

	r = newARig(t, proto.ScopeWindow)
	r.start()
	r.do(PageRead{Token: r.read, Origin: notes, Title: "x", Controls: regions, Print: "p1", Private: true})
	r.reply(typeText("hello"))
	if refusal, ok := find[Refused](r.all); len(r.acts) != 0 || !ok || refusal.Reason != "private-field" {
		t.Fatalf("refusal %+v, present %v, acts %d", refusal, ok, len(r.acts))
	}

	// another app read in the window scope is not the picked window
	r = newARig(t, proto.ScopeWindow)
	r.start()
	if got := r.kindsOf(r.do(PageRead{Token: r.read, Origin: "firefox", Controls: regions, Print: "p1"})); got != "paused|Frame,Arm" {
		t.Fatalf("foreign window read: %q", got)
	}
	if f, _ := find[Frame](r.all[len(r.all)-2:]); f.Pause != "window" {
		t.Fatalf("pause %+v", f)
	}
}

func TestNestedToolsAndFilesShownNotRun(t *testing.T) {
	r := newARig(t, proto.ScopeNested)
	r.start()
	r.page("p1")
	r.reply(findFiles("report"))
	r.pass()
	if a := r.acts[len(r.acts)-1]; a.Tool != proto.ToolFindFiles || a.Args.Query != "report" {
		t.Fatalf("act %+v", a)
	}
	r.acted(Result{Found: 2})
	r.pass()
	if !slices.Contains(r.m.history, fmt.Sprintf(histFound, 2)) {
		t.Fatalf("history %q", r.m.history)
	}
	r.page("p2")
	// a file past the list is refused by the reply contract
	if got := r.kindsOf(r.reply(showFile(3))); got != "thinking|Frame,Ask" {
		t.Fatalf("show_file 3 of 2: %q", got)
	}
	r.reply(showFile(2))
	r.pass()
	a := r.acts[len(r.acts)-1]
	if a.Tool != proto.ToolShowFile || a.Args.Index != 2 || a.Line != "Showing the folder it is in." {
		t.Fatalf("act %+v", a)
	}
	r.acted(Result{})
	r.pass()
	r.page("p3")
	r.reply(openApp("Text Editor"))
	r.pass()
	if a := r.acts[len(r.acts)-1]; a.Tool != proto.ToolOpenApp || a.Line != "Opening Text Editor." {
		t.Fatalf("act %+v", a)
	}

	// the live window offers none of these
	w := newARig(t, proto.ScopeWindow)
	w.start()
	w.page("p1")
	for _, raw := range []string{openApp("Files"), findFiles("x")} {
		if got := w.kindsOf(w.reply(raw)); got != "thinking|Frame,Ask" && got != "ended|End" {
			t.Fatalf("%s in a window: %q", raw, got)
		}
	}
	if len(w.acts) != 0 {
		t.Fatal("a window ran a desktop tool")
	}
	if e := w.lastEnd(); e.Reason != EndRefused {
		t.Fatalf("end %+v", e)
	}
}

func TestAFailedActionIsCheckedAfterAndNotRetried(t *testing.T) {
	r := newARig(t, proto.ScopeWindow)
	r.start()
	r.page("p1")
	r.reply(typeText(strings.Repeat("q", 90)))
	r.pass()
	if got := r.kindsOf(r.acted(Result{Err: "tool-failed"})); got != "verifying|Frame,Verify" || r.when != VerifyAfter {
		t.Fatalf("after a failed chunk: %q", got)
	}
	if got := r.kindsOf(r.pass()); got != "reading|Frame,ReadPage" {
		t.Fatalf("after its check: %q", got)
	}
	if len(r.acts) != 1 || len(r.reads) != 0 {
		t.Fatalf("%d acts, %d read-backs after a failed chunk", len(r.acts), len(r.reads))
	}
	if !slices.Contains(r.m.history, "type window: did not run (tool-failed)") {
		t.Fatalf("history %q", r.m.history)
	}
}

func TestChecksAndReadBacksTimeOut(t *testing.T) {
	r := newARig(t, proto.ScopeWindow)
	r.start()
	r.page("p1")
	r.reply(press("Tab"))
	if c := r.checks[len(r.checks)-1]; !c.Until.Equal(r.now.Add(r.m.cfg.VerifyTimeout)) {
		t.Fatalf("check until %v", c.Until)
	}
	r.do(Deadline{Token: r.verify})
	if e := r.lastEnd(); e.Reason != EndTimeout {
		t.Fatalf("end %+v", e)
	}

	r = newARig(t, proto.ScopeWindow)
	r.start()
	r.page("p1")
	r.typeAll("hi")
	if rb := r.reads[0]; !rb.Until.Equal(r.now.Add(r.m.cfg.ReadbackTimeout)) {
		t.Fatalf("read-back until %v", rb.Until)
	}
	r.do(Deadline{Token: r.check})
	if e := r.lastEnd(); e.Reason != EndTimeout {
		t.Fatalf("end %+v", e)
	}
}

// After a restart nothing resumes: a held machine answers only the Start
// button, with the goal said again.
func TestNothingResumesAfterRestart(t *testing.T) {
	r := newARig(t, proto.ScopeWindow)
	if got := r.kindsOf(r.do(Held{})); got != "held|" {
		t.Fatalf("held: %q", got)
	}
	for _, ev := range []Event{
		Resume{By: ByButton}, Resume{By: "chord"}, Away{Why: "key"}, Verified{Token: 1}, ReadBackResult{Token: 1, Seen: SeenYes},
		Acted{Token: 1}, PageRead{Token: 1, Origin: notes}, Reply{Token: 1, Text: press("Tab")}, Deadline{Token: 1}, UserDid{ID: "h-1"},
	} {
		if got := r.kindsOf(r.do(ev)); got != "held|" {
			t.Fatalf("%T while held: %q", ev, got)
		}
	}
	for _, by := range []string{"", "chord", "toggle", "wake", "open-mic", "cli", "harness"} {
		if got := r.kindsOf(r.do(Start{Goal: "write a note", Origin: notes, Scope: proto.ScopeWindow, By: by})); got != "held|" {
			t.Fatalf("start by %q while held: %q", by, got)
		}
	}
	if got := r.kindsOf(r.do(Start{Goal: "write a note", Origin: notes, Scope: proto.ScopeWindow, By: ByButton})); got != "reading|Frame,ReadPage" {
		t.Fatalf("start by the button: %q", got)
	}
	if got := r.kindsOf(r.do(Held{})); got != "reading|" {
		t.Fatalf("held while running: %q", got)
	}

	s := newARig(t, proto.ScopeNested)
	s.do(Held{})
	s.do(Stop{Why: "esc"})
	if s.phase() != PhaseStopped {
		t.Fatalf("stop while held: %s", s.phase())
	}
}

func TestStopDuringAnAppActionEndsEverything(t *testing.T) {
	for _, at := range []string{PhaseVerifying, PhaseActing, PhaseChecking} {
		t.Run(at, func(t *testing.T) {
			r := newARig(t, proto.ScopeWindow)
			r.start()
			r.page("p1")
			switch at {
			case PhaseVerifying:
				r.reply(typeText("hello"))
			case PhaseActing:
				r.reply(typeText("hello"))
				r.pass()
			case PhaseChecking:
				r.typeAll("hello")
			}
			if r.phase() != at {
				t.Fatalf("setup in %s", r.phase())
			}
			if got := r.kindsOf(r.do(Stop{Why: "esc"})); got != "stopped|Cancel,End" {
				t.Fatalf("stop: %q", got)
			}
			if r.m.typed != "" || len(r.m.chunks) != 0 {
				t.Fatal("text still held after stop")
			}
			for _, ev := range []Event{Stop{Why: "esc"}, Verified{Token: r.verify}, Acted{Token: r.act}, ReadBackResult{Token: r.check, Seen: SeenYes}, Resume{By: ByButton}} {
				if effs := r.do(ev); len(effs) != 0 {
					t.Fatalf("%T after stop: %s", ev, kinds(effs))
				}
			}
		})
	}
}

func TestAnAppTaskNeedsAKnownScopeAndAWindow(t *testing.T) {
	now := func() time.Time { return time.Time{} }
	m := New(Config{}, now)
	effs := m.Handle(Start{Goal: "x", Scope: proto.ScopeWindow})
	if e, _ := find[End](effs); e.Reason != EndFailed || e.Line != lineNoWindow {
		t.Fatalf("no window: %+v", e)
	}
	m = New(Config{}, now)
	if e, _ := find[End](m.Handle(Start{Goal: "x", Origin: notes, Scope: "whole"})); e.Reason != EndFailed {
		t.Fatalf("unknown scope: %+v", e)
	}
	m = New(Config{}, now)
	if got := kinds(m.Handle(Start{Goal: "x", Scope: proto.ScopeDesktop})); got != "Frame,ReadPage" {
		t.Fatalf("the nested desktop needs no origin: %q", got)
	}
}
