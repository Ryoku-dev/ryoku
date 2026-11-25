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
	return blocks[0].Text, blocks[0].Budget
}

func TestFullSkillContextFitsItsBudget(t *testing.T) {
	text, budget := skillsBlock(t, autolearnOn(), tool.Profile{Mode: tool.ModeAgent, Class: tool.ClassFull})
	if len(text) > budget {
		t.Errorf("block is %d bytes over a budget of %d", len(text), budget)
	}
	if !strings.Contains(text, "brainstorming") || !strings.Contains(text, "vendor-rebuild") {
		t.Fatalf("the full profile lost built-in or learned skill discovery:\n%s", text)
	}
}

func TestCompactBlockIsNamesOnlyWithinBudget(t *testing.T) {
	text, budget := skillsBlock(t, autolearnOn(), tool.Profile{Mode: tool.ModeAgent, Class: tool.ClassCompact})
	if len(text) > compactBudget || budget != compactBudget {
		t.Errorf("compact block is %d bytes, budget %d", len(text), budget)
	}
	if !strings.Contains(text, "vendor-rebuild") || strings.Contains(text, "brainstorming") {
		t.Fatalf("the compact profile lost learned skills or included the built-in catalogue:\n%s", text)
	}
}

func TestBlockLeavesLearnedSkillsOutWhileTheFeatureIsOff(t *testing.T) {
	text, _ := skillsBlock(t, protocol.Settings{}, tool.Profile{Mode: tool.ModeAgent, Class: tool.ClassFull})
	if strings.Contains(text, "vendor-rebuild") || !strings.Contains(text, "brainstorming") {
		t.Errorf("block with autolearn off:\n%s", text)
	}
}
