package teams

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"ryoku-rashin/internal/protocol"
)

func scheduledHarness(t *testing.T) *harness {
	t.Helper()
	return newHarnessWith(t, func(settings *protocol.Settings) {
		settings.Features = map[string]bool{"schedules": true, "research": true, "setupcards": true}
		settings.Teams.QuietHours = &protocol.QuietHours{From: "00:00", To: "23:59"}
	})
}

func TestExistingTaskRoutineRunsWithoutTheScheduledRunsFeature(t *testing.T) {
	h := newHarness(t, 0)
	team := h.newTeam(t, "Morning review", nil)
	routine := do[protocol.Routine](t, h, protocol.MTeamsRoutineCreate, protocol.RoutineCreateParams{
		Team: team.ID, Title: "Review changes", Brief: "Review the recent changes.", Schedule: "0 8 * * *",
	})
	task := do[protocol.TeamTask](t, h, protocol.MTeamsRoutineRun, protocol.RoutineRef{ID: routine.ID})
	if task.Team != team.ID || task.Routine != routine.ID || task.Origin != protocol.TaskFromRoutine {
		t.Fatalf("the existing routine did not make its team's task: %+v", task)
	}
	if _, err := os.Stat(filepath.Join(h.dir, "runs.jsonl")); !os.IsNotExist(err) {
		t.Fatalf("the disabled feature touched the run ledger: %v", err)
	}
	refused(t, h, protocol.MTeamsRoutineCreate, protocol.RoutineCreateParams{Kind: protocol.RoutineAsk, Title: "Plan", Brief: "Plan the day", Schedule: "0 8 * * *"}, protocol.CodeForbidden)
}

func TestPersonalQuestionRunKeepsItsSessionAndOutcome(t *testing.T) {
	h := scheduledHarness(t)
	h.clock.mu.Lock()
	h.clock.frozen = true
	h.clock.mu.Unlock()
	routine := do[protocol.Routine](t, h, protocol.MTeamsRoutineCreate, protocol.RoutineCreateParams{
		Kind: protocol.RoutineAsk, Title: "Plan", Brief: "Plan the morning", Schedule: "0 8 * * *", Enabled: true,
	})
	if routine.Team != protocol.RoutinePersonal || routine.NextRun == nil {
		t.Fatalf("the personal routine needed a team or has no next run: %+v", routine)
	}
	task := do[protocol.TeamTask](t, h, protocol.MTeamsRoutineRun, protocol.RoutineRef{ID: routine.ID})
	info, ok := h.sessions.Info(task.Session)
	if !ok || info.Kind != protocol.KindAsk || info.Team != "" {
		t.Fatalf("the routine did not open an independent Ask session: %+v", info)
	}
	h.clock.advance(42 * time.Second)
	info.State, info.Turns, info.Preview = protocol.StateIdle, 1, "Start with the important task."
	h.s.RoutineSessionEnded(info, protocol.TurnEndData{Turn: 1, Stop: protocol.StopEnd, Usage: &protocol.TokenUsage{Input: 17, Output: 9}})
	result := do[protocol.RoutineRunsResult](t, h, protocol.MTeamsRoutineRuns, protocol.RoutineRunsParams{Routine: routine.ID})
	if len(result.Runs) != 1 {
		t.Fatalf("got %d run records", len(result.Runs))
	}
	run := result.Runs[0]
	if run.Session != info.ID || run.Status != protocol.TaskDone || run.Ended == nil || run.Ended.Sub(run.Started) != 42*time.Second || run.Tokens != 26 {
		t.Fatalf("the durable run lost its outcome: %+v", run)
	}
	if stored, _ := h.s.state.routine(routine.ID); stored.LastStatus != protocol.TaskDone || stored.Failures != 0 {
		t.Fatalf("the routine did not follow the question's outcome: %+v", stored)
	}
}

