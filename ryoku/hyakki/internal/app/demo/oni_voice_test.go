package demo

import (
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	"ryoku-rashin/internal/oni/feature"
	"ryoku-rashin/internal/protocol"
)

func callErr(t *testing.T, s *server, method string, params any) *protocol.Error {
	t.Helper()
	raw, err := json.Marshal(params)
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.call(method, raw)
	var wire *protocol.Error
	if !errors.As(err, &wire) {
		t.Fatalf("%s %+v: %v, want a protocol error", method, params, err)
	}
	return wire
}

// With voice off the voice calls refuse as the daemon's policy does; the
// meter still runs, since Settings tests the microphone before turning on.
func TestDemoVoiceCallsFollowTheSwitches(t *testing.T) {
	s := &server{data: makeFixtures(FeatureOni)}
	if wire := callErr(t, s, protocol.MOniTalk, protocol.OniTalkParams{Action: protocol.OniVoiceStart}); wire.Code != protocol.CodeDisabled || wire.Reason != feature.ReasonFeatureOff {
		t.Fatalf("talk with voice off: %+v", wire)
	}
	if wire := callErr(t, s, protocol.MOniDictate, protocol.OniDictateParams{Action: protocol.OniVoiceStart}); wire.Code != protocol.CodeDisabled || wire.Reason != feature.ReasonFeatureOff {
		t.Fatalf("dictate with dictation off: %+v", wire)
	}
	test := call(t, s, protocol.MOniVoiceTest, protocol.OniVoiceTestParams{}).(protocol.OniVoiceTestResult)
	if len(test.Levels) != protocol.OniVoiceTestDefaultMs/100 || len(test.Sources) != 2 {
		t.Fatalf("the meter with voice off: %+v", test)
	}

	off := &server{data: makeFixtures()}
	if wire := callErr(t, off, protocol.MOniTalk, protocol.OniTalkParams{Action: protocol.OniVoiceStart}); wire.Code != protocol.CodeDisabled || wire.Reason != feature.ReasonOniOff {
		t.Fatalf("talk with Oni off: %+v", wire)
	}
}

