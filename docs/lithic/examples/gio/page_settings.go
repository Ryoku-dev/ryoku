package main

import (
	"fmt"
	"image"
	"image/color"

	"gioui.org/layout"
	"gioui.org/op/paint"
	"gioui.org/unit"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/theme"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/tokens"
)

// settingsGallery holds the specimens of the settings components: SettingRow,
// ChordField and ColorField on the controls page, DesignTile, DesignGallery
// and ScrollRail on the lists page.
type settingsGallery struct {
	rows        [8]kit.SettingRow
	rowToggles  [8]kit.Toggle
	stack       [3]kit.SettingRow
	stackToggle [3]kit.Toggle
	stackPicker kit.Segmented
	chords      [8]kit.ChordField
	colors      [5]kit.ColorField
	colorsReady bool
	tiles       [10]kit.DesignTile
	gallery     kit.DesignGallery
	galleryTile []kit.DesignTile
	selected    int
	rails       [4]kit.ScrollRail
	railLists   [4]layout.List
	picture     paint.ImageOp
	pictureFor  palette.Roles
}

var settingsSpecimens = &settingsGallery{}

func (page *settingsGallery) settingRowSection(th *theme.Theme) layout.Widget {
	states := [...]struct {
		label, title, detail, unavailable string
		preview                           kit.State
		changed, activatable              bool
		highlight                         float32
	}{
		{"Rest", "Dim inactive windows", "Darkens windows that do not have focus", "", kit.StateRest, false, false, 0},
		{"Changed, pointer over the row", "Window gaps", "Space between tiled windows", "", kit.StateHover, true, false, 0},
		{"Activatable, hover", "Night light", "Warms the screen after sunset", "", kit.StateHover, false, true, 0},
		{"Activatable, pressed", "Night light", "Warms the screen after sunset", "", kit.StatePressed, false, true, 0},
		{"Activatable, keyboard focus", "Night light", "Warms the screen after sunset", "", kit.StateFocus, true, true, 0},
		{"Unavailable", "Variable refresh rate", "Matches the refresh rate to the game", "This display does not support it", kit.StateRest, false, false, 0},
		{"Highlighted by search", "Blur behind windows", "Softens the desktop behind open windows", "", kit.StateRest, false, false, 1},
		{"Long title and detail wrap", "Keep the bar visible above fullscreen windows on every display", "When a window goes fullscreen, the bar stays on top on this display and every other connected display, so the clock and the tray remain in reach", "", kit.StateRest, true, false, 0},
	}
	widgets := make([]layout.Widget, len(states))
	for index := range states {
		index := index
		state := states[index]
		widgets[index] = specimen(th, state.label, func(gtx layout.Context) layout.Dimensions {
			row := &page.rows[index]
			toggle := &page.rowToggles[index]
			row.Title, row.Detail, row.Unavailable = state.title, state.detail, state.unavailable
			row.Changed, row.Activatable, row.First = state.changed, state.activatable, true
			row.Highlight, row.Preview = state.highlight, state.preview
			if row.Row.Clicked(gtx) {
				toggle.Value = !toggle.Value
			}
			for row.Reset.Clicked(gtx) {
				toggle.Value = false
			}
			return kit.SlabContent(gtx, th, func(gtx layout.Context) layout.Dimensions {
				return row.Layout(gtx, th, func(gtx layout.Context) layout.Dimensions { return toggle.Layout(gtx, th) })
			})
		})
	}
	stack := specimen(th, "Rows in a slab, vein line between them", func(gtx layout.Context) layout.Dimensions {
		titles := [...]string{"Dim inactive windows", "Window gaps", "Focus follows the pointer"}
		details := [...]string{"Darkens windows that do not have focus", "Space between tiled windows", "Focuses the window under the pointer without a click"}
		return kit.SlabContent(gtx, th, func(gtx layout.Context) layout.Dimensions {
			children := make([]layout.Widget, len(page.stack))
			for index := range page.stack {
				index := index
				children[index] = func(gtx layout.Context) layout.Dimensions {
					row := &page.stack[index]
					row.Title, row.Detail, row.First = titles[index], details[index], index == 0
					if index == 1 {
						page.stackPicker.Labels = []string{"None", "Small", "Large"}
						page.stackPicker.Selected = 1
						return row.Layout(gtx, th, func(gtx layout.Context) layout.Dimensions { return page.stackPicker.Layout(gtx, th) })
					}
					return row.Layout(gtx, th, func(gtx layout.Context) layout.Dimensions { return page.stackToggle[index].Layout(gtx, th) })
				}
			}
			return vstack(gtx, 0, children...)
		})
	})
	return section(th, "SettingRow", func(gtx layout.Context) layout.Dimensions {
		return vstack(gtx, th.Sp.S3,
			func(gtx layout.Context) layout.Dimensions { return grid(gtx, 2, th.Sp.S3, widgets) },
			stack,
		)
	})
}

