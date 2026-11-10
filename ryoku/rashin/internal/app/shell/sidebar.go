package shell

import (
	"image"
	"image/color"
	"strconv"

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
}

func destinations() [4]destination {
	return [4]destination{
		{protocol.TabAsk, i18n.T("Ask"), "Ctrl 1", look.Icon.Ask},
		{protocol.TabAgent, i18n.T("Agents"), "Ctrl 2", look.Icon.Agents},
		{protocol.TabProjects, i18n.T("Projects"), "Ctrl 3", look.Icon.Projects},
		{protocol.TabIndex, i18n.T("Index"), "Ctrl 4", look.Icon.Index},
	}
}

type navRow struct {
	click kit.Click
	hover anim.Tween
	press anim.Tween
	focus anim.Tween
	tip   look.Tip
}

type navContent struct {
	icon  *widget.Icon
	label string
	tip   string
	// zero draws no number
	count         int
	current, slab bool
	line          bool
	hint          string
}

type sidebar struct {
	// leaves out the destinations and settings, for a window whose bottom bar already has them
	bare bool

	fresh    navRow
	rows     [4]navRow
	settings navRow
	slab     look.Selection
}

type sidebarLayout struct {
	fresh, settings image.Rectangle
	rows            [4]image.Rectangle
	list            image.Rectangle
	vein            int
}

func resolveSidebar(gtx layout.Context, size image.Point, bare bool) sidebarLayout {
	inset, row, gap := gtx.Dp(8), gtx.Dp(40), gtx.Dp(4)
	width := max(0, size.X-2*inset)
	full := gtx.Dp(look.SidebarExpanded) - 2*inset
	var resolved sidebarLayout
	y := gtx.Dp(13)
	resolved.fresh = image.Rect(inset, y, inset+width, y+row)
	y += row + gtx.Dp(13)
	if bare {
		resolved.list = image.Rect(inset, y, inset+width, max(y, size.Y-gtx.Dp(8)))
		return resolved
	}
	for index := range resolved.rows {
		resolved.rows[index] = image.Rect(inset, y, inset+width, y+row)
		y += row + gap
	}
	y += gtx.Dp(13) - gap
	resolved.settings = image.Rect(inset, size.Y-gtx.Dp(13)-row, inset+width, size.Y-gtx.Dp(13))
	resolved.vein = resolved.settings.Min.Y - gtx.Dp(9)
	// the list keeps its open width while the pane folds so rows clip instead of reflowing
	resolved.list = image.Rect(inset, y, inset+full, max(y, resolved.vein-gtx.Dp(8)))
	return resolved
}

func (bar *sidebar) Layout(gtx layout.Context, s *uiShell, e *env.Env, page env.Page, fold float32) layout.Dimensions {
	size := gtx.Constraints.Max
	th := e.Theme
	geometry := resolveSidebar(gtx, size, bar.bare)
	bar.handle(gtx, s, e)

	listCall, list := bar.recordList(gtx, e, page, geometry.list, fold)
	bar.layoutSlab(gtx, th, s, geometry, list, fold)

	bar.layoutRow(gtx, th, e, &bar.fresh, geometry.fresh, fold, navContent{
		icon: look.Icon.Add, label: page.NewLabel(), tip: page.NewLabel() + "  Ctrl N", line: true, hint: "Ctrl N",
	})
	onList := list.HasSelected && fold < 1 && s.router.Tab != protocol.TabSettings
	for index, item := range destinations() {
		current := s.router.Tab == item.tab
		content := navContent{icon: item.icon, label: item.label, tip: item.label + "  " + item.shortcut, current: current, slab: current && !onList}
		if item.tab == protocol.TabAgent {
			content.count = s.router.NeedsYou
		}
		bar.layoutRow(gtx, th, e, &bar.rows[index], geometry.rows[index], fold, content)
	}
	if fold < 1 {
		opacity := paint.PushOpacity(gtx.Ops, 1-fold)
		offset := op.Offset(geometry.list.Min).Push(gtx.Ops)
		area := clip.Rect(image.Rectangle{Max: geometry.list.Size()}).Push(gtx.Ops)
		listCall.Add(gtx.Ops)
		area.Pop()
		offset.Pop()
		opacity.Pop()
	}

	if bar.bare {
		return layout.Dimensions{Size: size}
	}
	inset, line := gtx.Dp(8), max(1, gtx.Dp(1))
	paint.FillShape(gtx.Ops, th.R.Vein, clip.Rect(image.Rect(inset, geometry.vein, size.X-inset, geometry.vein+line)).Op())
	settings := s.router.Tab == protocol.TabSettings
	bar.layoutRow(gtx, th, e, &bar.settings, geometry.settings, fold, navContent{
		icon: look.Icon.Settings, label: i18n.T("Settings"), tip: i18n.T("Settings") + "  Ctrl ,", current: settings, slab: settings,
	})
	return layout.Dimensions{Size: size}
}

