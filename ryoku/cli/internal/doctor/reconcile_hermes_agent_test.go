package doctor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func hermesAgentTestEnv(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("PATH", filepath.Join(t.TempDir(), "bin"))
	return home
}

func TestInstalledHermesAgentVersionReadsInstalledBinary(t *testing.T) {
	home := hermesAgentTestEnv(t)
	if err := os.MkdirAll(filepath.Join(home, ".hermes", "hermes-agent"), 0o755); err != nil {
		t.Fatal(err)
	}
	binDir := filepath.Join(home, ".hermes", "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(binDir, "hermes"), []byte("#!/bin/sh\necho 'Hermes Agent v0.21.3 (2026.9.1)'\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	version, installed := installedHermesAgentVersion()
	if !installed || version != "0.21.3" {
		t.Fatalf("installedHermesAgentVersion = %q, %v", version, installed)
	}

	result := reconcileHermesAgent(true)
	if result.status != recNote || !strings.Contains(result.detail, "0.21.3") {
		t.Fatalf("reconcileHermesAgent = %+v", result)
	}
	if !strings.Contains(result.remedy, "own installer or checkout") {
		t.Fatalf("Hermes note has no owner-safe update advice: %+v", result)
	}
}

func TestInstalledHermesAgentVersionUsesPathBinary(t *testing.T) {
	hermesAgentTestEnv(t)
	binDir := t.TempDir()
	t.Setenv("PATH", binDir)
	bin := filepath.Join(binDir, "hermes")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\necho 'Hermes Agent v0.22.5 (2026.10.1)'\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	version, installed := installedHermesAgentVersion()
	if !installed || version != "0.22.5" {
		t.Fatalf("installedHermesAgentVersion = %q, %v", version, installed)
	}
}

func TestHermesAgentInstallWithoutBinaryReportsUnknownVersion(t *testing.T) {
	home := hermesAgentTestEnv(t)
	if err := os.MkdirAll(filepath.Join(home, ".hermes", "hermes-agent"), 0o755); err != nil {
		t.Fatal(err)
	}
	result := reconcileHermesAgent(true)
	if result.status != recNote || !strings.Contains(result.detail, "could not be read") {
		t.Fatalf("reconcileHermesAgent = %+v", result)
	}
}

func TestHermesAgentFindingIsQuietWhenAbsent(t *testing.T) {
	hermesAgentTestEnv(t)
	result := reconcileHermesAgent(true)
	if result.status != recOK {
		t.Fatalf("reconcileHermesAgent = %+v", result)
	}
}
