package demo

import (
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"sort"
	"strings"
	"time"
	"unicode"

	"ryoku-i18n"

	"ryoku-rashin/internal/cron"
	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/protocol"
)

// answers one call with params of the method's type
func teamsCall[P any](raw json.RawMessage, run func(P) (any, error)) (any, error) {
	var params P
	if err := decode(raw, &params); err != nil {
		return nil, err
	}
	return run(params)
}

// the teams.* and vault.* methods the pages call, answered from the fixtures and folded into them so a change stays
func (s *server) callTeams(method string, raw json.RawMessage) (any, error) {
	switch method {
	case protocol.MTeamsList:
		return protocol.TeamsResult{Teams: s.listTeams()}, nil
	case protocol.MTeamsTemplates:
		return protocol.TeamTemplatesResult{Templates: demoTemplates()}, nil
	case protocol.MTeamsCreate:
		return teamsCall(raw, func(p protocol.TeamCreateParams) (any, error) { return s.createTeam(p, false) })
	case protocol.MTeamsUpdate:
		return teamsCall(raw, func(p protocol.TeamUpdateParams) (any, error) { return s.updateTeam(p) })
	case protocol.MTeamsDelete:
		return teamsCall(raw, func(p protocol.TeamRef) (any, error) { return protocol.Empty{}, s.deleteTeam(p.ID) })
	case protocol.MTeamsPause:
		return teamsCall(raw, func(p protocol.TeamPauseParams) (any, error) { return s.pauseTeam(p) })
	case protocol.MTeamsExport:
		return teamsCall(raw, func(p protocol.TeamRef) (any, error) { return s.exportTeam(p.ID) })
	case protocol.MTeamsImport:
		return teamsCall(raw, func(p protocol.TeamImportParams) (any, error) { return s.importTeam(p.Path) })
	case protocol.MTeamsTasks:
		return teamsCall(raw, func(p protocol.TeamTasksParams) (any, error) { return s.listTeamTasks(p), nil })
	case protocol.MTeamsTaskCreate:
		return teamsCall(raw, func(p protocol.TeamTaskCreateParams) (any, error) { return s.createTeamTask(p) })
	case protocol.MTeamsTaskStop:
		return teamsCall(raw, func(p protocol.TeamTaskRef) (any, error) { return s.stopTask(p.ID) })
	case protocol.MTeamsTaskRetry:
		return teamsCall(raw, func(p protocol.TeamTaskRetryParams) (any, error) { return s.retryTask(p) })
	case protocol.MTeamsTaskRead:
		return teamsCall(raw, func(p protocol.TeamTaskRef) (any, error) { return s.readTask(p.ID) })
	case protocol.MTeamsTaskDelete:
		return teamsCall(raw, func(p protocol.TeamTaskRef) (any, error) { return protocol.Empty{}, s.deleteTask(p.ID) })
	case protocol.MTeamsRoutines:
		return teamsCall(raw, func(p protocol.TeamRef) (any, error) { return s.listRoutines(p.ID), nil })
	case protocol.MTeamsRoutineCreate:
		return teamsCall(raw, func(p protocol.RoutineCreateParams) (any, error) { return s.createRoutine(p) })
	case protocol.MTeamsRoutineUpdate:
		return teamsCall(raw, func(p protocol.RoutineUpdateParams) (any, error) { return s.updateRoutine(p) })
	case protocol.MTeamsRoutineDelete:
		return teamsCall(raw, func(p protocol.RoutineRef) (any, error) { return protocol.Empty{}, s.deleteRoutine(p.ID) })
	case protocol.MTeamsRoutineRun:
		return teamsCall(raw, func(p protocol.RoutineRef) (any, error) { return s.runRoutine(p.ID) })
	case protocol.MTeamsRoutinePreview:
		return teamsCall(raw, func(p protocol.RoutinePreviewParams) (any, error) {
			count := p.Count
			if count <= 0 {
				count = 5
			}
			return cron.Preview(p.Schedule, demoRoutineClock(), min(count, 20)), nil
		})
	case protocol.MTeamsApprovals:
		return teamsCall(raw, func(p protocol.ApprovalsParams) (any, error) { return s.listApprovals(p), nil })
	case protocol.MTeamsApprovalDecide:
		return teamsCall(raw, func(p protocol.ApprovalDecideParams) (any, error) { return s.decideApproval(p) })
	case protocol.MTeamsActivity:
		return teamsCall(raw, func(p protocol.TeamActivityParams) (any, error) { return s.teamActivity(p), nil })
	case protocol.MTeamsSpend:
		return teamsCall(raw, func(p protocol.TeamSpendParams) (any, error) { return s.teamSpend(p) })
	case protocol.MTeamsPrune:
		return teamsCall(raw, func(p protocol.TeamPruneParams) (any, error) { return protocol.Empty{}, s.pruneTasks(p) })
	case protocol.MVaultInfo:
		return s.vaultInfo(), nil
	case protocol.MVaultAdd:
		return teamsCall(raw, func(p protocol.VaultAddParams) (any, error) { return s.addCredential(p) })
	case protocol.MVaultUpdate:
		return teamsCall(raw, func(p protocol.VaultUpdateParams) (any, error) { return s.updateCredential(p) })
	case protocol.MVaultRemove:
		return teamsCall(raw, func(p protocol.VaultRef) (any, error) { return protocol.Empty{}, s.removeCredential(p.ID) })
	case protocol.MVaultEvents:
		return teamsCall(raw, func(p protocol.VaultEventsParams) (any, error) { return s.vaultEvents(p), nil })
	case protocol.MVaultMail:
		return teamsCall(raw, func(p protocol.MailAccountParams) (any, error) { return s.setMailAccount(p.Account) })
	case protocol.MVaultMailTest:
		return s.testMailAccount(), nil
	}
	// the agent methods belong to a running team task, which the demo has none of
	return nil, protocol.Errorf(protocol.CodeUnsupported, "demo method %s is not implemented", method)
}