func (bar *sidebar) handle(gtx layout.Context, s *uiShell, e *env.Env) {
	if bar.fresh.click.Clicked(gtx) {
		s.newCurrent(e)
	}
	for index, item := range destinations() {
		if bar.rows[index].click.Clicked(gtx) {
			s.openDestination(item.tab)
		}
	}
	if bar.settings.click.Clicked(gtx) {
		s.openDestination(protocol.TabSettings)
	}
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
	key, target, inList := s.router.Tab, geometry.settings, false
	switch {
	case s.router.Tab == protocol.TabSettings:
	case list.HasSelected && fold < 1:
		key, target, inList = "row:"+list.Key, list.Selected.Add(geometry.list.Min), true
	default:
		for index, item := range destinations() {
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
	kit.FillCut(gtx.Ops, rectangle, gtx.Dp(th.Cut.Small), th.R.Sel)
}

func (bar *sidebar) layoutRow(
	gtx layout.Context,
	th *look.Theme,
	e *env.Env,
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
	look.Place(gtx, rectangle, func(gtx layout.Context) layout.Dimensions {
		// only the rail needs a tooltip, the open row says its own name
		if fold < 1 {
			return body(gtx)
		}
		return row.tip.Layout(gtx, th, e.Overlays, content.tip, body)
	})
}

func drawNavRow(gtx layout.Context, th *look.Theme, row *navRow, fold float32, content navContent) {
	size := gtx.Constraints.Max
	bounds := image.Rectangle{Max: size}
	cut := gtx.Dp(th.Cut.Small)
	hover := look.Transition(gtx, &row.hover, row.click.Hovered() && gtx.Source.Enabled(), th.Motion.HoverIn, th.Motion.HoverOut)
	press := look.Transition(gtx, &row.press, row.click.Pressed(), th.Motion.Press, th.Motion.Press)
	ink := palette.Mix(th.R.Fg2, th.R.Fg, hover)
	switch {
	case content.slab:
		ink = th.R.OnSel
	case content.current || content.line:
		ink = th.R.Fg
	}
	if !content.slab && hover > 0 {
		kit.FillCut(gtx.Ops, bounds, cut, palette.WithAlpha(th.R.Stone3, hover))
	}
	if press > 0 {
		kit.FillCut(gtx.Ops, bounds, cut, palette.WithAlpha(th.R.Fg, .08*press))
	}
	if content.line {
		kit.StrokeCut(gtx.Ops, bounds, cut, max(1, gtx.Dp(1)), th.R.Vein)
	}

	glyph := gtx.Dp(20)
	rail := gtx.Dp(look.SidebarRail) - 2*gtx.Dp(8)
	x := gtx.Dp(look.SideInset) + int(float32((rail-glyph)/2-gtx.Dp(look.SideInset))*fold+.5)
	look.PlaceIcon(gtx, content.icon, image.Pt(x, (size.Y-glyph)/2), 20, ink)
	if content.count > 0 && fold >= 1 {
		kit.Notch(gtx.Ops, image.Point{}, gtx.Dp(8), th.R.Amber)
	}
	if fold >= 1 {
		return
	}
	opacity := paint.PushOpacity(gtx.Ops, 1-fold)
	defer opacity.Pop()
	trailing := gtx.Dp(look.SideInset)
	if content.count > 0 {
		trailing += drawTrailingText(gtx, th, strconv.Itoa(content.count), th.R.Amber)
	} else if content.hint != "" {
		trailing += drawTrailingText(gtx, th, content.hint, th.R.Fg3)
	}
	left := gtx.Dp(look.SideInset) + glyph + gtx.Dp(13)
	label := gtx
	label.Constraints = layout.Exact(image.Pt(max(0, gtx.Dp(look.SidebarExpanded)-2*gtx.Dp(8)-left-trailing), size.Y))
	offset := op.Offset(image.Pt(left, 0)).Push(gtx.Ops)
	layout.W.Layout(label, kit.RawText(th.Theme, th.T.Label, ink, content.label))
	offset.Pop()
}

func drawTrailingText(gtx layout.Context, th *look.Theme, value string, ink color.NRGBA) int {
	size := gtx.Constraints.Max
	width := gtx.Dp(look.SidebarExpanded) - 2*gtx.Dp(8)
	record := op.Record(gtx.Ops)
	measure := gtx
	measure.Constraints = layout.Constraints{Max: image.Pt(width/2, size.Y)}
	dimensions := kit.RawText(th.Theme, th.T.MonoSm, ink, value)(measure)
	call := record.Stop()
	offset := op.Offset(image.Pt(width-gtx.Dp(look.SideInset)-dimensions.Size.X, (size.Y-dimensions.Size.Y)/2)).Push(gtx.Ops)
	call.Add(gtx.Ops)
	offset.Pop()
	return dimensions.Size.X + gtx.Dp(8)
}
