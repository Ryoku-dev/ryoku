package git

import (
	"context"
	"errors"
	"os/exec"
	"path/filepath"
	"strings"

	i18n "ryoku-i18n"

	"ryoku-rashin/internal/protocol"
)

func stage(ctx context.Context, worktree string, files []string, all bool) error {
	args := []string{"add"}
	if all {
		args = append(args, "-A")
	} else {
		if len(files) == 0 {
			return errors.New(i18n.T("Files are required"))
		}
		args = append(args, "--")
		args = append(args, files...)
	}
	_, err := run(ctx, worktree, args...)
	return err
}

func unstage(ctx context.Context, worktree string, files []string, all bool) error {
	if all {
		_, err := run(ctx, worktree, "reset", "HEAD", "--")
		return err
	}
	if len(files) == 0 {
		return errors.New(i18n.T("Files are required"))
	}
	args := []string{"restore", "--staged", "--"}
	args = append(args, files...)
	_, err := run(ctx, worktree, args...)
	return err
}

func discard(ctx context.Context, worktree string, files []string, all bool) error {
	if all {
		if _, err := run(ctx, worktree, "reset", "--hard", "HEAD"); err != nil {
			return err
		}
		_, err := run(ctx, worktree, "clean", "-fd")
		return err
	}
	if len(files) == 0 {
		return errors.New(i18n.T("Files are required"))
	}
	trackedOut, err := run(ctx, worktree, append([]string{"ls-files", "-z", "--"}, files...)...)
	if err != nil {
		return err
	}
	tracked := splitNUL(trackedOut)
	if len(tracked) > 0 {
		args := []string{"restore", "--source=HEAD", "--staged", "--worktree", "--"}
		args = append(args, tracked...)
		if _, err := run(ctx, worktree, args...); err != nil {
			return err
		}
	}
	cleanArgs := []string{"clean", "-fd", "--"}
	cleanArgs = append(cleanArgs, files...)
	_, err = run(ctx, worktree, cleanArgs...)
	return err
}

func commit(ctx context.Context, worktree, message string, amend bool) (protocol.GitOpResult, error) {
	args := []string{"commit", "-m", message}
	if amend {
		args = append(args, "--amend")
	}
	return perform(ctx, worktree, args...)
}

func push(ctx context.Context, worktree string, setUpstream, forceWithLease bool) (protocol.GitOpResult, error) {
	args := []string{"push"}
	if forceWithLease {
		args = append(args, "--force-with-lease")
	}
	if setUpstream {
		branchOut, err := run(ctx, worktree, "symbolic-ref", "--quiet", "--short", "HEAD")
		if err != nil {
			return protocol.GitOpResult{}, err
		}
		branch := trimOutput(branchOut)
		remote, err := pushRemote(ctx, worktree, branch)
		if err != nil {
			return protocol.GitOpResult{}, err
		}
		args = append(args, "--set-upstream", "--end-of-options", remote, branch)
	}
	return perform(ctx, worktree, args...)
}

func fetch(ctx context.Context, dir string) (protocol.GitOpResult, error) {
	return perform(ctx, dir, "fetch", "--all", "--prune")
}

func pull(ctx context.Context, worktree string) (protocol.GitOpResult, error) {
	return perform(ctx, worktree, "pull", "--ff-only")
}

func (r *Repo) createBranch(ctx context.Context, name, at string) error {
	if err := validateBranch(ctx, r.Root, name); err != nil {
		return err
	}
	if at == "" {
		at = "HEAD"
	}
	commit, err := resolveCommit(ctx, r.Root, at)
	if err != nil {
		return err
	}
	_, err = run(ctx, r.Root, "branch", "--end-of-options", name, commit)
	return err
}

func (r *Repo) deleteBranch(ctx context.Context, name string, force bool) error {
	if err := validateBranch(ctx, r.Root, name); err != nil {
		return err
	}
	flag := "-d"
	if force {
		flag = "-D"
	}
	_, err := run(ctx, r.Root, "branch", flag, "--end-of-options", name)
	return err
}

func checkout(ctx context.Context, worktree, ref string) (protocol.GitOpResult, error) {
	if _, err := resolveCommit(ctx, worktree, ref); err != nil {
		return protocol.GitOpResult{}, err
	}
	state, err := status(ctx, worktree, "")
	if err != nil {
		return protocol.GitOpResult{}, err
	}
	if len(state.Files) > 0 {
		return protocol.GitOpResult{}, errors.New(i18n.T("Worktree has uncommitted changes"))
	}
	return perform(ctx, worktree, "checkout", "--end-of-options", ref)
}

func merge(ctx context.Context, worktree, ref string, ffOnly, noFF bool) (protocol.GitOpResult, error) {
	commit, err := resolveCommit(ctx, worktree, ref)
	if err != nil {
		return protocol.GitOpResult{}, err
	}
	args := []string{"merge"}
	if ffOnly {
		args = append(args, "--ff-only")
	}
	if noFF {
		args = append(args, "--no-ff")
	}
	args = append(args, "--end-of-options", commit)
	return perform(ctx, worktree, args...)
}

