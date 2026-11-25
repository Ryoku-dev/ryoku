package teams

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	i18n "ryoku-i18n"

	"ryoku-rashin/internal/config"
	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/protocol"
)

// teamEdit is the part of a team teams.update may change.
type teamEdit struct {
	Name         string              `json:"name"`
	Purpose      string              `json:"purpose"`
	Structure    string              `json:"structure,omitempty"`
	Culture      string              `json:"culture,omitempty"`
	Reporting    string              `json:"reporting,omitempty"`
	PlanApproval bool                `json:"planApproval,omitempty"`
	Folder       string              `json:"folder"`
	Members      []protocol.Member   `json:"members"`
	Tools        protocol.TeamTools  `json:"tools"`
	Approvals    string              `json:"approvals"`
	Budget       protocol.TeamBudget `json:"budget"`
	Notify       string              `json:"notify"`
	Running      int                 `json:"running"`
	Access       protocol.TeamAccess `json:"access"`
	Memory       protocol.TeamMemory `json:"memory"`
	SkillsOff    []string            `json:"skillsOff"`
}

// listTeams lists every team by name with its stats.
func (s *Service) listTeams() protocol.TeamsResult {
	teams := s.state.teams()
	s.fillStats(teams)
	return protocol.TeamsResult{Teams: teams}
}

// emitTeam tells clients a team changed, with fresh stats.
func (s *Service) emitTeam(id string) {
	if team, ok := s.teamWithStats(id); ok {
		s.broadcast(protocol.EvtTeam, team)
	}
}

func (s *Service) teamWithStats(id string) (protocol.Team, bool) {
	team, ok := s.state.team(id)
	if !ok {
		return protocol.Team{}, false
	}
	list := []protocol.Team{team}
	s.fillStats(list)
	return list[0], true
}

// fillStats derives each team's stats from its tasks, spend and routines.
func (s *Service) fillStats(teams []protocol.Team) {
	if len(teams) == 0 {
		return
	}
	now := s.now()
	index := make(map[string]*protocol.TeamStats, len(teams))
	for i := range teams {
		teams[i].Stats = protocol.TeamStats{MonthTokens: s.monthTokens(teams[i].ID, now), MonthUnmetered: s.monthUnmetered(teams[i].ID, now)}
		index[teams[i].ID] = &teams[i].Stats
	}
	for _, t := range s.state.tasks(nil) {
		stats := index[t.Team]
		if stats == nil {
			continue
		}
		stats.Tasks++
		switch t.Status {
		case protocol.TaskWorking:
			stats.Working++
			if t.Member != "" {
				if stats.Doing == nil {
					stats.Doing = make(map[string]string)
				}
				// tasks come newest first, so a member shows its latest task
				if _, busy := stats.Doing[t.Member]; !busy {
					stats.Doing[t.Member] = t.Title
				}
			}
		case protocol.TaskReady:
			stats.Queued++
		case protocol.TaskNeedsYou:
			stats.NeedsYou++
		case protocol.TaskDone:
			stats.Done++
		}
		if t.Report != nil && (stats.LastReport == nil || t.Report.At.After(*stats.LastReport)) {
			at := t.Report.At
			stats.LastReport = &at
		}
	}
	paused := make(map[string]bool, len(teams))
	for _, team := range teams {
		paused[team.ID] = team.Paused
	}
	for _, r := range s.listRoutines("").Routines {
		stats := index[r.Team]
		if stats == nil || paused[r.Team] || !r.Enabled || r.NextRun == nil {
			continue
		}
		if stats.NextRoutine == nil || r.NextRun.Before(*stats.NextRoutine) {
			next := *r.NextRun
			stats.NextRoutine = &next
		}
	}
}

// createTeam makes a team and the routines it starts with, then asks for the
// credentials its template needs that the vault does not hold.
func (s *Service) createTeam(ctx context.Context, p protocol.TeamCreateParams) (protocol.Team, error) {
	template, _ := templateByID(p.Template)
	return s.makeTeam(ctx, p, template.Credentials)
}

