package look

import (
	"image"
	"strings"

	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/io/semantic"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/anim"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/tokens"
	"ryoku-i18n"
)

const (
	// past this many a list is searched, not read
	pickerSearchFrom = 9
	// label rows share the kit menu's height and visible-item cap
	pickerHeight = tokens.SizeMenuVisibleItemsCount*tokens.SizeMenuItem + 2*tokens.SizeMenuPaddingVertical - 16
)

// the opened list of a select whose options need a second line or are too many for a menu
type picker struct {
	search        kit.Search
	keys          struct{}
	list          layout.List
	rows          []pickerRow
	matches       []int
	query         string
	cursor        int
	hovered       int
	fresh         bool
	rail          kit.ScrollRail
	scrollTag     struct{}
	viewport      int
	revealCursor  bool
	metrics       op.Ops
	labelHeight   int
	labelBaseline int
	rowHeights    []int
	// what matches was built from, so a list of hundreds is not searched again every frame
	matched      string
	matchedCount int
	matchedOnce  bool
}

type pickerRow struct {
	click kit.Click
	hover anim.Tween
}

func (p *picker) open() {
	p.search.SetText("")
	p.query, p.cursor, p.hovered, p.fresh = "", 0, -1, true
	p.matchedOnce = false
	p.list.Position = layout.Position{}
}

// what holds the keyboard while the list is open
func (p *picker) focus(searched bool) any {
	if searched {
		return &p.search.Editor
	}
	return &p.keys
}

func (p *picker) match(options []SelectOption) {
	if p.matchedOnce && p.matched == p.query && p.matchedCount == len(options) {
		return
	}
	p.matched, p.matchedCount, p.matchedOnce = p.query, len(options), true
	clear(p.rowHeights)
	words := strings.Fields(strings.ToLower(p.query))
	p.matches = p.matches[:0]
	for index, option := range options {
		text := strings.ToLower(option.Label + " " + option.Detail + " " + option.Group + " " + strings.Join(option.Chips, " "))
		found := true
		for _, word := range words {
			if !strings.Contains(text, word) {
				found = false
				break
			}
		}
		if found {
			p.matches = append(p.matches, index)
		}
	}
}

