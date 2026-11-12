package teams

import (
	"context"
	"encoding/json"
	"slices"
	"strings"
	"time"

	i18n "ryoku-i18n"

	"ryoku-rashin/internal/protocol"
)

// expirySweep is the longest the expiry timer waits. Timers run on the
// monotonic clock, which stops while the computer sleeps, so an hourly look
// at the wall clock catches approvals that expired during a suspend.
const expirySweep = time.Hour

// ask stores a pending approval with the exact action it would run, holds
// the caller's task on the user, and tells the user. It waits the settings'
// approval hours, except a spending limit card, whose no only keeps paused a
// team that already is, and a proposed team, which nothing runs without:
// those wait until answered.
func (s *Service) ask(_ context.Context, c caller, a protocol.Approval, action any) (protocol.Approval, error) {
	var raw json.RawMessage
	if action != nil {
		encoded, err := json.Marshal(action)
		if err != nil {
			return protocol.Approval{}, err
		}
		raw = encoded
	}
	now := s.now()
	if a.Team == "" {
		a.Team = c.Team
	}
	a.ID = newID("approval")
	a.Task, a.Session, a.Member = c.Task, c.Session, c.Member
	a.Title = s.redact(a.Title)
	a.Detail = s.redact(a.Detail)
	a.Status = protocol.ApprovalPending
	a.Decision, a.Note, a.Choice, a.Result, a.Decided = "", "", "", "", nil
	a.Created = now
	hours := approvalHours(s.settings())
	a.Expires = time.Time{}
	if a.Kind != protocol.ApprovalBudget && a.Kind != protocol.ApprovalTeam {
		a.Expires = now.Add(time.Duration(hours) * time.Hour)
	}
	a.Unwatched = s.nobodyWatching()
	if err := s.state.putApproval(approvalRecord{Approval: a, Action: raw}); err != nil {
		return protocol.Approval{}, err
	}
	if c.Task != "" {
		s.mu.Lock()
		task, ok := s.state.task(c.Task)
		held := ok && (task.Status == protocol.TaskWorking || task.Status == protocol.TaskNeedsYou)
		if held {
			task.Status, task.Waiting, task.Updated = protocol.TaskNeedsYou, protocol.WaitingOnYou, now
			held = s.state.putTask(task) == nil
		}
		s.mu.Unlock()
		if held {
			s.emitTask(task)
		}
	}
	team, _ := s.state.team(a.Team)
	actor, initiator := "Rashin", protocol.InitiatorRashin
	for _, m := range team.Members {
		if c.Task != "" && m.ID == a.Member {
			actor, initiator = m.Name, protocol.InitiatorAgent
		}
	}
	title := i18n.Tf("%s needs you", team.Name)
	if a.Kind == protocol.ApprovalTeam {
		actor, initiator = s.proposer(a.Session)
		title = i18n.T("A team is proposed")
	}
	waits := i18n.T("Waits for your answer.")
	if !a.Expires.IsZero() {
		waits = waitWords(hours)
	}
	s.record(protocol.Activity{
		Actor: actor, Action: "approval.ask", Target: a.Title, OK: true,
		Detail: waits, Team: a.Team, Task: a.Task, Initiator: initiator,
	})
	s.emitApproval(a)
	s.notify(notification{Team: a.Team, Task: a.Task, Title: title, Body: a.Title, Needs: true})
	s.armExpiry()
	return a, nil
}

// listApprovals lists approvals newest first.
func (s *Service) listApprovals(p protocol.ApprovalsParams) protocol.ApprovalsResult {
	limit := p.Limit
	if limit <= 0 {
		limit = 100
	}
	records := s.state.approvals(func(r approvalRecord) bool {
		return (!p.Pending || r.Status == protocol.ApprovalPending) && (p.Team == "" || r.Team == p.Team)
	})
	out := make([]protocol.Approval, 0, min(limit, len(records)))
	for _, rec := range records[:min(limit, len(records))] {
		out = append(out, rec.Approval)
	}
	return protocol.ApprovalsResult{Approvals: out}
}

