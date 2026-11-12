package kit

import (
	"image"
	"image/color"

	"gioui.org/gesture"
	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/io/semantic"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/unit"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/anim"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/symbols"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/theme"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/tokens"
)

var (
	iconListEdit   = Icon(symbols.Edit)
	iconListRemove = Icon(symbols.Delete)
	iconListDrag   = Icon(symbols.DragIndicator)
	iconListAdd    = Icon(symbols.Add)
)

// ListEditorNew is the index Editing and Opened report for an item being added.
const ListEditorNew = -1

// ListItem is the text one ListEditor row shows. The host composes it, so a
// rule can read "Firefox: floats, 80% opacity".
type ListItem struct {
	Title  string
	Detail string
}

type listRow struct {
	handle Click
	edit   IconPlaque
	remove IconPlaque
	drag   gesture.Drag
	focus  anim.Tween
}

// ListEditor is an editable list inside a settings slab. It never changes the
// host's items: it reports additions, edits, removals and moves, and the host
// applies them. The host draws the inline editor; while it is open, the host
// sets EditorValid, and lists its single-line fields in EditorFields so Enter
// in one means Done and Escape means Cancel.
type ListEditor struct {
	Label        string
	AddText      string
	DoneText     string
	EmptyText    string
	Reorderable  bool
	EditorValid  bool
	EditorFields []event.Tag
	Preview      State
	PreviewDrag  unit.Dp
	// Plane is the surface the list stands on, the slab when zero; a dragged
	// row paints it so the rows it passes do not show through.
	Plane color.NRGBA

	rows                []listRow
	add, cancel, done   Plaque
	open                bool
	editing             int
	focusNext           event.Tag
	openedIndex         int
	openedReady         bool
	added               bool
	editedIndex         int
	editedReady         bool
	removedIndex        int
	removedReady        bool
	moveFrom, moveTo    int
	movedReady          bool
	cancelled           bool
	dragging            bool
	dragFrom            int
	dragStart, dragDiff float32
	tops, heights       []int
}

// Edit opens the inline editor under item index, closing any other without
// saving.
func (l *ListEditor) Edit(index int) {
	l.open, l.editing = true, index
	l.openedIndex, l.openedReady = index, true
	l.dragging = false
}

// Add opens the inline editor for a new item in place of the add row.
func (l *ListEditor) Add() { l.Edit(ListEditorNew) }

// Editing reports the item whose editor is open, ListEditorNew for a new one.
func (l *ListEditor) Editing() (int, bool) { return l.editing, l.open }

// Opened reports and clears the item whose editor just opened, so the host can
// load a copy of it into its editor.
func (l *ListEditor) Opened() (int, bool) {
	index, ok := l.openedIndex, l.openedReady
	l.openedReady = false
	return index, ok
}

// Submit acts as Done: with a valid editor it reports the addition or edit and
// closes the editor. It returns whether it did.
func (l *ListEditor) Submit() bool {
	if !l.open || !l.EditorValid {
		return false
	}
	l.open = false
	if l.editing == ListEditorNew {
		l.added = true
		l.focusNext = &l.add.Clickable
		return true
	}
	l.editedIndex, l.editedReady = l.editing, true
	if l.editing >= 0 && l.editing < len(l.rows) {
		l.focusNext = &l.rows[l.editing].edit.Clickable
	}
	return true
}

// Cancel closes the editor without saving and reports whether one was open,
// so an AppFrame's DismissOverlay can route Escape to it.
func (l *ListEditor) Cancel() bool {
	if !l.open {
		return false
	}
	l.open, l.cancelled = false, true
	if l.editing == ListEditorNew {
		l.focusNext = &l.add.Clickable
	} else if l.editing >= 0 && l.editing < len(l.rows) {
		l.focusNext = &l.rows[l.editing].edit.Clickable
	}
	return true
}

// Added reports and clears a confirmed new item; the host reads it from its
// editor.
func (l *ListEditor) Added() bool {
	added := l.added
	l.added = false
	return added
}

// Edited reports and clears a confirmed edit of an item.
func (l *ListEditor) Edited() (int, bool) {
	index, ok := l.editedIndex, l.editedReady
	l.editedReady = false
	return index, ok
}

// Removed reports and clears a removal; it is immediate, so offer Undo.
func (l *ListEditor) Removed() (int, bool) {
	index, ok := l.removedIndex, l.removedReady
	l.removedReady = false
	return index, ok
}

// Moved reports and clears a requested move from one index to another.
func (l *ListEditor) Moved() (from, to int, ok bool) {
	from, to, ok = l.moveFrom, l.moveTo, l.movedReady
	l.movedReady = false
	return from, to, ok
}

