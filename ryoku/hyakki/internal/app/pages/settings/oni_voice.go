package settings

import (
	"errors"
	"image"
	"math"
	"net/url"
	"strings"
	"sync"

	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/oni/keys"
	"ryoku-rashin/internal/oni/spec"
	"ryoku-rashin/internal/oni/voice/stt"
	"ryoku-rashin/internal/protocol"
)

// oniVoice is talking to Oni: the switch, the microphone and its test, the
// chord, the voices and speech models on this computer, and the two ways
// the microphone stays on, each opened only past its disclosure
type oniVoice struct {
	talk, speak, wake, openMic kit.Toggle
	mic, voice, engine, model  look.Select

	// the meter: one test at a time, its levels and the sources it found
	test        kit.Plaque
	testing     bool
	levels      []float64
	heard       string
	sources     []protocol.OniMicSource
	testProblem string

	chord        kit.TextField
	chordUse     kit.Plaque
	chordSeeded  string
	chordProblem string

	phrase        kit.TextField
	phraseUse     kit.Plaque
	phraseSeeded  string
	phraseProblem string

	// what is installed, read when the section first shows and after a download
	survey    env.VoiceSurvey
	surveyed  bool
	surveying bool
	surveyFor string

	// the continuous-microphone question, for the wake phrase or open mic
	disclose                kit.Dialog
	discloseFor             string
	discloseYes, discloseNo kit.Plaque
	// the speech model waiting for a yes (with the fallback's voice
	// activity model when that is missing too), and the one downloading
	consent               kit.Dialog
	consentModel          stt.Model
	consentVAD            stt.Model
	consentYes, consentNo kit.Plaque
	downloading           stt.Model
	fetched               int64
	stopDownload          kit.Plaque
	cancelDownload        func()
	downloadNote          string
}

const (
	continuousWake = "wake"
	continuousOpen = "openMic"
)

// forget reads what is installed again the next time the section shows
func (v *oniVoice) forget() { v.surveyed = false }

func (v *oniVoice) read(e *env.Env, character string) {
	if e.Voice.Survey == nil || v.surveying || (v.surveyed && v.surveyFor == character) {
		return
	}
	v.surveying = true
	e.Voice.Survey(character, func(survey env.VoiceSurvey) {
		v.survey, v.surveyed, v.surveying, v.surveyFor = survey, true, false, character
	})
}

// voiceCeilings are the limits the spec sets on a microphone left on
var voiceCeilings = sync.OnceValues(func() (spec.Voice, error) {
	loaded, err := spec.Load()
	if err != nil {
		return spec.Voice{}, err
	}
	return loaded.Voice, nil
})

func voiceSet(e *env.Env, values map[string]any) {
	set(e, "oni", map[string]any{"voiceSettings": values})
}

// runTest listens to the chosen microphone for a moment; nothing is kept
func (v *oniVoice) runTest(e *env.Env, mic string) {
	if v.testing {
		return
	}
	v.testing, v.testProblem = true, ""
	result := new(protocol.OniVoiceTestResult)
	e.Call(protocol.MOniVoiceTest, protocol.OniVoiceTestParams{Mic: mic}, result, func(err error) {
		v.testing = false
		if err != nil {
			v.levels, v.heard = nil, ""
			v.testProblem = micProblem(err)
			return
		}
		v.levels, v.sources, v.heard = result.Levels, result.Sources, result.Mic
	})
}

// micProblem says what stopped the test, in the user's terms
func micProblem(err error) string {
	wire := new(protocol.Error)
	if !errors.As(err, &wire) {
		return err.Error()
	}
	switch wire.Reason {
	case protocol.OniReasonNoMicrophone:
		return i18n.T("No sound arrived from the microphone. Check that one is plugged in and not muted, or choose another.")
	case protocol.OniReasonNoRecorder:
		return i18n.T("Hyakki hears the microphone through pw-cat or parec, and neither is installed.")
	case protocol.OniReasonMicFailed:
		return i18n.T("The microphone could not be opened. Another program may hold it, or the desktop may not let Hyakki use it.")
	}
	return wire.Message
}

func (v *oniVoice) sourceLabel(name string) string {
	if name == "" {
		return i18n.T("Default microphone")
	}
	for _, source := range v.sources {
		if source.Name == name && source.Desc != "" {
			return source.Desc
		}
	}
	return name
}