// makeTeam makes a team and its routines, then opens a credential request
// for each of needs the vault does not hold.
func (s *Service) makeTeam(ctx context.Context, p protocol.TeamCreateParams, needs []protocol.CredentialNeed) (protocol.Team, error) {
	team, err := s.addTeam(ctx, newTeam(p, s.now()), p.Routines, "team.create", createdDetail)
	if err != nil || len(needs) == 0 {
		return team, err
	}
	s.askNeeds(ctx, team, needs)
	asked, _ := s.teamWithStats(team.ID)
	return asked, nil
}

// newTeam is the team teams.create would make from p, not yet checked.
func newTeam(p protocol.TeamCreateParams, now time.Time) protocol.Team {
	return protocol.Team{
		ID: newID("team"), Name: p.Name, Purpose: p.Purpose, Template: p.Template,
		Structure: p.Structure, Culture: p.Culture, Reporting: p.Reporting, PlanApproval: p.PlanApproval,
		Folder: p.Folder, Members: p.Members, Tools: p.Tools, Approvals: p.Approvals,
		Budget: p.Budget, Notify: p.Notify, Running: p.Running, Access: p.Access,
		Created: now, Updated: now,
	}
}

// createdDetail is the ledger's detail for a team just made.
func createdDetail(t protocol.Team) string {
	if len(t.Members) == 1 {
		return i18n.Tf("Made with one member, working in %s.", paths.Short(t.Folder))
	}
	return i18n.Tf("Made with %d members, working in %s.", len(t.Members), paths.Short(t.Folder))
}

// prepareTeam checks a new team as it would be stored: it keeps what the
// teams defaults say new teams keep, and a team that names no budget takes
// the teams defaults', with its template's turns per task. Nothing is stored
// and no folder is made.
func (s *Service) prepareTeam(team *protocol.Team, settings protocol.Settings) (map[string]string, error) {
	if team.Memory == nil {
		memory := settings.Teams.Defaults.Memory
		team.Memory = &memory
	}
	if team.Budget == (protocol.TeamBudget{}) {
		team.Budget = settings.Teams.Defaults.Budget
		if template, ok := templateByID(team.Template); ok {
			team.Budget.TaskTurns = template.Budget.TaskTurns
		}
	}
	return normaliseTeam(team, true, settings)
}

// checkNewTeam checks what teams.create would check of p, its routines
// included, without storing anything or making its folder.
func (s *Service) checkNewTeam(p protocol.TeamCreateParams) error {
	team := newTeam(p, s.now())
	renamed, err := s.prepareTeam(&team, s.settings())
	if err != nil {
		return err
	}
	on := 0
	for _, draft := range p.Routines {
		r := protocol.Routine{
			Team: team.ID, Title: draft.Title, Brief: draft.Brief,
			Member: routineMember(team, draft.Member, renamed), Schedule: draft.Schedule, Enabled: draft.Enabled,
		}
		if err := s.routineFits(team, &r, on); err != nil {
			return err
		}
		if r.Enabled {
			on++
		}
	}
	return nil
}

// addTeam validates and stores a new team, then creates its routines through
// the routine path. A routine that cannot be made takes the team back out,
// so a create either happens whole or not at all.
func (s *Service) addTeam(ctx context.Context, team protocol.Team, drafts []protocol.RoutineDraft, action string, detail func(protocol.Team) string) (protocol.Team, error) {
	renamed, err := s.prepareTeam(&team, s.settings())
	if err != nil {
		return protocol.Team{}, err
	}
	if err := makeFolder(team.Folder); err != nil {
		return protocol.Team{}, err
	}
	if err := s.state.putTeam(team); err != nil {
		return protocol.Team{}, err
	}
	for _, draft := range drafts {
		_, err := s.createRoutine(ctx, protocol.RoutineCreateParams{
			Team: team.ID, Title: draft.Title, Brief: draft.Brief,
			Member: routineMember(team, draft.Member, renamed), Schedule: draft.Schedule,
			Enabled: draft.Enabled,
		})
		if err != nil {
			s.dropRoutines(team.ID)
			_ = s.state.removeTeam(team.ID)
			return protocol.Team{}, err
		}
	}
	s.record(protocol.Activity{
		Actor: "you", Action: action, Target: team.Name, OK: true, Detail: detail(team),
		Team: team.ID, Initiator: protocol.InitiatorYou,
	})
	s.emitTeam(team.ID)
	created, _ := s.teamWithStats(team.ID)
	return created, nil
}

