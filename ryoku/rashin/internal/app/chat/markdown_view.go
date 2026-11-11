package chat

import (
	"image"
	"strconv"
	"strings"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/protocol"
)

type markdownState struct {
	source string
	blocks []markdownBlock
}

type codeVisual struct {
	hover  look.Hover
	scroll layout.List
}

// plan is what the turn's answer offers to run, nil when it offers nothing
func (state *viewState) layoutMarkdown(gtx layout.Context, th *look.Theme, key, source string, plan *protocol.CommandPlan) layout.Dimensions {
	markdown := state.markdown[key]
	if markdown == nil {
		markdown = new(markdownState)
		state.markdown[key] = markdown
	}
	if markdown.source != source {
		markdown.source, markdown.blocks = source, parseMarkdown(source)
	}
	children := make([]layout.FlexChild, 0, len(markdown.blocks)*2)
	placed := false
	for index, block := range markdown.blocks {
		// a command the answer offers to run stands once: the slab that runs it takes the place of the first block that shows it
		runs := block.kind == markdownCode && restates(block.text, plan)
		if runs && placed {
			continue
		}
		if len(children) > 0 {
			gap := th.Sp.S3
			if block.kind == markdownHeading {
				gap = th.Sp.S4
			}
			children = append(children, layout.Rigid(layout.Spacer{Height: gap}.Layout))
		}
		if runs {
			placed = true
			children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return state.layoutCommands(gtx, th, plan, key+":commands")
			}))
			continue
		}
		blockKey := key + ":" + strconv.Itoa(index)
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return state.layoutMarkdownBlock(gtx, th, block, blockKey)
		}))
	}
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

func (state *viewState) layoutMarkdownBlock(gtx layout.Context, th *look.Theme, block markdownBlock, key string) layout.Dimensions {
	switch block.kind {
	case markdownHeading:
		style := th.T.Title
		if block.level == 1 {
			style = th.T.Section
		}
		return state.layoutInline(gtx, th, key, style, th.R.Fg, block.text)
	case markdownList:
		return state.layoutMarkdownList(gtx, th, key, block.items)
	case markdownQuote:
		return state.layoutMarkdownQuote(gtx, th, key, block.text)
	case markdownCode:
		return state.layoutCodeBlock(gtx, th, key, block.language, block.text)
	case markdownTable:
		return state.layoutMarkdownTable(gtx, th, key, block.head, block.rows)
	case markdownRule:
		size := image.Pt(gtx.Constraints.Max.X, max(1, gtx.Dp(1)))
		paint.FillShape(gtx.Ops, th.R.Vein, clip.Rect(image.Rectangle{Max: size}).Op())
		return layout.Dimensions{Size: size}
	default:
		return state.layoutInline(gtx, th, key, readingStyle(th), th.R.Fg, block.text)
	}
}

func (state *viewState) layoutMarkdownList(gtx layout.Context, th *look.Theme, key string, items []markdownListItem) layout.Dimensions {
	style := readingStyle(th)
	line := gtx.Sp(style.Line)
	children := make([]layout.FlexChild, 0, len(items)*2)
	for index, item := range items {
		if index > 0 {
			children = append(children, layout.Rigid(layout.Spacer{Height: th.Sp.S2}.Layout))
		}
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			indent := gtx.Dp(21) * item.depth
			gutter := gtx.Dp(21)
			if item.ordered {
				marker := gtx
				marker.Constraints = layout.Exact(image.Pt(gutter-gtx.Dp(6), line))
				offset := op.Offset(image.Pt(indent, 0)).Push(gtx.Ops)
				layout.E.Layout(marker, kit.RawText(th.Theme, th.T.MonoSm, th.R.Fg3, strconv.Itoa(item.number)+"."))
				offset.Pop()
			} else {
				size := gtx.Dp(5)
				at := image.Pt(indent+gtx.Dp(6), (line-size)/2)
				kit.FillCut(gtx.Ops, image.Rectangle{Min: at, Max: at.Add(image.Pt(size, size))}, gtx.Dp(1), th.R.Fg3)
			}
			offset := op.Offset(image.Pt(indent+gutter, 0)).Push(gtx.Ops)
			body := gtx
			body.Constraints.Max.X = max(0, gtx.Constraints.Max.X-indent-gutter)
			body.Constraints.Min.X = 0
			dimensions := state.layoutInline(body, th, key+":"+strconv.Itoa(index), style, th.R.Fg, item.text)
			offset.Pop()
			return layout.Dimensions{Size: image.Pt(gtx.Constraints.Max.X, dimensions.Size.Y)}
		}))
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