func (page *settingsGallery) chordFieldSection(th *theme.Theme) layout.Widget {
	states := [...]struct {
		label, chord, conflict string
		preview                kit.State
		recording, disabled    bool
	}{
		{"Rest", "Super+Return", "", kit.StateRest, false, false},
		{"Hover", "Super+Shift+Q", "", kit.StateHover, false, false},
		{"Keyboard focus", "Super+Ctrl+Prior", "", kit.StateFocus, false, false},
		{"Recording", "Super+Return", "", kit.StateRest, true, false},
		{"Not set", "", "", kit.StateRest, false, false},
		{"Conflict", "Super+Shift+C", "Already used by Close window", kit.StateRest, false, false},
		{"Number pad key", "Super+Alt+KP_3", "", kit.StateRest, false, false},
		{"Disabled", "Super+grave", "", kit.StateRest, false, true},
	}
	widgets := make([]layout.Widget, len(states))
	for index := range states {
		index := index
		state := states[index]
		widgets[index] = specimen(th, state.label, func(gtx layout.Context) layout.Dimensions {
			field := &page.chords[index]
			field.Chord, field.Conflict, field.Preview = state.chord, state.conflict, state.preview
			field.PreviewRecording = state.recording
			field.Label = "Open a terminal shortcut"
			if chord, ok := field.Recorded(); ok {
				field.Chord = chord
			}
			if field.Cleared() {
				field.Chord = ""
			}
			gtx.Constraints.Max.X = min(gtx.Constraints.Max.X, gtx.Dp(tokens.SizeSettingsControlMax))
			if state.disabled {
				gtx = gtx.Disabled()
			}
			return field.Layout(gtx, th)
		})
	}
	return section(th, "ChordField", func(gtx layout.Context) layout.Dimensions { return grid(gtx, 4, th.Sp.S3, widgets) })
}

func (page *settingsGallery) prepareColors(th *theme.Theme) {
	swatches := []kit.ColorSwatch{
		{Label: "Accent", Color: th.R.Gold},
		{Label: "Selection", Color: th.R.Sel},
		{Label: "Jade", Color: th.R.Jade},
		{Label: "Lapis", Color: th.R.Lapis},
		{Label: "Amber", Color: th.R.Amber},
		{Label: "Jasper", Color: th.R.Jasper},
	}
	for index := range page.colors {
		page.colors[index].Swatches = swatches
		page.colors[index].Label = "Bar accent"
	}
	if page.colorsReady {
		return
	}
	page.colorsReady = true
	page.colors[0].Color = th.R.Gold
	page.colors[1].Color = palette.WithAlpha(th.R.Lapis, .5)
	page.colors[1].AllowAlpha = true
	page.colors[2].Color = th.R.Gold
	page.colors[2].SubmitHex("#ce5e3")
	page.colors[3].Color = th.R.Jade
	page.colors[4].Color = th.R.Lapis
	page.colors[4].AllowAlpha = true
	page.colors[4].Preview = kit.StateFocus
	page.colors[4].Show()
}

