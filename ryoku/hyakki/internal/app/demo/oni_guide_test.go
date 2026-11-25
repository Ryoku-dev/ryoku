package demo

import (
	"encoding/json"
	"errors"
	"reflect"
	"testing"
	"time"

	"ryoku-rashin/internal/oni/feature"
	"ryoku-rashin/internal/protocol"
)

// guideDemo is the Oni demo with guides and Keep on and a clock the test
// moves
func guideDemo(t *testing.T, features ...string) (*server, *time.Time) {
	t.Helper()
	s := &server{data: makeFixtures(append([]string{FeatureOni}, features...)...)}
	s.data.snapshot.Settings.Oni.Guide = protocol.OniGuideSettings{Enabled: true, Keep: true}
	clock := Epoch
	s.data.oni.now = func() time.Time { return clock }
	return s, &clock
}

func guideCall(t *testing.T, s *server, params protocol.OniGuideParams) protocol.OniAskResult {
	t.Helper()
	return call(t, s, protocol.MOniGuide, params).(protocol.OniAskResult)
}

func start(goal string) protocol.OniGuideParams {
	return protocol.OniGuideParams{Action: protocol.OniGuideStart, Goal: goal}
}

var next = protocol.OniGuideParams{Action: protocol.OniGuideNext}

func TestDemoGuideNeedsOniLookingAndItsSwitch(t *testing.T) {
	s := &server{data: makeFixtures(FeatureOni)}
	if refused := oniCallErr(t, s, protocol.MOniGuide, start("save this file")); refused.Code != protocol.CodeDisabled || refused.Reason != feature.ReasonFeatureOff {
		t.Fatalf("a guide with its switch off: %+v", refused)
	}
	s.data.snapshot.Settings.Oni.Guide.Enabled = true
	s.data.snapshot.Settings.Oni.Inspect = false
	if refused := oniCallErr(t, s, protocol.MOniGuide, start("save this file")); refused.Code != protocol.CodeDisabled || refused.Reason != feature.ReasonFeatureOff {
		t.Fatalf("a guide with looking off: %+v", refused)
	}
	s.data.snapshot.Settings.Oni.Inspect = true
	s.data.snapshot.Settings.Oni.Master = false
	if refused := oniCallErr(t, s, protocol.MOniGuide, start("save this file")); refused.Code != protocol.CodeDisabled || refused.Reason != feature.ReasonOniOff {
		t.Fatalf("a guide with Oni off: %+v", refused)
	}
	s.data.snapshot.Settings.Oni.Master = true
	call(t, s, protocol.MOniDetach, protocol.OniDetachParams{})
	if refused := oniCallErr(t, s, protocol.MOniGuide, start("save this file")); refused.Code != protocol.CodeUnavailable {
		t.Fatalf("a guide with no perch: %+v", refused)
	}
	for _, c := range [][2]string{
		{protocol.MOniGuide, `{"action":"start"}`},
		{protocol.MOniGuide, `{"action":"next","goal":"x"}`},
		{protocol.MOniKeep, `{}`},
		{protocol.MOniSkip, `{"id":"Keep 1"}`},
		{protocol.MOniPick, `{"action":"point"}`},
		{protocol.MOniPick, `{"action":"arm","window":"21"}`},
	} {
		var refused *protocol.Error
		if _, err := s.call(c[0], json.RawMessage(c[1])); !errors.As(err, &refused) || refused.Code != protocol.CodeBadRequest {
			t.Errorf("%s %s: %v", c[0], c[1], err)
		}
	}
}

