package kit

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"strconv"
	"strings"

	"gioui.org/io/key"
	"gioui.org/io/semantic"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/anim"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/theme"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/tokens"
)

const colorFieldInvalid = "Use a colour like #ce5e32"

// ColorSwatch is one role colour a ColorField offers in its popover.
type ColorSwatch struct {
	Label string
	Color color.NRGBA
}

// ColorField edits one colour: a swatch plate, a hex field and a popover of
// the host's role swatches, the hex field and, with AllowAlpha, an opacity
// slider.
type ColorField struct {
	Color      color.NRGBA
	Label      string
	AllowAlpha bool
	Swatches   []ColorSwatch
	Preview    State

	hex        widget.Editor
	hexFocused bool
	hexInvalid bool
	shown      color.NRGBA
	shownReady bool
	disclose   IconPlaque
	popover    Popover
	panelHex   TextField
	panelFocus bool
	alpha      Slider
	swatches   []colorSwatchState
	picked     bool
	focus      anim.Tween
}

type colorSwatchState struct {
	Click
	hover, press, focus anim.Tween
}

// Picked reports and clears the most recent pick. Color already holds it.
func (f *ColorField) Picked() (color.NRGBA, bool) {
	picked := f.picked
	f.picked = false
	return f.Color, picked
}

// Open reports whether the popover is open.
func (f *ColorField) Open() bool { return f.popover.Open }

// Show opens the popover.
func (f *ColorField) Show() { f.popover.Open = true }

// Close dismisses the popover and returns focus to its disclosure.
func (f *ColorField) Close(gtx layout.Context) { f.popover.Close(gtx) }

// Focus moves keyboard focus to the hex field.
func (f *ColorField) Focus(gtx layout.Context) {
	gtx.Execute(key.FocusCmd{Tag: &f.hex})
}

// SubmitHex enters text in the hex field and commits it as Enter would: a
// valid colour is picked, anything else shows the field's error.
func (f *ColorField) SubmitHex(text string) {
	f.hex.SetText(text)
	f.commit(text, false)
}

func (f *ColorField) pick(value color.NRGBA) {
	if !f.AllowAlpha {
		value.A = 0xff
	}
	f.Color = value
	f.picked = true
	f.hexInvalid = false
	f.panelHex.Error = ""
}

func (f *ColorField) commit(text string, panel bool) {
	value, ok := parseHexColor(text, f.AllowAlpha)
	if !ok {
		if panel {
			f.panelHex.Error = colorFieldInvalid
		} else {
			f.hexInvalid = true
		}
		return
	}
	f.pick(value)
	f.shownReady = false
}

func (f *ColorField) update(gtx layout.Context) {
	f.hex.SingleLine, f.hex.Submit = true, true
	f.panelHex.SingleLine, f.panelHex.Submit = true, true
	for {
		value, ok := f.hex.Update(gtx)
		if !ok {
			break
		}
		switch value.(type) {
		case widget.SubmitEvent:
			f.commit(f.hex.Text(), false)
		case widget.ChangeEvent:
			if gtx.Focused(&f.hex) {
				f.hexInvalid = false
			}
		}
	}
	for {
		value, ok := f.panelHex.Update(gtx)
		if !ok {
			break
		}
		switch value.(type) {
		case widget.SubmitEvent:
			f.commit(f.panelHex.Text(), true)
		case widget.ChangeEvent:
			f.panelHex.Error = ""
		}
	}
	focused := gtx.Focused(&f.hex)
	if f.hexFocused && !focused {
		f.commit(f.hex.Text(), false)
	}
	f.hexFocused = focused
	panelFocused := gtx.Focused(&f.panelHex.Editor)
	if f.panelFocus && !panelFocused {
		f.commit(f.panelHex.Text(), true)
	}
	f.panelFocus = panelFocused

	if f.disclose.Clicked(gtx) {
		f.popover.Open = !f.popover.Open
	}
	for index := range f.swatches {
		if index < len(f.Swatches) && f.swatches[index].Clicked(gtx) {
			f.pick(f.Swatches[index].Color)
			f.popover.Close(gtx)
		}
	}
	if f.alpha.Changed() {
		value := f.Color
		value.A = uint8(math.Round(float64(min(max(f.alpha.Value, 0), 1)) * 255))
		f.pick(value)
	}

	if !f.shownReady || f.shown != f.Color {
		text := formatHexColor(f.Color, f.AllowAlpha)
		if !focused && !f.hexInvalid {
			f.hex.SetText(text)
		}
		if !panelFocused && f.panelHex.Error == "" {
			f.panelHex.SetText(text)
		}
		f.shown, f.shownReady = f.Color, true
	}
}

