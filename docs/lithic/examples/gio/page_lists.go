package main

import (
	"image"

	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/symbols"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/theme"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/tokens"
)

func init() { registerPage("lists", pageLists) }

func pageLists(gtx layout.Context, th *theme.Theme) layout.Dimensions {
	return layout.Inset{Top: th.Sp.S6, Right: th.Sp.S6, Bottom: th.Sp.S6, Left: th.Sp.S6}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return vstack(gtx, th.Sp.S6,
			func(gtx layout.Context) layout.Dimensions {
				return vstack(gtx, th.Sp.S2,
					kit.Text(th, th.T.Name, th.R.Fg, "Lists, containers and progress"),
					kit.Text(th, th.T.Body, th.R.Fg2, "Fleet resources, state, measured work and draft composition in the shared Lithic language."),
				)
			},
			listsTablet(th),
			listsRowList(th),
			settingsSpecimens.scrollRailSection(th),
			settingsSpecimens.designTileSection(th),
			settingsSpecimens.designGallerySection(th),
			listsGroupHeader(th),
			listsSkeleton(th),
			listsEmpty(th),
			listsPane(th),
			listsMineralField(th),
			listsHeader(th),
			listsSlab(th),
			listsSection(th),
			listsFacts(th),
			listsAside(th),
			listsDraftColumns(th),
			listsClaim(th),
			listsMarks(th),
			listsSentence(th),
			listsBladeMeter(th),
			listsBladeProgress(th),
			listsWorking(th),
			listsBecomingName(th),
		)
	})
}

func listsTablet(th *theme.Theme) layout.Widget {
	states := []struct {
		label     string
		preview   kit.State
		selectRow bool
		tone      palette.Tone
		name      string
		claim     string
		proof     string
		progress  float64
	}{
		{"Rest", kit.StateRest, false, palette.Idle, "ubuntu-24.04", "Stopped", "ally", -1},
		{"Hover action", kit.StateHover, false, palette.Alive, "fedora-43", "Running", "prowl", -1},
		{"Pressed", kit.StatePressed, false, palette.Attention, "windows-11", "Needs you", "localhost", -1},
		{"Focus selected", kit.StateFocus, true, palette.Becoming, "alpine-instant", "Becoming", "mika-server", .64},
	}
	return section(th, "Tablet", func(gtx layout.Context) layout.Dimensions {
		widgets := make([]layout.Widget, len(states))
		for index, state := range states {
			state := state
			action := &kit.Plaque{Kind: kit.PlaqueQuiet, Label: "Inspect", Small: true}
			tablet := &kit.Tablet{Preview: state.preview, Pane: th.R.Ground}
			widgets[index] = specimen(th, state.label, func(gtx layout.Context) layout.Dimensions {
				draw := func(gtx layout.Context) layout.Dimensions {
					return tablet.Layout(gtx, th, kit.TabletContent{
						Mark: listMark(th, kit.Icon(symbols.Computer)), Title: state.name,
						Facts: "ops@192.0.2.24", FactsMono: true, Claim: state.claim,
						Proof: state.proof, Tone: state.tone, Progress: state.progress, Action: action,
					}, state.selectRow, false)
				}
				if state.preview == kit.StateFocus {
					inset := unit.Dp(tokens.SizeFocusRingGap + tokens.SizeFocusRingWidth)
					return layout.UniformInset(inset).Layout(gtx, draw)
				}
				return draw(gtx)
			})
		}
		return grid(gtx, 2, th.Sp.S3, widgets)
	})
}

func listsRowList(th *theme.Theme) layout.Widget {
	return section(th, "RowList", specimen(th, "Selected slab, wheel, trackpad and keyboard", func(gtx layout.Context) layout.Dimensions {
		rows := []struct {
			name, facts string
			icon        *widget.Icon
		}{
			{"ally", "ops@192.0.2.24", kit.Icon(symbols.Storage)},
			{"fedora-43", "prowl@198.51.100.43", kit.Icon(symbols.Computer)},
			{"scratch-arch", "ops@203.0.113.18", kit.Icon(symbols.Computer)},
		}
		var list kit.RowList
		tablets := make([]kit.Tablet, len(rows))
		gtx.Constraints.Max.Y = gtx.Dp(th.RowHeight())*2 + gtx.Dp(tokens.SizeRowListGap)
		return list.Layout(gtx, th, len(rows), 1, func(gtx layout.Context, index int) layout.Dimensions {
			return tablets[index].Layout(gtx, th, kit.TabletContent{
				Mark: listMark(th, rows[index].icon), Title: rows[index].name,
				Facts: rows[index].facts, FactsMono: true, Claim: "Running", Proof: "ally", Tone: palette.Alive, Progress: -1,
			}, index == 1, index == 1)
		})
	}))
}

