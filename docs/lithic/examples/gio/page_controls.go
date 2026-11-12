package main

import (
	"image"

	"gioui.org/layout"
	"gioui.org/unit"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/theme"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/tokens"
)

type controlsPage struct {
	plaques      [14]kit.Plaque
	splits       [5]kit.SplitPlaque
	icons        [12]kit.IconPlaque
	fields       [5]kit.TextField
	searches     [3]kit.Search
	paneField    kit.TextField
	paneSearch   kit.Search
	slabField    kit.TextField
	slabContrast kit.Segmented
	slabToggle   kit.Toggle
	toggles      [6]kit.Toggle
	sliders      [5]kit.Slider
	steppers     [3]kit.Stepper
	segmented    [5]kit.Segmented
	filters      [3]kit.Filters
	tabs         [3]kit.Tabs
	rows         [3]kit.Tablet
	menuPopover  kit.Popover
	menuAnchor   kit.Plaque
	menu         kit.Menu
	menuItems    []kit.MenuItem
	disclosures  [2]kit.Disclosure
	disclosureOp [2]kit.Plaque
}

func init() {
	page := newControlsPage()
	registerPage("controls", page.Layout)
}

func newControlsPage() *controlsPage {
	page := &controlsPage{}
	kinds := [...]kit.PlaqueKind{kit.PlaqueGold, kit.PlaqueLine, kit.PlaqueTonal, kit.PlaqueQuiet, kit.PlaqueOn, kit.PlaqueDanger, kit.PlaqueDangerLine}
	labels := [...]string{"Connect ally", "Inspect prowl", "Open console", "Cancel", "Running", "Stop mika-server", "Remove scratch-arch"}
	for i := range kinds {
		page.plaques[i].Kind = kinds[i]
		page.plaques[i].Label = labels[i]
	}
	states := [...]kit.State{kit.StateRest, kit.StateHover, kit.StatePressed, kit.StateFocus}
	for i := range states {
		page.plaques[7+i].Kind = kit.PlaqueTonal
		page.plaques[7+i].Label = "Open fedora-43"
		page.plaques[7+i].Preview = states[i]
	}
	page.plaques[11] = kit.Plaque{Kind: kit.PlaqueLine, Label: "Connecting ally", Busy: true}
	page.plaques[12] = kit.Plaque{Kind: kit.PlaqueTonal, Label: "Unavailable"}
	page.plaques[13] = kit.Plaque{Kind: kit.PlaqueQuiet, Label: "Open kolibrios-latest-en_US", Small: true}

	for i := range states {
		page.splits[i].Kind = kit.PlaqueLine
		page.splits[i].Icon = kit.IconByName("settings")
		page.splits[i].Label = "Start ubuntu-24.04"
		page.splits[i].Preview = states[i]
	}
	page.splits[3].Open = true
	page.splits[4] = kit.SplitPlaque{Kind: kit.PlaqueTonal, Label: "Open console"}
	page.splits[4].Icon = kit.IconByName("settings")

	for i := range kinds {
		page.icons[i].Kind = kinds[i]
		page.icons[i].Icon = kit.IconByName("settings")
		page.icons[i].Description = labels[i]
	}
	for i := len(kinds); i < len(page.icons); i++ {
		page.icons[i].Kind = kit.PlaqueQuiet
		page.icons[i].Icon = kit.IconByName("settings")
		page.icons[i].Description = "Server settings"
	}
	page.icons[7].Small = true
	page.icons[9].Preview = kit.StateHover
	page.icons[10].Preview = kit.StatePressed
	page.icons[11].Preview = kit.StateFocus

	page.fields[0].Label = "Server"
	page.fields[0].Placeholder = "prowl@192.0.2.24"
	page.fields[0].Help = "Use a documentation address"
	page.fields[1].Label = "Host"
	page.fields[1].Placeholder = "root@203.0.113.70:2222"
	page.fields[1].Preview = kit.StateFocus
	page.fields[2].Label = "Console"
	page.fields[2].Placeholder = "/dev/ttyUSB0"
	page.fields[2].Error = "Console is not connected"
	page.fields[2].Mono = true
	page.fields[3].Label = "Credential"
	page.fields[3].Secret = true
	page.fields[3].SetText("fleet-access")
	page.fields[4].Label = "Server"
	page.fields[4].Placeholder = "ops@198.51.100.14"

	page.searches[0].Placeholder = "Search machines"
	page.searches[0].Hint = "Ctrl K"
	page.searches[1].Placeholder = "Search servers"
	page.searches[1].Preview = kit.StateFocus
	page.searches[2].Placeholder = "Search fleet"
	page.searches[2].SetText("alpine-instant")

	page.toggles[0] = kit.Toggle{Label: "Wake on connect"}
	page.toggles[1] = kit.Toggle{Label: "Auto-start prowl", Value: true}
	page.toggles[2] = kit.Toggle{Label: "Share console", Preview: kit.StateHover}
	page.toggles[3] = kit.Toggle{Label: "Share console", Preview: kit.StatePressed}
	page.toggles[4] = kit.Toggle{Label: "Share console", Preview: kit.StateFocus}
	page.toggles[5] = kit.Toggle{Label: "Unavailable"}

	for i := range page.sliders {
		page.sliders[i].Min = 0
		page.sliders[i].Max = 100
		page.sliders[i].Value = 62
		page.sliders[i].Step = 5
		page.sliders[i].Label = "Console scale"
	}
	page.sliders[1].Preview = kit.StateHover
	page.sliders[2].Preview = kit.StatePressed
	page.sliders[3].Preview = kit.StateFocus

	page.steppers[0] = kit.Stepper{Value: 4, Min: 1, Max: 8, Unit: "cores"}
	page.steppers[1] = kit.Stepper{Value: 1, Min: 1, Max: 8, Unit: "core"}
	page.steppers[2] = kit.Stepper{Value: 8, Min: 1, Max: 8, Unit: "cores", Preview: kit.StateFocus}

	filterLabels := []string{"All", "Running", "Stopped"}
	filterCounts := []int{6, 3, 3}
	for i := range page.filters {
		page.filters[i].Labels = filterLabels
		page.filters[i].Counts = filterCounts
		page.filters[i].Selected = i
	}
	page.filters[1].Preview = kit.StateHover
	page.filters[2].Preview = kit.StateFocus

	tabLabels := []string{"Machines", "Servers", "Console"}
	for i := range page.tabs {
		page.tabs[i].Labels = tabLabels
		page.tabs[i].Selected = i
	}
	page.tabs[1].Preview = kit.StateHover
	page.tabs[2].Preview = kit.StateFocus

	page.disclosures[1].Open = true
	page.disclosureOp[0] = kit.Plaque{Kind: kit.PlaqueQuiet, Label: "Show connection facts"}
	page.disclosureOp[1] = kit.Plaque{Kind: kit.PlaqueQuiet, Label: "Hide connection facts"}

	page.paneField = kit.TextField{Label: "Server", Placeholder: "prowl@192.0.2.24"}
	page.paneSearch.Placeholder = "Search machines"
	page.slabField = kit.TextField{Label: "Console", Placeholder: "/dev/ttyUSB0", Mono: true}
	page.slabContrast.Labels = []string{"Standard", "High"}
	page.slabToggle.Value = true

	segmentLabels := [...][]string{{"Standard", "High"}, {"Off", "Low", "Medium"}, {"All", "Running", "Stopped"}, {"All", "Running", "Stopped"}, {"Off", "Low", "Medium"}}
	for i := range page.segmented {
		page.segmented[i].Labels = segmentLabels[i]
	}
	page.segmented[1].Selected = 1
	page.segmented[2].Preview = kit.StateHover
	page.segmented[3].Preview = kit.StateFocus
	page.segmented[4].Selected = 2

	page.rows[2].Preview = kit.StateHover

	page.menuPopover.Open = true
	page.menuPopover.Preview = kit.StateHover
	page.menuAnchor = kit.Plaque{Kind: kit.PlaqueTonal, Label: "Fleet view"}
	page.menuItems = []kit.MenuItem{
		{Label: "Show stopped machines", Checked: true},
		{Label: "Pinned first"},
		{Label: "Group by server", Checked: true},
	}
	return page
}

