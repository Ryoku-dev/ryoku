package vault

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func testEnvironment(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	home := filepath.Join(root, "home")
	share := filepath.Join(root, "share")
	for _, dir := range []string{home, filepath.Join(share, "skills", "ryoku"), filepath.Join(share, "wiki"), filepath.Join(root, "base"), filepath.Join(root, "bin"), filepath.Join(root, "cache"), filepath.Join(root, "runtime")} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(share, "skills", "ryoku", "SKILL.md"), []byte("---\nname: ryoku\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(share, "wiki", "README.md"), []byte("# Wiki\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	t.Setenv("XDG_DATA_HOME", filepath.Join(root, "data"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(root, "state"))
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(root, "config"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(root, "cache"))
	t.Setenv("XDG_RUNTIME_DIR", filepath.Join(root, "runtime"))
	t.Setenv("RYOKU_CONFIG_BASE", filepath.Join(root, "base"))
	t.Setenv("PATH", filepath.Join(root, "bin"))
	t.Setenv("RYOKU_VAULT_SHARE", share)
	t.Setenv("RYOKU_VAULT_VERSION", "test")
	t.Setenv("XDG_CURRENT_DESKTOP", "")
	t.Setenv("RYOKU_WM", "")
	t.Setenv("RYOKU_REPO", "")
	return home
}

func TestFencedRegenerationPreservesProse(t *testing.T) {
	doc := "My note\n\n" + generatedBegin + "\nold\n" + generatedEnd + "\n\nKeep this too\n"
	got := ReplaceFenced(doc, "new")
	if !strings.Contains(got, "My note") || !strings.Contains(got, "Keep this too") {
		t.Fatalf("outside prose was lost: %q", got)
	}
	if strings.Contains(got, "\nold\n") || !strings.Contains(got, "\nnew\n") {
		t.Fatalf("generated body was not replaced: %q", got)
	}
}

func TestWireIsIdempotentAndReplacesRashinBlock(t *testing.T) {
	home := testEnvironment(t)
	claude := filepath.Join(home, ".claude")
	if err := os.MkdirAll(claude, 0o755); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(claude, "CLAUDE.md")
	old := "User prose\n\n" + oldPointerBegin + "\nold\n" + oldPointerEnd + "\n"
	if err := os.WriteFile(file, []byte(old), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Wire(); err != nil {
		t.Fatal(err)
	}
	if err := Wire(); err != nil {
		t.Fatal(err)
	}
	doc, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(doc), pointerBegin) != 1 || strings.Contains(string(doc), oldPointerBegin) || !strings.Contains(string(doc), "User prose") {
		t.Fatalf("unexpected wired document: %q", doc)
	}
}

func TestWirePreservesHarnessInstructionSymlink(t *testing.T) {
	home := testEnvironment(t)
	claude := filepath.Join(home, ".claude")
	dotfiles := filepath.Join(home, "dotfiles")
	if err := os.MkdirAll(claude, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dotfiles, 0o755); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(dotfiles, "CLAUDE.md")
	if err := os.WriteFile(target, []byte("Dotfiles prose\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(claude, "CLAUDE.md")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	if err := Wire(); err != nil {
		t.Fatal(err)
	}
	info, err := os.Lstat(link)
	if err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("harness instruction symlink was replaced: info=%v err=%v", info, err)
	}
	doc := readFile(target)
	if !strings.Contains(doc, "Dotfiles prose") || !strings.Contains(doc, pointerBegin) {
		t.Fatalf("symlink target was not wired: %q", doc)
	}
	if err := Unwire(); err != nil {
		t.Fatal(err)
	}
	if info, err := os.Lstat(link); err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("unwire replaced the harness instruction symlink: info=%v err=%v", info, err)
	}
	if strings.Contains(readFile(target), pointerBegin) {
		t.Fatal("unwire left the pointer in the symlink target")
	}
}

