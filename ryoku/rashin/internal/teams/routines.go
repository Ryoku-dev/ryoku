package teams

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	i18n "ryoku-i18n"

	"ryoku-rashin/internal/cron"
	"ryoku-rashin/internal/protocol"
)

// routineSkipped is a routine's LastStatus after a run it did not start,
// because it was missed or the last run still worked.
const routineSkipped = "skipped"

// routineEditable are the routine fields teams.routine_update may patch.
var routineEditable = map[string]bool{
	"title": true, "brief": true, "member": true, "schedule": true,
	"enabled": true, "missed": true, "overlap": true,
}

// listRoutines lists a team's routines, or every team's when team is empty,
// with their sentence and next run filled.
func (s *Service) listRoutines(team string) protocol.RoutinesResult {
	now := s.now()
	out := []protocol.Routine{}
	for _, r := range s.state.routines() {
		if team == "" || r.Team == team {
			out = append(out, s.fillRoutine(r, now))
		}
	}
	return protocol.RoutinesResult{Routines: out}
}

// fillRoutine adds what the daemon derives: the schedule in words and, for a
// routine that will fire, when it next does.
func (s *Service) fillRoutine(r protocol.Routine, now time.Time) protocol.Routine {
	r.Sentence, r.NextRun = "", nil
	schedule, err := cron.Parse(r.Schedule)
	if err != nil {
		return r
	}
	r.Sentence = schedule.Sentence()
	if team, ok := s.state.team(r.Team); !r.Enabled || !ok || team.Paused || s.settings().Teams.Paused {
		return r
	}
	next := s.clock.due(r.ID)
	if next.IsZero() {
		next = schedule.Next(now)
	}
	if !next.IsZero() {
		r.NextRun = &next
	}
	return r
}

// createRoutine makes a routine for a team and, when asked, starts one run
// at once so the user sees what it does.
func (s *Service) createRoutine(ctx context.Context, p protocol.RoutineCreateParams) (protocol.Routine, error) {
	now := s.now()
	r := protocol.Routine{
		ID: newID("routine"), Team: p.Team, Title: p.Title, Brief: p.Brief, Member: p.Member,
		Schedule: p.Schedule, Enabled: p.Enabled, Missed: p.Missed, Overlap: p.Overlap,
		Created: now, Updated: now,
	}
	s.clock.edit.Lock()
	err := s.validRoutine(&r)
	if err == nil {
		err = s.state.putRoutine(r)
	}
	s.clock.edit.Unlock()
	if err != nil {
		return protocol.Routine{}, err
	}
	s.record(protocol.Activity{
		Actor: protocol.InitiatorYou, Action: "routine.create", Target: r.Title, OK: true,
		Team: r.Team, Initiator: protocol.InitiatorYou, Detail: s.fillRoutine(r, now).Sentence,
	})
	s.clock.rearm()
	if p.RunNow {
		if team, ok := s.state.team(r.Team); ok && !team.Paused && !s.settings().Teams.Paused {
			_, _ = s.fireRoutine(ctx, r, protocol.InitiatorYou)
		}
	}
	if current, ok := s.state.routine(r.ID); ok {
		r = current
	}
	filled := s.fillRoutine(r, s.now())
	s.broadcast(protocol.EvtRoutine, filled)
	return filled, nil
}

// updateRoutine applies a JSON merge patch to a routine's title, brief,
// member, schedule, enabled, missed and overlap.
func (s *Service) updateRoutine(_ context.Context, p protocol.RoutineUpdateParams) (protocol.Routine, error) {
	var patch map[string]any
	if err := json.Unmarshal(p.Patch, &patch); err != nil || patch == nil {
		return protocol.Routine{}, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("The request's parameters are not valid."))
	}
	for key := range patch {
		if !routineEditable[key] {
			return protocol.Routine{}, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.Tf("A routine's %s cannot be changed.", key))
		}
	}
	s.clock.edit.Lock()
	prev, next, err := s.patchRoutine(p.ID, patch)
	s.clock.edit.Unlock()
	if err != nil {
		return protocol.Routine{}, err
	}
	detail := i18n.T("Changed it.")
	switch {
	case next.Enabled && !prev.Enabled:
		detail = i18n.T("Turned it on.")
	case !next.Enabled && prev.Enabled:
		detail = i18n.T("Turned it off.")
	}
	s.record(protocol.Activity{
		Actor: protocol.InitiatorYou, Action: "routine.update", Target: next.Title, OK: true,
		Team: next.Team, Initiator: protocol.InitiatorYou, Detail: detail,
	})
	s.clock.rearm()
	filled := s.fillRoutine(next, s.now())
	s.broadcast(protocol.EvtRoutine, filled)
	return filled, nil
}