func (page *controlsPage) Layout(gtx layout.Context, th *theme.Theme) layout.Dimensions {
	return layout.Inset{Top: th.Sp.S6, Right: th.Sp.S6, Bottom: th.Sp.S6, Left: th.Sp.S6}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return vstack(gtx, th.Sp.S6,
			func(gtx layout.Context) layout.Dimensions {
				return vstack(gtx, th.Sp.S2,
					kit.Text(th, th.T.Name, th.R.Fg, "Controls"),
					kit.Text(th, th.T.Body, th.R.Fg2, "Actions and fields for the Ryoport fleet, shown with every treatment and interaction state."),
				)
			},
			page.plaqueSection(th),
			page.splitSection(th),
			page.iconSection(th),
			page.textFieldSection(th),
			page.searchSection(th),
			page.surfaceSection(th),
			page.toggleSection(th),
			page.sliderSection(th),
			page.stepperSection(th),
			page.segmentedSection(th),
			page.filtersSection(th),
			page.tabsSection(th),
			page.rowsSection(th),
			page.menuSection(th),
			page.disclosureSection(th),
			settingsSpecimens.settingRowSection(th),
			settingsSpecimens.chordFieldSection(th),
			settingsSpecimens.colorFieldSection(th),
			listPathSpecimens.listEditorSection(th),
			listPathSpecimens.pathFieldSection(th),
		)
	})
}

