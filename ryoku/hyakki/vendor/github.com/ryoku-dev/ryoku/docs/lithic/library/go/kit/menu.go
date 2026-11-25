package kit

import (
	"image"
	"image/color"

	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/io/semantic"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/anim"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/symbols"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/theme"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/tokens"
)

var iconMenuCheck = Icon(symbols.Check)

// MenuItem describes one command, disabled command, danger command, or divider.
// A checked item shows a trailing check in place of its shortcut.
type MenuItem struct {
	Icon            *widget.Icon
	Label, Shortcut string
	Checked         bool
	Danger          bool
	Disabled        bool
	Divider         bool
	Preview         State
}

// Menu is a cut command list with latched item activation. Inside a popover
// it takes keyboard focus on its current choice, or first enabled item; Up and
// Down move it, wrapping and skipping dividers and disabled items, Home and
// End go to the ends, Page Up and Down move a viewport, and Return, Enter
// or Space chooses through the item's click. Escape closes the popover.
type Menu struct {
	Preview State

	clicks              []Click
	disabled, divider   []bool
	pending             []bool
	hover, press, focus []anim.Tween
	start               int
	list                layout.List
	rail                ScrollRail
	offsets             []int
	viewport            int
	visibleRows         int
	scrollTag           int
	// openedFromKeys says whether the host opened the menu from the keys;
	// quiet is one more than the item a pointer-opened menu focused, which
	// shows no keyboard ring until a key moves focus
	openedFromKeys bool
	quiet          int
}

// menuMove returns the item a navigation key lands on from item from, among
// count items of which enabled reports the choosable ones.
func menuMove(count int, enabled func(int) bool, from int, name key.Name) (int, bool) {
	step := 1
	switch name {
	case key.NameHome:
		from, step = -1, 1
	case key.NameEnd:
		from, step = count, -1
	case key.NameUpArrow:
		step = -1
	case key.NameDownArrow:
	default:
		return from, false
	}
	wrap := name == key.NameUpArrow || name == key.NameDownArrow
	next := from
	for range count {
		next += step
		if wrap {
			next = (next%count + count) % count
		}
		if next < 0 || next >= count {
			break
		}
		if enabled(next) {
			return next, true
		}
	}
	return from, false
}

// menuStart returns the item keyboard focus starts on: start when it is
// choosable, otherwise the first choosable item.
func menuStart(count int, enabled func(int) bool, start int) (int, bool) {
	if start >= 0 && start < count && enabled(start) {
		return start, true
	}
	return menuMove(count, enabled, -1, key.NameHome)
}

func (m *Menu) enabled(index int) bool {
	return index >= 0 && index < len(m.clicks) && !m.disabled[index] && !m.divider[index]
}

// FocusItem moves keyboard focus to item index, or to the first enabled item
// when index cannot take it, for a host that opens the menu from the keys;
// the item shows the keyboard ring.
func (m *Menu) FocusItem(gtx layout.Context, index int) {
	m.focusStart(gtx, index, true)
}

func (m *Menu) focusStart(gtx layout.Context, index int, fromKeys bool) {
	if next, ok := menuStart(len(m.clicks), m.enabled, index); ok {
		m.quiet = 0
		if !fromKeys {
			m.quiet = next + 1
		}
		m.reveal(next)
		gtx.Execute(key.FocusCmd{Tag: &m.clicks[next].Clickable})
	}
}

func (m *Menu) handleKeys(gtx layout.Context) {
	overlay := overlayFocusOf(gtx)
	if overlay != nil && overlay.idle {
		m.focusStart(gtx, m.start, m.openedFromKeys)
	}
	for index := range m.clicks {
		if gtx.Focused(&m.clicks[index].Clickable) {
			m.handleItemKeys(gtx, index)
		}
	}
}

