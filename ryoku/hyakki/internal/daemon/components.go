package daemon

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"

	i18n "ryoku-i18n"

	"ryoku-rashin/internal/dictate/insert"
	"ryoku-rashin/internal/hands/chrome"
	"ryoku-rashin/internal/hands/nest"
	"ryoku-rashin/internal/oni/feature"
	"ryoku-rashin/internal/oni/hands/task"
	"ryoku-rashin/internal/oni/keep"
	"ryoku-rashin/internal/oni/profile"
	"ryoku-rashin/internal/oni/remind"
	"ryoku-rashin/internal/oni/routine"
	"ryoku-rashin/internal/oni/voice/stt"
	"ryoku-rashin/internal/oni/voice/tts"
	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/protocol"
	"ryoku-rashin/internal/rewrite/sel"
)

// Optional parts: components.list names every part Hyakki may use, with its
// state, size, place and what uses it; components.remove deletes one Hyakki
// downloaded or made. A listing reads the disk and looks programs up on PATH;
// it downloads, starts and probes nothing and keeps nothing once it returns.
// A removal deletes inside Hyakki's own folders only, through an os.Root at
// the part's folder, so no symlink leads it out; it is refused while the part
// is in use, and always for a program or anything Ryoku installed.

// part is one component and how to remove it; remove is nil when Hyakki may
// not.
type part struct {
	c      protocol.Component
	remove func() error
}

// The settings that use the parts.
const (
	partFeatureSTT       = "oni.voiceSettings.stt"
	partFeatureTTS       = "oni.voiceSettings.tts"
	partFeatureVoice     = "oni.voice"
	partFeatureDictation = "oni.voiceSettings.dictation"
	partFeatureCharacter = "oni.character"
	partFeatureHands     = "oni.hands"
	partFeatureInspect   = "oni.inspect"
	partFeatureGuide     = "oni.guide"
	partFeatureKeep      = "oni.guide.keep"
	partFeatureHints     = "oni.guide.hints"
	partFeatureRoutines  = "oni.reflexSettings.routines"
	partFeatureReminders = "oni.reflexSettings.reminders"
	partFeatureDecisions = "oni.reflexSettings.decisions"
	partFeatureRewrite   = "oni.rewrite"
)

// componentsList answers components.list.
func (d *Daemon) componentsList(ctx context.Context) protocol.ComponentsResult {
	parts := d.parts(ctx)
	out := make([]protocol.Component, len(parts))
	for i, p := range parts {
		out[i] = p.c
	}
	return protocol.ComponentsResult{Components: out}
}

// componentRemove answers components.remove: it deletes the part p names
// when Hyakki made it and nothing uses it now, then lists every part again.
func (d *Daemon) componentRemove(ctx context.Context, p protocol.ComponentRemoveParams) (protocol.ComponentsResult, error) {
	if !protocol.ValidComponentID(p.ID) {
		return protocol.ComponentsResult{}, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("That is not the name of an optional part."))
	}
	for _, pt := range d.parts(ctx) {
		if pt.c.ID != p.ID {
			continue
		}
		switch {
		case !pt.c.Removable || pt.remove == nil:
			message := pt.c.Why
			if message == "" {
				message = i18n.T("Hyakki does not remove this part.")
			}
			return protocol.ComponentsResult{}, protocol.Errorf(protocol.CodeConflict, "%s", message)
		case pt.c.InUse != "":
			return protocol.ComponentsResult{}, protocol.Errorf(protocol.CodeConflict, "%s", pt.c.InUse)
		case pt.c.State == protocol.ComponentAbsent:
			return d.componentsList(ctx), nil
		}
		if err := pt.remove(); err != nil {
			var refused *protocol.Error
			if errors.As(err, &refused) {
				return protocol.ComponentsResult{}, err
			}
			return protocol.ComponentsResult{}, protocol.Errorf(protocol.CodeInternal, "%s", i18n.Tf("%s could not be removed: %v.", pt.c.Title, err))
		}
		return d.componentsList(ctx), nil
	}
	return protocol.ComponentsResult{}, protocol.Errorf(protocol.CodeNotFound, "%s", i18n.T("Hyakki has no optional part by that name."))
}

