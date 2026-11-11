package protocol

import (
	"encoding/json"
	"time"
)

// Work is Rashin's coding control plane: projects, worktrees, Git, review,
// checkpoints and tasks. Everyday Ask never needs any of it.

// ProjectInfo is a registered folder, usually a Git repository. Its id is the
// same short hash of the root that the index uses.
type ProjectInfo struct {
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Root      string          `json:"root"`
	Git       bool            `json:"git"`
	BaseRef   string          `json:"baseRef,omitempty"`
	Branch    string          `json:"branch,omitempty"`
	Worktrees int             `json:"worktrees"`
	Sessions  int             `json:"sessions"`
	Tasks     int             `json:"tasks"`
	Attention int             `json:"attention"`
	Settings  ProjectSettings `json:"settings"`
	Created   time.Time       `json:"created"`
	Activity  time.Time       `json:"activity"`
}

// Worktree cleanup protection levels.
const (
	ProtectUncommitted = "uncommitted" // tracked changes block removal
	ProtectAny         = "any"         // untracked files block it too
)

// CleanupPolicy decides when Rashin-made worktrees may be removed without an
// explicit request. Auto is off unless the user turns it on; it runs only while
// the daemon is already awake for other work.
type CleanupPolicy struct {
	Auto           bool   `json:"auto"`
	AfterMergeDays int    `json:"afterMergeDays,omitempty"`
	InactiveDays   int    `json:"inactiveDays,omitempty"`
	Protect        string `json:"protect,omitempty"`
}

// ProjectSettings are per project and live in the daemon, never in the repo.
type ProjectSettings struct {
	BaseRef      string        `json:"baseRef,omitempty"`
	WorktreeRoot string        `json:"worktreeRoot,omitempty"`
	BranchPrefix string        `json:"branchPrefix,omitempty"`
	Setup        string        `json:"setup,omitempty"`
	CopyIgnored  []string      `json:"copyIgnored,omitempty"`
	Harness      string        `json:"harness,omitempty"`
	Cleanup      CleanupPolicy `json:"cleanup"`
	// TrustedScripts are sha256 sums of repository setup scripts the user
	// approved (.rashin/setup).
	TrustedScripts []string `json:"trustedScripts,omitempty"`
}

// ProjectsResult lists projects, most recently active first.
type ProjectsResult struct {
	Projects []ProjectInfo `json:"projects"`
}

// ProjectAddParams registers a folder.
type ProjectAddParams struct {
	Path string `json:"path"`
	Name string `json:"name,omitempty"`
}

// ProjectRef names a project.
type ProjectRef struct {
	Project string `json:"project"`
}

// ProjectSettingsParams applies a JSON merge patch to a project's settings.
type ProjectSettingsParams struct {
	Project string          `json:"project"`
	Patch   json.RawMessage `json:"patch"`
}

// Setup states of a worktree.
const (
	SetupNone      = "none"
	SetupRunning   = "running"
	SetupDone      = "done"
	SetupFailed    = "failed"
	SetupUntrusted = "untrusted"
)

// Operations in progress inside a worktree.
const (
	OpMerge      = "merge"
	OpRebase     = "rebase"
	OpCherryPick = "cherry-pick"
	OpRevert     = "revert"
	OpBisect     = "bisect"
)

// WorktreeStatus is one worktree's Git state.
type WorktreeStatus struct {
	Upstream   string    `json:"upstream,omitempty"`
	Ahead      int       `json:"ahead"`
	Behind     int       `json:"behind"`
	AheadBase  int       `json:"aheadBase"`
	BehindBase int       `json:"behindBase"`
	Staged     int       `json:"staged"`
	Modified   int       `json:"modified"`
	Untracked  int       `json:"untracked"`
	Conflicted int       `json:"conflicted"`
	Operation  string    `json:"operation,omitempty"`
	Added      int       `json:"added"`
	Removed    int       `json:"removed"`
	Checked    time.Time `json:"checked"`
}

