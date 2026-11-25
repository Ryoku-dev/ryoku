package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"ryoku-rashin/internal/llm"
	"ryoku-rashin/internal/oni/feature"
	"ryoku-rashin/internal/oni/keep"
	"ryoku-rashin/internal/oni/window"
	"ryoku-rashin/internal/protocol"
)

// These tests drive proactive hints over the daemon's own socket, with the
// desktop, grim, tesseract and the model faked and the hint clock the
// test's.

// hintRig is a serving daemon with the guide and Keep on, hints as on
// says, the given walkthroughs kept, and a presenter drawing Oni, not yet
// perched.
func hintRig(t *testing.T, on bool, kept ...keep.Entry) (r *wireRig, surface, cli *socketPeer) {
	t.Helper()
	return hintRigOn(t, localEyes, on, kept...)
}

// hintRigOn is hintRig with plan the route the gateway plans.
func hintRigOn(t *testing.T, plan llm.Plan, on bool, kept ...keep.Entry) (r *wireRig, surface, cli *socketPeer) {
	t.Helper()
	r = serveWire(t, true, plan)
	r.hintsOn(t, on)
	r.tess.set(fileWord, saveWord)
	for _, e := range kept {
		keepEntry(t, e)
	}
	surface, _ = r.presenting(t)
	cli = dialSocket(t, protocol.ClientCLI)
	return r, surface, cli
}

func (r *wireRig) hintsOn(t *testing.T, on bool) {
	t.Helper()
	patch := fmt.Sprintf(`{"oni":{"guide":{"enabled":true,"keep":true,"hints":%t}}}`, on)
	if _, err := r.d.applySettings(context.Background(), json.RawMessage(patch)); err != nil {
		t.Fatal(err)
	}
}

func keptFor(app, goal string) keep.Entry {
	return keep.Entry{App: app, Goal: goal, Steps: []keep.Step{keptFile, keptSave}}
}

func keepEntry(t *testing.T, e keep.Entry) {
	t.Helper()
	if err := keep.NewStore("").Put(e); err != nil {
		t.Fatal(err)
	}
}

// perchAgain perches Oni on the floating window id, as the user dropping
// it there again.
func (r *wireRig) perchAgain(t *testing.T, cli *socketPeer, id string) {
	t.Helper()
	perchOn(t, cli, id)
	r.desk.await(t, 1, window.ID(id))
}

func isHint(p feature.Payload) bool {
	g, ok := p.(*feature.GuideFrame)
	return ok && g.Phase == feature.GuideNudging
}

func isHintOffer(p feature.Payload) bool {
	c, ok := p.(*feature.ConsentFrame)
	return ok && c.Feature == feature.GuideHints
}

func answerTo(id string) func(feature.Payload) bool {
	return func(p feature.Payload) bool {
		a, ok := p.(*feature.ConsentAnswerFrame)
		return ok && a.ID == id
	}
}

// hint waits for a hint and its offer, and checks both.
func (p *socketPeer) hint() *feature.ConsentFrame {
	p.t.Helper()
	g := p.await("the hint", isHint).(*feature.GuideFrame)
	if g.Step != 0 || g.Kicker != oniHintKicker || g.Text != oniHintLine || len(g.Actions) != 2 || g.Actions[0] != feature.GuideActionNext || g.Actions[1] != feature.GuideActionSkip || g.Target != 0 || g.Plan != nil {
		p.t.Fatalf("hint card = %+v", g)
	}
	c := p.await("the hint's offer", isHintOffer).(*feature.ConsentFrame)
	if c.ID == "" || c.Class != feature.ConsentAnnounced || c.Destination != feature.EgressLocal || c.Route != "" || c.ExpiresMs != feature.HintOfferTTL().Milliseconds() {
		p.t.Fatalf("hint offer = %+v", c)
	}
	return c
}

// noHint reads the frames for a moment and fails on any hint.
func (p *socketPeer) noHint() {
	p.t.Helper()
	timeout := time.After(300 * time.Millisecond)
	for {
		select {
		case e := <-p.conn.Events():
			if f, err := feature.DecodeFrame(e.Data); err == nil && (isHint(f) || isHintOffer(f)) {
				p.t.Fatalf("a hint went out: %s", e.Data)
			}
		case <-timeout:
			return
		}
	}
}

// gone waits for the hint id to be answered no and the card to go.
func (p *socketPeer) gone(id string) {
	p.t.Helper()
	if a := p.await("the hint's answer", answerTo(id)).(*feature.ConsentAnswerFrame); a.Answer != "no" {
		p.t.Fatalf("hint answer = %+v", a)
	}
	p.await("the card going", guidePhase(feature.GuideIdle, 0))
}

