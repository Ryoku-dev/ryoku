package teams

import (
	"context"
	"fmt"
	"strings"
	"time"

	"ryoku-rashin/internal/protocol"
)

// automaticSeatReadyLocked is the common event admission barrier. Callers hold
// Service.mu through task persistence so edits cannot publish a stale assignee.
func (s *Service) automaticSeatReadyLocked(team protocol.Team, member protocol.Member, mode string) bool {
	if s.background.Err() != nil || s.settings().Teams.Paused || team.Paused || member.Paused || s.editing[team.ID][member.ID] || !memberWakeAllowed(member, mode) || !s.dreamAvailable(team) {
		return false
	}
	for taskID, editing := range s.taskEdits {
		if task, ok := s.state.task(taskID); editing && ok && task.Team == team.ID && task.Member == member.ID {
			return false
		}
	}
	return s.memberBudgetReady(team, member)
}

func routineSeat(team protocol.Team, r protocol.Routine) (protocol.Member, bool) {
	if r.Member != "" {
		return protocol.MemberByID(team.Members, r.Member)
	}
	member := teamLead(team)
	return member, member.ID != ""
}

func (s *Service) routineAutomaticAllowed(r protocol.Routine) bool {
	if !s.routineAvailable(r) {
		return false
	}
	if routineKind(r) != protocol.RoutineTask {
		return true
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	team, ok := s.state.team(r.Team)
	member, known := routineSeat(team, r)
	return ok && known && s.automaticSeatReadyLocked(team, member, protocol.HeartbeatRoutine)
}

// wakeMessage accepts only the durable, addressed request. A requested peer is
// context for the manager, never an assignee. Tasks commit before delivery is
// marked; their persisted Trigger closes the crash window between those writes.
func (s *Service) wakeMessage(ctx context.Context, message protocol.TeamMessage) (protocol.TeamTask, bool, error) {
	if err := ctx.Err(); err != nil {
		return protocol.TeamTask{}, false, err
	}
	s.mu.Lock()
	messages := s.state.messages(func(m protocol.TeamMessage) bool { return m.ID == message.ID })
	if len(messages) == 0 {
		s.mu.Unlock()
		return protocol.TeamTask{}, false, protocol.Errorf(protocol.CodeNotFound, "No message %s.", message.ID)
	}
	message = messages[0]
	if message.Retired {
		s.mu.Unlock()
		return protocol.TeamTask{}, false, nil
	}
	matched := s.state.tasks(func(task protocol.TeamTask) bool {
		return task.Team == message.Team && task.Origin == protocol.TaskFromMessage && task.Trigger == message.ID
	})
	if len(matched) > 0 {
		s.mu.Unlock()
		return matched[0], true, nil
	}
	if message.Delivered {
		s.mu.Unlock()
		return protocol.TeamTask{}, true, nil
	}
	team, ok := s.state.team(message.Team)
	member, known := protocol.MemberByID(team.Members, message.To)
	source, exists := s.state.task(message.Task)
	// Durable execution identity survives a daemon restart, unlike a run token.
	// Missing legacy identity, closed work and replacement assignees fail closed.
	if !exists || !taskIsOpen(source) || source.Team != message.Team || source.Member != message.From || message.SourceSession == "" || source.Session != message.SourceSession {
		err := s.state.retireMessage(message.ID)
		s.mu.Unlock()
		return protocol.TeamTask{}, false, err
	}
	from, sender := protocol.MemberByID(team.Members, message.From)
	manager := ""
	if sender {
		switch message.Kind {
		case "help":
			if _, peer := protocol.MemberByID(team.Members, message.Member); peer && message.Member != message.From {
				manager = protocol.CommonManager(team, message.From, message.Member)
			}
		case "escalate":
			manager = from.ReportsTo
		}
	}
	if !ok || !known || !exists || source.Team != team.ID || message.To == "" || manager != message.To || s.taskEdits[source.ID] || !s.automaticSeatReadyLocked(team, member, protocol.HeartbeatMessage) {
		s.mu.Unlock()
		return protocol.TeamTask{}, false, nil
	}
	// Serialize manager attention instead of flooding a seat during recovery.
	if len(s.state.tasks(func(task protocol.TeamTask) bool {
		return task.Team == team.ID && task.Member == member.ID && task.Origin == protocol.TaskFromMessage && taskIsOpen(task)
	})) > 0 {
		s.mu.Unlock()
		return protocol.TeamTask{}, false, nil
	}
	brief := fmt.Sprintf("%s\n\n%s\n\nRequest from %s concerning task %s. Requested peer: %s. Decide through the current reporting lines; this request does not assign that peer.", message.Title, message.Body, message.From, source.ID, message.Member)
	task, _, err := s.newTaskLocked(team.ID, member.ID, message.Title, brief, protocol.TaskFromMessage, message.ID, source.ID, "", nil, true)
	s.mu.Unlock()
	if err != nil {
		return protocol.TeamTask{}, false, err
	}
	s.publishWakeTask(task)
	return task, true, nil
}

func (s *Service) newRoutineTask(ctx context.Context, r protocol.Routine, brief, initiator string) (protocol.TeamTask, error) {
	if err := ctx.Err(); err != nil {
		return protocol.TeamTask{}, err
	}
	if initiator == protocol.InitiatorYou {
		return s.newTask(ctx, r.Team, r.Member, r.Title, brief, protocol.TaskFromRoutine, r.ID, "", "", nil)
	}
	s.clock.edit.Lock()
	s.mu.Lock()
	team, ok := s.state.team(r.Team)
	member, known := routineSeat(team, r)
	current, exists := s.state.routine(r.ID)
	if !ok || !known || !exists || !current.Enabled || !current.Updated.Equal(r.Updated) || current.Member != r.Member || current.Schedule != r.Schedule || current.Brief != r.Brief || current.Title != r.Title || !s.automaticSeatReadyLocked(team, member, protocol.HeartbeatRoutine) {
		s.mu.Unlock()
		s.clock.edit.Unlock()
		return protocol.TeamTask{}, protocol.Errorf(protocol.CodeConflict, "The routine's automatic wake is parked or changed.")
	}
	task, _, err := s.newTaskLocked(team.ID, member.ID, r.Title, brief, protocol.TaskFromRoutine, r.ID, "", "", nil, true)
	s.mu.Unlock()
	s.clock.edit.Unlock()
	if err != nil {
		return protocol.TeamTask{}, err
	}
	s.publishWakeTask(task)
	if current, ok := s.state.task(task.ID); ok {
		task = current
	}
	return task, nil
}

func (s *Service) publishWakeTask(task protocol.TeamTask) {
	actor, initiator := s.taskActor(task)
	s.record(protocol.Activity{Actor: actor, Action: "task.create", Target: task.Title, OK: true, Team: task.Team, Task: task.ID, Initiator: initiator})
	s.emitTask(task)
	s.pump()
}

// Called only by bind/rearm, never a second polling service. Oldest first,
// with at most one open message task per seat; parked requests stay pending.
func (s *Service) recoverHeartbeatMessages() {
	c := s.clock
	c.mu.Lock()
	if !c.running || c.recovering || s.background.Err() != nil {
		c.mu.Unlock()
		return
	}
	c.recovering = true
	c.mu.Unlock()
	defer func() {
		c.mu.Lock()
		c.recovering = false
		c.mu.Unlock()
	}()
	messages := s.state.messages(func(message protocol.TeamMessage) bool {
		return !message.Delivered && !message.Retired && message.To != ""
	})
	for i := len(messages) - 1; i >= 0; i-- {
		message := messages[i]
		_, accepted, err := s.wakeMessage(s.background, message)
		if err == nil && accepted {
			err = s.state.deliverMessage(message.ID)
		}
		if err != nil {
			s.recordWakeError(message.Team, message.Task, err)
		}
	}
}

func (s *Service) recordWakeError(team, task string, err error) {
	s.record(protocol.Activity{Actor: "Hyakki", Action: "heartbeat.error", OK: false, Team: team, Task: task, Initiator: protocol.InitiatorRashin, Detail: s.redact(err.Error())})
}

type heartbeatPlanError struct {
	team string
	err  error
}

// intervalPlans persists the first due time. A live pause/resume starts a fresh
// interval; restart preserves the existing time and coalesces downtime to one run.
// It executes outside scheduler.mu, including all Service callbacks and IO.
func (s *Service) intervalPlans(now time.Time, initial bool, active map[string]bool) (map[string]clockEntry, []heartbeatPlanError) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make(map[string]clockEntry)
	var failures []heartbeatPlanError
	for _, team := range s.state.teams() {
		for _, member := range team.Members {
			if !s.automaticSeatReadyLocked(team, member, protocol.HeartbeatInterval) || member.Heartbeat == nil || checkMemberHeartbeat(member.Heartbeat) != nil {
				continue
			}
			id := heartbeatKey(team.ID, member.ID)
			every := member.Heartbeat.EveryMinutes
			record, exists, err := s.state.heartbeatSchedule(team.ID, member.ID)
			if err != nil {
				failures = append(failures, heartbeatPlanError{team: team.ID, err: err})
				continue
			}
			if !exists || record.EveryMinutes != every || (!initial && !active[id]) {
				record = heartbeatSchedule{Team: team.ID, Member: member.ID, EveryMinutes: every, Next: now.Add(time.Duration(every) * time.Minute)}
				if err := s.state.putHeartbeatSchedule(record); err != nil {
					failures = append(failures, heartbeatPlanError{team: team.ID, err: err})
					continue
				}
			}
			out[id] = clockEntry{at: record.Next, team: team.ID, member: member.ID, everyMinutes: every}
		}
	}
	return out, failures
}

