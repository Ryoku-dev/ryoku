package shell

import (
	"image"
	"strings"

	"gioui.org/layout"
	"gioui.org/op"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/protocol"
)

type paletteAction struct {
	label string
	run   func()
}

type commandPalette struct {
	search kit.Search
	menu   kit.Menu
}

func (s *uiShell) openPalette(gtx layout.Context, e *env.Env) {
	actions := s.paletteActions(e)
	content := func(gtx layout.Context) layout.Dimensions {
		s.palette.search.Placeholder = i18n.T("Find an action")
		children := []layout.FlexChild{
			layout.Rigid(func(gtx layout.Context) layout.Dimensions { return s.palette.search.Layout(gtx, e.Theme.Theme) }),
			layout.Rigid(layout.Spacer{Height: 8}.Layout),
		}
		query := strings.ToLower(strings.TrimSpace(s.palette.search.Text()))
		filtered := make([]paletteAction, 0, len(actions))
		for _, action := range actions {
			if query == "" || strings.Contains(strings.ToLower(action.label), query) {
				filtered = append(filtered, action)
			}
		}
		items := make([]kit.MenuItem, len(filtered))
		for index, action := range filtered {
			items[index] = kit.MenuItem{Label: action.label}
		}
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			dimensions := s.palette.menu.Layout(gtx, e.Theme.Theme, items)
			if index, clicked := s.palette.menu.Clicked(gtx); clicked && index < len(filtered) {
				filtered[index].run()
				s.router.PaletteOpen = false
				s.overlay.Close(&s.palette)
				gtx.Execute(op.InvalidateCmd{})
			}
			return dimensions
		}))
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
	}
	window := s.overlay.Window()
	if window.X == 0 {
		window = gtx.Constraints.Max
	}
	anchor := image.Rect(max(8, window.X/2-280), 72, max(8, window.X/2-280), 72)
	s.overlay.Open(&s.palette, anchor, look.Placement{Width: 560, Align: look.Start}, content)
	s.palette.search.Focus(gtx)
}

func (s *uiShell) paletteActions(e *env.Env) []paletteAction {
	return []paletteAction{
		{label: i18n.T("New ask"), run: func() { s.router.Navigate(protocol.TabAsk, ""); s.ask.NewSession(e) }},
		{label: i18n.T("New agent session"), run: func() { s.router.Navigate(protocol.TabAgent, ""); s.agent.NewSession(e) }},
		{label: i18n.T("Open Ask"), run: func() { s.router.Navigate(protocol.TabAsk, "") }},
		{label: i18n.T("Open Agent"), run: func() { s.router.Navigate(protocol.TabAgent, "") }},
		{label: i18n.T("Open Index"), run: func() { s.router.Navigate(protocol.TabIndex, "") }},
		{label: i18n.T("Open Settings"), run: func() { s.router.Navigate(protocol.TabSettings, "") }},
		{label: i18n.T("Open provider settings"), run: func() { s.router.Navigate(protocol.TabSettings, ""); s.settings.OpenSection(0) }},
		{label: i18n.T("Open Ask settings"), run: func() { s.router.Navigate(protocol.TabSettings, ""); s.settings.OpenSection(1) }},
		{label: i18n.T("Open Agent settings"), run: func() { s.router.Navigate(protocol.TabSettings, ""); s.settings.OpenSection(2) }},
		{label: i18n.T("Open Index settings"), run: func() { s.router.Navigate(protocol.TabSettings, ""); s.settings.OpenSection(3) }},
		{label: i18n.T("Open About settings"), run: func() { s.router.Navigate(protocol.TabSettings, ""); s.settings.OpenSection(4) }},
		{label: i18n.T("Focus search"), run: func() { s.router.Navigate(protocol.TabIndex, ""); s.index.FocusSearch() }},
		{label: i18n.T("Turn Rashin off"), run: func() { env.Spawn("rashin", "off") }},
	}
}