// a folder name for a team, the way the daemon makes one
func slugName(name string) string {
	var slug strings.Builder
	dash := false
	for _, r := range strings.ToLower(name) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			slug.WriteRune(r)
			dash = false
		} else if !dash && slug.Len() > 0 {
			slug.WriteByte('-')
			dash = true
		}
	}
	if result := strings.TrimSuffix(slug.String(), "-"); result != "" {
		return result
	}
	return "team"
}

// one to six members with names, each with an id and an engine this computer knows, and exactly one lead
func (s *server) normaliseMembers(members []protocol.Member) ([]protocol.Member, error) {
	if len(members) == 0 {
		return nil, badRequest(i18n.T("A team needs at least one member."))
	}
	if len(members) > 6 {
		return nil, badRequest(i18n.T("A team has at most six members."))
	}
	result := make([]protocol.Member, len(members))
	names := make(map[string]bool, len(members))
	lead := -1
	for index, member := range members {
		member.Name = strings.TrimSpace(member.Name)
		if member.Name == "" {
			return nil, badRequest(i18n.T("Every member needs a name."))
		}
		if names[strings.ToLower(member.Name)] {
			return nil, badRequest(i18n.Tf("Two members are called %s.", member.Name))
		}
		names[strings.ToLower(member.Name)] = true
		member.Harness, member.Model = strings.TrimSpace(member.Harness), strings.TrimSpace(member.Model)
		if member.Harness != "" && !slices.ContainsFunc(s.data.snapshot.Harnesses, func(harness protocol.HarnessInfo) bool { return harness.ID == member.Harness }) {
			return nil, badRequest(i18n.Tf("No harness %s on this computer.", member.Harness))
		}
		if !protocol.ValidEngineModel(member.Model) || !protocol.ValidEngineOptions(member.Options) {
			return nil, badRequest(i18n.T("That model or option cannot be used."))
		}
		if member.ID == "" {
			member.ID = s.data.teams.newID("member")
		}
		if member.Lead && lead < 0 {
			lead = index
		}
		member.Lead = false
		result[index] = member
	}
	result[max(lead, 0)].Lead = true
	return result, nil
}

// a routine's member by id or by name, empty for the lead
func routineMember(team protocol.Team, member string) string {
	for _, candidate := range team.Members {
		if member != "" && (candidate.ID == member || strings.EqualFold(candidate.Name, member)) && !candidate.Lead {
			return candidate.ID
		}
	}
	return ""
}

func (s *server) createTeam(p protocol.TeamCreateParams, imported bool) (protocol.Team, error) {
	t := &s.data.teams
	name := strings.TrimSpace(p.Name)
	if name == "" {
		return protocol.Team{}, badRequest(i18n.T("A team needs a name."))
	}
	members, err := s.normaliseMembers(p.Members)
	if err != nil {
		return protocol.Team{}, err
	}
	now := time.Now()
	team := protocol.Team{
		ID: t.newID("team"), Name: name, Purpose: strings.TrimSpace(p.Purpose), Template: p.Template, Folder: p.Folder,
		Members: members, Tools: p.Tools, Approvals: p.Approvals, Budget: p.Budget, Notify: p.Notify,
		Running: p.Running, Access: p.Access, Created: now, Updated: now,
	}
	if team.Folder == "" {
		team.Folder = paths.TeamsHome() + "/" + slugName(name)
	}
	// what the create leaves out comes from what new teams start with
	defaults := s.data.snapshot.Settings.Teams.Defaults
	if !protocol.KnownApprovals(team.Approvals) {
		team.Approvals = defaults.Approvals
	}
	if team.Notify != protocol.NotifyAll && team.Notify != protocol.NotifyNeeds && team.Notify != protocol.NotifyNone {
		team.Notify = defaults.Notify
	}
	if team.Budget.MonthTokens == 0 && team.Budget.TaskTurns == 0 {
		team.Budget = defaults.Budget
	}
	if team.Budget.WarnPercent == 0 {
		team.Budget.WarnPercent = defaults.Budget.WarnPercent
	}
	if team.Budget.TaskTurns == 0 {
		team.Budget.TaskTurns = defaults.Budget.TaskTurns
	}
	action := "team.create"
	if imported {
		team.Paused, team.PauseReason, action = true, protocol.PausedByYou, "team.import"
	}
	// its routines must hold before anything is kept, so a refused team leaves nothing behind
	t.teams = append(t.teams, team)
	routines := make([]protocol.Routine, 0, len(p.Routines))
	for _, draft := range p.Routines {
		routine := protocol.Routine{
			ID: t.newID("routine"), Team: team.ID, Title: strings.TrimSpace(draft.Title), Brief: strings.TrimSpace(draft.Brief),
			Member: routineMember(team, draft.Member), Schedule: strings.TrimSpace(draft.Schedule), Enabled: draft.Enabled && !imported,
			Missed: protocol.MissedSkip, Overlap: protocol.OverlapSkip, Created: now, Updated: now,
		}
		if err := s.checkRoutine(routine); err != nil {
			t.teams = t.teams[:len(t.teams)-1]
			return protocol.Team{}, err
		}
		routines = append(routines, routine)
	}
	t.spend[team.ID] = protocol.TeamSpend{Team: team.ID, Month: demoMonth(Epoch), Cap: team.Budget.MonthTokens, Days: spreadMonth(Epoch, 0, 1)}
	s.record(protocol.Activity{Action: action, Target: team.Name, OK: true, Team: team.ID})
	for _, routine := range routines {
		t.routines = append(t.routines, routine)
		s.record(protocol.Activity{Action: "routine.create", Target: routine.Title, OK: true, Team: team.ID, Detail: s.readRoutine(routine).Sentence})
		s.emitRoutine(routine)
	}
	s.emitTeam(team.ID)
	return s.teamWithStats(team), nil
}

