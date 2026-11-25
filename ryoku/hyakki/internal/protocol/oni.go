package protocol

import (
	"encoding/json"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Oni's hands stages, exclusive: choosing one never enables another.
// OniHandsBrowser lets Hands work in one browser tab the user authorized;
// OniHandsApp in one live window the user picked, or on Hyakki's own nested
// desktop; OniHandsDesktop on the nested desktop. No stage reaches the
// user's whole live desktop. Each is an app-only setting.
const (
	OniHandsOff     = "off"
	OniHandsBrowser = "browser"
	OniHandsApp     = "app"
	OniHandsDesktop = "desktop"
)

// OniHandsStages are every stage the contract names, available or not.
var OniHandsStages = []string{OniHandsOff, OniHandsBrowser, OniHandsApp, OniHandsDesktop}

// Oni capture disclosure modes: ask before the first capture of a session
// leaves the computer, or before every one.
const (
	OniDisclosureSession = "session"
	OniDisclosureAlways  = "always"
)

// OniDefaultCharacter is the shipped character; no user character may take
// its id.
const OniDefaultCharacter = "oni"

// OniDefaultChord is the hold-to-talk chord: a modifier-only chord, read from
// raw key events rather than registered as a shortcut.
const OniDefaultChord = "ctrl+alt"

// Limits on Settings.oni values.
const (
	OniCharacterIDMax   = 32
	OniFeedWordsMax     = 32
	OniFeedWordMaxRunes = 40
)

// Oni's sizes on screen.
const (
	OniSizeSmall  = "small"
	OniSizeNormal = "normal"
	OniSizeBig    = "big"
)

// OniSizes are every size, smallest first.
var OniSizes = []string{OniSizeSmall, OniSizeNormal, OniSizeBig}

// Oni's reduced-motion modes: follow the desktop's setting, or force it.
const (
	OniReduceFollow = "follow"
	OniReduceOn     = "on"
	OniReduceOff    = "off"
)

// OniReduceModes are every reduced-motion mode.
var OniReduceModes = []string{OniReduceFollow, OniReduceOn, OniReduceOff}

// OniSettings is Settings.oni. Every feature is off until the user turns it
// on, and Oni as a whole is off while Hyakki is off (EffectiveOni). Size,
// ReduceMotion, Courtesy and Hidden change only how Oni is drawn: Courtesy
// steps Oni aside while a window covers it or is fullscreen, Hidden keeps it
// off screen without turning it off. Sounds lets the renderer play Oni's
// short motifs; it is off until the user turns it on. PerchFollowsFocus
// moves a perched Oni to the window that takes focus; off, a perch stays on
// its window. VoiceSettings is how the voice loop and dictation run; Voice
// is still the one switch for talking to Oni. Guide is the guided
// walkthroughs, Keep and proactive hints. ReflexSettings are the switches
// under Reflex, each of which counts only while Reflex is on. Recall,
// Rewrite and Assignment are recall, rewrite in place and assignment mode.
// MCP is the MCP bridge other programs reach Oni's tools through.
type OniSettings struct {
	Master            bool                  `json:"master"`
	Character         string                `json:"character"`
	Inspect           bool                  `json:"inspect"`
	Disclosure        string                `json:"disclosure"`
	Voice             bool                  `json:"voice"`
	Chord             string                `json:"chord"`
	Hands             string                `json:"hands"`
	Reflex            bool                  `json:"reflex"`
	Feeds             OniFeedSettings       `json:"feeds"`
	Size              string                `json:"size"`
	ReduceMotion      string                `json:"reduceMotion"`
	Courtesy          bool                  `json:"courtesy"`
	Hidden            bool                  `json:"hidden"`
	Sounds            bool                  `json:"sounds"`
	PerchFollowsFocus bool                  `json:"perchFollowsFocus"`
	VoiceSettings     OniVoiceSettings      `json:"voiceSettings"`
	Guide             OniGuideSettings      `json:"guide"`
	ReflexSettings    OniReflexSettings     `json:"reflexSettings"`
	Recall            OniRecallSettings     `json:"recall"`
	Rewrite           OniRewriteSettings    `json:"rewrite"`
	Assignment        OniAssignmentSettings `json:"assignment"`
	MCP               OniMCPSettings        `json:"mcp"`
}

// OniReflexSettings is Settings.oni.reflexSettings; every switch is off
// until the user turns it on, and each counts only while Reflex is on.
// Routines offers to keep a run of quick commands, asking each time, and
// replays one with no model. Reminders keeps reminders and timers on this
// computer and says them when due. Notes keeps "remember that" as a memory
// fact, never in a store of its own. Decisions lets the typed decision
// route in providers.decisions classify what the rules miss; a hosted
// decision also needs a press and a disclosure card answered by one.
type OniReflexSettings struct {
	Routines  bool `json:"routines"`
	Reminders bool `json:"reminders"`
	Notes     bool `json:"notes"`
	Decisions bool `json:"decisions"`
}

// OniGuideSettings is Settings.oni.guide; every switch is off until the user
// turns it on. Enabled is the guide feature, which also needs Inspect: a
// guide reads the perched window one step at a time. Keep offers to keep a
// finished guide, asking each time, and replays kept guides on this
// computer; it needs the guide. Hints offers a kept guide while Oni is
// perched on its app; it needs Keep.
type OniGuideSettings struct {
	Enabled bool `json:"enabled"`
	Keep    bool `json:"keep"`
	Hints   bool `json:"hints"`
}

// OniFeedSettings is the focus guard: Words name the feeds it may close.
type OniFeedSettings struct {
	Guard bool     `json:"guard"`
	Words []string `json:"words,omitempty"`
}

// OniDefaultWakePhrase is the wake phrase until the user changes it.
const OniDefaultWakePhrase = "hey oni"

// Speech-to-text engines: whisper.cpp's resident server, or transcribe.cpp
// through ryoku-dictate. An empty engine is the one the bench picked.
const (
	OniSTTWhisper = "whispercpp"
	OniSTTDictate = "transcribecpp"
)

// OniSTTEngines are every speech-to-text engine a setting may name.
var OniSTTEngines = []string{OniSTTWhisper, OniSTTDictate}

// Voices Oni speaks with: the system speech synthesizer, a recorded clip
// pack, or the optional Kokoro pack. An empty engine is the character's own
// voice.engine.
const (
	OniTTSSystem = "system"
	OniTTSClips  = "clips"
	OniTTSKokoro = "kokoro"
)

// OniTTSEngines are every voice engine a setting may name.
var OniTTSEngines = []string{OniTTSSystem, OniTTSClips, OniTTSKokoro}

// Dictation modes: the words as heard, tidied by fixed rules, or rewritten
// by a model with the tidied text as the fallback.
const (
	OniDictateExact    = "exact"
	OniDictateClean    = "clean"
	OniDictatePolished = "polished"
)

// OniDictateModes are every dictation mode.
var OniDictateModes = []string{OniDictateExact, OniDictateClean, OniDictatePolished}

// Where a polished dictation is rewritten: a model on this computer, or the
// Ask route, which may be hosted and is disclosed before anything leaves.
const (
	OniPolishLocal = "local"
	OniPolishRoute = "route"
)

// OniPolishRoutes are every polish route.
var OniPolishRoutes = []string{OniPolishLocal, OniPolishRoute}

// Limits on Settings.oni.voiceSettings values.
const (
	OniMicNodeMax    = 255
	OniSTTModelMax   = 64
	OniWakePhraseMax = 40
)

// OniVoiceSettings is Settings.oni.voiceSettings. Every switch is off and
// every choice empty until the user changes it. Wake and OpenMic keep the
// microphone open between presses (Settings calls both "continuous
// microphone"); SpeakReplies speaks answers aloud. Mic is a PipeWire source
// node, empty for the default source.
type OniVoiceSettings struct {
	Wake         OniWakeSettings      `json:"wake"`
	OpenMic      bool                 `json:"openMic"`
	SpeakReplies bool                 `json:"speakReplies"`
	Mic          string               `json:"mic"`
	STT          OniSTTSettings       `json:"stt"`
	TTS          OniTTSSettings       `json:"tts"`
	Dictation    OniDictationSettings `json:"dictation"`
}

// OniWakeSettings arms the wake phrase, stored as compared: lowercase words
// without punctuation.
type OniWakeSettings struct {
	Enabled bool   `json:"enabled"`
	Phrase  string `json:"phrase"`
}

// OniSTTSettings picks the speech-to-text engine, its model id and the
// spoken language; empty fields are the defaults.
type OniSTTSettings struct {
	Engine   string `json:"engine"`
	Model    string `json:"model"`
	Language string `json:"language"`
}

// OniTTSSettings picks the voice Oni speaks with; empty is the character's.
type OniTTSSettings struct {
	Engine string `json:"engine"`
}

// OniDictationSettings is the voice.dictation feature: its switch, the mode
// it types in and where a polished dictation is rewritten.
type OniDictationSettings struct {
	Enabled bool   `json:"enabled"`
	Mode    string `json:"mode"`
	Polish  string `json:"polish"`
}

// DefaultOniSettings is Oni on a machine that never changed a setting.
func DefaultOniSettings() OniSettings {
	return OniSettings{
		Character:    OniDefaultCharacter,
		Disclosure:   OniDisclosureSession,
		Chord:        OniDefaultChord,
		Hands:        OniHandsOff,
		Size:         OniSizeNormal,
		ReduceMotion: OniReduceFollow,
		Courtesy:     true,
		VoiceSettings: OniVoiceSettings{
			Wake:      OniWakeSettings{Phrase: OniDefaultWakePhrase},
			Dictation: OniDictationSettings{Mode: OniDictateClean, Polish: OniPolishLocal},
		},
		Rewrite:    OniRewriteSettings{Style: OniRewriteGrammar},
		Assignment: OniAssignmentSettings{Style: OniAssignmentWalkthrough},
	}
}

// ValidOniMicNode reports whether n can name a PipeWire or PulseAudio source
// node: letters, digits and . _ : @ -, not starting with a dash, at most
// OniMicNodeMax bytes. No path separator passes, so it never names a device
// file. The empty name, the default source, is not a node name.
func ValidOniMicNode(n string) bool {
	if n == "" || len(n) > OniMicNodeMax || n[0] == '-' {
		return false
	}
	for _, r := range n {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		case r == '.', r == '_', r == ':', r == '@', r == '-':
		default:
			return false
		}
	}
	return true
}

