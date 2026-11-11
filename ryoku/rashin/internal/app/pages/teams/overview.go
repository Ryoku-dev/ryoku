package teams

import (
	"gioui.org/layout"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/app/pages/teamsui"
	appstate "ryoku-rashin/internal/app/state"
	"ryoku-rashin/internal/protocol"
)

// the rows the right column shows at most per section
const overviewRows = 5

type overview struct {
	scroll  look.Scroller
	edit    kit.IconPlaque
	editTip look.Tip
	read    kit.Plaque
	tasks   linkRows
	coming  map[string]*kit.Click
	crew    crewLive
	steps   nextSteps
	files   teamFiles

	activity    load
	entries     []protocol.Activity
	entriesTeam string
}

func (o *overview) loadActivity(e *env.Env, team protocol.Team) {
	o.activity.want(e, teamKey(e.Snapshot().Epoch, team), func(done func()) {
		var result protocol.ActivityResult
		params := protocol.TeamActivityParams{Team: team.ID, Filter: protocol.ActivityAll, Limit: overviewRows}
		e.Call(protocol.MTeamsActivity, params, &result, func(err error) {
			done()
			if err != nil {
				e.Notify(err.Error())
				return
			}
			o.entries, o.entriesTeam = result.Activity, team.ID
		})
	})
}

func (o *overview) layout(gtx layout.Context, e *env.Env, team protocol.Team) layout.Dimensions {
	th := e.Theme
	o.loadActivity(e, team)
	snapshot := e.Snapshot()
	left := func(gtx layout.Context) layout.Dimensions {
		return sections(gtx,
			o.purpose(gtx, e, team),
			o.steps.layout(gtx, e, snapshot, team),
			section(th, i18n.T("The crew"), func(gtx layout.Context) layout.Dimensions {
				return crewChart(gtx, th, snapshot, team.Members, o.crew.of(team.ID), fullCrew(th))
			}),
			o.latestReport(gtx, e, snapshot, team),
			o.files.layout(gtx, e, snapshot, team),
		)
	}
	right := func(gtx layout.Context) layout.Dimensions {
		return sections(gtx,
			section(th, i18n.T("Now"), o.now(gtx, e, snapshot, team)),
			section(th, i18n.T("Coming up"), o.comingUp(gtx, e, snapshot, team)),
			section(th, i18n.T("This month"), teamsui.MonthMeter(th, team.Stats.MonthTokens, team.Budget.MonthTokens, team.Budget.WarnPercent)),
			section(th, i18n.T("Recent activity"), o.recent(th, team)),
		)
	}
	return o.scroll.Layout(gtx, th, 1, func(gtx layout.Context, _ int) layout.Dimensions {
		return pageInset(gtx, e, func(gtx layout.Context) layout.Dimensions {
			return look.Split(gtx, th, left, right)
		})
	})
}

func (o *overview) purpose(gtx layout.Context, e *env.Env, team protocol.Team) layout.Widget {
	th := e.Theme
	o.edit.Kind, o.edit.Small, o.edit.Icon = kit.PlaqueQuiet, true, look.Icon.Rename
	o.edit.Description = i18n.T("Change the name and purpose")
	if o.edit.Clicked(gtx) {
		openTeam(e, team.ID, env.TeamSettings)
	}
	body := look.Paragraph(th, th.T.Proof, th.R.Fg, team.Purpose)
	if team.Purpose == "" {
		body = quiet(th, i18n.T("No purpose yet. Say what the team is for, so its lead knows what to take on."))
	}
	return func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints.Min.X = gtx.Constraints.Max.X
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
					layout.Flexed(1, kit.Text(th.Theme, th.T.Section, th.R.Fg, i18n.T("Purpose"))),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return o.editTip.Layout(gtx, th, e.Overlays, o.edit.Description, func(gtx layout.Context) layout.Dimensions {
							return o.edit.Layout(gtx, th.Theme)
						})
					}),
				)
			}),
			layout.Rigid(layout.Spacer{Height: th.Sp.S3}.Layout),
			layout.Rigid(body),
			layout.Rigid(layout.Spacer{Height: th.Sp.S4}.Layout),
		)
	}
}

