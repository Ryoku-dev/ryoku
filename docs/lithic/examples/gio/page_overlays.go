package main

import (
	"image"
	"math"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/symbols"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/theme"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/tokens"
)

type overlaysPage struct {
	popover              kit.Popover
	popoverAnchor        kit.Plaque
	menuPopover          kit.Popover
	menuAnchor           kit.Plaque
	menu                 kit.Menu
	picker               kit.Picker
	dialog               kit.Dialog
	dialogCancel         kit.Plaque
	dialogAction         kit.Plaque
	confirm              kit.Confirm
	tooltip              kit.Tooltip
	tooltipAnchor        kit.Plaque
	placard              kit.Placard
	noticePushed         bool
	rail                 kit.Rail
	newPlaque            kit.NewPlaque
	sectionIndex         kit.SectionIndex
	sectionPicker        kit.SectionIndex
	chart                kit.EngravedChart
	appFrame             kit.AppFrame
	appRail              kit.Rail
	appPlacard           kit.Placard
	appNotice            bool
	fleetIcon            *widget.Icon
	activityIcon         *widget.Icon
	settingsIcon         *widget.Icon
	menuItems            []kit.MenuItem
	destinations         []kit.RailDestination
	longRail             kit.Rail
	longRailNew          kit.Rail
	compactRail          kit.Rail
	longDestinations     []kit.RailDestination
	settingsRail         kit.Rail
	germanRail           kit.Rail
	settingsDestinations []kit.RailDestination
	germanDestinations   []kit.RailDestination
}

var overlaysGallery = newOverlaysPage()

func init() { registerPage("overlays", pageOverlays) }

func pageOverlays(gtx layout.Context, th *theme.Theme) layout.Dimensions {
	return overlaysGallery.Layout(gtx, th)
}

func newOverlaysPage() *overlaysPage {
	page := &overlaysPage{
		fleetIcon:    kit.Icon(symbols.Computer),
		activityIcon: kit.Icon(symbols.History),
		settingsIcon: kit.Icon(symbols.Settings),
	}
	page.popover.Open = true
	page.popover.Preview = kit.StateHover
	page.popoverAnchor = kit.Plaque{Kind: kit.PlaqueQuiet, Label: "Inspect ally"}
	page.menuPopover.Open = true
	page.menuPopover.Preview = kit.StateHover
	page.menuAnchor = kit.Plaque{Kind: kit.PlaqueQuiet, Label: "Machine actions"}
	page.menuItems = []kit.MenuItem{
		{Icon: page.fleetIcon, Label: "Open ubuntu-24.04", Shortcut: "Enter"},
		{Icon: page.activityIcon, Label: "View activity", Shortcut: "Ctrl A", Preview: kit.StateHover},
		{Divider: true},
		{Label: "Stop scratch-arch", Danger: true},
		{Label: "GPU passthrough unavailable", Disabled: true},
	}
	page.picker.Preview = kit.StateHover
	page.dialog.Open = true
	page.dialog.Preview = kit.StateHover
	page.dialog.Width = unit.Dp(tokens.SizeCatalogueDialogWidth)
	page.dialogCancel = kit.Plaque{Kind: kit.PlaqueQuiet, Label: "Cancel"}
	page.dialogAction = kit.Plaque{Kind: kit.PlaqueGold, Label: "Add alpine-instant"}
	page.confirm.Dialog.Open = true
	page.confirm.Preview = kit.StateHover
	page.confirm.Danger = true
	page.confirm.Dialog.Width = unit.Dp(tokens.SizeDialogWidth)
	page.confirmField("scratch-arch")
	page.tooltip.Preview = kit.StateHover
	page.tooltipAnchor = kit.Plaque{Kind: kit.PlaqueQuiet, Label: "Console router"}
	page.placard.Preview = kit.StateHover
	page.appFrame.Preview = kit.StateHover
	page.newPlaque.Preview = kit.StateHover
	page.destinations = []kit.RailDestination{
		{Label: "Fleet", Icon: page.fleetIcon},
		{Label: "Activity", Icon: page.activityIcon},
		{Label: "Settings", Icon: page.settingsIcon, Notch: true},
	}
	page.longDestinations = []kit.RailDestination{
		{Label: "Look", Icon: kit.Icon(symbols.DesktopWindows)},
		{Label: "Desktop", Icon: page.fleetIcon},
		{Label: "Keys", Icon: kit.Icon(symbols.Key)},
		{Label: "Privacy", Icon: kit.Icon(symbols.Lock)},
		{Label: "Files", Icon: kit.Icon(symbols.Folder)},
		{Label: "Updates", Icon: page.activityIcon, Notch: true},
		{Label: "About", Icon: page.settingsIcon},
	}
	settingsIcons := []*widget.Icon{
		kit.Icon(symbols.DesktopWindows), page.fleetIcon, kit.Icon(symbols.Fullscreen),
		kit.Icon(symbols.DeviceHub), kit.Icon(symbols.Link), kit.Icon(symbols.FormatListBulleted),
		kit.Icon(symbols.Group), page.settingsIcon, kit.Icon(symbols.Download),
	}
	english := []string{"Appearance", "Desktop", "Windows", "Devices", "Connections", "Apps", "Accounts", "System", "Updates"}
	german := []string{"Erscheinungsbild", "Schreibtisch", "Fenster", "Geräte", "Verbindungen", "Apps", "Konten", "System", "Aktualisierungen"}
	for index, icon := range settingsIcons {
		notch := index == len(settingsIcons)-1
		page.settingsDestinations = append(page.settingsDestinations, kit.RailDestination{Label: english[index], Icon: icon, Notch: notch})
		page.germanDestinations = append(page.germanDestinations, kit.RailDestination{Label: german[index], Icon: icon, Notch: notch})
	}
	page.chart = kit.EngravedChart{
		Values: []float64{18, 22, 37, 31, 49, 44, 62, 53, 71, 58, 66, 73},
		Min:    math.NaN(), Max: math.NaN(), Height: unit.Dp(tokens.SizePinnedTileMinHeight),
		Ticks: []string{"09:00", "09:15", "09:30", "09:45"},
	}
	return page
}

