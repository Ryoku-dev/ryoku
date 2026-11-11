package chat

import (
	"image"
	"image/color"
	"strings"
	"unicode"

	"gioui.org/f32"
	"gioui.org/io/event"
	"gioui.org/io/pointer"
	"gioui.org/io/semantic"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"

	"ryoku-rashin/internal/app/look"
)

// how far a face without an italic is leaned for emphasis, in radians
const emphasisLean = .2

type inlineState struct {
	source string
	parsed bool
	spans  []inlineSpan
	// set when the paragraph has nothing to style or reads right to left
	plain string

	set       *inlineSet
	width     int
	faces     inlineFaces
	metric    unit.Metric
	selection inlineSelection
	links     []widget.Clickable
	frame     int
}

func (state *viewState) inline(key string) *inlineState {
	inline := state.inlines[key]
	if inline == nil {
		inline = new(inlineState)
		state.inlines[key] = inline
	}
	return inline
}

// a paragraph with nothing to style is plain selectable text
// one that reads right to left is too, set a span at a time its words would come out in the wrong order
func (state *viewState) layoutInline(
	gtx layout.Context,
	th *look.Theme,
	key string,
	style look.TextStyle,
	ink color.NRGBA,
	source string,
) layout.Dimensions {
	inline := state.inline(key)
	if !inline.parsed || inline.source != source {
		inline.source, inline.parsed, inline.set = source, true, nil
		inline.spans = parseInline(source)
		inline.plain = plainSpans(inline.spans)
	}
	if len(inline.spans) == 0 {
		return layout.Dimensions{}
	}
	if inline.plain != "" {
		return state.selectable(gtx, th, key, style, ink, inline.plain)
	}
	faces := inlineFaces{base: style, mono: codeStyle(th)}
	width := gtx.Constraints.Max.X
	if inline.set == nil || inline.width != width || inline.faces != faces || inline.metric != gtx.Metric {
		inline.set = setInline(th.Shaper, gtx.Metric, gtx.Locale, faces, inline.spans, width)
		inline.width, inline.faces, inline.metric = width, faces, gtx.Metric
		inline.links = make([]widget.Clickable, inline.set.linkRuns())
	}
	state.keepSet(inline)
	set := inline.set

	changed, released := inline.selection.update(gtx, set)
	from, to := inline.selection.stretch()
	state.hold(gtx, key, &inline.selection, changed, to > from, inline.selection.focused,
		func() string { from, to := inline.selection.stretch(); return string(set.text[from:to]) },
		inline.selection.clear)
	if released && to == from {
		state.act(Action{Kind: ActRelease})
	}

	size := image.Pt(max(set.size.X, gtx.Constraints.Min.X), set.size.Y)
	area := clip.Rect(image.Rectangle{Max: size}).Push(gtx.Ops)
	semantic.LabelOp(set.label).Add(gtx.Ops)
	pointer.CursorText.Add(gtx.Ops)
	event.Op(gtx.Ops, &inline.selection)
	inline.selection.click.Add(gtx.Ops)
	inline.selection.drag.Add(gtx.Ops)
	hovered := state.layoutLinks(gtx, inline, to > from)
	area.Pop()

	state.paintInline(gtx, th, inline, ink, hovered)
	return layout.Dimensions{Size: size, Baseline: size.Y - set.baseline}
}

// empty when the spans carry style that can be kept
func plainSpans(spans []inlineSpan) string {
	styled, backward := false, false
	for _, span := range spans {
		styled = styled || span.style != 0 || span.url != ""
		backward = backward || strings.ContainsFunc(span.text, readsRightToLeft)
	}
	if styled && !backward {
		return ""
	}
	var text strings.Builder
	for _, span := range spans {
		text.WriteString(span.text)
		// with its style gone a link still has to say where it leads
		if span.url != "" && span.url != span.text {
			text.WriteString(" (" + span.url + ")")
		}
	}
	return text.String()
}

func readsRightToLeft(character rune) bool {
	return unicode.In(character, unicode.Hebrew, unicode.Arabic, unicode.Syriac, unicode.Thaana, unicode.Nko)
}

