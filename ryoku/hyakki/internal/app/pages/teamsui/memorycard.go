package teamsui

import (
	"strings"

	"gioui.org/layout"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/app/pages/memoryui"
	"ryoku-rashin/internal/app/state"
	"ryoku-rashin/internal/protocol"
)

// an import can bring dozens, which would push every other card off the Inbox
const suggestionRows = 5

type suggestionRow struct {
	keep, dismiss kit.Plaque
}

func (card *needCard) suggestionRow(id string) *suggestionRow {
	if card.rows == nil {
		card.rows = make(map[string]*suggestionRow)
	}
	row := card.rows[id]
	if row == nil {
		row = new(suggestionRow)
		card.rows[id] = row
	}
	return row
}

// suggestions from one run come as one card, so a busy team asks once rather than once per fact
func (c *NeedCards) memoryField(gtx layout.Context, e *env.Env, snapshot state.Snapshot, card *needCard, need Need, gold bool) []layout.FlexChild {
	th := e.Theme
	first := need.Memories[0]
	children := []layout.FlexChild{
		layout.Rigid(claimRow(th, palette.Attention, i18n.T("Needs you"), look.Ago(gtx.Now, need.At))),
		layout.Rigid(layout.Spacer{Height: th.Sp.S1}.Layout),
		layout.Rigid(look.Paragraph(th, th.T.Detail, th.R.Fg2, SuggestedBy(snapshot, first))),
		layout.Rigid(layout.Spacer{Height: th.Sp.S2}.Layout),
	}
	if len(need.Memories) == 1 {
		children = append(children, c.oneSuggestion(e, card, need)...)
	} else {
		children = append(children, c.severalSuggestions(gtx, e, snapshot, card, need)...)
	}
	// the task is where the user checks why a team believes this
	if first.Source.Team != "" {
		children = append(children,
			layout.Rigid(layout.Spacer{Height: th.Sp.S2}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return c.layoutWhere(gtx, e, snapshot, card, first.Source.Team, snapshot.TeamTasks[first.Source.Task].Member, first.Source.Task)
			}),
		)
	}
	return append(children,
		layout.Rigid(layout.Spacer{Height: th.Sp.S3}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			plaques := c.memoryPlaques(gtx, e, card, need, gold)
			return look.Flow{Gap: 8, LineGap: 8}.Layout(gtx, len(plaques), func(gtx layout.Context, index int) layout.Dimensions {
				return plaques[index].Layout(gtx, th.Theme)
			})
		}),
	)
}

// a fact is set as the Memory page sets it, so a path in it never shows its backticks
func layoutFact(th *look.Theme, memory protocol.Memory) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			// drawn word by word, so the whole fact needs one name a screen reader can say
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return look.Named(gtx, env.PlainMemory(memory.Text), func(gtx layout.Context) layout.Dimensions {
					return memoryui.FactText(gtx, th, th.R.Fg, memory.Text)
				})
			}),
			layout.Rigid(kit.Text(th.Theme, th.T.Caption, th.R.Fg3, memory.ScopeName)),
		)
	}
}

func (c *NeedCards) oneSuggestion(e *env.Env, card *needCard, need Need) []layout.FlexChild {
	th := e.Theme
	if card.editing {
		card.note.Label = i18n.T("Memory")
		return []layout.FlexChild{layout.Rigid(func(gtx layout.Context) layout.Dimensions { return card.note.Layout(gtx, th.Theme) })}
	}
	return []layout.FlexChild{layout.Rigid(layoutFact(th, need.Memories[0]))}
}

