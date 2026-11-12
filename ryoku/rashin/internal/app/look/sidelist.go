package look

import (
	"image"
	"image/color"
	"strconv"
	"strings"

	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/io/semantic"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/anim"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"
)

// every left edge in the sidebar stands on SideInset
const (
	SideInset      unit.Dp = 13
	SideRowHeight  unit.Dp = 36
	SideRowTall    unit.Dp = 48
	SideRowGap     unit.Dp = 2
	SideIndent     unit.Dp = 21
	sideTrailing   unit.Dp = 40
	sideHeader     unit.Dp = 30
	sideHeaderLead unit.Dp = 22
)

// a group without rows isn't drawn, one without a title has no header
type SideGroup struct {
	Title string
	Count int
	Rows  []SideRow
}

// strings arrive translated
type SideRow struct {
	ID    string
	Title string
	Facts string
	Icon  *widget.Icon
	// Idle draws no notch
	Tone  palette.Tone
	Trail layout.Widget
	// the flakes replace Trail while work is live
	Working bool
	// inks the title as unread
	Strong bool
	Depth  int
	// Branch is a tree row with children, Open says they're shown
	Branch, Open bool
	// the row has a menu, so its trailing action is offered
	Menu bool
}

type SideEventKind uint8

const (
	SideOpen SideEventKind = iota
	SideToggle
	SideMenu
	SideRename
)

// Anchor is in window coordinates
type SideEvent struct {
	Kind   SideEventKind
	ID     string
	Title  string
	Anchor image.Rectangle
}

type ListDimensions struct {
	layout.Dimensions
	Selected    image.Rectangle
	HasSelected bool
	// names the selected row so the slab travels when it changes
	Key string
}

type sideRowState struct {
	click     kit.Click
	toggle    kit.Click
	action    IconAction
	hover     Hover
	press     anim.Tween
	swap      anim.Tween
	appear    anim.Tween
	secondary struct{}
	armed     bool
	editor    widget.Editor
	editing   bool
	focused   bool
}

type sideItem struct {
	header bool
	first  bool
	title  string
	count  int
	row    SideRow
}

type SideList struct {
	Scroller Scroller
	Preview  kit.State
	// said in the list's place while it has no rows, so an empty sidebar tells what will stand there
	Empty string

	states map[string]*sideRowState
	seen   map[string]bool
	items  []sideItem
	rows   []int
	cursor int
	keys   struct{}
	rename string
	events []SideEvent
	drawn  bool
}

func (list *SideList) Focus(gtx layout.Context) { gtx.Execute(key.FocusCmd{Tag: &list.keys}) }

func (list *SideList) Rename(id string) { list.rename = id }

func (list *SideList) Events() []SideEvent {
	events := list.events
	list.events = nil
	return events
}

func (list *SideList) Layout(
	gtx layout.Context,
	th *Theme,
	overlays *Overlays,
	groups []SideGroup,
	selected string,
) ListDimensions {
	list.flatten(gtx, th, groups)
	if len(list.items) == 0 && list.Empty != "" {
		list.drawn = true
		inset := layout.Inset{Top: 8, Right: SideInset, Left: SideInset}
		return ListDimensions{Dimensions: inset.Layout(gtx, Paragraph(th, th.T.Caption, th.R.Fg3, list.Empty))}
	}
	list.handleKeys(gtx)
	focused := gtx.Focused(&list.keys)
	list.Scroller.Preview = list.Preview
	list.Scroller.Gap = gtx.Dp(SideRowGap)

	area := clip.Rect(image.Rectangle{Max: gtx.Constraints.Max}).Push(gtx.Ops)
	event.Op(gtx.Ops, &list.keys)
	area.Pop()

	dimensions := list.Scroller.Layout(gtx, th, len(list.items), func(gtx layout.Context, index int) layout.Dimensions {
		item := list.items[index]
		if item.header {
			return list.layoutHeader(gtx, th, item)
		}
		cursor := focused && list.cursor < len(list.rows) && list.rows[list.cursor] == index
		return list.layoutRow(gtx, th, overlays, item.row, item.row.ID == selected, cursor)
	})
	list.drawn = true
	result := ListDimensions{Dimensions: dimensions}
	result.Selected, result.HasSelected = list.selectedRect(gtx, selected, dimensions.Size.X)
	result.Key = selected
	return result
}

