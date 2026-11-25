package stt

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"ryoku-rashin/internal/localmodels"
	"ryoku-rashin/internal/paths"
)

// ModelDir is where Hyakki keeps the speech models it downloads.
func ModelDir() string { return filepath.Join(paths.DataHome(), "hyakki", "oni", "voice", "models") }

// ryokuModelDirs are where Ryoku's dictation keeps its models: the system
// set first, then the user's downloads.
func ryokuModelDirs() []string {
	return []string{"/usr/share/ryoku/dictation/models", filepath.Join(paths.DataHome(), "ryoku", "dictation", "models")}
}

// Installed reports whether m's file is in dir at its pinned size. It does
// not hash it; the adapters verify the SHA-256 before every load.
func Installed(dir string, m Model) bool {
	info, err := os.Stat(filepath.Join(dir, m.File))
	return err == nil && info.Mode().IsRegular() && info.Size() == m.Size
}

// Locate returns the path of m's file: in ModelDir, or for a Ryoku
// dictation model also where Ryoku keeps it.
func Locate(m Model) (string, bool) {
	dirs := []string{ModelDir()}
	if m.Engine == EngineDictate {
		dirs = append(dirs, ryokuModelDirs()...)
	}
	for _, dir := range dirs {
		if Installed(dir, m) {
			return filepath.Join(dir, m.File), true
		}
	}
	return "", false
}

// Model is one speech model file Hyakki knows by its pinned size and
// SHA-256. Engine is the adapter that decodes it.
type Model struct {
	ID, Engine, File, URL, License string
	Size                           int64
	SHA256                         string
}

const (
	EngineWhisper = "whispercpp"
	EngineDictate = "transcribecpp"
	EngineVAD     = "whispercpp-vad"
)

// Catalogue is every speech model Hyakki may download. The Parakeet pins
// are the files Ryoku's dictation catalogue pins.
var Catalogue = []Model{
	{ID: "whisper-base.en-q5_1", Engine: EngineWhisper, File: "ggml-base.en-q5_1.bin", License: "MIT",
		URL:  "https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-base.en-q5_1.bin",
		Size: 59721011, SHA256: "4baf70dd0d7c4247ba2b81fafd9c01005ac77c2f9ef064e00dcf195d0e2fdd2f"},
	{ID: "whisper-large-v3-turbo-q5_0", Engine: EngineWhisper, File: "ggml-large-v3-turbo-q5_0.bin", License: "MIT",
		URL:  "https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-large-v3-turbo-q5_0.bin",
		Size: 574041195, SHA256: "394221709cd5ad1f40c46e6031ca61bce88931e6e088c188294c6d5a55ffa7e2"},
	{ID: "silero-v6.2.0", Engine: EngineVAD, File: "ggml-silero-v6.2.0.bin", License: "MIT",
		URL:  "https://huggingface.co/ggml-org/whisper-vad/resolve/main/ggml-silero-v6.2.0.bin",
		Size: 885098, SHA256: "2aa269b785eeb53a82983a20501ddf7c1d9c48e33ab63a41391ac6c9f7fb6987"},
	{ID: "parakeet-tdt_ctc-110m-q8_0", Engine: EngineDictate, File: "parakeet-tdt_ctc-110m-Q8_0.gguf", License: "CC-BY-4.0",
		URL:  "https://huggingface.co/handy-computer/parakeet-tdt_ctc-110m-gguf/resolve/main/parakeet-tdt_ctc-110m-Q8_0.gguf",
		Size: 135373280, SHA256: "7dd44c74a331d788a4e5f8b16913b3feb29ced22cf5613aad0e0f6cd30516296"},
	{ID: "parakeet-tdt-0.6b-v2-q8_0", Engine: EngineDictate, File: "parakeet-tdt-0.6b-v2-Q8_0.gguf", License: "CC-BY-4.0",
		URL:  "https://huggingface.co/handy-computer/parakeet-tdt-0.6b-v2-gguf/resolve/main/parakeet-tdt-0.6b-v2-Q8_0.gguf",
		Size: 729574912, SHA256: "f0d0e99cebb6d3b83f1f7069b82b5d3c2e39a54545b0da039cb4bafd9c4e5caa"},
}

// DefaultModel, FallbackModel and FallbackVAD are Catalogue IDs chosen by
// the bench (release/lab/oni/voicebench.sh, 2026-11-22, CPU, 50
// LibriSpeech test-clean utterances plus 6 espeak-ng clips): Parakeet TDT
// 0.6B v2 through ryoku-dictate had the lowest word error rate (1.06%)
// within both budgets (p90 first partial 596 ms, final 961 ms). Whisper
// base.en q5_1 with whisper.cpp's Silero VAD is the fallback for when
// ryoku-dictate is absent (6.46%; p90 104 ms and 305 ms, resident, 186
// MiB). Large-v3-turbo missed both budgets on CPU and is not offered as a
// default.
const (
	DefaultModel  = "parakeet-tdt-0.6b-v2-q8_0"
	FallbackModel = "whisper-base.en-q5_1"
	FallbackVAD   = "silero-v6.2.0"
)

// FindModel returns the catalogue entry with id.
func FindModel(id string) (Model, bool) {
	for _, m := range Catalogue {
		if m.ID == id {
			return m, true
		}
	}
	return Model{}, false
}

// Download fetches m into dir through the local-model pinned fetch: it
// resumes a partial file and renames it into place only when the bytes
// hash to the pin. It refuses unless consented is true, which only the
// user's explicit agreement may set; nothing in Hyakki downloads a speech
// model on its own. It returns the model's path.
func Download(ctx context.Context, m Model, dir string, consented bool, progress func(phase string, done int64)) (string, error) {
	if !consented {
		return "", ErrNoConsent
	}
	if !filepath.IsAbs(dir) || m.File == "" || filepath.Base(m.File) != m.File {
		return "", fmt.Errorf("%w: %q in %q", ErrModel, m.File, dir)
	}
	dest := filepath.Join(dir, m.File)
	if _, err := localmodels.FetchPinned(ctx, m.URL, dest, m.Size, m.SHA256, progress); err != nil {
		return "", err
	}
	return dest, nil
}
