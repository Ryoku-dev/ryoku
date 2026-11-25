package demo

import (
	"bytes"
	"encoding/json"
	"errors"
	"image/jpeg"
	"testing"
	"time"

	"ryoku-rashin/internal/oni/feature"
	"ryoku-rashin/internal/protocol"
)

func oniCallErr(t *testing.T, s *server, method string, params any) *protocol.Error {
	t.Helper()
	raw, _ := json.Marshal(params)
	_, err := s.call(method, raw)
	var wire *protocol.Error
	if !errors.As(err, &wire) {
		t.Fatalf("%s answered %v, want a refusal", method, err)
	}
	return wire
}

func TestDemoScreenAskWaitsForConsentAndSendsOnlyOnYes(t *testing.T) {
	s := &server{data: makeFixtures(FeatureOni)}
	result := call(t, s, protocol.MOniAsk, protocol.OniAskParams{Text: "what is this window for?"}).(protocol.OniAskResult)
	consent := result.Consent
	if result.Status != protocol.OniAskConsent || consent == nil || result.Answer != nil {
		t.Fatalf("the first ask did not stop for consent: %+v", result)
	}
	if consent.Destination != "hosted" || consent.Route != demoOniHostedRoute || consent.Path != "toplevel" || consent.Width != 960 || consent.Height != 620 || consent.ExpiresMs != 120000 || consent.Disclosure == "" {
		t.Fatalf("consent = %+v", consent)
	}
	if thumb, err := jpeg.Decode(bytes.NewReader(consent.Preview)); err != nil || thumb.Bounds().Dx() != 480 {
		t.Fatalf("the preview is not the thumbnail JPEG: %v", err)
	}
	if fate := s.data.oni.fates[consent.ID]; fate != "" {
		t.Fatalf("the picture was %s before any answer", fate)
	}

	answered := call(t, s, protocol.MOniConsentAnswer, protocol.OniConsentAnswerParams{ID: consent.ID, Answer: "yes"}).(protocol.OniAskResult)
	if answered.Status != protocol.OniAskAnswered || answered.Answer == nil || answered.Answer.Route != consent.Route || answered.Answer.Text == "" {
		t.Fatalf("yes did not answer: %+v", answered)
	}
	if again := oniCallErr(t, s, protocol.MOniConsentAnswer, protocol.OniConsentAnswerParams{ID: consent.ID, Answer: "yes"}); again.Code != protocol.CodeNotFound {
		t.Fatalf("a second yes: %+v", again)
	}

	// once per session: the same route is not asked about again
	direct := call(t, s, protocol.MOniAsk, protocol.OniAskParams{Text: "where is the rain jacket?"}).(protocol.OniAskResult)
	if direct.Status != protocol.OniAskAnswered || direct.Answer.Mode != "point" || direct.Answer.Point == nil {
		t.Fatalf("session consent asked again: %+v", direct)
	}
	// every time: it is
	s.data.snapshot.Settings.Oni.Disclosure = protocol.OniDisclosureAlways
	if every := call(t, s, protocol.MOniAsk, protocol.OniAskParams{Text: "and now?"}).(protocol.OniAskResult); every.Status != protocol.OniAskConsent {
		t.Fatalf("every-time disclosure skipped the question: %+v", every)
	}
}

func TestDemoScreenAskNoAndExpiryDropThePicture(t *testing.T) {
	s := &server{data: makeFixtures(FeatureOni, FeatureOniLocal)}
	clock := Epoch
	s.data.oni.now = func() time.Time { return clock }

	first := call(t, s, protocol.MOniAsk, protocol.OniAskParams{Text: "what is this?"}).(protocol.OniAskResult).Consent
	if first.Destination != feature.EgressLocal || first.Route != demoOniLocalRoute {
		t.Fatalf("the local demo routes to %s %s", first.Destination, first.Route)
	}
	declined := call(t, s, protocol.MOniConsentAnswer, protocol.OniConsentAnswerParams{ID: first.ID, Answer: "no"}).(protocol.OniAskResult)
	if declined.Status != protocol.OniAskDeclined || declined.Answer != nil || s.data.oni.fates[first.ID] != demoFateDropped {
		t.Fatalf("no = %+v, fate %q", declined, s.data.oni.fates[first.ID])
	}
	if late := oniCallErr(t, s, protocol.MOniConsentAnswer, protocol.OniConsentAnswerParams{ID: first.ID, Answer: "yes"}); late.Code != protocol.CodeNotFound {
		t.Fatalf("yes after no: %+v", late)
	}

	second := call(t, s, protocol.MOniAsk, protocol.OniAskParams{Text: "what is this?"}).(protocol.OniAskResult).Consent
	clock = clock.Add(120 * time.Second)
	if expired := oniCallErr(t, s, protocol.MOniConsentAnswer, protocol.OniConsentAnswerParams{ID: second.ID, Answer: "yes"}); expired.Code != protocol.CodeNotFound || s.data.oni.fates[second.ID] != demoFateDropped {
		t.Fatalf("an expired yes: %+v, fate %q", expired, s.data.oni.fates[second.ID])
	}

	// letting go drops what still waits
	third := call(t, s, protocol.MOniAsk, protocol.OniAskParams{Text: "what is this?"}).(protocol.OniAskResult).Consent
	call(t, s, protocol.MOniDetach, protocol.OniDetachParams{})
	if s.data.oni.fates[third.ID] != demoFateDropped {
		t.Fatal("detach kept a picture waiting")
	}
	if refused := oniCallErr(t, s, protocol.MOniAsk, protocol.OniAskParams{Text: "what is this?"}); refused.Code != protocol.CodeUnavailable {
		t.Fatalf("an ask with no perch: %+v", refused)
	}
}

func TestDemoPerchRefusesWithItsReason(t *testing.T) {
	s := &server{data: makeFixtures(FeatureOni)}
	perch := call(t, s, protocol.MOniPerch, protocol.OniPerchParams{Window: "21"}).(protocol.OniPerchResult)
	if perch.Window != "21" || perch.Box == nil || perch.Corner != "top-left" || perch.Seq == 0 {
		t.Fatalf("perch = %+v", perch)
	}
	for window, want := range map[string][2]string{"22": {protocol.CodeUnavailable, feature.PerchNoGeometry}, "99": {protocol.CodeNotFound, feature.PerchClosed}} {
		if refused := oniCallErr(t, s, protocol.MOniPerch, protocol.OniPerchParams{Window: window}); refused.Code != want[0] || refused.Reason != want[1] {
			t.Fatalf("perch %s: %+v, want %v", window, refused, want)
		}
	}
	s.data.snapshot.Settings.Oni.Inspect = false
	if refused := oniCallErr(t, s, protocol.MOniAsk, protocol.OniAskParams{Text: "hi"}); refused.Code != protocol.CodeDisabled {
		t.Fatalf("an ask with looking off: %+v", refused)
	}
	s.data.snapshot.Settings.Oni.Master = false
	if refused := oniCallErr(t, s, protocol.MOniPerch, protocol.OniPerchParams{Window: "21"}); refused.Reason != feature.PerchOniOff {
		t.Fatalf("a perch with Oni off: %+v", refused)
	}
}