func (list *SideList) flatten(gtx layout.Context, th *Theme, groups []SideGroup) {
	list.items, list.rows = list.items[:0], list.rows[:0]
	if list.states == nil {
		list.states = make(map[string]*sideRowState)
		list.seen = make(map[string]bool)
	}
	clear(list.seen)
	for _, group := range groups {
		if len(group.Rows) == 0 {
			continue
		}
		if group.Title != "" {
			list.items = append(list.items, sideItem{header: true, first: len(list.items) == 0, title: group.Title, count: group.Count})
		}
		for _, row := range group.Rows {
			list.rows = append(list.rows, len(list.items))
			list.items = append(list.items, sideItem{row: row})
			list.seen[row.ID] = true
			if list.states[row.ID] != nil {
				continue
			}
			state := &sideRowState{}
			state.editor.SingleLine, state.editor.Submit = true, true
			state.appear.Duration, state.appear.Ease = th.Motion.Unfold, anim.EaseChisel
			// a row that shows up after the first draw unfolds, the ones a list opens with are just there
			if list.drawn {
				state.appear.Set(gtx, 0)
			}
			state.appear.Set(gtx, 1)
			list.states[row.ID] = state
		}
	}
	for id := range list.states {
		if !list.seen[id] {
			delete(list.states, id)
		}
	}
	list.cursor = min(max(list.cursor, 0), max(0, len(list.rows)-1))
}

func (list *SideList) cursorRow() (SideRow, bool) {
	if list.cursor < 0 || list.cursor >= len(list.rows) {
		return SideRow{}, false
	}
	return list.items[list.rows[list.cursor]].row, true
}

func (list *SideList) handleKeys(gtx layout.Context) {
	// without the focus filter the list can't hold focus and the arrow keys below are never offered
	filters := []event.Filter{
		key.FocusFilter{Target: &list.keys},
		key.Filter{Focus: &list.keys, Name: key.NameUpArrow},
		key.Filter{Focus: &list.keys, Name: key.NameDownArrow},
		key.Filter{Focus: &list.keys, Name: key.NameLeftArrow},
		key.Filter{Focus: &list.keys, Name: key.NameRightArrow},
		key.Filter{Focus: &list.keys, Name: key.NameReturn},
		key.Filter{Focus: &list.keys, Name: key.NameEnter},
		key.Filter{Focus: &list.keys, Name: key.NameF2},
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
		row, ok := list.cursorRow()
		if !ok {
			continue
		}
		switch pressed.Name {
		case key.NameUpArrow:
			list.cursor = max(0, list.cursor-1)
			list.reveal(gtx)
		case key.NameDownArrow:
			list.cursor = min(len(list.rows)-1, list.cursor+1)
			list.reveal(gtx)
		case key.NameLeftArrow:
			if row.Branch && row.Open {
				list.events = append(list.events, SideEvent{Kind: SideToggle, ID: row.ID})
			}
		case key.NameRightArrow:
			if row.Branch && !row.Open {
				list.events = append(list.events, SideEvent{Kind: SideToggle, ID: row.ID})
			}
		case key.NameReturn, key.NameEnter:
			list.events = append(list.events, SideEvent{Kind: SideOpen, ID: row.ID})
		case key.NameF2:
			if row.Menu {
				list.rename = row.ID
			}
		}
	}
}

