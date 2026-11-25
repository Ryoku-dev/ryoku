package ask

import (
	"strings"

	"gioui.org/layout"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/protocol"
)

type handoffDialog struct {
	id, text string
	dialog   kit.Dialog
	move     kit.Plaque
	cancel   kit.Plaque

	project, worktree           look.Select
	projectIndex, worktreeIndex int
	model                       env.ModelForm
	folder                      env.PathField
	name, base, branch          kit.TextField
}

func (d *handoffDialog) open(e *env.Env, id, text string) {
	d.id, d.text, d.dialog.Open = id, text, true
	d.projectIndex, d.worktreeIndex = 0, 0
	d.model.Parts, d.model.Agents = env.ModelOptions|env.ModelPlan|env.ModelCheck, true
	settings := e.Snapshot().Settings.Agent
	d.model.Set(settings.Model, settings.Options)
	d.model.SetPlan(settings.Plan)
	for _, project := range e.Snapshot().Projects {
		e.LoadWorktrees(project.ID, nil)
	}
}

func (d *handoffDialog) layout(gtx layout.Context, e *env.Env) {
	if d.id == "" {
		return
	}
	snapshot := e.Snapshot()
	d.projectIndex = min(d.projectIndex, len(snapshot.Projects))
	d.cancel.Kind, d.cancel.Label = kit.PlaqueQuiet, i18n.T("Cancel")
	d.move.Kind, d.move.Label = kit.PlaqueGold, i18n.T("Move to agent")
	if d.cancel.Clicked(gtx) {
		d.dialog.Dismiss(gtx)
	}
	if d.move.Clicked(gtx) && d.model.Ready(e) {
		d.submit(e, snapshot.Projects)
		d.dialog.Dismiss(gtx)
	}
	th := e.Theme
	d.dialog.Layout(gtx, th.Theme, i18n.T("Move this ask to an agent"),
		func(gtx layout.Context) layout.Dimensions { return d.body(gtx, e, snapshot.Projects) },
		func(gtx layout.Context) layout.Dimensions { return d.cancel.Layout(gtx, th.Theme) },
		func(gtx layout.Context) layout.Dimensions {
			if !d.model.Ready(e) {
				gtx = gtx.Disabled()
			}
			return d.move.Layout(gtx, th.Theme)
		},
	)
	if d.dialog.Closed() {
		d.id = ""
	}
}

func (d *handoffDialog) submit(e *env.Env, projects []protocol.ProjectInfo) {
	model, _ := d.model.Effective()
	params := protocol.HandoffParams{ID: d.id, Model: model, Options: d.model.SessionOptions(), Run: true, Text: d.text}
	if d.projectIndex == 0 {
		params.Cwd = d.folder.Path()
	} else {
		project := projects[d.projectIndex-1]
		params.Project = project.ID
		worktrees := e.Snapshot().Worktrees[project.ID]
		switch {
		case d.worktreeIndex == 1:
			params.Worktree, params.Name = "new", strings.TrimSpace(d.name.Text())
			params.Base, params.Branch = strings.TrimSpace(d.base.Text()), strings.TrimSpace(d.branch.Text())
		case d.worktreeIndex >= 2 && d.worktreeIndex-2 < len(worktrees):
			params.Worktree = worktrees[d.worktreeIndex-2].Path
		}
	}
	var info protocol.SessionInfo
	e.Call(protocol.MSessionHandoff, params, &info, func(err error) {
		if err != nil {
			e.Notify(err.Error())
			return
		}
		e.Attach(info.ID, func(error) {})
		e.Navigate(protocol.TabAgent, info.ID)
	})
}

func (d *handoffDialog) body(gtx layout.Context, e *env.Env, projects []protocol.ProjectInfo) layout.Dimensions {
	th := e.Theme
	projectLabels := []string{i18n.T("No project")}
	for _, project := range projects {
		projectLabels = append(projectLabels, project.Name)
	}
	field := func(selection *look.Select, label string, options []string, selected int) layout.FlexChild {
		selection.Label = label
		return layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			return selection.Layout(gtx, th, e.Overlays, options, selected)
		})
	}
	text := func(input *kit.TextField, label, placeholder string) layout.FlexChild {
		input.Label, input.Placeholder = label, placeholder
		return layout.Rigid(func(gtx layout.Context) layout.Dimensions { return input.Layout(gtx, th.Theme) })
	}
	gap := layout.Rigid(layout.Spacer{Height: th.Sp.S3}.Layout)

	children := []layout.FlexChild{
		layout.Rigid(look.Paragraph(th, th.T.Body, th.R.Fg2,
			i18n.T("The agent starts with this question and its answer as context."))),
		layout.Rigid(layout.Spacer{Height: th.Sp.S4}.Layout),
		field(&d.project, i18n.T("Project"), projectLabels, d.projectIndex),
		gap,
	}
	if index, changed := d.project.Changed(); changed {
		d.projectIndex, d.worktreeIndex = index, 0
	}
	if d.projectIndex == 0 {
		if d.folder.Text() == "" {
			d.folder.SetText(paths.Home())
		}
		d.folder.Title, d.folder.Label, d.folder.Placeholder = i18n.T("Where the agent works"), i18n.T("Folder"), paths.Home()
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions { return d.folder.Layout(gtx, e) }), gap)
	} else {
		project := projects[d.projectIndex-1]
		worktreeLabels := []string{i18n.T("Project folder"), i18n.T("New worktree")}
		for _, worktree := range e.Snapshot().Worktrees[project.ID] {
			worktreeLabels = append(worktreeLabels, worktree.Name)
		}
		d.worktreeIndex = min(d.worktreeIndex, len(worktreeLabels)-1)
		children = append(children, field(&d.worktree, i18n.T("Worktree"), worktreeLabels, d.worktreeIndex), gap)
		if index, changed := d.worktree.Changed(); changed {
			d.worktreeIndex = index
		}
		if d.worktreeIndex == 1 {
			children = append(children,
				text(&d.name, i18n.T("Name"), ""), gap,
				text(&d.base, i18n.T("Base"), project.BaseRef), gap,
				text(&d.branch, i18n.T("Branch"), ""), gap,
			)
		}
	}
	children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
		return d.model.Layout(gtx, e)
	}))
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}
