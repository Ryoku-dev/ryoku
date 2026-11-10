package shell

import (
	"os/exec"
	"sync"

	gioapp "gioui.org/app"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/paint"
	"gioui.org/unit"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
	agentpage "ryoku-rashin/internal/app/pages/agent"
	askpage "ryoku-rashin/internal/app/pages/ask"
	indexpage "ryoku-rashin/internal/app/pages/index"
	projectspage "ryoku-rashin/internal/app/pages/projects"
	settingspage "ryoku-rashin/internal/app/pages/settings"
	"ryoku-rashin/internal/app/state"
	"ryoku-rashin/internal/paths"
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
	runtime        *runtime
	router         *Router
	posts          postQueue
	overlay        look.Overlays
	ask            askpage.Page
	agent          agentpage.Page
	projects       *projectspage.Page
	index          indexpage.Page
	settings       settingspage.Page
	palette        commandPalette
	turnOn         kit.Plaque
	retry          kit.Plaque
	lastFocus      protocol.UIOpenParams
	initialApplied bool
}

func newShell(r *runtime, options Options) *uiShell {
	route := options.Route
	if options.Project != "" {
		route = protocol.TabProjects
	}
	s := &uiShell{runtime: r, router: NewRouter(route, options.Session, r.hub.Invalidate), projects: projectspage.New()}
	s.router.paletteTag = &s.palette
	s.posts.invalidate = r.hub.Invalidate
	if options.SettingsSection >= 0 {
		s.settings.OpenSection(options.SettingsSection)
	}
	if options.Project != "" {
		s.navigateWork(options.Project, options.WorkTab, options.Worktree)
	}
	return s
}

func (r *runtime) runWindow(options Options) error {
	shell := newShell(r, options)
	window := new(gioapp.Window)
	creation := []gioapp.Option{
		gioapp.Title(i18n.T("Rashin")),
		gioapp.Size(unit.Dp(options.Width), unit.Dp(options.Height)),
		gioapp.MinSize(640, 520),
		gioapp.MaxSize(1048576, 1048576),
	}
	window.Option(creation...)
	remove := r.hub.Add(window.Invalidate)
	defer remove()
	var operations op.Ops
	reissued := false
	last := windowState{Width: options.Width, Height: options.Height}
	for {
		raw := window.Event()
		switch event := raw.(type) {
		case gioapp.ConfigEvent:
			if !reissued {
				reissued = true
				window.Option(creation...)
			}
		case gioapp.FrameEvent:
			shell.posts.Drain()
			last.Width = int(event.Metric.PxToDp(event.Size.X) + .5)
			last.Height = int(event.Metric.PxToDp(event.Size.Y) + .5)
			gtx := gioapp.NewContext(&operations, event)
			shell.Frame(gtx)
			event.Frame(&operations)
		case gioapp.DestroyEvent:
			_ = saveWindowState(last)
			return event.Err
		}
	}
}

func (s *uiShell) environment() *env.Env {
	return &env.Env{
		Theme: s.runtime.Theme(), Store: s.runtime.store, Client: s.runtime.client, Overlays: &s.overlay,
		Navigate: s.navigate, NavigateWork: s.navigateWork, Post: s.posts.Post,
	}
}

func (s *uiShell) navigate(tab, session string) {
	s.router.Navigate(tab, session)
	if session == "" {
		return
	}
	e := s.environment()
	switch tab {
	case protocol.TabAgent:
		s.agent.Select(e, session)
	default:
		s.ask.Select(e, session)
	}
}

func (s *uiShell) navigateWork(project, tab, worktree string) {
	s.router.Navigate(protocol.TabProjects, "")
	s.projects.Open(s.environment(), project, normalizeProjectView(tab), worktree)
}

func normalizeProjectView(view string) string {
	if view == "" {
		return projectspage.TabOverview
	}
	return view
}

func (s *uiShell) Frame(gtx layout.Context) layout.Dimensions {
	e := s.environment()
	s.router.NeedsYou = s.agent.NeedsYou(e)
	s.applyFocus(e)
	width := gtx.Metric.PxToDp(gtx.Constraints.Max.X)
	class := kit.ClassForWidth(width)
	paint.Fill(gtx.Ops, e.Theme.R.Ground)
	s.router.HandleKeys(gtx, &s.overlay)
	if s.router.PaletteOpen && !s.overlay.IsOpen(&s.palette) {
		s.openPalette(gtx, e)
	}
	if !s.router.PaletteOpen && s.overlay.IsOpen(&s.palette) {
		s.overlay.Close(&s.palette)
	}
	if class == kit.FrameCompact {
		s.layoutCompact(gtx, e, class)
	} else {
		s.layoutWide(gtx, e, class)
	}
	s.layoutPageOverlays(gtx, e)
	s.overlay.Layout(gtx, e.Theme)
	return layout.Dimensions{Size: gtx.Constraints.Max}
}

