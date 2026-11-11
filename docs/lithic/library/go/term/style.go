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

// NewTheme resolves the live Ryoku palette for a terminal application with the
// library's default look. An application offering contrast and tint builds a
// Theme from palette.FromPalette with the reader's palette.Look instead.
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

func centerPlaneRaw(width int, background color.Color, content string) string {
	content = lipgloss.NewStyle().MaxWidth(width).Render(content)
	gap := max(0, width-lipgloss.Width(content))
	left := gap / 2
	right := gap - left
	style := lipgloss.NewStyle().Background(background)
	return style.Render(strings.Repeat(" ", left)) + content + style.Render(strings.Repeat(" ", right))
}

func frameStyle(th Theme, background, border color.Color, disabled bool) lipgloss.Style {
	foreground := th.Roles.Fg
	if disabled {
		foreground = th.Roles.Fg3
	}
	return lipgloss.NewStyle().
		Foreground(foreground).
		Background(background).
		Border(CutBorder()).
		BorderForeground(border).
		Padding(0, horizontalPadding(tokens.SpaceS2))
}

func framed(th Theme, content string, background color.Color, focused, disabled bool) string {
	border := th.Roles.Vein
	if focused && !disabled {
		border = th.Roles.Fg2
	}
	return frameStyle(th, background, border, disabled).Render(content)
}

func framedWithBorder(th Theme, content string, background, border color.Color, disabled bool) string {
	return frameStyle(th, background, border, disabled).Render(content)
}

func framedOnPlane(th Theme, content string, background, planeBackground color.Color, focused, disabled bool) string {
	border := th.Roles.Vein
	if focused && !disabled {
		border = th.Roles.Fg2
	}
	return frameStyle(th, background, border, disabled).
		BorderBackground(planeBackground).
		Render(content)
}

func framedOnPlaneWithBorder(th Theme, content string, background, planeBackground, border color.Color, disabled bool) string {
	return frameStyle(th, background, border, disabled).
		BorderBackground(planeBackground).
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
