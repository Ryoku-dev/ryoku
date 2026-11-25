package shell

import (
	"context"
	"image"
	"slices"
	"strings"
	"testing"

	"gioui.org/io/input"
	"gioui.org/io/key"
	"gioui.org/io/semantic"

	"ryoku-rashin/internal/app/demo"
	"ryoku-rashin/internal/dictate"
	"ryoku-rashin/internal/oni/voice/stt"
	"ryoku-rashin/internal/protocol"
)

// newVoiceStage navigates to Voice and dictation. Only its own controls are
// drawn; scrolling reaches the remaining controls in the bounded viewport.
func newVoiceStage(t *testing.T, session string) *stage {
	t.Helper()
	stage := newStage(t, image.Pt(1440, 2200), Options{Route: protocol.TabSettings, SettingsSection: "oni-voice", Session: session})
	stage.open("settings/oni-voice")
	stage.await("the voice group reads the demo machine", func() bool {
		_, ok := stage.named("Speech model: Hyakki's choice")
		return ok
	})
	return stage
}

func (s *stage) daemonOni() protocol.OniSettings {
	s.t.Helper()
	var settings protocol.Settings
	if err := s.runtime.client.Call(context.Background(), protocol.MSettingsGet, protocol.Empty{}, &settings); err != nil {
		s.t.Fatal(err)
	}
	return settings.Oni
}

// toggle presses the switch called name: of the nodes with that name, the
// one that is a control rather than the row's text
func (s *stage) toggle(name string) {
	s.t.Helper()
	s.settingsReveal(name)
	for _, node := range s.nodes() {
		if node.Desc.Label == name && node.Desc.Class == semantic.Switch {
			s.clickAt(centre(node.Desc.Bounds))
			return
		}
	}
	for _, node := range s.nodes() {
		if node.Desc.Label != name {
			continue
		}
		for _, child := range s.nodes() {
			if child.ParentID == node.ID && child.Desc.Class == semantic.Switch {
				s.clickAt(centre(child.Desc.Bounds))
				return
			}
		}
	}
	s.t.Fatalf("no switch is called %q; there is: %s", name, strings.Join(s.names(), ", "))
}

// field focuses the text field called name: its editor, not its label
func (s *stage) field(name string) {
	s.t.Helper()
	s.settingsReveal(name)
	for _, node := range s.nodes() {
		if node.Desc.Label == name && node.Desc.Class == semantic.Editor {
			s.clickAt(centre(node.Desc.Bounds))
			return
		}
	}
	s.t.Fatalf("no field is called %q; there is: %s", name, strings.Join(s.names(), ", "))
}

// mentions reports whether anything on screen is called text or has it in
// its name
func (s *stage) mentions(text string) bool {
	return slices.ContainsFunc(s.names(), func(name string) bool { return strings.Contains(name, text) })
}

// Each switch, voice and mode writes its own setting and nothing else.
func TestOniVoiceSwitchesPatchSettings(t *testing.T) {
	stage := newVoiceStage(t, demo.FeatureOni)
	if oni := stage.daemonOni(); oni.Voice || oni.VoiceSettings.SpeakReplies || oni.VoiceSettings.Dictation.Enabled {
		t.Fatalf("voice starts on: %+v", oni)
	}
	stage.toggle("Talk to Oni")
	stage.await("talking to Oni turns on", func() bool { return stage.daemonOni().Voice })
	stage.toggle("Speak replies")
	stage.await("spoken replies turn on", func() bool { return stage.daemonOni().VoiceSettings.SpeakReplies })
	stage.toggle("Take dictation")
	stage.await("dictation turns on", func() bool { return stage.daemonOni().VoiceSettings.Dictation.Enabled })

	stage.choose("Voice: The character's own", "System voice")
	stage.await("the system voice is chosen", func() bool { return stage.daemonOni().VoiceSettings.TTS.Engine == protocol.OniTTSSystem })
	stage.click("Voice: System voice")
	var kokoro input.SemanticNode
	stage.await("Kokoro is listed", func() bool {
		for _, node := range stage.nodes() {
			if node.Desc.Label == "Kokoro" && node.Desc.Gestures&input.ClickGesture != 0 {
				kokoro = node
				return true
			}
		}
		return false
	})
	if !kokoro.Desc.Disabled {
		t.Fatalf("Kokoro is not installed on the demo machine but can be chosen: %+v", kokoro.Desc)
	}
	stage.clickAt(centre(kokoro.Desc.Bounds))
	stage.press(key.NameEscape, 0)
	if got := stage.daemonOni().VoiceSettings.TTS.Engine; got != protocol.OniTTSSystem {
		t.Fatalf("picking the missing Kokoro changed the voice to %q", got)
	}

	stage.choose("Dictation mode: Clean", "Exact")
	stage.await("dictation types exact", func() bool { return stage.daemonOni().VoiceSettings.Dictation.Mode == protocol.OniDictateExact })
	oni := stage.daemonOni()
	if oni.VoiceSettings.Wake.Enabled || oni.VoiceSettings.OpenMic || oni.VoiceSettings.Dictation.Polish != protocol.OniPolishLocal {
		t.Fatalf("the switches reached settings they do not own: %+v", oni.VoiceSettings)
	}
}

