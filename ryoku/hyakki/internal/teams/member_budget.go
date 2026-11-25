package teams

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"time"

	i18n "ryoku-i18n"

	"ryoku-rashin/internal/protocol"
)

// memberBudgetReady checks current membership rather than trusting a queued
// task's old roster. Accounting errors never give a capped seat free work.
func (s *Service) memberBudgetReady(team protocol.Team, member protocol.Member) bool {
	if s.state.writer.available() != nil {
		return false
	}
	current, ok := s.state.team(team.ID)
	if !ok || current.Paused {
		return false
	}
	member, ok = budgetMember(current, member.ID)
	if !ok || member.Paused {
		return false
	}
	if current.Budget.MonthTokens > 0 {
		used, err := s.budgetTokens(current.ID, "", s.now())
		if err != nil || used >= current.Budget.MonthTokens {
			return false
		}
	}
	if member.Budget == nil || member.Budget.MonthTokens <= 0 {
		return true
	}
	used, err := s.budgetTokens(current.ID, member.ID, s.now())
	return err == nil && used < member.Budget.MonthTokens
}

// memberBudgetAction snapshots the cap that the Summoner is being asked to
// raise. An old approval cannot raise a subsequently edited seat's budget.
type memberBudgetAction struct {
	Month string `json:"month"`
	Limit int64  `json:"limit"`
}

// checkMemberBudget follows durable monthly member totals, not spent (which
// may be a duplicate turn callback). The persisted warning marker and pause
// transaction make concurrent callbacks and daemon restarts idempotent.
func (s *Service) checkMemberBudget(teamID, memberID string, spent int64) {
	if memberID == "" {
		return
	}
	unwatched := s.nobodyWatching()
	s.mu.Lock()
	team, ok := s.state.team(teamID)
	member, known := budgetMember(team, memberID)
	if !ok || !known || member.Budget == nil || member.Budget.MonthTokens <= 0 {
		s.mu.Unlock()
		return
	}
	now := s.now()
	used, accountingErr := s.budgetTokens(teamID, memberID, now)
	limit := member.Budget.MonthTokens
	observed := member
	if accountingErr != nil || used >= limit {
		if member.Paused && member.PauseReason != protocol.PausedByBudget {
			s.mu.Unlock()
			s.stopMemberBudgetTasks(teamID, observed)
			return
		}
		if member.Paused {
			s.mu.Unlock()
			s.stopMemberBudgetTasks(teamID, observed)
			return
		}
		detail := i18n.Tf("%d of %d tokens this month.", used, limit)
		if accountingErr != nil {
			detail = i18n.T("The member's token accounting is incomplete or unavailable. It stays paused until accounting is available.")
		}
		action, _ := json.Marshal(memberBudgetAction{Month: now.Format(monthLayout), Limit: limit})
		approval := approvalRecord{Approval: protocol.Approval{
			ID: newID("approval"), Team: teamID, Member: memberID,
			Kind: protocol.ApprovalMemberBudget, Status: protocol.ApprovalPending,
			Title:  s.redact(i18n.Tf("%s reached its spending limit", member.Name)),
			Detail: s.redact(detail), Created: now, Unwatched: unwatched,
		}, Action: action}
		member.Paused, member.PauseReason = true, protocol.PausedByBudget
		if member.Revision == math.MaxUint64 {
			s.mu.Unlock()
			s.stopMemberBudgetTasks(teamID, observed)
			return
		}
		member.Revision++
		for i := range team.Members {
			if team.Members[i].ID == memberID {
				team.Members[i] = member
			}
		}
		team.Updated = now
		err := s.state.putMemberBudgetPause(team, approval)
		if err == nil {
			observed = member
		}
		s.mu.Unlock()
		// Even a failed state write must halt ongoing spend. Readiness continues
		// to reject the capped or unreadable account without the pause flag.
		s.stopMemberBudgetTasks(teamID, observed)
		if err != nil {
			return
		}
		s.emitTeam(teamID)
		s.record(protocol.Activity{Actor: "Hyakki", Action: "member.budget.stop", Target: member.Name, OK: accountingErr == nil, Team: teamID, Initiator: protocol.InitiatorRashin, Detail: detail})
		s.record(protocol.Activity{Actor: "Hyakki", Action: "approval.ask", Target: approval.Title, OK: true, Team: teamID, Initiator: protocol.InitiatorRashin, Detail: i18n.T("Waits for your answer.")})
		s.emitApproval(approval.Approval)
		s.notify(notification{Team: teamID, Title: approval.Title, Body: detail, Needs: true})
		return
	}
	if member.Paused {
		s.mu.Unlock()
		return
	}
	percent := budgetWarnPercent(member.Budget.WarnPercent)
	warn, err := s.markMemberBudgetWarning(teamID, memberID, now, limit, budgetWarnMark(limit, percent))
	s.mu.Unlock()
	if err != nil {
		s.checkMemberBudget(teamID, memberID, 0)
		return
	}
	if !warn {
		return
	}
	detail := i18n.Tf("%d of %d tokens this month.", used, limit)
	s.record(protocol.Activity{Actor: "Hyakki", Action: "member.budget.warn", Target: member.Name, OK: true, Team: teamID, Initiator: protocol.InitiatorRashin, Detail: detail})
	s.notify(notification{Team: teamID, Title: i18n.Tf("%s used %d%% of its spending limit", member.Name, percent), Body: detail})
}

