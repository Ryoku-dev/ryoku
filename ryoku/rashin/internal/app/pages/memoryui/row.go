package memoryui

import (
	"image"
	"image/color"
	"strings"

	"gioui.org/io/key"
	"gioui.org/io/semantic"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/unit"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/protocol"
)

// RowView is how a row stands: whether it names its scope (under All),
// whether it is the selected one, and what the engine can do.
type RowView struct {
	ShowScope bool
	Selected  bool
	Caps      protocol.MemoryCaps
	// Teams name the team a suggestion came from
	Teams []protocol.Team
}

// RowAct is what a row asks of the list holding it.
type RowAct uint8

const (
	RowNone RowAct = iota
	// RowSelect: the row was pressed or took the keyboard
	RowSelect
	// RowEdit: Edit, Enter or a double press
	RowEdit
	// RowPrevious and RowNext: Up and Down moved off it
	RowPrevious
	RowNext
)

// Row is one memory as a tablet that grows with its fact. It forgets, pins,
// keeps, dismisses and restores by itself; editing is the list's to open.
type Row struct {
	click  kit.Click
	source kit.Click

	edit, pin, forget          look.IconAction
	editTip, pinTip, forgetTip look.Tip
	keep, dismiss, restore     kit.Plaque
}

// Rows holds a list's rows by memory id.
type Rows struct {
	rows map[string]*Row
}

// Row is the row of a memory, made the first time it is asked for.
func (rows *Rows) Row(id string) *Row {
	if rows.rows == nil {
		rows.rows = make(map[string]*Row)
	}
	row, ok := rows.rows[id]
	if !ok {
		row = new(Row)
		rows.rows[id] = row
	}
	return row
}

// Keep lets go of the rows of every memory not in ids.
func (rows *Rows) Keep(ids []string) {
	kept := make(map[string]bool, len(ids))
	for _, id := range ids {
		kept[id] = true
	}
	for id := range rows.rows {
		if !kept[id] {
			delete(rows.rows, id)
		}
	}
}

// Focus gives the row the keyboard.
func (row *Row) Focus(gtx layout.Context) {
	gtx.Execute(key.FocusCmd{Tag: &row.click.Clickable})
}

func (row *Row) handle(gtx layout.Context, e *env.Env, memory protocol.Memory, view RowView) RowAct {
	act := RowNone
	tag := &row.click.Clickable
	for {
		event, ok := gtx.Event(
			key.Filter{Focus: tag, Name: key.NameUpArrow},
			key.Filter{Focus: tag, Name: key.NameDownArrow},
			key.Filter{Focus: tag, Name: key.NameReturn},
			key.Filter{Focus: tag, Name: key.NameEnter},
			key.Filter{Focus: tag, Name: key.NameDeleteForward},
			key.Filter{Focus: tag, Name: key.NameDeleteBackward},
			key.Filter{Focus: tag, Name: "P"},
			key.Filter{Focus: tag, Name: "K"},
			key.Filter{Focus: tag, Name: "D"},
		)
		if !ok {
			break
		}
		pressed, ok := event.(key.Event)
		if !ok || pressed.State != key.Press {
			continue
		}
		switch pressed.Name {
		case key.NameUpArrow:
			act = RowPrevious
		case key.NameDownArrow:
			act = RowNext
		case key.NameReturn, key.NameEnter:
			act = RowEdit
		case key.NameDeleteForward, key.NameDeleteBackward:
			e.ForgetMemory(memory)
		case "P":
			if view.Caps.Pin && memory.Status == protocol.MemoryActive {
				Pin(e, memory, !memory.Pinned)
			}
		case "K", "D":
			if memory.Status == protocol.MemorySuggested {
				e.AnswerSuggestions([]string{memory.ID}, pressed.Name == "K")
			}
		}
	}
	for {
		click, ok := row.click.Update(gtx)
		if !ok {
			break
		}
		// a pressed row takes the keyboard, so its keys work at once
		row.Focus(gtx)
		if act == RowNone {
			act = RowSelect
		}
		if click.NumClicks >= 2 {
			act = RowEdit
		}
	}
	if row.edit.Clicked(gtx) {
		act = RowEdit
	}
	if row.pin.Clicked(gtx) {
		Pin(e, memory, !memory.Pinned)
	}
	if row.forget.Clicked(gtx) {
		e.ForgetMemory(memory)
	}
	if row.keep.Clicked(gtx) {
		e.AnswerSuggestions([]string{memory.ID}, true)
	}
	if row.dismiss.Clicked(gtx) {
		e.AnswerSuggestions([]string{memory.ID}, false)
	}
	if row.restore.Clicked(gtx) {
		Restore(e, memory.ID, 0)
	}
	if row.source.Clicked(gtx) {
		OpenSource(e, memory.Source)
	}
	return act
}

