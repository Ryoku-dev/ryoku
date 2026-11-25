package git

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	i18n "ryoku-i18n"

	"ryoku-rashin/internal/protocol"
)

func topLevel(ctx context.Context, path string) (string, error) {
	out, err := run(ctx, path, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", err
	}
	return filepath.Abs(trimOutput(out))
}

func openRepo(ctx context.Context, path string) (*Repo, error) {
	top, err := topLevel(ctx, path)
	if err != nil {
		return nil, err
	}
	commonOut, err := run(ctx, path, "rev-parse", "--git-common-dir")
	if err != nil {
		return nil, err
	}
	common := trimOutput(commonOut)
	if !filepath.IsAbs(common) {
		common = filepath.Join(top, common)
	}
	common, err = filepath.Abs(common)
	if err != nil {
		return nil, err
	}
	temporary := &Repo{Root: top, CommonDir: filepath.Clean(common)}
	worktrees, err := temporary.worktrees(ctx)
	if err != nil {
		return nil, err
	}
	if len(worktrees) == 0 {
		return nil, errors.New(i18n.T("Git worktree list returned no worktrees"))
	}
	temporary.Root = worktrees[0].Path
	return temporary, nil
}

func (r *Repo) worktrees(ctx context.Context) ([]Worktree, error) {
	out, err := run(ctx, r.Root, "worktree", "list", "--porcelain", "-z")
	if err != nil {
		return nil, err
	}
	return parseWorktrees(out), nil
}

func parseWorktrees(out []byte) []Worktree {
	fields := strings.Split(string(out), "\x00")
	result := make([]Worktree, 0)
	var current *Worktree
	finish := func() {
		if current == nil {
			return
		}
		_, err := os.Stat(current.Path)
		current.Missing = err != nil && os.IsNotExist(err)
		current.Main = len(result) == 0
		result = append(result, *current)
		current = nil
	}
	for _, field := range fields {
		if field == "" {
			finish()
			continue
		}
		key, value, found := strings.Cut(field, " ")
		if key == "worktree" {
			finish()
			current = &Worktree{Path: value}
			continue
		}
		if current == nil {
			continue
		}
		if !found {
			key = field
			value = ""
		}
		switch key {
		case "HEAD":
			current.Head = value
		case "branch":
			current.Branch = strings.TrimPrefix(value, "refs/heads/")
		case "bare":
			current.Bare = true
		case "detached":
			current.Detached = true
		case "locked":
			current.Locked, current.LockReason = true, value
		case "prunable":
			current.Prunable, current.PruneReason = true, value
		}
	}
	finish()
	return result
}

func (r *Repo) addWorktree(ctx context.Context, o AddOptions) error {
	if o.Path == "" {
		return errors.New(i18n.T("Worktree path is required"))
	}
	if o.Mode == protocol.WorktreeExistingBranch {
		worktrees, err := r.worktrees(ctx)
		if err != nil {
			return err
		}
		for _, wt := range worktrees {
			if wt.Branch == o.Branch {
				return errors.New(i18n.Tf("Branch %q is already checked out in worktree %s", o.Branch, wt.Path))
			}
		}
	}
	args := []string{"worktree", "add"}
	if o.Lock {
		args = append(args, "--lock")
		if o.LockReason != "" {
			args = append(args, "--reason", o.LockReason)
		}
	}
	switch o.Mode {
	case protocol.WorktreeNewBranch:
		if err := validateBranch(ctx, r.Root, o.Branch); err != nil {
			return err
		}
		base := o.Base
		if base == "" {
			base = "HEAD"
		}
		commit, err := resolveCommit(ctx, r.Root, base)
		if err != nil {
			return err
		}
		args = append(args, "-b", o.Branch, "--end-of-options", o.Path, commit)
	case protocol.WorktreeExistingBranch:
		if err := validateBranch(ctx, r.Root, o.Branch); err != nil {
			return err
		}
		if _, err := resolveCommit(ctx, r.Root, "refs/heads/"+o.Branch); err != nil {
			return err
		}
		args = append(args, "--end-of-options", o.Path, o.Branch)
	case protocol.WorktreeDetached:
		base := o.Base
		if base == "" {
			base = "HEAD"
		}
		commit, err := resolveCommit(ctx, r.Root, base)
		if err != nil {
			return err
		}
		args = append(args, "--detach", "--end-of-options", o.Path, commit)
	default:
		return errors.New(i18n.Tf("Unsupported worktree mode %q", o.Mode))
	}
	_, err := run(ctx, r.Root, args...)
	return err
}

