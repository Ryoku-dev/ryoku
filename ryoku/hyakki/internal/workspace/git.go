package workspace

import (
	"context"
	"strings"

	i18n "ryoku-i18n"
	gitpkg "ryoku-rashin/internal/git"
	"ryoku-rashin/internal/protocol"
)

func (w *Work) gitStatus(ctx context.Context, path string, refresh bool) (protocol.GitStatusResult, error) {
	p, repo, err := w.projectForPath(ctx, path)
	if err != nil {
		return protocol.GitStatusResult{}, err
	}
	listed, err := repo.Worktrees(ctx)
	if err != nil {
		return protocol.GitStatusResult{}, err
	}
	base, err := resolveBase(ctx, p, repo, listed)
	if err != nil {
		return protocol.GitStatusResult{}, err
	}
	w.mu.RLock()
	for _, owned := range p.Worktrees {
		if samePath(owned.Path, path) && owned.Base != "" {
			base = owned.Base
			break
		}
	}
	w.mu.RUnlock()
	return w.cachedStatus(ctx, path, base, refresh)
}

func (w *Work) gitLog(ctx context.Context, params protocol.GitLogParams) (protocol.GitLogResult, error) {
	p, err := w.project(params.Project)
	if err != nil {
		return protocol.GitLogResult{}, err
	}
	if !p.Git {
		return protocol.GitLogResult{}, badRequest("The project is not a Git repository.")
	}
	repo, err := gitpkg.Open(ctx, p.Root)
	if err != nil {
		return protocol.GitLogResult{}, err
	}
	commits, next, err := repo.Log(ctx, gitpkg.LogQuery{
		Scope: params.Scope, Ref: params.Ref, Worktree: params.Worktree,
		Search: params.Search, Limit: params.Limit, Cursor: params.Cursor,
	})
	if err != nil {
		return protocol.GitLogResult{}, unavailable("The commit graph could not be read: %v.", err)
	}
	result := protocol.GitLogResult{Commits: commits, Next: next}
	if params.Cursor == "" {
		result.Refs, err = repo.Refs(ctx)
		if err != nil {
			return protocol.GitLogResult{}, err
		}
		listed, listErr := w.listWorktrees(ctx, p.ID, true)
		if listErr != nil {
			return protocol.GitLogResult{}, listErr
		}
		w.mu.RLock()
		sessions := w.sessions
		w.mu.RUnlock()
		for _, wt := range listed.Worktrees {
			if wt.Status == nil {
				continue
			}
			agent := ""
			if sessions != nil {
				for _, session := range sessions.List(protocol.KindAgent) {
					if samePath(session.Worktree, wt.Path) && session.State != protocol.StateStopped {
						agent = string(session.State)
						break
					}
				}
			}
			if wt.Status.Staged+wt.Status.Modified+wt.Status.Untracked+wt.Status.Conflicted > 0 || agent != "" {
				result.Wip = append(result.Wip, protocol.WipRow{
					Worktree: wt.Path, Branch: wt.Branch, Head: wt.Head, Staged: wt.Status.Staged,
					Modified: wt.Status.Modified, Untracked: wt.Status.Untracked,
					Conflicted: wt.Status.Conflicted, Agent: agent,
				})
			}
		}
	}
	return result, nil
}

func (w *Work) gitDiff(ctx context.Context, params protocol.DiffParams) (protocol.DiffResult, error) {
	worktree := params.Worktree
	if worktree == "" && params.Project != "" {
		p, err := w.project(params.Project)
		if err != nil {
			return protocol.DiffResult{}, err
		}
		worktree = p.Root
	}
	if worktree == "" {
		return protocol.DiffResult{}, badRequest("A worktree is required for the diff.")
	}
	options := gitpkg.DiffOptions{
		Mode: params.Mode, Commit: params.Commit, From: params.From, To: params.To,
		Path: params.Path, Context: params.Context,
	}
	if params.Mode == protocol.DiffBase {
		p, repo, err := w.projectForPath(ctx, worktree)
		if err != nil {
			return protocol.DiffResult{}, err
		}
		listed, err := repo.Worktrees(ctx)
		if err != nil {
			return protocol.DiffResult{}, err
		}
		options.Base, err = resolveBase(ctx, p, repo, listed)
		if err != nil {
			return protocol.DiffResult{}, err
		}
		w.mu.RLock()
		for _, owned := range p.Worktrees {
			if samePath(owned.Path, worktree) && owned.Base != "" {
				options.Base = owned.Base
				break
			}
		}
		w.mu.RUnlock()
	}
	if params.Mode != protocol.DiffCheckpoint {
		return gitpkg.Diff(ctx, worktree, options)
	}
	if params.Session == "" || params.Turn <= 0 {
		return protocol.DiffResult{}, badRequest("A session and turn are required for a checkpoint diff.")
	}
	from := checkpointRef(params.Session, params.Turn)
	to := checkpointRef(params.Session, params.Turn+1)
	if _, err := gitpkg.Run(ctx, worktree, "rev-parse", "--verify", to+"^{commit}"); err != nil {
		to = checkpointPostRef(params.Session, params.Turn)
		if _, postErr := gitpkg.Run(ctx, worktree, "rev-parse", "--verify", to+"^{commit}"); postErr != nil {
			tmp := "refs/rashin/tmp/" + newID("diff")
			if _, snapshotErr := gitpkg.Checkpoint(ctx, worktree, tmp); snapshotErr != nil {
				return protocol.DiffResult{}, unavailable("The current worktree snapshot could not be created: %v.", snapshotErr)
			}
			defer gitpkg.DeleteRefs(ctx, worktree, tmp)
			to = tmp
		}
	}
	options.Mode, options.From, options.To = protocol.DiffRange, from, to
	return gitpkg.Diff(ctx, worktree, options)
}

