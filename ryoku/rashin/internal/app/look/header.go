package look

import (
	"image"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/widget"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/anim"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"
	"ryoku-i18n"
)

type FactKind uint8

const (
	FactText FactKind = iota
	FactLink
	FactToken
)

type HeaderFact struct {
	Text  string
	Kind  FactKind
	Crumb bool
}

type HeaderAction struct {
	Icon        *widget.Icon
	Label       string
	Description string
	Trail       layout.Widget
	// the view this action is showing now
	On       bool
	Disabled bool
}

type PaneHeaderContent struct {
	Title     string
	Facts     []HeaderFact
	Claim     string
	ClaimTone palette.Tone
	Actions   []HeaderAction
	Menu      []kit.MenuItem
	// draws the vein that says content passed under the header
	Scrolled bool
}

type headerAction struct {
	icon  kit.IconPlaque
	label kit.Click
	hover anim.Tween
	tip   Tip
}

type PaneHeader struct {
	Preview kit.State

	actions  []headerAction
	links    []kit.Click
	overflow kit.IconPlaque
	moreTip  Tip
	menu     kit.Menu
	vein     anim.Tween

	action int
	link   int
	item   int
}

func (header *PaneHeader) ActionClicked() (int, bool) { return take(&header.action) }

func (header *PaneHeader) LinkClicked() (int, bool) { return take(&header.link) }

func (header *PaneHeader) MenuClicked() (int, bool) { return take(&header.item) }

// one-based latch so the zero value means nothing happened
func take(latch *int) (int, bool) {
	value := *latch
	*latch = 0
	return value - 1, value > 0
}

func (header *PaneHeader) Layout(
	gtx layout.Context,
	th *Theme,
	overlays *Overlays,
	content PaneHeaderContent,
) layout.Dimensions {
	header.update(gtx, content)
	size := image.Pt(gtx.Constraints.Max.X, min(gtx.Dp(PaneHeaderHeight), gtx.Constraints.Max.Y))
	gtx.Constraints = layout.Exact(size)
	// flex aligns to its tallest child, not to its own height, so the row is measured free and centred here
	row := gtx
	row.Constraints = layout.Constraints{Max: image.Pt(max(0, size.X-gtx.Dp(21)-gtx.Dp(13)), size.Y)}
	MiddleLeft(row, gtx.Dp(21), size.Y, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions { return header.layoutIdentity(gtx, th, content) }),
			layout.Rigid(layout.Spacer{Width: 13}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions { return header.layoutActions(gtx, th, overlays, content) }),
		)
	})
	header.layoutVein(gtx, th, content.Scrolled)
	return layout.Dimensions{Size: size}
}

func (header *PaneHeader) update(gtx layout.Context, content PaneHeaderContent) {
	for len(header.actions) < len(content.Actions) {
		header.actions = append(header.actions, headerAction{})
	}
	for len(header.links) < len(content.Facts) {
		header.links = append(header.links, kit.Click{})
	}
	for index, action := range content.Actions {
		state := &header.actions[index]
		clicked := state.icon.Clicked(gtx)
		if state.label.Clicked(gtx) {
			clicked = true
		}
		if clicked && !action.Disabled {
			header.action = index + 1
		}
	}
	for index, fact := range content.Facts {
		if header.links[index].Clicked(gtx) && fact.Kind == FactLink {
			header.link = index + 1
		}
	}
}

