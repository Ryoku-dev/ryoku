package teams

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"maps"
	"os"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	i18n "ryoku-i18n"

	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/protocol"
)

// run is a team task while an Agent session works it. Token is what the
// session's team tools prove the run with; a run ends with its task.
type run struct {
	Session, Task, Team, Member, Token string
	Lead                               bool
	Started                            time.Time
	// Model is the model the session last reported once it was made, so a
	// switch the user makes is told from it
	Model string
	// timer ends the task at its time limit
	timer *time.Timer
	// idle counts a dream manager's turns in a row without a tool call
	idle int
}

// maxPieces bounds the open pieces one task may hand out.
const (
	maxPieces = 4
	// pieceReport bounds each piece's report in the text the lead is sent on with
	pieceReport = 6000
)

// createTask gives a team a task from the user, with an engine for this
// task alone when one is asked for.
func (s *Service) createTask(ctx context.Context, p protocol.TeamTaskCreateParams) (protocol.TeamTask, error) {
	model := strings.TrimSpace(p.Model)
	if !protocol.ValidEngineModel(model) {
		return protocol.TeamTask{}, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("A model is a route such as auto, or a provider's model such as provider/model, without spaces."))
	}
	if !protocol.ValidEngineOptions(p.Options) {
		return protocol.TeamTask{}, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.Tf("A model takes at most %d options, each with an id and a value.", protocol.EngineOptionsMax))
	}
	options := p.Options
	if len(options) == 0 {
		options = nil
	}
	return s.newTask(ctx, p.Team, p.Member, p.Title, p.Brief, protocol.TaskFromYou, "", "", model, options)
}

// newTask commits a queued task against the current roster before publishing
// it. Config edits and competing delegations use the same service lock.
func (s *Service) newTask(ctx context.Context, team, member, title, brief, origin, sourceID, parent, model string, options map[string]string) (protocol.TeamTask, error) {
	if err := ctx.Err(); err != nil {
		return protocol.TeamTask{}, err
	}
	s.mu.Lock()
	task, assignee, err := s.newTaskLocked(team, member, title, brief, origin, sourceID, parent, model, options, origin == protocol.TaskFromLead)
	s.mu.Unlock()
	if err != nil {
		return protocol.TeamTask{}, err
	}
	actor, initiator := s.taskActor(task)
	s.record(protocol.Activity{
		Actor: actor, Action: "task.create", Target: task.Title, OK: true, Team: task.Team, Task: task.ID, Initiator: initiator,
		Detail: s.redact(i18n.Tf("%s takes it on.", assignee)),
	})
	s.emitTask(task)
	s.pump()
	if current, ok := s.state.task(task.ID); ok {
		task = current
	}
	return task, nil
}