func TestRunLedgerKeepsTheNewestThousandIndependentlyOfTasks(t *testing.T) {
	h := scheduledHarness(t)
	var data bytes.Buffer
	encoder := json.NewEncoder(&data)
	for index := range routineRunsKeep {
		if err := encoder.Encode(protocol.RoutineRun{ID: fmt.Sprintf("run-%04d", index), Routine: "morning", Task: "removed-task", Started: h.clock.now(), Status: protocol.TaskDone}); err != nil {
			t.Fatal(err)
		}
	}
	if err := writeStateFile(h.dir, "runs.jsonl", data.Bytes()); err != nil {
		t.Fatal(err)
	}
	if err := h.s.saveRoutineRun(protocol.RoutineRun{ID: "newest", Routine: "morning", Started: h.clock.now(), Status: protocol.TaskDone}); err != nil {
		t.Fatal(err)
	}
	runs, err := h.s.readRoutineRuns()
	if err != nil {
		t.Fatal(err)
	}
	if len(runs) != routineRunsKeep || runs[0].ID != "run-0001" || runs[len(runs)-1].ID != "newest" || runs[0].Task != "removed-task" {
		t.Fatalf("the durable ledger did not retain the newest runs after task removal: %d, first %+v, last %+v", len(runs), runs[0], runs[len(runs)-1])
	}
}

func TestSetupFinishedAndSkippedBothResumeWithTheOutcome(t *testing.T) {
	for _, decision := range []string{protocol.DecideOnce, protocol.DecideDeny} {
		t.Run(decision, func(t *testing.T) {
			h := scheduledHarness(t)
			team := h.newTeam(t, "Review", nil)
			task := h.newTask(t, team, "Review the local board.")
			ref := h.agent(t, task.ID)
			do[protocol.AgentAnswer](t, h, protocol.MTeamsAgentSetup, protocol.AgentSetupParams{AgentRef: ref, Title: "Connect the board", Steps: []string{"Plug in the cable", "Open the serial monitor"}, Why: "The last check needs the real board", Check: "The monitor shows a startup line"})
			asked := h.pending(t)
			if held := h.task(t, task.ID); held.Status != protocol.TaskNeedsYou || len(asked.Steps) != 2 || asked.Kind != protocol.ApprovalSetup {
				t.Fatalf("setup did not hold the task with its ordered steps: %+v, %+v", held, asked)
			}
			do[protocol.Approval](t, h, protocol.MTeamsApprovalDecide, protocol.ApprovalDecideParams{ID: asked.ID, Decision: decision, Note: "Use the USB port on the right"})
			waitFor(t, "the setup outcome to reach the agent", func() bool { return len(h.sessions.notes(ref.Session)) == 1 })
			note := h.sessions.notes(ref.Session)[0]
			want := "finished setup"
			if decision == protocol.DecideDeny {
				want = "skipped setup"
			}
			if !strings.Contains(note, want) || !strings.Contains(note, "USB port on the right") || h.task(t, task.ID).Status != protocol.TaskWorking {
				t.Fatalf("the agent did not continue with the right setup outcome: %q", note)
			}
		})
	}
}

func TestDisabledSetupDoesNotHoldAnExistingTask(t *testing.T) {
	h := newHarness(t, 0)
	team := h.newTeam(t, "Review", nil)
	task := h.newTask(t, team, "Review the local board.")
	ref := h.agent(t, task.ID)
	_, err := h.s.agentSetup(context.Background(), protocol.AgentSetupParams{AgentRef: ref, Title: "Connect the board", Why: "A final check", Steps: []string{"Connect it"}})
	if err == nil || h.task(t, task.ID).Status != protocol.TaskWorking || len(h.s.listApprovals(protocol.ApprovalsParams{Pending: true}).Approvals) != 0 {
		t.Fatal("the disabled setup feature held the task or created an approval")
	}
}

