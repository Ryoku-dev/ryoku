package shell

import (
	"image"
	"image/color"
	"strconv"
	"time"

	"gioui.org/io/semantic"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/widget"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/anim"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/protocol"
)

type destination struct {
	tab      string
	label    string
	shortcut string
	icon     *widget.Icon
	// drawn while the destination is current
	fill *widget.Icon
}

// the destination's glyph, filled while it is current
func (item destination) glyph(current bool) *widget.Icon {
	if current {
		return item.fill
	}
	return item.icon
}

// the most destinations a mode has, Teams' seven
const destinationsMax = 7

// each mode's destinations, Hyakki's fill the first four
func destinations(teams bool) (items [destinationsMax]destination, count int) {
	if teams {
		return [destinationsMax]destination{
			{protocol.TabInbox, i18n.T("Inbox"), "Ctrl 1", look.Icon.Inbox, look.IconFill.Inbox},
			{protocol.TabTeams, i18n.T("Teams"), "Ctrl 2", look.Icon.Teams, look.IconFill.Teams},
			{protocol.TabDreams, i18n.T("Dreams"), "Ctrl 3", look.Icon.Dreams, look.IconFill.Dreams},
			{protocol.TabWork, i18n.T("Work"), "Ctrl 4", look.Icon.Tasks, look.IconFill.Tasks},
			{protocol.TabRoutines, i18n.T("Routines"), "Ctrl 5", look.Icon.Routine, look.IconFill.Routine},
			{protocol.TabVault, i18n.T("Vault"), "Ctrl 6", look.Icon.Vault, look.IconFill.Vault},
			{protocol.TabActivity, i18n.T("Activity"), "Ctrl 7", look.Icon.Activity, look.IconFill.Activity},
		}, 7
	}
	return [destinationsMax]destination{
		{protocol.TabAsk, i18n.T("Ask"), "Ctrl 1", look.Icon.Ask, look.IconFill.Ask},
		{protocol.TabAgent, i18n.T("Agents"), "Ctrl 2", look.Icon.Agents, look.IconFill.Agents},
		{protocol.TabProjects, i18n.T("Projects"), "Ctrl 3", look.Icon.Projects, look.IconFill.Projects},
		{protocol.TabIndex, i18n.T("Index"), "Ctrl 4", look.Icon.Index, look.IconFill.Index},
	}, 4
}

type navRow struct {
	click kit.Click
	hover anim.Tween
	press anim.Tween
	focus anim.Tween
}

type navContent struct {
	icon  *widget.Icon
	label string
	// zero draws no number
	count         int
	current, slab bool
	tab           string
	hint          string
}

type sidebar struct {
	// leaves out the destinations and settings, for a window whose bottom bar already has them
	bare bool

	freshPlaque kit.Plaque
	freshIcon   kit.IconPlaque
	rows        [destinationsMax]navRow
	peekTag     struct{}
	peekTab     string
	peekSince   time.Time
	peekPointer image.Point
	foot        [3]navRow
	slab        look.Selection
}

type sidebarLayout struct {
	fresh image.Rectangle
	foot  [3]image.Rectangle
	rows  [destinationsMax]image.Rectangle
	list  image.Rectangle
	vein  int
}

// count rows stand for the mode's destinations, the rest stay empty
func resolveSidebar(gtx layout.Context, size image.Point, bare bool, count int) sidebarLayout {
	inset, row, gap := gtx.Dp(8), gtx.Dp(40), gtx.Dp(4)
	width := max(0, size.X-2*inset)
	full := gtx.Dp(look.SidebarExpanded) - 2*inset
	var resolved sidebarLayout
	y := gtx.Dp(13)
	resolved.fresh = image.Rect(inset, y, inset+width, y+row)
	y += row + gtx.Dp(13)
	if bare {
		foot := gtx.Dp(34)
		for index := range resolved.foot[:2] {
			y := size.Y - gtx.Dp(13) - (2-index)*foot
			resolved.foot[index] = image.Rect(inset, y, inset+width, y+foot)
		}
		resolved.vein = resolved.foot[0].Min.Y - gtx.Dp(9)
		resolved.list = image.Rect(inset+gtx.Dp(13), y, inset+full, max(y, resolved.vein-gtx.Dp(8)))
		return resolved
	}
	for index := range resolved.rows[:count] {
		resolved.rows[index] = image.Rect(inset, y, inset+width, y+row)
		y += row + gap
	}
	y += gtx.Dp(13) - gap
	foot := gtx.Dp(34)
	for index := range resolved.foot {
		y := size.Y - gtx.Dp(13) - (len(resolved.foot)-index)*foot
		resolved.foot[index] = image.Rect(inset, y, inset+width, y+foot)
	}
	resolved.vein = resolved.foot[0].Min.Y - gtx.Dp(9)
	// the list keeps its open width while the pane folds so rows clip instead of reflowing
	resolved.list = image.Rect(inset+gtx.Dp(13), y, inset+full, max(y, resolved.vein-gtx.Dp(8)))
	return resolved
}

