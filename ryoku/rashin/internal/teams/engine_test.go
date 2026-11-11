package teams

import (
	"context"
	"maps"
	"strings"
	"testing"

	"ryoku-rashin/internal/protocol"
)

// A member on the default harness runs on the teams defaults, the registry's
// default harness last; a task's own model and options go over them, and the
// task keeps what it started on.
func TestATaskStartsOnItsResolvedEngineAndKeepsIt(t *testing.T) {
	h := newHarnessWith(t, func(s *protocol.Settings) {
		s.Teams.Defaults.Model = "default-model"
		s.Teams.Defaults.Options = map[string]string{"effort": "low"}
	})
	team := h.newTeam(t, "Research", func(p *protocol.TeamCreateParams) {
		p.Members = []protocol.Member{{Name: "Ana", Lead: true}}
	})
	created := do[protocol.TeamTask](t, h, protocol.MTeamsTaskCreate, protocol.TeamTaskCreateParams{
		Team: team.ID, Brief: "Read the release notes.", Model: "task-model", Options: map[string]string{"mode": "fast"},
	})
	ref := h.agent(t, created.ID)
	made, _ := h.sessions.madeFor(created.ID)
	if made.Harness != "fake" {
		t.Fatalf("the session was made on harness %q, want the registry's default", made.Harness)
	}
	h.sessions.mu.Lock()
	model, options := h.sessions.models[ref.Session], maps.Clone(h.sessions.options[ref.Session])
	h.sessions.mu.Unlock()
	want := map[string]string{"effort": "low", "mode": "fast"}
	if model != "task-model" || !maps.Equal(options, want) {
		t.Fatalf("the session runs on model %q with %v, want task-model with %v", model, options, want)
	}
	started := h.task(t, created.ID)
	if started.Harness != "fake" || started.Model != "task-model" || !maps.Equal(started.Options, want) {
		t.Fatalf("the task keeps the engine %q %q %v", started.Harness, started.Model, started.Options)
	}
	var detail string
	for _, a := range h.ledger(t) {
		if a.Action == "task.start" && a.Task == created.ID {
			detail = a.Detail
		}
	}
	if detail != "Ana started work on it with Fake, task-model." {
		t.Fatalf("the ledger says %q", detail)
	}

	do[protocol.AgentAnswer](t, h, protocol.MTeamsAgentReport, protocol.AgentReportParams{AgentRef: ref, Body: "Read."})
	if ended := h.task(t, created.ID); ended.Harness != "fake" || ended.Model != "task-model" {
		t.Fatalf("a finished task forgot its engine: %q %q", ended.Harness, ended.Model)
	}
}

func TestARefusedOptionLeavesTheTaskWorkingOnTheHarnessDefault(t *testing.T) {
	h := newHarness(t, 0)
	h.sessions.mu.Lock()
	h.sessions.refuse["effort"] = true
	h.sessions.mu.Unlock()
	team := h.newTeam(t, "Ops", func(p *protocol.TeamCreateParams) {
		p.Members = []protocol.Member{{Name: "Ana", Lead: true, Harness: "fake", Options: map[string]string{"effort": "high", "mode": "plan"}}}
	})
	task := h.newTask(t, team, "Check the disks.")
	h.agent(t, task.ID)
	working := h.task(t, task.ID)
	if working.Status != protocol.TaskWorking {
		t.Fatalf("a task with a refused option is %q, want working", working.Status)
	}
	if _, kept := working.Options["effort"]; kept || working.Options["mode"] != "plan" {
		t.Fatalf("the task records the options %v, want only what the harness took", working.Options)
	}
	var said string
	for _, a := range h.ledger(t) {
		if a.Action == "task.option" {
			said = a.Detail
		}
	}
	if said != "Ana's effort high was not available; it ran with the harness default." {
		t.Fatalf("the ledger says %q about the refused option", said)
	}
}

func TestAMembersEngineMustBeOneThisComputerCanRun(t *testing.T) {
	h := newHarness(t, 0)
	for _, member := range []protocol.Member{
		{Name: "Ana", Lead: true, Harness: "nowhere"},
		{Name: "Ana", Lead: true, Harness: "fake", Model: "two words"},
		{Name: "Ana", Lead: true, Harness: "fake", Options: map[string]string{"effort": ""}},
	} {
		refused(t, h, protocol.MTeamsCreate, protocol.TeamCreateParams{Name: "Bad", Folder: t.TempDir(), Members: []protocol.Member{member}}, protocol.CodeBadRequest)
	}
	team := h.newTeam(t, "Good", nil)
	members := append([]protocol.Member(nil), team.Members...)
	members[0].Harness = "nowhere"
	refused(t, h, protocol.MTeamsUpdate, protocol.TeamUpdateParams{ID: team.ID, Patch: mustJSON(t, map[string]any{"members": members})}, protocol.CodeBadRequest)
	refused(t, h, protocol.MTeamsTaskCreate, protocol.TeamTaskCreateParams{Team: team.ID, Brief: "Go.", Model: "has space"}, protocol.CodeBadRequest)
}

func TestAnEngineChangeIsSaidInTheLedger(t *testing.T) {
	h := newHarness(t, 0)
	team := h.newTeam(t, "Desk", nil)
	members := append([]protocol.Member(nil), team.Members...)
	members[1].Model = "bigger"
	do[protocol.Team](t, h, protocol.MTeamsUpdate, protocol.TeamUpdateParams{ID: team.ID, Patch: mustJSON(t, map[string]any{
		"members": members, "budget": map[string]any{"taskMinutes": 15},
	})})
	var detail string
	for _, a := range h.ledger(t) {
		if a.Action == "team.update" {
			detail = a.Detail
		}
	}
	want := "Changed Ben's engine from Fake, its default model to Fake, bigger. Set minutes per task to 15."
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
	info.Model = "opus"
	h.s.SessionChanged(info)
	if got := h.task(t, task.ID).Model; got != "opus" {
		t.Fatalf("after a switch the task records model %q, want opus", got)
	}
	var detail string
	for _, a := range h.ledger(t) {
		if a.Action == "task.model" {
			detail = a.Detail
		}
	}
	if detail != "You switched Ana to opus for this task." {
		t.Fatalf("the ledger says %q", detail)
	}
	h.s.SessionChanged(info)
	if got := h.ledgerCount(t, "task.model", task.ID); got != 1 {
		t.Fatalf("one switch was recorded %d times", got)
	}
}

func TestSpendIsKeptByEngine(t *testing.T) {
	h := newHarness(t, 0)
	team := h.newTeam(t, "Counter", func(p *protocol.TeamCreateParams) {
		p.Members = []protocol.Member{{Name: "Ana", Lead: true, Harness: "fake", Model: "small"}}
	})
	ref := h.agent(t, h.newTask(t, team, "Count.").ID)
	h.s.AfterTurn(context.Background(), h.sessions.info(ref.Session), protocol.TurnEndData{Usage: &protocol.TokenUsage{Input: 100, Output: 50}})
	spend := do[protocol.TeamSpend](t, h, protocol.MTeamsSpend, protocol.TeamSpendParams{Team: team.ID})
	if len(spend.Engines) != 1 {
		t.Fatalf("the spend lists engines %+v, want one", spend.Engines)
	}
	engine := spend.Engines[0]
	id, model, _ := strings.Cut(engine.ID, protocol.EngineSpendSeparator)
	if id != "fake" || model != "small" || engine.Tokens != 150 || engine.Tasks != 1 {
		t.Fatalf("the engine spend is %+v", engine)
	}
}
