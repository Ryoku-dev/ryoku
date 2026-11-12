package term

import "charm.land/lipgloss/v2"

// Aside describes the quiet live summary beside a draft.
type Aside struct {
	Title   string
	Facts   Facts
	Preview string
	Width   int
}

// RenderAside renders title, facts, and literal preview on nested planes.
func RenderAside(th Theme, aside Aside) string {
	parts := []string{bold(th.Roles.Fg).Render(aside.Title), RenderFacts(th, aside.Facts)}
	if aside.Preview != "" {
		parts = append(parts, framed(th, aside.Preview, th.Roles.Field, false, false))
	}
	content := lipgloss.JoinVertical(lipgloss.Left, parts...)
	if aside.Width > 0 {
		content = lipgloss.NewStyle().Width(aside.Width).Render(content)
	}
	return framed(th, content, th.Roles.Slab, false, false)
}
