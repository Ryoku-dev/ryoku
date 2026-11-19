package teams

import (
	"context"
	"errors"
	"maps"
	"strings"
	"testing"

	"ryoku-rashin/internal/protocol"
)

// A member without a model runs on the teams defaults; a task's own model
// and options go over them, the session is made on them, and the task keeps
// what it started on.
func TestATaskStartsOnItsResolvedEngineAndKeepsIt(t *testing.T) {
	h := newHarnessWith(t, func(s *protocol.Settings) {
		s.Teams.Defaults.Model = "test/default-model"
		s.Teams.Defaults.Options = map[string]string{"effort": "low"}
	})
	team := h.newTeam(t, "Research", func(p *protocol.TeamCreateParams) {
		p.Members = []protocol.Member{{Name: "Ana", Lead: true}}
	})
	created := do[protocol.TeamTask](t, h, protocol.MTeamsTaskCreate, protocol.TeamTaskCreateParams{
		Team: team.ID, Brief: "Read the release notes.", Model: "test/task-model", Options: map[string]string{"plan": "on"},
	})
	ref := h.agent(t, created.ID)
	want := map[string]string{"effort": "low", "plan": "on"}
	made, _ := h.sessions.madeFor(created.ID)
	if made.Model != "test/task-model" || !maps.Equal(made.Options, want) {
		t.Fatalf("the session was made on %q with %v, want test/task-model with %v", made.Model, made.Options, want)
	}
	started := h.task(t, created.ID)
	if started.Model != "test/task-model" || !maps.Equal(started.Options, want) {
		t.Fatalf("the task keeps the engine %q %v", started.Model, started.Options)
	}
	var detail string
	for _, a := range h.ledger(t) {
		if a.Action == "task.start" && a.Task == created.ID {
			detail = a.Detail
		}
	}
	if detail != "Ana started work on it with test/task-model." {
		t.Fatalf("the ledger says %q", detail)
	}

	do[protocol.AgentAnswer](t, h, protocol.MTeamsAgentReport, protocol.AgentReportParams{AgentRef: ref, Body: "Read."})
	if ended := h.task(t, created.ID); ended.Model != "test/task-model" {
		t.Fatalf("a finished task forgot its engine: %q", ended.Model)
	}
}

// A task whose model cannot run ends blocked before any session is made,
// says what to change, and runs once its member is on a model that can.
func TestATaskOnAModelThatCannotRunBlocksUntilRetried(t *testing.T) {
	h := newHarness(t, 0)
	for _, test := range []struct{ model, says string }{
		{"nowhere/model", "Rashin has no provider nowhere for nowhere/model."},
		{"off/model", "off/model is turned off or has no key."},
		{"test/absent", "Rashin could not find test/absent among test's models."},
		{"test/text", "test/text cannot use tools, which a team task needs."},
	} {
		team := h.newTeam(t, "Ops "+strings.ReplaceAll(test.model, "/", "-"), func(p *protocol.TeamCreateParams) {
			p.Members = []protocol.Member{{Name: "Ana", Lead: true, Model: test.model}}
		})
		task := h.newTask(t, team, "Look at the logs.")
		waitFor(t, "the task to block", func() bool { return h.task(t, task.ID).Status == protocol.TaskBlocked })
		blocked := h.task(t, task.ID)
		if !strings.HasPrefix(blocked.Reason, test.says) || !strings.Contains(blocked.Reason, "retry the task") || blocked.Model != test.model {
			t.Fatalf("a task on %s blocked with %q on %q", test.model, blocked.Reason, blocked.Model)
		}
		if _, made := h.sessions.madeFor(task.ID); made {
			t.Fatalf("a session was made for a task on %s", test.model)
		}
		members := append([]protocol.Member(nil), team.Members...)
		members[0].Model = "test/small"
		do[protocol.Team](t, h, protocol.MTeamsUpdate, protocol.TeamUpdateParams{ID: team.ID, Patch: mustJSON(t, map[string]any{"members": members})})
		do[protocol.TeamTask](t, h, protocol.MTeamsTaskRetry, protocol.TeamTaskRetryParams{ID: task.ID})
		ref := h.agent(t, task.ID)
		if made, _ := h.sessions.madeFor(task.ID); made.Model != "test/small" {
			t.Fatalf("the retried task runs on %q, want test/small", made.Model)
		}
		do[protocol.AgentAnswer](t, h, protocol.MTeamsAgentReport, protocol.AgentReportParams{AgentRef: ref, Body: "Checked."})
	}
}

