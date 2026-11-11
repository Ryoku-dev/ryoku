package demo

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"ryoku-i18n"

	"ryoku-rashin/internal/cron"
	"ryoku-rashin/internal/protocol"
)

// routines read their next run off the demo's clock, so a rendered page shows the same times on any day
func demoRoutineClock() time.Time { return Epoch.In(time.Local) }

func teamsMethod(method string) bool {
	return strings.HasPrefix(method, "teams.") || strings.HasPrefix(method, "vault.")
}

func badRequest(text string) error { return protocol.Errorf(protocol.CodeBadRequest, "%s", text) }

func (t *teamsFixtures) teamIndex(id string) int {
	for index := range t.teams {
		if t.teams[index].ID == id {
			return index
		}
	}
	return -1
}

func (t *teamsFixtures) taskIndex(id string) int {
	for index := range t.tasks {
		if t.tasks[index].ID == id {
			return index
		}
	}
	return -1
}

func (t *teamsFixtures) routineIndex(id string) int {
	for index := range t.routines {
		if t.routines[index].ID == id {
			return index
		}
	}
	return -1
}

func (t *teamsFixtures) approvalIndex(id string) int {
	for index := range t.approvals {
		if t.approvals[index].ID == id {
			return index
		}
	}
	return -1
}

func (t *teamsFixtures) credentialIndex(id string) int {
	for index := range t.vault.Credentials {
		if t.vault.Credentials[index].ID == id {
			return index
		}
	}
	return -1
}

func (t *teamsFixtures) sessionIndex(id string) int {
	for index := range t.sessions {
		if t.sessions[index].ID == id {
			return index
		}
	}
	return -1
}

// the next id of a kind, told apart from the fixtures' by its counter
func (t *teamsFixtures) newID(prefix string) string {
	t.made++
	return fmt.Sprintf("%s-demo-%d", prefix, t.made)
}

func taskOpen(task protocol.TeamTask) bool {
	return task.Status != protocol.TaskDone && task.Status != protocol.TaskBlocked && task.Status != protocol.TaskStopped
}

// a team with the stats the daemon derives on every read
func (s *server) teamWithStats(team protocol.Team) protocol.Team {
	t := &s.data.teams
	stats := protocol.TeamStats{MonthTokens: t.spend[team.ID].Tokens}
	for _, task := range t.tasks {
		if task.Team != team.ID {
			continue
		}
		switch task.Status {
		case protocol.TaskWorking:
			stats.Working++
			if stats.Doing == nil {
				stats.Doing = make(map[string]string)
			}
			// a lead waiting on its members is doing less than a member at work
			if _, busy := stats.Doing[task.Member]; !busy || task.Waiting == "" {
				stats.Doing[task.Member] = task.Title
			}
		case protocol.TaskReady:
			stats.Queued++
		case protocol.TaskNeedsYou:
			stats.NeedsYou++
		case protocol.TaskDone:
			stats.Done++
		}
		if task.Report != nil && task.Parent == "" && (stats.LastReport == nil || task.Report.At.After(*stats.LastReport)) {
			at := task.Report.At
			stats.LastReport = &at
		}
	}
	for _, routine := range t.routines {
		if routine.Team != team.ID {
			continue
		}
		if next := s.readRoutine(routine).NextRun; next != nil && (stats.NextRoutine == nil || next.Before(*stats.NextRoutine)) {
			stats.NextRoutine = next
		}
	}
	team.Stats = stats
	return team
}

func (s *server) listTeams() []protocol.Team {
	teams := make([]protocol.Team, 0, len(s.data.teams.teams))
	for _, team := range s.data.teams.teams {
		teams = append(teams, s.teamWithStats(team))
	}
	sort.SliceStable(teams, func(i, j int) bool { return teams[i].Name < teams[j].Name })
	return teams
}

// a routine as the daemon reads it out: its schedule in words and, while it can fire, its next run
func (s *server) readRoutine(routine protocol.Routine) protocol.Routine {
	preview := cron.Preview(routine.Schedule, demoRoutineClock(), 1)
	routine.Sentence, routine.NextRun = preview.Sentence, nil
	index := s.data.teams.teamIndex(routine.Team)
	if routine.Enabled && index >= 0 && !s.data.teams.teams[index].Paused && len(preview.Next) > 0 {
		next := preview.Next[0]
		routine.NextRun = &next
	}
	return routine
}

func (s *server) vaultInfo() protocol.VaultInfo {
	vault := s.data.teams.vault
	vault.Credentials = append([]protocol.Credential(nil), vault.Credentials...)
	if vault.Mail != nil {
		mail := *vault.Mail
		mail.Trusted = append([]protocol.MailTrust(nil), mail.Trusted...)
		vault.Mail = &mail
	}
	return vault
}