// ValidOniSTTModel reports whether id can name a speech model: lowercase
// letters, digits and . _ -, starting with a letter or digit, at most
// OniSTTModelMax bytes.
func ValidOniSTTModel(id string) bool {
	if id == "" || len(id) > OniSTTModelMax || id[0] == '.' || id[0] == '_' || id[0] == '-' {
		return false
	}
	for _, r := range id {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '.', r == '_', r == '-':
		default:
			return false
		}
	}
	return true
}

// ValidOniLanguage reports whether lang is a two or three letter lowercase
// language code such as en.
func ValidOniLanguage(lang string) bool {
	if len(lang) < 2 || len(lang) > 3 {
		return false
	}
	for _, r := range lang {
		if r < 'a' || r > 'z' {
			return false
		}
	}
	return true
}

// EffectiveOni is the Oni settings policy decides on: Oni's master switch
// counts only while Hyakki itself is on.
func EffectiveOni(s Settings) OniSettings {
	o := s.Oni
	o.Feeds.Words = append([]string(nil), s.Oni.Feeds.Words...)
	if !s.Enabled {
		o.Master = false
	}
	return o
}

// ValidOniCharacterID reports whether id can name a character: lowercase
// letters, digits and inner hyphens, starting with a letter.
func ValidOniCharacterID(id string) bool {
	if id == "" || len(id) > OniCharacterIDMax || id[0] < 'a' || id[0] > 'z' || id[len(id)-1] == '-' {
		return false
	}
	for i := range len(id) {
		c := id[i]
		if (c < 'a' || c > 'z') && (c < '0' || c > '9') && c != '-' {
			return false
		}
	}
	return true
}