// Layout draws the swatch plate and hex field and owns the popover.
func (f *ColorField) Layout(gtx layout.Context, th *theme.Theme) layout.Dimensions {
	if len(f.swatches) != len(f.Swatches) {
		swatches := make([]colorSwatchState, len(f.Swatches))
		copy(swatches, f.swatches)
		f.swatches = swatches
	}
	f.update(gtx)
	f.popover.Opener = &f.disclose.Clickable
	f.popover.Preview = f.Preview
	return f.popover.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return f.layoutRow(gtx, th)
	}, func(gtx layout.Context) layout.Dimensions {
		return f.layoutPanel(gtx, th)
	})
}

func (f *ColorField) layoutRow(gtx layout.Context, th *theme.Theme) layout.Dimensions {
	height := gtx.Dp(unit.Dp(tokens.SizeTextFieldHeight))
	width := gtx.Constraints.Max.X
	plate := image.Rect(0, 0, height, height)
	drawColorPlate(gtx, th, plate, gtx.Dp(th.Cut.Token), f.Color)

	x := height + gtx.Dp(th.Sp.S2)
	fieldSize := image.Pt(max(0, width-x), height)
	offset := op.Offset(image.Pt(x, 0)).Push(gtx.Ops)
	f.layoutHexField(gtx, th, fieldSize)
	offset.Pop()

	size := image.Pt(x+fieldSize.X, height)
	if !f.hexInvalid {
		return layout.Dimensions{Size: size}
	}
	gap := gtx.Dp(unit.Dp(tokens.SizeTextFieldHelpGap))
	offset = op.Offset(image.Pt(x, height+gap)).Push(gtx.Ops)
	lineGTX := gtx
	lineGTX.Constraints = layout.Constraints{Max: image.Pt(fieldSize.X, gtx.Constraints.Max.Y)}
	message := Label{Style: th.T.Caption, Color: disabledInk(gtx, th.R.Jasper)}.Layout(lineGTX, th, colorFieldInvalid)
	offset.Pop()
	return layout.Dimensions{Size: image.Pt(size.X, height+gap+message.Size.Y)}
}

func (f *ColorField) layoutHexField(gtx layout.Context, th *theme.Theme, size image.Point) {
	bounds := image.Rectangle{Max: size}
	cut := gtx.Dp(th.Cut.Mid)
	focused := gtx.Focused(&f.hex)
	focus := previewTransition(gtx, &f.focus, f.Preview, StateFocus, focused, th.Motion.FocusIn, th.Motion.FocusOut)
	drawFieldSurface(gtx, th, bounds, cut, th.R.Field, focus, f.hexInvalid)

	inset := gtx.Dp(unit.Dp(tokens.SizeTextFieldInset))
	action := gtx.Dp(unit.Dp(tokens.SizeTextFieldSecretAction))
	actionInset := gtx.Dp(unit.Dp(tokens.SizeTextFieldSecretInset))
	editorWidth := max(0, size.X-inset-action-2*actionInset)
	editorHeight := max(0, size.Y-2*gtx.Dp(th.Sp.S2))
	offset := op.Offset(image.Pt(inset, (size.Y-editorHeight)/2)).Push(gtx.Ops)
	semantic.Editor.Add(gtx.Ops)
	name := "Colour"
	if f.Label != "" {
		name = f.Label
	}
	semantic.LabelOp(name).Add(gtx.Ops)
	if f.hexInvalid {
		semantic.DescriptionOp(colorFieldInvalid).Add(gtx.Ops)
	}
	layoutTextEditor(exactContext(gtx, image.Pt(editorWidth, editorHeight)), th, &f.hex, th.T.Mono, "#ce5e32")
	offset.Pop()

	f.disclose.Kind = PlaqueQuiet
	f.disclose.Small = true
	f.disclose.Icon = iconExpandMore
	f.disclose.Description = "Show colour choices"
	if f.popover.Open {
		f.disclose.Icon = iconExpandLess
		f.disclose.Description = "Hide colour choices"
	}
	offset = op.Offset(image.Pt(size.X-action-actionInset, (size.Y-action)/2)).Push(gtx.Ops)
	f.disclose.Layout(exactContext(gtx, image.Pt(action, action)), th)
	offset.Pop()
}

