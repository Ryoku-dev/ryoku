package teams

import (
	"context"
	"errors"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"ryoku-rashin/internal/protocol"
)

// newDream starts a dream in a folder of the test's own and waits for its
// manager's standing task to start, returning the dream and the manager's
// tool reference.
func (h *harness) newDream(t *testing.T) (protocol.Team, protocol.AgentRef) {
	t.Helper()
	dream := do[protocol.Team](t, h, protocol.MTeamsDreamCreate, protocol.DreamCreateParams{
		Statement: "Make an ESP32 page that appears when I join its Wi-Fi.", Name: "ESP32 setup",
		Folder: filepath.Join(h.dir, "dreams", "esp32"), Model: "test/big", Options: map[string]string{"effort": "high"},
	})
	if dream.Dream == nil || dream.Dream.Task == "" || dream.Dream.Manager == "" || dream.Approvals != protocol.ApprovalsAuto || dream.Budget.MonthTokens != 0 {
		t.Fatalf("the dream started as %+v", dream)
	}
	return dream, h.agent(t, dream.Dream.Task)
}

func (h *harness) dream(t *testing.T, id string) protocol.Dream {
	t.Helper()
	team, ok := h.s.state.team(id)
	if !ok || team.Dream == nil {
		t.Fatalf("no dream %s", id)
	}
	return *team.Dream
}

// answer runs an agent method that must answer in words.
func (h *harness) answer(t *testing.T, method string, params any) string {
	t.Helper()
	return do[protocol.AgentAnswer](t, h, method, params).Text
}

// interview has the manager ask the CEO one round and the CEO answer it.
func (h *harness) interview(t *testing.T, ref protocol.AgentRef) {
	t.Helper()
	h.answer(t, protocol.MTeamsAgentAsk, protocol.AgentAskParams{AgentRef: ref, Title: "Light or dark?", Detail: "Plan: Ada builds it, Bo tests it."})
	do[protocol.Approval](t, h, protocol.MTeamsApprovalDecide, protocol.ApprovalDecideParams{ID: h.pending(t).ID, Decision: protocol.DecideOnce, Note: "Dark."})
	waitFor(t, "the interview answer to reach the manager", func() bool {
		info := h.sessions.info(ref.Session)
		task := h.task(t, info.TeamTask)
		return task.Status == protocol.TaskWorking && task.Waiting == ""
	})
}

// buildRound has a hired member build a piece of this round and report it,
// hiring Ada first when the team has no one.
func (h *harness) buildRound(t *testing.T, dream protocol.Team, ref protocol.AgentRef) {
	t.Helper()
	waitFor(t, "the manager to be ready for this build round", func() bool {
		return h.task(t, dream.Dream.Task).Status == protocol.TaskWorking && h.task(t, dream.Dream.Task).Waiting == ""
	})
	if team, _ := h.s.state.team(dream.ID); len(team.Members) == 1 {
		do[protocol.Member](t, h, protocol.MTeamsAgentHire, protocol.AgentHireParams{AgentRef: ref, Name: "Ada", Title: "Engineer", Instructions: "You build it."})
	}
	h.answer(t, protocol.MTeamsAgentDelegate, protocol.AgentDelegateParams{AgentRef: ref, Member: "Ada", Title: "Build", Brief: "Build it."})
	piece := h.s.state.tasks(func(task protocol.TeamTask) bool { return task.Parent == dream.Dream.Task && taskIsOpen(task) })[0]
	h.answer(t, protocol.MTeamsAgentReport, protocol.AgentReportParams{AgentRef: h.agent(t, piece.ID), Title: "Built", Body: "It works."})
	waitFor(t, "the manager to hear the piece", func() bool { return h.task(t, dream.Dream.Task).Waiting == "" })
}

// readyToPresent interviews the CEO and has a member build this round.
func (h *harness) readyToPresent(t *testing.T, dream protocol.Team, ref protocol.AgentRef) {
	t.Helper()
	h.interview(t, ref)
	h.buildRound(t, dream, ref)
}

