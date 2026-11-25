package chat

import (
	"strconv"
	"strings"

	"gioui.org/layout"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/protocol"
)

type AnswerOptions struct {
	SessionID    string
	Epoch        int
	Choices      func(string) ([]look.SelectOption, []string, int)
	Call         func(string, any, any, func(error))
	Attach       func(string, func(error))
	Session      func(string) *Model
	Navigate     func(string)
	Quote        func(string)
	Notify       func(string)
	NotifyAction func(string, string, func())
	OpenFolder   func(string)
}

type compareVisual struct {
	source   string
	picking  bool
	busy     bool
	err      string
	models   [2]string
	selects  [2]look.Select
	result   protocol.CompareStartResult
	selected int
	lanes    map[string]*compareLaneVisual
	checking bool
	pending  int
	ended    bool
}

type compareLaneVisual struct {
	state viewState
	seq   int64
	text  string
	ready bool
	busy  bool
}

func (view *View) SetAnswers(options AnswerOptions) {
	state := view.ensureState()
	if previous := state.answerSettings(); previous.SessionID != options.SessionID || previous.Epoch != options.Epoch {
		for _, visual := range state.turns {
			visual.compare = nil
		}
	}
	state.turnVisual(-1).answers = options
}

func (state *viewState) answerSettings() AnswerOptions {
	return state.turnVisual(-1).answers
}

func (state *viewState) hasComparison() bool {
	for _, visual := range state.turns {
		if visual.compare != nil && (visual.compare.picking || (!visual.compare.checking && !visual.compare.ended && len(visual.compare.result.Lanes) > 0)) {
			return true
		}
	}
	return false
}

func (state *viewState) comparisonFor(block blockContext) bool {
	visual := state.turnVisual(block.turn).compare
	return visual != nil && visual.source == block.info.ID && !visual.checking && !visual.ended && len(visual.result.Lanes) > 0
}

func (state *viewState) syncComparison(block blockContext) {
	result, ok := block.model.comparisons[block.row.seq]
	if !ok || result.Source != block.info.ID {
		return
	}
	visual := state.turnVisual(block.turn)
	if visual.compare != nil && visual.compare.result.ID == result.ID {
		return
	}
	state.restoreComparison(block, result)
}

func (state *viewState) restoreComparison(block blockContext, result protocol.CompareStartResult) {
	visual := &compareVisual{source: block.info.ID, result: result, lanes: make(map[string]*compareLaneVisual)}
	state.turnVisual(block.turn).compare = visual
	if len(result.Lanes) == 0 {
		return
	}
	options := state.answerSettings()
	if options.Attach == nil {
		for _, lane := range result.Lanes {
			if options.Session == nil || options.Session(lane.Session) == nil {
				visual.ended = true
				visual.result.Lanes = nil
				visual.err = i18n.T("This comparison has ended. Your original answer is still here.")
				break
			}
		}
		return
	}
	visual.checking, visual.pending = true, len(result.Lanes)
	for _, lane := range result.Lanes {
		options.Attach(lane.Session, func(err error) {
			if state.turnVisual(block.turn).compare != visual || visual.ended {
				return
			}
			if err != nil {
				visual.checking, visual.ended = false, true
				visual.result.Lanes = nil
				visual.err = i18n.T("This comparison has ended. Your original answer is still here.")
				return
			}
			visual.pending--
			visual.checking = visual.pending > 0
		})
	}
}

func (state *viewState) answerPlaque(gtx layout.Context, th *look.Theme, key, label string, clicked func()) layout.Dimensions {
	plaque := state.plaque(key)
	plaque.Kind, plaque.Small, plaque.Label = kit.PlaqueTonal, true, label
	if plaque.Clicked(gtx) {
		clicked()
	}
	return plaque.Layout(gtx, th.Theme)
}

func (state *viewState) openCompare(block blockContext) {
	visual := state.turnVisual(block.turn)
	if visual.compare == nil || visual.compare.source != block.info.ID {
		visual.compare = &compareVisual{source: block.info.ID, lanes: make(map[string]*compareLaneVisual)}
	}
	visual.compare.picking = true
	state.follow = false
	state.scroller.SetFollow(false)
}