func (f *ColorField) layoutPanel(gtx layout.Context, th *theme.Theme) layout.Dimensions {
	width := min(gtx.Dp(tokens.SizeSettingsControlMax), gtx.Constraints.Max.X)
	gtx.Constraints = layout.Constraints{Min: image.Pt(width, 0), Max: image.Pt(width, gtx.Constraints.Max.Y)}
	record := op.Record(gtx.Ops)
	dims := layout.Inset{Top: unit.Dp(tokens.SizeMenuPaddingVertical), Bottom: th.Sp.S3}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		children := make([]layout.FlexChild, 0, len(f.Swatches)+2)
		for index := range f.Swatches {
			index := index
			children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return f.layoutSwatch(gtx, th, index)
			}))
		}
		if len(f.Swatches) > 0 {
			children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Top: th.Sp.S2, Bottom: th.Sp.S2, Left: unit.Dp(tokens.SizeMenuDividerInset), Right: unit.Dp(tokens.SizeMenuDividerInset)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					size := image.Pt(gtx.Constraints.Max.X, max(1, gtx.Dp(tokens.SizeHairline)))
					FillCut(gtx.Ops, image.Rectangle{Max: size}, 0, th.R.Vein)
					return layout.Dimensions{Size: size}
				})
			}))
		}
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: th.Sp.S1, Left: th.Sp.S3, Right: th.Sp.S3}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return f.layoutPanelFields(gtx, th)
			})
		}))
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
	})
	call := record.Stop()
	bounds := image.Rectangle{Max: dims.Size}
	cut := gtx.Dp(th.Cut.Menu)
	FillCut(gtx.Ops, bounds, cut, th.R.Stone4)
	StrokeCut(gtx.Ops, bounds, cut, max(1, gtx.Dp(tokens.SizeHairline)), th.R.Vein)
	call.Add(gtx.Ops)
	return dims
}

func (f *ColorField) layoutSwatch(gtx layout.Context, th *theme.Theme, index int) layout.Dimensions {
	swatch := f.Swatches[index]
	state := &f.swatches[index]
	height := gtx.Dp(tokens.SizeMenuItem)
	current := swatch.Color == f.Color
	dims := state.Layout(exactContext(gtx, image.Pt(gtx.Constraints.Max.X, height)), func(gtx layout.Context) layout.Dimensions {
		semantic.Button.Add(gtx.Ops)
		semantic.LabelOp(swatch.Label).Add(gtx.Ops)
		semantic.DescriptionOp(formatHexColor(swatch.Color, f.AllowAlpha)).Add(gtx.Ops)
		semantic.SelectedOp(current).Add(gtx.Ops)
		hover := hoverValue(gtx, th, &state.hover, state.Hovered())
		press := transitionValue(gtx, &state.press, state.Pressed(), th.Motion.Press, th.Motion.Press)
		if hover > 0 || press > 0 {
			amount := max(tokens.AlphaHoverPlane*hover, tokens.AlphaPressRow*press)
			FillCut(gtx.Ops, image.Rectangle{Max: gtx.Constraints.Min}, 0, palette.Mix(th.R.Stone4, th.R.Fg, amount))
		}
		return layout.Inset{Left: unit.Dp(tokens.SizeMenuInset), Right: unit.Dp(tokens.SizeMenuInset)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return centerY(gtx, func(gtx layout.Context) layout.Dimensions {
				children := []layout.FlexChild{
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						size := gtx.Dp(tokens.SizeMenuIcon)
						drawColorPlate(gtx, th, image.Rect(0, 0, size, size), gtx.Dp(th.Cut.Token), swatch.Color)
						return layout.Dimensions{Size: image.Pt(size, size)}
					}),
					layout.Rigid(layout.Spacer{Width: unit.Dp(tokens.SizeMenuIconGap)}.Layout),
					layout.Flexed(1, rawText(th, th.T.Label, th.R.Fg, swatch.Label)),
				}
				if current {
					children = append(children,
						layout.Rigid(layout.Spacer{Width: unit.Dp(tokens.SizeMenuShortcutGap)}.Layout),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							size := gtx.Dp(tokens.SizeMenuIcon)
							gtx.Constraints = layout.Exact(image.Pt(size, size))
							return iconMenuCheck.Layout(gtx, th.R.Fg2)
						}),
					)
				}
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx, children...)
			})
		})
	})
	focus := focusValue(gtx, th, &state.focus, gtx.Focused(&state.Clickable))
	FocusRingAlpha(gtx, th, image.Rectangle{Max: dims.Size}, 0, focus)
	return dims
}

