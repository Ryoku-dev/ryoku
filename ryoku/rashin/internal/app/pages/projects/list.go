package projects

import (
	"strings"

	"gioui.org/layout"
	"gioui.org/op"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/protocol"
)

func (p *pageState) layoutProjectList(gtx layout.Context, e *env.Env) layout.Dimensions {
	projects := e.Snapshot().Projects
	if p.addProject.Clicked(gtx) {
		path := strings.TrimSpace(p.addPath.Text())
		if path != "" {
			var added protocol.ProjectInfo
			e.Call(protocol.MProjectsAdd, protocol.ProjectAddParams{Path: path}, &added, func(err error) {
				if err != nil {
					p.error = err.Error()
					return
				}
				p.addPath.SetText("")
				p.open(e, added.ID, TabOverview, "")
			})
		}
	}
	if p.removeProjectID != "" {
		name := p.removeProjectID
		for _, project := range projects {
			if project.ID == p.removeProjectID {
				name = project.Name
			}
		}
		p.removeProject.Dialog.Open = true
		confirmed := p.removeProject.Layout(
			gtx,
			e.Theme.Theme,
			i18n.T("Forget project"),
			i18n.Tf("Forget %s? Its files and Git repository stay on disk.", name),
			i18n.T("Forget"),
			"",
		)
		if confirmed {
			id := p.removeProjectID
			p.removeProjectID = ""
			e.Call(
				protocol.MProjectsRemove,
				protocol.ProjectRef{Project: id},
				new(protocol.Empty),
				func(err error) {
					if err != nil {
						p.error = err.Error()
					}
				},
			)
		}
		if p.removeProject.Closed() {
			p.removeProjectID = ""
		}
	}
	return (kit.Pane{Kind: kit.PaneWork}).Layout(gtx, e.Theme.Theme, func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Top: 21, Right: 34, Bottom: 13, Left: 34}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(kit.Text(e.Theme.Theme, e.Theme.T.Title, e.Theme.R.Fg, i18n.T("Projects"))),
				layout.Rigid(layout.Spacer{Height: 13}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					p.addPath.Label, p.addPath.Placeholder, p.addPath.Mono = i18n.T("Folder"), i18n.T("/path/to/project"), true
					p.addProject.Kind, p.addProject.Label = kit.PlaqueGold, i18n.T("Add project")
					return layout.Flex{Axis: layout.Horizontal, Alignment: layout.End}.Layout(gtx,
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions { return p.addPath.Layout(gtx, e.Theme.Theme) }),
						layout.Rigid(layout.Spacer{Width: 8}.Layout),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions { return p.addProject.Layout(gtx, e.Theme.Theme) }),
					)
				}),
				layout.Rigid(layout.Spacer{Height: 13}.Layout),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					if len(projects) == 0 {
						empty := kit.Empty{Title: i18n.T("No projects"), Body: i18n.T("Add a project folder to use worktrees, review, tasks and the commit graph.")}
						return empty.Layout(gtx, e.Theme.Theme)
					}
					return p.projectScroll.Layout(gtx, e.Theme, len(projects), func(gtx layout.Context, index int) layout.Dimensions {
						project := projects[index]
						row := p.projectRows[project.ID]
						if row == nil {
							row = new(projectRow)
							p.projectRows[project.ID] = row
						}
						if row.row.Clicked(gtx) || row.row.DoubleClicked(gtx) {
							p.open(e, project.ID, TabOverview, "")
						}
						if row.more.Clicked(gtx) {
							p.openProjectMenu(gtx, e, project, row)
						}
						state, tone := i18n.T("Ready"), palette.Alive
						if project.Attention > 0 {
							state, tone = i18n.T("Needs you"), palette.Attention
						}
						meta := project.Root
						if project.Branch != "" {
							meta += " · " + project.Branch
						}
						meta += " · " + i18n.Tf(
							"%d worktrees · %d sessions · %d tasks",
							project.Worktrees,
							project.Sessions,
							project.Tasks,
						)
						row.row.Title, row.row.State = project.Name, state
						row.row.Meta, row.row.Tone = meta, tone
						row.more.Kind, row.more.Label, row.more.Small = kit.PlaqueQuiet, i18n.T("More"), true
						return layout.Inset{Bottom: 5}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
								layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
									return row.row.Layout(gtx, e.Theme, false)
								}),
								layout.Rigid(layout.Spacer{Width: 4}.Layout),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									return row.more.Layout(gtx, e.Theme.Theme)
								}),
							)
						})
					})
				}),
			)
		})
	})
}

func (p *pageState) openProjectMenu(
	gtx layout.Context,
	e *env.Env,
	project protocol.ProjectInfo,
	row *projectRow,
) {
	if e.Overlays == nil {
		return
	}
	if e.Overlays.IsOpen(&row.menuTag) {
		e.Overlays.Close(&row.menuTag)
		return
	}
	items := []kit.MenuItem{{Label: i18n.T("Forget"), Danger: true}}
	content := func(gtx layout.Context) layout.Dimensions {
		dimensions := row.menu.Layout(gtx, e.Theme.Theme, items)
		if _, clicked := row.menu.Clicked(gtx); clicked {
			e.Overlays.Close(&row.menuTag)
			p.removeProjectID = project.ID
			p.removeProject.Danger = true
			p.removeProject.Dialog.Open = true
			gtx.Execute(op.InvalidateCmd{})
		}
		return dimensions
	}
	e.Overlays.OpenAt(&row.menuTag, e.Overlays.Press(), content)
}
