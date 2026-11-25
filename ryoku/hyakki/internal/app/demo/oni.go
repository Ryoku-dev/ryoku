package demo

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"slices"
	"strings"
	"testing/fstest"

	"ryoku-i18n"

	"ryoku-rashin/internal/config"
	"ryoku-rashin/internal/oni/feature"
	"ryoku-rashin/internal/oni/profile"
	"ryoku-rashin/internal/oni/spec"
	"ryoku-rashin/internal/protocol"
)

// FeatureOni is no feature: it starts the demo with Oni's master switch on,
// for the Settings section that shows Oni. Oni may look at windows then, and
// sits perched on the demo's notes window.
const FeatureOni = "oni"

// demoOni is Oni on the demo machine, what it starts remembering, and the
// features left once the Oni demo switches are taken out of them.
func demoOni(features []string) (protocol.OniSettings, oniFixtures, []string) {
	settings := protocol.DefaultOniSettings()
	voice := slices.Contains(features, FeatureOniVoice)
	guide := slices.Contains(features, FeatureOniGuide)
	noWake := slices.Contains(features, FeatureOniReflexNoWake)
	reflex := noWake || slices.Contains(features, FeatureOniReflex)
	hands := slices.ContainsFunc(features, demoHandsFeature)
	phase16 := slices.Contains(features, FeatureOniPhase16)
	phase17 := slices.Contains(features, FeatureOniPhase17)
	on := voice || guide || reflex || hands || phase16 || phase17 || slices.Contains(features, FeatureOni)
	if on {
		settings.Master, settings.Inspect = true, true
	}
	if hands {
		settings.Hands = protocol.OniHandsBrowser
		for _, id := range features {
			if stage, ok := demoHandsAppFeature(id); ok {
				settings.Hands = stage
			}
		}
	}
	if voice {
		settings.Voice, settings.VoiceSettings.Dictation.Enabled = true, true
	}
	if guide {
		settings.Guide = protocol.OniGuideSettings{Enabled: true, Keep: true}
	}
	if reflex {
		settings.Voice, settings.Reflex = true, true
		settings.ReflexSettings = protocol.OniReflexSettings{Routines: true, Reminders: true, Notes: true}
	}
	if phase16 {
		applyPhase16(&settings)
	}
	if phase17 {
		applyPhase17(&settings)
	}
	rest := slices.DeleteFunc(slices.Clone(features), func(id string) bool {
		return id == FeatureOni || id == FeatureOniLocal || id == FeatureOniVoice || id == FeatureOniNoMic || id == FeatureOniGuide ||
			id == FeatureOniReflex || id == FeatureOniReflexNoWake || id == FeatureOniSmallComputer || demoHandsFeature(id) || id == FeatureOniPhase16 || id == FeatureOniPhase17
	})
	fixtures := newOniFixtures(on, slices.Contains(features, FeatureOniLocal))
	fixtures.voice = newVoiceFixtures(slices.Contains(features, FeatureOniNoMic))
	fixtures.reflex.noWake = noWake
	fixtures.smallComputer = slices.Contains(features, FeatureOniSmallComputer)
	fixtures.hands = newHandsFixtures(features)
	return settings, fixtures, rest
}

const (
	demoMothSVG  = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 48 48" width="48" height="48"><ellipse cx="24" cy="26" rx="4" ry="12" fill="#6B5B4B"/><path d="M22 20 C12 8 4 14 8 24 C10 30 18 30 22 26 Z M26 20 C36 8 44 14 40 24 C38 30 30 30 26 26 Z" fill="#C9B79C" stroke="#3A3028" stroke-width="1.5"/></svg>`
	demoRestSVG  = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 48 48"><ellipse cx="24" cy="30" rx="10" ry="6" fill="#C9B79C"/></svg>`
	demoAlertSVG = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 48 48"><defs><linearGradient id="w"><stop offset="0" stop-color="#C9B79C"/><stop offset="1" stop-color="#8A7A66"/></linearGradient></defs><path d="M8 12 L24 28 L40 12 L24 40 Z" fill="url(#w)"/></svg>`
)

// a moth with its own poses for every slot, as profile/testdata/testmoth; the
// verbs fill in its id and anything after its voice
const demoMothProfile = `{
  "version": 1,
  "id": %q,
  "name": "Moth",
  "artwork": {"kind": "svg", "image": "moth.svg"},
  "animations": {
    "states": {"idle": "poses/rest.svg", "ready": "poses/rest.svg", "listening": "poses/alert.svg", "working": "poses/alert.svg", "reading": "poses/rest.svg", "thinking": "poses/alert.svg", "speaking": "poses/rest.svg", "error": "poses/alert.svg", "sleeping": "poses/rest.svg"},
    "gestures": {"acknowledge": "poses/alert.svg", "done": "poses/alert.svg", "error": "poses/alert.svg", "poke": "poses/alert.svg", "nudge": "poses/alert.svg", "wake": "poses/alert.svg", "sleep": "poses/alert.svg", "arrive": "poses/alert.svg", "gulp": "poses/alert.svg", "settle": "poses/alert.svg"}
  },
  "personality": {"voiceLines": "lines.json", "promptPersona": "persona.md", "idleHumor": false},
  "voice": {"engine": "system", "speed": 1.1, "honorific": "none"}%s
}`

