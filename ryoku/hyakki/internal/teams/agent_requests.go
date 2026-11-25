package teams

import (
	"context"
	"strings"
	"unicode/utf8"

	"ryoku-rashin/internal/protocol"
)

func (s *Service) agentHelp(ctx context.Context, p protocol.AgentHelpParams) (protocol.AgentAnswer, error) {
	return s.agentGovernanceRequest(ctx, p.AgentRef, "help", p.Member, p.Title, p.Brief)
}

func (s *Service) agentEscalate(ctx context.Context, p protocol.AgentEscalateParams) (protocol.AgentAnswer, error) {
	return s.agentGovernanceRequest(ctx, p.AgentRef, "escalate", "", p.Title, p.Detail)
}

func (s *Service) agentGovernanceRequest(ctx context.Context, ref protocol.AgentRef, kind, target, title, body string) (protocol.AgentAnswer, error) {
	c, err := s.caller(ctx, ref)
	if err != nil {
		return protocol.AgentAnswer{}, err
	}
	title, body = strings.TrimSpace(title), strings.TrimSpace(body)
	if title == "" || utf8.RuneCountInString(title) > 200 || !utf8.ValidString(title) || strings.ContainsAny(title, "\r\n") || body == "" || len(body) > 16<<10 || !utf8.ValidString(body) {
		return protocol.AgentAnswer{}, protocol.Errorf(protocol.CodeBadRequest, "A handoff requires a one-line title up to 200 characters and details up to 16 KiB.")
	}
	s.mu.Lock()
	team, member, task, err := s.currentCallerLocked(c)
	if err != nil {
		s.mu.Unlock()
		return protocol.AgentAnswer{}, err
	}
	if task.Waiting != "" {
		s.mu.Unlock()
		return protocol.AgentAnswer{}, protocol.Errorf(protocol.CodeConflict, "This task already has a handoff in progress. End the turn and wait for its answer.")
	}
	to, requested := member.ReportsTo, ""
	if kind == "help" {
		peer, exists := taskMember(team, strings.TrimSpace(target))
		if !exists || peer.ID == member.ID {
			s.mu.Unlock()
			return protocol.AgentAnswer{}, protocol.Errorf(protocol.CodeBadRequest, "Request help from another current spirit in this team.")
		}
		requested = peer.ID
		to = protocol.CommonManager(team, member.ID, peer.ID)
		body = "Requested help from " + peer.Name + ":\n" + body
	}
	managerName := "the Summoner"
	if to != "" {
		manager, exists := protocol.MemberByID(team.Members, to)
		if !exists {
			s.mu.Unlock()
			return protocol.AgentAnswer{}, protocol.Errorf(protocol.CodeConflict, "The responsible manager no longer exists.")
		}
		managerName = manager.Name
		if !memberWakeAllowed(manager, protocol.HeartbeatMessage) {
			body = "The responsible manager, " + manager.Name + ", does not wake on messages. The Summoner must decide this handoff instead.\n\n" + body
			to, managerName = "", "the Summoner"
		}
	}
	message, err := s.recordTeamMessageLocked(WithAgentSession(ctx, c.Session), protocol.TeamMessage{Team: c.Team, Task: c.Task, From: c.Member, To: to, Member: requested, Kind: kind, Title: title, Body: body})
	if err != nil {
		s.mu.Unlock()
		return protocol.AgentAnswer{}, err
	}
	if to != "" {
		task.Waiting, task.Updated = protocol.WaitingOnMembers, s.now()
		if err := s.state.putTask(task); err != nil {
			s.mu.Unlock()
			s.halt(c.Session)
			return protocol.AgentAnswer{}, err
		}
	}
	s.mu.Unlock()
	if to == "" {
		if _, err := s.ask(ctx, c, protocol.Approval{Kind: protocol.ApprovalAction, Title: title, Detail: message.Body}, nil); err != nil {
			return protocol.AgentAnswer{}, err
		}
		if err := s.state.deliverMessage(message.ID); err != nil {
			return protocol.AgentAnswer{}, err
		}
	} else {
		s.emitTask(task)
		_, accepted, err := s.wakeMessage(ctx, message)
		if err != nil {
			return protocol.AgentAnswer{}, err
		}
		if accepted {
			if err := s.state.deliverMessage(message.ID); err != nil {
				return protocol.AgentAnswer{}, err
			}
		}
		s.clock.rearm()
	}
	s.record(protocol.Activity{Actor: member.Name, Action: "task." + kind, Target: title, OK: true, Team: c.Team, Task: c.Task, Initiator: protocol.InitiatorAgent, Detail: "Routed to " + managerName + "; no peer was assigned directly."})
	s.emitTeam(c.Team)
	return protocol.AgentAnswer{Text: "The request is routed to " + managerName + ". No peer was assigned directly. End your turn; this task waits for the governed answer."}, nil
}