// returns the option picked this frame, or -1
func (p *picker) Layout(gtx layout.Context, th *Theme, options []SelectOption, selected int, searched bool) (layout.Dimensions, int) {
	picked := -1
	if query := strings.TrimSpace(p.search.Text()); query != p.query {
		p.query, p.cursor = query, 0
		p.list.Position = layout.Position{}
	}
	p.match(options)
	if p.fresh {
		p.fresh = false
		for at, index := range p.matches {
			if index == selected {
				p.cursor = at
				p.revealCursor = true
			}
		}
	}
	p.cursor = p.usable(options, min(p.cursor, max(0, len(p.matches)-1)), 1)

	tag := p.focus(searched)
	filters := []event.Filter{
		key.Filter{Focus: tag, Name: key.NameUpArrow}, key.Filter{Focus: tag, Name: key.NameDownArrow},
		key.Filter{Focus: tag, Name: key.NameReturn}, key.Filter{Focus: tag, Name: key.NameEnter},
		key.Filter{Focus: tag, Name: key.NameHome}, key.Filter{Focus: tag, Name: key.NameEnd},
		key.Filter{Focus: tag, Name: key.NamePageUp}, key.Filter{Focus: tag, Name: key.NamePageDown},
	}
	if !searched {
		// the field is focusable by itself, a bare tag has to say so
		filters = append(filters, key.FocusFilter{Target: tag})
	}
	for {
		raw, ok := gtx.Event(filters...)
		if !ok {
			break
		}
		pressed, ok := raw.(key.Event)
		if !ok || pressed.State != key.Press || len(p.matches) == 0 {
			continue
		}
		switch pressed.Name {
		case key.NameUpArrow:
			p.cursor = p.usable(options, p.cursor-1, -1)
			p.revealCursor = true
		case key.NameDownArrow:
			p.cursor = p.usable(options, p.cursor+1, 1)
			p.revealCursor = true
		case key.NameHome:
			p.cursor = p.usable(options, 0, 1)
			p.revealCursor = true
		case key.NameEnd:
			p.cursor = p.usable(options, len(p.matches)-1, -1)
			p.revealCursor = true
		case key.NamePageUp, key.NamePageDown:
			step := max(1, p.list.Position.Count-1)
			direction := 1
			if pressed.Name == key.NamePageUp {
				step, direction = -step, -1
			}
			target := max(0, min(len(p.matches)-1, p.cursor+step))
			p.cursor = p.usable(options, target, direction)
			if direction > 0 && p.cursor < target || direction < 0 && p.cursor > target {
				p.cursor = p.usable(options, target, -direction)
			}
			p.revealCursor = true
		default:
			if p.cursor >= 0 && p.cursor < len(p.matches) {
				if index := p.matches[p.cursor]; !options[index].Disabled {
					picked = index
				}
			}
		}
	}

	if len(p.rows) < len(options) {
		p.rows = append(p.rows, make([]pickerRow, len(options)-len(p.rows))...)
	}
	if len(p.rowHeights) < len(p.matches) {
		p.rowHeights = append(p.rowHeights, make([]int, len(p.matches)-len(p.rowHeights))...)
	}
	width := gtx.Constraints.Max.X
	p.metrics.Reset()
	measure := gtx
	measure.Ops = &p.metrics
	measure.Constraints = layout.Constraints{Max: image.Pt(width, gtx.Dp(tokens.SizeMenuItem))}
	reference := kit.RawText(th.Theme, th.T.Label, th.R.Fg, "H")(measure)
	p.labelHeight, p.labelBaseline = reference.Size.Y, reference.Size.Y-reference.Baseline
	gtx.Constraints.Max.Y = min(gtx.Constraints.Max.Y, gtx.Dp(pickerHeight))
	gtx.Constraints.Min.Y = min(gtx.Constraints.Min.Y, gtx.Constraints.Max.Y)
	for {
		if _, ok := gtx.Event(pointer.Filter{Target: &p.scrollTag, Kinds: pointer.Scroll, ScrollX: pointer.ScrollRange{Min: -1 << 20, Max: 1 << 20}, ScrollY: pointer.ScrollRange{Min: -1 << 20, Max: 1 << 20}}); !ok {
			break
		}
	}
	scrollArea := clip.Rect(image.Rectangle{Max: gtx.Constraints.Max}).Push(gtx.Ops)
	event.Op(gtx.Ops, &p.scrollTag)
	scrollArea.Pop()
	children := make([]layout.FlexChild, 0, 3)
	if searched {
		p.search.Placeholder = i18n.Tf("Find in %d", len(options))
		children = append(children,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints.Min.X = width
				return p.search.Layout(gtx, th.Theme)
			}),
			layout.Rigid(layout.Spacer{Height: 6}.Layout),
		)
	} else {
		area := clip.Rect(image.Rectangle{Max: image.Pt(width, 1)}).Push(gtx.Ops)
		event.Op(gtx.Ops, &p.keys)
		area.Pop()
	}
	hovered := -1
	children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
		if len(p.matches) == 0 {
			return layout.Inset{Top: 8, Bottom: 8, Left: 10}.Layout(gtx, kit.RawText(th.Theme, th.T.Label, th.R.Fg3, i18n.T("Nothing matches")))
		}
		gtx.Constraints.Min = image.Pt(width, 0)
		if p.viewport != gtx.Constraints.Max.Y {
			p.revealCursor = true
		}
		p.viewport = gtx.Constraints.Max.Y
		if p.revealCursor && p.list.Position.Count > 0 {
			p.reveal()
			p.revealCursor = false
		}
		p.list.Axis = layout.Vertical
		dimensions := p.list.Layout(gtx, len(p.matches), func(gtx layout.Context, at int) layout.Dimensions {
			index := p.matches[at]
			row := &p.rows[index]
			disabled := options[index].Disabled
			if row.click.Clicked(gtx) && !disabled {
				picked = index
			}
			if row.click.Hovered() && !disabled {
				hovered = at
			}
			option := func(gtx layout.Context) layout.Dimensions {
				return p.layoutRow(gtx, th, row, options[index], index == selected, at == p.cursor && !disabled)
			}
			group := options[index].Group
			var dimensions layout.Dimensions
			if group == "" || (at > 0 && options[p.matches[at-1]].Group == group) {
				dimensions = option(gtx)
			} else {
				dimensions = layout.Flex{Axis: layout.Vertical}.Layout(gtx, layout.Rigid(groupHeading(th, group, at == 0)), layout.Rigid(option))
			}
			p.rowHeights[at] = dimensions.Size.Y
			return dimensions
		})
		railContext := gtx
		railContext.Constraints = layout.Exact(dimensions.Size)
		p.rail.Layout(railContext, th.Theme, &p.list, len(p.matches))
		if p.revealCursor && p.list.Position.Count > 0 {
			gtx.Execute(op.InvalidateCmd{})
		}
		return dimensions
	}))
	dimensions := layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
	// moving onto a row moves the cursor once, so the arrow keys stay free while the pointer rests
	if hovered != p.hovered {
		p.hovered = hovered
		if hovered >= 0 {
			p.cursor = hovered
		}
	}
	if picked >= 0 {
		gtx.Execute(op.InvalidateCmd{})
	}
	return dimensions, picked
}

func (p *picker) reveal() {
	first := p.list.Position.First
	if p.cursor < first || p.cursor == first && p.list.Position.Offset > 0 {
		p.list.ScrollTo(p.cursor)
		return
	}
	if p.cursor >= first+p.list.Position.Count {
		p.list.ScrollTo(p.cursor)
		return
	}
	top := -p.list.Position.Offset
	for at := first; at < p.cursor; at++ {
		top += p.rowHeights[at]
	}
	height := p.rowHeights[p.cursor]
	if height <= 0 {
		return
	}
	if top < 0 || height >= p.viewport {
		p.list.Position.Offset += top
	} else if bottom := top + height; bottom > p.viewport {
		p.list.Position.Offset += bottom - p.viewport
	}
}