func (page *settingsGallery) colorFieldSection(th *theme.Theme) layout.Widget {
	page.prepareColors(th)
	labels := [...]string{"Rest", "With alpha", "Invalid hex", "Disabled"}
	widgets := make([]layout.Widget, len(labels))
	for index := range labels {
		index := index
		widgets[index] = specimen(th, labels[index], func(gtx layout.Context) layout.Dimensions {
			field := &page.colors[index]
			field.Picked()
			gtx.Constraints.Max.X = min(gtx.Constraints.Max.X, gtx.Dp(tokens.SizeSettingsControlMax))
			if index == 3 {
				gtx = gtx.Disabled()
			}
			return field.Layout(gtx, th)
		})
	}
	open := overlaySpecimen(th, "Popover open, with role swatches and opacity", unit.Dp(tokens.SizeCatalogueTileHeight*4), func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Top: th.Sp.S4, Left: th.Sp.S4}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Max.X = gtx.Dp(tokens.SizeSettingsControlMax)
			page.colors[4].Picked()
			return page.colors[4].Layout(gtx, th)
		})
	})
	return section(th, "ColorField", func(gtx layout.Context) layout.Dimensions {
		return vstack(gtx, th.Sp.S3,
			func(gtx layout.Context) layout.Dimensions { return grid(gtx, 2, th.Sp.S3, widgets) },
			open,
		)
	})
}

// designPicture paints a small desktop sketch from the roles so the tile has a
// picture to crop.
func (page *settingsGallery) designPicture(th *theme.Theme) paint.ImageOp {
	if page.picture.Size().X > 0 && page.pictureFor == th.R {
		return page.picture
	}
	const width, height = 320, 180
	picture := image.NewNRGBA(image.Rect(0, 0, width, height))
	fill := func(bounds image.Rectangle, ink color.NRGBA) {
		for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
			for x := bounds.Min.X; x < bounds.Max.X; x++ {
				picture.SetNRGBA(x, y, ink)
			}
		}
	}
	fill(picture.Bounds(), th.R.Ground)
	fill(image.Rect(0, 0, width, 18), th.R.Stone1)
	fill(image.Rect(8, 5, 40, 13), th.R.Sel)
	fill(image.Rect(280, 5, 312, 13), th.R.Gold)
	fill(image.Rect(24, 34, 180, 160), th.R.Stone2)
	fill(image.Rect(196, 34, 296, 96), th.R.Stone3)
	fill(image.Rect(196, 108, 296, 160), th.R.Stone4)
	fill(image.Rect(40, 52, 140, 60), th.R.Fg2)
	fill(image.Rect(40, 68, 110, 74), th.R.Fg3)
	page.picture, page.pictureFor = paint.NewImageOp(picture), th.R
	return page.picture
}

func (page *settingsGallery) designTileSection(th *theme.Theme) layout.Widget {
	picture := page.designPicture(th)
	states := [...]struct {
		label, title, status, unavailable, action string
		resource                                  kit.Mark
		preview                                   kit.State
		current, selected, busy, gold, image      bool
	}{
		{"Rest, no preview yet", "Orpheus", "Music reactive", "", "", kit.Mark{Text: "Standard"}, kit.StateRest, false, false, false, false, false},
		{"Current, on action", "Sumi", "In use", "", "Edit", kit.Mark{Text: "Light"}, kit.StateRest, true, false, false, false, true},
		{"Selected, on action", "Iris", "Floating islands", "", "Preview", kit.Mark{Text: "Standard"}, kit.StateRest, false, true, false, false, true},
		{"Current and selected", "Sumi", "In use", "", "", kit.Mark{Text: "Light"}, kit.StateRest, true, true, false, false, true},
		{"Hover, line action", "Shima", "Stripes across the top", "", "Preview", kit.Mark{Text: "Light"}, kit.StateHover, false, false, false, false, true},
		{"Pressed", "Shima", "Stripes across the top", "", "", kit.Mark{Text: "Light"}, kit.StatePressed, false, false, false, false, true},
		{"Keyboard focus", "Python", "Bar with system graphs", "", "", kit.Mark{Text: "Heavy", Attention: true}, kit.StateFocus, false, false, false, false, true},
		{"Unavailable", "Serpantinum", "", "Needs niri", "", kit.Mark{Text: "Heavy", Attention: true}, kit.StateRest, false, false, false, false, true},
		{"Busy", "Orpheus", "Installing", "", "", kit.Mark{Text: "Standard"}, kit.StateRest, false, false, true, false, true},
		{"Gold action", "Iris", "Not installed", "", "Install", kit.Mark{Text: "Standard"}, kit.StateRest, false, false, false, true, true},
	}
	widgets := make([]layout.Widget, len(states))
	for index := range states {
		index := index
		state := states[index]
		widgets[index] = specimen(th, state.label, func(gtx layout.Context) layout.Dimensions {
			tile := &page.tiles[index]
			tile.Title, tile.Status, tile.Unavailable, tile.Resource = state.title, state.status, state.unavailable, state.resource
			tile.Current, tile.Selected, tile.Busy, tile.Preview = state.current, state.selected, state.busy, state.preview
			tile.ActionText, tile.ActionGold = state.action, state.gold
			tile.Image = paint.ImageOp{}
			if state.image {
				tile.Image = picture
			}
			tile.Clicked(gtx)
			tile.ActionTriggered(gtx)
			gtx.Constraints.Max.X = min(gtx.Constraints.Max.X, gtx.Dp(tokens.SizeCatalogueTileWidthMax))
			return tile.Layout(gtx, th)
		})
	}
	return section(th, "DesignTile", func(gtx layout.Context) layout.Dimensions { return grid(gtx, 5, th.Sp.S3, widgets) })
}

