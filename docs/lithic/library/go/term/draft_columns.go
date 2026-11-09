package term

import (
	"charm.land/lipgloss/v2"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/tokens"
)

// DraftColumns describes a form and optional live summary.
type DraftColumns struct {
	Form  string
	Aside *Aside
	Width int
}

// RenderDraftColumns stacks compact content and splits wide content.
func RenderDraftColumns(th Theme, columns DraftColumns) string {
	if columns.Aside == nil {
		return columns.Form
	}
	aside := RenderAside(th, *columns.Aside)
	wideAt := tokens.SizeDraftWideAt / tokens.SpaceS2
	if columns.Width < wideAt {
		return lipgloss.JoinVertical(lipgloss.Left, columns.Form, "", aside)
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, columns.Form, spaces(horizontalPadding(tokens.SpaceS5)), aside)
}