// updateTeam applies a JSON merge patch to the parts of a team the user may
// change; its id, creation time and pause stay as they are. The ledger says
// what changed; a change that changes nothing is not stored.
func (s *Service) updateTeam(ctx context.Context, p protocol.TeamUpdateParams) (protocol.Team, error) {
	return s.updateTeamConfig(ctx, p, "")
}

func (s *Service) updateTeamConfig(ctx context.Context, p protocol.TeamUpdateParams, forceMember string) (protocol.Team, error) {
	if err := s.state.writer.available(); err != nil {
		return protocol.Team{}, err
	}
	current, ok := s.state.team(p.ID)
	if !ok {
		return protocol.Team{}, protocol.Errorf(protocol.CodeNotFound, "%s", i18n.Tf("No team %s.", p.ID))
	}
	if !p.ExpectedUpdated.IsZero() && !p.ExpectedUpdated.Equal(current.Updated) {
		return protocol.Team{}, protocol.Errorf(protocol.CodeConflict, "%s", i18n.T("The team changed since it was opened. Reload before saving."))
	}
	base, err := json.Marshal(teamEdit{
		Name: current.Name, Purpose: current.Purpose, Folder: current.Folder, Members: current.Members,
		Structure: current.Structure, Culture: current.Culture, Reporting: current.Reporting, PlanApproval: current.PlanApproval,
		Tools: current.Tools, Approvals: current.Approvals, Budget: current.Budget, Notify: current.Notify,
		Running: current.Running, Access: current.Access, Memory: current.TeamMemory(), SkillsOff: current.SkillsOff,
	})
	if err != nil {
		return protocol.Team{}, err
	}
	var doc, patch any
	baseDecoder := json.NewDecoder(bytes.NewReader(base))
	baseDecoder.UseNumber()
	if err := baseDecoder.Decode(&doc); err != nil {
		return protocol.Team{}, err
	}
	if !json.Valid(p.Patch) {
		return protocol.Team{}, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("The team change is not a JSON object."))
	}
	patchDecoder := json.NewDecoder(bytes.NewReader(p.Patch))
	patchDecoder.UseNumber()
	if err := patchDecoder.Decode(&patch); err != nil {
		return protocol.Team{}, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("The team change is not a JSON object."))
	}
	if _, isObject := patch.(map[string]any); !isObject {
		return protocol.Team{}, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("The team change is not a JSON object."))
	}
	merged, err := json.Marshal(mergePatch(doc, patch))
	if err != nil {
		return protocol.Team{}, err
	}
	var next teamEdit
	if err := json.Unmarshal(merged, &next); err != nil {
		return protocol.Team{}, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("The team change has the wrong shape."))
	}
	team := current
	team.Name, team.Purpose, team.Folder, team.Members = next.Name, next.Purpose, next.Folder, next.Members
	team.Structure, team.Culture, team.Reporting, team.PlanApproval = next.Structure, next.Culture, next.Reporting, next.PlanApproval
	team.Tools, team.Approvals, team.Budget, team.Notify = next.Tools, next.Approvals, next.Budget, next.Notify
	team.Running, team.Access = next.Running, next.Access
	team.SkillsOff = slices.DeleteFunc(slices.Compact(slices.Sorted(slices.Values(next.SkillsOff))), func(id string) bool { return id == "" })
	if next.Memory != current.TeamMemory() {
		team.Memory = &next.Memory
	}
	if current.Dream != nil {
		dream := cloneDream(*current.Dream)
		for _, member := range team.Members {
			if member.Lead && member.ID != "" {
				dream.Manager = member.ID
			}
		}
		team.Dream = &dream
	}
	if _, err := normaliseTeam(&team, false, s.settings()); err != nil {
		return protocol.Team{}, err
	}
	if err := AdvanceMemberRevisions(current.Members, team.Members); err != nil {
		return protocol.Team{}, err
	}
	if forceMember != "" {
		old, exists := protocol.MemberByID(current.Members, forceMember)
		if !exists {
			return protocol.Team{}, protocol.Errorf(protocol.CodeNotFound, "This member no longer exists.")
		}
		for i := range team.Members {
			if team.Members[i].ID == forceMember && team.Members[i].Revision == old.Revision {
				if old.Revision == math.MaxUint64 {
					return protocol.Team{}, protocol.Errorf(protocol.CodeConflict, "Member configuration revisions are exhausted.")
				}
				team.Members[i].Revision++
			}
		}
	}
	if err := s.checkConfigReferences(team, current, forceMember); err != nil {
		return protocol.Team{}, err
	}
	if err := makeFolder(team.Folder); err != nil {
		return protocol.Team{}, err
	}
	changes := s.teamChanges(current, team)
	if forceMember != "" && len(changes) == 0 {
		changes = []string{i18n.T("Restored member configuration.")}
	}
	if len(changes) == 0 {
		unchanged, _ := s.teamWithStats(team.ID)
		if err := s.state.writer.available(); err != nil {
			return protocol.Team{}, err
		}
		return unchanged, nil
	}
	parked, err := s.beginTeamEdit(ctx, current, team)
	if err != nil {
		return protocol.Team{}, err
	}
	if parked {
		defer s.endTeamEdit(team.ID)
	}
	team.Updated = s.now()
	if !team.Updated.After(current.Updated) {
		team.Updated = current.Updated.Add(time.Nanosecond)
	}
	s.mu.Lock()
	live, exists := s.state.team(team.ID)
	if !exists || !reflect.DeepEqual(live, current) {
		s.mu.Unlock()
		return protocol.Team{}, protocol.Errorf(protocol.CodeConflict, "%s", i18n.T("The team changed while it was being edited. Reload before saving."))
	}
	if err := s.checkConfigReferences(team, current, forceMember); err != nil {
		s.mu.Unlock()
		return protocol.Team{}, err
	}
	err = s.state.putTeam(team)
	s.mu.Unlock()
	if err != nil {
		return protocol.Team{}, err
	}
	s.record(protocol.Activity{
		Actor: "you", Action: "team.update", Target: team.Name, OK: true,
		Detail: strings.Join(changes, " "), Team: team.ID, Initiator: protocol.InitiatorYou,
	})
	s.armTeamMinutes(team.ID)
	s.emitTeam(team.ID)
	s.checkBudget(team.ID, 0)
	for _, member := range team.Members {
		s.checkMemberBudget(team.ID, member.ID, 0)
	}
	s.pump()
	committed := []protocol.Team{team}
	s.fillStats(committed)
	return committed[0], nil
}