// The manager hires on its own engine, says where the dream stands and
// presents it; each step is in the ledger, and a member still working cannot
// be released until its piece is in.
func TestADreamsManagerHiresSetsPhasesReleasesAndPresents(t *testing.T) {
	h := newHarness(t, 0)
	dream, ref := h.newDream(t)
	hired := do[protocol.Member](t, h, protocol.MTeamsAgentHire, protocol.AgentHireParams{
		AgentRef: ref, Name: "Ada", Title: "Firmware engineer", Instructions: "You own the ESP32 firmware.", Why: "The page needs firmware.",
	})
	if hired.Model != "test/big" || hired.Options["effort"] != "high" || hired.Lead {
		t.Fatalf("hired %+v", hired)
	}
	h.answer(t, protocol.MTeamsAgentDream, protocol.AgentDreamParams{AgentRef: ref, Phase: protocol.DreamBuilding, Summary: "Ada builds the firmware.", Brief: "A captive page."})
	if d := h.dream(t, dream.ID); d.Phase != protocol.DreamBuilding || d.Summary != "Ada builds the firmware." || d.Brief != "A captive page." || d.Hired != 1 {
		t.Fatalf("the dream reads %+v", d)
	}
	refused(t, h, protocol.MTeamsAgentDream, protocol.AgentDreamParams{AgentRef: ref, Phase: protocol.DreamDone, Summary: "Done."}, protocol.CodeBadRequest)
	h.answer(t, protocol.MTeamsAgentDelegate, protocol.AgentDelegateParams{AgentRef: ref, Member: "Ada", Title: "Firmware", Brief: "Write the firmware."})
	refused(t, h, protocol.MTeamsAgentRelease, protocol.AgentReleaseParams{AgentRef: ref, Member: "Ada"}, protocol.CodeConflict)
	piece := h.s.state.tasks(func(task protocol.TeamTask) bool { return task.Parent == dream.Dream.Task })[0]
	member := h.agent(t, piece.ID)
	h.answer(t, protocol.MTeamsAgentReport, protocol.AgentReportParams{AgentRef: member, Title: "Firmware", Body: "It builds."})
	waitFor(t, "the manager to be continued with Ada's report", func() bool {
		return slices.ContainsFunc(h.sessions.notes(ref.Session), func(note string) bool { return strings.Contains(note, "It builds.") })
	})
	h.answer(t, protocol.MTeamsAgentRelease, protocol.AgentReleaseParams{AgentRef: ref, Member: "Ada", Why: "The firmware is in."})
	if team, _ := h.s.state.team(dream.ID); len(team.Members) != 1 {
		t.Fatalf("Ada is still on the team: %+v", team.Members)
	}
	h.interview(t, ref)
	h.answer(t, protocol.MTeamsAgentPresent, protocol.AgentPresentParams{AgentRef: ref, Title: "The setup page", Body: "Join ESP32-Setup.", Tests: "Served on 192.168.4.1.", Files: []string{"index.html"}})
	d := h.dream(t, dream.ID)
	if d.Phase != protocol.DreamPresenting || d.Rounds != 1 || d.Presentation == nil || d.Presentation.Files[0] != filepath.Join(dream.Folder, "index.html") {
		t.Fatalf("presented as %+v", d)
	}
	card := h.pending(t)
	if !slices.Equal(card.Options, []string{"Accept", "Send notes"}) || !card.Expires.IsZero() || h.task(t, dream.Dream.Task).Status != protocol.TaskNeedsYou {
		t.Fatalf("the presentation card is %+v", card)
	}
	for _, action := range []string{"dream.start", "dream.hire", "dream.phase", "dream.release", "dream.present"} {
		if h.ledgerCount(t, action, "") != 1 {
			t.Fatalf("the ledger has %d %s", h.ledgerCount(t, action, ""), action)
		}
	}
}