// parts are every optional part in a fixed order: speech models, voice
// packs, the Hands browser profile, Oni's stores, caches, then programs.
func (d *Daemon) parts(ctx context.Context) []part {
	settings := d.Settings()
	oni := protocol.EffectiveOni(settings)
	on := func(id string) bool { return feature.Decide(feature.Request{Feature: id}, oni).Allow }
	characters := profile.List(paths.CharactersDir())
	var out []part
	out = append(out, speechModelParts(oni, on(feature.Voice))...)
	out = append(out, kokoroPart(ctx, oni, on(feature.Voice), activeCharacter(characters, oni.Character)))
	out = append(out, clipPackParts(ctx, characters)...)
	out = append(out, d.handsProfilePart(ctx), d.handsDesktopPart(ctx))
	out = append(out, d.storeParts(on)...)
	out = append(out, voiceCachePart(ctx))
	out = append(out, d.programParts(settings)...)
	return out
}

// activeCharacter is the selected character when it loads, else the shipped
// one, as oniActiveProfile picks it.
func activeCharacter(characters []profile.Loaded, selected string) profile.Profile {
	for _, l := range characters {
		if l.Valid && l.Profile.ID == selected {
			return l.Profile
		}
	}
	return profile.Default().Profile
}

// speechInUse names the speech model, and the speech detection model with
// it, that Oni listens with while voice is on: the one the settings name, or
// the first downloaded one in the order buildSTT tries them.
func speechInUse(s protocol.OniSTTSettings, voice bool) (model, vad string) {
	if !voice {
		return "", ""
	}
	if s.Model != "" {
		model = s.Model
	} else {
		ids := []string{stt.DefaultModel, stt.FallbackModel}
		for _, m := range stt.Catalogue {
			ids = append(ids, m.ID)
		}
		for _, id := range ids {
			m, ok := stt.FindModel(id)
			if !ok || m.Engine == stt.EngineVAD || s.Engine != "" && m.Engine != s.Engine {
				continue
			}
			if _, ok := stt.Locate(m); ok {
				model = id
				break
			}
		}
	}
	if model == stt.FallbackModel {
		vad = stt.FallbackVAD
	}
	return model, vad
}

// speechModelParts are the catalogue's speech models: Hyakki's own downloads,
// whole, partial or unusable, and a Parakeet model Ryoku's dictation keeps,
// which Hyakki uses but never removes.
func speechModelParts(oni protocol.OniSettings, voice bool) []part {
	dir := stt.ModelDir()
	model, vad := speechInUse(oni.VoiceSettings.STT, voice)
	out := make([]part, 0, len(stt.Catalogue))
	for _, m := range stt.Catalogue {
		c := protocol.Component{ID: protocol.ComponentModel + ":" + m.ID, Kind: protocol.ComponentModel, Feature: partFeatureSTT, Removable: true, State: protocol.ComponentAbsent}
		switch m.Engine {
		case stt.EngineVAD:
			c.Title = i18n.Tf("Speech detection model %s", m.ID)
			c.Uses = i18n.T("Finds where speech starts and stops for the whisper fallback model.")
		case stt.EngineDictate:
			c.Title = i18n.Tf("Speech model %s", m.ID)
			c.Uses = i18n.T("Talking to Oni and dictation, through ryoku-dictate.")
		default:
			c.Title = i18n.Tf("Speech model %s", m.ID)
			c.Uses = i18n.T("Talking to Oni and dictation, through whisper.cpp.")
		}
		own, partial := filepath.Join(dir, m.File), filepath.Join(dir, m.File+".part")
		ownSize, ownOK := fileSize(own)
		partSize, partOK := fileSize(partial)
		c.Bytes = ownSize + partSize
		switch {
		case stt.Installed(dir, m):
			c.State, c.Path = protocol.ComponentPresent, own
		case ownOK:
			c.State, c.Path = protocol.ComponentUnusable, own
		case partOK:
			c.State, c.Path = protocol.ComponentPartial, partial
		default:
			if path, ok := stt.Locate(m); ok {
				c.State, c.Path, c.Bytes, c.Removable = protocol.ComponentPresent, path, m.Size, false
				c.Why = i18n.T("Ryoku's dictation keeps this model; Hyakki uses it where it is and never removes it.")
			}
		}
		switch m.ID {
		case model:
			c.InUse = i18n.T("Oni listens with this model while voice is on. Pick another model or turn voice off first.")
		case vad:
			c.InUse = i18n.T("The speech model Oni listens with uses it while voice is on. Turn voice off first.")
		}
		var remove func() error
		if c.Removable {
			remove = func() error { return removeWithin(dir, m.File, m.File+".part") }
		}
		out = append(out, part{c: c, remove: remove})
	}
	return out
}

