package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// writeTransitionConfig drops a ryogami-wall/config.json under XDG_CONFIG_HOME
// with the given transition.shader, the value the picker writes when a single
// transition is pinned.
func writeTransitionConfig(t *testing.T, shader string) {
	t.Helper()
	dir := filepath.Join(os.Getenv("XDG_CONFIG_HOME"), "ryogami-wall")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	doc := map[string]any{
		"transition": map[string]any{"enabled": true, "shader": shader},
	}
	b, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config.json"), b, 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestTransitionForMergedPool pins the contract that the picker's one transition
// list spans both engines: a skwd shader name resolves to the shader path and a
// reveal-preset name resolves to the reveal-mask path, both through the same
// transition.shader key. Before the pools were merged, a reveal-preset name in
// transition.shader fell through to the random picker, so the 22 presets were
// unreachable from the picker.
func TestTransitionForMergedPool(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(t.TempDir(), "config"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(t.TempDir(), "state"))

	// A pinned skwd shader names itself on the Shader field and carries no mask kind.
	writeTransitionConfig(t, "morph")
	d := &daemon{lastTransition: -1}
	got := d.transitionFor("switch")
	if got == nil || got.Shader != "morph" {
		t.Fatalf("pinned skwd shader: got %+v, want Shader=morph", got)
	}

	// A pinned reveal preset resolves to its mask kind, not a shader.
	writeTransitionConfig(t, "silk_fade")
	got = d.transitionFor("switch")
	if got == nil || got.Shader != "" || got.Name != "silk_fade" || got.Kind != "fade" {
		t.Fatalf("pinned reveal preset: got %+v, want Name=silk_fade Kind=fade no Shader", got)
	}

	// An unknown name is not silently treated as a shader; it falls back to a
	// reveal pick from the preset table.
	writeTransitionConfig(t, "not-a-transition")
	got = d.transitionFor("switch")
	if got == nil || got.Shader != "" {
		t.Fatalf("unknown name: got %+v, want a reveal pick (no Shader)", got)
	}
}

// TestTransitionForRandomCoversBothEngines checks that "random" can surface a
// reveal preset as well as a skwd shader, i.e. the random pick spans the merged
// pool rather than only the shader half.
func TestTransitionForRandomCoversBothEngines(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(t.TempDir(), "config"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(t.TempDir(), "state"))
	writeTransitionConfig(t, transitionRandom)

	d := &daemon{lastTransition: -1}
	sawShader, sawReveal := false, false
	for range 400 {
		got := d.transitionFor("switch")
		if got == nil {
			t.Fatal("random pick returned nil")
		}
		if got.Shader != "" {
			sawShader = true
		} else {
			sawReveal = true
		}
		if sawShader && sawReveal {
			return
		}
	}
	t.Fatalf("random over 400 picks never covered both engines (shader=%v reveal=%v)", sawShader, sawReveal)
}