func (state *viewState) layoutMarkdownQuote(gtx layout.Context, th *look.Theme, key, value string) layout.Dimensions {
	bar, gap := gtx.Dp(3), gtx.Dp(13)
	record := op.Record(gtx.Ops)
	body := gtx
	body.Constraints.Min.X = 0
	body.Constraints.Max.X = max(0, gtx.Constraints.Max.X-bar-gap)
	dimensions := state.layoutInline(body, th, key, readingStyle(th), th.R.Fg2, value)
	call := record.Stop()
	paint.FillShape(gtx.Ops, th.R.Vein, clip.Rect(image.Rect(0, 0, bar, dimensions.Size.Y)).Op())
	offset := op.Offset(image.Pt(bar+gap, 0)).Push(gtx.Ops)
	call.Add(gtx.Ops)
	offset.Pop()
	return layout.Dimensions{Size: image.Pt(gtx.Constraints.Max.X, dimensions.Size.Y)}
}

func (state *viewState) layoutCodeBlock(gtx layout.Context, th *look.Theme, key, language, value string) layout.Dimensions {
	visual := state.codes[key]
	if visual == nil {
		visual = new(codeVisual)
		visual.scroll.Axis = layout.Horizontal
		state.codes[key] = visual
	}
	value = strings.TrimRight(value, "\n")
	width := gtx.Constraints.Max.X
	strip := gtx.Dp(32)
	block := func(gtx layout.Context) layout.Dimensions {
		record := op.Record(gtx.Ops)
		code := gtx
		code.Constraints = layout.Constraints{Min: image.Pt(width, 0), Max: image.Pt(width, gtx.Constraints.Max.Y)}
		offset := op.Offset(image.Pt(0, strip)).Push(gtx.Ops)
		dimensions := layout.Inset{Top: 4, Bottom: 13, Left: 13, Right: 13}.Layout(code, func(gtx layout.Context) layout.Dimensions {
			return visual.scroll.Layout(gtx, 1, func(gtx layout.Context, _ int) layout.Dimensions {
				gtx.Constraints.Min.X = 0
				gtx.Constraints.Max.X = 1 << 20
				return state.selectable(gtx, th, key, th.T.Mono, th.R.Fg, value)
			})
		})
		offset.Pop()
		call := record.Stop()
		size := image.Pt(width, strip+dimensions.Size.Y)
		bounds, cut := image.Rectangle{Max: size}, gtx.Dp(th.Cut.Mid)
		kit.FillCut(gtx.Ops, bounds, cut, th.R.Slab)
		kit.StrokeCut(gtx.Ops, bounds, cut, max(1, gtx.Dp(1)), th.R.Vein)
		call.Add(gtx.Ops)
		look.Place(gtx, image.Rect(gtx.Dp(13), 0, width/2, strip), func(gtx layout.Context) layout.Dimensions {
			return layout.W.Layout(gtx, kit.RawText(th.Theme, th.T.MonoSm, th.R.Fg3, languageLabel(language)))
		})
		return layout.Dimensions{Size: size}
	}
	dimensions, hover := visual.hover.Layout(gtx, th, block)
	if hover > 0 {
		box := gtx.Dp(28)
		at := image.Pt(width-box-gtx.Dp(4), (strip-box)/2)
		opacity := paint.PushOpacity(gtx.Ops, hover)
		offset := op.Offset(at).Push(gtx.Ops)
		if _, clicked := state.icon(gtx, th, key+":copy", look.Icon.Copy, i18n.T("Copy")); clicked {
			state.act(Action{Kind: ActCopy, Value: value})
		}
		offset.Pop()
		opacity.Pop()
	}
	return dimensions
}

func offered(turn *turnState) *protocol.CommandPlan {
	if turn == nil || turn.answer == nil {
		return nil
	}
	return turn.answer.Plan
}

func restates(code string, plan *protocol.CommandPlan) bool {
	if plan == nil {
		return false
	}
	code = asTyped(code)
	all := make([]string, 0, len(plan.Commands))
	for _, command := range plan.Commands {
		typed := asTyped(command.Command)
		if code == typed {
			return true
		}
		all = append(all, typed)
	}
	return len(all) > 1 && code == strings.Join(all, "\n")
}

// a model writes the same command with or without a prompt mark before it
func asTyped(command string) string {
	return strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(command), "$ "))
}

func languageLabel(language string) string {
	if language = strings.TrimSpace(language); language != "" {
		return language
	}
	return i18n.T("code")
}