var oniChordModifiers = []string{"ctrl", "alt", "shift", "super"}

// NormalizeOniChord returns chord in canonical form: modifiers in the order
// ctrl, alt, shift, super, then at most one key. A chord needs ctrl, alt or
// super, so holding it never types.
func NormalizeOniChord(chord string) (string, bool) {
	parts := strings.Split(strings.ToLower(strings.TrimSpace(chord)), "+")
	if len(parts) < 2 || len(parts) > 4 {
		return "", false
	}
	mods := map[string]bool{}
	key := ""
	for _, part := range parts {
		part = strings.TrimSpace(part)
		switch {
		case isOniChordModifier(part):
			if mods[part] {
				return "", false
			}
			mods[part] = true
		case key == "" && validOniChordKey(part):
			key = part
		default:
			return "", false
		}
	}
	if !mods["ctrl"] && !mods["alt"] && !mods["super"] {
		return "", false
	}
	out := make([]string, 0, len(parts))
	for _, m := range oniChordModifiers {
		if mods[m] {
			out = append(out, m)
		}
	}
	if key != "" {
		out = append(out, key)
	}
	return strings.Join(out, "+"), true
}

func isOniChordModifier(s string) bool {
	for _, m := range oniChordModifiers {
		if s == m {
			return true
		}
	}
	return false
}

