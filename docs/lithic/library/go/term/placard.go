package term

import (
	"charm.land/lipgloss/v2"
)

// Placard describes a transient message and optional action.
type Placard struct {
	Text     string
	Action   string
	Progress float64
	Width    int
}

// RenderPlacard renders a selection-plane notice and draining blade.
func RenderPlacard(th Theme, placard Placard) string {
	content := plane(th.Roles.OnSel, th.Roles.Sel).Render(placard.Text)
	if placard.Action != "" {
		content += plane(th.Roles.OnSel, th.Roles.Sel).Render("  ") +
			RenderPlaque(th, Plaque{Label: placard.Action, Kind: PlaqueOn})
	}
	width := placard.Width
	if width <= 0 {
		width = lipgloss.Width(content)
	}
	content = renderPlaneRaw(width, th.Roles.Sel, content)
	body := framed(th, content, th.Roles.Sel, false, false)
	return body + "\n" + renderBladeColor(th, lipgloss.Width(body), placard.Progress, th.Roles.OnSel)
}