func (m *Menu) handleItemKeys(gtx layout.Context, index int) {
	tag := &m.clicks[index].Clickable
	for {
		value, ok := gtx.Event(
			key.Filter{Focus: tag, Name: key.NameUpArrow},
			key.Filter{Focus: tag, Name: key.NameDownArrow},
			key.Filter{Focus: tag, Name: key.NameHome},
			key.Filter{Focus: tag, Name: key.NameEnd},
			key.Filter{Focus: tag, Name: key.NamePageUp},
			key.Filter{Focus: tag, Name: key.NamePageDown},
		)
		if !ok {
			break
		}
		pressed, ok := value.(key.Event)
		if !ok || pressed.State != key.Press {
			continue
		}
		next, moves := menuMove(len(m.clicks), m.enabled, index, pressed.Name)
		if pressed.Name == key.NamePageUp || pressed.Name == key.NamePageDown {
			delta := m.visibleRows
			if pressed.Name == key.NamePageUp {
				delta = -delta
			}
			next, moves = menuPage(len(m.clicks), m.enabled, index, delta)
		}
		if moves {
			m.quiet = 0
			m.reveal(next)
			gtx.Execute(key.FocusCmd{Tag: &m.clicks[next].Clickable})
		}
	}
}

func menuPage(count int, enabled func(int) bool, from, delta int) (int, bool) {
	if count == 0 {
		return from, false
	}
	target := max(0, min(count-1, from+delta))
	step := 1
	if delta < 0 {
		step = -1
	}
	for next := target; next >= 0 && next < count; next += step {
		if enabled(next) {
			return next, true
		}
	}
	for next := target - step; next >= 0 && next < count; next -= step {
		if enabled(next) {
			return next, true
		}
	}
	return from, false
}

func (m *Menu) reveal(index int) {
	if index < 0 || index+1 >= len(m.offsets) || m.viewport <= 0 {
		return
	}
	first := max(0, min(m.list.Position.First, len(m.offsets)-2))
	top := m.offsets[first] + m.list.Position.Offset
	switch {
	case m.offsets[index] < top:
		m.list.ScrollTo(index)
	case m.offsets[index+1] > top+m.viewport:
		m.list.ScrollTo(index)
		m.list.Position.Offset = min(0, m.offsets[index+1]-m.offsets[index]-m.viewport)
	}
}

func (m *Menu) prepare(items []MenuItem) {
	if len(m.clicks) != len(items) {
		clicks := make([]Click, len(items))
		copy(clicks, m.clicks)
		m.clicks = clicks
	}
	if len(m.disabled) != len(items) {
		m.disabled = make([]bool, len(items))
	}
	if len(m.divider) != len(items) {
		m.divider = make([]bool, len(items))
	}
	if len(m.pending) != len(items) {
		m.pending = make([]bool, len(items))
	}
	if len(m.hover) != len(items) {
		hover := make([]anim.Tween, len(items))
		copy(hover, m.hover)
		m.hover = hover
	}
	if len(m.press) != len(items) {
		press := make([]anim.Tween, len(items))
		copy(press, m.press)
		m.press = press
	}
	if len(m.focus) != len(items) {
		focus := make([]anim.Tween, len(items))
		copy(focus, m.focus)
		m.focus = focus
	}
	for index, item := range items {
		m.disabled[index] = item.Disabled
		m.divider[index] = item.Divider
		if item.Disabled || item.Divider {
			m.pending[index] = false
		}
	}
}

