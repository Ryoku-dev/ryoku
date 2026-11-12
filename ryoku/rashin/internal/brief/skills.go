package brief

import (
	"fmt"
	"strings"

	"ryoku-rashin/internal/protocol"
)

// setNotes are Rashin's own words about a set a model has to hear before it
// uses one of its skills.
var setNotes = map[string]string{
	"speckit": "after `rashin skills setup speckit [dir]` in the project",
}

// Skills is what a model is told about the built-in skills it may use: each
// set that is on with its skills' names, where they lie and how to use them,
// short enough for every prompt. tools says the model holds the skills and
// skill tools; one without them is told the rashin command. Empty when no
// set is on.
func Skills(result protocol.SkillsResult, tools bool) string {
	var b strings.Builder
	for _, set := range result.Sets {
		if !set.On || len(set.Skills) == 0 {
			continue
		}
		b.WriteString("- " + set.Name + " (" + set.ID + ")")
		var notes []string
		for _, need := range set.Needs {
			if !need.Present {
				note := "needs " + need.Command + " (missing"
				if need.Install != "" {
					note += ": " + need.Install
				}
				notes = append(notes, note+")")
			}
		}
		if allManual(set.Skills) {
			notes = append(notes, "only when the user names one")
		}
		if note := setNotes[set.ID]; note != "" {
			notes = append(notes, note)
		}
		if len(notes) > 0 {
			b.WriteString(", " + strings.Join(notes, "; "))
		}
		b.WriteString(": " + skillNames(set.Skills) + "\n")
	}
	if b.Len() == 0 {
		return ""
	}
	list := b.String()
	b.Reset()
	fmt.Fprintf(&b, "Built-in skills, each at %s/<set>/skills/<name>/SKILL.md:\n", result.Root)
	b.WriteString(list)
	if tools {
		fmt.Fprintf(&b, "Before a task, find the skill that fits with the %s tool, read it with the %s tool or open its file, and follow it.", ToolSkills, ToolSkill)
	} else {
		b.WriteString("Before a task, find the skill that fits with `rashin skills search \"<the task>\"`, read it with `rashin skills read <set>/<name>` or open its file, and follow it.")
	}
	return b.String()
}

// allManual reports a set whose every skill the user starts by name.
func allManual(skills []protocol.Skill) bool {
	for _, skill := range skills {
		if !skill.Manual {
			return false
		}
	}
	return true
}

// skillNames lists the skills by name, a prefix every name shares written
// once as prefix-{a, b}, and a skill the user starts by name marked so
// where its set mixes both.
func skillNames(skills []protocol.Skill) string {
	prefix := sharedPrefix(skills)
	mixed := !allManual(skills)
	names := make([]string, len(skills))
	for i, skill := range skills {
		names[i] = strings.TrimPrefix(skill.Name, prefix)
		if mixed && skill.Manual {
			names[i] += " (only when the user names it)"
		}
	}
	if prefix == "" {
		return strings.Join(names, ", ")
	}
	return prefix + "{" + strings.Join(names, ", ") + "}"
}

// sharedPrefix is the leading word and dash every skill's name starts with,
// or "" when they do not share one or there is only one skill.
func sharedPrefix(skills []protocol.Skill) string {
	if len(skills) < 2 {
		return ""
	}
	first, _, found := strings.Cut(skills[0].Name, "-")
	if !found {
		return ""
	}
	prefix := first + "-"
	for _, skill := range skills {
		if !strings.HasPrefix(skill.Name, prefix) || skill.Name == prefix {
			return ""
		}
	}
	return prefix
}
