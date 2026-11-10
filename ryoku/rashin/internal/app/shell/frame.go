package shell

import (
	"image"
	"image/color"

	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/anim"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/app/state"
	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/protocol"
)

type sideSheet struct {
	open   bool
	motion anim.Tween
	scrim  kit.Click
}

func (s *uiShell) layoutFrame(gtx layout.Context, e *env.Env) {
	th := e.Theme
	size := gtx.Constraints.Max
	margin, gap, top := gtx.Dp(look.Margin), gtx.Dp(look.Gap), gtx.Dp(look.TopBar)
	bottom := size.Y - margin
	if s.class == look.Compact {
		bottom -= gtx.Dp(look.BottomNav) + gap
	}
	page, listPage := s.page(s.router.Tab), s.page(s.listTab)

	fold, sidebar := float32(1), 0
	switch s.class {
	case look.Expanded:
		fold = look.Transition(gtx, &s.fold, s.preferences.SidebarFolded, th.Motion.Unfold, th.Motion.Unfold)
		sidebar = gtx.Dp(look.SidebarExpanded) + int(float32(gtx.Dp(look.SidebarRail)-gtx.Dp(look.SidebarExpanded))*fold+.5)
		s.sheet.open = false
	case look.Medium:
		sidebar = gtx.Dp(look.SidebarRail)
	}

	wantPanel, docks := s.panelState(e, page)
	panel := look.Transition(gtx, &s.panel, wantPanel && docks, th.Motion.Unfold, th.Motion.Unfold)
	panelWidth := int(float32(gtx.Dp(look.Panel)+gap)*panel + .5)

	rects := frameRects{sidebar: image.Rect(margin, top, margin+sidebar, bottom)}
	contentLeft := margin
	if sidebar > 0 {
		contentLeft += sidebar + gap
	}
	contentRight := max(contentLeft, size.X-margin-panelWidth)
	rects.content = image.Rect(contentLeft, top, contentRight, bottom)
	rects.panel = image.Rect(contentRight+gap, top, contentRight+gap+gtx.Dp(look.Panel), bottom)

	if sidebar > 0 {
		layoutPane(gtx, th, rects.sidebar, th.R.Stone1, func(gtx layout.Context) layout.Dimensions {
			return s.sidebar.Layout(gtx, s, e, listPage, fold)
		})
	}
	layoutPane(gtx, th, rects.content, th.R.Stone2, func(gtx layout.Context) layout.Dimensions {
		return s.layoutContent(gtx, e, page)
	})
	if panelWidth > gap {
		s.layoutDockedPanel(gtx, e, page, rects.panel, size.X-margin)
	}
	if wantPanel && !docks {
		s.layoutPanelSheet(gtx, e, page, image.Rect(margin, top, size.X-margin, bottom))
	}
	if s.class != look.Expanded {
		s.layoutSidebarSheet(gtx, e, listPage, image.Rect(margin, top, size.X-margin, bottom))
	}
	if s.class == look.Compact {
		bar := image.Rect(margin, bottom+gap, size.X-margin, size.Y-margin)
		layoutPane(gtx, th, bar, th.R.Stone1, func(gtx layout.Context) layout.Dimensions {
			return s.bottom.Layout(gtx, s, e)
		})
	}
	look.Place(gtx, image.Rect(0, 0, size.X, top), func(gtx layout.Context) layout.Dimensions {
		return s.topBar.Layout(gtx, s, e, barState{
			class: s.class, ready: true, content: rects.content.Min.X,
			folded: s.class == look.Expanded && s.preferences.SidebarFolded,
		})
	})
	modal := false
	for _, tab := range []string{protocol.TabAsk, protocol.TabAgent, protocol.TabProjects, protocol.TabIndex, protocol.TabSettings} {
		modal = s.page(tab).Overlay(gtx, e) || modal
	}
	// a dialog that closed leaves the keyboard nowhere, so it goes back to where the page is typed into
	if home := s.focusHome(); s.router.Modal && !modal && home != nil {
		gtx.Execute(key.FocusCmd{Tag: home})
	}
	s.router.Modal = modal
}

func layoutPane(gtx layout.Context, th *look.Theme, rectangle image.Rectangle, plane color.NRGBA, body layout.Widget) {
	if rectangle.Dx() <= 0 || rectangle.Dy() <= 0 {
		return
	}
	offset := op.Offset(rectangle.Min).Push(gtx.Ops)
	bounds := image.Rectangle{Max: rectangle.Size()}
	cut := gtx.Dp(th.Cut.Large)
	area := clip.Outline{Path: kit.CutPath(gtx.Ops, bounds, cut, cut, 0, 0)}.Op().Push(gtx.Ops)
	paint.Fill(gtx.Ops, plane)
	child := gtx
	child.Constraints = layout.Exact(bounds.Max)
	body(child)
	area.Pop()
	offset.Pop()
}

