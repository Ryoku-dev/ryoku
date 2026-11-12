package protocol

// SkillSet is one set of skills Rashin ships built in: where it comes from,
// under which licence, what it relies on, and whether it is on. Sets are
// vendored unchanged at a pinned commit.
type SkillSet struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	By      string `json:"by"`
	URL     string `json:"url"`
	License string `json:"license"`
	Commit  string `json:"commit"`
	// Summary says in one sentence what the set is for.
	Summary string      `json:"summary"`
	Needs   []SkillNeed `json:"needs,omitempty"`
	Skills  []Skill     `json:"skills"`
	// On is false when the user turned the set off for every model.
	On bool `json:"on"`
}

// SkillNeed is a command a set relies on, whether this computer has it, and
// how to get it.
type SkillNeed struct {
	Command string `json:"command"`
	Present bool   `json:"present"`
	Install string `json:"install,omitempty"`
}

// Skill is one skill: its id ("<set>/<name>"), what it says it is for, and
// where its SKILL.md lies on this computer. Manual marks a skill its authors
// mean to be started only when the user asks for it by name.
type Skill struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Set         string `json:"set"`
	Description string `json:"description"`
	Path        string `json:"path"`
	Manual      bool   `json:"manual,omitempty"`
}

// SkillsResult lists every built-in set. Root is the folder the skills lie
// in, readable by Ask, agents and team sandboxes alike.
type SkillsResult struct {
	Sets []SkillSet `json:"sets"`
	Root string     `json:"root"`
}

// SkillParams names a skill and, optionally, one of its files by a path
// relative to the skill's folder; empty File reads SKILL.md.
type SkillParams struct {
	ID   string `json:"id"`
	File string `json:"file,omitempty"`
}

// SkillContent is a skill's file: its text, where it lies, and the other
// files of the skill a reader may open next.
type SkillContent struct {
	ID    string   `json:"id"`
	File  string   `json:"file"`
	Path  string   `json:"path"`
	Text  string   `json:"text"`
	Files []string `json:"files,omitempty"`
}

// SkillsSearchParams finds skills that fit what is about to be done.
type SkillsSearchParams struct {
	Query string `json:"query"`
	Limit int    `json:"limit,omitempty"`
}

// SkillsSearchResult is best first.
type SkillsSearchResult struct {
	Skills []Skill `json:"skills"`
}

// SkillsSettings say which built-in sets are off for every model; every set
// is on unless named here. Only the Rashin app may change them.
type SkillsSettings struct {
	Off []string `json:"off,omitempty"`
}
