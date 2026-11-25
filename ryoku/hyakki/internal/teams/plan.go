package teams

import (
	"context"
	"encoding/json"
	"strings"
	"unicode/utf8"

	"ryoku-rashin/internal/protocol"
)

type strategyAction struct {
	Plan           protocol.TeamPlan `json:"plan"`
	MemberRevision uint64            `json:"memberRevision"`
}

func strategyPlan(summary string, steps []string) (protocol.TeamPlan, error) {
	summary = strings.TrimSpace(summary)
	if summary == "" || len(summary) > 4096 || !utf8.ValidString(summary) || len(steps) == 0 || len(steps) > 32 {
		return protocol.TeamPlan{}, protocol.Errorf(protocol.CodeBadRequest, "A strategy requires a summary up to 4096 bytes and 1 to 32 steps.")
	}
	plan := protocol.TeamPlan{Summary: summary, Steps: make([]string, len(steps)), Revision: 1}
	for i, step := range steps {
		step = strings.TrimSpace(step)
		if step == "" || len(step) > 2048 || !utf8.ValidString(step) {
			return protocol.TeamPlan{}, protocol.Errorf(protocol.CodeBadRequest, "Each strategy step requires text up to 2048 bytes.")
		}
		plan.Steps[i] = step
	}
	return plan, nil
}

func (s *Service) redactPlan(plan *protocol.TeamPlan) error {
	plan.Summary = s.redact(plan.Summary)
	for i := range plan.Steps {
		plan.Steps[i] = s.redact(plan.Steps[i])
	}
	_, err := strategyPlan(plan.Summary, plan.Steps)
	return err
}

func (s *Service) agentPlan(ctx context.Context, p protocol.AgentPlanParams) (protocol.AgentAnswer, error) {
	c, err := s.caller(ctx, p.AgentRef)
	if err != nil {
		return protocol.AgentAnswer{}, err
	}
	plan, err := strategyPlan(p.Summary, p.Steps)
	if err != nil {
		return protocol.AgentAnswer{}, err
	}
	if err := s.redactPlan(&plan); err != nil {
		return protocol.AgentAnswer{}, err
	}
	s.mu.Lock()
	team, member, task, err := s.currentCallerLocked(c)
	if err != nil {
		s.mu.Unlock()
		return protocol.AgentAnswer{}, err
	}
	if task.Plan != nil && task.Plan.Approved {
		s.mu.Unlock()
		return protocol.AgentAnswer{}, protocol.Errorf(protocol.CodeConflict, "This execution already has an approved strategy. The Summoner must stop or retry it before replacing the strategy.")
	}
	if !team.PlanApproval {
		plan.Approved = true
		task.Plan, task.Updated = &plan, s.now()
		err = s.state.putTask(task)
		s.mu.Unlock()
		if err != nil {
			return protocol.AgentAnswer{}, err
		}
		s.emitTask(task)
		return protocol.AgentAnswer{Text: "The strategy is recorded. No strategy decision is required for this team; its existing access, approvals and budgets still apply. End this turn before execution."}, nil
	}
	s.mu.Unlock()
	detail := plan.Summary + "\n\n" + strings.Join(plan.Steps, "\n")
	_, err = s.ask(ctx, c, protocol.Approval{Kind: protocol.ApprovalStrategy, Title: "Approve strategy: " + task.Title, Detail: detail}, strategyAction{Plan: plan, MemberRevision: member.Revision})
	if err != nil {
		return protocol.AgentAnswer{}, err
	}
	return protocol.AgentAnswer{Text: "The Summoner must decide on this strategy. End your turn now; execution remains read-only until approval and never exceeds the seat's existing permissions."}, nil
}

func (s *Service) effectStrategy(ctx context.Context, rec *approvalRecord, p protocol.ApprovalDecideParams) (string, error) {
	if p.Decision == protocol.DecideDeny {
		rec.Result = "The strategy was declined. Stay read-only, revise it and propose another plan."
		return rec.Result, nil
	}
	var action strategyAction
	if err := json.Unmarshal(rec.Action, &action); err != nil {
		return "", protocol.Errorf(protocol.CodeBadRequest, "The strategy decision has no valid saved plan.")
	}
	plan, err := strategyPlan(action.Plan.Summary, action.Plan.Steps)
	if err != nil || action.Plan.Revision != 1 || action.MemberRevision == 0 {
		return "", protocol.Errorf(protocol.CodeBadRequest, "The saved strategy is invalid.")
	}
	s.taskControl.Lock()
	defer s.taskControl.Unlock()
	s.mu.Lock()
	token := ""
	if run := s.runs[rec.Task]; run != nil {
		token = run.Token
	}
	team, member, task, err := s.currentCallerLocked(caller{Team: rec.Team, Task: rec.Task, Member: rec.Member, Session: rec.Session, Token: token})
	if err != nil {
		s.mu.Unlock()
		return "", err
	}
	if member.Revision != action.MemberRevision || !team.PlanApproval || task.Plan != nil && task.Plan.Approved {
		s.mu.Unlock()
		return "", protocol.Errorf(protocol.CodeConflict, "This strategy decision no longer matches the current execution.")
	}
	if err := s.redactPlan(&plan); err != nil {
		s.mu.Unlock()
		return "", err
	}
	plan.Approved = true
	task.Plan, task.Updated = &plan, s.now()
	if err := s.state.putTask(task); err != nil {
		s.mu.Unlock()
		return "", err
	}
	mode := taskApprovals(team, member, task)
	s.mu.Unlock()
	s.emitTask(task)
	if s.sessions == nil {
		return "", protocol.Errorf(protocol.CodeUnavailable, "The current session controller is unavailable; execution was not resumed.")
	}
	if err := s.sessions.SetMode(ctx, protocol.SetModeParams{ID: rec.Session, Mode: mode}); err != nil {
		return "", err
	}
	rec.Result = "The strategy is approved. Follow its steps within the original team/member access, approvals and budget ceilings."
	return rec.Result, nil
}