// deleteTeam stops a team's work and removes it with its tasks, routines,
// approvals and grants, then forgets what it remembers. Its folder stays: it
// holds what the team made. Forgetting comes last, so no task of the team can
// remember anything after it, and a memory engine that fails never keeps the
// team.
func (s *Service) deleteTeam(ctx context.Context, id string) error {
	s.taskControl.Lock()
	defer s.taskControl.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	team, ok := s.state.team(id)
	if !ok {
		s.mu.Unlock()
		return protocol.Errorf(protocol.CodeNotFound, "%s", i18n.Tf("No team %s.", id))
	}
	if s.editing[id] != nil {
		s.mu.Unlock()
		return protocol.Errorf(protocol.CodeConflict, "This team is changing execution policy.")
	}
	tasks := s.state.tasks(func(t protocol.TeamTask) bool { return t.Team == id })
	for _, task := range tasks {
		if s.taskEdits[task.ID] {
			s.mu.Unlock()
			return protocol.Errorf(protocol.CodeConflict, "A task is changing execution or waiting for storage repair.")
		}
	}
	s.editing[id] = make(map[string]bool, len(team.Members))
	for _, member := range team.Members {
		s.editing[id][member.ID] = true
	}
	for _, task := range tasks {
		s.taskEdits[task.ID] = true
	}
	s.mu.Unlock()
	s.stopTeamTasks(id, i18n.T("The team was deleted."))
	var failure error
	for _, task := range tasks {
		if err := s.stopReassignedSessions(ctx, task); err != nil && failure == nil {
			failure = err
		}
	}
	if failure != nil {
		return failure
	}
	s.dropRoutines(id)
	s.dropGrants(id)
	if err := paths.RemoveWritableTree(filepath.Join(paths.ToolHomes(), id)); err != nil {
		return err
	}
	s.mu.Lock()
	err := s.state.removeTeam(id)
	if err == nil {
		delete(s.editing, id)
		for _, task := range tasks {
			delete(s.taskEdits, task.ID)
		}
	}
	s.mu.Unlock()
	if err != nil {
		return err
	}
	s.forgetTeam(ctx, team)
	for _, task := range tasks {
		s.broadcast(protocol.EvtTeamTaskRemoved, protocol.TeamRemoved{ID: task.ID})
	}
	s.broadcast(protocol.EvtTeamRemoved, protocol.TeamRemoved{ID: id})
	s.record(protocol.Activity{
		Actor: "you", Action: "team.delete", Target: team.Name, OK: true,
		Detail: i18n.Tf("Deleted the team with its tasks and routines; its folder %s stays.", paths.Short(team.Folder)),
		Team:   id, Initiator: protocol.InitiatorYou,
	})
	s.pump()
	s.clock.rearm()
	return nil
}

