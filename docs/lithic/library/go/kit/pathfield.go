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

var iconPathChoose = Icon(symbols.FolderOpen)

// PathKind says whether a PathField holds a folder or a file. It only words
// the defaults.
type PathKind uint8

const (
	PathFolder PathKind = iota
	PathFile
)

// PathField holds a file or folder path as literal text with a choose action
// inside the field. The library never opens a dialog: the host reads
// Choose.Clicked, opens the portal and sets Path.
type PathField struct {
	Path        string
	Label       string
	Placeholder string
	Error       string
	Kind        PathKind
	Choose      Click
	Preview     State

	editor      widget.Editor
	choose      IconPlaque
	shown       string
	shownReady  bool
	focused     bool
	edited      string
	editedReady bool
	focus       anim.Tween
}

// Edited reports and clears a path the reader typed and committed with Enter
// or by leaving the field.
func (f *PathField) Edited() (string, bool) {
	path, ok := f.edited, f.editedReady
	f.editedReady = false
	return path, ok
}

// Focus moves keyboard focus to the path text.
func (f *PathField) Focus(gtx layout.Context) {
	gtx.Execute(key.FocusCmd{Tag: &f.editor})
}

func (f *PathField) commit(text string) {
	if text == f.Path {
		return
	}
	f.edited, f.editedReady = text, true
}

func pathPlaceholder(kind PathKind, placeholder string) string {
	switch {
	case placeholder != "":
		return placeholder
	case kind == PathFile:
		return "Choose a file"
	default:
		return "Choose a folder"
	}
}

func (f *PathField) update(gtx layout.Context) {
	f.editor.SingleLine, f.editor.Submit = true, true
	for {
		value, ok := f.editor.Update(gtx)
		if !ok {
			break
		}
		if _, submit := value.(widget.SubmitEvent); submit {
			f.commit(f.editor.Text())
		}
	}
	focused := gtx.Focused(&f.editor)
	if f.focused && !focused {
		f.commit(f.editor.Text())
	}
	f.focused = focused
	if !focused && (!f.shownReady || f.shown != f.Path) {
		f.editor.SetText(f.Path)
		f.shown, f.shownReady = f.Path, true
	}
}

// Layout draws the label, the field with its choose action, and the error.
func (f *PathField) Layout(gtx layout.Context, th *theme.Theme) layout.Dimensions {
	f.update(gtx)
	placeholder := pathPlaceholder(f.Kind, f.Placeholder)
	children := make([]layout.FlexChild, 0, 5)
	if f.Label != "" {
		children = append(children,
			layout.Rigid(Text(th, th.T.Detail, th.R.Fg2, f.Label)),
			layout.Rigid(layout.Spacer{Height: unit.Dp(tokens.SizeTextFieldLabelGap)}.Layout),
		)
	}
	children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
		width := gtx.Constraints.Max.X
		if width <= 0 {
			width = gtx.Dp(unit.Dp(tokens.SizeTextFieldDefaultWidth))
		}
		size := gtx.Constraints.Constrain(image.Pt(width, gtx.Dp(unit.Dp(tokens.SizeTextFieldHeight))))
		cut := gtx.Dp(th.Cut.Mid)
		focus := previewTransition(gtx, &f.focus, f.Preview, StateFocus, gtx.Focused(&f.editor), th.Motion.FocusIn, th.Motion.FocusOut)
		drawFieldSurface(gtx, th, image.Rectangle{Max: size}, cut, th.R.Field, focus, f.Error != "")

		inset := gtx.Dp(unit.Dp(tokens.SizeTextFieldInset))
		action := gtx.Dp(unit.Dp(tokens.SizeTextFieldSecretAction))
		actionInset := gtx.Dp(unit.Dp(tokens.SizeTextFieldSecretInset))
		editorWidth := max(0, size.X-inset-action-2*actionInset)
		editorHeight := max(0, size.Y-2*gtx.Dp(th.Sp.S2))
		offset := op.Offset(image.Pt(inset, (size.Y-editorHeight)/2)).Push(gtx.Ops)
		semantic.Editor.Add(gtx.Ops)
		name := f.Label
		if name == "" {
			name = placeholder
		}
		semantic.LabelOp(name).Add(gtx.Ops)
		if f.Error != "" {
			semantic.DescriptionOp(f.Error).Add(gtx.Ops)
		}
		layoutTextEditor(exactContext(gtx, image.Pt(editorWidth, editorHeight)), th, &f.editor, th.T.Mono, placeholder)
		offset.Pop()

		f.choose.Kind, f.choose.Small, f.choose.Icon = PlaqueQuiet, true, iconPathChoose
		f.choose.Description = placeholder
		if f.Label != "" {
			f.choose.Description = "Choose " + f.Label
		}
		f.choose.Preview = StateRest
		if f.Preview == StateHover || f.Preview == StatePressed {
			f.choose.Preview = f.Preview
		}
		offset = op.Offset(image.Pt(size.X-action-actionInset, (size.Y-action)/2)).Push(gtx.Ops)
		f.choose.layoutWith(exactContext(gtx, image.Pt(action, action)), th, &f.Choose)
		offset.Pop()
		return layout.Dimensions{Size: size}
	}))
	if f.Error != "" {
		children = append(children,
			layout.Rigid(layout.Spacer{Height: unit.Dp(tokens.SizeTextFieldHelpGap)}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return Label{Style: th.T.Caption, Color: th.R.Jasper}.Layout(gtx, th, f.Error)
			}),
		)
	}
	return layout.Flex{Axis: layout.Vertical, Alignment: layout.Start}.Layout(gtx, children...)
}
