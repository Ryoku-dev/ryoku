package surface

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/design"

	"ryoku-rashin/internal/oni/feature"
	"ryoku-rashin/internal/oni/render"
	"ryoku-rashin/internal/protocol"
)

const handsTestApp = "org.example.Editor"

// windowFrameAt is a task in one live window of handsTestApp on output.
func windowFrameAt(phase, output string) *feature.HandsFrame {
	f := &feature.HandsFrame{Stage: protocol.OniHandsApp, Active: true, Reason: feature.ReasonAllowed, Scope: protocol.OniHandsScopeWindow,
		App: handsTestApp, Output: output, Phase: phase, Step: 2, Max: 25}
	if phase == protocol.OniHandsStatePaused {
		f.Pause = protocol.OniHandsPauseKey
	}
	return f
}

// desktopFrameAt is a task on Hyakki's separate desktop.
func desktopFrameAt(phase string) *feature.HandsFrame {
	f := &feature.HandsFrame{Stage: protocol.OniHandsDesktop, Active: true, Reason: feature.ReasonAllowed, Scope: protocol.OniHandsScopeDesktop,
		Phase: phase, Step: 1, Max: 25}
	if phase == protocol.OniHandsStatePaused {
		f.Pause = protocol.OniHandsPausePresenter
	}
	return f
}

// A task in a live window glows round the output that window is on and no
// other, masks only those strips, says on the pill which app it works in
// by its id, follows the window to another output, and goes at once with
// the inactive frame.
func TestAWindowTaskGlowsOnlyOnItsWindowsOutput(t *testing.T) {
	h, d, fd, buf := handsHostOn(t, design.Lithic)
	send(t, h, windowFrameAt(protocol.OniHandsStateWorking, "B"))
	if len(glowsOn(d, "B")) != 1 || !d.glowOn["B"] {
		t.Fatalf("B's glow: %d commits, mapped %v", len(glowsOn(d, "B")), d.glowOn["B"])
	}
	if len(glowsOn(d, "A")) != 0 || d.glowOn["A"] || h.glowRects("A") != nil {
		t.Fatal("a window on B glows on A too")
	}
	settleHands(t, h, time.Now())
	if k := h.pill.lay; k.Kicker.Text != "In "+handsTestApp || len(k.Lines) != 1 {
		t.Fatalf("the pill says %q over %q", k.Kicker.Text, labels(k.Lines))
	}
	d.drain(t, func() bool { _, _, due := h.overlayDue(); return !h.ovBusy && !due })
	latest := map[string][]protocol.OniRect{}
	for _, c := range fd.overlayCalls() {
		latest[c.Output] = c.Rects
	}
	if len(latest["B"]) != render.GlowStrips {
		t.Fatalf("B masks %+v, want its four strips", latest["B"])
	}
	for _, r := range latest["A"] {
		if r.W >= 1900 || r.H >= 1000 {
			t.Fatalf("A masks a strip-sized rect %+v though it has no glow", r)
		}
	}

	// the window went to A: the glow follows it, B's goes
	send(t, h, windowFrameAt(protocol.OniHandsStateWorking, "A"))
	if !d.glowOn["A"] || d.glowOn["B"] {
		t.Fatalf("after the move the glow is on %v", d.glowOn)
	}
	// an output this presenter does not know: every output, so the task
	// never runs unseen
	send(t, h, windowFrameAt(protocol.OniHandsStateWorking, "C"))
	if !d.glowOn["A"] || !d.glowOn["B"] {
		t.Fatalf("on an unknown output the glow is on %v, want every output", d.glowOn)
	}
	lines := ledgerLines(t, buf, EvHands)
	if len(lines) == 0 || lines[0].Scope != protocol.OniHandsScopeWindow || lines[0].Output != "B" {
		t.Fatalf("the hands lines are %+v", lines)
	}
	if strings.Contains(buf.String(), "In "+handsTestApp) {
		t.Fatal("the ledger carries the pill's words")
	}
	send(t, h, handsOffAs(protocol.OniHandsApp))
	if len(d.glowOn) != 0 || h.glow.timer != nil || d.ovMapped[overlayPill] {
		t.Fatalf("after the task: glow %v, pill mapped %v", d.glowOn, d.ovMapped[overlayPill])
	}
}

func handsOffAs(stage string) *feature.HandsFrame {
	return &feature.HandsFrame{Stage: stage, Reason: feature.ReasonAllowed, End: protocol.OniHandsEndStopped}
}