func cherryPick(ctx context.Context, worktree string, commits []string) (protocol.GitOpResult, error) {
	resolved, err := resolveCommits(ctx, worktree, commits)
	if err != nil {
		return protocol.GitOpResult{}, err
	}
	return perform(ctx, worktree, append([]string{"cherry-pick", "--end-of-options"}, resolved...)...)
}

func revert(ctx context.Context, worktree string, commits []string) (protocol.GitOpResult, error) {
	resolved, err := resolveCommits(ctx, worktree, commits)
	if err != nil {
		return protocol.GitOpResult{}, err
	}
	return perform(ctx, worktree, append([]string{"revert", "--no-edit", "--end-of-options"}, resolved...)...)
}

func rebase(ctx context.Context, worktree, onto string) (protocol.GitOpResult, error) {
	commit, err := resolveCommit(ctx, worktree, onto)
	if err != nil {
		return protocol.GitOpResult{}, err
	}
	return perform(ctx, worktree, "rebase", "--end-of-options", commit)
}

func abort(ctx context.Context, worktree string) (protocol.GitOpResult, error) {
	op := operation(ctx, worktree)
	var args []string
	switch op {
	case protocol.OpMerge:
		args = []string{"merge", "--abort"}
	case protocol.OpRebase:
		args = []string{"rebase", "--abort"}
	case protocol.OpCherryPick:
		args = []string{"cherry-pick", "--abort"}
	case protocol.OpRevert:
		args = []string{"revert", "--abort"}
	case protocol.OpBisect:
		args = []string{"bisect", "reset"}
	default:
		return protocol.GitOpResult{OK: true}, nil
	}
	return perform(ctx, worktree, args...)
}

func continueOperation(ctx context.Context, worktree string) (protocol.GitOpResult, error) {
	op := operation(ctx, worktree)
	var args []string
	switch op {
	case protocol.OpMerge:
		args = []string{"commit", "--no-edit"}
	case protocol.OpRebase:
		args = []string{"rebase", "--continue"}
	case protocol.OpCherryPick:
		args = []string{"cherry-pick", "--continue"}
	case protocol.OpRevert:
		args = []string{"revert", "--continue"}
	default:
		return protocol.GitOpResult{}, errors.New(i18n.T("No Git operation is in progress"))
	}
	return perform(ctx, worktree, args...)
}

func resolveCommits(ctx context.Context, worktree string, commits []string) ([]string, error) {
	if len(commits) == 0 {
		return nil, errors.New(i18n.T("Commits are required"))
	}
	resolved := make([]string, len(commits))
	for index, commit := range commits {
		var err error
		resolved[index], err = resolveCommit(ctx, worktree, commit)
		if err != nil {
			return nil, err
		}
	}
	return resolved, nil
}

func pushRemote(ctx context.Context, worktree, branch string) (string, error) {
	keys := []string{"branch." + branch + ".pushRemote", "remote.pushDefault"}
	for _, key := range keys {
		out, err := run(ctx, worktree, "config", "--get", key)
		if err == nil {
			remote := trimOutput(out)
			if remote != "" {
				if err := rejectOption(remote, "remote name"); err != nil {
					return "", err
				}
				return remote, nil
			}
			continue
		}
		var commandErr *commandError
		var exitErr *exec.ExitError
		if !errors.As(err, &commandErr) || !errors.As(commandErr.err, &exitErr) || exitErr.ExitCode() != 1 {
			return "", err
		}
	}
	out, err := run(ctx, worktree, "remote")
	if err != nil {
		return "", err
	}
	remotes := strings.Fields(string(out))
	if len(remotes) != 1 {
		return "", errors.New(i18n.T("Choose a push remote by setting branch.<name>.pushRemote or remote.pushDefault"))
	}
	if err := rejectOption(remotes[0], "remote name"); err != nil {
		return "", err
	}
	return remotes[0], nil
}

func perform(ctx context.Context, worktree string, args ...string) (protocol.GitOpResult, error) {
	out, err := runCombined(ctx, worktree, args...)
	result := protocol.GitOpResult{OK: err == nil, Output: strings.TrimSpace(string(out)), Operation: operation(ctx, worktree)}
	state, statusErr := status(ctx, worktree, "")
	if statusErr == nil {
		seen := make(map[string]bool)
		for _, file := range state.Files {
			if file.Status == protocol.ChangeConflict && !seen[file.Path] {
				result.Conflicts = append(result.Conflicts, file.Path)
				seen[file.Path] = true
			}
		}
	}
	if err != nil && len(result.Conflicts) == 0 {
		return result, err
	}
	return result, nil
}

func splitNUL(data []byte) []string {
	parts := strings.Split(string(data), "\x00")
	result := parts[:0]
	for _, part := range parts {
		if part != "" {
			result = append(result, part)
		}
	}
	return result
}

func safeWorktreePath(root, relative string) (string, error) {
	if filepath.IsAbs(relative) {
		return "", errors.New(i18n.Tf("Absolute path is outside worktree: %s", relative))
	}
	root, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	candidate, err := filepath.Abs(filepath.Join(root, filepath.FromSlash(relative)))
	if err != nil {
		return "", err
	}
	if candidate != root && !strings.HasPrefix(candidate, root+string(filepath.Separator)) {
		return "", errors.New(i18n.Tf("Path is outside worktree: %s", relative))
	}
	return candidate, nil
}
