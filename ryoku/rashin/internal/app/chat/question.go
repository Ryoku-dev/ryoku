package chat

import (
	"image"
	"strings"

	"gioui.org/io/event"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/protocol"
)

type messageTarget struct {
	armed bool
}

func (state *viewState) layoutQuestion(gtx layout.Context, th *look.Theme, user protocol.UserData, key string) layout.Dimensions {
	content := func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return layout.E.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Max.X = max(1, gtx.Constraints.Max.X*3/4)
			children := make([]layout.FlexChild, 0, 6)
			if user.Text != "" {
				children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return state.layoutQuestionSlab(gtx, th, "user:"+key, th.T.Proof, user.Text)
				}))
			}
			// a command the reader ran stands where a question would, as typed
			if user.Run != "" {
				children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return state.layoutQuestionSlab(gtx, th, "user:"+key+":run", th.T.Mono, "$ "+user.Run)
				}))
			}
			for _, note := range questionNotes(user) {
				children = append(children,
					layout.Rigid(layout.Spacer{Height: th.Sp.S1}.Layout),
					layout.Rigid(kit.Text(th.Theme, th.T.Caption, th.R.Fg3, note)),
				)
			}
			if user.Handoff != nil {
				children = append(children,
					layout.Rigid(layout.Spacer{Height: th.Sp.S2}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return state.layoutHandoff(gtx, th, user.Handoff, "user:"+key+":handoff")
					}),
				)
			}
			return layout.Flex{Axis: layout.Vertical, Alignment: layout.End}.Layout(gtx, children...)
		})
	}
	return state.rise("user:"+key).Layout(gtx, th, key, 8, content)
}

func (state *viewState) layoutQuestionSlab(gtx layout.Context, th *look.Theme, key string, style look.TextStyle, text string) layout.Dimensions {
	record := op.Record(gtx.Ops)
	dimensions := layout.Inset{Top: 8, Bottom: 8, Left: 13, Right: 13}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return state.selectable(gtx, th, key, style, th.R.Fg, text)
	})
	call := record.Stop()
	kit.FillCut(gtx.Ops, image.Rectangle{Max: dimensions.Size}, gtx.Dp(th.Cut.Mid), th.R.Stone3)
	call.Add(gtx.Ops)
	return dimensions
}

func questionNotes(user protocol.UserData) []string {
	notes := make([]string, 0, 1+len(user.Images))
	if user.Terminal != nil && user.Terminal.Cwd != "" {
		notes = append(notes, i18n.Tf("From a terminal in %s", user.Terminal.Cwd))
	}
	for _, attachment := range user.Images {
		notes = append(notes, attachmentName(attachment.Path))
	}
	return notes
}

func (state *viewState) layoutHandoff(gtx layout.Context, th *look.Theme, handoff *protocol.Handoff, key string) layout.Dimensions {
	title := i18n.T("Moved from an ask")
	if source := strings.TrimSpace(handoff.Title); source != "" {
		title = i18n.Tf("Moved from “%s”", source)
	}
	return slab(gtx, th, th.R.Stone1, 13, func(gtx layout.Context) layout.Dimensions {
		children := []layout.FlexChild{
			layout.Rigid(kit.Text(th.Theme, th.T.Caption, th.R.Fg3, title)),
			layout.Rigid(layout.Spacer{Height: th.Sp.S1}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return state.selectable(gtx, th, key+":question", th.T.Body, th.R.Fg, handoff.Question)
			}),
		}
		if handoff.Answer != "" {
			answer, _ := firstLines(handoff.Answer, 6)
			children = append(children,
				layout.Rigid(layout.Spacer{Height: th.Sp.S2}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return state.selectable(gtx, th, key+":answer", th.T.Body, th.R.Fg2, answer)
				}),
			)
		}
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
	})
}

// the menu opens on release so the press that asked for it can't also dismiss it
func (state *viewState) layoutMessage(
	gtx layout.Context,
	th *look.Theme,
	key string,
	seq int64,
	text string,
	canFork bool,
	content layout.Widget,
) layout.Dimensions {
	target := state.messages[key]
	if target == nil {
		target = new(messageTarget)
		state.messages[key] = target
	}
	for {
		raw, ok := gtx.Event(pointer.Filter{Target: target, Kinds: pointer.Press | pointer.Release | pointer.Cancel})
		if !ok {
			break
		}
		pointed, ok := raw.(pointer.Event)
		if !ok {
			continue
		}
		switch pointed.Kind {
		case pointer.Press:
			target.armed = pointed.Buttons == pointer.ButtonSecondary
		case pointer.Cancel:
			target.armed = false
		case pointer.Release:
			if target.armed {
				target.armed = false
				state.openMessageMenu(th, seq, text, canFork)
			}
		}
	}
	dimensions := content(gtx)
	pass := pointer.PassOp{}.Push(gtx.Ops)
	area := clip.Rect(image.Rectangle{Max: dimensions.Size}).Push(gtx.Ops)
	event.Op(gtx.Ops, target)
	area.Pop()
	pass.Pop()
	return dimensions
}

func (state *viewState) openMessageMenu(th *look.Theme, seq int64, text string, canFork bool) {
	if state.overlays == nil {
		return
	}
	items := []kit.MenuItem{{Icon: look.Icon.Copy, Label: i18n.T("Copy")}}
	actions := []Action{{Kind: ActCopy, Value: text}}
	// with words marked, those are what a copy most likely means
	if marked := state.markedText(); marked != "" {
		items = []kit.MenuItem{
			{Icon: look.Icon.Copy, Label: i18n.T("Copy selection")},
			{Icon: look.Icon.Quote, Label: i18n.T("Quote in the reply")},
			{Icon: look.Icon.Copy, Label: i18n.T("Copy all of it")},
		}
		actions = []Action{{Kind: ActCopy, Value: marked}, {Kind: ActQuote, Value: marked}, {Kind: ActCopy, Value: text}}
	}
	if canFork {
		items = append(items, kit.MenuItem{Icon: look.Icon.Fork, Label: i18n.T("Fork from here")})
		actions = append(actions, Action{Kind: ActFork, Seq: seq})
	}
	tag := &state.menu
	state.overlays.OpenAt(tag, state.overlays.Pointer(), func(gtx layout.Context) layout.Dimensions {
		dimensions := state.menu.Layout(gtx, th.Theme, items)
		if item, ok := state.menu.Clicked(gtx); ok {
			state.overlays.Close(tag)
			state.act(actions[item])
		}
		return dimensions
	})
}