func (list *SideList) reveal(gtx layout.Context) {
	if list.cursor >= len(list.rows) {
		return
	}
	index := list.rows[list.cursor]
	position := list.Scroller.Position
	if index <= position.First {
		list.Scroller.ScrollTo(index)
	} else if index >= position.First+position.Count-1 {
		list.Scroller.ScrollTo(max(0, index-max(1, position.Count-2)))
	}
	gtx.Execute(op.InvalidateCmd{})
}

func (list *SideList) itemHeight(gtx layout.Context, index int) int {
	item := list.items[index]
	if item.header {
		if item.first {
			return gtx.Dp(sideHeaderLead)
		}
		return gtx.Dp(sideHeader)
	}
	height := gtx.Dp(rowHeight(item.row))
	if state := list.states[item.row.ID]; state != nil {
		height = int(float32(height)*state.appear.Value(gtx) + .5)
	}
	return height
}

func rowHeight(row SideRow) unit.Dp {
	if row.Facts != "" {
		return SideRowTall
	}
	return SideRowHeight
}

// every item height is known so the selected row's place needs no second layout pass
func (list *SideList) selectedRect(gtx layout.Context, selected string, width int) (image.Rectangle, bool) {
	target := -1
	for index, item := range list.items {
		if !item.header && item.row.ID == selected {
			target = index
			break
		}
	}
	if target < 0 || selected == "" {
		return image.Rectangle{}, false
	}
	first := min(max(list.Scroller.Position.First, 0), len(list.items)-1)
	gap := gtx.Dp(SideRowGap)
	y := -list.Scroller.Position.Offset
	for index := first; index < target; index++ {
		y += list.itemHeight(gtx, index) + gap
	}
	for index := target; index < first; index++ {
		y -= list.itemHeight(gtx, index) + gap
	}
	return image.Rect(0, y, width, y+list.itemHeight(gtx, target)), true
}

func (list *SideList) layoutHeader(gtx layout.Context, th *Theme, item sideItem) layout.Dimensions {
	height := gtx.Dp(sideHeader)
	if item.first {
		height = gtx.Dp(sideHeaderLead)
	}
	size := image.Pt(gtx.Constraints.Max.X, height)
	line := gtx.Sp(th.T.Group.Line)
	inset := gtx.Dp(SideInset)
	text := gtx
	text.Constraints = layout.Constraints{Max: image.Pt(max(0, size.X-2*inset), line)}
	offset := op.Offset(image.Pt(inset, height-line-gtx.Dp(4))).Push(gtx.Ops)
	layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(text,
		layout.Flexed(1, kit.Text(th.Theme, th.T.Group, th.R.Fg3, item.title)),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if item.count <= 0 {
				return layout.Dimensions{}
			}
			return kit.Text(th.Theme, th.T.Caption, th.R.Fg3, itoa(item.count))(gtx)
		}),
	)
	offset.Pop()
	return layout.Dimensions{Size: size}
}

func (list *SideList) layoutRow(
	gtx layout.Context,
	th *Theme,
	overlays *Overlays,
	row SideRow,
	selected, cursor bool,
) layout.Dimensions {
	state := list.states[row.ID]
	if state == nil {
		return layout.Dimensions{}
	}
	list.consume(gtx, overlays, row, state)
	full := image.Pt(gtx.Constraints.Max.X, gtx.Dp(rowHeight(row)))
	appear := state.appear.Value(gtx)
	record := op.Record(gtx.Ops)
	rowGTX := gtx
	rowGTX.Constraints = layout.Exact(full)
	state.hover.Preview = list.Preview
	_, hover := state.hover.Layout(rowGTX, th, func(gtx layout.Context) layout.Dimensions {
		return state.click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			semantic.Button.Add(gtx.Ops)
			semantic.LabelOp(row.Title).Add(gtx.Ops)
			semantic.SelectedOp(selected).Add(gtx.Ops)
			list.drawRow(gtx, th, row, state, selected, cursor)
			return layout.Dimensions{Size: full}
		})
	})
	call := record.Stop()
	if appear >= 1 {
		list.drawPlane(gtx, th, state, full, hover, selected, cursor)
		call.Add(gtx.Ops)
		return layout.Dimensions{Size: full}
	}
	visible := image.Pt(full.X, int(float32(full.Y)*appear+.5))
	area := clip.Rect(image.Rectangle{Max: visible}).Push(gtx.Ops)
	opacity := paint.PushOpacity(gtx.Ops, appear)
	call.Add(gtx.Ops)
	opacity.Pop()
	area.Pop()
	return layout.Dimensions{Size: visible}
}