// the demo's own characters folder: a moth that loads whole, a paper one with
// no artwork whose every slot falls back, and a lantern the loader rejects
// because its profile asks for tools
var demoCharacters = func() fstest.MapFS {
	files := fstest.MapFS{}
	moth := func(id, extra string) {
		files[id+"/profile.json"] = &fstest.MapFile{Data: fmt.Appendf(nil, demoMothProfile, id, extra)}
		files[id+"/moth.svg"] = &fstest.MapFile{Data: []byte(demoMothSVG)}
		files[id+"/poses/rest.svg"] = &fstest.MapFile{Data: []byte(demoRestSVG)}
		files[id+"/poses/alert.svg"] = &fstest.MapFile{Data: []byte(demoAlertSVG)}
		files[id+"/persona.md"] = &fstest.MapFile{Data: []byte("Short and quiet.\n")}
		files[id+"/lines.json"] = &fstest.MapFile{Data: []byte(`{"acknowledge": ["Noted.", "Working on it."]}`)}
	}
	moth("testmoth", "")
	moth("lantern", `,
  "tools": ["shell"]`)
	files["paper/profile.json"] = &fstest.MapFile{Data: []byte(`{"version": 1, "id": "paper", "name": "Paper"}`)}
	return files
}()

// demoLoaded is the shipped oni followed by the demo's characters in name
// order, each as the real loader reads it
func demoLoaded() []profile.Loaded {
	out := []profile.Loaded{profile.Default()}
	entries, err := fs.ReadDir(demoCharacters, ".")
	if err != nil {
		return out
	}
	for _, entry := range entries {
		if entry.IsDir() {
			loaded, _ := profile.Load(demoCharacters, entry.Name())
			out = append(out, loaded)
		}
	}
	return out
}

func demoOniCharacters() []protocol.OniCharacter {
	loaded := demoLoaded()
	out := make([]protocol.OniCharacter, 0, len(loaded))
	for _, l := range loaded {
		c := protocol.OniCharacter{
			ID:      l.Profile.ID,
			Name:    l.Profile.Name,
			Shipped: l.Profile.Shipped,
			Valid:   l.Valid,
			Errors:  append([]string(nil), l.Report.Errors...),
		}
		for _, f := range l.Report.Fallbacks {
			c.Fallbacks = append(c.Fallbacks, protocol.OniFallback{Slot: f.Slot, Reason: f.Reason, Used: f.Used})
		}
		out = append(out, c)
	}
	return out
}

func demoActiveCharacter(selected string, characters []protocol.OniCharacter) string {
	for _, c := range characters {
		if c.ID == selected && c.Valid {
			return c.ID
		}
	}
	return protocol.OniDefaultCharacter
}

func (s *server) oniStatus() protocol.OniStatus {
	settings := s.data.snapshot.Settings
	effective := protocol.EffectiveOni(settings)
	characters := demoOniCharacters()
	return protocol.OniStatus{
		Enabled:      settings.Enabled,
		Master:       settings.Oni.Master,
		Active:       effective.Master,
		Character:    demoActiveCharacter(settings.Oni.Character, characters),
		Capabilities: feature.Capabilities(effective),
		Characters:   characters,
	}
}

