package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// runWrapper sources only the function definitions of the SDDM wrapper and
// calls import_session_environment with a fixed systemctl stub. Cutting before
// the first top-level call keeps a compositor from being started.
func runWrapper(t *testing.T, stub string) string {
	t.Helper()
	wrapper := filepath.Join("..", "..", "lockscreen", "sddm", "ryoku-wayland-session")
	raw, err := os.ReadFile(wrapper)
	if err != nil {
		t.Fatal(err)
	}
	definitions := string(raw)
	cut := strings.Index(definitions, "\nimport_session_environment\n")
	if cut < 0 {
		t.Fatal("session wrapper lost its top-level import_session_environment call")
	}
	definitions = definitions[:cut]

	binDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(binDir, "systemctl"), []byte(stub), 0o755); err != nil {
		t.Fatal(err)
	}

	// The sourced text is this repo's own wrapper plus a fixed stub, so no
	// untrusted input reaches the shell.
	script := definitions + "\nimport_session_environment\n" +
		"printf 'WLR_RENDERER=%s\\n' \"${WLR_RENDERER-<unset>}\"\n"
	cmd := exec.Command("bash", "-c", script)
	// Drop any renderer the developer exported: only the stub may supply it,
	// otherwise an inherited value would hide an allowlist regression.
	base := os.Environ()
	env := make([]string, 0, len(base)+1)
	for _, entry := range base {
		if strings.HasPrefix(entry, "WLR_RENDERER=") {
			continue
		}
		env = append(env, entry)
	}
	cmd.Env = append(env, "PATH="+binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("sourcing the session wrapper failed: %v\n%s", err, out)
	}
	return string(out)
}

// TestSessionWrapperImportsRendererSelection pins the SDDM wrapper's
// environment allowlist. Wayfire 0.11 on wlroots 0.20 only honours the HDR
// toggle with the Vulkan renderer, so a WLR_RENDERER override placed in
// ~/.config/environment.d must survive the wrapper and reach the compositor.
// Without it the compositor falls back to GLES2 and silently ignores HDR.
func TestSessionWrapperImportsRendererSelection(t *testing.T) {
	out := runWrapper(t, "#!/bin/sh\nprintf '%s\\n' 'WLR_RENDERER=vulkan' 'XCURSOR_THEME=breeze'\n")
	if !strings.Contains(out, "WLR_RENDERER=vulkan") {
		t.Fatalf("renderer selection did not reach the compositor; got:\n%s", out)
	}
}

// A developer's exported renderer must not satisfy the test: only the stub's
// value may reach the sourced wrapper, or the allowlist could regress unseen.
func TestSessionWrapperDoesNotInheritRenderer(t *testing.T) {
	t.Setenv("WLR_RENDERER", "parent-leak")
	out := runWrapper(t, "#!/bin/sh\nprintf '%s\\n' 'XCURSOR_THEME=breeze'\n")
	if !strings.Contains(out, "WLR_RENDERER=<unset>") {
		t.Fatalf("inherited renderer masked the wrapper's allowlist; got:\n%s", out)
	}
}
