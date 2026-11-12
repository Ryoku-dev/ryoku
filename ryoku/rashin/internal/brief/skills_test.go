package brief

import (
	"strings"
	"testing"

	"ryoku-rashin/internal/protocol"
)

func TestSkillsNamesEachSetThatIsOnWithWhatItNeeds(t *testing.T) {
	result := protocol.SkillsResult{Root: "/data/skills/abc", Sets: []protocol.SkillSet{
		{ID: "openspec", Name: "OpenSpec", On: true, Needs: []protocol.SkillNeed{{Command: "openspec", Install: "npm install -g @fission-ai/openspec"}},
			Skills: []protocol.Skill{{Name: "openspec-propose"}, {Name: "openspec-apply-change"}}},
		{ID: "mixed", Name: "Mixed", On: true, Skills: []protocol.Skill{{Name: "plan"}, {Name: "review", Manual: true}}},
		{ID: "egm", Name: "Elephant and Goldfish", On: true, Skills: []protocol.Skill{{Name: "egm-draft", Manual: true}, {Name: "egm-review", Manual: true}}},
		{ID: "gone", Name: "Gone", On: false, Skills: []protocol.Skill{{Name: "hidden"}}},
		{ID: "present", Name: "Present", On: true, Needs: []protocol.SkillNeed{{Command: "git", Present: true}}, Skills: []protocol.Skill{{Name: "one"}}},
	}}
	text := Skills(result, true)
	for _, want := range []string{
		"/data/skills/abc/<set>/skills/<name>/SKILL.md",
		"- OpenSpec (openspec), needs openspec (missing: npm install -g @fission-ai/openspec): openspec-{propose, apply-change}",
		"- Mixed (mixed): plan, review (only when the user names it)",
		"- Elephant and Goldfish (egm), only when the user names one: egm-{draft, review}",
		"- Present (present): one\n",
		"skills tool",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("lacks %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, "Gone") {
		t.Errorf("a set that is off is named:\n%s", text)
	}
	if command := Skills(result, false); !strings.Contains(command, "rashin skills search") || strings.Contains(command, "skills tool") {
		t.Errorf("without the tools:\n%s", command)
	}
	if Skills(protocol.SkillsResult{Sets: []protocol.SkillSet{{ID: "gone", On: false}}}, true) != "" {
		t.Error("with no set on there is still a Skills section")
	}
}