// pauseTeam pauses a team, stopping its working tasks, or lets it work again.
// A team its spending limit paused starts again only with a higher limit.
// Letting a dream work, paused or not, starts its manager again when no
// standing task is open.
func (s *Service) pauseTeam(_ context.Context, p protocol.TeamPauseParams) (protocol.Team, error) {
	s.mu.Lock()
	team, ok := s.state.team(p.ID)
	if !ok {
		s.mu.Unlock()
		return protocol.Team{}, protocol.Errorf(protocol.CodeNotFound, "%s", i18n.Tf("No team %s.", p.ID))
	}
	if team.Paused == p.Paused {
		s.mu.Unlock()
		if !p.Paused {
			s.runDream(team.ID)
		}
		current, _ := s.teamWithStats(team.ID)
		return current, nil
	}
	now := s.now()
	if p.Paused {
		team.Paused, team.PauseReason, team.Updated = true, protocol.PausedByYou, now
		err := s.state.putTeam(team)
		s.mu.Unlock()
		if err != nil {
			return protocol.Team{}, err
		}
		s.clock.rearm()
		s.stopTeamTasks(team.ID, i18n.T("The team was paused."))
		s.record(protocol.Activity{
			Actor: "you", Action: "team.pause", Target: team.Name, OK: true,
			Detail: i18n.T("Paused the team and stopped its working tasks."), Team: team.ID, Initiator: protocol.InitiatorYou,
		})
		s.emitTeam(team.ID)
		paused, _ := s.teamWithStats(team.ID)
		return paused, nil
	}
	detail := i18n.T("Let the team work again.")
	byBudget := team.PauseReason == protocol.PausedByBudget
	if byBudget && (team.Budget.MonthTokens > 0 || p.Raise > 0) {
		used, err := s.budgetTokens(team.ID, "", now)
		if err != nil {
			s.mu.Unlock()
			return protocol.Team{}, protocol.Errorf(protocol.CodeConflict, "Usage is incomplete or unavailable. This team cannot resume under a token cap.")
		}
		if p.Raise <= used {
			s.mu.Unlock()
			return protocol.Team{}, protocol.Errorf(protocol.CodeBadRequest, "%s",
				i18n.Tf("To start %s again, raise its spending limit above the %d tokens it used this month.", team.Name, used))
		}
		budget := team.Budget
		budget.MonthTokens = p.Raise
		if err := config.CheckBudget(budget); err != nil {
			s.mu.Unlock()
			return protocol.Team{}, err
		}
		team.Budget = budget
		detail = i18n.Tf("Raised the spending limit to %d tokens a month and let the team work again.", p.Raise)
	}
	team.Paused, team.PauseReason, team.Updated = false, "", now
	err := s.state.putTeam(team)
	s.mu.Unlock()
	if err != nil {
		return protocol.Team{}, err
	}
	if byBudget {
		s.settleBudgetAsks(team.ID)
	}
	s.clock.rearm()
	s.record(protocol.Activity{
		Actor: "you", Action: "team.resume", Target: team.Name, OK: true,
		Detail: detail, Team: team.ID, Initiator: protocol.InitiatorYou,
	})
	s.emitTeam(team.ID)
	s.pump()
	s.runDream(team.ID)
	resumed, _ := s.teamWithStats(team.ID)
	return resumed, nil
}

