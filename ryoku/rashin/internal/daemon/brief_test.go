package daemon

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/protocol"
)

func repository(t *testing.T, parts ...string) string {
	t.Helper()
	root := filepath.Join(append([]string{paths.Home()}, parts...)...)
	if err := os.MkdirAll(filepath.Join(root, ".git"), 0o700); err != nil {
		t.Fatal(err)
	}
	return root
}

// Starting an agent session is what gets its folder indexed and what hands
// the agent its brief, so the search the brief names is there when it looks.
func TestAnAgentSessionIsBriefedAndItsFolderIndexed(t *testing.T) {
	isolateDaemon(t)
	root := repository(t, "Work", "app")
	nested := filepath.Join(root, "cmd")
	if err := os.MkdirAll(nested, 0o700); err != nil {
		t.Fatal(err)
	}
	harnesses := &fakeHarnesses{runner: &fakeRunner{}}
	indexer := &fakeIndexer{indexes: []protocol.IndexInfo{
		{ID: protocol.IndexSystem, Root: "/"},
		{ID: "a1", Name: "app", Root: root, Files: 37},
	}}
	d := New(Deps{Harnesses: harnesses, Indexer: indexer, Version: "test"})
	info, err := d.manager.create(protocol.SessionNewParams{Kind: protocol.KindAgent, Cwd: nested})
	if err != nil {
		t.Fatal(err)
	}
	if err := d.manager.start(context.Background(), info.ID); err != nil {
		t.Fatal(err)
	}
	if harnesses.opts.Brief == nil {
		t.Fatal("the agent was started with no brief")
	}
	text := harnesses.opts.Brief()
	for _, want := range []string{"- Folder: " + nested, "an index of " + root + " (37 files)", "rashin search", "Rashin is Ryoku's optional assistant"} {
		if !strings.Contains(text, want) {
			t.Errorf("the brief does not say %q", want)
		}
	}
	deadline := time.After(2 * time.Second)
	for {
		indexer.mu.Lock()
		refreshed := append([]protocol.IndexRef(nil), indexer.refreshed...)
		indexer.mu.Unlock()
		if len(refreshed) == 1 && refreshed[0].Cwd == nested {
			break
		}
		select {
		case <-deadline:
			t.Fatalf("the session's folder was refreshed as %#v", refreshed)
		case <-time.After(5 * time.Millisecond):
		}
	}
	if got, _ := d.manager.Info(info.ID); !got.Alive {
		t.Fatal("starting the session did not bring its agent up")
	}
}

// The home directory is never indexed, so a session there is told about
// Rashin and the vault but offered no search, and nothing is registered.
func TestASessionInTheHomeFolderIsNotIndexed(t *testing.T) {
	isolateDaemon(t)
	if err := os.MkdirAll(filepath.Join(paths.Home(), ".git"), 0o700); err != nil {
		t.Fatal(err)
	}
	harnesses := &fakeHarnesses{runner: &fakeRunner{}}
	indexer := &fakeIndexer{}
	d := New(Deps{Harnesses: harnesses, Indexer: indexer, Version: "test"})
	info, err := d.manager.create(protocol.SessionNewParams{Kind: protocol.KindAgent, Cwd: paths.Home()})
	if err != nil {
		t.Fatal(err)
	}
	if err := d.manager.start(context.Background(), info.ID); err != nil {
		t.Fatal(err)
	}
	if text := harnesses.opts.Brief(); strings.Contains(text, "Code search\n") {
		t.Error("a session in the home folder was offered a code search")
	}
	time.Sleep(30 * time.Millisecond)
	indexer.mu.Lock()
	defer indexer.mu.Unlock()
	if len(indexer.refreshed) != 0 {
		t.Fatalf("the home folder was handed to the indexer: %#v", indexer.refreshed)
	}
}

// A repository no index covers yet is still named in the brief: the search
// commands register it themselves, and a session is briefed only once.
func TestARepositoryNotYetIndexedIsStillOfferedForSearch(t *testing.T) {
	isolateDaemon(t)
	root := repository(t, "Work", "fresh")
	harnesses := &fakeHarnesses{runner: &fakeRunner{}}
	d := New(Deps{Harnesses: harnesses, Indexer: &fakeIndexer{}, Version: "test"})
	info, err := d.manager.create(protocol.SessionNewParams{Kind: protocol.KindAgent, Cwd: root})
	if err != nil {
		t.Fatal(err)
	}
	if err := d.manager.start(context.Background(), info.ID); err != nil {
		t.Fatal(err)
	}
	if text := harnesses.opts.Brief(); !strings.Contains(text, "an index of "+root+",") && !strings.Contains(text, "an index of "+root+" with") {
		t.Errorf("the brief reads: %s", text)
	}
}