// rows are the Voice group's settings
func (p *Page) oniVoiceRows(e *env.Env, oni, defaults protocol.OniSettings) []look.Setting {
	th := e.Theme
	v := &p.oni.voice
	v.read(e, oni.Character)
	voice, dv := oni.VoiceSettings, defaults.VoiceSettings
	v.talk.Value, v.speak.Value = oni.Voice, voice.SpeakReplies
	v.wake.Value, v.openMic.Value = voice.Wake.Enabled, voice.OpenMic
	off := ""
	if !protocol.EffectiveOni(e.Snapshot().Settings).Master {
		off = i18n.T("Oni is off, so it cannot listen.")
	}
	talk := i18n.T("Talk to Oni")
	rows := []look.Setting{
		{Title: talk, Detail: i18n.T("Hold the talk chord and speak. Oni answers as it does a typed question, on the same route and with the same disclosure. The microphone is open only while you hold the chord."),
			Unavailable: off, Changed: oni.Voice != defaults.Voice, Reset: func() { set(e, "oni", map[string]any{"voice": defaults.Voice}) },
			Control: func(gtx layout.Context) layout.Dimensions {
				return namedSwitch(gtx, th, talk, &v.talk, func(on bool) { set(e, "oni", map[string]any{"voice": on}) })
			}},
		{Title: i18n.T("Microphone"), Detail: i18n.T("Which input Oni listens to. Test the microphone to list the others."),
			Changed: voice.Mic != dv.Mic, Reset: func() { voiceSet(e, map[string]any{"mic": dv.Mic}) },
			Control: func(gtx layout.Context) layout.Dimensions { return v.layoutMic(gtx, e, voice.Mic) }},
		{Title: i18n.T("Test the microphone"), Detail: v.testDetail(), Below: true,
			Control: func(gtx layout.Context) layout.Dimensions { return v.layoutTest(gtx, e, voice.Mic) }},
		v.chordRow(e, oni, defaults),
	}
	if v.survey.Keys != "" {
		rows = append(rows, look.Setting{Title: i18n.T("The keyboard cannot be read"), Detail: v.survey.Keys + "\n" + compositorBinds()})
	}
	speak := i18n.T("Speak replies")
	rows = append(rows,
		look.Setting{Title: speak, Detail: i18n.T("Oni says its answers aloud as well as showing them. Speaking stops the moment you hold the chord again."),
			Changed: voice.SpeakReplies != dv.SpeakReplies, Reset: func() { voiceSet(e, map[string]any{"speakReplies": dv.SpeakReplies}) },
			Control: func(gtx layout.Context) layout.Dimensions {
				return namedSwitch(gtx, th, speak, &v.speak, func(on bool) { voiceSet(e, map[string]any{"speakReplies": on}) })
			}},
		v.voiceRow(e, voice, dv),
		v.engineRow(e, voice, dv),
		v.modelRow(e, voice, dv),
	)
	if row, ok := v.downloadRow(e); ok {
		rows = append(rows, row)
	}
	return append(rows, v.continuousRows(e, voice, dv, off)...)
}

func (v *oniVoice) layoutMic(gtx layout.Context, e *env.Env, mic string) layout.Dimensions {
	th := e.Theme
	names := []string{""}
	options := []look.SelectOption{{Label: i18n.T("Default microphone"), Detail: i18n.T("The one the desktop uses")}}
	for _, source := range v.sources {
		names = append(names, source.Name)
		options = append(options, look.SelectOption{Label: v.sourceLabel(source.Name), Detail: source.Name})
	}
	if mic != "" && indexOf(names, mic) == 0 {
		names = append(names, mic)
		options = append(options, look.SelectOption{Label: mic})
	}
	v.mic.Label = i18n.T("Microphone")
	gtx.Constraints.Min.X = gtx.Dp(selectWidth)
	dimensions := v.mic.LayoutOptions(gtx, th, e.Overlays, options, indexOf(names, mic))
	if index, ok := v.mic.Changed(); ok && index >= 0 && index < len(names) && names[index] != mic {
		voiceSet(e, map[string]any{"mic": names[index]})
		v.levels, v.heard, v.testProblem = nil, "", ""
	}
	return dimensions
}

func (v *oniVoice) testDetail() string {
	switch {
	case v.testProblem != "":
		return v.testProblem
	case v.testing:
		return i18n.T("Listening for a moment…")
	case len(v.levels) > 0:
		peak := 0.0
		for _, level := range v.levels {
			peak = max(peak, level)
		}
		return i18n.Tf("Heard from %s, loudest at %d%%. Nothing was recorded.", v.sourceLabel(v.heard), int(math.Round(peak*100)))
	}
	return i18n.T("Listens to the microphone for a moment and shows how loud it is. Nothing is recorded or kept.")
}