func (page *controlsPage) plaqueSection(th *theme.Theme) layout.Widget {
	labels := [...]string{"Gold", "Line", "Tonal", "Quiet", "On", "Danger", "Danger line", "Tonal rest", "Tonal hover", "Tonal pressed", "Tonal focus", "Busy", "Disabled", "Small quiet"}
	widgets := make([]layout.Widget, len(page.plaques))
	for i := range page.plaques {
		i := i
		widgets[i] = specimen(th, labels[i], func(gtx layout.Context) layout.Dimensions {
			if i == 12 {
				gtx = gtx.Disabled()
			}
			return page.plaques[i].Layout(gtx, th)
		})
	}
	return section(th, "Plaque", func(gtx layout.Context) layout.Dimensions { return grid(gtx, 3, th.Sp.S3, widgets) })
}

func (page *controlsPage) splitSection(th *theme.Theme) layout.Widget {
	labels := [...]string{"Rest", "Hover", "Pressed", "Focus and open", "Tonal"}
	widgets := make([]layout.Widget, len(page.splits))
	for i := range page.splits {
		i := i
		widgets[i] = specimen(th, labels[i], func(gtx layout.Context) layout.Dimensions {
			if page.splits[i].MoreClicked(gtx) {
				page.splits[i].Open = !page.splits[i].Open
			}
			return page.splits[i].Layout(gtx, th)
		})
	}
	return section(th, "SplitPlaque", func(gtx layout.Context) layout.Dimensions { return grid(gtx, 2, th.Sp.S3, widgets) })
}

func (page *controlsPage) iconSection(th *theme.Theme) layout.Widget {
	labels := [...]string{"Gold", "Line", "Tonal", "Quiet", "On", "Danger", "Danger line", "Small", "Amber notch", "Hover", "Pressed", "Focus"}
	widgets := make([]layout.Widget, len(page.icons))
	for i := range page.icons {
		i := i
		widgets[i] = specimen(th, labels[i], func(gtx layout.Context) layout.Dimensions {
			if i == 8 {
				page.icons[i].Notch = th.R.Amber
			}
			return page.icons[i].Layout(gtx, th)
		})
	}
	return section(th, "IconPlaque", func(gtx layout.Context) layout.Dimensions { return grid(gtx, 3, th.Sp.S3, widgets) })
}

func (page *controlsPage) textFieldSection(th *theme.Theme) layout.Widget {
	labels := [...]string{"Rest with help", "Focus", "Error", "Secret with reveal", "Disabled"}
	widgets := make([]layout.Widget, len(page.fields))
	for i := range page.fields {
		i := i
		widgets[i] = specimen(th, labels[i], func(gtx layout.Context) layout.Dimensions {
			if i == 4 {
				gtx = gtx.Disabled()
			}
			return page.fields[i].Layout(gtx, th)
		})
	}
	return section(th, "TextField", func(gtx layout.Context) layout.Dimensions { return grid(gtx, 2, th.Sp.S3, widgets) })
}

func (page *controlsPage) searchSection(th *theme.Theme) layout.Widget {
	labels := [...]string{"Rest with shortcut", "Focus", "Text with clear"}
	widgets := make([]layout.Widget, len(page.searches))
	for i := range page.searches {
		i := i
		widgets[i] = specimen(th, labels[i], func(gtx layout.Context) layout.Dimensions { return page.searches[i].Layout(gtx, th) })
	}
	return section(th, "Search", func(gtx layout.Context) layout.Dimensions { return grid(gtx, 3, th.Sp.S3, widgets) })
}

