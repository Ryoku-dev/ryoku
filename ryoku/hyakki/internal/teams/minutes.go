package teams

import (
	"math"
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

// armMinutesLocked sets the one timer of a task's run to its effective
// team/seat limit. Time waiting on the user does not count. Dream tasks
// inherit no team time limit but still obey explicit seat caps. Caller holds mu.
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
	if !ok || !taskIsOpen(t) || t.Waiting == protocol.WaitingOnYou || t.Waiting == protocol.WaitingOnPermission {
		return
	}
	team, ok := s.state.team(t.Team)
	member, known := budgetMember(team, t.Member)
	if !ok || !known {
		return
	}
	minutes := effectiveTaskMinutes(team, member)
	if minutes <= 0 {
		return
	}
	left := s.minutesEnd(t, minutes).Sub(s.now())
	token := r.Token
	r.timer = time.AfterFunc(max(left, 0), func() {
		s.startWorker(func() { s.overrun(taskID, token) })
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
	duration := time.Duration(math.MaxInt64)
	if minutes <= 0 {
		duration = 0
	} else if int64(minutes) <= math.MaxInt64/int64(time.Minute) {
		duration = time.Duration(minutes) * time.Minute
	}
	waited := s.waitedOnYou(t, started)
	if waited > time.Duration(math.MaxInt64)-duration {
		return started.Add(time.Duration(math.MaxInt64))
	}
	return started.Add(duration + waited)
}

// waitedOnYou excludes only genuine owner holds, including delegated work.
// Completed descendants remain sources until their earned credit is folded
// into the surviving ancestor before deletion or replacement.
func (s *Service) waitedOnYou(task protocol.TeamTask, started time.Time) time.Duration {
	now := s.now()
	s.state.mu.Lock()
	spans := s.state.ownerWaitSpansLocked(task, now)
	s.state.mu.Unlock()
	for i := range spans {
		if spans[i].From.Before(started) {
			spans[i].From = started
		}
		if spans[i].Until.IsZero() || spans[i].Until.After(now) {
			spans[i].Until = now
		}
		if task.Ended != nil && spans[i].Until.After(*task.Ended) {
			spans[i].Until = *task.Ended
		}
	}
	sort.Slice(spans, func(i, j int) bool { return spans[i].From.Before(spans[j].From) })
	var total time.Duration
	var reached time.Time
	for _, span := range spans {
		if span.From.Before(reached) {
			span.From = reached
		}
		if span.Until.After(span.From) {
			waited := span.Until.Sub(span.From)
			if total > time.Duration(math.MaxInt64)-waited {
				return time.Duration(math.MaxInt64)
			}
			total += waited
			reached = span.Until
		}
	}
	return total
}

// ownerWaitSpansLocked follows task parentage, not roster reporting lines.
// A child predating any ancestor's current start belongs to an older attempt.
// Each source is bounded by its own end; the target's start clips the union.
func (st *state) ownerWaitSpansLocked(task protocol.TeamTask, now time.Time) []protocol.TaskPermissionWait {
	var spans []protocol.TaskPermissionWait
	for _, source := range st.taskSet {
		if task.Ended != nil && source.ID != task.ID {
			// A terminal task's earned union was frozen at publication.
			// Later descendant recovery cannot revise that ended attempt.
			continue
		}
		if source.Team != task.Team {
			continue
		}
		current := source
		for remaining := len(st.taskSet); current.ID != task.ID && current.Parent != "" && remaining > 0; remaining-- {
			parent, ok := st.taskSet[current.Parent]
			if current.Parent == task.ID {
				parent, ok = task, true
			}
			if !ok || parent.Team != task.Team || parent.Started == nil || current.Created.Before(*parent.Started) {
				break
			}
			current = parent
		}
		if current.ID != task.ID {
			continue
		}
		// Use the caller's snapshot for the task itself (e.g. a restored task).
		if source.ID == task.ID {
			source = task
		}
		until := now
		if source.Ended != nil && source.Ended.Before(until) {
			until = *source.Ended
		}
		add := func(span protocol.TaskPermissionWait) {
			if at, bounded := st.waitEnds[source.ID]; span.Until.IsZero() && bounded {
				// A failed closure retains earned time, but not an unlimited
				// open hold after execution was revoked.
				span.Until = at
			}
			if span.Until.IsZero() || span.Until.After(until) {
				span.Until = until
			}
			if span.Until.After(span.From) {
				spans = append(spans, span)
			}
		}
		for _, span := range source.PermissionWaits {
			add(span)
		}
		for _, rec := range st.approvalSet {
			if rec.Task != source.ID || rec.Session != source.Session {
				continue
			}
			var to time.Time
			if rec.Decided != nil {
				to = *rec.Decided
			}
			add(protocol.TaskPermissionWait{From: rec.Created, Until: to})
		}
	}
	return spans
}

// appendClosedWaits compacts earned owner-wait history without copying an open
// hold. Recovery closes unknown native holds; pre-session strategy holds remain
// open until the owner explicitly supplies the required plan.
func appendClosedWaits(task *protocol.TeamTask, spans []protocol.TaskPermissionWait) {
	var open, closed []protocol.TaskPermissionWait
	for _, span := range task.PermissionWaits {
		if span.Until.IsZero() {
			open = append(open, span)
		} else {
			closed = append(closed, span)
		}
	}
	for _, span := range spans {
		if !span.Until.IsZero() && span.Until.After(span.From) {
			closed = append(closed, span)
		}
	}
	sort.Slice(closed, func(i, j int) bool { return closed[i].From.Before(closed[j].From) })
	merged := closed[:0]
	for _, span := range closed {
		if len(merged) != 0 && !span.From.After(merged[len(merged)-1].Until) {
			if span.Until.After(merged[len(merged)-1].Until) {
				merged[len(merged)-1].Until = span.Until
			}
		} else {
			merged = append(merged, span)
		}
	}
	task.PermissionWaits = append(merged, open...)
}

// preserveAncestorWaitsLocked runs before a source's history is destroyed.
// Only closed, lifetime-clipped spans are copied; no source record is required
// afterwards, and runAgain clears credit before a fresh attempt starts.
func (st *state) preserveAncestorWaitsLocked(source protocol.TeamTask) error {
	until := source.Updated
	if source.Ended != nil {
		until = *source.Ended
	}
	seen := map[string]bool{source.ID: true}
	for source.Parent != "" && !seen[source.Parent] {
		parent, ok := st.taskSet[source.Parent]
		if !ok || parent.Team != source.Team || parent.Started == nil || source.Created.Before(*parent.Started) {
			break
		}
		seen[parent.ID] = true
		spans := st.ownerWaitSpansLocked(parent, until)
		for i := range spans {
			if spans[i].From.Before(*parent.Started) {
				spans[i].From = *parent.Started
			}
			if parent.Ended != nil && spans[i].Until.After(*parent.Ended) {
				spans[i].Until = *parent.Ended
			}
		}
		appendClosedWaits(&parent, spans)
		if err := st.writeTaskLocked(parent); err != nil {
			return err
		}
		source = parent
	}
	return nil
}

func (s *Service) armLineageMinutesLocked(task protocol.TeamTask) {
	seen := map[string]bool{}
	for !seen[task.ID] {
		seen[task.ID] = true
		s.armMinutesLocked(task.ID)
		parent, ok := s.state.task(task.Parent)
		if !ok || parent.Team != task.Team {
			return
		}
		task = parent
	}
}

func taskHasOpenOwnerWait(task protocol.TeamTask) bool {
	if task.Waiting == protocol.WaitingOnYou || task.Waiting == protocol.WaitingOnPermission {
		return true
	}
	for _, span := range task.PermissionWaits {
		if span.Until.IsZero() {
			return true
		}
	}
	return false
}

func closePermissionWait(task *protocol.TeamTask, at time.Time) {
	for i := range task.PermissionWaits {
		if task.PermissionWaits[i].Until.IsZero() {
			task.PermissionWaits[i].Until = at
		}
	}
}

// overrun ends a task that worked past its minutes per task as blocked,
// with the pieces it still waits on. One that waits on the user, or whose
// waiting moved its end, is left to its next timer.
func (s *Service) overrun(taskID, token string) {
	var team protocol.Team
	var member protocol.Member
	var minutes int
	rearm := false
	ended, ok := s.endTaskIf(taskID, func(task protocol.TeamTask) bool {
		r := s.runs[taskID]
		if r == nil || r.Token != token || s.taskEdits[taskID] || task.Waiting == protocol.WaitingOnYou || task.Waiting == protocol.WaitingOnPermission {
			return false
		}
		var known bool
		team, known = s.state.team(task.Team)
		if !known {
			return false
		}
		member, known = protocol.MemberByID(team.Members, task.Member)
		if !known {
			return false
		}
		minutes = effectiveTaskMinutes(team, member)
		if minutes <= 0 {
			return false
		}
		if s.now().Before(s.minutesEnd(task, minutes)) {
			rearm = true
			return false
		}
		return true
	}, func(task *protocol.TeamTask) {
		task.Status, task.Reason = protocol.TaskBlocked, i18n.Tf("It worked past its %d minutes.", minutes)
		if minutes == 1 {
			task.Reason = i18n.T("It worked past its one minute.")
		}
	})
	if !ok {
		if rearm {
			s.armMinutes(taskID)
		}
		return
	}
	s.halt(ended.Session)
	seatDeadline := member.Budget != nil && member.Budget.TaskMinutes > 0 &&
		(isDream(team) || team.Budget.TaskMinutes <= 0 || member.Budget.TaskMinutes < team.Budget.TaskMinutes)
	if !seatDeadline {
		for _, piece := range s.state.tasks(func(p protocol.TeamTask) bool { return p.Parent == taskID && taskIsOpen(p) }) {
			_, _ = s.stopTask(s.background, piece.ID, i18n.T("The lead's task ran out of time."))
		}
	}
	s.record(protocol.Activity{Actor: "Hyakki", Action: "task.block", Target: ended.Title, OK: false, Team: ended.Team, Task: ended.ID, Initiator: protocol.InitiatorRashin, Detail: ended.Reason})
	s.notifyEnd(ended)
	s.afterEnd(ended)
}