func (s *uiShell) layoutContent(gtx layout.Context, e *env.Env, page env.Page) layout.Dimensions {
	dimensions := s.rise.Layout(gtx, e.Theme, s.router.Current().arrival(), 8, func(gtx layout.Context) layout.Dimensions {
		return page.Content(gtx, e)
	})
	s.placards.Layout(gtx, e.Theme)
	return dimensions
}

func (s *uiShell) panelState(e *env.Env, page env.Page) (want, docks bool) {
	paneled, ok := page.(env.Paneled)
	if !ok || !paneled.HasPanel(e) || !s.preferences.SidePanelOpen {
		return false, false
	}
	return true, s.panelDocks()
}

func (s *uiShell) panelDocks() bool {
	return panelDockEligible(s.widthDp, s.class, s.preferences.SidebarFolded)
}

func (s *uiShell) panelSheet() bool { return !s.panelDocks() }

func (s *uiShell) layoutDockedPanel(gtx layout.Context, e *env.Env, page env.Page, rectangle image.Rectangle, right int) {
	paneled, ok := page.(env.Paneled)
	if !ok {
		return
	}
	// the panel keeps its full width while the pane unfolds so content is revealed, not reflowed
	area := clip.Rect(image.Rect(rectangle.Min.X, rectangle.Min.Y, right, rectangle.Max.Y)).Push(gtx.Ops)
	layoutPane(gtx, e.Theme, rectangle, e.Theme.R.Stone1, func(gtx layout.Context) layout.Dimensions {
		return paneled.Panel(gtx, e)
	})
	area.Pop()
}

func (sheet *sideSheet) layoutScrim(gtx layout.Context, th *look.Theme, region image.Rectangle, progress float32) bool {
	closed := sheet.scrim.Clicked(gtx)
	look.Place(gtx, region, func(gtx layout.Context) layout.Dimensions {
		return sheet.scrim.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			paint.FillShape(gtx.Ops, palette.WithAlpha(th.R.Scrim, progress), clip.Rect(image.Rectangle{Max: gtx.Constraints.Max}).Op())
			return layout.Dimensions{Size: gtx.Constraints.Max}
		})
	})
	return closed
}

func (s *uiShell) layoutSidebarSheet(gtx layout.Context, e *env.Env, listPage env.Page, region image.Rectangle) {
	th := e.Theme
	progress := look.Transition(gtx, &s.sheet.motion, s.sheet.open, th.Motion.DialogIn, th.Motion.DialogOut)
	if progress <= 0 {
		return
	}
	if s.sheet.layoutScrim(gtx, th, region, progress) {
		s.sheet.open = false
	}
	width := min(gtx.Dp(look.SidebarExpanded), region.Dx()-gtx.Dp(34))
	travel := int(float32(gtx.Dp(12)) * (1 - progress))
	rectangle := image.Rect(region.Min.X-travel, region.Min.Y, region.Min.X-travel+width, region.Max.Y)
	opacity := paint.PushOpacity(gtx.Ops, progress)
	layoutPane(gtx, th, rectangle, th.R.Stone1, func(gtx layout.Context) layout.Dimensions {
		if !s.sheet.open {
			gtx = gtx.Disabled()
		}
		s.sheetBar.bare = s.class == look.Compact
		return s.sheetBar.Layout(gtx, s, e, listPage, 0)
	})
	opacity.Pop()
}

func (s *uiShell) layoutPanelSheet(gtx layout.Context, e *env.Env, page env.Page, region image.Rectangle) {
	paneled, ok := page.(env.Paneled)
	if !ok {
		return
	}
	th := e.Theme
	if s.panelScrim.layoutScrim(gtx, th, region, 1) {
		s.preferences.SidePanelOpen = false
	}
	width := min(gtx.Dp(look.Panel), region.Dx()-gtx.Dp(34))
	rectangle := image.Rect(region.Max.X-width, region.Min.Y, region.Max.X, region.Max.Y)
	layoutPane(gtx, th, rectangle, th.R.Stone1, func(gtx layout.Context) layout.Dimensions {
		return paneled.Panel(gtx, e)
	})
}