func (s *server) updateTeam(p protocol.TeamUpdateParams) (protocol.Team, error) {
	t := &s.data.teams
	index := t.teamIndex(p.ID)
	if index < 0 {
		return protocol.Team{}, notFound("team")
	}
	before := t.teams[index]
	team := before
	if err := mergeInto(p.Patch, &team); err != nil {
		return protocol.Team{}, err
	}
	// a patch changes only what the user may change
	team.ID, team.Template, team.Paused, team.PauseReason = before.ID, before.Template, before.Paused, before.PauseReason
	team.Created, team.Stats = before.Created, protocol.TeamStats{}
	team.Name = strings.TrimSpace(team.Name)
	if team.Name == "" {
		return protocol.Team{}, badRequest(i18n.T("A team needs a name."))
	}
	members, err := s.normaliseMembers(team.Members)
	if err != nil {
		return protocol.Team{}, err
	}
	team.Members = members
	if !protocol.KnownApprovals(team.Approvals) {
		team.Approvals = before.Approvals
	}
	team.Updated = time.Now()
	t.teams[index] = team
	spend := t.spend[team.ID]
	spend.Cap = team.Budget.MonthTokens
	t.spend[team.ID] = spend
	s.record(protocol.Activity{Action: "team.update", Target: team.Name, OK: true, Team: team.ID})
	s.emitTeam(team.ID)
	return s.teamWithStats(team), nil
}

func (s *server) deleteTeam(id string) error {
	t := &s.data.teams
	index := t.teamIndex(id)
	if index < 0 {
		return notFound("team")
	}
	team := t.teams[index]
	// what still waits on the user is answered first, so no card is left asking for a team that is gone
	for approval := range t.approvals {
		if t.approvals[approval].Team == id && t.approvals[approval].Status == protocol.ApprovalPending {
			s.settleApproval(approval, protocol.DecideDeny, i18n.T("The team was deleted."))
		}
	}
	t.teams = append(t.teams[:index:index], t.teams[index+1:]...)
	keptTasks := t.tasks[:0:0]
	for _, task := range t.tasks {
		if task.Team != id {
			keptTasks = append(keptTasks, task)
		}
	}
	t.tasks = keptTasks
	keptRoutines := t.routines[:0:0]
	for _, routine := range t.routines {
		if routine.Team != id {
			keptRoutines = append(keptRoutines, routine)
		}
	}
	t.routines = keptRoutines
	delete(t.spend, id)
	s.dropGrants(id)
	s.record(protocol.Activity{Action: "team.delete", Target: team.Name, OK: true, Team: id})
	s.tellTeams(protocol.EvtTeamRemoved, protocol.TeamRemoved{ID: id})
	s.emitVault()
	return nil
}

func (s *server) pauseTeam(p protocol.TeamPauseParams) (protocol.Team, error) {
	t := &s.data.teams
	index := t.teamIndex(p.ID)
	if index < 0 {
		return protocol.Team{}, notFound("team")
	}
	team := t.teams[index]
	if team.Paused == p.Paused {
		return s.teamWithStats(team), nil
	}
	now := time.Now()
	if p.Paused {
		team.Paused, team.PauseReason, team.Updated = true, protocol.PausedByYou, now
		t.teams[index] = team
		s.record(protocol.Activity{Action: "team.pause", Target: team.Name, OK: true, Team: team.ID})
		for task := range t.tasks {
			if t.tasks[task].Team == team.ID && t.tasks[task].Status == protocol.TaskWorking {
				s.stopTeamTask(task, i18n.T("You paused the team."))
			}
		}
		s.emitTeam(team.ID)
		return s.teamWithStats(team), nil
	}
	if team.PauseReason == protocol.PausedByBudget {
		if p.Raise <= t.spend[team.ID].Tokens {
			return protocol.Team{}, badRequest(i18n.T("Raise the spending limit above what the team used this month to let it work again."))
		}
		team.Budget.MonthTokens = p.Raise
		for approval := range t.approvals {
			if t.approvals[approval].Team == team.ID && t.approvals[approval].Kind == protocol.ApprovalBudget && t.approvals[approval].Status == protocol.ApprovalPending {
				t.approvals[approval].Result = i18n.T("You raised the limit from the team's page.")
				s.settleApproval(approval, protocol.DecideOnce, "")
			}
		}
	}
	t.teams[index] = team
	s.resumeTeam(index)
	return s.teamWithStats(t.teams[index]), nil
}

