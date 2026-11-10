package chat

import (
	"image"
	"image/color"
	"slices"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"

	"ryoku-rashin/internal/app/look"
)

// how wide each column would like to be only changes when the table's words do
type tableState struct {
	head    []string
	rows    [][]string
	metric  unit.Metric
	natural []int
	numeric []bool
}

func (table *tableState) holds(head []string, rows [][]string, metric unit.Metric) bool {
	if table.metric != metric || !slices.Equal(table.head, head) || len(table.rows) != len(rows) {
		return false
	}
	for index := range rows {
		if !slices.Equal(table.rows[index], rows[index]) {
			return false
		}
	}
	return true
}

func (state *viewState) layoutMarkdownTable(gtx layout.Context, th *look.Theme, key string, head []string, rows [][]string) layout.Dimensions {
	columns := len(head)
	if columns == 0 {
		return layout.Dimensions{}
	}
	table := state.tables[key]
	if table == nil {
		table = new(tableState)
		state.tables[key] = table
	}
	if !table.holds(head, rows, gtx.Metric) {
		table.head, table.rows, table.metric = head, rows, gtx.Metric
		table.natural, table.numeric = state.measureTable(gtx, th, head, rows)
	}
	gap := gtx.Dp(34)
	widths := shareWidth(table.natural, gtx.Constraints.Max.X-gap*(columns-1), gtx.Dp(72))
	total := gap * (columns - 1)
	for _, width := range widths {
		total += width
	}

	line := max(1, gtx.Dp(1))
	// a row keeps the height it had with one line of text, and grows by the line when a cell wraps
	drawRow := func(y int, rowKey string, cells []string, style look.TextStyle, ink color.NRGBA, least int) int {
		pad := max(0, (least-gtx.Sp(style.Line))/2)
		calls := make([]op.CallOp, columns)
		sizes := make([]image.Point, columns)
		height := 0
		for column := range columns {
			value := ""
			if column < len(cells) {
				value = cells[column]
			}
			record := op.Record(gtx.Ops)
			cell := gtx
			cell.Constraints = layout.Constraints{Max: image.Pt(widths[column], gtx.Constraints.Max.Y)}
			sizes[column] = state.layoutInline(cell, th, rowKey+":"+strconv.Itoa(column), style, ink, value).Size
			calls[column] = record.Stop()
			height = max(height, sizes[column].Y)
		}
		x := 0
		for column := range columns {
			at := image.Pt(x, y+pad)
			if table.numeric[column] {
				at.X += widths[column] - sizes[column].X
			}
			offset := op.Offset(at).Push(gtx.Ops)
			calls[column].Add(gtx.Ops)
			offset.Pop()
			x += widths[column] + gap
		}
		return max(least, height+2*pad)
	}

	y := drawRow(0, key+":head", head, th.T.Caption, th.R.Fg3, gtx.Dp(28))
	paint.FillShape(gtx.Ops, th.R.Vein, clip.Rect(image.Rect(0, y, total, y+line)).Op())
	y += line
	for index, row := range rows {
		y += drawRow(y, key+":"+strconv.Itoa(index), row, th.T.Body, th.R.Fg, gtx.Dp(34))
		if index < len(rows)-1 {
			paint.FillShape(gtx.Ops, palette.WithAlpha(th.R.Vein, .5), clip.Rect(image.Rect(0, y, total, y+line)).Op())
			y += line
		}
	}
	return layout.Dimensions{Size: image.Pt(total, y)}
}

// a column of numbers stands against its right edge
func (state *viewState) measureTable(gtx layout.Context, th *look.Theme, head []string, rows [][]string) (natural []int, numeric []bool) {
	natural, numeric = make([]int, len(head)), make([]bool, len(head))
	measure := func(style look.TextStyle, value string) int {
		spans := parseInline(value)
		if len(spans) == 0 {
			return 0
		}
		faces := inlineFaces{base: style, mono: th.T.Mono, pad: 6}
		return setInline(th.Shaper, gtx.Metric, gtx.Locale, faces, spans, unwrapped).size.X
	}
	for column := range head {
		natural[column] = measure(th.T.Caption, head[column])
		numeric[column] = len(rows) > 0
		for _, row := range rows {
			if column >= len(row) {
				continue
			}
			natural[column] = max(natural[column], measure(th.T.Body, row[column]))
			numeric[column] = numeric[column] && isNumeric(row[column])
		}
	}
	return natural, numeric
}

// when not all fit, narrow columns keep their width and wide ones split the rest
// so a column of short names is not squeezed to make room for a column of prose
func shareWidth(natural []int, room, least int) []int {
	widths := make([]int, len(natural))
	order := make([]int, len(natural))
	for column := range natural {
		widths[column] = max(natural[column], least)
		order[column] = column
	}
	sort.SliceStable(order, func(a, b int) bool { return widths[order[a]] < widths[order[b]] })
	for placed, column := range order {
		share := room / (len(order) - placed)
		if widths[column] > share {
			for _, wide := range order[placed:] {
				widths[wide] = max(1, share)
			}
			break
		}
		room -= widths[column]
	}
	return widths
}

func isNumeric(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" {
		return false
	}
	digits := 0
	for _, character := range value {
		switch {
		case unicode.IsDigit(character):
			digits++
		case strings.ContainsRune(".,%+-−  ", character):
		default:
			return false
		}
	}
	return digits > 0
}