// page fills the list, the New plaque speaks for the destination the list belongs to
func (bar *sidebar) Layout(gtx layout.Context, s *uiShell, e *env.Env, page env.Page, fold float32) layout.Dimensions {
	size := gtx.Constraints.Max
	th := e.Theme
	items, count := destinations(s.router.TeamsMode())
	geometry := resolveSidebar(gtx, size, bar.bare, count)
	bar.handle(gtx, s, e)

	listCall, list := bar.recordList(gtx, e, page, geometry.list, fold)
	bar.layoutSlab(gtx, th, s, geometry, list, fold)

	bar.layoutNew(gtx, s, e, geometry.fresh, fold)
	onList := list.HasSelected && fold < 1
	for index, item := range items[:count] {
		current := s.router.Tab == item.tab
		content := navContent{
			icon: item.glyph(current), label: item.label, tab: item.tab, hint: item.shortcut, current: current, slab: current && !onList,
		}
		switch item.tab {
		case protocol.TabAgent:
			content.count = s.router.NeedsYou
		case protocol.TabInbox:
			content.count = s.router.Inbox
		}
		bar.layoutRow(gtx, th, &bar.rows[index], geometry.rows[index], fold, content)
	}
	peekTab, peekTitle := "", ""
	for index, item := range items[:count] {
		if bar.rows[index].click.Hovered() {
			peekTab, peekTitle = item.tab, item.label
		}
	}
	if fold < 1 {
		opacity := paint.PushOpacity(gtx.Ops, 1-fold)
		if !bar.bare && geometry.list.Dy() > 0 {
			x := geometry.list.Min.X + gtx.Dp(look.SideInset) + gtx.Dp(look.SideIndent)/2
			line := max(1, gtx.Dp(1))
			kit.Fill(gtx, th.Theme, image.Rect(x, geometry.list.Min.Y, x+line, geometry.list.Max.Y), 0, th.R.Guide())
		}
		offset := op.Offset(geometry.list.Min).Push(gtx.Ops)
		area := clip.Rect(image.Rectangle{Max: geometry.list.Size()}).Push(gtx.Ops)
		listCall.Add(gtx.Ops)
		area.Pop()
		offset.Pop()
		opacity.Pop()
	}

	if bar.bare {
		bar.layoutFoot(gtx, s, e, geometry, fold, onList, 2)
		bar.updatePeek(gtx, s, e, "", "")
		return layout.Dimensions{Size: size}
	}
	bar.layoutFoot(gtx, s, e, geometry, fold, onList, 3)
	for index, tab := range []string{protocol.TabMemory, protocol.TabLearning, protocol.TabSettings} {
		if bar.foot[index].click.Hovered() {
			place, _ := look.PlaceFor(tab)
			peekTab, peekTitle = tab, place.Title()
		}
	}
	if fold > 0 || s.router.Modal {
		peekTab = ""
	}
	bar.updatePeek(gtx, s, e, peekTab, peekTitle)
	return layout.Dimensions{Size: size}
}

func (bar *sidebar) layoutFoot(gtx layout.Context, s *uiShell, e *env.Env, geometry sidebarLayout, fold float32, onList bool, count int) {
	th := e.Theme
	inset, line := gtx.Dp(8), max(1, gtx.Dp(1))
	kit.Fill(gtx, th.Theme, image.Rect(inset, geometry.vein, gtx.Constraints.Max.X-inset, geometry.vein+line), 0, th.R.Line())
	for index, tab := range []string{protocol.TabMemory, protocol.TabLearning, protocol.TabSettings}[:count] {
		place, _ := look.PlaceFor(tab)
		current := s.router.Tab == tab
		icon := look.Icon.Memory
		switch tab {
		case protocol.TabLearning:
			icon = look.Icon.Skills
		case protocol.TabSettings:
			icon = look.Icon.Settings
		}
		bar.layoutRow(gtx, th, &bar.foot[index], geometry.foot[index], fold, navContent{
			icon: icon, label: place.Title(), tab: tab, current: current, slab: current && !onList,
		})
	}
}

func (bar *sidebar) handle(gtx layout.Context, s *uiShell, e *env.Env) {
	if bar.freshPlaque.Clicked(gtx) || bar.freshIcon.Clicked(gtx) {
		s.newCurrent(e)
	}
	items, count := destinations(s.router.TeamsMode())
	for index, item := range items[:count] {
		if bar.rows[index].click.Clicked(gtx) {
			s.openDestination(item.tab)
		}
	}
	for index, tab := range []string{protocol.TabMemory, protocol.TabLearning, protocol.TabSettings} {
		if bar.foot[index].click.Clicked(gtx) {
			s.openDestination(tab)
		}
	}
}