// putMemberBudgetPause commits the seat pause and its approval together in
// state.json. The caller holds Service.mu, serializing roster edits; no seat
// is inserted if it was removed. A crash cannot leave a pause without its card.
func (st *state) putMemberBudgetPause(team protocol.Team, approval approvalRecord) error {
	st.mu.Lock()
	defer st.mu.Unlock()
	before, ok := st.teamSet[team.ID]
	if !ok {
		return protocol.Errorf(protocol.CodeNotFound, "No team %s.", team.ID)
	}
	if _, ok := budgetMember(before, approval.Member); !ok {
		return protocol.Errorf(protocol.CodeNotFound, "No member %s.", approval.Member)
	}
	history, err := nextMemberHistory(before, &team, st.memberHistories[team.ID])
	if err != nil {
		return err
	}
	previousHistory := st.memberHistories[team.ID]
	st.memberHistories[team.ID] = history
	st.teamSet[team.ID] = cloneTeam(team)
	st.approvalSet[approval.ID] = cloneApproval(approval)
	if err := st.saveLocked(); err != nil {
		st.memberHistories[team.ID] = previousHistory
		st.teamSet[team.ID] = before
		delete(st.approvalSet, approval.ID)
		return err
	}
	return nil
}

// stopTask recursively stops children, including tasks assigned to other
// seats. A member cap must instead end only this seat's open tasks.
func (s *Service) stopMemberBudgetTasks(teamID string, observed protocol.Member) {
	// The cap event may have waited behind a raise or roster edit. Validate
	// the observed seat generation inside each task's locked end transition,
	// not merely before selecting tasks or before acquiring the task lock.
	allow := func(task protocol.TeamTask) bool {
		return task.Team == teamID && task.Member == observed.ID && s.memberBudgetStopCurrent(teamID, observed)
	}
	for _, task := range s.state.tasks(func(t protocol.TeamTask) bool {
		return t.Team == teamID && t.Member == observed.ID && taskIsOpen(t)
	}) {
		ended, ok := s.endTaskIf(task.ID, allow, func(t *protocol.TeamTask) {
			t.Status, t.Reason = protocol.TaskStopped, i18n.T("The member reached its spending limit.")
		})
		if !ok {
			// A broken task-file write must not leave its agent spending.
			// Revoke the live run even when its stopped status cannot persist.
			s.mu.Lock()
			current, exists := s.state.task(task.ID)
			halt := exists && taskIsOpen(current) && allow(current)
			if halt {
				if run := s.runs[task.ID]; run != nil && run.timer != nil {
					run.timer.Stop()
				}
				delete(s.runs, task.ID)
			}
			s.mu.Unlock()
			if halt {
				s.halt(current.Session)
			}
			continue
		}
		s.halt(ended.Session)
		s.denyPending(ended.ID, i18n.T("The task was stopped."))
		s.record(protocol.Activity{Actor: "Hyakki", Action: "task.stop", Target: ended.Title, OK: true, Team: teamID, Task: ended.ID, Initiator: protocol.InitiatorRashin, Detail: ended.Reason})
		s.afterEnd(ended)
	}
}

