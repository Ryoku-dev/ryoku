package git

import (
	"context"
	"errors"
	"strings"

	i18n "ryoku-i18n"
)

func rejectOption(value, kind string) error {
	if strings.HasPrefix(value, "-") {
		return errors.New(i18n.Tf("%s cannot start with '-': %q", kind, value))
	}
	return nil
}

func resolveCommit(ctx context.Context, dir, value string) (string, error) {
	if value == "" {
		return "", errors.New(i18n.T("Revision is required"))
	}
	if err := rejectOption(value, "revision"); err != nil {
		return "", err
	}
	out, err := run(ctx, dir, "rev-parse", "--verify", "--end-of-options", value+"^{commit}")
	if err != nil {
		return "", err
	}
	return trimOutput(out), nil
}

func validateBranch(ctx context.Context, dir, name string) error {
	if name == "" {
		return errors.New(i18n.T("Branch name is required"))
	}
	if err := rejectOption(name, "branch name"); err != nil {
		return err
	}
	_, err := run(ctx, dir, "check-ref-format", "--branch", name)
	return err
}

func diffPaths(oldPath, path string) []string {
	if oldPath != "" && oldPath != path {
		return []string{oldPath, path}
	}
	return []string{path}
}
