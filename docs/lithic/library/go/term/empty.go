package term

import (
	"charm.land/lipgloss/v2"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/tokens"
	"strings"
)

// Empty describes an actionable empty state.
type Empty struct {
	Title   string
	Body    string
	Actions []Plaque
	Width   int
}

// RenderEmpty renders the framed mark and centred next step on stone2.
func RenderEmpty(th Theme, empty Empty) string {
	width := max(tokens.SizeEmptyBodyMax/tokens.SpaceS2, empty.Width)
	background := th.Roles.Stone2
	mark := lipgloss.NewStyle().
		Foreground(th.Roles.Fg3).
		Background(background).
		Border(CutBorder()).
		BorderForeground(th.Roles.Vein).
		Padding(0, horizontalPadding(tokens.SpaceS4)).
		Render("No resources")
	parts := []string{
		centerPlaneRaw(width, background, mark),
		centerPlaneRaw(width, background, bold(th.Roles.Fg).Background(background).Render(empty.Title)),
		lipgloss.NewStyle().
			Foreground(th.Roles.Fg2).
			Background(background).
			Width(width).
			Align(lipgloss.Center).
			Render(empty.Body),
	}
	if len(empty.Actions) != 0 {
		actions := make([]string, len(empty.Actions))
		for i, action := range empty.Actions {
			actions[i] = RenderPlaque(th, action)
		}
		parts = append(parts, centerPlaneRaw(width, background, strings.Join(actions, "  ")))
	}
	return lipgloss.JoinVertical(lipgloss.Center, parts...)
}
