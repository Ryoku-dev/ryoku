package attachment

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAudioBytes(t *testing.T) {
	for _, test := range []struct {
		name string
		data string
		want bool
	}{
		{"wav", "RIFF\x24\x00\x00\x00WAVEfmt ", true},
		{"mp3 tag", "ID3\x04\x00\x00\x00\x00\x00\x00", true},
		{"mp3 frame", "\xff\xfb\x90\x64", true},
		{"reserved MPEG", "\xff\xeb\xf0\x00", false},
		{"m4a", "\x00\x00\x00\x18ftypM4A \x00\x00\x00\x00isom", true},
		{"ogg vorbis", "OggS\x00\x02\x01vorbis", true},
		{"opus", "OggS\x00\x02OpusHead", true},
		{"flac", "fLaC\x00\x00", true},
		{"webm audio", "\x1a\x45\xdf\xa3webmA_OPUS", true},
		{"webm video", "\x1a\x45\xdf\xa3webmV_VP9A_OPUS", false},
		{"ogg video", "OggS\x00\x02theora", false},
		{"picture", "RIFF\x24\x00\x00\x00WEBPVP8 ", false},
		{"text", "A recording is attached", false},
		{"short", "ID", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := audioBytes([]byte(test.data)); got != test.want {
				t.Fatalf("audioBytes(%q) = %v, want %v", test.data, got, test.want)
			}
		})
	}
}

func TestAudioSniffIgnoresFilename(t *testing.T) {
	dir := t.TempDir()
	for _, test := range []struct {
		name string
		data string
		want bool
	}{
		{"recording.txt", "fLaC\x00\x00\x00\x00", true},
		{"not-a-recording.mp3", "ordinary text", false},
	} {
		path := filepath.Join(dir, test.name)
		if err := os.WriteFile(path, []byte(test.data), 0600); err != nil {
			t.Fatal(err)
		}
		got, err := IsAudio(path)
		if err != nil || got != test.want {
			t.Fatalf("IsAudio(%q) = %v, %v", path, got, err)
		}
	}
	if got, err := IsAudio(dir); got || err != nil {
		t.Fatalf("directory = %v, %v", got, err)
	}
	if _, err := IsAudio(filepath.Join(dir, "missing")); err == nil {
		t.Fatal("missing file was not reported")
	}
}