// The wake phrase, open mic and a hosted polish are each a step past what
// they say: cancelling leaves them off, and only the yes turns them on.
func TestOniContinuousMicrophoneAndHostedPolishNeedTheirDisclosure(t *testing.T) {
	stage := newVoiceStage(t, demo.FeatureOniVoice)

	stage.toggle("Listen for a wake phrase")
	stage.await("the wake question opens", func() bool { return stage.mentions("Listen for the wake phrase?") })
	for _, ceiling := range []string{"Listens 15 seconds at most", "1 hour at most", "keeps the microphone on continuously"} {
		if !stage.mentions(ceiling) {
			t.Fatalf("the wake question does not say %q; there is: %v", ceiling, stage.names())
		}
	}
	if stage.daemonOni().VoiceSettings.Wake.Enabled {
		t.Fatal("the wake phrase turned on before the question was answered")
	}
	stage.click("Cancel")
	stage.await("the question closes", func() bool { return !stage.mentions("Listen for the wake phrase?") })
	if stage.daemonOni().VoiceSettings.Wake.Enabled {
		t.Fatal("cancelling turned the wake phrase on")
	}
	stage.toggle("Listen for a wake phrase")
	stage.await("the wake question opens again", func() bool { return stage.mentions("Listen for the wake phrase?") })
	stage.click("Keep the microphone on")
	stage.await("the wake phrase turns on", func() bool { return stage.daemonOni().VoiceSettings.Wake.Enabled })

	stage.toggle("Open mic")
	stage.await("the open mic question opens", func() bool { return stage.mentions("Turn on open mic?") })
	if !stage.mentions("3 minutes at most") || stage.daemonOni().VoiceSettings.OpenMic {
		t.Fatalf("open mic is on before its question, or the question hides its ceiling; there is: %v", stage.names())
	}
	stage.press(key.NameEscape, 0)
	stage.await("escape closes the question", func() bool { return !stage.mentions("Turn on open mic?") })
	if stage.daemonOni().VoiceSettings.OpenMic {
		t.Fatal("escaping the question turned open mic on")
	}
	stage.toggle("Open mic")
	stage.await("the open mic question opens again", func() bool { return stage.mentions("Turn on open mic?") })
	stage.click("Keep the microphone on")
	stage.await("open mic turns on", func() bool { return stage.daemonOni().VoiceSettings.OpenMic })
	stage.toggle("Open mic")
	stage.await("open mic turns off without a question", func() bool { return !stage.daemonOni().VoiceSettings.OpenMic })
	if stage.mentions("Turn on open mic?") {
		t.Fatal("turning open mic off asked first")
	}

	stage.choose("Polish with: This computer", "The Ask route")
	stage.await("the hosted polish question opens", func() bool { return stage.mentions("Polish on the Ask route?") })
	if stage.daemonOni().VoiceSettings.Dictation.Polish != protocol.OniPolishLocal {
		t.Fatal("the Ask route was chosen before the question was answered")
	}
	stage.click("Cancel")
	stage.await("the polish question closes", func() bool { return !stage.mentions("Polish on the Ask route?") })
	if stage.daemonOni().VoiceSettings.Dictation.Polish != protocol.OniPolishLocal {
		t.Fatal("cancelling chose the Ask route")
	}
	stage.choose("Polish with: This computer", "The Ask route")
	stage.await("the hosted polish question opens again", func() bool { return stage.mentions("Polish on the Ask route?") })
	stage.click("Use the Ask route")
	stage.await("the Ask route polishes", func() bool { return stage.daemonOni().VoiceSettings.Dictation.Polish == protocol.OniPolishRoute })
}

