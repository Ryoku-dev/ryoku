package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
	openaiPreset, _ := presetByKey("openai")
	if err := os.WriteFile(voxtypeConfigPath(), []byte(buildVoxtypeConfig(openaiPreset, "")), 0600); err != nil {
		t.Fatal(err)
	}
	if err := voxtypeSet(`{"preset":"whisper-fast","enabled":false}`); err != nil {
		t.Fatal(err)
	}
	if fileExists(voxtypeOpenAIKeyPath()) || storedVoxtypeOpenAIKey() != "" {
		t.Fatal("a key removed from the active OpenAI config was restored from the stale private copy")
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

func TestRunitPhononIsSelectableAndUsesPackagedUserService(t *testing.T) {
	root := t.TempDir()
	t.Setenv("HOME", root)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(root, "config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(root, "data"))
	bin := filepath.Join(root, "bin")
	if err := os.MkdirAll(bin, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bin, "ryoku-host"), []byte("#!/bin/sh\nif [ \"$1\" = init ]; then echo runit; exit 0; fi\nexit 0\n"), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	serviceRun := filepath.Join(phononRunitServicePath(), "run")
	for _, path := range []string{filepath.Join(dataHome(), "phonon-2/venv/bin/phonon"), phononModelPath(), serviceRun, phononReceiptPath()} {
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("fixture"), 0700); err != nil {
			t.Fatal(err)
		}
	}
	if !phononInstalled() {
		t.Fatal("Phonon should be installed when its runtime, model and runit service are present")
	}
	presets := voxtypePresets()
	for i, preset := range presets {
		if preset.Key == phononPresetKey && (i == 0 || presets[i-1].Key != "whisper-fast") {
			t.Fatal("Phonon must appear directly after Base English on runit")
		}
	}
	if err := voxtypeSet(`{"preset":"phonon-2","enabled":false}`); err != nil {
		t.Fatal(err)
	}
	if selectedPreset(readFileString(voxtypeConfigPath())) != phononPresetKey || fileExists(phononUnitPath()) {
		t.Fatal("runit selection did not write the local Phonon config without a systemd unit")
	}
}

func TestFailedPhononSelectionRestoresPreviouslyStoppedService(t *testing.T) {
	root := t.TempDir()
	t.Setenv("HOME", root)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(root, "config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(root, "data"))
	bin := filepath.Join(root, "bin")
	state := filepath.Join(root, "phonon-state")
	if err := os.MkdirAll(bin, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(state, 0700); err != nil {
		t.Fatal(err)
	}
	host := "#!/bin/sh\n" +
		"if [ \"$1\" = init ]; then echo systemd; exit 0; fi\n" +
		"case \"$*\" in\n" +
		"  'svc --user is-enabled phonon-2') test -e '" + state + "/enabled' ;;\n" +
		"  'svc --user is-active phonon-2') test -e '" + state + "/active' ;;\n" +
		"  'svc --user enable --now phonon-2') touch '" + state + "/enabled' '" + state + "/active' ;;\n" +
		"  'svc --user stop phonon-2') rm -f '" + state + "/active' ;;\n" +
		"  'svc --user disable phonon-2') rm -f '" + state + "/enabled' ;;\n" +
		"  *) exit 0 ;;\n" +
		"esac\n"
	if err := os.WriteFile(filepath.Join(bin, "ryoku-host"), []byte(host), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	for _, path := range []string{filepath.Join(dataHome(), "phonon-2/venv/bin/phonon"), phononModelPath(), phononUnitPath(), phononReceiptPath()} {
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("fixture"), 0700); err != nil {
			t.Fatal(err)
		}
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok", "model": phononPresetKey})
	}))
	defer server.Close()
	previousEndpoint := phononEndpoint
	phononEndpoint = server.URL
	t.Cleanup(func() { phononEndpoint = previousEndpoint })
	configDir := filepath.Join(configHome(), "voxtype")
	if err := os.MkdirAll(filepath.Dir(configDir), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configDir, []byte("blocks the config directory"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := voxtypeSet(`{"preset":"phonon-2","enabled":true}`); err == nil {
		t.Fatal("accepted a config write through a file that blocks its parent directory")
	}
	if fileExists(filepath.Join(state, "enabled")) || fileExists(filepath.Join(state, "active")) {
		t.Fatal("failed Phonon selection left its newly started service enabled or active")
	}
	if err := os.WriteFile(filepath.Join(state, "active"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	if err := voxtypeSet(`{"preset":"phonon-2","enabled":true}`); err == nil {
		t.Fatal("accepted a config write while restoring a previously active Phonon server")
	}
	if !fileExists(filepath.Join(state, "active")) || fileExists(filepath.Join(state, "enabled")) {
		t.Fatal("failed selection changed the prior active-but-disabled Phonon service state")
	}
}

func TestStoredOpenAIKeyUsesActiveVoxtypeConfig(t *testing.T) {
	root := t.TempDir()
	t.Setenv("HOME", root)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(root, "config"))
	configPath := voxtypeConfigPath()
	if err := os.MkdirAll(filepath.Dir(configPath), 0700); err != nil {
		t.Fatal(err)
	}
	preset, ok := presetByKey("openai")
	if !ok {
		t.Fatal("OpenAI preset unavailable")
	}
	if err := os.WriteFile(configPath, []byte(buildVoxtypeConfig(preset, "edited-key")), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(voxtypeOpenAIKeyPath(), []byte("stale-key"), 0600); err != nil {
		t.Fatal(err)
	}
	if got := storedVoxtypeOpenAIKey(); got != "edited-key" {
		t.Fatalf("saved key = %q, want active config value", got)
	}
}

func TestStoredOpenAIKeyHonorsClearedActiveConfig(t *testing.T) {
	root := t.TempDir()
	t.Setenv("HOME", root)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(root, "config"))
	configPath := voxtypeConfigPath()
	if err := os.MkdirAll(filepath.Dir(configPath), 0700); err != nil {
		t.Fatal(err)
	}
	preset, ok := presetByKey("openai")
	if !ok {
		t.Fatal("OpenAI preset unavailable")
	}
	if err := os.WriteFile(configPath, []byte(buildVoxtypeConfig(preset, "")), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(voxtypeOpenAIKeyPath(), []byte("stale-key"), 0600); err != nil {
		t.Fatal(err)
	}
	if got := storedVoxtypeOpenAIKey(); got != "" {
		t.Fatalf("saved key = %q after it was cleared from active config", got)
	}
}
