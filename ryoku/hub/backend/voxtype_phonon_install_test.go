package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRemovingSelectedPhononStopsDictationBeforeDeleting(t *testing.T) {
	root := t.TempDir()
	t.Setenv("HOME", root)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(root, "config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(root, "data"))
	bin := filepath.Join(root, "bin")
	t.Setenv("PATH", bin)
	write := func(path, contents string, mode os.FileMode) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(contents), mode); err != nil {
			t.Fatal(err)
		}
	}
	hostLog := filepath.Join(root, "host-calls")
	setupLog := filepath.Join(root, "setup-calls")
	write(filepath.Join(bin, "ryoku-host"), "#!/bin/sh\nif [ \"$1\" = init ]; then echo systemd; exit 0; fi\nprintf '%s\\n' \"$*\" >> '"+hostLog+"'\n", 0700)
	write(filepath.Join(bin, "python3"), "#!/bin/sh\nprintf '%s\\n' \"$2\" >> '"+setupLog+"'\n", 0700)
	for _, path := range []string{filepath.Join(dataHome(), "phonon-2/venv/bin/phonon"), phononModelPath(), phononUnitPath(), phononReceiptPath()} {
		write(path, "fixture", 0600)
	}
	p, _ := presetByKey(phononPresetKey)
	write(voxtypeConfigPath(), buildVoxtypeConfig(p, ""), 0600)
	if err := configurePhononService(true); err != nil {
		t.Fatal(err)
	}
	if err := removePhonon(); err != nil {
		t.Fatal(err)
	}
	if selectedPreset(readFileString(voxtypeConfigPath())) != "whisper-fast" {
		t.Fatal("removal left the selected model pointing at deleted Phonon files")
	}
	if got := readFileString(setupLog); got != "check-remove\nremove\n" {
		t.Fatalf("ownership validation must precede removal: %q", got)
	}
	for _, call := range []string{"svc --user restart voxtype", "svc --user disable --now phonon-2"} {
		if !strings.Contains(readFileString(hostLog), call) {
			t.Fatalf("missing service transition before removal: %s", call)
		}
	}
	if strings.Contains(readFileString(hostLog), "svc --user disable --now voxtype") {
		t.Fatal("removal disabled dictation that was enabled before the fallback")
	}
	if fileExists(filepath.Join(configHome(), "systemd/user/voxtype.service.d/ryoku-phonon.conf")) {
		t.Fatal("removal retained a dependency on the deleted Phonon service")
	}
}
