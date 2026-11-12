package kit

import (
	"image"
	"math"

	"gioui.org/f32"
	"gioui.org/io/key"
	"gioui.org/io/semantic"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/anim"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/theme"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/tokens"
)

// DesignTile is one look in a gallery of looks: a preview over its title, a
// status line, an optional resource mark and one optional small action.
type DesignTile struct {
	Click
	Title       string
	Status      string
	Resource    Mark
	Image       paint.ImageOp
	Current     bool
	Selected    bool
	Busy        bool
	Unavailable string
	ActionText  string
	ActionGold  bool
	ActionIcon  *widget.Icon
	Preview     State

	action              Plaque
	hover, press, focus anim.Tween
}

// ActionTriggered reports one activation of the tile's action plaque.
func (t *DesignTile) ActionTriggered(gtx layout.Context) bool {
	return t.ActionText != "" && t.action.Clicked(gtx)
}

// Layout draws the tile at the width the constraints allow.
func (t *DesignTile) Layout(gtx layout.Context, th *theme.Theme) layout.Dimensions {
	return t.layout(gtx, th, false)
}

func (t *DesignTile) layout(gtx layout.Context, th *theme.Theme, forceFocus bool) layout.Dimensions {
	width := gtx.Constraints.Max.X
	inset := gtx.Dp(th.Sp.S1)
	padX, padY := gtx.Dp(th.Sp.S3), gtx.Dp(th.Sp.S2)
	previewWidth := max(0, width-2*inset)
	previewHeight := int(math.Round(float64(previewWidth) / tokens.SizeDesignPreviewRatio))
	titleLine := gtx.Sp(th.T.Row.Line)
	actionLine := gtx.Dp(tokens.SizePlaqueHeightSmall)
	textTop := inset + previewHeight
	height := textTop + padY + titleLine + actionLine + padY
	size := image.Pt(width, height)
	cut := gtx.Dp(th.Cut.Small)
	bounds := image.Rectangle{Max: size}
	original := gtx

	var actionCall op.CallOp
	var actionDims layout.Dimensions
	if t.ActionText != "" {
		t.action.Small = true
		switch {
		case t.ActionGold:
			t.action.Kind = PlaqueGold
		case t.Current || t.Selected:
			t.action.Kind = PlaqueOn
		default:
			t.action.Kind = PlaqueLine
		}
		t.action.Label = t.ActionText
		t.action.Icon = t.ActionIcon
		record := op.Record(gtx.Ops)
		actionGTX := gtx
		actionGTX.Constraints = layout.Constraints{Max: image.Pt(max(0, width-2*padX), actionLine)}
		actionDims = t.action.Layout(actionGTX, th)
		actionCall = record.Stop()
	}

	dims := t.Click.Layout(exactContext(gtx, size), func(gtx layout.Context) layout.Dimensions {
		semantic.Button.Add(gtx.Ops)
		semantic.LabelOp(t.Title).Add(gtx.Ops)
		semantic.SelectedOp(t.Selected).Add(gtx.Ops)
		description := t.Status
		if t.Unavailable != "" {
			description = t.Unavailable
		}
		if description != "" {
			semantic.DescriptionOp(description).Add(gtx.Ops)
		}
		semantic.EnabledOp(gtx.Source.Enabled()).Add(gtx.Ops)
		hover := previewTransition(original, &t.hover, t.Preview, StateHover, t.Hovered(), th.Motion.HoverIn, th.Motion.HoverOut)
		press := previewTransition(original, &t.press, t.Preview, StatePressed, t.Pressed(), th.Motion.Press, th.Motion.Press)
		if !gtx.Source.Enabled() {
			hover, press = 0, 0
		}

		fill := th.R.Slab
		if t.Selected {
			fill = th.R.Sel
		} else {
			fill = palette.Mix(fill, th.R.Fg, tokens.AlphaHoverPlane*hover)
		}
		fill = palette.Mix(fill, th.R.Fg, tokens.AlphaPressRow*press)
		FillCut(gtx.Ops, bounds, cut, disabledInk(gtx, fill))
		if !t.Selected {
			StrokeCut(gtx.Ops, bounds, cut, max(1, gtx.Dp(unit.Dp(tokens.SizeHairline))), disabledInk(gtx, th.R.Vein))
			if t.Current {
				FillCutCorners(gtx.Ops, image.Rect(0, textTop, width, height), 0, 0, cut, cut, disabledInk(gtx, th.R.Sel))
			}
		}

		previewRect := image.Rect(inset, inset, inset+previewWidth, inset+previewHeight)
		t.drawPreview(gtx, th, previewRect, insetCut(cut, inset))

		onSel := t.Current || t.Selected
		titleInk, statusInk, markInk := th.R.Fg, th.R.Fg2, th.R.Fg2
		if t.Resource.Attention {
			markInk = th.R.Amber
		}
		if onSel {
			titleInk, statusInk, markInk = th.R.OnSel, th.R.OnSel2, th.R.OnSel
		}
		status := t.Status
		if t.Unavailable != "" {
			status = t.Unavailable
		}
		textGTX := exactContext(gtx, image.Pt(max(0, width-2*padX), titleLine+actionLine))
		offset := op.Offset(image.Pt(padX, textTop+padY)).Push(gtx.Ops)
		layout.Flex{Axis: layout.Vertical}.Layout(textGTX,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints = layout.Exact(image.Pt(gtx.Constraints.Max.X, titleLine))
				children := []layout.FlexChild{layout.Flexed(1, Text(th, th.T.Row, titleInk, t.Title))}
				if t.Resource.Text != "" {
					children = append(children,
						layout.Rigid(layout.Spacer{Width: th.Sp.S2}.Layout),
						layout.Rigid(Text(th, th.T.Detail, markInk, t.Resource.Text)),
					)
				}
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx, children...)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints = layout.Exact(image.Pt(gtx.Constraints.Max.X, actionLine))
				children := make([]layout.FlexChild, 0, 4)
				if t.Busy {
					children = append(children,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions { return Working{Active: true}.Layout(gtx, th) }),
						layout.Rigid(layout.Spacer{Width: th.Sp.S2}.Layout),
					)
				}
				children = append(children, layout.Flexed(1, Text(th, th.T.Caption, statusInk, status)))
				if t.ActionText != "" {
					children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.Dimensions{Size: image.Pt(gtx.Dp(th.Sp.S2)+actionDims.Size.X, 0)}
					}))
				}
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx, children...)
			}),
		)
		offset.Pop()
		return layout.Dimensions{Size: size}
	})

	if t.ActionText != "" {
		offset := op.Offset(image.Pt(width-padX-actionDims.Size.X, height-padY-actionLine+(actionLine-actionDims.Size.Y)/2)).Push(gtx.Ops)
		actionCall.Add(gtx.Ops)
		offset.Pop()
	}

	focused := forceFocus || original.Focused(&t.Click.Clickable)
	focus := previewTransition(original, &t.focus, t.Preview, StateFocus, focused, th.Motion.FocusIn, th.Motion.FocusOut)
	FocusRingAlpha(original, th, bounds, cut, focus)
	return dims
}

