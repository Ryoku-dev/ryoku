package feature

import (
	"encoding/json"
	"math"
	"slices"
	"strings"
	"testing"

	"ryoku-rashin/internal/config"
	"ryoku-rashin/internal/oni/spec"
	"ryoku-rashin/internal/protocol"
)

func TestVoicePhasesAreTheSpecsPhases(t *testing.T) {
	phases := []string{VoiceIdle, VoiceArmed, VoiceListening, VoiceTranscribing, VoiceSpeaking}
	if !slices.Equal(phases, spec.VoicePhaseIDs) {
		t.Fatalf("frame phases %v, spec phases %v", phases, spec.VoicePhaseIDs)
	}
	for _, phase := range phases {
		frame := VoiceFrame{Phase: phase}
		switch phase {
		case VoiceArmed:
			frame.Source = VoiceSourceWake
		case VoiceTranscribing, VoiceSpeaking:
			frame.Text = "what time is it"
		}
		if _, err := EncodeFrame(&frame); err != nil {
			t.Errorf("phase %s: %v", phase, err)
		}
	}
}

func TestVoiceSharesTheTypedAsksEgress(t *testing.T) {
	ask, _ := Lookup(Inspect)
	voice, _ := Lookup(Voice)
	if voice.Egress != ask.Egress {
		t.Fatalf("a voice ask's egress %q differs from a typed ask's %q", voice.Egress, ask.Egress)
	}
	dictation, ok := Lookup(VoiceDictation)
	want := Feature{ID: "voice.dictation", Stage: StageReversible, Setting: "oni.voiceSettings.dictation.enabled", Egress: ask.Egress, Consent: ConsentUserInitiated}
	if !ok || dictation != want {
		t.Fatalf("voice.dictation is %+v, want %+v", dictation, want)
	}
	if MustAsk(VoiceDictation, everythingOn(), false) {
		t.Fatal("dictation asks over a press the user made")
	}
	s := everythingOn()
	s.VoiceSettings.Dictation.Enabled = false
	if d := Decide(Request{Feature: VoiceDictation}, s); d.Allow || d.Reason != ReasonFeatureOff {
		t.Fatalf("dictation with its switch off: %+v", d)
	}
	s = everythingOn()
	s.Voice = false
	if d := Decide(Request{Feature: Voice}, s); d.Allow || d.Reason != ReasonFeatureOff {
		t.Fatalf("voice with its switch off: %+v", d)
	}
}

// the capabilities Phase 10 shipped, before the voice wire
const (
	phase10DefaultCapabilities = `{"master":false,"features":[` +
		`{"id":"vision.inspect","stage":"read-only","egress":"disclosed","consent":"disclosure","allow":false,"reason":"oni-off"},` +
		`{"id":"voice","stage":"read-only","egress":"local","consent":"user-initiated","allow":false,"reason":"oni-off"},` +
		`{"id":"reflex","stage":"reversible","egress":"disclosed","consent":"announced","allow":false,"reason":"oni-off"},` +
		`{"id":"feeds.guard","stage":"reversible","egress":"local","consent":"per-action","allow":false,"reason":"oni-off"},` +
		`{"id":"hands.browser","stage":"unavailable","egress":"disclosed","consent":"firm","allow":false,"reason":"unavailable"},` +
		`{"id":"hands.app","stage":"unavailable","egress":"disclosed","consent":"firm","allow":false,"reason":"unavailable"},` +
		`{"id":"hands.desktop","stage":"unavailable","egress":"disclosed","consent":"firm","allow":false,"reason":"unavailable"}]}`
	phase10InspectOnCapabilities = `{"master":true,"features":[` +
		`{"id":"vision.inspect","stage":"read-only","egress":"disclosed","consent":"disclosure","allow":true,"reason":"allowed"},` +
		`{"id":"voice","stage":"read-only","egress":"local","consent":"user-initiated","allow":false,"reason":"feature-off"},` +
		`{"id":"reflex","stage":"reversible","egress":"disclosed","consent":"announced","allow":false,"reason":"feature-off"},` +
		`{"id":"feeds.guard","stage":"reversible","egress":"local","consent":"per-action","allow":false,"reason":"feature-off"},` +
		`{"id":"hands.browser","stage":"unavailable","egress":"disclosed","consent":"firm","allow":false,"reason":"unavailable"},` +
		`{"id":"hands.app","stage":"unavailable","egress":"disclosed","consent":"firm","allow":false,"reason":"unavailable"},` +
		`{"id":"hands.desktop","stage":"unavailable","egress":"disclosed","consent":"firm","allow":false,"reason":"unavailable"}]}`
)