// Cancelled reports and clears an editor closed without saving.
func (l *ListEditor) Cancelled() bool {
	cancelled := l.cancelled
	l.cancelled = false
	return cancelled
}

func (l *ListEditor) removeAt(index, count int) {
	l.removedIndex, l.removedReady = index, true
	if l.open {
		switch {
		case l.editing == index:
			l.open = false
		case l.editing > index:
			l.editing--
		}
	}
	switch {
	case index+1 < count:
		l.focusNext = &l.rows[index+1].remove.Clickable
	case index > 0:
		l.focusNext = &l.rows[index-1].remove.Clickable
	default:
		l.focusNext = &l.add.Clickable
	}
}

func (l *ListEditor) moveItem(from, to int) {
	l.moveFrom, l.moveTo, l.movedReady = from, to, true
}

// listKeyMove returns where Alt+Up or Alt+Down moves item index in a list of
// count, and whether it moves at all.
func listKeyMove(index, count int, up bool) (int, bool) {
	to := index + 1
	if up {
		to = index - 1
	}
	if to < 0 || to >= count {
		return index, false
	}
	return to, true
}

// listMoveTarget returns the index a dragged row lands on: the number of other
// rows whose centre lies above the dragged row's centre.
func listMoveTarget(tops, heights []int, from, offset int) int {
	if from < 0 || from >= len(tops) {
		return from
	}
	centre := tops[from] + heights[from]/2 + offset
	target := 0
	for index := range tops {
		if index != from && tops[index]+heights[index]/2 < centre {
			target++
		}
	}
	return target
}

func (l *ListEditor) ensure(count int) {
	if len(l.rows) != count {
		rows := make([]listRow, count)
		copy(rows, l.rows)
		l.rows = rows
	}
	if cap(l.tops) < count {
		l.tops, l.heights = make([]int, count), make([]int, count)
	}
	l.tops, l.heights = l.tops[:count], l.heights[:count]
}

func (l *ListEditor) keyMoves(gtx layout.Context, index, count int, tag event.Tag, next func(row *listRow) event.Tag) {
	for {
		value, ok := gtx.Event(
			key.Filter{Focus: tag, Name: key.NameUpArrow, Required: key.ModAlt},
			key.Filter{Focus: tag, Name: key.NameDownArrow, Required: key.ModAlt},
		)
		if !ok {
			return
		}
		pressed, ok := value.(key.Event)
		if !ok || pressed.State != key.Press || !l.Reorderable || l.open {
			continue
		}
		if to, moves := listKeyMove(index, count, pressed.Name == key.NameUpArrow); moves {
			l.moveItem(index, to)
			// the host moves the item, so focus follows it to its new row
			l.focusNext = next(&l.rows[to])
		}
	}
}

func (l *ListEditor) update(gtx layout.Context, count int) {
	if l.focusNext != nil {
		gtx.Execute(key.FocusCmd{Tag: l.focusNext})
		l.focusNext = nil
	}
	if l.open {
		for _, tag := range l.EditorFields {
			for {
				value, ok := gtx.Event(
					key.Filter{Focus: tag, Name: key.NameReturn},
					key.Filter{Focus: tag, Name: key.NameEnter},
					key.Filter{Focus: tag, Name: key.NameEscape},
				)
				if !ok {
					break
				}
				if pressed, ok := value.(key.Event); ok && pressed.State == key.Press {
					if pressed.Name == key.NameEscape {
						l.Cancel()
					} else {
						l.Submit()
					}
				}
			}
		}
		for _, tag := range [...]event.Tag{&l.done.Clickable, &l.cancel.Clickable} {
			for {
				value, ok := gtx.Event(key.Filter{Focus: tag, Name: key.NameEscape})
				if !ok {
					break
				}
				if pressed, ok := value.(key.Event); ok && pressed.State == key.Press {
					l.Cancel()
				}
			}
		}
	}
	if l.cancel.Clicked(gtx) {
		l.Cancel()
	}
	if l.done.Clicked(gtx) {
		l.Submit()
	}
	if l.add.Clicked(gtx) {
		l.Add()
	}
	for index := range count {
		row := &l.rows[index]
		for row.handle.Clicked(gtx) {
		}
		if row.edit.Clicked(gtx) {
			l.Edit(index)
		}
		if row.remove.Clicked(gtx) {
			l.removeAt(index, count)
		}
		l.keyMoves(gtx, index, count, &row.handle.Clickable, func(row *listRow) event.Tag { return &row.handle.Clickable })
		l.keyMoves(gtx, index, count, &row.edit.Clickable, func(row *listRow) event.Tag { return &row.edit.Clickable })
		l.keyMoves(gtx, index, count, &row.remove.Clickable, func(row *listRow) event.Tag { return &row.remove.Clickable })
		for {
			value, ok := row.drag.Update(gtx.Metric, gtx.Source, gesture.Vertical)
			if !ok {
				break
			}
			switch value.Kind {
			case pointer.Press:
				if l.Reorderable && !l.open {
					l.dragging, l.dragFrom = true, index
					l.dragStart, l.dragDiff = value.Position.Y, 0
				}
			case pointer.Drag:
				if l.dragging && l.dragFrom == index {
					l.dragDiff = value.Position.Y - l.dragStart
					gtx.Execute(op.InvalidateCmd{})
				}
			case pointer.Release:
				if l.dragging && l.dragFrom == index {
					l.dragging = false
					if to := listMoveTarget(l.tops, l.heights, index, int(l.dragDiff)); to != index {
						l.moveItem(index, to)
					}
					gtx.Execute(op.InvalidateCmd{})
				}
			case pointer.Cancel:
				l.dragging = false
			}
		}
	}
	if l.dragging && l.dragFrom >= count {
		l.dragging = false
	}
}

