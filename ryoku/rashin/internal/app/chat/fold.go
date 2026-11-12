package chat

import (
	"strconv"

	"gioui.org/layout"

	"ryoku-i18n"

	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/protocol"
)

// foldInfo is a finished turn's work folded behind its first step: how long
// the turn took and how much it did
type foldInfo struct {
	turn     *turnState
	steps    int
	thoughts int
}

func isWorkRow(model *modelState, row *transcriptRow) bool {
	switch row.kind {
	case rowTool, rowThought:
		return true
	case rowTerminal:
		return model.terminalOwners[row.key] == ""
	}
	return false
}

// shownLines is how many lines a finished tool row shows: one for a lone call
// or a group of calls, and with subagents a box for each call that ran them
// besides a group for the calls between
func shownLines(tools []*protocol.ToolData) int {
	lines, grouping := 0, false
	for _, tool := range tools {
		switch {
		case tool == nil:
		case len(tool.Agents) > 0:
			lines, grouping = lines+1, false
		case !grouping:
			lines, grouping = lines+1, true
		}
	}
	return lines
}

// foldTurns finds each finished turn whose work shows more than one line and
// marks its first work row as the place the turn's work folds into
func (state *viewState) foldTurns(model *modelState) map[int]*foldInfo {
	folds := map[int]*foldInfo{}
	anchor := -1
	var current *foldInfo
	lines := 0
	flush := func(end *turnState) {
		if current != nil && end != nil && end.end != nil && lines > 1 {
			current.turn = end
			folds[anchor] = current
		}
		anchor, current, lines = -1, nil, 0
	}
	for index := range model.rows {
		row := &model.rows[index]
		switch {
		case row.kind == rowUser:
			flush(nil)
		case row.kind == rowTurnEnd:
			flush(row.turn)
		case isWorkRow(model, row):
			if current == nil {
				anchor, current = index, &foldInfo{}
			}
			switch row.kind {
			case rowThought:
				current.thoughts++
				lines++
			case rowTool:
				current.steps += len(row.tools)
				lines += shownLines(row.tools)
			default:
				current.steps++
				lines++
			}
		}
	}
	return folds
}

func foldKey(anchor int) string { return "fold:" + strconv.Itoa(anchor) }

// the line a finished turn's work folds behind: "Worked for 2 min 14 s",
// what it did, and on a click every step
func (state *viewState) layoutFold(gtx layout.Context, th *look.Theme, anchor int, fold *foldInfo, row layout.Widget) layout.Dimensions {
	visual := state.workVisual(foldKey(anchor))
	label := i18n.T("Worked")
	if !fold.turn.began.IsZero() && fold.turn.ended.After(fold.turn.began) {
		label = i18n.Tf("Worked for %s", waitedFor(fold.turn.ended.Sub(fold.turn.began)))
		if fold.turn.end.Stop == protocol.StopCancelled {
			label = i18n.Tf("You stopped it after %s", waitedFor(fold.turn.ended.Sub(fold.turn.began)))
		}
	}
	var steps, thoughts string
	switch {
	case fold.steps == 1:
		steps = i18n.T("1 step")
	case fold.steps > 1:
		steps = i18n.Tf("%d steps", fold.steps)
	}
	switch {
	case fold.thoughts == 1:
		thoughts = i18n.T("1 thought")
	case fold.thoughts > 1:
		thoughts = i18n.Tf("%d thoughts", fold.thoughts)
	}
	prose := steps + thoughts
	if steps != "" && thoughts != "" {
		prose = i18n.Tf("%s and %s", steps, thoughts)
	}
	line := workLine{icon: look.Icon.Steps, label: label, prose: prose, expandable: true}
	head := func(gtx layout.Context) layout.Dimensions { return state.layoutLine(gtx, th, visual, line) }
	if !visual.open {
		return head(gtx)
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(head),
		layout.Rigid(layout.Spacer{Height: 2}.Layout),
		layout.Rigid(row),
	)
}