func (s *Service) newTaskLocked(team, member, title, brief, origin, sourceID, parent, model string, options map[string]string, automatic bool) (protocol.TeamTask, string, error) {
	if err := s.state.writer.available(); err != nil {
		return protocol.TeamTask{}, "", err
	}
	t, ok := s.state.team(team)
	if !ok {
		return protocol.TeamTask{}, "", protocol.Errorf(protocol.CodeNotFound, "%s", i18n.Tf("No team %s.", team))
	}
	if !s.dreamAvailable(t) {
		return protocol.TeamTask{}, "", protocol.Errorf(protocol.CodeForbidden, "%s", i18n.T("Deep research is off in Settings."))
	}
	brief = strings.TrimSpace(brief)
	if brief == "" {
		return protocol.TeamTask{}, "", protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("A task needs a brief: what the team should do."))
	}
	assignee := teamLead(t)
	if member != "" {
		found, known := taskMember(t, member)
		if !known {
			return protocol.TeamTask{}, "", protocol.Errorf(protocol.CodeNotFound, "%s", i18n.Tf("%s has no member %s.", t.Name, member))
		}
		assignee = found
	}
	if assignee.ID == "" {
		return protocol.TeamTask{}, "", protocol.Errorf(protocol.CodeConflict, "%s", i18n.T("Choose a spirit for this task, or select a synthesizer for the peer team."))
	}
	if s.editing[t.ID][assignee.ID] {
		return protocol.TeamTask{}, "", protocol.Errorf(protocol.CodeConflict, "This seat is changing execution policy or its team is being removed.")
	}
	if !taskWakeAllowed(assignee, origin, automatic) {
		return protocol.TeamTask{}, "", protocol.Errorf(protocol.CodeConflict, "This spirit's automatic wake is disabled. The Summoner can still assign it work directly.")
	}
	if automatic && (s.settings().Teams.Paused || !s.memberBudgetReady(t, assignee)) {
		return protocol.TeamTask{}, "", protocol.Errorf(protocol.CodeConflict, "This spirit is paused or its effective spending limit does not permit automatic work.")
	}
	goal := t.Purpose
	if goal == "" {
		goal = brief
	}
	var plan *protocol.TeamPlan
	if parent != "" {
		if s.taskEdits[parent] {
			return protocol.TeamTask{}, "", protocol.Errorf(protocol.CodeConflict, "The parent task is being reassigned.")
		}
		source, known := s.state.task(parent)
		if !known || source.Team != t.ID || origin == protocol.TaskFromLead && !taskIsOpen(source) {
			return protocol.TeamTask{}, "", protocol.Errorf(protocol.CodeConflict, "%s", i18n.T("The parent task is missing, belongs to another team, or cannot accept delegated work."))
		}
		if origin == protocol.TaskFromLead && !protocol.CanDelegate(t, source.Member, assignee.ID) {
			return protocol.TeamTask{}, "", protocol.Errorf(protocol.CodeForbidden, "%s", i18n.T("Reporting lines changed; this spirit is no longer one of your permitted reports."))
		}
		if origin == protocol.TaskFromLead && taskNeedsPlan(t, source) {
			return protocol.TeamTask{}, "", protocol.Errorf(protocol.CodeForbidden, "The Summoner must approve this task's strategy before work is delegated.")
		}
		if len(s.state.tasks(func(piece protocol.TeamTask) bool { return piece.Parent == parent && taskIsOpen(piece) })) >= maxPieces {
			return protocol.TeamTask{}, "", protocol.Errorf(protocol.CodeConflict, "%s", i18n.Tf("A task has at most %d open pieces.", maxPieces))
		}
		if source.Goal != "" {
			goal = source.Goal
		}
		if source.Plan != nil && source.Plan.Approved {
			inherited := *source.Plan
			inherited.Steps = slices.Clone(source.Plan.Steps)
			plan = &inherited
		}
	}
	now := s.now()
	task := protocol.TeamTask{
		ID: newID("task"), Team: t.ID, Title: taskTitle(title, brief), Brief: brief, Status: protocol.TaskReady,
		Member: assignee.ID, Parent: parent, Goal: goal, Plan: plan, Origin: origin, Runtime: memberRuntimeKind(assignee), Automatic: automatic, MaxTurns: effectiveTaskTurns(t, assignee),
		Model: model, Options: options, Created: now, Updated: now,
	}
	if origin == protocol.TaskFromRoutine {
		task.Routine = sourceID
	} else {
		task.Trigger = sourceID
	}
	if err := s.state.putTask(task); err != nil {
		return protocol.TeamTask{}, "", err
	}
	return task, assignee.Name, nil
}

// listTasks pages tasks newest first; Before is the creation time of the
// oldest task already shown.
func (s *Service) listTasks(p protocol.TeamTasksParams) protocol.TeamTasksResult {
	var before time.Time
	if p.Before != "" {
		before, _ = time.Parse(time.RFC3339Nano, p.Before)
	}
	limit := p.Limit
	if limit <= 0 {
		limit = 50
	}
	limit = min(limit, 200)
	tasks := s.state.tasks(func(t protocol.TeamTask) bool {
		return (p.Team == "" || t.Team == p.Team) && (!p.Open || taskIsOpen(t)) && (before.IsZero() || t.Created.Before(before))
	})
	if len(tasks) > limit {
		tasks = tasks[:limit]
	}
	if tasks == nil {
		tasks = []protocol.TeamTask{}
	}
	return protocol.TeamTasksResult{Tasks: tasks}
}

// readTask marks a task's report as read.
func (s *Service) readTask(id string) (protocol.TeamTask, error) {
	s.mu.Lock()
	t, ok := s.state.task(id)
	if !ok {
		s.mu.Unlock()
		return protocol.TeamTask{}, protocol.Errorf(protocol.CodeNotFound, "%s", i18n.Tf("No task %s.", id))
	}
	if t.Read {
		s.mu.Unlock()
		return t, nil
	}
	t.Read = true
	err := s.state.putTask(t)
	s.mu.Unlock()
	if err != nil {
		return protocol.TeamTask{}, err
	}
	s.emitTask(t)
	return t, nil
}

// retryTask runs a finished, blocked or stopped task again in a new session.
func (s *Service) retryTask(ctx context.Context, p protocol.TeamTaskRetryParams) (protocol.TeamTask, error) {
	note := strings.TrimSpace(p.Note)
	if note != "" {
		note = i18n.Tf("When you ran this before: %s", note)
	}
	return s.changeTaskExecution(ctx, protocol.TeamTaskReassignParams{ID: p.ID}, note, taskChangeRetry)
}