// the demo's Oni calls answer as the daemon's do, over the demo's characters
func (s *server) callOni(method string, raw json.RawMessage) (any, error) {
	switch method {
	case protocol.MOniStatus:
		return s.oniStatus(), nil
	case protocol.MOniCharacters:
		characters := demoOniCharacters()
		return protocol.OniCharactersResult{Active: demoActiveCharacter(s.data.snapshot.Settings.Oni.Character, characters), Characters: characters}, nil
	case protocol.MOniCharacterSelect:
		var params protocol.OniCharacterSelectParams
		if err := decode(raw, &params); err != nil {
			return nil, err
		}
		return s.oniSelect(params.ID)
	case protocol.MOniCharacterReset:
		return s.oniSelect(protocol.OniDefaultCharacter)
	case protocol.MOniCharacterAssets:
		var params protocol.OniCharacterAssetsParams
		if err := decode(raw, &params); err != nil {
			return nil, err
		}
		loaded, err := demoCharacter(params.ID)
		if err != nil {
			return nil, err
		}
		return protocol.OniCharacterAssets{ID: loaded.Profile.ID, Files: loaded.Files}, nil
	case protocol.MOniPlace:
		var params protocol.OniPlaceParams
		if err := decode(raw, &params); err != nil {
			return nil, err
		}
		switch {
		case params.Corner == "" && (params.Output == "" || params.FX < 0 || params.FX > 1 || params.FY < 0 || params.FY > 1):
			return nil, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("A free place needs an output and fractions between 0 and 1."))
		case params.Corner != "" && !slices.Contains([]string{"tl", "tr", "bl", "br"}, params.Corner):
			return nil, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("Oni pins to a screen corner: upper left, upper right, lower left or lower right."))
		}
		return protocol.Empty{}, nil
	case protocol.MOniSpec:
		loaded, err := spec.Load()
		if err != nil {
			return nil, protocol.Errorf(protocol.CodeInternal, "%s", i18n.Tf("The Oni motion spec does not load: %v.", err))
		}
		document, err := json.Marshal(loaded)
		if err != nil {
			return nil, protocol.Errorf(protocol.CodeInternal, "%s", i18n.Tf("The Oni motion spec does not load: %v.", err))
		}
		return protocol.OniSpecResult{Spec: document, Features: feature.Features(), Frames: feature.Kinds(), Calls: feature.ReservedCalls()}, nil
	case protocol.MOniConsentAnswer:
		var peek protocol.OniConsentAnswerParams
		if json.Unmarshal(raw, &peek) == nil && strings.HasPrefix(peek.ID, feature.HandsStepCardPrefix) {
			return s.data.oni.hands.answer(raw)
		}
		return s.callOniAsk(method, raw)
	case protocol.MOniPerch, protocol.MOniDetach, protocol.MOniInspect, protocol.MOniAsk:
		return s.callOniAsk(method, raw)
	case protocol.MOniTalk, protocol.MOniSpeak, protocol.MOniDictate, protocol.MOniVoiceTest:
		return s.callOniVoice(method, raw)
	case protocol.MOniOpen:
		params, err := feature.DecodeCall(method, raw)
		if err != nil {
			return nil, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.Tf("The Oni call is invalid: %v.", err))
		}
		// the demo draws no Oni: whoever opens only watches, as an observer does
		reason := protocol.OniOpenObserver
		if params.(*protocol.OniOpenParams).Renderer != protocol.OniRendererObserver {
			reason = protocol.OniOpenUnavailable
		}
		frames := []json.RawMessage{}
		if protocol.EffectiveOni(s.data.snapshot.Settings).Master {
			frames = s.data.oni.guides.opening()
			for _, p := range s.data.oni.hands.opening() {
				if raw, err := feature.EncodeFrame(p); err == nil {
					frames = append(frames, raw)
				}
			}
		}
		return protocol.OniOpenResult{Reason: reason, Frames: frames}, nil
	case protocol.MOniClose:
		return protocol.Empty{}, nil
	case protocol.MOniGuide, protocol.MOniKeep, protocol.MOniSkip, protocol.MOniPick:
		return s.callOniGuide(method, raw)
	case protocol.MOniReminders, protocol.MOniRoutines, protocol.MOniReflexLog:
		return s.callOniReflex(method, raw)
	case protocol.MOniHands, protocol.MOniHandsLog:
		return s.callOniHands(method, raw)
	case protocol.MOniRewrite, protocol.MOniRecall, protocol.MOniRecallImport, protocol.MOniFocus, protocol.MOniAssignment, protocol.MOniAssignmentLog:
		return s.callOniPhase16(method, raw)
	case protocol.MOniActivity, protocol.MOniActivityClear, protocol.MOniNotify, protocol.MOniMCP:
		return s.callOniPhase17(method, raw)
	}
	return nil, notFound(method)
}

// demoCharacter is one character's load, refusing one that is not there or does not load
func demoCharacter(id string) (profile.Loaded, error) {
	if !protocol.ValidOniCharacterID(id) {
		return profile.Loaded{}, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("The Oni character id is invalid."))
	}
	for _, loaded := range demoLoaded() {
		if loaded.Profile.ID != id {
			continue
		}
		if !loaded.Valid {
			return profile.Loaded{}, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.Tf("The Oni character %s does not load: %s.", id, strings.Join(loaded.Report.Errors, "; ")))
		}
		return loaded, nil
	}
	return profile.Loaded{}, protocol.Errorf(protocol.CodeNotFound, "%s", i18n.Tf("There is no Oni character %s.", id))
}

// selecting changes the settings, so the app hears settings.changed as it does from the daemon
func (s *server) oniSelect(id string) (protocol.OniStatus, error) {
	if _, err := demoCharacter(id); err != nil {
		return protocol.OniStatus{}, err
	}
	s.data.snapshot.Settings.Oni.Character = id
	config.Validate(&s.data.snapshot.Settings)
	s.queueEvent(protocol.EvtSettingsChanged, s.data.snapshot.Settings)
	return s.oniStatus(), nil
}