// Layout draws the menu and its items.
func (m *Menu) Layout(gtx layout.Context, th *theme.Theme, items []MenuItem) layout.Dimensions {
	m.prepare(items)
	for index := range m.clicks {
		if m.enabled(index) && (m.visible(index) || gtx.Focused(&m.clicks[index].Clickable)) && m.clicks[index].Clicked(gtx) {
			m.pending[index] = true
		}
	}
	m.start = 0
	for index, item := range items {
		if item.Checked && !item.Disabled && !item.Divider {
			m.start = index
			break
		}
	}
	padding := gtx.Dp(tokens.SizeMenuPaddingVertical)
	rowHeight := max(1, gtx.Dp(tokens.SizeMenuItem))
	if len(m.offsets) != len(items)+1 {
		m.offsets = make([]int, len(items)+1)
	}
	m.offsets[0] = 0
	for index, item := range items {
		height := rowHeight
		if item.Divider {
			height = 2*gtx.Dp(th.Sp.S2) + max(1, gtx.Dp(tokens.SizeHairline))
		}
		m.offsets[index+1] = m.offsets[index] + height
	}
	height := min(gtx.Constraints.Max.Y, 2*padding+min(m.offsets[len(items)], gtx.Dp(unit.Dp(tokens.SizeMenuVisibleItemsCount*tokens.SizeMenuItem))))
	height = max(0, height)
	previousViewport := m.viewport
	m.viewport = max(0, height-2*padding)
	m.visibleRows = max(1, m.viewport/rowHeight)
	m.handleKeys(gtx)
	// Focus stays in the overlay when the pointer scrolls its row out of view.
	for index := range m.clicks {
		tag := &m.clicks[index].Clickable
		if gtx.Focused(tag) {
			if previousViewport != m.viewport {
				m.reveal(index)
			}
			// Key-only handlers must not invalidate Gio's implicit semantic root.
			area := clip.Rect(image.Rectangle{Max: gtx.Constraints.Max}).Push(gtx.Ops)
			event.Op(gtx.Ops, tag)
			area.Pop()
			noteOverlayFocus(gtx, tag)
		}
	}
	width := menuWidth(gtx, th, items)
	gtx.Constraints.Min.X = width
	gtx.Constraints.Max.X = width
	gtx.Constraints.Min.Y = height
	gtx.Constraints.Max.Y = height

	content := op.Record(gtx.Ops)
	for {
		if _, ok := gtx.Event(pointer.Filter{Target: &m.scrollTag, Kinds: pointer.Scroll, ScrollX: pointer.ScrollRange{Min: -overlayExtent, Max: overlayExtent}, ScrollY: pointer.ScrollRange{Min: -overlayExtent, Max: overlayExtent}}); !ok {
			break
		}
	}
	scrollArea := clip.Rect(image.Rectangle{Max: gtx.Constraints.Max}).Push(gtx.Ops)
	event.Op(gtx.Ops, &m.scrollTag)
	scrollArea.Pop()
	dimensions := layout.Inset{Top: unit.Dp(tokens.SizeMenuPaddingVertical), Bottom: unit.Dp(tokens.SizeMenuPaddingVertical)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		m.list.Axis = layout.Vertical
		m.list.Layout(gtx, len(items), func(gtx layout.Context, index int) layout.Dimensions {
			item := items[index]
			if m.enabled(index) {
				// Register the destination's keys in the same frame as a lazy focus jump.
				m.handleItemKeys(gtx, index)
			}
			if item.Divider {
				return layout.Inset{Top: th.Sp.S2, Bottom: th.Sp.S2, Left: unit.Dp(tokens.SizeMenuDividerInset), Right: unit.Dp(tokens.SizeMenuDividerInset)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					width := gtx.Constraints.Max.X
					height := max(1, gtx.Dp(tokens.SizeHairline))
					FillCut(gtx.Ops, image.Rect(0, 0, width, height), 0, th.R.Vein)
					return layout.Dimensions{Size: image.Pt(width, height)}
				})
			}
			row := func(gtx layout.Context) layout.Dimensions {
				height := gtx.Dp(tokens.SizeMenuItem)
				gtx.Constraints.Min.Y = height
				gtx.Constraints.Max.Y = height
				preview := item.Preview
				if preview == StateRest {
					preview = m.Preview
				}
				hovered := m.clicks[index].Hovered() || preview == StateHover
				pressed := m.clicks[index].Pressed() || preview == StatePressed
				hover := hoverValue(gtx, th, &m.hover[index], hovered && !item.Disabled)
				press := transitionValue(gtx, &m.press[index], pressed && !item.Disabled, th.Motion.Press, th.Motion.Press)
				if hover > 0 || press > 0 {
					amount := max(tokens.AlphaHoverPlane*hover, tokens.AlphaPressRow*press)
					fill := palette.Mix(th.R.Stone4, th.R.Fg, amount)
					FillCut(gtx.Ops, image.Rect(0, 0, gtx.Constraints.Max.X, height), 0, fill)
				}
				semantic.Button.Add(gtx.Ops)
				semantic.LabelOp(item.Label).Add(gtx.Ops)
				semantic.SelectedOp(item.Checked).Add(gtx.Ops)
				semantic.EnabledOp(!item.Disabled).Add(gtx.Ops)
				return layout.Inset{Left: unit.Dp(tokens.SizeMenuInset), Right: unit.Dp(tokens.SizeMenuInset)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return centerY(gtx, func(gtx layout.Context) layout.Dimensions {
						ink := th.R.Fg
						if item.Danger {
							ink = th.R.Jasper
						}
						if item.Disabled {
							ink = th.R.Fg3
						}
						children := make([]layout.FlexChild, 0, 4)
						if item.Icon != nil {
							children = append(children,
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									size := gtx.Dp(tokens.SizeMenuIcon)
									gtx.Constraints = layout.Exact(image.Pt(size, size))
									return item.Icon.Layout(gtx, ink)
								}),
								layout.Rigid(layout.Spacer{Width: unit.Dp(tokens.SizeMenuIconGap)}.Layout),
							)
						}
						children = append(children, layout.Flexed(1, menuText(th, th.T.Label, ink, item.Label)))
						switch {
						case item.Checked:
							children = append(children,
								layout.Rigid(layout.Spacer{Width: unit.Dp(tokens.SizeMenuShortcutGap)}.Layout),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									size := gtx.Dp(tokens.SizeMenuIcon)
									gtx.Constraints = layout.Exact(image.Pt(size, size))
									return iconMenuCheck.Layout(gtx, th.R.Fg2)
								}),
							)
						case item.Shortcut != "":
							children = append(children,
								layout.Rigid(layout.Spacer{Width: unit.Dp(tokens.SizeMenuShortcutGap)}.Layout),
								layout.Rigid(menuText(th, th.T.MonoSm, th.R.Fg3, item.Shortcut)),
							)
						}
						return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx, children...)
					})
				})
			}
			var dims layout.Dimensions
			if item.Disabled {
				dims = m.clicks[index].Layout(gtx.Disabled(), row)
			} else {
				dims = m.clicks[index].Layout(gtx, row)
			}
			preview := item.Preview
			if preview == StateRest {
				preview = m.Preview
			}
			focused := (gtx.Focused(&m.clicks[index].Clickable) && m.quiet != index+1) || preview == StateFocus
			focus := focusValue(gtx, th, &m.focus[index], focused && !item.Disabled)
			FocusRingAlpha(gtx, th, image.Rectangle{Max: dims.Size}, 0, focus)
			return dims
		})
		m.rail.Layout(gtx, th, &m.list, len(items))
		return layout.Dimensions{Size: gtx.Constraints.Max}
	})
	contentCall := content.Stop()

	overlay := op.Record(gtx.Ops)
	bounds := image.Rectangle{Max: dimensions.Size}
	Fill(gtx, th, bounds, gtx.Dp(tokens.CutMenu), th.R.Stone4)
	Stroke(gtx, th, bounds, gtx.Dp(tokens.CutMenu), max(1, gtx.Dp(tokens.SizeHairline)), th.R.Vein)
	cut := gtx.Dp(tokens.CutMenu)
	area := clip.Outline{Path: Shape(gtx, th, bounds, cut, cut, cut, cut)}.Op().Push(gtx.Ops)
	contentCall.Add(gtx.Ops)
	area.Pop()
	presentOverlay(gtx, overlay.Stop(), m.Preview != StateRest, dimensions.Size)
	return dimensions
}

