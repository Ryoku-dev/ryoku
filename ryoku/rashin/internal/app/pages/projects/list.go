package projects

import (
	"image"
	"strings"

	"gioui.org/layout"
	"gioui.org/widget"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
	appstate "ryoku-rashin/internal/app/state"
	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/protocol"
)

const (
	projectPrefix  = "project:"
	worktreePrefix = "worktree:"
)

func worktreeRowID(project, path string) string { return worktreePrefix + project + "\x00" + path }

func (p *Page) List(gtx layout.Context, e *env.Env) look.ListDimensions {
	state := &p.state
	snapshot := e.Snapshot()
	rows := make([]look.SideRow, 0, len(snapshot.Projects)*4)
	for _, project := range snapshot.Projects {
		state.ensureProjectData(e, project.ID)
		open := !state.collapsed[project.ID]
		rows = append(rows, look.SideRow{
			ID: projectPrefix + project.ID, Title: project.Name, Branch: true, Open: open,
			Strong: project.ID == state.project, Menu: true,
		})
		if !open {
			continue
		}
		for _, worktree := range snapshot.Worktrees[project.ID] {
			rows = append(rows, worktreeRow(e.Theme, snapshot, worktree))
		}
	}
	dimensions := state.list.Layout(gtx, e.Theme, e.Overlays, []look.SideGroup{{Rows: rows}}, state.selectedRow(e))
	for _, event := range state.list.Events() {
		project, worktree, isWorktree := splitRowID(event.ID)
		switch event.Kind {
		case look.SideToggle:
			state.collapsed[project] = !state.collapsed[project]
		case look.SideOpen:
			view := state.tab
			if project != state.project || (isWorktree && !worktreeScoped(view)) {
				view = TabOverview
			}
			e.NavigateWork(project, view, worktree)
		case look.SideMenu:
			if isWorktree {
				state.openWorktreeMenu(e, project, worktree, event.Anchor)
			} else {
				state.openProjectMenu(e, project, event.Anchor)
			}
		}
	}
	return dimensions
}

func worktreeScoped(view string) bool { return view == TabOverview || view == TabChanges }

func (p *pageState) selectedRow(e *env.Env) string {
	if p.project == "" {
		return ""
	}
	if worktree, ok := p.selectedWorktree(e); ok && worktreeScoped(p.tab) && !p.collapsed[p.project] {
		return worktreeRowID(p.project, worktree.Path)
	}
	return projectPrefix + p.project
}

func splitRowID(id string) (project, worktree string, isWorktree bool) {
	if rest, ok := strings.CutPrefix(id, worktreePrefix); ok {
		project, worktree, _ = strings.Cut(rest, "\x00")
		return project, worktree, true
	}
	return strings.TrimPrefix(id, projectPrefix), "", false
}

func worktreeRow(th *look.Theme, snapshot appstate.Snapshot, worktree protocol.WorktreeInfo) look.SideRow {
	row := look.SideRow{ID: worktreeRowID(worktree.Project, worktree.Path), Title: worktree.Name, Depth: 1, Menu: true}
	for _, id := range worktree.Sessions {
		switch snapshot.Sessions[id].State {
		case protocol.StateWorking:
			row.Working = true
		case protocol.StateWaiting, protocol.StateFailed:
			row.Tone = palette.Attention
		}
	}
	if status := worktree.Status; status != nil {
		if status.Conflicted > 0 {
			row.Tone = palette.Attention
		}
		if status.Added+status.Removed > 0 {
			row.Trail = look.DiffCount(th, status.Added, status.Removed, false)
		}
	}
	return row
}

func (p *pageState) openProjectMenu(e *env.Env, project string, anchor image.Rectangle) {
	items := []kit.MenuItem{
		{Icon: look.Icon.Add, Label: i18n.T("New worktree")},
		{Icon: look.Icon.Folder, Label: i18n.T("Open folder")},
		{Icon: look.Icon.Settings, Label: i18n.T("Project settings")},
		{Divider: true},
		{Icon: look.Icon.Close, Label: i18n.T("Forget project"), Danger: true},
	}
	p.openListMenu(e, anchor, items, func(item int) {
		info := protocol.ProjectInfo{}
		for _, candidate := range e.Snapshot().Projects {
			if candidate.ID == project {
				info = candidate
			}
		}
		switch item {
		case 0:
			e.NavigateWork(project, TabOverview, "")
			p.overview.openNew(e, p, info)
		case 1:
			openFolder(info.Root)
		case 2:
			p.settings.open(info)
		case 4:
			p.forget.open(info)
		}
	})
}

