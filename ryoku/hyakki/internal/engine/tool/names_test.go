package tool

import "testing"

// A light model loses track of its tools past eight schemas (contract §1.6).
func TestCompactProfilesStayWithinEightTools(t *testing.T) {
	for _, mode := range Modes {
		names := Names(mode, ClassCompact)
		if len(names) == 0 || len(names) > MaxCompactTools {
			t.Errorf("%s compact offers %d tools, want 1-%d", mode, len(names), MaxCompactTools)
		}
	}
}

// A repeated name would send the model two schemas for one tool.
func TestEveryToolSetNamesEachToolOnce(t *testing.T) {
	for _, mode := range Modes {
		for _, class := range []Class{ClassCompact, ClassFull} {
			seen := map[string]bool{}
			for _, name := range Names(mode, class) {
				if seen[name] {
					t.Errorf("%s %s names %s twice", mode, class, name)
				}
				seen[name] = true
			}
		}
	}
}

func TestHostMCPProvidersNeverJoinTeamModes(t *testing.T) {
	for _, mode := range []Mode{ModeTeam, ModeDreamManager, ModeSubagent} {
		if ProvidersJoin(mode, ClassFull) {
			t.Errorf("%s may start host MCP servers", mode)
		}
	}
	for _, mode := range []Mode{ModeAgent, ModePlan} {
		if !ProvidersJoin(mode, ClassFull) {
			t.Errorf("%s lost its user MCP tools", mode)
		}
	}
}

// The Ask prompt and its tests name these tools; renaming them breaks Ask.
func TestAskKeepsItsToolNames(t *testing.T) {
	for _, name := range []string{SystemQuery, ReadFile, ListDir, SearchSystem, SearchCode, ReadWiki, FetchURL, Doctor, Propose} {
		if !Offers(ModeAskRead, ClassFull, name) || !Offers(ModeAskAct, ClassFull, name) {
			t.Errorf("Ask lost %s", name)
		}
	}
	for _, name := range []string{RunCommand, WriteFile} {
		if Offers(ModeAskRead, ClassFull, name) || !Offers(ModeAskAct, ClassFull, name) {
			t.Errorf("%s must be offered in Ask first and YOLO only", name)
		}
	}
}

func TestLightModelsGetTheCompactClass(t *testing.T) {
	cases := []struct {
		params       int64
		light, local bool
		want         Class
	}{
		{2_000_000_000, false, true, ClassCompact},
		{0, false, true, ClassCompact},
		{0, true, false, ClassCompact},
		{8_000_000_000, false, true, ClassFull},
		{0, false, false, ClassFull},
	}
	for _, c := range cases {
		if got := ClassFor(c.params, c.light, c.local); got != c.want {
			t.Errorf("ClassFor(%d, %v, %v) = %s, want %s", c.params, c.light, c.local, got, c.want)
		}
	}
}

// Output caps follow the window so a 16K local model is not flooded and a
// large remote one is not starved.
func TestToolOutputCapsScaleWithTheWindow(t *testing.T) {
	small, large := LimitsFor(16384, ClassCompact), LimitsFor(200000, ClassFull)
	if small.Told >= large.Told || small.ReadLines > large.ReadLines {
		t.Fatalf("caps do not grow with the window: %+v vs %+v", small, large)
	}
	if small.Told < 4<<10 || large.Told > 50<<10 {
		t.Fatalf("Told outside 4-50 KiB: %d, %d", small.Told, large.Told)
	}
}
