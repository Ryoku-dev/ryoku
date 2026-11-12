package inbox

import (
	"gioui.org/layout"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/app/pages/teamsui"
	"ryoku-rashin/internal/protocol"
)

func (p *Page) List(gtx layout.Context, e *env.Env) look.ListDimensions {
	snapshot := e.Snapshot()
	needs := teamsui.Needs(snapshot)
	needed := make(map[string]bool, len(needs))
	rows := make([]look.SideRow, 0, len(needs))
	for _, need := range needs {
		key := teamsui.NeedKey(need)
		needed[key] = true
		row := look.SideRow{ID: key, Tone: palette.Attention}
		approval := need.Approval
		switch {
		// a proposed team has no team yet, so it reads by where it came from
		case approval != nil && approval.Kind == protocol.ApprovalTeam:
			row.Title, row.Facts = approval.Title, teamsui.ApprovalWord(approval.Kind)
			if info, ok := snapshot.Sessions[approval.Session]; ok {
				row.Facts = i18n.Tf("%s from %s", row.Facts, info.Title)
			}
		case approval != nil:
			row.Title = approval.Title
			row.Facts = i18n.Tf("%s on %s", teamsui.ApprovalWord(approval.Kind), teamsui.TeamName(snapshot.Teams, approval.Team))
		default:
			word, tone, _ := teamsui.TaskStanding(*need.Task)
			row.Title, row.Tone = need.Task.Title, tone
			row.Facts = i18n.Tf("%s on %s", word, teamsui.TeamName(snapshot.Teams, need.Task.Team))
		}
		rows = append(rows, row)
	}
	unread := teamsui.UnreadReports(snapshot)
	reports := make([]look.SideRow, 0, len(unread))
	for _, task := range unread {
		reports = append(reports, look.SideRow{ID: task.ID, Title: reportTitle(task), Facts: teamsui.TaskFacts(snapshot, task), Strong: true})
	}
	busy := teamsui.Working(snapshot)
	working := make([]look.SideRow, 0, len(busy))
	for _, task := range busy {
		working = append(working, look.SideRow{ID: task.ID, Title: task.Title, Facts: teamsui.TaskFacts(snapshot, task), Working: true})
	}
	groups := []look.SideGroup{
		{Title: i18n.T("Needs you"), Count: len(rows), Rows: rows},
		{Title: i18n.T("Reports"), Count: len(reports), Rows: reports},
		{Title: i18n.T("Working"), Count: len(working), Rows: working},
	}
	p.list.Empty = i18n.T("What your teams need from you, the reports they send back and what they work on show here.")
	dimensions := p.list.Layout(gtx, e.Theme, e.Overlays, groups, "")
	for _, event := range p.list.Events() {
		if event.Kind != look.SideOpen {
			continue
		}
		// a need is answered on its card here, the rest open their task
		if needed[event.ID] {
			p.board.reveal = event.ID
		} else {
			teamsui.OpenTask(e, event.ID)
		}
	}
	return dimensions
}

func reportTitle(task protocol.TeamTask) string {
	if task.Report != nil && task.Report.Title != "" {
		return task.Report.Title
	}
	return task.Title
}
