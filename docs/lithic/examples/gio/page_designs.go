package main

import (
	"image"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/design"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/symbols"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/theme"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/tokens"
)

func init() { registerPage("designs", pageDesigns) }

// the Style page's own words for each design (ryoku/hub/quickshell/schema/style.js)
var designColumns = [...]struct {
	style      design.Style
	name, note string
}{
	{design.Lithic, "Lithic", "Cut corners, panes drawn in hairlines"},
	{design.Material, "Material", "Pills and large soft cards on tonal surfaces"},
	{design.Adwaita, "Adwaita", "Rounded cards and lists, the sidebar set apart"},
	{design.Fluent, "Fluent", "Small corners and layered, outlined surfaces"},
}

// pageDesigns sets the same parts side by side in the four designs. Each
// column is a strip of window in its own design: the side plane, a list and a
// work pane, then a page pane holding the parts, so every part is seen on the
// plane it would lie on. The colours are the gallery's look, the same in all.
func pageDesigns(gtx layout.Context, th *theme.Theme) layout.Dimensions {
	return layout.UniformInset(th.Sp.S5).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return vstack(gtx, th.Sp.S4,
			func(gtx layout.Context) layout.Dimensions {
				return vstack(gtx, th.Sp.S2,
					kit.Text(th, th.T.Name, th.R.Fg, "The four designs"),
					kit.Text(th, th.T.Body, th.R.Fg2, "Planes, slabs, rows, marks, meters, fields and keycaps as each design draws them, in one look."),
				)
			},
			func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints.Min.X = gtx.Constraints.Max.X
				children := make([]layout.FlexChild, 0, len(designColumns)*2-1)
				for index, column := range designColumns {
					if index > 0 {
						children = append(children, layout.Rigid(layout.Spacer{Width: th.Sp.S3}.Layout))
					}
					children = append(children, layout.Flexed(1, designColumn(themeFor(th, column.style), column.name, column.note)))
				}
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Start}.Layout(gtx, children...)
			},
		)
	})
}

func designColumn(th *theme.Theme, name, note string) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		record := op.Record(gtx.Ops)
		dimensions := layout.UniformInset(th.Sp.S2).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return vstack(gtx, th.Sp.S3,
				func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{Top: th.Sp.S2, Left: th.Sp.S2}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return vstack(gtx, th.Sp.S1,
							kit.Text(th, th.T.Section, th.R.Fg, name),
							kit.Text(th, th.T.Caption, th.R.Fg3, note),
						)
					})
				},
				designFrame(th),
				designPage(th),
			)
		})
		call := record.Stop()
		// the strip of window the column stands for, set off where the side is the ground
		bounds := image.Rectangle{Max: dimensions.Size}
		cut := gtx.Dp(th.Cut.Mid)
		kit.Fill(gtx, th, bounds, cut, th.Planes().Side)
		kit.Stroke(gtx, th, bounds, cut, max(1, gtx.Dp(unit.Dp(tokens.SizeHairline))), th.R.Line())
		call.Add(gtx.Ops)
		return dimensions
	}
}

// designFrame is the list and detail window's two panes, 8 apart on the side.
func designFrame(th *theme.Theme) layout.Widget {
	pane := func(kind kit.PaneKind, label string) layout.Widget {
		return func(gtx layout.Context) layout.Dimensions {
			size := image.Pt(gtx.Constraints.Max.X, gtx.Dp(64))
			gtx.Constraints = layout.Exact(size)
			return (kit.Pane{Kind: kind}).Layout(gtx, th, func(gtx layout.Context) layout.Dimensions {
				layout.UniformInset(th.Sp.S3).Layout(gtx, kit.Text(th, th.T.Caption, th.R.Fg2, label))
				return layout.Dimensions{Size: size}
			})
		}
	}
	return func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
			layout.Flexed(2, pane(kit.PaneList, "List pane")),
			layout.Rigid(layout.Spacer{Width: tokens.SizePaneGap}.Layout),
			layout.Flexed(3, pane(kit.PaneWork, "Work pane")),
		)
	}
}