// WorktreeInfo describes one worktree. The absolute path is its identity.
type WorktreeInfo struct {
	Path       string          `json:"path"`
	Project    string          `json:"project"`
	Name       string          `json:"name"`
	Branch     string          `json:"branch,omitempty"`
	Head       string          `json:"head"`
	Detached   bool            `json:"detached,omitempty"`
	Main       bool            `json:"main,omitempty"`
	Owned      bool            `json:"owned,omitempty"`
	Locked     bool            `json:"locked,omitempty"`
	LockReason string          `json:"lockReason,omitempty"`
	Prunable   bool            `json:"prunable,omitempty"`
	Missing    bool            `json:"missing,omitempty"`
	Base       string          `json:"base,omitempty"`
	BaseSha    string          `json:"baseSha,omitempty"`
	Parent     string          `json:"parent,omitempty"`
	Task       string          `json:"task,omitempty"`
	Sessions   []string        `json:"sessions,omitempty"`
	Status     *WorktreeStatus `json:"status,omitempty"`
	Setup      string          `json:"setup,omitempty"`
	Created    *time.Time      `json:"created,omitempty"`
	Activity   *time.Time      `json:"activity,omitempty"`
}

// WorktreesResult lists a project's worktrees, main first.
type WorktreesResult struct {
	Project   string         `json:"project"`
	Worktrees []WorktreeInfo `json:"worktrees"`
}

// Worktree creation modes.
const (
	WorktreeNewBranch      = "new-branch"
	WorktreeExistingBranch = "existing-branch"
	WorktreeDetached       = "detached"
)

// WorktreeCreateParams creates a worktree. Name derives the folder and, with
// the project's branch prefix, the branch unless Branch is given. Base defaults
// to the project's base ref.
type WorktreeCreateParams struct {
	Project string `json:"project"`
	Name    string `json:"name"`
	Mode    string `json:"mode,omitempty"`
	Branch  string `json:"branch,omitempty"`
	Base    string `json:"base,omitempty"`
	Parent  string `json:"parent,omitempty"`
	Task    string `json:"task,omitempty"`
	Lock    bool   `json:"lock,omitempty"`
	// Setup runs the project's setup after creation when trusted.
	Setup bool `json:"setup,omitempty"`
}

// WorktreeRef names a worktree by path.
type WorktreeRef struct {
	Path string `json:"path"`
}

// WorktreeSetupParams shows a worktree's setup. It runs when it never ran,
// or again when Rerun is set; a finished setup is otherwise only reported.
type WorktreeSetupParams struct {
	Path  string `json:"path"`
	Rerun bool   `json:"rerun,omitempty"`
}

// RemovePreview is what removing a worktree would lose.
type RemovePreview struct {
	Path        string   `json:"path"`
	Branch      string   `json:"branch,omitempty"`
	Dirty       []string `json:"dirty,omitempty"`
	Untracked   []string `json:"untracked,omitempty"`
	Unpushed    []string `json:"unpushed,omitempty"`
	Unmerged    bool     `json:"unmerged,omitempty"`
	Locked      bool     `json:"locked,omitempty"`
	Running     []string `json:"running,omitempty"`
	Main        bool     `json:"main,omitempty"`
	NeedsForce  bool     `json:"needsForce"`
	CanRemove   bool     `json:"canRemove"`
	BlockReason string   `json:"blockReason,omitempty"`
}

// WorktreeRemoveParams removes a worktree. Force is required when the preview
// says NeedsForce; DeleteBranch also deletes its branch (with -d, or -D when
// ForceBranch).
type WorktreeRemoveParams struct {
	Path         string `json:"path"`
	Force        bool   `json:"force,omitempty"`
	DeleteBranch bool   `json:"deleteBranch,omitempty"`
	ForceBranch  bool   `json:"forceBranch,omitempty"`
}

// WorktreeLockParams locks or unlocks a worktree.
type WorktreeLockParams struct {
	Path   string `json:"path"`
	Lock   bool   `json:"lock"`
	Reason string `json:"reason,omitempty"`
}

// WorktreeMoveParams moves a worktree folder.
type WorktreeMoveParams struct {
	Path string `json:"path"`
	To   string `json:"to"`
}

// PruneParams prunes stale worktree records; DryRun only reports.
type PruneParams struct {
	Project string `json:"project"`
	DryRun  bool   `json:"dryRun"`
}

// PruneResult lists what was (or would be) pruned.
type PruneResult struct {
	Pruned []string `json:"pruned"`
}