func validOniChordKey(s string) bool {
	if s == "space" {
		return true
	}
	if len(s) == 1 {
		return (s[0] >= 'a' && s[0] <= 'z') || (s[0] >= '0' && s[0] <= '9')
	}
	if len(s) < 2 || len(s) > 3 || s[0] != 'f' {
		return false
	}
	n := 0
	for i := 1; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
		n = n*10 + int(s[i]-'0')
	}
	return s[1] != '0' && n >= 1 && n <= 24
}

// CleanOniFeedWords trims, lowercases and de-duplicates feed words, dropping
// the ones that cannot name a feed and any past OniFeedWordsMax. ok is false
// when anything had to be dropped.
func CleanOniFeedWords(words []string) (clean []string, ok bool) {
	ok = true
	seen := map[string]bool{}
	for _, w := range words {
		w = strings.ToLower(strings.TrimSpace(w))
		if !ValidOniText(w, OniFeedWordMaxRunes, false) {
			ok = false
			continue
		}
		if seen[w] {
			continue
		}
		if len(clean) == OniFeedWordsMax {
			ok = false
			continue
		}
		seen[w] = true
		clean = append(clean, w)
	}
	return clean, ok
}

// ValidOniText reports whether s is non-empty valid UTF-8 of at most limit
// runes, free of control and invisible formatting characters (bidi overrides,
// zero-width joiners). multiline admits newlines.
func ValidOniText(s string, limit int, multiline bool) bool {
	if s == "" || !utf8.ValidString(s) || utf8.RuneCountInString(s) > limit {
		return false
	}
	for _, r := range s {
		if r == '\n' && multiline {
			continue
		}
		if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) {
			return false
		}
	}
	return true
}

// OniFallback is one character slot that did not load and what replaced it.
type OniFallback struct {
	Slot   string `json:"slot"`
	Reason string `json:"reason"`
	Used   string `json:"used"`
}

// OniCharacter is one character as validated at load. An invalid one lists
// its Errors and cannot be selected; a valid one lists the slots that fell
// back.
type OniCharacter struct {
	ID        string        `json:"id"`
	Name      string        `json:"name"`
	Shipped   bool          `json:"shipped"`
	Valid     bool          `json:"valid"`
	Fallbacks []OniFallback `json:"fallbacks,omitempty"`
	Errors    []string      `json:"errors,omitempty"`
}

// OniFeature is one entry of Oni's feature contract: its build stage, the
// setting that turns it on, what it may send off the computer and the
// consent it needs.
type OniFeature struct {
	ID      string `json:"id"`
	Stage   string `json:"stage"`
	Setting string `json:"setting"`
	Egress  string `json:"egress"`
	Consent string `json:"consent"`
}