// patchRoutine merges and stores the patch; the caller holds the edit lock.
func (s *Service) patchRoutine(id string, patch map[string]any) (protocol.Routine, protocol.Routine, error) {
	prev, ok := s.state.routine(id)
	if !ok {
		return prev, prev, protocol.Errorf(protocol.CodeNotFound, "%s", i18n.Tf("No routine %s.", id))
	}
	invalid := protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("The request's parameters are not valid."))
	raw, err := json.Marshal(prev)
	if err != nil {
		return prev, prev, err
	}
	var doc any
	if err := json.Unmarshal(raw, &doc); err != nil {
		return prev, prev, err
	}
	if raw, err = json.Marshal(mergePatch(doc, patch)); err != nil {
		return prev, prev, invalid
	}
	var next protocol.Routine
	if err := json.Unmarshal(raw, &next); err != nil {
		return prev, prev, invalid
	}
	next.ID, next.Team, next.Created = prev.ID, prev.Team, prev.Created
	if err := s.validRoutine(&next); err != nil {
		return prev, prev, err
	}
	// a routine turned back on after it turned itself off starts its count of
	// failed runs again
	if next.Enabled && !prev.Enabled {
		next.Failures = 0
	}
	next.Updated = s.now()
	if err := s.state.putRoutine(next); err != nil {
		return prev, prev, err
	}
	return prev, next, nil
}

// validRoutine checks and normalises a routine before it is stored: its team
// exists and routineFits accepts it with the routines already on. The
// caller holds the edit lock.
func (s *Service) validRoutine(r *protocol.Routine) error {
	team, ok := s.state.team(r.Team)
	if !ok {
		return protocol.Errorf(protocol.CodeNotFound, "%s", i18n.Tf("No team %s.", r.Team))
	}
	return s.routineFits(team, r, 0)
}

// routineFits checks and normalises a routine of team: it has a title and a
// brief, its member belongs to the team, its schedule parses and runs no
// more often than RoutineMinInterval, and with the routines stored and
// pending more about to be turned on, no more than RoutinesEnabledMax are on.
func (s *Service) routineFits(team protocol.Team, r *protocol.Routine, pending int) error {
	r.Title, r.Brief, r.Schedule = strings.TrimSpace(r.Title), strings.TrimSpace(r.Brief), strings.TrimSpace(r.Schedule)
	r.Sentence, r.NextRun = "", nil
	if r.Title == "" {
		return protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("A routine needs a title."))
	}
	if r.Brief == "" {
		return protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("A routine needs to say what the team should do."))
	}
	if r.Member != "" {
		member, known := taskMember(team, r.Member)
		if !known {
			return protocol.Errorf(protocol.CodeNotFound, "%s", i18n.Tf("%s has no member %s.", team.Name, r.Member))
		}
		r.Member = member.ID
	}
	schedule, err := cron.Parse(r.Schedule)
	if err != nil {
		return protocol.Errorf(protocol.CodeBadRequest, "%s", err.Error())
	}
	if schedule.MinInterval() < protocol.RoutineMinInterval {
		return protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.Tf("A routine runs at most every %d minutes.", int(protocol.RoutineMinInterval/time.Minute)))
	}
	switch r.Missed {
	case "":
		r.Missed = protocol.MissedSkip
	case protocol.MissedSkip, protocol.MissedOnce:
	default:
		return protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("A missed run is either skipped or run once."))
	}
	switch r.Overlap {
	case "":
		r.Overlap = protocol.OverlapSkip
	case protocol.OverlapSkip, protocol.OverlapQueue:
	default:
		return protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("A run that falls due while the last one still works is either skipped or queued."))
	}
	if !r.Enabled {
		return nil
	}
	on := pending
	for _, other := range s.state.routines() {
		if other.Enabled && other.ID != r.ID {
			on++
		}
	}
	if on >= protocol.RoutinesEnabledMax {
		return protocol.Errorf(protocol.CodeConflict, "%s", i18n.Tf("At most %d routines can be on at once. Turn one off first.", protocol.RoutinesEnabledMax))
	}
	return nil
}

