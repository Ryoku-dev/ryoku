package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func agentEnv(t *testing.T) string {
	t.Helper()
	h := t.TempDir()
	t.Setenv("HOME", h)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(h, ".config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(h, ".local", "share"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(h, ".local", "state"))
	t.Setenv("RYOKU_CONFIG_BASE", filepath.Join(h, "packaged-config-absent"))
	t.Setenv("RYOKU_PROWL_PORT", "1")
	t.Setenv("RYOKU_RASHIN_VAULT", filepath.Join(h, "vault"))
	return h
}

func TestUpsertBlockIdempotent(t *testing.T) {
	doc := "# existing notes\n\nsome text\n"
	once := upsertBlock(doc)
	twice := upsertBlock(once)
	if once != twice {
		t.Fatalf("upsertBlock not idempotent:\n%q\nvs\n%q", once, twice)
	}
	if !strings.Contains(once, pointerBegin) || !strings.Contains(once, pointerEnd) {
		t.Fatal("upsertBlock did not insert the fenced block")
	}
	if !strings.Contains(once, "some text") {
		t.Fatal("upsertBlock dropped existing content")
	}
	if strings.Count(once, pointerBegin) != 1 {
		t.Fatal("upsertBlock inserted more than one block")
	}
}

func TestUpsertReplacesStaleBlock(t *testing.T) {
	stale := "top\n\n" + pointerBegin + "\nOLD POINTER TEXT\n" + pointerEnd + "\n\nbottom\n"
	got := upsertBlock(stale)
	if strings.Contains(got, "OLD POINTER TEXT") {
		t.Fatalf("stale block survived: %q", got)
	}
	if !strings.Contains(got, "top") || !strings.Contains(got, "bottom") {
		t.Fatalf("upsert clobbered surrounding text: %q", got)
	}
	if strings.Count(got, pointerBegin) != 1 {
		t.Fatal("upsert left a duplicate marker")
	}
}

func TestRemoveBlockClean(t *testing.T) {
	doc := "alpha\n"
	wired := upsertBlock(doc)
	unwired := removeBlock(wired)
	if strings.Contains(unwired, pointerBegin) || strings.Contains(unwired, pointerEnd) {
		t.Fatalf("removeBlock left markers: %q", unwired)
	}
	if !strings.Contains(unwired, "alpha") {
		t.Fatalf("removeBlock dropped user content: %q", unwired)
	}
	if strings.Contains(unwired, "\n\n\n") {
		t.Fatalf("removeBlock left a blank-line pileup: %q", unwired)
	}
	if removeBlock(unwired) != unwired {
		t.Fatal("removeBlock not idempotent on a clean doc")
	}
}

func TestWireSkipsAbsentAgentDir(t *testing.T) {
	h := agentEnv(t)
	// claude dir is absent: Wire must refuse and create nothing.
	if err := Wire("claude"); err == nil {
		t.Fatal("Wire(claude) should fail when ~/.claude is absent")
	}
	if _, err := os.Stat(filepath.Join(h, ".claude")); !os.IsNotExist(err) {
		t.Fatal("Wire created the agent home dir; it must not")
	}
	// Present the codex dir; Wire must create the file and wire it.
	if err := os.MkdirAll(filepath.Join(h, ".codex"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := Wire("codex"); err != nil {
		t.Fatalf("Wire(codex): %v", err)
	}
	b, err := os.ReadFile(filepath.Join(h, ".codex", "AGENTS.md"))
	if err != nil {
		t.Fatalf("codex AGENTS.md not created: %v", err)
	}
	if !strings.Contains(string(b), pointerBegin) {
		t.Fatal("codex file not wired")
	}
	// Wiring again is a no-op on disk.
	if err := Wire("codex"); err != nil {
		t.Fatal(err)
	}
	if b2, _ := os.ReadFile(filepath.Join(h, ".codex", "AGENTS.md")); string(b2) != string(b) {
		t.Fatal("re-Wire(codex) changed the file")
	}
}

func TestWireOpencodeCreatesConfigDir(t *testing.T) {
	h := agentEnv(t)
	// opencode's own dir is absent, but ~/.config exists, so Wire may create it.
	if err := os.MkdirAll(filepath.Join(h, ".config"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := Wire("opencode"); err != nil {
		t.Fatalf("Wire(opencode): %v", err)
	}
	if _, err := os.Stat(filepath.Join(h, ".config", "opencode", "AGENTS.md")); err != nil {
		t.Fatalf("opencode file not created: %v", err)
	}
}

func TestUnwireKeepsFile(t *testing.T) {
	h := agentEnv(t)
	if err := os.MkdirAll(filepath.Join(h, ".codex"), 0o755); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(h, ".codex", "AGENTS.md")
	if err := os.WriteFile(file, []byte("keep me\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Wire("codex"); err != nil {
		t.Fatal(err)
	}
	if err := Unwire("codex"); err != nil {
		t.Fatalf("Unwire(codex): %v", err)
	}
	b, err := os.ReadFile(file)
	if err != nil {
		t.Fatalf("Unwire removed the file: %v", err)
	}
	if strings.Contains(string(b), pointerBegin) {
		t.Fatal("Unwire left the block")
	}
	if !strings.Contains(string(b), "keep me") {
		t.Fatal("Unwire dropped user content")
	}
}

func TestWireAllOnlyPresent(t *testing.T) {
	h := agentEnv(t)
	if err := os.MkdirAll(filepath.Join(h, ".claude"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(h, ".omp", "agent"), 0o755); err != nil {
		t.Fatal(err)
	}
	n := WireAll()
	if n != 2 {
		t.Fatalf("WireAll wired %d agents; want 2 (claude, omp)", n)
	}
	for _, f := range []string{
		filepath.Join(h, ".claude", "CLAUDE.md"),
		filepath.Join(h, ".omp", "agent", "AGENTS.md"),
	} {
		b, err := os.ReadFile(f)
		if err != nil || !strings.Contains(string(b), pointerBegin) {
			t.Fatalf("expected %s wired", f)
		}
	}
	if _, err := os.Stat(filepath.Join(h, ".codex", "AGENTS.md")); !os.IsNotExist(err) {
		t.Fatal("WireAll touched an absent agent")
	}
}

func TestDetectAgentsPresentWired(t *testing.T) {
	h := agentEnv(t)
	if err := os.MkdirAll(filepath.Join(h, ".claude"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := Wire("claude"); err != nil {
		t.Fatal(err)
	}
	var claude Agent
	for _, a := range DetectAgents() {
		if a.ID == "claude" {
			claude = a
		}
		if a.ID == "codex" && a.Present {
			t.Fatal("codex reported present with no ~/.codex")
		}
	}
	if !claude.Present || !claude.Wired {
		t.Fatalf("claude should be present and wired: %+v", claude)
	}
	if !strings.HasPrefix(claude.File, "~/") {
		t.Fatalf("agent file path should be tilde-abbreviated: %q", claude.File)
	}
}

// prowlSkillClients maps every installed harness with a skills directory onto
// the gateway setup API's client ids.
func TestProwlSkillClients(t *testing.T) {
	h := agentEnv(t)
	has := func(ids []string, id string) bool {
		for _, g := range ids {
			if g == id {
				return true
			}
		}
		return false
	}
	// No coding-agent homes -> neither claude nor omp is reported.
	got := prowlSkillClients()
	if has(got, "claude") || has(got, "omp") {
		t.Fatalf("no agent dirs but got %v", got)
	}
	// claude + omp homes -> both detected.
	for _, d := range []string{".claude", ".omp", ".codex"} {
		if err := os.MkdirAll(filepath.Join(h, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	got = prowlSkillClients()
	if !has(got, "claude") || !has(got, "omp") {
		t.Fatalf("prowlSkillClients() = %v, want claude and omp present", got)
	}
	if !has(got, "codex") {
		t.Fatalf("prowlSkillClients() omitted codex: %v", got)
	}
	if has(got, "opencode") {
		t.Fatalf("opencode should be absent without its config home: %v", got)
	}
}

func TestProwlHarnessConfigSetKeepsStableUniqueOrder(t *testing.T) {
	cfg := Config{ProwlHarnesses: []string{"claude", "codex"}}
	if cfg.AddProwlHarness("claude") {
		t.Fatal("duplicate connection changed the set")
	}
	if !cfg.AddProwlHarness("omp") {
		t.Fatal("new connection was not added")
	}
	if !cfg.RemoveProwlHarness("codex") || cfg.RemoveProwlHarness("missing") {
		t.Fatal("connection removal reported the wrong result")
	}
	got := strings.Join(cfg.ProwlHarnesses, ",")
	if got != "claude,omp" {
		t.Fatalf("connected harness order = %q", got)
	}
}