func checkpointRef(session string, turn int) string {
	return "refs/rashin/checkpoints/" + session + "/" + strings.TrimSpace(intString(turn))
}

func checkpointPostRef(session string, turn int) string {
	return checkpointRef(session, turn) + "-post"
}

func intString(value int) string {
	if value == 0 {
		return "0"
	}
	negative := value < 0
	if negative {
		value = -value
	}
	var digits [20]byte
	index := len(digits)
	for value > 0 {
		index--
		digits[index] = byte('0' + value%10)
		value /= 10
	}
	if negative {
		index--
		digits[index] = '-'
	}
	return string(digits[index:])
}

func (w *Work) mutateStatus(ctx context.Context, path, action string, fn func() error) (protocol.GitStatusResult, error) {
	p, _, err := w.projectForPath(ctx, path)
	if err != nil {
		return protocol.GitStatusResult{}, err
	}
	if err := w.withMutation(p.ID, action, path, fn); err != nil {
		return protocol.GitStatusResult{}, conflict("The Git operation failed: %v.", err)
	}
	result, err := w.gitStatus(ctx, path, true)
	if err == nil {
		w.emitGitChanged(p.ID, path)
	}
	return result, err
}

func (w *Work) mutateGitResult(ctx context.Context, path, action string, fn func() (protocol.GitOpResult, error)) (protocol.GitOpResult, error) {
	p, _, err := w.projectForPath(ctx, path)
	if err != nil {
		return protocol.GitOpResult{}, err
	}
	w.busy.Add(1)
	result, runErr := fn()
	w.busy.Add(-1)
	w.recordActivity(p.ID, "you", action, path, runErr == nil, errorDetail(runErr))
	if runErr != nil {
		return result, conflict("The Git operation failed: %v.", runErr)
	}
	w.emitGitChanged(p.ID, path)
	return result, nil
}

func (w *Work) emitGitChanged(project, worktree string) {
	w.broadcast(protocol.EvtGitChanged, protocol.GitChanged{Project: project, Worktree: worktree})
	if listed, err := w.listWorktrees(context.Background(), project, true); err == nil {
		w.broadcast(protocol.EvtWorktreesChanged, listed)
	}
}

func (w *Work) gitCommit(ctx context.Context, params protocol.GitCommitWriteParams) (protocol.GitOpResult, error) {
	if strings.TrimSpace(params.Message) == "" {
		return protocol.GitOpResult{}, badRequest("A commit message is required.")
	}
	return w.mutateGitResult(ctx, params.Path, "git.commit", func() (protocol.GitOpResult, error) {
		return gitpkg.Commit(ctx, params.Path, params.Message, params.Amend)
	})
}

func (w *Work) gitProjectRepo(ctx context.Context, project string) (*projectRecord, *gitpkg.Repo, error) {
	p, err := w.project(project)
	if err != nil {
		return nil, nil, err
	}
	if !p.Git {
		return nil, nil, badRequest("The project is not a Git repository.")
	}
	repo, err := gitpkg.Open(ctx, p.Root)
	return p, repo, err
}

func (w *Work) gitRefRepo(ctx context.Context, params protocol.GitRefParams) (*projectRecord, *gitpkg.Repo, error) {
	if params.Project != "" {
		return w.gitProjectRepo(ctx, params.Project)
	}
	if params.Path != "" {
		return w.projectForPath(ctx, params.Path)
	}
	return nil, nil, badRequest("A project or worktree is required.")
}

func okResult(output string) protocol.GitOpResult {
	return protocol.GitOpResult{OK: true, Output: output}
}

func gitError(err error) error {
	if err == nil {
		return nil
	}
	return protocol.Errorf(protocol.CodeConflict, "%s", i18n.Tf("The Git operation failed: %v.", err))
}
