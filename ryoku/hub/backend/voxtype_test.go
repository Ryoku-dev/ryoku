package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestVoxtypePresetsAreCuratedWhisperCatalogue(t *testing.T) {
	want := []struct {
		key, model, size, language, speed, group, configLanguage string
		sizeBytes                                                int64
	}{
		{"whisper-tiny-en", "tiny.en", "39 MB", "English", "fastest", "english", "en", 39_000_000},
		{"whisper-fast", "base.en", "142 MB", "English", "fast", "english", "en", 142_000_000},
		{"whisper-small-en", "small.en", "466 MB", "English", "balanced", "english", "en", 466_000_000},
		{"whisper-base", "base", "142 MB", "99+ languages", "fast", "multilingual", "auto", 142_000_000},
		{"whisper-medium", "medium", "1.5 GB", "99+ languages", "quality", "multilingual", "auto", 1_500_000_000},
		{"whisper-accurate", "large-v3-turbo", "1.6 GB", "99+ languages", "fast + accurate", "multilingual", "auto", 1_600_000_000},
		{"openai", "whisper-1", "cloud", "99+ languages", "cloud", "cloud", "en", 0},
	}

	got := voxtypePresets()
	if len(got) != len(want) {
		t.Fatalf("preset count = %d, want %d", len(got), len(want))
	}
	seen := make(map[string]bool, len(got))
	for i, p := range got {
		w := want[i]
		if seen[p.Key] {
			t.Fatalf("duplicate preset key %q", p.Key)
		}
		seen[p.Key] = true
		if p.Key != w.key || p.model != w.model || p.Size != w.size || p.SizeBytes != w.sizeBytes ||
			p.Language != w.language || p.Speed != w.speed || p.Group != w.group || p.lang != w.configLanguage {
			t.Errorf("preset %d = %#v, want key=%q model=%q size=%q bytes=%d language=%q speed=%q group=%q config language=%q",
				i, p, w.key, w.model, w.size, w.sizeBytes, w.language, w.speed, w.group, w.configLanguage)
		}
	}
}

func TestVoxtypeLocalModelsUseWhisperFilenames(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_DATA_HOME", root)
	want := map[string]string{
		"whisper-tiny-en":  "ggml-tiny.en.bin",
		"whisper-fast":     "ggml-base.en.bin",
		"whisper-small-en": "ggml-small.en.bin",
		"whisper-base":     "ggml-base.bin",
		"whisper-medium":   "ggml-medium.bin",
		"whisper-accurate": "ggml-large-v3-turbo.bin",
	}
	for _, p := range voxtypePresets() {
		if p.Cloud {
			continue
		}
		expected := filepath.Join(root, "voxtype", "models", want[p.Key])
		if got := modelFilePath(p); got != expected {
			t.Errorf("%s model path = %q, want %q", p.Key, got, expected)
		}
	}
}

func TestVoxtypeMultilingualConfigsUseLanguageAuto(t *testing.T) {
	for _, key := range []string{"whisper-base", "whisper-medium", "whisper-accurate"} {
		p, ok := presetByKey(key)
		if !ok {
			t.Fatalf("missing preset %q", key)
		}
		want := "language = \"auto\""
		if config := buildVoxtypeConfig(p, ""); !strings.Contains(config, "\n"+want+"\n") {
			t.Errorf("%s config does not contain %q:\n%s", key, want, config)
		}
	}
}

func TestVoxtypeEnsureDoesNothingWithoutVoxtype(t *testing.T) {
	config := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", config)
	t.Setenv("PATH", t.TempDir())

	if err := voxtypeEnsure(); err != nil {
		t.Fatalf("voxtypeEnsure() error = %v", err)
	}
	if _, err := os.Stat(voxtypeConfigPath()); !os.IsNotExist(err) {
		t.Fatalf("config created without voxtype installed: stat error = %v", err)
	}
}