// kokoroPart is the optional Kokoro pack: Hyakki neither bundles nor
// downloads it, but the folder is Hyakki's, so it may go. Without it Oni
// speaks with the system voice.
func kokoroPart(ctx context.Context, oni protocol.OniSettings, voice bool, character profile.Profile) part {
	dir := tts.KokoroDir()
	c := protocol.Component{ID: protocol.ComponentPack + ":kokoro", Kind: protocol.ComponentPack, Title: i18n.T("Kokoro voice pack"),
		State: protocol.ComponentAbsent, Removable: true, Feature: partFeatureTTS,
		Uses: i18n.T("Oni's optional Kokoro voice. Without it Oni speaks with the system voice.")}
	if info, err := os.Lstat(dir); err == nil {
		c.Path = dir
		if info.IsDir() {
			c.Bytes = treeSize(ctx, dir)
		}
		c.State = protocol.ComponentPartial
		if tts.KokoroPackMissing(dir) == "" {
			c.State = protocol.ComponentPresent
		}
		if order := tts.Order(oni.VoiceSettings.TTS.Engine, character.Voice.Engine); voice && c.State == protocol.ComponentPresent && len(order) > 0 && order[0] == tts.Kokoro {
			c.InUse = i18n.T("Oni speaks with this voice while voice is on. Pick another voice or turn voice off first.")
		}
	}
	return part{c: c, remove: func() error { return removeWithin(filepath.Dir(dir), filepath.Base(dir)) }}
}

// clipPackParts are the recorded voices of the user's characters. A clip
// pack belongs to its character, which does not load without it, so it is
// listed and kept.
func clipPackParts(ctx context.Context, characters []profile.Loaded) []part {
	var out []part
	for _, l := range characters {
		p := l.Profile
		if !l.Valid || p.Shipped || p.Voice.Engine != profile.VoiceClips || p.Voice.ClipPack == "" {
			continue
		}
		id := protocol.ComponentPack + ":clips:" + p.ID
		if !protocol.ValidComponentID(id) {
			continue
		}
		dir := filepath.Join(paths.CharactersDir(), p.ID, filepath.FromSlash(filepath.Clean(p.Voice.ClipPack)))
		name := p.Name
		if name == "" {
			name = p.ID
		}
		out = append(out, part{c: protocol.Component{ID: id, Kind: protocol.ComponentPack, Title: i18n.Tf("Recorded voice of %s", name),
			State: protocol.ComponentPresent, Bytes: treeSize(ctx, dir), Path: dir, Feature: partFeatureCharacter,
			Uses: i18n.Tf("%s speaks with these clips.", name),
			Why:  i18n.T("It belongs to its character, which does not load without it. It goes with the character's folder.")}})
	}
	return out
}