// the nearest row from at, stepping by step and wrapping, that can be picked; at itself when none can
func (p *picker) usable(options []SelectOption, at, step int) int {
	count := len(p.matches)
	if count == 0 {
		return 0
	}
	for tried := range count {
		if candidate := ((at+step*tried)%count + count) % count; !options[p.matches[candidate]].Disabled {
			return candidate
		}
	}
	return (at%count + count) % count
}

// a run's name over its first row, in the type the side list heads its groups with
func groupHeading(th *Theme, group string, first bool) layout.Widget {
	inset := layout.Inset{Top: 12, Bottom: 4, Left: 12, Right: 12}
	if first {
		inset.Top = 4
	}
	return func(gtx layout.Context) layout.Dimensions {
		return inset.Layout(gtx, kit.Text(th.Theme, th.T.Group, th.R.Fg3, group))
	}
}

// a row is as tall as what it says: a second line wraps, it is never cut
func (p *picker) layoutRow(gtx layout.Context, th *Theme, row *pickerRow, option SelectOption, chosen, raised bool) layout.Dimensions {
	width := gtx.Constraints.Max.X
	inset, glyph, gap := gtx.Dp(12), gtx.Dp(18), gtx.Dp(10)
	lead := 0
	if option.Mark.ID != "" {
		lead = glyph + gap
	}
	gtx.Constraints.Min = image.Point{}
	if option.Disabled {
		// drawn at alpha.disabled by the kit's text, and screen readers hear it as unavailable
		gtx = gtx.Disabled()
	}
	return row.click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		semantic.Button.Add(gtx.Ops)
		semantic.LabelOp(option.Label).Add(gtx.Ops)
		semantic.SelectedOp(chosen).Add(gtx.Ops)
		semantic.EnabledOp(!option.Disabled).Add(gtx.Ops)
		record := op.Record(gtx.Ops)
		content := gtx
		content.Constraints = layout.Constraints{Max: image.Pt(max(0, width-2*inset-glyph-gap-lead), gtx.Constraints.Max.Y)}
		children := []layout.FlexChild{
			layout.Rigid(p.layoutLabel(th, option.Label)),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if option.Detail == "" {
					return layout.Dimensions{}
				}
				return layout.Inset{Top: th.Sp.S1}.Layout(gtx, Paragraph(th, th.T.Detail, th.R.Fg2, option.Detail))
			}),
		}
		if len(option.Chips) > 0 {
			children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Top: th.Sp.S1}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return Flow{Gap: th.Sp.S1, LineGap: th.Sp.S1}.Layout(gtx, len(option.Chips), func(gtx layout.Context, i int) layout.Dimensions {
						return Chip(th, nil, option.Chips[i], palette.Idle)(gtx)
					})
				})
			}))
		}
		said := layout.Flex{Axis: layout.Vertical}.Layout(content, children...)
		call := record.Stop()
		size := image.Pt(width, max(gtx.Dp(tokens.SizeMenuItem), said.Size.Y+gtx.Dp(16)))
		hover := Transition(gtx, &row.hover, raised, th.Motion.HoverIn, th.Motion.HoverOut)
		if hover > 0 {
			kit.Fill(gtx, th.Theme, image.Rectangle{Max: size}, gtx.Dp(th.Cut.Small), palette.WithAlpha(th.R.Hover(), hover))
		}
		top := (size.Y - said.Size.Y) / 2
		if chosen {
			// at the trailing edge as the menu marks it, level with the name wherever the second line ends
			ink := th.R.Fg2
			if option.Disabled {
				ink = palette.WithAlpha(ink, tokens.AlphaDisabled)
			}
			PlaceIcon(gtx, Icon.Check, image.Pt(width-inset-glyph, top+(gtx.Sp(th.T.Label.Line)-glyph)/2), 18, ink)
		}
		if lead > 0 {
			ink := th.R.Fg2
			if raised || chosen {
				ink = th.R.Fg
			}
			at := op.Offset(image.Pt(inset, top+(gtx.Sp(th.T.Label.Line)-glyph)/2)).Push(gtx.Ops)
			Logo(gtx, th, option.Mark, 18, ink)
			at.Pop()
		}
		offset := op.Offset(image.Pt(inset+lead, top)).Push(gtx.Ops)
		call.Add(gtx.Ops)
		offset.Pop()
		return layout.Dimensions{Size: size}
	})
}

func (p *picker) layoutLabel(th *Theme, value string) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min.Y, gtx.Constraints.Max.Y = p.labelHeight, p.labelHeight
		record := op.Record(gtx.Ops)
		dimensions := kit.RawText(th.Theme, th.T.Label, th.R.Fg, value)(gtx)
		call := record.Stop()
		offset := op.Offset(image.Pt(0, p.labelBaseline-(dimensions.Size.Y-dimensions.Baseline))).Push(gtx.Ops)
		call.Add(gtx.Ops)
		offset.Pop()
		return layout.Dimensions{Size: image.Pt(dimensions.Size.X, p.labelHeight), Baseline: p.labelHeight - p.labelBaseline}
	}
}