func (bar *sidebar) layoutNew(gtx layout.Context, s *uiShell, e *env.Env, rectangle image.Rectangle, fold float32) {
	label := s.page(s.listTab).NewLabel()
	look.Place(gtx, rectangle, func(gtx layout.Context) layout.Dimensions {
		if fold >= 1 {
			bar.freshIcon.Kind, bar.freshIcon.Small = kit.PlaqueLine, true
			bar.freshIcon.Icon, bar.freshIcon.Description = look.Icon.Add, label
			return bar.freshIcon.Layout(gtx, e.Theme.Theme)
		}
		bar.freshPlaque.Kind, bar.freshPlaque.Small = kit.PlaqueLine, true
		bar.freshPlaque.Icon, bar.freshPlaque.Label = look.Icon.Add, label
		return bar.freshPlaque.Layout(gtx, e.Theme.Theme)
	})
}

func (bar *sidebar) updatePeek(gtx layout.Context, s *uiShell, e *env.Env, tab, title string) {
	if tab == "" || !gtx.Source.Enabled() {
		bar.peekTab = ""
		s.overlay.Close(&bar.peekTag)
		return
	}
	pointer := e.Overlays.Pointer()
	if tab != bar.peekTab || pointer != bar.peekPointer {
		bar.peekTab, bar.peekSince, bar.peekPointer = tab, gtx.Now, pointer
		s.overlay.Close(&bar.peekTag)
		gtx.Execute(op.InvalidateCmd{At: gtx.Now.Add(480 * time.Millisecond)})
		return
	}
	if gtx.Now.Sub(bar.peekSince) < 480*time.Millisecond {
		return
	}
	place, _ := look.PlaceFor(tab)
	anchor := image.Rect(gtx.Dp(look.Margin+look.SidebarExpanded+look.Gap), e.Overlays.Pointer().Y,
		gtx.Dp(look.Margin+look.SidebarExpanded+look.Gap), e.Overlays.Pointer().Y)
	s.overlay.Open(&bar.peekTag, anchor, look.Placement{Passive: true, Frame: true, Width: 300}, func(gtx layout.Context) layout.Dimensions {
		th := e.Theme
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(kit.RawText(th.Theme, th.T.Label, th.R.Fg, title)),
			layout.Rigid(layout.Spacer{Height: th.Sp.S2}.Layout),
			layout.Rigid(kit.RawText(th.Theme, th.T.MonoSm, th.R.Fg3, look.Crumb(place.PagePlace(), title))),
		)
	})
}

// laid out without drawing first, the slab under it needs the selected row's place
func (bar *sidebar) recordList(
	gtx layout.Context,
	e *env.Env,
	page env.Page,
	rectangle image.Rectangle,
	fold float32,
) (op.CallOp, look.ListDimensions) {
	if fold >= 1 || rectangle.Dy() <= 0 {
		return op.CallOp{}, look.ListDimensions{}
	}
	record := op.Record(gtx.Ops)
	child := gtx
	child.Constraints = layout.Exact(rectangle.Size())
	if fold > 0 {
		child = child.Disabled()
	}
	dimensions := page.List(child, e)
	return record.Stop(), dimensions
}

func (bar *sidebar) layoutSlab(gtx layout.Context, th *look.Theme, s *uiShell, geometry sidebarLayout, list look.ListDimensions, fold float32) {
	key, target, inList := s.router.Tab, image.Rectangle{}, false
	switch {
	case list.HasSelected && fold < 1:
		key, target, inList = "row:"+list.Key, list.Selected.Add(geometry.list.Min), true
	case standaloneTab(s.router.Tab):
		for index, tab := range []string{protocol.TabMemory, protocol.TabLearning, protocol.TabSettings} {
			if tab == s.router.Tab {
				target = geometry.foot[index]
			}
		}
	default:
		items, count := destinations(s.router.TeamsMode())
		for index, item := range items[:count] {
			if item.tab == s.router.Tab {
				target = geometry.rows[index]
			}
		}
	}
	// in the bare sheet the marked place isn't drawn, so forget where the slab stood or it travels in from nowhere
	if target.Empty() {
		bar.slab = look.Selection{}
		return
	}
	rectangle := bar.slab.Rect(gtx, key, target, th.Motion.Select)
	// a row's slab is clipped with its list, but one travelling in from a destination has to cross the list's edge
	if inList && !bar.slab.Moving() {
		area := clip.Rect(geometry.list).Push(gtx.Ops)
		defer area.Pop()
	}
	look.RowWash(gtx, th, rectangle, 0, 0, true)
}

