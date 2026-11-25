package daemon

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"ryoku-rashin/internal/client"
	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/protocol"
)

// pathIndexer answers every read with each file its index holds.
type pathIndexer struct {
	fakeIndexer
	files map[string][]string
}

func (f *pathIndexer) symbols(id string) []protocol.Symbol {
	var out []protocol.Symbol
	for _, path := range f.files[id] {
		out = append(out, protocol.Symbol{Name: "Token", Path: path, Start: 1, End: 3})
	}
	return out
}

func (f *pathIndexer) Search(_ context.Context, p protocol.SearchParams) (protocol.SearchResult, error) {
	var hits []protocol.SearchHit
	for _, path := range f.files[p.ID] {
		hits = append(hits, protocol.SearchHit{Path: path, Start: 1, End: 3, Snippet: "text of " + path})
	}
	return protocol.SearchResult{Index: p.ID, Hits: hits}, nil
}

func (f *pathIndexer) Find(_ context.Context, p protocol.NameParams) (protocol.SymbolsResult, error) {
	return protocol.SymbolsResult{Index: p.ID, Symbols: f.symbols(p.ID)}, nil
}

func (f *pathIndexer) Outline(_ context.Context, p protocol.PathParams) (protocol.SymbolsResult, error) {
	return protocol.SymbolsResult{Index: p.ID, Symbols: f.symbols(p.ID)}, nil
}

// Def finds the symbol in each file the index holds, the last file first.
func (f *pathIndexer) Def(_ context.Context, p protocol.NameParams) (protocol.DefResult, error) {
	found := f.symbols(p.ID)
	slices.Reverse(found)
	return protocol.DefResult{Index: p.ID, Symbol: found[0], Body: "body of " + found[0].Path, Others: found[1:]}, nil
}

func (f *pathIndexer) References(_ context.Context, p protocol.NameParams) (protocol.ReferencesResult, error) {
	var refs []protocol.Reference
	for _, path := range f.files[p.ID] {
		refs = append(refs, protocol.Reference{Path: path, Line: 1, Text: "use in " + path})
	}
	return protocol.ReferencesResult{Index: p.ID, Name: p.Name, References: refs}, nil
}

func (f *pathIndexer) Peek(_ context.Context, p protocol.PeekParams) (protocol.PeekResult, error) {
	return protocol.PeekResult{Index: p.ID, Path: p.Path, Start: p.Start, End: p.End, Text: "lines of " + p.Path}, nil
}

func (f *pathIndexer) Overview(_ context.Context, ref protocol.IndexRef) (protocol.Overview, error) {
	files := f.files[ref.ID]
	var stats []protocol.FileStat
	for _, path := range files {
		stats = append(stats, protocol.FileStat{Path: path, Value: 1})
	}
	return protocol.Overview{Index: ref.ID, Entrypoints: slices.Clone(files), Docs: slices.Clone(files), Largest: stats, MostImported: slices.Clone(stats)}, nil
}

func (f *pathIndexer) Impact(_ context.Context, p protocol.PathParams) (protocol.Impact, error) {
	files := f.files[p.ID]
	return protocol.Impact{Index: p.ID, Path: p.Path, Direct: slices.Clone(files), All: slices.Clone(files), Total: len(files)}, nil
}

func (f *pathIndexer) Wip(_ context.Context, p protocol.WipParams) (protocol.WipResult, error) {
	var changes []protocol.GitChange
	for _, path := range f.files[p.ID] {
		changes = append(changes, protocol.GitChange{Path: path, Status: "modified"})
	}
	return protocol.WipResult{Index: p.ID, Changes: changes}, nil
}

