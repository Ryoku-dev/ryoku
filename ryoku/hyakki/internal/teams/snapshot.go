package teams

import "ryoku-rashin/internal/protocol"

// What the subscribe snapshot carries of finished work; older entries are a
// teams.tasks or teams.approvals call away.
const (
	snapshotFinished = 50
	snapshotDecided  = 20
)

// snapshotTasks is every open task plus the newest finished ones.
func (s *Service) snapshotTasks() []protocol.TeamTask {
	out := make([]protocol.TeamTask, 0)
	finished := 0
	for _, t := range s.state.tasks(nil) {
		switch t.Status {
		case protocol.TaskDone, protocol.TaskBlocked, protocol.TaskStopped:
			if finished == snapshotFinished {
				continue
			}
			finished++
		}
		out = append(out, t)
	}
	return out
}

// snapshotApprovals is every pending approval plus the newest decided ones.
func (s *Service) snapshotApprovals() []protocol.Approval {
	out := make([]protocol.Approval, 0)
	decided := 0
	for _, rec := range s.state.approvals(nil) {
		if rec.Status != protocol.ApprovalPending {
			if decided == snapshotDecided {
				continue
			}
			decided++
		}
		out = append(out, rec.Approval)
	}
	return out
}

// emitTask tells clients a task changed.
func (s *Service) emitTask(t protocol.TeamTask) {
	s.broadcast(protocol.EvtTeamTask, t)
}
