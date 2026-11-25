package protocol

import "time"

// What the self-improvement features keep, as the Learning page shows it:
// managed skills (FeatureAutolearn), decision banks (FeatureDecisions), rules
// (FeatureRules) and grievances (FeatureGrievances). Listing and deleting work
// whether a feature is on or off, so what it kept stays visible and removable
// after the user turns it off; learning.rule_forge needs FeatureRules and is
// CodeDisabled without it. Everything lies under Rashin's own directories and
// never leaves this computer.

// ManagedSkillSet is the Skill.Set of a managed skill, so skills.get reads one
// as "managed/<name>".
const ManagedSkillSet = "managed"

// ManagedSkill is one skill Rashin wrote itself (manage_skill, learn or
// auto-learn capture). Shadowed says a built-in or authored skill claims the
// same name, so this one never reaches a model.
type ManagedSkill struct {
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Path        string    `json:"path"`
	Bytes       int64     `json:"bytes"`
	Updated     time.Time `json:"updated"`
	Shadowed    bool      `json:"shadowed,omitempty"`
}

// LearningSkillsResult lists managed skills by name. Root is their folder.
type LearningSkillsResult struct {
	Skills []ManagedSkill `json:"skills"`
	Root   string         `json:"root"`
}

// LearningSkillParams names one managed skill.
type LearningSkillParams struct {
	Name string `json:"name"`
}

// Decision areas: the three files of a decision bank.
const (
	DecisionArchitecture = "architecture"
	DecisionProduct      = "product"
	DecisionStyle        = "style"
)

// DecisionAreas are the areas in the order a bank shows and injects them.
var DecisionAreas = []string{DecisionArchitecture, DecisionProduct, DecisionStyle}

// DecisionBank is one project's decision memory. Project is the folder it was
// learned in; Bank is its directory name (basename plus a FNV-1a hash of the
// path). Architecture, Product and Style are the files' text, at most 120
// lines each. Queued counts extracted decisions that wait for the next
// consolidation.
type DecisionBank struct {
	Project      string     `json:"project"`
	Bank         string     `json:"bank"`
	Architecture string     `json:"architecture"`
	Product      string     `json:"product"`
	Style        string     `json:"style"`
	Queued       int        `json:"queued,omitempty"`
	Consolidated *time.Time `json:"consolidated,omitempty"`
	Error        string     `json:"error,omitempty"`
}

// LearningDecisionsParams picks one project's bank by its folder; empty lists
// every bank.
type LearningDecisionsParams struct {
	Project string `json:"project,omitempty"`
}

// LearningDecisionsResult lists banks, most recently consolidated first.
type LearningDecisionsResult struct {
	Banks []DecisionBank `json:"banks"`
}

// DecisionDeleteParams removes one decision: the first line of the Area file
// of Project's bank that equals Line once both are trimmed. A line that is no
// longer there (a consolidation rewrote it) is CodeNotFound.
type DecisionDeleteParams struct {
	Project string `json:"project"`
	Area    string `json:"area"`
	Line    string `json:"line"`
}

// Rule interrupt modes: Rule.Interrupt.
const (
	RuleInterruptNever     = "never"
	RuleInterruptProseOnly = "prose-only"
	RuleInterruptToolOnly  = "tool-only"
	RuleInterruptAlways    = "always"
)

// Rule repeat modes: Rule.Repeat.
const (
	RuleRepeatOnce     = "once"
	RuleRepeatAfterGap = "after-gap"
	RuleRepeatAlways   = "always"
)

