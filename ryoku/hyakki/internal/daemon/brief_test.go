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

// The session's brief names its folder and index while indexing starts in
// the background; no agent process is needed at creation.
func TestAnAgentSessionIsBriefedAndItsFolderIndexed(t *testing.T) {
	isolateDaemon(t)
	root := repository(t, "Work", "app")
	nested := filepath.Join(root, "cmd")
	if err := os.MkdirAll(nested, 0o700); err != nil {
		t.Fatal(err)
	}
	agents := &fakeAgents{runner: &fakeRunner{}}
	indexer := &fakeIndexer{indexes: []protocol.IndexInfo{
		{ID: protocol.IndexSystem, Root: "/"},
		{ID: "a1", Name: "app", Root: root, Files: 37},
	}}
	d := New(Deps{Agents: agents, Indexer: indexer, Version: "test"})
	info, err := d.manager.newSession(context.Background(), protocol.SessionNewParams{Kind: protocol.KindAgent, Cwd: nested})
	if err != nil {
		t.Fatal(err)
	}
	if agents.opts.Brief == nil {
		t.Fatal("the agent was started with no brief")
	}
	text := agents.opts.Brief()
	for _, want := range []string{"- Folder: " + nested, "an index of " + root + " (37 files)"} {
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
	if got, _ := d.manager.Info(info.ID); got.State != protocol.StateStopped {
		t.Fatal("a new agent session started running before its first prompt")
	}
}

// The home directory is never indexed, so a session there has no index in
// its brief and nothing is registered.
func TestASessionInTheHomeFolderIsNotIndexed(t *testing.T) {
	isolateDaemon(t)
	if err := os.MkdirAll(filepath.Join(paths.Home(), ".git"), 0o700); err != nil {
		t.Fatal(err)
	}
	agents := &fakeAgents{runner: &fakeRunner{}}
	indexer := &fakeIndexer{}
	d := New(Deps{Agents: agents, Indexer: indexer, Version: "test"})
	_, err := d.manager.newSession(context.Background(), protocol.SessionNewParams{Kind: protocol.KindAgent, Cwd: paths.Home()})
	if err != nil {
		t.Fatal(err)
	}
	if text := agents.opts.Brief(); strings.Contains(text, "- Code index:") {
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
	agents := &fakeAgents{runner: &fakeRunner{}}
	d := New(Deps{Agents: agents, Indexer: &fakeIndexer{}, Version: "test"})
	_, err := d.manager.newSession(context.Background(), protocol.SessionNewParams{Kind: protocol.KindAgent, Cwd: root})
	if err != nil {
		t.Fatal(err)
	}
	if text := agents.opts.Brief(); !strings.Contains(text, "an index of "+root+",") && !strings.Contains(text, "an index of "+root+" (") {
		t.Errorf("the brief reads: %s", text)
	}
}