// Only a dream's manager may use the dream's tools: a member it hired may
// not, and neither may the lead of an ordinary team.
func TestOnlyADreamsManagerUsesTheDreamTools(t *testing.T) {
	h := newHarness(t, 0)
	dream, ref := h.newDream(t)
	do[protocol.Member](t, h, protocol.MTeamsAgentHire, protocol.AgentHireParams{AgentRef: ref, Name: "Ada", Title: "Engineer", Instructions: "You build it."})
	h.answer(t, protocol.MTeamsAgentDelegate, protocol.AgentDelegateParams{AgentRef: ref, Member: "Ada", Title: "Build", Brief: "Build it."})
	piece := h.s.state.tasks(func(task protocol.TeamTask) bool { return task.Parent == dream.Dream.Task })[0]
	member := h.agent(t, piece.ID)
	refused(t, h, protocol.MTeamsAgentHire, protocol.AgentHireParams{AgentRef: member, Name: "Bo", Title: "Tester", Instructions: "You test."}, protocol.CodeForbidden)
	refused(t, h, protocol.MTeamsAgentPresent, protocol.AgentPresentParams{AgentRef: member, Title: "Mine", Body: "Mine."}, protocol.CodeForbidden)
	team := h.newTeam(t, "Ordinary", nil)
	lead := h.agent(t, h.newTask(t, team, "Do it.").ID)
	refused(t, h, protocol.MTeamsAgentDream, protocol.AgentDreamParams{AgentRef: lead, Phase: protocol.DreamBuilding, Summary: "Mine."}, protocol.CodeForbidden)
	if tc := h.s.ToolContext(h.sessions.info(ref.Session)); tc == nil || !tc.Dream || !tc.Lead || tc.Token != ref.Token {
		t.Fatalf("the manager's tools are not the dream's: %+v", tc)
	}
	if tc := h.s.ToolContext(h.sessions.info(member.Session)); tc != nil && tc.Dream {
		t.Fatalf("a member was given the dream's tools: %+v", tc)
	}
}

// Accepting the presentation ends the dream: the standing task is done with
// the presentation as its report, its goal ends, and the dream is done.
func TestAcceptingAPresentationEndsTheDreamAndItsStandingTask(t *testing.T) {
	h := newHarness(t, 0)
	dream, ref := h.newDream(t)
	h.readyToPresent(t, dream, ref)
	h.answer(t, protocol.MTeamsAgentPresent, protocol.AgentPresentParams{AgentRef: ref, Title: "The setup page", Body: "Join ESP32-Setup."})
	refused(t, h, protocol.MTeamsDreamAnswer, protocol.DreamAnswerParams{Team: dream.ID, Verdict: protocol.DreamNotes}, protocol.CodeBadRequest)
	accepted := do[protocol.Team](t, h, protocol.MTeamsDreamAnswer, protocol.DreamAnswerParams{Team: dream.ID, Verdict: protocol.DreamAccepted, Note: "Lovely."})
	if d := accepted.Dream; d.Phase != protocol.DreamDone || d.Done == nil || d.Presentation.Verdict != protocol.DreamAccepted || d.Presentation.Note != "Lovely." {
		t.Fatalf("the accepted dream reads %+v", d)
	}
	task := h.task(t, dream.Dream.Task)
	if task.Status != protocol.TaskDone || task.Report == nil || task.Report.Title != "The setup page" {
		t.Fatalf("the standing task is %+v", task)
	}
	waitFor(t, "the manager's goal to end", func() bool { _, ok := h.sessions.goal(ref.Session, protocol.GoalStop); return ok })
	if h.ledgerCount(t, "dream.accept", "") != 1 || len(h.s.listApprovals(protocol.ApprovalsParams{Pending: true}).Approvals) != 0 {
		t.Fatal("the acceptance is not recorded, or a card still waits")
	}
	refused(t, h, protocol.MTeamsDreamAnswer, protocol.DreamAnswerParams{Team: dream.ID, Verdict: protocol.DreamAccepted}, protocol.CodeConflict)
}