func (o *overview) latestReport(gtx layout.Context, e *env.Env, snapshot appstate.Snapshot, team protocol.Team) layout.Widget {
	th := e.Theme
	task, ok := teamsui.NewestReport(snapshot, team.ID)
	if !ok {
		return nil
	}
	o.read.Kind, o.read.Small, o.read.Label = kit.PlaqueLine, true, i18n.T("Read the report")
	if o.read.Clicked(gtx) {
		teamsui.OpenTask(e, task.ID)
	}
	facts := i18n.Tf("%s · %s", teamsui.MemberName(team, task.Member), look.Ago(gtx.Now, task.Report.At))
	// a quiet run reported only that nothing changed since the one before
	if task.Report.Quiet {
		facts = i18n.Tf("%s · %s · nothing new", teamsui.MemberName(team, task.Member), look.Ago(gtx.Now, task.Report.At))
	}
	return section(th, i18n.T("Latest report"), func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(look.Paragraph(th, th.T.Title, th.R.Fg, task.Report.Title)),
			layout.Rigid(layout.Spacer{Height: th.Sp.S1}.Layout),
			layout.Rigid(kit.Text(th.Theme, th.T.Caption, th.R.Fg3, facts)),
			layout.Rigid(layout.Spacer{Height: th.Sp.S2}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return kit.Label{Style: th.T.Proof, Color: th.R.Fg2, MaxLines: 6}.Layout(gtx, th.Theme, teamsui.Excerpt(task.Report.Body, 12))
			}),
			layout.Rigid(layout.Spacer{Height: th.Sp.S3}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions { return o.read.Layout(gtx, th.Theme) }),
		)
	})
}

func (o *overview) now(gtx layout.Context, e *env.Env, snapshot appstate.Snapshot, team protocol.Team) layout.Widget {
	th := e.Theme
	var rows []layout.Widget
	for _, task := range teamsui.Working(snapshot) {
		if task.Team != team.ID {
			continue
		}
		row := o.tasks.row(task.ID)
		if row.click.Clicked(gtx) {
			teamsui.OpenTask(e, task.ID)
		}
		word, tone, _ := teamsui.TaskStanding(task)
		facts := i18n.Tf("%s · %s · started %s", teamsui.MemberName(team, task.Member), word, look.Ago(gtx.Now, teamsui.Started(task)))
		rows = append(rows, func(gtx layout.Context) layout.Dimensions {
			return row.layout(gtx, th, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return kit.Working{Active: tone == palette.Becoming, Size: 16}.Layout(gtx, th.Theme)
					}),
					layout.Rigid(layout.Spacer{Width: th.Sp.S2}.Layout),
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
							layout.Rigid(kit.Text(th.Theme, th.T.Label, th.R.Fg, task.Title)),
							layout.Rigid(kit.Text(th.Theme, th.T.Caption, th.R.Fg3, facts)),
						)
					}),
				)
			})
		})
	}
	if len(rows) == 0 {
		return quiet(th, i18n.T("Nobody on the team is at work right now."))
	}
	return stack(th, rows)
}

// the team's next runs, as the Inbox and Routines list them, without the team they all share
func (o *overview) comingUp(gtx layout.Context, e *env.Env, snapshot appstate.Snapshot, team protocol.Team) layout.Widget {
	th := e.Theme
	routines := teamsui.Upcoming(snapshot, team.ID)
	if len(routines) == 0 {
		return quiet(th, i18n.T("No routine of this team is on."))
	}
	if o.coming == nil {
		o.coming = make(map[string]*kit.Click)
	}
	routines = routines[:min(len(routines), overviewRows)]
	rows := make([]layout.Widget, len(routines))
	for index, routine := range routines {
		click := o.coming[routine.ID]
		if click == nil {
			click = new(kit.Click)
			o.coming[routine.ID] = click
		}
		if click.Clicked(gtx) {
			teamsui.OpenRoutine(e, routine.ID)
		}
		rows[index] = func(gtx layout.Context) layout.Dimensions {
			return teamsui.RunRow(gtx, th, click, *routine.NextRun, routine.Title, "")
		}
	}
	return stack(th, rows)
}

// the team's newest ledger entries, as the Activity page sets them
func (o *overview) recent(th *look.Theme, team protocol.Team) layout.Widget {
	if o.entriesTeam != team.ID || len(o.entries) == 0 {
		return quiet(th, i18n.T("Nothing has happened on this team yet."))
	}
	entries := o.entries
	return func(gtx layout.Context) layout.Dimensions {
		children := make([]layout.FlexChild, len(entries))
		for index, entry := range entries {
			children[index] = layout.Rigid(teamsui.LedgerRow(th, look.Ago(gtx.Now, entry.Time), 96, entry, entry.Detail))
		}
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
	}
}
