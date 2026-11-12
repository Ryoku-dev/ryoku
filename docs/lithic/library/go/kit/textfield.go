package kit

import (
	"image"

	"gioui.org/io/key"
	"gioui.org/io/semantic"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/anim"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/symbols"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/theme"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/tokens"
)

var (
	iconVisibility    = Icon(symbols.Visibility)
	iconVisibilityOff = Icon(symbols.VisibilityOff)
)

// TextField is a labelled single-line editor with help and error text.
type TextField struct {
	widget.Editor
	Label       string
	Placeholder string
	Help        string
	Error       string
	Secret      bool
	Mono        bool
	Preview     State
	reveal      IconPlaque
	revealed    bool
	focus       anim.Tween
}

// Focus moves keyboard focus to the editor.
func (f *TextField) Focus(gtx layout.Context) {
	gtx.Execute(key.FocusCmd{Tag: &f.Editor})
}

// Revealed reports whether a secret value is currently visible.
func (f *TextField) Revealed() bool { return f.revealed }

// Layout draws and operates the field.
func (f *TextField) Layout(gtx layout.Context, th *theme.Theme) layout.Dimensions {
	if f.reveal.Clicked(gtx) {
		f.revealed = !f.revealed
		f.Focus(gtx)
	}
	f.SingleLine = true
	f.Mask = 0
	if f.Secret && !f.revealed {
		f.Mask = '•'
	}
	style := th.T.Body
	if f.Mono {
		style = th.T.Mono
	}

	var children [5]layout.FlexChild
	count := 0
	if f.Label != "" {
		children[count] = layout.Rigid(Text(th, th.T.Detail, th.R.Fg2, f.Label))
		count++
		children[count] = layout.Rigid(layout.Spacer{Height: unit.Dp(tokens.SizeTextFieldLabelGap)}.Layout)
		count++
	}
	children[count] = layout.Rigid(func(gtx layout.Context) layout.Dimensions {
		width := gtx.Constraints.Max.X
		if width <= 0 {
			width = gtx.Dp(unit.Dp(tokens.SizeTextFieldDefaultWidth))
		}
		height := gtx.Dp(unit.Dp(tokens.SizeTextFieldHeight))
		size := gtx.Constraints.Constrain(image.Pt(width, height))
		bounds := image.Rectangle{Max: size}
		cut := gtx.Dp(th.Cut.Mid)
		focus := previewTransition(gtx, &f.focus, f.Preview, StateFocus, gtx.Focused(&f.Editor), th.Motion.FocusIn, th.Motion.FocusOut)
		drawFieldSurface(gtx, th, bounds, cut, th.R.Field, focus, f.Error != "")

		innerWidth := max(0, size.X-2*gtx.Dp(unit.Dp(tokens.SizeTextFieldInset)))
		if f.Secret {
			innerWidth = max(0, innerWidth-gtx.Dp(unit.Dp(tokens.SizeTextFieldSecretAction+tokens.SizeTextFieldSecretInset)))
		}
		editorHeight := max(0, size.Y-2*gtx.Dp(th.Sp.S2))
		editorGTX := exactContext(gtx, image.Pt(innerWidth, editorHeight))
		offset := op.Offset(image.Pt(gtx.Dp(unit.Dp(tokens.SizeTextFieldInset)), (size.Y-editorHeight)/2)).Push(gtx.Ops)
		semantic.Editor.Add(gtx.Ops)
		if f.Label != "" {
			semantic.LabelOp(f.Label).Add(gtx.Ops)
		}
		message := f.Help
		if f.Error != "" {
			message = f.Error
		}
		if message != "" {
			semantic.DescriptionOp(message).Add(gtx.Ops)
		}
		layoutTextEditor(editorGTX, th, &f.Editor, style, f.Placeholder)
		offset.Pop()

		if f.Secret {
			icon, description := iconVisibility, "Show value"
			if f.revealed {
				icon, description = iconVisibilityOff, "Hide value"
			}
			f.reveal.Small = true
			f.reveal.Kind = PlaqueQuiet
			f.reveal.Icon = icon
			f.reveal.Description = description
			buttonSize := gtx.Dp(unit.Dp(tokens.SizeTextFieldSecretAction))
			offset = op.Offset(image.Pt(size.X-buttonSize-gtx.Dp(unit.Dp(tokens.SizeTextFieldSecretInset)), (size.Y-buttonSize)/2)).Push(gtx.Ops)
			f.reveal.Layout(exactContext(gtx, image.Pt(buttonSize, buttonSize)), th)
			offset.Pop()
		}
		return layout.Dimensions{Size: size}
	})
	count++

	message, messageColor := f.Help, th.R.Fg3
	if f.Error != "" {
		message, messageColor = f.Error, th.R.Jasper
	}
	if message != "" {
		children[count] = layout.Rigid(layout.Spacer{Height: unit.Dp(tokens.SizeTextFieldHelpGap)}.Layout)
		count++
		children[count] = layout.Rigid(Text(th, th.T.Caption, disabledInk(gtx, messageColor), message))
		count++
	}
	return layout.Flex{Axis: layout.Vertical, Alignment: layout.Start}.Layout(gtx, children[:count]...)
}

func layoutTextEditor(gtx layout.Context, th *theme.Theme, editor *widget.Editor, style theme.TextStyle, placeholder string) layout.Dimensions {
	size := gtx.Constraints.Constrain(gtx.Constraints.Max)
	lineGTX := gtx
	lineGTX.Constraints.Min = image.Pt(size.X, 0)
	lineGTX.Constraints.Max = size

	record := op.Record(gtx.Ops)
	textCall := textMaterial(gtx.Ops, disabledInk(gtx, th.R.Fg))
	selectionCall := textMaterial(gtx.Ops, disabledInk(gtx, th.R.Sel))
	editor.LineHeight = 0
	noteOverlayFocus(gtx, editor)
	editorDims := editor.Layout(lineGTX, th.Shaper, style.Face, style.Size, textCall, selectionCall)
	call := record.Stop()
	y := max(0, (size.Y-editorDims.Size.Y)/2)
	offset := op.Offset(image.Pt(0, y)).Push(gtx.Ops)
	call.Add(gtx.Ops)
	offset.Pop()

	if editor.Len() == 0 {
		hintGTX := gtx
		hintGTX.Constraints.Min = image.Point{}
		hintGTX.Constraints.Max = size
		record = op.Record(gtx.Ops)
		hintDims := rawText(th, style, disabledInk(gtx, th.R.Fg3), placeholder)(hintGTX)
		call = record.Stop()
		hintY := max(0, (size.Y-hintDims.Size.Y)/2)
		offset = op.Offset(image.Pt(0, hintY)).Push(gtx.Ops)
		call.Add(gtx.Ops)
		offset.Pop()
	}
	return layout.Dimensions{Size: size, Baseline: size.Y - y - editorDims.Size.Y + editorDims.Baseline}
}