func (r *Repo) removeWorktree(ctx context.Context, path string, force int) error {
	if force < 0 || force > 2 {
		return errors.New(i18n.Tf("Invalid worktree force count %d", force))
	}
	args := []string{"worktree", "remove"}
	for i := 0; i < force; i++ {
		args = append(args, "--force")
	}
	args = append(args, path)
	_, err := run(ctx, r.Root, args...)
	return err
}

func (r *Repo) lockWorktree(ctx context.Context, path string, lock bool, reason string) error {
	args := []string{"worktree", "unlock", path}
	if lock {
		args = []string{"worktree", "lock"}
		if reason != "" {
			args = append(args, "--reason", reason)
		}
		args = append(args, path)
	}
	_, err := run(ctx, r.Root, args...)
	return err
}

func (r *Repo) moveWorktree(ctx context.Context, from, to string) error {
	_, err := run(ctx, r.Root, "worktree", "move", from, to)
	return err
}

func (r *Repo) repairWorktrees(ctx context.Context) error {
	_, err := run(ctx, r.Root, "worktree", "repair")
	return err
}

func (r *Repo) pruneWorktrees(ctx context.Context, dryRun bool) ([]string, error) {
	worktrees, err := r.worktrees(ctx)
	if err != nil {
		return nil, err
	}
	candidates := make([]Worktree, 0)
	for _, worktree := range worktrees {
		if !worktree.Main && !worktree.Locked && (worktree.Missing || worktree.Prunable) {
			candidates = append(candidates, worktree)
		}
	}
	args := []string{"worktree", "prune", "--verbose", "--expire=now"}
	if dryRun {
		args = append(args, "-n")
	}
	out, err := runCapture(ctx, r.Root, dryRun, args...)
	if err != nil {
		return nil, err
	}
	return parsePrunedWorktrees(out, candidates), nil
}

func parsePrunedWorktrees(out []byte, candidates []Worktree) []string {
	byAdmin := make(map[string]string)
	for _, worktree := range candidates {
		byAdmin[filepath.Base(worktree.Path)] = worktree.Path
	}
	used := make(map[string]bool)
	var result []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		const prefix = "Removing worktrees/"
		if !strings.HasPrefix(line, prefix) {
			continue
		}
		admin, _, found := strings.Cut(strings.TrimPrefix(line, prefix), ":")
		if !found {
			continue
		}
		path := byAdmin[admin]
		if path == "" {
			for _, worktree := range candidates {
				base := filepath.Base(worktree.Path)
				if !used[worktree.Path] && strings.HasPrefix(admin, base) {
					path = worktree.Path
					break
				}
			}
		}
		if path != "" && !used[path] {
			used[path] = true
			result = append(result, path)
		}
	}
	return result
}

