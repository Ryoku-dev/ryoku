package ask

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ryoku-rashin/internal/llm"
	"ryoku-rashin/internal/protocol"
)

type searchCodeIndexer struct {
	indexes    []protocol.IndexInfo
	addCalls   int
	searches   []protocol.SearchParams
	searchData protocol.SearchResult
}

func (f *searchCodeIndexer) List(context.Context) ([]protocol.IndexInfo, error) {
	return f.indexes, nil
}
func (f *searchCodeIndexer) Add(context.Context, protocol.IndexAddParams) (protocol.IndexInfo, error) {
	f.addCalls++
	return protocol.IndexInfo{}, nil
}
func (f *searchCodeIndexer) Remove(context.Context, protocol.IndexRef) error { return nil }
func (f *searchCodeIndexer) Refresh(context.Context, protocol.IndexRef) (protocol.IndexInfo, error) {
	return protocol.IndexInfo{}, nil
}
func (f *searchCodeIndexer) Status(context.Context, protocol.IndexRef) (protocol.IndexInfo, error) {
	return protocol.IndexInfo{}, nil
}
func (f *searchCodeIndexer) Search(_ context.Context, params protocol.SearchParams) (protocol.SearchResult, error) {
	f.searches = append(f.searches, params)
	return f.searchData, nil
}
func (f *searchCodeIndexer) Find(context.Context, protocol.NameParams) (protocol.SymbolsResult, error) {
	return protocol.SymbolsResult{}, nil
}
func (f *searchCodeIndexer) Def(context.Context, protocol.NameParams) (protocol.DefResult, error) {
	return protocol.DefResult{}, nil
}
func (f *searchCodeIndexer) Outline(context.Context, protocol.PathParams) (protocol.SymbolsResult, error) {
	return protocol.SymbolsResult{}, nil
}
func (f *searchCodeIndexer) References(context.Context, protocol.NameParams) (protocol.ReferencesResult, error) {
	return protocol.ReferencesResult{}, nil
}
func (f *searchCodeIndexer) Peek(context.Context, protocol.PeekParams) (protocol.PeekResult, error) {
	return protocol.PeekResult{}, nil
}
func (f *searchCodeIndexer) Overview(context.Context, protocol.IndexRef) (protocol.Overview, error) {
	return protocol.Overview{}, nil
}
func (f *searchCodeIndexer) Impact(context.Context, protocol.PathParams) (protocol.Impact, error) {
	return protocol.Impact{}, nil
}
func (f *searchCodeIndexer) Wip(context.Context, protocol.WipParams) (protocol.WipResult, error) {
	return protocol.WipResult{}, nil
}
func (f *searchCodeIndexer) Changed(context.Context, protocol.WipParams) (protocol.WipResult, error) {
	return protocol.WipResult{}, nil
}
func (f *searchCodeIndexer) Busy() bool   { return false }
func (f *searchCodeIndexer) Release()     {}
func (f *searchCodeIndexer) Close() error { return nil }

func TestSearchCodeUsesOnlyRegisteredProjectIndex(t *testing.T) {
	t.Run("unregistered project returns guidance without adding", func(t *testing.T) {
		root := t.TempDir()
		if err := os.Mkdir(filepath.Join(root, ".git"), 0o700); err != nil {
			t.Fatal(err)
		}
		cwd := filepath.Join(root, "nested")
		if err := os.Mkdir(cwd, 0o700); err != nil {
			t.Fatal(err)
		}
		indexer := &searchCodeIndexer{indexes: []protocol.IndexInfo{
			{ID: protocol.IndexSystem, Kind: protocol.IndexSystem, Root: "/"},
			{ID: "elsewhere", Kind: protocol.IndexProject, Root: filepath.Join(root, "other")},
		}}
		factory := New(nil, indexer, nil, nil)
		result, _, err := factory.runTool(context.Background(), llm.ToolCall{
			Name: "search_code", Arguments: `{"query":"harnesses"}`,
		}, &protocol.TerminalContext{Cwd: cwd}, "")
		if err != nil {
			t.Fatal(err)
		}
		if indexer.addCalls != 0 || len(indexer.searches) != 0 {
			t.Fatalf("unregistered search added %d indexes and made %d searches", indexer.addCalls, len(indexer.searches))
		}
		if !strings.Contains(result, "not indexed") || !strings.Contains(result, "Index page") ||
			!strings.Contains(result, "rashin index add "+quoteShellArg(root)) {
			t.Fatalf("unregistered search result = %q", result)
		}
	})

	t.Run("most specific registered project wins", func(t *testing.T) {
		root := t.TempDir()
		project := filepath.Join(root, "project")
		nested := filepath.Join(project, "nested")
		cwd := filepath.Join(nested, "package")
		if err := os.MkdirAll(cwd, 0o700); err != nil {
			t.Fatal(err)
		}
		indexer := &searchCodeIndexer{
			indexes: []protocol.IndexInfo{
				{ID: protocol.IndexSystem, Kind: protocol.IndexSystem, Root: "/"},
				{ID: "outer", Kind: protocol.IndexProject, Root: project},
				{ID: "inner", Kind: protocol.IndexProject, Root: nested},
			},
			searchData: protocol.SearchResult{Index: "inner"},
		}
		factory := New(nil, indexer, nil, nil)
		if _, _, err := factory.runTool(context.Background(), llm.ToolCall{
			Name: "search_code", Arguments: `{"query":"harnesses"}`,
		}, &protocol.TerminalContext{Cwd: cwd}, ""); err != nil {
			t.Fatal(err)
		}
		if indexer.addCalls != 0 {
			t.Fatalf("registered search called Add %d times", indexer.addCalls)
		}
		if len(indexer.searches) != 1 || indexer.searches[0].ID != "inner" ||
			indexer.searches[0].Cwd != "" || !indexer.searches[0].Existing {
			t.Fatalf("searches = %#v, want one existing ID-only search of inner", indexer.searches)
		}
	})
}

func TestSearchSystemUsesExistingIndex(t *testing.T) {
	indexer := &searchCodeIndexer{}
	factory := New(nil, indexer, nil, nil)
	if _, _, err := factory.runTool(context.Background(), llm.ToolCall{
		Name: "search_system", Arguments: `{"query":"harnesses"}`,
	}, nil, ""); err != nil {
		t.Fatal(err)
	}
	if len(indexer.searches) != 1 || indexer.searches[0].ID != protocol.IndexSystem ||
		!indexer.searches[0].Existing {
		t.Fatalf("searches = %#v, want existing system search", indexer.searches)
	}
}
