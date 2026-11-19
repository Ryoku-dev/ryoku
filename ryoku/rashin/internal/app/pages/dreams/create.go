package dreams

import (
	"strings"

	"gioui.org/layout"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/app/pages/teamsui"
	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/protocol"
)

// the monthly cap a capped dream starts from
const dreamCap int64 = 2_000_000

// the new dream dialog keeps the manager's model with the dream, without an account setup
type createDialog struct {
	dialog kit.Dialog
	scroll look.Scroller

	statement look.MultiLine
	name      kit.TextField
	engine    teamsui.EngineForm
	budget    kit.Segmented
	limit     teamsui.TokenStepper
	folder    env.PathField
	preset    look.Select
	presetID  string

	start, cancel kit.Plaque
	shown         bool
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
	d.engine.Set("", nil)
	d.budget.Selected, d.limit.Value = 0, dreamCap
	d.preset, d.presetID = look.Select{}, ""
	d.start.Busy = false
	d.statement.Focus()
}

func (d *createDialog) layout(gtx layout.Context, e *env.Env) {
	if !d.shown {
		return
	}
	th := e.Theme
	statement := strings.TrimSpace(d.statement.Text())
	ready := statement != "" && !d.start.Busy
	d.cancel.Kind, d.cancel.Label = kit.PlaqueQuiet, i18n.T("Cancel")
	d.start.Kind, d.start.Label = kit.PlaqueGold, i18n.T("Start")
	if d.cancel.Clicked(gtx) {
		d.dialog.Dismiss(gtx)
	}
	if d.start.Clicked(gtx) && ready {
		d.submit(e, statement)
	}
	rows := d.rows(e)
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

func (d *createDialog) rows(e *env.Env) []layout.Widget {
	th := e.Theme
	d.statement.Label, d.statement.Lines = i18n.T("Your dream"), 5
	d.statement.Placeholder = i18n.T("What you want made, in your own words.")
	d.statement.Help = i18n.T("More detail helps: what it is for, what it must do and how you will know it is done. The manager asks you about the rest.")
	if e.Feature("research") && d.presetID == "research" {
		d.statement.Label = i18n.T("Research question")
		d.statement.Placeholder = i18n.T("What do you want to understand or compare?")
		d.statement.Help = i18n.T("The manager plans the questions, gathers independent sources and presents a cited report.")
	}
	d.name.Label, d.name.Placeholder = i18n.T("Name"), i18n.T("Optional")
	d.name.Help = i18n.T("Empty takes a name from your dream.")
	d.folder.Label, d.folder.Title = i18n.T("Folder"), i18n.T("The dream's folder")
	d.folder.Placeholder = paths.Short(paths.DreamsHome())
	d.folder.Help = i18n.T("Where the team works and keeps what it makes. Empty makes one under ~/Dreams.")
	d.budget.Labels = []string{i18n.T("Unlimited"), i18n.T("A monthly cap")}
	rows := []layout.Widget{
		func(gtx layout.Context) layout.Dimensions { return d.statement.Layout(gtx, th) },
		func(gtx layout.Context) layout.Dimensions { return d.name.Layout(gtx, th.Theme) },
		kit.Text(th.Theme, th.T.Section, th.R.Fg, i18n.T("Manager model")),
		func(gtx layout.Context) layout.Dimensions { return d.engine.Layout(gtx, e) },
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
	if e.Feature("research") {
		rows = append([]layout.Widget{func(gtx layout.Context) layout.Dimensions {
			d.preset.Label = i18n.T("Dream preset")
			selected := 0
			if d.presetID == "research" {
				selected = 1
			}
			dimensions := d.preset.Layout(gtx, th, e.Overlays, []string{i18n.T("Make something"), i18n.T("Deep research")}, selected)
			if index, changed := d.preset.Changed(); changed {
				d.presetID = ""
				if index == 1 {
					d.presetID = "research"
				}
			}
			return dimensions
		}}, rows...)
	}
	return rows
}

func budgetHelp(capped bool) string {
	if capped {
		return i18n.T("The dream pauses at the cap and asks you whether to raise it.")
	}
	return i18n.T("The team works until the dream is done, however many tokens it takes.")
}

// start the native manager and open its dream page
func (d *createDialog) submit(e *env.Env, statement string) {
	model, options := d.engine.Value()
	params := protocol.DreamCreateParams{
		Statement: statement, Name: strings.TrimSpace(d.name.Text()), Folder: d.folder.Path(),
		Model: model, Options: options,
	}
	if e.Feature("research") {
		params.Preset = d.presetID
	}
	if d.budget.Selected == 1 {
		params.MonthTokens = d.limit.Value
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
		e.Notify(i18n.Tf("%s has started. Its manager will ask you about the design first.", dream.Name))
	})
}