// Layout draws the items, the open editor and the add row. editor draws the
// host's fields for the item at index, ListEditorNew for a new one; trailing
// draws an optional control for item index. Either may be nil.
func (l *ListEditor) Layout(gtx layout.Context, th *theme.Theme, items []ListItem, editor, trailing func(gtx layout.Context, index int) layout.Dimensions) layout.Dimensions {
	count := len(items)
	l.ensure(count)
	l.update(gtx, count)
	width := gtx.Constraints.Max.X
	gap := gtx.Dp(tokens.SizeSettingsRowGap)
	hairline := max(1, gtx.Dp(unit.Dp(tokens.SizeHairline)))
	handleWidth, handleHeight := gtx.Dp(tokens.SizeIconPlaqueIcon), gtx.Dp(tokens.SizeIconPlaqueSmall)
	vein := func(y int) {
		line := y + (gap-hairline)/2
		FillCut(gtx.Ops, image.Rect(0, line, width, line+hairline), 0, th.R.Vein)
	}
	dragFrom, dragOffset := -1, 0
	switch {
	case l.Reorderable && count > 0 && l.PreviewDrag != 0:
		dragFrom, dragOffset = 0, gtx.Dp(l.PreviewDrag)
	case l.dragging:
		dragFrom, dragOffset = l.dragFrom, int(l.dragDiff+.5)
	}

	all := op.Record(gtx.Ops)
	var draggedCall op.CallOp
	y := 0
	for index, item := range items {
		if index > 0 {
			vein(y)
			y += gap
		}
		record := op.Record(gtx.Ops)
		dims := l.layoutRow(gtx, th, index, item, trailing)
		call := record.Stop()
		l.tops[index], l.heights[index] = y, dims.Size.Y
		if index == dragFrom {
			draggedCall = call
		} else {
			offset := op.Offset(image.Pt(0, y)).Push(gtx.Ops)
			call.Add(gtx.Ops)
			offset.Pop()
		}
		if l.Reorderable {
			// the drag area stays put while the row follows the pointer
			handleY := y + (dims.Size.Y-handleHeight)/2
			pass := pointer.PassOp{}.Push(gtx.Ops)
			area := clip.Rect(image.Rect(0, handleY, handleWidth, handleY+handleHeight)).Push(gtx.Ops)
			pointer.CursorGrab.Add(gtx.Ops)
			l.rows[index].drag.Add(gtx.Ops)
			area.Pop()
			pass.Pop()
		}
		y += dims.Size.Y
		if l.open && l.editing == index {
			y += gtx.Dp(th.Sp.S2)
			y += l.layoutEditor(gtx, th, y, "Edit "+item.Title, index, editor)
		}
	}

	if count > 0 {
		vein(y)
		y += gap
	} else if l.EmptyText != "" {
		offset := op.Offset(image.Pt(0, y)).Push(gtx.Ops)
		lineGTX := gtx
		lineGTX.Constraints = layout.Constraints{Max: image.Pt(width, gtx.Constraints.Max.Y)}
		empty := Label{Style: th.T.Body, Color: th.R.Fg2}.Layout(lineGTX, th, l.EmptyText)
		offset.Pop()
		y += empty.Size.Y + gtx.Dp(th.Sp.S2)
	}
	addText := l.AddText
	if addText == "" {
		addText = "Add"
	}
	if l.open && l.editing == ListEditorNew {
		y += l.layoutEditor(gtx, th, y, addText, ListEditorNew, editor)
	} else {
		l.add.Kind, l.add.Icon, l.add.Label = PlaqueQuiet, iconListAdd, addText
		offset := op.Offset(image.Pt(0, y)).Push(gtx.Ops)
		rowGTX := gtx
		rowGTX.Constraints = layout.Constraints{Min: image.Pt(0, gtx.Dp(tokens.SizeFactsRowHeight)), Max: image.Pt(width, gtx.Constraints.Max.Y)}
		dims := centerY(rowGTX, func(gtx layout.Context) layout.Dimensions { return l.add.Layout(gtx, th) })
		offset.Pop()
		y += dims.Size.Y
	}

	if dragFrom >= 0 && dragFrom < count {
		top := l.tops[dragFrom] + dragOffset
		bleedX, bleedY := gtx.Dp(th.Sp.S2), gtx.Dp(th.Sp.S1)
		plane := image.Rect(-bleedX, top-bleedY, width+bleedX, top+l.heights[dragFrom]+bleedY)
		backing := l.Plane
		if backing == (color.NRGBA{}) {
			backing = th.R.Slab
		}
		FillCut(gtx.Ops, plane, gtx.Dp(th.Cut.Small), disabledInk(gtx, backing))
		FillCut(gtx.Ops, plane, gtx.Dp(th.Cut.Small), disabledInk(gtx, palette.WithAlpha(th.R.Fg, tokens.AlphaHoverPlane)))
		offset := op.Offset(image.Pt(0, top)).Push(gtx.Ops)
		draggedCall.Add(gtx.Ops)
		offset.Pop()
	}
	call := all.Stop()

	size := image.Pt(width, y)
	area := clip.Rect(image.Rectangle{Max: size}).Push(gtx.Ops)
	if l.Label != "" {
		semantic.LabelOp(l.Label).Add(gtx.Ops)
	}
	area.Pop()
	call.Add(gtx.Ops)
	return layout.Dimensions{Size: size}
}