// Rule is one rule file: Markdown with front matter. Name is the file name
// without .md. Project is the folder of a project rule, empty for a global
// one. Condition are regular expressions (JavaScript flavour, matched while a
// reply streams); Question makes a judged rule instead, checked after the
// reply; a rule with neither and AlwaysApply joins every system prompt.
// Scope says what a condition watches ("text", "thinking", "tool",
// "tool:edit(*.go)"; empty is text and every tool). Globs limit the rule to
// matching files. Forged marks a rule the forge wrote. Error says why a file
// on disk does not parse; such a rule is never applied.
type Rule struct {
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Project     string    `json:"project,omitempty"`
	Path        string    `json:"path,omitempty"`
	Body        string    `json:"body"`
	Condition   []string  `json:"condition,omitempty"`
	Question    string    `json:"question,omitempty"`
	Scope       string    `json:"scope,omitempty"`
	Globs       []string  `json:"globs,omitempty"`
	AlwaysApply bool      `json:"alwaysApply,omitempty"`
	Interrupt   string    `json:"interrupt,omitempty"`
	Repeat      string    `json:"repeat,omitempty"`
	Enabled     bool      `json:"enabled"`
	Forged      bool      `json:"forged,omitempty"`
	Updated     time.Time `json:"updated,omitzero"`
	Error       string    `json:"error,omitempty"`
}

// LearningRulesParams lists the global rules and, with Project, that
// project's rules too.
type LearningRulesParams struct {
	Project string `json:"project,omitempty"`
}

// LearningRulesResult lists rules by name, project rules first.
type LearningRulesResult struct {
	Rules []Rule `json:"rules"`
}

// RuleSaveParams writes a rule file. A rule of the same name and place is
// replaced only with Overwrite; otherwise it is CodeConflict. A condition
// that does not compile is CodeBadRequest with the reason.
type RuleSaveParams struct {
	Rule      Rule `json:"rule"`
	Overwrite bool `json:"overwrite,omitempty"`
}

// RuleRef names one rule: Project empty for a global rule.
type RuleRef struct {
	Name    string `json:"name"`
	Project string `json:"project,omitempty"`
}

// RuleForgeParams asks the session's own model for a rule that stops what
// Complaint describes, checked against the session's transcript. Feedback
// and Previous amend an earlier proposal. Nothing is saved: the app shows the
// proposal and saves it with learning.rule_save.
type RuleForgeParams struct {
	Session   string `json:"session"`
	Complaint string `json:"complaint"`
	Feedback  string `json:"feedback,omitempty"`
	Previous  *Rule  `json:"previous,omitempty"`
}

// RuleForgeResult is the proposed rule. OK is false when no attempt gave a
// rule that compiles and matches the session; Message then says why. Matched
// is the excerpt of the transcript the rule caught.
type RuleForgeResult struct {
	OK       bool   `json:"ok"`
	Message  string `json:"message,omitempty"`
	Rule     Rule   `json:"rule"`
	Matched  string `json:"matched,omitempty"`
	Attempts int    `json:"attempts"`
}

// Grievance is one report a model made with report_issue about a tool that
// did not behave as described. Source is "builtin" or "mcp:<server>". Paths
// and names in Report have the user's home and name replaced.
type Grievance struct {
	ID      int64     `json:"id"`
	Tool    string    `json:"tool"`
	Report  string    `json:"report"`
	Source  string    `json:"source,omitempty"`
	Model   string    `json:"model,omitempty"`
	Session string    `json:"session,omitempty"`
	At      time.Time `json:"at"`
}

// LearningGrievancesParams filters grievances by tool; Limit 0 lets the
// daemon choose a page.
type LearningGrievancesParams struct {
	Tool  string `json:"tool,omitempty"`
	Limit int    `json:"limit,omitempty"`
}

// LearningGrievancesResult is newest first. Total counts every stored one and
// Tools counts them per tool, most first.
type LearningGrievancesResult struct {
	Grievances []Grievance `json:"grievances"`
	Total      int         `json:"total"`
	Tools      []Count     `json:"tools,omitempty"`
}

// GrievancesClearParams deletes grievances: those in IDs, else every one of
// Tool, else all of them.
type GrievancesClearParams struct {
	IDs  []int64 `json:"ids,omitempty"`
	Tool string  `json:"tool,omitempty"`
}

// What learning.changed says changed: LearningChanged.Kind.
const (
	LearningSkills     = "skills"
	LearningDecisions  = "decisions"
	LearningRules      = "rules"
	LearningGrievances = "grievances"
)

// LearningChanged is the data of learning.changed: which part changed, and
// for decisions and rules the project folder when it was one project's.
// Clients read the part again.
type LearningChanged struct {
	Kind    string `json:"kind"`
	Project string `json:"project,omitempty"`
}
