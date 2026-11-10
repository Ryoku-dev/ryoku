package projects

import (
	"encoding/json"
	"strings"

	"gioui.org/layout"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/diffview"
	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/app/pages/graph"
	appstate "ryoku-rashin/internal/app/state"
	"ryoku-rashin/internal/protocol"
)

type projectEnvironment interface {
	Snapshot() appstate.Snapshot
	LoadWorktrees(string, func(error))
	LoadTasks(string, func(error))
	WatchProject(string)
	UnwatchProject(string)
	OnGitChanged(string, func()) func()
}

type projectLoad struct {
	attemptEpoch int
	successEpoch int
	attempted    bool
	succeeded    bool
	inFlight     bool
}

type pageState struct {
	project  string
	tab      string
	worktree string
	watching string
	unlisten func()

	worktreeLoads map[string]*projectLoad
	taskLoads     map[string]*projectLoad

	projectRows     map[string]*projectRow
	projectScroll   look.Scroller
	addPath         kit.TextField
	addProject      kit.Plaque
	removeProject   kit.Confirm
	removeProjectID string
	tabs            kit.Tabs
	graph           *graph.View
	overview        overviewState
	changes         changesState
	tasks           tasksState
	activity        activityState
	error           string
}

type projectRow struct {
	row     look.SummaryRow
	more    kit.Plaque
	menu    kit.Menu
	menuTag struct{}
}

type overviewState struct {
	scroll                                              look.Scroller
	rows                                                map[string]*worktreeRow
	newButton, repairButton, pruneButton, cleanupButton kit.Plaque
	newDialog                                           kit.Dialog
	newOpen                                             bool
	newName, newBase, newBranch                         kit.TextField
	newMode, newParent, newRef                          look.Choice
	newModeSelected, newParentSelected, newRefSelected  int
	newLock, newSetup                                   kit.Toggle
	newCreate, newCancel                                kit.Plaque
	refs                                                []protocol.GitRef
	removePreview                                       protocol.RemovePreview
	removeOpen                                          bool
	removeDeleteBranch, removeForceBranch               kit.Toggle
	removeConfirm                                       kit.Confirm
	removeApprove, removeCancel                         kit.Plaque
	moveDialog                                          kit.Dialog
	movePath, moveTo                                    string
	moveField                                           kit.TextField
	moveDo, moveCancel                                  kit.Plaque
	prune                                               protocol.PruneResult
	pruneOpen                                           bool
	pruneConfirm                                        kit.Confirm
	cleanup                                             protocol.CleanupResult
	cleanupOpen                                         bool
	cleanupConfirm                                      kit.Confirm
	settings                                            settingsState
}

type worktreeRow struct {
	openFolder, openTerminal, newSession, more kit.Plaque
	lock, move, remove, setup, repair, trust   kit.Plaque
	ports, showSetup                           kit.Plaque
	portOpen                                   []kit.Plaque
	sessionOpen                                map[string]*kit.Plaque
	setupResult                                protocol.SetupResult
	portsResult                                protocol.PortsResult
	setupDisclosure                            kit.Disclosure
	portsDisclosure                            kit.Disclosure
	moreMenu                                   kit.Menu
	moreTag                                    struct{}
}

type settingsState struct {
	base, root, prefix, setup, copyIgnored kit.TextField
	auto                                   kit.Toggle
	mergeDays, inactiveDays                kit.Stepper
	protect                                look.Choice
	save                                   kit.Plaque
	protectSelected                        int
	project                                string
}

type changesState struct {
	worktreeChoice                        look.Choice
	modeTabs                              kit.Tabs
	status                                protocol.GitStatusResult
	diff                                  protocol.DiffResult
	view                                  diffview.View
	loadedPath, loadedMode                string
	loading                               bool
	selectedPath                          string
	fileList                              layout.List
	fileRows                              map[string]*changeFileRow
	groupRows                             [4]changeGroupRow
	discardConfirm                        kit.Confirm
	discardFiles                          []string
	commitMessage                         kit.TextField
	amend                                 kit.Toggle
	commit, push, fetch, pull, sendReview kit.Plaque
	pushConfirm                           kit.Confirm
	forcePush                             bool
	abort, continueOp, resolveAgent       kit.Plaque
	error                                 string
}

type changeFileRow struct {
	selectFile              kit.Click
	stage, unstage, discard kit.Plaque
}

type changeGroupRow struct {
	stage, unstage, discard kit.Plaque
}

