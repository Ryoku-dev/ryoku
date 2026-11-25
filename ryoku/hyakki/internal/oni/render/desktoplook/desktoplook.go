// Package desktoplook builds the character's render.Look from the desktop's
// Lithic palette roles through render.Roles, so Oni follows the desktop's
// colours, light or dark, and its design language, and holds none of its
// own.
package desktoplook

import (
	"image/color"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"

	"ryoku-rashin/internal/oni/render"
)

// FromDesktop is the look for a reading of the desktop (palette.ReadDesktop
// or a palette.WatchDesktop callback): its roles and its design.
func FromDesktop(d palette.Desktop) render.Look {
	l := FromRoles(d.Roles)
	l.Design = d.Design
	return l
}

// FromRoles is the look for a set of roles; palette.Signature gives Ryoku's.
func FromRoles(r palette.Roles) render.Look {
	m := render.Roles(r.Dark)
	l := render.Look{
		Ink:    role(r, m.Ink),
		White:  role(r, m.White),
		Body:   role(r, m.Body),
		Accent: make(map[string]color.NRGBA, len(m.Accent)),
		Dark:   r.Dark,
	}
	for id, name := range m.Accent {
		l.Accent[id] = role(r, name)
	}
	return l
}

func role(r palette.Roles, name render.Role) color.NRGBA {
	switch name {
	case render.RoleGround:
		return r.Ground
	case render.RoleFg:
		return r.Fg
	case render.RoleFg3:
		return r.Fg3
	case render.RoleGold:
		return r.Gold
	case render.RoleJade:
		return r.Jade
	case render.RoleLapis:
		return r.Lapis
	case render.RoleAmber:
		return r.Amber
	case render.RoleJasper:
		return r.Jasper
	}
	panic("desktoplook: render.Roles names a role this package does not map: " + string(name))
}