func (r *Repo) removePreview(ctx context.Context, wt Worktree, base string) (protocol.RemovePreview, error) {
	preview := protocol.RemovePreview{Path: wt.Path, Branch: wt.Branch, Locked: wt.Locked, Main: wt.Main, CanRemove: !wt.Main}
	if wt.Main {
		preview.BlockReason = i18n.T("The main worktree cannot be removed")
		return preview, nil
	}
	statusResult, err := status(ctx, wt.Path, "")
	if err != nil {
		return preview, err
	}
	dirtySeen := make(map[string]bool)
	untrackedSeen := make(map[string]bool)
	for _, file := range statusResult.Files {
		if file.Status == protocol.ChangeUntracked {
			if !untrackedSeen[file.Path] {
				preview.Untracked = append(preview.Untracked, file.Path)
				untrackedSeen[file.Path] = true
			}
		} else if !dirtySeen[file.Path] {
			preview.Dirty = append(preview.Dirty, file.Path)
			dirtySeen[file.Path] = true
		}
	}
	branch := wt.Branch
	if branch == "" {
		branch = wt.Head
	}
	branchCommit, err := resolveCommit(ctx, wt.Path, branch)
	if err != nil {
		return preview, err
	}
	exclusions := make([]string, 0)
	seenExclusion := make(map[string]bool)
	addExclusion := func(commit string) {
		if commit != "" && !seenExclusion[commit] {
			exclusions = append(exclusions, "^"+commit)
			seenExclusion[commit] = true
		}
	}
	remoteOut, err := run(ctx, r.Root, "for-each-ref", "--format=%(objectname)%00", "refs/remotes")
	if err != nil {
		return preview, err
	}
	for _, remote := range splitNUL(remoteOut) {
		commit, resolveErr := resolveCommit(ctx, r.Root, strings.TrimSpace(remote))
		if resolveErr == nil {
			addExclusion(commit)
		}
	}
	localFormat := "%(refname:short)%00%(objectname)%1e"
	localOut, err := run(ctx, r.Root, "for-each-ref", "--format="+localFormat, "refs/heads")
	if err != nil {
		return preview, err
	}
	for _, record := range strings.Split(string(localOut), "\x1e") {
		record = strings.TrimPrefix(record, "\n")
		fields := strings.Split(record, "\x00")
		if len(fields) < 2 || fields[0] == wt.Branch {
			continue
		}
		commit, resolveErr := resolveCommit(ctx, r.Root, fields[1])
		if resolveErr == nil {
			addExclusion(commit)
		}
	}
	var baseCommit string
	if base != "" {
		baseCommit, err = resolveCommit(ctx, wt.Path, base)
		if err != nil {
			return preview, err
		}
		addExclusion(baseCommit)
	}
	revisions := append([]string{branchCommit}, exclusions...)
	countArgs := append([]string{"rev-list", "--count", "--end-of-options"}, revisions...)
	countOut, countErr := run(ctx, wt.Path, countArgs...)
	if countErr != nil {
		return preview, countErr
	}
	fullCount := parseCount(trimOutput(countOut))
	const subjectLimit = 100
	logArgs := []string{"log", "--format=%s", "--max-count=" + strconv.Itoa(subjectLimit), "--end-of-options"}
	logArgs = append(logArgs, revisions...)
	out, logErr := run(ctx, wt.Path, logArgs...)
	if logErr != nil {
		return preview, logErr
	}
	for _, subject := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if subject != "" {
			preview.Unpushed = append(preview.Unpushed, subject)
		}
	}
	if fullCount > subjectLimit {
		preview.Unpushed = append(preview.Unpushed, i18n.Tf("... and %d more", fullCount-subjectLimit))
	}
	if baseCommit != "" {
		_, mergeErr := run(ctx, wt.Path, "merge-base", "--is-ancestor", "--end-of-options", branchCommit, baseCommit)
		if mergeErr != nil {
			var commandErr *commandError
			var exitErr *exec.ExitError
			if errors.As(mergeErr, &commandErr) && errors.As(commandErr.err, &exitErr) && exitErr.ExitCode() == 1 {
				preview.Unmerged = true
			} else {
				return preview, mergeErr
			}
		}
	}
	preview.NeedsForce = len(preview.Dirty) > 0 ||
		len(preview.Untracked) > 0 ||
		len(preview.Unpushed) > 0 ||
		preview.Unmerged ||
		wt.Locked
	return preview, nil
}

func parseCount(value string) int {
	n, _ := strconv.Atoi(value)
	return n
}
