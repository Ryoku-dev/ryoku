package routines

import (
	"time"

	"gioui.org/layout"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/app/pages/teamsui"
	"ryoku-rashin/internal/app/state"
	"ryoku-rashin/internal/protocol"
)

// how many past runs the aside keeps in view
const historyLimit = 6

// the editor's aside: when the schedule runs next, and how the last runs went
type asideState struct {
	runs [historyLimit]kit.Click
}

func (a *asideState) layout(gtx layout.Context, e *env.Env, ed *editor, snapshot state.Snapshot, routine protocol.Routine) layout.Dimensions {
	th := e.Theme
	schedule := ed.when.reading
	var rows []layout.Widget
	if schedule.problem != "" {
		rows = append(rows, kit.AsideNote(th.Theme, schedule.problem))
	} else {
		for _, at := range schedule.next {
			rows = append(rows, nextRow(th, gtx.Now, at))
		}
	}
	if note := quietNote(ed, snapshot, routine); note != "" {
		rows = append(rows, kit.AsideNote(th.Theme, note))
	}
	rows = append(rows, kit.Text(th.Theme, th.T.Row, th.R.Fg, i18n.T("Last runs")))
	if ed.creating {
		rows = append(rows, kit.AsideNote(th.Theme, i18n.T("Each run becomes a task for the team, and its report lands in your Inbox.")))
		return kit.Aside(gtx, th.Theme, i18n.T("When it runs"), rows...)
	}
	if teamsui.QuietStreak(routine) != "" {
		rows = append(rows, kit.AsideNote(th.Theme, i18n.Tf("Quiet for %d runs. Each found nothing new, so none of them reached your Inbox.", routine.QuietStreak)))
	}
	tasks := routineTasks(snapshot, routine.ID)
	if len(tasks) == 0 {
		rows = append(rows, kit.AsideNote(th.Theme, i18n.T("No run yet. Run it now to see what it does.")))
	}
	for index, task := range tasks[:min(len(tasks), historyLimit)] {
		rows = append(rows, a.historyRow(th, e, index, task))
	}
	return kit.Aside(gtx, th.Theme, i18n.T("When it runs"), rows...)
}

// why the runs listed will not happen yet, if they will not
func quietNote(ed *editor, snapshot state.Snapshot, routine protocol.Routine) string {
	if team, ok := teamsui.TeamByID(snapshot.Teams, ed.teamID); ok && team.Paused {
		return i18n.T("Its team is paused, so nothing runs until the team works again.")
	}
	if (ed.creating && !ed.on.Value) || (!ed.creating && !routine.Enabled) {
		return i18n.T("It is off, so these runs wait until you turn it on.")
	}
	return ""
}

// a next run: its day and time, and how soon while it is near
func nextRow(th *look.Theme, now, at time.Time) layout.Widget {
	when, soon := look.When(now, at), look.Until(now, at)
	return func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		children := []layout.FlexChild{layout.Flexed(1, kit.Text(th.Theme, th.T.Body, th.R.Fg, when))}
		if soon != when {
			children = append(children, layout.Rigid(kit.Text(th.Theme, th.T.Caption, th.R.Fg3, soon)))
		}
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Baseline}.Layout(gtx, children...)
	}
}

// one past run: its state, when it ended, and what it reported; it opens the task
func (a *asideState) historyRow(th *look.Theme, e *env.Env, index int, task protocol.TeamTask) layout.Widget {
	click := &a.runs[index]
	word, tone, _ := teamsui.TaskStanding(task)
	// a run that found nothing new is done all the same, and says so
	if task.Report != nil && task.Report.Quiet {
		word, tone = i18n.T("Quiet"), palette.Idle
	}
	at := task.Created
	if task.Ended != nil {
		at = *task.Ended
	}
	said := word
	switch {
	case task.Report != nil:
		said = task.Report.Title
	case task.Reason != "":
		said = task.Reason
	}
	return func(gtx layout.Context) layout.Dimensions {
		if click.Clicked(gtx) {
			teamsui.OpenTask(e, task.ID)
		}
		ink := th.R.Fg
		if click.Hovered() {
			ink = th.R.Fg2
		}
		return click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return kit.Flake(gtx, th.Theme, tone, 1, 9)
				}),
				layout.Rigid(layout.Spacer{Width: th.Sp.S2}.Layout),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
						layout.Rigid(kit.Text(th.Theme, th.T.Body, ink, said)),
						layout.Rigid(kit.Text(th.Theme, th.T.Caption, th.R.Fg3, look.Clauses(word, look.Ago(gtx.Now, at)))),
					)
				}),
			)
		})
	}
}
