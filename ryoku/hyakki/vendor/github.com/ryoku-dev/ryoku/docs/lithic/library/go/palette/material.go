package palette

import (
	"image/color"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/tokens"
)

// Material is Lithic.qml's _materialFrom (Lithic.qml:671-700): the roles a style that
// wears the desktop's palette (Material, Adwaita, Fluent) takes from p's Material roles as
// they are. The surfaces step through the surface containers, the selection is the
// primary, the gold the tertiary, the container the secondary container. It starts from
// the toned ladder, so a role p lacks keeps the ladder's value, and scrim, the minerals
// and the mineral fields stay toned (the only roles contrast and tint still reach).
// p.Dark decides the ladder; ReadDesktop sets it from the surface's luma.
func Material(p Palette, look Look) Roles {
	dyed := p.Source != "signature"
	base := p
	if !dyed {
		// _materialFrom passes no edition accent
		base = Palette{Dark: p.Dark, Source: p.Source}
	}
	r := rolesFrom(base, look, dyed)
	take := func(role *color.NRGBA, key string) {
		if value, ok := p.Roles[key]; ok {
			*role = value
		}
	}
	take(&r.Ground, "surface")
	take(&r.Stone1, "surfaceContainerLow")
	take(&r.Stone2, "surface")
	take(&r.Stone3, "surfaceContainer")
	take(&r.Stone4, "surfaceContainerHigh")
	take(&r.Slab, "surface")
	take(&r.Field, "surfaceContainerHighest")
	take(&r.Vein, "outlineVariant")
	take(&r.Edge, "outline")
	take(&r.Fg, "onSurface")
	take(&r.Fg2, "onSurfaceVariant")
	variant, hasVariant := p.Roles["onSurfaceVariant"]
	surface, hasSurface := p.Roles["surface"]
	if hasVariant && hasSurface {
		r.Fg3 = Mix(variant, surface, 0.22)
	}
	take(&r.Sel, "primary")
	take(&r.OnSel, "onPrimary")
	r.OnSel2 = Mix(r.OnSel, r.Sel, tokens.AlphaOnSel2)
	take(&r.Gold, "tertiary")
	take(&r.GoldInk, "tertiary")
	take(&r.OnGold, "onTertiary")
	if value, ok := p.Roles["error"]; ok && value != r.Jasper {
		r.fieldJasper = r.Jasper
		r.Jasper = value
	}
	take(&r.Container, "secondaryContainer")
	take(&r.OnContainer, "onSecondaryContainer")
	return r
}