func (l *ListEditor) layoutRow(gtx layout.Context, th *theme.Theme, index int, item ListItem, trailing func(gtx layout.Context, index int) layout.Dimensions) layout.Dimensions {
	row := &l.rows[index]
	width := gtx.Constraints.Max.X
	preview := StateRest
	if index == 0 {
		preview = l.Preview
	}
	row.edit.Kind, row.edit.Small, row.edit.Icon = PlaqueQuiet, true, iconListEdit
	row.edit.Description = "Edit " + item.Title
	row.remove.Kind, row.remove.Small, row.remove.Icon = PlaqueQuiet, true, iconListRemove
	row.remove.Description = "Remove " + item.Title
	row.edit.Preview = StateRest
	if !l.Reorderable {
		row.edit.Preview = preview
	}

	children := make([]layout.FlexChild, 0, 9)
	if l.Reorderable {
		children = append(children,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return l.layoutHandle(gtx, th, row, item.Title, preview)
			}),
			layout.Rigid(layout.Spacer{Width: th.Sp.S3}.Layout),
		)
	}
	children = append(children,
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return listItemText(gtx, th, item)
		}),
		layout.Rigid(layout.Spacer{Width: th.Sp.S4}.Layout),
	)
	if trailing != nil {
		children = append(children,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions { return trailing(gtx, index) }),
			layout.Rigid(layout.Spacer{Width: th.Sp.S2}.Layout),
		)
	}
	children = append(children,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions { return row.edit.Layout(gtx, th) }),
		layout.Rigid(layout.Spacer{Width: th.Sp.S2}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions { return row.remove.Layout(gtx, th) }),
	)
	rowGTX := gtx
	rowGTX.Constraints = layout.Constraints{Min: image.Pt(width, gtx.Dp(tokens.SizeFactsRowHeight)), Max: image.Pt(width, gtx.Constraints.Max.Y)}
	record := op.Record(gtx.Ops)
	dims := centerY(rowGTX, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx, children...)
	})
	call := record.Stop()
	dims.Size.X = width
	area := clip.Rect(image.Rectangle{Max: dims.Size}).Push(gtx.Ops)
	semantic.LabelOp(item.Title).Add(gtx.Ops)
	if item.Detail != "" {
		semantic.DescriptionOp(item.Detail).Add(gtx.Ops)
	}
	call.Add(gtx.Ops)
	area.Pop()
	return dims
}

