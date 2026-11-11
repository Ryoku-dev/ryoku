package shell

import (
	"context"
	"image"
	"sync"
	"time"

	gioapp "gioui.org/app"
	"gioui.org/io/system"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/paint"
	"gioui.org/unit"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/anim"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
	activitypage "ryoku-rashin/internal/app/pages/activity"
	agentpage "ryoku-rashin/internal/app/pages/agent"
	askpage "ryoku-rashin/internal/app/pages/ask"
	inboxpage "ryoku-rashin/internal/app/pages/inbox"
	indexpage "ryoku-rashin/internal/app/pages/index"
	projectspage "ryoku-rashin/internal/app/pages/projects"
	routinespage "ryoku-rashin/internal/app/pages/routines"
	settingspage "ryoku-rashin/internal/app/pages/settings"
	teamspage "ryoku-rashin/internal/app/pages/teams"
	"ryoku-rashin/internal/app/pages/teamsui"
	vaultpage "ryoku-rashin/internal/app/pages/vault"
	workpage "ryoku-rashin/internal/app/pages/work"
	"ryoku-rashin/internal/app/state"
	"ryoku-rashin/internal/protocol"
)

type postQueue struct {
	mu         sync.Mutex
	pending    []func()
	invalidate func()
}

func (q *postQueue) Post(fn func()) {
	if fn == nil {
		return
	}
	q.mu.Lock()
	q.pending = append(q.pending, fn)
	q.mu.Unlock()
	if q.invalidate != nil {
		q.invalidate()
	}
}

func (q *postQueue) Drain() {
	q.mu.Lock()
	pending := q.pending
	q.pending = nil
	q.mu.Unlock()
	for _, fn := range pending {
		fn()
	}
}

type uiShell struct {
	runtime  *runtime
	router   *Router
	posts    postQueue
	overlay  look.Overlays
	ask      *askpage.Page
	agent    *agentpage.Page
	projects *projectspage.Page
	index    *indexpage.Page
	settings *settingspage.Page
	inbox    *inboxpage.Page
	teams    *teamspage.Page
	work     *workpage.Page
	routines *routinespage.Page
	vault    *vaultpage.Page
	activity *activitypage.Page
	palette  commandPalette

	topBar     topBar
	sidebar    sidebar
	sheetBar   sidebar
	bottom     bottomBar
	sheet      sideSheet
	panelScrim sideSheet
	fold       anim.Tween
	panel      anim.Tween
	rise       look.Rise
	placards   look.Placards
	// whose list the sidebar shows, settings keeps the list of where you came from
	listTab string

	preferences  state.AppPreferences
	desktop      env.Desktop
	attention    *attention
	windowAction func(system.Action)
	maximized    bool
	decorated    bool
	class        look.Class
	widthDp      float32
	now          time.Time

	turnOn         kit.Plaque
	retry          kit.Plaque
	lastFocus      protocol.UIOpenParams
	initialApplied bool
}

func newShell(r *runtime, options Options) *uiShell {
	preferences := options.Preferences.Normalize()
	route := options.Route
	if options.Teams.Tab != "" {
		route = options.Teams.Tab
	}
	if !validTab(route) {
		route = protocol.TabAsk
	}
	if options.Project != "" {
		route = protocol.TabProjects
	}
	listTab := route
	if route == protocol.TabSettings {
		listTab = protocol.TabAsk
	}
	s := &uiShell{
		runtime: r, router: NewRouter(route, options.Session, r.hub.Invalidate), preferences: preferences,
		ask: askpage.New(), agent: agentpage.New(), projects: projectspage.New(),
		index: indexpage.New(), settings: settingspage.New(), listTab: listTab,
		inbox: inboxpage.New(), teams: teamspage.New(), work: workpage.New(),
		routines: routinespage.New(), vault: vaultpage.New(), activity: activitypage.New(),
	}
	s.router.paletteTag = &s.palette
	s.router.OnNew = func() { s.newCurrent(s.environment()) }
	s.router.OnToggleFold = s.toggleSidebar
	s.router.OnTogglePanel = s.togglePanel
	s.router.OnZoom = s.changeZoom
	s.router.CanEscape = s.canEscape
	s.router.OnEscape = s.escape
	s.router.OnPlace = s.applyPlace
	s.posts.invalidate = r.hub.Invalidate
	s.settings.OpenSection(options.SettingsSection)
	switch {
	case options.Project != "":
		s.navigateWork(options.Project, options.WorkTab, options.Worktree)
	case options.Teams.Tab != "":
		s.router.Replace(teamsPlace(options.Teams))
	}
	// the mode the window did not open in still goes back to where it stood
	if s.router.lastRashin.Tab == "" {
		s.router.lastRashin = restoredPlace(preferences.Last)
	}
	if s.router.lastTeams.Tab == "" {
		s.router.lastTeams = restoredPlace(preferences.Teams)
	}
	return s
}

