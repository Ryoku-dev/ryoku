package main

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func fakeBin(t *testing.T, dir, name string) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestClaudeAdapterPrefersOfficialName(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "bin")
	fakeBin(t, bin, "claude-code-acp")
	t.Setenv("PATH", bin)
	if got := claudeAdapterArgv(); got != "claude-code-acp" {
		t.Fatalf("only the deprecated adapter installed: got %q", got)
	}
	fakeBin(t, bin, "claude-agent-acp")
	if got := claudeAdapterArgv(); got != "claude-agent-acp" {
		t.Fatalf("both installed: got %q, want the official adapter", got)
	}
}

func claudeAdapterTree(t *testing.T, withNative bool) (root, bin string) {
	t.Helper()
	root = t.TempDir()
	pkg := filepath.Join(root, "lib", "node_modules", "@agentclientprotocol", "claude-agent-acp")
	script := fakeBin(t, filepath.Join(pkg, "dist"), "index.js")
	bin = filepath.Join(root, "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(script, filepath.Join(bin, "claude-agent-acp")); err != nil {
		t.Fatal(err)
	}
	if withNative {
		arch := runtime.GOARCH
		if arch == "amd64" {
			arch = "x64"
		}
		fakeBin(t, filepath.Join(root, "lib", "node_modules", "@anthropic-ai", "claude-agent-sdk-linux-"+arch), "claude")
	}
	fakeBin(t, bin, "claude")
	return root, bin
}

func TestClaudeAdapterEnvFallsBackToUsersClaude(t *testing.T) {
	_, bin := claudeAdapterTree(t, false)
	t.Setenv("PATH", bin)
	t.Setenv("CLAUDE_CODE_EXECUTABLE", "")
	env := claudeAdapterEnv("claude-agent-acp")
	if len(env) != 1 || env[0] != "CLAUDE_CODE_EXECUTABLE="+filepath.Join(bin, "claude") {
		t.Fatalf("env = %v", env)
	}
}

func TestClaudeAdapterEnvLeavesWorkingInstallAlone(t *testing.T) {
	_, bin := claudeAdapterTree(t, true)
	t.Setenv("PATH", bin)
	t.Setenv("CLAUDE_CODE_EXECUTABLE", "")
	if env := claudeAdapterEnv("claude-agent-acp"); len(env) != 0 {
		t.Fatalf("adapter ships its own Claude, env = %v", env)
	}
	t.Setenv("CLAUDE_CODE_EXECUTABLE", "/opt/claude")
	if env := claudeAdapterEnv("claude-agent-acp"); len(env) != 0 {
		t.Fatalf("user's own CLAUDE_CODE_EXECUTABLE overridden: %v", env)
	}
}

func TestClaudeAdapterEnvSkipsDeprecatedAdapter(t *testing.T) {
	_, bin := claudeAdapterTree(t, false)
	t.Setenv("PATH", bin)
	t.Setenv("CLAUDE_CODE_EXECUTABLE", "")
	if env := claudeAdapterEnv("claude-code-acp"); len(env) != 0 {
		t.Fatalf("deprecated adapter bundles its CLI, env = %v", env)
	}
}