// A task on Hyakki's separate desktop puts no glow on the person's screens
// and arms no glow timer, since nothing of theirs is driven; the pill says
// where it works, offers Stop, and Stop sends oni.hands stop. A handoff
// from there says, whole, that Hands never presses it and Stop skips it.
func TestTheSeparateDesktopGlowsNowhereAndStillShowsThePill(t *testing.T) {
	for _, f := range []*feature.HandsFrame{
		desktopFrameAt(protocol.OniHandsStateWorking),
		{Stage: protocol.OniHandsApp, Active: true, Reason: feature.ReasonAllowed, Scope: protocol.OniHandsScopeNested, Phase: protocol.OniHandsStateWorking, Step: 1, Max: 25},
	} {
		t.Run(f.Scope, func(t *testing.T) {
			h, d, fd, buf := handsHostOn(t, design.Material)
			send(t, h, f)
			now := settleHands(t, h, time.Now())
			now = drive(h, now, 20)
			if len(d.glows) != 0 || len(d.glowOn) != 0 || h.glow.timer != nil || h.glow.ticking {
				t.Fatalf("the separate desktop's task glowed: %d commits on %v, timer %v", len(d.glows), d.glowOn, h.glow.timer != nil)
			}
			if !d.ovMapped[overlayPill] || h.pill.lay.Kicker.Text != "On Hyakki's separate desktop" {
				t.Fatalf("the pill mapped %v says %q", d.ovMapped[overlayPill], h.pill.lay.Kicker.Text)
			}
			if got := partActions(&h.pill); !slices.Equal(got, []string{handsStop}) {
				t.Fatalf("the pill offers %v", got)
			}
			waiting := *f
			waiting.Phase = protocol.OniHandsStateWaiting
			send(t, h, &waiting)
			feat := feature.HandsDesktop
			if f.Stage == protocol.OniHandsApp {
				feat = feature.HandsApp
			}
			card := feature.HandoffCard(feat, "h-9", handoffLine, time.Minute)
			send(t, h, card)
			now = settleHands(t, h, now)
			if !d.ovMapped[overlayHandoff] || h.handoff.lay.Hint.Text != "Hands never presses it there. Stop to skip it." {
				t.Fatalf("the handoff card mapped %v hints %q", d.ovMapped[overlayHandoff], h.handoff.lay.Hint.Text)
			}
			if len(d.glows) != 0 {
				t.Fatal("a waiting card on the separate desktop glowed")
			}
			pressHands(t, h, overlayPill, handsStop)
			settleCalls(t, fd, func() bool { return len(fd.handsCalls()) == 1 })
			if got := fd.handsCalls(); !slices.Equal(got, []string{protocol.OniHandsStop}) {
				t.Fatalf("hands calls = %v", got)
			}
			for _, e := range ledgerLines(t, buf, EvCommit) {
				if e.Part == PartGlow {
					t.Fatalf("a glow commit line %+v", e)
				}
			}
			drive(h, now, 3)
		})
	}
}

// Every pause a live window or the separate desktop can give says its own
// rule on the pill: none falls back to the bare "paused", no two say the
// same in one scope, a key is only ever "a key", and the browser's lines
// are what they were.
func TestEveryAppPauseSaysItsRule(t *testing.T) {
	for _, scope := range []string{protocol.OniHandsScopeWindow, protocol.OniHandsScopeNested, protocol.OniHandsScopeDesktop} {
		seen := map[string]string{}
		for _, p := range protocol.OniHandsAppPauses {
			line := pauseLine(scope, p)
			if line == "Hands is paused" || !strings.HasPrefix(line, "Paused: ") {
				t.Errorf("%s/%s says %q", scope, p, line)
			}
			if other, ok := seen[line]; ok {
				t.Errorf("%s: %s and %s both say %q", scope, other, p, line)
			}
			seen[line] = p
		}
	}
	if got := pauseLine(protocol.OniHandsScopeWindow, protocol.OniHandsPauseKey); got != "Paused: you pressed a key" {
		t.Fatalf("a key pause says %q", got)
	}
	if got := pauseLine(protocol.OniHandsScopeBrowser, protocol.OniHandsPauseHidden); got != "Paused: the tab is out of view" {
		t.Fatalf("the browser's hidden pause says %q", got)
	}
	if got := pauseLine(protocol.OniHandsScopeWindow, protocol.OniHandsPauseHidden); got != "Paused: the window is out of view" {
		t.Fatalf("a window's hidden pause says %q", got)
	}
	if got := pauseLine(protocol.OniHandsScopeWindow, "nonsense"); got != "Hands is paused" {
		t.Fatalf("an unknown pause says %q", got)
	}
	// a paused window task offers Resume and names the rule
	h, _, _, _ := handsHostOn(t, design.Adwaita)
	send(t, h, windowFrameAt(protocol.OniHandsStatePaused, "A"))
	settleHands(t, h, time.Now())
	if got := partActions(&h.pill); !slices.Equal(got, []string{handsStop, handsResume}) {
		t.Fatalf("a paused window pill offers %v", got)
	}
	if got := labels(h.pill.lay.Lines); len(got) != 1 || got[0] != "Paused: you pressed a key" {
		t.Fatalf("a paused window pill says %q", got)
	}
}

func (d *fakeDaemon) answerCalls() []protocol.OniConsentAnswerParams {
	d.mu.Lock()
	defer d.mu.Unlock()
	return slices.Clone(d.answers)
}

