package teams

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
	"unicode/utf8"

	"ryoku-rashin/internal/protocol"
)

type taskChange uint8

const (
	taskChangeReassign taskChange = iota
	taskChangeRetry
	taskChangeResume
)

// taskReassign is an owner operation. Every affected execution is durably
// stopped and its token revoked before waiting outside Service.mu for Stop.
// Publication then compares both the task and the current roster snapshot.
func (s *Service) taskReassign(ctx context.Context, p protocol.TeamTaskReassignParams) (protocol.TeamTask, error) {
	return s.changeTaskExecution(ctx, p, "", taskChangeReassign)
}

func (s *Service) changeTaskExecution(ctx context.Context, p protocol.TeamTaskReassignParams, note string, change taskChange) (protocol.TeamTask, error) {
	if err := ctx.Err(); err != nil {
		return protocol.TeamTask{}, err
	}
	// Visible revocation must fail fast rather than queue behind the Stop
	// whose caller is waiting for this callback to return.
	s.mu.Lock()
	editing := s.taskEdits[p.ID]
	s.mu.Unlock()
	if editing {
		return protocol.TeamTask{}, protocol.Errorf(protocol.CodeConflict, "This task is changing execution or waiting for storage repair.")
	}
	s.taskControl.Lock()
	defer s.taskControl.Unlock()
	if err := ctx.Err(); err != nil {
		return protocol.TeamTask{}, err
	}
	s.mu.Lock()
	if s.taskEdits[p.ID] {
		s.mu.Unlock()
		return protocol.TeamTask{}, protocol.Errorf(protocol.CodeConflict, "This task is changing execution or waiting for storage repair.")
	}
	original, ok := s.state.task(p.ID)
	if !ok {
		s.mu.Unlock()
		return protocol.TeamTask{}, protocol.Errorf(protocol.CodeNotFound, "No task %s.", p.ID)
	}
	if change != taskChangeReassign {
		if taskIsOpen(original) {
			s.mu.Unlock()
			return protocol.TeamTask{}, protocol.Errorf(protocol.CodeConflict, "Only a finished, blocked or stopped task runs again.")
		}
		p.Member = original.Member
	}
	team, ok := s.state.team(original.Team)
	if !ok {
		s.mu.Unlock()
		return protocol.TeamTask{}, protocol.Errorf(protocol.CodeNotFound, "No team %s.", original.Team)
	}
	member, known := protocol.MemberByID(team.Members, p.Member)
	if !known {
		s.mu.Unlock()
		return protocol.TeamTask{}, protocol.Errorf(protocol.CodeNotFound, "The assignee is not a current member of this team.")
	}
	if change == taskChangeResume && (s.settings().Teams.Paused || !s.memberBudgetReady(team, member) || s.taskAttemptExhausted(team, member, original)) {
		s.mu.Unlock()
		return protocol.TeamTask{}, protocol.Errorf(protocol.CodeConflict, "The standing task is paused or reached its explicit seat limits. The Summoner must review and retry it explicitly.")
	}
	if s.editing[team.ID][p.Member] {
		s.mu.Unlock()
		return protocol.TeamTask{}, protocol.Errorf(protocol.CodeConflict, "The assignee is being edited.")
	}
	// Descendants, not just immediate children, must lose delegated authority.
	// Closed descendants remain intact as history, including their parentage.
	all := s.state.tasks(func(task protocol.TeamTask) bool { return task.Team == team.ID })
	family := map[string]bool{original.ID: true}
	for changed := true; changed; {
		changed = false
		for _, task := range all {
			if family[task.Parent] && !family[task.ID] {
				family[task.ID], changed = true, true
			}
		}
	}
	affected := []protocol.TeamTask{original}
	for _, task := range all {
		if task.ID != original.ID && family[task.ID] && taskIsOpen(task) {
			affected = append(affected, task)
		}
	}
	for _, task := range affected {
		if s.taskEdits[task.ID] || s.editing[team.ID][task.Member] {
			s.mu.Unlock()
			return protocol.TeamTask{}, protocol.Errorf(protocol.CodeConflict, "An affected task or member is being edited.")
		}
	}
	if s.taskEdits == nil {
		s.taskEdits = make(map[string]bool)
	}
	for _, task := range affected {
		s.taskEdits[task.ID] = true
	}
	// Every failure after revocation retains the fences. In particular, an
	// unacknowledged Stop or lost final accounting is not a runnable retry.

	stopped := make([]protocol.TeamTask, 0, len(affected))
	var failure error
	for _, task := range affected {
		if err := s.retireTaskRequestsLocked(task.ID); err != nil && failure == nil {
			failure = err
		}
		next := cloneTask(task)
		if taskIsOpen(next) {
			now := s.now()
			next.Status, next.Waiting, next.Reason = protocol.TaskStopped, "", "The Summoner is reassigning the parent task."
			if task.ID == original.ID {
				next.Reason = "The Summoner is reassigning this task."
			}
			next.Ended, next.Updated = &now, now
			closePermissionWait(&next, now)
			if err := s.state.replaceTask(task, next); err != nil {
				if failure == nil {
					failure = err
				}
			} else {
				stopped = append(stopped, next)
			}
		} else {
			stopped = append(stopped, next)
		}
		if run := s.runs[task.ID]; run != nil {
			if run.timer != nil {
				run.timer.Stop()
			}
			delete(s.runs, task.ID)
		}
	}
	s.mu.Unlock()

	// Even if a write or Stop fails, stop every affected old session. Never
	// publish a replacement on any error. A stopped task is safe to retry.
	for _, task := range affected {
		if err := s.stopReassignedSessions(ctx, task); err != nil && failure == nil {
			failure = err
		}
	}
	for _, task := range stopped {
		s.emitTask(task)
		if err := s.denyReassignedApprovals(task.ID); err != nil && failure == nil {
			failure = err
		}
	}
	if failure != nil {
		return protocol.TeamTask{}, failure
	}
	if err := ctx.Err(); err != nil {
		return protocol.TeamTask{}, err
	}

	s.mu.Lock()
	for _, task := range affected {
		if err := s.taskEditErrors[task.ID]; err != nil {
			s.mu.Unlock()
			return protocol.TeamTask{}, err
		}
	}
	currentTeam, exists := s.state.team(team.ID)
	if !exists || !reflect.DeepEqual(currentTeam, team) || s.editing[team.ID][p.Member] {
		s.mu.Unlock()
		return protocol.TeamTask{}, protocol.Errorf(protocol.CodeConflict, "The team changed while the previous execution was stopping.")
	}
	for i, expected := range stopped {
		current, exists := s.state.task(expected.ID)
		matched := current
		matched.Usage, matched.Turns, matched.Steps = expected.Usage, expected.Turns, expected.Steps
		matched.Updated, matched.Read = expected.Updated, expected.Read
		// Approval retention may fold decided waits into durable task history
		// while the old sessions stop. This is accounting, not an owner edit.
		matched.PermissionWaits = expected.PermissionWaits
		if !exists || !reflect.DeepEqual(matched, expected) {
			s.mu.Unlock()
			return protocol.TeamTask{}, protocol.Errorf(protocol.CodeConflict, "An affected task changed while its execution was stopping.")
		}
		stopped[i] = current
	}
	currentTasks := s.state.tasks(func(task protocol.TeamTask) bool { return task.Team == team.ID })
	currentFamily := map[string]bool{original.ID: true}
	for changed := true; changed; {
		changed = false
		for _, task := range currentTasks {
			if currentFamily[task.Parent] && !currentFamily[task.ID] {
				currentFamily[task.ID], changed = true, true
			}
		}
	}
	for _, task := range currentTasks {
		if currentFamily[task.ID] && task.ID != original.ID && taskIsOpen(task) {
			s.mu.Unlock()
			return protocol.TeamTask{}, protocol.Errorf(protocol.CodeConflict, "New delegated work appeared while the previous execution was stopping.")
		}
	}
	if change == taskChangeResume && (s.settings().Teams.Paused || !s.memberBudgetReady(currentTeam, member) || s.taskAttemptExhausted(currentTeam, member, stopped[0])) {
		s.mu.Unlock()
		return protocol.TeamTask{}, protocol.Errorf(protocol.CodeConflict, "The standing task reached its explicit seat limits while stopping. The Summoner must review and retry it explicitly.")
	}
	current, _ := s.state.task(original.ID)
	if err := s.archiveTaskHistoryLocked(current); err != nil {
		s.mu.Unlock()
		return protocol.TeamTask{}, err
	}
	replacement := cloneTask(current)
	if replacement.Session != "" && !slices.Contains(replacement.Sessions, replacement.Session) {
		replacement.Sessions = append(replacement.Sessions, replacement.Session)
	}
	runAgain(&replacement, s.now())
	replacement.Member = p.Member
	// Strategy approval belongs to the old execution, not a transferable grant.
	// Its exact prior plan and report are in the durable execution timeline.
	replacement.Plan = nil
	if change == taskChangeResume {
		replacement.Turns, replacement.Started, replacement.Automatic = current.Turns, current.Started, current.Automatic
		replacement.Plan = current.Plan
		replacement.PermissionWaits = current.PermissionWaits
	}
	assignee, _ := protocol.MemberByID(currentTeam.Members, p.Member)
	replacement.MaxTurns = effectiveTaskTurns(currentTeam, assignee)
	replacement.Runtime = memberRuntimeKind(assignee)
	if note != "" {
		replacement.Brief += "\n\n" + s.redact(note)
	}
	if err := s.state.replaceTask(current, replacement); err != nil {
		s.mu.Unlock()
		return protocol.TeamTask{}, err
	}
	if err := s.state.writer.available(); err != nil {
		s.mu.Unlock()
		return protocol.TeamTask{}, err
	}
	// Release before pump, but only after the durable replacement exists.
	for _, task := range affected {
		delete(s.taskEdits, task.ID)
		delete(s.taskEditErrors, task.ID)
	}
	s.mu.Unlock()
	actor, initiator := protocol.InitiatorYou, protocol.InitiatorYou
	action, detail := "task.reassign", "The previous execution stopped before the new assignee was published."
	if change == taskChangeRetry {
		action, detail = "task.retry", "The previous execution stopped and was archived before a new conversation started."
	} else if change == taskChangeResume {
		actor, initiator = "Hyakki", protocol.InitiatorRashin
		action, detail = "task.resume", "Resumed the standing task without refreshing its explicit seat limits."
	}
	s.record(protocol.Activity{Actor: actor, Action: action, Target: replacement.Title, OK: true, Team: replacement.Team, Task: replacement.ID, Initiator: initiator, Detail: detail})
	s.emitTask(replacement)
	s.pump()
	s.clock.rearm()
	if latest, ok := s.state.task(replacement.ID); ok {
		replacement = latest
	}
	return replacement, nil
}