// CleanupItem is one worktree considered by cleanup, with the reason it was
// removed or kept.
type CleanupItem struct {
	Path    string `json:"path"`
	Removed bool   `json:"removed"`
	Reason  string `json:"reason"`
}

// CleanupParams runs (or with DryRun previews) the project's cleanup policy.
type CleanupParams struct {
	Project string `json:"project"`
	DryRun  bool   `json:"dryRun"`
}

// CleanupResult lists every Rashin-made worktree and what cleanup did.
type CleanupResult struct {
	Items []CleanupItem `json:"items"`
}

// SetupResult is the latest setup run of a worktree.
type SetupResult struct {
	Path    string `json:"path"`
	State   string `json:"state"`
	Command string `json:"command,omitempty"`
	Script  string `json:"script,omitempty"`
	Hash    string `json:"hash,omitempty"`
	Log     string `json:"log,omitempty"`
	Exit    *int   `json:"exit,omitempty"`
}

// TrustParams trusts a repository setup script by hash and runs it.
type TrustParams struct {
	Path string `json:"path"`
	Hash string `json:"hash"`
}

// Port is a listening socket owned by a process running in a worktree.
type Port struct {
	Port    int    `json:"port"`
	PID     int    `json:"pid"`
	Command string `json:"command"`
	URL     string `json:"url"`
}

// PortsResult lists a worktree's listening ports.
type PortsResult struct {
	Path  string `json:"path"`
	Ports []Port `json:"ports"`
}

// File change statuses (git letters, plus ? for untracked and U for conflict).
const (
	ChangeAdded     = "A"
	ChangeModified  = "M"
	ChangeDeleted   = "D"
	ChangeRenamed   = "R"
	ChangeCopied    = "C"
	ChangeType      = "T"
	ChangeConflict  = "U"
	ChangeUntracked = "?"
)

// FileChange is one changed path with line counts.
type FileChange struct {
	Path    string `json:"path"`
	OldPath string `json:"oldPath,omitempty"`
	Status  string `json:"status"`
	Staged  bool   `json:"staged,omitempty"`
	Added   int    `json:"added"`
	Removed int    `json:"removed"`
	Binary  bool   `json:"binary,omitempty"`
}

// GitStatusResult is a worktree's status with its files.
type GitStatusResult struct {
	Path   string         `json:"path"`
	Branch string         `json:"branch,omitempty"`
	Head   string         `json:"head"`
	Status WorktreeStatus `json:"status"`
	Files  []FileChange   `json:"files"`
}

// Ref kinds.
const (
	RefBranch = "branch"
	RefRemote = "remote"
	RefTag    = "tag"
	RefHead   = "head"
)

// GitRef is one ref decoration. Worktree is set when a branch is checked out
// in a worktree.
type GitRef struct {
	Name     string `json:"name"`
	Kind     string `json:"kind"`
	Target   string `json:"target"`
	Worktree string `json:"worktree,omitempty"`
	Current  bool   `json:"current,omitempty"`
	Upstream string `json:"upstream,omitempty"`
}

// GitCommit is one commit of the graph.
type GitCommit struct {
	Hash    string    `json:"hash"`
	Parents []string  `json:"parents,omitempty"`
	Author  string    `json:"author"`
	Email   string    `json:"email,omitempty"`
	Time    time.Time `json:"time"`
	Subject string    `json:"subject"`
}

// WipRow is a worktree's uncommitted work shown above its HEAD in the graph.
// It is never a commit.
type WipRow struct {
	Worktree   string `json:"worktree"`
	Branch     string `json:"branch,omitempty"`
	Head       string `json:"head"`
	Staged     int    `json:"staged"`
	Modified   int    `json:"modified"`
	Untracked  int    `json:"untracked"`
	Conflicted int    `json:"conflicted"`
	// Agent is the state of an agent session working in the worktree.
	Agent string `json:"agent,omitempty"`
}

// Graph scopes.
const (
	ScopeAll      = "all"
	ScopeBranch   = "branch"
	ScopeWorktree = "worktree"
)

