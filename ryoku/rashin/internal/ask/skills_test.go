package ask

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ryoku-rashin/internal/core"
	"ryoku-rashin/internal/llm"
	"ryoku-rashin/internal/protocol"
	"ryoku-rashin/internal/skills"
)

func skillsRunner(t *testing.T, client llm.Client, off []string) core.Runner {
	t.Helper()
	root := t.TempDir()
	t.Setenv("HOME", root)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(root, "config"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(root, "state"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(root, "data"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(root, "cache"))
	factory := New(client, nil, nil, nil)
	factory.SetSkills(skills.New(filepath.Join(root, "skills")))
	return factory.NewAsk(core.AskOptions{
		Session: protocol.SessionInfo{ID: "ask-skills", Model: "auto"},
		Settings: func() protocol.Settings {
			return protocol.Settings{Ask: protocol.AskSettings{Model: "auto"}, Skills: protocol.SkillsSettings{Off: off}}
		},
	})
}

// skillsSection cuts the # Skills section out of a system prompt.
func skillsSection(t *testing.T, system string) string {
	t.Helper()
	_, section, found := strings.Cut(system, "# Skills\n\n")
	if !found {
		t.Fatalf("no Skills section in the system prompt:\n%s", system)
	}
	if end := strings.Index(section, "\n\n#"); end >= 0 {
		section = section[:end]
	}
	return section
}

func TestAskIsToldTheSkillsAndReadsOne(t *testing.T) {
	client := &scriptedClient{streams: []*scriptedStream{
		finalStream(llm.Delta{ToolCalls: []llm.ToolCall{call("c1", "skills", map[string]string{"query": "write a failing test first"})}}),
		finalStream(llm.Delta{ToolCalls: []llm.ToolCall{call("c2", "skill", map[string]string{"id": "superpowers/test-driven-development"})}}),
		finalStream(llm.Delta{Text: "Write the test first."}),
	}}
	sink := &recordingSink{}
	if _, err := skillsRunner(t, client, nil).Prompt(context.Background(), core.PromptInput{Turn: 1, Text: "How should I add this feature?"}, sink); err != nil {
		t.Fatal(err)
	}
	first := client.calls[0]
	section := skillsSection(t, first.Messages[0].Content)
	if len(section) > 1200 {
		t.Errorf("the Skills section is %d characters, want at most about 1200:\n%s", len(section), section)
	}
	for _, want := range []string{"Superpowers", "test-driven-development", "OpenSpec", "Spec Kit", "Elephant and Goldfish", "egm-{", "rashin skills setup speckit", "skills tool", "skill tool"} {
		if !strings.Contains(section, want) {
			t.Errorf("the Skills section lacks %q:\n%s", want, section)
		}
	}
	if !toolNames(first)["skills"] || !toolNames(first)["skill"] {
		t.Fatalf("the skill tools are not offered: %v", toolNames(first))
	}
	found := client.calls[1].Messages[len(client.calls[1].Messages)-1]
	if found.Role != llm.RoleTool || !strings.HasPrefix(found.Content, "- superpowers/test-driven-development:") {
		t.Fatalf("the search answered %q", found.Content)
	}
	read := client.calls[2].Messages[len(client.calls[2].Messages)-1]
	header, text, _ := strings.Cut(read.Content, "]\n\n")
	if read.Role != llm.RoleTool || !strings.HasPrefix(header, "[SKILL.md of superpowers/test-driven-development, at ") {
		t.Fatalf("the skill tool answered %q", read.Content)
	}
	at := strings.TrimPrefix(header, "[SKILL.md of superpowers/test-driven-development, at ")
	at, _, _ = strings.Cut(at, ";")
	whole, err := os.ReadFile(at)
	if err != nil {
		t.Fatal(err)
	}
	if text != string(whole) || len(whole) <= 6000 {
		t.Fatalf("the model was not told the whole SKILL.md (%d of %d bytes)", len(text), len(whole))
	}
}

func TestAskIsNotToldASetThatIsOff(t *testing.T) {
	client := &scriptedClient{streams: []*scriptedStream{
		finalStream(llm.Delta{ToolCalls: []llm.ToolCall{call("c1", "skill", map[string]string{"id": "egm/egm-draft"})}}),
		finalStream(llm.Delta{Text: "It is off."}),
	}}
	if _, err := skillsRunner(t, client, []string{"egm"}).Prompt(context.Background(), core.PromptInput{Turn: 1, Text: "Draft a design."}, &recordingSink{}); err != nil {
		t.Fatal(err)
	}
	if section := skillsSection(t, client.calls[0].Messages[0].Content); strings.Contains(section, "Elephant") || strings.Contains(section, "egm") {
		t.Errorf("a set that is off is in the prompt:\n%s", section)
	}
	if answer := client.calls[1].Messages[len(client.calls[1].Messages)-1].Content; !strings.Contains(answer, "No skill egm/egm-draft") {
		t.Errorf("reading a skill of a set that is off answered %q", answer)
	}
}