// runAgain readies an ended task to run again in a new session, on its
// member's engine as it is now.
func runAgain(t *protocol.TeamTask, now time.Time) {
	t.Status, t.Waiting, t.Report, t.Reason = protocol.TaskReady, "", nil, ""
	t.Session, t.Started, t.Ended, t.Read, t.Steps = "", nil, nil, false, 0
	t.Model, t.Options, t.Runtime, t.Automatic = "", nil, "", false
	t.Turns = 0
	t.PermissionWaits = nil
	t.Updated = now
}

// deleteTask stops a task, then removes it and the pieces it handed out.
// Their sessions stay, so the conversations can still be read.
func (s *Service) deleteTaskTree(ctx context.Context, id string) error {
	task, ok := s.state.task(id)
	if !ok {
		return protocol.Errorf(protocol.CodeNotFound, "%s", i18n.Tf("No task %s.", id))
	}
	if _, err := s.stopTask(ctx, id, i18n.T("You deleted it.")); err != nil {
		return err
	}
	for _, piece := range s.state.tasks(func(p protocol.TeamTask) bool { return p.Parent == id }) {
		if err := s.deleteTaskTree(ctx, piece.ID); err != nil {
			return err
		}
	}
	if err := s.stopTaskSessions(ctx, task); err != nil {
		return err
	}
	if err := paths.RemoveWritableTree(paths.ToolHome(task.Team, task.ID)); err != nil {
		return err
	}
	s.mu.Lock()
	err := s.state.removeTask(id)
	s.mu.Unlock()
	if err != nil {
		return err
	}
	s.broadcast(protocol.EvtTeamTaskRemoved, protocol.TeamRemoved{ID: id})
	return nil
}

func (s *Service) stopTaskSessions(ctx context.Context, task protocol.TeamTask) error {
	if s.sessions == nil {
		return nil
	}
	stop := func(session string) error {
		if _, ok := s.sessions.Info(session); ok {
			return s.sessions.Stop(ctx, session)
		}
		return nil
	}
	for _, session := range task.Sessions {
		if err := stop(session); err != nil {
			return err
		}
	}
	if task.Session != "" && !slices.Contains(task.Sessions, task.Session) {
		return stop(task.Session)
	}
	return nil
}

// stopTeamTasks stops every open task of a team, queued ones included.
func (s *Service) stopTeamTasks(team, reason string) {
	for _, t := range s.state.tasks(func(t protocol.TeamTask) bool { return t.Team == team && taskIsOpen(t) }) {
		_, _ = s.stopTask(s.background, t.ID, reason)
	}
}

// pump starts queued tasks, oldest first, while fewer than the setting's
// tasks work across teams and fewer than each team's own limit in it. A
// task held on the user or its members does not count; a paused team, or
// every team while the teams settings pause them all, starts nothing.
func (s *Service) pump() {
	if s.state.writer.available() != nil {
		return
	}
	if s.sessions == nil || s.background.Err() != nil {
		return
	}
	settings := s.settings().Teams
	if settings.Paused {
		return
	}
	limit := settings.MaxRunning
	if limit <= 0 {
		limit = protocol.DefaultTeamsMaxRunning
	}
	s.mu.Lock()
	open := s.state.tasks(taskIsOpen)
	running := 0
	perTeam := map[string]int{}
	for _, t := range open {
		if t.Status == protocol.TaskWorking && t.Waiting == "" {
			running++
			perTeam[t.Team]++
		}
	}
	type claimedTask struct {
		task  protocol.TeamTask
		token string
	}
	var starts []claimedTask
	var parked []protocol.TeamTask
	for i := len(open) - 1; i >= 0 && running < limit; i-- {
		t := open[i]
		if t.Status != protocol.TaskReady || s.taskEdits[t.ID] {
			continue
		}
		team, ok := s.state.team(t.Team)
		if !ok || !s.dreamAvailable(team) || team.Paused || perTeam[t.Team] >= teamLimit(team, settings) {
			continue
		}
		member, known := protocol.MemberByID(team.Members, t.Member)
		if !known || s.editing[team.ID][member.ID] || !s.memberBudgetReady(team, member) || !taskWakeAllowed(member, t.Origin, t.Automatic) {
			continue
		}
		if taskNeedsPlan(team, t) && memberRuntimeKind(member) != protocol.MemberRuntimeNative {
			t.Status, t.Waiting = protocol.TaskNeedsYou, protocol.WaitingOnYou
			t.Reason, t.Updated = "Supply or approve a strategy before this external runtime starts. Strict read-only foreign processes are offline.", s.now()
			// No session exists yet, but this is still a genuine owner
			// decision hold for any already-running assigning ancestors.
			t.PermissionWaits = append(t.PermissionWaits, protocol.TaskPermissionWait{From: t.Updated})
			if s.state.putTask(t) == nil {
				parked = append(parked, t)
			}
			continue
		}
		now := s.now()
		t.Status, t.Waiting, t.Updated = protocol.TaskWorking, "", now
		if t.Started == nil {
			t.Started = &now
		}
		if s.state.putTask(t) != nil {
			continue
		}
		r := &run{Task: t.ID, Team: t.Team, Member: t.Member, Token: runToken(), Lead: member.Lead, Started: now}
		s.runs[t.ID] = r
		s.armMinutesLocked(t.ID)
		running++
		perTeam[t.Team]++
		starts = append(starts, claimedTask{task: t, token: r.Token})
	}
	s.mu.Unlock()
	for _, task := range parked {
		s.emitTask(task)
		s.notify(notification{Team: task.Team, Task: task.ID, Title: "A strategy is needed", Body: task.Title, Needs: true})
	}
	for _, claim := range starts {
		s.emitTask(claim.task)
		s.startWorker(func() { s.start(claim.task, claim.token) })
	}
}