// What a team's sandbox hides inside a folder it may change stays hidden
// from its code search: the places credentials live, the paths Settings >
// Teams hides and the team's own hidden paths drop out of every result, and
// a peek, outline or impact of one is refused.
func TestAnAgentsIndexReadsLeaveOutWhatItsSandboxHides(t *testing.T) {
	isolateDaemon(t)
	shortRuntime(t)
	home := paths.Home()
	config := filepath.Join(home, ".config")
	projects := filepath.Join(home, "Projects")
	site := filepath.Join(projects, "site")
	for _, dir := range []string{filepath.Join(config, "gh"), filepath.Join(site, "secrets")} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	indexer := &pathIndexer{
		fakeIndexer: fakeIndexer{indexes: []protocol.IndexInfo{
			{ID: "site", Kind: protocol.IndexProject, Root: site, Roots: []string{site}},
			{ID: "dots", Kind: protocol.IndexProject, Root: config, Roots: []string{config}},
			{ID: "secrets", Kind: protocol.IndexProject, Root: filepath.Join(site, "secrets"), Roots: []string{filepath.Join(site, "secrets")}},
		}},
		files: map[string][]string{
			"site": {"main.go", "secrets/token.txt", ".env"},
			"dots": {"nvim/init.lua", "gh/hosts.yml"},
		},
	}
	crew := &accessTeams{access: protocol.TeamAccess{Writable: []string{projects, config}, Hidden: []string{filepath.Join(site, "secrets")}}}
	d := New(Deps{Teams: crew, Indexer: indexer})
	d.mu.Lock()
	d.settings.Teams.Hidden = []string{"~/Projects/site/.env"}
	d.mu.Unlock()
	team, err := d.manager.create(protocol.SessionNewParams{Kind: protocol.KindAgent, Cwd: filepath.Join(paths.TeamsHome(), "one"), Team: "team-1", TeamTask: "task-1", Member: "member-1"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.agentSandbox(team); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(d.closeAgentSockets)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, err := client.Dial(ctx, client.Options{Client: protocol.ClientCLI, Socket: paths.AgentSocket(team.ID)})
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	var listed protocol.IndexesResult
	if err := conn.Call(ctx, protocol.MIndexList, protocol.Empty{}, &listed); err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, index := range listed.Indexes {
		ids = append(ids, index.ID)
	}
	if !slices.Equal(ids, []string{"site", "dots"}) {
		t.Fatalf("the agent is listed %v, want site and dots but not the hidden folder's index", ids)
	}

	visible := map[string]string{"site": "main.go", "dots": "nvim/init.lua"}
	for id, want := range visible {
		ref := protocol.IndexRef{ID: id}
		var search protocol.SearchResult
		if err := conn.Call(ctx, protocol.MIndexSearch, protocol.SearchParams{IndexRef: ref, Query: "token"}, &search); err != nil || len(search.Hits) != 1 || search.Hits[0].Path != want {
			t.Fatalf("search in %s: %v, %+v", id, err, search.Hits)
		}
		var found protocol.SymbolsResult
		if err := conn.Call(ctx, protocol.MIndexFind, protocol.NameParams{IndexRef: ref, Name: "Token"}, &found); err != nil || len(found.Symbols) != 1 || found.Symbols[0].Path != want {
			t.Fatalf("find in %s: %v, %+v", id, err, found.Symbols)
		}
		var refs protocol.ReferencesResult
		if err := conn.Call(ctx, protocol.MIndexReferences, protocol.NameParams{IndexRef: ref, Name: "Token"}, &refs); err != nil || len(refs.References) != 1 || refs.References[0].Path != want {
			t.Fatalf("references in %s: %v, %+v", id, err, refs.References)
		}
		var overview protocol.Overview
		if err := conn.Call(ctx, protocol.MIndexOverview, ref, &overview); err != nil || !slices.Equal(overview.Entrypoints, []string{want}) || !slices.Equal(overview.Docs, []string{want}) || len(overview.Largest) != 1 || len(overview.MostImported) != 1 {
			t.Fatalf("overview of %s: %v, %+v", id, err, overview)
		}
		var impact protocol.Impact
		if err := conn.Call(ctx, protocol.MIndexImpact, protocol.PathParams{IndexRef: ref, Path: want, All: true}, &impact); err != nil || !slices.Equal(impact.Direct, []string{want}) || impact.Total != 1 {
			t.Fatalf("impact in %s: %v, %+v", id, err, impact)
		}
		var wip protocol.WipResult
		if err := conn.Call(ctx, protocol.MIndexWip, protocol.WipParams{IndexRef: ref}, &wip); err != nil || len(wip.Changes) != 1 || wip.Changes[0].Path != want {
			t.Fatalf("wip in %s: %v, %+v", id, err, wip.Changes)
		}
		var outline protocol.SymbolsResult
		if err := conn.Call(ctx, protocol.MIndexOutline, protocol.PathParams{IndexRef: ref, Path: want}, &outline); err != nil || len(outline.Symbols) != 1 {
			t.Fatalf("outline in %s: %v, %+v", id, err, outline.Symbols)
		}
	}

	// the definition in the hidden file gives way to the visible one
	var def protocol.DefResult
	if err := conn.Call(ctx, protocol.MIndexDef, protocol.NameParams{IndexRef: protocol.IndexRef{ID: "site"}, Name: "Token"}, &def); err != nil || def.Symbol.Path != "main.go" || def.Body != "lines of main.go" || len(def.Others) != 0 {
		t.Fatalf("def in site: %v, %+v", err, def)
	}
	if err := conn.Call(ctx, protocol.MIndexDef, protocol.NameParams{IndexRef: protocol.IndexRef{ID: "dots"}, Name: "Token"}, &def); err != nil || def.Symbol.Path != "nvim/init.lua" {
		t.Fatalf("def in dots: %v, %+v", err, def)
	}

	for _, refused := range []struct {
		method string
		params any
	}{
		{protocol.MIndexPeek, protocol.PeekParams{IndexRef: protocol.IndexRef{ID: "site"}, Path: "secrets/token.txt", Start: 1}},
		{protocol.MIndexPeek, protocol.PeekParams{IndexRef: protocol.IndexRef{ID: "site"}, Path: ".env", Start: 1}},
		{protocol.MIndexPeek, protocol.PeekParams{IndexRef: protocol.IndexRef{ID: "site"}, Path: filepath.Join(site, "secrets", "token.txt"), Start: 1}},
		{protocol.MIndexPeek, protocol.PeekParams{IndexRef: protocol.IndexRef{ID: "dots"}, Path: "gh/hosts.yml", Start: 1}},
		{protocol.MIndexPeek, protocol.PeekParams{IndexRef: protocol.IndexRef{ID: "site"}, Path: "../../.config/gh/hosts.yml", Start: 1}},
		{protocol.MIndexOutline, protocol.PathParams{IndexRef: protocol.IndexRef{ID: "dots"}, Path: "gh/hosts.yml"}},
		{protocol.MIndexImpact, protocol.PathParams{IndexRef: protocol.IndexRef{ID: "site"}, Path: ".env"}},
		{protocol.MIndexStatus, protocol.IndexRef{ID: "secrets"}},
		{protocol.MIndexStatus, protocol.IndexRef{Cwd: filepath.Join(site, "secrets")}},
	} {
		if err := conn.Call(ctx, refused.method, refused.params, nil); !forbidden(err) {
			t.Fatalf("%s %+v: %v, want forbidden", refused.method, refused.params, err)
		}
	}
	var peeked protocol.PeekResult
	if err := conn.Call(ctx, protocol.MIndexPeek, protocol.PeekParams{IndexRef: protocol.IndexRef{ID: "site"}, Path: "main.go", Start: 1}, &peeked); err != nil || peeked.Text != "lines of main.go" {
		t.Fatalf("a visible peek: %v, %+v", err, peeked)
	}
}