type tasksState struct {
	scroll                          look.Scroller
	board                           layout.List
	columns                         [7]layout.List
	rows                            map[string]*taskRow
	selected                        string
	newTask, planTasks              kit.Plaque
	newDialog                       kit.Dialog
	newOpen                         bool
	title, goal, harness            kit.TextField
	newCreate, newCancel            kit.Plaque
	planDialog                      kit.Dialog
	depToggles                      map[string]*kit.Toggle
	planOpen                        bool
	planGoal                        kit.TextField
	planRun, planCreate, planCancel kit.Plaque
	drafts                          []protocol.TaskDraft
	draftSelected                   []bool
	draftToggles                    []kit.Toggle
	draftTitles                     []kit.TextField
	draftGoals                      []kit.TextField
	draftDeps                       []kit.TextField
	detail                          taskDetail
}

type taskRow struct{ click kit.Click }

type taskDetail struct {
	sessionOpen                                 map[string]*kit.Plaque
	start, done, remove, openWorktree, openGate kit.Plaque
	status                                      look.Choice
	deleteConfirm                               kit.Confirm
	gateQuestion, gateOptions                   kit.TextField
	gateDialog                                  kit.Dialog
	gateOpen                                    bool
	gateCreate, gateCancel                      kit.Plaque
	gateChoices                                 map[string]*look.Choice
}

type activityState struct {
	scroll  look.Scroller
	items   []protocol.Activity
	loaded  string
	loading bool
}

func (p *pageState) init() {
	p.projectRows = make(map[string]*projectRow)
	p.worktreeLoads = make(map[string]*projectLoad)
	p.taskLoads = make(map[string]*projectLoad)
	p.overview.rows = make(map[string]*worktreeRow)
	p.changes.fileRows = make(map[string]*changeFileRow)
	p.changes.fileList.Axis = layout.Vertical
	p.tasks.rows = make(map[string]*taskRow)
	p.tasks.depToggles = make(map[string]*kit.Toggle)
	p.tasks.board.Axis = layout.Horizontal
	for index := range p.tasks.columns {
		p.tasks.columns[index].Axis = layout.Vertical
	}
	p.tasks.detail.gateChoices = make(map[string]*look.Choice)
	p.tasks.detail.sessionOpen = make(map[string]*kit.Plaque)
	p.graph = graph.New()
	p.tab = TabOverview
}