// teamLimit is how many tasks a team works at once: its own Running, else
// the teams settings' PerTeam.
func teamLimit(team protocol.Team, settings protocol.TeamsSettings) int {
	if team.Running > 0 {
		return team.Running
	}
	if settings.PerTeam > 0 {
		return settings.PerTeam
	}
	return protocol.DefaultTeamsPerTeam
}

// start brings a claimed task up as an Agent session: on the engine resolved
// for it, which must resolve in the gateway, in the team's folder under the
// team's approvals, then the task as the session's goal, which starts its
// first turn. The task keeps the engine it started with; one whose model
// cannot run ends blocked with what to change, and a retry runs it again.
func (s *Service) start(task protocol.TeamTask, token string) {
	if s.state.writer.available() != nil {
		return
	}
	ctx := s.background
	s.mu.Lock()
	claim := s.runs[task.ID]
	currentClaim := claim != nil && claim.Token == token
	s.mu.Unlock()
	if !currentClaim {
		return
	}
	var model string
	var options map[string]string
	fail := func(session string, reason string) {
		ended, ok := s.endTaskIf(task.ID, func(t protocol.TeamTask) bool {
			r := s.runs[t.ID]
			return r != nil && r.Token == token
		}, func(t *protocol.TeamTask) {
			t.Status, t.Reason = protocol.TaskBlocked, reason
			if model != "" {
				t.Model, t.Options = model, maps.Clone(options)
			}
		})
		if session != "" {
			s.halt(session)
		}
		if !ok {
			return
		}
		s.record(protocol.Activity{Actor: "Hyakki", Action: "task.block", Target: ended.Title, OK: false, Team: ended.Team, Task: ended.ID, Initiator: protocol.InitiatorRashin, Detail: s.redact(reason)})
		s.notifyEnd(ended)
		s.afterEnd(ended)
	}
	team, ok := s.state.team(task.Team)
	if !ok {
		fail("", i18n.T("Its team no longer exists."))
		return
	}
	if !s.dreamAvailable(team) {
		fail("", i18n.T("Deep research is off in Settings."))
		return
	}
	member, ok := protocol.MemberByID(team.Members, task.Member)
	if !ok {
		fail("", i18n.T("The member it was given to is no longer on the team."))
		return
	}
	if taskTurnsExhausted(team, member, task) {
		fail("", "This task used its effective turn allowance. The Summoner must review and retry it explicitly.")
		return
	}
	model, options = s.taskEngine(member, task)
	if member.Runtime == nil || member.Runtime.Kind == "" || member.Runtime.Kind == protocol.MemberRuntimeNative {
		if problem := s.modelProblem(ctx, model); problem != "" {
			fail("", i18n.Tf("%s Pick another model for %s or set this one up in Settings, then retry the task.", problem, member.Name))
			return
		}
	}
	if err := os.MkdirAll(team.Folder, 0o700); err != nil {
		fail("", i18n.Tf("The team's folder could not be made: %v", err))
		return
	}
	if s.state.writer.available() != nil {
		return
	}
	info, err := s.sessions.New(ctx, protocol.SessionNewParams{
		Kind: protocol.KindAgent, Model: model, Options: maps.Clone(options), Cwd: team.Folder, Title: task.Title, Origin: protocol.OriginTeam,
		Team: team.ID, TeamTask: task.ID, Member: member.ID, Runtime: memberRuntimeKind(member), Approvals: taskApprovals(team, member, task),
	})
	if err != nil {
		fail("", i18n.Tf("The conversation could not start: %v", err))
		return
	}
	s.mu.Lock()
	r := s.runs[task.ID]
	current, known := s.state.task(task.ID)
	latestTeam, teamExists := s.state.team(task.Team)
	latestMember, memberExists := protocol.MemberByID(latestTeam.Members, task.Member)
	valid := r != nil && r.Token == token && known && taskIsOpen(current) && current.Member == member.ID &&
		teamExists && memberExists && !s.editing[task.Team][task.Member] && !s.taskEdits[task.ID] &&
		memberConfigEqual(member, latestMember) && !latestTeam.Paused && !s.settings().Teams.Paused &&
		taskWakeAllowed(latestMember, current.Origin, current.Automatic) && s.memberBudgetReady(latestTeam, latestMember)
	if !valid {
		stillClaimed := r != nil && r.Token == token
		s.mu.Unlock()
		s.halt(info.ID)
		if stillClaimed {
			fail("", "Its execution configuration changed while starting. Retry with the current settings.")
		}
		return
	}
	r.Session, r.Model = info.ID, model
	current.Session = info.ID
	current.Sessions = append(current.Sessions, info.ID)
	current.Model, current.Options = model, maps.Clone(options)
	current.Runtime = memberRuntimeKind(member)
	current.Updated = s.now()
	err = s.state.putTask(current)
	s.mu.Unlock()
	if err != nil {
		fail(info.ID, err.Error())
		return
	}
	s.emitTask(current)
	if !s.live(task.ID, info.ID) {
		s.halt(info.ID)
		return
	}
	if _, err := s.sessions.Goal(ctx, protocol.GoalParams{ID: info.ID, Op: protocol.GoalSet, Objective: objective(current), MaxTurns: taskGoalTurns(latestTeam, latestMember, current)}); err != nil {
		fail(info.ID, i18n.Tf("The task could not be set as the goal: %v", err))
		return
	}
	_, initiator := s.taskActor(task)
	s.record(protocol.Activity{
		Actor: member.Name, Action: "task.start", Target: task.Title, OK: true, Team: team.ID, Task: task.ID, Initiator: initiator,
		Detail: i18n.Tf("%s started work on it with %s.", member.Name, s.engineWords(model)),
	})
}

