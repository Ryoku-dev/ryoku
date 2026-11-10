package chat

import (
	"image"
	"image/color"
	"strconv"
	"strings"
	"unicode"

	"gioui.org/font"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/look"
)

type markdownState struct {
	source string
	blocks []markdownBlock
}

type codeVisual struct {
	hover  look.Hover
	scroll layout.List
}

type spaceKey struct {
	size   unit.Sp
	weight font.Weight
	scale  float32
}

type inlineWord struct {
	kind inlineKind
	text string
	url  string
	// whitespace separated this word from the one before
	space bool
}

func (state *viewState) layoutMarkdown(gtx layout.Context, th *look.Theme, key, source string) layout.Dimensions {
	markdown := state.markdown[key]
	if markdown == nil {
		markdown = new(markdownState)
		state.markdown[key] = markdown
	}
	if markdown.source != source {
		markdown.source, markdown.blocks = source, parseMarkdown(source)
	}
	children := make([]layout.FlexChild, 0, len(markdown.blocks)*2)
	for index, block := range markdown.blocks {
		if index > 0 {
			gap := th.Sp.S3
			if block.kind == markdownHeading {
				gap = th.Sp.S4
			}
			children = append(children, layout.Rigid(layout.Spacer{Height: gap}.Layout))
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
		return state.layoutMarkdownTable(gtx, th, block.head, block.rows)
	default:
		return state.layoutInline(gtx, th, key, th.T.Proof, th.R.Fg, block.text)
	}
}

// a plain paragraph is one selectable text, one with marks is set word by word and copied through the turn's copy action
func (state *viewState) layoutInline(
	gtx layout.Context,
	th *look.Theme,
	key string,
	style look.TextStyle,
	ink color.NRGBA,
	source string,
) layout.Dimensions {
	spans := parseInline(source)
	if len(spans) == 1 && spans[0].kind == inlinePlain {
		return state.selectable(gtx, th, key, style, ink, spans[0].text)
	}
	words := inlineWords(spans)
	limit := gtx.Constraints.Max.X
	if limit <= 0 || len(words) == 0 {
		return layout.Dimensions{}
	}
	line := gtx.Sp(style.Line)
	space := state.spaceWidth(gtx, th, style)
	x, y, widest := 0, 0, 0
	for index, word := range words {
		record := op.Record(gtx.Ops)
		measure := gtx
		measure.Constraints = layout.Constraints{Max: image.Pt(limit, line*4)}
		dimensions := state.layoutWord(measure, th, key+":"+strconv.Itoa(index), style, ink, word)
		call := record.Stop()
		lead := 0
		if word.space && x > 0 {
			lead = space
		}
		if x > 0 && x+lead+dimensions.Size.X > limit {
			x, y, lead = 0, y+line, 0
		}
		offset := op.Offset(image.Pt(x+lead, y+(line-dimensions.Size.Y)/2)).Push(gtx.Ops)
		call.Add(gtx.Ops)
		offset.Pop()
		x += lead + dimensions.Size.X
		widest = max(widest, x)
	}
	return layout.Dimensions{Size: image.Pt(widest, y+line)}
}

func (state *viewState) layoutWord(
	gtx layout.Context,
	th *look.Theme,
	key string,
	style look.TextStyle,
	ink color.NRGBA,
	word inlineWord,
) layout.Dimensions {
	label := func(style look.TextStyle, ink color.NRGBA) layout.Dimensions {
		return kit.Label{Style: style, Color: ink, MaxLines: 1}.Layout(gtx, th.Theme, word.text)
	}
	switch word.kind {
	case inlineEmphasis:
		style.Face.Style = font.Italic
	case inlineStrong:
		style.Face.Weight = font.Bold
	case inlineCode:
		return look.TokenChip(th, word.text)(gtx)
	case inlineLink:
		link := state.links[key]
		if link == nil {
			link = new(widget.Clickable)
			state.links[key] = link
		}
		for link.Clicked(gtx) {
			state.act(Action{Kind: ActOpenURL, Value: word.url})
		}
		return link.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			pointer.CursorPointer.Add(gtx.Ops)
			dimensions := label(style, th.R.GoldInk)
			underline := image.Rect(0, dimensions.Size.Y-gtx.Dp(2), dimensions.Size.X, dimensions.Size.Y-gtx.Dp(1))
			if link.Hovered() {
				paint.FillShape(gtx.Ops, th.R.GoldInk, clip.Rect(underline).Op())
			}
			return dimensions
		})
	}
	return label(style, ink)
}

// measured once so words set one by one stand as far apart as the shaper would put them
func (state *viewState) spaceWidth(gtx layout.Context, th *look.Theme, style look.TextStyle) int {
	key := spaceKey{size: style.Size, weight: style.Face.Weight, scale: gtx.Metric.PxPerSp}
	if width, ok := state.spaces[key]; ok {
		return width
	}
	measure := func(value string) int {
		record := op.Record(gtx.Ops)
		child := gtx
		child.Constraints = layout.Constraints{Max: image.Pt(1<<16, 1<<12)}
		dimensions := kit.Label{Style: style, MaxLines: 1}.Layout(child, th.Theme, value)
		record.Stop()
		return dimensions.Size.X
	}
	width := max(1, measure("n n")-measure("nn"))
	state.spaces[key] = width
	return width
}

