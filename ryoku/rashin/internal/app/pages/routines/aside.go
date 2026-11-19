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
	runs    [historyLimit]kit.Click
	ledger  []protocol.RoutineRun
	links   []kit.Click
	loaded  string
	busy    bool
	problem string
	shown   int
	more    kit.Plaque
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
	if e.Feature("schedules") && !ed.creating {
		return a.layoutRuns(gtx, e, routine, rows)
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

func (a *asideState) layoutRuns(gtx layout.Context, e *env.Env, routine protocol.Routine, rows []layout.Widget) layout.Dimensions {
	th := e.Theme
	version := routine.ID + ":" + routine.LastStatus
	if routine.LastRun != nil {
		version += routine.LastRun.String()
	}
	if a.loaded != version && !a.busy {
		a.busy, a.loaded, a.problem, a.shown = true, version, "", 20
		a.ledger, a.links = nil, nil
		var result protocol.RoutineRunsResult
		e.Call(protocol.MTeamsRoutineRuns, protocol.RoutineRunsParams{Routine: routine.ID}, &result, func(err error) {
			a.busy = false
			if a.loaded != version {
				return
			}
			if err != nil {
				a.problem = err.Error()
				return
			}
			a.ledger = result.Runs
			a.links = make([]kit.Click, len(result.Runs))
		})
	}
	rows = append(rows, kit.Text(th.Theme, th.T.Row, th.R.Fg, i18n.T("Runs")))
	if a.problem != "" {
		rows = append(rows, kit.AsideNote(th.Theme, a.problem))
	} else if a.busy {
		rows = append(rows, kit.AsideNote(th.Theme, i18n.T("Reading runs")))
	} else if len(a.ledger) == 0 {
		rows = append(rows, kit.AsideNote(th.Theme, i18n.T("No run yet. Run it now to see what it does.")))
	}
	for index, run := range a.ledger[:min(a.shown, len(a.ledger))] {
		click := &a.links[index]
		rows = append(rows, func(gtx layout.Context) layout.Dimensions {
			if click.Clicked(gtx) {
				switch {
				case run.Dream != "":
					e.NavigateTeams(env.TeamsPlace{Tab: protocol.TabDreams, Team: run.Dream})
				case run.Kind == protocol.RoutineAsk && run.Session != "":
					e.Attach(run.Session, func(err error) {
						if err == nil {
							e.Navigate(protocol.TabAsk, run.Session)
						} else {
							e.Notify(err.Error())
						}
					})
				case run.Task != "":
					teamsui.OpenTask(e, run.Task)
				}
			}
			duration := i18n.T("Still working")
			if run.Ended != nil {
				duration = run.Ended.Sub(run.Started).Round(time.Second).String()
			}
			status := run.Status
			switch status {
			case protocol.TaskDone:
				status = i18n.T("Done")
			case protocol.TaskBlocked:
				status = i18n.T("Did not finish")
			case protocol.TaskStopped:
				status = i18n.T("Stopped")
			case "skipped":
				status = i18n.T("Skipped")
			case protocol.TaskReady:
				status = i18n.T("Queued")
			case protocol.TaskWorking, "starting":
				status = i18n.T("Working")
			case protocol.TaskNeedsYou:
				status = i18n.T("Needs you")
			}
			return layout.Inset{Top: th.Sp.S2, Bottom: th.Sp.S2}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				children := []layout.FlexChild{
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						label := look.Clauses(status, duration)
						if run.Task == "" && run.Session == "" && run.Dream == "" {
							return look.Paragraph(th, th.T.Body, th.R.Fg, label)(gtx)
						}
						return look.Link(gtx, th, click, label)
					}),
					layout.Rigid(look.Paragraph(th, th.T.Caption, th.R.Fg3, look.When(gtx.Now, run.Started))),
				}
				if run.Detail != "" {
					children = append(children, layout.Rigid(look.Paragraph(th, th.T.Caption, th.R.Fg2, run.Detail)))
				}
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
			})
		})
	}
	if a.shown < len(a.ledger) {
		a.more.Kind, a.more.Label = kit.PlaqueTonal, i18n.T("Show more runs")
		if a.more.Clicked(gtx) {
			a.shown += 20
		}
		rows = append(rows, func(gtx layout.Context) layout.Dimensions { return a.more.Layout(gtx, th.Theme) })
	}
	return kit.Aside(gtx, th.Theme, i18n.T("When it runs"), rows...)
}
