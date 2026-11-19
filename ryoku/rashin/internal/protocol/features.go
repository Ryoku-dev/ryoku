package protocol

// Self-improvement features. Each is off until the user turns it on, runs
// offline, and keeps everything under Rashin's own directories, where the
// Learning page shows it and removes it.
const (
	// FeatureAutolearn is managed skills: the manage_skill and learn tools
	// and auto-learn capture after a turn with many tool calls.
	FeatureAutolearn = "autolearn"
	// FeatureDecisions is project decision memory: friction extracted from
	// prompts, consolidated per project into architecture, product and style
	// banks that join the system prompt.
	FeatureDecisions = "decisions"
	// FeatureRules is rule files, stream rules that interrupt a reply, and
	// the "That was wrong, make a rule" forge.
	FeatureRules = "rules"
	// FeatureGrievances is the report_issue tool: the model's own reports of
	// tools that misbehaved, kept on this computer and never uploaded.
	FeatureGrievances = "grievances"
	// FeatureMCP is the user's own MCP tool servers, whose tools join every
	// engine session's registry.
	FeatureMCP = "mcp"
)

// FeatureIDs lists the optional features stored in Settings.Features.
var FeatureIDs = []string{
	"compare",
	"research",
	"schedules",
	"prompts",
	"enhance",
	"askwriting",
	"analyze",
	"audio",
	"video",
	"share",
	"quote",
	"history",
	"searchsteps",
	"mindmap",
	"artifacts",
	"collections",
	"profile",
	"setupcards",
	FeatureMCP,
	"diskcare",
	FeatureAutolearn,
	FeatureDecisions,
	FeatureRules,
	FeatureGrievances,
}
