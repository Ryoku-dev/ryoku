package teams

import (
	"context"
	"crypto/subtle"
	"path/filepath"
	"strings"

	i18n "ryoku-i18n"

	"ryoku-rashin/internal/protocol"
)

// agentSession keys the session whose own team tools made a call.
type agentSession struct{}

// WithAgentSession marks ctx as a call that session id's own team tools
// made, so the call may act only as that session's run.
func WithAgentSession(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, agentSession{}, id)
}

// caller finds the live run a call from a session's team tools belongs to.
// It refuses a call that names another session than the one that made it, a
// session it does not know, a token that is not the run's, and a run whose
// task has ended.
func (s *Service) caller(ctx context.Context, ref protocol.AgentRef) (caller, error) {
	if bound, ok := ctx.Value(agentSession{}).(string); ok && bound != ref.Session {
		return caller{}, protocol.Errorf(protocol.CodeForbidden, "%s", i18n.T("This call does not come from the task's own conversation."))
	}
	s.mu.Lock()
	for _, r := range s.runs {
		if r.Session == "" || r.Session != ref.Session {
			continue
		}
		if ref.Token == "" || subtle.ConstantTimeCompare([]byte(r.Token), []byte(ref.Token)) != 1 {
			s.mu.Unlock()
			return caller{}, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("This call does not come from the task's own conversation."))
		}
		c := caller{Team: r.Team, Task: r.Task, Member: r.Member, Session: r.Session, Token: r.Token}
		_, _, _, err := s.currentCallerLocked(c)
		s.mu.Unlock()
		if err != nil {
			return caller{}, err
		}
		return c, nil
	}
	s.mu.Unlock()
	ended := s.state.tasks(func(t protocol.TeamTask) bool {
		for _, session := range t.Sessions {
			if session == ref.Session {
				return true
			}
		}
		return false
	})
	if len(ended) > 0 {
		return caller{}, protocol.Errorf(protocol.CodeConflict, "%s", i18n.T("This task already ended."))
	}
	return caller{}, protocol.Errorf(protocol.CodeNotFound, "%s", i18n.T("This conversation does not work on a team task."))
}

// agentReport takes the report that ends a task: it is kept for the user,
// the goal ends, and what waited on the task moves on. A quiet report, which
// only a routine's run may give, ends the run done without marking it
// unread or notifying. A dream's standing task takes no report.
func (s *Service) agentReport(ctx context.Context, p protocol.AgentReportParams) (protocol.AgentAnswer, error) {
	c, err := s.caller(ctx, p.AgentRef)
	if err != nil {
		return protocol.AgentAnswer{}, err
	}
	if err := s.dreamReport(c); err != nil {
		return protocol.AgentAnswer{}, err
	}
	team, ok := s.state.team(c.Team)
	if !ok {
		return protocol.AgentAnswer{}, protocol.Errorf(protocol.CodeNotFound, "%s", i18n.Tf("No team %s.", c.Team))
	}
	task, _ := s.state.task(c.Task)
	quiet := p.Quiet && task.Routine != ""
	body := strings.TrimSpace(p.Body)
	if body == "" && !quiet {
		return protocol.AgentAnswer{}, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("A report needs a body: what you found or did."))
	}
	if taskNeedsPlan(team, task) {
		return protocol.AgentAnswer{}, protocol.Errorf(protocol.CodeForbidden, "The Summoner must approve this strategy before completion can be reported.")
	}
	title := strings.TrimSpace(p.Title)
	if title == "" {
		title = task.Title
	}
	report := &protocol.TeamReport{Title: s.redact(title), Body: s.redact(body), Files: s.teamFiles(team.Folder, p.Files), At: s.now(), Quiet: quiet}
	var rejected error
	ended, ok := s.endTaskIf(c.Task, func(current protocol.TeamTask) bool {
		currentTeam, _, _, err := s.currentCallerLocked(c)
		if err != nil {
			rejected = err
			return false
		}
		if taskNeedsPlan(currentTeam, current) {
			rejected = protocol.Errorf(protocol.CodeForbidden, "The current strategy is not approved.")
			return false
		}
		if len(s.state.tasks(func(piece protocol.TeamTask) bool { return piece.Parent == current.ID && taskIsOpen(piece) })) > 0 {
			rejected = protocol.Errorf(protocol.CodeConflict, "Wait for the open pieces or review handoffs before reporting completion.")
			return false
		}
		return true
	}, func(t *protocol.TeamTask) {
		t.Status, t.Report, t.Reason, t.Read = protocol.TaskDone, report, "", quiet
	})
	if !ok {
		if rejected == nil {
			rejected = protocol.Errorf(protocol.CodeConflict, "This execution no longer owns an open task.")
		}
		return protocol.AgentAnswer{}, rejected
	}
	member, _ := taskMember(team, c.Member)
	detail := i18n.Tf("Reported: %s", report.Title)
	if quiet {
		detail = i18n.Tf("Nothing new: %s", report.Title)
	}
	s.record(protocol.Activity{Actor: member.Name, Action: "task.report", Target: ended.Title, OK: true, Team: team.ID, Task: ended.ID, Initiator: protocol.InitiatorAgent, Detail: detail})
	_, _ = s.sessions.Goal(ctx, protocol.GoalParams{ID: c.Session, Op: protocol.GoalComplete, Summary: report.Title})
	if !quiet {
		s.notifyEnd(ended)
	}
	s.afterEnd(ended)
	if quiet {
		return protocol.AgentAnswer{Text: i18n.T("The run is done and noted as quiet; the user is not notified. End your turn now.")}, nil
	}
	return protocol.AgentAnswer{Text: i18n.T("Your report is with the user and the task is done. End your turn now.")}, nil
}