// deleteRoutine removes a routine; the tasks it made stay.
func (s *Service) deleteRoutine(id string) error {
	s.clock.edit.Lock()
	r, ok := s.state.routine(id)
	err := s.state.removeRoutine(id)
	s.clock.edit.Unlock()
	if !ok {
		return protocol.Errorf(protocol.CodeNotFound, "%s", i18n.Tf("No routine %s.", id))
	}
	if err != nil {
		return err
	}
	s.record(protocol.Activity{
		Actor: protocol.InitiatorYou, Action: "routine.delete", Target: r.Title, OK: true,
		Team: r.Team, Initiator: protocol.InitiatorYou, Detail: i18n.T("The tasks it made stay."),
	})
	s.clock.rearm()
	s.broadcast(protocol.EvtRoutineRemoved, protocol.TeamRemoved{ID: id})
	return nil
}

// dropRoutines removes every routine of a team that is being deleted.
func (s *Service) dropRoutines(team string) {
	var dropped []string
	s.clock.edit.Lock()
	for _, r := range s.state.routines() {
		if r.Team == team && s.state.removeRoutine(r.ID) == nil {
			dropped = append(dropped, r.ID)
		}
	}
	s.clock.edit.Unlock()
	s.clock.rearm()
	for _, id := range dropped {
		s.broadcast(protocol.EvtRoutineRemoved, protocol.TeamRemoved{ID: id})
	}
}

// runRoutine starts one run of a routine now, outside its schedule.
func (s *Service) runRoutine(ctx context.Context, id, initiator string) (protocol.TeamTask, error) {
	r, ok := s.state.routine(id)
	if !ok {
		return protocol.TeamTask{}, protocol.Errorf(protocol.CodeNotFound, "%s", i18n.Tf("No routine %s.", id))
	}
	team, ok := s.state.team(r.Team)
	if !ok {
		return protocol.TeamTask{}, protocol.Errorf(protocol.CodeNotFound, "%s", i18n.Tf("No team %s.", r.Team))
	}
	if team.Paused {
		return protocol.TeamTask{}, protocol.Errorf(protocol.CodeConflict, "%s", i18n.Tf("%s is paused, so its routines do not run.", team.Name))
	}
	if s.settings().Teams.Paused {
		return protocol.TeamTask{}, protocol.Errorf(protocol.CodeConflict, "%s", i18n.T("Every team is paused, so no routine runs."))
	}
	return s.fireRoutine(ctx, r, initiator)
}

// fireRoutine gives the routine's team the routine's task and records the
// run on the routine.
func (s *Service) fireRoutine(ctx context.Context, r protocol.Routine, initiator string) (protocol.TeamTask, error) {
	actor := r.Title
	if initiator == protocol.InitiatorYou {
		actor = protocol.InitiatorYou
	}
	brief := r.Brief + i18n.Tf("\n\nThis run is from the routine \"%s\"; when nothing changed since the last run, report that in one line with quiet set.", r.Title)
	task, err := s.newTask(ctx, r.Team, r.Member, r.Title, brief, protocol.TaskFromRoutine, r.ID, "", "", nil)
	if err != nil {
		s.record(protocol.Activity{
			Actor: actor, Action: "routine.fire", Target: r.Title, OK: false,
			Team: r.Team, Initiator: initiator, Detail: err.Error(),
		})
		return protocol.TeamTask{}, err
	}
	now := s.now()
	updated, ok := s.clock.change(r.ID, func(r *protocol.Routine) bool {
		r.LastRun, r.LastTask, r.LastStatus = &now, task.ID, task.Status
		return true
	})
	s.record(protocol.Activity{
		Actor: actor, Action: "routine.fire", Target: r.Title, OK: true,
		Team: r.Team, Task: task.ID, Initiator: initiator, Detail: i18n.Tf("Started the task %s.", task.Title),
	})
	if ok {
		s.broadcast(protocol.EvtRoutine, s.fillRoutine(updated, now))
	}
	return task, nil
}