func (page *controlsPage) toggleSection(th *theme.Theme) layout.Widget {
	labels := [...]string{"Off", "On", "Hover", "Pressed", "Focus", "Disabled"}
	widgets := make([]layout.Widget, len(page.toggles))
	for i := range page.toggles {
		i := i
		widgets[i] = specimen(th, labels[i], func(gtx layout.Context) layout.Dimensions {
			if i == 5 {
				gtx = gtx.Disabled()
			}
			return page.toggles[i].Layout(gtx, th)
		})
	}
	return section(th, "Toggle", func(gtx layout.Context) layout.Dimensions { return grid(gtx, 3, th.Sp.S3, widgets) })
}

func (page *controlsPage) sliderSection(th *theme.Theme) layout.Widget {
	labels := [...]string{"Rest", "Hover", "Pressed", "Focus", "Disabled"}
	widgets := make([]layout.Widget, len(page.sliders))
	for i := range page.sliders {
		i := i
		widgets[i] = specimen(th, labels[i], func(gtx layout.Context) layout.Dimensions {
			if i == 4 {
				gtx = gtx.Disabled()
			}
			return page.sliders[i].Layout(gtx, th)
		})
	}
	return section(th, "Slider", func(gtx layout.Context) layout.Dimensions { return grid(gtx, 3, th.Sp.S3, widgets) })
}

func (page *controlsPage) stepperSection(th *theme.Theme) layout.Widget {
	labels := [...]string{"Middle", "Minimum bound", "Maximum bound and focus"}
	widgets := make([]layout.Widget, len(page.steppers))
	for i := range page.steppers {
		i := i
		widgets[i] = specimen(th, labels[i], func(gtx layout.Context) layout.Dimensions { return page.steppers[i].Layout(gtx, th) })
	}
	return section(th, "Stepper", func(gtx layout.Context) layout.Dimensions { return grid(gtx, 3, th.Sp.S3, widgets) })
}

func (page *controlsPage) filtersSection(th *theme.Theme) layout.Widget {
	labels := [...]string{"Selected", "Hover", "Focus"}
	widgets := make([]layout.Widget, len(page.filters))
	for i := range page.filters {
		i := i
		widgets[i] = specimen(th, labels[i], func(gtx layout.Context) layout.Dimensions { return page.filters[i].Layout(gtx, th) })
	}
	return section(th, "Filters", func(gtx layout.Context) layout.Dimensions { return grid(gtx, 2, th.Sp.S3, widgets) })
}

func (page *controlsPage) tabsSection(th *theme.Theme) layout.Widget {
	labels := [...]string{"Selected", "Hover", "Focus and arrow keys"}
	widgets := make([]layout.Widget, len(page.tabs))
	for i := range page.tabs {
		i := i
		widgets[i] = specimen(th, labels[i], func(gtx layout.Context) layout.Dimensions { return page.tabs[i].Layout(gtx, th) })
	}
	return section(th, "Tabs", func(gtx layout.Context) layout.Dimensions { return grid(gtx, 2, th.Sp.S3, widgets) })
}

func (page *controlsPage) disclosureSection(th *theme.Theme) layout.Widget {
	labels := [...]string{"Closed", "Open"}
	widgets := make([]layout.Widget, len(page.disclosures))
	for i := range page.disclosures {
		i := i
		widgets[i] = specimen(th, labels[i], func(gtx layout.Context) layout.Dimensions {
			if page.disclosureOp[i].Clicked(gtx) {
				page.disclosures[i].Open = !page.disclosures[i].Open
				if page.disclosures[i].Open {
					page.disclosureOp[i].Label = "Hide connection facts"
				} else {
					page.disclosureOp[i].Label = "Show connection facts"
				}
			}
			return vstack(gtx, th.Sp.S2,
				func(gtx layout.Context) layout.Dimensions { return page.disclosureOp[i].Layout(gtx, th) },
				func(gtx layout.Context) layout.Dimensions {
					return page.disclosures[i].Layout(gtx, th, func(gtx layout.Context) layout.Dimensions {
						gtx.Constraints.Min = image.Point{}
						return kit.Text(th, th.T.Body, th.R.Fg2, "prowl at 192.0.2.24 is alive")(gtx)
					})
				},
			)
		})
	}
	return section(th, "Disclosure", func(gtx layout.Context) layout.Dimensions { return grid(gtx, 2, th.Sp.S3, widgets) })
}