// GitLogParams pages the commit graph newest first. Cursor is the opaque
// continuation from the previous page; an empty cursor starts at the top.
type GitLogParams struct {
	Project  string `json:"project"`
	Scope    string `json:"scope,omitempty"`
	Ref      string `json:"ref,omitempty"`
	Worktree string `json:"worktree,omitempty"`
	Search   string `json:"search,omitempty"`
	Limit    int    `json:"limit,omitempty"`
	Cursor   string `json:"cursor,omitempty"`
}

// GitLogResult is one page. Refs and Wip come with the first page only.
type GitLogResult struct {
	Commits []GitCommit `json:"commits"`
	Refs    []GitRef    `json:"refs,omitempty"`
	Wip     []WipRow    `json:"wip,omitempty"`
	Next    string      `json:"next,omitempty"`
}

// GitCommitParams asks for one commit's details.
type GitCommitParams struct {
	Project string `json:"project"`
	Hash    string `json:"hash"`
}

// GitCommitDetail is a commit with its message and files.
type GitCommitDetail struct {
	Commit    GitCommit    `json:"commit"`
	Committer string       `json:"committer,omitempty"`
	Body      string       `json:"body,omitempty"`
	Refs      []GitRef     `json:"refs,omitempty"`
	Files     []FileChange `json:"files"`
}

// Diff modes.
const (
	DiffWorking    = "working"    // unstaged changes
	DiffStaged     = "staged"     // staged changes
	DiffBase       = "base"       // the worktree against its base, committed and not
	DiffCommit     = "commit"     // one commit against its first parent
	DiffRange      = "range"      // From..To
	DiffCheckpoint = "checkpoint" // one agent turn: its checkpoint against the next
)

// DiffParams selects a diff.
type DiffParams struct {
	Project  string `json:"project,omitempty"`
	Worktree string `json:"worktree,omitempty"`
	Mode     string `json:"mode"`
	Commit   string `json:"commit,omitempty"`
	From     string `json:"from,omitempty"`
	To       string `json:"to,omitempty"`
	Session  string `json:"session,omitempty"`
	Turn     int    `json:"turn,omitempty"`
	Path     string `json:"path,omitempty"`
	Context  int    `json:"context,omitempty"`
}

// Diff line kinds.
const (
	LineContext = " "
	LineAdd     = "+"
	LineDel     = "-"
)

// DiffLine is one line of a hunk with its old and new line numbers (0 when
// absent on that side).
type DiffLine struct {
	Kind string `json:"kind"`
	Text string `json:"text"`
	Old  int    `json:"old,omitempty"`
	New  int    `json:"new,omitempty"`
}

// Hunk is one diff hunk.
type Hunk struct {
	Header   string     `json:"header"`
	OldStart int        `json:"oldStart"`
	OldLines int        `json:"oldLines"`
	NewStart int        `json:"newStart"`
	NewLines int        `json:"newLines"`
	Lines    []DiffLine `json:"lines"`
}

// FileDiff is one file's diff.
type FileDiff struct {
	Path    string `json:"path"`
	OldPath string `json:"oldPath,omitempty"`
	Status  string `json:"status"`
	Added   int    `json:"added"`
	Removed int    `json:"removed"`
	Binary  bool   `json:"binary,omitempty"`
	Hunks   []Hunk `json:"hunks,omitempty"`
}

// DiffResult is a set of file diffs. Truncated marks files cut at the size cap.
type DiffResult struct {
	Files     []FileDiff `json:"files"`
	Truncated bool       `json:"truncated,omitempty"`
}

// GitPathsParams names files in a worktree; All means every changed file.
type GitPathsParams struct {
	Path  string   `json:"path"`
	Files []string `json:"files,omitempty"`
	All   bool     `json:"all,omitempty"`
}

// GitCommitWriteParams commits the staged changes of a worktree.
type GitCommitWriteParams struct {
	Path    string `json:"path"`
	Message string `json:"message"`
	Amend   bool   `json:"amend,omitempty"`
}

// GitPushParams pushes a worktree's branch. ForceWithLease is never implied.
type GitPushParams struct {
	Path           string `json:"path"`
	SetUpstream    bool   `json:"setUpstream,omitempty"`
	ForceWithLease bool   `json:"forceWithLease,omitempty"`
}