// the Teams part of a snapshot: open tasks with the newest finished, pending approvals with the newest decided
func (s *server) fillTeamsSnapshot(snapshot *protocol.Snapshot) {
	t := &s.data.teams
	snapshot.Teams = s.listTeams()
	snapshot.TeamTasks = nil
	finished := 0
	for _, task := range t.tasks {
		if taskOpen(task) {
			snapshot.TeamTasks = append(snapshot.TeamTasks, task)
		} else if finished < 50 {
			snapshot.TeamTasks = append(snapshot.TeamTasks, task)
			finished++
		}
	}
	snapshot.Approvals = nil
	decided := 0
	for _, approval := range t.approvals {
		if approval.Status == protocol.ApprovalPending {
			snapshot.Approvals = append(snapshot.Approvals, approval)
		} else if decided < 20 {
			snapshot.Approvals = append(snapshot.Approvals, approval)
			decided++
		}
	}
	snapshot.Routines = make([]protocol.Routine, 0, len(t.routines))
	for _, routine := range t.routines {
		snapshot.Routines = append(snapshot.Routines, s.readRoutine(routine))
	}
	vault := s.vaultInfo()
	snapshot.Vault = &vault
}

// queues one event for the client that made the call, as the daemon broadcasts it
func (s *server) tellTeams(name string, value any) {
	if wire, err := json.Marshal(value); err == nil {
		s.data.teams.told = append(s.data.teams.told, protocol.Event{Event: name, Data: wire})
	}
}

func (s *server) takeTeamsEvents() []protocol.Event {
	told := s.data.teams.told
	s.data.teams.told = nil
	return told
}

func (s *server) emitTeam(id string) {
	if index := s.data.teams.teamIndex(id); index >= 0 {
		s.tellTeams(protocol.EvtTeam, s.teamWithStats(s.data.teams.teams[index]))
	}
}

func (s *server) emitTask(task protocol.TeamTask) {
	s.tellTeams(protocol.EvtTeamTask, task)
	s.emitTeam(task.Team)
}

func (s *server) emitRoutine(routine protocol.Routine) {
	s.tellTeams(protocol.EvtRoutine, s.readRoutine(routine))
	s.emitTeam(routine.Team)
}

func (s *server) emitVault() { s.tellTeams(protocol.EvtVault, s.vaultInfo()) }

// appends to the ledger, newest first; the demo has a window open, so nothing it records went unwatched
func (s *server) record(entry protocol.Activity) {
	if entry.Time.IsZero() {
		entry.Time = time.Now()
	}
	if entry.Actor == "" {
		entry.Actor, entry.Initiator = "you", protocol.InitiatorYou
	}
	s.data.teams.activity = append([]protocol.Activity{entry}, s.data.teams.activity...)
}

func (s *server) recordVault(entry protocol.VaultEvent) {
	if entry.Time.IsZero() {
		entry.Time = time.Now()
	}
	s.data.teams.vaultLog = append([]protocol.VaultEvent{entry}, s.data.teams.vaultLog...)
}

// a task's session follows the task, and the open conversation hears it
func (s *server) settleTeamSession(task protocol.TeamTask) {
	t := &s.data.teams
	index := t.sessionIndex(task.Session)
	if index < 0 {
		return
	}
	standSession(&t.sessions[index], task)
	s.tellTeams(protocol.EvtSessionInfo, t.sessions[index])
}

// adds events to a team session's transcript and tells the client each one
func (s *server) appendTeamEvents(session string, events ...func(seq int64) protocol.SessionEvent) {
	t := &s.data.teams
	index := t.sessionIndex(session)
	if index < 0 {
		return
	}
	seq := int64(0)
	if existing := s.data.events[session]; len(existing) > 0 {
		seq = existing[len(existing)-1].Seq
	}
	for _, build := range events {
		seq++
		made := build(seq)
		made.TS = time.Now()
		s.data.events[session] = append(s.data.events[session], made)
		s.tellTeams(protocol.EvtSessionEvent, protocol.SessionEventData{Session: session, SessionEvent: made})
	}
	t.sessions[index].LastSeq, t.sessions[index].Steps = seq, lastTurnSteps(s.data.events[session])
}

func later(kind string, data any) func(seq int64) protocol.SessionEvent {
	return func(seq int64) protocol.SessionEvent { return event(seq, kind, data) }
}

// the turn a held session goes on with, numbered after the ones it took
func (s *server) nextTurn(session string) int {
	turn := 1
	for _, made := range s.data.events[session] {
		if made.Kind == protocol.EvTurnStart {
			turn++
		}
	}
	return turn
}

// a team's member by name or id, for the actor of what it did
func (s *server) memberName(teamID, memberID string) string {
	if index := s.data.teams.teamIndex(teamID); index >= 0 {
		if member, ok := teamMember(s.data.teams.teams[index], memberID); ok {
			return member.Name
		}
	}
	return i18n.T("A removed member")
}
