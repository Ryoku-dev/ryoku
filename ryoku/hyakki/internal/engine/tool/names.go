package tool

// Canonical tool names. The model sees exactly these; prompts, tests, the
// app's memory line and the team gate name them through these constants.
// The comment on each group names the unit (and package) that implements it.
const (
	// Files, internal/engine/tools/fs (FsTools).
	Read      = "read"
	Write     = "write"
	Edit      = "edit"
	Ls        = "ls"
	FindFiles = "find_files"
	Grep      = "grep"
	// Ask's file tools, kept under their names so the Ask prompt and its
	// tests stay valid (FsTools).
	ReadFile  = "read_file"
	ListDir   = "list_dir"
	WriteFile = "write_file"

	// Commands, internal/engine/tools/shell (ShellTools). Wait reads and
	// stops background jobs that bash started.
	Bash        = "bash"
	Wait        = "wait"
	SystemQuery = "system_query"
	RunCommand  = "run_command"

	// Code index and Git, internal/engine/tools/code (InfoTools).
	Search     = "search"
	Find       = "find"
	Def        = "def"
	Outline    = "outline"
	References = "references"
	Impact     = "impact"
	Peek       = "peek"
	GitStatus  = "git_status"
	GitDiff    = "git_diff"
	GitLog     = "git_log"
	GitShow    = "git_show"
	// SearchCode is Ask's project search (InfoTools).
	SearchCode = "search_code"

	// Web, internal/engine/tools/web (InfoTools).
	FetchURL = "fetch_url"

	// Ask only, internal/engine/tools/ask (InfoTools).
	Propose      = "propose"
	Doctor       = "doctor"
	Processes    = "processes"
	ReadWiki     = "read_wiki"
	SearchSystem = "search_system"

	// Subagents, todos, questions and goals (Subagents):
	// internal/engine/tools/{agents,todo,askuser,goal}.
	Task  = "task"
	Yield = "yield"
	Todo  = "todo"
	Ask   = "ask"
	Goal  = "goal"

	// Context notes kept through compaction, and a fresh window seeded from
	// them (EngineContext, internal/engine/notes.go).
	ContextNotes = "context_notes"
	NewContext   = "new_context"

	// Self-improvement: managed skills (SelfLearn,
	// internal/engine/tools/learn, feature autolearn) and grievances (Rules,
	// internal/engine/tools/report, feature grievances).
	Learn       = "learn"
	ManageSkill = "manage_skill"
	ReportIssue = "report_issue"

	// Rashin tools over tool.Host, internal/engine/tools/rashin
	// (RashinTools). Memory tools need ToolContext.Memory (SessionSearch
	// also MemorySessions).
	MemorySearch   = "memory_search"
	Remember       = "remember"
	Forget         = "forget"
	SessionSearch  = "session_search"
	Skills         = "skills"
	Skill          = "skill"
	TeamsTemplates = "teams_templates"
	TeamsList      = "teams_list"
	ProposeTeam    = "propose_team"

	// Screen tools, ToolContext.Screen (RashinTools).
	ScreenLook    = "screen_look"
	ScreenClick   = "screen_click"
	ScreenMove    = "screen_move"
	ScreenDrag    = "screen_drag"
	ScreenScroll  = "screen_scroll"
	ScreenType    = "screen_type"
	ScreenKey     = "screen_key"
	ScreenOpen    = "screen_open"
	ScreenAskUser = "screen_ask_user"
	ScreenStop    = "screen_stop"

	// Team task tools, ToolContext.Team (RashinTools). Delegate requires
	// TeamToolContext.Delegate; status and governance are available to every seat.
	Report            = "report"
	AskUser           = "ask_user"
	Credentials       = "credentials"
	HTTPRequest       = "http_request"
	SendMail          = "send_mail"
	RequestCredential = "request_credential"
	RequestSetup      = "request_setup"
	RequestHands      = "request_hands"
	Delegate          = "delegate"
	TeamStatus        = "team_status"
	ProposePlan       = "propose_plan"
	RequestHelp       = "request_help"
	Escalate          = "escalate"

	// Dream lead tools, TeamToolContext.Dream (RashinTools).
	Hire        = "hire"
	Release     = "release"
	DreamStatus = "dream_status"
	Present     = "present"
)