// memberBudgetStopCurrent is called only under Service.mu. A persisted pause
// is authoritative for its generation; if persisting the pause failed, the
// same unchanged seat must still be capped or have unreadable accounting.
func (s *Service) memberBudgetStopCurrent(teamID string, observed protocol.Member) bool {
	team, exists := s.state.team(teamID)
	current, known := budgetMember(team, observed.ID)
	if !exists || !known || current.Revision != observed.Revision ||
		current.Paused != observed.Paused || current.PauseReason != observed.PauseReason ||
		current.Budget == nil || observed.Budget == nil ||
		current.Budget.MonthTokens != observed.Budget.MonthTokens || current.Budget.MonthTokens <= 0 {
		return false
	}
	if current.Paused {
		return true
	}
	used, err := s.budgetTokens(teamID, current.ID, s.now())
	return err != nil || used >= current.Budget.MonthTokens
}

// effectMemberBudget accepts an explicit raise or denial, never an implicit
// permanent grant. It changes only a still-present seat at the approved cap.
func (s *Service) effectMemberBudget(_ context.Context, rec *approvalRecord, p protocol.ApprovalDecideParams) (string, error) {
	if p.Decision != protocol.DecideOnce && p.Decision != protocol.DecideDeny {
		return "", protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("Raise this member's limit once or keep it paused."))
	}
	s.mu.Lock()
	team, ok := s.state.team(rec.Team)
	member, known := budgetMember(team, rec.Member)
	if !ok || !known || rec.Member == "" {
		s.mu.Unlock()
		return "", protocol.Errorf(protocol.CodeNotFound, "%s", i18n.Tf("No member %s.", rec.Member))
	}
	if p.Decision == protocol.DecideDeny {
		s.mu.Unlock()
		rec.Result = i18n.T("The member stays paused.")
		return rec.Result, nil
	}
	var action memberBudgetAction
	if json.Unmarshal(rec.Action, &action) != nil || action.Month == "" || action.Limit <= 0 || member.Budget == nil || member.Budget.MonthTokens != action.Limit || !member.Paused || member.PauseReason != protocol.PausedByBudget {
		s.mu.Unlock()
		return "", protocol.Errorf(protocol.CodeConflict, "%s", i18n.T("The member's budget changed since this approval was requested."))
	}
	used, err := s.budgetTokens(team.ID, member.ID, s.now())
	if err != nil {
		s.mu.Unlock()
		return "", err
	}
	raised, err := checkedBudgetRaise(member.Budget.MonthTokens, used)
	if team.Budget.MonthTokens > 0 && raised > team.Budget.MonthTokens {
		raised = team.Budget.MonthTokens
	}
	if err != nil || raised <= used || raised <= member.Budget.MonthTokens || member.Revision == math.MaxUint64 {
		s.mu.Unlock()
		return "", protocol.Errorf(protocol.CodeConflict, "%s", i18n.T("The member's limit cannot be raised within the team's spending limit."))
	}
	budget := *member.Budget
	budget.MonthTokens = raised
	member.Budget = &budget
	member.Paused, member.PauseReason = false, ""
	member.Revision++
	for i := range team.Members {
		if team.Members[i].ID == member.ID {
			team.Members[i] = member
		}
	}
	team.Updated = s.now()
	err = s.state.putTeam(team)
	s.mu.Unlock()
	if err != nil {
		return "", err
	}
	rec.Result = i18n.Tf("The member's spending limit is now %d tokens a month.", raised)
	s.emitTeam(team.ID)
	s.record(protocol.Activity{Actor: protocol.InitiatorYou, Action: "member.resume", Target: member.Name, OK: true, Team: team.ID, Initiator: protocol.InitiatorYou, Detail: rec.Result})
	s.pump()
	s.runDream(team.ID)
	return rec.Result, nil
}

