package dreams

import (
	"strings"

	"gioui.org/layout"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/app/pages/teamsui"
	"ryoku-rashin/internal/app/state"
	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/protocol"
)

// the monthly cap a capped dream starts from
const dreamCap int64 = 2_000_000

// the New dream dialog: what the CEO wants, the manager's engine, the budget and the folder, and how the manager signs in
type createDialog struct {
	dialog kit.Dialog
	scroll look.Scroller

	statement look.MultiLine
	name      kit.TextField
	engine    teamsui.EngineForm
	budget    kit.Segmented
	limit     teamsui.TokenStepper
	folder    env.PathField
	signIn    look.Select
	// the sign-in picked among the dialog's setup sources
	source int

	start, cancel kit.Plaque
	// the dialog stands until its close has played out
	shown bool
}

// a sign-in the dialog offers: from scratch, or from the setup of a team signed in to the harness
type setupSource struct {
	label, team string
}

func (d *createDialog) open() {
	if d.dialog.Open {
		return
	}
	d.shown, d.dialog.Open, d.dialog.Width = true, true, 640
	d.scroll = look.Scroller{}
	d.statement.SetText("")
	d.name.SetText("")
	d.folder.SetText("")
	d.engine.Set("", "", nil)
	d.budget.Selected, d.limit.Value = 0, dreamCap
	d.signIn, d.source = look.Select{}, 0
	d.start.Busy = false
	d.statement.Focus()
}

func (d *createDialog) layout(gtx layout.Context, e *env.Env) {
	if !d.shown {
		return
	}
	th := e.Theme
	snapshot := e.Snapshot()
	statement := strings.TrimSpace(d.statement.Text())
	ready := statement != "" && !d.start.Busy
	d.cancel.Kind, d.cancel.Label = kit.PlaqueQuiet, i18n.T("Cancel")
	d.start.Kind, d.start.Label = kit.PlaqueGold, i18n.T("Start")
	if d.cancel.Clicked(gtx) {
		d.dialog.Dismiss(gtx)
	}
	sources, harness, signs := d.sources(snapshot)
	// the models listed are those of the setup the dream starts from, not the user's own
	d.engine.Team = ""
	if signs {
		d.engine.Team = sources[min(d.source, len(sources)-1)].team
	}
	if d.start.Clicked(gtx) && ready {
		d.submit(e, statement, harness, signs, sources)
	}
	rows := d.rows(e, harness, signs, sources)
	d.dialog.Layout(gtx, th.Theme, i18n.T("New dream"),
		func(gtx layout.Context) layout.Dimensions {
			// the body scrolls so the dialog fits a short window
			gtx.Constraints.Max.Y = min(gtx.Constraints.Max.Y, gtx.Dp(560))
			gtx.Constraints.Min.Y = 0
			return d.scroll.Layout(gtx, th, len(rows), func(gtx layout.Context, index int) layout.Dimensions {
				return layout.Inset{Bottom: th.Sp.S4, Right: th.Sp.S3}.Layout(gtx, rows[index])
			})
		},
		func(gtx layout.Context) layout.Dimensions { return d.cancel.Layout(gtx, th.Theme) },
		func(gtx layout.Context) layout.Dimensions {
			if !ready {
				gtx = gtx.Disabled()
			}
			return d.start.Layout(gtx, th.Theme)
		},
	)
	if d.dialog.Closed() {
		d.shown = false
	}
}

// the harness the manager would start on, whether it signs in for each dream, and the setups it may start from
func (d *createDialog) sources(snapshot state.Snapshot) ([]setupSource, protocol.HarnessInfo, bool) {
	id, _, _ := d.engine.Effective(snapshot)
	harness, ok := env.HarnessByID(snapshot.Harnesses, id)
	if !ok || !harness.Installed || harness.Teams == nil {
		return nil, harness, false
	}
	first := i18n.T("Sign in when it starts")
	if len(harness.Teams.Login) == 0 {
		first = i18n.T("Set it up when it starts")
	}
	sources := []setupSource{{label: first}}
	for _, home := range harness.Teams.Homes {
		if home.SignedIn == nil {
			continue
		}
		if team, known := teamsui.TeamByID(snapshot.Teams, home.Team); known {
			sources = append(sources, setupSource{label: i18n.Tf("Use the setup of %s", team.Name), team: team.ID})
		}
	}
	return sources, harness, true
}

