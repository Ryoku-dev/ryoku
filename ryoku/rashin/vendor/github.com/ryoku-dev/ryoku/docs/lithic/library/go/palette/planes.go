package palette

import (
	"image/color"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/design"
)

// Planes are the surfaces a window lays out in a design: Side (a rail or sidebar), Pane
// (the page) and Card (a group of rows), each with a stroke that is transparent (the zero
// colour) where the design draws none. The host lays its planes out from them; the
// library's own controls keep their fills.
type Planes struct {
	Side       color.NRGBA
	Pane       color.NRGBA
	PaneStroke color.NRGBA
	Card       color.NRGBA
	CardStroke color.NRGBA
}

// Planes is Lithic.planes (Lithic.qml:77-88) over r. Lithic lays one paper and draws each
// plane by its hairline; Material lifts the page a step off the ground and its cards a step
// more, with no lines; Adwaita sets the sidebar apart and lays cards on the page; Fluent
// layers its planes, each with a thin outline.
func (r Roles) Planes(s design.Style) Planes {
	switch s {
	case design.Material:
		return Planes{Side: r.Ground, Pane: r.Stone1, Card: r.Stone3}
	case design.Adwaita:
		return Planes{Side: r.Stone3, Pane: r.Ground, Card: r.Stone4}
	case design.Fluent:
		return Planes{Side: r.Ground, Pane: r.Stone3, PaneStroke: r.Vein, Card: r.Stone4, CardStroke: WithAlpha(r.Vein, 0.7)}
	default:
		// a plain slab is drawn by its hairline alone
		return Planes{Side: r.Ground, Pane: r.Stone2, PaneStroke: r.Vein, Card: r.Slab, CardStroke: r.Vein}
	}
}
