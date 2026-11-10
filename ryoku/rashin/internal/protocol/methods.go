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
	MAsk               = "ask"

	MProvidersList      = "providers.list"
	MProvidersAddKey    = "providers.add_key"
	MProvidersRemoveKey = "providers.remove_key"
	MProvidersTest      = "providers.test"
	MProvidersEnable    = "providers.enable"
	MModelsList         = "models.list"
	MGatewayUsage       = "gateway.usage"
	MGatewayAPI         = "gateway.api"
	MGatewayAPIKey      = "gateway.api_key"

	MHarnessesList = "harnesses.list"

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
)

// Event names.
const (
	EvtSessionEvent     = "session.event"     // SessionEventData
	EvtSessionInfo      = "session.info"      // SessionInfo
	EvtSessionsChanged  = "sessions.changed"  // SessionsChanged
	EvtSettingsChanged  = "settings.changed"  // Settings
	EvtProvidersChanged = "providers.changed" // ProvidersResult
	EvtHarnessesChanged = "harnesses.changed" // HarnessesResult
	EvtIndexProgress    = "index.progress"    // IndexProgress
	EvtIndexChanged     = "index.changed"     // IndexInfo
	EvtUIFocus          = "ui.focus"          // UIOpenParams, app clients only
	EvtNotice           = "notice"            // Notice
	EvtProjectsChanged  = "projects.changed"  // ProjectsResult
	EvtWorktreesChanged = "worktrees.changed" // WorktreesResult
	EvtTasksChanged     = "tasks.changed"     // TasksResult (one project's tasks)
	EvtGitChanged       = "git.changed"       // GitChanged, watchers of the project
	EvtReviewChanged    = "review.changed"    // ReviewResult
	EvtAPIChanged       = "api.changed"       // APIInfo
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
	m(MSessionSetMode, SetModeParams{}, Empty{}, "Pick the session's harness mode."),
	m(MSessionRename, RenameParams{}, SessionInfo{}, "Rename a session."),
	m(MSessionDelete, SessionRef{}, Empty{}, "Delete a session and its log."),
	m(MSessionStop, SessionRef{}, Empty{}, "Stop the session's agent process; the session stays resumable."),
	m(MSessionHandoff, HandoffParams{}, SessionInfo{}, "Move an Ask session's work to a new Agent session."),
	m(MSessionQueue, QueueParams{}, SessionInfo{}, "Queue a follow-up sent when the running turn ends."),
	m(MSessionUnqueue, UnqueueParams{}, SessionInfo{}, "Remove a queued follow-up."),
	m(MSessionSteer, PromptParams{}, PromptResult{}, "Cancel the running turn and prompt at once."),
	m(MSessionFork, ForkParams{}, SessionInfo{}, "Copy a session up to an event into a new session."),
	m(MSessionRead, SessionRef{}, Empty{}, "Mark a session's finished turns as seen."),
	m(MSessionRevertTurn, TurnParams{}, GitOpResult{}, "Restore the worktree files to the checkpoint taken before a turn."),
	m(MAsk, AskParams{}, AskResult{}, "Create or continue an Ask session, attach and prompt."),

	m(MProvidersList, Empty{}, ProvidersResult{}, "Catalogue providers and their keys."),
	m(MProvidersAddKey, AddKeyParams{}, ProviderInfo{}, "Store a provider key."),
	m(MProvidersRemoveKey, RemoveKeyParams{}, ProviderInfo{}, "Remove a provider key."),
	m(MProvidersTest, ProviderParams{}, ProviderTestResult{}, "Run a one-token completion through a provider."),
	m(MProvidersEnable, EnableProviderParams{}, ProviderInfo{}, "Turn a provider on or off for routing."),
	m(MModelsList, ModelsParams{}, ModelsResult{}, "Free chat models and routes."),
	m(MGatewayUsage, UsageParams{}, GatewayUsage{}, "Requests and tokens per day, provider and model."),
	m(MGatewayAPI, Empty{}, APIInfo{}, "The local API: whether it is on, where it listens."),
	m(MGatewayAPIKey, APIKeyParams{}, APIKey{}, "The key other programs present to the local API; rotate replaces it."),

	m(MHarnessesList, HarnessesListParams{}, HarnessesResult{}, "External harnesses and whether they are installed."),

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
	m(MTasksPlan, TaskPlanParams{}, TaskPlanResult{}, "Draft tasks for a goal with the free-model router."),

	m(MActivityList, ActivityParams{}, ActivityResult{}, "A project's action ledger, newest first."),
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