func (d *createDialog) rows(e *env.Env, harness protocol.HarnessInfo, signs bool, sources []setupSource) []layout.Widget {
	th := e.Theme
	d.statement.Label, d.statement.Lines = i18n.T("Your dream"), 5
	d.statement.Placeholder = i18n.T("What you want made, in your own words.")
	d.statement.Help = i18n.T("More detail helps: what it is for, what it must do and how you will know it is done. The manager asks you about the rest.")
	d.name.Label, d.name.Placeholder = i18n.T("Name"), i18n.T("Optional")
	d.name.Help = i18n.T("Empty takes a name from your dream.")
	d.folder.Label, d.folder.Title = i18n.T("Folder"), i18n.T("The dream's folder")
	d.folder.Placeholder = paths.Short(paths.DreamsHome())
	d.folder.Help = i18n.T("Where the team works and keeps what it makes. Empty makes one under ~/Dreams.")
	d.budget.Labels = []string{i18n.T("Unlimited"), i18n.T("A monthly cap")}
	rows := []layout.Widget{
		func(gtx layout.Context) layout.Dimensions { return d.statement.Layout(gtx, th) },
		func(gtx layout.Context) layout.Dimensions { return d.name.Layout(gtx, th.Theme) },
		kit.Text(th.Theme, th.T.Section, th.R.Fg, i18n.T("The manager's engine")),
		func(gtx layout.Context) layout.Dimensions {
			return d.engine.Layout(gtx, e, i18n.Tf("Teams default (%s)", teamsui.DefaultHarnessName(e.Snapshot())))
		},
		kit.Text(th.Theme, th.T.Section, th.R.Fg, i18n.T("Budget")),
		func(gtx layout.Context) layout.Dimensions {
			children := []layout.FlexChild{
				layout.Rigid(func(gtx layout.Context) layout.Dimensions { return d.budget.Layout(gtx, th.Theme) }),
			}
			if d.budget.Selected == 1 {
				children = append(children,
					layout.Rigid(layout.Spacer{Height: th.Sp.S3}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions { return d.limit.Layout(gtx, th) }),
				)
			}
			children = append(children,
				layout.Rigid(layout.Spacer{Height: th.Sp.S2}.Layout),
				layout.Rigid(look.Paragraph(th, th.T.Caption, th.R.Fg3, budgetHelp(d.budget.Selected == 1))),
			)
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
		},
		func(gtx layout.Context) layout.Dimensions { return d.folder.Layout(gtx, e) },
	}
	if signs {
		rows = append(rows,
			kit.Text(th.Theme, th.T.Section, th.R.Fg, i18n.T("Sign-in")),
			func(gtx layout.Context) layout.Dimensions {
				labels := make([]string, len(sources))
				for index, source := range sources {
					labels[index] = source.label
				}
				if index, changed := d.signIn.Changed(); changed {
					d.source = index
				}
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(look.Paragraph(th, th.T.Body, th.R.Fg2, i18n.Tf("%s signs in for each dream on its own. Sign it in when the dream starts, or start from the setup of a team that is signed in.", harness.Name))),
					layout.Rigid(layout.Spacer{Height: th.Sp.S3}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return d.signIn.Layout(gtx, th, e.Overlays, labels, min(d.source, len(labels)-1))
					}),
				)
			},
		)
	}
	return rows
}

func budgetHelp(capped bool) string {
	if capped {
		return i18n.T("The dream pauses at the cap and asks you whether to raise it.")
	}
	return i18n.T("The team works until the dream is done, however many tokens it takes.")
}

// the dream starts, its page opens, and the manager's harness signs in the way the dialog chose
func (d *createDialog) submit(e *env.Env, statement string, harness protocol.HarnessInfo, signs bool, sources []setupSource) {
	harnessID, model, options := d.engine.Value()
	params := protocol.DreamCreateParams{
		Statement: statement, Name: strings.TrimSpace(d.name.Text()), Folder: d.folder.Path(),
		Harness: harnessID, Model: model, Options: options,
	}
	if d.budget.Selected == 1 {
		params.MonthTokens = d.limit.Value
	}
	source := setupSource{}
	if signs {
		source = sources[min(d.source, len(sources)-1)]
	}
	d.start.Busy = true
	var dream protocol.Team
	e.Call(protocol.MTeamsDreamCreate, params, &dream, func(err error) {
		d.start.Busy = false
		if err != nil {
			e.Notify(err.Error())
			return
		}
		d.dialog.Open = false
		openDream(e, dream.ID)
		if !signs {
			e.Notify(i18n.Tf("%s has started. Its manager will ask you about the design first.", dream.Name))
			return
		}
		e.OpenTeamsHome(harness.ID, dream.ID, source.team != "" || len(harness.Teams.Login) == 0, source.team, nil)
		if source.team != "" {
			e.Notify(i18n.Tf("Setting up %s for this dream from %s's setup. The manager starts as soon as it is ready.", harness.Name, teamsui.TeamName(e.Snapshot().Teams, source.team)))
		} else {
			e.Notify(i18n.Tf("Signing %s in for this dream. The manager starts as soon as it is ready.", harness.Name))
		}
	})
}
