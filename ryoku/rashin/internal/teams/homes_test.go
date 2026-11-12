package teams

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	registry "ryoku-rashin/internal/harness"
	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/protocol"
)

// signInForTeams signs team in to harness id the way a finished setup
// terminal does.
func signInForTeams(t *testing.T, settings protocol.Settings, id, team string) {
	t.Helper()
	opened, err := registry.OpenHome(id, team, settings, true, "")
	if err != nil {
		t.Fatal(err)
	}
	if _, _, signed, err := registry.FinishHome(opened.Token, 0); err != nil || !signed {
		t.Fatalf("signing %s in to %s: %v", team, id, err)
	}
}

// A task ends blocked before any session starts when its team is not signed
// in to its harness, though another team is, says why and names the harness
// so the app offers the sign-in; a retry after the team signs in runs it.
func TestATaskOfATeamNotSignedInToItsHarnessBlocks(t *testing.T) {
	h := newHarness(t, 0)
	h.s.signedIn = registry.SignedIn
	team := h.newTeam(t, "Ops", nil)
	other := h.newTeam(t, "Mail", nil)
	signInForTeams(t, h.currentSettings(), "fake", other.ID)
	task := h.newTask(t, team, "Look at the logs")
	waitFor(t, "the task to block", func() bool { return h.task(t, task.ID).Status == protocol.TaskBlocked })
	blocked := h.task(t, task.ID)
	if blocked.SignIn != "fake" || !strings.Contains(blocked.Reason, "not signed in") {
		t.Fatalf("blocked with sign-in %q and reason %q", blocked.SignIn, blocked.Reason)
	}
	if _, made := h.sessions.madeFor(task.ID); made {
		t.Fatal("a session started for a team not signed in")
	}
	signInForTeams(t, h.currentSettings(), "fake", team.ID)
	if _, err := h.s.retryTask(context.Background(), protocol.TeamTaskRetryParams{ID: task.ID}); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "the retried task to start", func() bool {
		_, made := h.sessions.madeFor(task.ID)
		return made
	})
	if current := h.task(t, task.ID); current.SignIn != "" {
		t.Fatalf("a retried task still names a sign-in: %+v", current)
	}
}

// What a run planted in its team's home is moved out before the team's next
// task starts, other teams' homes are left alone, and the user hears of it
// once: the same notice again only counts up.
func TestATaskStartPutsItsTeamsHomeBackAndFoldsTheSameNotice(t *testing.T) {
	h := newHarness(t, 0)
	h.s.signedIn = registry.SignedIn
	team := h.newTeam(t, "Ops", nil)
	other := h.newTeam(t, "Mail", nil)
	for _, id := range []string{team.ID, other.ID} {
		signInForTeams(t, h.currentSettings(), "fake", id)
	}
	plant := func(team string) string {
		planted := filepath.Join(registry.HomeOf("fake", team), ".omp", "agent", "extensions", "x.ts")
		if err := os.MkdirAll(filepath.Dir(planted), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(planted, []byte("export default () => {}"), 0o600); err != nil {
			t.Fatal(err)
		}
		return planted
	}
	otherPlanted := plant(other.ID)
	for round := range 2 {
		planted := plant(team.ID)
		task := h.newTask(t, team, "Look at the logs")
		waitFor(t, "the task to start", func() bool {
			_, made := h.sessions.madeFor(task.ID)
			return made
		})
		if _, err := os.Lstat(planted); !os.IsNotExist(err) {
			t.Fatalf("round %d: the planted extension is still in the team's home: %v", round, err)
		}
		do[protocol.TeamTask](t, h, protocol.MTeamsTaskStop, protocol.TeamTaskRef{ID: task.ID})
	}
	if _, err := os.Lstat(otherPlanted); err != nil {
		t.Fatalf("another team's home was touched: %v", err)
	}
	notice := h.pending(t)
	if notice.Kind != protocol.ApprovalRestore || notice.Team != team.ID || !strings.Contains(notice.Detail, ".omp") || !notice.Expires.IsZero() || notice.Repeats != 1 {
		t.Fatalf("notice %+v", notice)
	}
	if h.ledgerCount(t, "home.restore", "") != 2 {
		t.Fatal("the ledger does not have both restores")
	}
	read, err := h.s.decideApproval(context.Background(), protocol.ApprovalDecideParams{ID: notice.ID, Decision: protocol.DecideOnce})
	if err != nil || read.Status != protocol.ApprovalApproved {
		t.Fatalf("marking the notice read: %+v %v", read, err)
	}
}

// A home every team on a harness once shared becomes the home of the team
// whose task last ran on it; the other team is not signed in to it.
func TestASharedHomeBecomesTheHomeOfTheTeamThatRanLast(t *testing.T) {
	h := newHarness(t, 0)
	onOmp := func(p *protocol.TeamCreateParams) {
		p.Members = []protocol.Member{{Name: "Ana", Lead: true, Harness: "omp"}}
	}
	first := h.newTeam(t, "Ops", onOmp)
	last := h.newTeam(t, "Mail", onOmp)
	task := h.newTask(t, last, "Sort the mail")
	waitFor(t, "the task to start on omp", func() bool { return h.task(t, task.ID).Harness == "omp" })

	shared := filepath.Join(paths.HarnessHomes(), "omp")
	guard := filepath.Join(paths.HarnessHomeGuard(), "omp")
	for path, content := range map[string]string{
		filepath.Join(shared, ".omp", "agent", "models.yml"):           "providers: {}\n",
		filepath.Join(guard, "blessed", ".omp", "agent", "models.yml"): "providers: {}\n",
		filepath.Join(shared, ".gitconfig"):                            "[include]\n",
		filepath.Join(guard, "blessed", ".gitconfig"):                  "[include]\n",
		filepath.Join(guard, "signed-in.json"):                         `{"at":"2026-11-11T10:00:00Z"}`,
	} {
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	h.s.migrateHomes()
	if _, signed := registry.SignedIn("omp", last.ID); !signed {
		t.Fatal("the team that ran last is not signed in to the shared home")
	}
	if _, signed := registry.SignedIn("omp", first.ID); signed {
		t.Fatal("the other team is signed in to the shared home")
	}
	if _, err := os.Stat(filepath.Join(registry.HomeOf("omp", last.ID), ".omp", "agent", "models.yml")); err != nil {
		t.Fatalf("the shared home did not move: %v", err)
	}
	if restored, err := registry.Restore("omp", last.ID); err != nil || !restored.Empty() {
		t.Fatalf("the moved home does not match what was kept: %+v %v", restored, err)
	}
	if len(registry.LegacyHomes()) != 0 {
		t.Fatal("a shared home is left")
	}
}
