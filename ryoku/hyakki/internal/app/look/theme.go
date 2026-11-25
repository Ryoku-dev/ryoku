// Package look contains the Hyakki app's theme and window-coordinate UI helpers.
package look

import (
	"gioui.org/text"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"
	libtheme "github.com/ryoku-dev/ryoku/docs/lithic/library/go/theme"
)

type TextStyle = libtheme.TextStyle

// Theme is the shared Lithic theme in the look the Ryoku desktop asks for.
// Hyakki adds no colours and keeps no copy of the choice: the design, colours,
// light or dark, contrast, tint, reduced motion and gloss are the desktop's
// (docs/lithic/styles.md), and Desktop is that reading, for the pages that
// show what the desktop chose.
type Theme struct {
	*libtheme.Theme
	// Desktop is the reading the theme was built from (palette.ReadDesktop or
	// a palette.WatchDesktop callback).
	Desktop palette.Desktop
}

// NewFor builds the theme the desktop asks for (theme.NewFor), which also
// sets the process's reduced motion from d. A nil shaper loads Lithic's faces;
// pass the current theme's shaper to keep them.
func NewFor(d palette.Desktop, shaper *text.Shaper) *Theme {
	var options []libtheme.Option
	if shaper != nil {
		options = append(options, libtheme.WithShaper(shaper))
	}
	return &Theme{Theme: libtheme.NewFor(d, options...), Desktop: d}
}

// NewWithRoles builds a Lithic theme on caller-chosen roles, for tests and
// galleries that read no desktop: Lithic design, gloss on, reduced motion
// left as it is. palette.Signature(true) gives the dark ryoku.dev stock.
func NewWithRoles(roles palette.Roles) *Theme {
	mode := palette.Dark
	if !roles.Dark {
		mode = palette.Light
	}
	desktop := palette.Desktop{
		Mode: mode, Gloss: true, Roles: roles,
		Palette: palette.Palette{Dark: roles.Dark, Source: roles.Source},
	}
	return &Theme{Theme: libtheme.New(roles), Desktop: desktop}
}

// SetDesktop wears a new reading of the desktop in place: a theme built by
// theme.NewFor on the same shaper, so no face loads again, with the density
// kept. It sets reduced motion from d. Call it on the window's goroutine,
// before a frame reads the theme.
func (t *Theme) SetDesktop(d palette.Desktop) {
	density := t.Density
	base := libtheme.NewFor(d, libtheme.WithShaper(t.Shaper))
	base.Density = density
	t.Theme, t.Desktop = base, d
}
