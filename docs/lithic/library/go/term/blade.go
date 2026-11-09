package term

import (
	"image/color"
	"strings"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"
)

// RenderBlade draws a mineral fill over a vein track with a leading edge.
func RenderBlade(th Theme, width int, fraction float64, tone palette.Tone) string {
	colour := th.Roles.Stone(tone)
	if tone == palette.Idle {
		colour = th.Roles.Fg2
	}
	return renderBladeColor(th, width, fraction, colour)
}

func renderBladeColor(th Theme, width int, fraction float64, colour color.Color) string {
	if width < 1 {
		return ""
	}
	fraction = max(0, min(1, fraction))
	filled := int(float64(width)*fraction + 0.5)
	g := glyphs()
	track := ink(th.Roles.Vein).Render(strings.Repeat(g.BladeTrack, width-filled))
	if filled == 0 {
		return track
	}
	fillCount := filled
	if g.BladeEdge != "" {
		fillCount--
	}
	fill := strings.Repeat(g.BladeFill, fillCount) + g.BladeEdge
	return ink(colour).Render(fill) + track
}