func TestAModelListingFailureBlocksWithARetryReason(t *testing.T) {
	h := newHarness(t, 0)
	h.s.SetModels(func(context.Context, protocol.ModelsParams) (protocol.ModelsResult, error) {
		return protocol.ModelsResult{}, errors.New("the provider list timed out")
	})
	team := h.newTeam(t, "Check models", nil)
	task := h.newTask(t, team, "Prepare the plan.")
	waitFor(t, "model listing to block the task", func() bool {
		return h.task(t, task.ID).Status == protocol.TaskBlocked
	})
	if blocked := h.task(t, task.ID); !strings.Contains(blocked.Reason, "provider list timed out") || !strings.Contains(blocked.Reason, "retry the task") {
		t.Fatalf("a listing failure was not retryable: %+v", blocked)
	}
	if _, made := h.sessions.madeFor(task.ID); made {
		t.Fatal("the task started without a checked model")
	}
}

func TestAMembersEngineMustBeOneRashinCanRun(t *testing.T) {
	h := newHarness(t, 0)
	for _, member := range []protocol.Member{
		{Name: "Ana", Lead: true, Model: "haiku"},
		{Name: "Ana", Lead: true, Model: "two words"},
		{Name: "Ana", Lead: true, Options: map[string]string{"effort": ""}},
	} {
		refused(t, h, protocol.MTeamsCreate, protocol.TeamCreateParams{Name: "Bad", Folder: t.TempDir(), Members: []protocol.Member{member}}, protocol.CodeBadRequest)
	}
	team := h.newTeam(t, "Good", nil)
	members := append([]protocol.Member(nil), team.Members...)
	members[0].Model = "haiku"
	refused(t, h, protocol.MTeamsUpdate, protocol.TeamUpdateParams{ID: team.ID, Patch: mustJSON(t, map[string]any{"members": members})}, protocol.CodeBadRequest)
	for _, model := range []string{"has space", "haiku"} {
		refused(t, h, protocol.MTeamsTaskCreate, protocol.TeamTaskCreateParams{Team: team.ID, Brief: "Go.", Model: model}, protocol.CodeBadRequest)
	}
}

func TestAnEngineChangeIsSaidInTheLedger(t *testing.T) {
	h := newHarness(t, 0)
	team := h.newTeam(t, "Desk", nil)
	members := append([]protocol.Member(nil), team.Members...)
	members[1].Model = "test/bigger"
	do[protocol.Team](t, h, protocol.MTeamsUpdate, protocol.TeamUpdateParams{ID: team.ID, Patch: mustJSON(t, map[string]any{
		"members": members, "budget": map[string]any{"taskMinutes": 15},
	})})
	var detail string
	for _, a := range h.ledger(t) {
		if a.Action == "team.update" {
			detail = a.Detail
		}
	}
	want := "Changed Ben's engine from the teams default to test/bigger. Set minutes per task to 15."
	if detail != want {
		t.Fatalf("the ledger says %q, want %q", detail, want)
	}
	do[protocol.Team](t, h, protocol.MTeamsUpdate, protocol.TeamUpdateParams{ID: team.ID, Patch: mustJSON(t, map[string]any{"name": "Desk"})})
	if got := h.ledgerCount(t, "team.update", ""); got != 1 {
		t.Fatalf("a change that changed nothing was recorded; %d updates in the ledger", got)
	}
}

func TestATaskFollowsAModelTheUserSwitchesTo(t *testing.T) {
	h := newHarness(t, 0)
	team := h.newTeam(t, "Desk", nil)
	task := h.newTask(t, team, "Draft the notes.")
	ref := h.agent(t, task.ID)
	info := h.sessions.info(ref.Session)
	info.Model = "test/opus"
	h.s.SessionChanged(info)
	if got := h.task(t, task.ID).Model; got != "test/opus" {
		t.Fatalf("after a switch the task records model %q, want test/opus", got)
	}
	var detail string
	for _, a := range h.ledger(t) {
		if a.Action == "task.model" {
			detail = a.Detail
		}
	}
	if detail != "You switched Ana to test/opus for this task." {
		t.Fatalf("the ledger says %q", detail)
	}
	h.s.SessionChanged(info)
	if got := h.ledgerCount(t, "task.model", task.ID); got != 1 {
		t.Fatalf("one switch was recorded %d times", got)
	}
}

func TestSpendIsKeptByModel(t *testing.T) {
	h := newHarness(t, 0)
	team := h.newTeam(t, "Counter", func(p *protocol.TeamCreateParams) {
		p.Members = []protocol.Member{{Name: "Ana", Lead: true, Model: "test/small"}}
	})
	ref := h.agent(t, h.newTask(t, team, "Count.").ID)
	h.s.AfterTurn(context.Background(), h.sessions.info(ref.Session), protocol.TurnEndData{Usage: &protocol.TokenUsage{Input: 100, Output: 50}})
	spend := do[protocol.TeamSpend](t, h, protocol.MTeamsSpend, protocol.TeamSpendParams{Team: team.ID})
	if len(spend.Engines) != 1 {
		t.Fatalf("the spend lists engines %+v, want one", spend.Engines)
	}
	if engine := spend.Engines[0]; engine.ID != "test/small" || engine.Tokens != 150 || engine.Tasks != 1 {
		t.Fatalf("the engine spend is %+v", engine)
	}
}