// With voice off the capabilities are Phase 10's byte for byte but for the
// voice.dictation row and the voice row's egress, which follows the typed
// Ask's, the guide rows Phase 12 adds before the hands rows, the reflex
// rows Phase 13 adds before the guide's, the hands.browser row Phase 14
// makes guarded, and the Phase 16 rows after the guide's.
func TestCapabilitiesWithVoiceOffDifferOnlyByTheVoiceRows(t *testing.T) {
	inspectOn := config.Defaults()
	inspectOn.Enabled, inspectOn.Oni.Master, inspectOn.Oni.Inspect = true, true, true
	for _, c := range []struct {
		settings protocol.Settings
		before   string
		reason   string
	}{
		{config.Defaults(), phase10DefaultCapabilities, ReasonOniOff},
		{inspectOn, phase10InspectOnCapabilities, ReasonFeatureOff},
	} {
		got, err := json.Marshal(Capabilities(protocol.EffectiveOni(c.settings)))
		if err != nil {
			t.Fatal(err)
		}
		oldVoice := `{"id":"voice","stage":"read-only","egress":"local","consent":"user-initiated","allow":false,"reason":"` + c.reason + `"},`
		newVoice := `{"id":"voice","stage":"read-only","egress":"disclosed","consent":"user-initiated","allow":false,"reason":"` + c.reason + `"},` +
			`{"id":"voice.dictation","stage":"reversible","egress":"disclosed","consent":"user-initiated","allow":false,"reason":"` + c.reason + `"},`
		if !strings.Contains(c.before, oldVoice) {
			t.Fatal("the Phase 10 fixture has no voice row")
		}
		if want := withPhase16Rows(withHandsRow(withReflexRows(withGuideRows(strings.Replace(c.before, oldVoice, newVoice, 1), c.reason), c.reason), c.reason), c.reason); string(got) != want {
			t.Errorf("capabilities\n%s\nwant\n%s", got, want)
		}
	}
}

func TestDictationWordsNeverRideAFrame(t *testing.T) {
	const words = "my card number is four two four two"
	for _, phase := range spec.VoicePhaseIDs {
		if _, err := EncodeFrame(&VoiceFrame{Phase: phase, Text: words, Source: VoiceSourceDictation}); err == nil {
			t.Errorf("a %s dictation frame carried its words", phase)
		}
		raw := `{"kind":"voice","data":{"phase":"` + phase + `","text":"` + words + `","source":"dictation"}}`
		if _, err := DecodeFrame([]byte(raw)); err == nil {
			t.Errorf("a %s dictation frame with its words decoded", phase)
		}
	}
	for _, phase := range []string{VoiceListening, VoiceTranscribing} {
		out, err := EncodeFrame(&VoiceFrame{Phase: phase, Source: VoiceSourceDictation})
		if err != nil {
			t.Fatalf("a %s dictation frame: %v", phase, err)
		}
		if strings.Contains(string(out), "text") {
			t.Fatalf("a dictation frame carries a text key: %s", out)
		}
	}
	if _, err := EncodeFrame(&VoiceFrame{Phase: VoiceTranscribing, Source: VoiceSourceChord}); err == nil {
		t.Fatal("a spoken question's transcribing frame went out empty")
	}
}

func TestLevelFramesHoldFractions(t *testing.T) {
	for _, l := range []LevelFrame{{}, {Mic: 1}, {Speech: 1}, {Mic: 0.5, Speech: 0.25}} {
		if _, err := EncodeFrame(&l); err != nil {
			t.Errorf("%+v: %v", l, err)
		}
	}
	for _, l := range []LevelFrame{{Mic: math.NaN()}, {Speech: math.Inf(1)}, {Mic: -0.01}, {Speech: 1.0001}} {
		if _, err := EncodeFrame(&l); err == nil {
			t.Errorf("%+v encoded", l)
		}
	}
}
