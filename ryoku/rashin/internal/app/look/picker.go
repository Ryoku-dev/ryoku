package look

import (
	"image"
	"strings"

	"gioui.org/io/event"
	"gioui.org/io/key"
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
	// the list scrolls past this height
	pickerHeight = 336
)

// the opened list of a select whose options need a second line or are too many for a menu
type picker struct {
	search  kit.Search
	keys    struct{}
	list    Scroller
	rows    []pickerRow
	matches []int
	query   string
	cursor  int
	hovered int
	fresh   bool
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
	words := strings.Fields(strings.ToLower(p.query))
	p.matches = p.matches[:0]
	for index, option := range options {
		text := strings.ToLower(option.Label + " " + option.Detail + " " + option.Group)
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
				p.list.ScrollTo(max(0, at-2))
			}
		}
	}
	p.cursor = p.usable(options, min(p.cursor, max(0, len(p.matches)-1)), 1)

	tag := p.focus(searched)
	filters := []event.Filter{
		key.Filter{Focus: tag, Name: key.NameUpArrow}, key.Filter{Focus: tag, Name: key.NameDownArrow},
		key.Filter{Focus: tag, Name: key.NameReturn}, key.Filter{Focus: tag, Name: key.NameEnter},
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
			p.reveal()
		case key.NameDownArrow:
			p.cursor = p.usable(options, p.cursor+1, 1)
			p.reveal()
		default:
			if index := p.matches[p.cursor]; !options[index].Disabled {
				picked = index
			}
		}
	}

	if len(p.rows) < len(options) {
		p.rows = append(p.rows, make([]pickerRow, len(options)-len(p.rows))...)
	}
	width := gtx.Constraints.Max.X
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
		gtx.Constraints.Max.Y = min(gtx.Constraints.Max.Y, gtx.Dp(pickerHeight))
		return p.list.Layout(gtx, th, len(p.matches), func(gtx layout.Context, at int) layout.Dimensions {
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
			if group == "" || (at > 0 && options[p.matches[at-1]].Group == group) {
				return option(gtx)
			}
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx, layout.Rigid(groupHeading(th, group, at == 0)), layout.Rigid(option))
		})
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
	if p.cursor < first {
		p.list.ScrollTo(p.cursor)
	} else if shown := max(1, p.list.Position.Count-1); p.cursor >= first+shown {
		p.list.ScrollTo(p.cursor - shown + 1)
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
		content.Constraints = layout.Constraints{Max: image.Pt(max(0, width-2*inset-glyph-gap), gtx.Constraints.Max.Y)}
		said := layout.Flex{Axis: layout.Vertical}.Layout(content,
			layout.Rigid(kit.Text(th.Theme, th.T.Label, th.R.Fg, option.Label)),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if option.Detail == "" {
					return layout.Dimensions{}
				}
				return layout.Inset{Top: 2}.Layout(gtx, Paragraph(th, th.T.Detail, th.R.Fg2, option.Detail))
			}),
		)
		call := record.Stop()
		size := image.Pt(width, max(gtx.Dp(36), said.Size.Y+gtx.Dp(16)))
		hover := Transition(gtx, &row.hover, raised, th.Motion.HoverIn, th.Motion.HoverOut)
		if hover > 0 {
			kit.FillCut(gtx.Ops, image.Rectangle{Max: size}, gtx.Dp(th.Cut.Small), palette.Mix(th.R.Stone4, th.R.Fg, tokens.AlphaHoverPlane*hover))
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
		offset := op.Offset(image.Pt(inset, top)).Push(gtx.Ops)
		call.Add(gtx.Ops)
		offset.Pop()
		return layout.Dimensions{Size: size}
	})
}
