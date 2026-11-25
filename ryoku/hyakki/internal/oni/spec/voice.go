package spec

import (
	"math"
	"slices"
)

// VoicePhaseIDs are the voice phases a VoiceFrame carries (see Voice.Phases).
var VoicePhaseIDs = []string{"idle", "armed", "listening", "transcribing", "speaking"}

// NoState is the Voice.Phases state of a phase that leaves the character's
// state alone.
const NoState = "none"

// Voice holds the voice loop's numbers: the segmenter (Vad), the decode
// budgets, the press, wake and turn limits, engine unload times, the level
// frame rate, transcript hygiene and the voice phase to character state map.
type Voice struct {
	Vad     VoiceVad              `json:"vad"`
	Budgets VoiceBudgets          `json:"budgets"`
	Press   VoicePress            `json:"press"`
	Wake    VoiceWake             `json:"wake"`
	Turn    VoiceTurn             `json:"turn"`
	Engines VoiceEngines          `json:"engines"`
	Levels  VoiceLevels           `json:"levels"`
	Hygiene VoiceHygiene          `json:"hygiene"`
	Phases  map[string]VoicePhase `json:"phases"`
}

// VoiceVad is the energy segmenter over 16 kHz s16 mono capture: a frame of
// FrameMs is speech when its RMS reaches SpeechRms. Every timing is a whole
// number of frames.
type VoiceVad struct {
	SampleRateHz   int     `json:"sampleRateHz"`
	FrameMs        float64 `json:"frameMs"`
	SpeechRms      float64 `json:"speechRms"`
	PauseMs        float64 `json:"pauseMs"`
	MinSpeechMs    float64 `json:"minSpeechMs"`
	FirstPartialMs float64 `json:"firstPartialMs"`
	PartialEveryMs float64 `json:"partialEveryMs"`
	MaxSegmentS    float64 `json:"maxSegmentS"`
	PreRollMs      float64 `json:"preRollMs"`
}

// VoiceBudgets bound one decode: a partial abandoned after PartialMs, a
// final after FinalS.
type VoiceBudgets struct {
	PartialMs float64 `json:"partialMs"`
	FinalS    float64 `json:"finalS"`
}

// VoicePress: a hold shorter than TapDropMs with nothing heard is dropped;
// a hold is cut at MaxHoldS; open mic ends at OpenMicS.
type VoicePress struct {
	TapDropMs float64 `json:"tapDropMs"`
	MaxHoldS  float64 `json:"maxHoldS"`
	OpenMicS  float64 `json:"openMicS"`
}

// VoiceWake: a session the wake phrase opens listens at most SessionS; wake
// stays armed at most ArmedMaxS, then disarms.
type VoiceWake struct {
	SessionS  float64 `json:"sessionS"`
	ArmedMaxS float64 `json:"armedMaxS"`
}

// VoiceTurn: acknowledgements at least AckGapMs apart; after a spoken line
// of n characters the mic ignores itself for EchoBaseMs + n·EchoPerCharMs.
type VoiceTurn struct {
	AckGapMs      float64 `json:"ackGapMs"`
	EchoBaseMs    float64 `json:"echoBaseMs"`
	EchoPerCharMs float64 `json:"echoPerCharMs"`
}

// VoiceEngines are the idle times after which a resident engine unloads.
type VoiceEngines struct {
	SttIdleUnloadS float64 `json:"sttIdleUnloadS"`
	TtsIdleUnloadS float64 `json:"ttsIdleUnloadS"`
}

// VoiceLevels caps the mic and speech level frames.
type VoiceLevels struct {
	MaxHz float64 `json:"maxHz"`
}

// VoiceHygiene: a phrase of at most LoopMaxWords words repeated LoopRepeats
// times in a row is kept once; a transcript is plausible with at most
// ceil(seconds·WordsPerS)+SlackWords words; a sentence of at least
// SentenceMinWords words that repeats an earlier one is dropped.
type VoiceHygiene struct {
	LoopRepeats      int     `json:"loopRepeats"`
	LoopMaxWords     int     `json:"loopMaxWords"`
	WordsPerS        float64 `json:"wordsPerS"`
	SlackWords       int     `json:"slackWords"`
	SentenceMinWords int     `json:"sentenceMinWords"`
}

// VoicePhase is the character state a voice phase derives (NoState leaves
// it alone) and whether the presenter draws the persistent mic mark.
type VoicePhase struct {
	State   string `json:"state"`
	MicMark bool   `json:"micMark"`
}

// VoiceState returns the character state a voice phase derives (NoState
// when the phase changes none) and whether it shows the mic mark.
func (s *OniSpec) VoiceState(phase string) (state string, micMark, ok bool) {
	p, ok := s.Voice.Phases[phase]
	return p.State, p.MicMark, ok
}

// EchoMs is how long the mic ignores itself after Oni speaks a line of
// chars characters.
func (s *OniSpec) EchoMs(chars int) float64 {
	t := s.Voice.Turn
	return t.EchoBaseMs + float64(max(chars, 0))*t.EchoPerCharMs
}