// settleBudgetAsks closes the team's open spending limit cards once the user
// raised the limit from the team itself, so the Inbox does not ask again.
func (s *Service) settleBudgetAsks(team string) {
	pending := s.state.approvals(func(r approvalRecord) bool {
		return r.Team == team && r.Kind == protocol.ApprovalBudget && r.Status == protocol.ApprovalPending
	})
	for _, rec := range pending {
		claimed, err := s.claimApproval(rec.ID, protocol.ApprovalApproved, protocol.DecideOnce, "")
		if err != nil {
			continue
		}
		claimed.Result = i18n.T("You raised the limit from the team.")
		if s.state.putApproval(claimed) == nil {
			s.emitApproval(claimed.Approval)
		}
	}
	s.armExpiry()
}

// normaliseTeam trims and checks what the user gave a team and fills what
// it left empty from the teams defaults. With fresh every member gets a new
// id; renamed maps the ids they came with to the new ones, so routines that
// named a member keep it.
func normaliseTeam(t *protocol.Team, fresh bool, settings protocol.Settings) (map[string]string, error) {
	t.Name = strings.TrimSpace(t.Name)
	if t.Name == "" {
		return nil, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("A team needs a name."))
	}
	if utf8.RuneCountInString(t.Name) > protocol.TeamNameMaxRunes {
		return nil, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.Tf("A team's name has at most %d characters.", protocol.TeamNameMaxRunes))
	}
	t.Purpose = strings.TrimSpace(t.Purpose)
	if _, ok := templateByID(t.Template); !ok {
		t.Template = ""
	}
	members, renamed, err := normaliseMembers(t.Members, fresh, t.Reporting)
	if err != nil {
		return nil, err
	}
	t.Members = members
	if err := normaliseReporting(t, renamed); err != nil {
		return nil, err
	}
	if err := normaliseMemberConfig(t, settings); err != nil {
		return nil, err
	}
	defaults := settings.Teams.Defaults
	if t.Approvals == "" {
		t.Approvals = defaults.Approvals
	}
	if t.Approvals == "" {
		t.Approvals = protocol.ApprovalsAsk
	}
	if !protocol.KnownApprovals(t.Approvals) {
		return nil, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.Tf("%s is not a permission setting.", t.Approvals))
	}
	if t.Notify == "" {
		t.Notify = defaults.Notify
	}
	switch t.Notify {
	case "":
		t.Notify = protocol.NotifyAll
	case protocol.NotifyAll, protocol.NotifyNeeds, protocol.NotifyNone:
	default:
		return nil, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.Tf("%s is not a notification setting.", t.Notify))
	}
	if err := normaliseBudget(&t.Budget, defaults.Budget); err != nil {
		return nil, err
	}
	if t.Running < 0 || t.Running > protocol.TeamsPerTeamMax {
		return nil, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.Tf("Tasks at once in one team must be between 1 and %d.", protocol.TeamsPerTeamMax))
	}
	folder, err := teamFolder(t.Folder, t.Name)
	if err != nil {
		return nil, err
	}
	t.Folder = folder
	if err := normaliseAccess(&t.Access, t.Folder); err != nil {
		return nil, err
	}
	return renamed, nil
}