func (c *NeedCards) severalSuggestions(gtx layout.Context, e *env.Env, snapshot state.Snapshot, card *needCard, need Need) []layout.FlexChild {
	th := e.Theme
	waiting := card.waiting(need.Memories)
	shown := waiting
	long := len(waiting) > suggestionRows
	if long && !card.all {
		shown = waiting[:suggestionRows]
	}
	children := []layout.FlexChild{
		layout.Rigid(look.Paragraph(th, th.T.Title, th.R.Fg, i18n.Tf("%d suggestions from %s", len(waiting), SuggestedFrom(snapshot, need.Memories[0])))),
	}
	for _, memory := range shown {
		row := card.suggestionRow(memory.ID)
		row.keep.Kind, row.keep.Small, row.keep.Label = kit.PlaqueTonal, true, i18n.T("Keep")
		row.dismiss.Kind, row.dismiss.Small, row.dismiss.Label = kit.PlaqueQuiet, true, i18n.T("Dismiss")
		if row.keep.Clicked(gtx) {
			c.answerOne(e, card, memory, true)
		}
		if row.dismiss.Clicked(gtx) {
			c.answerOne(e, card, memory, false)
		}
		children = append(children,
			layout.Rigid(layout.Spacer{Height: th.Sp.S3}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints.Min.X = gtx.Constraints.Max.X
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Start}.Layout(gtx,
					layout.Flexed(1, layoutFact(th, memory)),
					layout.Rigid(layout.Spacer{Width: th.Sp.S3}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions { return row.keep.Layout(gtx, th.Theme) }),
					layout.Rigid(layout.Spacer{Width: th.Sp.S1}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions { return row.dismiss.Layout(gtx, th.Theme) }),
				)
			}),
		)
	}
	if long {
		card.more.Kind, card.more.Small = kit.PlaqueQuiet, true
		card.more.Label = i18n.Tf("Show all %d", len(waiting))
		if card.all {
			card.more.Label = i18n.T("Show less")
		}
		if card.more.Clicked(gtx) {
			card.all = !card.all
		}
		children = append(children,
			layout.Rigid(layout.Spacer{Height: th.Sp.S2}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions { return card.more.Layout(gtx, th.Theme) }),
		)
	}
	return children
}

// Keep is the card's one filled act, and gold only on the oldest open card so a page holds one gold
func (c *NeedCards) memoryPlaques(gtx layout.Context, e *env.Env, card *needCard, need Need, gold bool) []*kit.Plaque {
	card.approve.Kind, card.approve.Small = kit.PlaqueTonal, true
	if gold {
		card.approve.Kind = kit.PlaqueGold
	}
	card.always.Kind, card.always.Small, card.always.Label = kit.PlaqueTonal, true, i18n.T("Edit and keep")
	card.deny.Kind, card.deny.Small, card.deny.Label = kit.PlaqueQuiet, true, i18n.T("Dismiss")
	key := NeedKey(need)
	if len(need.Memories) > 1 {
		waiting := card.waiting(need.Memories)
		card.approve.Label, card.deny.Label = i18n.T("Keep all"), i18n.T("Dismiss all")
		if card.approve.Clicked(gtx) {
			c.answerAll(e, key, need, waiting, true)
		}
		if card.deny.Clicked(gtx) {
			c.answerAll(e, key, need, waiting, false)
		}
		return []*kit.Plaque{&card.approve, &card.deny}
	}
	memory := need.Memories[0]
	card.approve.Label = i18n.T("Keep")
	if card.editing {
		card.deny.Label = i18n.T("Cancel")
		if card.approve.Clicked(gtx) {
			c.keepEdited(e, card, key, need)
		}
		if card.deny.Clicked(gtx) {
			card.editing, card.note.Error = false, ""
		}
		return []*kit.Plaque{&card.approve, &card.deny}
	}
	if card.approve.Clicked(gtx) {
		c.answerAll(e, key, need, need.Memories, true)
	}
	if card.always.Clicked(gtx) {
		card.editing, card.note.Error = true, ""
		card.note.SetText(memory.Text)
	}
	if card.deny.Clicked(gtx) {
		c.answerAll(e, key, need, need.Memories, false)
	}
	return []*kit.Plaque{&card.approve, &card.always, &card.deny}
}

// the edit lands before the keep, so a refused edit never keeps the old text
func (c *NeedCards) keepEdited(e *env.Env, card *needCard, key string, need Need) {
	memory := need.Memories[0]
	text := strings.TrimSpace(card.note.Text())
	if problem := memoryui.TextProblem(text); problem != "" {
		card.note.Error = problem
		return
	}
	card.note.Error = ""
	edited := []protocol.Memory{memory}
	edited[0].Text = text
	c.fold(key, need, foldedLine(edited, true))
	e.ChangeMemory(protocol.MMemoryEdit, protocol.MemoryEditParams{ID: memory.ID, Text: &text}, func(_ protocol.MemoryChange, err error) {
		if err != nil {
			c.undo(e, key)(err)
			return
		}
		card.editing = false
		c.answered(e, key, []string{memory.ID}, true)
	})
}

