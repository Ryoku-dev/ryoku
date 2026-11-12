// Package term renders the terminal subset of Lithic with Lip Gloss and Bubble Tea.
package term

import (
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/tokens"
)

// Theme carries the semantic palette roles used by terminal components.
type Theme struct {
	Roles palette.Roles
}

// NewTheme resolves the palette for mode with the library's default look:
// the signature in Dark and Light, the desktop palette in Follow. An
// application offering contrast and tint builds a Theme from
// palette.FromPalette with the reader's palette.Look instead.
func NewTheme(mode palette.Mode) Theme {
	return Theme{Roles: palette.FromPalette(palette.Load(mode), palette.Look{})}
}

// CutBorder is the terminal translation of a four-corner cut frame.
func CutBorder() lipgloss.Border {
	g := glyphs()
	return lipgloss.Border{
		Top: g.FrameTop, Bottom: g.FrameBottom, Left: g.FrameLeft, Right: g.FrameRight,
		TopLeft: g.FrameTopLeft, TopRight: g.FrameTopRight,
		BottomLeft: g.FrameBottomLeft, BottomRight: g.FrameBottomRight,
	}
}

func ink(c color.Color) lipgloss.Style {
	return lipgloss.NewStyle().Foreground(c)
}

func plane(fg, bg color.Color) lipgloss.Style {
	return lipgloss.NewStyle().Foreground(fg).Background(bg)
}

func bold(c color.Color) lipgloss.Style {
	return ink(c).Bold(true)
}

type planeSpan struct {
	text string
	ink  color.Color
	bold bool
}

func renderPlaneLine(width int, background color.Color, spans ...planeSpan) string {
	var b strings.Builder
	for _, span := range spans {
		b.WriteString(lipgloss.NewStyle().
			Foreground(span.ink).
			Background(background).
			Bold(span.bold).
			Render(span.text))
	}
	line := b.String()
	if gap := width - lipgloss.Width(line); gap > 0 {
		line += lipgloss.NewStyle().Background(background).Render(strings.Repeat(" ", gap))
	}
	return line
}

func renderPlaneText(width int, background, foreground color.Color, text string, boldText bool) string {
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		lines[i] = renderPlaneLine(width, background, planeSpan{text: line, ink: foreground, bold: boldText})
	}
	return strings.Join(lines, "\n")
}

func renderPlaneRaw(width int, background color.Color, text string) string {
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		rendered := lipgloss.NewStyle().Background(background).MaxWidth(width).Render(line)
		if gap := width - lipgloss.Width(rendered); gap > 0 {
			rendered += lipgloss.NewStyle().Background(background).Render(strings.Repeat(" ", gap))
		}
		lines[i] = rendered
	}
	return strings.Join(lines, "\n")
}

// centerPlaneRaw centres a block on a plane line by line, so a framed mark
// keeps its shape.
func centerPlaneRaw(width int, background color.Color, content string) string {
	content = lipgloss.NewStyle().MaxWidth(width).Render(content)
	gap := max(0, width-lipgloss.Width(content))
	left := gap / 2
	style := lipgloss.NewStyle().Background(background)
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		right := max(0, width-left-lipgloss.Width(line))
		lines[i] = style.Render(strings.Repeat(" ", left)) + line + style.Render(strings.Repeat(" ", right))
	}
	return strings.Join(lines, "\n")
}