// objective is what a task's session works toward: the brief, under the
// title unless the brief already opens with it, as one typed in the app does
// (its title is the first line, cut short with an ellipsis when long).
func objective(task protocol.TeamTask) string {
	brief := strings.TrimSpace(task.Brief)
	title := strings.TrimSuffix(strings.TrimSpace(task.Title), "…")
	first, _, _ := strings.Cut(brief, "\n")
	switch {
	case brief == "":
		return task.Title
	case title != "" && strings.HasPrefix(strings.TrimSpace(first), title):
		return brief
	}
	return task.Title + "\n\n" + brief
}

// live reports whether a task's run is still on that session.
func (s *Service) live(task, session string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	r := s.runs[task]
	if r == nil || r.Session != session || s.taskEdits[task] || s.editing[r.Team][r.Member] {
		return false
	}
	_, _, _, err := s.currentCallerLocked(caller{Team: r.Team, Task: r.Task, Member: r.Member, Session: session, Token: r.Token})
	return err == nil
}

// endTask finishes an open task once: change sets how it ended, the run is
// dropped so its token answers no more, the steps its session took are kept,
// the day's spend counts it, and the session is let go once its turn is
// over. It reports false when the task had already ended.
func (s *Service) endTask(id string, change func(*protocol.TeamTask)) (protocol.TeamTask, bool) {
	return s.endTaskIf(id, nil, change)
}

// endTaskIf evaluates allow under Service.mu, atomically with the open-task
// transition. The predicate must not acquire Service.mu itself. Session
// inspection and completed-task effects retain the ordinary endTask lifecycle.
func (s *Service) endTaskIf(id string, allow func(protocol.TeamTask) bool, change func(*protocol.TeamTask)) (protocol.TeamTask, bool) {
	// the session is asked before the lock, so its manager never waits on it
	var steps int
	inspectedSession := ""
	if t, ok := s.state.task(id); ok && t.Session != "" && s.sessions != nil {
		inspectedSession = t.Session
		info, _ := s.sessions.Info(t.Session)
		steps = info.Steps
	}
	s.mu.Lock()
	t, ok := s.state.task(id)
	if !ok || !taskIsOpen(t) || (allow != nil && !allow(t)) {
		s.mu.Unlock()
		return t, false
	}
	change(&t)
	now := s.now()
	closePermissionWait(&t, now)
	t.Waiting, t.Ended, t.Updated = "", &now, now
	if t.Session == inspectedSession {
		t.Steps = steps
	}
	err := s.state.putTask(t)
	if err == nil {
		err = s.retireTaskRequestsLocked(id)
	}
	if err != nil {
		s.taskEdits[id], s.taskEditErrors[id] = true, err
		if run := s.runs[id]; run != nil && run.timer != nil {
			run.timer.Stop()
		}
		delete(s.runs, id)
		s.mu.Unlock()
		s.record(protocol.Activity{Actor: "Hyakki", Action: "task.stop", Target: t.Title, OK: false, Team: t.Team, Task: t.ID, Initiator: protocol.InitiatorRashin, Detail: "The task's final state could not be saved. Its authority was revoked; it must not resume or be replaced."})
		if t.Session != "" {
			s.halt(t.Session)
		}
		return t, false
	}
	// Storage freezes delegated owner-wait history at terminal publication.
	// Return and emit that same durable execution snapshot.
	t, _ = s.state.task(id)
	if r := s.runs[id]; r != nil && r.timer != nil {
		r.timer.Stop()
	}
	delete(s.runs, id)
	s.mu.Unlock()
	s.countEnded(t)
	s.emitTask(t)
	if t.Session != "" {
		s.retire(t.Session)
	}
	return t, true
}