// keeps where whitespace stood so a styled word right after plain text keeps its space
func inlineWords(spans []inlineSpan) []inlineWord {
	words := make([]inlineWord, 0, len(spans)*4)
	pending := false
	for _, span := range spans {
		if span.kind == inlineCode {
			words = append(words, inlineWord{kind: span.kind, text: span.text, space: pending})
			pending = false
			continue
		}
		start := -1
		for index, character := range span.text {
			if !unicode.IsSpace(character) {
				if start < 0 {
					start = index
				}
				continue
			}
			if start >= 0 {
				words = append(words, inlineWord{kind: span.kind, text: span.text[start:index], url: span.url, space: pending})
				start = -1
			}
			pending = true
		}
		if start >= 0 {
			words = append(words, inlineWord{kind: span.kind, text: span.text[start:], url: span.url, space: pending})
			pending = false
		}
	}
	return words
}

func (state *viewState) layoutMarkdownList(gtx layout.Context, th *look.Theme, key string, items []markdownListItem) layout.Dimensions {
	line := gtx.Sp(th.T.Proof.Line)
	children := make([]layout.FlexChild, 0, len(items)*2)
	for index, item := range items {
		if index > 0 {
			children = append(children, layout.Rigid(layout.Spacer{Height: th.Sp.S1}.Layout))
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
			dimensions := state.layoutInline(body, th, key+":"+strconv.Itoa(index), th.T.Proof, th.R.Fg, item.text)
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
	dimensions := state.layoutInline(body, th, key, th.T.Proof, th.R.Fg2, value)
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
		kit.FillCut(gtx.Ops, image.Rectangle{Max: size}, gtx.Dp(th.Cut.Mid), th.R.Stone1)
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

func languageLabel(language string) string {
	if language = strings.TrimSpace(language); language != "" {
		return language
	}
	return i18n.T("code")
}

func (state *viewState) layoutMarkdownTable(gtx layout.Context, th *look.Theme, head []string, rows [][]string) layout.Dimensions {
	columns := len(head)
	if columns == 0 {
		return layout.Dimensions{}
	}
	limit := gtx.Constraints.Max.X
	gap := gtx.Dp(34)
	widths := make([]int, columns)
	measure := func(style look.TextStyle, value string) int {
		record := op.Record(gtx.Ops)
		child := gtx
		child.Constraints = layout.Constraints{Max: image.Pt(limit, 1<<12)}
		dimensions := kit.Label{Style: style, MaxLines: 1}.Layout(child, th.Theme, value)
		record.Stop()
		return dimensions.Size.X
	}
	numeric := make([]bool, columns)
	for column := range columns {
		widths[column] = max(gtx.Dp(72), measure(th.T.Caption, head[column]))
		numeric[column] = len(rows) > 0
		for _, row := range rows {
			if column >= len(row) {
				continue
			}
			widths[column] = max(widths[column], measure(th.T.Body, row[column]))
			numeric[column] = numeric[column] && isNumeric(row[column])
		}
	}
	total := gap * (columns - 1)
	for _, width := range widths {
		total += width
	}
	// columns that don't fit share the width in proportion, the rest stay as narrow as their content
	if total > limit {
		scale := float32(limit-gap*(columns-1)) / float32(total-gap*(columns-1))
		for column := range widths {
			widths[column] = int(float32(widths[column]) * scale)
		}
		total = limit
	}
	rowHeight := gtx.Dp(34)
	drawRow := func(y int, cells []string, style look.TextStyle, ink color.NRGBA) {
		x := 0
		for column := range columns {
			value := ""
			if column < len(cells) {
				value = cells[column]
			}
			cell := gtx
			cell.Constraints = layout.Exact(image.Pt(widths[column], rowHeight))
			offset := op.Offset(image.Pt(x, y)).Push(gtx.Ops)
			align, direction := text.Start, layout.W
			if numeric[column] {
				align, direction = text.End, layout.E
			}
			direction.Layout(cell, func(gtx layout.Context) layout.Dimensions {
				return kit.Label{Style: style, Color: ink, MaxLines: 1, Align: align}.Layout(gtx, th.Theme, value)
			})
			offset.Pop()
			x += widths[column] + gap
		}
	}
	line := max(1, gtx.Dp(1))
	headHeight := gtx.Dp(28)
	y := 0
	headRow := rowHeight
	rowHeight = headHeight
	drawRow(0, head, th.T.Caption, th.R.Fg3)
	rowHeight = headRow
	y += headHeight
	paint.FillShape(gtx.Ops, th.R.Vein, clip.Rect(image.Rect(0, y, total, y+line)).Op())
	y += line
	for index, row := range rows {
		drawRow(y, row, th.T.Body, th.R.Fg)
		y += rowHeight
		if index < len(rows)-1 {
			paint.FillShape(gtx.Ops, palette.WithAlpha(th.R.Vein, .5), clip.Rect(image.Rect(0, y, total, y+line)).Op())
			y += line
		}
	}
	return layout.Dimensions{Size: image.Pt(total, y)}
}

func isNumeric(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" {
		return false
	}
	digits := 0
	for _, character := range value {
		switch {
		case unicode.IsDigit(character):
			digits++
		case strings.ContainsRune(".,%+-−  ", character):
		default:
			return false
		}
	}
	return digits > 0
}
