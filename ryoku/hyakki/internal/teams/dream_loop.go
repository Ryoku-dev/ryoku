package teams

import (
	"context"
	"encoding/json"
	"strings"

	i18n "ryoku-i18n"

	"ryoku-rashin/internal/protocol"
)

// stallTurns bounds a manager that never uses a tool, without asking the
// CEO for a generic "keep going" confirmation.
const stallTurns = 6

// noProgress is the reason the session manager pauses a goal with when two
// turns in a row called no tool.
func noProgress() string { return i18n.T("Two turns in a row made no progress.") }

// dreamTurn stops a manager after six turns without a tool call. A turn
// with work starts the count again; the CEO can resume the blocked dream.
func (s *Service) dreamTurn(info protocol.SessionInfo) {
	t, ok := s.state.task(info.TeamTask)
	if !ok || !taskIsOpen(t) || t.Session != info.ID {
		return
	}
	team, ok := s.state.team(t.Team)
	if !ok || !standing(team, t) {
		return
	}
	s.mu.Lock()
	r := s.runs[t.ID]
	if r == nil || r.Session != info.ID {
		s.mu.Unlock()
		return
	}
	if info.Steps > 0 {
		r.idle = 0
	} else {
		r.idle++
	}
	stalled := r.idle >= stallTurns
	token := r.Token
	s.mu.Unlock()
	if !stalled {
		return
	}
	reason := i18n.Tf("The manager took %d turns without making progress. Review its last words and resume the dream to try again.", stallTurns)
	if words := strings.TrimSpace(info.Preview); words != "" {
		reason += " " + i18n.Tf("Its last words: %s", s.redact(words))
	}
	ended, ok := s.endTaskIf(t.ID, func(task protocol.TeamTask) bool {
		current := s.runs[task.ID]
		return task.Session == info.ID && current != nil && current.Token == token && !s.taskEdits[task.ID]
	}, func(task *protocol.TeamTask) {
		task.Status, task.Reason = protocol.TaskBlocked, reason
	})
	if !ok {
		return
	}
	s.record(protocol.Activity{Actor: "Hyakki", Action: "dream.stall", Target: team.Name, OK: false, Team: team.ID, Task: t.ID, Initiator: protocol.InitiatorRashin, Detail: reason})
	s.notifyEnd(ended)
	s.afterEnd(ended)
}

// resetIdle starts the count of idle turns again: the manager did something
// the user can see, or the user answered it.
func (s *Service) resetIdle(task string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if r := s.runs[task]; r != nil {
		r.idle = 0
	}
}

// dreamGoalEnded keeps a dream running where a team task would end. Its
// standing task ends only when the CEO accepts a presentation; an exhausted
// turn budget starts another goal, and the no-progress guard keeps trying
// until dreamTurn reaches its bounded stall limit.
func (s *Service) dreamGoalEnded(info protocol.SessionInfo, goal protocol.Goal) bool {
	t, ok := s.state.task(info.TeamTask)
	if !ok || !taskIsOpen(t) || t.Session != info.ID {
		return false
	}
	team, ok := s.state.team(t.Team)
	if !ok || !isDream(team) || !s.dreamAvailable(team) || team.Dream.Done != nil {
		return false
	}
	member, exists := protocol.MemberByID(team.Members, t.Member)
	if !exists || taskTurnsExhausted(team, member, t) || member.Budget != nil && member.Budget.TaskTurns > 0 && goal.MaxTurns > 0 && goal.Turns >= goal.MaxTurns {
		return false
	}
	if goal.Status == protocol.GoalStopped && goal.MaxTurns > 0 && goal.Turns >= goal.MaxTurns {
		s.startWorker(func() { s.ensureGoal(t.ID) })
		return true
	}
	if !standing(team, t) {
		return false
	}
	switch {
	case goal.Status == protocol.GoalDone:
		s.startWorker(func() { s.ensureGoal(t.ID) })
		return true
	case goal.Status == protocol.GoalPaused && goal.Reason == noProgress():
		s.startWorker(func() { s.ensureGoal(t.ID) })
		return true
	case goal.Status == protocol.GoalBlocked:
		reason := strings.TrimSpace(goal.Reason)
		if reason == "" {
			return false
		}
		title, _, _ := strings.Cut(reason, "\n")
		asked := protocol.Approval{Team: team.ID, Kind: protocol.ApprovalAction, Title: clipRunes(title, 200), Detail: reason}
		c := caller{Team: team.ID, Task: t.ID, Member: t.Member, Session: t.Session}
		s.mu.Lock()
		if run := s.runs[t.ID]; run != nil {
			c.Token = run.Token
		}
		s.mu.Unlock()
		_, err := s.ask(s.background, c, asked, dreamCard{Dream: cardBlocked})
		return err == nil
	}
	return false
}