// OniCapability is a feature with the policy's decision under the current
// settings.
type OniCapability struct {
	ID      string `json:"id"`
	Stage   string `json:"stage"`
	Egress  string `json:"egress"`
	Consent string `json:"consent"`
	Allow   bool   `json:"allow"`
	Reason  string `json:"reason"`
}

// OniCapabilitySet is every feature's decision. It depends on settings only,
// never on the character.
type OniCapabilitySet struct {
	Master   bool            `json:"master"`
	Features []OniCapability `json:"features"`
}

// OniStatus answers oni.status. Enabled is Hyakki's switch, Master Oni's as
// stored and Active both together. Character is the character in use, the
// shipped one when the selected character does not load.
type OniStatus struct {
	Enabled      bool             `json:"enabled"`
	Master       bool             `json:"master"`
	Active       bool             `json:"active"`
	Character    string           `json:"character"`
	Capabilities OniCapabilitySet `json:"capabilities"`
	Characters   []OniCharacter   `json:"characters"`
}

// OniCharactersResult answers oni.characters: the shipped character first,
// then the user's, each with its load report.
type OniCharactersResult struct {
	Active     string         `json:"active"`
	Characters []OniCharacter `json:"characters"`
}

// OniCharacterSelectParams names the character oni.character.select makes
// current.
type OniCharacterSelectParams struct {
	ID string `json:"id"`
}

// OniSpecResult answers oni.spec: the motion spec document as shipped and
// the feature contract. Frames are the frame kinds; Calls are the methods
// later phases serve, none of them registered yet.
type OniSpecResult struct {
	Spec     json.RawMessage `json:"spec"`
	Features []OniFeature    `json:"features"`
	Frames   []string        `json:"frames"`
	Calls    []string        `json:"calls"`
}

// Oni renderers that open the oni event. Only a native presenter draws Oni
// in this build; an observer only watches the frames.
const (
	OniRendererNative   = "native"
	OniRendererRyoku    = "ryoku"
	OniRendererObserver = "observer"
)

// Why oni.open made the caller no presenter.
const (
	OniOpenOff         = "oni-off"
	OniOpenBusy        = "presenter-busy"
	OniOpenUnavailable = "unavailable"
	OniOpenObserver    = "observer"
)

// Oni's corners; an empty corner places Oni freely.
const (
	OniCornerTopLeft     = "tl"
	OniCornerTopRight    = "tr"
	OniCornerBottomLeft  = "bl"
	OniCornerBottomRight = "br"
)

// OniCorners are every corner pin.
var OniCorners = []string{OniCornerTopLeft, OniCornerTopRight, OniCornerBottomLeft, OniCornerBottomRight}

// What a presenter reports the user did to Oni.
const (
	OniInteractPoke = "poke"
	OniInteractWake = "wake"
	OniInteractDrag = "drag"
	OniInteractDrop = "drop"
)

// OniOpenParams opens the oni event on this connection as Renderer.
type OniOpenParams struct {
	Renderer string `json:"renderer"`
}

// OniOpenResult answers oni.open. Frames are encoded oni frames: state,
// character, motion, placement and voice, in that order, or only the state
// when Oni is off. Reason says why the caller is no presenter.
type OniOpenResult struct {
	Presenter bool              `json:"presenter"`
	Reason    string            `json:"reason,omitempty"`
	Frames    []json.RawMessage `json:"frames"`
}

// OniPlaceParams saves where Oni stands: pinned to Corner, or, with no
// corner, at fractions FX, FY of Output.
type OniPlaceParams struct {
	Output string  `json:"output"`
	FX     float64 `json:"fx"`
	FY     float64 `json:"fy"`
	Corner string  `json:"corner"`
}

// OniInteractParams reports a poke, wake, drag or drop.
type OniInteractParams struct {
	Kind string `json:"kind"`
}

