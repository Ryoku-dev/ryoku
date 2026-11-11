package teams

import (
	"context"
	"crypto/subtle"
	"path/filepath"
	"strings"

	i18n "ryoku-i18n"

	"ryoku-rashin/internal/protocol"
)

// agentSession keys the session whose own socket a call arrived on.
type agentSession struct{}

// WithAgentSession marks ctx as a call that arrived on session id's own agent
// socket, so the call may act only as that session's run.
func WithAgentSession(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, agentSession{}, id)
}

// caller finds the live run a call from an agent's MCP server belongs to. It
// refuses a call that names another session than the socket it came on, a
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
		if subtle.ConstantTimeCompare([]byte(r.Token), []byte(ref.Token)) != 1 {
			s.mu.Unlock()
			return caller{}, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("This call does not come from the task's own conversation."))
		}
		c := caller{Team: r.Team, Task: r.Task, Member: r.Member, Session: r.Session}
		s.mu.Unlock()
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
// the goal ends, and what waited on the task moves on.
func (s *Service) agentReport(ctx context.Context, p protocol.AgentReportParams) (protocol.AgentAnswer, error) {
	c, err := s.caller(ctx, p.AgentRef)
	if err != nil {
		return protocol.AgentAnswer{}, err
	}
	team, ok := s.state.team(c.Team)
	if !ok {
		return protocol.AgentAnswer{}, protocol.Errorf(protocol.CodeNotFound, "%s", i18n.Tf("No team %s.", c.Team))
	}
	body := strings.TrimSpace(p.Body)
	if body == "" {
		return protocol.AgentAnswer{}, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("A report needs a body: what you found or did."))
	}
	task, _ := s.state.task(c.Task)
	title := strings.TrimSpace(p.Title)
	if title == "" {
		title = task.Title
	}
	report := &protocol.TeamReport{Title: s.redact(title), Body: s.redact(body), Files: teamFiles(team.Folder, p.Files), At: s.now()}
	ended, ok := s.endTask(c.Task, func(t *protocol.TeamTask) {
		t.Status, t.Report, t.Reason = protocol.TaskDone, report, ""
	})
	if !ok {
		return protocol.AgentAnswer{Text: i18n.T("This task already ended.")}, nil
	}
	member, _ := taskMember(team, c.Member)
	s.record(protocol.Activity{Actor: member.Name, Action: "task.report", Target: ended.Title, OK: true, Team: team.ID, Task: ended.ID, Initiator: protocol.InitiatorAgent, Detail: i18n.Tf("Reported: %s", report.Title)})
	_, _ = s.sessions.Goal(ctx, protocol.GoalParams{ID: c.Session, Op: protocol.GoalComplete, Summary: report.Title})
	s.notifyEnd(ended)
	s.afterEnd(ended)
	return protocol.AgentAnswer{Text: i18n.T("Your report is with the user and the task is done. End your turn now.")}, nil
}