// a team at work again starts what queued up while it was paused
func (s *server) resumeTeam(index int) {
	t := &s.data.teams
	team := t.teams[index]
	team.Paused, team.PauseReason, team.Updated = false, "", time.Now()
	t.teams[index] = team
	spend := t.spend[team.ID]
	spend.Cap = team.Budget.MonthTokens
	t.spend[team.ID] = spend
	s.record(protocol.Activity{Action: "team.resume", Target: team.Name, OK: true, Team: team.ID})
	s.startQueued(team)
	s.emitTeam(team.ID)
}

// what waited for a free slot starts, unless every team is held
func (s *server) startQueued(team protocol.Team) {
	t := &s.data.teams
	if !s.mayWork(team) {
		return
	}
	for task := range t.tasks {
		if t.tasks[task].Team == team.ID && t.tasks[task].Status == protocol.TaskReady {
			t.tasks[task] = s.beginTeamTask(team, t.tasks[task])
			s.tellTeams(protocol.EvtTeamTask, t.tasks[task])
		}
	}
}

// a team works unless it or every team is paused
func (s *server) mayWork(team protocol.Team) bool {
	return !team.Paused && !s.data.snapshot.Settings.Teams.Paused
}

func (s *server) exportTeam(id string) (protocol.TeamExportResult, error) {
	t := &s.data.teams
	index := t.teamIndex(id)
	if index < 0 {
		return protocol.TeamExportResult{}, notFound("team")
	}
	team := t.teams[index]
	export := protocol.TeamExport{
		Version: 1, Name: team.Name, Purpose: team.Purpose, Template: team.Template, Members: team.Members, Tools: team.Tools,
		Approvals: team.Approvals, Budget: team.Budget, Notify: team.Notify,
	}
	for _, routine := range t.routines {
		if routine.Team == id {
			export.Routines = append(export.Routines, protocol.RoutineDraft{Title: routine.Title, Brief: routine.Brief, Member: routine.Member, Schedule: routine.Schedule, Enabled: routine.Enabled})
		}
	}
	for _, credential := range t.vault.Credentials {
		for _, granted := range credential.Teams {
			if granted == id {
				export.Credentials = append(export.Credentials, credential.Name)
			}
		}
	}
	s.record(protocol.Activity{Action: "team.export", Target: team.Name, OK: true, Team: id})
	return protocol.TeamExportResult{Export: export, Path: team.Folder + "/" + slugName(team.Name) + ".rashin.json"}, nil
}

// reads the file the user picked, as the daemon does; the team arrives paused with its routines off
func (s *server) importTeam(path string) (protocol.Team, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return protocol.Team{}, badRequest(i18n.T("That file could not be read."))
	}
	var export protocol.TeamExport
	if json.Unmarshal(raw, &export) != nil || export.Version == 0 {
		return protocol.Team{}, badRequest(i18n.T("That file is not an exported team."))
	}
	for index := range export.Members {
		export.Members[index].ID = ""
	}
	return s.createTeam(protocol.TeamCreateParams{
		Name: export.Name, Purpose: export.Purpose, Template: export.Template, Members: export.Members, Tools: export.Tools,
		Approvals: export.Approvals, Budget: export.Budget, Notify: export.Notify, Routines: export.Routines,
	}, true)
}

func (s *server) listTeamTasks(p protocol.TeamTasksParams) protocol.TeamTasksResult {
	t := &s.data.teams
	var before time.Time
	if p.Before != "" {
		if at, err := time.Parse(time.RFC3339Nano, p.Before); err == nil {
			before = at
		} else if index := t.taskIndex(p.Before); index >= 0 {
			before = t.tasks[index].Created
		}
	}
	limit := p.Limit
	if limit <= 0 {
		limit = 50
	}
	tasks := make([]protocol.TeamTask, 0)
	for _, task := range t.tasks {
		if p.Team != "" && task.Team != p.Team || p.Open && !taskOpen(task) || !before.IsZero() && !task.Created.Before(before) {
			continue
		}
		tasks = append(tasks, task)
		if len(tasks) == limit {
			break
		}
	}
	return protocol.TeamTasksResult{Tasks: tasks}
}