// markMemberBudgetWarning keeps notification deduplication next to the same
// month's member totals, not in a prunable activity or approval history.
func (s *Service) markMemberBudgetWarning(teamID, memberID string, at time.Time, limit, mark int64) (bool, error) {
	st := s.state
	st.spendMu.Lock()
	defer st.spendMu.Unlock()
	file, err := st.spendLocked()
	if err != nil {
		return false, err
	}
	ms := file[at.Format(monthLayout)][teamID]
	if ms == nil || ms.Members[memberID] < mark || ms.MemberWarnings[memberID] == limit {
		return false, nil
	}
	if ms.MemberWarnings == nil {
		ms.MemberWarnings = make(map[string]int64)
	}
	before, had := ms.MemberWarnings[memberID]
	ms.MemberWarnings[memberID] = limit
	err = st.saveSpendLocked(file)
	if err != nil {
		if had {
			ms.MemberWarnings[memberID] = before
		} else {
			delete(ms.MemberWarnings, memberID)
		}
		return false, err
	}
	return true, nil
}

// budgetTokens returns checked totals; absent monthly data is zero, but a
// malformed or overflowed account is never interpreted as an empty month.
func (s *Service) budgetTokens(teamID, memberID string, at time.Time) (int64, error) {
	st := s.state
	st.spendMu.Lock()
	defer st.spendMu.Unlock()
	file, err := st.spendLocked()
	if err != nil {
		return 0, err
	}
	ms := file[at.Format(monthLayout)][teamID]
	if ms == nil {
		return 0, nil
	}
	var total int64
	for _, day := range ms.Days {
		if day.Input < 0 || day.Output < 0 || day.Input > math.MaxInt64-day.Output || total > math.MaxInt64-day.Input-day.Output {
			return 0, errors.New("monthly spending overflow")
		}
		total += day.Input + day.Output
	}
	if memberID == "" {
		if ms.Unmetered {
			return total, errUnmeteredBudget
		}
		return total, nil
	}
	used := ms.Members[memberID]
	if ms.UnmeteredMembers[memberID] || ms.UnmeteredMembers[""] {
		return used, errUnmeteredBudget
	}
	if used < 0 || used > total {
		return 0, errors.New("invalid member spending")
	}
	return used, nil
}

func effectiveTaskTurns(team protocol.Team, member protocol.Member) int {
	limit := team.Budget.TaskTurns
	if isDream(team) {
		limit = protocol.TeamTaskTurnsMax
	}
	if member.Budget != nil {
		limit = narrowerTaskLimit(limit, member.Budget.TaskTurns)
	}
	return limit
}

func effectiveTaskMinutes(team protocol.Team, member protocol.Member) int {
	limit := team.Budget.TaskMinutes
	if isDream(team) {
		limit = 0
	}
	if member.Budget != nil {
		limit = narrowerTaskLimit(limit, member.Budget.TaskMinutes)
	}
	return limit
}

func narrowerTaskLimit(team, member int) int {
	if member > 0 && (team <= 0 || member < team) {
		return member
	}
	return team
}

// checkedBudgetRaise follows the existing half-plus-rounded-step raise but
// saturates before overflow. At MaxInt64 there is no possible further grant.
func checkedBudgetRaise(limit, used int64) (int64, error) {
	const step int64 = 100_000
	if limit < 0 || used < 0 || limit == math.MaxInt64 || used == math.MaxInt64 {
		return 0, errors.New("spending limit cannot be raised")
	}
	raised := int64(math.MaxInt64)
	half := limit/2 + limit%2
	if limit <= math.MaxInt64-half {
		raised = limit + half
	}
	if raised <= used {
		raised = math.MaxInt64
		if used <= math.MaxInt64-step {
			raised = used + step
		}
	}
	if remainder := raised % step; remainder != 0 && raised <= math.MaxInt64-(step-remainder) {
		raised += step - remainder
	}
	return raised, nil
}

// Budget identity never accepts the user-facing name fallback: a deleted
// member's ID may happen to be another current member's display name.
func budgetMember(team protocol.Team, id string) (protocol.Member, bool) {
	if id != "" {
		for _, member := range team.Members {
			if member.ID == id {
				return member, true
			}
		}
	}
	return protocol.Member{}, false
}