func (l *ListEditor) layoutHandle(gtx layout.Context, th *theme.Theme, row *listRow, title string, preview State) layout.Dimensions {
	size := image.Pt(gtx.Dp(tokens.SizeIconPlaqueIcon), gtx.Dp(tokens.SizeIconPlaqueSmall))
	dims := row.handle.Layout(exactContext(gtx, size), func(gtx layout.Context) layout.Dimensions {
		semantic.Button.Add(gtx.Ops)
		semantic.LabelOp("Move " + title).Add(gtx.Ops)
		semantic.EnabledOp(gtx.Source.Enabled()).Add(gtx.Ops)
		icon := gtx.Dp(tokens.SizeIconPlaqueIcon)
		offset := op.Offset(image.Pt((size.X-icon)/2, (size.Y-icon)/2)).Push(gtx.Ops)
		iconListDrag.Layout(exactContext(gtx, image.Pt(icon, icon)), disabledInk(gtx, th.R.Fg3))
		offset.Pop()
		return layout.Dimensions{Size: size}
	})
	focus := previewTransition(gtx, &row.focus, preview, StateFocus, gtx.Focused(&row.handle.Clickable), th.Motion.FocusIn, th.Motion.FocusOut)
	FocusRingAlpha(gtx, th, image.Rectangle{Max: dims.Size}, gtx.Dp(th.Cut.Token), focus)
	return dims
}

// listItemText takes the whole width it is given so the controls sit at the
// right of the row.
func listItemText(gtx layout.Context, th *theme.Theme, item ListItem) layout.Dimensions {
	width := gtx.Constraints.Max.X
	gtx.Constraints.Min = image.Point{}
	title := Label{Style: th.T.Row, Color: th.R.Fg}.Layout(gtx, th, item.Title)
	if item.Detail == "" {
		return layout.Dimensions{Size: image.Pt(width, title.Size.Y), Baseline: title.Baseline}
	}
	y := title.Size.Y + gtx.Dp(tokens.SizeSettingsDetailGap)
	offset := op.Offset(image.Pt(0, y)).Push(gtx.Ops)
	detail := Label{Style: th.T.Detail, Color: th.R.Fg2}.Layout(gtx, th, item.Detail)
	offset.Pop()
	return layout.Dimensions{Size: image.Pt(width, y+detail.Size.Y), Baseline: title.Baseline}
}

// layoutEditor draws the field-tinted editor plane at top with the host's
// fields and the Cancel and Done actions, and returns its height.
func (l *ListEditor) layoutEditor(gtx layout.Context, th *theme.Theme, top int, name string, index int, editor func(gtx layout.Context, index int) layout.Dimensions) int {
	width := gtx.Constraints.Max.X
	bleed, pad := gtx.Dp(th.Sp.S2), gtx.Dp(th.Sp.S3)
	inner := max(0, width+2*bleed-2*pad)
	done := l.DoneText
	if done == "" {
		done = "Add"
	}
	if index != ListEditorNew {
		done = "Done"
	}
	l.cancel.Kind, l.cancel.Small, l.cancel.Label = PlaqueQuiet, true, "Cancel"
	l.done.Kind, l.done.Small, l.done.Label = PlaqueLine, true, done

	contentGTX := gtx
	contentGTX.Constraints = layout.Constraints{Min: image.Pt(inner, 0), Max: image.Pt(inner, gtx.Constraints.Max.Y)}
	record := op.Record(gtx.Ops)
	content := layout.Flex{Axis: layout.Vertical}.Layout(contentGTX,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min.X = 0
			if editor == nil {
				return layout.Dimensions{}
			}
			return editor(gtx, index)
		}),
		layout.Rigid(layout.Spacer{Height: th.Sp.S3}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return layout.Dimensions{Size: image.Pt(gtx.Constraints.Min.X, 0)}
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions { return l.cancel.Layout(gtx, th) }),
				layout.Rigid(layout.Spacer{Width: th.Sp.S2}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					if !l.EditorValid {
						gtx = gtx.Disabled()
					}
					return l.done.Layout(gtx, th)
				}),
			)
		}),
	)
	call := record.Stop()
	height := content.Size.Y + 2*pad
	plane := image.Rect(-bleed, top, width+bleed, top+height)
	FillCut(gtx.Ops, plane, gtx.Dp(th.Cut.Small), disabledInk(gtx, th.R.Field))
	area := clip.Rect(plane).Push(gtx.Ops)
	semantic.LabelOp(name).Add(gtx.Ops)
	offset := op.Offset(image.Pt(-bleed+pad, top+pad)).Push(gtx.Ops)
	call.Add(gtx.Ops)
	offset.Pop()
	area.Pop()
	return height
}