// demoGuideRun walks the demo's guide: consent, two steps, the keep offer
// and keeping it, then the same goal again replayed with no picture sent
func demoGuideRun(t *testing.T, s *server) []protocol.OniAskResult {
	t.Helper()
	var out []protocol.OniAskResult
	first := guideCall(t, s, start("how do I save this file"))
	out = append(out, first)
	if first.Status != protocol.OniAskConsent || first.Consent == nil || first.Answer != nil {
		t.Fatalf("the first step did not ask about its picture: %+v", first)
	}
	step1 := call(t, s, protocol.MOniConsentAnswer, protocol.OniConsentAnswerParams{ID: first.Consent.ID, Answer: "yes"}).(protocol.OniAskResult)
	out = append(out, step1)
	if a := step1.Answer; step1.Status != protocol.OniAskAnswered || a.Mode != protocol.OniModeGuide || a.Step != 1 || a.Point == nil || a.Seq == 0 || a.Done || a.Replay || a.Route != demoOniHostedRoute {
		t.Fatalf("step 1 = %+v", step1.Answer)
	}
	circled := guideCall(t, s, protocol.OniGuideParams{Action: protocol.OniGuideCircle})
	out = append(out, circled)
	if a := circled.Answer; a.Step != 1 || a.Seq <= step1.Answer.Seq || *a.Point != *step1.Answer.Point {
		t.Fatalf("circling = %+v", a)
	}
	step2 := guideCall(t, s, next)
	out = append(out, step2)
	if a := step2.Answer; step2.Status != protocol.OniAskAnswered || a.Step != 2 || a.Text == step1.Answer.Text {
		t.Fatalf("step 2 = %+v (once a session asks once)", step2)
	}
	done := guideCall(t, s, next)
	out = append(out, done)
	if a := done.Answer; done.Status != protocol.OniAskEnded || !a.Done || a.Step != 2 || a.Replay || done.Keep == nil {
		t.Fatalf("the end = %+v", done)
	}
	if done.Keep.ExpiresMs != feature.KeepOfferTTL().Milliseconds() || done.Keep.Question == "" {
		t.Fatalf("keep offer = %+v", done.Keep)
	}
	kept := call(t, s, protocol.MOniKeep, protocol.OniKeepParams{ID: done.Keep.ID}).(protocol.OniKeepResult)
	if !kept.Kept || kept.Entries != 1 {
		t.Fatalf("keep = %+v", kept)
	}
	if again := oniCallErr(t, s, protocol.MOniKeep, protocol.OniKeepParams{ID: done.Keep.ID}); again.Code != protocol.CodeNotFound {
		t.Fatalf("a second keep: %+v", again)
	}
	asked := s.data.oni.asked
	replay := guideCall(t, s, start("  How do I SAVE this file "))
	out = append(out, replay)
	if a := replay.Answer; replay.Status != protocol.OniAskAnswered || !a.Replay || a.Step != 1 || a.Route != "" || a.Destination != feature.EgressLocal {
		t.Fatalf("the kept guide did not replay: %+v", replay)
	}
	replayed := guideCall(t, s, next)
	out = append(out, replayed)
	end := guideCall(t, s, next)
	out = append(out, end)
	if a := end.Answer; end.Status != protocol.OniAskEnded || !a.Done || !a.Replay || end.Keep != nil || a.Text != "All done, just as you kept it." {
		t.Fatalf("the replay's end = %+v", end)
	}
	if s.data.oni.asked != asked {
		t.Fatal("the replay asked about a picture")
	}
	return out
}

func TestDemoGuideWalksKeepsAndReplaysDeterministically(t *testing.T) {
	a, _ := guideDemo(t)
	b, _ := guideDemo(t)
	first, second := demoGuideRun(t, a), demoGuideRun(t, b)
	if !reflect.DeepEqual(first, second) {
		t.Fatal("two runs of the demo's guide differ")
	}
}

