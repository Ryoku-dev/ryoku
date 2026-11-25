package git

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"ryoku-rashin/internal/protocol"
)

func status(ctx context.Context, worktree, base string) (protocol.GitStatusResult, error) {
	result := protocol.GitStatusResult{Path: worktree, Files: []protocol.FileChange{}}
	result.Status.Checked = time.Now()
	out, err := run(ctx, worktree, "status", "--porcelain=v2", "-z", "--branch", "--untracked-files=normal")
	if err != nil {
		return result, err
	}
	stagedOut, err := run(ctx, worktree, "diff", "--cached", "--numstat", "-z", "-M")
	if err != nil {
		return result, err
	}
	workingOut, err := run(ctx, worktree, "diff", "--numstat", "-z", "-M")
	if err != nil {
		return result, err
	}
	stagedCounts := parseNumstat(stagedOut)
	workingCounts := parseNumstat(workingOut)
	records := strings.Split(string(out), "\x00")
	for index := 0; index < len(records); index++ {
		record := records[index]
		if record == "" {
			continue
		}
		if strings.HasPrefix(record, "# ") {
			parseStatusHeader(&result, record)
			continue
		}
		switch record[0] {
		case '1':
			fields := strings.SplitN(record, " ", 9)
			if len(fields) == 9 {
				appendStatusChanges(&result, fields[1], fields[8], "", stagedCounts, workingCounts)
			}
		case '2':
			fields := strings.SplitN(record, " ", 10)
			if len(fields) == 10 {
				oldPath := ""
				if index+1 < len(records) {
					index++
					oldPath = records[index]
				}
				appendStatusChanges(&result, fields[1], fields[9], oldPath, stagedCounts, workingCounts)
			}
		case 'u':
			fields := strings.SplitN(record, " ", 11)
			if len(fields) == 11 {
				change := protocol.FileChange{Path: fields[10], Status: protocol.ChangeConflict}
				applyNumstat(&change, workingCounts[change.Path])
				result.Files = append(result.Files, change)
				result.Status.Conflicted++
			}
		case '?':
			path := strings.TrimPrefix(record, "? ")
			change := protocol.FileChange{Path: path, Status: protocol.ChangeUntracked}
			change.Added, change.Binary = untrackedLines(filepath.Join(worktree, filepath.FromSlash(path)))
			result.Files = append(result.Files, change)
			result.Status.Untracked++
		}
	}
	result.Status.Operation = operation(ctx, worktree)
	if base != "" {
		baseCommit, resolveErr := resolveCommit(ctx, worktree, base)
		if resolveErr != nil {
			return result, resolveErr
		}
		counts, countErr := run(ctx, worktree, "rev-list", "--left-right", "--count", "--end-of-options", baseCommit+"...HEAD")
		if countErr != nil {
			return result, countErr
		}
		parts := strings.Fields(string(counts))
		if len(parts) == 2 {
			result.Status.BehindBase = parseCount(parts[0])
			result.Status.AheadBase = parseCount(parts[1])
		}
		numstat, diffErr := run(ctx, worktree, "diff", "--numstat", "-z", "--end-of-options", baseCommit)
		if diffErr != nil {
			return result, diffErr
		}
		result.Status.Added, result.Status.Removed = sumNumstat(numstat)
	}
	return result, nil
}

func parseStatusHeader(result *protocol.GitStatusResult, record string) {
	line := strings.TrimPrefix(record, "# ")
	key, value, found := strings.Cut(line, " ")
	if !found {
		return
	}
	switch key {
	case "branch.oid":
		result.Head = value
	case "branch.head":
		if value != "(detached)" {
			result.Branch = value
		}
	case "branch.upstream":
		result.Status.Upstream = value
	case "branch.ab":
		parts := strings.Fields(value)
		if len(parts) == 2 {
			result.Status.Ahead = parseCount(strings.TrimPrefix(parts[0], "+"))
			result.Status.Behind = parseCount(strings.TrimPrefix(parts[1], "-"))
		}
	}
}

