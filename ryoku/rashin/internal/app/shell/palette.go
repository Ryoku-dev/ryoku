package shell

import (
	"strings"

	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/widget"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
	projectspage "ryoku-rashin/internal/app/pages/projects"
	"ryoku-rashin/internal/protocol"
)

// keeps the results a list the eye can take in at once
const paletteLimit = 9

type paletteAction struct {
	icon  *widget.Icon
	label string
	kind  string
	run   func()
	// title of the run this result belongs to, results that answer a query have none
	group string
}

type commandPalette struct {
	search kit.Search
	list   paletteList
	cursor int
	query  string
	// what the list held while open, so it leaves with those and not with what an emptied field would give
	shown []paletteAction
	// last frame's focus and open state, so a change in either is seen once
	focused, open bool
}

// takes the keys that move through the results before the field's editor sees them
func (s *uiShell) syncPalette(gtx layout.Context, e *env.Env) {
	editor := &s.palette.search.Editor
	focused := gtx.Focused(editor)
	// closes on the router's escape or on focus leaving the field, either way the field empties
	if (s.palette.open && !s.router.PaletteOpen) || (s.router.PaletteOpen && !focused && s.palette.focused) {
		s.closePalette(gtx)
	}
	s.palette.focused, s.palette.open = focused, s.router.PaletteOpen
	if s.router.PaletteOpen {
		if !focused {
			s.palette.search.Focus(gtx)
		}
		s.handlePaletteKeys(gtx, e)
	}
	open := s.overlay.IsOpen(&s.palette)
	switch {
	case s.router.PaletteOpen && !open:
		s.openPalette(gtx, e)
	case !s.router.PaletteOpen && open:
		s.overlay.Close(&s.palette)
	}
}

func (s *uiShell) closePalette(gtx layout.Context) {
	s.router.PaletteOpen = false
	s.topBar.searchShown = false
	s.palette.search.SetText("")
	s.overlay.Close(&s.palette)
	// a press somewhere else already moved the keyboard there, only a field that still holds it gives it back
	if gtx.Focused(&s.palette.search.Editor) {
		gtx.Execute(key.FocusCmd{Tag: s.focusHome()})
	}
	gtx.Execute(op.InvalidateCmd{})
}

// nil when the page shown has no place the keyboard belongs
func (s *uiShell) focusHome() any {
	if homed, ok := s.page(s.router.Tab).(env.Homed); ok {
		return homed.FocusHome()
	}
	return nil
}

func (s *uiShell) openPalette(gtx layout.Context, e *env.Env) {
	content := func(gtx layout.Context) layout.Dimensions {
		if s.router.PaletteOpen {
			s.palette.shown = s.filteredPaletteActions(e)
		}
		actions := s.palette.shown
		s.palette.cursor = min(s.palette.cursor, max(0, len(actions)-1))
		dimensions, pressed, entered := s.palette.list.Layout(gtx, e.Theme, actions, s.palette.cursor)
		if entered >= 0 {
			s.palette.cursor = entered
		}
		if pressed >= 0 {
			s.runPaletteAction(gtx, actions[pressed])
		}
		return dimensions
	}
	anchor := s.topBar.searchAnchor
	s.overlay.Open(&s.palette, anchor, look.Placement{
		Prefer: look.Below, Align: look.Start, Gap: 4, Width: gtx.Metric.PxToDp(anchor.Dx()), Passive: true, Frame: true,
	}, content)
}

func (s *uiShell) handlePaletteKeys(gtx layout.Context, e *env.Env) {
	if query := strings.TrimSpace(s.palette.search.Text()); query != s.palette.query {
		s.palette.query, s.palette.cursor = query, 0
	}
	editor := &s.palette.search.Editor
	filters := []event.Filter{
		key.Filter{Focus: editor, Name: key.NameUpArrow}, key.Filter{Focus: editor, Name: key.NameDownArrow},
		key.Filter{Focus: editor, Name: key.NameReturn}, key.Filter{Focus: editor, Name: key.NameEnter},
		key.Filter{Focus: editor, Name: key.NameEscape},
	}
	for {
		raw, ok := gtx.Event(filters...)
		if !ok {
			return
		}
		pressed, ok := raw.(key.Event)
		if !ok || pressed.State != key.Press {
			continue
		}
		actions := s.filteredPaletteActions(e)
		switch pressed.Name {
		case key.NameUpArrow:
			if len(actions) > 0 {
				s.palette.cursor = (s.palette.cursor + len(actions) - 1) % len(actions)
			}
		case key.NameDownArrow:
			if len(actions) > 0 {
				s.palette.cursor = (s.palette.cursor + 1) % len(actions)
			}
		case key.NameReturn, key.NameEnter:
			if s.palette.cursor < len(actions) {
				s.runPaletteAction(gtx, actions[s.palette.cursor])
			}
		case key.NameEscape:
			s.closePalette(gtx)
		}
	}
}

func (s *uiShell) runPaletteAction(gtx layout.Context, action paletteAction) {
	s.closePalette(gtx)
	action.run()
}

