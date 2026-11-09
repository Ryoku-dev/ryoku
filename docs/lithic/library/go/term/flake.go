package term

import "github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"

// RenderFlake draws idle, filled, or partially becoming state.
func RenderFlake(th Theme, tone palette.Tone, fraction float64) string {
	g := glyphs()
	if tone == palette.Idle || fraction <= 0 {
		return ink(th.Roles.Fg3).Render(g.FlakeIdle)
	}
	if tone == palette.Becoming && fraction < 1 {
		return ink(th.Roles.Lapis).Render(g.FlakeBecoming)
	}
	return ink(th.Roles.Stone(tone)).Render(g.FlakeActive)
}
