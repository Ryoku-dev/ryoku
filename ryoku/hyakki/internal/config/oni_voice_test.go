package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"ryoku-rashin/internal/dictate"
	"ryoku-rashin/internal/oni/profile"
	"ryoku-rashin/internal/oni/voice/stt"
	"ryoku-rashin/internal/oni/voice/wake"
	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/protocol"
)

func TestOniVoiceIsOffOnAMachineThatNeverChangedASetting(t *testing.T) {
	isolateConfig(t)
	loaded, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	v := loaded.Oni.VoiceSettings
	want := protocol.OniVoiceSettings{
		Wake:      protocol.OniWakeSettings{Phrase: "hey oni"},
		Dictation: protocol.OniDictationSettings{Mode: protocol.OniDictateClean, Polish: protocol.OniPolishLocal},
	}
	if loaded.Oni.Voice || !reflect.DeepEqual(v, want) {
		t.Fatalf("voice = %v, voiceSettings = %#v", loaded.Oni.Voice, v)
	}
	if v.Wake.Enabled || v.OpenMic || v.SpeakReplies || v.Dictation.Enabled || v.Mic != "" {
		t.Fatalf("a voice switch is on by default: %#v", v)
	}
}

// The voice choices name what the voice packages accept, so a stored value
// can never reach them unrecognised.
func TestOniVoiceChoicesAreTheVoicePackagesOwn(t *testing.T) {
	if protocol.OniDefaultWakePhrase != wake.DefaultPhrase {
		t.Errorf("default wake phrase %q, the listener's %q", protocol.OniDefaultWakePhrase, wake.DefaultPhrase)
	}
	if protocol.OniWakePhraseMax != wake.MaxPhraseRunes {
		t.Errorf("wake phrase limit %d, the listener's %d", protocol.OniWakePhraseMax, wake.MaxPhraseRunes)
	}
	if want := []string{stt.EngineWhisper, stt.EngineDictate}; !reflect.DeepEqual(protocol.OniSTTEngines, want) {
		t.Errorf("speech engines %v, stt's %v", protocol.OniSTTEngines, want)
	}
	for _, m := range stt.Catalogue {
		if !protocol.ValidOniSTTModel(m.ID) {
			t.Errorf("the speech model id %q cannot be stored", m.ID)
		}
	}
	if want := []string{profile.VoiceSystem, profile.VoiceClips, profile.VoiceKokoro}; !reflect.DeepEqual(protocol.OniTTSEngines, want) {
		t.Errorf("voices %v, the profile's %v", protocol.OniTTSEngines, want)
	}
	for _, name := range protocol.OniDictateModes {
		var m dictate.Mode
		if err := m.UnmarshalText([]byte(name)); err != nil || m.String() != name {
			t.Errorf("dictation mode %q is %v, %v in dictate", name, m, err)
		}
	}
}

func TestOniVoiceSettingsSurviveASave(t *testing.T) {
	isolateConfig(t)
	next, err := Merge(Defaults(), json.RawMessage(`{"oni":{"voiceSettings":{"wake":{"enabled":true,"phrase":"  Okay, Lantern! "},"openMic":true,"speakReplies":true,"mic":"alsa_input.usb-Blue_Yeti-00.analog-stereo","stt":{"engine":"transcribecpp","model":"parakeet-tdt_ctc-110m-q8_0","language":"en"},"tts":{"engine":"kokoro"},"dictation":{"enabled":true,"mode":"polished","polish":"route"}}}}`))
	if err != nil {
		t.Fatal(err)
	}
	want := protocol.OniVoiceSettings{
		Wake:         protocol.OniWakeSettings{Enabled: true, Phrase: "okay lantern"},
		OpenMic:      true,
		SpeakReplies: true,
		Mic:          "alsa_input.usb-Blue_Yeti-00.analog-stereo",
		STT:          protocol.OniSTTSettings{Engine: protocol.OniSTTDictate, Model: "parakeet-tdt_ctc-110m-q8_0", Language: "en"},
		TTS:          protocol.OniTTSSettings{Engine: protocol.OniTTSKokoro},
		Dictation:    protocol.OniDictationSettings{Enabled: true, Mode: protocol.OniDictatePolished, Polish: protocol.OniPolishRoute},
	}
	if !reflect.DeepEqual(next.Oni.VoiceSettings, want) {
		t.Fatalf("patched voice = %#v\nwant %#v", next.Oni.VoiceSettings, want)
	}
	if next.Oni.Voice || next.Oni.Master {
		t.Fatalf("a voice settings patch turned on a switch it never named: %#v", next.Oni)
	}
	if err := Save(next); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(loaded.Oni, next.Oni) {
		t.Fatalf("after a save and a load oni = %#v\nwant %#v", loaded.Oni, next.Oni)
	}
	cleared, err := Merge(loaded, json.RawMessage(`{"oni":{"voiceSettings":{"wake":{"phrase":""},"mic":""}}}`))
	if err != nil || cleared.Oni.VoiceSettings.Wake.Phrase != wake.DefaultPhrase || cleared.Oni.VoiceSettings.Mic != "" {
		t.Fatalf("clearing the phrase and mic left %#v, %v", cleared.Oni.VoiceSettings, err)
	}
}

