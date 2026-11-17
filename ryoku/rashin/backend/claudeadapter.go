package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// claudeAdapters are the ACP adapters for Claude Code, official name first.
// @zed-industries/claude-code-acp is the deprecated name of the same project.
var claudeAdapters = []string{"claude-agent-acp", "claude-code-acp"}

func claudeAdapterArgv() string {
	for _, name := range claudeAdapters {
		if _, err := exec.LookPath(name); err == nil {
			return name
		}
	}
	return claudeAdapters[0]
}

// claudeAdapterEnv points a current adapter at the user's own Claude Code when
// the adapter's bundled native one never installed (npm skips optional
// dependencies under omit=optional or on an unsupported libc). The deprecated
// adapter carries its own CLI, so it is left alone.
func claudeAdapterEnv(argv0 string) []string {
	if filepath.Base(argv0) != "claude-agent-acp" || os.Getenv("CLAUDE_CODE_EXECUTABLE") != "" {
		return nil
	}
	adapter, err := exec.LookPath(argv0)
	if err != nil {
		return nil
	}
	if real, err := filepath.EvalSymlinks(adapter); err == nil {
		adapter = real
	}
	if adapterHasNativeClaude(adapter) {
		return nil
	}
	claude, err := exec.LookPath("claude")
	if err != nil {
		return nil
	}
	return []string{"CLAUDE_CODE_EXECUTABLE=" + claude}
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
	var cands []string
	for _, suffix := range []string{"", "-musl"} {
		cands = append(cands, filepath.Join("@anthropic-ai", "claude-agent-sdk-linux-"+arch+suffix, "claude"))
	}
	for d := dir; ; d = filepath.Dir(d) {
		for _, c := range cands {
			roots := []string{filepath.Join(d, "node_modules")}
			if filepath.Base(d) == "node_modules" {
				roots = append(roots, d)
			}
			for _, r := range roots {
				if st, err := os.Stat(filepath.Join(r, c)); err == nil && !st.IsDir() {
					return true
				}
			}
		}
		if filepath.Dir(d) == d {
			return false
		}
	}
}
