package media

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestVideoUsesPreferredSubtitlesWithoutVoiceEngine(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PATH", dir)
	t.Setenv("TMPDIR", dir)
	script := "#!/bin/sh\nwhile [ \"$#\" -gt 0 ]; do\nif [ \"$1\" = -o ]; then shift; out=$1; fi\nshift\ndone\nprintf 'WEBVTT\\n\\n00:00:01.000 --> 00:00:04.000\\nEnglish fallback\\n' > \"${out%.*}.en.vtt\"\nprintf 'WEBVTT\\n\\n00:00:02.000 --> 00:00:05.000\\nPreferred captions\\n' > \"${out%.*}.fr-CA.vtt\"\n"
	if err := os.WriteFile(filepath.Join(dir, "yt-dlp"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	var steps []string
	text, err := Video(context.Background(), "https://93.184.216.34/example", "fr_CA.UTF-8", func(step string) { steps = append(steps, step) })
	if err != nil || text != "[00:02] Preferred captions" || len(steps) != 1 {
		t.Fatalf("Video = %q, %v, steps %v", text, err, steps)
	}
	files, err := filepath.Glob(filepath.Join(dir, "rashin-video-*"))
	if err != nil || len(files) != 0 {
		t.Fatalf("subtitle temporary files remain: %v, %v", files, err)
	}
}

func TestVideoWithoutCaptionsNamesMissingVoiceEngine(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PATH", dir)
	t.Setenv("TMPDIR", dir)
	if err := os.WriteFile(filepath.Join(dir, "yt-dlp"), []byte("#!/bin/sh\nexit 0\n"), 0700); err != nil {
		t.Fatal(err)
	}
	_, err := Video(context.Background(), "https://93.184.216.34/123", "en", func(string) {})
	if err == nil || !strings.Contains(err.Error(), "Settings > Voice typing") {
		t.Fatalf("missing fallback engine error = %v", err)
	}
	files, globErr := filepath.Glob(filepath.Join(dir, "rashin-video-*"))
	if globErr != nil || len(files) != 0 {
		t.Fatalf("video temporary files remain: %v, %v", files, globErr)
	}
}