func listsGroupHeader(th *theme.Theme) layout.Widget {
	return section(th, "GroupHeader", func(gtx layout.Context) layout.Dimensions {
		labels := []struct {
			title string
			count int
		}{{"Needs you", 2}, {"Pinned", 3}, {"Running", 4}, {"Becoming", 1}, {"Stopped", 6}}
		widgets := make([]layout.Widget, len(labels))
		for index, group := range labels {
			widgets[index] = specimen(th, group.title, kit.GroupHeader(th, group.title, group.count))
		}
		return grid(gtx, 3, th.Sp.S3, widgets)
	})
}

func listsSkeleton(th *theme.Theme) layout.Widget {
	return section(th, "Skeleton", specimen(th, "Active static rows", func(gtx layout.Context) layout.Dimensions {
		return (kit.Skeleton{Active: true, Rows: 2}).Layout(gtx, th)
	}))
}

func listsEmpty(th *theme.Theme) layout.Widget {
	return section(th, "Empty", specimen(th, "Next step named", func(gtx layout.Context) layout.Dimensions {
		action := &kit.Plaque{Kind: kit.PlaqueGold, Label: "Connect server"}
		gtx.Constraints.Min.Y = gtx.Dp(tokens.SizeEmptyFrame) + gtx.Dp(th.Sp.S6)*3
		return (kit.Empty{Icon: kit.IconByName("search"), Title: "No servers match", Body: "Clear the filter or connect a server to bring its machines into the fleet.", Actions: []*kit.Plaque{action}}).Layout(gtx, th)
	}))
}

func listsPane(th *theme.Theme) layout.Widget {
	return section(th, "Pane", func(gtx layout.Context) layout.Dimensions {
		widgets := []layout.Widget{
			specimen(th, "Rail and list plane", func(gtx layout.Context) layout.Dimensions {
				return kit.Slab(gtx, th.R.Ground, th.Cut.Mid, func(gtx layout.Context) layout.Dimensions {
					return layout.UniformInset(th.Sp.S3).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						gtx.Constraints.Min.Y = gtx.Dp(tokens.SizeTabletHeight * 2)
						return (kit.Pane{Kind: kit.PaneList}).Layout(gtx, th, func(gtx layout.Context) layout.Dimensions {
							return layout.Dimensions{Size: gtx.Constraints.Min}
						})
					})
				})
			}),
			specimen(th, "Work plane", func(gtx layout.Context) layout.Dimensions {
				return kit.Slab(gtx, th.R.Ground, th.Cut.Mid, func(gtx layout.Context) layout.Dimensions {
					return layout.UniformInset(th.Sp.S3).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						gtx.Constraints.Min.Y = gtx.Dp(tokens.SizeTabletHeight * 2)
						return (kit.Pane{Kind: kit.PaneWork}).Layout(gtx, th, func(gtx layout.Context) layout.Dimensions {
							return layout.Dimensions{Size: gtx.Constraints.Min}
						})
					})
				})
			}),
		}
		return grid(gtx, 2, th.Sp.S3, widgets)
	})
}

func listsMineralField(th *theme.Theme) layout.Widget {
	return section(th, "MineralField", func(gtx layout.Context) layout.Dimensions {
		states := []struct {
			label string
			tone  palette.Tone
		}{{"Idle", palette.Idle}, {"Alive", palette.Alive}, {"Becoming", palette.Becoming}, {"Attention", palette.Attention}, {"Failed", palette.Failed}}
		widgets := make([]layout.Widget, len(states))
		for index, state := range states {
			state := state
			widgets[index] = specimen(th, state.label, func(gtx layout.Context) layout.Dimensions {
				var field kit.MineralField
				return field.Layout(gtx, th, state.tone, func(gtx layout.Context) layout.Dimensions {
					return layout.UniformInset(th.Sp.S4).Layout(gtx, kit.Claim(th, state.tone, state.label))
				})
			})
		}
		return grid(gtx, 3, th.Sp.S3, widgets)
	})
}

