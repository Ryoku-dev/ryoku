package teams

import (
	i18n "ryoku-i18n"

	"ryoku-rashin/internal/protocol"
)

// defaultWarnPercent is when the user hears a team nears its limit, for a
// budget that names no percent.
const defaultWarnPercent = 80

// checkBudget follows a team's monthly tokens after a turn that used spent
// of them: crossing the warn mark tells the user once, reaching the limit
// pauses the team and asks whether to raise it.
func (s *Service) checkBudget(teamID string, spent int64) {
	team, ok := s.state.team(teamID)
	if !ok || team.Paused || team.Budget.MonthTokens <= 0 {
		return
	}
	limit := team.Budget.MonthTokens
	used := s.monthTokens(team.ID, s.now())
	if used >= limit {
		s.stopForBudget(team.ID, used)
		return
	}
	percent := team.Budget.WarnPercent
	if percent <= 0 || percent >= 100 {
		percent = defaultWarnPercent
	}
	mark := limit * int64(percent) / 100
	if used < mark || used-spent >= mark {
		return
	}
	detail := i18n.Tf("%d of %d tokens this month.", used, limit)
	s.record(protocol.Activity{Actor: "Rashin", Action: "budget.warn", Target: team.Name, OK: true, Team: team.ID, Initiator: protocol.InitiatorRashin, Detail: detail})
	s.notify(notification{Team: team.ID, Title: i18n.Tf("%s used %d%% of its spending limit", team.Name, percent), Body: detail})
}

// stopForBudget pauses a team at its spending limit, stops its tasks and
// asks the user to raise the limit or keep it paused.
func (s *Service) stopForBudget(teamID string, used int64) {
	s.mu.Lock()
	team, ok := s.state.team(teamID)
	if !ok || team.Paused {
		s.mu.Unlock()
		return
	}
	team.Paused, team.PauseReason, team.Updated = true, protocol.PausedByBudget, s.now()
	err := s.state.putTeam(team)
	s.mu.Unlock()
	if err != nil {
		return
	}
	s.emitTeam(team.ID)
	s.clock.rearm()
	s.stopTeamTasks(team.ID, i18n.T("The team reached its spending limit."))
	limit := team.Budget.MonthTokens
	detail := i18n.Tf("%d of %d tokens this month.", used, limit)
	s.record(protocol.Activity{Actor: "Rashin", Action: "budget.stop", Target: team.Name, OK: true, Team: team.ID, Initiator: protocol.InitiatorRashin, Detail: detail})
	asked := protocol.Approval{Team: team.ID, Kind: protocol.ApprovalBudget, Title: i18n.Tf("%s reached its spending limit", team.Name), Detail: detail}
	_, _ = s.ask(s.background, caller{Team: team.ID}, asked, nil)
}
