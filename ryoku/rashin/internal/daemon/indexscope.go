package daemon

import (
	"context"
	"encoding/json"
	"path/filepath"
	"slices"

	i18n "ryoku-i18n"
	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/protocol"
	"ryoku-rashin/internal/sandbox"
)

// agentIndexCall serves an index read that arrived on a team session's own
// socket. The agent reads only the indexes whose every root lies inside
// its team's folder or a folder the team may change, never the system
// index: the rest cover what its sandbox hides, and peek and def would hand
// it their files' text. index.list names only those indexes, and any other
// call goes to the index named by id, or to the deepest one of them that
// holds the cwd, which must lie inside those folders too. Inside them, what
// the sandbox hides stays hidden: a request naming a hidden file is refused
// and every hidden file is taken out of what comes back.
func (d *Daemon) agentIndexCall(ctx context.Context, c *connection, method string, raw json.RawMessage) (any, error) {
	info, ok := d.manager.Info(c.session)
	if !ok || info.Team == "" || d.deps.Indexer == nil {
		return nil, indexRefused()
	}
	view := d.agentView(info)
	indexes, err := d.deps.Indexer.List(ctx)
	if err != nil {
		return nil, err
	}
	readable := make([]protocol.IndexInfo, 0, len(indexes))
	for _, index := range indexes {
		if view.readable(index) {
			readable = append(readable, index)
		}
	}
	if method == protocol.MIndexList {
		return protocol.IndexesResult{Indexes: readable}, nil
	}
	ref, err := unmarshal[protocol.IndexRef](raw)
	if err != nil {
		return nil, err
	}
	index, err := d.agentIndex(ctx, info, ref, readable, view)
	if err != nil {
		return nil, err
	}
	if method == protocol.MIndexPeek || method == protocol.MIndexOutline || method == protocol.MIndexImpact {
		named, err := unmarshal[protocol.PathParams](raw)
		if err != nil {
			return nil, err
		}
		if named.Path != "" && view.hidesFile(index, named.Path) {
			return nil, indexRefused()
		}
	}
	scoped, err := withIndexID(raw, index.ID)
	if err != nil {
		return nil, err
	}
	result, err := d.dispatch(ctx, c, method, scoped)
	if err != nil {
		return nil, err
	}
	return d.withoutHidden(ctx, result, index, view)
}

// agentIndexView is what a team session's agent sees of the host: the
// sandbox agentSandbox builds for it, without its socket.
type agentIndexView struct{ sandbox.Spec }

// agentView is the sandbox a team session's agent sees the host through,
// built as agentSandbox builds it.
func (d *Daemon) agentView(info protocol.SessionInfo) agentIndexView {
	spec := d.sandboxBase()
	spec.Folder = paths.Resolved(info.Cwd)
	if d.deps.Teams != nil {
		spec = withAccess(spec, d.deps.Teams.Access(info))
	}
	return agentIndexView{spec}
}

// agentIndexRoots are the folders a team session's agent may read indexes
// of, resolved: its team's folder and the folders the team may change that
// its sandbox binds.
func (d *Daemon) agentIndexRoots(info protocol.SessionInfo) []string {
	return d.agentView(info).roots()
}

func (v agentIndexView) roots() []string {
	return append([]string{v.Folder}, v.Writable...)
}

// readable reports an index the agent may read: not the system index, every
// root inside its folders and none of them hidden.
func (v agentIndexView) readable(index protocol.IndexInfo) bool {
	if !indexInside(index, v.roots()) {
		return false
	}
	return !slices.ContainsFunc(indexRoots(index), func(root string) bool { return v.Hides(paths.Resolved(root)) })
}

// hidesFile reports whether a path an index names, relative to one of its
// roots or absolute, is one the sandbox hides, its links followed.
func (v agentIndexView) hidesFile(index protocol.IndexInfo, path string) bool {
	if filepath.IsAbs(path) {
		return v.Hides(paths.Resolved(path))
	}
	return slices.ContainsFunc(indexRoots(index), func(root string) bool {
		return v.Hides(paths.Resolved(filepath.Join(root, filepath.FromSlash(path))))
	})
}

// indexRoots are an index's roots, the single Root when it lists none.
func indexRoots(index protocol.IndexInfo) []string {
	if len(index.Roots) == 0 && index.Root != "" {
		return []string{index.Root}
	}
	return index.Roots
}

// indexInside reports an index other than the system index whose every
// root lies inside one of roots.
func indexInside(index protocol.IndexInfo, roots []string) bool {
	if index.ID == protocol.IndexSystem || index.Kind == protocol.IndexSystem {
		return false
	}
	list := indexRoots(index)
	if len(list) == 0 {
		return false
	}
	for _, root := range list {
		if !insideAny(paths.Resolved(root), roots) {
			return false
		}
	}
	return true
}

// insideAny reports whether path is one of roots or below one.
func insideAny(path string, roots []string) bool {
	return slices.ContainsFunc(roots, func(root string) bool { return paths.Within(root, path) })
}

