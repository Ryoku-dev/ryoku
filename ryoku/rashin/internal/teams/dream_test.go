package teams

import (
	"context"
	"errors"
	"path/filepath"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"ryoku-rashin/internal/protocol"
)

// newDream starts a dream in a folder of the test's own and waits for its
// manager's standing task to start, returning the dream and the manager's
// tool reference.
func (h *harness) newDream(t *testing.T) (protocol.Team, protocol.AgentRef) {
	t.Helper()
	dream := do[protocol.Team](t, h, protocol.MTeamsDreamCreate, protocol.DreamCreateParams{
		Statement: "Make an ESP32 page that appears when I join its Wi-Fi.", Name: "ESP32 setup",
		Folder: filepath.Join(h.dir, "dreams", "esp32"), Harness: "fake",
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
}

// buildRound has a hired member build a piece of this round and report it,
// hiring Ada first when the team has no one.
func (h *harness) buildRound(t *testing.T, dream protocol.Team, ref protocol.AgentRef) {
	t.Helper()
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
	if hired.Harness != "fake" || hired.Lead {
		t.Fatalf("hired %+v", hired)
	}
	progress := 30
	h.answer(t, protocol.MTeamsAgentDream, protocol.AgentDreamParams{AgentRef: ref, Phase: protocol.DreamBuilding, Summary: "Ada builds the firmware.", Progress: &progress, Brief: "A captive page."})
	if d := h.dream(t, dream.ID); d.Phase != protocol.DreamBuilding || *d.Progress != 30 || d.Brief != "A captive page." || d.Hired != 1 {
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
	if env := h.s.ToolEnv(h.sessions.info(ref.Session)); env["RASHIN_TEAM_DREAM"] != "1" {
		t.Fatalf("the manager's tools are not the dream's: %v", env)
	}
	if env := h.s.ToolEnv(h.sessions.info(member.Session)); env["RASHIN_TEAM_DREAM"] != "" {
		t.Fatalf("a member was given the dream's tools: %v", env)
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
		// two idle turns pause a goal; a dream's goes on until the stall guard asks
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

// Six turns in a row without a tool call stop the dream to ask the user,
// with the manager's last words; a turn with a tool call starts the count
// again, and keep going continues the manager.
func TestTheStallGuardAsksAfterSixIdleTurns(t *testing.T) {
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
	if pending := h.s.listApprovals(protocol.ApprovalsParams{Pending: true}).Approvals; len(pending) != 0 {
		t.Fatalf("the guard asked before six idle turns in a row: %+v", pending)
	}
	turn(0)
	card := h.pending(t)
	if !strings.Contains(card.Detail, "I am thinking about it.") || !slices.Equal(card.Options, []string{"Keep going", "Pause the dream"}) {
		t.Fatalf("the stall card is %+v", card)
	}
	if !h.s.Hold(h.sessions.info(ref.Session)) || h.ledgerCount(t, "dream.stall", "") != 1 {
		t.Fatal("the stalled dream goes on without the user")
	}
	do[protocol.Approval](t, h, protocol.MTeamsApprovalDecide, protocol.ApprovalDecideParams{ID: card.ID, Decision: protocol.DecideChoose, Choice: card.Options[0]})
	waitFor(t, "the manager to go on", func() bool {
		return slices.ContainsFunc(h.sessions.notes(ref.Session), func(note string) bool { return strings.Contains(note, "keep going") })
	})
	turn(0)
	if pending := h.s.listApprovals(protocol.ApprovalsParams{Pending: true}).Approvals; len(pending) != 0 {
		t.Fatal("the count did not start again after keep going")
	}
	// pausing from the card pauses the dream; resuming starts the manager again
	for range 5 {
		turn(0)
	}
	card = h.pending(t)
	do[protocol.Approval](t, h, protocol.MTeamsApprovalDecide, protocol.ApprovalDecideParams{ID: card.ID, Decision: protocol.DecideChoose, Choice: card.Options[1]})
	if team, _ := h.s.state.team(dream.ID); !team.Paused || taskIsOpen(h.task(t, dream.Dream.Task)) {
		t.Fatal("pause the dream left it working")
	}
	resumed := do[protocol.Team](t, h, protocol.MTeamsPause, protocol.TeamPauseParams{ID: dream.ID, Paused: false})
	// the stopped standing task runs again, rather than a second one being added
	if resumed.Dream.Task != dream.Dream.Task || len(h.s.state.tasks(func(task protocol.TeamTask) bool { return task.Team == dream.ID })) != 1 {
		t.Fatal("resuming the dream did not run its standing task again")
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
	if refusal := present(); !strings.Contains(refusal.Message, "ask_user") {
		t.Fatalf("presenting before any question says %q", refusal.Message)
	}
	h.interview(t, ref)
	if refusal := present(); !strings.Contains(refusal.Message, "delegate") {
		t.Fatalf("presenting without the team's work says %q", refusal.Message)
	}
	h.buildRound(t, dream, ref)
	h.answer(t, protocol.MTeamsAgentPresent, protocol.AgentPresentParams{AgentRef: ref, Title: "Page", Body: "Done."})
	do[protocol.Team](t, h, protocol.MTeamsDreamAnswer, protocol.DreamAnswerParams{Team: dream.ID, Verdict: protocol.DreamNotes, Note: "Bigger."})
	// a new round needs new work from the team, not the last round's
	if refusal := present(); !strings.Contains(refusal.Message, "this round") {
		t.Fatalf("presenting again without new work says %q", refusal.Message)
	}
	h.buildRound(t, dream, ref)
	h.answer(t, protocol.MTeamsAgentPresent, protocol.AgentPresentParams{AgentRef: ref, Title: "Bigger page", Body: "Done."})
}

// A dream started before its harness was signed in goes on by itself once
// the sign-in finishes: the same standing task runs again, and the blocked
// attempt is neither a finished task nor left as a card.
func TestADreamBlockedOnSignInGoesOnOnceSignedIn(t *testing.T) {
	h := newHarness(t, 0)
	var signed atomic.Bool
	h.s.signedIn = func(string, string) (time.Time, bool) { return time.Time{}, signed.Load() }
	dream := do[protocol.Team](t, h, protocol.MTeamsDreamCreate, protocol.DreamCreateParams{Statement: "A page.", Folder: filepath.Join(h.dir, "dreams", "page"), Harness: "fake"})
	waitFor(t, "the dream to block on its sign-in", func() bool { return h.task(t, dream.Dream.Task).Status == protocol.TaskBlocked })
	if task := h.task(t, dream.Dream.Task); task.SignIn != "fake" {
		t.Fatalf("the blocked task is %+v", task)
	}
	h.s.HomeSignedIn("other", dream.ID)
	if h.task(t, dream.Dream.Task).Status != protocol.TaskBlocked {
		t.Fatal("a sign-in to another harness started the dream")
	}
	signed.Store(true)
	h.s.HomeSignedIn("fake", dream.ID)
	h.agent(t, dream.Dream.Task)
	team, _ := h.s.teamWithStats(dream.ID)
	if team.Dream.Task != dream.Dream.Task || team.Stats.Tasks != 1 || team.Stats.Done != 0 || team.Stats.Working != 1 || h.task(t, dream.Dream.Task).Reason != "" {
		t.Fatalf("after the sign-in the dream reads %+v, %+v", team.Dream, team.Stats)
	}
}

// A hire on a harness the dream is not signed in to is refused, naming the
// harnesses it is signed in to.
func TestAHireOnAnEngineNotSignedInIsRefused(t *testing.T) {
	h := newHarnessWith(t, func(s *protocol.Settings) {
		s.Agent.Instances = append(s.Agent.Instances, protocol.HarnessInstance{ID: "other", Name: "Other", Command: []string{"other-acp"}})
	})
	h.s.signedIn = func(harness, _ string) (time.Time, bool) { return time.Time{}, harness == "fake" }
	dream, ref := h.newDream(t)
	_, err := h.call(protocol.MTeamsAgentHire, protocol.AgentHireParams{AgentRef: ref, Name: "Bo", Title: "Tester", Instructions: "You test.", Harness: "other"})
	var failure *protocol.Error
	if !errors.As(err, &failure) || failure.Code != protocol.CodeConflict || !strings.Contains(failure.Message, "Other") || !strings.Contains(failure.Message, "signed in to: Fake") {
		t.Fatalf("a hire on a harness not signed in: %v", err)
	}
	if team, _ := h.s.state.team(dream.ID); len(team.Members) != 1 {
		t.Fatalf("the refused hire joined: %+v", team.Members)
	}
	options := map[string]string{}
	for i := range protocol.EngineOptionsMax + 1 {
		options[strings.Repeat("o", i+1)] = "on"
	}
	refused(t, h, protocol.MTeamsDreamCreate, protocol.DreamCreateParams{Statement: "Too many options.", Folder: filepath.Join(h.dir, "dreams", "x"), Harness: "fake", Options: options}, protocol.CodeBadRequest)
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
	h.s.Snapshot(context.Background(), &snapshot)
	if !slices.ContainsFunc(snapshot.Teams, func(t protocol.Team) bool { return t.ID == dream.ID && t.Dream != nil }) {
		t.Fatal("the snapshot lost the dream")
	}
}
