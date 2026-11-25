package daemon

import (
	"context"
	"errors"
	"os"
	"slices"

	i18n "ryoku-i18n"
	"ryoku-rashin/internal/dictate"
	"ryoku-rashin/internal/dictate/insert"
	"ryoku-rashin/internal/oni/presence"
	"ryoku-rashin/internal/oni/profile"
	"ryoku-rashin/internal/oni/sound"
	"ryoku-rashin/internal/oni/spec"
	"ryoku-rashin/internal/oni/voice/audio"
	"ryoku-rashin/internal/oni/voice/clips"
	"ryoku-rashin/internal/oni/voice/stt"
	"ryoku-rashin/internal/oni/voice/tts"
	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/protocol"
	"ryoku-rashin/internal/writing"
)

// OniVoiceDeps are the voice loop's parts. A nil field is the real one, so
// a test runs the whole loop with no microphone, speech engine, speaker or
// typing tool. Nothing here starts until a press, a dictation, the meter or
// an armed wake phrase or open mic asks for it.
type OniVoiceDeps struct {
	// Capture records the microphone, one press at a time; nil is pw-cat,
	// else parec.
	Capture audio.Capture
	// Sources lists the microphones the chooser offers; nil asks pactl.
	Sources func(ctx context.Context) ([]audio.Source, error)
	// STT builds the speech-to-text engine the settings name, for the
	// character called character; nil builds whisper.cpp or transcribe.cpp
	// over a downloaded model.
	STT func(sp *spec.OniSpec, s protocol.OniSTTSettings, character string) (stt.Transcriber, error)
	// Voices are the voice engines to try, in order; nil finds the system
	// voice, the character's clip pack and the Kokoro pack.
	Voices func(s protocol.OniTTSSettings, p profile.Profile) []tts.Engine
	// Sink plays speech; nil is sound.PipeSink.
	Sink func() (sound.Sink, error)
	// Type runs the typing tool for dictation; nil is wtype.
	Type insert.Runner
	// Store holds the dictation dictionary, snippets and styles; nil is the
	// user's.
	Store *dictate.Store
	// Clock arms the voice loop's timers; nil is the wall clock.
	Clock presence.Clock
}

func (d *Daemon) voiceClock() presence.Clock {
	if d.deps.OniVoice.Clock != nil {
		return d.deps.OniVoice.Clock
	}
	return presence.SystemClock()
}

func (d *Daemon) voiceCapture() audio.Capture {
	if d.deps.OniVoice.Capture != nil {
		return d.deps.OniVoice.Capture
	}
	return audio.NewRecorder(nil)
}

func (d *Daemon) voiceSources(ctx context.Context) ([]audio.Source, error) {
	if d.deps.OniVoice.Sources != nil {
		return d.deps.OniVoice.Sources(ctx)
	}
	return audio.Sources(ctx, audio.Exec(nil))
}

func (d *Daemon) voiceSTT(sp *spec.OniSpec, s protocol.OniSTTSettings, character string) (stt.Transcriber, error) {
	if d.deps.OniVoice.STT != nil {
		return d.deps.OniVoice.STT(sp, s, character)
	}
	return buildSTT(sp, s, character)
}

func (d *Daemon) voiceVoices(s protocol.OniTTSSettings, p profile.Profile) []tts.Engine {
	if d.deps.OniVoice.Voices != nil {
		return d.deps.OniVoice.Voices(s, p)
	}
	return buildVoices(s, p)
}

func (d *Daemon) voiceSink() (sound.Sink, error) {
	if d.deps.OniVoice.Sink != nil {
		return d.deps.OniVoice.Sink()
	}
	return sound.PipeSink()
}

func (d *Daemon) voiceTyper() insert.Runner {
	if d.deps.OniVoice.Type != nil {
		return d.deps.OniVoice.Type
	}
	return insert.Exec(os.Environ())
}

func (d *Daemon) voiceStore() *dictate.Store {
	if d.deps.OniVoice.Store != nil {
		return d.deps.OniVoice.Store
	}
	return dictate.NewStore("")
}

// errNoSpeechModel: no model the settings allow is downloaded.
var errNoSpeechModel = errors.New("no speech model is downloaded")