// NormalizeDefinition lets the demo use the production definition boundary
// without creating folders, running agents or persisting a real team. It checks
// only the submitted roster/policy, never rehydrates a structure recipe.
func NormalizeDefinition(t *protocol.Team, fresh bool, settings protocol.Settings) (map[string]string, error) {
	return normaliseTeam(t, fresh, settings)
}

// normaliseMembers assigns IDs before reporting references are rebased. An
// unspecified mode retains the existing star as a one-way configuration default.
func normaliseMembers(in []protocol.Member, fresh bool, reporting string) ([]protocol.Member, map[string]string, error) {
	if len(in) == 0 {
		return nil, nil, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("A team needs at least one member."))
	}
	if len(in) > protocol.TeamsMembersMax {
		return nil, nil, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.Tf("A team has at most %d members.", protocol.TeamsMembersMax))
	}
	out := make([]protocol.Member, 0, len(in))
	renamed := make(map[string]string)
	names := make(map[string]bool, len(in))
	ids := make(map[string]bool, len(in))
	lead := -1
	for i, m := range in {
		m.Name = strings.TrimSpace(m.Name)
		m.Title = strings.TrimSpace(m.Title)
		m.Model = strings.TrimSpace(m.Model)
		m.Instructions = strings.TrimSpace(m.Instructions)
		if len(m.Options) == 0 {
			m.Options = nil
		}
		if m.Name == "" {
			return nil, nil, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("Every member needs a name."))
		}
		if err := checkEngine(m); err != nil {
			return nil, nil, err
		}
		if utf8.RuneCountInString(m.Name) > protocol.TeamNameMaxRunes {
			return nil, nil, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.Tf("A member's name has at most %d characters.", protocol.TeamNameMaxRunes))
		}
		key := strings.ToLower(m.Name)
		if names[key] {
			return nil, nil, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.Tf("Two members are called %s.", m.Name))
		}
		names[key] = true
		old := strings.TrimSpace(m.ID)
		if old != "" && ids[old] {
			return nil, nil, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.Tf("Two members use the same ID %s.", old))
		}
		if old != "" {
			ids[old] = true
		}
		if fresh || old == "" {
			m.ID = newID("member")
			if old != "" {
				renamed[old] = m.ID
			}
		} else {
			m.ID = old
		}
		if m.Lead && lead < 0 {
			lead = i
		}
		if reporting == "" {
			m.Lead = false
		}
		out = append(out, m)
	}
	if reporting == "" {
		out[max(lead, 0)].Lead = true
	}
	return out, renamed, nil
}

// normaliseBudget checks a team's budget and fills the warning and turns it
// left at 0 from defaults; a monthly limit or minutes per task of 0 is no
// limit.
func normaliseBudget(b *protocol.TeamBudget, defaults protocol.TeamBudget) error {
	if err := config.CheckBudget(*b); err != nil {
		return err
	}
	if b.WarnPercent == 0 {
		b.WarnPercent = defaults.WarnPercent
	}
	if b.TaskTurns == 0 {
		b.TaskTurns = defaults.TaskTurns
	}
	return nil
}

// normaliseAccess checks what a team's sandbox adds: folders it may change
// (writableFolder) and paths hidden from it (config.CleanHidden), each kept
// once in "~/" form.
func normaliseAccess(a *protocol.TeamAccess, own string) error {
	hidden, err := config.CleanHidden(a.Hidden)
	if err != nil {
		return err
	}
	if len(hidden) == 0 {
		hidden = nil
	}
	a.Hidden = hidden
	if len(a.Writable) > protocol.TeamWritableMax {
		return protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.Tf("A team changes at most %d folders besides its own.", protocol.TeamWritableMax))
	}
	var writable []string
	for _, path := range a.Writable {
		clean, err := writableFolder(path, own)
		if err != nil {
			return err
		}
		if !slices.Contains(writable, clean) {
			writable = append(writable, clean)
		}
	}
	a.Writable = writable
	return nil
}