func (r *runtime) runWindow(options Options) error {
	shell := newShell(r, resume(options))
	work, done := context.WithCancel(context.Background())
	defer done()
	shell.desktop = windowDesktop(work, &shell.posts)
	shell.attention = r.watchAttention(work, shell)
	anim.SetReduce(shell.preferences.ReduceMotion)
	window := new(gioapp.Window)
	creation := []gioapp.Option{
		gioapp.Title(i18n.T("Rashin")), gioapp.Decorated(false),
		gioapp.Size(unit.Dp(options.Width), unit.Dp(options.Height)),
		gioapp.MinSize(640, 520), gioapp.MaxSize(1048576, 1048576),
	}
	window.Option(creation...)
	shell.windowAction = func(action system.Action) {
		remaining := action
		if action&system.ActionMaximize != 0 {
			window.Option(gioapp.Maximized.Option())
			remaining &^= system.ActionMaximize
		}
		if action&system.ActionUnmaximize != 0 {
			window.Option(gioapp.Windowed.Option())
			remaining &^= system.ActionUnmaximize
		}
		if remaining != 0 {
			window.Perform(remaining)
		}
	}
	redraw := &redrawer{window: window}
	remove := r.hub.Add(redraw.ask)
	defer remove()
	var operations op.Ops
	reissued := false
	preferences := shell.preferences
	preferences.Width, preferences.Height = options.Width, options.Height
	for {
		raw := window.Event()
		switch event := raw.(type) {
		case gioapp.ConfigEvent:
			shell.decorated = event.Config.Decorated
			shell.maximized = event.Config.Mode == gioapp.Maximized
			shell.attention.focused.Store(event.Config.Focused)
			// the compositor may ignore options sent before the first configure so they go out once more after it
			if !reissued {
				reissued = true
				window.Option(creation...)
			}
		case gioapp.FrameEvent:
			redraw.frame()
			shell.posts.Drain()
			if !shell.maximized {
				preferences.Width = int(event.Metric.PxToDp(event.Size.X) + .5)
				preferences.Height = int(event.Metric.PxToDp(event.Size.Y) + .5)
			}
			gtx := gioapp.NewContext(&operations, event)
			shell.applyZoom(&gtx)
			shell.Frame(gtx)
			event.Frame(&operations)
		case gioapp.DestroyEvent:
			_ = state.SaveAppPreferences(shell.persistedPreferences(preferences))
			return event.Err
		}
	}
}

// scales the metric so every dp and sp follows and the size classes see the zoomed width
func (s *uiShell) applyZoom(gtx *layout.Context) {
	scale := float32(s.preferences.ZoomPercent) / 100
	gtx.Metric.PxPerDp *= scale
	gtx.Metric.PxPerSp *= scale
}

func (s *uiShell) persistedPreferences(preferences state.AppPreferences) state.AppPreferences {
	preferences.ZoomPercent = s.preferences.ZoomPercent
	preferences.SidebarFolded = s.preferences.SidebarFolded
	preferences.SidePanelOpen = s.preferences.SidePanelOpen
	preferences.ReduceMotion = s.preferences.ReduceMotion
	preferences.Contrast, preferences.Tint = s.preferences.Contrast, s.preferences.Tint
	preferences.MuteNotices = s.preferences.MuteNotices
	preferences.HiddenSteps = s.preferences.HiddenSteps
	preferences.Last = lastPlace(s.router.lastRashin)
	preferences.Teams = lastPlace(s.router.lastTeams)
	preferences.InTeams = s.router.TeamsMode()
	return preferences
}