func (state *viewState) layoutComparison(gtx layout.Context, th *look.Theme, block blockContext) layout.Dimensions {
	visual := state.turnVisual(block.turn).compare
	if !state.feature("compare") || visual == nil || visual.source != block.info.ID || len(visual.result.Lanes) == 0 {
		return layout.Dimensions{}
	}
	options := state.answerSettings()
	key := "compare:" + block.info.ID + ":" + strconv.Itoa(block.turn)
	return layout.Inset{Top: th.Sp.S3, Bottom: th.Sp.S2, Left: th.Sp.S3, Right: th.Sp.S3}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		wide := gtx.Constraints.Max.X >= gtx.Dp(900)
		if !wide {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return look.Flow{Gap: th.Sp.S2, LineGap: th.Sp.S2}.Layout(gtx, len(visual.result.Lanes), func(gtx layout.Context, index int) layout.Dimensions {
						lane := visual.result.Lanes[index]
						label := lane.Model
						if label == "" {
							label = i18n.T("Original answer")
						}
						plaque := state.plaque(key + ":tab:" + lane.Session)
						plaque.Kind, plaque.Small, plaque.Label = kit.PlaqueQuiet, true, label
						if index == visual.selected {
							plaque.Kind = kit.PlaqueTonal
						}
						if plaque.Clicked(gtx) {
							visual.selected = index
						}
						return plaque.Layout(gtx, th.Theme)
					})
				}),
				layout.Rigid(layout.Spacer{Height: th.Sp.S2}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return state.layoutCompareLane(gtx, th, visual, visual.result.Lanes[visual.selected], key, options)
				}),
			)
		}
		columns := make([]layout.FlexChild, 0, 2*len(visual.result.Lanes)-1)
		for index, lane := range visual.result.Lanes {
			if index > 0 {
				columns = append(columns, layout.Rigid(layout.Spacer{Width: th.Sp.S3}.Layout))
			}
			columns = append(columns, layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints.Min.X = gtx.Constraints.Max.X
				return state.layoutCompareLane(gtx, th, visual, lane, key, options)
			}))
		}
		return layout.Flex{Axis: layout.Horizontal}.Layout(gtx, columns...)
	})
}

func (state *viewState) layoutComparePicker(gtx layout.Context, th *look.Theme, block blockContext, visual *compareVisual, key string) layout.Dimensions {
	options := state.answerSettings()
	gtx.Constraints.Min.X = 0
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(kit.Text(th.Theme, th.T.Detail, th.R.Fg2, i18n.T("Choose up to two other models"))),
		layout.Rigid(layout.Spacer{Height: th.Sp.S2}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return look.Flow{Gap: th.Sp.S3, LineGap: th.Sp.S2}.Layout(gtx, 2, func(gtx layout.Context, index int) layout.Dimensions {
				if options.Choices == nil {
					return layout.Dimensions{}
				}
				rows, ids, selected := options.Choices(visual.models[index])
				rows = append([]look.SelectOption{{Label: i18n.T("Not selected")}}, rows...)
				ids = append([]string{""}, ids...)
				for at, id := range ids {
					if id != "" && (id == block.info.Model || id == visual.models[1-index]) {
						rows[at].Disabled = true
					}
				}
				if visual.models[index] == "" {
					selected = 0
				} else {
					selected++
				}
				picker := &visual.selects[index]
				picker.Label, picker.Form, picker.Searched = i18n.Tf("Comparison model %d", index+1), look.SelectQuiet, true
				dimensions := picker.LayoutOptions(gtx, th, state.overlays, rows, selected)
				if chosen, changed := picker.Changed(); changed && chosen >= 0 && chosen < len(ids) {
					visual.models[index] = ids[chosen]
				}
				return dimensions
			})
		}),
		layout.Rigid(layout.Spacer{Height: th.Sp.S3}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					gtx.Constraints.Min.X = 0
					if visual.busy || visual.models == [2]string{} || options.Call == nil {
						gtx = gtx.Disabled()
					}
					return state.answerPlaque(gtx, th, key+":start", i18n.T("Compare answers"), func() {
						visual.busy, visual.err = true, ""
						models := []string{}
						for _, model := range visual.models {
							if model != "" {
								models = append(models, model)
							}
						}
						var result protocol.CompareStartResult
						options.Call(protocol.MCompareStart, protocol.CompareStartParams{ID: block.info.ID, Seq: block.row.seq, Models: models}, &result, func(err error) {
							visual.busy = false
							if err != nil {
								visual.err = err.Error()
								return
							}
							state.restoreComparison(block, result)
						})
					})
				}),
				layout.Rigid(layout.Spacer{Width: th.Sp.S2}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					gtx.Constraints.Min.X = 0
					return state.answerPlaque(gtx, th, key+":cancel", i18n.T("Cancel"), func() { visual.picking = false })
				}),
			)
		}),
	)
}

