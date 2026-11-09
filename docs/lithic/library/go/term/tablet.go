package term

import (
	"charm.land/lipgloss/v2"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/tokens"
)

// Tablet describes one resource row.
type Tablet struct {
	Name     string
	Facts    string
	Claim    string
	Proof    string
	Action   string
	Tone     palette.Tone
	Progress float64
	Selected bool
	Focused  bool
	Disabled bool
	Width    int
}

// RenderTablet renders a resource row and its state mark.
func RenderTablet(th Theme, tablet Tablet) string {
	width := tablet.Width
	if width <= 0 {
		width = tokens.SizeListMedium / tokens.SpaceS2
	}
	background := th.Roles.Stone2
	nameColour, quietColour := th.Roles.Fg, th.Roles.Fg2
	if tablet.Selected {
		background = th.Roles.Sel
		nameColour, quietColour = th.Roles.OnSel, th.Roles.OnSel2
	}
	if tablet.Disabled {
		background = th.Roles.Stone2
		nameColour, quietColour = th.Roles.Fg3, th.Roles.Fg3
	}
	gutter := focusGutterOn(th, tablet.Focused && !tablet.Disabled, background)
	left := gutter +
		lipgloss.NewStyle().Foreground(th.Roles.Stone(tablet.Tone)).Background(background).Render(glyphs().Notch) +
		lipgloss.NewStyle().Background(background).Render(" ") +
		lipgloss.NewStyle().Foreground(nameColour).Background(background).Bold(true).Render(tablet.Name)
	if tablet.Facts != "" {
		left += lipgloss.NewStyle().Foreground(quietColour).Background(background).Render("  " + tablet.Facts)
	}
	right := tablet.Claim
	if tablet.Focused && tablet.Action != "" {
		right = "[ " + tablet.Action + " ]"
	} else if tablet.Proof != "" {
		right += "  " + tablet.Proof
	}
	right = lipgloss.NewStyle().Foreground(quietColour).Background(background).Render(right)
	gap := max(1, width-lipgloss.Width(left)-lipgloss.Width(right))
	content := left + lipgloss.NewStyle().Background(background).Render(spaces(gap)) + right
	row := lipgloss.NewStyle().
		Foreground(nameColour).
		Background(background).
		Width(width).
		MaxWidth(width).
		Render(content)
	if tablet.Tone == palette.Becoming {
		row += "\n" + renderBladeColor(th, width, tablet.Progress, th.Roles.Lapis)
	}
	return row
}

func spaces(count int) string {
	if count < 1 {
		return ""
	}
	b := make([]byte, count)
	for i := range b {
		b[i] = ' '
	}
	return string(b)
}