func (s *uiShell) environment() *env.Env {
	e := &env.Env{
		Theme: s.runtime.Theme(), Store: s.runtime.store, Client: s.runtime.client, Overlays: &s.overlay,
		Navigate: s.navigate, NavigateWork: s.navigateWork, NavigateTeams: s.navigateTeams,
		OpenSettings: s.openSettings, Post: s.posts.Post,
		Class: s.class, PanelOpen: s.preferences.SidePanelOpen, TogglePanel: s.togglePanel,
		Notify: func(text string) { s.placards.Push(s.now, text, "") },
		Zoom:   s.preferences.ZoomPercent, SetZoom: s.setZoom,
		ReduceMotion: s.preferences.ReduceMotion, SetReduceMotion: s.setReduceMotion,
		Notices: !s.preferences.MuteNotices, SetNotices: s.setNotices,
		Desktop: s.desktop,
	}
	e.HiddenSteps, e.HideStep = s.preferences.HiddenSteps, s.hideStep
	e.Contrast, e.Tint, e.SetLook = s.preferences.Contrast, s.preferences.Tint, s.setLook
	// the work page owns the dialog, any page may open it
	e.NewTeamTask = func(team string) { s.work.Ask(e, team) }
	return e
}

func (s *uiShell) page(tab string) env.Page {
	if teamsTab(tab) {
		return s.teamsPage(tab)
	}
	switch tab {
	case protocol.TabAgent:
		return s.agent
	case protocol.TabProjects:
		return s.projects
	case protocol.TabIndex:
		return s.index
	case protocol.TabSettings:
		return s.settings
	default:
		return s.ask
	}
}

func (s *uiShell) navigate(tab, session string) {
	s.router.Navigate(tab, session)
}

func (s *uiShell) navigateWork(project, view, worktree string) {
	s.router.Push(Place{
		Tab: protocol.TabProjects, Project: project, ProjectView: normalizeProjectView(view),
		Worktree: worktree,
	})
}

func (s *uiShell) openSettings(section string) {
	s.router.Push(Place{Tab: protocol.TabSettings, SettingsSection: section})
}

func (s *uiShell) openDestination(tab string) {
	s.router.Push(Place{Tab: tab})
}

func (s *uiShell) applyPlace(place Place) {
	e := s.environment()
	s.sheet.open = false
	if place.Tab != protocol.TabSettings {
		s.listTab = place.Tab
	}
	if place.Tab != protocol.TabProjects {
		s.projects.Hidden(e)
	}
	switch place.Tab {
	case protocol.TabAgent:
		s.agent.Select(e, place.Session)
	case protocol.TabAsk:
		s.ask.Select(e, place.Session)
	case protocol.TabProjects:
		place = s.projectsHome(e, place)
		s.projects.Open(e, place.Project, normalizeProjectView(place.ProjectView), place.Worktree)
	case protocol.TabSettings:
		section := place.SettingsSection
		// from teams mode settings opens on what every team shares, the rest of settings is a scroll away
		if section == "" && s.router.TeamsMode() {
			section = "teams"
		}
		s.settings.OpenSection(section)
	default:
		if teamsTab(place.Tab) {
			s.teamsPage(place.Tab).Select(e, place.teams())
		}
	}
}

// projects has no page above its projects so I swap the bare place for the one it opens at, there's nothing else to go back to
func (s *uiShell) projectsHome(e *env.Env, place Place) Place {
	if place.Project != "" {
		return place
	}
	project, view, worktree, ok := s.projects.Home(e)
	if !ok {
		return place
	}
	place = Place{Tab: protocol.TabProjects, Project: project, ProjectView: view, Worktree: worktree}
	s.router.Replace(place)
	return place
}

func normalizeProjectView(view string) string {
	if view == "" {
		return projectspage.TabOverview
	}
	return view
}

func (s *uiShell) newCurrent(e *env.Env) {
	if s.router.Tab == protocol.TabSettings {
		s.openDestination(s.listTab)
	}
	s.page(s.listTab).New(e)
}

func (s *uiShell) toggleSidebar() {
	if s.class == look.Expanded {
		s.preferences.SidebarFolded = !s.preferences.SidebarFolded
	} else {
		s.sheet.open = !s.sheet.open
	}
	s.runtime.hub.Invalidate()
}

func (s *uiShell) togglePanel() {
	s.preferences.SidePanelOpen = !s.preferences.SidePanelOpen
	s.runtime.hub.Invalidate()
}

func (s *uiShell) canEscape() bool {
	return s.sheet.open || (s.preferences.SidePanelOpen && s.panelSheet())
}