// With hints off nothing is armed and nothing shown, whatever was kept;
// with them on, no hint comes before a perch, nor for an app with nothing
// kept.
func TestOniHintsOffArmNothingAndShowNothing(t *testing.T) {
	t.Run("off", func(t *testing.T) {
		r, surface, cli := hintRig(t, false, keptFor("editor", "save this file"))
		r.perchAgain(t, cli, "0x1")
		surface.noHint()
		r.clock.Advance(keep.HintCooldown)
		r.perchAgain(t, cli, "0x1")
		surface.noHint()
		if n := r.clock.Armed(); n != 0 {
			t.Fatalf("%d timers armed with hints off", n)
		}
		if _, err := os.Stat(keep.DefaultHintsPath()); !os.IsNotExist(err) {
			t.Fatalf("the hint record was written with hints off: %v", err)
		}
	})
	t.Run("not perched", func(t *testing.T) {
		r, surface, _ := hintRig(t, true, keptFor("editor", "save this file"))
		r.hintsOn(t, false)
		r.hintsOn(t, true)
		surface.noHint()
		if n := r.clock.Armed(); n != 0 {
			t.Fatalf("%d timers armed with Oni perched nowhere", n)
		}
	})
	t.Run("nothing kept for the app", func(t *testing.T) {
		r, surface, cli := hintRig(t, true, keptFor("editor", "save this file"))
		r.perchAgain(t, cli, "0x4")
		surface.noHint()
		if n := r.clock.Armed(); n != 0 {
			t.Fatalf("%d timers armed for an app with nothing kept", n)
		}
		if _, err := os.Stat(keep.DefaultHintsPath()); !os.IsNotExist(err) {
			t.Fatalf("the hint record was written for no hint: %v", err)
		}
	})
}

// A hint shows once per kept walkthrough a day, no sooner than the
// cooldown after the last one, and retracts untouched after the spec's
// time with its one timer.
func TestOniHintShowsOncePerKeyPerDayAfterItsCooldownAndRetracts(t *testing.T) {
	r, surface, cli := hintRig(t, true, keptFor("editor", "save this file"))
	r.perchAgain(t, cli, "0x1")
	offer := surface.hint()
	if n := r.clock.Armed(); n != 1 {
		t.Fatalf("%d timers armed while a hint shows, want its retract alone", n)
	}
	r.clock.Advance(feature.HintOfferTTL() - time.Second)
	surface.noHint()
	r.clock.Advance(time.Second)
	surface.gone(offer.ID)
	if n := r.clock.Armed(); n != 0 {
		t.Fatalf("%d timers armed after the retract", n)
	}
	if len(r.model.sent()) != 0 || r.grim.count() != 0 {
		t.Fatalf("a hint sent %d requests and took %d pictures", len(r.model.sent()), r.grim.count())
	}

	// a second walkthrough kept for the app waits out the cooldown
	keepEntry(t, keptFor("editor", "export to pdf"))
	r.perchAgain(t, cli, "0x1")
	surface.noHint()
	r.clock.Advance(keep.HintCooldown - feature.HintOfferTTL() - time.Second)
	r.perchAgain(t, cli, "0x1")
	surface.noHint()
	r.clock.Advance(time.Second)
	r.perchAgain(t, cli, "0x1")
	second := surface.hint()
	r.clock.Advance(feature.HintOfferTTL())
	surface.gone(second.ID)

	// both were hinted today: a cooldown later, nothing
	r.clock.Advance(keep.HintCooldown)
	r.perchAgain(t, cli, "0x1")
	surface.noHint()

	// the next day, again
	r.clock.Advance(24 * time.Hour)
	r.perchAgain(t, cli, "0x1")
	surface.hint()
}

// Not now lets the hint go and snoozes its app for the rest of the day;
// another app's kept walkthrough is still offered, and the next day the
// first app's is again.
func TestOniHintNotNowSnoozesTheApp(t *testing.T) {
	r, surface, cli := hintRig(t, true, keptFor("editor", "save this file"), keptFor("editor", "export to pdf"), keptFor("viewer", "rotate the photo"))
	r.perchAgain(t, cli, "0x1")
	offer := surface.hint()
	if err := surface.call(protocol.MOniSkip, protocol.OniKeepParams{ID: offer.ID}, nil); err != nil {
		t.Fatal(err)
	}
	surface.gone(offer.ID)
	if n := r.clock.Armed(); n != 0 {
		t.Fatalf("%d timers armed after Not now", n)
	}
	if code, _ := reasonOf(surface.call(protocol.MOniSkip, protocol.OniKeepParams{ID: offer.ID}, nil)); code != protocol.CodeNotFound {
		t.Fatalf("a second Not now: %s", code)
	}

	r.clock.Advance(keep.HintCooldown)
	r.perchAgain(t, cli, "0x1")
	surface.noHint()
	r.perchAgain(t, cli, "0x4")
	other := surface.hint()
	r.clock.Advance(feature.HintOfferTTL())
	surface.gone(other.ID)

	r.clock.Advance(24 * time.Hour)
	r.perchAgain(t, cli, "0x1")
	surface.hint()
}

