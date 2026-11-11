package look

import (
	"image"
	"image/color"

	"gioui.org/io/key"
	"gioui.org/io/semantic"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/anim"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/tokens"
)

// MultiLine is a field of several lines, for what kit.TextField's one line
// cannot hold: a brief, a purpose, a member's instructions.
type MultiLine struct {
	Editor      widget.Editor
	Label       string
	Placeholder string
	Help        string
	// Lines is how tall it stands empty; it grows with its text to MaxLines,
	// twice Lines when unset, then scrolls
	Lines, MaxLines int

	focus anim.Tween
	// the field takes the keyboard on its next frame
	wantFocus bool
}

func (m *MultiLine) Text() string { return m.Editor.Text() }

func (m *MultiLine) SetText(value string) { m.Editor.SetText(value) }

// Focus gives the field the keyboard on its next frame.
func (m *MultiLine) Focus() { m.wantFocus = true }

// Submitted reports Ctrl Enter, which starts what the field describes.
func (m *MultiLine) Submitted(gtx layout.Context) bool {
	submitted := false
	for {
		event, ok := gtx.Event(
			key.Filter{Focus: &m.Editor, Name: key.NameReturn, Required: key.ModShortcut},
			key.Filter{Focus: &m.Editor, Name: key.NameEnter, Required: key.ModShortcut},
		)
		if !ok {
			return submitted
		}
		if pressed, ok := event.(key.Event); ok && pressed.State == key.Press {
			submitted = true
		}
	}
}

func (m *MultiLine) Layout(gtx layout.Context, th *Theme) layout.Dimensions {
	if m.wantFocus {
		m.wantFocus = false
		gtx.Execute(key.FocusCmd{Tag: &m.Editor})
	}
	children := make([]layout.FlexChild, 0, 5)
	if m.Label != "" {
		children = append(children,
			layout.Rigid(kit.Text(th.Theme, th.T.Detail, th.R.Fg2, m.Label)),
			layout.Rigid(layout.Spacer{Height: unit.Dp(tokens.SizeTextFieldLabelGap)}.Layout),
		)
	}
	children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions { return m.layoutBox(gtx, th) }))
	if m.Help != "" {
		children = append(children,
			layout.Rigid(layout.Spacer{Height: th.Sp.S1}.Layout),
			layout.Rigid(Paragraph(th, th.T.Caption, th.R.Fg3, m.Help)),
		)
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

func (m *MultiLine) layoutBox(gtx layout.Context, th *Theme) layout.Dimensions {
	style := th.T.Body
	lines := max(1, m.Lines)
	most := m.MaxLines
	if most < lines {
		most = 2 * lines
	}
	width := gtx.Constraints.Max.X
	side, pad := gtx.Dp(unit.Dp(tokens.SizeTextFieldInset)), gtx.Dp(th.Sp.S2)
	line := gtx.Sp(style.Line)
	inner := gtx
	inner.Constraints = layout.Constraints{
		Min: image.Pt(max(0, width-2*side), line*lines),
		Max: image.Pt(max(0, width-2*side), line*most),
	}
	m.Editor.SingleLine, m.Editor.Submit = false, false
	m.Editor.LineHeight, m.Editor.LineHeightScale = style.Line, 1

	record := op.Record(gtx.Ops)
	if m.Editor.Len() == 0 && m.Placeholder != "" {
		kit.Label{Style: style, Color: th.R.Fg3, MaxLines: lines}.Layout(inner, th.Theme, m.Placeholder)
	}
	area := clip.Rect(image.Rectangle{Max: inner.Constraints.Max}).Push(gtx.Ops)
	semantic.Editor.Add(gtx.Ops)
	name := m.Label
	if name == "" {
		name = m.Placeholder
	}
	if name != "" {
		semantic.LabelOp(name).Add(gtx.Ops)
	}
	written := m.Editor.Layout(inner, th.Shaper, style.Face, style.Size, editorInk(gtx.Ops, th.R.Fg), editorInk(gtx.Ops, palette.WithAlpha(th.R.Sel, .6)))
	area.Pop()
	call := record.Stop()

	bounds := image.Rectangle{Max: image.Pt(width, max(written.Size.Y, inner.Constraints.Min.Y)+2*pad)}
	cut := gtx.Dp(th.Cut.Mid)
	kit.FillCut(gtx.Ops, bounds, cut, th.R.Field)
	focused := gtx.Focused(&m.Editor) && gtx.Source.Enabled()
	lit := Transition(gtx, &m.focus, focused, th.Motion.FocusIn, th.Motion.FocusOut)
	kit.StrokeCut(gtx.Ops, bounds, cut, max(1, gtx.Dp(unit.Dp(tokens.SizeHairline))), palette.Mix(th.R.Edge, th.R.Fg2, lit))
	stack := op.Offset(image.Pt(side, pad)).Push(gtx.Ops)
	call.Add(gtx.Ops)
	stack.Pop()
	return layout.Dimensions{Size: bounds.Max}
}

// the material an editor paints its text and selection with
func editorInk(ops *op.Ops, value color.NRGBA) op.CallOp {
	record := op.Record(ops)
	paint.ColorOp{Color: value}.Add(ops)
	return record.Stop()
}