func (page *controlsPage) surfaceSection(th *theme.Theme) layout.Widget {
	return section(th, "Fields on a pane and in a slab", func(gtx layout.Context) layout.Dimensions {
		return grid(gtx, 2, th.Sp.S3, []layout.Widget{
			func(gtx layout.Context) layout.Dimensions {
				return (kit.Pane{Kind: kit.PaneWork}).Layout(gtx, th, func(gtx layout.Context) layout.Dimensions {
					return layout.UniformInset(th.Sp.S4).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return vstack(gtx, th.Sp.S3,
							func(gtx layout.Context) layout.Dimensions { return page.paneField.Layout(gtx, th) },
							func(gtx layout.Context) layout.Dimensions { return page.paneSearch.Layout(gtx, th) },
						)
					})
				})
			},
			func(gtx layout.Context) layout.Dimensions {
				return kit.SlabRows(gtx, th,
					func(gtx layout.Context) layout.Dimensions { return page.slabField.Layout(gtx, th) },
					settingRow(th, "Contrast", "Raises every stone step against the ground", func(gtx layout.Context) layout.Dimensions {
						return page.slabContrast.Layout(gtx, th)
					}),
					settingRow(th, "Wake on connect", "Starts ally before its console opens", func(gtx layout.Context) layout.Dimensions {
						return page.slabToggle.Layout(gtx, th)
					}),
				)
			},
		})
	})
}

func (page *controlsPage) segmentedSection(th *theme.Theme) layout.Widget {
	labels := [...]string{"Two options", "Three options", "Hover", "Focus and arrow keys", "Disabled"}
	widgets := make([]layout.Widget, len(page.segmented))
	for i := range page.segmented {
		i := i
		widgets[i] = specimen(th, labels[i], func(gtx layout.Context) layout.Dimensions {
			if i == 4 {
				gtx = gtx.Disabled()
			}
			return page.segmented[i].Layout(gtx, th)
		})
	}
	return section(th, "Segmented", func(gtx layout.Context) layout.Dimensions { return grid(gtx, 3, th.Sp.S3, widgets) })
}

func (page *controlsPage) rowsSection(th *theme.Theme) layout.Widget {
	rows := [...]struct {
		label, name, claim string
		tone               palette.Tone
		selected           bool
	}{
		{"Rest", "ubuntu-24.04", "Stopped", palette.Idle, false},
		{"Selected", "fedora-43", "Running", palette.Alive, true},
		{"Hover", "alpine-instant", "Running", palette.Alive, false},
	}
	widgets := make([]layout.Widget, len(rows))
	for i, row := range rows {
		i, row := i, row
		widgets[i] = specimen(th, row.label, func(gtx layout.Context) layout.Dimensions {
			page.rows[i].Pane = th.R.Ground
			return page.rows[i].Layout(gtx, th, kit.TabletContent{
				Title: row.name, Facts: "ops@192.0.2.24", FactsMono: true,
				Claim: row.claim, Proof: "ally", Tone: row.tone, Progress: -1,
			}, row.selected, false)
		})
	}
	return section(th, "Selected row", func(gtx layout.Context) layout.Dimensions { return grid(gtx, 3, th.Sp.S3, widgets) })
}

func (page *controlsPage) menuSection(th *theme.Theme) layout.Widget {
	return section(th, "Menu", overlaySpecimen(th, "Checked items", unit.Dp(tokens.SizeCatalogueTileHeight*2), func(gtx layout.Context) layout.Dimensions {
		page.menuPopover.Open = true
		page.menuPopover.Opener = &page.menuAnchor.Clickable
		return layout.Inset{Top: th.Sp.S4, Left: th.Sp.S4}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return page.menuPopover.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return page.menuAnchor.Layout(gtx, th)
			}, func(gtx layout.Context) layout.Dimensions {
				return page.menu.Layout(gtx, th, page.menuItems)
			})
		})
	}))
}

func settingRow(th *theme.Theme, title, detail string, control layout.Widget) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return vstack(gtx, th.Sp.S1,
					kit.Text(th, th.T.Row, th.R.Fg, title),
					kit.Text(th, th.T.Detail, th.R.Fg2, detail),
				)
			}),
			layout.Rigid(layout.Spacer{Width: th.Sp.S4}.Layout),
			layout.Rigid(control),
		)
	}
}
