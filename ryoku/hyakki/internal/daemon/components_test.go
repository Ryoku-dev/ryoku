package daemon

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"ryoku-rashin/internal/config"
	"ryoku-rashin/internal/dictate/insert"
	"ryoku-rashin/internal/hands/nest"
	"ryoku-rashin/internal/oni/profile"
	"ryoku-rashin/internal/oni/remind"
	"ryoku-rashin/internal/oni/routine"
	"ryoku-rashin/internal/oni/voice/stt"
	"ryoku-rashin/internal/oni/voice/tts"
	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/protocol"
	"ryoku-rashin/internal/rewrite/sel"
)

// partsDaemon is a daemon that is not serving, on isolated folders, with
// nothing on PATH but bin, no browser to find and no nested desktop, and the
// settings of a machine that never changed one, edited by edit.
func partsDaemon(t *testing.T, edit func(*protocol.Settings)) (*Daemon, string) {
	t.Helper()
	isolateDaemon(t)
	bin := t.TempDir()
	t.Setenv("PATH", bin)
	d := New(Deps{OniHands: OniHandsDeps{
		Find: func() (OniHandsProgram, error) { return OniHandsProgram{}, errHandsNoBrowser },
		Nest: func(context.Context, nest.Options) (OniHandsNest, error) {
			return nil, errors.New("no nested desktop in this test")
		},
	}})
	partsSettings(d, edit)
	return d, bin
}

func partsSettings(d *Daemon, edit func(*protocol.Settings)) {
	s := config.Defaults()
	if edit != nil {
		edit(&s)
	}
	d.mu.Lock()
	d.settings = s
	d.mu.Unlock()
}

func partOf(t *testing.T, components []protocol.Component, id string) protocol.Component {
	t.Helper()
	for _, c := range components {
		if c.ID == id {
			return c
		}
	}
	t.Fatalf("no component %s in %+v", id, components)
	return protocol.Component{}
}

func partsList(t *testing.T, d *Daemon) []protocol.Component {
	t.Helper()
	return d.componentsList(context.Background()).Components
}

func partRemove(d *Daemon, id string) (protocol.ComponentsResult, error) {
	return d.componentRemove(context.Background(), protocol.ComponentRemoveParams{ID: id})
}

func partCode(t *testing.T, err error, code string) {
	t.Helper()
	var refused *protocol.Error
	if !errors.As(err, &refused) || refused.Code != code {
		t.Fatalf("err = %v, want code %s", err, code)
	}
}

