package git

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"

	i18n "ryoku-i18n"

	"ryoku-rashin/internal/protocol"
)

const logFormat = "%H%x00%P%x00%an%x00%ae%x00%at%x00%s%x1e"

type logCursor struct {
	Offset  int      `json:"offset"`
	Tips    []string `json:"tips"`
	Search  string   `json:"search,omitempty"`
	Matches []string `json:"matches,omitempty"`
}

func (r *Repo) log(ctx context.Context, q LogQuery) ([]protocol.GitCommit, string, error) {
	limit := q.Limit
	if limit <= 0 {
		limit = 200
	}
	if limit > 2000 {
		limit = 2000
	}
	cursor := logCursor{Search: q.Search}
	if q.Cursor == "" {
		tips, err := r.logTips(ctx, q)
		if err != nil {
			return nil, "", err
		}
		cursor.Tips = tips
	} else {
		decoded, err := decodeLogCursor(q.Cursor)
		if err != nil {
			return nil, "", err
		}
		if decoded.Search != q.Search {
			return nil, "", errors.New(i18n.T("Log cursor does not match the search"))
		}
		cursor = decoded
	}
	if len(cursor.Tips) == 0 {
		return []protocol.GitCommit{}, "", nil
	}
	baseArgs := []string{"log", "--topo-order", "--full-history", "--parents", "--format=" + logFormat}
	var page []protocol.GitCommit
	hasMore := false
	if q.Search == "" {
		args := append(append([]string{}, baseArgs...),
			"-n", strconv.Itoa(limit+1),
			"--skip", strconv.Itoa(cursor.Offset),
			"--end-of-options",
		)
		args = append(args, cursor.Tips...)
		out, err := run(ctx, r.Root, args...)
		if err != nil {
			return nil, "", err
		}
		page = parseLog(out)
		if len(page) > limit {
			page = page[:limit]
			hasMore = true
		}
	} else if len(cursor.Matches) == 0 {
		args := append(append([]string{}, baseArgs...), "--end-of-options")
		args = append(args, cursor.Tips...)
		out, err := run(ctx, r.Root, args...)
		if err != nil {
			return nil, "", err
		}
		all := parseLog(out)
		matches, err := r.logSearchMatches(ctx, q.Search, cursor.Tips)
		if err != nil {
			return nil, "", err
		}
		needle := strings.ToLower(q.Search)
		filtered := all[:0]
		for _, commit := range all {
			if matches[commit.Hash] || strings.HasPrefix(strings.ToLower(commit.Hash), needle) {
				filtered = append(filtered, commit)
				cursor.Matches = append(cursor.Matches, commit.Hash)
			}
		}
		end := limit
		if end > len(filtered) {
			end = len(filtered)
		}
		page = filtered[:end]
		hasMore = end < len(filtered)
	} else {
		if cursor.Offset >= len(cursor.Matches) {
			return []protocol.GitCommit{}, "", nil
		}
		end := cursor.Offset + limit
		if end > len(cursor.Matches) {
			end = len(cursor.Matches)
		}
		args := []string{"log", "--no-walk=unsorted", "--parents", "--format=" + logFormat, "--end-of-options"}
		args = append(args, cursor.Matches[cursor.Offset:end]...)
		out, err := run(ctx, r.Root, args...)
		if err != nil {
			return nil, "", err
		}
		page = parseLog(out)
		hasMore = end < len(cursor.Matches)
	}
	next := ""
	if hasMore {
		cursor.Offset += len(page)
		next = encodeLogCursor(cursor)
	}
	return page, next, nil
}