// hover, press and the keyboard cursor only, the selection slab belongs to the sidebar
func (list *SideList) drawPlane(
	gtx layout.Context,
	th *Theme,
	state *sideRowState,
	size image.Point,
	hover float32,
	selected, cursor bool,
) {
	bounds := image.Rectangle{Max: size}
	cut := gtx.Dp(th.Cut.Small)
	pressed := state.click.Pressed() || list.Preview == kit.StatePressed
	press := transition(gtx, &state.press, pressed, th.Motion.Press, th.Motion.Press)
	if !selected && hover > 0 {
		kit.FillCut(gtx.Ops, bounds, cut, palette.WithAlpha(th.R.Stone3, hover))
	}
	if press > 0 {
		kit.FillCut(gtx.Ops, bounds, cut, palette.WithAlpha(th.R.Fg, .08*press))
	}
	if cursor && !selected {
		kit.StrokeCut(gtx.Ops, bounds, cut, max(1, gtx.Dp(1)), th.R.Fg3)
	}
}

func (list *SideList) drawRow(
	gtx layout.Context,
	th *Theme,
	row SideRow,
	state *sideRowState,
	selected, cursor bool,
) {
	size := gtx.Constraints.Max
	title, facts, quiet := th.R.Fg2, th.R.Fg2, th.R.Fg3
	hovered := state.hover.Hovered() || list.Preview == kit.StateHover || list.Preview == kit.StatePressed
	if row.Strong || hovered || cursor {
		title = th.R.Fg
	}
	if selected {
		title, facts, quiet = th.R.OnSel, th.R.OnSel2, th.R.OnSel2
	}
	if row.Tone != palette.Idle {
		kit.Notch(gtx.Ops, image.Point{}, gtx.Dp(8), th.R.Stone(row.Tone))
	}

	x := gtx.Dp(SideInset) + max(row.Depth, 0)*gtx.Dp(SideIndent)
	if row.Branch {
		glyph := Icon.ChevronRight
		if row.Open {
			glyph = Icon.ExpandMore
		}
		box := gtx.Dp(20)
		Place(gtx, image.Rect(x-gtx.Dp(4), (size.Y-box)/2, x-gtx.Dp(4)+box, (size.Y+box)/2), func(gtx layout.Context) layout.Dimensions {
			return state.toggle.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				PlaceIcon(gtx, glyph, image.Pt(gtx.Dp(1), gtx.Dp(1)), 18, quiet)
				return layout.Dimensions{Size: gtx.Constraints.Max}
			})
		})
		x += gtx.Dp(SideIndent)
	}
	if row.Icon != nil {
		pixels := gtx.Dp(18)
		PlaceIcon(gtx, row.Icon, image.Pt(x, (size.Y-pixels)/2), 18, title)
		x += pixels + gtx.Dp(10)
	}

	trailing := list.drawTrailing(gtx, th, row, state, selected, hovered || cursor)
	width := max(0, size.X-x-trailing)
	text := gtx
	text.Constraints = layout.Exact(image.Pt(width, size.Y))
	offset := op.Offset(image.Pt(x, 0)).Push(gtx.Ops)
	switch {
	case list.rename == row.ID:
		list.drawEditor(text, th, row, state)
	case row.Facts != "":
		layout.Flex{Axis: layout.Vertical, Spacing: layout.SpaceSides}.Layout(text,
			layout.Rigid(kit.RawText(th.Theme, th.T.Row, title, row.Title)),
			layout.Rigid(kit.RawText(th.Theme, th.T.Detail, facts, row.Facts)),
		)
	default:
		layout.W.Layout(text, kit.RawText(th.Theme, th.T.Row, title, row.Title))
	}
	offset.Pop()

	pass := pointer.PassOp{}.Push(gtx.Ops)
	area := clip.Rect(image.Rectangle{Max: size}).Push(gtx.Ops)
	event.Op(gtx.Ops, &state.secondary)
	area.Pop()
	pass.Pop()
}

