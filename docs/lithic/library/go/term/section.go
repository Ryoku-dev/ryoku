package term

import "charm.land/lipgloss/v2"

// Section describes a titled region.
type Section struct {
	Title string
	Body  string
}

// RenderSection renders the bold section title above its body.
func RenderSection(th Theme, section Section) string {
	return lipgloss.JoinVertical(lipgloss.Left, bold(th.Roles.Fg).Render(section.Title), section.Body)
}