// An approval card shows in the handoff card's place with the frame's
// buttons in order, answers once with the option pressed, queues behind
// the one showing, gives way to a task's own handoff card and comes back,
// outlives the task's end, offers only the app when the pill may not
// answer, and keeps its words off the ledger.
func TestApprovalCardsAnswerOnceQueueAndOutliveTheTask(t *testing.T) {
	h, d, fd, buf := handsHostOn(t, design.Lithic)
	const title = "Ada's researcher wants Hands to sort the reports folder"
	send(t, h, &feature.ApprovalFrame{ID: "ta-1", Open: true, Title: title, Scope: protocol.OniHandsScopeNested,
		Options: []string{feature.ApprovalOnce, feature.ApprovalNo, feature.ApprovalInApp}})
	send(t, h, &feature.ApprovalFrame{ID: "tp-2", Open: true, Title: "Run the deploy script", Options: []string{feature.ApprovalInApp}})
	now := settleHands(t, h, time.Now())
	if !d.ovMapped[overlayHandoff] || d.ovMapped[overlayPill] {
		t.Fatalf("with no task: card mapped %v, pill mapped %v", d.ovMapped[overlayHandoff], d.ovMapped[overlayPill])
	}
	if got := partActions(&h.handoff); !slices.Equal(got, []string{feature.ApprovalOnce, feature.ApprovalNo, feature.ApprovalInApp}) {
		t.Fatalf("the first card offers %v", got)
	}
	if k := h.handoff.lay; k.Kicker.Text != "Hands for a team member" || k.Hint.Text != "On Hyakki's separate desktop, once." {
		t.Fatalf("the card says %q / %q", k.Kicker.Text, k.Hint.Text)
	}
	var buttons []ButtonInfo
	for _, e := range ledgerLines(t, buf, EvCommit) {
		if e.Part == PartApproval {
			buttons = e.Buttons
		}
	}
	if len(buttons) != 3 || buttons[0].Action != feature.ApprovalOnce {
		t.Fatalf("the approval's commit line names %+v", buttons)
	}
	if strings.Contains(buf.String(), "Ada") || strings.Contains(buf.String(), "deploy") {
		t.Fatal("the ledger carries an approval's words")
	}

	pressHands(t, h, overlayHandoff, feature.ApprovalOnce)
	pressHands(t, h, overlayHandoff, feature.ApprovalNo)
	settleCalls(t, fd, func() bool { return len(fd.answerCalls()) == 1 })
	if got := fd.answerCalls(); !slices.Equal(got, []protocol.OniConsentAnswerParams{{ID: "ta-1", Answer: feature.ApprovalOnce}}) {
		t.Fatalf("answers = %+v", got)
	}
	if len(fd.handsCalls()) != 0 {
		t.Fatal("an approval press made an oni.hands call")
	}

	// the daemon closes it: the next waits no more
	send(t, h, &feature.ApprovalFrame{ID: "ta-1"})
	now = settleHands(t, h, now)
	if got := partActions(&h.handoff); !slices.Equal(got, []string{feature.ApprovalInApp}) || h.handoff.lay.Hint.Text != "Only the Hyakki app can answer this one." {
		t.Fatalf("the danger card offers %v and says %q", got, h.handoff.lay.Hint.Text)
	}

	// a task's handoff takes the place; once pressed the approval is back
	send(t, h, windowFrameAt(protocol.OniHandsStateWaiting, "A"))
	send(t, h, feature.HandoffCard(feature.HandsApp, "h-3", handoffLine, time.Minute))
	now = settleHands(t, h, now)
	if got := partActions(&h.handoff); !slices.Equal(got, []string{handsNotNow}) {
		t.Fatalf("with a handoff waiting the card offers %v", got)
	}
	send(t, h, windowFrameAt(protocol.OniHandsStateWorking, "A"))
	now = settleHands(t, h, now)
	if got := partActions(&h.handoff); !slices.Equal(got, []string{feature.ApprovalInApp}) {
		t.Fatalf("after the handoff the card offers %v", got)
	}

	// the task ends: the pill goes, the approval stays without a hide
	buf.Reset()
	send(t, h, handsOffAs(protocol.OniHandsApp))
	now = settleHands(t, h, now)
	if d.ovMapped[overlayPill] || !d.ovMapped[overlayHandoff] {
		t.Fatalf("after the task: pill mapped %v, card mapped %v", d.ovMapped[overlayPill], d.ovMapped[overlayHandoff])
	}
	for _, e := range ledgerLines(t, buf, EvHide) {
		if e.Part == PartApproval {
			t.Fatalf("the task's end hid the approval: %+v", e)
		}
	}
	send(t, h, &feature.ApprovalFrame{ID: "tp-2"})
	settleHands(t, h, now)
	drive(h, now, 30)
	if d.ovMapped[overlayHandoff] {
		t.Fatal("the last approval closed and its card stays")
	}
	for _, word := range []string{"Ada", "reports", "deploy"} {
		if strings.Contains(buf.String(), word) {
			t.Fatalf("the ledger carries the card's words (%q)", word)
		}
	}
}
