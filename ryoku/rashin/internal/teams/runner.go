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

	registry "ryoku-rashin/internal/harness"
	"ryoku-rashin/internal/protocol"
)

// run is a team task while an Agent session works it. Token is what the
// session's MCP server proves the run with; a run ends with its task.
type run struct {
	Session, Task, Team, Member, Token string
	Lead                               bool
	Started                            time.Time
	// Ready is set once the session's engine is set up; from then on Model,
	// the model the session last reported, follows a switch the user makes
	Ready bool
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
		return protocol.TeamTask{}, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.Tf("A model id has at most %d characters and no spaces.", protocol.EngineModelMax))
	}
	if !protocol.ValidEngineOptions(p.Options) {
		return protocol.TeamTask{}, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.Tf("A harness takes at most %d options, each with an id and a value.", protocol.EngineOptionsMax))
	}
	options := p.Options
	if len(options) == 0 {
		options = nil
	}
	return s.newTask(ctx, p.Team, p.Member, p.Title, p.Brief, protocol.TaskFromYou, "", "", model, options)
}

// newTask queues a task for a team. A paused team keeps it queued until it
// works again; an empty member hands it to the lead. Model and options are
// an engine for this task alone, empty to keep its member's.
func (s *Service) newTask(ctx context.Context, team, member, title, brief, origin, routine, parent, model string, options map[string]string) (protocol.TeamTask, error) {
	t, ok := s.state.team(team)
	if !ok {
		return protocol.TeamTask{}, protocol.Errorf(protocol.CodeNotFound, "%s", i18n.Tf("No team %s.", team))
	}
	brief = strings.TrimSpace(brief)
	if brief == "" {
		return protocol.TeamTask{}, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("A task needs a brief: what the team should do."))
	}
	assignee := teamLead(t)
	if member != "" {
		found, known := taskMember(t, member)
		if !known {
			return protocol.TeamTask{}, protocol.Errorf(protocol.CodeNotFound, "%s", i18n.Tf("%s has no member %s.", t.Name, member))
		}
		assignee = found
	}
	if assignee.ID == "" {
		return protocol.TeamTask{}, protocol.Errorf(protocol.CodeConflict, "%s", i18n.Tf("%s has no member to work on it.", t.Name))
	}
	now := s.now()
	turns := t.Budget.TaskTurns
	// a dream's tasks are exempt from the turns per task: their goal resumes when spent
	if isDream(t) {
		turns = protocol.TeamTaskTurnsMax
	}
	task := protocol.TeamTask{
		ID: newID("task"), Team: t.ID, Title: taskTitle(title, brief), Brief: brief, Status: protocol.TaskReady,
		Member: assignee.ID, Parent: parent, Origin: origin, Routine: routine, MaxTurns: turns,
		Model: model, Options: options, Created: now, Updated: now,
	}
	if err := s.state.putTask(task); err != nil {
		return protocol.TeamTask{}, err
	}
	actor, initiator := s.taskActor(task)
	s.record(protocol.Activity{
		Actor: actor, Action: "task.create", Target: task.Title, OK: true, Team: t.ID, Task: task.ID, Initiator: initiator,
		Detail: s.redact(i18n.Tf("%s takes it on.", assignee.Name)),
	})
	s.emitTask(task)
	s.pump()
	if current, ok := s.state.task(task.ID); ok {
		task = current
	}
	return task, nil
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

// stopTask ends a task the user or Rashin stops, with the pieces it handed
// out, and says no to what it still waited on.
func (s *Service) stopTask(ctx context.Context, id, reason string) (protocol.TeamTask, error) {
	t, ok := s.state.task(id)
	if !ok {
		return protocol.TeamTask{}, protocol.Errorf(protocol.CodeNotFound, "%s", i18n.Tf("No task %s.", id))
	}
	if !taskIsOpen(t) {
		return t, nil
	}
	ended, ok := s.endTask(id, func(t *protocol.TeamTask) {
		t.Status, t.Reason = protocol.TaskStopped, reason
	})
	if !ok {
		current, _ := s.state.task(id)
		return current, nil
	}
	s.halt(ended.Session)
	for _, piece := range s.state.tasks(func(p protocol.TeamTask) bool { return p.Parent == id && taskIsOpen(p) }) {
		_, _ = s.stopTask(ctx, piece.ID, i18n.T("The lead's task was stopped."))
	}
	s.denyPending(id, i18n.T("The task was stopped."))
	actor, initiator := "Rashin", protocol.InitiatorRashin
	if stoppedByYou(reason) {
		actor, initiator = protocol.InitiatorYou, protocol.InitiatorYou
	}
	s.record(protocol.Activity{Actor: actor, Action: "task.stop", Target: ended.Title, OK: true, Team: ended.Team, Task: ended.ID, Initiator: initiator, Detail: s.redact(reason)})
	s.afterEnd(ended)
	return ended, nil
}