func (p *pageState) openWorktreeMenu(e *env.Env, project, worktree string, anchor image.Rectangle) {
	items := []kit.MenuItem{
		{Icon: look.Icon.Agents, Label: i18n.T("New session here")},
		{Icon: look.Icon.Diff, Label: i18n.T("Open changes")},
		{Icon: look.Icon.Terminal, Label: i18n.T("Open terminal")},
		{Icon: look.Icon.Folder, Label: i18n.T("Open folder")},
	}
	p.openListMenu(e, anchor, items, func(item int) {
		switch item {
		case 0:
			p.project = project
			p.newSession(e, worktree)
		case 1:
			e.NavigateWork(project, TabChanges, worktree)
		case 2:
			openTerminal(worktree)
		case 3:
			openFolder(worktree)
		}
	})
}

func (p *pageState) openListMenu(e *env.Env, anchor image.Rectangle, items []kit.MenuItem, chosen func(int)) {
	tag := &p.listMenu
	e.Overlays.Open(tag, anchor, look.Placement{}, func(gtx layout.Context) layout.Dimensions {
		dimensions := p.listMenu.Layout(gtx, e.Theme.Theme, items)
		if item, ok := p.listMenu.Clicked(gtx); ok {
			e.Overlays.Close(tag)
			chosen(item)
		}
		return dimensions
	})
}

type addDialog struct {
	shown       bool
	dialog      kit.Dialog
	path        kit.TextField
	add, cancel kit.Plaque
	start       kit.Plaque
}

func (d *addDialog) open() {
	d.shown, d.dialog.Open = true, true
	d.path.Editor.Submit = true
}

func (d *addDialog) layout(gtx layout.Context, e *env.Env) {
	if !d.shown {
		return
	}
	th := e.Theme
	submitted := false
	for {
		event, ok := d.path.Editor.Update(gtx)
		if !ok {
			break
		}
		if _, ok := event.(widget.SubmitEvent); ok {
			submitted = true
		}
	}
	d.path.Label, d.path.Placeholder, d.path.Mono = i18n.T("Folder"), paths.Home()+"/Work/project", true
	d.path.Help = i18n.T("A folder with a Git repository gets worktrees, review and its history.")
	d.cancel.Kind, d.cancel.Label = kit.PlaqueQuiet, i18n.T("Cancel")
	d.add.Kind, d.add.Label = kit.PlaqueGold, i18n.T("Add project")
	folder := strings.TrimSpace(d.path.Text())
	if d.cancel.Clicked(gtx) {
		d.dialog.Dismiss(gtx)
	}
	if (d.add.Clicked(gtx) || submitted) && folder != "" {
		var added protocol.ProjectInfo
		e.Call(protocol.MProjectsAdd, protocol.ProjectAddParams{Path: folder}, &added, func(err error) {
			if err != nil {
				e.Notify(err.Error())
				return
			}
			d.path.SetText("")
			e.NavigateWork(added.ID, TabOverview, "")
		})
		d.dialog.Dismiss(gtx)
	}
	d.dialog.Layout(gtx, th.Theme, i18n.T("Add a project"),
		func(gtx layout.Context) layout.Dimensions {
			dimensions := d.path.Layout(gtx, th.Theme)
			if d.dialog.Open && !gtx.Focused(&d.path.Editor) && d.path.Editor.Len() == 0 {
				d.path.Focus(gtx)
			}
			return dimensions
		},
		func(gtx layout.Context) layout.Dimensions { return d.cancel.Layout(gtx, th.Theme) },
		func(gtx layout.Context) layout.Dimensions { return d.add.Layout(gtx, th.Theme) },
	)
	if d.dialog.Closed() {
		d.shown = false
	}
}

type forgetDialog struct {
	project protocol.ProjectInfo
	confirm kit.Confirm
}

func (d *forgetDialog) open(project protocol.ProjectInfo) {
	d.project, d.confirm.Dialog.Open, d.confirm.Danger = project, true, true
}

func (d *forgetDialog) layout(gtx layout.Context, e *env.Env, page *pageState) {
	if d.project.ID == "" {
		return
	}
	body := i18n.Tf("Rashin stops tracking %s. Its files, its repository and its worktrees stay on disk.", d.project.Name)
	if d.confirm.Layout(gtx, e.Theme.Theme, i18n.T("Forget this project?"), body, i18n.T("Forget"), "") {
		id := d.project.ID
		e.Call(protocol.MProjectsRemove, protocol.ProjectRef{Project: id}, new(protocol.Empty), func(err error) {
			if err != nil {
				e.Notify(err.Error())
				return
			}
			if page.project == id {
				e.NavigateWork("", TabOverview, "")
			}
		})
	}
	if d.confirm.Closed() {
		d.project = protocol.ProjectInfo{}
	}
}