func (bar *sidebar) layoutRow(
	gtx layout.Context,
	th *look.Theme,
	row *navRow,
	rectangle image.Rectangle,
	fold float32,
	content navContent,
) {
	if rectangle.Dx() <= 0 {
		return
	}
	body := func(gtx layout.Context) layout.Dimensions {
		original := gtx
		dimensions := row.click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			semantic.Button.Add(gtx.Ops)
			semantic.LabelOp(content.label).Add(gtx.Ops)
			semantic.SelectedOp(content.current).Add(gtx.Ops)
			drawNavRow(gtx, th, row, fold, content)
			return layout.Dimensions{Size: gtx.Constraints.Max}
		})
		focus := look.Transition(original, &row.focus, original.Focused(&row.click.Clickable), th.Motion.FocusIn, th.Motion.FocusOut)
		kit.FocusRingAlpha(original, th.Theme, image.Rectangle{Max: dimensions.Size}, gtx.Dp(th.Cut.Small), focus)
		return dimensions
	}
	look.Place(gtx, rectangle, body)
}

func drawNavRow(gtx layout.Context, th *look.Theme, row *navRow, fold float32, content navContent) {
	size := gtx.Constraints.Max
	bounds := image.Rectangle{Max: size}
	hover := look.Transition(gtx, &row.hover, row.click.Hovered() && gtx.Source.Enabled(), th.Motion.HoverIn, th.Motion.HoverOut)
	press := look.Transition(gtx, &row.press, row.click.Pressed(), th.Motion.Press, th.Motion.Press)
	ink, hint := palette.Mix(th.R.Fg2, th.R.Fg, hover), th.R.Fg3
	mark := ink
	if content.slab {
		ink, hint = look.RowInk(th, true)
		mark = th.R.Sel
	} else if content.current {
		ink, mark = th.R.Fg, th.R.Sel
	}
	look.RowWash(gtx, th, bounds, hover, press, false)

	glyph := gtx.Dp(20)
	rail := gtx.Dp(look.SidebarRail) - 2*gtx.Dp(8)
	x := gtx.Dp(look.SideInset) + int(float32((rail-glyph)/2-gtx.Dp(look.SideInset))*fold+.5)
	look.PlaceIcon(gtx, content.icon, image.Pt(x+int(float32(gtx.Dp(2))*hover+.5), (size.Y-glyph)/2), 20, mark)
	if content.count > 0 && fold >= 1 {
		kit.ToneMark(gtx, th.Theme, bounds, palette.Attention)
	}
	if fold >= 1 {
		return
	}
	opacity := paint.PushOpacity(gtx.Ops, 1-fold)
	defer opacity.Pop()
	trailing := gtx.Dp(look.SideInset)
	number, gloss := look.Numbered(content.tab)
	if content.count > 0 {
		trailing += drawTrailingText(gtx, th, th.T.MonoSm, strconv.Itoa(content.count), th.R.Amber)
	} else if th.Desktop.Gloss && gloss != "" {
		glossInk := th.R.Fg3
		if content.current {
			glossInk = mark
		}
		trailing += drawTrailingText(gtx, th, th.T.Caption, gloss, glossInk)
	} else if content.hint != "" {
		trailing += drawTrailingText(gtx, th, th.T.MonoSm, content.hint, hint)
	}
	left := gtx.Dp(look.SideInset) + glyph + gtx.Dp(8) + int(float32(gtx.Dp(2))*hover+.5)
	if number != "" {
		look.Place(gtx, image.Rect(left, 0, left+gtx.Dp(24), size.Y), func(gtx layout.Context) layout.Dimensions {
			return layout.W.Layout(gtx, kit.RawText(th.Theme, th.T.MonoSm, mark, number))
		})
		left += gtx.Dp(28)
	}
	label := gtx
	label.Constraints = layout.Exact(image.Pt(max(0, gtx.Dp(look.SidebarExpanded)-2*gtx.Dp(8)-left-trailing), size.Y))
	offset := op.Offset(image.Pt(left, 0)).Push(gtx.Ops)
	layout.W.Layout(label, kit.RawText(th.Theme, th.T.Row, ink, content.label))
	offset.Pop()
}

func drawTrailingText(gtx layout.Context, th *look.Theme, style look.TextStyle, value string, ink color.NRGBA) int {
	size := gtx.Constraints.Max
	width := gtx.Dp(look.SidebarExpanded) - 2*gtx.Dp(8)
	record := op.Record(gtx.Ops)
	measure := gtx
	measure.Constraints = layout.Constraints{Max: image.Pt(width/2, size.Y)}
	dimensions := kit.RawText(th.Theme, style, ink, value)(measure)
	call := record.Stop()
	offset := op.Offset(image.Pt(width-gtx.Dp(look.SideInset)-dimensions.Size.X, (size.Y-dimensions.Size.Y)/2)).Push(gtx.Ops)
	call.Add(gtx.Ops)
	offset.Pop()
	return dimensions.Size.X + gtx.Dp(8)
}