// OniStrandArrivedParams is the presenter's report that the strand target
// Seq landed, or, with Skipped, that it could not be drawn.
type OniStrandArrivedParams struct {
	Seq     uint64 `json:"seq"`
	Skipped string `json:"skipped,omitempty"`
}

// OniCharacterAssetsParams names the character whose files are read.
type OniCharacterAssetsParams struct {
	ID string `json:"id"`
}

// OniCharacterAssets is a valid character's profile.json and the SVGs it
// declares, each as validated at load, by its path in the character's
// folder.
type OniCharacterAssets struct {
	ID    string            `json:"id"`
	Files map[string][]byte `json:"files"`
}

// OniRect is a rectangle in output-local logical pixels, or, where a field
// says so, in fractions of a window: X, Y its top-left corner, W, H its
// size. It has the JSON of the oni frames' box.
type OniRect struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
	W float64 `json:"w"`
	H float64 `json:"h"`
}

// OniPerchParams perches Oni on Window, the seam's id, as the app and the
// CLI do; or, as the presenter's drop does, on the window under X, Y in
// output-local logical pixels of Output. Hover only hit-tests: nothing
// perches and nothing is sent.
type OniPerchParams struct {
	Window string  `json:"window,omitempty"`
	Output string  `json:"output,omitempty"`
	X      float64 `json:"x,omitempty"`
	Y      float64 `json:"y,omitempty"`
	Hover  bool    `json:"hover,omitempty"`
}

// OniPerchResult is the window Oni perched on, or, for a hover, the window
// under the point. Seq is the perch frame's; a hover has none. A hover that
// finds no window Oni could perch on says why in Reason, one of the perch
// reasons.
type OniPerchResult struct {
	Seq    uint64   `json:"seq,omitempty"`
	Window string   `json:"window,omitempty"`
	App    string   `json:"app,omitempty"`
	Title  string   `json:"title,omitempty"`
	Output string   `json:"output,omitempty"`
	Corner string   `json:"corner,omitempty"`
	Box    *OniRect `json:"box,omitempty"`
	Reason string   `json:"reason,omitempty"`
}

// OniDetachParams lets go of the perched window. Reason is user (the
// default), or, from the presenter, dragged-off or hidden.
type OniDetachParams struct {
	Reason string `json:"reason,omitempty"`
}

// OniOverlayParams are the presenter's own rects on Output, in its
// output-local logical pixels; an empty list clears them.
type OniOverlayParams struct {
	Output string    `json:"output"`
	Rects  []OniRect `json:"rects"`
}

// OniAskParams is a question about the perched window.
type OniAskParams struct {
	Text string `json:"text"`
}

// OniConsentAnswerParams answers the consent question ID yes or no.
type OniConsentAnswerParams struct {
	ID     string `json:"id"`
	Answer string `json:"answer"`
}

// What oni.talk and oni.dictate do: open the microphone for one press,
// close it and use what was heard, or close it and drop what was heard.
// oni.speak only stops.
const (
	OniVoiceStart  = "start"
	OniVoiceStop   = "stop"
	OniVoiceCancel = "cancel"
)

// OniVoiceActions are every oni.talk and oni.dictate action.
var OniVoiceActions = []string{OniVoiceStart, OniVoiceStop, OniVoiceCancel}

// Bounds on an oni.voice.test run, in milliseconds; 0 is the default.
const (
	OniVoiceTestMinMs     = 100
	OniVoiceTestMaxMs     = 5000
	OniVoiceTestDefaultMs = 1500
)

// Why a voice call could not open the microphone or transcribe, as the
// Reason of its unavailable error: no recorder (pw-cat or parec) is
// installed, the source gave no sound, capture failed otherwise, the speech
// engine is not installed, or its model is not downloaded.
const (
	OniReasonNoRecorder     = "no_recorder"
	OniReasonNoMicrophone   = "no_microphone"
	OniReasonMicFailed      = "mic_failed"
	OniReasonNoSpeechEngine = "no_speech_engine"
	OniReasonNoSpeechModel  = "no_speech_model"
)