func menuWidth(gtx layout.Context, th *theme.Theme, items []MenuItem) int {
	maximum := min(gtx.Dp(tokens.SizeMenuWidthMax), gtx.Constraints.Max.X)
	minimum := min(gtx.Dp(tokens.SizeMenuWidthMin), maximum)
	widest := minimum
	// A long menu uses its bounded width without shaping off-screen labels.
	if float64(len(items)) > tokens.SizeMenuVisibleItemsCount {
		return maximum
	}
	var operations op.Ops
	for _, item := range items {
		if item.Divider {
			continue
		}
		fixed := gtx.Dp(2 * tokens.SizeMenuInset)
		if item.Icon != nil {
			fixed += gtx.Dp(tokens.SizeMenuIcon + tokens.SizeMenuIconGap)
		}
		if item.Checked {
			fixed += gtx.Dp(tokens.SizeMenuShortcutGap + tokens.SizeMenuIcon)
		} else if item.Shortcut != "" {
			measure := gtx
			measure.Constraints.Min = image.Point{}
			measure.Constraints.Max.X = max(0, maximum-fixed-gtx.Dp(tokens.SizeMenuShortcutGap))
			operations.Reset()
			measure.Ops = &operations
			dims := Label{Style: th.T.MonoSm, Color: th.R.Fg3, MaxLines: 1}.Layout(measure, th, item.Shortcut)
			fixed += gtx.Dp(tokens.SizeMenuShortcutGap) + dims.Size.X
		}
		measure := gtx
		measure.Constraints.Min = image.Point{}
		measure.Constraints.Max.X = max(0, maximum-fixed)
		operations.Reset()
		measure.Ops = &operations
		dims := Label{Style: th.T.Label, Color: th.R.Fg, MaxLines: 1}.Layout(measure, th, item.Label)
		widest = max(widest, min(maximum, fixed+dims.Size.X))
	}
	return widest
}