func (p *pageState) open(
	e projectEnvironment,
	project string,
	tab string,
	worktree string,
) {
	if p.projectRows == nil {
		p.init()
	}
	if project == "" {
		p.hidden(e)
		p.project, p.worktree, p.tab = "", "", TabOverview
		return
	}
	if tab == "" {
		tab = TabOverview
	}
	if tab != TabOverview &&
		tab != TabGraph &&
		tab != TabChanges &&
		tab != TabTasks &&
		tab != TabActivity {
		tab = TabOverview
	}
	changed := project != p.project
	p.project, p.tab = project, tab
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

func (p *pageState) layout(
	gtx layout.Context,
	e *env.Env,
	class kit.FrameClass,
) layout.Dimensions {
	if p.projectRows == nil {
		p.init()
	}
	if p.project == "" {
		return p.layoutProjectList(gtx, e)
	}
	p.ensureProjectData(e, p.project)
	return p.layoutProject(gtx, e, class)
}

func (p *pageState) ensureProjectData(e projectEnvironment, project string) {
	snapshot := e.Snapshot()
	if snapshot.Connection != appstate.Ready {
		return
	}
	p.requestProjectData(e, project)
}

func (p *pageState) requestProjectData(e projectEnvironment, project string) {
	p.requestLoad(e, project, p.worktreeLoads, false, e.LoadWorktrees)
	p.requestLoad(e, project, p.taskLoads, false, e.LoadTasks)
}

func (p *pageState) requestLoad(
	e projectEnvironment,
	project string,
	loads map[string]*projectLoad,
	force bool,
	loadFn func(string, func(error)),
) {
	epoch := e.Snapshot().Epoch
	load := loads[project]
	if load == nil {
		load = new(projectLoad)
		loads[project] = load
	}
	if load.inFlight {
		return
	}
	if !force {
		if load.succeeded && load.successEpoch == epoch {
			return
		}
		if load.attempted && load.attemptEpoch == epoch {
			return
		}
	}
	load.inFlight = true
	load.attempted = true
	load.attemptEpoch = epoch
	loadFn(project, func(err error) {
		load.inFlight = false
		if err != nil {
			p.error = err.Error()
		} else {
			load.succeeded = true
			load.successEpoch = epoch
		}
		snapshot := e.Snapshot()
		if snapshot.Connection == appstate.Ready && snapshot.Epoch != epoch {
			p.requestLoad(e, project, loads, false, loadFn)
		}
	})
}

func (p *pageState) allowFailedRetry(
	loads map[string]*projectLoad,
	project string,
	epoch int,
) {
	load := loads[project]
	if load == nil || load.inFlight {
		return
	}
	if load.succeeded && load.successEpoch == epoch {
		return
	}
	if load.attemptEpoch == epoch {
		load.attempted = false
	}
}

func (p *pageState) layoutProject(gtx layout.Context, e *env.Env, class kit.FrameClass) layout.Dimensions {
	labels := []string{i18n.T("Overview"), i18n.T("Graph"), i18n.T("Changes"), i18n.T("Tasks"), i18n.T("Activity")}
	tabs := []string{TabOverview, TabGraph, TabChanges, TabTasks, TabActivity}
	selected := 0
	for i, tab := range tabs {
		if tab == p.tab {
			selected = i
		}
	}
	p.tabs.Labels, p.tabs.Selected = labels, selected
	return (kit.Pane{Kind: kit.PaneWork}).Layout(gtx, e.Theme.Theme, func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Top: 18, Right: 24, Bottom: 13, Left: 24}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
						layout.Flexed(1, kit.Text(e.Theme.Theme, e.Theme.T.Title, e.Theme.R.Fg, p.projectName(e))),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions { return p.tabs.Layout(gtx, e.Theme.Theme) }),
					)
				}),
				layout.Rigid(layout.Spacer{Height: 13}.Layout),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					if index, changed := p.tabs.Changed(); changed && index >= 0 && index < len(tabs) {
						p.tab = tabs[index]
						if e.NavigateWork != nil {
							e.NavigateWork(p.project, p.tab, p.worktree)
						}
					}
					switch p.tab {
					case TabGraph:
						return p.layoutGraph(gtx, e)
					case TabChanges:
						return p.changes.layout(gtx, e, p)
					case TabTasks:
						return p.tasks.layout(gtx, e, p)
					case TabActivity:
						return p.activity.layout(gtx, e, p)
					default:
						return p.overview.layout(gtx, e, p, class)
					}
				}),
			)
		})
	})
}

func (p *pageState) layoutGraph(gtx layout.Context, e *env.Env) layout.Dimensions {
	dimensions := p.graph.Layout(gtx, e, p.project)
	for _, action := range p.graph.Actions() {
		switch action.Kind {
		case graph.ActOpenWorktree:
			p.open(e, action.Project, TabOverview, action.Worktree)
		case graph.ActOpenChanges:
			p.open(e, action.Project, TabChanges, action.Worktree)
		case graph.ActNewSession:
			p.newSession(e, action.Worktree)
		case graph.ActOpenSession:
			if e.Navigate != nil {
				e.Navigate(protocol.TabAgent, action.Session)
			}
		}
	}
	return dimensions
}

func (p *pageState) projectName(e *env.Env) string {
	for _, project := range e.Snapshot().Projects {
		if project.ID == p.project {
			return project.Name
		}
	}
	return p.project
}

func (p *pageState) projectInfo(e *env.Env) (protocol.ProjectInfo, bool) {
	for _, project := range e.Snapshot().Projects {
		if project.ID == p.project {
			return project, true
		}
	}
	return protocol.ProjectInfo{}, false
}

func (p *pageState) newSession(e *env.Env, worktree string) {
	var info protocol.SessionInfo
	params := protocol.SessionNewParams{
		Kind:     protocol.KindAgent,
		Origin:   protocol.OriginApp,
		Project:  p.project,
		Worktree: worktree,
	}
	e.Call(protocol.MSessionNew, params, &info, func(err error) {
		if err != nil {
			p.error = err.Error()
			return
		}
		e.Attach(info.ID, nil)
		if e.Navigate != nil {
			e.Navigate(protocol.TabAgent, info.ID)
		}
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
	fields := strings.FieldsFunc(value, func(r rune) bool { return r == ',' || r == '\n' })
	result := make([]string, 0, len(fields))
	for _, field := range fields {
		if field = strings.TrimSpace(field); field != "" {
			result = append(result, field)
		}
	}
	return result
}
