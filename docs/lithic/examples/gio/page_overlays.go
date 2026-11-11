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
	popover       kit.Popover
	popoverAnchor kit.Plaque
	menuPopover   kit.Popover
	menuAnchor    kit.Plaque
	menu          kit.Menu
	picker        kit.Picker
	dialog        kit.Dialog
	dialogCancel  kit.Plaque
	dialogAction  kit.Plaque
	confirm       kit.Confirm
	tooltip       kit.Tooltip
	tooltipAnchor kit.Plaque
	placard       kit.Placard
	noticePushed  bool
	rail          kit.Rail
	newPlaque     kit.NewPlaque
	sectionIndex  kit.SectionIndex
	sectionPicker kit.SectionIndex
	chart         kit.EngravedChart
	appFrame      kit.AppFrame
	appRail       kit.Rail
	appPlacard    kit.Placard
	appNotice     bool
	fleetIcon     *widget.Icon
	activityIcon  *widget.Icon
	settingsIcon  *widget.Icon
	menuItems     []kit.MenuItem
	destinations  []kit.RailDestination
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
	return overlaySpecimen(th, "Popover · Open", unit.Dp(tokens.SizeCatalogueTileHeight*2), func(gtx layout.Context) layout.Dimensions {
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
	return overlaySpecimen(th, "Menu · Open", unit.Dp(tokens.SizeCatalogueTileHeight*2), func(gtx layout.Context) layout.Dimensions {
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
	return overlaySpecimen(th, "Picker · Open", unit.Dp(tokens.SizeCatalogueTileHeight*2), func(gtx layout.Context) layout.Dimensions {
		page.picker.Show()
		return layout.Inset{Top: th.Sp.S4, Left: th.Sp.S4, Right: th.Sp.S4}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Max.X = min(gtx.Constraints.Max.X, gtx.Dp(tokens.SizeSearchWidth))
			return page.picker.Layout(gtx, th, "Display", []string{"Window", "Spice", "Headless"}, 1)
		})
	})
}

func (page *overlaysPage) tooltipSpecimen(th *theme.Theme) layout.Widget {
	return overlaySpecimen(th, "Tooltip · Open after hover", unit.Dp(tokens.SizeCatalogueTileHeight*2), func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Top: th.Sp.S4, Left: th.Sp.S4}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return page.tooltip.Layout(gtx, th, "Connected through /dev/ttyUSB0", func(gtx layout.Context) layout.Dimensions {
				return page.tooltipAnchor.Layout(gtx, th)
			})
		})
	})
}

func (page *overlaysPage) dialogSpecimen(th *theme.Theme) layout.Widget {
	return overlaySpecimen(th, "Dialog · Catalogue open", unit.Dp(tokens.SizeWindowMinHeight), func(gtx layout.Context) layout.Dimensions {
		page.dialog.Open = true
		body := func(gtx layout.Context) layout.Dimensions {
			return vstack(gtx, th.Sp.S3,
				kit.Text(th, th.T.Body, th.R.Fg2, "Choose a fleet image for ally."),
				fleetChoice(th, "ubuntu-24.04", "Ready · 2 cores · 4 GB"),
				fleetChoice(th, "fedora-43", "Ready · 4 cores · 8 GB"),
				fleetChoice(th, "alpine-instant", "Instant · 1 core · 1 GB"),
			)
		}
		cancel := func(gtx layout.Context) layout.Dimensions { return page.dialogCancel.Layout(gtx, th) }
		action := func(gtx layout.Context) layout.Dimensions { return page.dialogAction.Layout(gtx, th) }
		return page.dialog.Layout(gtx, th, "Add a machine", body, cancel, action)
	})
}

func (page *overlaysPage) confirmSpecimen(th *theme.Theme) layout.Widget {
	return overlaySpecimen(th, "Confirm · Typed name open", unit.Dp(tokens.SizeWindowMinHeight), func(gtx layout.Context) layout.Dimensions {
		page.confirm.Dialog.Open = true
		page.confirm.Layout(gtx, th, "Remove scratch-arch?", "This removes the machine record from ally.", "Remove scratch-arch", "scratch-arch")
		return layout.Dimensions{Size: gtx.Constraints.Max}
	})
}

func (page *overlaysPage) placardSpecimen(th *theme.Theme) layout.Widget {
	return overlaySpecimen(th, "Placard · Open with Undo", unit.Dp(tokens.SizeCatalogueTileHeight*2), func(gtx layout.Context) layout.Dimensions {
		return page.placard.Layout(gtx, th)
	})
}

func (page *overlaysPage) navigationSection(th *theme.Theme) layout.Widget {
	return section(th, "Navigation", func(gtx layout.Context) layout.Dimensions {
		return vstack(gtx, th.Sp.S3,
			func(gtx layout.Context) layout.Dimensions {
				return grid(gtx, 3, th.Sp.S3, []layout.Widget{
					specimen(th, "BrandMark · Home", func(gtx layout.Context) layout.Dimensions {
						return layout.Center.Layout(gtx, kit.BrandMark(th, unit.Dp(tokens.SizeBrandMark)))
					}),
					specimen(th, "NewPlaque · Hover, menu open", func(gtx layout.Context) layout.Dimensions {
						return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return page.newPlaque.Layout(gtx, th, false, true)
						})
					}),
					specimen(th, "Rail · Settings selected", func(gtx layout.Context) layout.Dimensions {
						size := image.Pt(gtx.Dp(tokens.SizeRailExpanded), gtx.Dp(tokens.SizeWindowMinHeight))
						child := gtx
						child.Constraints = layout.Exact(size)
						page.rail.Layout(child, th, page.destinations, 2, kit.FrameExpanded)
						return layout.Dimensions{Size: size}
					}),
				})
			},
			func(gtx layout.Context) layout.Dimensions {
				return grid(gtx, 2, th.Sp.S3, []layout.Widget{
					specimen(th, "SectionIndex · Expanded", func(gtx layout.Context) layout.Dimensions {
						return page.sectionIndex.Layout(gtx, th, settingsSections(), 2, kit.FrameExpanded)
					}),
					specimen(th, "SectionIndex · Medium picker", func(gtx layout.Context) layout.Dimensions {
						return page.sectionPicker.Layout(gtx, th, settingsSections(), 2, kit.FrameMedium)
					}),
				})
			},
		)
	})
}

func (page *overlaysPage) dataSection(th *theme.Theme) layout.Widget {
	return section(th, "Data", specimen(th, "EngravedChart · ally host load", func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Max.X = min(gtx.Constraints.Max.X, gtx.Dp(tokens.SizeBoardMax))
		return page.chart.Layout(gtx, th)
	}))
}

func (page *overlaysPage) frameSection(th *theme.Theme) layout.Widget {
	return section(th, "Frame", specimen(th, "AppFrame · Compact fleet", func(gtx layout.Context) layout.Dimensions {
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
			fleetChoice(th, "ubuntu-24.04", "ally · ops@192.0.2.18"),
			fleetChoice(th, "fedora-43", "prowl · 198.51.100.24"),
			fleetChoice(th, "alpine-instant", "mika-server · root@203.0.113.70:2222"),
			fleetChoice(th, "kolibrios-latest-en_US", "Console router · /dev/ttyUSB0"),
		)
	})
}
