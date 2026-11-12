package daemon

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"testing"
	"time"

	"ryoku-rashin/internal/client"
	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/protocol"
)

// readIndexer records which index each read reached.
type readIndexer struct {
	fakeIndexer
	readMu sync.Mutex
	read   []string
}

func (f *readIndexer) Status(_ context.Context, ref protocol.IndexRef) (protocol.IndexInfo, error) {
	f.readMu.Lock()
	defer f.readMu.Unlock()
	f.read = append(f.read, ref.ID+"|"+ref.Cwd)
	return protocol.IndexInfo{ID: ref.ID}, nil
}

func (f *readIndexer) Peek(_ context.Context, p protocol.PeekParams) (protocol.PeekResult, error) {
	f.readMu.Lock()
	defer f.readMu.Unlock()
	f.read = append(f.read, p.ID+"|"+p.Cwd)
	return protocol.PeekResult{Index: p.ID, Path: p.Path, Text: "inside"}, nil
}

func (f *readIndexer) reads() []string {
	f.readMu.Lock()
	defer f.readMu.Unlock()
	return slices.Clone(f.read)
}

// A team agent reads only the indexes inside its team's folder and the
// folders the team may change: never the system index, one elsewhere, or
// one that only holds its folders, which covers what its sandbox hides.
func TestAnAgentReadsOnlyTheIndexesInsideItsFolders(t *testing.T) {
	isolateDaemon(t)
	shortRuntime(t)
	home := paths.Home()
	shared := filepath.Join(home, "Projects", "shared")
	app := filepath.Join(shared, "app")
	other := filepath.Join(home, "Work", "other")
	for _, dir := range []string{filepath.Join(app, "cmd"), other} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	folder := filepath.Join(paths.TeamsHome(), "one")
	indexer := &readIndexer{fakeIndexer: fakeIndexer{indexes: []protocol.IndexInfo{
		{ID: protocol.IndexSystem, Kind: protocol.IndexSystem, Roots: []string{paths.VaultDir(), filepath.Join(home, ".config")}},
		{ID: "app", Kind: protocol.IndexProject, Root: app, Roots: []string{app}},
		{ID: "other", Kind: protocol.IndexProject, Root: other, Roots: []string{other}},
		{ID: "projects", Kind: protocol.IndexProject, Root: filepath.Join(home, "Projects"), Roots: []string{filepath.Join(home, "Projects")}},
	}}}
	crew := &accessTeams{access: protocol.TeamAccess{Writable: []string{shared}}}
	d := New(Deps{Teams: crew, Indexer: indexer})
	team, err := d.manager.create(protocol.SessionNewParams{Kind: protocol.KindAgent, Cwd: folder, Team: "team-1", TeamTask: "task-1", Member: "member-1"})
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
	if len(listed.Indexes) != 1 || listed.Indexes[0].ID != "app" {
		t.Fatalf("the agent is listed %+v, want only app", listed.Indexes)
	}

	for _, refused := range []struct {
		method string
		params any
	}{
		{protocol.MIndexStatus, protocol.IndexRef{ID: protocol.IndexSystem}},
		{protocol.MIndexStatus, protocol.IndexRef{ID: "other"}},
		{protocol.MIndexStatus, protocol.IndexRef{ID: "projects"}},
		{protocol.MIndexPeek, protocol.PeekParams{IndexRef: protocol.IndexRef{ID: "other"}, Path: "main.go", Start: 1}},
		{protocol.MIndexPeek, protocol.PeekParams{IndexRef: protocol.IndexRef{ID: protocol.IndexSystem}, Path: "rashin.json", Start: 1}},
		{protocol.MIndexPeek, protocol.PeekParams{IndexRef: protocol.IndexRef{Cwd: other}, Path: "main.go", Start: 1}},
		{protocol.MIndexPeek, protocol.PeekParams{IndexRef: protocol.IndexRef{Cwd: "../../../Work/other"}, Path: "main.go", Start: 1}},
	} {
		if err := conn.Call(ctx, refused.method, refused.params, nil); !forbidden(err) {
			t.Fatalf("%s %+v on an agent socket: %v, want forbidden", refused.method, refused.params, err)
		}
	}
	// a folder the team may change that only a wider index covers reaches
	// no index at all
	if err := conn.Call(ctx, protocol.MIndexStatus, protocol.IndexRef{Cwd: shared}, nil); err == nil {
		t.Fatal("a folder only the wider index covers was read through it")
	}
	if got := indexer.reads(); len(got) != 0 {
		t.Fatalf("refused reads reached the index: %v", got)
	}

	var status protocol.IndexInfo
	if err := conn.Call(ctx, protocol.MIndexStatus, protocol.IndexRef{ID: "app"}, &status); err != nil || status.ID != "app" {
		t.Fatalf("the index inside: %v, %+v", err, status)
	}
	var peeked protocol.PeekResult
	if err := conn.Call(ctx, protocol.MIndexPeek, protocol.PeekParams{IndexRef: protocol.IndexRef{Cwd: filepath.Join(app, "cmd")}, Path: "main.go", Start: 1}, &peeked); err != nil || peeked.Index != "app" {
		t.Fatalf("a peek inside: %v, %+v", err, peeked)
	}
	if got := indexer.reads(); !slices.Equal(got, []string{"app|", "app|"}) {
		t.Fatalf("the reads reached %v, want app by id twice", got)
	}
}
