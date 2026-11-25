package media

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func audioEngines(t *testing.T, dictate string) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("PATH", dir)
	t.Setenv("TMPDIR", dir)
	for name, script := range map[string]string{
		"ffmpeg":        "#!/bin/sh\nprintf '%s\\n' \"$@\" > \"$MEDIA_TEST_ARGS\"\nfor arg do last=$arg; done\nprintf wav > \"$last\"\n",
		"ryoku-dictate": "#!/bin/sh\n" + dictate,
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(script), 0700); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("MEDIA_TEST_ARGS", filepath.Join(dir, "args"))
	return dir
}

func assertAudioTempsRemoved(t *testing.T, dir string) {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(dir, "rashin-audio-*"))
	if err != nil || len(files) != 0 {
		t.Fatalf("temporary recordings remain: %v, %v", files, err)
	}
}

func TestAudioTranscribesConvertedPCMAndCleansUp(t *testing.T) {
	dir := audioEngines(t, "[ \"$1\" = --selftest ] && [ -f \"$2\" ] || exit 2\nprintf 'The meeting starts at noon.'\n")
	var steps []string
	text, err := Audio(context.Background(), filepath.Join(dir, "voice note.ogg"), func(step string) { steps = append(steps, step) })
	if err != nil || text != "The meeting starts at noon." || len(steps) != 2 {
		t.Fatalf("Audio = %q, %v, steps %v", text, err, steps)
	}
	args, err := os.ReadFile(filepath.Join(dir, "args"))
	if err != nil {
		t.Fatal(err)
	}
	for _, sequence := range []string{"-ar\n16000\n", "-ac\n1\n", "-c:a\npcm_s16le\n", "-f\nwav\n", "voice note.ogg\n"} {
		if !strings.Contains(string(args), sequence) {
			t.Errorf("conversion lacks %q: %s", sequence, args)
		}
	}
	assertAudioTempsRemoved(t, dir)
}

func TestAudioNamesVoiceModelDownloadWhenMissing(t *testing.T) {
	dir := audioEngines(t, "printf 'no voice model' >&2\nexit 1\n")
	_, err := Audio(context.Background(), filepath.Join(dir, "voice.wav"), func(string) {})
	if err == nil || !strings.Contains(err.Error(), "Settings > Voice typing") || !strings.Contains(strings.ToLower(err.Error()), "model") {
		t.Fatalf("missing model error = %v", err)
	}
	assertAudioTempsRemoved(t, dir)
}

func TestCancelledAudioCleansUp(t *testing.T) {
	dir := audioEngines(t, "printf transcript\n")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := Audio(ctx, filepath.Join(dir, "voice.wav"), func(string) {})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled Audio = %v", err)
	}
	assertAudioTempsRemoved(t, dir)
}
