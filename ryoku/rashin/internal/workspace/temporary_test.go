package workspace

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"ryoku-rashin/internal/protocol"
)

// A temporary conversation's turn leaves no checkpoint ref in the
// repository, while a stored one's keeps its pre and post refs.
func TestTemporaryTurnLeavesNoCheckpointRef(t *testing.T) {
	work, repo, _ := testWork(t, fakeClient{})
	_ = addTestProject(t, work, repo)
	file := filepath.Join(repo, "file.txt")
	stored := protocol.SessionInfo{ID: "agent-stored", Kind: protocol.KindAgent, Cwd: repo, State: protocol.StateIdle}
	if ok, err := work.BeforeTurn(context.Background(), stored, 1); err != nil || !ok {
		t.Fatalf("checkpoint: ok=%v err=%v", ok, err)
	}
	if err := os.WriteFile(file, []byte("two\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	work.AfterTurn(context.Background(), stored, 1)
	if refs := runGit(t, repo, "for-each-ref", "--format=%(refname)", "refs/rashin/"); refs != "refs/rashin/checkpoints/agent-stored/1\nrefs/rashin/checkpoints/agent-stored/1-post" {
		t.Fatalf("stored refs = %q", refs)
	}
	runGit(t, repo, "update-ref", "-d", "refs/rashin/checkpoints/agent-stored/1")
	runGit(t, repo, "update-ref", "-d", "refs/rashin/checkpoints/agent-stored/1-post")

	temporary := protocol.SessionInfo{ID: "agent-temporary", Kind: protocol.KindAgent, Cwd: repo, State: protocol.StateIdle, Temporary: true}
	if err := os.WriteFile(file, []byte("three\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	work.AfterTurn(context.Background(), temporary, 1)
	if refs := runGit(t, repo, "for-each-ref", "--format=%(refname)", "refs/rashin/"); refs != "" {
		t.Fatalf("a temporary turn left %q", refs)
	}
}