// GitRefParams names a ref within a project or worktree.
type GitRefParams struct {
	Project string `json:"project,omitempty"`
	Path    string `json:"path,omitempty"`
	Ref     string `json:"ref"`
	At      string `json:"at,omitempty"`
	Force   bool   `json:"force,omitempty"`
}

// GitApplyParams brings commits into the worktree at Path: merge Ref, cherry-pick
// Commits, revert Commits, or rebase onto Ref.
type GitApplyParams struct {
	Path    string   `json:"path"`
	Ref     string   `json:"ref,omitempty"`
	Commits []string `json:"commits,omitempty"`
	FFOnly  bool     `json:"ffOnly,omitempty"`
	NoFF    bool     `json:"noFF,omitempty"`
}

// GitOpResult reports a Git operation. Conflicts lists conflicted paths when the
// worktree stopped in the middle of an operation.
type GitOpResult struct {
	OK        bool     `json:"ok"`
	Output    string   `json:"output,omitempty"`
	Operation string   `json:"operation,omitempty"`
	Conflicts []string `json:"conflicts,omitempty"`
}

// ReviewComment is a note pinned to a line of a worktree's diff.
type ReviewComment struct {
	ID       string    `json:"id"`
	Worktree string    `json:"worktree"`
	Path     string    `json:"path"`
	Line     int       `json:"line"`
	Side     string    `json:"side"`
	Text     string    `json:"text"`
	Resolved bool      `json:"resolved,omitempty"`
	Created  time.Time `json:"created"`
}

// ReviewResult lists a worktree's comments.
type ReviewResult struct {
	Worktree string          `json:"worktree"`
	Comments []ReviewComment `json:"comments"`
}

// ReviewAddParams pins a comment. Side is "new" or "old".
type ReviewAddParams struct {
	Worktree string `json:"worktree"`
	Path     string `json:"path"`
	Line     int    `json:"line"`
	Side     string `json:"side"`
	Text     string `json:"text"`
}

// ReviewRef names one comment.
type ReviewRef struct {
	Worktree string `json:"worktree"`
	ID       string `json:"id"`
	Resolved bool   `json:"resolved,omitempty"`
}

// ReviewSendParams sends every unresolved comment of a worktree to an agent
// session (the worktree's latest one unless Session is set) as one prompt.
type ReviewSendParams struct {
	Worktree string `json:"worktree"`
	Session  string `json:"session,omitempty"`
	Note     string `json:"note,omitempty"`
}

// Task statuses, in board order.
const (
	TaskBacklog  = "backlog"
	TaskReady    = "ready"
	TaskWorking  = "working"
	TaskNeedsYou = "needs-you"
	TaskReview   = "review"
	TaskDone     = "done"
	TaskBlocked  = "blocked"
)

// Gate is a decision a task waits on.
type Gate struct {
	ID         string    `json:"id"`
	Question   string    `json:"question"`
	Options    []string  `json:"options"`
	Resolution string    `json:"resolution,omitempty"`
	Created    time.Time `json:"created"`
}

// TaskProducts summarise what a task made.
type TaskProducts struct {
	Branch  string `json:"branch,omitempty"`
	Commits int    `json:"commits"`
	Files   int    `json:"files"`
	Added   int    `json:"added"`
	Removed int    `json:"removed"`
}

// TaskInfo is a durable work item, separate from the sessions that work on it.
type TaskInfo struct {
	ID       string       `json:"id"`
	Project  string       `json:"project"`
	Title    string       `json:"title"`
	Goal     string       `json:"goal"`
	Status   string       `json:"status"`
	Deps     []string     `json:"deps,omitempty"`
	Gates    []Gate       `json:"gates,omitempty"`
	Worktree string       `json:"worktree,omitempty"`
	Sessions []string     `json:"sessions,omitempty"`
	Harness  string       `json:"harness,omitempty"`
	Products TaskProducts `json:"products"`
	Order    int          `json:"order"`
	Created  time.Time    `json:"created"`
	Updated  time.Time    `json:"updated"`
}

// TasksParams filters tasks by project.
type TasksParams struct {
	Project string `json:"project,omitempty"`
}

// TasksResult lists tasks in board order.
type TasksResult struct {
	Tasks []TaskInfo `json:"tasks"`
}

