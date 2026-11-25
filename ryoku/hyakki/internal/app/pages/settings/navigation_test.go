package settings

import (
	"testing"

	"ryoku-rashin/internal/protocol"
)

func TestSettingsSectionsKeepExistingRoutesAndSearch(t *testing.T) {
	p := New()
	for _, route := range []string{"rashin", "features", "providers", "api", "ask", "agents", "teams", "memory", "skills", "index", "oni", "hands", "appearance", "keys", "oni-voice", "oni-screen", "oni-commands", "oni-writing", "oni-privacy", "parts"} {
		p.OpenSection(route)
		if got := sections()[p.current].name; got != route {
			t.Fatalf("OpenSection(%q) opened %q", route, got)
		}
	}
	for alias, want := range map[string]string{"about": "rashin", "agent": "agents", "keyboard": "keys"} {
		p.OpenSection(alias)
		if got := sections()[p.current].name; got != want {
			t.Fatalf("%s opened %s, want %s", alias, got, want)
		}
	}
	for query, want := range map[string]string{"wake phrase": "oni-voice", "screen consent": "oni-screen", "reminders timers": "oni-commands", "rewrite": "oni-writing", "mcp approvals": "oni-privacy", "voice packs": "parts"} {
		found := false
		for _, section := range sections() {
			if section.name == want && section.matches(query) {
				found = true
			}
		}
		if !found {
			t.Fatalf("%q does not find %s", query, want)
		}
	}
}

func TestOniChangesCountIndependentSettings(t *testing.T) {
	stock := protocol.DefaultOniSettings()
	if got := oniChanges(stock, stock); got != 0 {
		t.Fatalf("unchanged settings count %d", got)
	}
	now := stock
	now.Master = !stock.Master
	now.VoiceSettings.OpenMic = !stock.VoiceSettings.OpenMic
	now.Recall.Enabled = !stock.Recall.Enabled
	now.Rewrite.Voice = !stock.Rewrite.Voice
	now.Assignment.Model = "local/small"
	now.Feeds.Words = []string{"a feed"}
	now.MCP.Tools = map[string]bool{protocol.OniMCPTools[0]: true}
	if got := oniChanges(now, stock); got != 7 {
		t.Fatalf("seven independently edited values count %d", got)
	}
	// Turning off a parent does not erase the choices made under it.
	now.Master = stock.Master
	if got := oniChanges(now, stock); got != 6 {
		t.Fatalf("disabled children count %d, want 6", got)
	}
	now = stock
	now.MCP.Tools = map[string]bool{protocol.OniMCPTools[0]: false}
	if got := oniChanges(now, stock); got != 0 {
		t.Fatalf("an explicit off value counts %d changes", got)
	}
}
