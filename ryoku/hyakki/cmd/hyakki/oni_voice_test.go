package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"ryoku-rashin/internal/app/demo"
	"ryoku-rashin/internal/oni/feature"
	"ryoku-rashin/internal/protocol"
)

func decodeOne(t *testing.T, out *bytes.Buffer, v any) {
	t.Helper()
	dec := json.NewDecoder(bytes.NewReader(out.Bytes()))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		t.Fatalf("stdout %q: %v", out.String(), err)
	}
	if dec.More() {
		t.Fatalf("stdout carries more than one object: %q", out.String())
	}
}

// A compositor bind runs talk start on press and talk stop on release; the
// JSON is the phase the press left the microphone in.
func TestOniTalkAndSpeakPrintThePhase(t *testing.T) {
	demoOniDaemon(t, demo.FeatureOniVoice)
	for _, step := range []struct {
		args  []string
		phase string
	}{
		{[]string{"start", "--json"}, feature.VoiceListening},
		{[]string{"--json", "stop"}, feature.VoiceTranscribing},
		{[]string{"start", "--json"}, feature.VoiceListening},
		{[]string{"cancel", "--json"}, feature.VoiceIdle},
	} {
		var out, errs bytes.Buffer
		if code := runOniTalk(step.args, &out, &errs); code != 0 {
			t.Fatalf("talk %v exited %d: %s", step.args, code, errs.String())
		}
		var r protocol.OniVoiceResult
		decodeOne(t, &out, &r)
		if r.Phase != step.phase {
			t.Fatalf("talk %v = %+v, want %s", step.args, r, step.phase)
		}
	}
	var out, errs bytes.Buffer
	if code := runOniTalk([]string{"start"}, &out, &errs); code != 0 || out.String() != "Oni is listening.\n" {
		t.Fatalf("talk start: exit %d, %q", code, out.String())
	}
	out.Reset()
	if code := runOniTalk([]string{"cancel"}, &out, &errs); code != 0 || out.String() != "The microphone is closed.\n" {
		t.Fatalf("talk cancel: exit %d, %q", code, out.String())
	}
	out.Reset()
	if code := runOniSpeak([]string{"stop", "--json"}, &out, &errs); code != 0 {
		t.Fatalf("speak stop exited %d: %s", code, errs.String())
	}
	var r protocol.OniVoiceResult
	decodeOne(t, &out, &r)
	if r.Phase != feature.VoiceIdle {
		t.Fatalf("speak stop = %+v", r)
	}
	for _, args := range [][]string{
		{"oni", "talk"}, {"oni", "talk", "hold"}, {"oni", "talk", "start", "stop"}, {"oni", "talk", "start", "--mode", "clean"},
		{"oni", "speak"}, {"oni", "speak", "start"},
	} {
		if code := run(args); code != 2 {
			t.Fatalf("%v exited %d, want the usage", args, code)
		}
	}
}

func TestOniTalkWithVoiceOffIsRefused(t *testing.T) {
	demoOniDaemon(t, demo.FeatureOni)
	var out, errs bytes.Buffer
	if code := runOniTalk([]string{"start", "--json"}, &out, &errs); code != 1 || out.Len() != 0 || !strings.Contains(errs.String(), feature.ReasonFeatureOff) {
		t.Fatalf("talk with voice off: exit %d, stdout %q, stderr %q", code, out.String(), errs.String())
	}
}

