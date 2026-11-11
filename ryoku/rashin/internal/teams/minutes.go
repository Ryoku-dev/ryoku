package teams

import (
	"sort"
	"time"

	i18n "ryoku-i18n"

	"ryoku-rashin/internal/protocol"
)

// armMinutes sets a working task's time limit again, after its hold on the
// user ended or its team's limit changed.
func (s *Service) armMinutes(taskID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.armMinutesLocked(taskID)
}

// armMinutesLocked sets the one timer of a task's run to the end of its
// team's minutes per task. The time it waited on the user's answers does
// not count, so a task held on the user has no timer: releasing it arms one
// again. The caller holds mu.
func (s *Service) armMinutesLocked(taskID string) {
	r := s.runs[taskID]
	if r == nil {
		return
	}
	if r.timer != nil {
		r.timer.Stop()
		r.timer = nil
	}
	t, ok := s.state.task(taskID)
	if !ok || !taskIsOpen(t) || t.Waiting == protocol.WaitingOnYou {
		return
	}
	team, ok := s.state.team(t.Team)
	if !ok || team.Budget.TaskMinutes <= 0 {
		return
	}
	left := s.minutesEnd(t, team.Budget.TaskMinutes).Sub(s.now())
	r.timer = time.AfterFunc(max(left, 0), func() {
		if s.background.Err() == nil {
			s.overrun(taskID)
		}
	})
}

// armTeamMinutes sets the time limit of every running task of a team again
// once the team's minutes per task changed.
func (s *Service) armTeamMinutes(team string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, r := range s.runs {
		if r.Team == team {
			s.armMinutesLocked(id)
		}
	}
}

// minutesEnd is when a task runs out of its minutes: its start plus the
// minutes plus the time it waited on the user's answers.
func (s *Service) minutesEnd(t protocol.TeamTask, minutes int) time.Time {
	started := s.now()
	if t.Started != nil {
		started = *t.Started
	}
	return started.Add(time.Duration(minutes)*time.Minute + s.waitedOnYou(t.ID, started))
}

// waitedOnYou is how long a task waited on the user's answers since it
// started: the time its approvals were pending, overlaps counted once.
func (s *Service) waitedOnYou(task string, started time.Time) time.Duration {
	now := s.now()
	type span struct{ from, to time.Time }
	var spans []span
	for _, rec := range s.state.approvals(func(r approvalRecord) bool { return r.Task == task }) {
		from, to := rec.Created, now
		if rec.Decided != nil {
			to = *rec.Decided
		}
		if from.Before(started) {
			from = started
		}
		if to.After(from) {
			spans = append(spans, span{from, to})
		}
	}
	sort.Slice(spans, func(i, j int) bool { return spans[i].from.Before(spans[j].from) })
	var total time.Duration
	var reached time.Time
	for _, sp := range spans {
		if sp.from.Before(reached) {
			sp.from = reached
		}
		if sp.to.After(sp.from) {
			total += sp.to.Sub(sp.from)
			reached = sp.to
		}
	}
	return total
}

// overrun ends a task that worked past its minutes per task as blocked,
// with the pieces it still waits on. One that waits on the user, or whose
// waiting moved its end, is left to its next timer.
func (s *Service) overrun(taskID string) {
	t, ok := s.state.task(taskID)
	if !ok || !taskIsOpen(t) || t.Waiting == protocol.WaitingOnYou {
		return
	}
	team, ok := s.state.team(t.Team)
	if !ok || team.Budget.TaskMinutes <= 0 {
		return
	}
	minutes := team.Budget.TaskMinutes
	if s.now().Before(s.minutesEnd(t, minutes)) {
		s.armMinutes(taskID)
		return
	}
	reason := i18n.Tf("It worked past its %d minutes.", minutes)
	if minutes == 1 {
		reason = i18n.T("It worked past its one minute.")
	}
	ended, ok := s.endTask(taskID, func(t *protocol.TeamTask) {
		t.Status, t.Reason = protocol.TaskBlocked, reason
	})
	if !ok {
		return
	}
	s.halt(ended.Session)
	for _, piece := range s.state.tasks(func(p protocol.TeamTask) bool { return p.Parent == taskID && taskIsOpen(p) }) {
		_, _ = s.stopTask(s.background, piece.ID, i18n.T("The lead's task ran out of time."))
	}
	s.record(protocol.Activity{Actor: "Rashin", Action: "task.block", Target: ended.Title, OK: false, Team: ended.Team, Task: ended.ID, Initiator: protocol.InitiatorRashin, Detail: ended.Reason})
	s.notifyEnd(ended)
	s.afterEnd(ended)
}
