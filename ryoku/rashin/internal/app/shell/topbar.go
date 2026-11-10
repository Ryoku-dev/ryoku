package shell

import (
	"image"

	"gioui.org/io/system"
	"gioui.org/layout"
	"gioui.org/op/clip"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
)

// its empty stretches move the window
type topBar struct {
	move                  kit.Click
	toggle, back, forward kit.IconPlaque
	search                kit.IconPlaque
	maximize, close       kit.IconPlaque
	tips                  [6]look.Tip
	searchAnchor          image.Rectangle
	searchShown           bool
	searchFocused         bool
}

type barState struct {
	class look.Class
	// false while Rashin is off or unreachable, the bar then shows only the mark and the window controls
	ready bool
	// left edge of the content pane, where the frame controls start
	content int
	folded  bool
}

func (bar *topBar) Layout(gtx layout.Context, s *uiShell, e *env.Env, state barState) layout.Dimensions {
	size := image.Pt(gtx.Constraints.Max.X, gtx.Dp(look.TopBar))
	gtx.Constraints = layout.Exact(size)
	defer clip.Rect(image.Rectangle{Max: size}).Push(gtx.Ops).Pop()
	th := e.Theme
	bar.handle(gtx, s)

	plaque, top := gtx.Dp(32), (size.Y-gtx.Dp(32))/2
	place := func(x int, button *kit.IconPlaque, tip *look.Tip, enabled bool) {
		look.Place(gtx, image.Rect(x, top, x+plaque, top+plaque), func(gtx layout.Context) layout.Dimensions {
			if !enabled {
				gtx = gtx.Disabled()
			}
			return tip.Layout(gtx, th, e.Overlays, button.Description, func(gtx layout.Context) layout.Dimensions {
				return button.Layout(gtx, th.Theme)
			})
		})
	}
	set := func(button *kit.IconPlaque, description string) {
		button.Kind, button.Small, button.Description = kit.PlaqueQuiet, true, description
	}

	mark := bar.layoutMark(gtx, th, state)
	controls := size.X - gtx.Dp(13)
	if !s.decorated {
		set(&bar.maximize, i18n.T("Maximise"))
		bar.maximize.Icon = look.Icon.Fullscreen
		if s.maximized {
			bar.maximize.Icon, bar.maximize.Description = look.Icon.FullscreenExit, i18n.T("Restore")
		}
		set(&bar.close, i18n.T("Close"))
		bar.close.Icon = look.Icon.Close
		controls -= 2*plaque + gtx.Dp(4)
		place(controls, &bar.maximize, &bar.tips[4], true)
		place(controls+plaque+gtx.Dp(4), &bar.close, &bar.tips[5], true)
	}

	leading, searchEnd := mark, mark
	if state.ready {
		leading, searchEnd = bar.layoutControls(gtx, s, e, state, place, set, controls)
	}
	bar.moveRegion(gtx, image.Rect(0, 0, leading, size.Y))
	bar.moveRegion(gtx, image.Rect(searchEnd, 0, controls-gtx.Dp(8), size.Y))
	return layout.Dimensions{Size: size}
}

func (bar *topBar) layoutMark(gtx layout.Context, th *look.Theme, state barState) int {
	size := gtx.Constraints.Max
	glyph := gtx.Dp(24)
	x := gtx.Dp(21)
	if state.ready && (state.class == look.Medium || state.folded) {
		x = gtx.Dp(look.Margin) + (gtx.Dp(look.SidebarRail)-glyph)/2
	} else if state.class == look.Compact {
		x = gtx.Dp(13)
	}
	look.Place(gtx, image.Rect(x, (size.Y-glyph)/2, x+glyph, (size.Y+glyph)/2), look.RyokuMark(24, th.R.Fg))
	end := x + glyph
	named := !state.ready || (state.class == look.Expanded && !state.folded)
	if named {
		look.Place(gtx, image.Rect(end+gtx.Dp(8), 0, end+gtx.Dp(8)+gtx.Dp(120), size.Y), func(gtx layout.Context) layout.Dimensions {
			return layout.W.Layout(gtx, kit.RawText(th.Theme, th.T.Title, th.R.Fg, i18n.T("Rashin")))
		})
	}
	return end
}