func (s *server) createTeamTask(p protocol.TeamTaskCreateParams) (protocol.TeamTask, error) {
	t := &s.data.teams
	index := t.teamIndex(p.Team)
	if index < 0 {
		return protocol.TeamTask{}, notFound("team")
	}
	brief := strings.TrimSpace(p.Brief)
	if brief == "" {
		return protocol.TeamTask{}, badRequest(i18n.T("Say what the team should do."))
	}
	team := t.teams[index]
	member, ok := teamMember(team, p.Member)
	if !ok {
		return protocol.TeamTask{}, badRequest(i18n.T("That member is not on the team."))
	}
	title := strings.TrimSpace(p.Title)
	if title == "" {
		title = strings.TrimSpace(strings.SplitN(brief, "\n", 2)[0])
		if runes := []rune(title); len(runes) > 80 {
			title = string(runes[:80])
		}
	}
	model := strings.TrimSpace(p.Model)
	if !protocol.ValidEngineModel(model) || !protocol.ValidEngineOptions(p.Options) {
		return protocol.TeamTask{}, badRequest(i18n.T("That model or option cannot be used."))
	}
	return s.newTeamTask(team, member.ID, title, brief, protocol.TaskFromYou, "", model, p.Options), nil
}

// a new task starts at once while its team may work, as a free slot would take it; a model and options set apply to it alone
func (s *server) newTeamTask(team protocol.Team, member, title, brief, origin, routine, model string, options map[string]string) protocol.TeamTask {
	t := &s.data.teams
	now := time.Now()
	task := protocol.TeamTask{
		ID: t.newID("task"), Team: team.ID, Title: title, Brief: brief, Status: protocol.TaskReady, Member: member,
		Origin: origin, Routine: routine, MaxTurns: team.Budget.TaskTurns, Model: model, Options: options, Created: now, Updated: now,
	}
	s.record(protocol.Activity{Action: "task.create", Target: title, OK: true, Team: team.ID, Task: task.ID})
	if s.mayWork(team) {
		task = s.beginTeamTask(team, task)
	}
	t.tasks = append([]protocol.TeamTask{task}, t.tasks...)
	s.emitTask(task)
	return task
}

// a task's new session in its first seconds: the goal set, the brief sent, the agent starting to read
func (s *server) beginTeamTask(team protocol.Team, task protocol.TeamTask) protocol.TeamTask {
	t := &s.data.teams
	now := time.Now()
	task.Status, task.Waiting, task.Started, task.Updated = protocol.TaskWorking, "", &now, now
	task.Turns, task.Usage = 0, protocol.TokenUsage{}
	member, _ := teamMember(team, task.Member)
	startEngine(&task, member, s.data.snapshot.Settings, s.registryHarness())
	task.Session = fmt.Sprintf("agent-%s-%d", task.ID, len(task.Sessions)+1)
	task.Sessions = append(append([]string(nil), task.Sessions...), task.Session)
	transcript := teamTranscript(task, []line{said(i18n.T("I'm reading the brief and the team's folder before I start."))})
	for index := range transcript {
		transcript[index].TS = now
	}
	s.data.events[task.Session] = transcript
	t.sessions = append([]protocol.SessionInfo{teamSession(team, task, transcript, i18n.T("Reading the brief."), i18n.T("Reading the brief"))}, t.sessions...)
	s.record(protocol.Activity{
		Actor: "Rashin", Initiator: protocol.InitiatorRashin, Action: "task.start", Target: task.Title, OK: true, Team: team.ID, Task: task.ID,
		Detail: i18n.Tf("%s started work on it with %s, %s.", member.Name, s.harnessName(task.Harness), engineModel(task.Model)),
	})
	return task
}

func (s *server) stopTask(id string) (protocol.TeamTask, error) {
	t := &s.data.teams
	index := t.taskIndex(id)
	if index < 0 {
		return protocol.TeamTask{}, notFound("task")
	}
	if taskOpen(t.tasks[index]) {
		s.stopTeamTask(index, i18n.T("You stopped it."))
	}
	return t.tasks[t.taskIndex(id)], nil
}

// stops a task, the pieces it handed out and what it asked the user
func (s *server) stopTeamTask(index int, reason string) {
	t := &s.data.teams
	before := t.tasks[index]
	now := time.Now()
	task := before
	task.Status, task.Waiting, task.Reason, task.Ended, task.Updated = protocol.TaskStopped, "", reason, &now, now
	t.tasks[index] = task
	if before.Session != "" {
		var ends []func(seq int64) protocol.SessionEvent
		if before.Status == protocol.TaskWorking && before.Waiting == "" {
			ends = append(ends, later(protocol.EvTurnEnd, protocol.TurnEndData{Turn: s.nextTurn(before.Session) - 1, Stop: protocol.StopCancelled}))
		}
		ends = append(ends, later(protocol.EvGoal, protocol.GoalData{Goal: teamGoal(task)}))
		s.appendTeamEvents(task.Session, ends...)
		s.settleTeamSession(task)
	}
	for approval := range t.approvals {
		if t.approvals[approval].Task == task.ID && t.approvals[approval].Status == protocol.ApprovalPending {
			s.settleApproval(approval, protocol.DecideDeny, i18n.T("The task was stopped."))
		}
	}
	s.record(protocol.Activity{Action: "task.stop", Target: task.Title, OK: true, Team: task.Team, Task: task.ID, Detail: reason})
	s.emitTask(task)
	for piece := range t.tasks {
		if t.tasks[piece].Parent == task.ID && taskOpen(t.tasks[piece]) {
			s.stopTeamTask(piece, reason)
		}
	}
}

