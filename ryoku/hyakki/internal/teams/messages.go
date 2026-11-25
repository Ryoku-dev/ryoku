package teams

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"ryoku-rashin/internal/protocol"
)

const (
	maxTeamMessageBody  = 64 << 10
	maxTeamMessageTitle = 512
)

func validMessageText(message protocol.TeamMessage) bool {
	return len(message.Body) > 0 && len(message.Body) <= maxTeamMessageBody && utf8.ValidString(message.Body) &&
		(message.Kind == "reassign" || strings.TrimSpace(message.Body) != "") &&
		len(message.Title) <= maxTeamMessageTitle && utf8.ValidString(message.Title) && !strings.ContainsAny(message.Title, "\r\n") &&
		(message.Kind == "help" || message.Kind == "escalate" || message.Kind == "comment" || message.Kind == "reassign")
}

// recordTeamMessage is in-process only. The owner dispatch and the authenticated
// help/escalation tools are responsible for authority; no arbitrary agent RPC
// accepts a TeamMessage. In particular, Member never becomes an assignee here.
func (s *Service) recordTeamMessage(ctx context.Context, message protocol.TeamMessage) (protocol.TeamMessage, error) {
	if err := ctx.Err(); err != nil {
		return protocol.TeamMessage{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.recordTeamMessageLocked(ctx, message)
}

func (s *Service) recordTeamMessageLocked(ctx context.Context, message protocol.TeamMessage) (protocol.TeamMessage, error) {
	if err := ctx.Err(); err != nil {
		return protocol.TeamMessage{}, err
	}
	message.SourceSession, message.Retired = "", false
	if session, bound := ctx.Value(agentSession{}).(string); bound {
		token := ""
		if run := s.runs[message.Task]; run != nil {
			token = run.Token
		}
		if _, _, _, err := s.currentCallerLocked(caller{Team: message.Team, Task: message.Task, Member: message.From, Session: session, Token: token}); err != nil {
			return protocol.TeamMessage{}, err
		}
		message.SourceSession = session
	}
	message.Body = strings.TrimSpace(message.Body)
	message.Title = strings.TrimSpace(message.Title)
	if !validMessageText(message) {
		return protocol.TeamMessage{}, protocol.Errorf(protocol.CodeBadRequest, "Messages require UTF-8 text up to 64 KiB and a one-line title up to 512 bytes.")
	}
	message.Body, message.Title = s.redact(message.Body), s.redact(message.Title)
	if !validMessageText(message) {
		return protocol.TeamMessage{}, protocol.Errorf(protocol.CodeBadRequest, "The redacted message exceeds the text limits.")
	}
	message.ID, message.Created, message.Delivered = newID("message"), s.now(), false
	if err := s.state.appendMessage(message); err != nil {
		return protocol.TeamMessage{}, err
	}
	return message, nil
}

// retireTaskRequestsLocked fences queued governance requests without deleting
// owner-visible history or claiming delivery. Caller holds Service.mu, not state.mu.
func (s *Service) retireTaskRequestsLocked(task string) error {
	var failure error
	for _, message := range s.state.messages(func(message protocol.TeamMessage) bool {
		return message.Task == task && !message.Delivered && !message.Retired && (message.Kind == "help" || message.Kind == "escalate")
	}) {
		failure = errors.Join(failure, s.state.retireMessage(message.ID))
	}
	return failure
}

// pendingTaskRequestsLocked preserves a waiting source before a busy manager
// can accept its decision child. Caller holds Service.mu, not state.mu.
func (s *Service) pendingTaskRequestsLocked(task protocol.TeamTask) bool {
	if !taskIsOpen(task) || task.Session == "" {
		return false
	}
	s.state.mu.Lock()
	defer s.state.mu.Unlock()
	for _, message := range s.state.messageSet {
		if message.Task == task.ID && message.Team == task.Team && message.From == task.Member &&
			message.SourceSession == task.Session && message.To != "" && !message.Delivered && !message.Retired &&
			(message.Kind == "help" || message.Kind == "escalate") {
			accepted := false
			for _, child := range s.state.taskSet {
				if child.Team == task.Team && child.Parent == task.ID && child.Origin == protocol.TaskFromMessage && child.Trigger == message.ID && child.Member == message.To {
					accepted = true
					break
				}
			}
			if !accepted {
				return true
			}
		}
	}
	return false
}

type teamMessageCursor struct {
	Team    string    `json:"team"`
	Task    string    `json:"task"`
	Created time.Time `json:"created"`
	ID      string    `json:"id"`
}

// teamMessages uses the immutable (Created, ID) pair, so equal timestamps and
// deleting a previous page's boundary record cannot lose or repeat messages.
func (s *Service) teamMessages(p protocol.TeamMessagesParams) (protocol.TeamMessagesResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if p.Team != "" {
		if _, ok := s.state.team(p.Team); !ok {
			return protocol.TeamMessagesResult{}, protocol.Errorf(protocol.CodeNotFound, "No team %s.", p.Team)
		}
	}
	if p.Task != "" {
		task, ok := s.state.task(p.Task)
		if !ok {
			return protocol.TeamMessagesResult{}, protocol.Errorf(protocol.CodeNotFound, "No task %s.", p.Task)
		}
		if p.Team != "" && task.Team != p.Team {
			return protocol.TeamMessagesResult{}, protocol.Errorf(protocol.CodeBadRequest, "The task does not belong to this team.")
		}
	}
	var cursor teamMessageCursor
	if p.Before != "" {
		if len(p.Before) > 4096 {
			return protocol.TeamMessagesResult{}, protocol.Errorf(protocol.CodeBadRequest, "The message cursor is not valid.")
		}
		raw, err := base64.RawURLEncoding.DecodeString(p.Before)
		if err != nil || json.Unmarshal(raw, &cursor) != nil || cursor.Created.IsZero() || !safeID(cursor.ID) || cursor.Team != p.Team || cursor.Task != p.Task {
			return protocol.TeamMessagesResult{}, protocol.Errorf(protocol.CodeBadRequest, "The message cursor is not valid for this query.")
		}
	}
	limit := p.Limit
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	messages := s.state.messages(func(message protocol.TeamMessage) bool {
		if (p.Team != "" && message.Team != p.Team) || (p.Task != "" && message.Task != p.Task) {
			return false
		}
		return p.Before == "" || message.Created.Before(cursor.Created) || (message.Created.Equal(cursor.Created) && message.ID < cursor.ID)
	})
	result := protocol.TeamMessagesResult{Messages: messages}
	if len(messages) > limit {
		result.Messages = messages[:limit]
		last := result.Messages[limit-1]
		raw, err := json.Marshal(teamMessageCursor{Team: p.Team, Task: p.Task, Created: last.Created, ID: last.ID})
		if err != nil {
			return protocol.TeamMessagesResult{}, err
		}
		result.Next = base64.RawURLEncoding.EncodeToString(raw)
	}
	return result, nil
}

func (s *Service) taskComment(ctx context.Context, p protocol.TeamTaskCommentParams) (protocol.TeamMessage, error) {
	if err := ctx.Err(); err != nil {
		return protocol.TeamMessage{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	task, ok := s.state.task(p.ID)
	if !ok {
		return protocol.TeamMessage{}, protocol.Errorf(protocol.CodeNotFound, "No task %s.", p.ID)
	}
	return s.recordTeamMessageLocked(ctx, protocol.TeamMessage{Team: task.Team, Task: task.ID, Kind: "comment", Title: "Summoner comment", Body: p.Body})
}