func (t *DesignTile) drawPreview(gtx layout.Context, th *theme.Theme, bounds image.Rectangle, cut int) {
	if bounds.Empty() {
		return
	}
	opacity := float32(1)
	if t.Unavailable != "" || !gtx.Source.Enabled() {
		opacity = tokens.AlphaDisabled
	}
	fade := paint.PushOpacity(gtx.Ops, opacity)
	defer fade.Pop()
	shape := CutPath(gtx.Ops, bounds, cut, cut, 0, 0)
	picture := t.Image.Size()
	if picture.X <= 0 || picture.Y <= 0 {
		paint.FillShape(gtx.Ops, th.R.Field, clip.Outline{Path: shape}.Op())
		Hatch(gtx.Ops, CutPath(gtx.Ops, bounds, cut, cut, 0, 0), bounds, gtx.Dp(tokens.SizeChartHatchGap), max(1, gtx.Dp(tokens.SizeChartHatchLine)), palette.WithAlpha(th.R.Fg, tokens.AlphaHatch))
		return
	}
	area := clip.Outline{Path: shape}.Op().Push(gtx.Ops)
	scale := max(float32(bounds.Dx())/float32(picture.X), float32(bounds.Dy())/float32(picture.Y))
	drawn := f32.Pt(float32(picture.X)*scale, float32(picture.Y)*scale)
	origin := f32.Pt(float32(bounds.Min.X)+(float32(bounds.Dx())-drawn.X)/2, float32(bounds.Min.Y)+(float32(bounds.Dy())-drawn.Y)/2)
	transform := op.Affine(f32.AffineId().Scale(f32.Point{}, f32.Pt(scale, scale)).Offset(origin)).Push(gtx.Ops)
	image := t.Image
	image.Filter = paint.FilterLinear
	image.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
	transform.Pop()
	area.Pop()
}

