package daemon

import (
	"os"
	"path/filepath"
	"testing"

	"ryoku-rashin/internal/oni/spec"
	"ryoku-rashin/internal/oni/voice/stt"
	"ryoku-rashin/internal/protocol"
)

// stageModel puts a file of m's pinned size where stt.Locate looks. The
// adapters hash a model only when the engine starts, which never happens
// here.
func stageModel(t *testing.T, m stt.Model) {
	t.Helper()
	path := filepath.Join(stt.ModelDir(), m.File)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := f.Truncate(m.Size); err != nil {
		t.Fatal(err)
	}
}

// The bench chose whisper base.en with whisper.cpp's Silero VAD as the
// fallback (stt.FallbackModel with stt.FallbackVAD: 6.46% word errors,
// against 12.14% without the VAD). With the VAD model downloaded, the
// fallback engine must run it, whether the settings leave the choice to
// Hyakki or name the fallback; without it the fallback still runs, plainly.
func TestOniVoiceFallbackEngineRunsItsVAD(t *testing.T) {
	sp, err := spec.Load()
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	bin := t.TempDir()
	if err := os.WriteFile(filepath.Join(bin, "whisper-server"), []byte("#!/bin/sh\nexit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	// no ryoku-dictate on PATH: Hyakki's choice falls to the fallback
	t.Setenv("PATH", bin)
	fallback, _ := stt.FindModel(stt.FallbackModel)
	vad, _ := stt.FindModel(stt.FallbackVAD)
	stageModel(t, fallback)
	settings := []protocol.OniSTTSettings{{}, {Engine: protocol.OniSTTWhisper}, {Engine: protocol.OniSTTWhisper, Model: stt.FallbackModel}}

	build := func(s protocol.OniSTTSettings) stt.Info {
		t.Helper()
		engine, err := buildSTT(sp, s, "Oni")
		if err != nil {
			t.Fatalf("buildSTT(%+v): %v", s, err)
		}
		defer engine.Close()
		return engine.Info()
	}
	for _, s := range settings {
		if info := build(s); info.Model != fallback.File || info.VAD != "" {
			t.Fatalf("settings %+v, no VAD downloaded: engine %+v, want %s and no VAD", s, info, fallback.File)
		}
	}
	stageModel(t, vad)
	for _, s := range settings {
		if info := build(s); info.Model != fallback.File || info.VAD != vad.File {
			t.Fatalf("settings %+v, VAD downloaded: engine %+v, want %s with VAD %s", s, info, fallback.File, vad.File)
		}
	}
}
