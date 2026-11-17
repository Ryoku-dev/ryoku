package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func seedHermes(t *testing.T, files map[string]string) string {
	t.Helper()
	h := t.TempDir()
	t.Setenv("HOME", h)
	for rel, content := range files {
		p := filepath.Join(h, ".hermes", rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return h
}

func TestSkillsReportOriginsAndLayouts(t *testing.T) {
	seedHermes(t, map[string]string{
		"skills/.bundled_manifest": "arxiv:abc\nobsidian:def\n",
		// categorized layout
		"skills/research/arxiv/SKILL.md": "---\nname: arxiv\ndescription: Search papers\nversion: 1.0.0\n---\nbody",
		// loose layout, agent-grown (not in manifest)
		"skills/my-fix/SKILL.md": "---\nname: my-fix\ndescription: Grown by the agent\n---\nbody",
	})
	rep := SkillsReportNow()
	if rep.Counts["bundled"] != 1 || rep.Counts["agent"] != 1 {
		t.Fatalf("counts %+v", rep.Counts)
	}
	var sawCat, sawLoose bool
	for _, c := range rep.Categories {
		for _, s := range c.Skills {
			if s.Name == "arxiv" && c.Name == "research" && s.Origin == "bundled" && s.Version == "1.0.0" {
				sawCat = true
			}
			if s.Name == "my-fix" && c.Name == "general" && s.Origin == "agent" {
				sawLoose = true
			}
		}
	}
	if !sawCat || !sawLoose {
		t.Fatalf("categories %+v", rep.Categories)
	}
}

func TestToolbeltParsesCLIList(t *testing.T) {
	seedHermes(t, map[string]string{
		"config.yaml": "model: x\nplatform_toolsets:\n  cli:\n    - terminal\n    - file\n    - web\n    - memory\n  telegram:\n    - web\nother: y\n",
	})
	belt := toolbeltNow()
	got := map[string][]string{}
	for _, f := range belt {
		got[f.Family] = f.Tools
	}
	if len(got["System & Code"]) != 2 || len(got["Web"]) != 1 || len(got["Mind"]) != 1 {
		t.Fatalf("toolbelt %+v", belt)
	}
}

func TestMemoryProviderDetection(t *testing.T) {
	seedHermes(t, map[string]string{
		"config.yaml": "memory:\n  memory_enabled: true\n  provider: honcho\n  honcho:\n    workspace: w\n",
		".env":        "OBSIDIAN_VAULT_PATH=/home/u/notes\n",
	})
	kind, vault := memoryProviderKind()
	if kind != "honcho" || vault != "/home/u/notes" {
		t.Fatalf("provider %q vault %q", kind, vault)
	}
}

func TestMemoryProviderBuiltinDefault(t *testing.T) {
	seedHermes(t, map[string]string{
		"config.yaml": "memory:\n  memory_enabled: true\n",
	})
	if kind, _ := memoryProviderKind(); kind != "builtin" {
		t.Fatalf("kind %q", kind)
	}
}

func TestMemoryEntriesSplitAndFenceStrip(t *testing.T) {
	raw := "<!-- ryoku-rashin:begin -->\npointer wiring\n<!-- ryoku-rashin:end -->\n" +
		"User prefers fish shell over bash.\n\u00a7\n" +
		"## Project ryoku\nWorks on a Hyprland distro.\n\u00a7\n" +
		"   \n"
	entries := memoryEntries(raw)
	if len(entries) != 2 {
		t.Fatalf("entries = %d (%q)", len(entries), entries)
	}
	if entries[0] != "User prefers fish shell over bash." {
		t.Fatalf("first entry %q", entries[0])
	}
	if entryLabel(entries[1]) != "Project ryoku" {
		t.Fatalf("label %q", entryLabel(entries[1]))
	}
}

func TestMemoryEntriesEmptyWhenOnlyFence(t *testing.T) {
	raw := "<!-- ryoku-rashin:begin -->\nwiring only\n<!-- ryoku-rashin:end -->\n"
	if got := memoryEntries(raw); len(got) != 0 {
		t.Fatalf("expected no learned entries, got %q", got)
	}
}

func TestMemoryReportNotesMissingSQLite(t *testing.T) {
	seedHermes(t, map[string]string{"state.db": "present"})
	t.Setenv("PATH", t.TempDir())

	rep := MemoryReportNow()
	const want = "session metadata unavailable: sqlite3 is not installed"
	if rep.SessionsNote != want {
		t.Fatalf("sessions note = %q, want %q", rep.SessionsNote, want)
	}
	if len(rep.Sessions) != 0 {
		t.Fatalf("sessions = %+v, want none without sqlite3", rep.Sessions)
	}
}

func TestSkillsReportEmptyListsAreArrays(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	b, err := json.Marshal(SkillsReportNow())
	if err != nil {
		t.Fatal(err)
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"categories", "toolbelt"} {
		if string(raw[key]) != "[]" {
			t.Fatalf("%s = %s, want [] (the console reads .reduce on it)", key, raw[key])
		}
	}
}
