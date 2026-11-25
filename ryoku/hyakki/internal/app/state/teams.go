package state

import (
	"encoding/json"
	"sort"

	"ryoku-rashin/internal/protocol"
)

// teamsState is the Teams part of the store: teams, their tasks, approvals,
// routines and the vault without any value.
type teamsState struct {
	teams     map[string]protocol.Team
	teamTasks map[string]protocol.TeamTask
	approvals map[string]protocol.Approval
	routines  map[string]protocol.Routine
	vault     protocol.VaultInfo
}

func newTeamsState() teamsState {
	return teamsState{
		teams:     make(map[string]protocol.Team),
		teamTasks: make(map[string]protocol.TeamTask),
		approvals: make(map[string]protocol.Approval),
		routines:  make(map[string]protocol.Routine),
	}
}

// applySnapshotLocked replaces every Teams collection with the daemon's.
func (t *teamsState) applySnapshotLocked(snapshot protocol.Snapshot) {
	*t = newTeamsState()
	for _, team := range snapshot.Teams {
		t.teams[team.ID] = team
	}
	for _, task := range snapshot.TeamTasks {
		t.teamTasks[task.ID] = task
	}
	for _, approval := range snapshot.Approvals {
		t.approvals[approval.ID] = approval
	}
	for _, routine := range snapshot.Routines {
		t.routines[routine.ID] = routine
	}
	if snapshot.Vault != nil {
		t.vault = cloneVault(*snapshot.Vault)
	}
}

// applyLocked applies one Teams event; handled is false for any other.
func (t *teamsState) applyLocked(event protocol.Event) (handled bool, err error) {
	switch event.Event {
	case protocol.EvtTeam:
		var team protocol.Team
		if err = json.Unmarshal(event.Data, &team); err == nil {
			t.teams[team.ID] = team
		}
	case protocol.EvtTeamRemoved:
		var removed protocol.TeamRemoved
		if err = json.Unmarshal(event.Data, &removed); err == nil {
			delete(t.teams, removed.ID)
			for id, task := range t.teamTasks {
				if task.Team == removed.ID {
					delete(t.teamTasks, id)
				}
			}
			for id, routine := range t.routines {
				if routine.Team == removed.ID {
					delete(t.routines, id)
				}
			}
		}
	case protocol.EvtTeamTask:
		var task protocol.TeamTask
		if err = json.Unmarshal(event.Data, &task); err == nil {
			t.teamTasks[task.ID] = task
		}
	case protocol.EvtTeamTaskRemoved:
		var removed protocol.TeamRemoved
		if err = json.Unmarshal(event.Data, &removed); err == nil {
			delete(t.teamTasks, removed.ID)
		}
	case protocol.EvtApproval:
		var approval protocol.Approval
		if err = json.Unmarshal(event.Data, &approval); err == nil {
			t.approvals[approval.ID] = approval
		}
	case protocol.EvtRoutine:
		var routine protocol.Routine
		if err = json.Unmarshal(event.Data, &routine); err == nil {
			t.routines[routine.ID] = routine
		}
	case protocol.EvtRoutineRemoved:
		var removed protocol.TeamRemoved
		if err = json.Unmarshal(event.Data, &removed); err == nil {
			delete(t.routines, removed.ID)
		}
	case protocol.EvtVault:
		var vault protocol.VaultInfo
		if err = json.Unmarshal(event.Data, &vault); err == nil {
			t.vault = vault
		}
	default:
		return false, nil
	}
	return true, err
}

// snapshotLocked copies the Teams collections into a published snapshot.
func (t *teamsState) snapshotLocked(into *Snapshot) {
	into.Teams = make([]protocol.Team, 0, len(t.teams))
	for _, team := range t.teams {
		into.Teams = append(into.Teams, cloneTeam(team))
	}
	sort.Slice(into.Teams, func(i, j int) bool {
		if into.Teams[i].Name != into.Teams[j].Name {
			return into.Teams[i].Name < into.Teams[j].Name
		}
		return into.Teams[i].ID < into.Teams[j].ID
	})
	into.TeamTasks = make(map[string]protocol.TeamTask, len(t.teamTasks))
	for id, task := range t.teamTasks {
		into.TeamTasks[id] = cloneTeamTask(task)
	}
	into.Approvals = make([]protocol.Approval, 0, len(t.approvals))
	for _, approval := range t.approvals {
		into.Approvals = append(into.Approvals, approval)
	}
	sort.Slice(into.Approvals, func(i, j int) bool {
		return into.Approvals[i].Created.After(into.Approvals[j].Created)
	})
	into.Routines = make([]protocol.Routine, 0, len(t.routines))
	for _, routine := range t.routines {
		into.Routines = append(into.Routines, routine)
	}
	sort.Slice(into.Routines, func(i, j int) bool {
		a, b := into.Routines[i], into.Routines[j]
		if a.Team != b.Team {
			return a.Team < b.Team
		}
		if a.Title != b.Title {
			return a.Title < b.Title
		}
		return a.ID < b.ID
	})
	into.Vault = cloneVault(t.vault)
}

// mergeTasksLocked stores tasks a page loaded, such as older finished ones.
func (t *teamsState) mergeTasksLocked(tasks []protocol.TeamTask) {
	for _, task := range tasks {
		t.teamTasks[task.ID] = task
	}
}

// SetTeamTasks stores a teams.tasks result so pages can show tasks the
// snapshot left out.
func (s *Store) SetTeamTasks(result protocol.TeamTasksResult) {
	s.mu.Lock()
	s.teamsState.mergeTasksLocked(result.Tasks)
	invalidate := s.changedLocked()
	s.mu.Unlock()
	if invalidate != nil {
		invalidate()
	}
}

func cloneTeam(team protocol.Team) protocol.Team {
	team.Members = append([]protocol.Member(nil), team.Members...)
	if team.Stats.Doing != nil {
		doing := make(map[string]string, len(team.Stats.Doing))
		for member, what := range team.Stats.Doing {
			doing[member] = what
		}
		team.Stats.Doing = doing
	}
	return team
}

func cloneTeamTask(task protocol.TeamTask) protocol.TeamTask {
	task.Sessions = append([]string(nil), task.Sessions...)
	if task.Report != nil {
		report := *task.Report
		report.Files = append([]string(nil), report.Files...)
		task.Report = &report
	}
	return task
}

func cloneVault(vault protocol.VaultInfo) protocol.VaultInfo {
	credentials := make([]protocol.Credential, len(vault.Credentials))
	for i, credential := range vault.Credentials {
		credential.Hosts = append([]string(nil), credential.Hosts...)
		credential.Teams = append([]string(nil), credential.Teams...)
		credential.Trusted = append([]protocol.CredentialTrust(nil), credential.Trusted...)
		credentials[i] = credential
	}
	vault.Credentials = credentials
	if vault.Mail != nil {
		mail := *vault.Mail
		mail.Trusted = append([]protocol.MailTrust(nil), mail.Trusted...)
		vault.Mail = &mail
	}
	return vault
}
