package daemon

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"ryoku-rashin/internal/protocol"
)

// projectWork is Work that knows the projects at roots.
type projectWork struct {
	*fakeWork
	roots []string
}

func (p projectWork) Projects(context.Context) []protocol.ProjectInfo {
	out := make([]protocol.ProjectInfo, 0, len(p.roots))
	for _, root := range p.roots {
		out = append(out, protocol.ProjectInfo{Root: root})
	}
	return out
}

func git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_AUTHOR_NAME=Test", "GIT_AUTHOR_EMAIL=test@example.invalid", "GIT_COMMITTER_NAME=Test", "GIT_COMMITTER_EMAIL=test@example.invalid")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v: %s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func checkpointManager(t *testing.T, work projectWork) *sessionManager {
	t.Helper()
	isolateDaemon(t)
	settings := func() protocol.Settings {
		return protocol.Settings{Agent: protocol.AgentSettings{Approvals: protocol.ApprovalsReadOnly}}
	}
	return newSessionManager(nil, &fakeHarnesses{runner: answering("done")}, work, settings, func(string, any) {}, func() {}, func(string, ...any) {})
}

// A temporary agent conversation takes no turn checkpoint, so it has no
// turn to revert; a stored one takes one each turn.
func TestTemporaryAgentTakesNoCheckpoint(t *testing.T) {
	work := projectWork{fakeWork: &fakeWork{checkpoint: true}}
	manager := checkpointManager(t, work)
	temporary, err := manager.create(protocol.SessionNewParams{Kind: protocol.KindAgent, Temporary: true})
	if err != nil {
		t.Fatal(err)
	}
	finishTurn(t, manager, temporary.ID, "a question")
	if work.beforeCalls != 0 {
		t.Fatalf("a temporary turn took %d checkpoints", work.beforeCalls)
	}
	if info, _ := manager.Info(temporary.ID); info.Checkpoints {
		t.Fatal("a temporary session says it has checkpoints")
	}
	if _, err := manager.revertTurn(context.Background(), protocol.TurnParams{ID: temporary.ID, Turn: 1}); codeOf(err) != protocol.CodeUnavailable {
		t.Fatalf("revert in a temporary conversation = %v", err)
	}
	stored, err := manager.create(protocol.SessionNewParams{Kind: protocol.KindAgent})
	if err != nil {
		t.Fatal(err)
	}
	finishTurn(t, manager, stored.ID, "a question")
	if work.beforeCalls != 1 {
		t.Fatalf("a stored turn took %d checkpoints", work.beforeCalls)
	}
}

// At start the checkpoint refs of sessions that are not stored are swept
// from every project; a stored session's and a held one's stay.
func TestSweepCheckpointsKeepsOnlyLiveSessions(t *testing.T) {
	repo := t.TempDir()
	git(t, repo, "init", "-b", "main")
	if err := os.WriteFile(filepath.Join(repo, "file.txt"), []byte("one\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	git(t, repo, "add", ".")
	git(t, repo, "commit", "-m", "initial")
	work := projectWork{fakeWork: &fakeWork{}, roots: []string{repo, filepath.Join(repo, "missing")}}
	manager := checkpointManager(t, work)
	stored, err := manager.create(protocol.SessionNewParams{Kind: protocol.KindAgent})
	if err != nil {
		t.Fatal(err)
	}
	held, err := manager.create(protocol.SessionNewParams{Kind: protocol.KindAgent, Temporary: true})
	if err != nil {
		t.Fatal(err)
	}
	gone := "agent-20261112-120000-dead"
	for _, id := range []string{stored.ID, held.ID, gone} {
		git(t, repo, "update-ref", checkpointRefs+id+"/1", "HEAD")
		git(t, repo, "update-ref", checkpointRefs+id+"/1-post", "HEAD")
	}
	manager.sweepCheckpoints(context.Background())
	refs := git(t, repo, "for-each-ref", "--format=%(refname)", checkpointRefs)
	want := []string{
		checkpointRefs + held.ID + "/1", checkpointRefs + held.ID + "/1-post",
		checkpointRefs + stored.ID + "/1", checkpointRefs + stored.ID + "/1-post",
	}
	if strings.Contains(refs, gone) || len(strings.Fields(refs)) != len(want) {
		t.Fatalf("refs after the sweep = %q", refs)
	}
	for _, ref := range want {
		if !strings.Contains(refs, ref) {
			t.Fatalf("the sweep took %s: %q", ref, refs)
		}
	}
}