// A speech model that is not on the computer downloads only after the user
// has seen its size, licence and hash and said yes; one that is there is
// simply chosen.
func TestOniSpeechModelDownloadsOnlyAfterConsent(t *testing.T) {
	stage := newVoiceStage(t, demo.FeatureOniVoice)
	turbo, _ := stt.FindModel("whisper-large-v3-turbo-q5_0")

	stage.choose("Speech model: Hyakki's choice", turbo.ID)
	stage.await("the download question opens", func() bool { return stage.mentions("Download this speech model?") })
	for _, fact := range []string{turbo.SHA256, turbo.License, "huggingface.co"} {
		if !stage.mentions(fact) {
			t.Fatalf("the download question does not show %q; there is: %v", fact, stage.names())
		}
	}
	if got := stage.daemonOni().VoiceSettings.STT.Model; got != "" {
		t.Fatalf("the model %q was chosen before the question was answered", got)
	}
	stage.click("Cancel")
	stage.await("the question closes", func() bool { return !stage.mentions("Download this speech model?") })
	if stage.runtime.demoVoice.models[turbo.ID] || stage.daemonOni().VoiceSettings.STT.Model != "" {
		t.Fatal("cancelling downloaded or chose the model")
	}

	stage.choose("Speech model: Hyakki's choice", turbo.ID)
	stage.await("the download question opens again", func() bool { return stage.mentions("Download this speech model?") })
	stage.click("Download")
	stage.await("the downloaded model is chosen", func() bool {
		stt := stage.daemonOni().VoiceSettings.STT
		return stt.Model == turbo.ID && stt.Engine == protocol.OniSTTWhisper
	})
	if !stage.runtime.demoVoice.models[turbo.ID] {
		t.Fatal("the model was chosen without being downloaded")
	}
	stage.await("the row says the model arrived", func() bool { return stage.mentions("checked against its hash") })

	stage.choose("Speech model: "+turbo.ID, stt.DefaultModel)
	stage.await("the installed model is chosen at once", func() bool { return stage.daemonOni().VoiceSettings.STT.Model == stt.DefaultModel })
	if stage.mentions("Download this speech model?") {
		t.Fatal("a model already on the computer asked to download")
	}
}

// The fallback model runs with its voice activity model, as the bench
// measured it, so agreeing to download the fallback shows and fetches both,
// and chooses the fallback once both are on the computer.
func TestOniFallbackModelDownloadsWithItsVAD(t *testing.T) {
	stage := newVoiceStage(t, demo.FeatureOniVoice)
	vad, _ := stt.FindModel(stt.FallbackVAD)

	stage.choose("Speech model: Hyakki's choice", stt.FallbackModel)
	stage.await("the download question opens", func() bool { return stage.mentions("Download this speech model?") })
	for _, fact := range []string{vad.ID, vad.SHA256} {
		if !stage.mentions(fact) {
			t.Fatalf("the download question does not show the voice activity model's %q; there is: %v", fact, stage.names())
		}
	}
	stage.click("Download")
	stage.await("the fallback is chosen", func() bool { return stage.daemonOni().VoiceSettings.STT.Model == stt.FallbackModel })
	stage.runtime.demoVoice.mu.Lock()
	got := []bool{stage.runtime.demoVoice.models[stt.FallbackModel], stage.runtime.demoVoice.models[stt.FallbackVAD]}
	stage.runtime.demoVoice.mu.Unlock()
	if !got[0] || !got[1] {
		t.Fatalf("downloaded: fallback %v, its VAD %v; want both", got[0], got[1])
	}
}

// The chord takes Ctrl, Alt or Super and is refused, with why, otherwise;
// AltGr never counts.
func TestOniChordRebindRefusesChordsThatWouldType(t *testing.T) {
	stage := newVoiceStage(t, demo.FeatureOniVoice)
	typed := protocol.OniDefaultChord
	retype := func(chord string) {
		stage.field("Chord")
		stage.typeOver(len([]rune(typed)), chord)
		typed = chord
		stage.click("Use this chord")
		stage.settle()
	}
	for _, chord := range []string{"shift+a", "a", "ctrl+alt+shift+super+k", "altgr+k"} {
		retype(chord)
		if got := stage.daemonOni().Chord; got != protocol.OniDefaultChord {
			t.Fatalf("%q was taken as the chord %q", chord, got)
		}
	}
	if !stage.mentions("AltGr cannot be part of the chord") {
		t.Fatalf("AltGr is refused without saying why; there is: %v", stage.names())
	}
	retype("shift+a")
	if !stage.mentions("Use Ctrl, Alt or Super") {
		t.Fatalf("a chord that would type is refused without saying why; there is: %v", stage.names())
	}
	retype("Super+Space")
	stage.await("the chord is kept", func() bool { return stage.daemonOni().Chord == "super+space" })
}

// The test listens without recording and lists the sources; a machine with
// no microphone and a keyboard Oni may not read says both.
func TestOniMicrophoneTestAndItsProblems(t *testing.T) {
	stage := newVoiceStage(t, demo.FeatureOniVoice)
	if stage.mentions("The keyboard cannot be read") {
		t.Fatal("the demo keyboard is readable but Settings says it is not")
	}
	stage.click("Test")
	stage.await("the meter reads the default microphone", func() bool { return stage.mentions("Heard from Default microphone, loudest at") })
	if !stage.mentions("Nothing was recorded") {
		t.Fatalf("the test does not say it kept nothing; there is: %v", stage.names())
	}
	stage.choose("Microphone: Default microphone", "Yeti Stereo Microphone")
	stage.await("the Yeti is chosen", func() bool {
		return stage.daemonOni().VoiceSettings.Mic == "alsa_input.usb-Yeti_Stereo_Microphone-00.analog-stereo"
	})

	quiet := newVoiceStage(t, demo.FeatureOniNoMic)
	quiet.click("Test")
	quiet.await("the meter says no sound arrived", func() bool { return quiet.mentions("No sound arrived from the microphone") })
	if !quiet.mentions("The keyboard cannot be read") || !quiet.mentions("bindr = CTRL ALT, Space, exec, hyakki oni talk stop") {
		t.Fatalf("a keyboard Oni may not read is not explained with the desktop bind; there is: %v", quiet.names())
	}
}