func (list *SideList) drawTrailing(
	gtx layout.Context,
	th *Theme,
	row SideRow,
	state *sideRowState,
	selected, active bool,
) int {
	size := gtx.Constraints.Max
	hasClaim := row.Working || row.Trail != nil
	if !row.Menu && !hasClaim {
		return gtx.Dp(SideInset)
	}
	swap := transition(gtx, &state.swap, active && row.Menu, th.Motion.HoverIn, th.Motion.HoverOut)
	if list.rename == row.ID {
		swap = 0
	}
	reserve := gtx.Dp(sideTrailing)
	if hasClaim && swap < 1 {
		record := op.Record(gtx.Ops)
		claim := gtx
		claim.Constraints = layout.Constraints{Max: image.Pt(size.X/2, size.Y)}
		var dimensions layout.Dimensions
		if row.Working {
			dimensions = kit.Working{Active: true, Size: 16}.Layout(claim, th.Theme)
		} else {
			dimensions = row.Trail(claim)
		}
		call := record.Stop()
		reserve = max(reserve, dimensions.Size.X+gtx.Dp(SideInset)+gtx.Dp(8))
		opacity := paint.PushOpacity(gtx.Ops, 1-swap)
		offset := op.Offset(image.Pt(size.X-gtx.Dp(SideInset)-dimensions.Size.X, (size.Y-dimensions.Size.Y)/2)).Push(gtx.Ops)
		call.Add(gtx.Ops)
		offset.Pop()
		opacity.Pop()
	}
	if swap > 0 {
		box := gtx.Dp(28)
		ink := th.R.Fg2
		if selected {
			ink = th.R.OnSel
		}
		state.action.Icon, state.action.Description = Icon.More, row.Title
		state.action.Preview = kit.StateRest
		opacity := paint.PushOpacity(gtx.Ops, swap)
		Place(gtx, image.Rect(size.X-gtx.Dp(6)-box, (size.Y-box)/2, size.X-gtx.Dp(6), (size.Y+box)/2), func(gtx layout.Context) layout.Dimensions {
			return state.action.Layout(gtx, th, ink)
		})
		opacity.Pop()
	}
	return reserve
}

func (list *SideList) drawEditor(gtx layout.Context, th *Theme, row SideRow, state *sideRowState) {
	if !state.editing {
		state.editor.SetText(row.Title)
		state.editor.SetCaret(len(row.Title), 0)
		state.editing = true
		gtx.Execute(key.FocusCmd{Tag: &state.editor})
	}
	line := gtx.Sp(th.T.Row.Line)
	field := image.Rect(-gtx.Dp(6), (gtx.Constraints.Max.Y-line)/2-gtx.Dp(4), gtx.Constraints.Max.X, (gtx.Constraints.Max.Y+line)/2+gtx.Dp(4))
	kit.FillCut(gtx.Ops, field, gtx.Dp(th.Cut.Token), th.R.Field)
	kit.StrokeCut(gtx.Ops, field, gtx.Dp(th.Cut.Token), max(1, gtx.Dp(1)), th.R.Fg2)
	ink := colorOp(gtx.Ops, th.R.Fg)
	selection := colorOp(gtx.Ops, palette.WithAlpha(th.R.Sel, .6))
	offset := op.Offset(image.Pt(0, (gtx.Constraints.Max.Y-line)/2)).Push(gtx.Ops)
	editor := gtx
	editor.Constraints = layout.Exact(image.Pt(max(0, gtx.Constraints.Max.X-gtx.Dp(6)), line))
	state.editor.LineHeight = th.T.Row.Line
	state.editor.Layout(editor, th.Shaper, th.T.Row.Face, th.T.Row.Size, ink, selection)
	offset.Pop()
}