// DesignGallery lays out design tiles in a grid that fits as many columns as
// the minimum tile width allows, and moves the selection with the arrow keys.
type DesignGallery struct {
	Preview State

	columns       int
	changed       int
	changedReady  bool
	activated     int
	activeReady   bool
	selectedFocus bool
}

// designGalleryColumns returns how many tiles of at least minimum width fit
// width with gap between them.
func designGalleryColumns(width, minimum, gap int) int {
	if minimum <= 0 || width <= minimum {
		return 1
	}
	return max(1, (width+gap)/(minimum+gap))
}

// Changed reports the most recent selection change by key or pointer.
func (g *DesignGallery) Changed() (int, bool) { return g.changed, g.changedReady }

// Activated reports the most recent activation by Enter, Space or a click.
func (g *DesignGallery) Activated() (int, bool) { return g.activated, g.activeReady }

// Columns reports the column count of the last layout.
func (g *DesignGallery) Columns() int { return max(1, g.columns) }

// Focus moves keyboard focus to the selected tile.
func (g *DesignGallery) Focus(gtx layout.Context, tiles []DesignTile, selected int) {
	if selected >= 0 && selected < len(tiles) {
		gtx.Execute(key.FocusCmd{Tag: &tiles[selected].Clickable})
	}
}

func (g *DesignGallery) update(gtx layout.Context, tiles []DesignTile, selected, columns int) int {
	g.changedReady, g.activeReady = false, false
	current := selected
	for index := range tiles {
		tag := &tiles[index].Clickable
		for {
			value, ok := gtx.Event(
				key.Filter{Focus: tag, Name: key.NameLeftArrow},
				key.Filter{Focus: tag, Name: key.NameRightArrow},
				key.Filter{Focus: tag, Name: key.NameUpArrow},
				key.Filter{Focus: tag, Name: key.NameDownArrow},
				key.Filter{Focus: tag, Name: key.NameHome},
				key.Filter{Focus: tag, Name: key.NameEnd},
			)
			if !ok {
				break
			}
			pressed, ok := value.(key.Event)
			if !ok || pressed.State != key.Press {
				continue
			}
			from := index
			next := from
			switch pressed.Name {
			case key.NameLeftArrow:
				next = max(0, from-1)
			case key.NameRightArrow:
				next = min(len(tiles)-1, from+1)
			case key.NameUpArrow:
				if from-columns >= 0 {
					next = from - columns
				}
			case key.NameDownArrow:
				if from+columns < len(tiles) {
					next = from + columns
				}
			case key.NameHome:
				next = 0
			case key.NameEnd:
				next = len(tiles) - 1
			}
			if next != current {
				current = next
				g.changed, g.changedReady = current, true
			}
			gtx.Execute(key.FocusCmd{Tag: &tiles[next].Clickable})
		}
		if tiles[index].Clicked(gtx) {
			if index != current {
				current = index
				g.changed, g.changedReady = current, true
			}
			g.activated, g.activeReady = index, true
		}
	}
	return current
}

// Layout draws tiles in the grid, marking tiles[selected] as selected. The
// gallery reads every tile's clicks; use Changed and Activated instead.
func (g *DesignGallery) Layout(gtx layout.Context, th *theme.Theme, tiles []DesignTile, selected int) layout.Dimensions {
	width := gtx.Constraints.Max.X
	// s2 keeps the focus ring's outset clear of the neighbouring tile
	gap := gtx.Dp(th.Sp.S2)
	columns := designGalleryColumns(width, gtx.Dp(tokens.SizeCatalogueTileWidthMin), gap)
	g.columns = columns
	selected = g.update(gtx, tiles, selected, columns)
	tileWidth := min((width-gap*(columns-1))/columns, gtx.Dp(tokens.SizeCatalogueTileWidthMax))
	tileWidth = max(0, tileWidth)
	y, rowHeight := 0, 0
	for index := range tiles {
		column := index % columns
		if column == 0 && index > 0 {
			y += rowHeight + gap
			rowHeight = 0
		}
		tiles[index].Selected = index == selected
		child := gtx
		child.Constraints = layout.Constraints{Min: image.Pt(tileWidth, 0), Max: image.Pt(tileWidth, gtx.Constraints.Max.Y)}
		offset := op.Offset(image.Pt(column*(tileWidth+gap), y)).Push(gtx.Ops)
		dims := tiles[index].layout(child, th, g.Preview == StateFocus && index == selected)
		offset.Pop()
		rowHeight = max(rowHeight, dims.Size.Y)
	}
	height := 0
	if len(tiles) > 0 {
		height = y + rowHeight
	}
	return layout.Dimensions{Size: image.Pt(width, height)}
}
