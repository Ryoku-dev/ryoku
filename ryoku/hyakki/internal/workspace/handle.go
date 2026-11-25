package workspace

import (
	"context"
	"encoding/json"
	"errors"

	i18n "ryoku-i18n"
	gitpkg "ryoku-rashin/internal/git"
	"ryoku-rashin/internal/protocol"
)

func (w *Work) Handle(ctx context.Context, method string, raw json.RawMessage) (result any, err error) {
	defer func() {
		if err == nil {
			return
		}
		var protocolError *protocol.Error
		if !errors.As(err, &protocolError) {
			err = protocol.Errorf(protocol.CodeUnavailable, "%s", i18n.Tf("Work is unavailable: %v.", err))
		}
	}()
	switch method {
	case protocol.MProjectsList:
		return protocol.ProjectsResult{Projects: w.Projects(ctx)}, nil
	case protocol.MProjectsAdd:
		p, err := decode[protocol.ProjectAddParams](raw)
		if err != nil {
			return nil, err
		}
		return w.addProject(ctx, p)
	case protocol.MProjectsRemove:
		p, err := decode[protocol.ProjectRef](raw)
		if err != nil {
			return nil, err
		}
		return protocol.Empty{}, w.removeProject(ctx, p.Project)
	case protocol.MProjectsSettings:
		p, err := decode[protocol.ProjectSettingsParams](raw)
		if err != nil {
			return nil, err
		}
		return w.updateProjectSettings(ctx, p)
	case protocol.MProjectsWatch:
		p, err := decode[protocol.WatchParams](raw)
		if err != nil {
			return nil, err
		}
		if _, err := w.project(p.Project); err != nil {
			return nil, err
		}
		w.Watch(p.Project, p.Watch)
		return protocol.Empty{}, nil
	case protocol.MWorktreesList:
		p, err := decode[protocol.ProjectRef](raw)
		if err != nil {
			return nil, err
		}
		return w.listWorktrees(ctx, p.Project, false)
	case protocol.MWorktreesCreate:
		p, err := decode[protocol.WorktreeCreateParams](raw)
		if err != nil {
			return nil, err
		}
		return w.createWorktree(ctx, p, "you")
	case protocol.MWorktreesPreviewRemove:
		p, err := decode[protocol.WorktreeRef](raw)
		if err != nil {
			return nil, err
		}
		return w.previewRemove(ctx, p.Path)
	case protocol.MWorktreesRemove:
		p, err := decode[protocol.WorktreeRemoveParams](raw)
		if err != nil {
			return nil, err
		}
		return protocol.Empty{}, w.removeWorktree(ctx, p, "you")
	case protocol.MWorktreesLock:
		p, err := decode[protocol.WorktreeLockParams](raw)
		if err != nil {
			return nil, err
		}
		return w.lockWorktree(ctx, p)
	case protocol.MWorktreesMove:
		p, err := decode[protocol.WorktreeMoveParams](raw)
		if err != nil {
			return nil, err
		}
		return w.moveWorktree(ctx, p)
	case protocol.MWorktreesRepair:
		p, err := decode[protocol.ProjectRef](raw)
		if err != nil {
			return nil, err
		}
		return w.repairWorktrees(ctx, p.Project)
	case protocol.MWorktreesPrune:
		p, err := decode[protocol.PruneParams](raw)
		if err != nil {
			return nil, err
		}
		return w.pruneWorktrees(ctx, p)
	case protocol.MWorktreesCleanup:
		p, err := decode[protocol.CleanupParams](raw)
		if err != nil {
			return nil, err
		}
		return w.cleanupWorktrees(ctx, p)
	case protocol.MWorktreesSetup:
		p, err := decode[protocol.WorktreeSetupParams](raw)
		if err != nil {
			return nil, err
		}
		return w.setupWorktreeMode(ctx, p.Path, "", p.Rerun)
	case protocol.MWorktreesTrust:
		p, err := decode[protocol.TrustParams](raw)
		if err != nil {
			return nil, err
		}
		return w.trustSetup(ctx, p)
	case protocol.MWorktreesPorts:
		p, err := decode[protocol.WorktreeRef](raw)
		if err != nil {
			return nil, err
		}
		return w.ports(ctx, p.Path)
	case protocol.MGitStatus:
		p, err := decode[protocol.WorktreeRef](raw)
		if err != nil {
			return nil, err
		}
		return w.gitStatus(ctx, p.Path, true)
	case protocol.MGitLog:
		p, err := decode[protocol.GitLogParams](raw)
		if err != nil {
			return nil, err
		}
		return w.gitLog(ctx, p)
	case protocol.MGitShow:
		p, err := decode[protocol.GitCommitParams](raw)
		if err != nil {
			return nil, err
		}
		_, repo, err := w.gitProjectRepo(ctx, p.Project)
		if err != nil {
			return nil, err
		}
		return repo.Show(ctx, p.Hash)
	case protocol.MGitDiff:
		p, err := decode[protocol.DiffParams](raw)
		if err != nil {
			return nil, err
		}
		return w.gitDiff(ctx, p)
	case protocol.MGitStage:
		p, err := decode[protocol.GitPathsParams](raw)
		if err != nil {
			return nil, err
		}
		return w.mutateStatus(ctx, p.Path, "git.stage", func() error { return gitpkg.Stage(ctx, p.Path, p.Files, p.All) })
	case protocol.MGitUnstage:
		p, err := decode[protocol.GitPathsParams](raw)
		if err != nil {
			return nil, err
		}
		return w.mutateStatus(ctx, p.Path, "git.unstage", func() error { return gitpkg.Unstage(ctx, p.Path, p.Files, p.All) })
	case protocol.MGitDiscard:
		p, err := decode[protocol.GitPathsParams](raw)
		if err != nil {
			return nil, err
		}
		return w.mutateStatus(ctx, p.Path, "git.discard", func() error { return gitpkg.Discard(ctx, p.Path, p.Files, p.All) })
	case protocol.MGitCommit:
		p, err := decode[protocol.GitCommitWriteParams](raw)
		if err != nil {
			return nil, err
		}
		return w.gitCommit(ctx, p)
	case protocol.MGitPush:
		p, err := decode[protocol.GitPushParams](raw)
		if err != nil {
			return nil, err
		}
		return w.mutateGitResult(ctx, p.Path, "git.push", func() (protocol.GitOpResult, error) { return gitpkg.Push(ctx, p.Path, p.SetUpstream, p.ForceWithLease) })
	case protocol.MGitFetch:
		p, err := decode[protocol.ProjectRef](raw)
		if err != nil {
			return nil, err
		}
		project, _, err := w.gitProjectRepo(ctx, p.Project)
		if err != nil {
			return nil, err
		}
		w.busy.Add(1)
		result, runErr := gitpkg.Fetch(ctx, project.Root)
		w.busy.Add(-1)
		w.recordActivity(project.ID, "you", "git.fetch", project.Root, runErr == nil, errorDetail(runErr))
		if runErr != nil {
			return nil, gitError(runErr)
		}
		w.emitGitChanged(project.ID, "")
		return result, nil
	case protocol.MGitPull:
		p, err := decode[protocol.WorktreeRef](raw)
		if err != nil {
			return nil, err
		}
		return w.mutateGitResult(ctx, p.Path, "git.pull", func() (protocol.GitOpResult, error) { return gitpkg.Pull(ctx, p.Path) })
	case protocol.MGitBranchCreate:
		p, err := decode[protocol.GitRefParams](raw)
		if err != nil {
			return nil, err
		}
		project, repo, err := w.gitRefRepo(ctx, p)
		if err != nil {
			return nil, err
		}
		err = w.withMutation(project.ID, "git.branch_create", p.Ref, func() error { return repo.CreateBranch(ctx, p.Ref, p.At) })
		if err != nil {
			return nil, gitError(err)
		}
		w.emitGitChanged(project.ID, p.Path)
		return okResult(""), nil
	case protocol.MGitBranchDelete:
		p, err := decode[protocol.GitRefParams](raw)
		if err != nil {
			return nil, err
		}
		project, repo, err := w.gitRefRepo(ctx, p)
		if err != nil {
			return nil, err
		}
		err = w.withMutation(project.ID, "git.branch_delete", p.Ref, func() error { return repo.DeleteBranch(ctx, p.Ref, p.Force) })
		if err != nil {
			return nil, gitError(err)
		}
		w.emitGitChanged(project.ID, p.Path)
		return okResult(""), nil
	case protocol.MGitCheckout:
		p, err := decode[protocol.GitRefParams](raw)
		if err != nil {
			return nil, err
		}
		return w.mutateGitResult(ctx, p.Path, "git.checkout", func() (protocol.GitOpResult, error) { return gitpkg.Checkout(ctx, p.Path, p.Ref) })
	case protocol.MGitMerge:
		p, err := decode[protocol.GitApplyParams](raw)
		if err != nil {
			return nil, err
		}
		return w.mutateGitResult(ctx, p.Path, "git.merge", func() (protocol.GitOpResult, error) { return gitpkg.Merge(ctx, p.Path, p.Ref, p.FFOnly, p.NoFF) })
	case protocol.MGitCherryPick:
		p, err := decode[protocol.GitApplyParams](raw)
		if err != nil {
			return nil, err
		}
		return w.mutateGitResult(ctx, p.Path, "git.cherry_pick", func() (protocol.GitOpResult, error) { return gitpkg.CherryPick(ctx, p.Path, p.Commits) })
	case protocol.MGitRevert:
		p, err := decode[protocol.GitApplyParams](raw)
		if err != nil {
			return nil, err
		}
		return w.mutateGitResult(ctx, p.Path, "git.revert", func() (protocol.GitOpResult, error) { return gitpkg.Revert(ctx, p.Path, p.Commits) })
	case protocol.MGitRebase:
		p, err := decode[protocol.GitApplyParams](raw)
		if err != nil {
			return nil, err
		}
		return w.mutateGitResult(ctx, p.Path, "git.rebase", func() (protocol.GitOpResult, error) { return gitpkg.Rebase(ctx, p.Path, p.Ref) })
	case protocol.MGitAbort:
		p, err := decode[protocol.WorktreeRef](raw)
		if err != nil {
			return nil, err
		}
		return w.mutateGitResult(ctx, p.Path, "git.abort", func() (protocol.GitOpResult, error) { return gitpkg.Abort(ctx, p.Path) })
	case protocol.MGitContinue:
		p, err := decode[protocol.WorktreeRef](raw)
		if err != nil {
			return nil, err
		}
		return w.mutateGitResult(ctx, p.Path, "git.continue", func() (protocol.GitOpResult, error) { return gitpkg.Continue(ctx, p.Path) })
	case protocol.MReviewList:
		p, err := decode[protocol.WorktreeRef](raw)
		if err != nil {
			return nil, err
		}
		if _, _, err := w.projectForPath(ctx, p.Path); err != nil {
			return nil, err
		}
		return w.reviewList(p.Path), nil
	case protocol.MReviewAdd:
		p, err := decode[protocol.ReviewAddParams](raw)
		if err != nil {
			return nil, err
		}
		return w.reviewAdd(ctx, p)
	case protocol.MReviewResolve:
		p, err := decode[protocol.ReviewRef](raw)
		if err != nil {
			return nil, err
		}
		return w.reviewResolve(ctx, p)
	case protocol.MReviewDelete:
		p, err := decode[protocol.ReviewRef](raw)
		if err != nil {
			return nil, err
		}
		return protocol.Empty{}, w.reviewDelete(ctx, p)
	case protocol.MReviewSend:
		p, err := decode[protocol.ReviewSendParams](raw)
		if err != nil {
			return nil, err
		}
		return w.reviewSend(ctx, p)
	case protocol.MTasksList:
		p, err := decode[protocol.TasksParams](raw)
		if err != nil {
			return nil, err
		}
		return w.tasksList(p.Project)
	case protocol.MTasksCreate:
		p, err := decode[protocol.TaskCreateParams](raw)
		if err != nil {
			return nil, err
		}
		return w.taskCreate(p)
	case protocol.MTasksUpdate:
		p, err := decode[protocol.TaskUpdateParams](raw)
		if err != nil {
			return nil, err
		}
		return w.taskUpdate(p)
	case protocol.MTasksDelete:
		p, err := decode[protocol.TaskRef](raw)
		if err != nil {
			return nil, err
		}
		return protocol.Empty{}, w.taskDelete(p.ID)
	case protocol.MTasksStart:
		p, err := decode[protocol.TaskStartParams](raw)
		if err != nil {
			return nil, err
		}
		return w.taskStart(ctx, p)
	case protocol.MTasksGate:
		p, err := decode[protocol.GateParams](raw)
		if err != nil {
			return nil, err
		}
		return w.taskGate(p)
	case protocol.MTasksResolveGate:
		p, err := decode[protocol.GateResolveParams](raw)
		if err != nil {
			return nil, err
		}
		return w.resolveGate(ctx, p)
	case protocol.MTasksPlan:
		p, err := decode[protocol.TaskPlanParams](raw)
		if err != nil {
			return nil, err
		}
		return w.taskPlan(ctx, p)
	case protocol.MActivityList:
		p, err := decode[protocol.ActivityParams](raw)
		if err != nil {
			return nil, err
		}
		return w.listActivity(p)
	default:
		return nil, protocol.Errorf(protocol.CodeUnsupported, "%s", i18n.Tf("Work method %s is not supported.", method))
	}
}