func (f *ColorField) layoutPanelFields(gtx layout.Context, th *theme.Theme) layout.Dimensions {
	f.panelHex.Label = "Hex"
	f.panelHex.Mono = true
	children := []layout.FlexChild{layout.Rigid(func(gtx layout.Context) layout.Dimensions {
		return f.panelHex.Layout(gtx, th)
	})}
	if f.AllowAlpha {
		f.alpha.Min, f.alpha.Max = 0, 1
		f.alpha.Label = "Opacity"
		if !f.alpha.Pressed() {
			f.alpha.Value = float32(f.Color.A) / 255
		}
		percent := strconv.Itoa(int(math.Round(float64(f.Color.A)/255*100))) + "%"
		children = append(children,
			layout.Rigid(layout.Spacer{Height: th.Sp.S3}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints.Min.X = gtx.Constraints.Max.X
				return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
					layout.Flexed(1, Text(th, th.T.Detail, th.R.Fg2, "Opacity")),
					layout.Rigid(Text(th, th.T.Detail, th.R.Fg2, percent)),
				)
			}),
			layout.Rigid(layout.Spacer{Height: th.Sp.S1}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return f.alpha.Layout(gtx, th)
			}),
		)
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

// drawColorPlate paints value over a hatch so its alpha shows, inside an edge
// hairline.
func drawColorPlate(gtx layout.Context, th *theme.Theme, bounds image.Rectangle, cut int, value color.NRGBA) {
	FillCut(gtx.Ops, bounds, cut, disabledInk(gtx, th.R.Field))
	if value.A < 0xff {
		Hatch(gtx.Ops, CutPath(gtx.Ops, bounds, cut, cut, cut, cut), bounds, gtx.Dp(tokens.SizeChartHatchGap), max(1, gtx.Dp(tokens.SizeChartHatchLine)), disabledInk(gtx, palette.WithAlpha(th.R.Fg, tokens.AlphaHatch)))
	}
	FillCut(gtx.Ops, bounds, cut, disabledInk(gtx, value))
	StrokeCut(gtx.Ops, bounds, cut, max(1, gtx.Dp(unit.Dp(tokens.SizeHairline))), disabledInk(gtx, th.R.Edge))
}

// parseHexColor reads #rgb or #rrggbb, plus #rgba or #rrggbbaa when alpha is
// allowed. The hash is optional and case does not matter.
func parseHexColor(text string, allowAlpha bool) (color.NRGBA, bool) {
	value := strings.TrimPrefix(strings.TrimSpace(text), "#")
	switch len(value) {
	case 3, 6:
	case 4, 8:
		if !allowAlpha {
			return color.NRGBA{}, false
		}
	default:
		return color.NRGBA{}, false
	}
	if len(value) <= 4 {
		var long strings.Builder
		for _, digit := range value {
			long.WriteRune(digit)
			long.WriteRune(digit)
		}
		value = long.String()
	}
	number, err := strconv.ParseUint(value, 16, 32)
	if err != nil {
		return color.NRGBA{}, false
	}
	if len(value) == 6 {
		number = number<<8 | 0xff
	}
	return color.NRGBA{R: uint8(number >> 24), G: uint8(number >> 16), B: uint8(number >> 8), A: uint8(number)}, true
}

// formatHexColor writes lowercase #rrggbb, or #rrggbbaa when alpha is allowed
// and the colour is not opaque.
func formatHexColor(value color.NRGBA, allowAlpha bool) string {
	if allowAlpha && value.A < 0xff {
		return fmt.Sprintf("#%02x%02x%02x%02x", value.R, value.G, value.B, value.A)
	}
	return fmt.Sprintf("#%02x%02x%02x", value.R, value.G, value.B)
}