func (s *OniSpec) checkVoice(bad func(string, ...any)) {
	v := s.Voice
	vad := v.Vad
	if vad.SampleRateHz != 16000 {
		bad("voice.vad.sampleRateHz: the speech engines read 16000 Hz only")
	}
	if fs := float64(vad.SampleRateHz) * vad.FrameMs / 1000; !(vad.FrameMs > 0) || fs != math.Trunc(fs) {
		bad("voice.vad.frameMs: must be positive and a whole number of samples")
	} else {
		for _, d := range []struct {
			path string
			ms   float64
		}{
			{"voice.vad.pauseMs", vad.PauseMs},
			{"voice.vad.minSpeechMs", vad.MinSpeechMs},
			{"voice.vad.firstPartialMs", vad.FirstPartialMs},
			{"voice.vad.partialEveryMs", vad.PartialEveryMs},
			{"voice.vad.maxSegmentS", vad.MaxSegmentS * 1000},
			{"voice.vad.preRollMs", vad.PreRollMs},
		} {
			if n := d.ms / vad.FrameMs; n != math.Trunc(n) {
				bad("%s: must be a whole number of voice.vad.frameMs", d.path)
			}
		}
	}
	if !(vad.SpeechRms > 0 && vad.SpeechRms < 32768) {
		bad("voice.vad.speechRms: must lie in (0, 32768)")
	}
	for _, d := range []struct {
		path string
		v    float64
	}{
		{"voice.vad.pauseMs", vad.PauseMs},
		{"voice.vad.minSpeechMs", vad.MinSpeechMs},
		{"voice.vad.partialEveryMs", vad.PartialEveryMs},
		{"voice.budgets.partialMs", v.Budgets.PartialMs},
		{"voice.budgets.finalS", v.Budgets.FinalS},
		{"voice.press.tapDropMs", v.Press.TapDropMs},
		{"voice.wake.sessionS", v.Wake.SessionS},
		{"voice.turn.ackGapMs", v.Turn.AckGapMs},
		{"voice.turn.echoBaseMs", v.Turn.EchoBaseMs},
		{"voice.engines.sttIdleUnloadS", v.Engines.SttIdleUnloadS},
		{"voice.engines.ttsIdleUnloadS", v.Engines.TtsIdleUnloadS},
		{"voice.hygiene.wordsPerS", v.Hygiene.WordsPerS},
	} {
		if !(d.v > 0) {
			bad("%s: must be positive", d.path)
		}
	}
	if !(vad.MinSpeechMs < vad.FirstPartialMs) {
		bad("voice.vad.firstPartialMs: speech must start before the first partial (more than minSpeechMs)")
	}
	if !(vad.FirstPartialMs < vad.MaxSegmentS*1000) {
		bad("voice.vad.maxSegmentS: must outlast the first partial")
	}
	if !(v.Budgets.PartialMs < v.Budgets.FinalS*1000) {
		bad("voice.budgets.partialMs: must be shorter than the final budget")
	}
	if !(v.Press.TapDropMs < v.Press.MaxHoldS*1000) {
		bad("voice.press.maxHoldS: must outlast the tap drop")
	}
	if !(vad.MaxSegmentS <= v.Press.MaxHoldS) {
		bad("voice.press.maxHoldS: a hold must fit a whole segment (at least maxSegmentS)")
	}
	if !(v.Press.MaxHoldS < v.Press.OpenMicS) {
		bad("voice.press.openMicS: must outlast a hold")
	}
	if !(v.Wake.SessionS <= v.Press.OpenMicS) {
		bad("voice.wake.sessionS: a wake session may not outlast open mic")
	}
	if !(v.Wake.SessionS < v.Wake.ArmedMaxS) {
		bad("voice.wake.armedMaxS: must outlast a wake session")
	}
	if hz := v.Levels.MaxHz; !(hz > 0 && hz <= float64(s.Frames.TiersFps["active"])) {
		bad("voice.levels.maxHz: must lie in (0, frames.tiersFps.active]")
	}
	h := v.Hygiene
	if h.LoopRepeats < 2 {
		bad("voice.hygiene.loopRepeats: a loop needs at least 2 repeats")
	}
	if h.LoopMaxWords < 1 {
		bad("voice.hygiene.loopMaxWords: must be at least 1")
	}
	if h.SentenceMinWords < 1 {
		bad("voice.hygiene.sentenceMinWords: must be at least 1")
	}

	if err := sameSet("voice.phases", v.Phases, VoicePhaseIDs); err != nil {
		bad("%v", err)
	}
	for _, id := range sortedKeys(v.Phases) {
		p := v.Phases[id]
		if p.State != NoState && !slices.Contains(StateIDs, p.State) {
			bad("voice.phases.%s.state: unknown state %q", id, p.State)
		}
	}
	if idle, ok := v.Phases["idle"]; ok && (idle.State != NoState || idle.MicMark) {
		bad("voice.phases.idle: a quiet mic derives no state and shows no mic mark")
	}
	for _, id := range sortedKeys(v.Phases) {
		if p := v.Phases[id]; id != "idle" && p.State == NoState && !p.MicMark {
			bad("voice.phases.%s: a live mic that changes no state must show the mic mark", id)
		}
	}
}
