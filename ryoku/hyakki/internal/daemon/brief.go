package daemon

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"ryoku-rashin/internal/brief"
	"ryoku-rashin/internal/core"
	"ryoku-rashin/internal/engine"
	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/protocol"
)

// agentBrief is the session part of an agent's system prompt: the folder it
// works in, its project, the code search over the folder, its screen and, for
// a team task, its team and what its sandbox lets it change. A team task is
// offered only an index it may read. The search is named whenever the folder
// can be indexed, even while indexSession is still registering it: the tools
// wait for the index themselves, and the brief is built once per session.
func (d *Daemon) agentBrief(info protocol.SessionInfo) string {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	session := brief.Session{Info: info, Semantic: d.Settings().Index.Semantic, Screen: d.screenFor(info)}
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

// memoryBudget caps the memory block in bytes: small models get the facts
// that matter most and keep their window for the work.
const (
	memoryBudget        = 8000
	memoryBudgetCompact = 1500
)

// memoryContext puts what Rashin remembers into an Agent session's system
// prompt. An Ask recalls per question in the engine instead, and a team task
// hears what its team remembers through the team's brief.
type memoryContext struct{ memory core.Memory }

// MemoryContext is the memory block provider for engine.Deps.Context, last
// in its order; nil memory gives nil.
func MemoryContext(memory core.Memory) engine.ContextProvider {
	if memory == nil {
		return nil
	}
	return memoryContext{memory: memory}
}

func (c memoryContext) Blocks(_ context.Context, tc engine.TurnContext) []engine.Block {
	info := tc.Session
	if info.Kind != protocol.KindAgent || info.Team != "" {
		return nil
	}
	text := c.memory.Brief(info)
	if text == "" {
		return nil
	}
	budget := memoryBudget
	if tc.Profile.Compact() {
		budget = memoryBudgetCompact
	}
	return []engine.Block{{Name: engine.BlockMemory, Text: text, Budget: budget}}
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