// Notes send the dream back to work: the manager is continued with them,
// the phase is building again, and the Inbox card answers the same way.
func TestNotesContinueTheManagerWithTheNote(t *testing.T) {
	h := newHarness(t, 0)
	dream, ref := h.newDream(t)
	h.readyToPresent(t, dream, ref)
	h.answer(t, protocol.MTeamsAgentPresent, protocol.AgentPresentParams{AgentRef: ref, Title: "The setup page", Body: "Join ESP32-Setup."})
	answered := do[protocol.Team](t, h, protocol.MTeamsDreamAnswer, protocol.DreamAnswerParams{Team: dream.ID, Verdict: protocol.DreamNotes, Note: "Make the page dark."})
	if d := answered.Dream; d.Phase != protocol.DreamBuilding || d.Presentation.Verdict != protocol.DreamNotes || d.Done != nil {
		t.Fatalf("the dream after notes reads %+v", d)
	}
	waitFor(t, "the manager to hear the notes", func() bool {
		return slices.ContainsFunc(h.sessions.notes(ref.Session), func(note string) bool { return strings.Contains(note, "Make the page dark.") })
	})
	if task := h.task(t, dream.Dream.Task); task.Status != protocol.TaskWorking || h.ledgerCount(t, "dream.notes", "") != 1 {
		t.Fatalf("the standing task is %+v", task)
	}
	// a second round through the Inbox card itself
	h.buildRound(t, dream, ref)
	h.answer(t, protocol.MTeamsAgentPresent, protocol.AgentPresentParams{AgentRef: ref, Title: "Dark page", Body: "Now dark."})
	card := h.pending(t)
	do[protocol.Approval](t, h, protocol.MTeamsApprovalDecide, protocol.ApprovalDecideParams{ID: card.ID, Decision: protocol.DecideChoose, Choice: card.Options[1], Note: "Bigger buttons."})
	waitFor(t, "the card's notes to reach the manager", func() bool {
		return slices.ContainsFunc(h.sessions.notes(ref.Session), func(note string) bool { return strings.Contains(note, "Bigger buttons.") })
	})
	if d := h.dream(t, dream.ID); d.Rounds != 2 || d.Phase != protocol.DreamBuilding {
		t.Fatalf("after the card's notes the dream reads %+v", d)
	}
}

// A report does not end a dream, and neither does the manager completing
// its goal: the goal is set again and the standing task stays open.
func TestAReportDoesNotEndADream(t *testing.T) {
	h := newHarness(t, 0)
	dream, ref := h.newDream(t)
	refused(t, h, protocol.MTeamsAgentReport, protocol.AgentReportParams{AgentRef: ref, Title: "Done", Body: "I think it is done."}, protocol.CodeConflict)
	if task := h.task(t, dream.Dream.Task); task.Status != protocol.TaskWorking {
		t.Fatalf("a report ended the dream's task: %+v", task)
	}
	for index, ended := range []protocol.Goal{
		{Status: protocol.GoalDone, Reason: "Done."},
		// a spent turn budget goes on too: the dream is exempt from turns per task
		{Status: protocol.GoalStopped, Turns: 200, MaxTurns: 200},
		// two idle turns pause a goal; a dream's continues until the stall limit
		{Status: protocol.GoalPaused, Reason: noProgress()},
	} {
		h.endGoal(ref.Session, ended)
		h.s.GoalEnded(h.sessions.info(ref.Session), ended)
		if task := h.task(t, dream.Dream.Task); !taskIsOpen(task) {
			t.Fatalf("a goal %s ended the dream's task: %+v", ended.Status, task)
		}
		waitFor(t, "the goal to go on after "+ended.Status, func() bool {
			return len(h.goalsOf(ref.Session, protocol.GoalSet))+len(h.goalsOf(ref.Session, protocol.GoalResume)) == index+2
		})
	}
	if set, resumed := len(h.goalsOf(ref.Session, protocol.GoalSet)), len(h.goalsOf(ref.Session, protocol.GoalResume)); set != 2 || resumed != 2 {
		t.Fatalf("the goal was set %d times and resumed %d times", set, resumed)
	}
	if task := h.task(t, dream.Dream.Task); task.MaxTurns != protocol.TeamTaskTurnsMax {
		t.Fatalf("the standing task has %d turns", task.MaxTurns)
	}
}

// endGoal puts a session's goal in the state its session manager left it.
func (h *harness) endGoal(id string, ended protocol.Goal) {
	h.sessions.mu.Lock()
	defer h.sessions.mu.Unlock()
	info := h.sessions.infos[id]
	goal := *info.Goal
	goal.Status, goal.Reason, goal.Turns = ended.Status, ended.Reason, ended.Turns
	info.Goal = &goal
	h.sessions.infos[id] = info
}

// goalsOf lists the goal changes of op a session got.
func (h *harness) goalsOf(id, op string) []protocol.GoalParams {
	h.sessions.mu.Lock()
	defer h.sessions.mu.Unlock()
	var out []protocol.GoalParams
	for _, p := range h.sessions.goals {
		if p.ID == id && p.Op == op {
			out = append(out, p)
		}
	}
	return out
}

