package teams

import (
	"context"
	"errors"

	"ryoku-rashin/internal/protocol"
)

// stopTask revokes the whole task family before synchronously stopping every
// recorded execution. Closed ancestors do not exempt their descendants.
func (s *Service) stopTask(ctx context.Context, id, reason string) (protocol.TeamTask, error) {
	if err := ctx.Err(); err != nil {
		return protocol.TeamTask{}, err
	}
	s.mu.Lock()
	root, exists := s.state.task(id)
	if !exists {
		s.mu.Unlock()
		return protocol.TeamTask{}, protocol.Errorf(protocol.CodeNotFound, "No task %s.", id)
	}
	all := s.state.tasks(func(task protocol.TeamTask) bool { return task.Team == root.Team })
	family := map[string]bool{id: true}
	for changed := true; changed; {
		changed = false
		for _, task := range all {
			if family[task.Parent] && !family[task.ID] {
				family[task.ID], changed = true, true
			}
		}
	}
	for _, task := range all {
		if family[task.ID] && s.taskEdits[task.ID] && (taskIsOpen(task) || s.runs[task.ID] != nil) {
			s.mu.Unlock()
			return root, protocol.Errorf(protocol.CodeConflict, "An affected task is changing execution or waiting for storage repair.")
		}
	}
	if s.taskEdits == nil {
		s.taskEdits = make(map[string]bool)
	}
	owned := make(map[string]bool)
	var ended []protocol.TeamTask
	var failure error
	for _, task := range all {
		if !family[task.ID] {
			continue
		}
		if !s.taskEdits[task.ID] {
			owned[task.ID] = true
			s.taskEdits[task.ID] = true
		}
		failure = errors.Join(failure, s.retireTaskRequestsLocked(task.ID))
		if taskIsOpen(task) {
			next := cloneTask(task)
			now := s.now()
			next.Status, next.Waiting, next.Reason = protocol.TaskStopped, "", reason
			next.Ended, next.Updated = &now, now
			closePermissionWait(&next, now)
			if task.ID != id {
				next.Reason = "The parent task was stopped."
			}
			if err := s.state.replaceTask(task, next); err != nil {
				failure = errors.Join(failure, err)
			} else {
				ended = append(ended, next)
			}
		}
		if run := s.runs[task.ID]; run != nil {
			if run.timer != nil {
				run.timer.Stop()
			}
			delete(s.runs, task.ID)
		}
	}
	s.mu.Unlock()
	for _, task := range all {
		if family[task.ID] {
			failure = errors.Join(failure, s.stopReassignedSessions(ctx, task))
			failure = errors.Join(failure, s.denyReassignedApprovals(task.ID))
		}
	}
	s.mu.Lock()
	for task := range family {
		failure = errors.Join(failure, s.taskEditErrors[task])
	}
	failure = errors.Join(failure, s.state.writer.available())
	if failure == nil {
		for task := range owned {
			delete(s.taskEdits, task)
			delete(s.taskEditErrors, task)
		}
	}
	current, _ := s.state.task(id)
	s.mu.Unlock()
	for _, task := range ended {
		if latest, ok := s.state.task(task.ID); ok {
			task = latest
		}
		s.countEnded(task)
		s.emitTask(task)
	}
	if failure != nil {
		return current, failure
	}
	actor, initiator := "Hyakki", protocol.InitiatorRashin
	if stoppedByYou(reason) {
		actor, initiator = protocol.InitiatorYou, protocol.InitiatorYou
	}
	for _, task := range ended {
		s.record(protocol.Activity{Actor: actor, Action: "task.stop", Target: task.Title, OK: true, Team: task.Team, Task: task.ID, Initiator: initiator, Detail: s.redact(task.Reason)})
		s.afterEnd(task)
	}
	return current, nil
}

func (s *Service) stopTaskByOwner(ctx context.Context, id, reason string) (protocol.TeamTask, error) {
	s.mu.Lock()
	editing := s.taskEdits[id]
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
	editing = s.taskEdits[id]
	s.mu.Unlock()
	if editing {
		return protocol.TeamTask{}, protocol.Errorf(protocol.CodeConflict, "This task is changing execution or waiting for storage repair.")
	}
	return s.stopTask(ctx, id, reason)
}

// deleteTask fences the complete subtree before waiting on sessions. Callback
// bookkeeping can finish, but no late message, delegate or owner reassignment
// can recreate work while its files and references are being removed.
func (s *Service) deleteTask(ctx context.Context, id string) error {
	s.mu.Lock()
	editing := s.taskEdits[id]
	s.mu.Unlock()
	if editing {
		return protocol.Errorf(protocol.CodeConflict, "This task is changing execution or waiting for storage repair.")
	}
	s.taskControl.Lock()
	defer s.taskControl.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	root, exists := s.state.task(id)
	if !exists {
		s.mu.Unlock()
		return protocol.Errorf(protocol.CodeNotFound, "No task %s.", id)
	}
	all := s.state.tasks(func(task protocol.TeamTask) bool { return task.Team == root.Team })
	family := map[string]bool{id: true}
	for changed := true; changed; {
		changed = false
		for _, task := range all {
			if family[task.Parent] && !family[task.ID] {
				family[task.ID], changed = true, true
			}
		}
	}
	for task := range family {
		if s.taskEdits[task] {
			s.mu.Unlock()
			return protocol.Errorf(protocol.CodeConflict, "An affected task is changing execution or waiting for storage repair.")
		}
	}
	if s.taskEdits == nil {
		s.taskEdits = make(map[string]bool)
	}
	for task := range family {
		s.taskEdits[task] = true
	}
	var failure error
	var stopped []protocol.TeamTask
	for _, task := range all {
		if !family[task.ID] {
			continue
		}
		failure = errors.Join(failure, s.retireTaskRequestsLocked(task.ID))
		if taskIsOpen(task) {
			next := cloneTask(task)
			now := s.now()
			next.Status, next.Waiting, next.Reason = protocol.TaskStopped, "", "You deleted it."
			next.Ended, next.Updated = &now, now
			closePermissionWait(&next, now)
			if err := s.state.replaceTask(task, next); err != nil {
				failure = errors.Join(failure, err)
			} else {
				stopped = append(stopped, next)
			}
		}
		if run := s.runs[task.ID]; run != nil {
			if run.timer != nil {
				run.timer.Stop()
			}
			delete(s.runs, task.ID)
		}
	}
	s.mu.Unlock()
	for _, task := range all {
		if family[task.ID] {
			if err := s.stopReassignedSessions(ctx, task); err != nil {
				failure = errors.Join(failure, err)
			}
		}
	}
	for _, task := range stopped {
		s.emitTask(task)
		if err := s.denyReassignedApprovals(task.ID); err != nil {
			failure = errors.Join(failure, err)
		}
	}
	s.mu.Lock()
	for task := range family {
		failure = errors.Join(failure, s.taskEditErrors[task])
	}
	s.mu.Unlock()
	if failure != nil {
		return failure
	}
	if err := s.deleteTaskTree(ctx, id); err != nil {
		return err
	}
	s.mu.Lock()
	if err := s.state.writer.available(); err != nil {
		s.mu.Unlock()
		return err
	}
	for task := range family {
		delete(s.taskEdits, task)
		delete(s.taskEditErrors, task)
	}
	s.mu.Unlock()
	s.pump()
	s.clock.rearm()
	return nil
}
