package term

import (
	"strings"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/tokens"
)

// BladeProgress describes determinate or frozen indeterminate job progress.
type BladeProgress struct {
	Width         int
	Fraction      float64
	Indeterminate bool
	Frame         int
	Active        bool
}

// RenderBladeProgress renders lapis job progress with a gold active edge.
func RenderBladeProgress(th Theme, progress BladeProgress) string {
	width := max(1, progress.Width)
	g := glyphs()
	if progress.Indeterminate {
		segment := max(1, int(float64(width)*tokens.SizeBladeIndeterminateFraction+0.5))
		start := progress.Frame % max(1, width-segment+1)
		return ink(th.Roles.Vein).Render(strings.Repeat(g.BladeTrack, start)) + ink(th.Roles.Lapis).Render(strings.Repeat(g.BladeFill, segment)) + ink(th.Roles.Vein).Render(strings.Repeat(g.BladeTrack, width-start-segment))
	}
	if !progress.Active || progress.Fraction <= 0 || progress.Fraction >= 1 {
		return renderBladeColor(th, width, progress.Fraction, th.Roles.Lapis)
	}
	filled := max(1, int(float64(width)*max(0, min(1, progress.Fraction))+0.5))
	return ink(th.Roles.Lapis).Render(strings.Repeat(g.BladeFill, max(0, filled-1))) +
		ink(th.Roles.Gold).Render(g.FrameLeft) +
		ink(th.Roles.Vein).Render(strings.Repeat(g.BladeTrack, width-filled))
}