// Six idle turns block a dream without asking the CEO for permission to
// continue. Its standing task is reused if the CEO resumes it.
func TestTheStallGuardBlocksWithoutAskingForConfirmation(t *testing.T) {
	h := newHarness(t, 0)
	dream, ref := h.newDream(t)
	turn := func(steps int) {
		info := h.sessions.info(ref.Session)
		info.Steps, info.Preview = steps, "I am thinking about it."
		h.s.AfterTurn(context.Background(), info, protocol.TurnEndData{})
	}
	for range 5 {
		turn(0)
	}
	turn(2)
	for range 5 {
		turn(0)
	}
	if task := h.task(t, dream.Dream.Task); task.Status != protocol.TaskWorking {
		t.Fatalf("the manager stopped before six idle turns: %+v", task)
	}
	turn(0)
	task := h.task(t, dream.Dream.Task)
	if task.Status != protocol.TaskBlocked || !strings.Contains(task.Reason, "I am thinking about it.") || h.ledgerCount(t, "dream.stall", task.ID) != 1 {
		t.Fatalf("the stalled manager was not blocked with its last words: %+v", task)
	}
	if pending := h.s.listApprovals(protocol.ApprovalsParams{Pending: true}).Approvals; len(pending) != 0 {
		t.Fatalf("the guard asked for confirmation: %+v", pending)
	}
	waitFor(t, "the stalled session to stop", func() bool { return h.sessions.wasStopped(ref.Session) })
	resumed := do[protocol.Team](t, h, protocol.MTeamsPause, protocol.TeamPauseParams{ID: dream.ID, Paused: false})
	if resumed.Dream.Task != dream.Dream.Task || len(h.s.state.tasks(func(task protocol.TeamTask) bool { return task.Team == dream.ID })) != 1 {
		t.Fatal("resuming the dream did not reuse its standing task")
	}
	h.agent(t, resumed.Dream.Task)
}

// A presentation is refused until the CEO answered a question and a hired
// member finished a piece of this round; each refusal says what to do.
func TestPresentingNeedsAnInterviewAndTheTeamsWork(t *testing.T) {
	h := newHarness(t, 0)
	dream, ref := h.newDream(t)
	present := func() *protocol.Error {
		return refused(t, h, protocol.MTeamsAgentPresent, protocol.AgentPresentParams{AgentRef: ref, Title: "Page", Body: "Done."}, protocol.CodeConflict)
	}
	present()
	h.interview(t, ref)
	present()
	h.buildRound(t, dream, ref)
	h.answer(t, protocol.MTeamsAgentPresent, protocol.AgentPresentParams{AgentRef: ref, Title: "Page", Body: "Done."})
	do[protocol.Team](t, h, protocol.MTeamsDreamAnswer, protocol.DreamAnswerParams{Team: dream.ID, Verdict: protocol.DreamNotes, Note: "Bigger."})
	// a new round needs new work from the team, not the last round's
	present()
	h.buildRound(t, dream, ref)
	h.answer(t, protocol.MTeamsAgentPresent, protocol.AgentPresentParams{AgentRef: ref, Title: "Bigger page", Body: "Done."})
}

// A dream whose manager's model cannot run blocks its standing task before
// any session; once the model is fixed, resuming the dream runs that same
// task again rather than counting the blocked attempt as another.
func TestADreamBlockedOnItsModelGoesOnWhenResumed(t *testing.T) {
	h := newHarness(t, 0)
	dream := do[protocol.Team](t, h, protocol.MTeamsDreamCreate, protocol.DreamCreateParams{Statement: "A page.", Folder: filepath.Join(h.dir, "dreams", "page"), Model: "off/model"})
	waitFor(t, "the dream to block on its model", func() bool { return h.task(t, dream.Dream.Task).Status == protocol.TaskBlocked })
	if task := h.task(t, dream.Dream.Task); !strings.Contains(task.Reason, "off/model is turned off") {
		t.Fatalf("the blocked task is %+v", task)
	}
	members := append([]protocol.Member(nil), dream.Members...)
	members[0].Model = "test/small"
	do[protocol.Team](t, h, protocol.MTeamsUpdate, protocol.TeamUpdateParams{ID: dream.ID, Patch: mustJSON(t, map[string]any{"members": members})})
	do[protocol.Team](t, h, protocol.MTeamsPause, protocol.TeamPauseParams{ID: dream.ID})
	h.agent(t, dream.Dream.Task)
	team, _ := h.s.teamWithStats(dream.ID)
	if team.Dream.Task != dream.Dream.Task || team.Stats.Tasks != 1 || team.Stats.Done != 0 || team.Stats.Working != 1 || h.task(t, dream.Dream.Task).Reason != "" {
		t.Fatalf("after resuming the dream reads %+v, %+v", team.Dream, team.Stats)
	}
}

