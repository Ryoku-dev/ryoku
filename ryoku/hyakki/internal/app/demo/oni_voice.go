package demo

import (
	"encoding/json"
	"math"
	"slices"
	"unicode/utf8"

	"ryoku-i18n"

	"ryoku-rashin/internal/dictate"
	"ryoku-rashin/internal/oni/feature"
	"ryoku-rashin/internal/protocol"
)

// FeatureOniVoice is no feature: it starts the demo with Oni on and both
// talking to Oni and dictation switched on, for the voice settings and the
// CLI's voice verbs.
const FeatureOniVoice = "oni-voice"

// FeatureOniNoMic is no feature: the demo machine has no microphone, so the
// meter's test answers as the daemon does when no sound arrives.
const FeatureOniNoMic = "oni-no-mic"

// the demo's capture sources, as the daemon lists them without monitors
var demoMicSources = []protocol.OniMicSource{
	{Name: "alsa_input.usb-Yeti_Stereo_Microphone-00.analog-stereo", Desc: "Yeti Stereo Microphone"},
	{Name: "alsa_input.pci-0000_00_1f.3.analog-stereo", Desc: "Built-in Audio Analog Stereo"},
}

// demoDictation is what the demo hears for every dictation, as a speech
// engine writes it: fillers, spoken marks and all
const demoDictation = "um meet me at the north gate at five period bring the lantern comma and the map period"

// voiceFixtures is the demo's voice loop: the phase its last call left, and
// whether a dictation is open, with the mode it was opened in
type voiceFixtures struct {
	noMic     bool
	phase     string
	dictating bool
	mode      string
}

func newVoiceFixtures(noMic bool) voiceFixtures {
	return voiceFixtures{noMic: noMic, phase: feature.VoiceIdle}
}

// the demo's voice calls answer as the daemon's do: the switches decide,
// and nothing listens to a real microphone or types anywhere
func (s *server) callOniVoice(method string, raw json.RawMessage) (any, error) {
	switch method {
	case protocol.MOniTalk:
		var params protocol.OniTalkParams
		if err := decode(raw, &params); err != nil {
			return nil, err
		}
		if err := s.voiceAllowed(feature.Voice, params.Action); err != nil {
			return nil, err
		}
		return protocol.OniVoiceResult{Phase: s.data.oni.voice.talk(params.Action)}, nil
	case protocol.MOniSpeak:
		var params protocol.OniSpeakParams
		if err := decode(raw, &params); err != nil {
			return nil, err
		}
		if params.Action != protocol.OniVoiceStop {
			return nil, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("Oni can only be told to stop speaking."))
		}
		if err := s.voiceAllowed(feature.Voice, ""); err != nil {
			return nil, err
		}
		voice := &s.data.oni.voice
		if voice.phase == feature.VoiceSpeaking {
			voice.phase = feature.VoiceIdle
		}
		return protocol.OniVoiceResult{Phase: voice.phase}, nil
	case protocol.MOniDictate:
		var params protocol.OniDictateParams
		if err := decode(raw, &params); err != nil {
			return nil, err
		}
		if params.Mode != "" && !slices.Contains(protocol.OniDictateModes, params.Mode) {
			return nil, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("Dictation types exact, clean or polished text."))
		}
		if err := s.voiceAllowed(feature.VoiceDictation, params.Action); err != nil {
			return nil, err
		}
		return s.data.oni.voice.dictate(params, s.data.snapshot.Settings.Oni.VoiceSettings.Dictation.Mode)
	case protocol.MOniVoiceTest:
		var params protocol.OniVoiceTestParams
		if err := decode(raw, &params); err != nil {
			return nil, err
		}
		return s.data.oni.voice.test(params, s.data.snapshot.Settings.Oni.VoiceSettings.Mic)
	}
	return nil, notFound(method)
}

// voiceAllowed refuses an unknown action, then a feature that is off, as
// the daemon's policy does
func (s *server) voiceAllowed(id, action string) error {
	if action != "" && !slices.Contains(protocol.OniVoiceActions, action) {
		return protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("The action is start, stop or cancel."))
	}
	decision := feature.Decide(feature.Request{Feature: id}, protocol.EffectiveOni(s.data.snapshot.Settings))
	switch {
	case decision.Allow:
		return nil
	case decision.Reason == feature.ReasonOniOff:
		return &protocol.Error{Code: protocol.CodeDisabled, Reason: decision.Reason, Message: i18n.T("Oni is off. Turn it on in Hyakki's Settings first.")}
	case id == feature.VoiceDictation:
		return &protocol.Error{Code: protocol.CodeDisabled, Reason: decision.Reason, Message: i18n.T("Dictation is off. Turn it on in Settings, under Oni.")}
	}
	return &protocol.Error{Code: protocol.CodeDisabled, Reason: decision.Reason, Message: i18n.T("Talking to Oni is off. Turn it on in Settings, under Oni.")}
}

