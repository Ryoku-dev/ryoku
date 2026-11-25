package brief

import (
	"strings"
	"testing"

	"ryoku-rashin/internal/protocol"
)

func TestManagedSkillsUseTheirOwnPaths(t *testing.T) {
	result := protocol.SkillsResult{Root: "/data/skills/builtin", Sets: []protocol.SkillSet{
		{ID: protocol.ManagedSkillSet, Name: "Learned skills", On: true, Skills: []protocol.Skill{
			{ID: "managed/fix-build", Name: "fix-build", Path: "/data/skills/managed/fix-build/SKILL.md"},
		}},
	}}
	text := Skills(result, true)
	if !strings.Contains(text, "/data/skills/managed/fix-build/SKILL.md") {
		t.Errorf("managed path was described as a built-in path: %s", text)
	}
}