func (s *server) retryTask(p protocol.TeamTaskRetryParams) (protocol.TeamTask, error) {
	t := &s.data.teams
	index := t.taskIndex(p.ID)
	if index < 0 {
		return protocol.TeamTask{}, notFound("task")
	}
	task := t.tasks[index]
	if taskOpen(task) {
		return protocol.TeamTask{}, badRequest(i18n.T("Only a finished, blocked or stopped task can run again."))
	}
	team := t.teamIndex(task.Team)
	if team < 0 {
		return protocol.TeamTask{}, notFound("team")
	}
	if note := strings.TrimSpace(p.Note); note != "" {
		task.Brief = i18n.Tf("%s\n\nWhen you ran this before: %s", task.Brief, note)
	}
	task.Status, task.Waiting, task.Report, task.Reason, task.Ended, task.Read = protocol.TaskReady, "", nil, "", nil, false
	task.Updated = time.Now()
	s.record(protocol.Activity{Action: "task.retry", Target: task.Title, OK: true, Team: task.Team, Task: task.ID, Detail: strings.TrimSpace(p.Note)})
	if s.mayWork(t.teams[team]) {
		task = s.beginTeamTask(t.teams[team], task)
	}
	t.tasks[index] = task
	s.emitTask(task)
	return task, nil
}

func (s *server) readTask(id string) (protocol.TeamTask, error) {
	t := &s.data.teams
	index := t.taskIndex(id)
	if index < 0 {
		return protocol.TeamTask{}, notFound("task")
	}
	if !t.tasks[index].Read {
		t.tasks[index].Read = true
		s.emitTask(t.tasks[index])
	}
	return t.tasks[index], nil
}

// deletes a task and the pieces it handed out; their sessions stay
func (s *server) deleteTask(id string) error {
	t := &s.data.teams
	index := t.taskIndex(id)
	if index < 0 {
		return notFound("task")
	}
	team := t.tasks[index].Team
	if taskOpen(t.tasks[index]) {
		s.stopTeamTask(index, i18n.T("You deleted it."))
	}
	kept := t.tasks[:0:0]
	for _, task := range t.tasks {
		if task.ID == id || task.Parent == id {
			s.tellTeams(protocol.EvtTeamTaskRemoved, protocol.TeamRemoved{ID: task.ID})
			continue
		}
		kept = append(kept, task)
	}
	t.tasks = kept
	s.emitTeam(team)
	return nil
}

func (s *server) listRoutines(team string) protocol.RoutinesResult {
	routines := make([]protocol.Routine, 0)
	for _, routine := range s.data.teams.routines {
		if team == "" || routine.Team == team {
			routines = append(routines, s.readRoutine(routine))
		}
	}
	sort.SliceStable(routines, func(i, j int) bool {
		if routines[i].Team != routines[j].Team {
			return routines[i].Team < routines[j].Team
		}
		return routines[i].Title < routines[j].Title
	})
	return protocol.RoutinesResult{Routines: routines}
}

// refuses a routine the daemon would refuse: no team, no title or brief, not a schedule, too often, too many on
func (s *server) checkRoutine(routine protocol.Routine) error {
	t := &s.data.teams
	team := t.teamIndex(routine.Team)
	if team < 0 {
		return notFound("team")
	}
	if strings.TrimSpace(routine.Title) == "" {
		return badRequest(i18n.T("A routine needs a title."))
	}
	if strings.TrimSpace(routine.Brief) == "" {
		return badRequest(i18n.T("Say what the routine should do."))
	}
	if routine.Member != "" {
		if _, ok := teamMember(t.teams[team], routine.Member); !ok {
			return badRequest(i18n.T("That member is not on the team."))
		}
	}
	schedule, err := cron.Parse(routine.Schedule)
	if err != nil {
		return badRequest(err.Error())
	}
	if schedule.MinInterval() < protocol.RoutineMinInterval {
		return badRequest(i18n.T("A routine runs at most every 15 minutes."))
	}
	if routine.Enabled {
		on := 0
		for _, other := range t.routines {
			if other.Enabled && other.ID != routine.ID {
				on++
			}
		}
		if on >= protocol.RoutinesEnabledMax {
			return badRequest(i18n.Tf("At most %d routines can be on at once.", protocol.RoutinesEnabledMax))
		}
	}
	return nil
}

func (s *server) createRoutine(p protocol.RoutineCreateParams) (protocol.Routine, error) {
	t := &s.data.teams
	now := time.Now()
	routine := protocol.Routine{
		ID: t.newID("routine"), Team: p.Team, Title: strings.TrimSpace(p.Title), Brief: strings.TrimSpace(p.Brief), Member: p.Member,
		Schedule: strings.TrimSpace(p.Schedule), Enabled: p.Enabled, Missed: p.Missed, Overlap: p.Overlap, Created: now, Updated: now,
	}
	if routine.Missed != protocol.MissedOnce {
		routine.Missed = protocol.MissedSkip
	}
	if routine.Overlap != protocol.OverlapQueue {
		routine.Overlap = protocol.OverlapSkip
	}
	if err := s.checkRoutine(routine); err != nil {
		return protocol.Routine{}, err
	}
	t.routines = append(t.routines, routine)
	s.record(protocol.Activity{Action: "routine.create", Target: routine.Title, OK: true, Team: routine.Team, Detail: s.readRoutine(routine).Sentence})
	s.emitRoutine(routine)
	if p.RunNow {
		if _, err := s.runRoutine(routine.ID); err != nil {
			return protocol.Routine{}, err
		}
	}
	return s.readRoutine(t.routines[t.routineIndex(routine.ID)]), nil
}

