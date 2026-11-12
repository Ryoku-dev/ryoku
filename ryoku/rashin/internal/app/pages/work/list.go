package work

import (
	"gioui.org/layout"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/app/pages/teamsui"
)

func (p *Page) List(gtx layout.Context, e *env.Env) look.ListDimensions {
	th := e.Theme
	snapshot := e.Snapshot()
	tasks := teamsui.GroupTasks(snapshot, "", false)
	groups := make([]look.SideGroup, 0, teamsui.GroupCount)
	for at := teamsui.GroupNeeds; at < teamsui.GroupCount; at++ {
		list := tasks[at]
		if at == teamsui.GroupDone && len(list) > sideDone {
			list = list[:sideDone]
		}
		rows := make([]look.SideRow, 0, len(list))
		for _, task := range list {
			row := look.SideRow{
				ID: task.ID, Title: task.Title, Facts: teamsui.TaskFacts(snapshot, task),
				Working: at == teamsui.GroupWorking, Strong: task.Report != nil && !task.Read,
			}
			// only a task that wants the user or stopped short gets a notch, the rest say when
			switch at {
			case teamsui.GroupNeeds, teamsui.GroupHalted:
				_, row.Tone, _ = teamsui.TaskStanding(task)
			}
			if at != teamsui.GroupWorking && at != teamsui.GroupNeeds {
				when := look.Ago(gtx.Now, teamsui.Ended(task))
				if at == teamsui.GroupQueued {
					when = look.Ago(gtx.Now, task.Created)
				}
				row.Trail = kit.Text(th.Theme, th.T.Caption, th.R.Fg3, when)
			}
			rows = append(rows, row)
		}
		groups = append(groups, look.SideGroup{Title: at.Label(), Count: len(tasks[at]), Rows: rows})
	}
	p.list.Empty = i18n.T("Tasks you give your teams are kept here, sorted by what they need from you.")
	dimensions := p.list.Layout(gtx, th, e.Overlays, groups, p.selected)
	for _, event := range p.list.Events() {
		if event.Kind == look.SideOpen {
			teamsui.OpenTask(e, event.ID)
		}
	}
	return dimensions
}