func menuText(th *theme.Theme, style theme.TextStyle, ink color.NRGBA, value string) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		height := gtx.Constraints.Max.Y
		reference := trackedLine(gtx, th, style, 0, "H", false)
		baseline := (height-reference.ascent-reference.descent)/2 + reference.ascent
		record := op.Record(gtx.Ops)
		dimensions := rawText(th, style, ink, value)(gtx)
		call := record.Stop()
		offset := op.Offset(image.Pt(0, baseline-(dimensions.Size.Y-dimensions.Baseline))).Push(gtx.Ops)
		call.Add(gtx.Ops)
		offset.Pop()
		return layout.Dimensions{Size: image.Pt(dimensions.Size.X, height), Baseline: height - baseline}
	}
}

func (m *Menu) visible(index int) bool {
	return index >= m.list.Position.First && index < m.list.Position.First+m.list.Position.Count
}

// Clicked returns the activated item index.
func (m *Menu) Clicked(gtx layout.Context) (int, bool) {
	for index := range m.clicks {
		if m.pending[index] {
			m.pending[index] = false
			return index, true
		}
		if m.enabled(index) && (m.visible(index) || gtx.Focused(&m.clicks[index].Clickable)) && m.clicks[index].Clicked(gtx) {
			return index, true
		}
	}
	return 0, false
}

func centerY(gtx layout.Context, widget layout.Widget) layout.Dimensions {
	original := gtx.Constraints
	gtx.Constraints.Min.Y = 0
	record := op.Record(gtx.Ops)
	dimensions := widget(gtx)
	call := record.Stop()
	size := original.Constrain(image.Pt(dimensions.Size.X, max(original.Min.Y, dimensions.Size.Y)))
	y := (size.Y - dimensions.Size.Y) / 2
	offset := op.Offset(image.Pt(0, y)).Push(gtx.Ops)
	call.Add(gtx.Ops)
	offset.Pop()
	return layout.Dimensions{Size: size, Baseline: dimensions.Baseline + size.Y - dimensions.Size.Y - y}
}