// taskPlan is the explicit owner path for supplying or editing a pending plan.
// In particular a networked foreign runtime cannot plan inside strict offline
// read-only isolation, so it stays parked rather than silently using a provider.
func (s *Service) taskPlan(ctx context.Context, p protocol.TeamTaskPlanParams) (protocol.TeamTask, error) {
	plan, err := strategyPlan(p.Summary, p.Steps)
	if err != nil {
		return protocol.TeamTask{}, err
	}
	if err := s.redactPlan(&plan); err != nil {
		return protocol.TeamTask{}, err
	}
	plan.Approved = true
	if err := ctx.Err(); err != nil {
		return protocol.TeamTask{}, err
	}
	s.taskControl.Lock()
	defer s.taskControl.Unlock()
	s.mu.Lock()
	task, exists := s.state.task(p.ID)
	team, known := s.state.team(task.Team)
	member, seatExists := protocol.MemberByID(team.Members, task.Member)
	if !exists || !known || !seatExists {
		s.mu.Unlock()
		return protocol.TeamTask{}, protocol.Errorf(protocol.CodeNotFound, "The task and its current team seat must exist.")
	}
	if !taskIsOpen(task) || !taskNeedsPlan(team, task) || s.taskEdits[task.ID] || s.editing[task.Team][task.Member] {
		s.mu.Unlock()
		return protocol.TeamTask{}, protocol.Errorf(protocol.CodeConflict, "Only a current pending strategy may be supplied. Stop or retry an already approved execution before changing its strategy.")
	}
	var replaced []protocol.Approval
	for _, rec := range s.state.approvals(func(rec approvalRecord) bool {
		return rec.Task == task.ID && rec.Kind == protocol.ApprovalStrategy && rec.Status == protocol.ApprovalPending
	}) {
		now := s.now()
		rec.Status, rec.Decision, rec.Note, rec.Decided = protocol.ApprovalDenied, protocol.DecideDeny, "The Summoner supplied a replacement strategy.", &now
		if err := s.state.putApproval(rec); err != nil {
			s.mu.Unlock()
			return protocol.TeamTask{}, err
		}
		replaced = append(replaced, rec.Approval)
	}
	task.Plan, task.Updated = &plan, s.now()
	if task.Session == "" {
		closePermissionWait(&task, task.Updated)
		task.Status, task.Waiting, task.Reason = protocol.TaskReady, "", ""
	}
	if err := s.state.putTask(task); err != nil {
		s.mu.Unlock()
		return protocol.TeamTask{}, err
	}
	s.armLineageMinutesLocked(task)
	mode := taskApprovals(team, member, task)
	s.mu.Unlock()
	for _, rec := range replaced {
		s.emitApproval(rec)
	}
	s.emitTask(task)
	if task.Session != "" {
		if s.sessions == nil {
			return protocol.TeamTask{}, protocol.Errorf(protocol.CodeUnavailable, "The current session controller is unavailable; execution was not resumed.")
		}
		if err := s.sessions.SetMode(ctx, protocol.SetModeParams{ID: task.Session, Mode: mode}); err != nil {
			return protocol.TeamTask{}, err
		}
		s.release(task.ID, task.Session, task.Waiting, task.ResumeVersion, "The Summoner supplied and approved this strategy:\n"+plan.Summary+"\n"+strings.Join(plan.Steps, "\n"))
	} else {
		s.pump()
	}
	s.record(protocol.Activity{Actor: protocol.InitiatorYou, Action: "task.plan", Target: task.Title, OK: true, Team: task.Team, Task: task.ID, Initiator: protocol.InitiatorYou, Detail: "Supplied and approved a pending strategy without widening the seat's grants."})
	current, _ := s.state.task(task.ID)
	return current, nil
}