func (v *oniVoice) layoutTest(gtx layout.Context, e *env.Env, mic string) layout.Dimensions {
	th := e.Theme
	v.test.Kind, v.test.Small, v.test.Label, v.test.Busy = kit.PlaqueTonal, true, i18n.T("Test"), v.testing
	if v.test.Clicked(gtx) {
		v.runTest(e, mic)
	}
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions { return v.test.Layout(gtx, th.Theme) }),
		layout.Rigid(layout.Spacer{Width: th.Sp.S4}.Layout),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return look.Named(gtx, i18n.T("Microphone level"), func(gtx layout.Context) layout.Dimensions { return levelMeter(gtx, th, v.levels) })
		}),
	)
}

// levelMeter draws each level the test heard as a bar standing on a vein,
// left to right in the order heard
func levelMeter(gtx layout.Context, th *look.Theme, levels []float64) layout.Dimensions {
	width, height := gtx.Constraints.Max.X, gtx.Dp(unit.Dp(28))
	hairline := max(1, gtx.Dp(unit.Dp(1)))
	kit.Fill(gtx, th.Theme, image.Rect(0, height-hairline, width, height), 0, th.R.Vein)
	if count := len(levels); count > 0 {
		gap := gtx.Dp(unit.Dp(2))
		bar := min(gtx.Dp(unit.Dp(8)), max(1, (width-gap*(count-1))/count))
		// the tops take the design's corner, square on Lithic's cut and round elsewhere
		cut := min(bar/4, gtx.Dp(th.Cut.Token))
		for index, level := range levels {
			x := index * (bar + gap)
			tall := max(hairline, int(float64(height)*min(max(level, 0), 1)))
			kit.FillCorners(gtx, th.Theme, image.Rect(x, height-tall, x+bar, height), min(cut, tall/2), min(cut, tall/2), 0, 0, th.R.Gold)
		}
	}
	return layout.Dimensions{Size: image.Pt(width, height)}
}

// chordRow rebinds the talk chord; one the daemon could not hold without
// typing is refused here, before anything is saved
func (v *oniVoice) chordRow(e *env.Env, oni, defaults protocol.OniSettings) look.Setting {
	th := e.Theme
	if v.chordSeeded != oni.Chord {
		v.chord.SetText(oni.Chord)
		v.chordSeeded, v.chordProblem = oni.Chord, ""
	}
	return look.Setting{Title: i18n.T("Talk chord"), Below: true,
		Detail:  i18n.T("Hold it to talk and let go to send. It needs Ctrl, Alt or Super, with at most one other key, so holding it never types. AltGr never counts: many layouts send it as Ctrl and Alt together, so a chord is read from the left Ctrl and Alt only."),
		Changed: oni.Chord != defaults.Chord, Reset: func() { set(e, "oni", map[string]any{"chord": defaults.Chord}) },
		Control: func(gtx layout.Context) layout.Dimensions {
			v.chord.Label, v.chord.Placeholder, v.chord.Mono, v.chord.Editor.SingleLine, v.chord.Editor.Submit = i18n.T("Chord"), "ctrl+alt", true, true, true
			v.chord.Error = v.chordProblem
			submitted := false
			for {
				event, ok := v.chord.Editor.Update(gtx)
				if !ok {
					break
				}
				switch event.(type) {
				case widget.SubmitEvent:
					submitted = true
				case widget.ChangeEvent:
					v.chordProblem = ""
				}
			}
			v.chordUse.Kind, v.chordUse.Small, v.chordUse.Label = kit.PlaqueTonal, true, i18n.T("Use this chord")
			if v.chordUse.Clicked(gtx) || submitted {
				v.useChord(e, oni.Chord)
			}
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(kit.Keycaps(th.Theme, chordFaces(oni.Chord)...)),
				layout.Rigid(layout.Spacer{Height: th.Sp.S2}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Alignment: layout.End}.Layout(gtx,
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions { return v.chord.Layout(gtx, th.Theme) }),
						layout.Rigid(layout.Spacer{Width: th.Sp.S2}.Layout),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions { return v.chordUse.Layout(gtx, th.Theme) }),
					)
				}),
			)
		}}
}

