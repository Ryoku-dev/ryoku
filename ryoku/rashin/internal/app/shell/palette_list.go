package shell

import (
	"gioui.org/io/semantic"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"image"
	"strings"
	"unicode/utf8"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/anim"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/look"
)

// a menu in everything but its width, which a menu caps
type paletteList struct {
	rows []paletteRow
	// the row under the pointer, so moving onto a row moves the cursor once and the arrow keys stay free
	hovered int
}

type paletteRow struct {
	click kit.Click
	hover anim.Tween
	press anim.Tween
}

func (list *paletteList) Layout(
	gtx layout.Context, th *look.Theme, actions []paletteAction, cursor int, query string,
) (dimensions layout.Dimensions, pressed, entered int) {
	if len(list.rows) < len(actions) {
		list.rows = append(list.rows, make([]paletteRow, len(actions)-len(list.rows))...)
	}
	width := gtx.Constraints.Max.X
	pad, row, caption := gtx.Dp(6), gtx.Dp(46), gtx.Dp(24)
	pressed, entered = -1, -1
	hovered := -1

	record := op.Record(gtx.Ops)
	y := pad
	group := ""
	for index, action := range actions {
		if action.group != group {
			group = action.group
			look.Place(gtx, image.Rect(pad, y, width-pad, y+caption), func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints.Min = image.Point{}
				look.MiddleLeft(gtx, gtx.Dp(10), caption, kit.RawText(th.Theme, th.T.Caption, th.R.Fg3, group))
				return layout.Dimensions{Size: gtx.Constraints.Max}
			})
			y += caption
		}
		state := &list.rows[index]
		if state.click.Clicked(gtx) {
			pressed = index
		}
		if state.click.Hovered() {
			hovered = index
		}
		look.Place(gtx, image.Rect(pad, y, width-pad, y+row), func(gtx layout.Context) layout.Dimensions {
			return state.click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				semantic.Button.Add(gtx.Ops)
				semantic.LabelOp(action.label).Add(gtx.Ops)
				drawPaletteRow(gtx, th, state, action, index == cursor, query)
				return layout.Dimensions{Size: gtx.Constraints.Max}
			})
		})
		y += row
	}
	if len(actions) == 0 {
		look.Place(gtx, image.Rect(pad, y, width-pad, y+row), func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min = image.Point{}
			look.MiddleLeft(gtx, gtx.Dp(10), row, kit.RawText(th.Theme, th.T.Label, th.R.Fg3, i18n.T("Nothing matches")))
			return layout.Dimensions{Size: gtx.Constraints.Max}
		})
		y += row
	}
	y += pad
	call := record.Stop()

	bounds := image.Rect(0, 0, width, y)
	cut := gtx.Dp(th.Cut.Menu)
	kit.Fill(gtx, th.Theme, bounds, cut, th.R.Stone4)
	kit.Stroke(gtx, th.Theme, bounds, cut, max(1, gtx.Dp(1)), th.R.Vein)
	call.Add(gtx.Ops)

	if hovered != list.hovered {
		list.hovered, entered = hovered, hovered
	}
	return layout.Dimensions{Size: bounds.Max}, pressed, entered
}

func drawPaletteRow(gtx layout.Context, th *look.Theme, state *paletteRow, action paletteAction, raised bool, query string) {
	size := gtx.Constraints.Max
	hover := look.Transition(gtx, &state.hover, raised, th.Motion.HoverIn, th.Motion.HoverOut)
	press := look.Transition(gtx, &state.press, state.click.Pressed(), th.Motion.Press, th.Motion.Press)
	look.RowWash(gtx, th, image.Rectangle{Max: size}, hover, press, false)
	inset, glyph := gtx.Dp(10), gtx.Dp(20)
	look.PlaceIcon(gtx, action.icon, image.Pt(inset, (size.Y-glyph)/2), 20, palette.Mix(th.R.Fg2, th.R.Fg, hover))
	left := inset + glyph + gtx.Dp(12)
	crumb := action.crumb
	if crumb == "" {
		crumb = action.kind
	}
	area := clip.Rect(image.Rect(left, 0, size.X-inset, size.Y)).Push(gtx.Ops)
	look.Place(gtx, image.Rect(left, gtx.Dp(4), size.X-inset, gtx.Dp(4)+gtx.Sp(th.T.Label.Line)), func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min = image.Point{}
		return paletteLabel(gtx, th, action.label, query)
	})
	look.Place(gtx, image.Rect(left, gtx.Dp(4)+gtx.Sp(th.T.Label.Line), size.X-inset, size.Y), func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min = image.Point{}
		return kit.RawText(th.Theme, th.T.MonoSm, th.R.Fg3, crumb)(gtx)
	})
	area.Pop()
}

func paletteLabel(gtx layout.Context, th *look.Theme, label, query string) layout.Dimensions {
	if query == "" {
		return kit.RawText(th.Theme, th.T.Label, th.R.Fg, label)(gtx)
	}
	runes := utf8.RuneCountInString(query)
	at, end := -1, -1
	for start := range label {
		finish := start
		for range runes {
			if finish >= len(label) {
				break
			}
			_, width := utf8.DecodeRuneInString(label[finish:])
			finish += width
		}
		if strings.EqualFold(label[start:finish], query) {
			at, end = start, finish
			break
		}
	}
	if at < 0 {
		return kit.RawText(th.Theme, th.T.Label, th.R.Fg, label)(gtx)
	}
	start, match, after := label[:at], label[at:end], label[end:]
	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Baseline}.Layout(gtx,
		layout.Rigid(kit.RawText(th.Theme, th.T.Label, th.R.Fg, start)),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			d := kit.RawText(th.Theme, th.T.Label, th.R.Fg, match)(gtx)
			kit.Fill(gtx, th.Theme, image.Rect(0, d.Size.Y-gtx.Dp(2), d.Size.X, d.Size.Y-gtx.Dp(1)), 0, th.R.Fg)
			return d
		}),
		layout.Rigid(kit.RawText(th.Theme, th.T.Label, th.R.Fg, after)),
	)
}