func (s *Service) checkHeartbeat(entry clockEntry, now time.Time) {
	s.mu.Lock()
	team, ok := s.state.team(entry.team)
	member, known := protocol.MemberByID(team.Members, entry.member)
	if !ok || !known || !s.automaticSeatReadyLocked(team, member, protocol.HeartbeatInterval) || member.Heartbeat == nil || member.Heartbeat.EveryMinutes != entry.everyMinutes {
		s.mu.Unlock()
		return
	}
	record, exists, err := s.state.heartbeatSchedule(team.ID, member.ID)
	if err != nil || !exists || !record.Next.Equal(entry.at) || record.Next.After(now) {
		s.mu.Unlock()
		if err != nil {
			s.recordWakeError(team.ID, "", err)
		}
		return
	}
	trigger := heartbeatKey(team.ID, member.ID) + "/" + entry.at.UTC().Format(time.RFC3339Nano)
	existing := s.state.tasks(func(task protocol.TeamTask) bool {
		return task.Team == team.ID && task.Origin == protocol.TaskFromHeartbeat && (task.Trigger == trigger || task.Member == member.ID && taskIsOpen(task))
	})
	var task protocol.TeamTask
	if len(existing) == 0 {
		brief := strings.TrimSpace(member.Heartbeat.Instructions)
		if brief == "" {
			brief = "Review the team goal and your current responsibilities, report useful changes, and escalate blockers through your manager."
		}
		task, _, err = s.newTaskLocked(team.ID, member.ID, "Interval review", brief, protocol.TaskFromHeartbeat, trigger, "", "", nil, true)
	}
	if err == nil {
		record.Next = now.Add(time.Duration(record.EveryMinutes) * time.Minute)
		err = s.state.putHeartbeatSchedule(record)
	}
	s.mu.Unlock()
	if task.ID != "" {
		s.publishWakeTask(task)
	}
	if err != nil {
		s.recordWakeError(team.ID, task.ID, err)
	}
}
