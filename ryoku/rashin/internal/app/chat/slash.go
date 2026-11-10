package chat

import (
	"image"
	"strings"

	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/theme"

	"ryoku-rashin/internal/protocol"
)

const (
	slashLimit = 6
	slashRow   = 36
)

type slashState struct {
	open    bool
	cursor  int
	matches []protocol.Command
	rows    [slashLimit]struct{}
	// the text Escape shut the list at, it stays shut until that changes
	dismissed string
}

func slashQuery(text string) (string, bool) {
	if !strings.HasPrefix(text, "/") || strings.ContainsAny(text, " \t\n") {
		return "", false
	}
	return strings.ToLower(text[1:]), true
}

func commandName(command protocol.Command) string { return strings.TrimPrefix(command.Name, "/") }

func matchCommands(commands []protocol.Command, query string) []protocol.Command {
	var leading, inside []protocol.Command
	for _, command := range commands {
		name := strings.ToLower(commandName(command))
		switch {
		case strings.HasPrefix(name, query):
			leading = append(leading, command)
		case strings.Contains(name, query):
			inside = append(inside, command)
		}
	}
	matches := append(leading, inside...)
	return matches[:min(len(matches), slashLimit)]
}

func (state *composerState) processSlash(gtx layout.Context, options ComposerOptions) {
	slash := &state.slash
	text := state.editor.Text()
	if text != slash.dismissed {
		slash.dismissed = ""
	}
	query, typing := slashQuery(text)
	slash.open = typing && options.Commands != nil && !options.Disabled &&
		slash.dismissed == "" && state.recall.back == 0 && gtx.Focused(&state.editor)
	if slash.open {
		slash.matches = matchCommands(options.Commands(), query)
		slash.open = len(slash.matches) > 0
	}
	if !slash.open {
		slash.cursor = 0
		return
	}
	slash.cursor = min(slash.cursor, len(slash.matches)-1)
	for index := range slash.matches {
		for {
			raw, ok := gtx.Event(pointer.Filter{Target: &slash.rows[index], Kinds: pointer.Enter | pointer.Press})
			if !ok {
				break
			}
			switch raw.(pointer.Event).Kind {
			case pointer.Enter:
				slash.cursor = index
			case pointer.Press:
				state.completeCommand(index)
				return
			}
		}
	}

	filters := []event.Filter{
		key.Filter{Focus: &state.editor, Name: key.NameUpArrow},
		key.Filter{Focus: &state.editor, Name: key.NameDownArrow},
		key.Filter{Focus: &state.editor, Name: key.NameTab},
		key.Filter{Focus: &state.editor, Name: key.NameEscape},
	}
	// a command typed in full is sent by Enter, anything shorter is completed by it
	if !strings.EqualFold(commandName(slash.matches[slash.cursor]), query) {
		filters = append(filters, key.Filter{Focus: &state.editor, Name: key.NameReturn})
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
		switch pressed.Name {
		case key.NameUpArrow:
			slash.cursor = (slash.cursor + len(slash.matches) - 1) % len(slash.matches)
		case key.NameDownArrow:
			slash.cursor = (slash.cursor + 1) % len(slash.matches)
		case key.NameEscape:
			slash.open, slash.dismissed = false, text
			return
		default:
			state.completeCommand(slash.cursor)
			return
		}
	}
}

func (state *composerState) completeCommand(index int) {
	state.setText("/" + commandName(state.slash.matches[index]) + " ")
	state.slash.open, state.slash.cursor = false, 0
}

// drawn deferred so it floats over what is above the field and the field never moves under the caret
func (state *composerState) layoutSlash(gtx layout.Context, th *theme.Theme, options ComposerOptions, width int) {
	slash := &state.slash
	if !slash.open || options.Disabled {
		return
	}
	row, inset, edge := gtx.Dp(slashRow), gtx.Dp(13), gtx.Dp(4)
	height := len(slash.matches)*row + 2*edge
	bounds := image.Rect(0, 0, width, height)
	cut := gtx.Dp(th.Cut.Mid)

	macro := op.Record(gtx.Ops)
	kit.FillCut(gtx.Ops, bounds, cut, th.R.Stone4)
	kit.StrokeCut(gtx.Ops, bounds, cut, max(1, gtx.Dp(1)), th.R.Vein)
	for index, command := range slash.matches {
		top := edge + index*row
		area := clip.Rect(image.Rect(edge, top, width-edge, top+row)).Push(gtx.Ops)
		event.Op(gtx.Ops, &slash.rows[index])
		pointer.CursorPointer.Add(gtx.Ops)
		area.Pop()
		if index == slash.cursor {
			kit.FillCut(gtx.Ops, image.Rect(edge, top, width-edge, top+row), gtx.Dp(th.Cut.Small), palette.Mix(th.R.Stone4, th.R.Fg, .06))
		}
		offset := op.Offset(image.Pt(inset, top)).Push(gtx.Ops)
		line := gtx
		line.Constraints = layout.Exact(image.Pt(max(0, width-2*inset), row))
		layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(line,
			layout.Rigid(kit.RawText(th, th.T.Mono, th.R.Fg, "/"+commandName(command))),
			layout.Rigid(layout.Spacer{Width: th.Sp.S2}.Layout),
			layout.Rigid(kit.RawText(th, th.T.MonoSm, th.R.Fg3, command.Hint)),
			layout.Rigid(layout.Spacer{Width: th.Sp.S3}.Layout),
			layout.Flexed(1, kit.RawText(th, th.T.Caption, th.R.Fg2, command.Description)),
		)
		offset.Pop()
	}
	call := macro.Stop()

	offset := op.Offset(image.Pt(0, -height-gtx.Dp(4))).Push(gtx.Ops)
	op.Defer(gtx.Ops, call)
	offset.Pop()
}
