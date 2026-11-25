package shell

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"ryoku-i18n"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/dictate"
	"ryoku-rashin/internal/oni/characters"
	"ryoku-rashin/internal/oni/keys"
	"ryoku-rashin/internal/oni/profile"
	"ryoku-rashin/internal/oni/voice/clips"
	"ryoku-rashin/internal/oni/voice/stt"
	"ryoku-rashin/internal/oni/voice/tts"
	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/protocol"
)

// voiceHost is what the voice settings read and fetch: on this computer, or
// the demo's fixed machine when the demo daemon answers
func (r *runtime) voiceHost(posts *postQueue) env.Voice {
	if r.demoVoice != nil {
		return r.demoVoice.host()
	}
	return env.Voice{
		Survey: func(character string, done func(env.VoiceSurvey)) {
			go func() {
				survey := surveyVoice(character)
				posts.Post(func() { done(survey) })
			}()
		},
		Download: func(id string, progress func(int64), done func(error)) func() {
			return downloadSpeechModel(r.ctx, posts, id, progress, done)
		},
		Dictation: dictate.NewStore(""),
	}
}

// surveyVoice looks once at what is installed: binaries on PATH, the speech
// models' files, the character's clip pack and whether a keyboard opens
func surveyVoice(character string) env.VoiceSurvey {
	survey := env.VoiceSurvey{
		Voices: map[string]string{protocol.OniTTSSystem: "", protocol.OniTTSClips: clipsMissing(character), protocol.OniTTSKokoro: ""},
		Engines: map[string]string{
			protocol.OniSTTWhisper: binaryMissing("whisper-server"),
			protocol.OniSTTDictate: binaryMissing("ryoku-dictate"),
		},
		Models: map[string]bool{},
		Keys:   keys.Probe(keys.System()).Reason.Message(),
	}
	if _, err := tts.NewSystem(tts.SystemOptions{}); err != nil {
		survey.Voices[protocol.OniTTSSystem] = i18n.T("The system voice needs espeak-ng, which is not installed.")
	}
	if _, err := tts.FindKokoro(tts.KokoroOptions{}); err != nil {
		survey.Voices[protocol.OniTTSKokoro] = i18n.Tf("Kokoro needs %s and its voice pack in %s.", tts.KokoroBinary, tts.KokoroDir())
	}
	for _, model := range stt.Catalogue {
		if _, ok := stt.Locate(model); ok {
			survey.Models[model.ID] = true
		}
	}
	return survey
}

func binaryMissing(name string) string {
	if _, err := exec.LookPath(name); err != nil {
		return i18n.Tf("%s is not installed.", name)
	}
	return ""
}

// clipsMissing is why the character cannot speak in recorded clips: it has
// no clip pack, or its pack does not open
func clipsMissing(character string) string {
	var loaded profile.Loaded
	var folder fs.FS
	if character == "" || character == profile.ShippedID {
		loaded, folder = profile.Default(), characters.FS
	} else {
		folder = os.DirFS(paths.CharactersDir())
		loaded, _ = profile.Load(folder, character)
	}
	if !loaded.Valid {
		return i18n.T("The character does not load, so it has no clips.")
	}
	if _, err := clips.ForProfile(folder, loaded.Profile.ID, loaded.Profile.Voice); err != nil {
		return i18n.T("The character has no recorded clip pack.")
	}
	return ""
}

// downloadSpeechModel fetches one catalogue model the user agreed to,
// pinned by its size and hash; progress reaches the frame at most ten times
// a second
func downloadSpeechModel(parent context.Context, posts *postQueue, id string, progress func(int64), done func(error)) func() {
	model, ok := stt.FindModel(id)
	if !ok {
		done(protocol.Errorf(protocol.CodeNotFound, "%s", i18n.Tf("There is no speech model %s.", id)))
		return func() {}
	}
	ctx, cancel := context.WithCancel(parent)
	go func() {
		defer cancel()
		var last time.Time
		_, err := stt.Download(ctx, model, stt.ModelDir(), true, func(_ string, fetched int64) {
			if now := time.Now(); now.Sub(last) >= 100*time.Millisecond || fetched == model.Size {
				last = now
				posts.Post(func() { progress(fetched) })
			}
		})
		if errors.Is(err, context.Canceled) {
			err = errors.New(i18n.T("The download stopped. What it fetched is kept for next time."))
		}
		posts.Post(func() { done(err) })
	}()
	return cancel
}

// demoVoice is the demo's computer: espeak-ng and whisper.cpp with its
// smaller model installed, the rest missing, and downloads that fetch
// nothing from the network
type demoVoice struct {
	mu        sync.Mutex
	models    map[string]bool
	keys      string
	dictation *dictate.Store
}

func newDemoVoice(directory string, keysBlocked bool) *demoVoice {
	v := &demoVoice{models: map[string]bool{stt.DefaultModel: true}, dictation: dictate.NewStore(filepath.Join(directory, "dictation.json"))}
	if keysBlocked {
		v.keys = keys.ReasonPermission.Message()
	}
	return v
}

func (v *demoVoice) host() env.Voice {
	return env.Voice{Survey: v.survey, Download: v.download, Dictation: v.dictation}
}

func (v *demoVoice) survey(_ string, done func(env.VoiceSurvey)) {
	v.mu.Lock()
	models := make(map[string]bool, len(v.models))
	for id, ok := range v.models {
		models[id] = ok
	}
	v.mu.Unlock()
	done(env.VoiceSurvey{
		Voices: map[string]string{
			protocol.OniTTSSystem: "",
			protocol.OniTTSClips:  i18n.T("The character has no recorded clip pack."),
			protocol.OniTTSKokoro: i18n.Tf("Kokoro needs %s and its voice pack in %s.", tts.KokoroBinary, "~/.local/share/hyakki/oni/voice/kokoro"),
		},
		Engines: map[string]string{
			protocol.OniSTTWhisper: "",
			protocol.OniSTTDictate: i18n.Tf("%s is not installed.", "ryoku-dictate"),
		},
		Models: models,
		Keys:   v.keys,
	})
}

// download pretends the fetch at once: half the file, all of it, then the
// model is on the demo's disk
func (v *demoVoice) download(id string, progress func(int64), done func(error)) func() {
	model, ok := stt.FindModel(id)
	if !ok {
		done(protocol.Errorf(protocol.CodeNotFound, "%s", i18n.Tf("There is no speech model %s.", id)))
		return func() {}
	}
	progress(model.Size / 2)
	progress(model.Size)
	v.mu.Lock()
	v.models[id] = true
	v.mu.Unlock()
	done(nil)
	return func() {}
}
