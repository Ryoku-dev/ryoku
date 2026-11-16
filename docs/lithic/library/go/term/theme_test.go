package term

import (
	"image/color"
	"testing"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/design"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"
)

func TestNewThemeForHonoursTheDesktop(t *testing.T) {
	for _, test := range []struct {
		name string
		d    palette.Desktop
		want palette.Roles
	}{
		{
			"light signature, high contrast",
			palette.Desktop{Design: design.Material, Mode: palette.Light, Look: palette.Look{Contrast: palette.High},
				Palette: palette.Palette{Source: "signature"}},
			palette.FromPalette(palette.Palette{Source: "signature"}, palette.Look{Contrast: palette.High}),
		},
		{
			"wallpaper in a round design, tinted",
			palette.Desktop{Design: design.Fluent, Mode: palette.Follow, Colours: palette.Wallpaper, Look: palette.Look{Tint: palette.TintMedium},
				Palette: palette.Palette{Roles: map[string]color.NRGBA{"surface": {R: 0x10, G: 0x14, B: 0x18, A: 0xff}, "primary": {R: 0x9d, G: 0xcb, B: 0xfc, A: 0xff}}, Dark: true, Source: "wallpaper"}},
			palette.FromPalette(palette.Palette{Roles: map[string]color.NRGBA{"surface": {R: 0x10, G: 0x14, B: 0x18, A: 0xff}, "primary": {R: 0x9d, G: 0xcb, B: 0xfc, A: 0xff}}, Dark: true, Source: "wallpaper"}, palette.Look{Tint: palette.TintMedium}),
		},
	} {
		if got := NewThemeFor(test.d).Roles; got != test.want {
			t.Errorf("%s: the terminal does not take the Lithic roles", test.name)
		}
	}
}
