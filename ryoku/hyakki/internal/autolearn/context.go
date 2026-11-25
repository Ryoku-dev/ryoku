// Package autolearn is the procedural self-improvement loop (feature
// autolearn): the skills block of the system prompt, which lists the
// built-in sets that are on and the managed skills, and the TurnObserver
// that, after a turn with many tool calls, asks a light model whether the
// turn taught a procedure worth keeping and writes it as a managed skill.
package autolearn

// Derived from oh-my-pi (MIT), packages/coding-agent/src/extensibility/skill-descriptions.ts@c7fff67

import (
	"context"
	"slices"
	"strings"

	"ryoku-rashin/internal/engine"
	"ryoku-rashin/internal/engine/tool"
	"ryoku-rashin/internal/protocol"
	"ryoku-rashin/internal/skills"
)

// Budgets of the skills block, in bytes: the compact profile shares about
// 1.2 KB among every provider's blocks, so it gets names only.
const (
	fullBudget    = 8192
	compactBudget = 640
	hintWords     = 12
)

// SkillsContext is the engine.ContextProvider for skills.
type SkillsContext struct {
	library  *skills.Library
	settings func() protocol.Settings
	seatOff  func(team, member string) []string
}

// NewSkillsContext lists library's sets and its attached managed skills
// under settings; seatOff adds the team's and current seat's exclusions.
func NewSkillsContext(library *skills.Library, settings func() protocol.Settings, seatOff func(team, member string) []string) *SkillsContext {
	return &SkillsContext{library: library, settings: settings, seatOff: seatOff}
}

// Blocks is the skills block: nil when no skill is on or the profile has no
// tool that reads one. It is rebuilt from the files each turn and is the
// same text while they do not change.
func (c *SkillsContext) Blocks(_ context.Context, tc engine.TurnContext) []engine.Block {
	if c == nil || c.library == nil || c.settings == nil {
		return nil
	}
	var extra []string
	if tc.Session.Team != "" && c.seatOff != nil {
		extra = c.seatOff(tc.Session.Team, tc.Session.Member)
	}
	off := skills.Off(c.settings(), extra...)
	listing, err := c.library.List(off)
	if err != nil {
		listing = protocol.SkillsResult{}
	}
	var managed []protocol.ManagedSkill
	managedRoot := ""
	if m := c.library.Managed(); m != nil && !slices.Contains(off, protocol.ManagedSkillSet) {
		managedRoot = m.Dir()
		for _, set := range listing.Sets {
			if set.ID != protocol.ManagedSkillSet || !set.On {
				continue
			}
			for _, skill := range set.Skills {
				managed = append(managed, protocol.ManagedSkill{Name: skill.Name, Description: skill.Description, Path: skill.Path})
			}
		}
	}
	how := reading(tc.Profile, listing.Root, managedRoot)
	if how == "" {
		return nil
	}
	var text string
	budget := fullBudget
	if tc.Profile.Compact() {
		// a small model treats a list of procedures as a list of things to
		// open, whatever the task: Ask gets none, an agent only what it learned
		// in this user's own work
		if tc.Profile.Mode == tool.ModeAskRead || tc.Profile.Mode == tool.ModeAskAct || len(managed) == 0 {
			return nil
		}
		text, budget = compactText(how, protocol.SkillsResult{}, managed), compactBudget
	} else {
		text = fullText(how, listing, managed)
	}
	if text == "" {
		return nil
	}
	return []engine.Block{{Name: engine.BlockSkills, Text: text, Budget: budget}}
}

// reading is the line that says how this profile reads a skill, "" when it
// has no tool for it.
func reading(p tool.Profile, root, managedRoot string) string {
	if tool.Offers(p.Mode, p.Class, tool.Skill) {
		return "Read one with the " + tool.Skill + " tool by its id: <set>/<name>, managed/<name> for a learned one."
	}
	read := ""
	for _, name := range []string{tool.Read, tool.ReadFile} {
		if tool.Offers(p.Mode, p.Class, name) {
			read = name
			break
		}
	}
	if read == "" {
		return ""
	}
	paths := root + "/<set>/skills/<name>/SKILL.md"
	if root == "" {
		paths = ""
	}
	if managedRoot != "" {
		if paths != "" {
			paths += ", learned ones "
		}
		paths += managedRoot + "/<name>/SKILL.md"
	}
	return "Open its SKILL.md with " + read + ": " + paths + "."
}