// handsProfilePart is the Hands browser's own profile, removed as Forget
// removes it.
func (d *Daemon) handsProfilePart(ctx context.Context) part {
	dir := handsProfile()
	c := protocol.Component{ID: protocol.ComponentProfile + ":" + chrome.ProfileName, Kind: protocol.ComponentProfile, Title: i18n.T("Hands browser profile"),
		State: protocol.ComponentAbsent, Removable: true, Feature: partFeatureHands,
		Uses: i18n.T("The Hands browser's sign-ins, cookies and history, apart from your own browser.")}
	if dirExists(dir) {
		c.State, c.Path, c.Bytes = protocol.ComponentPresent, dir, treeSize(ctx, dir)
	}
	if d.oni.hands.open.Load() {
		c.InUse = handsLineOpenNow
	}
	return part{c: c, remove: func() error { _, err := d.handsForget(); return err }}
}

// handsDesktopPart is what the separate desktop leaves: the home its
// programs keep between tasks, and a runtime folder a killed daemon left.
func (d *Daemon) handsDesktopPart(ctx context.Context) part {
	home := handsNestHome()
	runtime := ""
	if d.deps.OniHands.Nest == nil {
		runtime = handsNestDir(os.Getenv)
	}
	c := protocol.Component{ID: protocol.ComponentData + ":hands-desktop", Kind: protocol.ComponentData, Title: i18n.T("Separate desktop leftovers"),
		State: protocol.ComponentAbsent, Removable: true, Feature: partFeatureHands,
		Uses: i18n.T("What the programs on Hands' separate desktop keep between tasks.")}
	for _, dir := range []string{home, runtime} {
		if dir != "" && dirExists(dir) {
			c.Bytes += treeSize(ctx, dir)
			if c.Path == "" {
				c.Path = dir
			}
			c.State = protocol.ComponentPresent
		}
	}
	if d.oni.hands.active.Load() {
		c.InUse = i18n.T("A Hands task is running. Stop it first.")
	}
	return part{c: c, remove: func() error {
		if runtime != "" && dirExists(runtime) {
			if err := nest.Sweep(runtime); err != nil {
				return err
			}
		}
		return removeWithin(filepath.Dir(home), filepath.Base(home))
	}}
}

// partStore is a store file Oni keeps: its file and where a damaged one was
// moved.
type partStore interface {
	Path() string
	DamagedPath() string
}

// storeParts are Oni's own stores. Each is removable while the feature that
// writes it is off; turned off, nothing holds it open.
func (d *Daemon) storeParts(on func(string) bool) []part {
	keeps := d.deps.OniGuide.Keeps
	if keeps == nil {
		keeps = keep.NewStore("")
	}
	reminders := d.deps.OniReflex.Reminders
	if reminders == nil {
		reminders = remind.NewStore("")
	}
	routines := d.deps.OniReflex.Routines
	if routines == nil {
		routines = routine.NewStore("")
	}
	inUse := func(busy bool, line string) string {
		if busy {
			return line
		}
		return ""
	}
	handsBusy := on(feature.HandsBrowser) || on(feature.HandsApp) || on(feature.HandsDesktop) || d.oni.hands.active.Load()
	return []part{
		storePart("walkthroughs", i18n.T("Kept walkthroughs"), i18n.T("Guides Oni kept to replay on this computer."), partFeatureKeep, keeps,
			inUse(on(feature.GuideKeep), i18n.T("Kept walkthroughs are on. Turn them off first."))),
		storePart("hints", i18n.T("Hint record"), i18n.T("When Oni last offered each kept walkthrough as a hint."), partFeatureHints, keeps.Hints(),
			inUse(on(feature.GuideHints), i18n.T("Hints are on. Turn them off first."))),
		storePart("reminders", i18n.T("Reminders and timers"), i18n.T("The reminders and timers waiting to be said."), partFeatureReminders, reminders,
			inUse(d.remindKindsNow().any(), i18n.T("Reminders or timers are on. Turn them off first."))),
		storePart("routines", i18n.T("Routines"), i18n.T("The runs of quick commands you kept."), partFeatureRoutines, routines,
			inUse(on(feature.Routines), i18n.T("Routines are on. Turn them off first."))),
		storePart("hands-journal", i18n.T("Hands task journal"), i18n.T("The tasks Hands was given and how each ended."), partFeatureHands, task.Open(d.handsJournalPath()),
			inUse(handsBusy, i18n.T("Hands is on. Turn it off first."))),
		storePart("dictation", i18n.T("Dictation words"), i18n.T("Your dictation words, snippets and corrections."), partFeatureDictation, d.voiceStore(),
			inUse(on(feature.VoiceDictation), i18n.T("Dictation is on. Turn it off first."))),
	}
}