func appendStatusChanges(result *protocol.GitStatusResult, xy, path, oldPath string, stagedCounts, workingCounts map[string]numstat) {
	if len(xy) != 2 {
		return
	}
	if xy[0] != '.' {
		change := protocol.FileChange{Path: path, OldPath: oldPath, Status: normalizeChange(xy[0]), Staged: true}
		applyNumstat(&change, stagedCounts[path])
		result.Files = append(result.Files, change)
		result.Status.Staged++
	}
	if xy[1] != '.' {
		change := protocol.FileChange{Path: path, OldPath: oldPath, Status: normalizeChange(xy[1])}
		applyNumstat(&change, workingCounts[path])
		result.Files = append(result.Files, change)
		result.Status.Modified++
	}
}

func normalizeChange(value byte) string {
	switch value {
	case 'A':
		return protocol.ChangeAdded
	case 'D':
		return protocol.ChangeDeleted
	case 'R':
		return protocol.ChangeRenamed
	case 'C':
		return protocol.ChangeCopied
	case 'T':
		return protocol.ChangeType
	case 'U':
		return protocol.ChangeConflict
	default:
		return protocol.ChangeModified
	}
}

type numstat struct {
	added   int
	removed int
	binary  bool
}

func parseNumstat(out []byte) map[string]numstat {
	records := strings.Split(string(out), "\x00")
	result := make(map[string]numstat)
	for index := 0; index < len(records); index++ {
		fields := strings.SplitN(records[index], "\t", 3)
		if len(fields) != 3 {
			continue
		}
		path := fields[2]
		if path == "" && index+2 < len(records) {
			index++
			index++
			path = records[index]
		}
		value := numstat{binary: fields[0] == "-" || fields[1] == "-"}
		if !value.binary {
			value.added, _ = strconv.Atoi(fields[0])
			value.removed, _ = strconv.Atoi(fields[1])
		}
		result[path] = value
	}
	return result
}

func applyNumstat(change *protocol.FileChange, value numstat) {
	change.Added = value.added
	change.Removed = value.removed
	change.Binary = value.binary
}

func sumNumstat(out []byte) (int, int) {
	var added, removed int
	for _, record := range strings.Split(string(out), "\x00") {
		fields := strings.SplitN(record, "\t", 3)
		if len(fields) < 3 || fields[0] == "-" || fields[1] == "-" {
			continue
		}
		a, _ := strconv.Atoi(fields[0])
		r, _ := strconv.Atoi(fields[1])
		added += a
		removed += r
	}
	return added, removed
}

func untrackedLines(path string) (int, bool) {
	info, err := os.Lstat(path)
	if err != nil {
		return 0, false
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return 1, false
	}
	if !info.Mode().IsRegular() {
		return 0, false
	}
	file, err := os.Open(path)
	if err != nil {
		return 0, false
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, 1<<20))
	if err != nil {
		return 0, false
	}
	if bytes.IndexByte(data, 0) >= 0 {
		return 0, true
	}
	if len(data) == 0 {
		return 0, false
	}
	lines := bytes.Count(data, []byte{'\n'})
	if data[len(data)-1] != '\n' {
		lines++
	}
	return lines, false
}

func operation(ctx context.Context, worktree string) string {
	out, err := run(ctx, worktree, "rev-parse", "--absolute-git-dir")
	if err != nil {
		return ""
	}
	gitDir := trimOutput(out)
	checks := []struct {
		path string
		op   string
	}{
		{"MERGE_HEAD", protocol.OpMerge},
		{"rebase-merge", protocol.OpRebase},
		{"rebase-apply", protocol.OpRebase},
		{"CHERRY_PICK_HEAD", protocol.OpCherryPick},
		{"REVERT_HEAD", protocol.OpRevert},
		{"BISECT_LOG", protocol.OpBisect},
	}
	for _, check := range checks {
		if _, err := os.Stat(filepath.Join(gitDir, check.path)); err == nil {
			return check.op
		}
	}
	return ""
}