// fullText lists every skill with a short routing hint, sets in catalog
// order and learned skills last.
func fullText(how string, listing protocol.SkillsResult, managed []protocol.ManagedSkill) string {
	var b strings.Builder
	for _, set := range listing.Sets {
		if set.ID == protocol.ManagedSkillSet || !set.On || len(set.Skills) == 0 {
			continue
		}
		b.WriteString(set.Name + " (" + set.ID + ")" + needs(set) + ":\n")
		for _, s := range set.Skills {
			b.WriteString("- " + s.Name)
			if h := hint(s.Description); h != "" {
				b.WriteString(": " + h)
			}
			if s.Manual {
				b.WriteString(" (only when the user names it)")
			}
			b.WriteByte('\n')
		}
	}
	if len(managed) > 0 {
		b.WriteString("Learned from earlier work (managed):\n")
		for _, s := range managed {
			b.WriteString("- " + s.Name + ": " + hint(s.Description) + "\n")
		}
	}
	if b.Len() == 0 {
		return ""
	}
	return "Skills are written procedures. When one fits the task, read it first and follow it. " + how + "\n" + strings.TrimRight(b.String(), "\n")
}

// compactText is names only, learned skills first, taking whole names
// within the compact budget.
func compactText(how string, listing protocol.SkillsResult, managed []protocol.ManagedSkill) string {
	var groups []struct {
		id    string
		names []string
	}
	if len(managed) > 0 {
		names := make([]string, len(managed))
		for i, s := range managed {
			names[i] = s.Name
		}
		groups = append(groups, struct {
			id    string
			names []string
		}{"managed", names})
	}
	for _, set := range listing.Sets {
		if set.ID == protocol.ManagedSkillSet || !set.On || len(set.Skills) == 0 {
			continue
		}
		names := make([]string, 0, len(set.Skills))
		for _, s := range set.Skills {
			if !s.Manual {
				names = append(names, s.Name)
			}
		}
		if len(names) > 0 {
			groups = append(groups, struct {
				id    string
				names []string
			}{set.ID, names})
		}
	}
	if len(groups) == 0 {
		return ""
	}
	text := "Skills you learned in earlier work. Read one only when its name matches the task. " + how
	for _, group := range groups {
		line := group.id + ": " + grouped(group.names)
		if len(text)+1+len(line) <= compactBudget {
			text += "\n" + line
			continue
		}
		line = group.id + ": "
		for _, name := range group.names {
			separator := ""
			if line != group.id+": " {
				separator = ", "
			}
			if len(text)+1+len(line)+len(separator)+len(name) > compactBudget {
				break
			}
			line += separator + name
		}
		if line != group.id+": " {
			text += "\n" + line
		}
	}
	return text
}

// needs says which commands a set relies on are missing here.
func needs(set protocol.SkillSet) string {
	var missing []string
	for _, n := range set.Needs {
		if n.Present {
			continue
		}
		note := n.Command + " (missing"
		if n.Install != "" {
			note += ": " + n.Install
		}
		missing = append(missing, note+")")
	}
	if len(missing) == 0 {
		return ""
	}
	return ", needs " + strings.Join(missing, ", ")
}

// grouped writes names that share a hyphenated prefix once as
// prefix-{a, b}.
func grouped(names []string) string {
	if len(names) < 2 {
		return strings.Join(names, ", ")
	}
	prefix := names[0]
	for _, n := range names[1:] {
		for !strings.HasPrefix(n, prefix) {
			prefix = prefix[:len(prefix)-1]
		}
	}
	cut := strings.LastIndexByte(prefix, '-')
	if cut <= 0 {
		return strings.Join(names, ", ")
	}
	prefix = prefix[:cut+1]
	rest := make([]string, len(names))
	for i, n := range names {
		rest[i] = strings.TrimPrefix(n, prefix)
		if rest[i] == "" {
			return strings.Join(names, ", ")
		}
	}
	return prefix + "{" + strings.Join(rest, ", ") + "}"
}

// hint is a description as a routing hint of at most twelve words: its
// first sentence when that says enough, cut at the word limit. Deterministic,
// so the block stays byte-stable.
func hint(description string) string {
	text := strings.Join(strings.Fields(description), " ")
	if end := sentenceEnd(text); end > 0 && len(strings.Fields(text[:end])) >= 4 {
		text = text[:end]
	}
	words := strings.Fields(text)
	if len(words) > hintWords {
		words = words[:hintWords]
	}
	return strings.TrimRight(strings.Join(words, " "), ".,;:!?-")
}

// sentenceEnd is the index just past the first sentence's full stop, or 0.
func sentenceEnd(text string) int {
	for i := range len(text) - 1 {
		switch text[i] {
		case '.', '!', '?':
			if text[i+1] == ' ' {
				return i + 1
			}
		}
	}
	return 0
}
