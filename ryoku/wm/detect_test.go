package wm

import "testing"

// LeafScriptsDir is the single definition of where a provider's bare-name
// scripts live; deploy.sh and the switch resolve them through it, so a wrong
// or empty path silently strands a compositor's display or workspace tools.
func TestLeafScriptsDir(t *testing.T) {
	if got, want := LeafScriptsDir(ProviderHyprland), "ryoku/hyprland/scripts"; got != want {
		t.Errorf("hyprland = %q, want %q", got, want)
	}
	if got, want := LeafScriptsDir(ProviderNiri), "ryoku/niri/scripts"; got != want {
		t.Errorf("niri = %q, want %q", got, want)
	}
	if got := LeafScriptsDir(""); got != "" {
		t.Errorf("empty name = %q, want empty", got)
	}
}