// OniTalkParams starts, stops or cancels one hold-to-talk press. The key
// sources (the presenter's chord reader, a compositor bind running hyakki
// oni talk) all send it.
type OniTalkParams struct {
	Action string `json:"action"`
}

// OniSpeakParams stops what Oni is saying; stop is its only action.
type OniSpeakParams struct {
	Action string `json:"action"`
}

// OniVoiceResult answers oni.talk and oni.speak with the voice phase the
// call left the pipeline in.
type OniVoiceResult struct {
	Phase string `json:"phase"`
}

// OniDictateParams starts, stops or cancels one dictation into the window
// focused at start. Mode, one of OniDictateModes, overrides the setting for
// this dictation.
type OniDictateParams struct {
	Action string `json:"action"`
	Mode   string `json:"mode,omitempty"`
}

// OniDictateResult answers oni.dictate. Phase is the voice phase after the
// call. A stop reports what ran: Mode, and Fallback when a polished
// dictation fell back to clean; Landed runes typed into the target; Rest
// the text not typed, which only this result carries so the caller can
// offer to copy it; Unsure the runes of a chunk the typing tool failed
// partway through; Refused why the target was refused (closed, unfocused,
// moved-output, hidden or unavailable). Dictated text never goes on a frame.
type OniDictateResult struct {
	Phase    string `json:"phase"`
	Mode     string `json:"mode,omitempty"`
	Fallback string `json:"fallback,omitempty"`
	Landed   int    `json:"landed,omitempty"`
	Rest     string `json:"rest,omitempty"`
	Unsure   int    `json:"unsure,omitempty"`
	Refused  string `json:"refused,omitempty"`
}

// OniVoiceTestParams measures the microphone for Settings' meter: Mic is a
// source node (empty for the setting, then the default source), Ms how long
// to listen, 0 for OniVoiceTestDefaultMs. Nothing is recorded to disk.
type OniVoiceTestParams struct {
	Mic string `json:"mic,omitempty"`
	Ms  int    `json:"ms,omitempty"`
}

// OniMicSource is one capture source the mic chooser offers: its node name
// and its description. Monitors of outputs are never offered.
type OniMicSource struct {
	Name string `json:"name"`
	Desc string `json:"desc"`
}

// OniVoiceTestResult is the meter's run: the node listened to (empty for
// the default source), its levels in [0, 1] in order, and the sources the
// chooser can offer.
type OniVoiceTestResult struct {
	Mic     string         `json:"mic"`
	Levels  []float64      `json:"levels"`
	Sources []OniMicSource `json:"sources"`
}

// What oni.ask, oni.inspect, oni.consent.answer, oni.guide and oni.pick
// came to. Ended is a guide that finished or stopped, or a pick cancelled:
// Answer, when there is one, carries the closing line. Armed is a pick
// waiting for its one click.
const (
	OniAskConsent  = "consent"
	OniAskAnswered = "answered"
	OniAskDeclined = "declined"
	OniAskEnded    = "ended"
	OniAskArmed    = "armed"
)

// Answer modes beyond the screen asks' own: a guide's step, and the answer
// about a picked spot.
const (
	OniModeGuide  = "guide"
	OniModeSelect = "select"
)

// OniAskResult answers oni.ask, oni.inspect, oni.consent.answer, oni.guide
// and oni.pick. With Status consent nothing has been sent: Consent shows
// what would be and where. With answered, Answer is what came back. Keep is
// the offer to keep a guide that just ended, answered with oni.keep or
// oni.skip.
type OniAskResult struct {
	Status  string        `json:"status"`
	Consent *OniConsent   `json:"consent,omitempty"`
	Answer  *OniAnswer    `json:"answer,omitempty"`
	Keep    *OniKeepOffer `json:"keep,omitempty"`
}