func (page *overlaysPage) confirmField(value string) {
	page.confirm.SetConfirmationText(value)
}

// settingsRailSpecimen draws a Settings rail: no New, Updates alone at the
// foot, and a host mark in place of Ryoport's octagon.
func (page *overlaysPage) settingsRailSpecimen(gtx layout.Context, th *theme.Theme, rail *kit.Rail, destinations []kit.RailDestination) layout.Dimensions {
	size := image.Pt(gtx.Dp(tokens.SizeRailExpanded), gtx.Dp(galleryHeight))
	child := gtx
	child.Constraints = layout.Exact(size)
	rail.HideNew = true
	if rail.Brand == nil {
		rail.Brand = hostMark(th)
	}
	rail.Layout(child, th, destinations, 4, kit.FrameExpanded)
	return layout.Dimensions{Size: size}
}

// hostMark is a sample app mark: two onSel stones and one jasper accent, each
// cut on one corner so the chamfer reads at this small size.
func hostMark(th *theme.Theme) kit.BrandMarkFunc {
	return func(gtx layout.Context, size unit.Dp) layout.Dimensions {
		side := gtx.Dp(size)
		part := func(fraction float32) int { return int(float32(side)*fraction + .5) }
		cut := gtx.Dp(th.Cut.Small)
		kit.FillCutCorners(gtx.Ops, image.Rect(0, part(.3), part(.45), side), cut, 0, 0, 0, th.R.OnSel)
		kit.FillCutCorners(gtx.Ops, image.Rect(part(.55), part(.55), side, side), 0, cut, 0, 0, th.R.OnSel)
		kit.FillCutCorners(gtx.Ops, image.Rect(part(.55), 0, side, part(.4)), 0, cut, 0, 0, th.R.Jasper)
		return layout.Dimensions{Size: image.Pt(side, side)}
	}
}

func (page *overlaysPage) Layout(gtx layout.Context, th *theme.Theme) layout.Dimensions {
	if !page.noticePushed {
		page.placard.Push(gtx.Now, "ubuntu-24.04 joined the fleet", "Undo")
		page.noticePushed = true
	}
	return layout.Inset{Top: th.Sp.S6, Right: th.Sp.S6, Bottom: th.Sp.S6, Left: th.Sp.S6}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return vstack(gtx, th.Sp.S6,
			func(gtx layout.Context) layout.Dimensions {
				return vstack(gtx, th.Sp.S2,
					kit.Text(th, th.T.Name, th.R.Fg, "Overlays, navigation and frame"),
					kit.Text(th, th.T.Body, th.R.Fg2, "Deferred choices, fleet navigation, engraved data and responsive window composition."),
				)
			},
			page.overlaySection(th),
			page.navigationSection(th),
			page.dataSection(th),
			page.frameSection(th),
		)
	})
}