func (v *oniVoice) useChord(e *env.Env, current string) {
	typed := strings.ToLower(strings.TrimSpace(v.chord.Text()))
	if strings.Contains(typed, "altgr") || strings.Contains(typed, "alt_r") || strings.Contains(typed, "ralt") {
		v.chordProblem = i18n.T("AltGr cannot be part of the chord: many layouts send it as Ctrl and Alt, so it would also type. Use the left Ctrl and Alt.")
		return
	}
	parsed, err := keys.ParseChord(typed)
	if err != nil {
		v.chordProblem = i18n.T("Use Ctrl, Alt or Super with at most one other key, such as ctrl+alt or super+space.")
		return
	}
	chord := parsed.String()
	v.chordProblem = ""
	if chord == current {
		v.chord.SetText(chord)
		return
	}
	e.SetSettings(map[string]any{"oni": map[string]any{"chord": chord}}, func(err error) {
		if err != nil {
			v.chordProblem = err.Error()
		}
	})
}

// chordFaces are the keycaps of a stored chord
func chordFaces(chord string) []string {
	parts := strings.Split(chord, "+")
	faces := make([]string, 0, len(parts))
	for _, part := range parts {
		switch part {
		case "ctrl":
			faces = append(faces, "Ctrl")
		case "alt":
			faces = append(faces, "Alt")
		case "shift":
			faces = append(faces, "Shift")
		case "super":
			faces = append(faces, "Super")
		case "space":
			faces = append(faces, "Space")
		default:
			faces = append(faces, strings.ToUpper(part))
		}
	}
	return faces
}

// compositorBinds are the binds that start and stop talking from the desktop
func compositorBinds() string {
	return i18n.T("Hyprland: bind = CTRL ALT, Space, exec, hyakki oni talk start, and bindr = CTRL ALT, Space, exec, hyakki oni talk stop. niri has no release binds, so bind start and stop to two keys.")
}

func (v *oniVoice) voiceRow(e *env.Env, voice, dv protocol.OniVoiceSettings) look.Setting {
	th := e.Theme
	engines := []string{"", protocol.OniTTSSystem, protocol.OniTTSClips, protocol.OniTTSKokoro}
	labels := []string{i18n.T("The character's own"), i18n.T("System voice"), i18n.T("Recorded clips"), i18n.T("Kokoro")}
	details := []string{i18n.T("Whatever voice the character asks for"), i18n.T("espeak-ng, which speaks any text"), i18n.T("The character's recorded lines; other text in the system voice"), i18n.T("An optional neural voice pack")}
	options := make([]look.SelectOption, len(engines))
	for index, engine := range engines {
		options[index] = look.SelectOption{Label: labels[index], Detail: details[index]}
		if reason := v.missing(v.survey.Voices, engine); reason != "" {
			options[index].Detail, options[index].Disabled = reason, true
		}
	}
	return look.Setting{Title: i18n.T("Oni's voice"), Detail: i18n.T("How Oni sounds when it speaks. One that is not installed is listed with what it needs."),
		Changed: voice.TTS.Engine != dv.TTS.Engine, Reset: func() { voiceSet(e, map[string]any{"tts": map[string]any{"engine": dv.TTS.Engine}}) },
		Control: func(gtx layout.Context) layout.Dimensions {
			v.voice.Label = i18n.T("Voice")
			gtx.Constraints.Min.X = gtx.Dp(selectWidth)
			dimensions := v.voice.LayoutOptions(gtx, th, e.Overlays, options, indexOf(engines, voice.TTS.Engine))
			if index, ok := v.voice.Changed(); ok && index >= 0 && index < len(engines) && !options[index].Disabled {
				voiceSet(e, map[string]any{"tts": map[string]any{"engine": engines[index]}})
			}
			return dimensions
		}}
}

// missing is why a choice is not on this computer; the default ("") and an
// unread survey always offer it
func (v *oniVoice) missing(reasons map[string]string, choice string) string {
	if choice == "" || !v.surveyed {
		return ""
	}
	return reasons[choice]
}

