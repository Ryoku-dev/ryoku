package protocol

import "reflect"

// Method names.
const (
	MHello     = "hello"
	MStatus    = "status"
	MSubscribe = "subscribe"
	MShutdown  = "shutdown"

	MSessionList       = "session.list"
	MSessionNew        = "session.new"
	MSessionAttach     = "session.attach"
	MSessionDetach     = "session.detach"
	MSessionPrompt     = "session.prompt"
	MSessionCancel     = "session.cancel"
	MSessionPermission = "session.permission"
	MSessionSetModel   = "session.set_model"
	MSessionSetMode    = "session.set_mode"
	MSessionSetOption  = "session.set_option"
	MSessionRename     = "session.rename"
	MSessionDelete     = "session.delete"
	MSessionStop       = "session.stop"
	MSessionHandoff    = "session.handoff"
	MSessionQueue      = "session.queue"
	MSessionUnqueue    = "session.unqueue"
	MSessionSteer      = "session.steer"
	MSessionFork       = "session.fork"
	MSessionRead       = "session.read"
	MSessionRevertTurn = "session.revert_turn"
	MSessionGoal       = "session.goal"
	MAsk               = "ask"
	MGlance            = "glance"

	MScreenList    = "screen.list"
	MScreenAct     = "screen.act"
	MScreenControl = "screen.control"

	MProvidersList      = "providers.list"
	MProvidersAddKey    = "providers.add_key"
	MProvidersRemoveKey = "providers.remove_key"
	MProvidersTest      = "providers.test"
	MProvidersEnable    = "providers.enable"
	MProvidersLimits    = "providers.limits"
	MModelsList         = "models.list"
	MModelsProbe        = "models.probe"
	MGatewayUsage       = "gateway.usage"
	MFastDecide         = "fastdecisions.evaluate"
	MFastDecideLocal    = "fastdecisions.local"
	MGatewayAPI         = "gateway.api"
	MGatewayAPIKey      = "gateway.api_key"

	MLocalModels         = "local.models"
	MLocalFound          = "local.found"
	MLocalUseFound       = "local.use_found"
	MLocalDevices        = "local.devices"
	MLocalDownload       = "local.download"
	MLocalCancel         = "local.cancel"
	MLocalRemove         = "local.remove"
	MLocalSearch         = "local.search"
	MLocalAdd            = "local.add"
	MLocalRuntimeInstall = "local.runtime_install"
	MLocalStart          = "local.start"
	MLocalStop           = "local.stop"

	MAccountStatus  = "account.status"
	MAccountSignIn  = "account.signin"
	MAccountCancel  = "account.cancel"
	MAccountRefresh = "account.refresh"
	MAccountSignOut = "account.signout"

	MLearningSkills          = "learning.skills"
	MLearningSkillDelete     = "learning.skill_delete"
	MLearningDecisions       = "learning.decisions"
	MLearningDecisionDelete  = "learning.decision_delete"
	MLearningRules           = "learning.rules"
	MLearningRuleSave        = "learning.rule_save"
	MLearningRuleDelete      = "learning.rule_delete"
	MLearningRuleForge       = "learning.rule_forge"
	MLearningGrievances      = "learning.grievances"
	MLearningGrievancesClear = "learning.grievances_clear"

	MIndexList       = "index.list"
	MIndexAdd        = "index.add"
	MIndexRemove     = "index.remove"
	MIndexRefresh    = "index.refresh"
	MIndexStatus     = "index.status"
	MIndexSearch     = "index.search"
	MIndexFind       = "index.find"
	MIndexDef        = "index.def"
	MIndexOutline    = "index.outline"
	MIndexReferences = "index.references"
	MIndexPeek       = "index.peek"
	MIndexOverview   = "index.overview"
	MIndexImpact     = "index.impact"
	MIndexWip        = "index.wip"
	MIndexChanged    = "index.changed"

	MSettingsGet = "settings.get"
	MSettingsSet = "settings.set"

	MUIOpen = "ui.open"

	MOniStatus          = "oni.status"
	MOniCharacters      = "oni.characters"
	MOniCharacterSelect = "oni.character.select"
	MOniCharacterReset  = "oni.character.reset"
	MOniSpec            = "oni.spec"
	MOniOpen            = "oni.open"
	MOniClose           = "oni.close"
	MOniPlace           = "oni.place"
	MOniInteract        = "oni.interact"
	MOniCharacterAssets = "oni.character.assets"
	MOniStrandArrived   = "oni.strandArrived"
	MOniPerch           = "oni.perch"
	MOniDetach          = "oni.detach"
	MOniInspect         = "oni.inspect"
	MOniAsk             = "oni.ask"
	MOniConsentAnswer   = "oni.consent.answer"
	MOniOverlay         = "oni.overlay"
	// The voice calls: hold-to-talk, stopping speech, dictation and the
	// Settings mic meter.
	MOniTalk      = "oni.talk"
	MOniSpeak     = "oni.speak"
	MOniDictate   = "oni.dictate"
	MOniVoiceTest = "oni.voice.test"
	// Guides, Keep and select-and-ask.
	MOniGuide = "oni.guide"
	MOniKeep  = "oni.keep"
	MOniSkip  = "oni.skip"
	MOniPick  = "oni.pick"
	// Reflex: the reminders and timers kept, the routines kept and the
	// reflex ledger.
	MOniReminders = "oni.reminders"
	MOniRoutines  = "oni.routines"
	MOniReflexLog = "oni.reflex.log"
	// Hands in one authorized browser tab: the Hands browser, its tabs, the
	// task, and the Hands ledger.
	MOniHands    = "oni.hands"
	MOniHandsLog = "oni.hands.log"
	// Rewrite in place, recall with its chat-export import, the focus guard
	// and assignment mode with its ledger.
	MOniRewrite       = "oni.rewrite"
	MOniRecall        = "oni.recall"
	MOniRecallImport  = "oni.recall.import"
	MOniFocus         = "oni.focus"
	MOniAssignment    = "oni.assignment"
	MOniAssignmentLog = "oni.assignment.log"
	// Oni's local activity with its delete-all, a line on Oni's pill from
	// the MCP bridge, and the bridge's state.
	MOniActivity      = "oni.activity"
	MOniActivityClear = "oni.activity.clear"
	MOniNotify        = "oni.notify"
	MOniMCP           = "oni.mcp"

	// Optional parts: the models, packs, profiles, data and programs Hyakki
	// may use, with their sizes, and removing one Hyakki downloaded.
	MComponents      = "components.list"
	MComponentRemove = "components.remove"

	MProjectsList     = "projects.list"
	MProjectsAdd      = "projects.add"
	MProjectsRemove   = "projects.remove"
	MProjectsSettings = "projects.settings"
	MProjectsWatch    = "projects.watch"

	MWorktreesList          = "worktrees.list"
	MWorktreesCreate        = "worktrees.create"
	MWorktreesPreviewRemove = "worktrees.preview_remove"
	MWorktreesRemove        = "worktrees.remove"
	MWorktreesLock          = "worktrees.lock"
	MWorktreesMove          = "worktrees.move"
	MWorktreesRepair        = "worktrees.repair"
	MWorktreesPrune         = "worktrees.prune"
	MWorktreesCleanup       = "worktrees.cleanup"
	MWorktreesSetup         = "worktrees.setup"
	MWorktreesTrust         = "worktrees.trust"
	MWorktreesPorts         = "worktrees.ports"

	MGitStatus       = "git.status"
	MGitLog          = "git.log"
	MGitShow         = "git.show"
	MGitDiff         = "git.diff"
	MGitStage        = "git.stage"
	MGitUnstage      = "git.unstage"
	MGitDiscard      = "git.discard"
	MGitCommit       = "git.commit"
	MGitPush         = "git.push"
	MGitFetch        = "git.fetch"
	MGitPull         = "git.pull"
	MGitBranchCreate = "git.branch_create"
	MGitBranchDelete = "git.branch_delete"
	MGitCheckout     = "git.checkout"
	MGitMerge        = "git.merge"
	MGitCherryPick   = "git.cherry_pick"
	MGitRevert       = "git.revert"
	MGitRebase       = "git.rebase"
	MGitAbort        = "git.abort"
	MGitContinue     = "git.continue"

	MReviewList    = "review.list"
	MReviewAdd     = "review.add"
	MReviewResolve = "review.resolve"
	MReviewDelete  = "review.delete"
	MReviewSend    = "review.send"

	MTasksList        = "tasks.list"
	MTasksCreate      = "tasks.create"
	MTasksUpdate      = "tasks.update"
	MTasksDelete      = "tasks.delete"
	MTasksStart       = "tasks.start"
	MTasksGate        = "tasks.gate"
	MTasksResolveGate = "tasks.resolve_gate"
	MTasksPlan        = "tasks.plan"

	MActivityList = "activity.list"

	MTeamsList                = "teams.list"
	MTeamsTemplates           = "teams.templates"
	MTeamsStructures          = "teams.structures"
	MTeamsStructureDraft      = "teams.structure_draft"
	MTeamsStructureSuggest    = "teams.structure_suggest"
	MTeamsStructureSave       = "teams.structure_save"
	MTeamsStructureClone      = "teams.structure_clone"
	MTeamsStructureDelete     = "teams.structure_delete"
	MTeamsStructureRestore    = "teams.structure_restore"
	MTeamsCreate              = "teams.create"
	MTeamsPropose             = "teams.propose"
	MTeamsUpdate              = "teams.update"
	MTeamsMemberConfigGet     = "teams.member_config_get"
	MTeamsMemberConfigSave    = "teams.member_config_save"
	MTeamsMemberConfigHistory = "teams.member_config_history"
	MTeamsMemberConfigRestore = "teams.member_config_restore"
	MTeamsDelete              = "teams.delete"
	MTeamsPause               = "teams.pause"
	MTeamsExport              = "teams.export"
	MTeamsImport              = "teams.import"
	MTeamsTasks               = "teams.tasks"
	MTeamsTaskCreate          = "teams.task_create"
	MTeamsTaskStop            = "teams.task_stop"
	MTeamsTaskRetry           = "teams.task_retry"
	MTeamsTaskRead            = "teams.task_read"
	MTeamsTaskDelete          = "teams.task_delete"
	MTeamsRoutines            = "teams.routines"
	MTeamsRoutineCreate       = "teams.routine_create"
	MTeamsRoutineUpdate       = "teams.routine_update"
	MTeamsRoutineDelete       = "teams.routine_delete"
	MTeamsRoutineRun          = "teams.routine_run"
	MTeamsRoutinePreview      = "teams.routine_preview"
	MTeamsApprovals           = "teams.approvals"
	MTeamsApprovalDecide      = "teams.approval_decide"
	MTeamsActivity            = "teams.activity"
	MTeamsSpend               = "teams.spend"
	MTeamsPrune               = "teams.prune"
	MTeamsDreamCreate         = "teams.dream_create"
	MTeamsDreamAnswer         = "teams.dream_answer"

	MTeamsAgentReport      = "teams.agent_report"
	MTeamsAgentDelegate    = "teams.agent_delegate"
	MTeamsAgentStatus      = "teams.agent_status"
	MTeamsAgentAsk         = "teams.agent_ask"
	MTeamsAgentHTTP        = "teams.agent_http"
	MTeamsAgentMail        = "teams.agent_mail"
	MTeamsAgentRequest     = "teams.agent_request"
	MTeamsAgentCredentials = "teams.agent_credentials"
	MTeamsAgentHire        = "teams.agent_hire"
	MTeamsAgentRelease     = "teams.agent_release"
	MTeamsAgentDream       = "teams.agent_dream"
	MTeamsAgentPresent     = "teams.agent_present"

	MVaultInfo     = "vault.info"
	MVaultAdd      = "vault.add"
	MVaultUpdate   = "vault.update"
	MVaultRemove   = "vault.remove"
	MVaultEvents   = "vault.events"
	MVaultMail     = "vault.mail"
	MVaultMailTest = "vault.mail_test"

	MSessionSetMemory = "session.set_memory"

	MMemoryStatus      = "memory.status"
	MMemoryList        = "memory.list"
	MMemoryGet         = "memory.get"
	MMemoryAdd         = "memory.add"
	MMemoryEdit        = "memory.edit"
	MMemoryForget      = "memory.forget"
	MMemoryRestore     = "memory.restore"
	MMemoryKeep        = "memory.keep"
	MMemoryDismiss     = "memory.dismiss"
	MMemoryUndo        = "memory.undo"
	MMemoryRedo        = "memory.redo"
	MMemoryChanges     = "memory.changes"
	MMemoryForgetAll   = "memory.forget_all"
	MMemorySearch      = "memory.search"
	MMemorySessions    = "memory.sessions"
	MMemoryCard        = "memory.card"
	MMemoryImport      = "memory.import"
	MMemorySuppress    = "memory.suppress"
	MMemoryLearn       = "memory.learn"
	MMemoryEngineTest  = "memory.engine_test"
	MMemoryEngineSetup = "memory.engine_setup"
	MMemoryEngineMove  = "memory.engine_move"

	MSkillsList   = "skills.list"
	MSkillsGet    = "skills.get"
	MSkillsSearch = "skills.search"

	MWritingCheck       = "writing.check"
	MWritingTransform   = "writing.transform"
	MWritingCancel      = "writing.cancel"
	MWritingVoiceGet    = "writing.voice.get"
	MWritingVoiceSet    = "writing.voice.set"
	MWritingVoiceLearn  = "writing.voice.learn"
	MWritingWords       = "writing.words"
	MWritingWordsAdd    = "writing.words.add"
	MWritingWordsRemove = "writing.words.remove"
	MPromptsList        = "prompts.list"
	MPromptsSave        = "prompts.save"
	MPromptsDelete      = "prompts.delete"
	MPromptsExpand      = "prompts.expand"
	MCompareStart       = "compare.start"
	MCompareKeep        = "compare.keep"
	MSessionSetFlags    = "session.set_flags"
	MMCPList            = "mcp.list"
	MMCPSave            = "mcp.save"
	MMCPDelete          = "mcp.delete"
	MMCPCheck           = "mcp.check"
	MMCPSignIn          = "mcp.signin"
	MDiskUsage          = "disk.usage"
	MDiskClean          = "disk.clean"
	MArtifactOpen       = "artifact.open"
	MArtifactErrors     = "artifact.errors"
	MArtifactRepair     = "artifact.repair"
	MArtifactClose      = "artifact.close"
	MCollectionsList    = "collections.list"
	MCollectionsSave    = "collections.save"
	MCollectionsAdd     = "collections.add"
	MCollectionsItems   = "collections.items"
	MCollectionsRemove  = "collections.remove"
	MCollectionsCheck   = "collections.check"
	MCollectionsChat    = "collections.chat"
	MTeamsRoutineRuns   = "teams.routine_runs"
	MTeamsAgentSetup    = "teams.agent_setup"
)

