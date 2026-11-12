package daemon

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"ryoku-rashin/internal/brief"
	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/protocol"
)

// agentBrief is what an agent session is told ahead of its first prompt:
// Ryoku, the vault, Rashin, the session's own place, the code search over
// its folder and, for a team task, its team. A team task's agent is told
// what its sandbox lets it reach in place of the vault it cannot see, and
// offered only an index it may read. The search is named whenever the
// folder can be indexed, even while indexSession is still registering it:
// the commands wait for the index themselves, and a session is only ever
// briefed once.
func (d *Daemon) agentBrief(info protocol.SessionInfo) string {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	tools := len(d.agentTools(info)) > 0
	session := brief.Session{Info: info, Semantic: d.Settings().Index.Semantic, Tools: tools, Screen: tools && d.screenFor(info)}
	var roots []string
	if info.Team != "" {
		roots = d.agentIndexRoots(info)
		session.Writable = roots[1:]
	}
	if d.deps.Teams != nil {
		session.Team = d.deps.Teams.Brief(info)
	}
	if d.deps.Work != nil && info.Project != "" {
		for _, project := range d.deps.Work.Projects(ctx) {
			if project.ID == info.Project {
				session.Project = project.Name
				break
			}
		}
	}
	if d.deps.Indexer != nil {
		if index, ok := d.coveringIndex(ctx, info.Cwd); ok && (roots == nil || indexInside(index, roots)) {
			session.Index = &index
		} else if root := repositoryRoot(info.Cwd); root != "" && (roots == nil || insideAny(paths.Resolved(root), roots)) {
			session.Index = &protocol.IndexInfo{Root: root}
		}
	}
	return brief.Agent(session)
}

// indexSession makes sure the folder an agent session works in is indexed and
// current, so the search it was told about answers at once. A folder outside
// any repository, and the home directory itself, are left alone.
func (d *Daemon) indexSession(info protocol.SessionInfo) {
	if d.deps.Indexer == nil || repositoryRoot(info.Cwd) == "" {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	// Refresh registers the repository when no index covers the folder yet
	_, _ = d.deps.Indexer.Refresh(ctx, protocol.IndexRef{Cwd: info.Cwd})
}

// coveringIndex is the registered index whose root holds dir, the deepest one
// when several do.
func (d *Daemon) coveringIndex(ctx context.Context, dir string) (protocol.IndexInfo, bool) {
	indexes, err := d.deps.Indexer.List(ctx)
	if err != nil {
		return protocol.IndexInfo{}, false
	}
	if resolved, err := filepath.EvalSymlinks(dir); err == nil {
		dir = resolved
	}
	var best protocol.IndexInfo
	for _, index := range indexes {
		if index.ID == protocol.IndexSystem || index.Root == "" || !paths.Within(index.Root, dir) {
			continue
		}
		if len(index.Root) > len(best.Root) {
			best = index
		}
	}
	return best, best.Root != ""
}

// repositoryRoot is the Git repository dir sits in, or "" when it sits in
// none or the repository is the home directory or above it, which is never
// indexed.
func repositoryRoot(dir string) string {
	if resolved, err := filepath.EvalSymlinks(dir); err == nil {
		dir = resolved
	}
	home := paths.Home()
	if resolved, err := filepath.EvalSymlinks(home); err == nil {
		home = resolved
	}
	for current := filepath.Clean(dir); ; current = filepath.Dir(current) {
		if _, err := os.Stat(filepath.Join(current, ".git")); err == nil {
			if current == string(filepath.Separator) || paths.Within(current, home) {
				return ""
			}
			return current
		}
		if current == filepath.Dir(current) {
			return ""
		}
	}
}
