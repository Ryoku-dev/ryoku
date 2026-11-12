package ask

import (
	"encoding/json"
	"errors"

	i18n "ryoku-i18n"
	"ryoku-rashin/internal/brief"
	"ryoku-rashin/internal/llm"
	"ryoku-rashin/internal/protocol"
	"ryoku-rashin/internal/skills"
)

// SetSkills hands Ask the built-in skill sets; nil leaves them out.
func (f *Factory) SetSkills(library *skills.Library) { f.skills = library }

// skillToolset are Ask's tools over the built-in skills, offered in every
// mode since they only read.
var skillToolset = func() []llm.Tool {
	tools := brief.SkillTools()
	out := make([]llm.Tool, 0, len(tools))
	for _, tool := range tools {
		parameters, _ := json.Marshal(tool.Schema)
		out = append(out, llm.Tool{Name: tool.Name, Description: tool.Description, Parameters: parameters})
	}
	return out
}()

// skillsSection is the system prompt's part on the built-in skills and the
// tools that reach them; empty with no skills or none on.
func (f *Factory) skillsSection(settings protocol.Settings) (string, []llm.Tool) {
	if f.skills == nil {
		return "", nil
	}
	result, err := f.skills.List(settings.Skills.Off)
	if err != nil {
		return "", nil
	}
	text := brief.Skills(result, true)
	if text == "" {
		return "", nil
	}
	return "# Skills\n\n" + text, skillToolset
}

// toldLimit is how much of a tool's output the model hears: all of a skill,
// which is followed as a whole, and the start of anything else.
func toldLimit(tool string) int {
	if brief.IsSkillTool(tool) {
		return 32000
	}
	return 6000
}

// runSkillTool reads the built-in skills for one skill tool call within the
// sets that are on.
func (f *Factory) runSkillTool(call llm.ToolCall, settings protocol.Settings) (string, error) {
	if f.skills == nil {
		return "", errors.New(i18n.T("Skills are not available."))
	}
	method, params, err := brief.SkillParams(call.Name, json.RawMessage(call.Arguments))
	if err != nil {
		return "", err
	}
	var result any
	switch p := params.(type) {
	case protocol.SkillsSearchParams:
		result, err = f.skills.Search(p, settings.Skills.Off)
	case protocol.SkillParams:
		result, err = f.skills.Get(p, settings.Skills.Off)
	}
	if err != nil {
		return "", err
	}
	raw, err := json.Marshal(result)
	if err != nil {
		return "", err
	}
	return brief.SkillAnswer(method, raw), nil
}

// skillToolPresentation is how a skill tool's call reads in the transcript;
// ok is false for any other tool.
func skillToolPresentation(call llm.ToolCall) (title, kind string, ok bool) {
	switch call.Name {
	case brief.ToolSkills:
		return i18n.T("Searching skills"), protocol.ToolSearch, true
	case brief.ToolSkill:
		return i18n.T("Reading a skill"), protocol.ToolRead, true
	}
	return "", "", false
}