// the card folds before the daemon answers, so a second press can't answer it twice
func (c *NeedCards) answerAll(e *env.Env, key string, need Need, memories []protocol.Memory, keep bool) {
	ids := make([]string, len(memories))
	for index, memory := range memories {
		ids[index] = memory.ID
	}
	c.fold(key, need, foldedLine(memories, keep))
	c.answered(e, key, ids, keep)
}

// undoing brings the card back with the suggestions
func (c *NeedCards) answered(e *env.Env, key string, ids []string, keep bool) {
	method := protocol.MMemoryDismiss
	if keep {
		method = protocol.MMemoryKeep
	}
	line := answeredPlacard(len(ids), keep)
	e.ChangeMemory(method, protocol.MemoryIDsParams{IDs: ids}, func(change protocol.MemoryChange, err error) {
		if err != nil {
			c.undo(e, key)(err)
			return
		}
		if e.NotifyAction == nil || change.ID == "" {
			e.Notify(line)
			return
		}
		e.NotifyAction(line, i18n.T("Undo"), func() {
			e.UndoMemory(change.ID)
			delete(c.answers, key)
		})
	})
}

// answering one row leaves the rest of the card waiting
func (c *NeedCards) answerOne(e *env.Env, card *needCard, memory protocol.Memory, keep bool) {
	if card.answered == nil {
		card.answered = make(map[string]bool)
	}
	card.answered[memory.ID] = true
	method := protocol.MMemoryDismiss
	if keep {
		method = protocol.MMemoryKeep
	}
	e.ChangeMemory(method, protocol.MemoryIDsParams{IDs: []string{memory.ID}}, func(change protocol.MemoryChange, err error) {
		if err != nil {
			delete(card.answered, memory.ID)
			e.Report(err)
			return
		}
		e.NotifyChange(answeredPlacard(1, keep), change)
	})
}

func (card *needCard) waiting(memories []protocol.Memory) []protocol.Memory {
	if len(card.answered) == 0 {
		return memories
	}
	waiting := make([]protocol.Memory, 0, len(memories))
	for _, memory := range memories {
		if !card.answered[memory.ID] {
			waiting = append(waiting, memory)
		}
	}
	return waiting
}

// the placard says the verb the Memory page's placards say
func answeredPlacard(count int, keep bool) string {
	switch {
	case count == 1 && keep:
		return i18n.T("Kept.")
	case count == 1:
		return i18n.T("Dismissed.")
	case keep:
		return i18n.Tf("Kept %s memories.", look.Count(count))
	}
	return i18n.Tf("Dismissed %s suggestions.", look.Count(count))
}

// the folded card stays for the session, so one fact is named to tell the cards apart
func foldedLine(memories []protocol.Memory, keep bool) string {
	if len(memories) != 1 {
		return answeredPlacard(len(memories), keep)
	}
	text := env.PlainMemory(memories[0].Text)
	if keep {
		return i18n.Tf("Kept \"%s\".", text)
	}
	return i18n.Tf("Dismissed \"%s\".", text)
}

// the Inbox list and the card word who suggests the same way
func SuggestedBy(snapshot state.Snapshot, memory protocol.Memory) string {
	source := memory.Source
	switch {
	case source.Team != "":
		return i18n.Tf("%s suggests remembering", TeamName(snapshot.Teams, source.Team))
	case source.Kind == protocol.MemoryFromImport && source.From != "":
		return i18n.Tf("Imported from %s", source.From)
	case source.Kind == protocol.MemoryFromImport:
		return i18n.T("Imported from another assistant")
	case source.Session != "":
		return i18n.Tf("Suggested in %s", SuggestedFrom(snapshot, memory))
	}
	return i18n.T("Suggested for your memory")
}

// a task's title reads better than a session id, so it wins when the task is known
func SuggestedFrom(snapshot state.Snapshot, memory protocol.Memory) string {
	source := memory.Source
	if task, ok := snapshot.TeamTasks[source.Task]; ok {
		return task.Title
	}
	if info, ok := snapshot.Sessions[source.Session]; ok && info.Title != "" {
		return info.Title
	}
	if source.Title != "" {
		return source.Title
	}
	if source.From != "" {
		return source.From
	}
	return i18n.T("another assistant")
}
