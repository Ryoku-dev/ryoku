package term

import (
	"strconv"

	"charm.land/lipgloss/v2"
)

// GroupHeader describes a populated list group.
type GroupHeader struct {
	Title string
	Count int
	Width int
}

// RenderGroupHeader renders the group title and count in quiet ink.
func RenderGroupHeader(th Theme, header GroupHeader) string {
	count := strconv.Itoa(header.Count)
	width := max(header.Width, lipgloss.Width(header.Title)+lipgloss.Width(count)+1)
	return ink(th.Roles.Fg3).Render(header.Title + spaces(width-lipgloss.Width(header.Title)-lipgloss.Width(count)) + count)
}