// teamFiles keeps the reported files that lie inside the team's folder, as
// absolute paths; a path given relative is taken from the folder.
func (s *Service) teamFiles(folder string, files []string) []string {
	var kept []string
	for _, file := range files {
		file = s.redact(strings.TrimSpace(file))
		if file == "" {
			continue
		}
		if !filepath.IsAbs(file) {
			file = filepath.Join(folder, file)
		}
		file = filepath.Clean(file)
		rel, err := filepath.Rel(folder, file)
		if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			continue
		}
		kept = append(kept, file)
	}
	return kept
}

// agentDelegate hands work downward through the configured reporting graph.
// Each task retains its own parent, token and independent permission boundary.
func (s *Service) agentDelegate(ctx context.Context, p protocol.AgentDelegateParams) (protocol.AgentAnswer, error) {
	c, err := s.caller(ctx, p.AgentRef)
	if err != nil {
		return protocol.AgentAnswer{}, err
	}
	if err := ctx.Err(); err != nil {
		return protocol.AgentAnswer{}, err
	}
	if strings.TrimSpace(p.Brief) == "" {
		return protocol.AgentAnswer{}, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("A piece needs a brief: what the member should do."))
	}
	s.mu.Lock()
	team, manager, waiting, err := s.currentCallerLocked(c)
	if err != nil {
		s.mu.Unlock()
		return protocol.AgentAnswer{}, err
	}
	member, known := taskMember(team, p.Member)
	if !known || !protocol.CanDelegate(team, manager.ID, member.ID) {
		var names []string
		for _, report := range team.Members {
			if protocol.CanDelegate(team, manager.ID, report.ID) {
				names = append(names, report.Name)
			}
		}
		s.mu.Unlock()
		if len(names) == 0 {
			return protocol.AgentAnswer{}, protocol.Errorf(protocol.CodeForbidden, "%s", i18n.T("You have no reports to delegate to. Request help or escalate to your manager instead."))
		}
		return protocol.AgentAnswer{}, protocol.Errorf(protocol.CodeForbidden, "%s", i18n.Tf("You may delegate only to your reports: %s.", strings.Join(names, ", ")))
	}
	if waiting.Waiting == protocol.WaitingOnYou || waiting.Waiting == protocol.WaitingOnPermission || s.pendingApprovals(waiting.ID) > 0 {
		s.mu.Unlock()
		return protocol.AgentAnswer{}, protocol.Errorf(protocol.CodeConflict, "The Summoner must answer the current handoff before more work is assigned.")
	}
	if taskNeedsPlan(team, waiting) {
		s.mu.Unlock()
		return protocol.AgentAnswer{}, protocol.Errorf(protocol.CodeForbidden, "The Summoner must approve this strategy before work is delegated.")
	}
	before := waiting
	waiting.Waiting, waiting.Updated = protocol.WaitingOnMembers, s.now()
	if err := s.state.putTask(waiting); err != nil {
		s.mu.Unlock()
		return protocol.AgentAnswer{}, err
	}
	piece, assignee, err := s.newTaskLocked(team.ID, member.ID, p.Title, p.Brief, protocol.TaskFromLead, "", c.Task, "", nil, true)
	if err != nil {
		restoreErr := s.state.putTask(before)
		s.mu.Unlock()
		if restoreErr != nil {
			s.halt(c.Session)
			return protocol.AgentAnswer{}, protocol.Errorf(protocol.CodeUnavailable, "The delegated task could not be stored and its previous hold could not be restored.")
		}
		return protocol.AgentAnswer{}, err
	}
	s.mu.Unlock()
	s.emitTask(waiting)
	s.emitTask(piece)
	s.record(protocol.Activity{Actor: manager.Name, Action: "task.create", Target: piece.Title, OK: true, Team: team.ID, Task: piece.ID, Initiator: protocol.InitiatorAgent, Detail: i18n.Tf("%s takes it on.", assignee)})
	s.record(protocol.Activity{Actor: manager.Name, Action: "task.delegate", Target: piece.Title, OK: true, Team: team.ID, Task: c.Task, Initiator: protocol.InitiatorAgent, Detail: i18n.Tf("Handed to %s.", member.Name)})
	s.pump()
	return protocol.AgentAnswer{Text: i18n.Tf("Started \"%s\" for %s. End your turn now; Hyakki continues you with their report.", piece.Title, member.Name)}, nil
}

