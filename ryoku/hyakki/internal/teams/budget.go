package teams

import (
	"errors"
	"math"

	i18n "ryoku-i18n"

	"ryoku-rashin/internal/protocol"
)

// defaultWarnPercent is when the user hears a team nears its limit, for a
// budget that names no percent.
const defaultWarnPercent = 80

// Unknown external-runtime usage cannot authorize another token-capped turn.
// Known measured tokens remain reportable separately from this uncertainty.
var errUnmeteredBudget = errors.New("monthly token accounting includes unmetered runtime usage; a token cap cannot be enforced")

// checkBudget follows a team's monthly tokens after a turn that used spent
// of them: crossing the warn mark tells the user once, reaching the limit
// pauses the team and asks whether to raise it.
func (s *Service) checkBudget(teamID string, spent int64) {
	team, ok := s.state.team(teamID)
	if !ok || team.Paused || team.Budget.MonthTokens <= 0 {
		return
	}
	limit := team.Budget.MonthTokens
	used, err := s.budgetTokens(team.ID, "", s.now())
	if err != nil || used >= limit {
		s.stopForBudget(team.ID, used)
		return
	}
	percent := budgetWarnPercent(team.Budget.WarnPercent)
	mark := budgetWarnMark(limit, percent)
	if spent <= 0 || used < mark || (used >= spent && used-spent >= mark) {
		return
	}
	detail := i18n.Tf("%d of %d tokens this month.", used, limit)
	s.record(protocol.Activity{Actor: "Hyakki", Action: "budget.warn", Target: team.Name, OK: true, Team: team.ID, Initiator: protocol.InitiatorRashin, Detail: detail})
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
	used, accountingErr := s.budgetTokens(team.ID, "", s.now())
	if team.Budget.MonthTokens <= 0 || accountingErr == nil && used < team.Budget.MonthTokens {
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
	reason := i18n.T("The team reached its spending limit.")
	if accountingErr != nil {
		reason = i18n.T("The team's token accounting is incomplete or unavailable.")
	}
	for _, task := range s.state.tasks(func(task protocol.TeamTask) bool { return task.Team == team.ID && taskIsOpen(task) }) {
		ended, stopped := s.endTaskIf(task.ID, func(current protocol.TeamTask) bool {
			latest, exists := s.state.team(team.ID)
			return exists && latest.Paused && latest.PauseReason == protocol.PausedByBudget && latest.Budget.MonthTokens == team.Budget.MonthTokens &&
				current.Session == task.Session && current.Member == task.Member && !s.taskEdits[current.ID]
		}, func(current *protocol.TeamTask) {
			current.Status, current.Reason = protocol.TaskStopped, reason
		})
		if stopped {
			// This may run inside the session's own AfterTurn callback. Revoke
			// immediately, but never wait here for that callback's turnDone.
			s.halt(ended.Session)
			s.denyPending(ended.ID, reason)
			s.afterEnd(ended)
		}
	}
	limit := team.Budget.MonthTokens
	detail := i18n.Tf("%d of %d tokens this month.", used, limit)
	if accountingErr != nil {
		detail = i18n.T("The team's token accounting is incomplete or unavailable. It stays paused until accounting is available.")
	}
	s.record(protocol.Activity{Actor: "Hyakki", Action: "budget.stop", Target: team.Name, OK: accountingErr == nil, Team: team.ID, Initiator: protocol.InitiatorRashin, Detail: detail})
	asked := protocol.Approval{Team: team.ID, Kind: protocol.ApprovalBudget, Title: i18n.Tf("%s reached its spending limit", team.Name), Detail: detail}
	_, _ = s.ask(s.background, caller{Team: team.ID}, asked, nil)
}

func budgetWarnPercent(percent int) int {
	if percent <= 0 || percent >= 100 {
		return defaultWarnPercent
	}
	return percent
}

// budgetWarnMark rounds upward to the first whole token at the percentage.
// Dividing before multiplying avoids overflow even at the int64 ceiling.
func budgetWarnMark(limit int64, percent int) int64 {
	if limit <= 0 {
		return 0
	}
	return limit/100*int64(percent) + (limit%100*int64(percent)+99)/100
}

func saturatedTokens(a, b int64) int64 {
	if a < 0 || b < 0 || a > math.MaxInt64-b {
		return math.MaxInt64
	}
	return a + b
}