func (r *Repo) logTips(ctx context.Context, q LogQuery) ([]string, error) {
	switch q.Scope {
	case "", protocol.ScopeAll:
		format := "%(refname)%00%(*objectname)%00%(objectname)%1e"
		out, err := run(ctx, r.Root, "for-each-ref", "--format="+format)
		if err != nil {
			return nil, err
		}
		seen := make(map[string]bool)
		var tips []string
		for _, record := range strings.Split(string(out), "\x1e") {
			record = strings.TrimPrefix(record, "\n")
			fields := strings.Split(record, "\x00")
			if len(fields) < 3 || strings.HasPrefix(fields[0], "refs/rashin/") {
				continue
			}
			value := fields[1]
			if value == "" {
				value = fields[2]
			}
			commit, resolveErr := resolveCommit(ctx, r.Root, value)
			if resolveErr != nil {
				if ctx.Err() != nil {
					return nil, ctx.Err()
				}
				continue
			}
			if !seen[commit] {
				seen[commit] = true
				tips = append(tips, commit)
			}
		}
		return tips, nil
	case protocol.ScopeBranch:
		if q.Ref == "" {
			return nil, errors.New(i18n.T("Branch log ref is required"))
		}
		if strings.HasPrefix(q.Ref, "refs/rashin/") {
			return nil, nil
		}
		commit, err := resolveCommit(ctx, r.Root, q.Ref)
		if err != nil {
			return nil, err
		}
		return []string{commit}, nil
	case protocol.ScopeWorktree:
		if q.Worktree == "" {
			return nil, errors.New(i18n.T("Worktree log path is required"))
		}
		commit, err := resolveCommit(ctx, q.Worktree, "HEAD")
		if err != nil {
			return nil, err
		}
		return []string{commit}, nil
	default:
		return nil, errors.New(i18n.Tf("Unsupported log scope %q", q.Scope))
	}
}

func encodeLogCursor(cursor logCursor) string {
	data, _ := json.Marshal(cursor)
	return base64.RawURLEncoding.EncodeToString(data)
}

func decodeLogCursor(value string) (logCursor, error) {
	var cursor logCursor
	data, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return cursor, errors.New(i18n.T("Invalid log cursor"))
	}
	if err := json.Unmarshal(data, &cursor); err != nil || cursor.Offset < 0 || len(cursor.Tips) == 0 {
		return logCursor{}, errors.New(i18n.T("Invalid log cursor"))
	}
	for _, hash := range append(append([]string{}, cursor.Tips...), cursor.Matches...) {
		if !objectID(hash) {
			return logCursor{}, errors.New(i18n.T("Invalid log cursor"))
		}
	}
	return cursor, nil
}

func objectID(value string) bool {
	if len(value) != 40 && len(value) != 64 {
		return false
	}
	for _, character := range value {
		if character < '0' || character > '9' && character < 'a' || character > 'f' {
			return false
		}
	}
	return true
}

func (r *Repo) logSearchMatches(ctx context.Context, search string, tips []string) (map[string]bool, error) {
	matches := make(map[string]bool)
	queries := [][]string{
		{"log", "--format=%H", "--fixed-strings", "--regexp-ignore-case", "--grep=" + search},
		{"log", "--format=%H", "--fixed-strings", "--regexp-ignore-case", "--author=" + search},
	}
	for _, args := range queries {
		args = append(args, "--end-of-options")
		args = append(args, tips...)
		out, err := run(ctx, r.Root, args...)
		if err != nil {
			return nil, err
		}
		for _, hash := range strings.Fields(string(out)) {
			matches[hash] = true
		}
	}
	return matches, nil
}

func parseLog(out []byte) []protocol.GitCommit {
	records := strings.Split(string(out), "\x1e")
	commits := make([]protocol.GitCommit, 0, len(records))
	for _, record := range records {
		record = strings.TrimPrefix(record, "\n")
		if record == "" {
			continue
		}
		fields := strings.SplitN(record, "\x00", 6)
		if len(fields) != 6 {
			continue
		}
		unix, _ := strconv.ParseInt(fields[4], 10, 64)
		commit := protocol.GitCommit{
			Hash: fields[0], Author: fields[2], Email: fields[3],
			Time: time.Unix(unix, 0), Subject: strings.TrimSuffix(fields[5], "\n"),
		}
		if fields[1] != "" {
			commit.Parents = strings.Fields(fields[1])
		}
		commits = append(commits, commit)
	}
	return commits
}