// afterEnd is what follows any end: what the task still asked is answered
// no (stopTask does that with its own note), the routine hears how its run
// went, the lead is sent on once its last piece is in, and the freed slot
// starts the next queued task.
func (s *Service) afterEnd(t protocol.TeamTask) {
	if t.Status != protocol.TaskStopped {
		s.denyPending(t.ID, i18n.T("The task ended."))
	}
	if t.Routine != "" {
		s.routineTaskEnded(t)
	}
	if t.Parent != "" {
		s.releaseParent(t.Parent)
	}
	s.pump()
	s.clock.rearm()
}

// releaseParent sends a lead on with its pieces' reports once none is open.
func (s *Service) releaseParent(parent string) {
	s.mu.Lock()
	lead, ok := s.state.task(parent)
	if !ok || !taskIsOpen(lead) || lead.Waiting != protocol.WaitingOnMembers || s.pendingTaskRequestsLocked(lead) {
		s.mu.Unlock()
		return
	}
	pieces := s.state.tasks(func(piece protocol.TeamTask) bool { return piece.Parent == parent })
	for _, piece := range pieces {
		if taskIsOpen(piece) {
			s.mu.Unlock()
			return
		}
	}
	team, _ := s.state.team(lead.Team)
	s.mu.Unlock()
	s.release(parent, lead.Session, protocol.WaitingOnMembers, lead.ResumeVersion, piecesText(team, pieces))
}

// release ends a hold: the task works again and its agent is continued under
// its goal with text, what the user decided or what the members reported.
// A task that already ended, or never started a session, has nothing to
// continue.
func (s *Service) release(taskID, expectedSession, expectedWaiting string, expectedVersion uint64, text string) {
	if s.sessions == nil || s.settings().Teams.Paused {
		return
	}
	s.mu.Lock()
	task, exists := s.state.task(taskID)
	run := s.runs[taskID]
	if !exists || !taskIsOpen(task) || task.Waiting != expectedWaiting || task.ResumeVersion != expectedVersion || run == nil || run.Session != expectedSession || task.Session != expectedSession || s.taskEdits[taskID] || s.editing[task.Team][task.Member] {
		s.mu.Unlock()
		return
	}
	caller := caller{Team: run.Team, Task: run.Task, Member: run.Member, Session: run.Session, Token: run.Token}
	s.mu.Unlock()
	s.startWorker(func() {
		err := s.sessions.ContinueAfter(s.background, expectedSession, func() (string, bool, error) {
			s.mu.Lock()
			team, member, current, err := s.currentCallerLocked(caller)
			if err != nil || current.Waiting != expectedWaiting || current.ResumeVersion != expectedVersion || !s.dreamAvailable(team) || !taskWakeAllowed(member, current.Origin, current.Automatic) || s.pendingApprovals(taskID) > 0 || s.pendingTaskRequestsLocked(current) {
				s.mu.Unlock()
				return "", false, nil
			}
			if current.Waiting == protocol.WaitingOnMembers {
				pieces := s.state.tasks(func(piece protocol.TeamTask) bool { return piece.Parent == taskID })
				for _, piece := range pieces {
					if taskIsOpen(piece) {
						s.mu.Unlock()
						return "", false, nil
					}
				}
				text = piecesText(team, pieces)
			}
			if current.ResumeVersion == ^uint64(0) {
				s.mu.Unlock()
				return "", false, protocol.Errorf(protocol.CodeConflict, "This task's handoff generation is exhausted.")
			}
			current.ResumeVersion++
			current.Status, current.Waiting, current.Updated = protocol.TaskWorking, "", s.now()
			if err := s.state.putTask(current); err != nil {
				s.mu.Unlock()
				return "", false, err
			}
			s.armMinutesLocked(taskID)
			s.mu.Unlock()
			s.emitTask(current)
			return text, true, nil
		})
		if err != nil && s.background.Err() == nil {
			ended, stopped := s.endTaskIf(taskID, func(current protocol.TeamTask) bool {
				run := s.runs[taskID]
				return current.Session == expectedSession && run != nil && run.Token == caller.Token && !s.taskEdits[taskID]
			}, func(current *protocol.TeamTask) {
				current.Status, current.Reason = protocol.TaskBlocked, i18n.Tf("Hyakki could not continue the task: %v", err)
			})
			if stopped {
				s.record(protocol.Activity{Actor: "Hyakki", Action: "task.block", Target: ended.Title, OK: false, Team: ended.Team, Task: ended.ID, Initiator: protocol.InitiatorRashin, Detail: s.redact(ended.Reason)})
				s.notifyEnd(ended)
				s.afterEnd(ended)
			}
		}
	})
}