// Event names.
const (
	EvtSessionEvent         = "session.event"            // SessionEventData
	EvtSessionInfo          = "session.info"             // SessionInfo
	EvtSessionsChanged      = "sessions.changed"         // SessionsChanged
	EvtSettingsChanged      = "settings.changed"         // Settings
	EvtProvidersChanged     = "providers.changed"        // ProvidersResult
	EvtLocalProgress        = "local.progress"           // LocalProgress
	EvtLocalChanged         = "local.changed"            // LocalResult
	EvtAccountChanged       = "account.changed"          // AccountState, app clients only
	EvtLearningChanged      = "learning.changed"         // LearningChanged
	EvtIndexProgress        = "index.progress"           // IndexProgress
	EvtIndexChanged         = "index.changed"            // IndexInfo
	EvtUIFocus              = "ui.focus"                 // UIOpenParams, app clients only
	EvtNotice               = "notice"                   // Notice
	EvtProjectsChanged      = "projects.changed"         // ProjectsResult
	EvtWorktreesChanged     = "worktrees.changed"        // WorktreesResult
	EvtTasksChanged         = "tasks.changed"            // TasksResult (one project's tasks)
	EvtGitChanged           = "git.changed"              // GitChanged, watchers of the project
	EvtReviewChanged        = "review.changed"           // ReviewResult
	EvtAPIChanged           = "api.changed"              // APIInfo
	EvtScreen               = "screen"                   // ScreenState
	EvtTeam                 = "team"                     // Team
	ETeamsStructuresChanged = "teams.structures_changed" // Empty invalidation
	EvtTeamRemoved          = "team.removed"             // TeamRemoved
	EvtTeamTask             = "team.task"                // TeamTask
	EvtTeamTaskRemoved      = "team.task.removed"        // TeamRemoved
	EvtApproval             = "team.approval"            // Approval
	EvtRoutine              = "routine"                  // Routine
	EvtRoutineRemoved       = "routine.removed"          // TeamRemoved
	EvtVault                = "vault"                    // VaultInfo
	EvtMemoryChanged        = "memory.changed"           // MemoryChanged
	EvtMemorySetup          = "memory.setup"             // MemorySetupProgress
	EvtOni                  = "oni"                      // an oni frame, to the connections that opened it only
)