// agentStatus lists the pieces the calling task handed out, with each one's
// state and report.
func (s *Service) agentStatus(ctx context.Context, ref protocol.AgentRef) (protocol.AgentStatusResult, error) {
	c, err := s.caller(ctx, ref)
	if err != nil {
		return protocol.AgentStatusResult{}, err
	}
	pieces := s.state.tasks(func(t protocol.TeamTask) bool { return t.Parent == c.Task })
	if pieces == nil {
		pieces = []protocol.TeamTask{}
	}
	context, err := s.taskLineage(c.Task)
	if err != nil {
		return protocol.AgentStatusResult{}, err
	}
	team, exists := s.state.team(c.Team)
	if !exists {
		return protocol.AgentStatusResult{}, protocol.Errorf(protocol.CodeNotFound, "The team no longer exists.")
	}
	members := make([]protocol.MemberDirectoryEntry, len(team.Members))
	for i, member := range team.Members {
		members[i] = protocol.MemberDirectoryEntry{
			ID: member.ID, Name: member.Name, Title: member.Title, Rank: member.Rank, Spirit: member.Spirit,
			Capabilities: member.Capabilities, ReportsTo: member.ReportsTo, Paused: member.Paused,
		}
	}
	return protocol.AgentStatusResult{Tasks: pieces, Context: &context, Members: members, Dream: team.Dream}, nil
}

// agentAsk puts an action, or a choice between options, to the user as an
// approval; the task waits on the answer between turns.
func (s *Service) agentAsk(ctx context.Context, p protocol.AgentAskParams) (protocol.AgentAnswer, error) {
	c, err := s.caller(ctx, p.AgentRef)
	if err != nil {
		return protocol.AgentAnswer{}, err
	}
	title := strings.TrimSpace(p.Title)
	if title == "" {
		return protocol.AgentAnswer{}, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("Say in one line what the user should decide."))
	}
	var options []string
	for _, option := range p.Options {
		options = append(options, strings.TrimSpace(option))
	}
	if !protocol.ValidApprovalOptions(options) {
		return protocol.AgentAnswer{}, protocol.Errorf(protocol.CodeBadRequest, "%s",
			i18n.Tf("Offer 2 to %d different options of at most %d characters each, or none.", protocol.ApprovalOptionsMax, protocol.ApprovalOptionRunesMax))
	}
	for i, option := range options {
		options[i] = s.redact(option)
	}
	asked := protocol.Approval{
		Team: c.Team, Task: c.Task, Session: c.Session, Member: c.Member, Kind: protocol.ApprovalAction,
		Title: s.redact(title), Detail: s.redact(strings.TrimSpace(p.Detail)), Options: options,
	}
	if _, err := s.ask(ctx, c, asked, nil); err != nil {
		return protocol.AgentAnswer{}, err
	}
	if len(options) > 0 {
		return protocol.AgentAnswer{Text: i18n.Tf("Waiting for the user to choose: %s. End your turn now; Hyakki continues this task with their choice.", title)}, nil
	}
	return protocol.AgentAnswer{Text: i18n.Tf("Waiting for the user: %s. End your turn now; Hyakki continues this task with their answer.", title)}, nil
}

// piecesText is what a lead is sent on with once its pieces are in: each
// member, piece, how it ended and its report or reason.
func piecesText(team protocol.Team, pieces []protocol.TeamTask) string {
	var b strings.Builder
	b.WriteString(i18n.T("Your members finished the pieces you handed out."))
	for i := len(pieces) - 1; i >= 0; i-- {
		piece := pieces[i]
		member, _ := taskMember(team, piece.Member)
		var standing, text string
		switch piece.Status {
		case protocol.TaskDone:
			standing = i18n.T("done")
			if piece.Report != nil {
				text = piece.Report.Body
			}
		case protocol.TaskBlocked:
			standing, text = i18n.T("blocked"), piece.Reason
		default:
			standing, text = i18n.T("stopped"), piece.Reason
		}
		b.WriteString("\n\n")
		b.WriteString(i18n.Tf("%s, \"%s\": %s", member.Name, piece.Title, standing))
		if text = strings.TrimSpace(text); text != "" {
			b.WriteString("\n")
			b.WriteString(clipRunes(text, pieceReport))
		}
	}
	return b.String()
}