// decideApproval answers a pending approval, carries out the answer through
// the effect of its kind, and lets its task go on once nothing else of it
// waits on the user. A choice question takes one of its options or a no.
func (s *Service) decideApproval(ctx context.Context, p protocol.ApprovalDecideParams) (protocol.Approval, error) {
	rec, ok := s.state.approval(p.ID)
	if !ok {
		return protocol.Approval{}, protocol.Errorf(protocol.CodeNotFound, "%s", i18n.Tf("No approval %s.", p.ID))
	}
	if len(rec.Options) > 0 && p.Decision != protocol.DecideChoose && p.Decision != protocol.DecideDeny {
		return protocol.Approval{}, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("Answer a choice question with one of its options or with no."))
	}
	status := protocol.ApprovalApproved
	switch p.Decision {
	case protocol.DecideChoose:
		if len(rec.Options) == 0 {
			return protocol.Approval{}, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("Only a choice question takes a choice."))
		}
		if !slices.Contains(rec.Options, p.Choice) {
			return protocol.Approval{}, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.Tf("%s is not one of the options.", p.Choice))
		}
	case protocol.DecideOnce:
		if rec.Kind == protocol.ApprovalRequest {
			return protocol.Approval{}, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("A credential request is answered with its value or with no."))
		}
	case protocol.DecideAlways:
		if rec.Always == "" {
			return protocol.Approval{}, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("This can only be allowed once."))
		}
	case protocol.DecideProvide:
		if rec.Kind != protocol.ApprovalRequest {
			return protocol.Approval{}, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("Only a credential request takes a value."))
		}
		if strings.TrimSpace(p.Value) == "" {
			return protocol.Approval{}, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("Paste the credential's value first."))
		}
	case protocol.DecideDeny:
		status = protocol.ApprovalDenied
	default:
		return protocol.Approval{}, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.Tf("%s is not an answer to an approval.", p.Decision))
	}
	p.Note = strings.TrimSpace(p.Note)
	rec, err := s.claimApproval(p.ID, status, p.Decision, p.Note)
	if err != nil {
		return protocol.Approval{}, err
	}
	action := "approval.approve"
	switch {
	case p.Decision == protocol.DecideChoose:
		rec.Choice = p.Choice
		action = "approval.choose"
	case status == protocol.ApprovalDenied:
		action = "approval.deny"
	}
	rec, err = s.settleApproval(ctx, rec, p, action, "you", protocol.InitiatorYou)
	s.armExpiry()
	return rec.Approval, err
}

// claimApproval marks a pending approval answered before its effect runs, so
// a second answer or the expiry sweep finds it taken and nothing runs twice.
func (s *Service) claimApproval(id, status, decision, note string) (approvalRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rec, ok := s.state.approval(id)
	if !ok {
		return approvalRecord{}, protocol.Errorf(protocol.CodeNotFound, "%s", i18n.Tf("No approval %s.", id))
	}
	if rec.Status != protocol.ApprovalPending {
		return approvalRecord{}, protocol.Errorf(protocol.CodeConflict, "%s", i18n.T("This was already answered."))
	}
	at := s.now()
	rec.Status, rec.Decision, rec.Note, rec.Decided = status, decision, note, &at
	if err := s.state.putApproval(rec); err != nil {
		return approvalRecord{}, err
	}
	return rec, nil
}

// settleApproval runs a claimed approval's effect, keeps what came of it,
// records and announces the answer, and continues the task with what its
// agent should read. An effect that fails leaves the approval answered; the
// failure goes into Result and to the agent.
func (s *Service) settleApproval(ctx context.Context, rec approvalRecord, p protocol.ApprovalDecideParams, action, actor, initiator string) (approvalRecord, error) {
	var text string
	var failed error
	if effect := s.effects[rec.Kind]; effect != nil {
		text, failed = effect(ctx, &rec, p)
		if failed != nil {
			rec.Result = s.redact(failed.Error())
			text = strings.TrimSpace(text + "\n\n" + i18n.Tf("Carrying out the answer failed: %s", rec.Result))
		}
	}
	saveErr := s.state.putApproval(rec)
	detail := approvalDetail(rec)
	if failed != nil {
		detail = rec.Result
	}
	s.record(protocol.Activity{
		Actor: actor, Action: action, Target: rec.Title, OK: failed == nil, Detail: detail, Choice: rec.Choice,
		Team: rec.Team, Task: rec.Task, Initiator: initiator,
	})
	s.emitApproval(rec.Approval)
	if rec.Task != "" && text != "" {
		others := s.state.approvals(func(r approvalRecord) bool {
			return r.Task == rec.Task && r.Status == protocol.ApprovalPending
		})
		if len(others) == 0 {
			s.release(rec.Task, text)
		}
	}
	return rec, saveErr
}

