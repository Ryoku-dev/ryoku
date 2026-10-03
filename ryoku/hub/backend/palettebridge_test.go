package main

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func makePaletteBridgeSource(t *testing.T, names ...string) string {
	t.Helper()
	dir := t.TempDir()
	for _, name := range names {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("#!/bin/sh\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestPaletteBridgeSourceRequiresRecipe(t *testing.T) {
	dir := makePaletteBridgeSource(t, "install.sh")
	if _, err := paletteBridgeSource(dir, "install.sh"); err != nil {
		t.Fatalf("valid recipe: %v", err)
	}
	if _, err := paletteBridgeSource(dir, "remove-integrations.sh"); err == nil {
		t.Fatal("source without the requested recipe was accepted")
	}
}

func TestPaletteBridgeInstallUsesTypedCommand(t *testing.T) {
	source := makePaletteBridgeSource(t, "install.sh")
	original := paletteBridgeRun
	t.Cleanup(func() { paletteBridgeRun = original })
	var got []string
	paletteBridgeRun = func(name string, args ...string) ([]byte, error) {
		got = append([]string{name}, args...)
		return nil, nil
	}
	if err := runPaletteBridge([]string{"install", source}); err != nil {
		t.Fatal(err)
	}
	want := []string{filepath.Join(source, "install.sh")}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("command = %q, want %q", got, want)
	}
}

func TestPaletteBridgeRejectsUnknownIntegration(t *testing.T) {
	if err := runPaletteBridge([]string{"integration", "install", "browser", t.TempDir()}); err == nil {
		t.Fatal("unknown integration was accepted")
	}
}

func TestPaletteBridgeCommandReturnsOutput(t *testing.T) {
	original := paletteBridgeRun
	t.Cleanup(func() { paletteBridgeRun = original })
	paletteBridgeRun = func(string, ...string) ([]byte, error) {
		return []byte("specific failure\n"), errors.New("exit status 1")
	}
	if got := paletteBridgeCommand("helper"); got == nil || got.Error() != "specific failure" {
		t.Fatalf("error = %v", got)
	}
}

func TestPaletteBridgeVesktopStatusUsesPaletteTemplate(t *testing.T) {
	config := t.TempDir()
	state := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", config)
	t.Setenv("XDG_STATE_HOME", state)
	original := paletteBridgeRun
	t.Cleanup(func() { paletteBridgeRun = original })
	paletteBridgeRun = func(string, ...string) ([]byte, error) { return nil, nil }
	source := makePaletteBridgeSource(t, "install.sh", "install-integrations.sh")
	template := filepath.Join(config, "ryoku", "user_edits", "matugen", "templates", "vesktop-colors.css")
	midnight := filepath.Join(config, "vesktop", "themes", "midnight-ryoku.theme.css")
	ledger := filepath.Join(state, "ryoku", "palette-bridge", "owned-files.tsv")
	write := func(path, content string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	check := func(installed, managed bool) {
		t.Helper()
		for _, integration := range paletteBridgeStatusFor(source).Integrations {
			if integration.ID == "vesktop" {
				if integration.Installed != installed || integration.Managed != managed {
					t.Fatalf("Vesktop status = %+v, want installed=%v managed=%v", integration, installed, managed)
				}
				return
			}
		}
		t.Fatal("missing Vesktop integration")
	}
	write(midnight, "/* old base theme */")
	write(ledger, "vesktop\t"+midnight+"\n")
	check(false, false)
	if err := os.Remove(midnight); err != nil {
		t.Fatal(err)
	}
	write(template, "/* live palette */")
	check(true, false)
	write(ledger, "vesktop\t"+template+"\n")
	check(true, true)
	if err := os.Remove(template); err != nil {
		t.Fatal(err)
	}
	check(false, true)
}