// teamFiles keeps the reported files that lie inside the team's folder, as
// absolute paths; a path given relative is taken from the folder.
func teamFiles(folder string, files []string) []string {
	var kept []string
	for _, file := range files {
		file = strings.TrimSpace(file)
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

// agentDelegate lets a lead hand one piece of its task to a member. Pieces go
// one level deep, at most four open at a time, and the lead waits on them
// between turns.
func (s *Service) agentDelegate(ctx context.Context, p protocol.AgentDelegateParams) (protocol.AgentAnswer, error) {
	c, err := s.caller(ctx, p.AgentRef)
	if err != nil {
		return protocol.AgentAnswer{}, err
	}
	team, ok := s.state.team(c.Team)
	if !ok {
		return protocol.AgentAnswer{}, protocol.Errorf(protocol.CodeNotFound, "%s", i18n.Tf("No team %s.", c.Team))
	}
	lead, _ := taskMember(team, c.Member)
	if !lead.Lead {
		return protocol.AgentAnswer{}, protocol.Errorf(protocol.CodeConflict, "%s", i18n.T("Only the team's lead hands out pieces of a task."))
	}
	if task, _ := s.state.task(c.Task); task.Parent != "" {
		return protocol.AgentAnswer{}, protocol.Errorf(protocol.CodeConflict, "%s", i18n.T("A piece cannot be handed on again."))
	}
	member, ok := taskMember(team, p.Member)
	if !ok || member.Lead {
		var names []string
		for _, m := range team.Members {
			if !m.Lead {
				names = append(names, m.Name)
			}
		}
		if len(names) == 0 {
			return protocol.AgentAnswer{}, protocol.Errorf(protocol.CodeNotFound, "%s", i18n.T("Your team has no member to hand work to. Do the task yourself."))
		}
		return protocol.AgentAnswer{}, protocol.Errorf(protocol.CodeNotFound, "%s", i18n.Tf("No member %s. Your members are: %s.", p.Member, strings.Join(names, ", ")))
	}
	if strings.TrimSpace(p.Brief) == "" {
		return protocol.AgentAnswer{}, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("A piece needs a brief: what the member should do."))
	}
	s.mu.Lock()
	open := s.state.tasks(func(t protocol.TeamTask) bool { return t.Parent == c.Task && taskIsOpen(t) })
	if len(open) >= maxPieces {
		s.mu.Unlock()
		return protocol.AgentAnswer{}, protocol.Errorf(protocol.CodeConflict, "%s", i18n.Tf("You already have %d pieces out. Wait for their reports first.", maxPieces))
	}
	waiting, known := s.state.task(c.Task)
	if !known || !taskIsOpen(waiting) {
		s.mu.Unlock()
		return protocol.AgentAnswer{Text: i18n.T("This task already ended.")}, nil
	}
	before := waiting.Waiting
	waiting.Waiting, waiting.Updated = protocol.WaitingOnMembers, s.now()
	err = s.state.putTask(waiting)
	s.mu.Unlock()
	if err != nil {
		return protocol.AgentAnswer{}, err
	}
	s.emitTask(waiting)
	piece, err := s.newTask(ctx, team.ID, member.ID, p.Title, p.Brief, protocol.TaskFromLead, "", c.Task)
	if err != nil {
		if len(open) == 0 {
			s.mu.Lock()
			if current, ok := s.state.task(c.Task); ok && current.Waiting == protocol.WaitingOnMembers {
				current.Waiting = before
				_ = s.state.putTask(current)
				waiting = current
			}
			s.mu.Unlock()
			s.emitTask(waiting)
		}
		return protocol.AgentAnswer{}, err
	}
	s.record(protocol.Activity{Actor: lead.Name, Action: "task.delegate", Target: piece.Title, OK: true, Team: team.ID, Task: c.Task, Initiator: protocol.InitiatorAgent, Detail: i18n.Tf("Handed to %s.", member.Name)})
	return protocol.AgentAnswer{Text: i18n.Tf("Started \"%s\" for %s. End your turn now; Rashin continues you with their report.", piece.Title, member.Name)}, nil
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
	return protocol.AgentStatusResult{Tasks: pieces}, nil
}

// agentAsk puts an action to the user as an approval; the task waits on the
// answer between turns.
func (s *Service) agentAsk(ctx context.Context, p protocol.AgentAskParams) (protocol.AgentAnswer, error) {
	c, err := s.caller(ctx, p.AgentRef)
	if err != nil {
		return protocol.AgentAnswer{}, err
	}
	title := strings.TrimSpace(p.Title)
	if title == "" {
		return protocol.AgentAnswer{}, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("Say in one line what the user should decide."))
	}
	asked := protocol.Approval{
		Team: c.Team, Task: c.Task, Session: c.Session, Member: c.Member, Kind: protocol.ApprovalAction,
		Title: s.redact(title), Detail: s.redact(strings.TrimSpace(p.Detail)),
	}
	if _, err := s.ask(ctx, c, asked, nil); err != nil {
		return protocol.AgentAnswer{}, err
	}
	return protocol.AgentAnswer{Text: i18n.Tf("Waiting for the user: %s. End your turn now; Rashin continues this task with their answer.", title)}, nil
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
