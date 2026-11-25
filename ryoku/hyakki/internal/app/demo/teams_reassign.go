package demo

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"slices"
	"sort"
	"time"
	"unicode/utf8"

	"ryoku-i18n"

	"ryoku-rashin/internal/protocol"
)

// Reassignment only changes the in-memory demo. The native service owns real
// execution revocation, durable history and policy enforcement.
func (s *server) reassignTask(p protocol.TeamTaskReassignParams) (protocol.TeamTask, error) {
	t := &s.data.teams
	index := t.taskIndex(p.ID)
	if index < 0 {
		return protocol.TeamTask{}, notFound("task")
	}
	task := t.tasks[index]
	teamIndex := t.teamIndex(task.Team)
	if teamIndex < 0 {
		return protocol.TeamTask{}, notFound("team")
	}
	team := t.teams[teamIndex]
	member, known := protocol.MemberByID(team.Members, p.Member)
	if !known {
		return protocol.TeamTask{}, notFound("member")
	}

	// Include open descendants even beneath an already finished intermediate.
	family := map[string]bool{task.ID: true}
	for changed := true; changed; {
		changed = false
		for _, child := range t.tasks {
			if child.Team == team.ID && family[child.Parent] && !family[child.ID] {
				family[child.ID], changed = true, true
			}
		}
	}
	for child := range t.tasks {
		if family[t.tasks[child].ID] && taskOpen(t.tasks[child]) {
			s.stopTeamTask(child, i18n.T("You reassigned the task."))
		}
	}
	for approval := range t.approvals {
		if family[t.approvals[approval].Task] && t.approvals[approval].Status == protocol.ApprovalPending {
			s.settleApproval(approval, protocol.DecideDeny, i18n.T("The task was reassigned."))
		}
	}
	task = t.tasks[index]
	if err := s.archiveDemoTask(task); err != nil {
		return protocol.TeamTask{}, err
	}
	if task.Session != "" && !slices.Contains(task.Sessions, task.Session) {
		task.Sessions = append(append([]string(nil), task.Sessions...), task.Session)
	}
	task.Member, task.Status, task.Updated = member.ID, protocol.TaskReady, time.Now()
	task.Waiting, task.Reason, task.Session = "", "", ""
	task.Plan, task.Report, task.Started, task.Ended = nil, nil, nil, nil
	task.Model, task.Options, task.PermissionWaits = "", nil, nil
	task.Turns, task.Steps, task.Usage = 0, 0, protocol.TokenUsage{}
	task.Read, task.Automatic = false, false
	startEngine(&task, team, member, s.data.snapshot.Settings)
	s.record(protocol.Activity{Action: "task.reassign", Target: task.Title, OK: true, Team: team.ID, Task: task.ID})
	// A queued replacement resolves its model/options when it actually starts.
	task.Model, task.Options = "", nil
	if s.mayWork(team) && !member.Paused {
		task = s.beginTeamTask(team, task)
	}
	t.tasks[index] = task
	s.emitTask(task)
	return task, nil
}

// Use the existing task-message protocol to keep the exact previous result and
// strategy readable without turning demo history into a second execution engine.
func (s *server) archiveDemoTask(task protocol.TeamTask) error {
	raw, err := json.Marshal(task)
	if err != nil {
		return err
	}
	const maxBody = 64 << 10
	var chunks []string
	for len(raw) > 0 {
		end := min(len(raw), maxBody)
		for end < len(raw) && !utf8.RuneStart(raw[end]) {
			end--
		}
		chunks = append(chunks, string(raw[:end]))
		raw = raw[end:]
	}
	t := &s.data.teams
	id, now := t.newID("message-history"), time.Now()
	history := make([]protocol.TeamMessage, len(chunks), len(chunks)+len(t.messages))
	for index, chunk := range chunks {
		history[len(chunks)-index-1] = protocol.TeamMessage{
			ID: fmt.Sprintf("%s-%06d", id, index+1), Team: task.Team, Task: task.ID,
			Kind: "reassign", Title: fmt.Sprintf("Prior execution snapshot (%d/%d)", index+1, len(chunks)),
			Body: chunk, Created: now,
		}
	}
	t.messages = append(history, t.messages...)
	return nil
}

type demoMessageCursor struct {
	Team    string    `json:"team"`
	Task    string    `json:"task"`
	Created time.Time `json:"created"`
	ID      string    `json:"id"`
}

// This read is deliberately limited to the history that reassignment produces.
func (s *server) teamMessages(p protocol.TeamMessagesParams) (protocol.TeamMessagesResult, error) {
	t := &s.data.teams
	if p.Team != "" && t.teamIndex(p.Team) < 0 {
		return protocol.TeamMessagesResult{}, notFound("team")
	}
	if p.Task != "" {
		index := t.taskIndex(p.Task)
		if index < 0 {
			return protocol.TeamMessagesResult{}, notFound("task")
		}
		if p.Team != "" && t.tasks[index].Team != p.Team {
			return protocol.TeamMessagesResult{}, badRequest(i18n.T("The task does not belong to this team."))
		}
	}
	var cursor demoMessageCursor
	if p.Before != "" {
		if len(p.Before) > 4096 {
			return protocol.TeamMessagesResult{}, badRequest(i18n.T("The message cursor is not valid."))
		}
		raw, err := base64.RawURLEncoding.DecodeString(p.Before)
		if err != nil || json.Unmarshal(raw, &cursor) != nil || cursor.Created.IsZero() || cursor.ID == "" || cursor.Team != p.Team || cursor.Task != p.Task {
			return protocol.TeamMessagesResult{}, badRequest(i18n.T("The message cursor is not valid for this query."))
		}
	}
	messages := make([]protocol.TeamMessage, 0)
	for _, message := range t.messages {
		if p.Team != "" && message.Team != p.Team || p.Task != "" && message.Task != p.Task {
			continue
		}
		if p.Before == "" || message.Created.Before(cursor.Created) || message.Created.Equal(cursor.Created) && message.ID < cursor.ID {
			messages = append(messages, message)
		}
	}
	sort.Slice(messages, func(i, j int) bool {
		if messages[i].Created.Equal(messages[j].Created) {
			return messages[i].ID > messages[j].ID
		}
		return messages[i].Created.After(messages[j].Created)
	})
	limit := p.Limit
	if limit <= 0 {
		limit = 50
	}
	limit = min(limit, 200)
	result := protocol.TeamMessagesResult{Messages: messages}
	if len(messages) > limit {
		result.Messages = messages[:limit]
		last := result.Messages[limit-1]
		raw, err := json.Marshal(demoMessageCursor{Team: p.Team, Task: p.Task, Created: last.Created, ID: last.ID})
		if err != nil {
			return protocol.TeamMessagesResult{}, err
		}
		result.Next = base64.RawURLEncoding.EncodeToString(raw)
	}
	return result, nil
}