func TestDemoGuideStopsSkipsAndLetsTheOfferLapse(t *testing.T) {
	s, clock := guideDemo(t, FeatureOniLocal)
	s.data.oni.allowed[demoOniLocalRoute] = true
	if refused := oniCallErr(t, s, protocol.MOniGuide, next); refused.Code != protocol.CodeConflict {
		t.Fatalf("next with no guide: %+v", refused)
	}
	if stopped := guideCall(t, s, protocol.OniGuideParams{Action: protocol.OniGuideStop}); stopped.Status != protocol.OniAskEnded || stopped.Answer != nil {
		t.Fatalf("stop with no guide = %+v", stopped)
	}
	guideCall(t, s, start("save this file"))
	stopped := guideCall(t, s, protocol.OniGuideParams{Action: protocol.OniGuideStop})
	if stopped.Status != protocol.OniAskEnded || stopped.Answer == nil || stopped.Answer.Done || stopped.Answer.Step != 1 {
		t.Fatalf("stop = %+v", stopped)
	}
	if refused := oniCallErr(t, s, protocol.MOniGuide, next); refused.Code != protocol.CodeConflict {
		t.Fatalf("next after stop: %+v", refused)
	}

	finish := func() *protocol.OniKeepOffer {
		guideCall(t, s, start("save this file"))
		guideCall(t, s, next)
		return guideCall(t, s, next).Keep
	}
	offer := finish()
	call(t, s, protocol.MOniSkip, protocol.OniKeepParams{ID: offer.ID})
	if refused := oniCallErr(t, s, protocol.MOniKeep, protocol.OniKeepParams{ID: offer.ID}); refused.Code != protocol.CodeNotFound {
		t.Fatalf("keep after skip: %+v", refused)
	}
	offer = finish()
	*clock = clock.Add(feature.KeepOfferTTL())
	if refused := oniCallErr(t, s, protocol.MOniKeep, protocol.OniKeepParams{ID: offer.ID}); refused.Code != protocol.CodeNotFound {
		t.Fatalf("keep after the offer lapsed: %+v", refused)
	}
	// a new guide drops the offer of the last
	offer = finish()
	guideCall(t, s, start("save this file"))
	if refused := oniCallErr(t, s, protocol.MOniSkip, protocol.OniKeepParams{ID: offer.ID}); refused.Code != protocol.CodeNotFound {
		t.Fatalf("skip after a new guide: %+v", refused)
	}
	if len(s.data.oni.guides.kept) != 0 {
		t.Fatalf("skipped and lapsed offers kept %v", s.data.oni.guides.kept)
	}
	// with Keep off nothing is offered
	s.data.snapshot.Settings.Oni.Guide.Keep = false
	if offer := finish(); offer != nil {
		t.Fatalf("Keep off offered %+v", offer)
	}
	// every-time disclosure asks before each step's picture, the first too
	s.data.snapshot.Settings.Oni.Disclosure = protocol.OniDisclosureAlways
	guideCall(t, s, start("save this file"))
	first := call(t, s, protocol.MOniConsentAnswer, protocol.OniConsentAnswerParams{ID: s.data.oni.lastConsent(t), Answer: "yes"}).(protocol.OniAskResult)
	if first.Answer == nil || first.Answer.Step != 1 {
		t.Fatalf("yes to the first picture = %+v", first)
	}
	second := guideCall(t, s, next)
	if second.Status != protocol.OniAskConsent || second.Consent == nil || s.data.oni.guides.running.step != 1 {
		t.Fatalf("every-time disclosure skipped the second picture: %+v", second)
	}
	// letting go ends the guide and drops its picture
	call(t, s, protocol.MOniDetach, protocol.OniDetachParams{})
	if s.data.oni.fates[second.Consent.ID] != demoFateDropped || len(s.data.oni.pending) != 0 || s.data.oni.guides.running != nil {
		t.Fatalf("detach left the guide: %d pictures waiting, fate %q", len(s.data.oni.pending), s.data.oni.fates[second.Consent.ID])
	}
}

// lastConsent is the id of the one picture waiting
func (o *oniFixtures) lastConsent(t *testing.T) string {
	t.Helper()
	if len(o.pending) != 1 {
		t.Fatalf("%d pictures wait", len(o.pending))
	}
	for id := range o.pending {
		return id
	}
	return ""
}

