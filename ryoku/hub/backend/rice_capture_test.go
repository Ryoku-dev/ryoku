package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRiceCaptureCLIProtectsSavedRice(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	p := filepath.Join(ricesDir(), "my-setup", "rice.json")
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	original := []byte(`{"name":"My Setup","slug":"my-setup","look":{"shell":{"barStyle":"qsbar"}}}`)
	if err := os.WriteFile(p, original, 0o644); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"My Setup", "my-setup", "my setup"} {
		if err := runRice([]string{"capture", name}); err == nil || !strings.Contains(err.Error(), "--force") {
			t.Fatalf("capture %q did not explain replacement: %v", name, err)
		}
		got, err := os.ReadFile(p)
		if err != nil || string(got) != string(original) {
			t.Fatalf("capture %q changed the saved rice: %s, %v", name, got, err)
		}
	}
}

func TestRiceCaptureCLIForceReplacesSavedRice(t *testing.T) {
	for _, args := range [][]string{
		{"capture", "--force", "My Setup", "input"},
		{"capture", "My Setup", "all", "--force"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			t.Setenv("XDG_CONFIG_HOME", t.TempDir())
			p := filepath.Join(ricesDir(), "my-setup", "rice.json")
			if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(p, []byte("old saved rice"), 0o644); err != nil {
				t.Fatal(err)
			}
			writeDesktopStore(t, `{"input":{"sensitivity":0.2}}`)
			var err error
			out := captureStdout(t, func() { err = runRice(args) })
			if err != nil {
				t.Fatal(err)
			}
			var result Rice
			if err := json.Unmarshal([]byte(out), &result); err != nil {
				t.Fatal(err)
			}
			if result.Slug != "my-setup" || result.Layers["input"] == nil {
				t.Fatalf("capture lost the name or requested layer: %+v", result)
			}
			saved, err := loadRiceFrom(filepath.Dir(p))
			if err != nil || saved.Slug != result.Slug {
				t.Fatalf("forced capture was not saved: %+v, %v", saved, err)
			}
		})
	}
}

func TestRiceCaptureCLIHelpAndInvalidNamesDoNotWrite(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	for _, flag := range []string{"--help", "-h"} {
		var err error
		out := captureStdout(t, func() { err = runRice([]string{"capture", flag}) })
		if err != nil || strings.TrimSpace(out) == "" {
			t.Fatalf("help %q: output %q, error %v", flag, out, err)
		}
	}
	for _, args := range [][]string{
		{"capture"}, {"capture", "--force"}, {"capture", "-my-rice"},
		{"capture", " -my-rice"}, {"capture", "--unknown"}, {"capture", ""},
	} {
		if err := runRice(args); err == nil {
			t.Fatalf("invalid capture %q succeeded", args)
		}
	}
	if _, err := os.Stat(ricesDir()); !os.IsNotExist(err) {
		t.Fatalf("help or invalid input created a rice directory: %v", err)
	}
}

func TestRiceCaptureCLICreatesNewRice(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	var err error
	captureStdout(t, func() { err = runRice([]string{"capture", "New Setup"}) })
	if err != nil {
		t.Fatal(err)
	}
	if _, err := loadRiceFrom(filepath.Join(ricesDir(), "new-setup")); err != nil {
		t.Fatal(err)
	}
}
