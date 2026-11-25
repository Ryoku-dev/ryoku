package render

import (
	"image/color"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/design"

	"ryoku-rashin/internal/oni/spec"
)

// Look is the colours the character wears, taken from the desktop's palette
// roles (render/desktoplook); Accent is keyed by spec.PaletteIDs. Design is
// the desktop's design language, which shapes the perch chip and highlight.
type Look struct {
	Ink, White, Body color.NRGBA
	Accent           map[string]color.NRGBA
	Dark             bool
	Design           design.Style
}

// Role names one desktop palette role (Lithic's palette.Roles field).
type Role string

const (
	RoleGround Role = "Ground"
	RoleFg     Role = "Fg"
	RoleFg3    Role = "Fg3"
	RoleGold   Role = "Gold"
	RoleJade   Role = "Jade"
	RoleLapis  Role = "Lapis"
	RoleAmber  Role = "Amber"
	RoleJasper Role = "Jasper"
)

// RoleMap says which desktop role each part of the character wears. Oni's
// own artwork is drawn in Ink, White and Body; a state's accent is
// Accent[spec colour id].
type RoleMap struct {
	Ink, White, Body Role
	Accent           map[string]Role
}

// Roles is the mapping for a dark or light desktop. Ink and White trade
// places so the outline stays the darker one, and the body is the quiet ink
// between them, a role no state uses, so Oni at rest never reads as a state.
// The accents keep their meaning in both: the desktop accent for listening,
// attention for thinking, the cool stone for speaking, alive for done and
// failed for error. The four state accents are four different roles, so
// they stay apart under any palette that keeps its roles apart.
func Roles(dark bool) RoleMap {
	m := RoleMap{
		Ink: RoleGround, White: RoleFg, Body: RoleFg3,
		Accent: map[string]Role{
			"idle":      RoleFg,
			"listening": RoleGold,
			"thinking":  RoleAmber,
			"speaking":  RoleLapis,
			"done":      RoleJade,
			"error":     RoleJasper,
		},
	}
	if !dark {
		m.Ink, m.White = RoleFg, RoleGround
		m.Accent["idle"] = RoleGround
	}
	return m
}

// accent is palette id's accent colour, and whether it draws one at all: the
// idle colour is the character at rest and draws none.
func (l Look) accent(id string) (color.NRGBA, bool) {
	if id == "" || id == spec.PaletteIDs[0] {
		return color.NRGBA{}, false
	}
	c, ok := l.Accent[id]
	return c, ok && c.A > 0
}
