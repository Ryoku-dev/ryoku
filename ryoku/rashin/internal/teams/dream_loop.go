package teams

import (
	"context"
	"encoding/json"
	"strings"

	i18n "ryoku-i18n"

	"ryoku-rashin/internal/protocol"
)

// stallTurns is how many turns in a row a dream's manager may take without
// a tool call before the dream stops to ask the user whether to go on.
const stallTurns = 6

// noProgress is the reason the session manager pauses a goal with when two
// turns in a row called no tool.
func noProgress() string { return i18n.T("Two turns in a row made no progress.") }

// dreamTurn counts the turns in a row the manager of a dream took without a
// tool call, from its standing task's turns. At stallTurns the dream asks the
// user whether to go on, with the manager's last words, and waits.
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
	s.mu.Unlock()
	if !stalled || s.dreamCardOpen(t.ID, cardStall) {
		return
	}
	s.askStall(team, t, info)
}

// askStall puts the stall guard's question to the user: keep going, or
// pause the dream.
func (s *Service) askStall(team protocol.Team, t protocol.TeamTask, info protocol.SessionInfo) {
	words := strings.TrimSpace(info.Preview)
	if words == "" {
		words = i18n.T("The manager said nothing.")
	}
	asked := protocol.Approval{
		Team: team.ID, Kind: protocol.ApprovalAction, Title: i18n.Tf("%s has stalled: keep going?", team.Name),
		Detail:  i18n.Tf("The manager took %d turns in a row without doing anything. Its last words: %s", stallTurns, words),
		Options: []string{i18n.T("Keep going"), i18n.T("Pause the dream")},
	}
	c := caller{Team: team.ID, Task: t.ID, Member: t.Member, Session: t.Session}
	if _, err := s.ask(s.background, c, asked, dreamCard{Dream: cardStall}); err == nil {
		s.record(protocol.Activity{Actor: "Rashin", Action: "dream.stall", Target: team.Name, OK: true, Team: team.ID, Task: t.ID, Initiator: protocol.InitiatorRashin, Detail: asked.Detail})
	}
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

// dreamGoalEnded keeps a dream running where a team task would end, and
// reports whether it did. A dream's task is exempt from the turns per
// task, so a spent turn budget goes on. Its standing task ends only when
// the user accepts a presentation: a goal the manager completes is set
// again, a pause for idle turns goes on until the stall guard asks, and a
// blocker becomes a question to the user. Anything else, a failed turn or
// the user's own stop, ends the task as for any team.
func (s *Service) dreamGoalEnded(info protocol.SessionInfo, goal protocol.Goal) bool {
	t, ok := s.state.task(info.TeamTask)
	if !ok || !taskIsOpen(t) || t.Session != info.ID {
		return false
	}
	team, ok := s.state.team(t.Team)
	if !ok || !isDream(team) || team.Dream.Done != nil {
		return false
	}
	if goal.Status == protocol.GoalStopped && goal.MaxTurns > 0 && goal.Turns >= goal.MaxTurns {
		go s.ensureGoal(t.ID)
		return true
	}
	if !standing(team, t) {
		return false
	}
	switch {
	case goal.Status == protocol.GoalDone:
		go s.ensureGoal(t.ID)
		return true
	case goal.Status == protocol.GoalPaused && goal.Reason == noProgress():
		if s.dreamCardOpen(t.ID, cardStall) {
			return true
		}
		go s.ensureGoal(t.ID)
		return true
	case goal.Status == protocol.GoalBlocked:
		reason := strings.TrimSpace(goal.Reason)
		if reason == "" {
			reason = i18n.T("The manager needs you to go on.")
		}
		title, _, _ := strings.Cut(reason, "\n")
		asked := protocol.Approval{Team: team.ID, Kind: protocol.ApprovalAction, Title: clipRunes(title, 200), Detail: reason}
		c := caller{Team: team.ID, Task: t.ID, Member: t.Member, Session: t.Session}
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
	info, ok := s.sessions.Info(t.Session)
	if !ok || info.Goal == nil {
		return
	}
	switch info.Goal.Status {
	case protocol.GoalActive:
		return
	case protocol.GoalDone:
		_, _ = s.sessions.Goal(s.background, protocol.GoalParams{ID: t.Session, Op: protocol.GoalSet, Objective: objective(t), MaxTurns: t.MaxTurns})
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
	case cardStall:
		if first {
			s.resetIdle(rec.Task)
			s.ensureGoal(rec.Task)
			text := i18n.T("The user says keep going. Take the next concrete step toward the dream now, with a tool.")
			if note != "" {
				text += "\n" + i18n.Tf("Their note: %s", note)
			}
			return text, nil
		}
		if _, err := s.pauseTeam(ctx, protocol.TeamPauseParams{ID: rec.Team, Paused: true}); err != nil {
			return "", err
		}
		rec.Result = i18n.T("The dream is paused.")
		return "", nil
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