func (page *settingsGallery) designGallerySection(th *theme.Theme) layout.Widget {
	picture := page.designPicture(th)
	names := [...]string{"Orpheus", "Sumi", "Iris", "Shima", "Python", "Serpantinum", "Kumo", "Hoshi"}
	weights := [...]string{"Standard", "Light", "Standard", "Light", "Heavy", "Heavy", "Light", "Standard"}
	if len(page.galleryTile) != len(names) {
		page.galleryTile = make([]kit.DesignTile, len(names))
		page.selected = 2
	}
	return section(th, "DesignGallery", specimen(th, "Keyboard focus on the selected look; columns fit the width", func(gtx layout.Context) layout.Dimensions {
		for index := range page.galleryTile {
			tile := &page.galleryTile[index]
			tile.Title, tile.Image = names[index], picture
			tile.Resource = kit.Mark{Text: weights[index], Attention: weights[index] == "Heavy"}
			tile.Current = index == 0
			tile.Status = "Bar style"
			if tile.Current {
				tile.Status = "In use"
			}
		}
		page.gallery.Preview = kit.StateFocus
		dims := page.gallery.Layout(gtx, th, page.galleryTile, page.selected)
		if selected, ok := page.gallery.Changed(); ok {
			page.selected = selected
		}
		return dims
	}))
}

func (page *settingsGallery) scrollRailSection(th *theme.Theme) layout.Widget {
	states := [...]struct {
		label   string
		preview kit.State
		count   int
		first   int
	}{
		{"Rest", kit.StateRest, 24, 0},
		{"Hover", kit.StateHover, 24, 9},
		{"Dragging", kit.StatePressed, 24, 18},
		{"No overflow, no thumb", kit.StateRest, 3, 0},
	}
	widgets := make([]layout.Widget, len(states))
	for index := range states {
		index := index
		state := states[index]
		widgets[index] = specimen(th, state.label, func(gtx layout.Context) layout.Dimensions {
			list := &page.railLists[index]
			list.Axis = layout.Vertical
			if list.Position.First == 0 && list.Position.Offset == 0 && state.first > 0 {
				list.Position.First = state.first
			}
			rail := &page.rails[index]
			rail.Preview = state.preview
			size := image.Pt(gtx.Constraints.Max.X, gtx.Dp(tokens.SizeCatalogueTileHeight*2))
			return (kit.Pane{Kind: kit.PaneWork}).Layout(exactContextFor(gtx, size), th, func(gtx layout.Context) layout.Dimensions {
				pane := gtx
				pane.Constraints = layout.Exact(size)
				list.Layout(pane, state.count, func(gtx layout.Context, row int) layout.Dimensions {
					return layout.Inset{Left: th.Sp.S4, Right: th.Sp.S4, Top: th.Sp.S1, Bottom: th.Sp.S1}.Layout(gtx,
						kit.Text(th, th.T.Body, th.R.Fg, fmt.Sprintf("Workspace %d", row+1)))
				})
				return rail.Layout(pane, th, list, state.count)
			})
		})
	}
	return section(th, "ScrollRail", func(gtx layout.Context) layout.Dimensions { return grid(gtx, 4, th.Sp.S3, widgets) })
}

func exactContextFor(gtx layout.Context, size image.Point) layout.Context {
	gtx.Constraints = layout.Exact(size)
	return gtx
}