// Dictionary, snippet and style edits land in the dictation store and
// survive leaving Settings; an entry past the store's bounds is refused in
// the editor and changes nothing.
func TestDictationStoreEditsPersistAndRefuseOverBounds(t *testing.T) {
	stage := newVoiceStage(t, demo.FeatureOniVoice)
	store := dictate.NewStore(stage.runtime.demoVoice.dictation.Path())
	fill := func(first, second string) {
		stage.field("You say")
		stage.typeText(first)
		stage.field("Hyakki types")
		stage.typeText(second)
	}

	stage.click("Add a word")
	fill("hyakki dev", "HyakkiDev")
	stage.click("Add")
	stage.await("the word is kept", func() bool {
		personal, err := store.Load()
		return err == nil && slices.Equal(personal.Dictionary, []dictate.Entry{{Spoken: "hyakki dev", Written: "HyakkiDev"}})
	})

	stage.click("Add a word")
	fill("one two three four five six seven", "Seven")
	stage.click("Add")
	stage.settle()
	if !stage.mentions("Say it in up to 6 words") || !stage.mentions("You say") {
		t.Fatalf("a seven-word phrase was not refused in the open editor; there is: %v", stage.names())
	}
	if personal, err := store.Load(); err != nil || len(personal.Dictionary) != 1 {
		t.Fatalf("a refused word reached the store: %+v, %v", personal.Dictionary, err)
	}
	stage.click("Cancel")

	stage.click("Add a snippet")
	fill("sign off", "Thanks, Nero")
	stage.click("Add")
	stage.await("the snippet is kept", func() bool {
		personal, err := store.Load()
		return err == nil && slices.Equal(personal.Snippets, []dictate.Snippet{{Trigger: "sign off", Text: "Thanks, Nero"}})
	})

	stage.click("Add an app")
	stage.field("App id")
	stage.typeText("org.gnome.Console")
	stage.choose("Mode: The mode above", "Exact")
	stage.click("Add")
	stage.await("the style is kept", func() bool {
		personal, err := store.Load()
		return err == nil && len(personal.Styles) == 1 && personal.Styles[0].App == "org.gnome.Console" && personal.Styles[0].Mode == protocol.OniDictateExact
	})

	stage.click("Add an app")
	stage.field("App id")
	stage.typeText("an app with spaces")
	stage.click("Add")
	stage.settle()
	if personal, err := store.Load(); err != nil || len(personal.Styles) != 1 || !stage.mentions("Name the app by its id, without spaces") {
		t.Fatalf("an app id with spaces was not refused: %+v, %v; there is: %v", personal.Styles, err, stage.names())
	}
	stage.click("Cancel")

	stage.click("Remove hyakki dev")
	stage.await("the word is removed", func() bool {
		personal, err := store.Load()
		return err == nil && len(personal.Dictionary) == 0
	})

	stage.open("settings/agents")
	stage.open("settings/oni-voice")
	stage.settingsReveal("Add a snippet")
	stage.await("the store is read again", func() bool { return stage.mentions("sign off") && stage.mentions("org.gnome.Console") })
	if stage.mentions("hyakki dev") {
		t.Fatal("the removed word came back")
	}
}

// A dictation that could not type everything keeps the rest here, with a
// Copy that copies exactly it.
func TestDictationRestStaysWithCopy(t *testing.T) {
	stage := newVoiceStage(t, demo.FeatureOniVoice)
	stage.click("Start dictation")
	stage.await("dictation listens", func() bool { return stage.mentions("Stop dictation") })
	stage.click("Stop dictation")
	stage.await("the rest stays here", func() bool { return stage.mentions("This text stayed here") })
	if !stage.mentions("The window closed before all of it was typed.") || !stage.mentions("34 characters reached the window.") {
		t.Fatalf("the rest does not say why it stayed; there is: %v", stage.names())
	}
	stage.click("Copy")
	stage.settle()
	if got := stage.clipboard(); got != " Bring the lantern, and the map." {
		t.Fatalf("Copy put %q on the clipboard", got)
	}
	stage.click("Clear")
	stage.await("the rest goes", func() bool { return !stage.mentions("This text stayed here") })
}