// storePart lists one store: its file and any damaged copy beside it.
func storePart(name, title, uses, setting string, s partStore, inUse string) part {
	c := protocol.Component{ID: protocol.ComponentData + ":" + name, Kind: protocol.ComponentData, Title: title, Uses: uses, Feature: setting,
		State: protocol.ComponentAbsent, Removable: true, InUse: inUse}
	file, damaged := s.Path(), s.DamagedPath()
	for _, p := range []string{file, damaged} {
		if lexists(p) {
			size, _ := fileSize(p)
			c.Bytes += size
			c.State = protocol.ComponentPresent
		}
	}
	if c.State == protocol.ComponentPresent {
		c.Path = file
	}
	dir := filepath.Dir(file)
	return part{c: c, remove: func() error {
		if filepath.Dir(damaged) != dir {
			return errors.New(i18n.T("the store's damaged copy is not beside it"))
		}
		return removeWithin(dir, filepath.Base(file), filepath.Base(damaged))
	}}
}

// voiceCachePart is the cache of rendered sentences, rebuilt as Oni speaks.
func voiceCachePart(ctx context.Context) part {
	dir := tts.CacheDir()
	c := protocol.Component{ID: protocol.ComponentCache + ":voice", Kind: protocol.ComponentCache, Title: i18n.T("Spoken sentence cache"),
		State: protocol.ComponentAbsent, Removable: true, Feature: partFeatureVoice,
		Uses: i18n.T("Sentences Oni already said, kept so they are said again at once. It fills again as Oni speaks.")}
	if dirExists(dir) {
		c.State, c.Path, c.Bytes = protocol.ComponentPresent, dir, treeSize(ctx, dir)
	}
	return part{c: c, remove: func() error { return removeWithin(filepath.Dir(dir), filepath.Base(dir)) }}
}

