package config

import (
	"path/filepath"
	"slices"
	"strings"

	i18n "ryoku-i18n"
	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/protocol"
)

// defaultTeams are the teams settings of a machine that never changed one.
func defaultTeams() protocol.TeamsSettings {
	return protocol.TeamsSettings{
		MaxRunning:    protocol.DefaultTeamsMaxRunning,
		PerTeam:       protocol.DefaultTeamsPerTeam,
		RoutinesWake:  true,
		ApprovalHours: protocol.DefaultApprovalHours,
		Defaults: protocol.TeamDefaults{
			Approvals: protocol.ApprovalsAsk,
			Budget:    protocol.TeamBudget{MonthTokens: 2_000_000, WarnPercent: 80, TaskTurns: 20, TaskMinutes: 60},
			Notify:    protocol.NotifyAll,
			Memory:    protocol.DefaultTeamMemory,
		},
	}
}

// validateTeams normalises the teams settings read from a file: numbers are
// clamped, unknown values fall back to d, and what cannot be read is dropped.
func validateTeams(t *protocol.TeamsSettings, d protocol.TeamsSettings) {
	t.MaxRunning = clamp(t.MaxRunning, 1, protocol.TeamsMaxRunningMax, d.MaxRunning)
	t.PerTeam = min(clamp(t.PerTeam, 1, protocol.TeamsPerTeamMax, d.PerTeam), t.MaxRunning)
	t.ApprovalHours = clamp(t.ApprovalHours, 1, protocol.TeamsApprovalHoursMax, d.ApprovalHours)
	t.KeepDays = snapKeepDays(t.KeepDays)
	if t.QuietHours != nil && checkQuietHours(*t.QuietHours) != nil {
		t.QuietHours = nil
	}
	if t.Hidden != nil {
		hidden := make([]string, 0, len(t.Hidden))
		for _, path := range t.Hidden {
			clean, err := cleanHiddenPath(path)
			if err == nil && !slices.Contains(hidden, clean) && len(hidden) < protocol.TeamsHiddenMax {
				hidden = append(hidden, clean)
			}
		}
		t.Hidden = hidden
	}
	defaults := &t.Defaults
	defaults.Harness = strings.TrimSpace(defaults.Harness)
	defaults.Model = strings.TrimSpace(defaults.Model)
	if !protocol.ValidEngineModel(defaults.Model) {
		defaults.Model = ""
	}
	if !protocol.ValidEngineOptions(defaults.Options) {
		defaults.Options = nil
	}
	if !protocol.KnownApprovals(defaults.Approvals) {
		defaults.Approvals = d.Defaults.Approvals
	}
	if !knownNotify(defaults.Notify) {
		defaults.Notify = d.Defaults.Notify
	}
	budget, fallback := &defaults.Budget, d.Defaults.Budget
	if budget.MonthTokens < 0 {
		budget.MonthTokens = fallback.MonthTokens
	}
	budget.WarnPercent = clamp(budget.WarnPercent, 1, 99, fallback.WarnPercent)
	budget.TaskTurns = clamp(budget.TaskTurns, 1, protocol.TeamTaskTurnsMax, fallback.TaskTurns)
	budget.TaskMinutes = min(max(budget.TaskMinutes, 0), protocol.TeamTaskMinutesMax)
}

// checkTeams says what is wrong with the teams settings a patch would leave.
func checkTeams(t protocol.TeamsSettings) error {
	if t.MaxRunning < 1 || t.MaxRunning > protocol.TeamsMaxRunningMax {
		return badRequest(i18n.Tf("Tasks at once must be between 1 and %d.", protocol.TeamsMaxRunningMax))
	}
	// a per-team value above a lowered maxRunning is clamped, not refused, so lowering it never fails
	if t.PerTeam < 1 || t.PerTeam > protocol.TeamsPerTeamMax {
		return badRequest(i18n.Tf("Tasks at once in one team must be between 1 and %d.", protocol.TeamsPerTeamMax))
	}
	if t.ApprovalHours < 1 || t.ApprovalHours > protocol.TeamsApprovalHoursMax {
		return badRequest(i18n.Tf("An unanswered question can wait between 1 and %d hours.", protocol.TeamsApprovalHoursMax))
	}
	if !slices.Contains(protocol.TeamsKeepDays, t.KeepDays) {
		return badRequest(i18n.T("Finished tasks are kept for 30, 90 or 365 days, or forever."))
	}
	if t.QuietHours != nil {
		if err := checkQuietHours(*t.QuietHours); err != nil {
			return err
		}
	}
	if _, err := CleanHidden(t.Hidden); err != nil {
		return err
	}
	defaults := t.Defaults
	if !protocol.ValidEngineModel(strings.TrimSpace(defaults.Model)) {
		return badRequest(i18n.Tf("A model id has at most %d characters and no spaces.", protocol.EngineModelMax))
	}
	if !protocol.ValidEngineOptions(defaults.Options) {
		return badRequest(i18n.Tf("A harness takes at most %d options, each with an id and a value.", protocol.EngineOptionsMax))
	}
	if !protocol.KnownApprovals(defaults.Approvals) {
		return badRequest(i18n.Tf("%s is not a permission setting.", defaults.Approvals))
	}
	if !knownNotify(defaults.Notify) {
		return badRequest(i18n.Tf("%s is not a notification setting.", defaults.Notify))
	}
	return CheckBudget(defaults.Budget)
}