func TestResearchPresentationKeepsItsCitedSources(t *testing.T) {
	h := scheduledHarness(t)
	dream := do[protocol.Team](t, h, protocol.MTeamsDreamCreate, protocol.DreamCreateParams{Statement: "Compare the battery-health references.", Name: "Battery research", Preset: "research", Folder: filepath.Join(h.dir, "dreams", "research"), Harness: "fake"})
	ref := h.agent(t, dream.Dream.Task)
	h.readyToPresent(t, dream, ref)
	source := "https://docs.kernel.org/power/power_supply_class.html"
	do[protocol.AgentAnswer](t, h, protocol.MTeamsAgentPresent, protocol.AgentPresentParams{AgentRef: ref, Title: "Battery health", Body: "## Summary\nCharge and health differ.", Sources: []string{source, source}})
	kept := h.dream(t, dream.ID)
	if kept.Preset != "research" || kept.Presentation == nil || len(kept.Presentation.Sources) != 1 || kept.Presentation.Sources[0] != source {
		t.Fatalf("the report lost or duplicated its cited sources: %+v", kept)
	}
}

func setTeamFeature(h *harness, id string, on bool) {
	h.settingsMu.Lock()
	features := maps.Clone(h.settings.Features)
	features[id] = on
	h.settings.Features = features
	settings := h.settings
	h.settingsMu.Unlock()
	h.s.SettingsChanged(settings)
}

func TestDisablingSchedulesStopsQuestionsAndReenableReconcilesHistory(t *testing.T) {
	h := scheduledHarness(t)
	routine := do[protocol.Routine](t, h, protocol.MTeamsRoutineCreate, protocol.RoutineCreateParams{
		Kind: protocol.RoutineAsk, Title: "Plan", Brief: "Plan the morning", Schedule: "0 8 * * *", Enabled: true,
	})
	task := do[protocol.TeamTask](t, h, protocol.MTeamsRoutineRun, protocol.RoutineRef{ID: routine.ID})
	h.sessions.mu.Lock()
	info := h.sessions.infos[task.Session]
	info.State = protocol.StateWorking
	h.sessions.infos[task.Session] = info
	h.sessions.mu.Unlock()
	setTeamFeature(h, "schedules", false)
	runs, err := h.s.readRoutineRuns()
	if err != nil || len(runs) != 1 || runs[0].Ended == nil || runs[0].Status != protocol.TaskStopped || !h.sessions.wasStopped(task.Session) {
		t.Fatalf("disabling schedules left the question or its record open: %+v, %v", runs, err)
	}
	orphan := protocol.RoutineRun{ID: "interrupted-question", Routine: routine.ID, Kind: protocol.RoutineAsk, Session: "missing-question", Started: h.clock.now(), Status: protocol.TaskWorking}
	if err := h.s.saveRoutineRun(orphan); err != nil {
		t.Fatal(err)
	}
	setTeamFeature(h, "schedules", true)
	if h.s.scheduledOpen(routine.ID) {
		t.Fatal("unfinished history still blocks the next run after re-enabling schedules")
	}
	now := h.clock.now()
	h.s.checkRoutine(routine.ID, now, now)
	result := do[protocol.RoutineRunsResult](t, h, protocol.MTeamsRoutineRuns, protocol.RoutineRunsParams{Routine: routine.ID})
	if len(result.Runs) != 3 || result.Runs[0].Session == task.Session || result.Runs[0].Ended != nil {
		t.Fatalf("the next scheduled question was skipped after re-enabling: %+v", result.Runs)
	}
}

func TestAnOutstandingQuestionCanFinishWhileSchedulesTurnOff(t *testing.T) {
	h := scheduledHarness(t)
	routine := do[protocol.Routine](t, h, protocol.MTeamsRoutineCreate, protocol.RoutineCreateParams{Kind: protocol.RoutineAsk, Title: "Plan", Brief: "Plan the morning", Schedule: "0 8 * * *"})
	task := do[protocol.TeamTask](t, h, protocol.MTeamsRoutineRun, protocol.RoutineRef{ID: routine.ID})
	h.settingsMu.Lock()
	h.settings.Features = maps.Clone(h.settings.Features)
	h.settings.Features["schedules"] = false
	h.settingsMu.Unlock()
	info, _ := h.sessions.Info(task.Session)
	info.State, info.Preview = protocol.StateIdle, "Ready"
	h.s.RoutineSessionEnded(info, protocol.TurnEndData{Stop: protocol.StopEnd})
	runs, err := h.s.readRoutineRuns()
	if err != nil || len(runs) != 1 || runs[0].Ended == nil || runs[0].Status != protocol.TaskDone {
		t.Fatalf("the in-flight completion was lost when the setting changed: %+v, %v", runs, err)
	}
}

