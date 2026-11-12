package skills

import "ryoku-rashin/internal/protocol"

// set is what Rashin says about one built-in set: who made it, where it
// comes from and what it relies on. Its skills are read from the tree.
type set struct {
	id, name, by, url, license, commit, summary string
	needs                                       []need
}

// need is a command a set relies on and how to get it.
type need struct {
	command, install string
}

// catalog are the built-in sets, in the order they are shown. Each one's
// files lie under sets/<id>, unchanged from the pinned commit.
var catalog = []set{
	{
		id: "superpowers", name: "Superpowers", by: "Jesse Vincent", url: "https://github.com/obra/superpowers",
		license: "MIT", commit: "22f0b463e7a5",
		summary: "Habits for coding work: brainstorm before building, plan, write the failing test first, debug by method and verify before calling anything done.",
	},
	{
		id: "openspec", name: "OpenSpec", by: "Fission AI", url: "https://github.com/Fission-AI/OpenSpec",
		license: "MIT", commit: "d12c28177767",
		summary: "Spec-driven changes: propose a change with its specs, design and tasks, carry it out, verify it and archive it.",
		needs:   []need{{command: "openspec", install: "npm install -g @fission-ai/openspec"}},
	},
	{
		id: "speckit", name: "Spec Kit", by: "GitHub", url: "https://github.com/github/spec-kit",
		license: "MIT", commit: "979b0a3c1277",
		summary: "Spec-driven development: a project's principles, then a specification, a plan and tasks before the code is written.",
		needs:   []need{{command: "bash"}, {command: "git"}},
	},
	{
		id: "egm", name: "Elephant and Goldfish", by: "Google", url: "https://github.com/google/elephant-goldfish-model",
		license: "Apache-2.0", commit: "84145300b9e0",
		summary: "Design documents written with a reviewer who remembers everything and reviewers who see each draft fresh, then kept in step with the code.",
	},
}

// info is the set as the protocol shows it, without its skills.
func (s set) info() protocol.SkillSet {
	return protocol.SkillSet{ID: s.id, Name: s.name, By: s.by, URL: s.url, License: s.license, Commit: s.commit, Summary: s.summary}
}