// approvalDetail says in one sentence how an approval was answered. A
// choice's entry names the option itself, so its detail is the user's note.
func approvalDetail(rec approvalRecord) string {
	switch {
	case rec.Status == protocol.ApprovalExpired:
		_, detail := expiryWords(askedHours(rec))
		return detail
	case rec.Decision == protocol.DecideChoose:
		return rec.Note
	case rec.Decision == protocol.DecideAlways:
		return i18n.Tf("Allowed from now on: %s", rec.Always)
	case rec.Decision == protocol.DecideProvide:
		return i18n.T("The value went into the vault.")
	case rec.Decision == protocol.DecideOnce:
		return i18n.T("Allowed once.")
	}
	return i18n.T("Said no.")
}

// approvalHours is how long a question waits, from the teams settings.
func approvalHours(settings protocol.Settings) int {
	if hours := settings.Teams.ApprovalHours; hours > 0 {
		return hours
	}
	return protocol.DefaultApprovalHours
}

// askedHours is how many hours an approval was given to wait.
func askedHours(rec approvalRecord) int {
	return int((rec.Expires.Sub(rec.Created) + time.Hour/2) / time.Hour)
}

// waitWords says how long a question waits, for the ledger.
func waitWords(hours int) string {
	switch hours {
	case 1:
		return i18n.T("Waits for your answer for up to an hour.")
	case 24:
		return i18n.T("Waits for your answer for up to a day.")
	}
	return i18n.Tf("Waits for your answer for up to %d hours.", hours)
}

// expiryWords are the note the agent hears and the ledger's detail when a
// question waited its hours unanswered.
func expiryWords(hours int) (note, detail string) {
	switch hours {
	case 1:
		return i18n.T("Nobody answered within an hour."), i18n.T("Nobody answered within an hour, so it counts as a no.")
	case 24:
		return i18n.T("Nobody answered within a day."), i18n.T("Nobody answered within a day, so it counts as a no.")
	}
	return i18n.Tf("Nobody answered within %d hours.", hours), i18n.Tf("Nobody answered within %d hours, so it counts as a no.", hours)
}

// armExpiry expires every pending approval past its time, then sets the one
// timer to the next expiry, or at most an hour ahead while any is pending.
// An approval without an expiry waits until answered.
func (s *Service) armExpiry() {
	now := s.now()
	pending := s.state.approvals(func(r approvalRecord) bool {
		return r.Status == protocol.ApprovalPending && !r.Expires.IsZero()
	})
	var next time.Time
	for _, rec := range pending {
		if !rec.Expires.After(now) {
			s.expireApproval(rec.ID)
			continue
		}
		if next.IsZero() || rec.Expires.Before(next) {
			next = rec.Expires
		}
	}
	if next.IsZero() {
		s.state.disarmExpiry()
		return
	}
	if sweep := now.Add(expirySweep); next.After(sweep) {
		next = sweep
	}
	s.state.armExpiry(next, now, func() {
		if s.background.Err() == nil {
			s.armExpiry()
		}
	})
}

// expireApproval answers an approval nobody answered in time with a no.
func (s *Service) expireApproval(id string) {
	asked, ok := s.state.approval(id)
	if !ok {
		return
	}
	note, _ := expiryWords(askedHours(asked))
	rec, err := s.claimApproval(id, protocol.ApprovalExpired, protocol.DecideDeny, note)
	if err != nil {
		return
	}
	p := protocol.ApprovalDecideParams{ID: id, Decision: protocol.DecideDeny, Note: note}
	_, _ = s.settleApproval(s.background, rec, p, "approval.expire", "Rashin", protocol.InitiatorRashin)
}

// emitApproval tells clients an approval was asked or answered.
func (s *Service) emitApproval(a protocol.Approval) {
	s.broadcast(protocol.EvtApproval, a)
}
