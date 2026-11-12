package memory

import (
	"context"
	"testing"

	"ryoku-rashin/internal/protocol"
)

// Past-conversation search keeps a kept project's conversations to it, and
// finds nothing while the search is off.
func TestSessionSearchRespectsKeptAndSetting(t *testing.T) {
	te := newTestEngine(t, protocol.ProjectMemoryKept)
	te.sessions.add(protocol.SessionInfo{ID: "agent-g", Kind: protocol.KindAgent, Project: "garden", Title: "Kyoto garden"}, turn{user: "Plan the Kyoto temple garden beds", reply: "Done."})
	te.sessions.add(protocol.SessionInfo{ID: "ask-o", Title: "Kyoto trip"}, turn{user: "Plan my Kyoto temple visit", reply: "Done."})
	te.sessions.add(protocol.SessionInfo{ID: "agent-g2", Kind: protocol.KindAgent, Project: "garden"}, turn{user: "More beds", reply: "Ok."})
	outside, _ := te.sessionsSearch(context.Background(), protocol.SessionInfo{ID: "ask-q"}, protocol.MemorySessionsParams{Query: "Kyoto temple"})
	for _, hit := range outside.Sessions {
		if hit.Session == "agent-g" {
			t.Fatal("a conversation outside the kept project found it")
		}
	}
	inside, _ := te.sessionsSearch(context.Background(), protocol.SessionInfo{ID: "agent-g2", Kind: protocol.KindAgent, Project: "garden"}, protocol.MemorySessionsParams{Query: "Kyoto temple"})
	if len(inside.Sessions) != 1 || inside.Sessions[0].Session != "agent-g" {
		t.Fatalf("inside = %+v", inside.Sessions)
	}
	te.settings.Memory.Sessions = false
	if off, _ := te.sessionsSearch(context.Background(), protocol.SessionInfo{}, protocol.MemorySessionsParams{Query: "Kyoto"}); len(off.Sessions) != 0 {
		t.Fatalf("found %+v with the search off", off.Sessions)
	}
}