func TestSkillLinksOnlyUseExistingParentsAndUnwireOptsOut(t *testing.T) {
	home := testEnvironment(t)
	for _, dir := range []string{filepath.Join(home, ".claude"), filepath.Join(home, ".agents")} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	oldLink := filepath.Join(home, ".claude", "skills", "ryoku")
	if err := os.MkdirAll(filepath.Dir(oldLink), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("/usr/share/ryoku/rashin/skills/ryoku", oldLink); err != nil {
		t.Fatal(err)
	}
	if err := Wire(); err != nil {
		t.Fatal(err)
	}
	for _, link := range []string{
		filepath.Join(home, ".claude", "skills", "ryoku"),
		filepath.Join(home, ".agents", "skills", "ryoku"),
	} {
		if info, err := os.Lstat(link); err != nil || info.Mode()&os.ModeSymlink == 0 {
			t.Fatalf("expected skill link %s, info=%v err=%v", link, info, err)
		}
	}
	if target, err := os.Readlink(filepath.Join(home, ".claude", "skills", "ryoku")); err != nil || target != sourceDir(filepath.Join("skills", "ryoku")) {
		t.Fatalf("old Rashin skill link was not repointed: target=%q err=%v", target, err)
	}
	if _, err := os.Lstat(filepath.Join(home, ".codex")); !os.IsNotExist(err) {
		t.Fatalf("wire created an absent harness parent: %v", err)
	}
	if err := Unwire(); err != nil {
		t.Fatal(err)
	}
	if !IsOptedOut() {
		t.Fatal("unwire did not record the opt-out")
	}
	if err := Refresh(); err != nil {
		t.Fatal(err)
	}
	if Wired() {
		t.Fatal("refresh rewired an opted-out vault")
	}
}

func TestMigrationMergesMemoryAndJournalWithoutOverwrite(t *testing.T) {
	testEnvironment(t)
	if err := Ensure(); err != nil {
		t.Fatal(err)
	}
	newNote := filepath.Join(Path(), "memory", "note.txt")
	if err := os.WriteFile(newNote, []byte("new"), 0o644); err != nil {
		t.Fatal(err)
	}
	old := legacyPath()
	for _, dir := range []string{filepath.Join(old, "memory"), filepath.Join(old, "journal")} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	legacyAgents := "Before the old fence\n\n<!-- rashin:generated:begin -->\nlegacy\n<!-- rashin:generated:end -->\n\nAfter the old fence\n"
	if err := os.WriteFile(filepath.Join(old, "AGENTS.md"), []byte(legacyAgents), 0o644); err != nil {
		t.Fatal(err)
	}
	legacySystem := "# Legacy system\n\n<!-- rashin:generated:begin -->\nold system\n<!-- rashin:generated:end -->\n\nKeep this machine note\n"
	if err := os.WriteFile(filepath.Join(old, "system.md"), []byte(legacySystem), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(old, "memory", "note.txt"), []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(old, "journal", "2026-11-10.md"), []byte("journal"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(old, "personal.txt"), []byte("personal"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Migrate(); err != nil {
		t.Fatal(err)
	}
	agents := readFile(filepath.Join(Path(), "AGENTS.md"))
	if !strings.Contains(agents, "Before the old fence") || !strings.Contains(agents, "After the old fence") || strings.Contains(agents, "legacy\n") {
		t.Fatalf("legacy AGENTS prose was not preserved cleanly: %q", agents)
	}
	if err := writeGenerated("system.md", "# System", "current system"); err != nil {
		t.Fatal(err)
	}
	system := readFile(filepath.Join(Path(), "system.md"))
	if !strings.Contains(system, "# Legacy system") || !strings.Contains(system, "Keep this machine note") || !strings.Contains(system, "current system") || strings.Contains(system, "old system") {
		t.Fatalf("legacy generated-file prose was not preserved: %q", system)
	}
	if got := readFile(newNote); got != "new" {
		t.Fatalf("new note was overwritten: %q", got)
	}
	if got := readFile(newNote + ".rashin-1"); got != "old" {
		t.Fatalf("old note was not kept as a conflict: %q", got)
	}
	if !fileExists(filepath.Join(Path(), "journal", "2026-11-10.md")) || !fileExists(filepath.Join(Path(), "personal.txt")) {
		t.Fatal("migration did not move journal and user files")
	}
	if _, err := os.Stat(old); !os.IsNotExist(err) {
		t.Fatalf("legacy vault remains: %v", err)
	}
	if err := Migrate(); err != nil {
		t.Fatalf("idempotent migration failed: %v", err)
	}
}
