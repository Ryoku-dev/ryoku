package teams

import "ryoku-rashin/internal/protocol"

func taskTurnLimit(team protocol.Team, member protocol.Member) int {
	if isDream(team) && (member.Budget == nil || member.Budget.TaskTurns <= 0) {
		return 0
	}
	return effectiveTaskTurns(team, member)
}

func taskTurnsExhausted(team protocol.Team, member protocol.Member, task protocol.TeamTask) bool {
	limit := taskTurnLimit(team, member)
	return limit > 0 && task.Turns >= limit
}

func taskGoalTurns(team protocol.Team, member protocol.Member, task protocol.TeamTask) int {
	limit := taskTurnLimit(team, member)
	if limit <= 0 {
		return effectiveTaskTurns(team, member)
	}
	return max(limit-task.Turns, 0)
}

func (s *Service) taskAttemptExhausted(team protocol.Team, member protocol.Member, task protocol.TeamTask) bool {
	if taskTurnsExhausted(team, member, task) {
		return true
	}
	minutes := effectiveTaskMinutes(team, member)
	return minutes > 0 && task.Started != nil && !s.now().Before(s.minutesEnd(task, minutes))
}

// A model's goal changes never refresh the task's effective turn allowance.
// Only an explicit owner retry starts a new attempt after its history is kept.
func (s *Service) enforceTaskTurns(info protocol.SessionInfo) {
	ended, ok := s.endTaskIf(info.TeamTask, func(task protocol.TeamTask) bool {
		if task.Session != info.ID || s.taskEdits[task.ID] {
			return false
		}
		team, exists := s.state.team(task.Team)
		member, known := protocol.MemberByID(team.Members, task.Member)
		if !exists || !known || !taskTurnsExhausted(team, member, task) {
			return false
		}
		return true
	}, func(task *protocol.TeamTask) {
		task.Status, task.Reason = protocol.TaskBlocked, "It reached its effective task turn limit. The Summoner must review and retry explicitly."
	})
	if !ok {
		return
	}
	s.halt(ended.Session)
	s.record(protocol.Activity{Actor: "Hyakki", Action: "task.turn-limit", Target: ended.Title, OK: false, Team: ended.Team, Task: ended.ID, Initiator: protocol.InitiatorRashin, Detail: "The task stopped at its configured turn ceiling."})
	s.notifyEnd(ended)
	s.afterEnd(ended)
}