// Pin pins or unpins a memory and says so with Undo.
func Pin(e *env.Env, memory protocol.Memory, pinned bool) {
	text := i18n.T("Unpinned.")
	if pinned {
		text = i18n.T("Pinned.")
	}
	e.ChangeMemory(protocol.MMemoryEdit, protocol.MemoryEditParams{ID: memory.ID, Pinned: &pinned}, func(change protocol.MemoryChange, err error) {
		if err != nil {
			e.Report(err)
			return
		}
		e.NotifyChange(text, change)
	})
}

// Restore brings back a replaced memory (version 0) or an earlier text.
func Restore(e *env.Env, id string, version int) {
	e.ChangeMemory(protocol.MMemoryRestore, protocol.MemoryRestoreParams{ID: id, Version: version}, func(change protocol.MemoryChange, err error) {
		if err != nil {
			e.Report(err)
			return
		}
		e.NotifyChange(i18n.T("Restored."), change)
	})
}

// OpenSource opens the conversation, task or run a memory came from.
func OpenSource(e *env.Env, source protocol.MemorySource) {
	switch {
	case source.Task != "" && e.NavigateTeams != nil:
		e.NavigateTeams(env.TeamsPlace{Tab: protocol.TabWork, Task: source.Task})
	case source.Session != "" && source.Kind == protocol.MemoryFromAgent:
		e.Navigate(protocol.TabAgent, source.Session)
	case source.Session != "":
		e.Navigate(protocol.TabAsk, source.Session)
	}
}

func opensSource(source protocol.MemorySource) bool {
	return source.Task != "" || source.Session != ""
}

// Layout draws the row and reports what it asks of its list.
func (row *Row) Layout(gtx layout.Context, e *env.Env, memory protocol.Memory, view RowView) (layout.Dimensions, RowAct) {
	th := e.Theme
	act := row.handle(gtx, e, memory, view)
	width := gtx.Constraints.Max.X
	// the acts stand over the row, so pointing at one must not count as leaving it
	hovered := row.click.Hovered() || row.edit.Hovered() || row.pin.Hovered() || row.forget.Hovered() ||
		row.source.Hovered() || gtx.Focused(&row.click.Clickable)
	showActs := hovered || view.Selected
	ink, quiet := th.R.Fg, th.R.Fg3
	if view.Selected {
		ink, quiet = th.R.OnSel, th.R.OnSel2
	}
	narrow := width < gtx.Dp(520)

	padX, padY := gtx.Dp(16), gtx.Dp(10)
	content := gtx
	content.Constraints = layout.Constraints{Max: image.Pt(max(0, width-2*padX), gtx.Constraints.Max.Y)}
	record := op.Record(gtx.Ops)
	inner := layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(content,
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			children := []layout.FlexChild{
				layout.Rigid(func(gtx layout.Context) layout.Dimensions { return FactText(gtx, th, ink, memory.Text) }),
				layout.Rigid(layout.Spacer{Height: 2}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions { return row.layoutProof(gtx, e, memory, view, quiet) }),
			}
			if narrow {
				if answers := row.answers(th, memory); answers != nil {
					children = append(children, layout.Rigid(layout.Spacer{Height: th.Sp.S2}.Layout), layout.Rigid(answers))
				}
			}
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
		}),
		layout.Rigid(layout.Spacer{Width: th.Sp.S3}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return row.layoutTrailing(gtx, e, memory, view, showActs, narrow)
		}),
	)
	call := record.Stop()
	size := image.Pt(width, max(gtx.Dp(56), inner.Size.Y+2*padY))
	bounds := image.Rectangle{Max: size}

	fill := th.R.Stone2
	switch {
	case view.Selected:
		fill = th.R.Sel
	case hovered:
		fill = th.R.Stone3
	}
	kit.FillCut(gtx.Ops, bounds, 0, fill)
	if notch := notchInk(th, memory.Status); notch != (color.NRGBA{}) {
		kit.Notch(gtx.Ops, image.Point{}, gtx.Dp(th.Cut.Notch), notch)
	}
	area := clip.Rect(bounds).Push(gtx.Ops)
	row.click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		semantic.Button.Add(gtx.Ops)
		semantic.LabelOp(env.PlainMemory(memory.Text)).Add(gtx.Ops)
		semantic.SelectedOp(view.Selected).Add(gtx.Ops)
		return layout.Dimensions{Size: size}
	})
	area.Pop()
	offset := op.Offset(image.Pt(padX, (size.Y-inner.Size.Y)/2)).Push(gtx.Ops)
	call.Add(gtx.Ops)
	offset.Pop()
	if gtx.Focused(&row.click.Clickable) && !view.Selected {
		kit.FocusRingAlpha(gtx, th.Theme, bounds, 0, 1)
	}
	return layout.Dimensions{Size: size}, act
}