func (r *Repo) refs(ctx context.Context) ([]protocol.GitRef, error) {
	worktrees, err := r.worktrees(ctx)
	if err != nil {
		return nil, err
	}
	owners := make(map[string]string)
	mainBranch := ""
	for _, wt := range worktrees {
		if wt.Branch != "" {
			owners[wt.Branch] = wt.Path
		}
		if wt.Main {
			mainBranch = wt.Branch
		}
	}
	format := "%(*objectname)%00%(objectname)%00%(refname)%00%(upstream)%1e"
	out, err := run(ctx, r.Root, "for-each-ref", "--format="+format, "refs/heads", "refs/remotes", "refs/tags")
	if err != nil {
		return nil, err
	}
	result := make([]protocol.GitRef, 0)
	for _, record := range strings.Split(string(out), "\x1e") {
		record = strings.TrimPrefix(record, "\n")
		fields := strings.Split(record, "\x00")
		if len(fields) < 4 {
			continue
		}
		peeled, target, fullName, upstream := fields[0], fields[1], fields[2], fields[3]
		if strings.HasPrefix(fullName, "refs/rashin/") {
			continue
		}
		kind, name := refKindName(fullName)
		if kind == "" {
			continue
		}
		if peeled != "" {
			target = peeled
		}
		branch := strings.TrimPrefix(fullName, "refs/heads/")
		entry := protocol.GitRef{Name: name, Kind: kind, Target: target, Upstream: strings.TrimPrefix(upstream, "refs/remotes/")}
		if kind == protocol.RefBranch {
			entry.Worktree = owners[branch]
			entry.Current = branch == mainBranch
		}
		result = append(result, entry)
	}
	for _, wt := range worktrees {
		result = append(result, protocol.GitRef{Name: "HEAD", Kind: protocol.RefHead, Target: wt.Head, Worktree: wt.Path, Current: wt.Main})
	}
	return result, nil
}

func refKindName(full string) (string, string) {
	switch {
	case strings.HasPrefix(full, "refs/heads/"):
		return protocol.RefBranch, strings.TrimPrefix(full, "refs/heads/")
	case strings.HasPrefix(full, "refs/remotes/"):
		return protocol.RefRemote, strings.TrimPrefix(full, "refs/remotes/")
	case strings.HasPrefix(full, "refs/tags/"):
		return protocol.RefTag, strings.TrimPrefix(full, "refs/tags/")
	default:
		return "", ""
	}
}

func (r *Repo) show(ctx context.Context, hash string) (protocol.GitCommitDetail, error) {
	var detail protocol.GitCommitDetail
	commit, err := resolveCommit(ctx, r.Root, hash)
	if err != nil {
		return detail, err
	}
	format := "%H%x00%P%x00%an%x00%ae%x00%at%x00%s%x00%cn%x00%ce%x00%B%x1e"
	out, err := run(ctx, r.Root, "show", "-s", "--format="+format, "--end-of-options", commit)
	if err != nil {
		return detail, err
	}
	record := strings.TrimPrefix(strings.SplitN(string(out), "\x1e", 2)[0], "\n")
	fields := strings.SplitN(record, "\x00", 9)
	if len(fields) != 9 {
		return detail, errors.New(i18n.Tf("Invalid Git show record for %s", hash))
	}
	unix, _ := strconv.ParseInt(fields[4], 10, 64)
	detail.Commit = protocol.GitCommit{Hash: fields[0], Parents: strings.Fields(fields[1]), Author: fields[2], Email: fields[3], Time: time.Unix(unix, 0), Subject: fields[5]}
	detail.Committer = fields[6]
	if fields[7] != "" {
		detail.Committer += " <" + fields[7] + ">"
	}
	detail.Body = strings.TrimSuffix(fields[8], "\n")
	refs, err := r.refs(ctx)
	if err != nil {
		return detail, err
	}
	for _, ref := range refs {
		if ref.Target == detail.Commit.Hash {
			detail.Refs = append(detail.Refs, ref)
		}
	}
	parent := "4b825dc642cb6eb9a060e54bf8d69288fbee4904"
	if len(detail.Commit.Parents) > 0 {
		parent = detail.Commit.Parents[0]
	}
	nameOut, err := run(ctx, r.Root, "diff", "--name-status", "-z", "-M", "--end-of-options", parent, commit)
	if err != nil {
		return detail, err
	}
	named, err := parseNameStatus(nameOut)
	if err != nil {
		return detail, err
	}
	for _, file := range named {
		change := protocol.FileChange{Path: file.path, OldPath: file.oldPath, Status: file.status}
		numstatArgs := []string{"diff", "--numstat", "-z", "-M", "--end-of-options", parent, commit, "--"}
		numstatArgs = append(numstatArgs, diffPaths(file.oldPath, file.path)...)
		numstatOut, numErr := run(ctx, r.Root, numstatArgs...)
		if numErr != nil {
			return detail, numErr
		}
		applyNumstat(&change, parseNumstat(numstatOut)[file.path])
		detail.Files = append(detail.Files, change)
	}
	return detail, nil
}
