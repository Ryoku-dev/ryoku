// Package look contains the Rashin app's Lithic theme and window-coordinate UI helpers.
package look

import (
	"context"
	"fmt"

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

func ParseMode(value string) (palette.Mode, error) {
	switch value {
	case "", "follow":
		return palette.Follow, nil
	case "dark":
		return palette.Dark, nil
	case "light":
		return palette.Light, nil
	default:
		return palette.Follow, fmt.Errorf("unknown palette mode %q", value)
	}
}

// Watch follows the resolved Ryoku palette and returns replacements to apply on
// the window goroutine.
func Watch(ctx context.Context, mode func() palette.Mode, look func() palette.Look, changed func(*Theme)) {
	palette.Watch(ctx, mode, func(resolved palette.Palette) {
		current := look()
		roles := palette.FromPalette(resolved, current)
		changed(&Theme{Theme: libtheme.New(roles), Mode: mode(), Look: current})
	})
}