// TaskCreateParams creates a task in the backlog.
type TaskCreateParams struct {
	Project string   `json:"project"`
	Title   string   `json:"title"`
	Goal    string   `json:"goal,omitempty"`
	Deps    []string `json:"deps,omitempty"`
	Harness string   `json:"harness,omitempty"`
}

// TaskUpdateParams applies a JSON merge patch to a task (title, goal, status,
// deps, harness, order).
type TaskUpdateParams struct {
	ID    string          `json:"id"`
	Patch json.RawMessage `json:"patch"`
}

// TaskRef names a task.
type TaskRef struct {
	ID string `json:"id"`
}

// TaskStartParams starts a task: a new worktree from Base (or the project base)
// and an Agent session in it prompted with the goal.
type TaskStartParams struct {
	ID      string `json:"id"`
	Harness string `json:"harness,omitempty"`
	Base    string `json:"base,omitempty"`
}

// TaskStartResult names what was created.
type TaskStartResult struct {
	Task     TaskInfo     `json:"task"`
	Worktree WorktreeInfo `json:"worktree"`
	Session  SessionInfo  `json:"session"`
}

// GateParams opens a gate on a task.
type GateParams struct {
	Task     string   `json:"task"`
	Question string   `json:"question"`
	Options  []string `json:"options"`
}

// GateResolveParams resolves a gate with one of its options.
type GateResolveParams struct {
	Task       string `json:"task"`
	Gate       string `json:"gate"`
	Resolution string `json:"resolution"`
}

// TaskDraft is a proposed task from tasks.plan; Deps index into the same list.
type TaskDraft struct {
	Title string `json:"title"`
	Goal  string `json:"goal"`
	Deps  []int  `json:"deps,omitempty"`
}

// TaskPlanParams asks the free-model router to split a goal into tasks.
type TaskPlanParams struct {
	Project string `json:"project"`
	Goal    string `json:"goal"`
}

// TaskPlanResult carries the drafts; nothing is created until the user picks.
type TaskPlanResult struct {
	Drafts []TaskDraft `json:"drafts"`
	Via    string      `json:"via,omitempty"`
}

// Activity is one state-changing action recorded in a project's or the
// teams' ledger. Team, Task, Initiator and Unwatched are set for teams:
// Initiator is who started it, Unwatched that no Rashin window was open.
type Activity struct {
	Time      time.Time `json:"time"`
	Actor     string    `json:"actor"`
	Action    string    `json:"action"`
	Target    string    `json:"target,omitempty"`
	OK        bool      `json:"ok"`
	Detail    string    `json:"detail,omitempty"`
	Team      string    `json:"team,omitempty"`
	Task      string    `json:"task,omitempty"`
	Initiator string    `json:"initiator,omitempty"`
	Unwatched bool      `json:"unwatched,omitempty"`
}

// ActivityParams pages a project's ledger, newest first.
type ActivityParams struct {
	Project string `json:"project"`
	Limit   int    `json:"limit,omitempty"`
}

// ActivityResult lists ledger entries.
type ActivityResult struct {
	Activity []Activity `json:"activity"`
}

// WatchParams starts or stops push updates (git.changed) for a project while a
// client shows it.
type WatchParams struct {
	Project string `json:"project"`
	Watch   bool   `json:"watch"`
}

// GitChanged is the data of a git.changed event.
type GitChanged struct {
	Project  string `json:"project"`
	Worktree string `json:"worktree,omitempty"`
}

// QueueParams queues a follow-up prompt sent when the running turn ends.
type QueueParams struct {
	ID     string       `json:"id"`
	Text   string       `json:"text"`
	Images []Attachment `json:"images,omitempty"`
}

// UnqueueParams removes a queued prompt.
type UnqueueParams struct {
	ID    string `json:"id"`
	Queue string `json:"queue"`
}

// QueuedPrompt is a follow-up waiting for the running turn to end.
type QueuedPrompt struct {
	ID     string       `json:"id"`
	Text   string       `json:"text"`
	Images []Attachment `json:"images,omitempty"`
}

// ForkParams copies a session's log up to Seq into a new session.
type ForkParams struct {
	ID  string `json:"id"`
	Seq int64  `json:"seq"`
}

// TurnParams names one turn of a session.
type TurnParams struct {
	ID   string `json:"id"`
	Turn int    `json:"turn"`
}