func (v *oniVoice) engineRow(e *env.Env, voice, dv protocol.OniVoiceSettings) look.Setting {
	th := e.Theme
	engines := []string{"", protocol.OniSTTWhisper, protocol.OniSTTDictate}
	labels := []string{i18n.T("Hyakki's choice"), "whisper.cpp", i18n.T("transcribe.cpp (Parakeet)")}
	details := []string{i18n.T("The engine Hyakki measured best here"), i18n.T("Hyakki's own whisper-server"), i18n.T("Through ryoku-dictate, when it is installed")}
	options := make([]look.SelectOption, len(engines))
	for index, engine := range engines {
		options[index] = look.SelectOption{Label: labels[index], Detail: details[index]}
		if reason := v.missing(v.survey.Engines, engine); reason != "" {
			options[index].Detail, options[index].Disabled = reason, true
		}
	}
	return look.Setting{Title: i18n.T("Speech to text"), Detail: i18n.T("What turns your speech into words. It runs on this computer; nothing you say is sent to hear it."),
		Changed: voice.STT.Engine != dv.STT.Engine, Reset: func() { voiceSet(e, map[string]any{"stt": map[string]any{"engine": dv.STT.Engine}}) },
		Control: func(gtx layout.Context) layout.Dimensions {
			v.engine.Label = i18n.T("Speech engine")
			gtx.Constraints.Min.X = gtx.Dp(selectWidth)
			dimensions := v.engine.LayoutOptions(gtx, th, e.Overlays, options, indexOf(engines, voice.STT.Engine))
			if index, ok := v.engine.Changed(); ok && index >= 0 && index < len(engines) && !options[index].Disabled {
				voiceSet(e, map[string]any{"stt": map[string]any{"engine": engines[index]}})
			}
			return dimensions
		}}
}

// speechModels are the catalogue models that hear speech, the voice
// activity model being the engine's own
func speechModels() []stt.Model {
	models := make([]stt.Model, 0, len(stt.Catalogue))
	for _, model := range stt.Catalogue {
		if model.Engine != stt.EngineVAD {
			models = append(models, model)
		}
	}
	return models
}

func engineLabel(engine string) string {
	if engine == stt.EngineDictate {
		return "transcribe.cpp"
	}
	return "whisper.cpp"
}

// modelRow picks the speech model; one not on this computer is offered for
// download, and nothing downloads before the user has seen what and agreed
func (v *oniVoice) modelRow(e *env.Env, voice, dv protocol.OniVoiceSettings) look.Setting {
	th := e.Theme
	models := speechModels()
	ids := make([]string, 0, len(models)+1)
	ids = append(ids, "")
	options := []look.SelectOption{{Label: i18n.T("Hyakki's choice"), Detail: i18n.Tf("%s, or %s when that is missing", stt.DefaultModel, stt.FallbackModel)}}
	for _, model := range models {
		ids = append(ids, model.ID)
		detail := i18n.Tf("%s, on this computer", engineLabel(model.Engine))
		if !v.survey.Models[model.ID] {
			detail = i18n.Tf("%s, downloads %s when you agree", engineLabel(model.Engine), env.LocalSize(model.Size))
		}
		options = append(options, look.SelectOption{Label: model.ID, Detail: detail})
	}
	return look.Setting{Title: i18n.T("Speech model"), Detail: i18n.T("A model not on this computer downloads only after you have seen its size, licence and checksum and agreed."),
		Unavailable: v.downloadingNote(),
		Changed:     voice.STT.Model != dv.STT.Model, Reset: func() { voiceSet(e, map[string]any{"stt": map[string]any{"model": dv.STT.Model}}) },
		Control: func(gtx layout.Context) layout.Dimensions {
			v.model.Label = i18n.T("Speech model")
			gtx.Constraints.Min.X = gtx.Dp(selectWidth)
			dimensions := v.model.LayoutOptions(gtx, th, e.Overlays, options, indexOf(ids, voice.STT.Model))
			if index, ok := v.model.Changed(); ok && index >= 0 && index < len(ids) && ids[index] != voice.STT.Model {
				v.pickModel(e, ids[index])
			}
			return dimensions
		}}
}

func (v *oniVoice) downloadingNote() string {
	if v.downloading.ID != "" {
		return i18n.Tf("Downloading %s.", v.downloading.ID)
	}
	return ""
}

func (v *oniVoice) pickModel(e *env.Env, id string) {
	model, ok := stt.FindModel(id)
	if id == "" || (ok && v.survey.Models[id]) {
		values := map[string]any{"model": id}
		if ok {
			values["engine"] = model.Engine
		}
		voiceSet(e, map[string]any{"stt": values})
		return
	}
	if !ok || e.Voice.Download == nil {
		return
	}
	v.consentModel, v.consentVAD, v.downloadNote = model, stt.Model{}, ""
	// the fallback runs with its voice activity model, as the bench measured it
	if vad, ok := stt.FindModel(stt.FallbackVAD); ok && model.ID == stt.FallbackModel && !v.survey.Models[vad.ID] {
		v.consentVAD = vad
	}
	v.consent.Open, v.consent.Width = true, 560
}