func (s *uiShell) layoutNotReady(gtx layout.Context, e *env.Env) {
	th := e.Theme
	size := gtx.Constraints.Max
	margin, top := gtx.Dp(look.Margin), gtx.Dp(look.TopBar)
	connection := e.Snapshot().Connection
	if s.turnOn.Clicked(gtx) {
		e.Rashin(func(error) { s.runtime.client.Reconnect() }, "on")
	}
	if s.retry.Clicked(gtx) {
		s.runtime.client.Reconnect()
	}
	layoutPane(gtx, th, image.Rect(margin, top, size.X-margin, size.Y-margin), th.R.Stone2, func(gtx layout.Context) layout.Dimensions {
		var empty kit.Empty
		switch connection {
		case state.Off:
			s.turnOn.Kind, s.turnOn.Label = kit.PlaqueGold, i18n.T("Turn on Rashin")
			empty = kit.Empty{
				Icon: look.Icon.Ask, Title: i18n.T("Rashin is off"),
				Body:    i18n.T("While it is off nothing runs and nothing is indexed. Turn it on to ask about this machine, run agents and search your code."),
				Actions: []*kit.Plaque{&s.turnOn},
			}
		case state.Unreachable:
			s.retry.Kind, s.retry.Label = kit.PlaqueGold, i18n.T("Try again")
			empty = kit.Empty{
				Icon: look.Icon.Warning, Title: i18n.T("Rashin did not answer"),
				Body: i18n.Tf("Its log is at %s.", paths.DaemonLog()), Actions: []*kit.Plaque{&s.retry},
			}
		default:
			empty = kit.Empty{Icon: look.Icon.Ask, Title: i18n.T("Starting Rashin"), Body: i18n.T("Your sessions appear when it answers.")}
		}
		return empty.Layout(gtx, th.Theme)
	})
	look.Place(gtx, image.Rect(0, 0, size.X, top), func(gtx layout.Context) layout.Dimensions {
		return s.topBar.Layout(gtx, s, e, barState{class: s.class})
	})
}

type bottomBar struct {
	tiles [5]navRow
	slab  look.Selection
}

func (bar *bottomBar) Layout(gtx layout.Context, s *uiShell, e *env.Env) layout.Dimensions {
	th := e.Theme
	size := gtx.Constraints.Max
	items := destinations()
	tabs := [5]destination{items[0], items[1], items[2], items[3], {protocol.TabSettings, i18n.T("Settings"), "", look.Icon.Settings}}
	inset, gap := gtx.Dp(4), gtx.Dp(4)
	width := (size.X - 2*inset - 4*gap) / 5
	height := gtx.Dp(48)
	rect := func(index int) image.Rectangle {
		x := inset + index*(width+gap)
		return image.Rect(x, (size.Y-height)/2, x+width, (size.Y+height)/2)
	}
	for index, tab := range tabs {
		if bar.tiles[index].click.Clicked(gtx) {
			s.openDestination(tab.tab)
		}
		if tab.tab == s.router.Tab {
			kit.FillCut(gtx.Ops, bar.slab.Rect(gtx, tab.tab, rect(index), th.Motion.Select), gtx.Dp(th.Cut.Small), th.R.Sel)
		}
	}
	for index, tab := range tabs {
		tile := &bar.tiles[index]
		look.Place(gtx, rect(index), func(gtx layout.Context) layout.Dimensions {
			return tile.click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				drawBottomTile(gtx, th, tile, tab, tab.tab == s.router.Tab)
				return layout.Dimensions{Size: gtx.Constraints.Max}
			})
		})
	}
	return layout.Dimensions{Size: size}
}

func drawBottomTile(gtx layout.Context, th *look.Theme, tile *navRow, tab destination, selected bool) {
	size := gtx.Constraints.Max
	hover := look.Transition(gtx, &tile.hover, tile.click.Hovered(), th.Motion.HoverIn, th.Motion.HoverOut)
	ink := palette.Mix(th.R.Fg2, th.R.Fg, hover)
	if selected {
		ink = th.R.OnSel
	} else if hover > 0 {
		kit.FillCut(gtx.Ops, image.Rectangle{Max: size}, gtx.Dp(th.Cut.Small), palette.WithAlpha(th.R.Stone3, hover))
	}
	glyph := gtx.Dp(20)
	line := gtx.Sp(th.T.Caption.Line)
	y := (size.Y - glyph - gtx.Dp(2) - line) / 2
	look.PlaceIcon(gtx, tab.icon, image.Pt((size.X-glyph)/2, y), unit.Dp(20), ink)
	look.Place(gtx, image.Rect(0, y+glyph+gtx.Dp(2), size.X, y+glyph+gtx.Dp(2)+line), func(gtx layout.Context) layout.Dimensions {
		return layout.Center.Layout(gtx, kit.RawText(th.Theme, th.T.Caption, ink, tab.label))
	})
}