func (s *uiShell) layoutWide(gtx layout.Context, e *env.Env, class kit.FrameClass) {
	railWidth := gtx.Dp(128)
	if class == kit.FrameExpanded {
		railWidth = gtx.Dp(168)
	}
	layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min.X, gtx.Constraints.Max.X = railWidth, railWidth
			return layout.Inset{Top: 10}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return (kit.Pane{Kind: kit.PaneList}).Layout(gtx, e.Theme.Theme, func(gtx layout.Context) layout.Dimensions {
					return s.router.LayoutRail(gtx, e.Theme, class)
				})
			})
		}),
		layout.Rigid(layout.Spacer{Width: 8}.Layout),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: 10}.Layout(gtx, func(gtx layout.Context) layout.Dimensions { return s.layoutPage(gtx, e, class) })
		}),
	)
}

func (s *uiShell) layoutCompact(gtx layout.Context, e *env.Env, class kit.FrameClass) {
	barHeight, gap := gtx.Dp(64), gtx.Dp(8)
	layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: 10, Bottom: unit.Dp(float32(gap) / gtx.Metric.PxPerDp)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions { return s.layoutPage(gtx, e, class) })
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min.Y, gtx.Constraints.Max.Y = barHeight, barHeight
			return s.router.LayoutRail(gtx, e.Theme, class)
		}),
	)
}

func (s *uiShell) layoutPage(gtx layout.Context, e *env.Env, class kit.FrameClass) layout.Dimensions {
	connection := e.Snapshot().Connection
	if connection != state.Ready {
		return s.layoutConnection(gtx, e, connection)
	}
	if s.router.Tab != protocol.TabProjects {
		s.projects.Hidden(e)
	}
	switch s.router.Tab {
	case protocol.TabAgent:
		return s.agent.Layout(gtx, e, class)
	case protocol.TabProjects:
		return s.projects.Layout(gtx, e, class)
	case protocol.TabIndex:
		return s.index.Layout(gtx, e, class)
	case protocol.TabSettings:
		return s.settings.Layout(gtx, e, class)
	default:
		return s.ask.Layout(gtx, e, class)
	}
}

func (s *uiShell) layoutConnection(gtx layout.Context, e *env.Env, connection state.Connection) layout.Dimensions {
	if s.turnOn.Clicked(gtx) {
		go func() {
			_ = exec.Command("rashin", "on").Run()
			s.runtime.client.Reconnect()
		}()
	}
	if s.retry.Clicked(gtx) {
		s.runtime.client.Reconnect()
	}
	var empty kit.Empty
	switch connection {
	case state.Off:
		s.turnOn.Kind, s.turnOn.Label = kit.PlaqueGold, i18n.T("Turn on")
		empty = kit.Empty{Title: i18n.T("Rashin is off"), Body: i18n.T("Turn it on to use Ask, Agent and the code index."), Actions: []*kit.Plaque{&s.turnOn}}
	case state.Unreachable:
		s.retry.Kind, s.retry.Label = kit.PlaqueGold, i18n.T("Try again")
		empty = kit.Empty{Title: i18n.T("Rashin is unreachable"), Body: i18n.Tf("The daemon did not answer. Check %s, then try again.", paths.DaemonLog()), Actions: []*kit.Plaque{&s.retry}}
	default:
		empty = kit.Empty{Title: i18n.T("Connecting to Rashin"), Body: i18n.T("Your sessions will appear when the daemon answers.")}
	}
	return (kit.Pane{Kind: kit.PaneWork}).Layout(gtx, e.Theme.Theme, func(gtx layout.Context) layout.Dimensions { return empty.Layout(gtx, e.Theme.Theme) })
}

func (s *uiShell) layoutPageOverlays(gtx layout.Context, e *env.Env) {
	s.agent.Overlay(gtx, e)
	s.ask.Overlay(gtx, e)
	s.index.Overlay(gtx, e)
}

func (s *uiShell) applyFocus(e *env.Env) {
	focus := e.Snapshot().Focus
	if focus == s.lastFocus {
		if !s.initialApplied && s.router.Session != "" && e.Snapshot().Connection == state.Ready {
			s.initialApplied = true
			s.navigate(s.router.Tab, s.router.Session)
		}
		return
	}
	s.lastFocus = focus
	if focus.Project != "" {
		s.navigateWork(focus.Project, focus.View, focus.Worktree)
		return
	}
	if focus.Tab != "" || focus.Session != "" {
		tab := focus.Tab
		if tab == "" {
			if info, ok := e.Snapshot().Sessions[focus.Session]; ok && info.Kind == protocol.KindAgent {
				tab = protocol.TabAgent
			} else {
				tab = protocol.TabAsk
			}
		}
		s.navigate(tab, focus.Session)
	}
}
