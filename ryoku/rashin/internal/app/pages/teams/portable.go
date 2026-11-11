package teams

import (
	"strings"

	"gioui.org/layout"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/protocol"
)

// the daemon writes the file into the team's folder, so the note says where it went
func exportTeam(e *env.Env, team protocol.Team) {
	var result protocol.TeamExportResult
	e.Call(protocol.MTeamsExport, protocol.TeamRef{ID: team.ID}, &result, func(err error) {
		if err != nil {
			e.Notify(err.Error())
			return
		}
		e.Notify(i18n.Tf("Exported %s to %s.", team.Name, paths.Short(result.Path)))
	})
}

func importTeam(e *env.Env, path string) {
	var team protocol.Team
	e.Call(protocol.MTeamsImport, protocol.TeamImportParams{Path: path}, &team, func(err error) {
		if err != nil {
			e.Notify(err.Error())
			return
		}
		openTeam(e, team.ID, env.TeamOverview)
		e.Notify(i18n.Tf("%s arrived paused, with its routines off. Resume it when you are ready.", team.Name))
	})
}

// a team file is picked in the system chooser; without one the path is typed here
type importDialog struct {
	dialog kit.Dialog
	path   kit.TextField
	start  kit.Plaque
	cancel kit.Plaque
}

func (d *importDialog) choose(e *env.Env) {
	e.ChooseFiles(i18n.T("Import a team"), func(chosen []string) { importTeam(e, chosen[0]) }, func() {
		d.path.SetText("")
		d.dialog.Open, d.dialog.Width = true, 600
	})
}

func (d *importDialog) layout(gtx layout.Context, e *env.Env) {
	th := e.Theme
	path := strings.TrimSpace(d.path.Text())
	d.cancel.Kind, d.cancel.Label = kit.PlaqueQuiet, i18n.T("Cancel")
	d.start.Kind, d.start.Label = kit.PlaqueGold, i18n.T("Import team")
	if d.cancel.Clicked(gtx) {
		d.dialog.Dismiss(gtx)
	}
	if d.start.Clicked(gtx) && path != "" {
		importTeam(e, path)
		d.dialog.Dismiss(gtx)
	}
	d.path.Label, d.path.Mono = i18n.T("Team file"), true
	d.path.Placeholder = "~/Teams/research/research-desk.rashin.json"
	d.path.Help = i18n.T("No file chooser answered. Type the path of a .rashin.json file a team was exported to.")
	d.dialog.Layout(gtx, th.Theme, i18n.T("Import a team"),
		func(gtx layout.Context) layout.Dimensions { return d.path.Layout(gtx, th.Theme) },
		func(gtx layout.Context) layout.Dimensions { return d.cancel.Layout(gtx, th.Theme) },
		func(gtx layout.Context) layout.Dimensions {
			if path == "" {
				gtx = gtx.Disabled()
			}
			return d.start.Layout(gtx, th.Theme)
		},
	)
}