// checkRoutine handles a routine the clock found due: it fires the run, or
// skips it when it was missed or the last run still works and the routine
// says to skip.
func (s *Service) checkRoutine(id string, due, now time.Time) {
	r, ok := s.state.routine(id)
	if !ok || !r.Enabled {
		return
	}
	if team, ok := s.state.team(r.Team); !ok || team.Paused || s.settings().Teams.Paused {
		return
	}
	if now.Sub(due) > routineLate && r.Missed != protocol.MissedOnce {
		s.skipRoutine(r, i18n.T("Missed while the computer was off."), now)
		return
	}
	if r.Overlap != protocol.OverlapQueue && s.routineOpen(r) {
		s.skipRoutine(r, i18n.T("The last run was still working."), now)
		return
	}
	_, _ = s.fireRoutine(s.background, r, protocol.InitiatorRoutine)
}

// routineOpen reports a routine whose last run has not ended.
func (s *Service) routineOpen(r protocol.Routine) bool {
	if r.LastTask == "" {
		return false
	}
	task, ok := s.state.task(r.LastTask)
	return ok && taskIsOpen(task)
}

// lastReport is the report of the routine's run before task: the newest
// other run of its routine that ended done with a report. A quiet report is
// passed over, since it says only that nothing changed since the one before.
func (s *Service) lastReport(task protocol.TeamTask) (protocol.TeamReport, bool) {
	runs := s.state.tasks(func(t protocol.TeamTask) bool {
		return t.Routine == task.Routine && t.ID != task.ID && t.Status == protocol.TaskDone &&
			t.Report != nil && !t.Report.Quiet && strings.TrimSpace(t.Report.Body) != ""
	})
	var last protocol.TeamReport
	found := false
	for _, run := range runs {
		if !found || run.Report.At.After(last.At) {
			last, found = *run.Report, true
		}
	}
	return last, found
}

// skipRoutine records a run the routine did not start.
func (s *Service) skipRoutine(r protocol.Routine, reason string, now time.Time) {
	updated, ok := s.clock.change(r.ID, func(r *protocol.Routine) bool {
		r.LastRun, r.LastStatus = &now, routineSkipped
		return true
	})
	s.record(protocol.Activity{
		Actor: r.Title, Action: "routine.skip", Target: r.Title, OK: true,
		Team: r.Team, Initiator: protocol.InitiatorRoutine, Detail: reason,
	})
	if ok {
		s.broadcast(protocol.EvtRoutine, s.fillRoutine(updated, now))
	}
}

// routineTaskEnded counts how a routine's run ended: a report clears its
// failures, a block or stop adds one, and RoutineFailuresMax in a row turn
// the routine off and tell the user. A quiet report adds to its quiet
// streak; any other ending sets the streak back to 0.
func (s *Service) routineTaskEnded(task protocol.TeamTask) {
	off := false
	r, ok := s.clock.change(task.Routine, func(r *protocol.Routine) bool {
		switch task.Status {
		case protocol.TaskDone:
			r.Failures = 0
		case protocol.TaskBlocked, protocol.TaskStopped:
			r.Failures++
			if r.Enabled && r.Failures >= protocol.RoutineFailuresMax {
				r.Enabled, off = false, true
			}
		default:
			return false
		}
		if task.Status == protocol.TaskDone && task.Report != nil && task.Report.Quiet {
			r.QuietStreak++
		} else {
			r.QuietStreak = 0
		}
		if r.LastTask == task.ID {
			r.LastStatus = task.Status
		}
		return true
	})
	if !ok {
		return
	}
	s.broadcast(protocol.EvtRoutine, s.fillRoutine(r, s.now()))
	if !off {
		return
	}
	detail := i18n.Tf("It did not finish %d runs in a row, so it turned itself off.", protocol.RoutineFailuresMax)
	s.record(protocol.Activity{
		Actor: "Rashin", Action: "routine.off", Target: r.Title, OK: false,
		Team: r.Team, Task: task.ID, Initiator: protocol.InitiatorRashin, Detail: detail,
	})
	s.clock.rearm()
	if team, ok := s.state.team(r.Team); ok {
		s.notify(notification{
			Team: r.Team, Task: task.ID, Needs: true,
			Title: i18n.Tf("%s: %s turned itself off", team.Name, r.Title), Body: detail,
		})
	}
}

// previewSchedule says a schedule in words and lists its next runs: five
// unless asked, at most twenty.
func previewSchedule(p protocol.RoutinePreviewParams, now time.Time) protocol.RoutinePreview {
	return cron.Preview(p.Schedule, now, p.Count)
}