func notchInk(th *look.Theme, status string) color.NRGBA {
	switch status {
	case protocol.MemorySuggested:
		return th.R.Stone(palette.Attention)
	case protocol.MemoryReplaced, protocol.MemoryBlocked:
		return th.R.Vein
	}
	return color.NRGBA{}
}

// the proof sentence with the source's title as a link, then the marks set apart by space
func (row *Row) layoutProof(gtx layout.Context, e *env.Env, memory protocol.Memory, view RowView, quiet color.NRGBA) layout.Dimensions {
	th := e.Theme
	parts := ProofOf(gtx.Now, memory)
	var pieces []layout.Widget
	words := func(list []Word) {
		for _, word := range list {
			pieces = append(pieces, kit.RawText(th.Theme, th.T.Caption, quiet, word.Text))
		}
	}
	words(Words(parts.Before))
	after := Words(parts.After)
	if parts.Title != "" {
		// the comma after the title belongs to it, not to the next word
		title, stuck := parts.Title, ""
		if len(after) > 0 && strings.ContainsAny(after[0].Text[:1], ",.;:") {
			stuck, after = after[0].Text, after[1:]
		}
		link := func(gtx layout.Context) layout.Dimensions {
			if !opensSource(memory.Source) {
				return kit.RawText(th.Theme, th.T.Caption, quiet, title)(gtx)
			}
			return row.source.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				semantic.Button.Add(gtx.Ops)
				semantic.LabelOp(title).Add(gtx.Ops)
				dimensions := kit.RawText(th.Theme, th.T.Caption, quiet, title)(gtx)
				if row.source.Hovered() {
					line := max(1, gtx.Dp(1))
					look.FillRect(gtx, image.Rect(0, dimensions.Size.Y-line, dimensions.Size.X, dimensions.Size.Y), quiet)
				}
				return dimensions
			})
		}
		pieces = append(pieces, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Baseline}.Layout(gtx,
				layout.Rigid(link),
				layout.Rigid(kit.RawText(th.Theme, th.T.Caption, quiet, stuck)),
			)
		})
	}
	words(after)
	var marks []string
	if memory.Pinned {
		marks = append(marks, i18n.T("Pinned"))
	}
	if view.ShowScope && memory.ScopeName != "" {
		marks = append(marks, memory.ScopeName)
	}
	if memory.Sensitive {
		marks = append(marks, i18n.T("Sensitive"))
	}
	for _, mark := range marks {
		pieces = append(pieces, layout.Spacer{Width: th.Sp.S2}.Layout, kit.RawText(th.Theme, th.T.Caption, quiet, mark))
	}
	return look.Flow{Gap: unit.Dp(3.5), LineGap: 1}.Layout(gtx, len(pieces), func(gtx layout.Context, index int) layout.Dimensions {
		return pieces[index](gtx)
	})
}

