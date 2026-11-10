// Package git drives the git command for Rashin's Work layer: repository and
// worktree discovery, porcelain status, diffs with hunks, the paged commit log
// and refs, safe mutations, hidden-ref checkpoints and worktree ports. Every
// call runs git with machine formats (-z, --porcelain=v2) and never scrapes
// human output. Workspace builds on it; it holds no state of its own.
//
// This file is the seam the Workspace package codes against; the package owner
// replaces the bodies.
package git

import (
	"context"

	"ryoku-rashin/internal/protocol"
)

// Run executes git in dir with args and returns stdout. A failure carries
// git's stderr in the error message.
func Run(ctx context.Context, dir string, args ...string) ([]byte, error) {
	return run(ctx, dir, args...)
}

// Repo is one repository, addressed through any of its worktrees.
type Repo struct {
	// Root is the main worktree's top level.
	Root string
	// CommonDir is the shared git directory.
	CommonDir string
}

// Open resolves the repository containing path. It fails when path is not in a
// Git repository.
func Open(ctx context.Context, path string) (*Repo, error) { return openRepo(ctx, path) }

// TopLevel returns the top level of the worktree containing path.
func TopLevel(ctx context.Context, path string) (string, error) {
	return topLevel(ctx, path)
}

// Worktree is one record of `git worktree list --porcelain -z`.
type Worktree struct {
	Path        string
	Head        string
	Branch      string
	Main        bool
	Detached    bool
	Bare        bool
	Locked      bool
	LockReason  string
	Prunable    bool
	PruneReason string
	Missing     bool
}

// Worktrees lists the repository's worktrees, main first.
func (r *Repo) Worktrees(ctx context.Context) ([]Worktree, error) {
	return r.worktrees(ctx)
}

// AddOptions create a worktree. Mode is protocol.WorktreeNewBranch (Branch is
// created at Base), WorktreeExistingBranch (Branch is checked out) or
// WorktreeDetached (Base is checked out detached).
type AddOptions struct {
	Path       string
	Mode       string
	Branch     string
	Base       string
	Lock       bool
	LockReason string
}

// AddWorktree runs git worktree add. It never forces; a branch checked out
// elsewhere is an error naming that worktree.
func (r *Repo) AddWorktree(ctx context.Context, o AddOptions) error {
	return r.addWorktree(ctx, o)
}

// RemoveWorktree runs git worktree remove with force repeated force times (0,
// 1 for dirty trees, 2 for locked ones).
func (r *Repo) RemoveWorktree(ctx context.Context, path string, force int) error {
	return r.removeWorktree(ctx, path, force)
}

// LockWorktree locks (or with lock false unlocks) a worktree.
func (r *Repo) LockWorktree(ctx context.Context, path string, lock bool, reason string) error {
	return r.lockWorktree(ctx, path, lock, reason)
}

// MoveWorktree moves a worktree folder.
func (r *Repo) MoveWorktree(ctx context.Context, from, to string) error {
	return r.moveWorktree(ctx, from, to)
}

// RepairWorktrees repairs worktree links.
func (r *Repo) RepairWorktrees(ctx context.Context) error { return r.repairWorktrees(ctx) }

// PruneWorktrees prunes missing worktree records and returns what it pruned;
// with dryRun it only reports.
func (r *Repo) PruneWorktrees(ctx context.Context, dryRun bool) ([]string, error) {
	return r.pruneWorktrees(ctx, dryRun)
}

// RemovePreview reports what removing wt would lose, measured against base
// (the ref the worktree branched from, or the project base).
func (r *Repo) RemovePreview(ctx context.Context, wt Worktree, base string) (protocol.RemovePreview, error) {
	return r.removePreview(ctx, wt, base)
}

// Status reads a worktree's porcelain v2 status with branch headers, the
// operation in progress and per-file line counts. base, when set, also fills
// AheadBase, BehindBase, Added and Removed against it.
func Status(ctx context.Context, worktree, base string) (protocol.GitStatusResult, error) {
	return status(ctx, worktree, base)
}

// Operation reports the operation in progress in a worktree (protocol.OpMerge
// and friends) or "".
func Operation(ctx context.Context, worktree string) string { return operation(ctx, worktree) }

// DiffOptions select a diff. Mode is protocol.DiffWorking, DiffStaged,
// DiffBase (Base...working tree, committed and not), DiffCommit (Commit
// against its first parent) or DiffRange (From..To between two commits; to
// compare with the working tree including untracked files, snapshot it with
// Checkpoint first). Path limits it to one file. MaxBytes caps each file's
// patch (0 means 256 KiB).
type DiffOptions struct {
	Mode     string
	Base     string
	Commit   string
	From     string
	To       string
	Path     string
	Context  int
	MaxBytes int
}

// Diff returns parsed file diffs with hunks and line numbers.
func Diff(ctx context.Context, worktree string, o DiffOptions) (protocol.DiffResult, error) {
	return diff(ctx, worktree, o)
}