// ensureGoal makes a dream task's goal active again where it stopped
// needing the agent: set again when it was completed, resumed otherwise.
// A held task starts no turn by it; one that is not held goes on at once.
func (s *Service) ensureGoal(taskID string) {
	t, ok := s.state.task(taskID)
	if !ok || !taskIsOpen(t) || t.Session == "" || s.sessions == nil {
		return
	}
	team, known := s.state.team(t.Team)
	member, exists := protocol.MemberByID(team.Members, t.Member)
	if !known || !exists || !s.dreamAvailable(team) || taskTurnsExhausted(team, member, t) || !s.live(t.ID, t.Session) {
		return
	}
	info, ok := s.sessions.Info(t.Session)
	if !ok || info.Goal == nil {
		return
	}
	switch info.Goal.Status {
	case protocol.GoalActive:
		return
	case protocol.GoalDone:
		_, _ = s.sessions.Goal(s.background, protocol.GoalParams{ID: t.Session, Op: protocol.GoalSet, Objective: objective(t), MaxTurns: taskGoalTurns(team, member, t)})
	default:
		_, _ = s.sessions.Goal(s.background, protocol.GoalParams{ID: t.Session, Op: protocol.GoalResume})
	}
}

// dreamCardOpen reports a question of kind the task still waits on.
func (s *Service) dreamCardOpen(task, kind string) bool {
	return len(s.state.approvals(func(r approvalRecord) bool {
		return r.Task == task && r.Status == protocol.ApprovalPending && cardOf(r) == kind
	})) > 0
}

// cardOf is the kind of dream question an approval is, empty for any other.
func cardOf(rec approvalRecord) string {
	if len(rec.Action) == 0 {
		return ""
	}
	var card dreamCard
	if json.Unmarshal(rec.Action, &card) != nil {
		return ""
	}
	return card.Dream
}

// effectAsked carries out the answer to a question an agent asked: a dream's
// own questions as the dream's, any other as effectAction tells it.
func (s *Service) effectAsked(ctx context.Context, rec *approvalRecord, p protocol.ApprovalDecideParams) (string, error) {
	first := p.Decision == protocol.DecideChoose && len(rec.Options) > 0 && p.Choice == rec.Options[0]
	note := strings.TrimSpace(p.Note)
	switch cardOf(*rec) {
	case cardPresent:
		if first {
			if err := s.acceptDream(rec.Team, note); err != nil {
				return "", err
			}
			rec.Result = i18n.T("Accepted: the dream is done.")
			return "", nil
		}
		if note == "" {
			note = i18n.T("The user did not accept it yet and left no note. Find what is missing, finish it, test it and present again.")
		}
		s.ensureGoal(rec.Task)
		text, err := s.dreamNotes(rec.Team, note)
		rec.Result = i18n.T("Sent back with notes.")
		return text, err
	case cardBlocked:
		s.resetIdle(rec.Task)
		s.ensureGoal(rec.Task)
	}
	return s.effectAction(ctx, rec, p)
}

// dreamReport refuses the report that would end a dream's standing task: a
// dream ends only when the user accepts what its manager presents.
func (s *Service) dreamReport(c caller) error {
	team, ok := s.state.team(c.Team)
	if !ok {
		return nil
	}
	if t, ok := s.state.task(c.Task); ok && standing(team, t) {
		return protocol.Errorf(protocol.CodeConflict, "%s", i18n.T("A dream does not end on a report. Say where it stands with dream_status, and present it once it is built and tested."))
	}
	return nil
}

// notesText is what the manager is continued with when the user sends a
// presentation back.
func notesText(title, note string) string {
	return i18n.Tf("The user sent notes on your presentation \"%s\":\n%s\nWork them in with your team, test again, and present again when it is ready.", title, note)
}