// Method is one entry of the registry: its name, the Go types of its params and
// result, and what it does.
type Method struct {
	Name   string
	Params reflect.Type
	Result reflect.Type
	Doc    string
}

func m(name string, params, result any, doc string) Method {
	return Method{Name: name, Params: reflect.TypeOf(params), Result: reflect.TypeOf(result), Doc: doc}
}

// Methods is the complete registry. The daemon dispatches only these names.
var Methods = []Method{
	m(MHello, HelloParams{}, HelloResult{}, "Open a connection; must be the first request."),
	m(MStatus, Empty{}, Status{}, "Daemon state at a glance."),
	m(MSubscribe, Empty{}, Snapshot{}, "Snapshot, then global events on this connection."),
	m(MShutdown, Empty{}, Empty{}, "Cancel turns, stop agents and exit."),

	m(MSessionList, SessionListParams{}, SessionListResult{}, "Sessions, newest first."),
	m(MSessionNew, SessionNewParams{}, SessionInfo{}, "Create a session."),
	m(MSessionAttach, SessionAttachParams{}, SessionAttachResult{}, "Replay events after since, then stream session.event."),
	m(MSessionDetach, SessionRef{}, Empty{}, "Stop streaming a session to this connection."),
	m(MSessionPrompt, PromptParams{}, PromptResult{}, "Start a turn."),
	m(MSessionCancel, SessionRef{}, Empty{}, "Cancel the running turn."),
	m(MSessionPermission, PermissionParams{}, Empty{}, "Answer a permission request."),
	m(MSessionSetModel, SetModelParams{}, Empty{}, "Pick the session's model."),
	m(MSessionSetMode, SetModeParams{}, Empty{}, "Pick the session's mode: what it may do without asking."),
	m(MSessionSetOption, SetOptionParams{}, Empty{}, "Set one option the session's model offers, or Plan first."),
	m(MSessionRename, RenameParams{}, SessionInfo{}, "Rename a session."),
	m(MSessionDelete, SessionRef{}, Empty{}, "Delete a session and its log."),
	m(MSessionStop, SessionRef{}, Empty{}, "Stop what runs for the session (its tool host, background terminals); the session stays resumable."),
	m(MSessionHandoff, HandoffParams{}, SessionInfo{}, "Move an Ask session's work to a new Agent session."),
	m(MSessionQueue, QueueParams{}, SessionInfo{}, "Queue a follow-up sent when the running turn ends."),
	m(MSessionUnqueue, UnqueueParams{}, SessionInfo{}, "Remove a queued follow-up."),
	m(MSessionSteer, PromptParams{}, PromptResult{}, "Cancel the running turn and prompt at once."),
	m(MSessionFork, ForkParams{}, SessionInfo{}, "Copy a session up to an event into a new session."),
	m(MSessionRead, SessionRef{}, Empty{}, "Mark a session's finished turns as seen."),
	m(MSessionGoal, GoalParams{}, SessionInfo{}, "Set, pause, resume, end or report an Agent session's goal."),

	m(MScreenList, Empty{}, ScreensResult{}, "The agent screens that are up."),
	m(MScreenAct, ScreenActParams{}, ScreenActResult{}, "Act on an Agent session's screen, starting it once the user agrees."),
	m(MScreenControl, ScreenControlParams{}, ScreenState{}, "Take, hand back, watch or stop a session's screen, or have the agent ask the user."),
	m(MSessionRevertTurn, TurnParams{}, GitOpResult{}, "Restore the worktree files to the checkpoint taken before a turn."),
	m(MAsk, AskParams{}, AskResult{}, "Create or continue an Ask session, attach and prompt."),
	m(MGlance, GlanceParams{}, AskResult{}, "Create or continue a temporary capture conversation, attach and prompt."),

	m(MProvidersList, Empty{}, ProvidersResult{}, "Catalogue providers and their keys."),
	m(MProvidersAddKey, AddKeyParams{}, ProviderInfo{}, "Store a provider key."),
	m(MProvidersRemoveKey, RemoveKeyParams{}, ProviderInfo{}, "Remove a provider key."),
	m(MProvidersTest, ProviderParams{}, ProviderTestResult{}, "Run a one-token completion through a provider."),
	m(MProvidersEnable, EnableProviderParams{}, ProviderInfo{}, "Turn a provider on or off for routing."),
	m(MProvidersLimits, ProviderParams{}, Empty{}, "Read the providers' account limits again where they are older than ten minutes; the result arrives as providers.changed."),
	m(MModelsList, ModelsParams{}, ModelsResult{}, "Chat models, their tier and the routes."),
	m(MModelsProbe, ModelProbeParams{}, ModelProbeResult{}, "Check that a model answers and calls tools with the options given. The Rashin app's alone."),
	m(MFastDecide, FastDecisionParams{}, FastDecisionResult{}, "Evaluate typed choices without action authority. The Hyakki app's alone; explicit hosted use spends finite Ember credits."),
	m(MFastDecideLocal, Empty{}, DecisionLocalStatus{}, "Look once for the local decision server at the configured loopback address, behind the hardware gate. The Hyakki app's alone."),
	m(MGatewayUsage, UsageParams{}, GatewayUsage{}, "Requests and tokens per day, provider and model."),
	m(MGatewayAPI, Empty{}, APIInfo{}, "The local API: whether it is on, where it listens."),
	m(MGatewayAPIKey, APIKeyParams{}, APIKey{}, "The key other programs present to the local API; rotate replaces it."),

	m(MLocalModels, Empty{}, LocalResult{}, "Rashin's own models, their catalogue and the llama.cpp runtime; starts nothing."),
	m(MLocalFound, Empty{}, LocalFoundResult{}, "Search existing GGUF models only when the Rashin app requests it."),
	m(MLocalUseFound, LocalUseFoundParams{}, LocalModel{}, "Use an existing GGUF in place, without copying or deleting the user's file."),
	m(MLocalDevices, Empty{}, LocalDevicesResult{}, "Probe devices on explicit request, cached until the llama.cpp runtime binary changes. The Rashin app's alone."),
	m(MLocalDownload, LocalModelParams{}, Empty{}, "Download a model or resume its download; progress arrives as local.progress. The Rashin app's alone."),
	m(MLocalCancel, LocalModelParams{}, Empty{}, "Pause a download, keeping what it has. The Rashin app's alone."),
	m(MLocalRemove, LocalModelParams{}, LocalResult{}, "Delete managed model files or forget an in-place model without deleting its source. The Rashin app's alone."),
	m(MLocalSearch, LocalSearchParams{}, LocalSearchResult{}, "A Hugging Face repo's GGUF files at its current revision."),
	m(MLocalAdd, LocalAddParams{}, LocalModel{}, "Add a model from Hugging Face, pinned to its revision, and start its download. The Rashin app's alone."),
	m(MLocalRuntimeInstall, LocalRuntimeParams{}, Empty{}, "Download Rashin's own llama-server; progress arrives as local.progress. The Rashin app's alone."),
	m(MLocalStart, LocalModelParams{}, LocalRuntime{}, "Start llama-server, loading a model when one is named. The Rashin app's alone."),
	m(MLocalStop, LocalModelParams{}, LocalRuntime{}, "Stop llama-server. The Rashin app's alone."),

	m(MAccountStatus, Empty{}, AccountState{}, "The Ryoku Account as last read; reads nothing new. The Rashin app's alone."),
	m(MAccountSignIn, Empty{}, AccountState{}, "Start signing in with a code the user approves in the browser; the outcome arrives as account.changed. The Rashin app's alone."),
	m(MAccountCancel, Empty{}, AccountState{}, "Stop a sign-in that is waiting for approval. The Rashin app's alone."),
	m(MAccountRefresh, Empty{}, AccountState{}, "Read the account and its balance from the server again. The Rashin app's alone."),
	m(MAccountSignOut, Empty{}, AccountState{}, "Forget the account's tokens on this computer. The Rashin app's alone."),

	m(MLearningSkills, Empty{}, LearningSkillsResult{}, "The skills Rashin wrote itself."),
	m(MLearningSkillDelete, LearningSkillParams{}, LearningSkillsResult{}, "Delete a skill Rashin wrote. The Rashin app's alone."),
	m(MLearningDecisions, LearningDecisionsParams{}, LearningDecisionsResult{}, "Decision memory per project: architecture, product and style."),
	m(MLearningDecisionDelete, DecisionDeleteParams{}, DecisionBank{}, "Delete one decision from a project's memory. The Rashin app's alone."),
	m(MLearningRules, LearningRulesParams{}, LearningRulesResult{}, "Rules, global and for a project."),
	m(MLearningRuleSave, RuleSaveParams{}, Rule{}, "Write a rule file. The Rashin app's alone."),
	m(MLearningRuleDelete, RuleRef{}, Empty{}, "Delete a rule file. The Rashin app's alone."),
	m(MLearningRuleForge, RuleForgeParams{}, RuleForgeResult{}, "Propose a rule from a complaint about a conversation; nothing is saved. The Rashin app's alone."),
	m(MLearningGrievances, LearningGrievancesParams{}, LearningGrievancesResult{}, "What models reported about tools that misbehaved, newest first."),
	m(MLearningGrievancesClear, GrievancesClearParams{}, Empty{}, "Delete reports about tools. The Rashin app's alone."),

	m(MIndexList, Empty{}, IndexesResult{}, "Indexes."),
	m(MIndexAdd, IndexAddParams{}, IndexInfo{}, "Register and index a project folder."),
	m(MIndexRemove, IndexRef{}, Empty{}, "Forget an index and delete its data."),
	m(MIndexRefresh, IndexRef{}, IndexInfo{}, "Bring an index up to date."),
	m(MIndexStatus, IndexRef{}, IndexInfo{}, "One index's state."),
	m(MIndexSearch, SearchParams{}, SearchResult{}, "Hybrid lexical and semantic search."),
	m(MIndexFind, NameParams{}, SymbolsResult{}, "Find symbols by name."),
	m(MIndexDef, NameParams{}, DefResult{}, "A symbol with its source."),
	m(MIndexOutline, PathParams{}, SymbolsResult{}, "A file's symbols."),
	m(MIndexReferences, NameParams{}, ReferencesResult{}, "Uses of a name."),
	m(MIndexPeek, PeekParams{}, PeekResult{}, "Read a line range."),
	m(MIndexOverview, IndexRef{}, Overview{}, "Map an index."),
	m(MIndexImpact, PathParams{}, Impact{}, "Files that depend on a path."),
	m(MIndexWip, WipParams{}, WipResult{}, "Uncommitted changes with their dependents."),
	m(MIndexChanged, WipParams{}, WipResult{}, "Changes against a base ref with their dependents."),

	m(MSettingsGet, Empty{}, Settings{}, "Current settings."),
	m(MSettingsSet, SettingsSetParams{}, Settings{}, "Apply a JSON merge patch to the settings."),

	m(MUIOpen, UIOpenParams{}, UIOpenResult{}, "Ask a running app to show a session or tab."),

	m(MOniStatus, Empty{}, OniStatus{}, "Oni's switches, the character in use, every feature's decision under the settings and the characters with their load reports; starts nothing."),
	m(MOniCharacters, Empty{}, OniCharactersResult{}, "The shipped character first, then the user's, each with its fallbacks or the errors that reject it."),
	m(MOniCharacterSelect, OniCharacterSelectParams{}, OniStatus{}, "Make a valid character current; an invalid one is refused and the current one stays. Changes presentation only, never capabilities. The Hyakki app's alone."),
	m(MOniCharacterReset, Empty{}, OniStatus{}, "Make the shipped oni current again. The Hyakki app's alone."),
	m(MOniSpec, Empty{}, OniSpecResult{}, "The motion spec as shipped, the feature contract, the frame kinds and the calls later phases serve."),
	m(MOniOpen, OniOpenParams{}, OniOpenResult{}, "Open the oni event on this connection: a native surface becomes the one presenter, an observer only watches. Answers with the state, character, motion, placement and voice frames."),
	m(MOniClose, Empty{}, Empty{}, "Stop the oni event on this connection; a presenter lets go of Oni."),
	m(MOniPlace, OniPlaceParams{}, Empty{}, "Save where Oni stands: a corner, or fractions of an output. Not from an agent's tools."),
	m(MOniInteract, OniInteractParams{}, Empty{}, "Report a poke, wake, drag or drop; any of them wakes a sleeping Oni. Not from an agent's tools."),
	m(MOniCharacterAssets, OniCharacterAssetsParams{}, OniCharacterAssets{}, "A valid character's profile.json and declared SVGs, as validated at load."),
	m(MOniStrandArrived, OniStrandArrivedParams{}, Empty{}, "The presenter's report that a strand target landed or was skipped. Only the connection drawing Oni may send it; a stale or repeated seq advances nothing. Not from an agent's tools."),
	m(MOniPerch, OniPerchParams{}, OniPerchResult{}, "Perch Oni on a window: one the seam names, or, from the presenter's drop, the one under a point on an output. Hover only hit-tests. Refused with a reason when the window is gone, cannot be placed or is off screen. The app, the CLI and the presenter only; never an agent's tools."),
	m(MOniDetach, OniDetachParams{}, Empty{}, "Let go of the perched window: the strand rewinds and a detached perch frame follows. The app, the CLI and the presenter only."),
	m(MOniInspect, Empty{}, OniAskResult{}, "Look at the perched window and describe it. Captures only that window; the route and the exact picture are shown before anything is sent. The app and the CLI only."),
	m(MOniAsk, OniAskParams{}, OniAskResult{}, "Ask about the perched window: Oni points at the answer or says it. Captures only that window; the route and the exact picture are shown before anything is sent. The app and the CLI only."),
	m(MOniConsentAnswer, OniConsentAnswerParams{}, OniAskResult{}, "Answer the question oni.ask or oni.inspect returned: yes sends the picture shown and returns the answer, no clears it. The asking connection or the app only."),
	m(MOniOverlay, OniOverlayParams{}, Empty{}, "The presenter's own rects on an output, so a capture of what lies under them masks Oni out. The presenter only."),
	m(MOniTalk, OniTalkParams{}, OniVoiceResult{}, "Start, stop or cancel one hold-to-talk press: start opens the microphone, stop transcribes what was heard and answers it, cancel drops it. The app, the CLI and the presenter only."),
	m(MOniSpeak, OniSpeakParams{}, OniVoiceResult{}, "Stop what Oni is saying, fading it out. The app, the CLI and the presenter only."),
	m(MOniDictate, OniDictateParams{}, OniDictateResult{}, "Start, stop or cancel one dictation into the window focused at start. The text is typed only while that window still holds focus, a chunk at a time; what was not typed comes back only in rest. The app, the CLI and the presenter only."),
	m(MOniVoiceTest, OniVoiceTestParams{}, OniVoiceTestResult{}, "Measure the microphone for Settings' meter: its levels over a short listen and the sources the chooser offers. Records nothing to disk. The app and the CLI only."),
	m(MOniGuide, OniGuideParams{}, OniAskResult{}, "Start a guide toward a goal on the perched window, or move it on: next, stop, or throw the ring round the step again. Each step's picture is asked about as oni.ask's is; a kept guide replays on this computer with nothing sent. Returns the step pointed at, the consent question, or the closing line with any offer to keep it. The app and the CLI; the presenter next, stop and circle only."),
	m(MOniKeep, OniKeepParams{}, OniKeepResult{}, "Keep the finished guide, or the run of quick commands, the offer names, on this computer, so it replays with no model. The app and the CLI only."),
	m(MOniSkip, OniKeepParams{}, Empty{}, "Let the keep or routine offer go; nothing is written. The app, the CLI and the presenter."),
	m(MOniPick, OniPickParams{}, OniAskResult{}, "Select-and-ask: arm the presenter's pick overlay with an optional question, report its one click, or cancel. A private field is refused before any model call; the picture is asked about as oni.ask's is. The app and the CLI; the presenter point and cancel only."),
	m(MOniReminders, OniRemindersParams{}, OniRemindersResult{}, "List the reminders and timers kept on this computer, soonest first, add one, or cancel one or every one. Each carries its own words and times, never what was said; noWake says the daemon stays up while one is pending because the wake unit cannot be armed. Needs reflex.reminders. The app and the CLI only; the MCP bridge lists with reminders_list and adds with reminders_add turned on."),
	m(MOniRoutines, OniRoutinesParams{}, OniRoutinesResult{}, "List the routines kept from quick commands, replay one (the most recently used when no name is given) through the same actions and limits with no model asked, or forget one or every one. Needs reflex.routines. The app and the CLI only."),
	m(MOniReflexLog, Empty{}, OniReflexLogResult{}, "The quick-command ledger, newest first: each action's intent, decision route, time, confidence and outcome, never the words said. Needs reflex. The app and the CLI only."),
	m(MOniHands, OniHandsParams{}, OniHandsResult{}, "Hands at its one stage: in one browser tab (open the Hands browser on its own profile, list its tabs, authorize one), in one live window the person picked (list windows, select one), or on Hyakki's own nested desktop; start a task toward a goal there, stop it, resume a paused one, report where it stands, list or clear the task journal, or forget the Hands browser's profile. Safe steps run; anything risky is handed to the person, who presses it themselves. Never the person's whole live desktop. Needs hands.browser, hands.app or hands.desktop. Every action from the app; open, tabs, stop, status and tasks from the CLI; authorize, start, stop and resume from the presenter; select, clear and forget from the app only."),
	m(MOniHandsLog, Empty{}, OniHandsLogResult{}, "The Hands ledger, newest first: each step's tool, outcome, rule code and time, never a control name, page text or typed word. The app and the CLI only."),
	m(MOniFocus, OniFocusParams{}, OniFocusResult{}, "The focus guard: arm it for some minutes, at most the spec's focus.maxMinutes, or until an app's windows close, with feed words (the Settings ones when none are given); stop it; lift it (back); or report it (status). While armed it hears window events only: a window whose app id names a feed is closed after a card naming the feed and the app, where the Ryoku seam can close it, and announced otherwise; a window naming a feed only in its title is announced, never closed. Needs feeds.guard and window events. Every action from the app and the CLI; stop from the presenter."),
	m(MOniRewrite, OniRewriteParams{}, OniRewriteResult{}, "Rewrite the text selected in the focused window in place: start reads the PRIMARY selection (never the clipboard) and shows it, its length and the route on a card; apply, or a yes to the card, sends it and types the checked result back over the selection while that window keeps focus. A result of more than one paragraph is not typed but offered to copy; one that says the same is not typed. Undo presses Ctrl+Z and reads the window back, offering the original to copy when it cannot see it return. Terminals, Hyakki's own windows and private windows are refused. The app and the CLI only."),
	m(MOniRecall, OniRecallParams{}, OniRecallResult{}, "Answer a question from what the user kept (ask): imported chats, past conversations, memory facts and, only with recall.vault on, the Ryoku vault, read as their indexes are and fenced as data. A route on this computer sends nothing off it; a hosted route first shows a card with the route and the passages' titles, answered through oni.consent.answer. Or say what memory holds about the user or a topic (know), masked, with no model. Needs recall. The app and the CLI only."),
	m(MOniRecallImport, OniRecallImportParams{}, OniRecallImportResult{}, "Import the ChatGPT, Claude or Gemini export files the user chose, .json or .zip, read where they are: each chat is kept as Markdown, secrets scrubbed and card, identity and account numbers masked, under one index root recall reads. An archive naming a file outside itself or holding an oversized member is refused whole. Nothing watches a folder. Needs recall. The app and the CLI only."),
	m(MOniAssignment, OniAssignmentParams{}, OniAssignmentResult{}, "Assignment mode on the perched window: start looks at it and solves a new problem after a card showing the picture and route (a view solved before comes back with no request); next and back move through the steps and the views solved, next past the last step looks again, and an answer typed into the same problem is not a new one; done and stop end it; status reports it. With no pin the smallest local model that reads pictures answers, never a hosted or paid one; a hosted pin stops at the day's attempt and token caps, and unmetered hosted usage stops it until the next day. Needs assignment and looking at windows. Every action from the app and the CLI; next, back, done and stop from the presenter."),
	m(MOniAssignmentLog, Empty{}, OniAssignmentLogResult{}, "Today's assignment spend and ledger, newest first: each lane's route, measured tokens with their estimated or unmetered flags, and outcome, never a picture, problem, answer or step. The app and the CLI only."),
	m(MOniActivity, Empty{}, OniActivityResult{}, "Oni's local activity: how much each store holds and the newest ledger rows across them, at most fifty, with routes, timings and measured tokens flagged estimated or unmetered, never a transcript, selection, passage, picture or title. Reads the stores as they are; starts nothing. The app, the CLI and the Hub."),
	m(MOniActivityClear, OniActivityClearParams{}, OniActivityResult{}, "Delete the named activity stores, or all of them, each through its own owner: the ledgers, kept walkthroughs, routines, reminders and imported chats. Memory facts are never touched here. The Hyakki app's alone."),
	m(MOniNotify, OniNotifyParams{}, Empty{}, "Show one line of at most 120 characters on Oni's pill, with the name of the program that sent it; a few a minute at most, and refused while Oni is off, hidden or asleep. The MCP bridge with notify turned on, and the app."),
	m(MOniMCP, Empty{}, OniMCPStatus{}, "The MCP bridge: its switch, the tools turned on and the programs approved or waiting; a program asking through the bridge sees its own entry alone. Changes nothing: the switch, the tools and the approvals are the Hyakki app's settings alone. The app, the CLI and the bridge."),
	m(MComponents, Empty{}, ComponentsResult{}, "The optional parts Hyakki may use: downloaded speech models, voice packs, the Hands browser profile, imported chats, caches and the programs the distribution provides, each with its state, size, where it lies and what uses it. Downloads, starts and probes nothing. The app, the CLI and the Hub."),
	m(MComponentRemove, ComponentRemoveParams{}, ComponentsResult{}, "Delete one part Hyakki downloaded or made, inside Hyakki's own folders only; refused while it is in use and for anything a distribution or Ryoku installed. The Hyakki app's alone."),

	m(MProjectsList, Empty{}, ProjectsResult{}, "Projects, most recently active first."),
	m(MProjectsAdd, ProjectAddParams{}, ProjectInfo{}, "Register a folder as a project."),
	m(MProjectsRemove, ProjectRef{}, Empty{}, "Forget a project; no file is touched."),
	m(MProjectsSettings, ProjectSettingsParams{}, ProjectInfo{}, "Apply a JSON merge patch to a project's settings."),
	m(MProjectsWatch, WatchParams{}, Empty{}, "Start or stop git.changed events for a project on this connection."),

	m(MWorktreesList, ProjectRef{}, WorktreesResult{}, "A project's worktrees with status."),
	m(MWorktreesCreate, WorktreeCreateParams{}, WorktreeInfo{}, "Create a worktree."),
	m(MWorktreesPreviewRemove, WorktreeRef{}, RemovePreview{}, "What removing a worktree would lose."),
	m(MWorktreesRemove, WorktreeRemoveParams{}, Empty{}, "Remove a worktree, and its branch when asked."),
	m(MWorktreesLock, WorktreeLockParams{}, WorktreeInfo{}, "Lock or unlock a worktree."),
	m(MWorktreesMove, WorktreeMoveParams{}, WorktreeInfo{}, "Move a worktree folder."),
	m(MWorktreesRepair, ProjectRef{}, WorktreesResult{}, "Repair worktree links after folders moved."),
	m(MWorktreesPrune, PruneParams{}, PruneResult{}, "Prune records of missing worktrees."),
	m(MWorktreesCleanup, CleanupParams{}, CleanupResult{}, "Apply or preview the project's cleanup policy."),
	m(MWorktreesSetup, WorktreeSetupParams{}, SetupResult{}, "Show a worktree's setup; run it when it never ran or on rerun."),
	m(MWorktreesTrust, TrustParams{}, SetupResult{}, "Trust a repository setup script and run it."),
	m(MWorktreesPorts, WorktreeRef{}, PortsResult{}, "Ports listened on by processes in a worktree."),

	m(MGitStatus, WorktreeRef{}, GitStatusResult{}, "A worktree's status and changed files."),
	m(MGitLog, GitLogParams{}, GitLogResult{}, "One page of the commit graph."),
	m(MGitShow, GitCommitParams{}, GitCommitDetail{}, "One commit with its files."),
	m(MGitDiff, DiffParams{}, DiffResult{}, "A diff with hunks and line numbers."),
	m(MGitStage, GitPathsParams{}, GitStatusResult{}, "Stage files."),
	m(MGitUnstage, GitPathsParams{}, GitStatusResult{}, "Unstage files."),
	m(MGitDiscard, GitPathsParams{}, GitStatusResult{}, "Discard changes to files."),
	m(MGitCommit, GitCommitWriteParams{}, GitOpResult{}, "Commit the staged changes."),
	m(MGitPush, GitPushParams{}, GitOpResult{}, "Push a worktree's branch."),
	m(MGitFetch, ProjectRef{}, GitOpResult{}, "Fetch every remote."),
	m(MGitPull, WorktreeRef{}, GitOpResult{}, "Fast-forward a worktree's branch from its upstream."),
	m(MGitBranchCreate, GitRefParams{}, GitOpResult{}, "Create a branch at a commit."),
	m(MGitBranchDelete, GitRefParams{}, GitOpResult{}, "Delete a branch."),
	m(MGitCheckout, GitRefParams{}, GitOpResult{}, "Switch a worktree to a branch or commit."),
	m(MGitMerge, GitApplyParams{}, GitOpResult{}, "Merge a ref into a worktree."),
	m(MGitCherryPick, GitApplyParams{}, GitOpResult{}, "Cherry-pick commits into a worktree."),
	m(MGitRevert, GitApplyParams{}, GitOpResult{}, "Revert commits in a worktree."),
	m(MGitRebase, GitApplyParams{}, GitOpResult{}, "Rebase a worktree's branch onto a ref."),
	m(MGitAbort, WorktreeRef{}, GitOpResult{}, "Abort the operation in progress."),
	m(MGitContinue, WorktreeRef{}, GitOpResult{}, "Continue the operation in progress."),

	m(MReviewList, WorktreeRef{}, ReviewResult{}, "A worktree's review comments."),
	m(MReviewAdd, ReviewAddParams{}, ReviewComment{}, "Pin a comment to a diff line."),
	m(MReviewResolve, ReviewRef{}, ReviewComment{}, "Resolve or reopen a comment."),
	m(MReviewDelete, ReviewRef{}, Empty{}, "Delete a comment."),
	m(MReviewSend, ReviewSendParams{}, SessionInfo{}, "Send unresolved comments to an agent as one prompt."),

	m(MTasksList, TasksParams{}, TasksResult{}, "Tasks in board order."),
	m(MTasksCreate, TaskCreateParams{}, TaskInfo{}, "Create a task."),
	m(MTasksUpdate, TaskUpdateParams{}, TaskInfo{}, "Apply a JSON merge patch to a task."),
	m(MTasksDelete, TaskRef{}, Empty{}, "Delete a task; its worktree and sessions stay."),
	m(MTasksStart, TaskStartParams{}, TaskStartResult{}, "Start a task in a new worktree with an Agent session."),
	m(MTasksGate, GateParams{}, TaskInfo{}, "Open a decision gate on a task."),
	m(MTasksResolveGate, GateResolveParams{}, TaskInfo{}, "Resolve a decision gate."),
	m(MTasksPlan, TaskPlanParams{}, TaskPlanResult{}, "Draft tasks for a goal with the model router."),

	m(MActivityList, ActivityParams{}, ActivityResult{}, "A project's action ledger, newest first."),

	m(MTeamsList, Empty{}, TeamsResult{}, "Teams by name."),
	m(MTeamsTemplates, Empty{}, TeamTemplatesResult{}, "The ready-made teams a new one can start from."),
	m(MTeamsStructures, Empty{}, TeamStructuresResult{}, "Local structure recipes, visibility, deleted snapshots and source identities."),
	m(MTeamsStructureDraft, StructureDraftParams{}, StructureDraftResult{}, "Preview a visible recipe without creating a team or selecting a model."),
	m(MTeamsStructureSuggest, StructureAnswers{}, StructureSuggestionsResult{}, "Deterministic local advice from the visible catalogue."),
	m(MTeamsStructureSave, StructureSaveParams{}, StructureTemplate{}, "Save a custom recipe using its current revision."),
	m(MTeamsStructureClone, StructureCloneParams{}, StructureTemplate{}, "Clone a recipe into a new custom ID."),
	m(MTeamsStructureDelete, StructureRef{}, Empty{}, "Hide a default or retain a deleted custom recipe for explicit restore."),
	m(MTeamsStructureRestore, StructureRef{}, StructureTemplate{}, "Restore a hidden or deleted recipe using its current revision."),
	m(MTeamsCreate, TeamCreateParams{}, Team{}, "Create a team, with its routines."),
	m(MTeamsPropose, TeamProposeParams{}, Approval{}, "Propose a team; the user approves it in the Inbox, which creates it."),
	m(MTeamsUpdate, TeamUpdateParams{}, Team{}, "Apply a JSON merge patch to a team."),
	m(MTeamsMemberConfigGet, MemberConfigRef{}, MemberConfigDocument{}, "Read a masked current or retained member configuration."),
	m(MTeamsMemberConfigSave, MemberConfigSaveParams{}, MemberConfigDocument{}, "Save versioned member YAML against the current revision."),
	m(MTeamsMemberConfigHistory, MemberConfigRef{}, MemberConfigHistoryResult{}, "List retained member revisions newest first."),
	m(MTeamsMemberConfigRestore, MemberConfigRestoreParams{}, MemberConfigDocument{}, "Restore a retained member configuration as a new revision."),
	m(MTeamsDelete, TeamRef{}, Empty{}, "Delete a team, its tasks, routines and grants; its folder stays."),
	m(MTeamsPause, TeamPauseParams{}, Team{}, "Pause a team, or let it work again."),
	m(MTeamsExport, TeamRef{}, TeamExportResult{}, "Write a team's definition into its folder, without history or credential values."),
	m(MTeamsImport, TeamImportParams{}, Team{}, "Create a paused team from an exported definition."),
	m(MTeamsTasks, TeamTasksParams{}, TeamTasksResult{}, "Team tasks, newest first."),
	m(MTeamsTaskCreate, TeamTaskCreateParams{}, TeamTask{}, "Give a team a task; it starts when the team has room."),
	m(MTeamsTaskContext, TeamTaskRef{}, TeamTaskContext{}, "For the owner: read a task's preserved goal and nearest-parent-first because chain."),
	m(MTeamsTaskStop, TeamTaskRef{}, TeamTask{}, "Stop a task and the tasks it delegated."),
	m(MTeamsTaskRetry, TeamTaskRetryParams{}, TeamTask{}, "Run a finished, blocked or stopped task again."),
	m(MTeamsTaskRead, TeamTaskRef{}, TeamTask{}, "Mark a task's report as read."),
	m(MTeamsTaskDelete, TeamTaskRef{}, Empty{}, "Delete a task and its record; its sessions stay."),
	m(MTeamsMessages, TeamMessagesParams{}, TeamMessagesResult{}, "For the owner: read team or task messages with cursor pagination."),
	m(MTeamsTaskComment, TeamTaskCommentParams{}, TeamMessage{}, "For the owner: add a comment to a task without granting permissions."),
	m(MTeamsTaskReassign, TeamTaskReassignParams{}, TeamTask{}, "For the owner: reassign a task to a member under the current team policy."),
	m(MTeamsTaskPlan, TeamTaskPlanParams{}, TeamTask{}, "For the owner: supply and approve a pending strategy without widening team/member grants."),
	m(MTeamsRoutines, TeamRef{}, RoutinesResult{}, "Routines of a team, or of every team when the id is empty."),
	m(MTeamsRoutineCreate, RoutineCreateParams{}, Routine{}, "Create a routine."),
	m(MTeamsRoutineUpdate, RoutineUpdateParams{}, Routine{}, "Apply a JSON merge patch to a routine."),
	m(MTeamsRoutineDelete, RoutineRef{}, Empty{}, "Delete a routine; the tasks it made stay."),
	m(MTeamsRoutineRun, RoutineRef{}, TeamTask{}, "Run a routine once now."),
	m(MTeamsRoutinePreview, RoutinePreviewParams{}, RoutinePreview{}, "A schedule in words and its next times."),
	m(MTeamsApprovals, ApprovalsParams{}, ApprovalsResult{}, "Approvals, newest first."),
	m(MTeamsApprovalDecide, ApprovalDecideParams{}, Approval{}, "Answer an approval; an approved action runs once."),
	m(MTeamsActivity, TeamActivityParams{}, ActivityResult{}, "The teams' ledger, newest first."),
	m(MTeamsSpend, TeamSpendParams{}, TeamSpend{}, "A month of tokens by day, member, routine and engine."),
	m(MTeamsPrune, TeamPruneParams{}, Empty{}, "Delete finished tasks with their sessions: past the kept days, or all of them."),
	m(MTeamsDreamCreate, DreamCreateParams{}, Team{}, "Start a dream: a team whose manager hires and works until the objective is built and accepted."),
	m(MTeamsDreamAnswer, DreamAnswerParams{}, Team{}, "Accept a dream's presentation, or send notes that keep its team working."),

	m(MTeamsAgentReport, AgentReportParams{}, AgentAnswer{}, "For a team task's agent: hand back the report and end the task."),
	m(MTeamsAgentDelegate, AgentDelegateParams{}, AgentAnswer{}, "For a team task's agent: delegate downward through its configured reporting lines."),
	m(MTeamsAgentStatus, AgentRef{}, AgentStatusResult{}, "For a team task's agent: its delegated work, goal lineage and safe member directory."),
	m(MTeamsAgentAsk, AgentAskParams{}, AgentAnswer{}, "For a team task's agent: ask the user to decide."),
	m(MTeamsAgentPlan, AgentPlanParams{}, AgentAnswer{}, "For a live team agent: propose a strategy and yield for the Summoner's decision; acceptance never widens permissions."),
	m(MTeamsAgentHelp, AgentHelpParams{}, AgentAnswer{}, "For a live team agent: request a peer's help through the common manager, without assigning that peer, then yield."),
	m(MTeamsAgentEscalate, AgentEscalateParams{}, AgentAnswer{}, "For a live team agent: escalate a blocker to the nearest manager or Summoner, then yield."),
	m(MTeamsAgentHTTP, AgentHTTPParams{}, AgentHTTPResult{}, "For a team task's agent: an HTTPS request carrying a credential it never sees."),
	m(MTeamsAgentMail, AgentMailParams{}, AgentAnswer{}, "For a team task's agent: send a mail once the user approves it."),
	m(MTeamsAgentRequest, AgentCredentialRequestParams{}, AgentAnswer{}, "For a team task's agent: ask the user to add a credential."),
	m(MTeamsAgentCredentials, AgentRef{}, AgentCredentialsResult{}, "For a team task's agent: the credentials its team may use."),
	m(MTeamsAgentHire, AgentHireParams{}, Member{}, "For a dream's manager: add a member to the team."),
	m(MTeamsAgentRelease, AgentReleaseParams{}, AgentAnswer{}, "For a dream's manager: let a member go once its work is done."),
	m(MTeamsAgentDream, AgentDreamParams{}, AgentAnswer{}, "For a dream's manager: say where the dream stands."),
	m(MTeamsAgentPresent, AgentPresentParams{}, AgentAnswer{}, "For a dream's manager: present the finished dream to the user."),
	m(MTeamsAgentHands, AgentHandsParams{}, AgentAnswer{}, "For a team task's agent: ask the user to let Hyakki's Hands work toward a goal in the nested desktop or one picked window; Hyakki's own loop acts."),

	m(MVaultInfo, Empty{}, VaultInfo{}, "The vault's state and credentials, never a value."),
	m(MVaultAdd, VaultAddParams{}, Credential{}, "Store a credential."),
	m(MVaultUpdate, VaultUpdateParams{}, Credential{}, "Change a credential's details or replace its value."),
	m(MVaultRemove, VaultRef{}, Empty{}, "Remove a credential and its sealed values."),
	m(MVaultEvents, VaultEventsParams{}, VaultEventsResult{}, "The vault's record of use, newest first."),
	m(MVaultMail, MailAccountParams{}, VaultInfo{}, "Set or remove the mail account."),
	m(MVaultMailTest, Empty{}, MailTestResult{}, "Log in to the mail account's server without sending."),

	m(MSessionSetMemory, SetMemoryParams{}, SessionInfo{}, "Set what a conversation does with memory; after its first message only narrower."),

	m(MMemoryStatus, Empty{}, MemoryStatus{}, "Memory as a whole: engine, counts, chart, learner and card."),
	m(MMemoryList, MemoryListParams{}, MemoryListResult{}, "Memories by scope, state, words and sort."),
	m(MMemoryGet, MemoryRefParams{}, Memory{}, "One memory with its earlier versions."),
	m(MMemoryAdd, MemoryAddParams{}, MemoryChange{}, "Remember a fact."),
	m(MMemoryEdit, MemoryEditParams{}, MemoryChange{}, "Change a memory's text, scope or pin."),
	m(MMemoryForget, MemoryForgetParams{}, MemoryChange{}, "Forget memories; they are never learned again."),
	m(MMemoryRestore, MemoryRestoreParams{}, MemoryChange{}, "Bring back a replaced memory or an earlier text."),
	m(MMemoryKeep, MemoryIDsParams{}, MemoryChange{}, "Keep suggested memories."),
	m(MMemoryDismiss, MemoryIDsParams{}, MemoryChange{}, "Dismiss suggested memories."),
	m(MMemoryUndo, MemoryChangeParams{}, MemoryChange{}, "Undo a change."),
	m(MMemoryRedo, MemoryChangeParams{}, MemoryChange{}, "Do an undone change again."),
	m(MMemoryChanges, MemoryChangesParams{}, MemoryChangesResult{}, "Changes by id, with whether each was undone."),
	m(MMemoryForgetAll, MemoryForgetAllParams{}, MemoryChange{}, "Forget every memory in a scope, or everything."),
	m(MMemorySearch, MemorySearchParams{}, MemorySearchResult{}, "Memories by meaning and words, best first."),
	m(MMemorySessions, MemorySessionsParams{}, MemorySessionsResult{}, "Past conversations by words, best first."),
	m(MMemoryCard, MemoryCardParams{}, MemoryCard{}, "The card about the user that answers are given."),
	m(MMemoryImport, MemoryImportParams{}, MemoryChange{}, "Split another assistant's memories into suggestions."),
	m(MMemorySuppress, MemorySuppressParams{}, Empty{}, "Stop a memory being used in one conversation."),
	m(MMemoryLearn, MemoryLearnParams{}, MemoryChange{}, "Learn from a conversation now."),
	m(MMemoryEngineTest, MemoryEngineParams{}, MemoryEngineTest{}, "Check that an engine answers."),
	m(MMemoryEngineSetup, MemoryEngineParams{}, Empty{}, "Set an engine up on this computer; progress arrives as memory.setup."),
	m(MMemoryEngineMove, MemoryEngineParams{}, MemoryStatus{}, "Copy every memory to an engine and use it from then on."),

	m(MSkillsList, Empty{}, SkillsResult{}, "The built-in skill sets, their skills and where they lie."),
	m(MSkillsGet, SkillParams{}, SkillContent{}, "Read a skill's SKILL.md or one of its files."),
	m(MSkillsSearch, SkillsSearchParams{}, SkillsSearchResult{}, "Skills that fit what is about to be done, best first."),

	m(MWritingCheck, WritingCheckParams{}, WritingCheckResult{}, "Check writing and return fixes."),
	m(MWritingTransform, WritingTransformParams{}, WritingTransformResult{}, "Transform a piece of writing."),
	m(MWritingCancel, WritingCancelParams{}, Empty{}, "Cancel a writing request."),
	m(MWritingVoiceGet, WritingVoiceGetParams{}, WritingVoice{}, "Read your writing voice."),
	m(MWritingVoiceSet, WritingVoiceSetParams{}, WritingVoice{}, "Save your writing voice."),
	m(MWritingVoiceLearn, WritingVoiceLearnParams{}, WritingVoice{}, "Learn your writing voice from samples."),
	m(MWritingWords, WritingWordsParams{}, WritingWords{}, "Read your dictionary."),
	m(MWritingWordsAdd, WritingWordParams{}, WritingWords{}, "Add a word to your dictionary."),
	m(MWritingWordsRemove, WritingWordParams{}, WritingWords{}, "Remove a word from your dictionary."),
	m(MPromptsList, PromptsListParams{}, PromptsListResult{}, "List prompts and templates."),
	m(MPromptsSave, PromptSaveParams{}, Prompt{}, "Save a prompt or template."),
	m(MPromptsDelete, PromptDeleteParams{}, Empty{}, "Delete a saved prompt."),
	m(MPromptsExpand, PromptExpandParams{}, PromptExpandResult{}, "Fill a prompt's variables."),
	m(MCompareStart, CompareStartParams{}, CompareStartResult{}, "Ask other models the same question."),
	m(MCompareKeep, CompareKeepParams{}, SessionInfo{}, "Keep one comparison conversation."),
	m(MSessionSetFlags, SessionFlagsParams{}, SessionInfo{}, "Star or archive a conversation."),
	m(MMCPList, MCPListParams{}, MCPListResult{}, "List tool servers and their state."),
	m(MMCPSave, MCPServer{}, MCPServerInfo{}, "Save a tool server; app only."),
	m(MMCPDelete, MCPDeleteParams{}, Empty{}, "Delete a tool server; app only."),
	m(MMCPCheck, MCPCheckParams{}, MCPServerInfo{}, "Check a tool server and read its tools."),
	m(MMCPSignIn, MCPSignInParams{}, MCPSignInResult{}, "Sign in to a tool server; app only."),
	m(MDiskUsage, DiskUsageParams{}, DiskUsage{}, "Read agent storage and free space."),
	m(MDiskClean, DiskCleanParams{}, DiskUsage{}, "Clean selected agent storage; app only."),
	m(MArtifactOpen, ArtifactOpenParams{}, ArtifactOpenResult{}, "Open a local page preview."),
	m(MArtifactErrors, ArtifactErrorsParams{}, ArtifactErrorsResult{}, "Read problems reported by a preview."),
	m(MArtifactRepair, ArtifactRepairParams{}, AskResult{}, "Ask the conversation to fix a preview."),
	m(MArtifactClose, ArtifactCloseParams{}, Empty{}, "Close a page preview."),
	m(MCollectionsList, CollectionsListParams{}, CollectionsListResult{}, "List saved collections."),
	m(MCollectionsSave, CollectionSaveParams{}, Collection{}, "Create or rename a collection."),
	m(MCollectionsAdd, CollectionAddParams{}, CollectionItem{}, "Save text or a file to a collection."),
	m(MCollectionsItems, CollectionItemsParams{}, CollectionItemsResult{}, "Read a collection's items."),
	m(MCollectionsRemove, CollectionRemoveParams{}, Empty{}, "Remove a saved collection item."),
	m(MCollectionsCheck, CollectionCheckParams{}, CollectionItem{}, "Check whether a saved source changed."),
	m(MCollectionsChat, CollectionChatParams{}, AskResult{}, "Ask about a whole collection."),
	m(MTeamsRoutineRuns, RoutineRunsParams{}, RoutineRunsResult{}, "Read a routine's run history."),
	m(MTeamsAgentSetup, AgentSetupParams{}, AgentAnswer{}, "Ask the user to complete setup steps."),
}

var methodIndex = func() map[string]Method {
	idx := make(map[string]Method, len(Methods))
	for _, mt := range Methods {
		idx[mt.Name] = mt
	}
	return idx
}()

// Lookup returns the registry entry for name.
func Lookup(name string) (Method, bool) {
	mt, ok := methodIndex[name]
	return mt, ok
}
