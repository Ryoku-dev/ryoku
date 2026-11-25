package engine

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"ryoku-rashin/internal/brief"
	"ryoku-rashin/internal/engine/tool"
	"ryoku-rashin/internal/protocol"
	"ryoku-rashin/internal/sandbox"
)

// promptMachine gives the test a home with a Ryoku vault.
func promptMachine(t *testing.T) {
	t.Helper()
	root := t.TempDir()
	t.Setenv("HOME", root)
	t.Setenv("XDG_DATA_HOME", filepath.Join(root, "data"))
	vault := filepath.Join(root, "data", "ryoku", "vault")
	if err := os.MkdirAll(vault, 0o700); err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string]string{"AGENTS.md": "Read desktop.md first.", "desktop.md": strings.Repeat("bar: ~/.config/quickshell\n", 40)} {
		if err := os.WriteFile(filepath.Join(vault, name), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

func promptFor(mode tool.Mode, class tool.Class, approvals string) PromptInput {
	in := PromptInput{
		Profile: tool.Profile{Mode: mode, Class: class, Window: 16384},
		Session: protocol.SessionInfo{Mode: approvals, Cwd: "/work/app"},
		Cwd:     "/work/app",
		Tools:   tool.Names(mode, class),
	}
	if mode != tool.ModeAskRead && mode != tool.ModeAskAct {
		in.Brief = brief.Agent(brief.Session{Info: protocol.SessionInfo{Cwd: "/work/app", Branch: "main"}})
	}
	return in
}

// A small local model reads at most 3 KB of instructions in every mode, so
// its few rules are not crowded out.
func TestTheCompactPromptStaysWithinItsBudgetInEveryMode(t *testing.T) {
	promptMachine(t)
	for _, mode := range tool.Modes {
		for _, approvals := range []string{protocol.ApprovalsAsk, protocol.ApprovalsAuto} {
			text := SystemPrompt(promptFor(mode, tool.ClassCompact, approvals))
			if len(text) > CompactPromptMax {
				t.Errorf("the compact %s prompt is %d bytes:\n%s", mode, len(text), text)
			}
			if !strings.Contains(text, "{\"") {
				t.Errorf("the compact %s prompt shows no tool call", mode)
			}
		}
	}
}

// Every mode has its own compact and full prompt, and each one names only
// tools the mode offers in that class.
func TestEveryModeHasACompactAndAFullPrompt(t *testing.T) {
	promptMachine(t)
	every := map[string]bool{}
	for _, mode := range tool.Modes {
		for _, class := range []tool.Class{tool.ClassCompact, tool.ClassFull} {
			name := filepath.ToSlash(filepath.Join("prompts", string(class), string(mode)+".md"))
			if _, err := fs.Stat(promptFiles, name); err != nil {
				t.Errorf("no %s template for %s", class, mode)
			}
			prefix, _ := BuildSystem(promptFor(mode, class, protocol.ApprovalsAsk))
			if every[prefix] {
				t.Errorf("the %s %s prompt repeats another mode's", class, mode)
			}
			every[prefix] = true
		}
	}
}

// What changes from turn to turn (memory, habits, the session brief) stays
// out of the prefix, so llama.cpp reuses the cached prompt.
func TestThePrefixIsByteStableAcrossTurns(t *testing.T) {
	promptMachine(t)
	for _, class := range []tool.Class{tool.ClassCompact, tool.ClassFull} {
		first := promptFor(tool.ModeAgent, class, protocol.ApprovalsAsk)
		first.Blocks = []Block{{Name: BlockMemory, Text: "The user prefers tabs.", Budget: 400}}
		second := first
		second.Brief = brief.Agent(brief.Session{Info: protocol.SessionInfo{Cwd: "/work/other", Branch: "fix"}})
		second.Blocks = []Block{{Name: BlockMemory, Text: "The user moved to Lisbon.", Budget: 400}}
		second.Habits = "shell: fish"
		a, footerA := BuildSystem(first)
		b, footerB := BuildSystem(second)
		if a != b {
			t.Errorf("the %s prefix changed between turns", class)
		}
		if footerA == footerB || !strings.Contains(footerB, "Lisbon") || strings.Contains(a, "prefers tabs") {
			t.Errorf("the %s footer does not carry the turn's blocks", class)
		}
		if again, _ := BuildSystem(first); again != a {
			t.Errorf("the %s prefix is not deterministic", class)
		}
	}
}

func TestReminderCarriesChangingDateAndTerminalOutsideThePrefix(t *testing.T) {
	in := promptFor(tool.ModeAskRead, tool.ClassCompact, protocol.ApprovalsReadOnly)
	prefix, _ := BuildSystem(in)
	status := 2
	note := Reminder(time.Date(2026, 11, 18, 8, 0, 0, 0, time.UTC), "/work/app", &protocol.TerminalContext{
		Cwd: "/work/app", Shell: "fish", LastCommand: "make check", LastStatus: &status, Buffer: true,
	})
	for _, want := range []string{"Wednesday 18 November 2026", "/work/app", "make check", "last exit status: 2", "- buffer: true"} {
		if !strings.Contains(note, want) || strings.Contains(prefix, want) {
			t.Errorf("%q should only be in the turn's reminder", want)
		}
	}
}

func writePromptFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

// Context files are found from the folder up to the project root, one per
// folder, the closest last; @path imports expand inside the project only.
func TestProjectContextFilesAreFoundAndImported(t *testing.T) {
	outside := t.TempDir()
	project := t.TempDir()
	writePromptFile(t, filepath.Join(outside, "secret.md"), "SECRET")
	writePromptFile(t, filepath.Join(project, "AGENTS.md"), "Root rules.\nSee @docs/style.md and @"+filepath.Join(outside, "secret.md")+" and `@docs/style.md`.\nMail me@example.com.")
	writePromptFile(t, filepath.Join(project, "docs", "style.md"), "Use tabs.")
	writePromptFile(t, filepath.Join(project, "app", "CLAUDE.md"), "App rules.")
	writePromptFile(t, filepath.Join(project, "app", "AGENTS.md"), "App agents rules win.")
	text := projectContext(filepath.Join(project, "app"), project, 8<<10)
	root, app := strings.Index(text, "Root rules."), strings.Index(text, "App agents rules win.")
	if root < 0 || app < 0 || root > app {
		t.Fatalf("the files read, farthest first:\n%s", text)
	}
	if strings.Contains(text, "App rules.") {
		t.Error("a folder gave two context files")
	}
	if strings.Count(text, "Use tabs.") != 1 || !strings.Contains(text, `<import path="docs/style.md">`) {
		t.Errorf("the import was not expanded once:\n%s", text)
	}
	if strings.Contains(text, "SECRET") {
		t.Error("an import reached outside the project")
	}
	if outsideOnly := projectContext(filepath.Join(project, "app"), "", 8<<10); strings.Contains(outsideOnly, "Root rules.") {
		t.Error("outside Git the walk went above the folder")
	}
}

func TestSandboxedProjectContextSkipsHiddenRulesAndImports(t *testing.T) {
	project := t.TempDir()
	writePromptFile(t, filepath.Join(project, ".env"), "HIDDEN-SECRET")
	writePromptFile(t, filepath.Join(project, "visible.md"), "VISIBLE-RULE")
	writePromptFile(t, filepath.Join(project, "app", "AGENTS.md"), "HIDDEN-RULE")
	writePromptFile(t, filepath.Join(project, "app", "CLAUDE.md"), "VISIBLE-APP-RULE")
	if err := os.Symlink(filepath.Join(project, ".env"), filepath.Join(project, "link.md")); err != nil {
		t.Fatal(err)
	}
	writePromptFile(t, filepath.Join(project, "AGENTS.md"), "See @.env @link.md and @visible.md.")
	spec := &sandbox.Spec{Folder: project, Hidden: []string{
		filepath.Join(project, ".env"),
		filepath.Join(project, "app", "AGENTS.md"),
	}}
	in := promptFor(tool.ModeTeam, tool.ClassFull, protocol.ApprovalsAuto)
	in.Cwd, in.Project, in.Sandbox = filepath.Join(project, "app"), project, spec
	_, footer := BuildSystem(in)
	for _, forbidden := range []string{"HIDDEN-SECRET", "HIDDEN-RULE"} {
		if strings.Contains(footer, forbidden) {
			t.Fatalf("hidden context %q crossed into team prompt: %s", forbidden, footer)
		}
	}
	for _, visible := range []string{"VISIBLE-RULE", "VISIBLE-APP-RULE"} {
		if !strings.Contains(footer, visible) {
			t.Fatalf("visible context %q was lost: %s", visible, footer)
		}
	}
}

// Imports stop at five hops and at their cycle, and the files together stay
// within the budget.
func TestProjectContextImportsAndSizeAreCapped(t *testing.T) {
	project := t.TempDir()
	writePromptFile(t, filepath.Join(project, "AGENTS.md"), "start @f1.md")
	for i := 1; i <= 7; i++ {
		writePromptFile(t, filepath.Join(project, "f"+string(rune('0'+i))+".md"), "level "+string(rune('0'+i))+" @f"+string(rune('0'+i+1))+".md")
	}
	writePromptFile(t, filepath.Join(project, "loop.md"), "loop @AGENTS.md")
	text := projectContext(project, project, 8<<10)
	if !strings.Contains(text, "level 5") || strings.Contains(text, "level 6") {
		t.Errorf("imports went past five hops:\n%s", text)
	}
	writePromptFile(t, filepath.Join(project, "AGENTS.md"), "start @loop.md")
	if text := projectContext(project, project, 8<<10); strings.Count(text, "start") != 1 {
		t.Errorf("an import cycle repeated:\n%s", text)
	}
	writePromptFile(t, filepath.Join(project, "AGENTS.md"), strings.Repeat("A long rule line.\n", 400))
	if text := projectContext(project, project, 2<<10); len(text) > 2<<10+200 || !strings.Contains(text, "[trimmed]") {
		t.Errorf("a long file read %d bytes for a 2 KiB budget", len(text))
	}
}

func TestProjectContextCountsItsWrappersAndIgnoresExternalLinks(t *testing.T) {
	project := t.TempDir()
	outside := t.TempDir()
	writePromptFile(t, filepath.Join(outside, "secret.md"), "PRIVATE-DATA")
	if err := os.Symlink(filepath.Join(outside, "secret.md"), filepath.Join(project, "AGENTS.md")); err != nil {
		t.Fatal(err)
	}
	if text := projectContext(project, project, 400); strings.Contains(text, "PRIVATE-DATA") {
		t.Errorf("an external symlink entered the project context: %s", text)
	}
	writePromptFile(t, filepath.Join(project, "app", "AGENTS.md"), strings.Repeat("Keep dependencies small.\n", 100))
	text := projectContext(filepath.Join(project, "app"), project, 400)
	if len(text) > 400 || !strings.Contains(text, "[trimmed]") {
		t.Errorf("project context took %d bytes for a 400-byte budget: %s", len(text), text)
	}
}

// Each block is cut to its own budget, and blocks past the class's overall
// cap give way, the last first.
func TestContextBlocksKeepToTheirBudgets(t *testing.T) {
	promptMachine(t)
	in := promptFor(tool.ModeAgent, tool.ClassCompact, protocol.ApprovalsAsk)
	in.Blocks = []Block{
		{Name: BlockSkills, Text: strings.Repeat("skill line\n", 200), Budget: 600},
		{Name: BlockDecisions, Text: strings.Repeat("decision line\n", 2000), Budget: 64 << 10},
		{Name: BlockMemory, Text: "The user prefers tabs.", Budget: 400},
	}
	_, footer := BuildSystem(in)
	skills := footer[strings.Index(footer, "# Skills"):strings.Index(footer, "# Project decisions")]
	if len(skills) > 600+len("# Skills\n\n") {
		t.Errorf("the skills block took %d bytes for a 600 byte budget", len(skills))
	}
	if len(footer) > blocksBudget(in.Profile)+1024 || strings.Contains(footer, "prefers tabs") {
		t.Errorf("the blocks overran the compact cap: %d bytes", len(footer))
	}
}