func (s *uiShell) filteredPaletteActions(e *env.Env) []paletteAction {
	query := strings.ToLower(strings.TrimSpace(s.palette.search.Text()))
	if query == "" {
		recent := titled(s.recentPlaces(e), i18n.T("Recent"))
		return append(recent, titled(s.commands(e)[:4], i18n.T("Commands"))...)
	}
	candidates := append(s.places(e), s.commands(e)...)
	filtered := make([]paletteAction, 0, paletteLimit)
	for _, action := range candidates {
		if len(filtered) == paletteLimit {
			break
		}
		if strings.Contains(strings.ToLower(action.label), query) {
			filtered = append(filtered, action)
		}
	}
	return filtered
}

func titled(actions []paletteAction, title string) []paletteAction {
	for index := range actions {
		actions[index].group = title
	}
	return actions
}

func (s *uiShell) commands(e *env.Env) []paletteAction {
	command := i18n.T("Command")
	action := func(icon *widget.Icon, label, kind string, run func()) paletteAction {
		return paletteAction{icon: icon, label: label, kind: kind, run: run}
	}
	return []paletteAction{
		action(look.Icon.Add, i18n.T("New ask"), "Ctrl N", func() { s.ask.New(e) }),
		action(look.Icon.Add, i18n.T("New agent session"), command, func() { s.openDestination(protocol.TabAgent); s.agent.New(e) }),
		action(look.Icon.Add, i18n.T("Add project"), command, func() { s.openDestination(protocol.TabProjects); s.projects.New(e) }),
		action(look.Icon.Settings, i18n.T("Open Settings"), "Ctrl ,", func() { s.openDestination(protocol.TabSettings) }),
		action(look.Icon.Key, i18n.T("Providers and keys"), command, func() { s.openSettings("providers") }),
		action(look.Icon.Link, i18n.T("Local API address and key"), command, func() { s.openSettings("api") }),
		action(look.Icon.Steps, i18n.T("Keyboard shortcuts"), command, func() { s.openSettings("keys") }),
		action(look.Icon.SidebarToggle, i18n.T("Fold or open the sidebar"), "Ctrl B", s.toggleSidebar),
		action(look.Icon.Search, i18n.T("Search the index"), command, func() { s.openDestination(protocol.TabIndex) }),
		action(look.Icon.Add, i18n.T("Zoom in"), "Ctrl +", func() { s.changeZoom(10) }),
		action(look.Icon.Undo, i18n.T("Reset zoom"), "Ctrl 0", func() { s.changeZoom(0) }),
		// it stops every agent, so it goes through the same question as the switch in Settings
		action(look.Icon.Close, i18n.T("Turn Rashin off"), command, func() {
			s.openSettings("rashin")
			s.settings.AskTurnOff()
		}),
	}
}

func (s *uiShell) recentPlaces(e *env.Env) []paletteAction {
	actions := make([]paletteAction, 0, 5)
	seen := map[string]bool{s.router.Current().key(): true}
	for index := s.router.historyAt - 1; index >= 0 && len(actions) < 5; index-- {
		place := s.router.history[index]
		if seen[place.key()] {
			continue
		}
		seen[place.key()] = true
		if action, ok := s.placeAction(e, place); ok {
			actions = append(actions, action)
		}
	}
	return actions
}

func (s *uiShell) placeAction(e *env.Env, place Place) (paletteAction, bool) {
	snapshot := e.Snapshot()
	switch {
	case place.Session != "":
		info, ok := snapshot.Sessions[place.Session]
		if !ok {
			return paletteAction{}, false
		}
		return s.sessionAction(info), true
	case place.Project != "":
		for _, project := range snapshot.Projects {
			if project.ID == place.Project {
				run := func() { s.router.Push(place) }
				return paletteAction{icon: look.Icon.Projects, label: project.Name, kind: i18n.T("Project"), run: run}, true
			}
		}
		return paletteAction{}, false
	}
	for _, item := range destinations() {
		if item.tab == place.Tab {
			return paletteAction{icon: item.icon, label: item.label, run: func() { s.openDestination(place.Tab) }}, true
		}
	}
	return paletteAction{}, false
}

func (s *uiShell) sessionAction(info protocol.SessionInfo) paletteAction {
	tab, icon, kind := protocol.TabAsk, look.Icon.Ask, i18n.T("Ask")
	if info.Kind == protocol.KindAgent {
		tab, icon, kind = protocol.TabAgent, look.Icon.Agents, i18n.T("Agent")
	}
	title := info.Title
	if title == "" {
		title = i18n.T("Untitled")
	}
	return paletteAction{icon: icon, label: title, kind: kind, run: func() { s.navigate(tab, info.ID) }}
}

func (s *uiShell) places(e *env.Env) []paletteAction {
	snapshot := e.Snapshot()
	actions := make([]paletteAction, 0, len(snapshot.Order)+len(snapshot.Projects)*4)
	for _, id := range snapshot.Order {
		if info, ok := snapshot.Sessions[id]; ok {
			actions = append(actions, s.sessionAction(info))
		}
	}
	for _, project := range snapshot.Projects {
		id := project.ID
		actions = append(actions, paletteAction{
			icon: look.Icon.Projects, label: project.Name, kind: i18n.T("Project"),
			run: func() { s.navigateWork(id, projectspage.TabOverview, "") },
		})
		for _, worktree := range snapshot.Worktrees[id] {
			path := worktree.Path
			actions = append(actions, paletteAction{
				icon: look.Icon.Branch, label: project.Name + " / " + worktree.Name, kind: i18n.T("Worktree"),
				run: func() { s.navigateWork(id, projectspage.TabOverview, path) },
			})
		}
	}
	return actions
}
