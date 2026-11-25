package shell

import (
	"strings"

	"ryoku-i18n"

	"ryoku-rashin/internal/app/desktop"
	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/pages/teamsui"
	"ryoku-rashin/internal/app/state"
	"ryoku-rashin/internal/protocol"
)

// keys of Teams notices, so a press opens the Inbox or the task, not an agent session
const (
	approvalNotice = "approval:"
	reportNotice   = "report:"
)

// the pending approvals and unread reports, what the next change is told against
func teamsStanding(snapshot state.Snapshot) (asked, reported map[string]bool) {
	asked, reported = make(map[string]bool), make(map[string]bool)
	for _, approval := range snapshot.Approvals {
		if approval.Status == protocol.ApprovalPending {
			asked[approval.ID] = true
		}
	}
	for _, task := range snapshot.TeamTasks {
		if task.Report != nil && !task.Read {
			reported[task.ID] = true
		}
	}
	return asked, reported
}

// a new approval or report, as far as its team's notify level lets it reach the desktop
func teamsNotices(snapshot state.Snapshot, asked, reported map[string]bool) []desktop.Notice {
	var notices []desktop.Notice
	for _, approval := range snapshot.Approvals {
		team, ok := teamsui.TeamByID(snapshot.Teams, approval.Team)
		if approval.Status != protocol.ApprovalPending || asked[approval.ID] || !ok || team.Notify == protocol.NotifyNone {
			continue
		}
		detail, _, _ := strings.Cut(approval.Detail, "\n")
		notices = append(notices, desktop.Notice{
			Key: approvalNotice + approval.ID, Title: i18n.Tf("%s needs you: %s", team.Name, approval.Title), Body: detail,
		})
	}
	for _, task := range teamsui.UnreadReports(snapshot) {
		team, ok := teamsui.TeamByID(snapshot.Teams, task.Team)
		if reported[task.ID] || !ok || team.Notify == protocol.NotifyNeeds || team.Notify == protocol.NotifyNone {
			continue
		}
		notices = append(notices, desktop.Notice{
			Key: reportNotice + task.ID, Title: i18n.Tf("%s sent a report: %s", team.Name, task.Report.Title), Body: teamsui.Excerpt(task.Report.Body, 2),
		})
	}
	return notices
}

// a press on a notice opens what it told about
func (s *uiShell) openNotice(key string) {
	if strings.HasPrefix(key, approvalNotice) {
		s.navigateTeams(env.TeamsPlace{Tab: protocol.TabInbox})
		return
	}
	if task, ok := strings.CutPrefix(key, reportNotice); ok {
		s.navigateTeams(env.TeamsPlace{Tab: protocol.TabWork, Task: task})
		return
	}
	s.navigate(protocol.TabAgent, key)
}