// Unlike denyPending's best-effort end notification, a reassignment cannot
// silently carry a pending old-session approval into a replacement execution.
func (s *Service) denyReassignedApprovals(task string) error {
	s.mu.Lock()
	var decided []protocol.Approval
	var failure error
	for _, rec := range s.state.approvals(func(rec approvalRecord) bool { return rec.Task == task && rec.Status == protocol.ApprovalPending }) {
		now := s.now()
		rec.Status, rec.Decision, rec.Note, rec.Decided = protocol.ApprovalDenied, protocol.DecideDeny, "The previous task execution was stopped.", &now
		if err := s.state.putApproval(rec); err != nil {
			failure = err
			break
		}
		decided = append(decided, rec.Approval)
	}
	s.mu.Unlock()
	for _, approval := range decided {
		s.emitApproval(approval)
	}
	return failure
}

// archiveTaskHistoryLocked preserves the actual previous execution, including
// reports larger than one message. Consecutive numbered chunks reconstruct the
// JSON snapshot exactly. Stable content IDs make an interrupted append retry
// idempotent; no fake report is substituted when storage fails. Caller holds mu.
func (s *Service) archiveTaskHistoryLocked(task protocol.TeamTask) error {
	// Redact string leaves before encoding: multiline secrets must not escape
	// detection merely because JSON replaced their newlines with backslashes.
	raw, err := json.Marshal(task)
	if err != nil {
		return err
	}
	var value any
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(&value); err != nil {
		return err
	}
	var redact func(any) any
	redact = func(value any) any {
		switch value := value.(type) {
		case string:
			return s.redact(value)
		case []any:
			for i := range value {
				value[i] = redact(value[i])
			}
		case map[string]any:
			for key := range value {
				value[key] = redact(value[key])
			}
		}
		return value
	}
	raw, err = json.Marshal(redact(value))
	if err != nil {
		return err
	}
	digest := sha256.Sum256(raw)
	prefix := "message-history-" + hex.EncodeToString(digest[:])
	var chunks []string
	for len(raw) > 0 {
		end := min(len(raw), maxTeamMessageBody)
		for end < len(raw) && !utf8.RuneStart(raw[end]) {
			end--
		}
		chunks = append(chunks, string(raw[:end]))
		raw = raw[end:]
	}
	existing := make(map[string]protocol.TeamMessage)
	for _, message := range s.state.messages(func(message protocol.TeamMessage) bool { return message.Task == task.ID && message.Kind == "reassign" }) {
		existing[message.ID] = message
	}
	at := s.now()
	for i, chunk := range chunks {
		message := protocol.TeamMessage{ID: fmt.Sprintf("%s-%06d", prefix, i+1), Team: task.Team, Task: task.ID, Kind: "reassign", Title: fmt.Sprintf("Prior execution snapshot (%d/%d)", i+1, len(chunks)), Body: chunk, Created: at}
		if old, ok := existing[message.ID]; ok {
			if old.Body != message.Body || old.Title != message.Title || old.Team != message.Team {
				return protocol.Errorf(protocol.CodeConflict, "The execution history changed.")
			}
			continue
		}
		if err := s.state.appendMessage(message); err != nil {
			return err
		}
	}
	return nil
}

// Stop every recorded session even when one returns an error. An old process
// that has not acknowledged Stop can never be treated as quiescent.
func (s *Service) stopReassignedSessions(ctx context.Context, task protocol.TeamTask) error {
	if s.sessions == nil {
		if task.Session != "" || len(task.Sessions) != 0 {
			return protocol.Errorf(protocol.CodeConflict, "The session manager is unavailable; the previous execution cannot be confirmed stopped.")
		}
		return nil
	}
	ids := slices.Clone(task.Sessions)
	if task.Session != "" && !slices.Contains(ids, task.Session) {
		ids = append(ids, task.Session)
	}
	var failure error
	seen := make(map[string]bool)
	for _, id := range ids {
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		info, exists := s.sessions.Info(id)
		if !exists {
			continue
		}
		if info.Goal != nil && (info.Goal.Status == protocol.GoalActive || info.Goal.Status == protocol.GoalPaused) {
			if _, err := s.sessions.Goal(ctx, protocol.GoalParams{ID: id, Op: protocol.GoalStop}); err != nil && failure == nil {
				failure = err
			}
		}
		if err := s.sessions.Stop(ctx, id); err != nil && failure == nil {
			failure = err
		}
	}
	return failure
}