var (
	codeRead   = []string{Search, Find, Def, Outline, References, Impact, Peek}
	gitRead    = []string{GitStatus, GitDiff, GitLog, GitShow}
	memoryAll  = []string{MemorySearch, Remember, Forget, SessionSearch}
	skillsRead = []string{Skills, Skill}
	teamsAsk   = []string{TeamsTemplates, TeamsList, ProposeTeam}
	screenAll  = []string{ScreenLook, ScreenClick, ScreenMove, ScreenDrag, ScreenScroll, ScreenType, ScreenKey, ScreenOpen, ScreenAskUser, ScreenStop}
	teamTask   = []string{Report, AskUser, Credentials, HTTPRequest, SendMail, RequestCredential, RequestSetup, RequestHands, Delegate, TeamStatus, ProposePlan, RequestHelp, Escalate}
	dreamLead  = []string{Hire, Release, DreamStatus, Present}

	askBase = []string{SystemQuery, Processes, ReadFile, ListDir, SearchSystem, SearchCode, ReadWiki, FetchURL, Doctor, Propose}

	agentFull = join(
		[]string{Read, Write, Edit, Ls, FindFiles, Grep, Bash, Wait, Todo, Task, Ask, Goal, ContextNotes, NewContext},
		codeRead, gitRead,
		[]string{FetchURL, Learn, ManageSkill, ReportIssue},
		memoryAll, skillsRead, teamsAsk, screenAll,
	)
	teamFull = join(
		[]string{Read, Write, Edit, Ls, FindFiles, Grep, Bash, Wait, Todo, Task, Goal, ContextNotes, NewContext},
		codeRead, gitRead,
		[]string{FetchURL, Learn, ManageSkill, ReportIssue},
		memoryAll, skillsRead, teamsAsk, teamTask, screenAll,
	)
)

// sets is the canonical table: the names each mode offers per class, in the
// order the model sees them (fixed, so the tool block stays byte-stable for
// the llama.cpp prompt cache). A name is offered only when a registered tool
// has it and that tool is enabled for the session (Conditional), and, in a
// ReadOnly profile, only when the tool is ReadOnly.
var sets = map[Mode]map[Class][]string{
	ModeAskRead: {
		ClassFull:    join(askBase, teamsAsk, memoryAll, skillsRead),
		ClassCompact: {SystemQuery, Processes, ReadFile, SearchSystem, FetchURL, Doctor, Propose, Remember},
	},
	ModeAskAct: {
		ClassFull:    join(askBase, []string{RunCommand, WriteFile}, teamsAsk, memoryAll, skillsRead),
		ClassCompact: {SystemQuery, Processes, ReadFile, SearchSystem, Propose, RunCommand, WriteFile},
	},
	ModeAgent: {
		ClassFull:    agentFull,
		ClassCompact: {Read, Write, Edit, Bash, Grep, FindFiles, Todo, Learn},
	},
	ModePlan: {
		ClassFull: join(
			[]string{Read, Ls, FindFiles, Grep, Todo, Task, Ask, ContextNotes, NewContext},
			codeRead, gitRead,
			[]string{FetchURL, Learn, ReportIssue, MemorySearch, Remember, SessionSearch},
			skillsRead, []string{TeamsTemplates, TeamsList},
		),
		ClassCompact: {Read, Ls, FindFiles, Grep, Todo, Learn},
	},
	ModeTeam: {
		ClassFull:    teamFull,
		ClassCompact: {Bash, Read, Report, Delegate, TeamStatus, ProposePlan, RequestHelp, Escalate},
	},
	ModeDreamManager: {
		ClassFull:    join(teamFull, dreamLead),
		ClassCompact: {Delegate, TeamStatus, Hire, Release, DreamStatus, Present, ProposePlan, Escalate},
	},
	ModeSubagent: {
		ClassFull: join(
			[]string{Read, Ls, FindFiles, Grep, Bash, Wait, Write, Edit},
			codeRead, gitRead,
			[]string{FetchURL, MemorySearch}, skillsRead,
			[]string{Yield},
		),
		ClassCompact: {Read, FindFiles, Grep, Bash, Write, Edit, Yield},
	},
}

// Names returns the canonical tool names of a mode and class, in order; nil
// for an unknown mode. An unknown class reads as full.
func Names(mode Mode, class Class) []string {
	byClass, ok := sets[mode]
	if !ok {
		return nil
	}
	names, ok := byClass[class]
	if !ok {
		names = byClass[ClassFull]
	}
	return append([]string(nil), names...)
}

// Offers reports whether mode and class name tool.
func Offers(mode Mode, class Class, name string) bool {
	for _, n := range Names(mode, class) {
		if n == name {
			return true
		}
	}
	return false
}

// ProvidersJoin reports whether a full standalone Agent or Plan session takes
// tools from the user's MCP servers. Team and dream tasks run inside bwrap.
func ProvidersJoin(mode Mode, class Class) bool {
	if class == ClassCompact {
		return false
	}
	switch mode {
	case ModeAgent, ModePlan:
		return true
	}
	return false
}

func join(groups ...[]string) []string {
	var out []string
	for _, g := range groups {
		out = append(out, g...)
	}
	return out
}
