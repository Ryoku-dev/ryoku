package git

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"

	i18n "ryoku-i18n"
)

func temporaryIndex() (string, func(), error) {
	file, err := os.CreateTemp("", "rashin-git-index-*")
	if err != nil {
		return "", nil, err
	}
	name := file.Name()
	if err := file.Close(); err != nil {
		os.Remove(name)
		return "", nil, err
	}
	if err := os.Remove(name); err != nil {
		return "", nil, err
	}
	return name, func() { _ = os.Remove(name) }, nil
}

func checkpoint(ctx context.Context, worktree, ref string) (string, error) {
	if !strings.HasPrefix(ref, "refs/rashin/") {
		return "", errors.New(i18n.T("Checkpoint ref must be under refs/rashin/"))
	}
	index, cleanup, err := temporaryIndex()
	if err != nil {
		return "", err
	}
	defer cleanup()
	env := []string{"GIT_INDEX_FILE=" + index}
	if _, err := runEnv(ctx, worktree, env, "read-tree", "HEAD"); err != nil {
		return "", err
	}
	if _, err := runEnv(ctx, worktree, env, "add", "-A", "--", "."); err != nil {
		return "", err
	}
	treeOut, err := runEnv(ctx, worktree, env, "write-tree")
	if err != nil {
		return "", err
	}
	headOut, err := run(ctx, worktree, "rev-parse", "HEAD")
	if err != nil {
		return "", err
	}
	commitOut, err := runEnv(ctx, worktree, env, "commit-tree", trimOutput(treeOut), "-p", trimOutput(headOut), "-m", "rashin checkpoint")
	if err != nil {
		return "", err
	}
	commit := trimOutput(commitOut)
	if _, err := run(ctx, worktree, "update-ref", ref, commit); err != nil {
		return "", err
	}
	return commit, nil
}

func restore(ctx context.Context, worktree, ref string) error {
	if !strings.HasPrefix(ref, "refs/rashin/") {
		return errors.New(i18n.T("Checkpoint ref must be under refs/rashin/"))
	}
	targetOut, err := run(ctx, worktree, "ls-tree", "-r", "--name-only", "-z", ref)
	if err != nil {
		return err
	}
	currentOut, err := snapshotPaths(ctx, worktree)
	if err != nil {
		return err
	}
	target := make(map[string]bool)
	for _, path := range splitNUL(targetOut) {
		target[path] = true
	}
	for _, path := range splitNUL(currentOut) {
		if target[path] {
			continue
		}
		absolute, pathErr := safeWorktreePath(worktree, path)
		if pathErr != nil {
			return pathErr
		}
		if removeErr := os.RemoveAll(absolute); removeErr != nil {
			return removeErr
		}
	}
	index, cleanup, err := temporaryIndex()
	if err != nil {
		return err
	}
	defer cleanup()
	env := []string{"GIT_INDEX_FILE=" + index}
	if _, err := runEnv(ctx, worktree, env, "read-tree", ref); err != nil {
		return err
	}
	prefix, err := filepath.Abs(worktree)
	if err != nil {
		return err
	}
	prefix += string(filepath.Separator)
	_, err = runEnv(ctx, worktree, env, "checkout-index", "--all", "--force", "--prefix="+prefix)
	return err
}

func snapshotPaths(ctx context.Context, worktree string) ([]byte, error) {
	index, cleanup, err := temporaryIndex()
	if err != nil {
		return nil, err
	}
	defer cleanup()
	env := []string{"GIT_INDEX_FILE=" + index}
	if _, err := runEnv(ctx, worktree, env, "read-tree", "HEAD"); err != nil {
		return nil, err
	}
	if _, err := runEnv(ctx, worktree, env, "add", "-A", "--", "."); err != nil {
		return nil, err
	}
	return runEnv(ctx, worktree, env, "ls-files", "-z")
}

func deleteRefs(ctx context.Context, dir, prefix string) error {
	if !strings.HasPrefix(prefix, "refs/rashin/") {
		return errors.New(i18n.T("Checkpoint ref prefix must be under refs/rashin/"))
	}
	out, err := run(ctx, dir, "for-each-ref", "--format=%(refname)%00", prefix)
	if err != nil {
		return err
	}
	namespace := strings.TrimSuffix(prefix, "/")
	for _, ref := range splitNUL(out) {
		ref = strings.TrimSpace(ref)
		if ref == "" || ref != namespace && !strings.HasPrefix(ref, namespace+"/") {
			continue
		}
		if _, err := run(ctx, dir, "update-ref", "-d", ref); err != nil {
			return err
		}
	}
	return nil
}