func listsHeader(th *theme.Theme) layout.Widget {
	return section(th, "Header", specimen(th, "Selected resource", func(gtx layout.Context) layout.Dimensions {
		var header kit.Header
		start := &kit.Plaque{Kind: kit.PlaqueGold, Label: "Start"}
		inspect := &kit.Plaque{Kind: kit.PlaqueLine, Label: "Inspect"}
		return header.Layout(gtx, th, kit.HeaderContent{
			Tone: palette.Becoming, Mark: listMark(th, kit.Icon(symbols.Computer)), Name: "alpine-instant", Progress: .64,
			Proof: []kit.Span{{Text: "Creating on "}, {Text: "mika-server", Mono: true}},
			Note:  "Latest stage: preparing the root filesystem", Marks: kit.ResourceMarks(true, true, true),
			Actions: []*kit.Plaque{start, inspect},
			Below: func(gtx layout.Context) layout.Dimensions {
				return (kit.BladeProgress{Fraction: .64, Active: true}).Layout(gtx, th)
			},
		})
	}))
}

func listsSlab(th *theme.Theme) layout.Widget {
	plain := specimen(th, "Rows on a lifted slab", func(gtx layout.Context) layout.Dimensions {
		return kit.SlabRows(gtx, th,
			settingRow(th, "Console router", "Reached from ally over the serial line", kit.Text(th, th.T.Mono, th.R.Fg, "/dev/ttyUSB0")),
			settingRow(th, "Snapshots", "Kept on mika-server for seven days", kit.Text(th, th.T.Mono, th.R.Fg, "7 d")),
		)
	})
	toned := specimen(th, "Toned, the hero plane", func(gtx layout.Context) layout.Dimensions {
		return kit.TonedSlabContent(gtx, th, palette.Attention, kit.Claim(th, palette.Attention, "scratch-arch has a new host key"))
	})
	return section(th, "Slab", func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(plain),
			layout.Rigid(layout.Spacer{Height: th.Sp.S4}.Layout),
			layout.Rigid(toned),
		)
	})
}

func listsSection(th *theme.Theme) layout.Widget {
	return section(th, "Section", specimen(th, "Titled body", kit.Section(th, "Connection", kit.Text(th, th.T.Body, th.R.Fg2, "ops@192.0.2.24"))))
}

func listsFacts(th *theme.Theme) layout.Widget {
	return section(th, "Facts", specimen(th, "Label and value rows", kit.Facts(th, []kit.Fact{
		{Label: "Server", Value: "ally"}, {Label: "Address", Value: "192.0.2.24", Mono: true}, {Label: "Console", Value: "/dev/ttyUSB0", Mono: true}, {Label: "Snapshot"},
	})))
}

func listsAside(th *theme.Theme) layout.Widget {
	return section(th, "Aside", specimen(th, "Live save preview", func(gtx layout.Context) layout.Dimensions {
		return kit.Aside(gtx, th, "Connect ally",
			kit.AsideFact(th, "User", "ops"), kit.AsideFact(th, "Address", "192.0.2.24"),
			kit.AsideNote(th, "Saving adds this host to the fleet."),
			kit.AsidePreview(th, "Host ally\n  HostName 192.0.2.24\n  User ops"),
		)
	}))
}

func listsDraftColumns(th *theme.Theme) layout.Widget {
	return section(th, "DraftColumns", specimen(th, "Wide at 820 dp", func(gtx layout.Context) layout.Dimensions {
		form := kit.SlabContent
		return kit.DraftColumns(gtx, th,
			func(gtx layout.Context) layout.Dimensions {
				return form(gtx, th, kit.Facts(th, []kit.Fact{{Label: "Name", Value: "ally"}, {Label: "Address", Value: "192.0.2.24", Mono: true}}))
			},
			func(gtx layout.Context) layout.Dimensions {
				return kit.Aside(gtx, th, "Save connection", kit.AsideFact(th, "Target", "ally"))
			},
		)
	}))
}

