// Package look contains the Rashin app's Lithic theme and window-coordinate UI helpers.
package look

import (
	"context"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"
	libtheme "github.com/ryoku-dev/ryoku/docs/lithic/library/go/theme"
)

type TextStyle = libtheme.TextStyle

// Theme embeds the shared Lithic theme. Rashin adds no private colours.
type Theme struct {
	*libtheme.Theme
	Mode palette.Mode
	Look palette.Look
}

func New(mode palette.Mode, look palette.Look) *Theme {
	roles := palette.FromPalette(palette.Load(mode), look)
	return &Theme{Theme: libtheme.New(roles), Mode: mode, Look: look}
}

// Watch follows the desktop's palette, which only the Follow mode reads, and
// returns replacements to apply on the window goroutine.
func Watch(ctx context.Context, look func() palette.Look, changed func(*Theme)) {
	palette.Watch(ctx, func() palette.Mode { return palette.Follow }, func(resolved palette.Palette) {
		current := look()
		roles := palette.FromPalette(resolved, current)
		changed(&Theme{Theme: libtheme.New(roles), Mode: palette.Follow, Look: current})
	})
}
