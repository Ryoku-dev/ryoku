package teams

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode/utf8"

	i18n "ryoku-i18n"

	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/protocol"
)

// Team limits.
const (
	maxMembers   = 6
	maxNameRunes = 80
	maxTaskTurns = 200
)

// defaultBudget is the spending limit of a team that names none.
var defaultBudget = protocol.TeamBudget{MonthTokens: 2_000_000, WarnPercent: 80, TaskTurns: 20}

// teamEdit is the part of a team teams.update may change.
type teamEdit struct {
	Name      string              `json:"name"`
	Purpose   string              `json:"purpose"`
	Folder    string              `json:"folder"`
	Members   []protocol.Member   `json:"members"`
	Tools     protocol.TeamTools  `json:"tools"`
	Approvals string              `json:"approvals"`
	Budget    protocol.TeamBudget `json:"budget"`
	Notify    string              `json:"notify"`
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
		teams[i].Stats = protocol.TeamStats{MonthTokens: s.monthTokens(teams[i].ID, now)}
		index[teams[i].ID] = &teams[i].Stats
	}
	for _, t := range s.state.tasks(nil) {
		stats := index[t.Team]
		if stats == nil {
			continue
		}
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

// createTeam makes a team and the routines it starts with.
func (s *Service) createTeam(ctx context.Context, p protocol.TeamCreateParams) (protocol.Team, error) {
	now := s.now()
	team := protocol.Team{
		ID: newID("team"), Name: p.Name, Purpose: p.Purpose, Template: p.Template,
		Folder: p.Folder, Members: p.Members, Tools: p.Tools, Approvals: p.Approvals,
		Budget: p.Budget, Notify: p.Notify, Created: now, Updated: now,
	}
	detail := func(t protocol.Team) string {
		if len(t.Members) == 1 {
			return i18n.Tf("Made with one member, working in %s.", paths.Short(t.Folder))
		}
		return i18n.Tf("Made with %d members, working in %s.", len(t.Members), paths.Short(t.Folder))
	}
	return s.addTeam(ctx, team, p.Routines, "team.create", detail)
}

// addTeam validates and stores a new team, then creates its routines through
// the routine path. A routine that cannot be made takes the team back out,
// so a create either happens whole or not at all.
func (s *Service) addTeam(ctx context.Context, team protocol.Team, drafts []protocol.RoutineDraft, action string, detail func(protocol.Team) string) (protocol.Team, error) {
	if team.Budget == (protocol.TeamBudget{}) {
		team.Budget = defaultBudget
		if template, ok := templateByID(team.Template); ok {
			team.Budget = template.Budget
		}
	}
	renamed, err := normaliseTeam(&team, true)
	if err != nil {
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
// change; its id, creation time and pause stay as they are.
func (s *Service) updateTeam(ctx context.Context, p protocol.TeamUpdateParams) (protocol.Team, error) {
	current, ok := s.state.team(p.ID)
	if !ok {
		return protocol.Team{}, protocol.Errorf(protocol.CodeNotFound, "%s", i18n.Tf("No team %s.", p.ID))
	}
	base, err := json.Marshal(teamEdit{
		Name: current.Name, Purpose: current.Purpose, Folder: current.Folder, Members: current.Members,
		Tools: current.Tools, Approvals: current.Approvals, Budget: current.Budget, Notify: current.Notify,
	})
	if err != nil {
		return protocol.Team{}, err
	}
	var doc, patch any
	if err := json.Unmarshal(base, &doc); err != nil {
		return protocol.Team{}, err
	}
	if err := json.Unmarshal(p.Patch, &patch); err != nil {
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
	team.Tools, team.Approvals, team.Budget, team.Notify = next.Tools, next.Approvals, next.Budget, next.Notify
	if _, err := normaliseTeam(&team, false); err != nil {
		return protocol.Team{}, err
	}
	team.Updated = s.now()
	if err := s.state.putTeam(team); err != nil {
		return protocol.Team{}, err
	}
	s.record(protocol.Activity{
		Actor: "you", Action: "team.update", Target: team.Name, OK: true,
		Detail: i18n.T("Changed the team's settings."), Team: team.ID, Initiator: protocol.InitiatorYou,
	})
	s.emitTeam(team.ID)
	updated, _ := s.teamWithStats(team.ID)
	return updated, nil
}

// deleteTeam stops a team's work and removes it with its tasks, routines,
// approvals and grants. Its folder stays: it holds what the team made.
func (s *Service) deleteTeam(_ context.Context, id string) error {
	team, ok := s.state.team(id)
	if !ok {
		return protocol.Errorf(protocol.CodeNotFound, "%s", i18n.Tf("No team %s.", id))
	}
	s.stopTeamTasks(id, i18n.T("The team was deleted."))
	s.dropRoutines(id)
	s.dropGrants(id)
	tasks := s.state.tasks(func(t protocol.TeamTask) bool { return t.Team == id })
	if err := s.state.removeTeam(id); err != nil {
		return err
	}
	for _, t := range tasks {
		s.broadcast(protocol.EvtTeamTaskRemoved, protocol.TeamRemoved{ID: t.ID})
	}
	s.broadcast(protocol.EvtTeamRemoved, protocol.TeamRemoved{ID: id})
	s.record(protocol.Activity{
		Actor: "you", Action: "team.delete", Target: team.Name, OK: true,
		Detail: i18n.Tf("Deleted the team with its tasks and routines; its folder %s stays.", paths.Short(team.Folder)),
		Team:   id, Initiator: protocol.InitiatorYou,
	})
	return nil
}

// pauseTeam pauses a team, stopping its working tasks, or lets it work again.
// A team its spending limit paused starts again only with a higher limit.
func (s *Service) pauseTeam(_ context.Context, p protocol.TeamPauseParams) (protocol.Team, error) {
	team, ok := s.state.team(p.ID)
	if !ok {
		return protocol.Team{}, protocol.Errorf(protocol.CodeNotFound, "%s", i18n.Tf("No team %s.", p.ID))
	}
	if team.Paused == p.Paused {
		current, _ := s.teamWithStats(team.ID)
		return current, nil
	}
	now := s.now()
	if p.Paused {
		team.Paused, team.PauseReason, team.Updated = true, protocol.PausedByYou, now
		if err := s.state.putTeam(team); err != nil {
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
	if byBudget {
		used := s.monthTokens(team.ID, now)
		if p.Raise <= used {
			return protocol.Team{}, protocol.Errorf(protocol.CodeBadRequest, "%s",
				i18n.Tf("To start %s again, raise its spending limit above the %d tokens it used this month.", team.Name, used))
		}
		team.Budget.MonthTokens = p.Raise
		detail = i18n.Tf("Raised the spending limit to %d tokens a month and let the team work again.", p.Raise)
	}
	team.Paused, team.PauseReason, team.Updated = false, "", now
	if err := s.state.putTeam(team); err != nil {
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

// normaliseTeam trims and checks what the user gave a team and fills the
// defaults. With fresh every member gets a new id; renamed maps the ids they
// came with to the new ones, so routines that named a member keep it.
func normaliseTeam(t *protocol.Team, fresh bool) (map[string]string, error) {
	t.Name = strings.TrimSpace(t.Name)
	if t.Name == "" {
		return nil, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("A team needs a name."))
	}
	if utf8.RuneCountInString(t.Name) > maxNameRunes {
		return nil, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.Tf("A team's name has at most %d characters.", maxNameRunes))
	}
	t.Purpose = strings.TrimSpace(t.Purpose)
	if _, ok := templateByID(t.Template); !ok {
		t.Template = ""
	}
	members, renamed, err := normaliseMembers(t.Members, fresh)
	if err != nil {
		return nil, err
	}
	t.Members = members
	if t.Approvals == "" {
		t.Approvals = protocol.ApprovalsAsk
	}
	if !protocol.KnownApprovals(t.Approvals) {
		return nil, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.Tf("%s is not a permission setting.", t.Approvals))
	}
	switch t.Notify {
	case "":
		t.Notify = protocol.NotifyAll
	case protocol.NotifyAll, protocol.NotifyNeeds, protocol.NotifyNone:
	default:
		return nil, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.Tf("%s is not a notification setting.", t.Notify))
	}
	if err := normaliseBudget(&t.Budget); err != nil {
		return nil, err
	}
	folder, err := resolveFolder(t.Folder, t.Name)
	if err != nil {
		return nil, err
	}
	t.Folder = folder
	return renamed, nil
}

// normaliseMembers checks a team's members: 1 to maxMembers, each named,
// names unique, exactly one lead (the first marked one, else the first).
func normaliseMembers(in []protocol.Member, fresh bool) ([]protocol.Member, map[string]string, error) {
	if len(in) == 0 {
		return nil, nil, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("A team needs at least one member."))
	}
	if len(in) > maxMembers {
		return nil, nil, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.Tf("A team has at most %d members.", maxMembers))
	}
	out := make([]protocol.Member, 0, len(in))
	renamed := make(map[string]string)
	names := make(map[string]bool, len(in))
	ids := make(map[string]bool, len(in))
	lead := -1
	for i, m := range in {
		m.Name = strings.TrimSpace(m.Name)
		m.Title = strings.TrimSpace(m.Title)
		m.Harness = strings.TrimSpace(m.Harness)
		m.Model = strings.TrimSpace(m.Model)
		m.Instructions = strings.TrimSpace(m.Instructions)
		if m.Name == "" {
			return nil, nil, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("Every member needs a name."))
		}
		if utf8.RuneCountInString(m.Name) > maxNameRunes {
			return nil, nil, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.Tf("A member's name has at most %d characters.", maxNameRunes))
		}
		key := strings.ToLower(m.Name)
		if names[key] {
			return nil, nil, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.Tf("Two members are called %s.", m.Name))
		}
		names[key] = true
		old := strings.TrimSpace(m.ID)
		if fresh || old == "" || ids[old] {
			m.ID = newID("member")
			if old != "" {
				renamed[old] = m.ID
			}
		} else {
			m.ID = old
		}
		ids[m.ID] = true
		if m.Lead && lead < 0 {
			lead = i
		}
		m.Lead = false
		out = append(out, m)
	}
	out[max(lead, 0)].Lead = true
	return out, renamed, nil
}