func listsClaim(th *theme.Theme) layout.Widget {
	return section(th, "Claim", func(gtx layout.Context) layout.Dimensions {
		claims := []struct {
			label string
			tone  palette.Tone
		}{{"Idle", palette.Idle}, {"Alive", palette.Alive}, {"Becoming", palette.Becoming}, {"Attention", palette.Attention}, {"Paused", palette.Paused}, {"Failed", palette.Failed}}
		widgets := make([]layout.Widget, len(claims))
		for index, claim := range claims {
			widgets[index] = specimen(th, claim.label, kit.Claim(th, claim.tone, claim.label))
		}
		return grid(gtx, 3, th.Sp.S3, widgets)
	})
}

func listsMarks(th *theme.Theme) layout.Widget {
	return section(th, "Marks", specimen(th, "Ordered resource traits", kit.Marks(th, kit.ResourceMarks(true, true, true))))
}

func listsSentence(th *theme.Theme) layout.Widget {
	return section(th, "Sentence", specimen(th, "Prose and literal spans", func(gtx layout.Context) layout.Dimensions {
		return kit.Sentence(gtx, th, th.T.Body, th.R.Fg2, []kit.Span{{Text: "Connect "}, {Text: "ops@192.0.2.24", Mono: true}, {Text: " to "}, {Text: "ally"}, {Text: "."}})
	}))
}

func listsBladeMeter(th *theme.Theme) layout.Widget {
	return section(th, "BladeMeter", func(gtx layout.Context) layout.Dimensions {
		return grid(gtx, 2, th.Sp.S3, []layout.Widget{
			specimen(th, "Memory 62 percent", func(gtx layout.Context) layout.Dimensions { return (kit.BladeMeter{Fraction: .62}).Layout(gtx, th) }),
			specimen(th, "Disk attention 94 percent", func(gtx layout.Context) layout.Dimensions { return (kit.BladeMeter{Fraction: .94}).Layout(gtx, th) }),
		})
	})
}

func listsBladeProgress(th *theme.Theme) layout.Widget {
	return section(th, "BladeProgress", func(gtx layout.Context) layout.Dimensions {
		return grid(gtx, 2, th.Sp.S3, []layout.Widget{
			specimen(th, "Determinate 64 percent", func(gtx layout.Context) layout.Dimensions {
				return (kit.BladeProgress{Fraction: .64, Active: true}).Layout(gtx, th)
			}),
			specimen(th, "Indeterminate sweep", func(gtx layout.Context) layout.Dimensions {
				return (kit.BladeProgress{Fraction: -1, Active: true}).Layout(gtx, th)
			}),
		})
	})
}

func listsWorking(th *theme.Theme) layout.Widget {
	return section(th, "Working", func(gtx layout.Context) layout.Dimensions {
		return grid(gtx, 2, th.Sp.S3, []layout.Widget{
			specimen(th, "Active", func(gtx layout.Context) layout.Dimensions {
				return (kit.Working{Active: true, Size: tokens.SizeIconPlaque}).Layout(gtx, th)
			}),
			specimen(th, "Inactive", func(gtx layout.Context) layout.Dimensions {
				return (kit.Working{Active: false, Size: tokens.SizeIconPlaque}).Layout(gtx, th)
			}),
		})
	})
}

func listsBecomingName(th *theme.Theme) layout.Widget {
	return section(th, "BecomingName", specimen(th, "alpine-instant at 64 percent", func(gtx layout.Context) layout.Dimensions {
		return (kit.BecomingName{Name: "alpine-instant", Fraction: .64, Tone: palette.Becoming}).Layout(gtx, th)
	}))
}

func listMark(th *theme.Theme, icon *widget.Icon) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		if icon == nil {
			return layout.Dimensions{Size: gtx.Constraints.Min}
		}
		size := min(gtx.Constraints.Max.X, gtx.Constraints.Max.Y)
		gtx.Constraints = layout.Exact(image.Pt(size, size))
		return icon.Layout(gtx, th.R.Fg2)
	}
}