// download fetches the agreed models one after another; when the last
// lands the first is chosen
func (v *oniVoice) download(e *env.Env, models ...stt.Model) {
	chosen, model := models[0], models[len(models)-1]
	v.downloading, v.fetched, v.downloadNote = model, 0, ""
	v.cancelDownload = e.Voice.Download(model.ID, func(fetched int64) { v.fetched = fetched }, func(err error) {
		v.downloading, v.cancelDownload = stt.Model{}, nil
		v.surveyed = false
		if err != nil {
			v.downloadNote = i18n.Tf("%s did not download: %v", model.ID, err)
			return
		}
		if len(models) > 1 {
			v.download(e, models[:len(models)-1]...)
			return
		}
		v.downloadNote = i18n.Tf("%s is on this computer and checked against its hash.", chosen.ID)
		voiceSet(e, map[string]any{"stt": map[string]any{"model": chosen.ID, "engine": chosen.Engine}})
	})
}

func (v *oniVoice) downloadRow(e *env.Env) (look.Setting, bool) {
	th := e.Theme
	switch {
	case v.downloading.ID != "":
		model := v.downloading
		v.stopDownload.Kind, v.stopDownload.Small, v.stopDownload.Label = kit.PlaqueQuiet, true, i18n.T("Stop")
		return look.Setting{Title: i18n.Tf("Downloading %s", model.ID), Detail: i18n.Tf("%s of %s", env.LocalSize(v.fetched), env.LocalSize(model.Size)), Below: true,
			Control: func(gtx layout.Context) layout.Dimensions {
				if v.stopDownload.Clicked(gtx) && v.cancelDownload != nil {
					v.cancelDownload()
				}
				fraction := 0.0
				if model.Size > 0 {
					fraction = float64(v.fetched) / float64(model.Size)
				}
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						return kit.BladeProgress{Fraction: fraction, Active: true}.Layout(gtx, th.Theme)
					}),
					layout.Rigid(layout.Spacer{Width: th.Sp.S3}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions { return v.stopDownload.Layout(gtx, th.Theme) }),
				)
			}}, true
	case v.downloadNote != "":
		return look.Setting{Title: i18n.T("Speech model download"), Detail: v.downloadNote}, true
	}
	return look.Setting{}, false
}

// continuousRows are the wake phrase and open mic: both keep the microphone
// on, so switching either on first asks, and switching off never does
func (v *oniVoice) continuousRows(e *env.Env, voice, dv protocol.OniVoiceSettings, off string) []look.Setting {
	th := e.Theme
	if v.phraseSeeded != voice.Wake.Phrase {
		v.phrase.SetText(voice.Wake.Phrase)
		v.phraseSeeded, v.phraseProblem = voice.Wake.Phrase, ""
	}
	wake := i18n.T("Listen for a wake phrase")
	open := i18n.T("Open mic")
	return []look.Setting{
		{Title: wake, Detail: i18n.T("Continuous microphone: say the phrase and Oni listens without the chord. The microphone stays on the whole time this is on, and Oni shows a mic mark you cannot hide."),
			Unavailable: off, Changed: voice.Wake.Enabled != dv.Wake.Enabled, Reset: func() { voiceSet(e, map[string]any{"wake": map[string]any{"enabled": false}}) },
			Control: func(gtx layout.Context) layout.Dimensions {
				return namedSwitch(gtx, th, wake, &v.wake, func(on bool) { v.flipContinuous(e, continuousWake, on) })
			}},
		{Title: i18n.T("Wake phrase"), Below: true, Detail: i18n.T("Two to four words. Saying it opens one question, as holding the chord does."),
			Changed: voice.Wake.Phrase != dv.Wake.Phrase, Reset: func() { voiceSet(e, map[string]any{"wake": map[string]any{"phrase": dv.Wake.Phrase}}) },
			Control: func(gtx layout.Context) layout.Dimensions { return v.layoutPhrase(gtx, e, voice.Wake.Phrase) }},
		{Title: open, Detail: i18n.T("Continuous microphone: everything Oni hears is a question, with no chord and no phrase, until it turns itself off. Oni shows a mic mark you cannot hide."),
			Unavailable: off, Changed: voice.OpenMic != dv.OpenMic, Reset: func() { voiceSet(e, map[string]any{"openMic": false}) },
			Control: func(gtx layout.Context) layout.Dimensions {
				return namedSwitch(gtx, th, open, &v.openMic, func(on bool) { v.flipContinuous(e, continuousOpen, on) })
			}},
	}
}

func (v *oniVoice) flipContinuous(e *env.Env, which string, on bool) {
	if !on {
		if which == continuousWake {
			voiceSet(e, map[string]any{"wake": map[string]any{"enabled": false}})
		} else {
			voiceSet(e, map[string]any{"openMic": false})
		}
		return
	}
	v.discloseFor = which
	v.disclose.Open, v.disclose.Width = true, 560
}

