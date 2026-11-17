package harness

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"ryoku-rashin/internal/protocol"
)

func fakeBinary(t *testing.T, dir, name string) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestClaudePrefersTheCurrentAdapter(t *testing.T) {
	bin := t.TempDir()
	t.Setenv("PATH", bin)
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	fakeBinary(t, bin, "claude-code-acp")
	claude := func() candidate {
		for _, c := range candidates(protocol.Settings{}) {
			if c.id == "claude" {
				return c
			}
		}
		t.Fatal("no claude harness")
		return candidate{}
	}
	if got := claude(); !got.installed || filepath.Base(got.path) != "claude-code-acp" {
		t.Fatalf("only the deprecated adapter installed: %+v", got)
	}
	fakeBinary(t, bin, "claude-agent-acp")
	if got := claude(); filepath.Base(got.path) != "claude-agent-acp" {
		t.Fatalf("both installed, want the current adapter: %+v", got)
	}
}

// adapterTree is a global npm prefix holding claude-agent-acp, with the SDK's
// native Claude beside it only when withNative, and a claude on PATH.
func adapterTree(t *testing.T, withNative bool) (adapter, bin string) {
	t.Helper()
	root := t.TempDir()
	script := fakeBinary(t, filepath.Join(root, "lib", "node_modules", "@agentclientprotocol", "claude-agent-acp", "dist"), "index.js")
	bin = filepath.Join(root, "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	adapter = filepath.Join(bin, "claude-agent-acp")
	if err := os.Symlink(script, adapter); err != nil {
		t.Fatal(err)
	}
	if withNative {
		arch := runtime.GOARCH
		if arch == "amd64" {
			arch = "x64"
		}
		fakeBinary(t, filepath.Join(root, "lib", "node_modules", "@anthropic-ai", "claude-agent-sdk-linux-"+arch), "claude")
	}
	fakeBinary(t, bin, "claude")
	return adapter, bin
}

func TestAdapterWithoutNativeClaudeUsesTheOneOnPath(t *testing.T) {
	adapter, bin := adapterTree(t, false)
	t.Setenv("PATH", bin)
	t.Setenv(claudeExecutableVariable, "")
	env := withClaudeExecutable(map[string]string{"KEEP": "1"}, []string{adapter})
	if env[claudeExecutableVariable] != filepath.Join(bin, "claude") || env["KEEP"] != "1" {
		t.Fatalf("env = %v", env)
	}
}

func TestWorkingAdapterAndUsersChoiceAreLeftAlone(t *testing.T) {
	adapter, bin := adapterTree(t, true)
	t.Setenv("PATH", bin)
	t.Setenv(claudeExecutableVariable, "")
	if env := withClaudeExecutable(nil, []string{adapter}); len(env) != 0 {
		t.Fatalf("adapter ships its own Claude, env = %v", env)
	}
	broken, bin := adapterTree(t, false)
	t.Setenv("PATH", bin)
	t.Setenv(claudeExecutableVariable, "/opt/claude")
	if env := withClaudeExecutable(nil, []string{broken}); len(env) != 0 {
		t.Fatalf("CLAUDE_CODE_EXECUTABLE from the user's environment was overridden: %v", env)
	}
	t.Setenv(claudeExecutableVariable, "")
	own := map[string]string{claudeExecutableVariable: "/opt/mine"}
	if env := withClaudeExecutable(own, []string{broken}); env[claudeExecutableVariable] != "/opt/mine" {
		t.Fatalf("an instance's own CLAUDE_CODE_EXECUTABLE was overridden: %v", env)
	}
}

func TestDeprecatedAdapterAndOtherHarnessesGetNoExecutable(t *testing.T) {
	adapter, bin := adapterTree(t, false)
	t.Setenv("PATH", bin)
	t.Setenv(claudeExecutableVariable, "")
	old := fakeBinary(t, bin, "claude-code-acp")
	for _, command := range [][]string{{old}, {filepath.Join(bin, "codex-acp")}, nil} {
		if env := withClaudeExecutable(nil, command); len(env) != 0 {
			t.Fatalf("%v got env %v", command, env)
		}
	}
	t.Setenv("PATH", filepath.Dir(adapter))
	_ = os.Remove(filepath.Join(bin, "claude"))
	if env := withClaudeExecutable(nil, []string{adapter}); len(env) != 0 {
		t.Fatalf("no claude on PATH, env = %v", env)
	}
}