// Show me replays the kept walkthrough on this computer: no model call, no
// consent, the strand on the kept control and the hint answered yes.
func TestOniHintShowMeReplaysWithNoModel(t *testing.T) {
	r, surface, cli := hintRig(t, true, keptFor("editor", "save this file"))
	r.screen.level.Store(120)
	r.perchAgain(t, cli, "0x1")
	offer := surface.hint()
	var res protocol.OniAskResult
	if err := surface.call(protocol.MOniGuide, protocol.OniGuideParams{Action: protocol.OniGuideNext}, &res); err != nil {
		t.Fatal(err)
	}
	if res.Status != protocol.OniAskAnswered || res.Answer == nil || !res.Answer.Replay || res.Answer.Step != 1 || res.Answer.Destination != feature.EgressLocal || res.Answer.Point == nil || *res.Answer.Point != (protocol.OniRect{X: 0.1, Y: 0.05, W: 0.1, H: 0.05}) {
		t.Fatalf("Show me = %+v %+v", res, res.Answer)
	}
	if a := surface.await("the hint's answer", answerTo(offer.ID)).(*feature.ConsentAnswerFrame); a.Answer != "yes" {
		t.Fatalf("hint answer = %+v", a)
	}
	if g := surface.await("the replay card", guidePhase(feature.GuidePointing, 1)).(*feature.GuideFrame); !g.Replay {
		t.Fatalf("card = %+v", g)
	}
	if err := surface.arrived(res.Answer.Seq, ""); err != nil {
		t.Fatal(err)
	}
	surface.await("step 1 waiting", guidePhase(feature.GuideWaiting, 1))
	res = r.guide(t, surface, protocol.OniGuideNext)
	if err := surface.arrived(res.Answer.Seq, ""); err != nil {
		t.Fatal(err)
	}
	surface.await("step 2 waiting", guidePhase(feature.GuideWaiting, 2))
	res = r.guide(t, surface, protocol.OniGuideNext)
	if res.Status != protocol.OniAskEnded || !res.Answer.Done || !res.Answer.Replay {
		t.Fatalf("replay finish = %+v %+v", res, res.Answer)
	}
	if len(r.model.sent()) != 0 {
		t.Fatalf("Show me sent %d model requests", len(r.model.sent()))
	}
}

// A hint goes, answered no, when Oni lets go of the window and when hints
// are turned off; neither snoozes the app.
func TestOniHintGoesWithThePerchAndTheSetting(t *testing.T) {
	r, surface, cli := hintRig(t, true, keptFor("editor", "save this file"), keptFor("editor", "export to pdf"))
	r.perchAgain(t, cli, "0x1")
	offer := surface.hint()
	if err := cli.call(protocol.MOniDetach, protocol.OniDetachParams{}, nil); err != nil {
		t.Fatal(err)
	}
	surface.gone(offer.ID)
	if n := r.clock.Armed(); n != 0 {
		t.Fatalf("%d timers armed after the detach", n)
	}

	r.clock.Advance(keep.HintCooldown)
	r.perchAgain(t, cli, "0x1")
	offer = surface.hint()
	r.hintsOn(t, false)
	surface.gone(offer.ID)
	if n := r.clock.Armed(); n != 0 {
		t.Fatalf("%d timers armed after hints went off", n)
	}
}

// On a hosted route, which only the Hyakki app may use, Show me from the
// card still replays the kept walkthrough, since a replay sends nothing.
// A step that misses falls through to a model read; that read meets the
// route's rule and the guide ends refused, with the line saying why, before
// any picture is sent.
func TestOniHintShowMeOnAHostedRouteReplaysAndRefusesAMissedStep(t *testing.T) {
	hosted := llm.Plan{Provider: protocol.ProviderEmber, Model: "eyes", Vision: true, Paid: true}
	r, surface, cli := hintRigOn(t, hosted, true, keptFor("editor", "save this file"))
	r.screen.level.Store(120)
	r.perchAgain(t, cli, "0x1")
	surface.hint()
	var res protocol.OniAskResult
	if err := surface.call(protocol.MOniGuide, protocol.OniGuideParams{Action: protocol.OniGuideNext}, &res); err != nil {
		t.Fatalf("Show me on a hosted route: %v", err)
	}
	if res.Status != protocol.OniAskAnswered || res.Answer == nil || !res.Answer.Replay || res.Answer.Step != 1 || res.Answer.Destination != feature.EgressLocal {
		t.Fatalf("Show me = %+v %+v", res, res.Answer)
	}
	if err := surface.arrived(res.Answer.Seq, ""); err != nil {
		t.Fatal(err)
	}
	surface.await("step 1 waiting", guidePhase(feature.GuideWaiting, 1))
	// Save is gone from the window, so step 2 misses
	r.tess.set(fileWord)
	res = r.guide(t, surface, protocol.OniGuideNext)
	if res.Status != protocol.OniAskEnded || res.Answer == nil || res.Answer.Done || res.Answer.Refused != oniRefusedAppOnly || !strings.Contains(res.Answer.Text, "Ryoku Ember") {
		t.Fatalf("the missed step = %+v %+v", res, res.Answer)
	}
	if g := surface.await("the refusal on the card", guidePhase(feature.GuideDone, 0)).(*feature.GuideFrame); g.Text != res.Answer.Text {
		t.Fatalf("card = %+v", g)
	}
	if len(r.model.sent()) != 0 {
		t.Fatalf("%d model requests on a route the card may not use", len(r.model.sent()))
	}
	r.awaitNoGuide(t)
}
