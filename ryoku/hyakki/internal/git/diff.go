package git

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	i18n "ryoku-i18n"

	"ryoku-rashin/internal/protocol"
)

var hunkHeader = regexp.MustCompile(`^@@ -(\d+)(?:,(\d+))? \+(\d+)(?:,(\d+))? @@`)

type diffSpec struct {
	options   []string
	revisions []string
}

type namedDiff struct {
	status  string
	path    string
	oldPath string
}

func diff(ctx context.Context, worktree string, o DiffOptions) (protocol.DiffResult, error) {
	result := protocol.DiffResult{Files: []protocol.FileDiff{}}
	spec, err := makeDiffSpec(ctx, worktree, o)
	if err != nil {
		return result, err
	}
	common := []string{"diff", "--no-color", "--no-ext-diff", "-M"}
	nameArgs := append(append([]string{}, common...), "--name-status", "-z")
	nameArgs = append(nameArgs, spec.options...)
	nameArgs = append(nameArgs, "--end-of-options")
	nameArgs = append(nameArgs, spec.revisions...)
	if o.Path != "" {
		nameArgs = append(nameArgs, "--", o.Path)
	}
	out, err := run(ctx, worktree, nameArgs...)
	if err != nil {
		return result, err
	}
	files, err := parseNameStatus(out)
	if err != nil {
		return result, err
	}
	limit := o.MaxBytes
	if limit <= 0 {
		limit = 256 << 10
	}
	contextLines := o.Context
	if contextLines < 0 {
		contextLines = 3
	}
	for _, named := range files {
		patchArgs := append(append([]string{}, common...), fmt.Sprintf("--unified=%d", contextLines))
		patchArgs = append(patchArgs, spec.options...)
		patchArgs = append(patchArgs, "--end-of-options")
		patchArgs = append(patchArgs, spec.revisions...)
		patchArgs = append(patchArgs, "--")
		patchArgs = append(patchArgs, diffPaths(named.oldPath, named.path)...)
		patch, patchErr := run(ctx, worktree, patchArgs...)
		if patchErr != nil {
			return result, patchErr
		}
		truncated := len(patch) > limit
		if truncated {
			patch = patch[:limit]
			result.Truncated = true
		}
		file := parsePatch(patch, named)
		if truncated && len(file.Hunks) > 0 {
			last := &file.Hunks[len(file.Hunks)-1]
			if len(last.Lines) > 0 && patch[len(patch)-1] != '\n' {
				last.Lines = last.Lines[:len(last.Lines)-1]
			}
		}
		result.Files = append(result.Files, file)
	}
	return result, nil
}

func makeDiffSpec(ctx context.Context, worktree string, o DiffOptions) (diffSpec, error) {
	switch o.Mode {
	case protocol.DiffWorking, "":
		return diffSpec{}, nil
	case protocol.DiffStaged:
		return diffSpec{options: []string{"--cached"}}, nil
	case protocol.DiffBase:
		if o.Base == "" {
			return diffSpec{}, errors.New(i18n.T("Base ref is required"))
		}
		base, err := resolveCommit(ctx, worktree, o.Base)
		if err != nil {
			return diffSpec{}, err
		}
		head, err := resolveCommit(ctx, worktree, "HEAD")
		if err != nil {
			return diffSpec{}, err
		}
		out, err := run(ctx, worktree, "merge-base", "--end-of-options", base, head)
		if err != nil {
			return diffSpec{}, err
		}
		return diffSpec{revisions: []string{trimOutput(out)}}, nil
	case protocol.DiffCommit:
		if o.Commit == "" {
			return diffSpec{}, errors.New(i18n.T("Commit is required"))
		}
		commit, err := resolveCommit(ctx, worktree, o.Commit)
		if err != nil {
			return diffSpec{}, err
		}
		out, err := run(ctx, worktree, "rev-list", "--parents", "-n", "1", "--end-of-options", commit)
		if err != nil {
			return diffSpec{}, err
		}
		parts := strings.Fields(string(out))
		parent := "4b825dc642cb6eb9a060e54bf8d69288fbee4904"
		if len(parts) > 1 {
			parent = parts[1]
		}
		return diffSpec{revisions: []string{parent, commit}}, nil
	case protocol.DiffRange, protocol.DiffCheckpoint:
		if o.From == "" || o.To == "" {
			return diffSpec{}, errors.New(i18n.T("From and to refs are required"))
		}
		from, err := resolveCommit(ctx, worktree, o.From)
		if err != nil {
			return diffSpec{}, err
		}
		to, err := resolveCommit(ctx, worktree, o.To)
		if err != nil {
			return diffSpec{}, err
		}
		return diffSpec{revisions: []string{from, to}}, nil
	default:
		return diffSpec{}, errors.New(i18n.Tf("Unsupported diff mode %q", o.Mode))
	}
}