// retryTask runs a finished, blocked or stopped task again in a new session.
func (s *Service) retryTask(_ context.Context, p protocol.TeamTaskRetryParams) (protocol.TeamTask, error) {
	s.mu.Lock()
	t, ok := s.state.task(p.ID)
	if !ok {
		s.mu.Unlock()
		return protocol.TeamTask{}, protocol.Errorf(protocol.CodeNotFound, "%s", i18n.Tf("No task %s.", p.ID))
	}
	if taskIsOpen(t) {
		s.mu.Unlock()
		return protocol.TeamTask{}, protocol.Errorf(protocol.CodeConflict, "%s", i18n.T("Only a finished, blocked or stopped task runs again."))
	}
	if _, ok := s.state.team(t.Team); !ok {
		s.mu.Unlock()
		return protocol.TeamTask{}, protocol.Errorf(protocol.CodeNotFound, "%s", i18n.Tf("No team %s.", t.Team))
	}
	if note := strings.TrimSpace(p.Note); note != "" {
		t.Brief += "\n\n" + i18n.Tf("When you ran this before: %s", note)
	}
	runAgain(&t, s.now())
	err := s.state.putTask(t)
	s.mu.Unlock()
	if err != nil {
		return protocol.TeamTask{}, err
	}
	s.record(protocol.Activity{Actor: protocol.InitiatorYou, Action: "task.retry", Target: t.Title, OK: true, Team: t.Team, Task: t.ID, Initiator: protocol.InitiatorYou, Detail: i18n.T("It runs again in a new conversation.")})
	s.emitTask(t)
	s.pump()
	if current, ok := s.state.task(t.ID); ok {
		t = current
	}
	return t, nil
}

// runAgain readies an ended task to run again in a new session, on its
// member's engine as it is now.
func runAgain(t *protocol.TeamTask, now time.Time) {
	t.Status, t.Waiting, t.Report, t.Reason, t.SignIn = protocol.TaskReady, "", nil, "", ""
	t.Session, t.Started, t.Ended, t.Read, t.Steps = "", nil, nil, false, 0
	t.Harness, t.Model, t.Options = "", "", nil
	t.Updated = now
}