func TestDemoPickArmsAndAnswersOneClick(t *testing.T) {
	s, _ := guideDemo(t)
	if refused := oniCallErr(t, s, protocol.MOniPick, protocol.OniPickParams{Action: protocol.OniPickPoint, Output: demoOniOutput, X: 700, Y: 300}); refused.Code != protocol.CodeConflict {
		t.Fatalf("a click with nothing armed: %+v", refused)
	}
	if armed := call(t, s, protocol.MOniPick, protocol.OniPickParams{Action: protocol.OniPickArm}).(protocol.OniAskResult); armed.Status != protocol.OniAskArmed {
		t.Fatalf("arm = %+v", armed)
	}
	if refused := oniCallErr(t, s, protocol.MOniPick, protocol.OniPickParams{Action: protocol.OniPickPoint, Output: demoOniOutput, X: 10, Y: 10}); refused.Code != protocol.CodeNotFound || refused.Reason != feature.PerchClosed {
		t.Fatalf("a click on empty desktop: %+v", refused)
	}
	// the click disarmed the pick
	if refused := oniCallErr(t, s, protocol.MOniPick, protocol.OniPickParams{Action: protocol.OniPickPoint, Output: demoOniOutput, X: 700, Y: 300}); refused.Code != protocol.CodeConflict {
		t.Fatalf("a second click: %+v", refused)
	}
	call(t, s, protocol.MOniPick, protocol.OniPickParams{Action: protocol.OniPickArm, Text: "What is this line?"})
	asked := call(t, s, protocol.MOniPick, protocol.OniPickParams{Action: protocol.OniPickPoint, Output: demoOniOutput, X: 700, Y: 300}).(protocol.OniAskResult)
	if asked.Status != protocol.OniAskConsent || asked.Consent == nil {
		t.Fatalf("the click did not ask about the picture: %+v", asked)
	}
	answered := call(t, s, protocol.MOniConsentAnswer, protocol.OniConsentAnswerParams{ID: asked.Consent.ID, Answer: "yes"}).(protocol.OniAskResult)
	a := answered.Answer
	if answered.Status != protocol.OniAskAnswered || a.Mode != protocol.OniModeSelect || !a.Walk || a.Point == nil || a.Seq == 0 || a.Text == "" {
		t.Fatalf("the pick's answer = %+v", a)
	}
	if p := a.Point; p.X < 0 || p.Y < 0 || p.X+p.W > 1 || p.Y+p.H > 1 {
		t.Fatalf("the picked box leaves the window: %+v", p)
	}
	// a private field is refused before any picture is taken
	call(t, s, protocol.MOniPick, protocol.OniPickParams{Action: protocol.OniPickArm, Text: "what is my password here?"})
	before := s.data.oni.asked
	refused := call(t, s, protocol.MOniPick, protocol.OniPickParams{Action: protocol.OniPickPoint, Output: demoOniOutput, X: 700, Y: 300}).(protocol.OniAskResult)
	if refused.Answer == nil || refused.Answer.Refused != "sensitive" || refused.Answer.Point != nil || s.data.oni.asked != before {
		t.Fatalf("a private field = %+v", refused)
	}
	call(t, s, protocol.MOniPick, protocol.OniPickParams{Action: protocol.OniPickArm})
	if cancelled := call(t, s, protocol.MOniPick, protocol.OniPickParams{Action: protocol.OniPickCancel}).(protocol.OniAskResult); cancelled.Status != protocol.OniAskEnded || s.data.oni.guides.pick != nil {
		t.Fatalf("cancel = %+v", cancelled)
	}
	s.data.snapshot.Settings.Oni.Inspect = false
	if refused := oniCallErr(t, s, protocol.MOniPick, protocol.OniPickParams{Action: protocol.OniPickArm}); refused.Code != protocol.CodeDisabled {
		t.Fatalf("arming with looking off: %+v", refused)
	}
}