// A hire on a model that cannot run is refused, saying what is wrong, and
// joins nobody; a dream takes no more options than a member does.
func TestAHireOnAModelThatCannotRunIsRefused(t *testing.T) {
	h := newHarness(t, 0)
	dream, ref := h.newDream(t)
	_, err := h.call(protocol.MTeamsAgentHire, protocol.AgentHireParams{AgentRef: ref, Name: "Bo", Title: "Tester", Instructions: "You test.", Model: "nowhere/model"})
	var failure *protocol.Error
	if !errors.As(err, &failure) || failure.Code != protocol.CodeBadRequest || !strings.Contains(failure.Message, "no provider nowhere") {
		t.Fatalf("a hire on a model that cannot run: %v", err)
	}
	if team, _ := h.s.state.team(dream.ID); len(team.Members) != 1 {
		t.Fatalf("the refused hire joined: %+v", team.Members)
	}
	options := map[string]string{}
	for i := range protocol.EngineOptionsMax + 1 {
		options[strings.Repeat("o", i+1)] = "on"
	}
	refused(t, h, protocol.MTeamsDreamCreate, protocol.DreamCreateParams{Statement: "Too many options.", Folder: filepath.Join(h.dir, "dreams", "x"), Options: options}, protocol.CodeBadRequest)
}

// teams.list leaves dreams out, as do the teams agents read; the snapshot
// the app gets keeps them, for the Dreams page.
func TestTheTeamsListLeavesDreamsOut(t *testing.T) {
	h := newHarness(t, 0)
	team := h.newTeam(t, "Ordinary", nil)
	dream, _ := h.newDream(t)
	listed := do[protocol.TeamsResult](t, h, protocol.MTeamsList, protocol.Empty{})
	if len(listed.Teams) != 1 || listed.Teams[0].ID != team.ID {
		t.Fatalf("teams.list = %+v", listed.Teams)
	}
	var snapshot protocol.Snapshot
	if err := h.s.Snapshot(context.Background(), &snapshot); err != nil {
		t.Fatal(err)
	}
	if !slices.ContainsFunc(snapshot.Teams, func(t protocol.Team) bool { return t.ID == dream.ID && t.Dream != nil }) {
		t.Fatal("the snapshot lost the dream")
	}
}

func TestDreamManagerCannotHireAnUnpinnedPaidModel(t *testing.T) {
	h := newHarness(t, 0)
	h.s.SetModels(func(ctx context.Context, p protocol.ModelsParams) (protocol.ModelsResult, error) {
		out, err := listModels(ctx, p)
		out.Models = append(out.Models, protocol.ModelInfo{ID: "test/paid", Provider: "test", Tier: protocol.TierPaid, Tools: true, Available: true})
		return out, err
	})
	dream, ref := h.newDream(t)
	p := protocol.AgentHireParams{AgentRef: ref, Name: "Ada", Title: "Developer", Instructions: "Build the software.", Model: "test/paid"}
	refused(t, h, protocol.MTeamsAgentHire, p, protocol.CodeForbidden)
	if current, _ := h.s.state.team(dream.ID); len(current.Members) != 1 || current.Dream.Hired != 0 {
		t.Fatalf("rejected paid hire changed the team: %+v", current)
	}
	h.settingsMu.Lock()
	h.settings.Agent.Model = "test/paid"
	h.settingsMu.Unlock()
	hired := do[protocol.Member](t, h, protocol.MTeamsAgentHire, p)
	if hired.Model != "test/paid" {
		t.Fatalf("user-pinned paid model was refused: %+v", hired)
	}
}
