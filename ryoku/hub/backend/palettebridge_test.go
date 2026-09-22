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
