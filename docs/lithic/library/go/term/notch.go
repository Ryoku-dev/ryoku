package term

import "github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"

// RenderNotch draws the terminal state notch in the tone colour.
func RenderNotch(th Theme, tone palette.Tone) string {
	return ink(th.Roles.Stone(tone)).Render(glyphs().Notch)
}