// designPage is the Hub's page pane with the parts on it.
func designPage(th *theme.Theme) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		return (kit.Pane{Kind: kit.PanePage}).Layout(gtx, th, func(gtx layout.Context) layout.Dimensions {
			return layout.UniformInset(th.Sp.S3).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints.Min.X = gtx.Constraints.Max.X
				return vstack(gtx, th.Sp.S4,
					specimen(th, "Page pane, a slab and a slab outlined inside it", designSlab(th)),
					specimen(th, "Slab with a tone mark", designMarkedSlab(th)),
					specimen(th, "Toned slab", func(gtx layout.Context) layout.Dimensions {
						return kit.TonedSlabContent(gtx, th, palette.Attention, kit.Claim(th, palette.Attention, "New host key"))
					}),
					specimen(th, "Tablet: rest, hover, pressed, selected, focus", designTablets(th)),
					specimen(th, "RowList with listed tablets", designRowList(th)),
					specimen(th, "Row with the selection wash and a tone mark", designRow(th)),
					specimen(th, "BladeMeter 62 and 94 percent", designMeters(th)),
					specimen(th, "BladeProgress 64 percent, and sweeping", designProgress(th)),
					specimen(th, "FieldSurface: rest, focus, invalid", designFields(th)),
					specimen(th, "Keycap and Keycaps", designKeycaps(th)),
					specimen(th, "IconPlaque mark and Slider track", designControls(th)),
				)
			})
		})
	}
}

func designSlab(th *theme.Theme) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		return kit.SlabContent(gtx, th, func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			return vstack(gtx, th.Sp.S3,
				func(gtx layout.Context) layout.Dimensions {
					return vstack(gtx, th.Sp.S1,
						kit.Text(th, th.T.Row, th.R.Fg, "Snapshots"),
						kit.Text(th, th.T.Detail, th.R.Fg2, "Kept for seven days"),
					)
				},
				func(gtx layout.Context) layout.Dimensions {
					return kit.OutlinedSlabContent(gtx, th, func(gtx layout.Context) layout.Dimensions {
						gtx.Constraints.Min.X = gtx.Constraints.Max.X
						return kit.Text(th, th.T.Mono, th.R.Fg, "/var/lib/snapshots")(gtx)
					})
				},
			)
		})
	}
}

// designMarkedSlab is a plain slab that says a state with its mark, as a card
// that needs the reader does.
func designMarkedSlab(th *theme.Theme) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		record := op.Record(gtx.Ops)
		dimensions := layout.UniformInset(tokens.SizeSlabInset).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			return vstack(gtx, th.Sp.S1,
				kit.Text(th, th.T.Row, th.R.Fg, "scratch-arch"),
				kit.Text(th, th.T.Detail, th.R.Fg2, "Waiting for your answer"),
			)
		})
		call := record.Stop()
		bounds := image.Rectangle{Max: dimensions.Size}
		kit.SlabSurface(gtx, th, bounds, gtx.Dp(th.Cut.Mid), th.R.Slab)
		kit.ToneMark(gtx, th, bounds, palette.Attention)
		call.Add(gtx.Ops)
		return dimensions
	}
}

func designTablets(th *theme.Theme) layout.Widget {
	rows := []struct {
		preview  kit.State
		selected bool
		tone     palette.Tone
		name     string
		claim    string
		progress float64
	}{
		{kit.StateRest, false, palette.Idle, "ubuntu-24.04", "Stopped", -1},
		{kit.StateHover, false, palette.Alive, "fedora-43", "Running", -1},
		{kit.StatePressed, false, palette.Attention, "windows-11", "Needs you", -1},
		{kit.StateRest, true, palette.Becoming, "alpine-instant", "Becoming", .64},
		{kit.StateFocus, false, palette.Failed, "scratch-arch", "Failed", -1},
	}
	return func(gtx layout.Context) layout.Dimensions {
		widgets := make([]layout.Widget, len(rows))
		for index, row := range rows {
			tablet := &kit.Tablet{Preview: row.preview}
			action := &kit.Plaque{Kind: kit.PlaqueQuiet, Label: "Open", Small: true}
			widgets[index] = func(gtx layout.Context) layout.Dimensions {
				return tablet.Layout(gtx, th, kit.TabletContent{
					Mark: listMark(th, kit.Icon(symbols.Computer)), Title: row.name,
					Facts: "ops@192.0.2.24", FactsMono: true, Claim: row.claim, Proof: "ally",
					Tone: row.tone, Progress: row.progress, Action: action,
				}, row.selected, false)
			}
		}
		// the focus ring stands 4 outside its row, so the rows keep clear of it
		return vstack(gtx, th.Sp.S3, widgets...)
	}
}

func designRowList(th *theme.Theme) layout.Widget {
	names := [...]string{"ally", "fedora-43", "scratch-arch"}
	return func(gtx layout.Context) layout.Dimensions {
		var list kit.RowList
		tablets := make([]kit.Tablet, len(names))
		gtx.Constraints.Max.Y = gtx.Dp(th.RowHeight())*len(names) + gtx.Dp(tokens.SizeRowListGap)*(len(names)-1)
		return list.Layout(gtx, th, len(names), 1, func(gtx layout.Context, index int) layout.Dimensions {
			tablets[index].Listed = true
			return tablets[index].Layout(gtx, th, kit.TabletContent{
				Mark: listMark(th, kit.Icon(symbols.Storage)), Title: names[index],
				Facts: "ops@192.0.2.24", FactsMono: true, Claim: "Running", Proof: "ally",
				Tone: palette.Alive, Progress: -1,
			}, index == 1, false)
		})
	}
}