func (set *inlineSet) linkRuns() int {
	count := 0
	for _, line := range set.lines {
		for _, run := range line.runs {
			if set.spans[run.span].url != "" {
				count++
			}
		}
	}
	return count
}

// reports the span under the pointer so a link broken over two lines lights as one
func (state *viewState) layoutLinks(gtx layout.Context, inline *inlineState, marking bool) (hovered int) {
	set, link := inline.set, 0
	hovered = -1
	for index, line := range set.lines {
		for _, run := range line.runs {
			url := set.spans[run.span].url
			if url == "" {
				continue
			}
			clickable := &inline.links[link]
			link++
			// a drag that began on a link marks its words, it does not follow it
			for clickable.Clicked(gtx) {
				if !marking {
					state.act(Action{Kind: ActOpenURL, Value: url})
				}
			}
			if clickable.Hovered() {
				hovered = run.span
			}
			bounds := image.Rect(run.from.Floor(), index*set.pitch, run.to.Ceil(), (index+1)*set.pitch)
			offset := op.Offset(bounds.Min).Push(gtx.Ops)
			clickable.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				pointer.CursorPointer.Add(gtx.Ops)
				return layout.Dimensions{Size: bounds.Size()}
			})
			offset.Pop()
		}
	}
	return hovered
}

func (state *viewState) paintInline(gtx layout.Context, th *look.Theme, inline *inlineState, ink color.NRGBA, hovered int) {
	set := inline.set
	from, to := inline.selection.stretch()
	for index := range set.lines {
		line := &set.lines[index]
		baseline := set.baseline + index*set.pitch
		if first, last := max(from, line.start), min(to, line.end); last > first {
			left, right := line.edges[first-line.start].Floor(), min(line.edges[last-line.start].Ceil(), max(inline.width, set.size.X))
			bounds := image.Rect(left, index*set.pitch, right, (index+1)*set.pitch)
			paint.FillShape(gtx.Ops, palette.WithAlpha(th.R.Sel, .6), clip.Rect(bounds).Op())
		}
		for _, run := range line.runs {
			if len(run.glyphs) == 0 {
				continue
			}
			span := set.spans[run.span]
			tone := ink
			switch {
			case span.url != "":
				tone = th.R.GoldInk
			case span.style&inlineCode != 0:
				tone = th.R.Fg2
			}
			origin := f32.Pt(float32(run.glyphs[0].X)/64, float32(baseline))
			offset := op.Affine(f32.AffineId().Offset(origin)).Push(gtx.Ops)
			lean := op.TransformStack{}
			if leaning := set.lean && span.style&inlineEmphasis != 0; leaning {
				lean = op.Affine(f32.AffineId().Shear(f32.Point{}, -emphasisLean, 0)).Push(gtx.Ops)
			}
			outline := clip.Outline{Path: th.Shaper.Shape(run.glyphs)}.Op().Push(gtx.Ops)
			paint.ColorOp{Color: tone}.Add(gtx.Ops)
			paint.PaintOp{}.Add(gtx.Ops)
			outline.Pop()
			if bitmaps := th.Shaper.Bitmaps(run.glyphs); bitmaps != (op.CallOp{}) {
				bitmaps.Add(gtx.Ops)
			}
			if set.lean && span.style&inlineEmphasis != 0 {
				lean.Pop()
			}
			offset.Pop()
			if span.url != "" && run.span == hovered {
				under := image.Rect(run.from.Floor(), baseline+gtx.Dp(2), run.to.Ceil(), baseline+gtx.Dp(3))
				paint.FillShape(gtx.Ops, tone, clip.Rect(under).Op())
			}
		}
	}
}

// only what is on screen keeps its lines, or a long transcript holds the glyphs of everything scrolled past
func (state *viewState) keepSet(inline *inlineState) {
	inline.frame = state.frame
	state.shown = append(state.shown, inline)
}

func (state *viewState) dropUnseenSets() {
	for _, inline := range state.wasShown {
		if inline.frame != state.frame {
			inline.set = nil
		}
	}
	state.wasShown, state.shown = state.shown, state.wasShown[:0]
	state.frame++
}