// stone renders content as a filled plane cut like Lithic stone: the content
// rows on the fill, between a third-height cap above and below whose ends slope
// like the chamfer. A stone has no outline; its fill shows the cut. plane is the
// colour around it, or nil for the terminal's own background.
func stone(th Theme, content string, fill, plane color.Color, disabled bool) string {
	foreground := th.Roles.Fg
	if disabled {
		foreground = th.Roles.Fg3
	}
	body := lipgloss.NewStyle().
		Foreground(foreground).
		Background(fill).
		Padding(0, horizontalPadding(tokens.SpaceS2)).
		Render(onFill(content, fill))
	inner := max(0, lipgloss.Width(body)-2)
	capStyle := lipgloss.NewStyle().Foreground(fill)
	if plane != nil {
		capStyle = capStyle.Background(plane)
	}
	g := glyphs()
	top := capStyle.Render(g.StoneTopLeft + strings.Repeat(g.StoneTop, inner) + g.StoneTopRight)
	bottom := capStyle.Render(g.StoneBottomLeft + strings.Repeat(g.StoneBottom, inner) + g.StoneBottomRight)
	return top + "\n" + body + "\n" + bottom
}

// onFill keeps a plane's fill under pre-styled spans: every reset inside
// content re-opens the fill's background, so the end of a span does not punch
// a hole through the plane to the terminal behind it.
func onFill(content string, fill color.Color) string {
	opened := lipgloss.NewStyle().Background(fill).Render(" ")
	opened = opened[:strings.Index(opened, " ")]
	if opened == "" {
		return content
	}
	return strings.ReplaceAll(content, "\x1b[m", "\x1b[m"+opened)
}

// fieldFill is a field's plane: lighter while it has focus.
func fieldFill(th Theme, fill color.NRGBA, focused, disabled bool) color.NRGBA {
	if !focused || disabled {
		return fill
	}
	return palette.Mix(fill, th.Roles.Fg, tokens.AlphaHoverFill)
}

func framed(th Theme, content string, fill color.NRGBA, focused, disabled bool) string {
	return stone(th, content, fieldFill(th, fill, focused, disabled), nil, disabled)
}

func framedOnPlane(th Theme, content string, fill color.NRGBA, plane color.Color, focused, disabled bool) string {
	return stone(th, content, fieldFill(th, fill, focused, disabled), plane, disabled)
}

// outlined renders content inside a cut hairline frame on the terminal's own
// background: a group set off by its vein, not by a fill.
func outlined(th Theme, content string) string {
	return lipgloss.NewStyle().
		Foreground(th.Roles.Fg).
		Border(CutBorder()).
		BorderForeground(th.Roles.Vein).
		Padding(0, horizontalPadding(tokens.SpaceS2)).
		Render(content)
}

func focusGutter(th Theme, focused bool) string {
	if focused {
		return ink(th.Roles.Fg).Render(glyphs().Cursor + " ")
	}
	return "  "
}

func focusGutterOn(th Theme, focused bool, background color.Color) string {
	style := lipgloss.NewStyle().Background(background)
	if focused {
		return style.Foreground(th.Roles.Fg).Render(glyphs().Cursor + " ")
	}
	return style.Render("  ")
}

func horizontalPadding(space int) int {
	if space >= tokens.SpaceS4 {
		return 2
	}
	return 1
}

func verticalPadding(space int) int {
	if space >= tokens.SpaceS3 {
		return 1
	}
	return 0
}

func fit(text string, width int) string {
	if width <= 0 {
		return ""
	}
	if lipgloss.Width(text) <= width {
		return text + strings.Repeat(" ", width-lipgloss.Width(text))
	}
	runes := []rune(text)
	for len(runes) > 1 && lipgloss.Width(string(runes))+1 > width {
		runes = runes[:len(runes)-1]
	}
	if width == 1 {
		return "…"
	}
	return string(runes) + "…"
}

func clampIndex(index, count int) int {
	if count <= 0 {
		return 0
	}
	if index < 0 {
		return 0
	}
	if index >= count {
		return count - 1
	}
	return index
}

func toneName(tone palette.Tone) string {
	switch tone {
	case palette.Alive:
		return "Alive"
	case palette.Becoming:
		return "Becoming"
	case palette.Attention:
		return "Needs you"
	case palette.Paused:
		return "Paused"
	case palette.Failed:
		return "Failed"
	default:
		return "Idle"
	}
}