// talk opens the microphone on start, hands what was heard on to be
// transcribed on stop, and drops it on cancel
func (v *voiceFixtures) talk(action string) string {
	switch action {
	case protocol.OniVoiceStart:
		v.phase = feature.VoiceListening
	case protocol.OniVoiceStop:
		if v.phase == feature.VoiceListening {
			v.phase = feature.VoiceTranscribing
		} else {
			v.phase = feature.VoiceIdle
		}
	default:
		v.phase = feature.VoiceIdle
	}
	return v.phase
}

// dictate opens a dictation, or ends it the way the demo's always ends: the
// first sentence is typed, then the notes window closes and the rest stays
// with the caller
func (v *voiceFixtures) dictate(params protocol.OniDictateParams, setting string) (protocol.OniDictateResult, error) {
	switch params.Action {
	case protocol.OniVoiceStart:
		v.dictating, v.phase, v.mode = true, feature.VoiceListening, params.Mode
		if v.mode == "" {
			v.mode = setting
		}
		return protocol.OniDictateResult{Phase: v.phase}, nil
	case protocol.OniVoiceCancel:
		v.dictating, v.phase = false, feature.VoiceIdle
		return protocol.OniDictateResult{Phase: v.phase}, nil
	}
	if !v.dictating {
		v.phase = feature.VoiceIdle
		return protocol.OniDictateResult{Phase: v.phase}, nil
	}
	v.dictating, v.phase = false, feature.VoiceIdle
	mode, err := dictate.ParseMode(v.mode)
	if err != nil {
		return protocol.OniDictateResult{}, err
	}
	processed, err := dictate.Process(demoDictation, dictate.Options{Mode: mode})
	if err != nil {
		return protocol.OniDictateResult{}, err
	}
	landed, rest := demoTyped(processed.Text)
	return protocol.OniDictateResult{
		Phase: v.phase, Mode: processed.Mode.String(), Fallback: string(processed.Fallback),
		Landed: utf8.RuneCountInString(landed), Rest: rest, Refused: "closed",
	}, nil
}

// demoTyped splits text after its first sentence: what reached the window
// before it closed, and what did not
func demoTyped(text string) (landed, rest string) {
	for index, r := range text {
		if r == '.' || r == '?' || r == '!' {
			return text[:index+1], text[index+1:]
		}
	}
	return "", text
}

// test answers the meter with levels drawn from the source's own curve, or
// as the daemon does when no sound arrives
func (v *voiceFixtures) test(params protocol.OniVoiceTestParams, setting string) (protocol.OniVoiceTestResult, error) {
	if params.Ms != 0 && (params.Ms < protocol.OniVoiceTestMinMs || params.Ms > protocol.OniVoiceTestMaxMs) {
		return protocol.OniVoiceTestResult{}, protocol.Errorf(protocol.CodeBadRequest, "%s",
			i18n.Tf("A microphone test listens for %d to %d milliseconds.", protocol.OniVoiceTestMinMs, protocol.OniVoiceTestMaxMs))
	}
	mic := params.Mic
	if mic == "" {
		mic = setting
	}
	if mic != "" && !protocol.ValidOniMicNode(mic) {
		return protocol.OniVoiceTestResult{}, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("The microphone is not a PipeWire source name."))
	}
	if v.noMic {
		return protocol.OniVoiceTestResult{}, &protocol.Error{Code: protocol.CodeUnavailable, Reason: protocol.OniReasonNoMicrophone,
			Message: i18n.T("No sound arrived from the microphone. It may be unplugged, muted or blocked.")}
	}
	loudness := 1.0
	if mic != "" {
		index := slices.IndexFunc(demoMicSources, func(source protocol.OniMicSource) bool { return source.Name == mic })
		if index < 0 {
			return protocol.OniVoiceTestResult{}, &protocol.Error{Code: protocol.CodeUnavailable, Reason: protocol.OniReasonNoMicrophone,
				Message: i18n.T("No sound arrived from the microphone. It may be unplugged, muted or blocked.")}
		}
		loudness = 1 / float64(index+1)
	}
	ms := params.Ms
	if ms == 0 {
		ms = protocol.OniVoiceTestDefaultMs
	}
	levels := make([]float64, ms/100)
	for index := range levels {
		level := loudness * (0.08 + 0.7*math.Abs(math.Sin(float64(index)*0.7)))
		levels[index] = math.Round(level*1000) / 1000
	}
	return protocol.OniVoiceTestResult{Mic: mic, Levels: levels, Sources: slices.Clone(demoMicSources)}, nil
}