// writableFolder checks one folder a team may change besides its own: an
// existing folder given by its full path, neither the home folder nor one
// above it, and not inside the teams' or dreams' folders unless inside the
// team's own.
func writableFolder(path, own string) (string, error) {
	full := paths.Expand(strings.TrimSpace(path))
	if !filepath.IsAbs(full) {
		return "", protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.Tf("%s is not a full path.", path))
	}
	full = filepath.Clean(full)
	if paths.Within(full, paths.Home()) {
		return "", protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.Tf("Letting a team change %s would let it change everything.", path))
	}
	inside := paths.Within(paths.TeamsHome(), full) || paths.Within(paths.DreamsHome(), full)
	if inside && (own == "" || !paths.Within(own, full)) {
		return "", protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.Tf("%s belongs to another team.", path))
	}
	info, err := os.Stat(full)
	if err != nil || !info.IsDir() {
		return "", protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.Tf("%s is not a folder on this computer.", path))
	}
	return paths.Short(full), nil
}

// teamFolder turns a team's folder into a full path, defaulting to its own
// folder under the Teams home. A path that exists must be a folder.
func teamFolder(folder, name string) (string, error) {
	folder = paths.Expand(strings.TrimSpace(folder))
	if folder == "" {
		folder = filepath.Join(paths.TeamsHome(), slugName(name))
	}
	if !filepath.IsAbs(folder) {
		return "", protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("The team's folder must be a full path."))
	}
	folder = filepath.Clean(folder)
	if info, err := os.Stat(folder); err == nil && !info.IsDir() {
		return "", protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.Tf("%s is not a folder.", paths.Short(folder)))
	}
	return folder, nil
}

// makeFolder makes sure a team's folder exists.
func makeFolder(folder string) error {
	if err := paths.EnsureDir(folder); err != nil {
		return protocol.Errorf(protocol.CodeUnavailable, "%s", i18n.Tf("Hyakki cannot make the folder %s: %v.", paths.Short(folder), err))
	}
	return nil
}

var slugPart = regexp.MustCompile(`[^\p{L}\p{N}]+`)

// slugName makes a folder and file name from a team's name, keeping letters
// of any script so a name in another language keeps its words.
func slugName(name string) string {
	slug := strings.Trim(slugPart.ReplaceAllString(strings.ToLower(strings.TrimSpace(name)), "-"), "-")
	if slug == "" {
		return "team"
	}
	return slug
}

// routineMember resolves the member a routine draft names, by its old id,
// its id or its name; anything else hands the routine to the lead.
func routineMember(team protocol.Team, ref string, renamed map[string]string) string {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return ""
	}
	if id, ok := renamed[ref]; ok {
		return id
	}
	for _, m := range team.Members {
		if m.ID == ref || strings.EqualFold(m.Name, ref) {
			return m.ID
		}
	}
	return ""
}

// mergePatch applies a JSON merge patch (RFC 7396) to decoded JSON.
func mergePatch(target, patch any) any {
	pm, ok := patch.(map[string]any)
	if !ok {
		return patch
	}
	tm, ok := target.(map[string]any)
	if !ok {
		tm = map[string]any{}
	}
	for k, v := range pm {
		if v == nil {
			delete(tm, k)
			continue
		}
		tm[k] = mergePatch(tm[k], v)
	}
	return tm
}

// nobodyWatching reports a bound service with no Rashin window open; before
// Bind nobody is known to be away.
func (s *Service) nobodyWatching() bool {
	s.mu.Lock()
	sessions := s.sessions
	s.mu.Unlock()
	return sessions != nil && !sessions.Watched()
}
