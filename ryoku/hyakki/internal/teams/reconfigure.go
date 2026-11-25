package teams

import (
	"context"
	"reflect"

	"ryoku-rashin/internal/protocol"
)

func reconfiguredMembers(before, next protocol.Team) map[string]bool {
	global := before.Folder != next.Folder || before.Approvals != next.Approvals || before.Tools != next.Tools ||
		before.Reporting != next.Reporting || before.PlanApproval != next.PlanApproval ||
		!reflect.DeepEqual(before.Access, next.Access) || !reflect.DeepEqual(before.SkillsOff, next.SkillsOff) || before.TeamMemory() != next.TeamMemory()
	for _, member := range before.Members {
		after, exists := budgetMember(next, member.ID)
		if !exists || member.ReportsTo != after.ReportsTo || member.Lead != after.Lead {
			global = true
			break
		}
	}
	affected := make(map[string]bool)
	for _, member := range before.Members {
		after, exists := budgetMember(next, member.ID)
		changed := global || !exists || member.Trust != after.Trust || member.Paused != after.Paused ||
			!reflect.DeepEqual(member.Tools, after.Tools) || !reflect.DeepEqual(member.Skills, after.Skills) ||
			!reflect.DeepEqual(member.Runtime, after.Runtime) || !reflect.DeepEqual(member.Heartbeat, after.Heartbeat)
		if !changed && member.Budget != nil && after.Budget != nil {
			changed = member.Budget.TaskTurns != after.Budget.TaskTurns
		} else if !changed && (member.Budget == nil) != (after.Budget == nil) {
			changed = true
		}
		if changed {
			affected[member.ID] = true
		}
	}
	return affected
}

// beginTeamEdit revokes the old execution capability before the new settings
// become observable. No lock is held while session shutdown waits on callbacks.
func (s *Service) beginTeamEdit(ctx context.Context, before, next protocol.Team) (bool, error) {
	affected := reconfiguredMembers(before, next)
	s.mu.Lock()
	current, exists := s.state.team(before.ID)
	if !exists || !reflect.DeepEqual(current, before) || s.editing[before.ID] != nil {
		s.mu.Unlock()
		return false, protocol.Errorf(protocol.CodeConflict, "The team changed while it was being edited. Reload before saving.")
	}
	for id := range s.taskEdits {
		task, exists := s.state.task(id)
		if !exists || task.Team == before.ID {
			s.mu.Unlock()
			return false, protocol.Errorf(protocol.CodeConflict, "A task is changing execution. Wait for it to stop before editing this team.")
		}
	}
	if len(affected) == 0 {
		s.mu.Unlock()
		return false, nil
	}
	s.editing[before.ID] = affected
	var tasks []string
	for id, run := range s.runs {
		if run.Team == before.ID && affected[run.Member] {
			tasks = append(tasks, id)
		}
	}
	s.mu.Unlock()
	for _, id := range tasks {
		ended, stopped := s.endTaskIf(id, func(task protocol.TeamTask) bool {
			return task.Team == before.ID && s.editing[before.ID][task.Member]
		}, func(task *protocol.TeamTask) {
			task.Status, task.Reason = protocol.TaskStopped, "Its execution configuration changed. Retry with the new settings."
		})
		if ended.Session != "" && s.sessions != nil {
			if _, exists := s.sessions.Info(ended.Session); exists {
				if err := s.sessions.Stop(ctx, ended.Session); err != nil {
					s.endTeamEdit(before.ID)
					return false, err
				}
			}
		}
		if !stopped {
			if task, exists := s.state.task(id); exists && taskIsOpen(task) {
				s.endTeamEdit(before.ID)
				return false, protocol.Errorf(protocol.CodeUnavailable, "The old task could not be stopped durably; its configuration was not replaced.")
			}
			continue
		}
		s.denyPending(id, "Its execution configuration changed.")
		s.afterEnd(ended)
	}
	return true, nil
}

func (s *Service) endTeamEdit(team string) {
	s.mu.Lock()
	if s.state.writer.available() != nil {
		s.mu.Unlock()
		return
	}
	delete(s.editing, team)
	s.mu.Unlock()
	s.pump()
	s.clock.rearm()
}
