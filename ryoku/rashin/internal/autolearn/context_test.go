package autolearn

import (
	"path/filepath"
	"strings"
	"testing"

	"ryoku-rashin/internal/engine"
	"ryoku-rashin/internal/engine/tool"
	"ryoku-rashin/internal/protocol"
	"ryoku-rashin/internal/skills"
)

func skillsBlock(t *testing.T, settings protocol.Settings, profile tool.Profile) (string, int) {
	t.Helper()
	dir := t.TempDir()
	library := skills.New(filepath.Join(dir, "builtin"))
	managed := library.WithManaged(filepath.Join(dir, "managed"))
	if _, err := managed.Create("vendor-rebuild", "Use when vendored Go modules are missing and the build fails with a long explanation after it.", "1. Run go mod vendor."); err != nil {
		t.Fatal(err)
	}
	provider := NewSkillsContext(library, func() protocol.Settings { return settings }, nil)
	blocks := provider.Blocks(t.Context(), engine.TurnContext{Profile: profile})
	if len(blocks) != 1 || blocks[0].Name != engine.BlockSkills {
		t.Fatalf("blocks = %+v", blocks)
	}
	again := provider.Blocks(t.Context(), engine.TurnContext{Profile: profile})
	if again[0].Text != blocks[0].Text {
		t.Error("the block changed while nothing did")
	}
	return blocks[0].Text, blocks[0].Budget
}

func TestFullBlockListsSetsAndLearnedSkillsWithShortHints(t *testing.T) {
	text, budget := skillsBlock(t, autolearnOn(), tool.Profile{Mode: tool.ModeAgent, Class: tool.ClassFull})
	if len(text) > budget {
		t.Errorf("block is %d bytes over a budget of %d", len(text), budget)
	}
	for _, want := range []string{"Superpowers (superpowers):", "- brainstorming:", "the skill tool", "Learned from earlier work (managed):\n- vendor-rebuild: Use when vendored Go modules are missing and the build fails with"} {
		if !strings.Contains(text, want) {
			t.Errorf("block lacks %q:\n%s", want, text)
		}
	}
	for _, line := range strings.Split(text, "\n") {
		if name, h, ok := strings.Cut(strings.TrimPrefix(line, "- "), ": "); ok && strings.HasPrefix(line, "- ") {
			if words := len(strings.Fields(strings.TrimSuffix(h, " (only when the user names it)"))); words > hintWords {
				t.Errorf("hint of %s has %d words", name, words)
			}
		}
	}
}

func TestCompactBlockIsNamesOnlyWithinBudget(t *testing.T) {
	text, budget := skillsBlock(t, autolearnOn(), tool.Profile{Mode: tool.ModeAgent, Class: tool.ClassCompact})
	if len(text) > compactBudget || budget != compactBudget {
		t.Errorf("compact block is %d bytes, budget %d", len(text), budget)
	}
	lines := strings.Split(text, "\n")
	if len(lines) < 2 || lines[1] != "managed: vendor-rebuild" {
		t.Errorf("learned skills do not come first:\n%s", text)
	}
	if strings.Contains(text, "Use when") || !strings.Contains(text, "with read:") {
		t.Errorf("compact block carries hints or no way to read:\n%s", text)
	}
}

func TestBlockLeavesLearnedSkillsOutWhileTheFeatureIsOff(t *testing.T) {
	text, _ := skillsBlock(t, protocol.Settings{}, tool.Profile{Mode: tool.ModeAgent, Class: tool.ClassFull})
	if strings.Contains(text, "vendor-rebuild") || !strings.Contains(text, "brainstorming") {
		t.Errorf("block with autolearn off:\n%s", text)
	}
}

func TestHintIsTheFirstSentenceAtMostTwelveWords(t *testing.T) {
	for in, want := range map[string]string{
		"Use when a task has many steps. Plans them first.":                       "Use when a task has many steps",
		"Short. Then a much longer second sentence follows.":                      "Short. Then a much longer second sentence follows",
		"one two three four five six seven eight nine ten eleven twelve thirteen": "one two three four five six seven eight nine ten eleven twelve",
	} {
		if got := hint(in); got != want {
			t.Errorf("hint(%q) = %q, want %q", in, got, want)
		}
	}
}