// deleteTask stops a task, then removes it and the pieces it handed out.
// Their sessions stay, so the conversations can still be read.
func (s *Service) deleteTask(ctx context.Context, id string) error {
	if _, ok := s.state.task(id); !ok {
		return protocol.Errorf(protocol.CodeNotFound, "%s", i18n.Tf("No task %s.", id))
	}
	if _, err := s.stopTask(ctx, id, i18n.T("You deleted it.")); err != nil {
		return err
	}
	for _, piece := range s.state.tasks(func(p protocol.TeamTask) bool { return p.Parent == id }) {
		if err := s.state.removeTask(piece.ID); err != nil {
			return err
		}
		s.broadcast(protocol.EvtTeamTaskRemoved, protocol.TeamRemoved{ID: piece.ID})
	}
	if err := s.state.removeTask(id); err != nil {
		return err
	}
	s.broadcast(protocol.EvtTeamTaskRemoved, protocol.TeamRemoved{ID: id})
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
	if s.sessions == nil {
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
	var starts []protocol.TeamTask
	for i := len(open) - 1; i >= 0 && running < limit; i-- {
		t := open[i]
		if t.Status != protocol.TaskReady {
			continue
		}
		team, ok := s.state.team(t.Team)
		if !ok || team.Paused || perTeam[t.Team] >= teamLimit(team, settings) {
			continue
		}
		member, _ := taskMember(team, t.Member)
		now := s.now()
		t.Status, t.Waiting, t.Started, t.Updated = protocol.TaskWorking, "", &now, now
		if s.state.putTask(t) != nil {
			continue
		}
		s.runs[t.ID] = &run{Task: t.ID, Team: t.Team, Member: t.Member, Token: runToken(), Lead: member.Lead, Started: now}
		s.armMinutesLocked(t.ID)
		running++
		perTeam[t.Team]++
		starts = append(starts, t)
	}
	s.mu.Unlock()
	for _, t := range starts {
		s.emitTask(t)
		go s.start(t)
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

// start brings a claimed task up as an Agent session: the engine resolved
// for it, in the team's folder under the team's approvals, then the task as
// the session's goal, which starts its first turn. The task keeps the
// engine it started with; an option the harness refuses is left at the
// harness's default and said so in the ledger. A harness not signed in for
// teams blocks it; one that is runs in its team home, put back as it was
// signed in.
func (s *Service) start(task protocol.TeamTask) {
	ctx := s.background
	var harnessID, model, signIn string
	var options map[string]string
	fail := func(session string, reason string) {
		ended, ok := s.endTask(task.ID, func(t *protocol.TeamTask) {
			t.Status, t.Reason, t.SignIn = protocol.TaskBlocked, reason, signIn
			if harnessID != "" {
				t.Harness, t.Model, t.Options = harnessID, model, maps.Clone(options)
			}
		})
		if session != "" {
			s.halt(session)
		}
		if !ok {
			return
		}
		s.record(protocol.Activity{Actor: "Rashin", Action: "task.block", Target: ended.Title, OK: false, Team: ended.Team, Task: ended.ID, Initiator: protocol.InitiatorRashin, Detail: s.redact(reason)})
		s.notifyEnd(ended)
		s.afterEnd(ended)
	}
	team, ok := s.state.team(task.Team)
	if !ok {
		fail("", i18n.T("Its team no longer exists."))
		return
	}
	member, ok := taskMember(team, task.Member)
	if !ok {
		fail("", i18n.T("The member it was given to is no longer on the team."))
		return
	}
	harnessID, model, options = s.taskEngine(member, task)
	if harnessID == "" {
		fail("", i18n.T("No harness is installed to run it."))
		return
	}
	if _, signed := s.signedIn(harnessID, team.ID); !signed {
		signIn = harnessID
		fail("", i18n.Tf("This team is not signed in to %s. Sign it in for Teams, then retry the task.", registry.Name(harnessID, s.settings())))
		return
	}
	if err := s.RestoreHome(harnessID, team.ID); err != nil {
		fail("", i18n.Tf("Rashin could not put the team's %s home back as it was signed in: %v", registry.Name(harnessID, s.settings()), err))
		return
	}
	if err := os.MkdirAll(team.Folder, 0o700); err != nil {
		fail("", i18n.Tf("The team's folder could not be made: %v", err))
		return
	}
	info, err := s.sessions.New(ctx, protocol.SessionNewParams{
		Kind: protocol.KindAgent, Harness: harnessID, Cwd: team.Folder, Title: task.Title, Origin: protocol.OriginTeam,
		Team: team.ID, TeamTask: task.ID, Member: member.ID, Approvals: team.Approvals,
	})
	if err != nil {
		fail("", i18n.Tf("The conversation could not start: %v", err))
		return
	}
	s.mu.Lock()
	r := s.runs[task.ID]
	current, known := s.state.task(task.ID)
	if r == nil || !known || !taskIsOpen(current) {
		s.mu.Unlock()
		return
	}
	r.Session = info.ID
	current.Session = info.ID
	current.Sessions = append(current.Sessions, info.ID)
	current.Harness, current.Model, current.Options = harnessID, model, maps.Clone(options)
	current.Updated = s.now()
	err = s.state.putTask(current)
	s.mu.Unlock()
	if err != nil {
		fail(info.ID, err.Error())
		return
	}
	s.emitTask(current)
	if err := s.sessions.Start(ctx, info.ID); err != nil {
		fail(info.ID, i18n.Tf("The harness did not start: %v", err))
		return
	}
	if model != "" {
		if err := s.sessions.SetModel(ctx, info.ID, model); err != nil {
			fail(info.ID, i18n.Tf("The model %s could not be used: %v", model, err))
			return
		}
	}
	var refused []string
	for _, option := range slices.Sorted(maps.Keys(options)) {
		if err := s.sessions.SetOption(ctx, info.ID, option, options[option]); err != nil {
			refused = append(refused, option)
			s.record(protocol.Activity{
				Actor: "Rashin", Action: "task.option", Target: task.Title, OK: false, Team: team.ID, Task: task.ID, Initiator: protocol.InitiatorRashin,
				Detail: i18n.Tf("%s's %s %s was not available; it ran with the harness default.", member.Name, option, options[option]),
			})
		}
	}
	if !s.live(task.ID, info.ID) {
		s.halt(info.ID)
		return
	}
	s.engage(task.ID, info.ID, model, refused)
	if _, err := s.sessions.Goal(ctx, protocol.GoalParams{ID: info.ID, Op: protocol.GoalSet, Objective: objective(task), MaxTurns: task.MaxTurns}); err != nil {
		fail(info.ID, i18n.Tf("The task could not be set as the goal: %v", err))
		return
	}
	_, initiator := s.taskActor(task)
	s.record(protocol.Activity{
		Actor: member.Name, Action: "task.start", Target: task.Title, OK: true, Team: team.ID, Task: task.ID, Initiator: initiator,
		Detail: i18n.Tf("%s started work on it with %s.", member.Name, s.engineWords(harnessID, model)),
	})
}

// engage marks a run's engine set up: the options its harness refused leave
// the task's record, and the model its session runs on is the one a later
// switch is told from.
func (s *Service) engage(taskID, session, model string, refused []string) {
	info, _ := s.sessions.Info(session)
	s.mu.Lock()
	r := s.runs[taskID]
	t, ok := s.state.task(taskID)
	if r == nil || r.Session != session || !ok {
		s.mu.Unlock()
		return
	}
	r.Ready, r.Model = true, info.Model
	if model != "" {
		r.Model = model
	}
	changed := false
	for _, option := range refused {
		if _, had := t.Options[option]; had {
			delete(t.Options, option)
			changed = true
		}
	}
	if changed {
		if len(t.Options) == 0 {
			t.Options = nil
		}
		t.Updated = s.now()
		changed = s.state.putTask(t) == nil
	}
	s.mu.Unlock()
	if changed {
		s.emitTask(t)
	}
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
	return r != nil && r.Session == session
}

// endTask finishes an open task once: change sets how it ended, the run is
// dropped so its token answers no more, the steps its session took are kept,
// the day's spend counts it, and the session is let go once its turn is
// over. It reports false when the task had already ended.
func (s *Service) endTask(id string, change func(*protocol.TeamTask)) (protocol.TeamTask, bool) {
	// the session is asked before the lock, as engage does
	var steps int
	if t, ok := s.state.task(id); ok && t.Session != "" && s.sessions != nil {
		info, _ := s.sessions.Info(t.Session)
		steps = info.Steps
	}
	s.mu.Lock()
	t, ok := s.state.task(id)
	if !ok || !taskIsOpen(t) {
		s.mu.Unlock()
		return t, false
	}
	change(&t)
	now := s.now()
	t.Waiting, t.Ended, t.Updated, t.Steps = "", &now, now, steps
	if err := s.state.putTask(t); err != nil {
		s.mu.Unlock()
		return t, false
	}
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
}

// releaseParent sends a lead on with its pieces' reports once none is open.
func (s *Service) releaseParent(parent string) {
	lead, ok := s.state.task(parent)
	if !ok || !taskIsOpen(lead) || lead.Waiting != protocol.WaitingOnMembers {
		return
	}
	pieces := s.state.tasks(func(p protocol.TeamTask) bool { return p.Parent == parent })
	for _, piece := range pieces {
		if taskIsOpen(piece) {
			return
		}
	}
	team, _ := s.state.team(lead.Team)
	s.release(parent, piecesText(team, pieces))
}

// release ends a hold: the task works again and its agent is continued under
// its goal with text, what the user decided or what the members reported.
// A task that already ended, or never started a session, has nothing to
// continue.
func (s *Service) release(taskID, text string) {
	if s.sessions == nil || s.settings().Teams.Paused {
		return
	}
	s.mu.Lock()
	t, ok := s.state.task(taskID)
	if !ok || !taskIsOpen(t) || t.Session == "" {
		s.mu.Unlock()
		return
	}
	if team, known := s.state.team(t.Team); !known || team.Paused {
		s.mu.Unlock()
		return
	}
	if s.pendingApprovals(taskID) > 0 {
		s.mu.Unlock()
		return
	}
	t.Status, t.Waiting, t.Updated = protocol.TaskWorking, "", s.now()
	err := s.state.putTask(t)
	if err == nil {
		s.armMinutesLocked(taskID)
	}
	s.mu.Unlock()
	if err != nil {
		return
	}
	s.emitTask(t)
	session := t.Session
	go func() {
		if err := s.sessions.Continue(s.background, session, text); err != nil && s.background.Err() == nil {
			ended, ok := s.endTask(taskID, func(t *protocol.TeamTask) {
				t.Status, t.Reason = protocol.TaskBlocked, i18n.Tf("Rashin could not continue the task: %v", err)
			})
			if ok {
				s.record(protocol.Activity{Actor: "Rashin", Action: "task.block", Target: ended.Title, OK: false, Team: ended.Team, Task: ended.ID, Initiator: protocol.InitiatorRashin, Detail: s.redact(ended.Reason)})
				s.notifyEnd(ended)
				s.afterEnd(ended)
			}
		}
	}()
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
		s.record(protocol.Activity{Actor: "Rashin", Action: "approval.deny", Target: rec.Title, OK: true, Team: rec.Team, Task: task, Initiator: protocol.InitiatorRashin, Detail: note})
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
	go func() {
		if info, ok := s.sessions.Info(session); ok && info.Goal != nil && (info.Goal.Status == protocol.GoalActive || info.Goal.Status == protocol.GoalPaused) {
			_, _ = s.sessions.Goal(s.background, protocol.GoalParams{ID: session, Op: protocol.GoalStop})
		}
		_ = s.sessions.Stop(s.background, session)
	}()
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
	go func() { _ = s.sessions.Stop(s.background, session) }()
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
		team, _ := s.state.team(t.Team)
		member, _ := taskMember(team, t.Member)
		started := s.now()
		if t.Started != nil {
			started = *t.Started
		}
		found[t.ID] = &run{Session: t.Session, Task: t.ID, Team: t.Team, Member: t.Member, Token: runToken(), Lead: member.Lead, Started: started, Ready: true, Model: info.Model}
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
	for _, id := range unheld {
		if t, ok := s.state.task(id); ok {
			t.Status, t.Waiting, t.Updated = protocol.TaskWorking, "", s.now()
			if s.state.putTask(t) == nil {
				changed = append(changed, t)
			}
		}
	}
	for id := range found {
		s.armMinutesLocked(id)
	}
	s.mu.Unlock()
	for _, t := range changed {
		s.emitTask(t)
	}
	for _, id := range lost {
		ended, ok := s.endTask(id, func(t *protocol.TeamTask) {
			t.Status, t.Reason = protocol.TaskBlocked, i18n.T("Rashin restarted before the task ended.")
		})
		if ok {
			s.record(protocol.Activity{Actor: "Rashin", Action: "task.block", Target: ended.Title, OK: false, Team: ended.Team, Task: ended.ID, Initiator: protocol.InitiatorRashin, Detail: ended.Reason})
			s.afterEnd(ended)
		}
	}
	for _, session := range carryOn {
		go func(session string) {
			_ = s.sessions.Continue(s.background, session, i18n.T("Rashin restarted while you worked. Carry on with the task."))
		}(session)
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
		return "Rashin", protocol.InitiatorRoutine
	case protocol.TaskFromLead:
		if team, ok := s.state.team(t.Team); ok {
			return teamLead(team).Name, protocol.InitiatorAgent
		}
		return "Rashin", protocol.InitiatorAgent
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

// teamLead is the member that leads, the first one when none is marked.
func teamLead(team protocol.Team) protocol.Member {
	for _, m := range team.Members {
		if m.Lead {
			return m
		}
	}
	if len(team.Members) > 0 {
		return team.Members[0]
	}
	return protocol.Member{}
}

// runToken is a fresh secret a run's MCP server proves itself with.
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