// facts that don't fit are clipped whole, never squeezed
func (header *PaneHeader) layoutIdentity(gtx layout.Context, th *Theme, content PaneHeaderContent) layout.Dimensions {
	children := []layout.FlexChild{
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if len(content.Facts) > 0 || content.Claim != "" {
				gtx.Constraints.Max.X = gtx.Constraints.Max.X * 3 / 5
			}
			gtx.Constraints.Min.X = 0
			return kit.Text(th.Theme, th.T.Title, th.R.Fg, content.Title)(gtx)
		}),
	}
	// the claim goes before the facts, facts are what a narrow header can lose
	if content.Claim != "" {
		children = append(children,
			layout.Rigid(layout.Spacer{Width: 13}.Layout),
			layout.Rigid(kit.Claim(th.Theme, content.ClaimTone, content.Claim)),
		)
	}
	if len(content.Facts) > 0 {
		children = append(children,
			layout.Rigid(layout.Spacer{Width: 13}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions { return header.layoutFacts(gtx, th, content) }),
		)
	}
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	area := clip.Rect(image.Rectangle{Max: gtx.Constraints.Max}).Push(gtx.Ops)
	dimensions := layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx, children...)
	area.Pop()
	return dimensions
}

func (header *PaneHeader) layoutFacts(gtx layout.Context, th *Theme, content PaneHeaderContent) layout.Dimensions {
	children := make([]layout.FlexChild, 0, len(content.Facts)*2)
	for index, fact := range content.Facts {
		if index > 0 {
			children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if fact.Crumb {
					return layout.Inset{Left: 2, Right: 2}.Layout(gtx, IconWidget(Icon.ChevronRight, 14, th.R.Fg3))
				}
				return layout.Spacer{Width: 10}.Layout(gtx)
			}))
		}
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return header.layoutFact(gtx, th, index, fact)
		}))
	}
	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx, children...)
}

func (header *PaneHeader) layoutFact(gtx layout.Context, th *Theme, index int, fact HeaderFact) layout.Dimensions {
	switch fact.Kind {
	case FactToken:
		return TokenChip(th, fact.Text)(gtx)
	case FactLink:
		link := &header.links[index]
		ink := th.R.Fg3
		if link.Hovered() || header.Preview == kit.StateHover {
			ink = th.R.Fg
		}
		return link.Layout(gtx, kit.Text(th.Theme, th.T.Caption, ink, fact.Text))
	default:
		return kit.Text(th.Theme, th.T.Caption, th.R.Fg3, fact.Text)(gtx)
	}
}

func (header *PaneHeader) layoutActions(gtx layout.Context, th *Theme, overlays *Overlays, content PaneHeaderContent) layout.Dimensions {
	children := make([]layout.FlexChild, 0, len(content.Actions)*2+2)
	for index, action := range content.Actions {
		if len(children) > 0 {
			children = append(children, layout.Rigid(layout.Spacer{Width: 4}.Layout))
		}
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return header.layoutAction(gtx, th, overlays, &header.actions[index], action)
		}))
	}
	if len(content.Menu) > 0 {
		if len(children) > 0 {
			children = append(children, layout.Rigid(layout.Spacer{Width: 4}.Layout))
		}
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return header.layoutOverflow(gtx, th, overlays, content)
		}))
	}
	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx, children...)
}

func (header *PaneHeader) layoutAction(
	gtx layout.Context,
	th *Theme,
	overlays *Overlays,
	state *headerAction,
	action HeaderAction,
) layout.Dimensions {
	if action.Disabled {
		gtx = gtx.Disabled()
	}
	body := func(gtx layout.Context) layout.Dimensions {
		if action.Label == "" && action.Trail == nil {
			state.icon.Icon, state.icon.Description = action.Icon, action.Description
			state.icon.Small, state.icon.Preview = true, header.Preview
			state.icon.Kind = kit.PlaqueQuiet
			if action.On {
				state.icon.Kind = kit.PlaqueOn
			}
			return state.icon.Layout(gtx, th.Theme)
		}
		return header.layoutLabelled(gtx, th, state, action)
	}
	return state.tip.Layout(gtx, th, overlays, action.Description, body)
}