// pendingApprovals counts the approvals a task still waits on.
func (s *Service) pendingApprovals(task string) int {
	return len(s.state.approvals(func(rec approvalRecord) bool {
		return rec.Task == task && rec.Status == protocol.ApprovalPending
	}))
}

// denyPending answers no to what an ended task still waited on, without
// carrying any of it out. Each answer is claimed like the user's own, so one
// the user gave meanwhile stands and is never overwritten.
func (s *Service) denyPending(task, note string) {
	pending := s.state.approvals(func(rec approvalRecord) bool {
		return rec.Task == task && rec.Status == protocol.ApprovalPending
	})
	for _, waiting := range pending {
		rec, err := s.claimApproval(waiting.ID, protocol.ApprovalDenied, protocol.DecideDeny, note)
		if err != nil {
			continue
		}
		s.record(protocol.Activity{Actor: "Hyakki", Action: "approval.deny", Target: rec.Title, OK: true, Team: rec.Team, Task: task, Initiator: protocol.InitiatorRashin, Detail: note})
		s.emitApproval(rec.Approval)
	}
}

// halt stops a session at once: its goal ends and its agent stops. It runs
// on its own goroutine, since stopping waits for the running turn and the
// caller may be that turn.
func (s *Service) halt(session string) {
	if session == "" || s.sessions == nil {
		return
	}
	s.startWorker(func() {
		if info, ok := s.sessions.Info(session); ok && info.Goal != nil && (info.Goal.Status == protocol.GoalActive || info.Goal.Status == protocol.GoalPaused) {
			_, _ = s.sessions.Goal(s.background, protocol.GoalParams{ID: session, Op: protocol.GoalStop})
		}
		_ = s.sessions.Stop(s.background, session)
	})
}

// retire lets an ended task's session go: at once when no turn runs, else
// AfterTurn stops it once the turn that ended the task is over, so the
// agent's last words reach the transcript.
func (s *Service) retire(session string) {
	if s.sessions == nil {
		return
	}
	info, ok := s.sessions.Info(session)
	if !ok || info.State == protocol.StateWorking || info.State == protocol.StateWaiting {
		return
	}
	s.startWorker(func() { _ = s.sessions.Stop(s.background, session) })
}