func (list *SideList) consume(gtx layout.Context, overlays *Overlays, row SideRow, state *sideRowState) {
	if state.toggle.Clicked(gtx) {
		list.events = append(list.events, SideEvent{Kind: SideToggle, ID: row.ID})
	}
	if state.click.Clicked(gtx) {
		list.setCursor(row.ID)
		list.Focus(gtx)
		list.events = append(list.events, SideEvent{Kind: SideOpen, ID: row.ID})
	}
	if state.action.Clicked(gtx) && overlays != nil {
		box := gtx.Dp(28)
		anchor := overlays.AnchorOf(&state.action.Clickable, image.Pt(box, box))
		list.events = append(list.events, SideEvent{Kind: SideMenu, ID: row.ID, Anchor: anchor})
	}
	list.consumeSecondary(gtx, overlays, row, state)
	if list.rename == row.ID {
		list.consumeRename(gtx, row, state)
	}
}

// opens the row menu on release so the press that asked for it can't also dismiss it
func (list *SideList) consumeSecondary(gtx layout.Context, overlays *Overlays, row SideRow, state *sideRowState) {
	for {
		raw, ok := gtx.Event(pointer.Filter{Target: &state.secondary, Kinds: pointer.Press | pointer.Release | pointer.Cancel})
		if !ok {
			return
		}
		pointed, ok := raw.(pointer.Event)
		if !ok {
			continue
		}
		switch pointed.Kind {
		case pointer.Press:
			state.armed = pointed.Buttons == pointer.ButtonSecondary
		case pointer.Cancel:
			state.armed = false
		case pointer.Release:
			if !state.armed {
				continue
			}
			state.armed = false
			if !row.Menu || overlays == nil {
				continue
			}
			at := overlays.Pointer()
			list.events = append(list.events, SideEvent{Kind: SideMenu, ID: row.ID, Anchor: image.Rectangle{Min: at, Max: at}})
		}
	}
}

func (list *SideList) consumeRename(gtx layout.Context, row SideRow, state *sideRowState) {
	finish := func() {
		state.editing, state.focused = false, false
		list.rename = ""
		list.Focus(gtx)
	}
	for {
		raw, ok := gtx.Event(key.Filter{Focus: &state.editor, Name: key.NameEscape})
		if !ok {
			break
		}
		if pressed, ok := raw.(key.Event); ok && pressed.State == key.Press {
			finish()
			return
		}
	}
	for {
		raw, ok := state.editor.Update(gtx)
		if !ok {
			break
		}
		submitted, ok := raw.(widget.SubmitEvent)
		if !ok {
			continue
		}
		if title := strings.TrimSpace(submitted.Text); title != "" && title != row.Title {
			list.events = append(list.events, SideEvent{Kind: SideRename, ID: row.ID, Title: title})
		}
		finish()
		return
	}
	// focus leaving the editor ends the rename and keeps the title
	focused := gtx.Focused(&state.editor)
	if state.focused && !focused {
		state.editing, state.focused = false, false
		list.rename = ""
		return
	}
	state.focused = focused
}

func (list *SideList) setCursor(id string) {
	for cursor, index := range list.rows {
		if list.items[index].row.ID == id {
			list.cursor = cursor
			return
		}
	}
}

func itoa(value int) string { return strconv.Itoa(value) }

func colorOp(ops *op.Ops, ink color.NRGBA) op.CallOp {
	record := op.Record(ops)
	paint.ColorOp{Color: ink}.Add(ops)
	return record.Stop()
}