func (page *overlaysPage) overlaySection(th *theme.Theme) layout.Widget {
	return section(th, "Overlays", func(gtx layout.Context) layout.Dimensions {
		frames := []layout.Widget{
			page.popoverSpecimen(th),
			page.menuSpecimen(th),
			page.pickerSpecimen(th),
			page.tooltipSpecimen(th),
			page.dialogSpecimen(th),
			page.confirmSpecimen(th),
			page.placardSpecimen(th),
		}
		return grid(gtx, 2, th.Sp.S3, frames)
	})
}

func (page *overlaysPage) popoverSpecimen(th *theme.Theme) layout.Widget {
	return overlaySpecimen(th, "Popover, open", unit.Dp(tokens.SizeCatalogueTileHeight*2), func(gtx layout.Context) layout.Dimensions {
		page.popover.Open = true
		page.popover.Opener = &page.popoverAnchor.Clickable
		return layout.Inset{Top: th.Sp.S4, Left: th.Sp.S4}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return page.popover.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return page.popoverAnchor.Layout(gtx, th)
			}, func(gtx layout.Context) layout.Dimensions {
				return popoverPanel(gtx, th, "ally", "ops@192.0.2.18")
			})
		})
	})
}

func (page *overlaysPage) menuSpecimen(th *theme.Theme) layout.Widget {
	return overlaySpecimen(th, "Menu, open", unit.Dp(tokens.SizeCatalogueTileHeight*2), func(gtx layout.Context) layout.Dimensions {
		page.menuPopover.Open = true
		page.menuPopover.Opener = &page.menuAnchor.Clickable
		return layout.Inset{Top: th.Sp.S4, Left: th.Sp.S4}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return page.menuPopover.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return page.menuAnchor.Layout(gtx, th)
			}, func(gtx layout.Context) layout.Dimensions {
				return page.menu.Layout(gtx, th, page.menuItems)
			})
		})
	})
}

func (page *overlaysPage) pickerSpecimen(th *theme.Theme) layout.Widget {
	return overlaySpecimen(th, "Picker, open", unit.Dp(tokens.SizeCatalogueTileHeight*2), func(gtx layout.Context) layout.Dimensions {
		page.picker.Show()
		return layout.Inset{Top: th.Sp.S4, Left: th.Sp.S4, Right: th.Sp.S4}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Max.X = min(gtx.Constraints.Max.X, gtx.Dp(tokens.SizeSearchWidth))
			return page.picker.Layout(gtx, th, "Display", []string{"Window", "Spice", "Headless"}, 1)
		})
	})
}

func (page *overlaysPage) tooltipSpecimen(th *theme.Theme) layout.Widget {
	return overlaySpecimen(th, "Tooltip, open after hover", unit.Dp(tokens.SizeCatalogueTileHeight*2), func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Top: th.Sp.S4, Left: th.Sp.S4}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return page.tooltip.Layout(gtx, th, "Connected through /dev/ttyUSB0", func(gtx layout.Context) layout.Dimensions {
				return page.tooltipAnchor.Layout(gtx, th)
			})
		})
	})
}

func (page *overlaysPage) dialogSpecimen(th *theme.Theme) layout.Widget {
	return overlaySpecimen(th, "Dialog with the catalogue open", unit.Dp(tokens.SizeWindowMinHeight), func(gtx layout.Context) layout.Dimensions {
		page.dialog.Open = true
		body := func(gtx layout.Context) layout.Dimensions {
			return vstack(gtx, th.Sp.S3,
				kit.Text(th, th.T.Body, th.R.Fg2, "Choose a fleet image for ally."),
				fleetChoice(th, "ubuntu-24.04", "Ready with 2 cores and 4 GB"),
				fleetChoice(th, "fedora-43", "Ready with 4 cores and 8 GB"),
				fleetChoice(th, "alpine-instant", "Instant with 1 core and 1 GB"),
			)
		}
		cancel := func(gtx layout.Context) layout.Dimensions { return page.dialogCancel.Layout(gtx, th) }
		action := func(gtx layout.Context) layout.Dimensions { return page.dialogAction.Layout(gtx, th) }
		return page.dialog.Layout(gtx, th, "Add a machine", body, cancel, action)
	})
}

func (page *overlaysPage) confirmSpecimen(th *theme.Theme) layout.Widget {
	return overlaySpecimen(th, "Confirm asking for the typed name", unit.Dp(tokens.SizeWindowMinHeight), func(gtx layout.Context) layout.Dimensions {
		page.confirm.Dialog.Open = true
		page.confirm.Layout(gtx, th, "Remove scratch-arch?", "This removes the machine record from ally.", "Remove scratch-arch", "scratch-arch")
		return layout.Dimensions{Size: gtx.Constraints.Max}
	})
}

