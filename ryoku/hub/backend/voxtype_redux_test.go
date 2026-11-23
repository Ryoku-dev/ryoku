package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReduxInstalledPresetAndSwitching(t *testing.T) {
	root := t.TempDir()
	systemPath := os.Getenv("PATH")
	t.Setenv("HOME", root)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(root, "config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(root, "data"))
	bin := filepath.Join(root, "bin")
	os.MkdirAll(bin, 0700)
	log := filepath.Join(root, "calls")
	host := "#!/bin/sh\nif [ \"$1\" = init ]; then echo systemd; exit 0; fi\nprintf '%s\\n' \"$*\" >> '" + log + "'\n"
	os.WriteFile(filepath.Join(bin, "ryoku-host"), []byte(host), 0700)
	t.Setenv("PATH", bin+":"+systemPath)
	if reduxInstalled() {
		t.Fatal("Redux installed without files")
	}
	for _, path := range []string{reduxModelPath(), reduxAdapterPath(), filepath.Join(configHome(), "systemd/user/voxtype-redux.service")} {
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("fixture"), 0700); err != nil {
			t.Fatal(err)
		}
	}
	p, ok := presetByKey(reduxPresetKey)
	if !ok || p.Label != "Redux Multilingual + Adapter" || p.Group != "multilingual" || p.Language != "25 languages" {
		t.Fatalf("invalid Redux preset: %#v", p)
	}
	config := buildVoxtypeConfig(p, "")
	for _, expected := range []string{"mode = \"cli\"", "language = \"auto\"", "max_duration_secs = 120", reduxAdapterPath(), reduxModelPath()} {
		if !strings.Contains(config, expected) {
			t.Fatalf("config missing %s", expected)
		}
	}
	if err := voxtypeRmModel(reduxPresetKey); err == nil {
		t.Fatal("allowed deleting shared Redux model")
	}
	if reduxInstalled() {
		t.Fatal("Redux counted as installed without the Hub's receipt")
	}
	if err := configureReduxService(true, true); err != nil {
		t.Fatal(err)
	}
	current := filepath.Join(configHome(), "systemd/user/voxtype.service.d/ryoku-redux.conf")
	if !strings.Contains(readFileString(current), "Requires=voxtype-redux.service") {
		t.Fatal("missing worker dependency")
	}
	if !strings.Contains(readFileString(log), "enable --now voxtype-redux") {
		t.Fatal("worker not started")
	}
	if err := configureReduxService(false, true); err != nil {
		t.Fatal(err)
	}
	if fileExists(current) {
		t.Fatal("Redux override still forces worker after switching models")
	}
}

func TestSelectingExistingReduxFilesAdoptsMissingReceipt(t *testing.T) {
	root := t.TempDir()
	systemPath := os.Getenv("PATH")
	t.Setenv("HOME", root)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(root, "config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(root, "data"))
	bin := filepath.Join(root, "bin")
	if err := os.MkdirAll(bin, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bin, "ryoku-host"), []byte("#!/bin/sh\nif [ \"$1\" = init ]; then echo systemd; fi\nexit 0\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	actions := filepath.Join(root, "setup-actions")
	python := "#!/bin/sh\nprintf '%s\\n' \"$2\" >> '" + actions + "'\n" +
		"if [ \"$2\" = adopt ]; then mkdir -p \"$XDG_DATA_HOME/voxtype-redux\"; " +
		"touch \"$XDG_DATA_HOME/voxtype-redux/hub-install.json\"; fi\n"
	if err := os.WriteFile(filepath.Join(bin, "python3"), []byte(python), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+":"+systemPath)
	for _, path := range []string{reduxModelPath(), reduxAdapterPath(), filepath.Join(configHome(), "systemd/user/voxtype-redux.service")} {
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("fixture"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if reduxInstalled() || !reduxFilesPresent() {
		t.Fatal("fixture should have adoptable Redux files without an ownership receipt")
	}

	if err := voxtypeSet(`{"preset":"parakeet-redux","enabled":false}`); err != nil {
		t.Fatal(err)
	}
	if got := readFileString(actions); got != "adopt\n" {
		t.Fatalf("Redux adoption was not run before selection: %q", got)
	}
	if !reduxInstalled() || selectedPreset(readFileString(voxtypeConfigPath())) != reduxPresetKey {
		t.Fatal("selection did not adopt and configure the existing Redux installation")
	}
}
