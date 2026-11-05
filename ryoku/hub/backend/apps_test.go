package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCandidateAliasesReportInstalledAlternative(t *testing.T) {
	binDir := t.TempDir()
	path := filepath.Join(binDir, "zen-browser")
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir)

	aliases := candidateAliases("zen|zen-browser")
	if len(aliases) != 2 {
		t.Fatalf("aliases = %#v", aliases)
	}
	if aliases[0].Installed {
		t.Fatal("zen unexpectedly reported installed")
	}
	if !aliases[1].Installed {
		t.Fatal("zen-browser was not reported installed")
	}
	cmd, installed := resolveAliases(aliases)
	if !installed || cmd != "zen-browser" {
		t.Fatalf("resolved (%q, %v), want (zen-browser, true)", cmd, installed)
	}
}
