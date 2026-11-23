package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstalledPhononIsLocalAndSelectable(t *testing.T) {
	root := t.TempDir()
	t.Setenv("HOME", root)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(root, "config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(root, "data"))
	bin := filepath.Join(root, "bin")
	if err := os.MkdirAll(bin, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bin, "ryoku-host"), []byte("#!/bin/sh\nif [ \"$1\" = init ]; then echo systemd; fi\n"), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	if _, ok := presetByKey(phononPresetKey); !ok || phononInstalled() {
		t.Fatal("Phonon must remain available to install without claiming it is installed")
	}
	for _, path := range []string{filepath.Join(dataHome(), "phonon-2/venv/bin/phonon"), phononModelPath(), phononUnitPath(), phononReceiptPath()} {
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("fixture"), 0700); err != nil {
			t.Fatal(err)
		}
	}
	p, ok := presetByKey(phononPresetKey)
	if !ok || p.Cloud || p.KeyKind != "" || p.Language != "English" {
		t.Fatalf("invalid installed Phonon preset: %#v", p)
	}
	presets := voxtypePresets()
	for i, preset := range presets {
		if preset.Key == phononPresetKey && (i == 0 || presets[i-1].Key != "whisper-fast") {
			t.Fatal("Phonon must appear directly after Base English")
		}
	}
	config := buildVoxtypeConfig(p, "cloud-secret")
	for _, want := range []string{"mode = \"remote\"", "\nmodel = \"phonon-2\"\n", "remote_endpoint = \"http://127.0.0.1:8010\"", "remote_model = \"phonon-2\"", "language = \"en\"", "max_duration_secs = 120"} {
		if !strings.Contains(config, want) {
			t.Fatalf("missing %s in config", want)
		}
	}
	if selectedPreset(config) != phononPresetKey || strings.Contains(config, "cloud-secret") {
		t.Fatal("selection did not round trip or cloud credentials leaked to local engine")
	}
	if err := configurePhononService(true); err != nil {
		t.Fatal(err)
	}
	dropin := filepath.Join(configHome(), "systemd/user/voxtype.service.d/ryoku-phonon.conf")
	if !strings.Contains(readFileString(dropin), "Requires=phonon-2.service") {
		t.Fatal("Phonon selection did not establish its service dependency")
	}
	if err := configurePhononService(false); err != nil {
		t.Fatal(err)
	}
	if fileExists(dropin) {
		t.Fatal("switching away retained the Phonon dependency")
	}
	if err := voxtypeSet(`{"preset":"phonon-2","enabled":false,"openaiKey":"cloud-secret"}`); err != nil {
		t.Fatal(err)
	}
	if storedVoxtypeOpenAIKey() != "cloud-secret" || strings.Contains(readFileString(voxtypeConfigPath()), "cloud-secret") {
		t.Fatal("cloud key was lost or included in localhost configuration")
	}
	if err := voxtypeSet(`{"preset":"openai","enabled":false}`); err != nil {
		t.Fatal(err)
	}
	if extractConfigString(readFileString(voxtypeConfigPath()), "remote_api_key") != "cloud-secret" {
		t.Fatal("switching back to OpenAI lost its saved key")
	}
	previous := readFileString(voxtypeConfigPath())
	failingHost := "#!/bin/sh\nif [ \"$1\" = init ]; then echo systemd; fi\nif [ \"$*\" = 'svc --user enable --now phonon-2' ]; then exit 1; fi\n"
	if err := os.WriteFile(filepath.Join(bin, "ryoku-host"), []byte(failingHost), 0700); err != nil {
		t.Fatal(err)
	}
	if err := voxtypeSet(`{"preset":"phonon-2","enabled":true}`); err == nil {
		t.Fatal("accepted a failed Phonon server start")
	}
	if readFileString(voxtypeConfigPath()) != previous || fileExists(dropin) {
		t.Fatal("a failed Phonon start changed the current preset or its dependency")
	}
}