// the state's claim, or the row's acts under the pointer, and the answers a state wants
func (row *Row) layoutTrailing(gtx layout.Context, e *env.Env, memory protocol.Memory, view RowView, showActs, narrow bool) layout.Dimensions {
	th := e.Theme
	var children []layout.FlexChild
	if showActs {
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions { return row.layoutActs(gtx, e, memory, view) }))
	} else if claim, proof := StateWords(gtx.Now, memory, view.Teams); claim != "" {
		claimInk, proofInk := th.R.Fg2, th.R.Fg3
		if memory.Status == protocol.MemorySuggested {
			claimInk = th.R.Stone(palette.Attention)
		}
		if view.Selected {
			claimInk, proofInk = th.R.OnSel, th.R.OnSel2
		}
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Max.X = min(gtx.Constraints.Max.X, gtx.Dp(220))
			return layout.Flex{Axis: layout.Vertical, Alignment: layout.End}.Layout(gtx,
				layout.Rigid(kit.Text(th.Theme, th.T.Label, claimInk, claim)),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return kit.Label{Style: th.T.Caption, Color: proofInk, MaxLines: 1}.Layout(gtx, th.Theme, proof)
				}),
			)
		}))
	}
	if !narrow {
		if answers := row.answers(th, memory); answers != nil {
			children = append(children, layout.Rigid(layout.Spacer{Width: th.Sp.S3}.Layout), layout.Rigid(answers))
		}
	}
	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx, children...)
}

// Keep and Dismiss for a suggestion, Restore for a replaced memory; always shown, they are the row's point
func (row *Row) answers(th *look.Theme, memory protocol.Memory) layout.Widget {
	switch memory.Status {
	case protocol.MemorySuggested:
		row.keep.Kind, row.keep.Small, row.keep.Label = kit.PlaqueLine, true, i18n.T("Keep")
		row.dismiss.Kind, row.dismiss.Small, row.dismiss.Label = kit.PlaqueQuiet, true, i18n.T("Dismiss")
		return func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions { return row.keep.Layout(gtx, th.Theme) }),
				layout.Rigid(layout.Spacer{Width: th.Sp.S2}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions { return row.dismiss.Layout(gtx, th.Theme) }),
			)
		}
	case protocol.MemoryReplaced:
		row.restore.Kind, row.restore.Small, row.restore.Label = kit.PlaqueQuiet, true, i18n.T("Restore")
		return func(gtx layout.Context) layout.Dimensions { return row.restore.Layout(gtx, th.Theme) }
	}
	return nil
}

func (row *Row) layoutActs(gtx layout.Context, e *env.Env, memory protocol.Memory, view RowView) layout.Dimensions {
	th := e.Theme
	ink := th.R.Fg2
	if view.Selected {
		ink = th.R.OnSel
	}
	row.edit.Icon, row.edit.Description = look.Icon.Rename, i18n.T("Edit")
	row.forget.Icon, row.forget.Description = look.Icon.Delete, i18n.T("Forget")
	row.pin.Icon, row.pin.Description = look.Icon.Pin, i18n.T("Pin")
	pinTip := i18n.T("Pin")
	if memory.Pinned {
		row.pin.Icon, row.pin.Description, pinTip = look.Icon.Pinned, i18n.T("Unpin"), i18n.T("Unpin")
	}
	row.pin.Disabled = !view.Caps.Pin || memory.Status != protocol.MemoryActive
	if !view.Caps.Pin {
		pinTip = i18n.T("This engine can't pin memories.")
	}
	children := []layout.FlexChild{}
	if memory.Status == protocol.MemoryActive {
		children = append(children,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return row.editTip.Layout(gtx, th, e.Overlays, i18n.T("Edit"), func(gtx layout.Context) layout.Dimensions { return row.edit.Layout(gtx, th, ink) })
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return row.pinTip.Layout(gtx, th, e.Overlays, pinTip, func(gtx layout.Context) layout.Dimensions { return row.pin.Layout(gtx, th, ink) })
			}),
		)
	}
	children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
		return row.forgetTip.Layout(gtx, th, e.Overlays, i18n.T("Forget"), func(gtx layout.Context) layout.Dimensions { return row.forget.Layout(gtx, th, ink) })
	}))
	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx, children...)
}

// FactText sets a memory's text in body type, wrapping by word, with the
// words the engine marked literal in mono.
func FactText(gtx layout.Context, th *look.Theme, ink color.NRGBA, text string) layout.Dimensions {
	words := Words(text)
	if len(words) == 0 {
		return layout.Dimensions{}
	}
	return look.Flow{Gap: 4, LineGap: 2}.Layout(gtx, len(words), func(gtx layout.Context, index int) layout.Dimensions {
		style := th.T.Body
		if words[index].Literal {
			style = th.T.Mono
		}
		return kit.RawText(th.Theme, style, ink, words[index].Text)(gtx)
	})
}
