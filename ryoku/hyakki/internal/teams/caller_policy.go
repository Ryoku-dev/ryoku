package teams

import (
	"crypto/subtle"

	"ryoku-rashin/internal/protocol"
)

// currentCallerLocked checks an already authenticated in-process identity against
// the live execution generation. The caller holds Service.mu, never a session lock.
func (s *Service) currentCallerLocked(c caller) (protocol.Team, protocol.Member, protocol.TeamTask, error) {
	r := s.runs[c.Task]
	if r == nil || r.Team != c.Team || r.Member != c.Member || r.Session == "" || r.Session != c.Session || c.Token == "" || subtle.ConstantTimeCompare([]byte(r.Token), []byte(c.Token)) != 1 || s.taskEdits[c.Task] || s.editing[c.Team][c.Member] {
		return protocol.Team{}, protocol.Member{}, protocol.TeamTask{}, protocol.Errorf(protocol.CodeConflict, "This execution attempt is no longer authorized.")
	}
	task, exists := s.state.task(c.Task)
	team, known := s.state.team(c.Team)
	member, memberExists := protocol.MemberByID(team.Members, c.Member)
	if !exists || !taskIsOpen(task) || task.Session != c.Session || task.Member != c.Member || !known || !memberExists || team.Paused || s.settings().Teams.Paused || taskTurnsExhausted(team, member, task) || !s.memberBudgetReady(team, member) {
		return protocol.Team{}, protocol.Member{}, protocol.TeamTask{}, protocol.Errorf(protocol.CodeConflict, "This task's current execution policy no longer permits work.")
	}
	return team, member, task, nil
}