// LogQuery pages the commit log newest first in topological order. Scope is
// protocol.ScopeAll (every ref), ScopeBranch (Ref) or ScopeWorktree (the HEAD
// of Worktree). Cursor is the opaque Next of the previous page.
type LogQuery struct {
	Scope    string
	Ref      string
	Worktree string
	Search   string
	Limit    int
	Cursor   string
}

// Log returns one page of commits and the cursor of the next page ("" at the
// end).
func (r *Repo) Log(ctx context.Context, q LogQuery) ([]protocol.GitCommit, string, error) {
	return r.log(ctx, q)
}

// Refs lists branches, remote branches, tags and every worktree's HEAD, with
// the worktree each branch is checked out in.
func (r *Repo) Refs(ctx context.Context) ([]protocol.GitRef, error) { return r.refs(ctx) }

// Show returns one commit with its message, refs and changed files.
func (r *Repo) Show(ctx context.Context, hash string) (protocol.GitCommitDetail, error) {
	return r.show(ctx, hash)
}

// Stage stages files (or every change with all).
func Stage(ctx context.Context, worktree string, files []string, all bool) error {
	return stage(ctx, worktree, files, all)
}

// Unstage unstages files (or everything with all).
func Unstage(ctx context.Context, worktree string, files []string, all bool) error {
	return unstage(ctx, worktree, files, all)
}

// Discard drops changes to files: tracked files return to HEAD, untracked ones
// are deleted. all applies it to every change.
func Discard(ctx context.Context, worktree string, files []string, all bool) error {
	return discard(ctx, worktree, files, all)
}

// Commit commits the staged changes.
func Commit(ctx context.Context, worktree, message string, amend bool) (protocol.GitOpResult, error) {
	return commit(ctx, worktree, message, amend)
}

// Push pushes the worktree's branch. It never forces unless forceWithLease.
func Push(ctx context.Context, worktree string, setUpstream, forceWithLease bool) (protocol.GitOpResult, error) {
	return push(ctx, worktree, setUpstream, forceWithLease)
}

// Fetch fetches every remote.
func Fetch(ctx context.Context, dir string) (protocol.GitOpResult, error) {
	return fetch(ctx, dir)
}

// Pull fast-forwards the worktree's branch from its upstream; it never merges.
func Pull(ctx context.Context, worktree string) (protocol.GitOpResult, error) {
	return pull(ctx, worktree)
}

// CreateBranch creates name at the commit at.
func (r *Repo) CreateBranch(ctx context.Context, name, at string) error {
	return r.createBranch(ctx, name, at)
}

// DeleteBranch deletes name (-d, or -D with force).
func (r *Repo) DeleteBranch(ctx context.Context, name string, force bool) error {
	return r.deleteBranch(ctx, name, force)
}

// Checkout switches a worktree to ref; it refuses a dirty worktree.
func Checkout(ctx context.Context, worktree, ref string) (protocol.GitOpResult, error) {
	return checkout(ctx, worktree, ref)
}

// Merge merges ref into the worktree's branch.
func Merge(ctx context.Context, worktree, ref string, ffOnly, noFF bool) (protocol.GitOpResult, error) {
	return merge(ctx, worktree, ref, ffOnly, noFF)
}

// CherryPick applies commits in order.
func CherryPick(ctx context.Context, worktree string, commits []string) (protocol.GitOpResult, error) {
	return cherryPick(ctx, worktree, commits)
}

// Revert reverts commits in order.
func Revert(ctx context.Context, worktree string, commits []string) (protocol.GitOpResult, error) {
	return revert(ctx, worktree, commits)
}

// Rebase rebases the worktree's branch onto ref without an editor.
func Rebase(ctx context.Context, worktree, onto string) (protocol.GitOpResult, error) {
	return rebase(ctx, worktree, onto)
}

// Abort aborts the operation in progress.
func Abort(ctx context.Context, worktree string) (protocol.GitOpResult, error) {
	return abort(ctx, worktree)
}

// Continue continues the operation in progress without an editor.
func Continue(ctx context.Context, worktree string) (protocol.GitOpResult, error) {
	return continueOperation(ctx, worktree)
}

// Checkpoint snapshots the worktree's files (tracked and untracked, ignored
// excluded) into the hidden ref without touching the index, HEAD or branch,
// and returns the snapshot commit.
func Checkpoint(ctx context.Context, worktree, ref string) (string, error) {
	return checkpoint(ctx, worktree, ref)
}

// Restore makes the worktree's files match the snapshot at ref: changed files
// return to it and files created after it are deleted. HEAD, the index and
// ignored files are untouched.
func Restore(ctx context.Context, worktree, ref string) error { return restore(ctx, worktree, ref) }

// DeleteRefs deletes hidden refs under prefix (a session's checkpoints).
func DeleteRefs(ctx context.Context, dir, prefix string) error { return deleteRefs(ctx, dir, prefix) }

// Ports lists ports listened on by processes whose working directory is inside
// worktree, read from /proc.
func Ports(worktree string) ([]protocol.Port, error) { return ports(worktree) }