// CheckBudget says what is wrong with a team budget. A WarnPercent or
// TaskTurns of 0 passes: it stands for the default the caller fills in.
func CheckBudget(b protocol.TeamBudget) error {
	if b.MonthTokens < 0 {
		return badRequest(i18n.T("The spending limit cannot be below zero."))
	}
	if b.WarnPercent < 0 || b.WarnPercent > 99 {
		return badRequest(i18n.T("The spending warning must be between 1 and 99 percent."))
	}
	if b.TaskTurns < 0 || b.TaskTurns > protocol.TeamTaskTurnsMax {
		return badRequest(i18n.Tf("Turns per task must be between 1 and %d.", protocol.TeamTaskTurnsMax))
	}
	if b.TaskMinutes < 0 || b.TaskMinutes > protocol.TeamTaskMinutesMax {
		return badRequest(i18n.Tf("Minutes per task must be between 0 and %d.", protocol.TeamTaskMinutesMax))
	}
	return nil
}

// CleanHidden checks paths to hide from team agents and writes each the way
// it is kept: "~/" first for one under the home folder, else absolute. A
// leading ~ is read as the home folder; "/", the home folder and the folders
// above it are refused, since hiding them hides everything. Nil stays nil.
func CleanHidden(list []string) ([]string, error) {
	if list == nil {
		return nil, nil
	}
	if len(list) > protocol.TeamsHiddenMax {
		return nil, badRequest(i18n.Tf("Teams hide at most %d paths.", protocol.TeamsHiddenMax))
	}
	out := make([]string, 0, len(list))
	for _, path := range list {
		clean, err := cleanHiddenPath(path)
		if err != nil {
			return nil, err
		}
		if !slices.Contains(out, clean) {
			out = append(out, clean)
		}
	}
	return out, nil
}

func cleanHiddenPath(path string) (string, error) {
	path = strings.TrimSpace(path)
	home := paths.Home()
	full := paths.Expand(path)
	if !filepath.IsAbs(full) {
		return "", badRequest(i18n.Tf("%s is not a full path.", path))
	}
	full = filepath.Clean(full)
	if paths.Within(full, home) {
		return "", badRequest(i18n.Tf("Hiding %s would hide everything a team needs.", path))
	}
	return paths.Short(full), nil
}

func checkQuietHours(q protocol.QuietHours) error {
	from, okFrom := protocol.ParseClock(q.From)
	to, okTo := protocol.ParseClock(q.To)
	if !okFrom || !okTo {
		return badRequest(i18n.T("Quiet hours are times like 22:00."))
	}
	if from == to {
		return badRequest(i18n.T("Quiet hours need a start and an end that differ."))
	}
	return nil
}

// a hand-edited value keeps finished tasks at least as long as it asked, up to the longest choice
func snapKeepDays(days int) int {
	if days <= 0 {
		return 0
	}
	for _, allowed := range protocol.TeamsKeepDays {
		if allowed >= days {
			return allowed
		}
	}
	return protocol.TeamsKeepDays[len(protocol.TeamsKeepDays)-1]
}

func knownNotify(value string) bool {
	return value == protocol.NotifyAll || value == protocol.NotifyNeeds || value == protocol.NotifyNone
}

func badRequest(message string) error {
	return protocol.Errorf(protocol.CodeBadRequest, "%s", message)
}