func TestSettingsSetRefusesBadOniVoiceValues(t *testing.T) {
	for _, patch := range []string{
		`{"oni":{"voiceSettings":{"wake":{"phrase":"oni"}}}}`,
		`{"oni":{"voiceSettings":{"wake":{"phrase":"hey there little oni friend"}}}}`,
		`{"oni":{"voiceSettings":{"wake":{"phrase":"um hey oni"}}}}`,
		`{"oni":{"voiceSettings":{"wake":{"phrase":"hey o"}}}}`,
		`{"oni":{"voiceSettings":{"wake":{"phrase":"hey [oni]"}}}}`,
		`{"oni":{"voiceSettings":{"wake":{"phrase":"hey oni\u202e"}}}}`,
		`{"oni":{"voiceSettings":{"wake":{"phrase":"hey\noni"}}}}`,
		`{"oni":{"voiceSettings":{"wake":{"phrase":"` + strings.Repeat("hey ", 11) + `"}}}}`,
		`{"oni":{"voiceSettings":{"wake":{"enabled":"yes"}}}}`,
		`{"oni":{"voiceSettings":{"mic":"/dev/snd/pcmC0D0c"}}}`,
		`{"oni":{"voiceSettings":{"mic":"-d"}}}`,
		`{"oni":{"voiceSettings":{"mic":"my mic"}}}`,
		`{"oni":{"voiceSettings":{"mic":"` + strings.Repeat("a", protocol.OniMicNodeMax+1) + `"}}}`,
		`{"oni":{"voiceSettings":{"stt":{"engine":"parakeet"}}}}`,
		`{"oni":{"voiceSettings":{"stt":{"model":"../ggml.bin"}}}}`,
		`{"oni":{"voiceSettings":{"stt":{"model":"Whisper"}}}}`,
		`{"oni":{"voiceSettings":{"stt":{"language":"english"}}}}`,
		`{"oni":{"voiceSettings":{"stt":{"language":"EN"}}}}`,
		`{"oni":{"voiceSettings":{"tts":{"engine":"piper"}}}}`,
		`{"oni":{"voiceSettings":{"dictation":{"mode":"loud"}}}}`,
		`{"oni":{"voiceSettings":{"dictation":{"mode":""}}}}`,
		`{"oni":{"voiceSettings":{"dictation":{"polish":"hosted"}}}}`,
		`{"oni":{"voiceSettings":{"openMic":1}}}`,
	} {
		if next, err := Merge(Defaults(), json.RawMessage(patch)); err == nil {
			t.Errorf("patch %s accepted: %#v", patch, next.Oni.VoiceSettings)
		}
	}
}

func TestAStoredBadOniVoiceValueLoadsAsItsDefault(t *testing.T) {
	isolateConfig(t)
	if err := os.MkdirAll(filepath.Dir(paths.ConfigFile()), 0o755); err != nil {
		t.Fatal(err)
	}
	file := `{"version":2,"oni":{"master":true,"voiceSettings":{"wake":{"enabled":true,"phrase":"oni"},"openMic":true,"mic":"../../dev/snd","stt":{"engine":"vosk","model":"A B","language":"english"},"tts":{"engine":"piper"},"dictation":{"enabled":true,"mode":"loud","polish":"cloud"}}}}`
	if err := os.WriteFile(paths.ConfigFile(), []byte(file), 0o644); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	want := protocol.DefaultOniSettings().VoiceSettings
	want.Wake.Enabled, want.OpenMic, want.Dictation.Enabled = true, true, true
	if !reflect.DeepEqual(loaded.Oni.VoiceSettings, want) {
		t.Fatalf("loaded voice = %#v\nwant %#v", loaded.Oni.VoiceSettings, want)
	}
	// a file from before the voice settings loads them at their defaults
	if err := os.WriteFile(paths.ConfigFile(), []byte(`{"version":2,"oni":{"master":true,"voice":true}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if loaded, err = Load(); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(loaded.Oni.VoiceSettings, protocol.DefaultOniSettings().VoiceSettings) || !loaded.Oni.Voice {
		t.Fatalf("an older file loads oni as %#v", loaded.Oni)
	}
}
