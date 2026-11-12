package brief

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"ryoku-rashin/internal/protocol"
)

// The skill tools' names, the same in an agent's rashin MCP server and in
// Ask.
const (
	ToolSkills = "skills"
	ToolSkill  = "skill"
)

// skillsMax is the most skills one search returns.
const skillsMax = 20

// SkillTool is one of the tools that find and read the built-in skills.
// Agents get them from the rashin MCP server and Ask from its own tool list,
// from this one definition.
type SkillTool struct {
	Name        string
	Description string
	// Schema is the JSON schema of the tool's arguments.
	Schema map[string]any
}

// SkillTools are the skill tools in the order they are offered; both only
// read.
func SkillTools() []SkillTool {
	return []SkillTool{
		{
			Name:        ToolSkills,
			Description: "Search the skills Rashin ships built in (Superpowers, OpenSpec, Spec Kit, Elephant and Goldfish) for the ones that fit what you are about to do, best first, each with its id, what it is for and where its SKILL.md lies. An empty query lists them all. Search before a task, then read the skill that fits with " + ToolSkill + ".",
			Schema: memorySchema([]string{}, map[string]any{
				"query": map[string]any{"type": "string", "description": "What you are about to do, in a few words; empty lists every skill."},
				"limit": map[string]any{"type": "integer", "minimum": 1, "maximum": skillsMax, "description": fmt.Sprintf("How many to return; 5 unless set, at most %d.", skillsMax)},
			}),
		},
		{
			Name:        ToolSkill,
			Description: "Read a built-in skill's SKILL.md by its id (set/name, from " + ToolSkills + "), or one of its files by a path relative to the skill's folder, and follow it. The answer names the skill's other files. A skill marked for the user runs only when the user names it.",
			Schema: memorySchema([]string{"id"}, map[string]any{
				"id":   map[string]any{"type": "string", "description": "The skill's id, such as superpowers/test-driven-development."},
				"file": map[string]any{"type": "string", "description": "A file of the skill relative to its folder; SKILL.md unless set."},
			}),
		},
	}
}

// IsSkillTool reports a skill tool's name.
func IsSkillTool(name string) bool { return name == ToolSkills || name == ToolSkill }

// SkillParams reads a skill tool's arguments into the params of its method.
func SkillParams(name string, arguments json.RawMessage) (string, any, error) {
	if len(arguments) == 0 {
		arguments = json.RawMessage(`{}`)
	}
	var args struct {
		Query string `json:"query"`
		Limit int    `json:"limit"`
		ID    string `json:"id"`
		File  string `json:"file"`
	}
	if err := json.Unmarshal(arguments, &args); err != nil {
		return "", nil, errors.New("the arguments are not a JSON object of the tool's fields")
	}
	switch name {
	case ToolSkills:
		return protocol.MSkillsSearch, protocol.SkillsSearchParams{Query: args.Query, Limit: min(args.Limit, skillsMax)}, nil
	case ToolSkill:
		if strings.TrimSpace(args.ID) == "" {
			return "", nil, errors.New("id is required")
		}
		return protocol.MSkillsGet, protocol.SkillParams{ID: strings.TrimSpace(args.ID), File: args.File}, nil
	}
	return "", nil, fmt.Errorf("no skill tool %s", name)
}

// SkillAnswer is what a model is told of a skill tool's result: the skills
// found, or the file read with where it lies and the skill's other files.
// raw is the method's result as JSON.
func SkillAnswer(method string, raw json.RawMessage) string {
	if method == protocol.MSkillsSearch {
		var result protocol.SkillsSearchResult
		if json.Unmarshal(raw, &result) != nil || len(result.Skills) == 0 {
			return "No skill matched."
		}
		var b strings.Builder
		for _, skill := range result.Skills {
			fmt.Fprintf(&b, "- %s: %s", skill.ID, skill.Description)
			if skill.Manual {
				b.WriteString(" (only when the user names it)")
			}
			fmt.Fprintf(&b, "\n  %s\n", skill.Path)
		}
		return strings.TrimRight(b.String(), "\n")
	}
	var content protocol.SkillContent
	if json.Unmarshal(raw, &content) != nil {
		return "The skill could not be read."
	}
	var b strings.Builder
	fmt.Fprintf(&b, "[%s of %s, at %s", content.File, content.ID, content.Path)
	if len(content.Files) > 0 {
		fmt.Fprintf(&b, "; its other files: %s", strings.Join(content.Files, ", "))
	}
	b.WriteString("]\n\n")
	b.WriteString(content.Text)
	return b.String()
}