// OniConsent is the question asked before a picture is sent: Destination
// local or hosted, Route the provider/model that will answer, Path the
// capture path that ran and Disclosure what it means, Preview a JPEG
// thumbnail of the exact picture, Width and Height the size of the picture
// that would be sent. The question can be answered for ExpiresMs.
type OniConsent struct {
	ID          string `json:"id"`
	Destination string `json:"destination"`
	Route       string `json:"route"`
	Path        string `json:"path"`
	Disclosure  string `json:"disclosure"`
	ScopeExact  bool   `json:"scopeExact"`
	Question    string `json:"question"`
	Preview     []byte `json:"preview"`
	Width       int    `json:"width"`
	Height      int    `json:"height"`
	ExpiresMs   int64  `json:"expiresMs"`
}

// OniAnswer is what came back about the window: Text to show, Mode the
// pointing or answer mode (guide for a guide's step, select for a picked
// spot), Refused a refusal code when Oni would not point (Text is then the
// refusal line), Point the box pointed at in fractions of the window and Seq
// the strand target sent to it. A guide's answer names its Step from 1, Done
// on the last, and Replay when it came from a kept guide with no model. A
// select answer offers Walk, a guide through what it explained. Route,
// Destination and Path say what ran.
type OniAnswer struct {
	Text        string   `json:"text"`
	Mode        string   `json:"mode"`
	Refused     string   `json:"refused,omitempty"`
	Point       *OniRect `json:"point,omitempty"`
	Seq         uint64   `json:"seq,omitempty"`
	Step        int      `json:"step,omitempty"`
	Done        bool     `json:"done,omitempty"`
	Replay      bool     `json:"replay,omitempty"`
	Walk        bool     `json:"walk,omitempty"`
	Route       string   `json:"route"`
	Destination string   `json:"destination"`
	Path        string   `json:"path"`
}

// What oni.guide does: start a guide toward Goal, move to the next step,
// stop, or throw the ring around the current target again.
const (
	OniGuideStart  = "start"
	OniGuideNext   = "next"
	OniGuideStop   = "stop"
	OniGuideCircle = "circle"
)

// OniGuideActions are every oni.guide action.
var OniGuideActions = []string{OniGuideStart, OniGuideNext, OniGuideStop, OniGuideCircle}

// OniGuideGoalMax bounds a guide's goal, in runes.
const OniGuideGoalMax = 300

// OniGuideParams starts, advances, stops or re-circles the guide. Goal, what
// the user wants done, comes with a start only.
type OniGuideParams struct {
	Action string `json:"action"`
	Goal   string `json:"goal,omitempty"`
}

// OniKeepOffer is the offer to keep a finished guide: oni.keep or oni.skip
// answers ID within ExpiresMs, after which it lapses to skip.
type OniKeepOffer struct {
	ID        string `json:"id"`
	Question  string `json:"question"`
	ExpiresMs int64  `json:"expiresMs"`
}

// OniKeepParams answers the keep offer ID: oni.keep keeps the guide,
// oni.skip drops the offer.
type OniKeepParams struct {
	ID string `json:"id"`
}

// OniKeepResult answers oni.keep: whether the guide was kept and how many
// kept guides there are now.
type OniKeepResult struct {
	Kept    bool `json:"kept"`
	Entries int  `json:"entries"`
}

// What oni.pick does: arm the pick overlay, report its one click, cancel
// it, or wait on it.
const (
	OniPickArm    = "arm"
	OniPickPoint  = "point"
	OniPickCancel = "cancel"
	OniPickWait   = "wait"
)

// OniPickActions are every oni.pick action.
var OniPickActions = []string{OniPickArm, OniPickPoint, OniPickCancel, OniPickWait}

// OniPickTextMax bounds the question an armed pick carries, in runes.
const OniPickTextMax = 2000

// OniPickParams is select-and-ask. Arm maps the pick overlay, with Text the
// optional question about the spot; Point is its one click at X, Y in
// output-local logical pixels of Output; Cancel unmaps it. Wait, from the
// connection that armed the pick, holds until its click, cancel or lapse
// and answers with what the click came to (a picture waiting for that
// connection's yes, an answer, a refusal) or ended.
type OniPickParams struct {
	Action string  `json:"action"`
	Output string  `json:"output,omitempty"`
	X      float64 `json:"x,omitempty"`
	Y      float64 `json:"y,omitempty"`
	Text   string  `json:"text,omitempty"`
}