// normaliseBudget fills the warning and turn defaults and checks the ranges;
// a monthly limit of 0 is no limit.
func normaliseBudget(b *protocol.TeamBudget) error {
	if b.MonthTokens < 0 {
		return protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("The spending limit cannot be below zero."))
	}
	if b.WarnPercent == 0 {
		b.WarnPercent = defaultBudget.WarnPercent
	}
	if b.WarnPercent < 1 || b.WarnPercent > 99 {
		return protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("The spending warning must be between 1 and 99 percent."))
	}
	if b.TaskTurns == 0 {
		b.TaskTurns = defaultBudget.TaskTurns
	}
	if b.TaskTurns < 1 || b.TaskTurns > maxTaskTurns {
		return protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.Tf("Turns per task must be between 1 and %d.", maxTaskTurns))
	}
	return nil
}

// resolveFolder turns a team's folder into a full path, defaulting to its
// own folder under the Teams home, and makes sure it exists.
func resolveFolder(folder, name string) (string, error) {
	folder = expandHome(strings.TrimSpace(folder))
	if folder == "" {
		folder = filepath.Join(paths.TeamsHome(), slugName(name))
	}
	if !filepath.IsAbs(folder) {
		return "", protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("The team's folder must be a full path."))
	}
	folder = filepath.Clean(folder)
	if err := paths.EnsureDir(folder); err != nil {
		return "", protocol.Errorf(protocol.CodeUnavailable, "%s", i18n.Tf("Rashin cannot make the folder %s: %v.", paths.Short(folder), err))
	}
	info, err := os.Stat(folder)
	if err != nil || !info.IsDir() {
		return "", protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.Tf("%s is not a folder.", paths.Short(folder)))
	}
	return folder, nil
}

// expandHome reads a leading ~ as the home directory.
func expandHome(path string) string {
	if path == "~" {
		return paths.Home()
	}
	if rest, ok := strings.CutPrefix(path, "~/"); ok {
		return filepath.Join(paths.Home(), rest)
	}
	return path
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
