package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRiceCarriesThePromptBothWays(t *testing.T) {
	config := setupStarshipTest(t)
	bin := t.TempDir()
	if err := os.WriteFile(filepath.Join(bin, "ryogami"), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	origRun, origReload := riceRun, riceReload
	riceRun = func(string, ...string) error { return nil }
	riceReload = func() {}
	t.Cleanup(func() { riceRun, riceReload = origRun, origReload })
	if err := os.WriteFile(filepath.Join(config, "layouts", "plain.toml"), []byte(starshipTestLayout), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := applyStarship(starshipState{Layout: "pill", Palette: "fixed"}, true); err != nil {
		t.Fatal(err)
	}
	var err error
	captureStdout(t, func() { err = runRice([]string{"capture", "Prompt"}) })
	if err != nil {
		t.Fatal(err)
	}
	r, err := loadRiceFrom(filepath.Join(ricesDir(), "prompt"))
	if err != nil {
		t.Fatal(err)
	}
	if got := r.Look["starship"]["layout"]; got != "pill" {
		t.Fatalf("captured prompt layout = %v, want pill", got)
	}

	if err := atomicWrite(starshipStatePath(), []byte(`{"layout":"plain","palette":"fixed"}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := applyRice("prompt", nil); err != nil {
		t.Fatal(err)
	}
	if got := readStarshipState(); got != (starshipState{Layout: "pill", Palette: "fixed"}) {
		t.Fatalf("prompt after apply = %+v, want pill/fixed", got)
	}
}
