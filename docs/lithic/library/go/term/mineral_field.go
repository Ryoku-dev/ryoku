package term

import (
	"charm.land/lipgloss/v2"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"
)

// MineralField describes content on a tone-derived plane.
type MineralField struct {
	Tone     palette.Tone
	Content  string
	Focused  bool
	Disabled bool
}

// RenderMineralField renders content on the tone's unframed field colour.
func RenderMineralField(th Theme, field MineralField) string {
	foreground := th.Roles.Fg
	if field.Disabled {
		foreground = th.Roles.Fg3
	}
	return renderPlaneText(max(1, lipgloss.Width(field.Content)), th.Roles.MineralField(field.Tone), foreground, field.Content, false)
}