// buildSTT builds the engine for the settings: the model they name, or with
// none named the default and then the fallback model, each only when it is
// downloaded (or, for a Parakeet model, where Ryoku keeps it). The fallback
// runs with its VAD model (stt.FallbackVAD) when that is downloaded, as the
// bench measured it. It starts nothing; a resident engine starts on its
// first decode.
func buildSTT(sp *spec.OniSpec, s protocol.OniSTTSettings, character string) (stt.Transcriber, error) {
	ids := []string{s.Model}
	if s.Model == "" {
		ids = []string{stt.DefaultModel, stt.FallbackModel}
		for _, m := range stt.Catalogue {
			ids = append(ids, m.ID)
		}
	}
	limits := stt.LimitsFromSpec(sp)
	var tried []string
	err := errNoSpeechModel
	for _, id := range ids {
		m, ok := stt.FindModel(id)
		if !ok || slices.Contains(tried, id) || m.Engine == stt.EngineVAD || s.Engine != "" && m.Engine != s.Engine {
			continue
		}
		tried = append(tried, id)
		path, ok := stt.Locate(m)
		if !ok {
			continue
		}
		var t stt.Transcriber
		var buildErr error
		switch m.Engine {
		case stt.EngineWhisper:
			c := stt.WhisperConfig{Model: path, SHA256: m.SHA256, License: m.License, Language: s.Language, Character: character, Limits: limits}
			if vad, ok := stt.FindModel(stt.FallbackVAD); ok && m.ID == stt.FallbackModel {
				if vadPath, ok := stt.Locate(vad); ok {
					c.VADModel, c.VADSHA256 = vadPath, vad.SHA256
				}
			}
			t, buildErr = stt.NewWhisper(c)
		case stt.EngineDictate:
			language := s.Language
			if language == "auto" {
				language = ""
			}
			t, buildErr = stt.NewDictate(stt.DictateConfig{Model: path, SHA256: m.SHA256, License: m.License, Language: language, Limits: limits})
		default:
			continue
		}
		if buildErr == nil {
			return t, nil
		}
		if err == errNoSpeechModel || errors.Is(buildErr, stt.ErrMissingEngine) {
			err = buildErr
		}
	}
	return nil, err
}

// sttRefusal says why the speech engine cannot run.
func sttRefusal(err error) error {
	if errors.Is(err, stt.ErrMissingEngine) {
		return &protocol.Error{Code: protocol.CodeUnavailable, Reason: protocol.OniReasonNoSpeechEngine, Message: i18n.T("The speech-to-text program is not installed.")}
	}
	if errors.Is(err, errNoSpeechModel) || errors.Is(err, stt.ErrModel) {
		return &protocol.Error{Code: protocol.CodeUnavailable, Reason: protocol.OniReasonNoSpeechModel, Message: i18n.T("Download a speech model in Oni's voice settings first.")}
	}
	return &protocol.Error{Code: protocol.CodeUnavailable, Reason: protocol.OniReasonNoSpeechEngine, Message: i18n.Tf("The speech engine cannot run: %v.", err)}
}

// micRefusal says why the microphone did not open.
func micRefusal(err error) error {
	switch {
	case errors.Is(err, audio.ErrNoRecorder):
		return &protocol.Error{Code: protocol.CodeUnavailable, Reason: protocol.OniReasonNoRecorder, Message: i18n.T("Neither pw-cat nor parec is installed, so the microphone cannot be read.")}
	case errors.Is(err, audio.ErrNoAudio):
		return &protocol.Error{Code: protocol.CodeUnavailable, Reason: protocol.OniReasonNoMicrophone, Message: i18n.T("The microphone gave no sound. Check that one is connected and chosen.")}
	case errors.Is(err, audio.ErrBadNode):
		return protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("That is not a microphone name."))
	}
	return &protocol.Error{Code: protocol.CodeUnavailable, Reason: protocol.OniReasonMicFailed, Message: i18n.Tf("The microphone could not be read: %v.", err)}
}

// buildVoices finds the voices the settings and the character allow, in the
// order tts.Order gives: the setting's, the character's, then the system
// voice. One that is not installed is left out; nothing is started.
func buildVoices(s protocol.OniTTSSettings, p profile.Profile) []tts.Engine {
	have := map[string]tts.Engine{}
	if e, err := tts.NewSystem(tts.SystemOptions{Speed: p.Voice.Speed}); err == nil {
		have[tts.System] = e
	}
	if p.Voice.Engine == profile.VoiceClips && !p.Shipped {
		if pack, err := clips.ForProfile(os.DirFS(paths.CharactersDir()), p.ID, p.Voice); err == nil {
			have[tts.Clips] = tts.NewClips(pack, p.ID)
		}
	}
	if e, err := tts.FindKokoro(tts.KokoroOptions{Speaker: p.Voice.Speaker, Speed: p.Voice.Speed}); err == nil {
		have[tts.Kokoro] = e
	}
	return tts.Pick(tts.Order(s.Engine, p.Voice.Engine), have)
}

// oniActiveProfile is the selected character when it loads, else the
// shipped one.
func oniActiveProfile(selected string) profile.Profile {
	for _, l := range profile.List(paths.CharactersDir()) {
		if l.Valid && l.Profile.ID == selected {
			return l.Profile
		}
	}
	return profile.Default().Profile
}

// voiceVocab is the vocabulary hint for a press or a dictation: the written
// forms of the dictation dictionary, then the user's writing words, as
// stt.Vocabulary keeps them (trimmed, deduplicated, at most stt.MaxVocab,
// none holding the character's name). Nil when there are none. It is read
// when the microphone opens, so an edit counts from the next press.
func (d *Daemon) voiceVocab(character string) []string {
	var names []string
	if personal, err := d.voiceStore().Load(); err == nil {
		for _, e := range personal.Dictionary {
			names = append(names, e.Written)
		}
	}
	if words, err := writing.WordsGet(); err == nil {
		names = append(names, words.Words...)
	}
	vocab := stt.Vocabulary(names, character)
	if len(vocab) == 0 {
		return nil
	}
	return vocab
}