// designRow is a host's own list row: the Hub's selection wash in the row
// cut, the ink that goes on it, and the row's tone as its mark.
func designRow(th *theme.Theme) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		size := image.Pt(gtx.Constraints.Max.X, gtx.Dp(40))
		bounds := image.Rectangle{Max: size}
		kit.Fill(gtx, th, bounds, gtx.Dp(th.Cut.Small), th.R.Select())
		kit.ToneMark(gtx, th, bounds, palette.Attention)
		middleLeft(gtx, size, tokens.SizeTabletInsetLeft, kit.Text(th, th.T.Row, th.R.OnContainer, "Review the host key"))
		return layout.Dimensions{Size: size}
	}
}

func designMeters(th *theme.Theme) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		return vstack(gtx, th.Sp.S3,
			func(gtx layout.Context) layout.Dimensions { return (kit.BladeMeter{Fraction: .62}).Layout(gtx, th) },
			func(gtx layout.Context) layout.Dimensions { return (kit.BladeMeter{Fraction: .94}).Layout(gtx, th) },
		)
	}
}

func designProgress(th *theme.Theme) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		return vstack(gtx, th.Sp.S3,
			func(gtx layout.Context) layout.Dimensions {
				return (kit.BladeProgress{Fraction: .64, Active: true}).Layout(gtx, th)
			},
			func(gtx layout.Context) layout.Dimensions {
				return (kit.BladeProgress{Fraction: -1, Active: true}).Layout(gtx, th)
			},
		)
	}
}

func designFields(th *theme.Theme) layout.Widget {
	fields := [...]struct {
		text    string
		focus   float32
		invalid bool
	}{
		{"A host's own field at rest", 0, false},
		{"With keyboard focus", 1, false},
		{"Holding a value it rejects", 0, true},
	}
	return func(gtx layout.Context) layout.Dimensions {
		widgets := make([]layout.Widget, len(fields))
		for index, field := range fields {
			widgets[index] = designFieldSurface(th, field.text, field.focus, field.invalid, tokens.SizeTextFieldHeight)
		}
		return vstack(gtx, th.Sp.S2, widgets...)
	}
}

// designFieldSurface is a host's own field of the given height on the shared
// field surface; one taller than a text field sets its text at the top, as a
// multi-line editor does.
func designFieldSurface(th *theme.Theme, text string, focus float32, invalid bool, height unit.Dp) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		size := image.Pt(gtx.Constraints.Max.X, gtx.Dp(height))
		kit.FieldSurface(gtx, th, image.Rectangle{Max: size}, gtx.Dp(th.Cut.Mid), th.R.Field, focus, invalid)
		line := kit.Text(th, th.T.Body, th.R.Fg2, text)
		if height > tokens.SizeTextFieldHeight {
			gtx.Constraints = layout.Constraints{Max: size}
			layout.UniformInset(tokens.SizeTextFieldInset).Layout(gtx, line)
		} else {
			middleLeft(gtx, size, tokens.SizeTextFieldInset, line)
		}
		return layout.Dimensions{Size: size}
	}
}

func designKeycaps(th *theme.Theme) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(kit.Keycaps(th, kit.ChordFaces("Super+Shift+Return")...)),
			layout.Rigid(layout.Spacer{Width: th.Sp.S3}.Layout),
			layout.Rigid(kit.Keycap(th, "Esc")),
		)
	}
}

func designControls(th *theme.Theme) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		pending := &kit.IconPlaque{Kind: kit.PlaqueQuiet, Icon: kit.IconByName("settings"), Description: "Settings, one change pending", Notch: th.R.Amber, Preview: kit.StateHover}
		scale := &kit.Slider{Min: 0, Max: 100, Value: 62, Label: "Console scale"}
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions { return pending.Layout(gtx, th) }),
			layout.Rigid(layout.Spacer{Width: th.Sp.S4}.Layout),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions { return scale.Layout(gtx, th) }),
		)
	}
}

// middleLeft sets widget at the vertical middle of a box of size, left in from
// its left edge.
func middleLeft(gtx layout.Context, size image.Point, left unit.Dp, widget layout.Widget) {
	inset := gtx.Dp(left)
	gtx.Constraints = layout.Constraints{Max: image.Pt(max(0, size.X-inset), size.Y)}
	record := op.Record(gtx.Ops)
	dimensions := widget(gtx)
	call := record.Stop()
	offset := op.Offset(image.Pt(inset, (size.Y-dimensions.Size.Y)/2)).Push(gtx.Ops)
	call.Add(gtx.Ops)
	offset.Pop()
}