func partWrite(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

// partModel lays a file of m's pinned size in dir, without its bytes.
func partModel(t *testing.T, dir string, m stt.Model) string {
	t.Helper()
	path := filepath.Join(dir, m.File)
	partWrite(t, path, "")
	if err := os.Truncate(path, m.Size); err != nil {
		t.Fatal(err)
	}
	return path
}

func partProgram(t *testing.T, bin, name string) string {
	t.Helper()
	path := filepath.Join(bin, name)
	if err := os.WriteFile(path, []byte("#!/bin/sh\nexit 1\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	return path
}

func partExists(path string) bool {
	_, err := os.Lstat(path)
	return err == nil
}

func partTree(t *testing.T, root string) []string {
	t.Helper()
	var out []string
	if err := filepath.WalkDir(root, func(path string, _ fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		out = append(out, path)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return out
}

// A fresh install lists every part absent, nothing in use and the decision
// server not looked for, and a listing writes nothing.
func TestPartsFreshInstallListsEverythingAbsent(t *testing.T) {
	d, _ := partsDaemon(t, nil)
	before := partTree(t, paths.Home())
	components := partsList(t, d)
	if after := partTree(t, paths.Home()); !slices.Equal(before, after) {
		t.Fatalf("a listing wrote to disk: before %q, after %q", before, after)
	}
	if len(components) == 0 {
		t.Fatal("nothing listed")
	}
	seen := map[string]bool{}
	for _, c := range components {
		if seen[c.ID] || !protocol.ValidComponentID(c.ID) || !strings.HasPrefix(c.ID, c.Kind+":") || c.Title == "" || c.Uses == "" {
			t.Fatalf("component %+v: duplicate, bad id or unnamed", c)
		}
		seen[c.ID] = true
		if c.InUse != "" {
			t.Fatalf("%s in use on a fresh install: %q", c.ID, c.InUse)
		}
		switch {
		case c.ID == protocol.ComponentProgram+":decision-server":
			if c.State != protocol.ComponentUnknown || c.Removable {
				t.Fatalf("decision server %+v", c)
			}
		case c.Kind == protocol.ComponentModel && !c.Removable && strings.HasPrefix(c.Path, "/usr/share/ryoku/"):
			// Ryoku's system dictation models, where this computer has them
		case c.State != protocol.ComponentAbsent || c.Bytes != 0:
			t.Fatalf("%s is not absent on a fresh install: %+v", c.ID, c)
		}
		if c.Kind == protocol.ComponentProgram && (c.Removable || c.Why == "" || c.Feature == "") {
			t.Fatalf("program %+v: removable or without its reason", c)
		}
	}
	for _, id := range []string{"model:" + stt.DefaultModel, "model:" + stt.FallbackVAD, "pack:kokoro", "profile:hands-browser", "data:hands-desktop",
		"data:walkthroughs", "data:hints", "data:reminders", "data:routines", "data:hands-journal", "data:dictation", "cache:voice",
		"program:tesseract", "program:espeak-ng", "program:" + insert.Tool, "program:" + sel.Tool, "program:grim", "program:weston", "program:niri", "program:browser"} {
		if !seen[id] {
			t.Errorf("%s is not listed", id)
		}
	}
}

// A downloaded model is listed whole with its size, and removing it deletes
// that file and nothing beside it; a partial download goes the same way.
func TestPartsSpeechModelIsListedAndRemovedExactly(t *testing.T) {
	d, _ := partsDaemon(t, nil)
	m, _ := stt.FindModel(stt.FallbackModel)
	own := partModel(t, stt.ModelDir(), m)
	beside := filepath.Join(stt.ModelDir(), "notes.txt")
	partWrite(t, beside, "mine")
	turbo, _ := stt.FindModel("whisper-large-v3-turbo-q5_0")
	partial := filepath.Join(stt.ModelDir(), turbo.File+".part")
	partWrite(t, partial, "0123456789")

	components := partsList(t, d)
	c := partOf(t, components, "model:"+m.ID)
	if c.State != protocol.ComponentPresent || c.Bytes != m.Size || !c.Removable || c.Path != own || c.Feature == "" {
		t.Fatalf("downloaded model %+v", c)
	}
	if c := partOf(t, components, "model:"+turbo.ID); c.State != protocol.ComponentPartial || c.Bytes != 10 || c.Path != partial {
		t.Fatalf("partial model %+v", c)
	}
	res, err := partRemove(d, "model:"+m.ID)
	if err != nil {
		t.Fatal(err)
	}
	if partExists(own) || !partExists(beside) || !partExists(partial) {
		t.Fatalf("removal took the wrong files: model %v, beside %v, partial %v", partExists(own), partExists(beside), partExists(partial))
	}
	if c := partOf(t, res.Components, "model:"+m.ID); c.State != protocol.ComponentAbsent || c.Bytes != 0 {
		t.Fatalf("after removal %+v", c)
	}
	if _, err := partRemove(d, "model:"+turbo.ID); err != nil || partExists(partial) {
		t.Fatalf("partial download not removed: %v", err)
	}
}

// A Parakeet model Ryoku's dictation keeps is used where it is, listed as
// kept, and never removed.
func TestPartsRyokuDictationModelIsKept(t *testing.T) {
	d, _ := partsDaemon(t, nil)
	m, _ := stt.FindModel(stt.DefaultModel)
	ryoku := partModel(t, filepath.Join(paths.DataHome(), "ryoku", "dictation", "models"), m)
	c := partOf(t, partsList(t, d), "model:"+m.ID)
	if c.State != protocol.ComponentPresent || c.Removable || c.Path != ryoku || c.Why == "" {
		t.Fatalf("Ryoku's model %+v", c)
	}
	_, err := partRemove(d, "model:"+m.ID)
	partCode(t, err, protocol.CodeConflict)
	if !partExists(ryoku) {
		t.Fatal("Ryoku's model was deleted")
	}
}

// The model Oni listens with, and its speech detection model, are refused
// while voice is on and removed once it is off.
func TestPartsSpeechModelRemovalRefusedWhileVoiceIsOn(t *testing.T) {
	voice := func(on bool) func(*protocol.Settings) {
		return func(s *protocol.Settings) {
			s.Enabled, s.Oni.Master, s.Oni.Inspect, s.Oni.Voice = true, true, true, on
			s.Oni.VoiceSettings.STT.Model = stt.FallbackModel
		}
	}
	d, _ := partsDaemon(t, voice(true))
	m, _ := stt.FindModel(stt.FallbackModel)
	vad, _ := stt.FindModel(stt.FallbackVAD)
	own, ownVAD := partModel(t, stt.ModelDir(), m), partModel(t, stt.ModelDir(), vad)
	components := partsList(t, d)
	for _, id := range []string{m.ID, vad.ID} {
		if c := partOf(t, components, "model:"+id); c.InUse == "" {
			t.Fatalf("%s not in use while voice is on: %+v", id, c)
		}
		_, err := partRemove(d, "model:"+id)
		partCode(t, err, protocol.CodeConflict)
	}
	if !partExists(own) || !partExists(ownVAD) {
		t.Fatal("a model in use was deleted")
	}
	partsSettings(d, voice(false))
	for _, id := range []string{m.ID, vad.ID} {
		if _, err := partRemove(d, "model:"+id); err != nil {
			t.Fatal(err)
		}
	}
	if partExists(own) || partExists(ownVAD) {
		t.Fatal("models not removed once voice was off")
	}
}

// The Hands browser's profile is refused while it is open, the separate
// desktop's leftovers while a task runs, and a store while its feature is
// on; each goes once nothing uses it.
func TestPartsRemovalRefusedWhileInUse(t *testing.T) {
	reminders := func(on bool) func(*protocol.Settings) {
		return func(s *protocol.Settings) {
			s.Enabled, s.Oni.Master, s.Oni.Reflex, s.Oni.ReflexSettings.Reminders = true, true, on, on
		}
	}
	d, _ := partsDaemon(t, reminders(true))
	profileFile := filepath.Join(handsProfile(), "Default", "Cookies")
	partWrite(t, profileFile, "cookies")
	desktopFile := filepath.Join(handsNestHome(), ".config", "app.ini")
	partWrite(t, desktopFile, "app")
	store := remind.NewStore("")
	partWrite(t, store.Path(), "[]")
	partWrite(t, store.DamagedPath(), "garbled")
	d.oni.hands.open.Store(true)
	d.oni.hands.active.Store(true)

	components := partsList(t, d)
	for id, path := range map[string]string{"profile:hands-browser": profileFile, "data:hands-desktop": desktopFile, "data:reminders": store.Path()} {
		c := partOf(t, components, id)
		if c.State != protocol.ComponentPresent || c.Bytes == 0 || c.InUse == "" {
			t.Fatalf("%s in use: %+v", id, c)
		}
		_, err := partRemove(d, id)
		partCode(t, err, protocol.CodeConflict)
		if !partExists(path) {
			t.Fatalf("%s was deleted while in use", id)
		}
	}

	d.oni.hands.open.Store(false)
	d.oni.hands.active.Store(false)
	partsSettings(d, reminders(false))
	for _, id := range []string{"profile:hands-browser", "data:hands-desktop", "data:reminders"} {
		res, err := partRemove(d, id)
		if err != nil {
			t.Fatalf("%s: %v", id, err)
		}
		if c := partOf(t, res.Components, id); c.State != protocol.ComponentAbsent {
			t.Fatalf("%s after removal: %+v", id, c)
		}
	}
	for _, path := range []string{handsProfile(), handsNestHome(), store.Path(), store.DamagedPath()} {
		if partExists(path) {
			t.Fatalf("%s is still there", path)
		}
	}
	if !partExists(paths.StateDir()) {
		t.Fatal("the state folder itself was removed")
	}
}

// Removing the Kokoro pack deletes its folder and nothing a link in it leads
// to, a pack folder that is itself a link goes as a link, and Oni still has
// the system voice.
func TestPartsKokoroRemovalStaysInsideAndKeepsTheSystemVoice(t *testing.T) {
	d, bin := partsDaemon(t, nil)
	partProgram(t, bin, "espeak-ng")
	outside := t.TempDir()
	precious := filepath.Join(outside, "precious.txt")
	partWrite(t, precious, "not Hyakki's")
	dir := tts.KokoroDir()
	for _, name := range []string{"model.onnx", "voices.bin", "tokens.txt", filepath.Join("espeak-ng-data", "phontab")} {
		partWrite(t, filepath.Join(dir, name), "x")
	}
	if err := os.Symlink(precious, filepath.Join(dir, "lexicon-us-en.txt")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(dir, "espeak-ng-data", "elsewhere")); err != nil {
		t.Fatal(err)
	}
	c := partOf(t, partsList(t, d), "pack:kokoro")
	if c.State != protocol.ComponentPresent || !c.Removable || c.Path != dir || c.InUse != "" {
		t.Fatalf("Kokoro pack %+v", c)
	}
	if _, err := partRemove(d, "pack:kokoro"); err != nil {
		t.Fatal(err)
	}
	if partExists(dir) || !partExists(precious) {
		t.Fatalf("pack there %v, outside file there %v", partExists(dir), partExists(precious))
	}
	var names []string
	for _, e := range buildVoices(protocol.OniTTSSettings{Engine: protocol.OniTTSKokoro}, profile.Default().Profile) {
		names = append(names, e.Name())
	}
	if !slices.Contains(names, tts.System) || slices.Contains(names, tts.Kokoro) {
		t.Fatalf("voices after removing Kokoro = %q", names)
	}

	if err := os.Symlink(outside, dir); err != nil {
		t.Fatal(err)
	}
	if _, err := partRemove(d, "pack:kokoro"); err != nil {
		t.Fatal(err)
	}
	if partExists(dir) || !partExists(precious) {
		t.Fatalf("linked pack there %v, its target's file there %v", partExists(dir), partExists(precious))
	}
}

// A store file that is a link goes as a link; the file it leads to stays.
func TestPartsStoreLinkIsRemovedAsALink(t *testing.T) {
	d, _ := partsDaemon(t, nil)
	outside := filepath.Join(t.TempDir(), "routines.json")
	partWrite(t, outside, "[]")
	store := routine.NewStore("")
	if err := os.MkdirAll(filepath.Dir(store.Path()), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, store.Path()); err != nil {
		t.Fatal(err)
	}
	if _, err := partRemove(d, "data:routines"); err != nil {
		t.Fatal(err)
	}
	if partExists(store.Path()) || !partExists(outside) {
		t.Fatalf("link there %v, target there %v", partExists(store.Path()), partExists(outside))
	}
}

// Distribution programs stay listed but cannot be removed. Unknown and
// malformed component names are refused.
func TestPartsProgramsAreListedNeverRemoved(t *testing.T) {
	d, bin := partsDaemon(t, nil)
	tesseract := partProgram(t, bin, "tesseract")
	c := partOf(t, partsList(t, d), "program:tesseract")
	if c.State != protocol.ComponentPresent || c.Path != tesseract || c.Removable {
		t.Fatalf("tesseract %+v", c)
	}
	_, err := partRemove(d, "program:tesseract")
	partCode(t, err, protocol.CodeConflict)
	if !partExists(tesseract) {
		t.Fatal("a program was deleted")
	}
	for _, id := range []string{"model:no-such-model", "files:x"} {
		_, err = partRemove(d, id)
		partCode(t, err, protocol.CodeNotFound)
	}
	for _, id := range []string{"", "model", "model:../x", "model:X", "Model:x"} {
		_, err = partRemove(d, id)
		partCode(t, err, protocol.CodeBadRequest)
	}
}