func (v *oniVoice) layoutPhrase(gtx layout.Context, e *env.Env, current string) layout.Dimensions {
	th := e.Theme
	v.phrase.Label, v.phrase.Placeholder, v.phrase.Editor.SingleLine, v.phrase.Editor.Submit = i18n.T("Phrase"), protocol.OniDefaultWakePhrase, true, true
	v.phrase.Error = v.phraseProblem
	submitted := false
	for {
		event, ok := v.phrase.Editor.Update(gtx)
		if !ok {
			break
		}
		switch event.(type) {
		case widget.SubmitEvent:
			submitted = true
		case widget.ChangeEvent:
			v.phraseProblem = ""
		}
	}
	v.phraseUse.Kind, v.phraseUse.Small, v.phraseUse.Label = kit.PlaqueTonal, true, i18n.T("Use this phrase")
	if (v.phraseUse.Clicked(gtx) || submitted) && strings.TrimSpace(v.phrase.Text()) != current {
		e.SetSettings(map[string]any{"oni": map[string]any{"voiceSettings": map[string]any{"wake": map[string]any{"phrase": strings.TrimSpace(v.phrase.Text())}}}}, func(err error) {
			if err != nil {
				v.phraseProblem = err.Error()
			}
		})
	}
	return layout.Flex{Alignment: layout.End}.Layout(gtx,
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions { return v.phrase.Layout(gtx, th.Theme) }),
		layout.Rigid(layout.Spacer{Width: th.Sp.S2}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions { return v.phraseUse.Layout(gtx, th.Theme) }),
	)
}

// overlay draws whichever question is open: the continuous microphone or a
// speech model's download
func (v *oniVoice) overlay(gtx layout.Context, e *env.Env) bool {
	if v.disclose.Open || v.discloseFor != "" {
		return v.layoutDisclose(gtx, e)
	}
	if v.consent.Open || v.consentModel.ID != "" {
		return v.layoutConsent(gtx, e)
	}
	return false
}

func (v *oniVoice) layoutDisclose(gtx layout.Context, e *env.Env) bool {
	th := e.Theme
	v.discloseNo.Kind, v.discloseNo.Label = kit.PlaqueQuiet, i18n.T("Cancel")
	v.discloseYes.Kind, v.discloseYes.Label = kit.PlaqueGold, i18n.T("Keep the microphone on")
	if v.discloseNo.Clicked(gtx) {
		v.disclose.Dismiss(gtx)
	}
	if v.discloseYes.Clicked(gtx) && v.disclose.Open {
		if v.discloseFor == continuousWake {
			voiceSet(e, map[string]any{"wake": map[string]any{"enabled": true}})
		} else {
			voiceSet(e, map[string]any{"openMic": true})
		}
		v.disclose.Dismiss(gtx)
	}
	title := i18n.T("Listen for the wake phrase?")
	what := i18n.T("Listening for the wake phrase keeps the microphone on continuously, not only while you hold the chord. What it hears is checked on this computer for the phrase and dropped; nothing is recorded, kept or sent until the phrase opens a question.")
	if v.discloseFor == continuousOpen {
		title = i18n.T("Turn on open mic?")
		what = i18n.T("Open mic keeps the microphone on continuously and treats everything it hears as a question, with no chord and no phrase. What it hears goes to Oni the way a held chord does.")
	}
	v.disclose.Layout(gtx, th.Theme, title, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(look.Paragraph(th, th.T.Row, th.R.Fg, what)),
			layout.Rigid(layout.Spacer{Height: th.Sp.S3}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions { return look.Facts(gtx, th, ceilingFacts(v.discloseFor)) }),
			layout.Rigid(layout.Spacer{Height: th.Sp.S3}.Layout),
			layout.Rigid(look.Paragraph(th, th.T.Detail, th.R.Fg2, i18n.T("What it hears never answers a consent question and never reaches Hands. Oni shows a mic mark the whole time, and the mark cannot be hidden. Switch it off here at any time."))),
		)
	},
		func(gtx layout.Context) layout.Dimensions { return v.discloseNo.Layout(gtx, th.Theme) },
		func(gtx layout.Context) layout.Dimensions { return v.discloseYes.Layout(gtx, th.Theme) },
	)
	if v.disclose.Closed() {
		v.discloseFor = ""
	}
	return v.disclose.Open
}