func (s *server) updateRoutine(p protocol.RoutineUpdateParams) (protocol.Routine, error) {
	t := &s.data.teams
	index := t.routineIndex(p.ID)
	if index < 0 {
		return protocol.Routine{}, notFound("routine")
	}
	before := t.routines[index]
	routine := before
	if err := mergeInto(p.Patch, &routine); err != nil {
		return protocol.Routine{}, err
	}
	// a patch changes only what the user may change; what the clock and its runs set stays
	routine.ID, routine.Team, routine.Created, routine.Failures = before.ID, before.Team, before.Created, before.Failures
	routine.LastRun, routine.LastTask, routine.LastStatus, routine.NextRun, routine.Sentence = before.LastRun, before.LastTask, before.LastStatus, nil, ""
	routine.Title, routine.Brief, routine.Schedule = strings.TrimSpace(routine.Title), strings.TrimSpace(routine.Brief), strings.TrimSpace(routine.Schedule)
	if routine.Enabled && !before.Enabled {
		routine.Failures = 0
	}
	if err := s.checkRoutine(routine); err != nil {
		return protocol.Routine{}, err
	}
	routine.Updated = time.Now()
	t.routines[index] = routine
	detail := ""
	switch {
	case routine.Enabled && !before.Enabled:
		detail = i18n.T("Turned on.")
	case !routine.Enabled && before.Enabled:
		detail = i18n.T("Turned off.")
	}
	s.record(protocol.Activity{Action: "routine.update", Target: routine.Title, OK: true, Team: routine.Team, Detail: detail})
	s.emitRoutine(routine)
	return s.readRoutine(routine), nil
}

func (s *server) deleteRoutine(id string) error {
	t := &s.data.teams
	index := t.routineIndex(id)
	if index < 0 {
		return notFound("routine")
	}
	routine := t.routines[index]
	t.routines = append(t.routines[:index:index], t.routines[index+1:]...)
	s.record(protocol.Activity{Action: "routine.delete", Target: routine.Title, OK: true, Team: routine.Team})
	s.tellTeams(protocol.EvtRoutineRemoved, protocol.TeamRemoved{ID: id})
	s.emitTeam(routine.Team)
	return nil
}

// one run now, the way the clock would start it
func (s *server) runRoutine(id string) (protocol.TeamTask, error) {
	t := &s.data.teams
	index := t.routineIndex(id)
	if index < 0 {
		return protocol.TeamTask{}, notFound("routine")
	}
	routine := t.routines[index]
	teamIndex := t.teamIndex(routine.Team)
	if teamIndex < 0 {
		return protocol.TeamTask{}, notFound("team")
	}
	team := t.teams[teamIndex]
	if team.Paused {
		return protocol.TeamTask{}, badRequest(i18n.Tf("%s is paused, so its routines do not run.", team.Name))
	}
	if s.data.snapshot.Settings.Teams.Paused {
		return protocol.TeamTask{}, badRequest(i18n.T("Every team is paused, so no routine runs."))
	}
	member, _ := teamMember(team, routine.Member)
	brief := i18n.Tf("%s\n\nThis run is from the routine \"%s\"; when nothing changed since the last run, say so in one line in your report.", routine.Brief, routine.Title)
	s.record(protocol.Activity{Action: "routine.fire", Target: routine.Title, OK: true, Team: team.ID})
	task := s.newTeamTask(team, member.ID, routine.Title, brief, protocol.TaskFromRoutine, routine.ID, "", nil)
	now := time.Now()
	index = t.routineIndex(id)
	t.routines[index].LastRun, t.routines[index].LastTask, t.routines[index].LastStatus = &now, task.ID, task.Status
	s.emitRoutine(t.routines[index])
	return task, nil
}

func (s *server) listApprovals(p protocol.ApprovalsParams) protocol.ApprovalsResult {
	approvals := make([]protocol.Approval, 0)
	for _, approval := range s.data.teams.approvals {
		if p.Team != "" && approval.Team != p.Team || p.Pending && approval.Status != protocol.ApprovalPending {
			continue
		}
		approvals = append(approvals, approval)
		if p.Limit > 0 && len(approvals) == p.Limit {
			break
		}
	}
	return protocol.ApprovalsResult{Approvals: approvals}
}