// recoverRuns picks up the tasks a daemon restart left: one working with an
// active goal goes on, a held one stays held, and one whose session is gone
// or finished is blocked. A time limit counts on from when the task started.
func (s *Service) recoverRuns() {
	s.armExpiry()
	if s.sessions == nil {
		return
	}
	var carryOn []string
	var lost []string
	var unheld []string
	found := map[string]*run{}
	for _, t := range s.state.tasks(func(t protocol.TeamTask) bool {
		return t.Status == protocol.TaskWorking || t.Status == protocol.TaskNeedsYou
	}) {
		team, teamExists := s.state.team(t.Team)
		member, memberExists := protocol.MemberByID(team.Members, t.Member)
		if teamExists && memberExists && t.Session == "" && taskNeedsPlan(team, t) && memberRuntimeKind(member) != protocol.MemberRuntimeNative {
			continue
		}
		if !teamExists || !memberExists || !s.dreamAvailable(team) || s.settings().Teams.Paused ||
			!s.memberBudgetReady(team, member) || !taskWakeAllowed(member, t.Origin, t.Automatic) || taskTurnsExhausted(team, member, t) {
			_, _ = s.stopTask(s.background, t.ID, "The current seat policy does not permit this task to resume.")
			s.halt(t.Session)
			continue
		}
		info, ok := protocol.SessionInfo{}, false
		if t.Session != "" {
			info, ok = s.sessions.Info(t.Session)
		}
		// a permission card the restart took away no longer holds the task
		held := t.Waiting == protocol.WaitingOnMembers || s.pendingApprovals(t.ID) > 0
		active := info.Goal != nil && (info.Goal.Status == protocol.GoalActive || held && info.Goal.Status == protocol.GoalPaused)
		if !ok || !active {
			lost = append(lost, t.ID)
			continue
		}
		started := s.now()
		if t.Started != nil {
			started = *t.Started
		}
		found[t.ID] = &run{Session: t.Session, Task: t.ID, Team: t.Team, Member: t.Member, Token: runToken(), Lead: member.Lead, Started: started, Model: info.Model}
		if !held {
			carryOn = append(carryOn, t.Session)
			if t.Status != protocol.TaskWorking || t.Waiting != "" {
				unheld = append(unheld, t.ID)
			}
		}
	}
	s.mu.Lock()
	for id, r := range found {
		s.runs[id] = r
	}
	var changed []protocol.TeamTask
	var failedSessions []string
	for id := range found {
		if t, ok := s.state.task(id); ok {
			needsSave := slices.Contains(unheld, id)
			if needsSave {
				t.Status, t.Waiting, t.Updated = protocol.TaskWorking, "", s.now()
			}
			for _, span := range t.PermissionWaits {
				needsSave = needsSave || span.Until.IsZero()
			}
			if !needsSave {
				continue
			}
			// Native permission cards do not survive restart, even if a
			// separate governance decision still legitimately holds this task.
			closePermissionWait(&t, s.now())
			if err := s.state.putTask(t); err != nil {
				s.taskEdits[id], s.taskEditErrors[id] = true, err
				delete(s.runs, id)
				delete(found, id)
				failedSessions = append(failedSessions, t.Session)
			} else {
				changed = append(changed, t)
			}
		}
	}
	for id := range found {
		s.armMinutesLocked(id)
	}
	s.mu.Unlock()
	for _, session := range failedSessions {
		s.halt(session)
	}
	for _, t := range changed {
		s.emitTask(t)
	}
	for _, id := range lost {
		ended, ok := s.endTask(id, func(t *protocol.TeamTask) {
			t.Status, t.Reason = protocol.TaskBlocked, i18n.T("Hyakki restarted before the task ended.")
		})
		if ok {
			s.record(protocol.Activity{Actor: "Hyakki", Action: "task.block", Target: ended.Title, OK: false, Team: ended.Team, Task: ended.ID, Initiator: protocol.InitiatorRashin, Detail: ended.Reason})
			s.afterEnd(ended)
		}
	}
	for id := range found {
		if task, exists := s.state.task(id); exists && task.Waiting == protocol.WaitingOnMembers {
			s.releaseParent(id)
		}
	}
	for _, session := range carryOn {
		s.startWorker(func() {
			if info, ok := s.sessions.Info(session); !ok || !s.live(info.TeamTask, session) {
				return
			}
			_ = s.sessions.Continue(s.background, session, i18n.T("Hyakki restarted while you worked. Carry on with the task."))
		})
	}
}

// taskActor names who started a task for the ledger: the user, the routine
// it came from, or the lead that handed it out.
func (s *Service) taskActor(t protocol.TeamTask) (actor, initiator string) {
	switch t.Origin {
	case protocol.TaskFromRoutine:
		if r, ok := s.state.routine(t.Routine); ok {
			return r.Title, protocol.InitiatorRoutine
		}
		return "Hyakki", protocol.InitiatorRoutine
	case protocol.TaskFromLead:
		if team, ok := s.state.team(t.Team); ok {
			return teamLead(team).Name, protocol.InitiatorAgent
		}
		return "Hyakki", protocol.InitiatorAgent
	}
	return protocol.InitiatorYou, protocol.InitiatorYou
}

// taskIsOpen reports a task that has not ended.
func taskIsOpen(t protocol.TeamTask) bool {
	return t.Status == protocol.TaskReady || t.Status == protocol.TaskWorking || t.Status == protocol.TaskNeedsYou
}

// stoppedByYou reports a stop the user asked for, by the reason it was given.
func stoppedByYou(reason string) bool {
	return reason == i18n.T("You stopped it.") || reason == i18n.T("You deleted it.") || reason == i18n.T("The team was paused.") || reason == i18n.T("Every team was paused.")
}

// taskMember finds a member by id, or by name for what an agent or a person
// typed.
func taskMember(team protocol.Team, ref string) (protocol.Member, bool) {
	for _, m := range team.Members {
		if m.ID == ref {
			return m, true
		}
	}
	for _, m := range team.Members {
		if strings.EqualFold(strings.TrimSpace(m.Name), strings.TrimSpace(ref)) {
			return m, true
		}
	}
	return protocol.Member{}, false
}

// teamLead is the configured root or synthesizer, never an implicit peer king.
func teamLead(team protocol.Team) protocol.Member {
	for _, m := range team.Members {
		if m.Lead {
			return m
		}
	}
	return protocol.Member{}
}

// runToken is a fresh secret a run's team tools prove the run with.
func runToken() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

// taskTitle is the given title, or protocol.BriefTitle of the brief.
func taskTitle(title, brief string) string {
	if title = strings.TrimSpace(title); title != "" {
		return title
	}
	return protocol.BriefTitle(brief)
}

// clipRunes cuts text to at most n runes.
func clipRunes(text string, n int) string {
	if utf8.RuneCountInString(text) <= n {
		return text
	}
	return string([]rune(text)[:n])
}
