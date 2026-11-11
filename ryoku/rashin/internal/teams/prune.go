package teams

import (
	"context"
	"errors"
	"time"

	i18n "ryoku-i18n"

	"ryoku-rashin/internal/protocol"
)

// pruneEvery is how often finished tasks past the kept days are deleted.
const pruneEvery = 24 * time.Hour

// pruneDaily deletes the finished tasks past the kept days now, then once a
// day until the service closes.
func (s *Service) pruneDaily() {
	s.pruneTasks(s.background, protocol.TeamPruneParams{}, protocol.InitiatorRashin)
	go func() {
		ticker := time.NewTicker(pruneEvery)
		defer ticker.Stop()
		for {
			select {
			case <-s.background.Done():
				return
			case <-ticker.C:
				s.pruneTasks(s.background, protocol.TeamPruneParams{}, protocol.InitiatorRashin)
			}
		}
	}()
}

// prune serves teams.prune for the user.
func (s *Service) prune(ctx context.Context, p protocol.TeamPruneParams) (protocol.Empty, error) {
	if p.Team != "" {
		if _, ok := s.state.team(p.Team); !ok {
			return protocol.Empty{}, protocol.Errorf(protocol.CodeNotFound, "%s", i18n.Tf("No team %s.", p.Team))
		}
	}
	s.pruneTasks(ctx, p, protocol.InitiatorYou)
	return protocol.Empty{}, nil
}

// pruneTasks deletes finished tasks with their sessions: those that ended
// more than the settings' kept days ago, or every finished one with All, of
// one team or of all. A piece whose lead still works stays for the lead's
// report, and a task whose session could not be deleted stays for the next
// prune. What the tasks made stays in the team's folder. Each team that
// lost tasks gets one ledger entry.
func (s *Service) pruneTasks(ctx context.Context, p protocol.TeamPruneParams, initiator string) {
	if s.sessions == nil {
		return
	}
	days := s.settings().Teams.KeepDays
	if !p.All && days <= 0 {
		return
	}
	cutoff := s.now().AddDate(0, 0, -days)
	finished := s.state.tasks(func(t protocol.TeamTask) bool {
		if taskIsOpen(t) || (p.Team != "" && t.Team != p.Team) {
			return false
		}
		ended := t.Updated
		if t.Ended != nil {
			ended = *t.Ended
		}
		return p.All || ended.Before(cutoff)
	})
	counts := make(map[string]int)
	for _, t := range finished {
		if lead, ok := s.state.task(t.Parent); ok && taskIsOpen(lead) {
			continue
		}
		if !s.deleteSessions(ctx, t.Sessions) || s.state.removeTask(t.ID) != nil {
			continue
		}
		s.broadcast(protocol.EvtTeamTaskRemoved, protocol.TeamRemoved{ID: t.ID})
		counts[t.Team]++
	}
	actor := "Rashin"
	if initiator == protocol.InitiatorYou {
		actor = protocol.InitiatorYou
	}
	for teamID, count := range counts {
		team, ok := s.state.team(teamID)
		if !ok {
			continue
		}
		detail := i18n.Tf("Deleted %d finished tasks with their conversations; what they made stays in the folder.", count)
		if count == 1 {
			detail = i18n.T("Deleted one finished task with its conversation; what it made stays in the folder.")
		}
		s.record(protocol.Activity{Actor: actor, Action: "task.prune", Target: team.Name, OK: true, Team: team.ID, Initiator: initiator, Detail: detail})
		s.emitTeam(team.ID)
	}
}

// deleteSessions deletes a pruned task's sessions; one already gone counts
// as deleted.
func (s *Service) deleteSessions(ctx context.Context, sessions []string) bool {
	for _, session := range sessions {
		err := s.sessions.Delete(ctx, session)
		var failure *protocol.Error
		if err != nil && !(errors.As(err, &failure) && failure.Code == protocol.CodeNotFound) {
			return false
		}
	}
	return true
}