func (s *server) teamActivity(p protocol.TeamActivityParams) protocol.ActivityResult {
	var before time.Time
	if p.Before != "" {
		before, _ = time.Parse(time.RFC3339Nano, p.Before)
	}
	limit := p.Limit
	if limit <= 0 {
		limit = 50
	}
	entries := make([]protocol.Activity, 0)
	for _, entry := range s.data.teams.activity {
		if p.Team != "" && entry.Team != p.Team || !before.IsZero() && !entry.Time.Before(before) {
			continue
		}
		keep := true
		switch p.Filter {
		case protocol.ActivityUnwatched:
			keep = entry.Unwatched
		case protocol.ActivityApprovals:
			keep = strings.HasPrefix(entry.Action, "approval.")
		case protocol.ActivityCredentials:
			keep = strings.HasPrefix(entry.Action, "credential.") || strings.HasPrefix(entry.Action, "mail.")
		case protocol.ActivityFailures:
			keep = !entry.OK
		}
		if !keep {
			continue
		}
		entries = append(entries, entry)
		if len(entries) == limit {
			break
		}
	}
	return protocol.ActivityResult{Activity: entries}
}

// a month of one team's tokens, or every team's together; the demo only remembers this month
func (s *server) teamSpend(p protocol.TeamSpendParams) (protocol.TeamSpend, error) {
	t := &s.data.teams
	month := demoMonth(Epoch)
	if p.Month != "" && p.Month != month {
		return protocol.TeamSpend{Team: p.Team, Month: p.Month, Days: []protocol.DaySpend{}}, nil
	}
	if p.Team != "" {
		index := t.teamIndex(p.Team)
		if index < 0 {
			return protocol.TeamSpend{}, notFound("team")
		}
		spend := t.spend[p.Team]
		spend.Cap = t.teams[index].Budget.MonthTokens
		return spend, nil
	}
	all := protocol.TeamSpend{Month: month, Days: spreadMonth(Epoch, 0, 1)}
	for _, team := range s.listTeams() {
		spend := t.spend[team.ID]
		all.Tokens += spend.Tokens
		all.Cap += team.Budget.MonthTokens
		all.Estimated = all.Estimated || spend.Estimated
		tasks := 0
		for _, member := range spend.Members {
			tasks += member.Tasks
		}
		all.Teams = append(all.Teams, protocol.NamedSpend{ID: team.ID, Name: team.Name, Tokens: spend.Tokens, Tasks: tasks})
		all.Engines = addEngines(all.Engines, spend.Engines)
		for index := range all.Days {
			if index < len(spend.Days) {
				all.Days[index].Input += spend.Days[index].Input
				all.Days[index].Output += spend.Days[index].Output
			}
		}
	}
	return all, nil
}

// adds one team's spend by engine to a sum of several, each engine once
func addEngines(sum, team []protocol.NamedSpend) []protocol.NamedSpend {
	for _, engine := range team {
		if index := slices.IndexFunc(sum, func(named protocol.NamedSpend) bool { return named.ID == engine.ID }); index >= 0 {
			sum[index].Tokens += engine.Tokens
			sum[index].Tasks += engine.Tasks
			continue
		}
		sum = append(sum, engine)
	}
	return sum
}

// deletes finished tasks with their sessions: past the kept days, or every one when asked; what they made stays in the folder
func (s *server) pruneTasks(p protocol.TeamPruneParams) error {
	t := &s.data.teams
	if p.Team != "" && t.teamIndex(p.Team) < 0 {
		return notFound("team")
	}
	days := s.data.snapshot.Settings.Teams.KeepDays
	if !p.All && days == 0 {
		return nil
	}
	before := time.Now().AddDate(0, 0, -days)
	pruned := make(map[string]int)
	kept := t.tasks[:0:0]
	for _, task := range t.tasks {
		ended := task.Updated
		if task.Ended != nil {
			ended = *task.Ended
		}
		if p.Team != "" && task.Team != p.Team || taskOpen(task) || !p.All && !ended.Before(before) {
			kept = append(kept, task)
			continue
		}
		for _, session := range task.Sessions {
			s.deleteSession(session)
		}
		pruned[task.Team]++
		s.tellTeams(protocol.EvtTeamTaskRemoved, protocol.TeamRemoved{ID: task.ID})
	}
	t.tasks = kept
	for _, team := range t.teams {
		if count := pruned[team.ID]; count > 0 {
			s.record(protocol.Activity{Action: "task.prune", Target: team.Name, OK: true, Team: team.ID, Detail: i18n.Tf("Deleted %d finished tasks and their conversations.", count)})
			s.emitTeam(team.ID)
		}
	}
	return nil
}

// every team held at once stops what works; let go, each team that is not paused itself starts what queued
func (s *server) holdEveryTeam(paused bool) {
	t := &s.data.teams
	if paused {
		s.record(protocol.Activity{Action: "teams.pause", Target: i18n.T("Every team"), OK: true, Detail: i18n.T("Nothing starts and no routine fires until you turn this off.")})
		for task := range t.tasks {
			if t.tasks[task].Status == protocol.TaskWorking {
				s.stopTeamTask(task, i18n.T("You paused every team."))
			}
		}
		return
	}
	s.record(protocol.Activity{Action: "teams.resume", Target: i18n.T("Every team"), OK: true})
	for _, team := range t.teams {
		s.startQueued(team)
		s.emitTeam(team.ID)
	}
}

// a harness by its name, its id when it is gone
func (s *server) harnessName(id string) string {
	for _, harness := range s.data.snapshot.Harnesses {
		if harness.ID == id {
			return harness.Name
		}
	}
	return id
}

func engineModel(model string) string {
	if model == "" {
		return i18n.T("its default model")
	}
	return model
}