func parseNameStatus(out []byte) ([]namedDiff, error) {
	fields := strings.Split(string(out), "\x00")
	result := make([]namedDiff, 0)
	for index := 0; index < len(fields); {
		status := fields[index]
		index++
		if status == "" {
			continue
		}
		kind := status[:1]
		if index >= len(fields) {
			return nil, errors.New(i18n.T("Incomplete Git name-status record"))
		}
		first := fields[index]
		index++
		entry := namedDiff{status: kind, path: first}
		if kind == protocol.ChangeRenamed || kind == protocol.ChangeCopied {
			if index >= len(fields) {
				return nil, errors.New(i18n.T("Incomplete Git rename record"))
			}
			entry.oldPath = first
			entry.path = fields[index]
			index++
		}
		result = append(result, entry)
	}
	return result, nil
}

func parsePatch(patch []byte, named namedDiff) protocol.FileDiff {
	file := protocol.FileDiff{Path: named.path, OldPath: named.oldPath, Status: named.status, Hunks: []protocol.Hunk{}}
	if patchIsBinary(patch) {
		file.Binary = true
		return file
	}
	lines := strings.Split(string(patch), "\n")
	var current *protocol.Hunk
	var oldLine, newLine int
	for _, line := range lines {
		match := hunkHeader.FindStringSubmatch(line)
		if match != nil {
			hunk := protocol.Hunk{
				Header:   line,
				OldStart: parseHunkNumber(match[1]),
				OldLines: parseOptionalCount(match[2]),
				NewStart: parseHunkNumber(match[3]),
				NewLines: parseOptionalCount(match[4]),
				Lines:    []protocol.DiffLine{},
			}
			file.Hunks = append(file.Hunks, hunk)
			current = &file.Hunks[len(file.Hunks)-1]
			oldLine, newLine = current.OldStart, current.NewStart
			continue
		}
		if current == nil || line == "\\ No newline at end of file" || line == "" && oldLine == 0 && newLine == 0 {
			continue
		}
		switch {
		case strings.HasPrefix(line, "+"):
			current.Lines = append(current.Lines, protocol.DiffLine{Kind: protocol.LineAdd, Text: line[1:], New: newLine})
			newLine++
			file.Added++
		case strings.HasPrefix(line, "-"):
			current.Lines = append(current.Lines, protocol.DiffLine{Kind: protocol.LineDel, Text: line[1:], Old: oldLine})
			oldLine++
			file.Removed++
		case strings.HasPrefix(line, " "):
			current.Lines = append(current.Lines, protocol.DiffLine{Kind: protocol.LineContext, Text: line[1:], Old: oldLine, New: newLine})
			oldLine++
			newLine++
		}
	}
	return file
}

func patchIsBinary(patch []byte) bool {
	for _, line := range strings.Split(string(patch), "\n") {
		if strings.HasPrefix(line, "@@ ") {
			return false
		}
		if line == "GIT binary patch" || strings.HasPrefix(line, "Binary files ") {
			return true
		}
	}
	return false
}

func parseHunkNumber(value string) int {
	n, _ := strconv.Atoi(value)
	return n
}

func parseOptionalCount(value string) int {
	if value == "" {
		return 1
	}
	return parseHunkNumber(value)
}