func TestOniDictateTakesTheModeAndPrintsWhatWasNotTyped(t *testing.T) {
	demoOniDaemon(t, demo.FeatureOniVoice)
	var out, errs bytes.Buffer
	// the harness puts flags after the action; either order works
	if code := runOniDictate([]string{"start", "--json", "--mode", "clean"}, &out, &errs); code != 0 {
		t.Fatalf("dictate start exited %d: %s", code, errs.String())
	}
	var started protocol.OniDictateResult
	decodeOne(t, &out, &started)
	if started.Phase != feature.VoiceListening {
		t.Fatalf("dictate start = %+v", started)
	}
	out.Reset()
	if code := runOniDictate([]string{"--json", "stop"}, &out, &errs); code != 0 {
		t.Fatalf("dictate stop exited %d: %s", code, errs.String())
	}
	var stopped protocol.OniDictateResult
	decodeOne(t, &out, &stopped)
	if stopped.Phase != feature.VoiceIdle || stopped.Mode != protocol.OniDictateClean || stopped.Landed == 0 || stopped.Rest == "" || stopped.Refused != "closed" {
		t.Fatalf("dictate stop = %+v", stopped)
	}
	// the text form ends with the words left to copy
	out.Reset()
	runOniDictate([]string{"start", "--mode=clean"}, &out, &errs)
	out.Reset()
	if code := runOniDictate([]string{"stop"}, &out, &errs); code != 0 {
		t.Fatalf("dictate stop exited %d: %s", code, errs.String())
	}
	if text := out.String(); !strings.Contains(text, "Not typed, to copy:\n"+stopped.Rest+"\n") || !strings.Contains(text, "the window was closed") {
		t.Fatalf("dictate stop printed %q", text)
	}
	out.Reset()
	if code := runOniDictate([]string{"cancel", "--json"}, &out, &errs); code != 0 {
		t.Fatalf("dictate cancel exited %d: %s", code, errs.String())
	}
	for _, args := range [][]string{
		{"oni", "dictate"}, {"oni", "dictate", "start", "--mode"}, {"oni", "dictate", "start", "--mode", "loud"},
		{"oni", "dictate", "stop", "--mode", "clean"}, {"oni", "dictate", "start", "--seconds", "2"},
	} {
		if code := run(args); code != 2 {
			t.Fatalf("%v exited %d, want the usage", args, code)
		}
	}
}

func TestOniVoiceTestPrintsLevelsAndSources(t *testing.T) {
	demoOniDaemon(t, demo.FeatureOniVoice)
	var out, errs bytes.Buffer
	if code := runOniVoiceTest([]string{"--seconds", "0.5", "--json"}, &out, &errs); code != 0 {
		t.Fatalf("voice-test exited %d: %s", code, errs.String())
	}
	var r protocol.OniVoiceTestResult
	decodeOne(t, &out, &r)
	if len(r.Levels) != 5 || len(r.Sources) == 0 {
		t.Fatalf("voice-test --seconds 0.5 = %+v, want 5 levels", r)
	}
	for _, l := range r.Levels {
		if l < 0 || l > 1 {
			t.Fatalf("level %v outside [0, 1]", l)
		}
	}
	out.Reset()
	if code := runOniVoiceTest([]string{"--ms", "300"}, &out, &errs); code != 0 || !strings.Contains(out.String(), "Microphones:") || !strings.Contains(out.String(), r.Sources[0].Name) {
		t.Fatalf("voice-test text: exit %d, %q", code, out.String())
	}
	for _, args := range [][]string{
		{"oni", "voice-test", "--seconds", "9"}, {"oni", "voice-test", "--seconds", "0.01"}, {"oni", "voice-test", "--ms", "x"},
		{"oni", "voice-test", "now"}, {"oni", "voice-test", "--mode", "clean"},
	} {
		if code := run(args); code != 2 {
			t.Fatalf("%v exited %d, want the usage", args, code)
		}
	}
}

func TestOniVoiceTestWithNoMicrophoneSaysWhy(t *testing.T) {
	demoOniDaemon(t, demo.FeatureOniVoice, demo.FeatureOniNoMic)
	var out, errs bytes.Buffer
	if code := runOniVoiceTest([]string{"--json"}, &out, &errs); code != 1 || out.Len() != 0 || !strings.Contains(errs.String(), protocol.OniReasonNoMicrophone) {
		t.Fatalf("no microphone: exit %d, stdout %q, stderr %q", code, out.String(), errs.String())
	}
}
