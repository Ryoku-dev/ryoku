package settings

import (
	"image"

	"gioui.org/layout"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/design"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/desktop"
	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/app/state"
)

// what the desktop chose, one word each; a slot keeps its word's roll when tint comes and goes
const (
	lookDesign = iota
	lookColours
	lookMode
	lookContrast
	lookTint
	lookMotion
	lookGloss
	lookCount
)

// the look is the desktop's: Rashin says what it wears and hands off to Ryoku Settings, only the zoom is its own
type appearancePanel struct {
	words [lookCount]look.Swap
	hub   kit.Plaque
	zoom  kit.Stepper
}

type lookClaim struct {
	slot        int
	name, value string
}

// the reading the window wears now, in the order Ryoku Settings lists it
func lookClaims(th *look.Theme) []lookClaim {
	desk := th.Desktop
	style := i18n.T("Lithic")
	switch desk.Design {
	case design.Material:
		style = i18n.T("Material")
	case design.Adwaita:
		style = i18n.T("Adwaita")
	case design.Fluent:
		style = i18n.T("Fluent")
	}
	colours := i18n.T("Ryoku")
	if desk.Colours == palette.Wallpaper {
		colours = i18n.T("Wallpaper")
	}
	mode := i18n.T("Light")
	if th.R.Dark {
		mode = i18n.T("Dark")
	}
	contrast := i18n.T("Standard")
	if desk.Look.Contrast == palette.High {
		contrast = i18n.T("High")
	}
	onOff := func(on bool) string {
		if on {
			return i18n.T("On")
		}
		return i18n.T("Off")
	}
	claims := []lookClaim{
		{lookDesign, i18n.T("Design"), style},
		{lookColours, i18n.T("Colours"), colours},
		{lookMode, i18n.T("Light or dark"), mode},
		{lookContrast, i18n.T("Contrast"), contrast},
	}
	// tint dyes Lithic's paper with the wallpaper, so it means nothing anywhere else
	if desk.Design == design.Lithic && desk.Colours == palette.Wallpaper {
		tint := i18n.T("Off")
		switch desk.Look.Tint {
		case palette.TintLow:
			tint = i18n.T("Low")
		case palette.TintMedium:
			tint = i18n.T("Medium")
		}
		claims = append(claims, lookClaim{lookTint, i18n.T("Tint"), tint})
	}
	return append(claims,
		lookClaim{lookMotion, i18n.T("Reduce motion"), onOff(desk.ReduceMotion)},
		lookClaim{lookGloss, i18n.T("Japanese gloss"), onOff(desk.Gloss)},
	)
}

func (p *Page) layoutAppearance(gtx layout.Context, e *env.Env) layout.Dimensions {
	th := e.Theme
	a := &p.appearance
	claims := lookClaims(th)
	rows := make([]look.Setting, 0, len(claims)+1)
	for _, claim := range claims {
		rows = append(rows, look.Setting{Title: claim.name, Control: a.word(th, claim)})
	}
	a.hub.Kind, a.hub.Small, a.hub.Label, a.hub.Icon = kit.PlaqueQuiet, true, i18n.T("Open Ryoku Settings"), look.Icon.OpenInNew
	if a.hub.Clicked(gtx) && e.Desktop.Launch != nil {
		request := desktop.LaunchRequest{Name: "ryoku-shell", Args: []string{"hub", "open", "style"}, Front: image.Pt(1100, 820)}
		e.Desktop.Launch(request, func(err error) {
			if err != nil {
				e.Notify(i18n.T("Could not open Ryoku Settings"))
			}
		})
	}
	rows = append(rows, look.Setting{
		Title:   i18n.T("Ryoku Settings"),
		Detail:  i18n.T("The look is the desktop's: change it once in Style and every Ryoku app follows."),
		Control: func(gtx layout.Context) layout.Dimensions { return a.hub.Layout(gtx, th.Theme) },
	})
	a.zoom.Value, a.zoom.Min, a.zoom.Max, a.zoom.Step, a.zoom.Unit = e.Zoom, state.MinZoom, state.MaxZoom, state.ZoomStep, i18n.T("%")
	own := []look.Setting{{
		Title:   i18n.T("Zoom"),
		Detail:  i18n.T("Ctrl + and Ctrl - change it from anywhere. Ctrl 0 sets it back."),
		Changed: p.changed.zoom,
		Reset:   func() { e.SetZoom(state.DefaultZoom) },
		Control: func(gtx layout.Context) layout.Dimensions {
			dimensions := a.zoom.Layout(gtx, th.Theme)
			if a.zoom.Changed() {
				e.SetZoom(a.zoom.Value)
			}
			return dimensions
		},
	}}
	return look.Section(gtx, th, ordinal(sectionAppearance), func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(group(th, i18n.T("The desktop's look"), "", rows, nil)),
			layout.Rigid(layout.Spacer{Height: th.Sp.S5}.Layout),
			layout.Rigid(group(th, i18n.T("Rashin's own"), "", own, nil)),
		)
	})
}

// a claim's word in the row's control place: read only, and it rolls to the new word when the desktop changes
func (a *appearancePanel) word(th *look.Theme, claim lookClaim) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		return a.words[claim.slot].Layout(gtx, th, claim.value, th.Sp.S1, th.Motion.Move, func(gtx layout.Context, value string) layout.Dimensions {
			return kit.Text(th.Theme, th.T.Label, th.R.Fg, value)(gtx)
		})
	}
}