func (bar *topBar) layoutControls(
	gtx layout.Context,
	s *uiShell,
	e *env.Env,
	state barState,
	place func(int, *kit.IconPlaque, *look.Tip, bool),
	set func(*kit.IconPlaque, string),
	controls int,
) (int, int) {
	size := gtx.Constraints.Max
	plaque, step := gtx.Dp(32), gtx.Dp(36)
	x := state.content
	leading := x
	if state.class == look.Compact {
		// the list has no pane here, its toggle next to the mark raises it as a sheet
		set(&bar.toggle, i18n.T("Open the list")+"  Ctrl B")
		bar.toggle.Icon = look.Icon.SidebarToggle
		toggle := gtx.Dp(13) + gtx.Dp(24) + gtx.Dp(13)
		place(toggle, &bar.toggle, &bar.tips[0], true)
		set(&bar.search, i18n.T("Search"))
		bar.search.Icon = look.Icon.Search
		x = controls - gtx.Dp(8) - plaque
		if !bar.searchShown {
			bar.searchFocused = false
			place(x, &bar.search, &bar.tips[3], true)
			bar.searchAnchor = image.Rect(gtx.Dp(look.Margin), size.Y, size.X-gtx.Dp(look.Margin), size.Y)
			bar.moveRegion(gtx, image.Rect(toggle+plaque, 0, x, size.Y))
			return toggle, x + plaque
		}
		field := image.Rect(toggle+plaque+gtx.Dp(8), (size.Y-gtx.Dp(44))/2, controls-gtx.Dp(8), (size.Y+gtx.Dp(44))/2)
		bar.layoutSearch(gtx, s, e, field)
		return toggle, field.Max.X
	}
	set(&bar.toggle, i18n.T("Fold the sidebar")+"  Ctrl B")
	bar.toggle.Icon = look.Icon.SidebarToggle
	if state.folded || state.class == look.Medium {
		bar.toggle.Description = i18n.T("Open the sidebar") + "  Ctrl B"
	}
	set(&bar.back, i18n.T("Back")+"  Alt Left")
	bar.back.Icon = look.Icon.Back
	set(&bar.forward, i18n.T("Forward")+"  Alt Right")
	bar.forward.Icon = look.Icon.Forward
	place(x, &bar.toggle, &bar.tips[0], true)
	place(x+step, &bar.back, &bar.tips[1], s.router.CanBack())
	place(x+2*step, &bar.forward, &bar.tips[2], s.router.CanForward())
	x += 2*step + plaque + gtx.Dp(13)
	field := image.Rect(x, (size.Y-gtx.Dp(44))/2, min(x+gtx.Dp(520), controls-gtx.Dp(21)), (size.Y+gtx.Dp(44))/2)
	if field.Dx() < gtx.Dp(160) {
		return leading, x
	}
	bar.layoutSearch(gtx, s, e, field)
	return leading, field.Max.X
}

func (bar *topBar) layoutSearch(gtx layout.Context, s *uiShell, e *env.Env, field image.Rectangle) {
	bar.searchAnchor = field
	look.Place(gtx, field, func(gtx layout.Context) layout.Dimensions {
		s.palette.search.Placeholder = i18n.T("Search sessions, projects and commands")
		s.palette.search.Hint = "Ctrl K"
		return s.palette.search.Layout(gtx, e.Theme.Theme)
	})
	// focus arriving opens the results, focus that's just still there after they closed doesn't
	focused := gtx.Focused(&s.palette.search.Editor)
	if focused && !bar.searchFocused && !s.router.PaletteOpen {
		s.router.OpenPalette()
	}
	bar.searchFocused = focused
}

func (bar *topBar) moveRegion(gtx layout.Context, rectangle image.Rectangle) {
	look.Place(gtx, rectangle, func(gtx layout.Context) layout.Dimensions {
		return bar.move.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			defer clip.Rect(image.Rectangle{Max: gtx.Constraints.Max}).Push(gtx.Ops).Pop()
			system.ActionInputOp(system.ActionMove).Add(gtx.Ops)
			return layout.Dimensions{Size: gtx.Constraints.Max}
		})
	})
}

func (bar *topBar) handle(gtx layout.Context, s *uiShell) {
	toggleMaximise := func() {
		if s.windowAction == nil {
			return
		}
		if s.maximized {
			s.windowAction(system.ActionUnmaximize)
		} else {
			s.windowAction(system.ActionMaximize)
		}
	}
	for {
		click, ok := bar.move.Update(gtx)
		if !ok {
			break
		}
		if click.NumClicks >= 2 {
			toggleMaximise()
		}
	}
	if bar.toggle.Clicked(gtx) {
		s.toggleSidebar()
	}
	if bar.back.Clicked(gtx) {
		s.router.Back()
	}
	if bar.forward.Clicked(gtx) {
		s.router.Forward()
	}
	if bar.search.Clicked(gtx) {
		bar.searchShown = true
		s.router.OpenPalette()
		s.palette.search.Focus(gtx)
	}
	if bar.maximize.Clicked(gtx) {
		toggleMaximise()
	}
	if bar.close.Clicked(gtx) && s.windowAction != nil {
		s.windowAction(system.ActionClose)
	}
}
