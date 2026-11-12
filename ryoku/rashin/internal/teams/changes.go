package teams

import (
	"maps"
	"slices"

	i18n "ryoku-i18n"

	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/protocol"
)

// teamChanges says, one sentence each, what an update changed in a team,
// for the ledger. None means nothing changed.
func (s *Service) teamChanges(old, next protocol.Team) []string {
	var out []string
	if old.Name != next.Name {
		out = append(out, i18n.Tf("Renamed it from %s to %s.", old.Name, next.Name))
	}
	if old.Purpose != next.Purpose {
		out = append(out, i18n.T("Changed its purpose."))
	}
	if old.Folder != next.Folder {
		out = append(out, i18n.Tf("Moved its folder to %s.", paths.Short(next.Folder)))
	}
	out = append(out, s.memberChanges(old.Members, next.Members)...)
	if old.Tools.Screen != next.Tools.Screen {
		if next.Tools.Screen {
			out = append(out, i18n.T("Gave it a screen of its own."))
		} else {
			out = append(out, i18n.T("Took its own screen away."))
		}
	}
	if old.Tools.Mail != next.Tools.Mail {
		if next.Tools.Mail {
			out = append(out, i18n.T("Let it send mail."))
		} else {
			out = append(out, i18n.T("Stopped it sending mail."))
		}
	}
	if old.Approvals != next.Approvals {
		switch next.Approvals {
		case protocol.ApprovalsReadOnly:
			out = append(out, i18n.T("Made it read only."))
		case protocol.ApprovalsAuto:
			out = append(out, i18n.T("Let it act without asking."))
		default:
			out = append(out, i18n.T("Made it ask before changes."))
		}
	}
	out = append(out, budgetChanges(old.Budget, next.Budget)...)
	if old.Notify != next.Notify {
		switch next.Notify {
		case protocol.NotifyNeeds:
			out = append(out, i18n.T("Set it to notify you only when it needs you."))
		case protocol.NotifyNone:
			out = append(out, i18n.T("Turned its notifications off."))
		default:
			out = append(out, i18n.T("Set it to notify you of reports and of what needs you."))
		}
	}
	if old.Running != next.Running {
		if next.Running == 0 {
			out = append(out, i18n.T("Set its tasks at once back to the teams setting."))
		} else {
			out = append(out, i18n.Tf("Let it work on %d tasks at once.", next.Running))
		}
	}
	for _, path := range added(old.Access.Writable, next.Access.Writable) {
		out = append(out, i18n.Tf("Let it change %s.", path))
	}
	for _, path := range added(next.Access.Writable, old.Access.Writable) {
		out = append(out, i18n.Tf("Stopped it changing %s.", path))
	}
	for _, path := range added(old.Access.Hidden, next.Access.Hidden) {
		out = append(out, i18n.Tf("Hid %s from it.", path))
	}
	for _, path := range added(next.Access.Hidden, old.Access.Hidden) {
		out = append(out, i18n.Tf("Stopped hiding %s from it.", path))
	}
	if old.TeamMemory() != next.TeamMemory() {
		out = append(out, i18n.T("Changed what it remembers."))
	}
	for _, set := range added(old.SkillsOff, next.SkillsOff) {
		out = append(out, i18n.Tf("Took the skill set %s from it.", set))
	}
	for _, set := range added(next.SkillsOff, old.SkillsOff) {
		out = append(out, i18n.Tf("Gave it the skill set %s again.", set))
	}
	return out
}

// memberChanges says what changed in a team's members, by member id.
func (s *Service) memberChanges(old, next []protocol.Member) []string {
	var out []string
	before := make(map[string]protocol.Member, len(old))
	for _, m := range old {
		before[m.ID] = m
	}
	stays := make(map[string]bool, len(next))
	for _, m := range next {
		stays[m.ID] = true
		was, ok := before[m.ID]
		if !ok {
			out = append(out, i18n.Tf("Added %s.", m.Name))
			continue
		}
		if was.Name != m.Name {
			out = append(out, i18n.Tf("Renamed %s to %s.", was.Name, m.Name))
		}
		if was.Title != m.Title {
			out = append(out, i18n.Tf("Changed %s's title.", m.Name))
		}
		if was.Instructions != m.Instructions {
			out = append(out, i18n.Tf("Changed %s's instructions.", m.Name))
		}
		if m.Lead && !was.Lead {
			out = append(out, i18n.Tf("Made %s the lead.", m.Name))
		}
		if was.Harness != m.Harness || was.Model != m.Model || !maps.Equal(was.Options, m.Options) {
			out = append(out, i18n.Tf("Changed %s's engine from %s to %s.", m.Name, s.memberEngineWords(was), s.memberEngineWords(m)))
		}
	}
	for _, m := range old {
		if !stays[m.ID] {
			out = append(out, i18n.Tf("Removed %s.", m.Name))
		}
	}
	return out
}

// budgetChanges says what changed in a team's budget.
func budgetChanges(old, next protocol.TeamBudget) []string {
	var out []string
	switch {
	case old.MonthTokens == next.MonthTokens:
	case next.MonthTokens == 0:
		out = append(out, i18n.T("Removed its spending limit."))
	case old.MonthTokens == 0:
		out = append(out, i18n.Tf("Set its spending limit to %d tokens a month.", next.MonthTokens))
	case next.MonthTokens > old.MonthTokens:
		out = append(out, i18n.Tf("Raised the spending limit to %d tokens a month.", next.MonthTokens))
	default:
		out = append(out, i18n.Tf("Lowered the spending limit to %d tokens a month.", next.MonthTokens))
	}
	if old.WarnPercent != next.WarnPercent {
		out = append(out, i18n.Tf("Set the spending warning to %d%%.", next.WarnPercent))
	}
	if old.TaskTurns != next.TaskTurns {
		out = append(out, i18n.Tf("Set turns per task to %d.", next.TaskTurns))
	}
	if old.TaskMinutes != next.TaskMinutes {
		if next.TaskMinutes == 0 {
			out = append(out, i18n.T("Removed the time limit per task."))
		} else {
			out = append(out, i18n.Tf("Set minutes per task to %d.", next.TaskMinutes))
		}
	}
	return out
}

// added lists what next has that old lacks, in next's order.
func added(old, next []string) []string {
	var out []string
	for _, item := range next {
		if !slices.Contains(old, item) {
			out = append(out, item)
		}
	}
	return out
}
