package term

import (
	"charm.land/lipgloss/v2"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"
	"image/color"
)

// PlaqueKind selects the semantic plaque treatment.
type PlaqueKind uint8

const (
	PlaqueQuiet PlaqueKind = iota
	PlaqueGold
	PlaqueLine
	PlaqueOn
	PlaqueDanger
	PlaqueDangerLine
)

// Plaque describes a static terminal plaque state.
type Plaque struct {
	Label    string
	Kind     PlaqueKind
	Focused  bool
	Disabled bool
	Busy     bool
}

// RenderPlaque renders a plaque without owning interaction state.
func RenderPlaque(th Theme, p Plaque) string {
	label := p.Label
	if p.Busy {
		frames := glyphs().WorkingFrames
		if len(frames) != 0 {
			label = frames[min(1, len(frames)-1)] + " " + label
		}
	}
	focused := p.Focused && !p.Disabled
	fg := th.Roles.Fg2
	var bg color.Color
	filled := false
	bracketed := true
	boldLabel := focused
	switch p.Kind {
	case PlaqueGold:
		fg, bg, filled, bracketed, boldLabel = th.Roles.OnGold, th.Roles.Gold, true, false, true
	case PlaqueLine:
		fg = th.Roles.GoldInk
	case PlaqueOn:
		fg, bg, filled, bracketed = th.Roles.OnSel, th.Roles.Sel, true, false
	case PlaqueDanger:
		fg, bg, filled, bracketed, boldLabel = palette.InkOn(th.Roles.Jasper), th.Roles.Jasper, true, false, true
	case PlaqueDangerLine:
		fg = th.Roles.Jasper
	default:
		bg, filled = th.Roles.Stone3, true
	}
	if p.Disabled {
		fg, bg, filled, bracketed, boldLabel = th.Roles.Fg3, nil, false, true, false
	}
	labelStyle := lipgloss.NewStyle().Foreground(fg).Bold(boldLabel)
	if filled {
		labelStyle = labelStyle.Background(bg)
		gutter := focusGutterOn(th, focused, bg)
		return gutter + labelStyle.Render(" "+label+" ")
	}
	gutter := focusGutter(th, focused)
	if bracketed {
		return labelStyle.Render("[") + gutter + labelStyle.Render(label+" ]")
	}
	return gutter + labelStyle.Render(label)
}