func TestDemoVoiceAnswersDeterministically(t *testing.T) {
	s := &server{data: makeFixtures(FeatureOniVoice)}
	for _, flag := range []string{FeatureOniVoice, FeatureOni} {
		if s.data.snapshot.Settings.Features[flag] {
			t.Fatalf("the demo switch %s leaked into the feature flags", flag)
		}
	}
	oni := s.data.snapshot.Settings.Oni
	if !oni.Master || !oni.Voice || !oni.VoiceSettings.Dictation.Enabled || oni.VoiceSettings.Wake.Enabled || oni.VoiceSettings.OpenMic {
		t.Fatalf("the voice demo starts with talking and dictation on and no continuous microphone, got %+v", oni)
	}

	for _, step := range []struct{ action, phase string }{
		{protocol.OniVoiceStop, feature.VoiceIdle},
		{protocol.OniVoiceStart, feature.VoiceListening},
		{protocol.OniVoiceStop, feature.VoiceTranscribing},
		{protocol.OniVoiceStart, feature.VoiceListening},
		{protocol.OniVoiceCancel, feature.VoiceIdle},
	} {
		got := call(t, s, protocol.MOniTalk, protocol.OniTalkParams{Action: step.action}).(protocol.OniVoiceResult)
		if got.Phase != step.phase {
			t.Fatalf("talk %s: phase %q, want %q", step.action, got.Phase, step.phase)
		}
	}
	if got := call(t, s, protocol.MOniSpeak, protocol.OniSpeakParams{Action: protocol.OniVoiceStop}).(protocol.OniVoiceResult); got.Phase != feature.VoiceIdle {
		t.Fatalf("speak stop: %+v", got)
	}
	if wire := callErr(t, s, protocol.MOniSpeak, protocol.OniSpeakParams{Action: protocol.OniVoiceStart}); wire.Code != protocol.CodeBadRequest {
		t.Fatalf("speak start: %+v", wire)
	}
	if wire := callErr(t, s, protocol.MOniTalk, protocol.OniTalkParams{Action: "hold"}); wire.Code != protocol.CodeBadRequest {
		t.Fatalf("talk hold: %+v", wire)
	}

	started := call(t, s, protocol.MOniDictate, protocol.OniDictateParams{Action: protocol.OniVoiceStart}).(protocol.OniDictateResult)
	if started.Phase != feature.VoiceListening || started.Rest != "" {
		t.Fatalf("dictate start: %+v", started)
	}
	clean := call(t, s, protocol.MOniDictate, protocol.OniDictateParams{Action: protocol.OniVoiceStop}).(protocol.OniDictateResult)
	want := protocol.OniDictateResult{Phase: feature.VoiceIdle, Mode: protocol.OniDictateClean, Landed: len("Meet me at the north gate at five."),
		Rest: " Bring the lantern, and the map.", Refused: "closed"}
	if !reflect.DeepEqual(clean, want) {
		t.Fatalf("a clean dictation ends\n%+v, want\n%+v", clean, want)
	}
	again := call(t, s, protocol.MOniDictate, protocol.OniDictateParams{Action: protocol.OniVoiceStop}).(protocol.OniDictateResult)
	if again != (protocol.OniDictateResult{Phase: feature.VoiceIdle}) {
		t.Fatalf("a stop with nothing open typed %+v", again)
	}

	call(t, s, protocol.MOniDictate, protocol.OniDictateParams{Action: protocol.OniVoiceStart, Mode: protocol.OniDictatePolished})
	polished := call(t, s, protocol.MOniDictate, protocol.OniDictateParams{Action: protocol.OniVoiceStop}).(protocol.OniDictateResult)
	if polished.Mode != protocol.OniDictateClean || polished.Fallback == "" || polished.Rest != want.Rest {
		t.Fatalf("a polished dictation with no polisher falls back to clean, got %+v", polished)
	}
	call(t, s, protocol.MOniDictate, protocol.OniDictateParams{Action: protocol.OniVoiceStart, Mode: protocol.OniDictateExact})
	exact := call(t, s, protocol.MOniDictate, protocol.OniDictateParams{Action: protocol.OniVoiceStop}).(protocol.OniDictateResult)
	if exact.Mode != protocol.OniDictateExact || exact.Landed != 0 || exact.Rest != demoDictation {
		t.Fatalf("an exact dictation types the words as heard, got %+v", exact)
	}
	if wire := callErr(t, s, protocol.MOniDictate, protocol.OniDictateParams{Action: protocol.OniVoiceStart, Mode: "loud"}); wire.Code != protocol.CodeBadRequest {
		t.Fatalf("an unknown mode: %+v", wire)
	}

	first := call(t, s, protocol.MOniVoiceTest, protocol.OniVoiceTestParams{Ms: 500}).(protocol.OniVoiceTestResult)
	second := call(t, s, protocol.MOniVoiceTest, protocol.OniVoiceTestParams{Ms: 500}).(protocol.OniVoiceTestResult)
	if !reflect.DeepEqual(first, second) || len(first.Levels) != 5 || first.Mic != "" {
		t.Fatalf("the meter is not the same twice: %+v %+v", first, second)
	}
	for _, level := range first.Levels {
		if level < 0 || level > 1 {
			t.Fatalf("a level outside 0..1: %v", first.Levels)
		}
	}
	quiet := call(t, s, protocol.MOniVoiceTest, protocol.OniVoiceTestParams{Mic: demoMicSources[1].Name, Ms: 500}).(protocol.OniVoiceTestResult)
	if quiet.Mic != demoMicSources[1].Name || quiet.Levels[1] >= first.Levels[1] {
		t.Fatalf("the built-in microphone is not the quieter one: %+v", quiet)
	}
	for _, params := range []protocol.OniVoiceTestParams{{Ms: 50}, {Ms: protocol.OniVoiceTestMaxMs + 1}, {Mic: "../dev/snd"}} {
		if wire := callErr(t, s, protocol.MOniVoiceTest, params); wire.Code != protocol.CodeBadRequest {
			t.Fatalf("test %+v: %+v", params, wire)
		}
	}
	if wire := callErr(t, s, protocol.MOniVoiceTest, protocol.OniVoiceTestParams{Mic: "alsa_input.none"}); wire.Code != protocol.CodeUnavailable || wire.Reason != protocol.OniReasonNoMicrophone {
		t.Fatalf("a source that is not there: %+v", wire)
	}
}

func TestDemoWithoutAMicrophoneSaysSo(t *testing.T) {
	s := &server{data: makeFixtures(FeatureOniVoice, FeatureOniNoMic)}
	if s.data.snapshot.Settings.Features[FeatureOniNoMic] {
		t.Fatal("the no-microphone switch leaked into the feature flags")
	}
	if wire := callErr(t, s, protocol.MOniVoiceTest, protocol.OniVoiceTestParams{}); wire.Code != protocol.CodeUnavailable || wire.Reason != protocol.OniReasonNoMicrophone {
		t.Fatalf("the meter with no microphone: %+v", wire)
	}
}