func (state *viewState) layoutCompareLane(gtx layout.Context, th *look.Theme, visual *compareVisual, lane protocol.CompareLane, key string, options AnswerOptions) layout.Dimensions {
	var model *Model
	if options.Session != nil {
		model = options.Session(lane.Session)
	}
	laneVisual := visual.lanes[lane.Session]
	if laneVisual == nil {
		laneVisual = &compareLaneVisual{busy: true}
		laneVisual.state.ensure()
		visual.lanes[lane.Session] = laneVisual
	}
	if model != nil {
		model.mu.RLock()
		if model.state != nil && (!laneVisual.ready || laneVisual.seq != model.state.lastSeq) {
			var parts []string
			for _, row := range model.state.rows {
				if row.kind == rowUser {
					parts = parts[:0]
				}
				if row.kind == rowText {
					parts = append(parts, row.text)
				}
				if row.kind == rowError {
					parts = append(parts, row.err.Message)
				}
			}
			laneVisual.text, laneVisual.busy = strings.Join(parts, "\n\n"), model.state.activeTurn != nil
			laneVisual.seq, laneVisual.ready = model.state.lastSeq, true
		}
		model.mu.RUnlock()
	}
	text, busy := laneVisual.text, laneVisual.busy
	label := lane.Model
	if label == "" {
		label = i18n.T("Original answer")
	}
	return state.slab(gtx, th, th.Sp.S3, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(kit.Text(th.Theme, th.T.Detail, th.R.Fg2, label)),
			layout.Rigid(layout.Spacer{Height: th.Sp.S3}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if text == "" {
					return kit.Text(th.Theme, th.T.Body, th.R.Fg3, i18n.T("Waiting for an answer"))(gtx)
				}
				return laneVisual.state.layoutMarkdown(gtx, th, key+":"+lane.Session, text, nil)
			}),
			layout.Rigid(layout.Spacer{Height: th.Sp.S3}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if busy {
					return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return kit.Working{Active: true, Size: 16}.Layout(gtx, th.Theme)
						}),
						layout.Rigid(layout.Spacer{Width: th.Sp.S2}.Layout),
						layout.Rigid(kit.Text(th.Theme, th.T.Caption, th.R.Fg3, i18n.T("Answering"))),
					)
				}
				if visual.busy || options.Call == nil {
					gtx = gtx.Disabled()
				}
				return state.answerPlaque(gtx, th, key+":keep:"+lane.Session, i18n.T("Keep this one"), func() {
					visual.busy = true
					var kept protocol.SessionInfo
					options.Call(protocol.MCompareKeep, protocol.CompareKeepParams{ID: visual.result.ID, Session: lane.Session}, &kept, func(err error) {
						visual.busy = false
						if err != nil {
							visual.err = err.Error()
							return
						}
						visual.result = protocol.CompareStartResult{}
						if options.Navigate != nil {
							options.Navigate(kept.ID)
						}
					})
				})
			}),
		)
	})
}
