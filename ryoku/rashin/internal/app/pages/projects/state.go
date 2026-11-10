package projects

import (
	"encoding/json"
	"strings"

	"gioui.org/layout"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/app/pages/graph"
	"ryoku-rashin/internal/protocol"
)

type pageState struct {
	project  string
	tab      string
	worktree string
	watching string
	unlisten func()
	notify   func(string)

	worktreeLoads map[string]*projectLoad
	taskLoads     map[string]*projectLoad

	list      look.SideList
	listMenu  kit.Menu
	collapsed map[string]bool
	header    look.PaneHeader
	tabs      kit.Tabs
	rise      look.Rise

	add      addDialog
	forget   forgetDialog
	settings settingsDialog

	graph    *graph.View
	overview overviewState
	changes  changesState
	tasks    tasksState
	activity activityState
}

func (p *pageState) init() {
	p.worktreeLoads = make(map[string]*projectLoad)
	p.taskLoads = make(map[string]*projectLoad)
	p.collapsed = make(map[string]bool)
	p.overview.init()
	p.changes.init()
	p.tasks.init()
	p.graph = graph.New()
	p.tab = TabOverview
}

func (p *pageState) fail(err error) {
	if err != nil && p.notify != nil {
		p.notify(err.Error())
	}
}

func (p *pageState) report() func(error) { return p.fail }

func (p *pageState) open(e projectEnvironment, project, tab, worktree string) {
	if p.worktreeLoads == nil {
		p.init()
	}
	if project == "" {
		p.hidden(e)
		return
	}
	switch tab {
	case TabOverview, TabGraph, TabChanges, TabTasks, TabActivity:
	default:
		tab = TabOverview
	}
	changed := project != p.project
	p.project, p.tab = project, tab
	if changed {
		p.worktree = ""
	}
	if worktree != "" {
		p.worktree = worktree
	}
	if changed {
		p.startWatch(e, project)
		p.activity.loaded = ""
		p.changes.loadedPath = ""
	}
	epoch := e.Snapshot().Epoch
	p.allowFailedRetry(p.worktreeLoads, project, epoch)
	p.allowFailedRetry(p.taskLoads, project, epoch)
	p.requestProjectData(e, project)
}

func (p *pageState) startWatch(e projectEnvironment, project string) {
	if p.watching == project {
		return
	}
	p.hidden(e)
	p.watching = project
	e.WatchProject(project)
	p.unlisten = e.OnGitChanged(project, func() {
		p.requestLoad(e, project, p.worktreeLoads, true, e.LoadWorktrees)
		p.graph.Refresh()
		p.changes.loadedPath = ""
		p.activity.loaded = ""
	})
}

func (p *pageState) hidden(e projectEnvironment) {
	if p.unlisten != nil {
		p.unlisten()
		p.unlisten = nil
	}
	if p.watching != "" {
		e.UnwatchProject(p.watching)
		p.watching = ""
	}
}

func (p *pageState) projectInfo(e *env.Env) (protocol.ProjectInfo, bool) {
	for _, project := range e.Snapshot().Projects {
		if project.ID == p.project {
			return project, true
		}
	}
	return protocol.ProjectInfo{}, false
}

func (p *pageState) selectedWorktree(e *env.Env) (protocol.WorktreeInfo, bool) {
	worktrees := e.Snapshot().Worktrees[p.project]
	for _, worktree := range worktrees {
		// a launch can name the worktree instead of giving its path
		if worktree.Path == p.worktree || worktree.Name == p.worktree {
			return worktree, true
		}
	}
	if len(worktrees) > 0 {
		return worktrees[0], true
	}
	return protocol.WorktreeInfo{}, false
}

func (p *pageState) newSession(e *env.Env, worktree string) {
	var info protocol.SessionInfo
	params := protocol.SessionNewParams{Kind: protocol.KindAgent, Origin: protocol.OriginApp, Project: p.project, Worktree: worktree}
	e.Call(protocol.MSessionNew, params, &info, func(err error) {
		if err != nil {
			p.fail(err)
			return
		}
		e.Attach(info.ID, nil)
		e.Navigate(protocol.TabAgent, info.ID)
	})
}

func (p *pageState) loadTasks(e *env.Env) {
	p.requestLoad(e, p.project, p.taskLoads, true, e.LoadTasks)
}

func rawPatch(value any) json.RawMessage {
	raw, _ := json.Marshal(value)
	return raw
}

func splitList(value string) []string {
	fields := strings.FieldsFunc(value, func(separator rune) bool { return separator == ',' || separator == '\n' })
	result := make([]string, 0, len(fields))
	for _, field := range fields {
		if field = strings.TrimSpace(field); field != "" {
			result = append(result, field)
		}
	}
	return result
}

func shortHash(hash string) string {
	if len(hash) > 8 {
		return hash[:8]
	}
	return hash
}

func (p *pageState) layout(gtx layout.Context, e *env.Env) layout.Dimensions {
	p.notify = e.Notify
	if _, ok := p.projectInfo(e); !ok {
		return p.layoutEmpty(gtx, e)
	}
	p.ensureProjectData(e, p.project)
	return p.layoutProject(gtx, e)
}

func (p *pageState) layoutEmpty(gtx layout.Context, e *env.Env) layout.Dimensions {
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return p.header.Layout(gtx, e.Theme, e.Overlays, look.PaneHeaderContent{Title: i18n.T("Projects")})
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			p.add.start.Kind, p.add.start.Label = kit.PlaqueGold, i18n.T("Add project")
			if p.add.start.Clicked(gtx) {
				p.add.ask(e)
			}
			empty := kit.Empty{
				Icon: look.Icon.Projects, Title: i18n.T("No projects yet"),
				Body:    i18n.T("Add a folder with a Git repository to work in worktrees, review changes, plan tasks and read its history."),
				Actions: []*kit.Plaque{&p.add.start},
			}
			return empty.Layout(gtx, e.Theme.Theme)
		}),
	)
}
