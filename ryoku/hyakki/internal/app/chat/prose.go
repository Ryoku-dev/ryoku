package chat

import (
	"image"

	"gioui.org/io/event"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op/clip"

	"ryoku-rashin/internal/app/look"
)

// Prose sets one Markdown text the way an answer reads in a transcript, for a
// page that shows it in a column of its own, such as a team's report. It does
// not scroll: the page around it does. Copying, links and code raise the same
// Actions a View does.
type Prose struct {
	state *viewState
}

// Layout draws source at the width it is given.
func (p *Prose) Layout(gtx layout.Context, th *look.Theme, overlays *look.Overlays, source string) layout.Dimensions {
	if p.state == nil {
		p.state = &viewState{}
	}
	state := p.state
	state.ensure()
	state.overlays = overlays
	state.releaseMarked(gtx)
	state.watchPresses(gtx)
	dimensions := state.layoutMarkdown(gtx, th, "prose", source, nil)
	pass := pointer.PassOp{}.Push(gtx.Ops)
	area := clip.Rect(image.Rectangle{Max: dimensions.Size}).Push(gtx.Ops)
	event.Op(gtx.Ops, &state.presses)
	area.Pop()
	pass.Pop()
	state.dropUnseenSets()
	return dimensions
}

// Actions returns and clears the actions recorded since the last call.
func (p *Prose) Actions() []Action {
	if p.state == nil {
		return nil
	}
	return p.state.takeActions()
}
