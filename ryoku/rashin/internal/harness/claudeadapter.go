package harness

import (
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

const claudeExecutableVariable = "CLAUDE_CODE_EXECUTABLE"

// withClaudeExecutable adds the Claude Code on PATH to a claude-agent-acp
// command's environment when the adapter has no native CLI of its own (npm
// skips optional dependencies under omit=optional or on an unsupported libc)
// and the user set none. The deprecated adapter bundles its CLI.
func withClaudeExecutable(env map[string]string, command []string) map[string]string {
	if len(command) == 0 || filepath.Base(command[0]) != "claude-agent-acp" {
		return env
	}
	if _, set := env[claudeExecutableVariable]; set || os.Getenv(claudeExecutableVariable) != "" {
		return env
	}
	adapter := command[0]
	if real, err := filepath.EvalSymlinks(adapter); err == nil {
		adapter = real
	}
	if adapterHasNativeClaude(adapter) {
		return env
	}
	claude, err := exec.LookPath("claude")
	if err != nil {
		return env
	}
	out := make(map[string]string, len(env)+1)
	maps.Copy(out, env)
	out[claudeExecutableVariable] = claude
	return out
}

// adapterHasNativeClaude follows node's lookup for the SDK's platform package
// (any node_modules above the adapter), plus the claude binary a standalone
// adapter release ships beside itself.
func adapterHasNativeClaude(adapter string) bool {
	dir := filepath.Dir(adapter)
	if st, err := os.Stat(filepath.Join(dir, "claude")); err == nil && !st.IsDir() {
		return true
	}
	arch := runtime.GOARCH
	if arch == "amd64" {
		arch = "x64"
	}
	var candidates []string
	for _, suffix := range []string{"", "-musl"} {
		candidates = append(candidates, filepath.Join("@anthropic-ai", "claude-agent-sdk-linux-"+arch+suffix, "claude"))
	}
	for d := dir; ; d = filepath.Dir(d) {
		roots := []string{filepath.Join(d, "node_modules")}
		if filepath.Base(d) == "node_modules" {
			roots = append(roots, d)
		}
		for _, root := range roots {
			for _, candidate := range candidates {
				if st, err := os.Stat(filepath.Join(root, candidate)); err == nil && !st.IsDir() {
					return true
				}
			}
		}
		if filepath.Dir(d) == d {
			return false
		}
	}
}