// kit.Plaque can't hold an icon, a label and trailing content, this small one can
func (header *PaneHeader) layoutLabelled(gtx layout.Context, th *Theme, state *headerAction, action HeaderAction) layout.Dimensions {
	height := gtx.Dp(32)
	cut := gtx.Dp(th.Cut.Small)
	return state.label.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		hovered := state.label.Hovered() || header.Preview == kit.StateHover
		hover := transition(gtx, &state.hover, hovered && gtx.Source.Enabled(), th.Motion.HoverIn, th.Motion.HoverOut)
		ink := th.R.Fg2
		if action.On {
			ink = th.R.OnSel
		}
		record := op.Record(gtx.Ops)
		content := gtx
		content.Constraints = layout.Constraints{Max: image.Pt(gtx.Constraints.Max.X, height)}
		dimensions := layout.Inset{Left: 10, Right: 10}.Layout(content, func(gtx layout.Context) layout.Dimensions {
			children := make([]layout.FlexChild, 0, 5)
			if action.Icon != nil {
				children = append(children, layout.Rigid(IconWidget(action.Icon, 17, ink)), layout.Rigid(layout.Spacer{Width: 6}.Layout))
			}
			if action.Label != "" {
				children = append(children, layout.Rigid(kit.RawText(th.Theme, th.T.Label, ink, action.Label)))
			}
			if action.Trail != nil {
				if action.Label != "" {
					children = append(children, layout.Rigid(layout.Spacer{Width: 8}.Layout))
				}
				children = append(children, layout.Rigid(action.Trail))
			}
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx, children...)
		})
		call := record.Stop()
		bounds := image.Rect(0, 0, dimensions.Size.X, height)
		if action.On {
			kit.FillCut(gtx.Ops, bounds, cut, palette.Mix(th.R.Sel, th.R.Fg, .10*hover))
		} else {
			kit.FillCut(gtx.Ops, bounds, cut, palette.WithAlpha(th.R.Fg2, .10*hover))
			kit.StrokeCut(gtx.Ops, bounds, cut, max(1, gtx.Dp(1)), th.R.Vein)
		}
		offset := op.Offset(image.Pt(0, (height-dimensions.Size.Y)/2)).Push(gtx.Ops)
		call.Add(gtx.Ops)
		offset.Pop()
		return layout.Dimensions{Size: bounds.Max}
	})
}

func (header *PaneHeader) layoutOverflow(gtx layout.Context, th *Theme, overlays *Overlays, content PaneHeaderContent) layout.Dimensions {
	header.overflow.Icon, header.overflow.Description = Icon.More, i18n.T("More")
	header.overflow.Small, header.overflow.Kind, header.overflow.Preview = true, kit.PlaqueQuiet, header.Preview
	tag := &header.overflow.Clickable
	if header.overflow.Clicked(gtx) && overlays != nil {
		if overlays.IsOpen(tag) {
			overlays.Close(tag)
		} else {
			items := content.Menu
			size := gtx.Dp(32)
			overlays.Open(tag, overlays.AnchorOf(tag, image.Pt(size, size)), Placement{Align: End}, func(gtx layout.Context) layout.Dimensions {
				dimensions := header.menu.Layout(gtx, th.Theme, items)
				if item, ok := header.menu.Clicked(gtx); ok {
					header.item = item + 1
					overlays.Close(tag)
				}
				return dimensions
			})
		}
	}
	if overlays != nil && overlays.IsOpen(tag) {
		header.overflow.Kind = kit.PlaqueOn
	}
	return header.moreTip.Layout(gtx, th, overlays, header.overflow.Description, func(gtx layout.Context) layout.Dimensions {
		return header.overflow.Layout(gtx, th.Theme)
	})
}

func (header *PaneHeader) layoutVein(gtx layout.Context, th *Theme, scrolled bool) {
	alpha := transition(gtx, &header.vein, scrolled, th.Motion.HoverIn, th.Motion.HoverOut)
	if alpha <= 0 {
		return
	}
	line := max(1, gtx.Dp(1))
	bounds := image.Rect(0, gtx.Constraints.Max.Y-line, gtx.Constraints.Max.X, gtx.Constraints.Max.Y)
	paint.FillShape(gtx.Ops, palette.WithAlpha(th.R.Vein, alpha), clip.Rect(bounds).Op())
}