func (page *overlaysPage) placardSpecimen(th *theme.Theme) layout.Widget {
	return overlaySpecimen(th, "Placard with Undo", unit.Dp(tokens.SizeCatalogueTileHeight*2), func(gtx layout.Context) layout.Dimensions {
		return page.placard.Layout(gtx, th)
	})
}

func (page *overlaysPage) navigationSection(th *theme.Theme) layout.Widget {
	return section(th, "Navigation", func(gtx layout.Context) layout.Dimensions {
		return vstack(gtx, th.Sp.S3,
			func(gtx layout.Context) layout.Dimensions {
				return grid(gtx, 3, th.Sp.S3, []layout.Widget{
					specimen(th, "BrandMark as home", func(gtx layout.Context) layout.Dimensions {
						return layout.Center.Layout(gtx, kit.BrandMark(th, unit.Dp(tokens.SizeBrandMark)))
					}),
					specimen(th, "NewPlaque hovered with its menu open", func(gtx layout.Context) layout.Dimensions {
						return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return page.newPlaque.Layout(gtx, th, false, true)
						})
					}),
					specimen(th, "Rail with Settings selected", func(gtx layout.Context) layout.Dimensions {
						size := image.Pt(gtx.Dp(tokens.SizeRailExpanded), gtx.Dp(tokens.SizeWindowMinHeight))
						child := gtx
						child.Constraints = layout.Exact(size)
						page.rail.Layout(child, th, page.destinations, 2, kit.FrameExpanded)
						return layout.Dimensions{Size: size}
					}),
				})
			},
			func(gtx layout.Context) layout.Dimensions {
				return grid(gtx, 3, th.Sp.S3, []layout.Widget{
					specimen(th, "Long rail without New, two destinations at the foot, Updates notched", func(gtx layout.Context) layout.Dimensions {
						size := image.Pt(gtx.Dp(tokens.SizeRailExpanded), gtx.Dp(tokens.SizeWindowDefaultHeight))
						child := gtx
						child.Constraints = layout.Exact(size)
						page.longRail.HideNew = true
						page.longRail.FootIndex = len(page.longDestinations) - 2
						page.longRail.Layout(child, th, page.longDestinations, 1, kit.FrameExpanded)
						return layout.Dimensions{Size: size}
					}),
					specimen(th, "Long rail, medium, with New", func(gtx layout.Context) layout.Dimensions {
						size := image.Pt(gtx.Dp(tokens.SizeRailMedium), gtx.Dp(tokens.SizeWindowDefaultHeight))
						child := gtx
						child.Constraints = layout.Exact(size)
						page.longRailNew.FootIndex = len(page.longDestinations) - 2
						page.longRailNew.Layout(child, th, page.longDestinations, 0, kit.FrameMedium)
						return layout.Dimensions{Size: size}
					}),
					specimen(th, "Compact bottom rail without New", func(gtx layout.Context) layout.Dimensions {
						size := image.Pt(gtx.Constraints.Max.X, gtx.Dp(tokens.SizeRailBottomHeight))
						child := gtx
						child.Constraints = layout.Exact(size)
						page.compactRail.HideNew = true
						page.compactRail.Layout(child, th, page.destinations, 0, kit.FrameCompact)
						return layout.Dimensions{Size: size}
					}),
				})
			},
			func(gtx layout.Context) layout.Dimensions {
				return grid(gtx, 3, th.Sp.S3, []layout.Widget{
					specimen(th, "Settings rail with a host mark, labels fit the tile", func(gtx layout.Context) layout.Dimensions {
						return page.settingsRailSpecimen(gtx, th, &page.settingsRail, page.settingsDestinations)
					}),
					specimen(th, "Settings rail in German, long labels wrap", func(gtx layout.Context) layout.Dimensions {
						return page.settingsRailSpecimen(gtx, th, &page.germanRail, page.germanDestinations)
					}),
				})
			},
			func(gtx layout.Context) layout.Dimensions {
				return grid(gtx, 2, th.Sp.S3, []layout.Widget{
					specimen(th, "SectionIndex, expanded", func(gtx layout.Context) layout.Dimensions {
						return page.sectionIndex.Layout(gtx, th, settingsSections(), 2, kit.FrameExpanded)
					}),
					specimen(th, "SectionIndex as the medium picker", func(gtx layout.Context) layout.Dimensions {
						return page.sectionPicker.Layout(gtx, th, settingsSections(), 2, kit.FrameMedium)
					}),
				})
			},
		)
	})
}

