package teams

import (
	"context"
	"strings"

	i18n "ryoku-i18n"

	"ryoku-rashin/internal/config"
	"ryoku-rashin/internal/protocol"
)

// effectAction tells the agent how the user answered what it asked before
// acting, or which option they chose; the agent itself carries the action
// out once approved.
func (s *Service) effectAction(_ context.Context, rec *approvalRecord, p protocol.ApprovalDecideParams) (string, error) {
	var text string
	switch p.Decision {
	case protocol.DecideChoose:
		text = i18n.Tf("The user chose: %s", p.Choice)
	case protocol.DecideDeny:
		text = i18n.Tf("The user said no: %s.", strings.TrimSuffix(rec.Title, "."))
	default:
		text = i18n.Tf("The user approved: %s.", strings.TrimSuffix(rec.Title, "."))
	}
	if note := strings.TrimSpace(p.Note); note != "" {
		text += "\n" + i18n.Tf("Their note: %s", note)
	}
	return text, nil
}

// effectBudget answers a team that reached its spending limit: approving
// raises the limit by half and lets the team work again; denying keeps it
// paused.
func (s *Service) effectBudget(_ context.Context, rec *approvalRecord, p protocol.ApprovalDecideParams) (string, error) {
	if p.Decision == protocol.DecideDeny {
		rec.Result = i18n.T("No budget change was made.")
		return rec.Result, nil
	}
	s.mu.Lock()
	team, ok := s.state.team(rec.Team)
	if !ok {
		s.mu.Unlock()
		return "", protocol.Errorf(protocol.CodeNotFound, "%s", i18n.Tf("No team %s.", rec.Team))
	}
	if !team.Paused || team.PauseReason != protocol.PausedByBudget || team.Budget.MonthTokens <= 0 {
		s.mu.Unlock()
		rec.Result = "This budget card no longer applies to the current team settings."
		return rec.Result, nil
	}
	used, err := s.budgetTokens(rec.Team, "", s.now())
	if err != nil {
		s.mu.Unlock()
		return "", protocol.Errorf(protocol.CodeConflict, "Usage is incomplete or unavailable. The capped team stays paused.")
	}
	raised, err := checkedBudgetRaise(team.Budget.MonthTokens, used)
	if err != nil {
		s.mu.Unlock()
		return "", err
	}
	budget := team.Budget
	budget.MonthTokens = raised
	if err := config.CheckBudget(budget); err != nil {
		s.mu.Unlock()
		return "", err
	}
	team.Budget, team.Paused, team.PauseReason = budget, false, ""
	team.Updated = s.now()
	err = s.state.putTeam(team)
	s.mu.Unlock()
	if err != nil {
		return "", err
	}
	s.emitTeam(team.ID)
	s.record(protocol.Activity{Actor: protocol.InitiatorYou, Action: "team.resume", Target: team.Name, OK: true, Team: team.ID, Initiator: protocol.InitiatorYou, Detail: i18n.Tf("The spending limit is now %d tokens a month.", team.Budget.MonthTokens)})
	s.clock.rearm()
	s.pump()
	s.runDream(team.ID)
	rec.Result = i18n.Tf("The spending limit is now %d tokens a month.", team.Budget.MonthTokens)
	return rec.Result, nil
}
