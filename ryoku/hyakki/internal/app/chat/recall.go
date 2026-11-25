package chat

import (
	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/layout"
)

type recallState struct {
	// how many prompts back the field shows, 0 for the reader's own draft
	back int
	text string
}

func (state *composerState) processRecall(gtx layout.Context, options ComposerOptions) {
	text := state.editor.Text()
	if state.recall.back > 0 && text != state.recall.text {
		// edited, so it's a draft of its own now
		state.recall = recallState{}
	}
	if options.Disabled || options.History == nil || state.slash.open {
		return
	}
	recalling := state.recall.back > 0
	if !recalling && (text != "" || len(state.images) > 0) {
		return
	}
	// only asked for while they recall, the rest of the time the arrows move the caret
	filters := []event.Filter{key.Filter{Focus: &state.editor, Name: key.NameUpArrow}}
	if recalling {
		filters = append(filters, key.Filter{Focus: &state.editor, Name: key.NameDownArrow})
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
		history := options.History()
		back := state.recall.back - 1
		if pressed.Name == key.NameUpArrow {
			back = min(state.recall.back+1, len(history))
		}
		state.recall = recallState{back: max(0, min(back, len(history)))}
		if state.recall.back > 0 {
			state.recall.text = history[len(history)-state.recall.back]
		}
		state.setText(state.recall.text)
	}
}