func (page *overlaysPage) dataSection(th *theme.Theme) layout.Widget {
	return section(th, "Data", specimen(th, "EngravedChart of ally host load", func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Max.X = min(gtx.Constraints.Max.X, gtx.Dp(tokens.SizeBoardMax))
		return page.chart.Layout(gtx, th)
	}))
}

func (page *overlaysPage) frameSection(th *theme.Theme) layout.Widget {
	return section(th, "Frame", specimen(th, "AppFrame, compact fleet", func(gtx layout.Context) layout.Dimensions {
		size := image.Pt(gtx.Dp(tokens.SizeWindowMinWidth), gtx.Dp(tokens.SizeWindowMinHeight))
		child := gtx
		child.Constraints = layout.Exact(size)
		if !page.appNotice {
			page.appPlacard.Push(gtx.Now, "prowl is ready at 198.51.100.24", "")
			page.appNotice = true
		}
		area := clip.Rect{Max: size}.Push(gtx.Ops)
		page.appFrame.Layout(child, th, kit.AppFrameContent{
			Rail: func(gtx layout.Context) layout.Dimensions {
				return page.appRail.Layout(gtx, th, page.destinations, 0, page.appFrame.Class)
			},
			Work:     func(gtx layout.Context) layout.Dimensions { return compactFleet(gtx, th) },
			Overlays: func(gtx layout.Context) layout.Dimensions { return page.appPlacard.Layout(gtx, th) },
		})
		area.Pop()
		return layout.Dimensions{Size: size}
	}))
}

func overlaySpecimen(th *theme.Theme, label string, height unit.Dp, body layout.Widget) layout.Widget {
	return specimen(th, label, func(gtx layout.Context) layout.Dimensions {
		size := image.Pt(gtx.Constraints.Max.X, gtx.Dp(height))
		child := gtx
		child.Constraints = layout.Exact(size)
		record := op.Record(gtx.Ops)
		area := clip.Rect{Max: size}.Push(gtx.Ops)
		body(child)
		area.Pop()
		call := record.Stop()
		bounds := image.Rectangle{Max: size}
		kit.FillCut(gtx.Ops, bounds, gtx.Dp(th.Cut.Mid), th.R.Stone2)
		kit.StrokeCut(gtx.Ops, bounds, gtx.Dp(th.Cut.Mid), max(1, gtx.Dp(tokens.SizeHairline)), th.R.Vein)
		call.Add(gtx.Ops)
		return layout.Dimensions{Size: size}
	})
}

func popoverPanel(gtx layout.Context, th *theme.Theme, name, proof string) layout.Dimensions {
	gtx.Constraints.Min.X = min(gtx.Constraints.Max.X, gtx.Dp(tokens.SizeTextFieldDefaultWidth))
	return kit.Slab(gtx, th.R.Stone4, unit.Dp(tokens.CutOverlay), func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Top: th.Sp.S3, Right: th.Sp.S3, Bottom: th.Sp.S3, Left: th.Sp.S3}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return vstack(gtx, th.Sp.S1,
				kit.Text(th, th.T.Row, th.R.Fg, name),
				kit.Text(th, th.T.MonoSm, th.R.Fg3, proof),
			)
		})
	})
}

func fleetChoice(th *theme.Theme, name, proof string) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		return kit.Slab(gtx, th.R.Stone3, th.Cut.Small, func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: th.Sp.S2, Right: th.Sp.S3, Bottom: th.Sp.S2, Left: th.Sp.S3}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return vstack(gtx, th.Sp.S1,
					kit.Text(th, th.T.Row, th.R.Fg, name),
					kit.Text(th, th.T.Proof, th.R.Fg3, proof),
				)
			})
		})
	}
}

func settingsSections() []string {
	return []string{"Appearance", "New machines", "VM engine", "GPU passthrough", "SSH keys", "Updates", "About"}
}

func compactFleet(gtx layout.Context, th *theme.Theme) layout.Dimensions {
	return layout.Inset{Top: th.Sp.S4, Right: th.Sp.S4, Bottom: th.Sp.S4, Left: th.Sp.S4}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return vstack(gtx, th.Sp.S3,
			kit.Text(th, th.T.Section, th.R.Fg, "Fleet"),
			fleetChoice(th, "ubuntu-24.04", "ally at ops@192.0.2.18"),
			fleetChoice(th, "fedora-43", "prowl at 198.51.100.24"),
			fleetChoice(th, "alpine-instant", "mika-server at root@203.0.113.70:2222"),
			fleetChoice(th, "kolibrios-latest-en_US", "Console router on /dev/ttyUSB0"),
		)
	})
}