// programParts are the programs a distribution installs that Oni may run,
// each with the setting that needs it and why. Hyakki never installs or
// removes one; a listing looks each up on PATH and starts none.
func (d *Daemon) programParts(settings protocol.Settings) []part {
	program := func(name, title, setting, uses, why string, names ...string) part {
		c := protocol.Component{ID: protocol.ComponentProgram + ":" + name, Kind: protocol.ComponentProgram, Title: title, State: protocol.ComponentAbsent,
			Feature: setting, Uses: uses, Why: why, Needs: names}
		for _, n := range names {
			if path, err := exec.LookPath(n); err == nil {
				c.State, c.Path = protocol.ComponentPresent, path
				break
			}
		}
		return part{c: c}
	}
	installed := i18n.T("Your distribution installs it; Hyakki never installs or removes it.")
	out := []part{
		program("tesseract", i18n.T("Tesseract text reader"), partFeatureGuide, i18n.T("Guides."),
			i18n.T("Reads the words in a window so a guide can point at them.")+" "+installed, "tesseract", "tesseract-ocr"),
		program("grim", i18n.T("grim screenshots"), partFeatureInspect, i18n.T("Looking at a window, and Hands' separate desktop."),
			i18n.T("Takes the picture of the window Oni looks at.")+" "+installed, "grim"),
		program("recorder", i18n.T("PipeWire or PulseAudio sound tools"), partFeatureVoice, i18n.T("Voice."),
			i18n.T("Reads the microphone and plays Oni's voice: pw-cat, or parec and paplay.")+" "+installed, "pw-cat", "parec"),
		program("whisper-server", i18n.T("whisper.cpp server"), partFeatureSTT, i18n.T("Voice, with a whisper model."),
			i18n.T("Turns speech into text with the whisper models.")+" "+installed, "whisper-server"),
		program("ryoku-dictate", i18n.T("Ryoku dictation"), partFeatureSTT, i18n.T("Voice, with a Parakeet model."),
			i18n.T("Turns speech into text with the Parakeet models.")+" "+installed, "ryoku-dictate"),
		program("espeak-ng", i18n.T("eSpeak NG"), partFeatureTTS, i18n.T("Spoken replies."),
			i18n.T("The system voice Oni speaks with when no other voice is there.")+" "+installed, "espeak-ng"),
		program(tts.KokoroBinary, i18n.T("sherpa-onnx speech"), partFeatureTTS, i18n.T("The Kokoro voice pack."),
			i18n.T("Speaks with the optional Kokoro voice pack.")+" "+installed, tts.KokoroBinary),
		program(insert.Tool, i18n.T("Typing tool"), partFeatureDictation, i18n.T("Dictation, and Hands' separate desktop."),
			i18n.T("Types dictation into the window you are in, and Hands' keys on its own desktop.")+" "+installed, insert.Tool),
		program(sel.Tool, i18n.T("wl-clipboard"), partFeatureRewrite, i18n.T("Rewrite in place."),
			i18n.T("Reads the text you selected for a rewrite.")+" "+installed, sel.Tool),
		program("weston", i18n.T("Weston"), partFeatureHands, i18n.T("Hands' separate desktop."),
			i18n.T("Holds Hands' separate desktop, apart from yours.")+" "+installed, "weston"),
		program("niri", i18n.T("niri"), partFeatureHands, i18n.T("Hands' separate desktop."),
			i18n.T("Arranges the windows on Hands' separate desktop.")+" "+installed, "niri"),
	}
	browser := protocol.Component{ID: protocol.ComponentProgram + ":browser", Kind: protocol.ComponentProgram, Title: i18n.T("A Chromium-family browser"),
		State: protocol.ComponentAbsent, Feature: partFeatureHands, Uses: i18n.T("The Hands browser."), Needs: []string{"chromium"},
		Why: i18n.T("Hands starts its own browser from it; Firefox and Flatpak or snap builds can't be driven.") + " " + installed}
	if prog, err := d.handsFind(); err == nil && len(prog.Exec) > 0 {
		browser.State, browser.Path = protocol.ComponentPresent, prog.Exec[0]
	}
	out = append(out, part{c: browser})
	decisions := protocol.Component{ID: protocol.ComponentProgram + ":decision-server", Kind: protocol.ComponentProgram, Title: i18n.T("Local decision server"),
		State: protocol.ComponentUnknown, Feature: partFeatureDecisions, Uses: i18n.T("Quick-command decisions on the local route."),
		Why: i18n.T("A server you run yourself on this computer. Hyakki asks it only when quick-command decisions use the local route; a listing does not look for it, and Hyakki never installs or starts one.")}
	if address := settings.Providers.Decisions.LocalAddress; address != "" {
		decisions.Path = address
	}
	return append(out, part{c: decisions})
}

// removeWithin deletes names inside dir and nothing else: through an os.Root
// at dir, so no name and no symlink on the way leads out of it, and a
// symlink is removed as a link. A dir that is not there has nothing to
// remove.
func removeWithin(dir string, names ...string) error {
	root, err := os.OpenRoot(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	defer root.Close()
	for _, name := range names {
		if name == "" || name == "." || filepath.Base(name) != name {
			return errors.New(i18n.T("a part's name is not a plain file name"))
		}
		if err := root.RemoveAll(name); err != nil {
			return err
		}
	}
	return nil
}

// fileSize is the size of the regular file at path; ok is false when there
// is none.
func fileSize(path string) (int64, bool) {
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() {
		return 0, false
	}
	return info.Size(), true
}

// treeSize is the size of the regular files under dir, symlinks not
// followed.
func treeSize(ctx context.Context, dir string) int64 {
	size, _, _ := diskWalk(ctx, dir, "", nil)
	return size
}

// lexists reports whether anything, a link included, lies at path.
func lexists(path string) bool {
	_, err := os.Lstat(path)
	return err == nil
}