func (s *uiShell) escape() bool {
	switch {
	case s.sheet.open:
		s.sheet.open = false
	case s.preferences.SidePanelOpen && s.panelSheet():
		s.preferences.SidePanelOpen = false
	default:
		return false
	}
	return true
}

func (s *uiShell) changeZoom(delta int) {
	if delta == 0 {
		s.setZoom(state.DefaultZoom)
	} else {
		s.setZoom(s.preferences.ZoomPercent + delta)
	}
}

func (s *uiShell) setZoom(percent int) {
	s.preferences.ZoomPercent = min(state.MaxZoom, max(state.MinZoom, percent))
	s.runtime.hub.Invalidate()
}

func (s *uiShell) setNotices(on bool) {
	s.preferences.MuteNotices = !on
	if s.attention != nil {
		s.attention.muted.Store(!on)
	}
	s.runtime.hub.Invalidate()
}

// a next step hidden on a team's Overview stays hidden; teams that went take their hidden steps with them
func (s *uiShell) hideStep(team, step string) {
	kept := make(map[string][]string, len(s.preferences.HiddenSteps)+1)
	for _, known := range s.runtime.store.Snapshot().Teams {
		if steps, ok := s.preferences.HiddenSteps[known.ID]; ok {
			kept[known.ID] = steps
		}
	}
	for _, hidden := range kept[team] {
		if hidden == step {
			return
		}
	}
	kept[team] = append(kept[team], step)
	s.preferences.HiddenSteps = kept
	s.runtime.hub.Invalidate()
}

func (s *uiShell) setReduceMotion(reduce bool) {
	s.preferences.ReduceMotion = reduce
	anim.SetReduce(reduce)
	s.runtime.hub.Invalidate()
}

// unknown words fall back to Rashin's own look, and the window restyles on the next frame
func (s *uiShell) setLook(contrast, tint string) {
	s.preferences.Contrast, s.preferences.Tint = contrast, tint
	s.preferences = s.preferences.Normalize()
	s.runtime.setLook(s.preferences.Contrast, s.preferences.Tint)
}

// ui.open from the daemon, a second launch, the bar or the command line asking the running app to show something
func (s *uiShell) applyFocus(e *env.Env) {
	focus := e.Snapshot().Focus
	if focus == s.lastFocus {
		if !s.initialApplied && e.Snapshot().Connection == state.Ready {
			s.initialApplied = true
			s.applyPlace(s.router.Current())
		}
		return
	}
	s.lastFocus = focus
	if focus.Project != "" {
		s.navigateWork(focus.Project, focus.View, focus.Worktree)
		return
	}
	if place, ok := focusedTeams(focus); ok {
		s.navigateTeams(place)
		return
	}
	if focus.Tab == "" && focus.Session == "" {
		return
	}
	tab := focus.Tab
	if tab == "" {
		tab = protocol.TabAsk
		// the id says the kind, also for a session the list has not heard of yet
		if kind, ok := protocol.KindOfSession(focus.Session); ok && kind == protocol.KindAgent {
			tab = protocol.TabAgent
		}
	}
	s.navigate(tab, focus.Session)
}

func (s *uiShell) Frame(gtx layout.Context) layout.Dimensions {
	s.now, s.widthDp = gtx.Now, float32(gtx.Metric.PxToDp(gtx.Constraints.Max.X))
	s.class = look.ClassFor(s.widthDp)
	e := s.environment()
	s.router.NeedsYou = s.agent.NeedsYou(e)
	s.router.Inbox = 0
	if s.router.TeamsMode() {
		s.router.Inbox = len(teamsui.Needs(e.Snapshot()))
	}
	// the project list can arrive after projects was opened without one
	if current := s.router.Current(); current.Tab == protocol.TabProjects && current.Project == "" {
		if _, _, _, ok := s.projects.Home(e); ok {
			s.applyPlace(current)
		}
	}
	s.applyFocus(e)
	paint.Fill(gtx.Ops, e.Theme.R.Ground)
	s.router.HandleKeys(gtx, &s.overlay)
	s.syncPalette(gtx, e)

	ready := e.Snapshot().Connection == state.Ready
	if ready {
		s.layoutFrame(gtx, e)
	} else {
		s.layoutNotReady(gtx, e)
	}
	s.overlay.Layout(gtx, e.Theme)
	return layout.Dimensions{Size: gtx.Constraints.Max}
}

type frameRects struct {
	sidebar, content, panel image.Rectangle
}