func TestDisablingResearchStopsItsDreamAndBlocksEveryRestartPath(t *testing.T) {
	h := scheduledHarness(t)
	dream := do[protocol.Team](t, h, protocol.MTeamsDreamCreate, protocol.DreamCreateParams{Statement: "Compare the battery-health references.", Name: "Battery research", Preset: "research", Folder: filepath.Join(h.dir, "dreams", "research"), Harness: "fake"})
	ref := h.agent(t, dream.Dream.Task)
	setTeamFeature(h, "research", false)
	waitFor(t, "the research session to stop", func() bool { return h.sessions.wasStopped(ref.Session) })
	if task := h.task(t, dream.Dream.Task); task.Status != protocol.TaskStopped {
		t.Fatalf("research kept running after it was disabled: %+v", task)
	}
	kept := h.dream(t, dream.ID)
	if kept.Statement != dream.Dream.Statement || kept.Preset != "research" || kept.Task != dream.Dream.Task {
		t.Fatalf("disabling research discarded its dream history: %+v", kept)
	}
	refused(t, h, protocol.MTeamsTaskCreate, protocol.TeamTaskCreateParams{Team: dream.ID, Brief: "Continue the research"}, protocol.CodeForbidden)
	queued := h.task(t, dream.Dream.Task)
	queued.Status, queued.Ended = protocol.TaskReady, nil
	if err := h.s.state.putTask(queued); err != nil {
		t.Fatal(err)
	}
	h.s.pump()
	h.s.runDream(dream.ID, protocol.InitiatorYou)
	if task := h.task(t, queued.ID); task.Status != protocol.TaskReady {
		t.Fatalf("a queued research task started while the feature was off: %+v", task)
	}
	queued.Status = protocol.TaskWorking
	if err := h.s.state.putTask(queued); err != nil {
		t.Fatal(err)
	}
	h.sessions.mu.Lock()
	info := h.sessions.infos[ref.Session]
	info.Goal = &protocol.Goal{Status: protocol.GoalActive}
	h.sessions.infos[ref.Session] = info
	h.sessions.mu.Unlock()
	h.s.recoverRuns()
	if task := h.task(t, queued.ID); task.Status != protocol.TaskStopped || !h.s.Hold(info) {
		t.Fatalf("startup recovery resumed disabled research: %+v", task)
	}
}

func TestDisablingSetupCardsSkipsTheCardAndReleasesItsTask(t *testing.T) {
	h := scheduledHarness(t)
	team := h.newTeam(t, "Review", nil)
	task := h.newTask(t, team, "Review the board.")
	ref := h.agent(t, task.ID)
	do[protocol.AgentAnswer](t, h, protocol.MTeamsAgentSetup, protocol.AgentSetupParams{AgentRef: ref, Title: "Connect the board", Steps: []string{"Connect its cable"}, Why: "The check needs the board"})
	asked := h.pending(t)
	setTeamFeature(h, "setupcards", false)
	waitFor(t, "the skipped setup to reach the agent", func() bool { return len(h.sessions.notes(ref.Session)) == 1 })
	rec, _ := h.s.state.approval(asked.ID)
	if rec.Status != protocol.ApprovalDenied || !strings.Contains(rec.Result, "skipped setup") {
		t.Fatalf("the disabled card was not settled through its setup effect: %+v", rec)
	}
	if current := h.task(t, task.ID); current.Status != protocol.TaskWorking || current.Waiting != "" || h.s.Hold(h.sessions.info(ref.Session)) {
		t.Fatalf("hiding the setup card left the task held: %+v", current)
	}
}