// ceilingFacts are the spec's limits on a microphone left on, as the
// daemon enforces them
func ceilingFacts(which string) []look.FactRow {
	limits, err := voiceCeilings()
	if err != nil {
		return []look.FactRow{{Label: i18n.T("Limits"), Value: i18n.Tf("The Oni motion spec does not load: %v.", err)}}
	}
	if which == continuousOpen {
		return []look.FactRow{
			{Label: i18n.T("Open mic"), Value: i18n.Tf("%s at most, then it turns itself off and must be switched on again", spokenSeconds(limits.Press.OpenMicS))},
		}
	}
	return []look.FactRow{
		{Label: i18n.T("Each question"), Value: i18n.Tf("Listens %s at most", spokenSeconds(limits.Wake.SessionS))},
		{Label: i18n.T("Armed"), Value: i18n.Tf("%s at most, then it turns itself off, tells you and must be switched on again", spokenSeconds(limits.Wake.ArmedMaxS))},
	}
}

func spokenSeconds(seconds float64) string {
	whole := int(math.Round(seconds))
	switch {
	case whole == 3600:
		return i18n.T("1 hour")
	case whole > 3600 && whole%3600 == 0:
		return i18n.Tf("%d hours", whole/3600)
	case whole == 60:
		return i18n.T("1 minute")
	case whole > 60 && whole%60 == 0:
		return i18n.Tf("%d minutes", whole/60)
	}
	return i18n.Tf("%d seconds", whole)
}

func (v *oniVoice) layoutConsent(gtx layout.Context, e *env.Env) bool {
	th := e.Theme
	model := v.consentModel
	v.consentNo.Kind, v.consentNo.Label = kit.PlaqueQuiet, i18n.T("Cancel")
	v.consentYes.Kind, v.consentYes.Label = kit.PlaqueGold, i18n.T("Download")
	if v.consentNo.Clicked(gtx) {
		v.consent.Dismiss(gtx)
	}
	if v.consentYes.Clicked(gtx) && v.consent.Open && v.downloading.ID == "" {
		v.consent.Dismiss(gtx)
		if v.consentVAD.ID != "" {
			v.download(e, model, v.consentVAD)
		} else {
			v.download(e, model)
		}
	}
	source := model.URL
	if parsed, err := url.Parse(model.URL); err == nil && parsed.Host != "" {
		source = parsed.Host
	}
	facts := []look.FactRow{
		{Label: i18n.T("Model"), Value: model.ID, Mono: true},
		{Label: i18n.T("Engine"), Value: engineLabel(model.Engine)},
		{Label: i18n.T("Size"), Value: env.LocalSize(model.Size)},
		{Label: i18n.T("Licence"), Value: model.License},
		{Label: i18n.T("SHA-256"), Value: model.SHA256, Mono: true},
		{Label: i18n.T("From"), Value: source},
	}
	if vad := v.consentVAD; vad.ID != "" {
		facts = append(facts,
			look.FactRow{Label: i18n.T("With"), Value: vad.ID, Mono: true},
			look.FactRow{Label: i18n.T("Its size"), Value: env.LocalSize(vad.Size)},
			look.FactRow{Label: i18n.T("Its licence"), Value: vad.License},
			look.FactRow{Label: i18n.T("Its SHA-256"), Value: vad.SHA256, Mono: true},
		)
	}
	note := i18n.Tf("Hyakki downloads it once into %s and keeps it only if it matches this checksum. Nothing downloads unless you agree here.", env.LocalPlace(stt.ModelDir()))
	if v.consentVAD.ID != "" {
		note = i18n.Tf("This model runs with %s, a small voice activity model that skips silence before it listens. Hyakki downloads both once into %s and keeps each only if it matches its checksum. Nothing downloads unless you agree here.", v.consentVAD.ID, env.LocalPlace(stt.ModelDir()))
	}
	v.consent.Layout(gtx, th.Theme, i18n.T("Download this speech model?"), func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions { return look.Facts(gtx, th, facts) }),
			layout.Rigid(layout.Spacer{Height: th.Sp.S3}.Layout),
			layout.Rigid(look.Paragraph(th, th.T.Detail, th.R.Fg2, note)),
		)
	},
		func(gtx layout.Context) layout.Dimensions { return v.consentNo.Layout(gtx, th.Theme) },
		func(gtx layout.Context) layout.Dimensions { return v.consentYes.Layout(gtx, th.Theme) },
	)
	if v.consent.Closed() {
		v.consentModel, v.consentVAD = stt.Model{}, stt.Model{}
	}
	return v.consent.Open
}