// agentIndex is the readable index ref names: the one its id names, or the
// deepest that holds its cwd, the team's folder when it names none. A cwd
// the sandbox hides is refused. A repository inside the folders that no
// index covers yet is registered, as the index does for any caller.
func (d *Daemon) agentIndex(ctx context.Context, info protocol.SessionInfo, ref protocol.IndexRef, readable []protocol.IndexInfo, view agentIndexView) (protocol.IndexInfo, error) {
	if ref.ID != "" {
		at := slices.IndexFunc(readable, func(index protocol.IndexInfo) bool { return index.ID == ref.ID })
		if at < 0 {
			return protocol.IndexInfo{}, indexRefused()
		}
		return readable[at], nil
	}
	cwd := ref.Cwd
	if cwd == "" {
		cwd = info.Cwd
	} else if !filepath.IsAbs(cwd) {
		cwd = filepath.Join(info.Cwd, cwd)
	}
	cwd = paths.Resolved(cwd)
	roots := view.roots()
	if !insideAny(cwd, roots) || view.Hides(cwd) {
		return protocol.IndexInfo{}, indexRefused()
	}
	var best protocol.IndexInfo
	depth := -1
	for _, index := range readable {
		for _, root := range indexRoots(index) {
			root = paths.Resolved(root)
			if paths.Within(root, cwd) && len(root) > depth {
				best, depth = index, len(root)
			}
		}
	}
	if depth >= 0 {
		return best, nil
	}
	notFound := protocol.Errorf(protocol.CodeNotFound, "%s", i18n.T("No index matches the current directory."))
	repository := repositoryRoot(cwd)
	if repository == "" || !insideAny(paths.Resolved(repository), roots) || view.Hides(paths.Resolved(repository)) {
		return protocol.IndexInfo{}, notFound
	}
	added, err := d.deps.Indexer.Add(ctx, protocol.IndexAddParams{Path: repository})
	if err != nil {
		return protocol.IndexInfo{}, err
	}
	if added.ID == "" {
		return protocol.IndexInfo{}, notFound
	}
	if len(indexRoots(added)) == 0 {
		added.Root = repository
	}
	return added, nil
}

// withoutHidden takes every file the sandbox hides out of an index result:
// hits, symbols, references, changes and the files an overview or an impact
// names. A definition found in a hidden file gives way to its first visible
// namesake, read from the index, or to none.
func (d *Daemon) withoutHidden(ctx context.Context, result any, index protocol.IndexInfo, view agentIndexView) (any, error) {
	hidden := func(path string) bool { return view.hidesFile(index, path) }
	symbols := func(list []protocol.Symbol) []protocol.Symbol {
		return slices.DeleteFunc(list, func(s protocol.Symbol) bool { return hidden(s.Path) })
	}
	names := func(list []string) []string { return slices.DeleteFunc(list, hidden) }
	stats := func(list []protocol.FileStat) []protocol.FileStat {
		return slices.DeleteFunc(list, func(s protocol.FileStat) bool { return hidden(s.Path) })
	}
	switch r := result.(type) {
	case protocol.SearchResult:
		r.Hits = slices.DeleteFunc(r.Hits, func(h protocol.SearchHit) bool { return hidden(h.Path) })
		return r, nil
	case protocol.SymbolsResult:
		r.Symbols = symbols(r.Symbols)
		return r, nil
	case protocol.ReferencesResult:
		r.References = slices.DeleteFunc(r.References, func(ref protocol.Reference) bool { return hidden(ref.Path) })
		return r, nil
	case protocol.PeekResult:
		if hidden(r.Path) {
			return nil, indexRefused()
		}
		return r, nil
	case protocol.Overview:
		r.Entrypoints, r.Docs = names(r.Entrypoints), names(r.Docs)
		r.Largest, r.MostImported = stats(r.Largest), stats(r.MostImported)
		return r, nil
	case protocol.Impact:
		before := len(r.All)
		r.Direct, r.All = names(r.Direct), names(r.All)
		if before > 0 {
			r.Total -= before - len(r.All)
		}
		return r, nil
	case protocol.WipResult:
		r.Changes = slices.DeleteFunc(r.Changes, func(change protocol.GitChange) bool { return hidden(change.Path) })
		return r, nil
	case protocol.DefResult:
		r.Others = symbols(r.Others)
		if !hidden(r.Symbol.Path) {
			return r, nil
		}
		if len(r.Others) == 0 {
			return nil, protocol.Errorf(protocol.CodeNotFound, "%s", i18n.Tf("Symbol %s was not found.", r.Symbol.Name))
		}
		symbol := r.Others[0]
		end := symbol.End
		if end < symbol.Start {
			end = symbol.Start
		}
		peek, err := d.deps.Indexer.Peek(ctx, protocol.PeekParams{IndexRef: protocol.IndexRef{ID: index.ID}, Path: symbol.Path, Start: symbol.Start, End: end})
		if err != nil {
			return nil, err
		}
		return protocol.DefResult{Index: r.Index, Symbol: symbol, Body: peek.Text, Truncated: peek.Truncated, Others: r.Others[1:]}, nil
	}
	return result, nil
}

// withIndexID is the params raw with the index named by id in place of any
// cwd, so the index resolves nothing itself.
func withIndexID(raw json.RawMessage, id string) (json.RawMessage, error) {
	fields := map[string]json.RawMessage{}
	if len(raw) > 0 && string(raw) != "null" {
		if err := json.Unmarshal(raw, &fields); err != nil {
			return nil, protocol.Errorf(protocol.CodeBadRequest, "%s", err.Error())
		}
	}
	encoded, err := json.Marshal(id)
	if err != nil {
		return nil, err
	}
	fields["id"] = encoded
	delete(fields, "cwd")
	return json.Marshal(fields)
}

// indexRefused is the error an agent gets for an index outside its folders.
func indexRefused() error {
	return protocol.Errorf(protocol.CodeForbidden, "%s", i18n.T("An agent can read only the indexes inside its team's folders."))
}
