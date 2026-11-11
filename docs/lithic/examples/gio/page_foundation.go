package main

import (
	"image"
	"image/color"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/theme"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/tokens"
)

func pageFoundation(gtx layout.Context, th *theme.Theme) layout.Dimensions {
	return layout.Inset{Top: th.Sp.S6, Right: th.Sp.S6, Bottom: th.Sp.S6, Left: th.Sp.S6}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return vstack(gtx, th.Sp.S6,
			pageTitle(th),
			foundationRoles(th),
			foundationType(th),
			foundationCutScale(th),
			foundationSpace(th),
			foundationCutRect(th),
			foundationNotch(th),
			foundationBlade(th),
			foundationFlake(th),
			foundationHatch(th),
			foundationFocus(th),
			foundationGlyph(th),
			foundationMotion(th),
		)
	})
}

func pageTitle(th *theme.Theme) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		return vstack(gtx, th.Sp.S2,
			kit.Text(th, th.T.Name, th.R.Fg, "Foundation"),
			kit.Text(th, th.T.Body, th.R.Fg2, "Roles, type, cut geometry, state marks and motion shared by every Lithic component."),
		)
	}
}

func foundationRoles(th *theme.Theme) layout.Widget {
	type role struct {
		group string
		name  string
		ink   color.NRGBA
	}
	roles := []role{
		{"Planes", "ground", th.R.Ground}, {"Planes", "stone1", th.R.Stone1}, {"Planes", "stone2", th.R.Stone2},
		{"Planes", "stone3", th.R.Stone3}, {"Planes", "stone4", th.R.Stone4}, {"Planes", "slab", th.R.Slab},
		{"Planes", "field", th.R.Field}, {"Lines", "vein", th.R.Vein}, {"Lines", "edge", th.R.Edge},
		{"Ink", "fg", th.R.Fg}, {"Ink", "fg2", th.R.Fg2}, {"Ink", "fg3", th.R.Fg3},
		{"Selection", "sel", th.R.Sel}, {"Selection", "onSel", th.R.OnSel}, {"Selection", "onSel2", th.R.OnSel2},
		{"Action", "gold", th.R.Gold}, {"Action", "onGold", th.R.OnGold}, {"Action", "goldInk", th.R.GoldInk},
		{"Minerals", "jade", th.R.Jade}, {"Minerals", "lapis", th.R.Lapis}, {"Minerals", "amber", th.R.Amber}, {"Minerals", "jasper", th.R.Jasper},
		{"Overlay", "scrim", th.R.Scrim},
	}
	return section(th, "Roles", func(gtx layout.Context) layout.Dimensions {
		widgets := make([]layout.Widget, 0, len(roles))
		for _, item := range roles {
			item := item
			widgets = append(widgets, roleTile(th, item.group, item.name, item.ink))
		}
		return grid(gtx, 4, th.Sp.S2, widgets)
	})
}

func roleTile(th *theme.Theme, group, name string, ink color.NRGBA) layout.Widget {
	return tile(th, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				size := image.Pt(gtx.Dp(44), gtx.Dp(44))
				paint.FillShape(gtx.Ops, ink, clip.Rect{Max: size}.Op())
				return layout.Dimensions{Size: size}
			}),
			layout.Rigid(layout.Spacer{Width: th.Sp.S2}.Layout),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return vstack(gtx, th.Sp.S1,
					kit.Text(th, th.T.Caption, th.R.Fg3, group),
					kit.Text(th, th.T.Label, th.R.Fg, name),
				)
			}),
		)
	})
}

func foundationType(th *theme.Theme) layout.Widget {
	type sample struct {
		name  string
		text  string
		style theme.TextStyle
	}
	samples := []sample{
		{"Name", "ubuntu-24.04", th.T.Name}, {"Sentence", "Three running. One needs you.", th.T.Sentence},
		{"Section", "Fleet", th.T.Section}, {"Title", "Connection", th.T.Title},
		{"Row", "mika-server", th.T.Row}, {"Body", "Reached over SSH from ally.", th.T.Body},
		{"Proof", "Online in 23 ms", th.T.Proof}, {"Label", "Connect", th.T.Label},
		{"Detail", "Wakes ally before the console opens", th.T.Detail},
		{"Caption", "Last checked now", th.T.Caption}, {"Group", "Needs you", th.T.Group},
		{"Mono", "ops@127.0.0.1", th.T.Mono}, {"Mono small", "/dev/ttyUSB0", th.T.MonoSm},
	}
	return section(th, "Type", func(gtx layout.Context) layout.Dimensions {
		widgets := make([]layout.Widget, 0, len(samples))
		for _, sample := range samples {
			sample := sample
			widgets = append(widgets, tile(th, func(gtx layout.Context) layout.Dimensions {
				return vstack(gtx, th.Sp.S1,
					kit.Text(th, th.T.Caption, th.R.Fg3, sample.name),
					kit.Text(th, sample.style, th.R.Fg, sample.text),
				)
			}))
		}
		return grid(gtx, 2, th.Sp.S2, widgets)
	})
}

func foundationCutScale(th *theme.Theme) layout.Widget {
	items := []struct {
		name string
		cut  unit.Dp
	}{
		{"Token", th.Cut.Token}, {"Small", th.Cut.Small}, {"Segment", th.Cut.Segment},
		{"Mid", th.Cut.Mid}, {"Overlay", th.Cut.Overlay}, {"Notch", th.Cut.Notch}, {"Large", th.Cut.Large},
	}
	return section(th, "Cut scale", func(gtx layout.Context) layout.Dimensions {
		widgets := make([]layout.Widget, 0, len(items))
		for _, item := range items {
			item := item
			widgets = append(widgets, specimen(th, item.name, func(gtx layout.Context) layout.Dimensions {
				return canvas(gtx, 150, 58, func(ops *op.Ops, bounds image.Rectangle) {
					kit.FillCut(ops, bounds.Inset(4), gtx.Dp(item.cut), th.R.Sel)
				})
			}))
		}
		return grid(gtx, 3, th.Sp.S2, widgets)
	})
}

func foundationSpace(th *theme.Theme) layout.Widget {
	items := []struct {
		name string
		size unit.Dp
	}{{"s1", th.Sp.S1}, {"s2", th.Sp.S2}, {"s3", th.Sp.S3}, {"s4", th.Sp.S4}, {"s5", th.Sp.S5}, {"s6", th.Sp.S6}}
	return section(th, "Space", func(gtx layout.Context) layout.Dimensions {
		widgets := make([]layout.Widget, 0, len(items))
		for _, item := range items {
			item := item
			widgets = append(widgets, specimen(th, item.name, func(gtx layout.Context) layout.Dimensions {
				return canvas(gtx, 150, 58, func(ops *op.Ops, bounds image.Rectangle) {
					width := gtx.Dp(item.size)
					paint.FillShape(ops, th.R.Fg2, clip.Rect(image.Rect(4, 16, 4+width, 42)).Op())
				})
			}))
		}
		return grid(gtx, 3, th.Sp.S2, widgets)
	})
}

func foundationCutRect(th *theme.Theme) layout.Widget {
	items := []struct {
		name string
		draw func(*op.Ops, image.Rectangle, int)
	}{
		{"Filled", func(ops *op.Ops, bounds image.Rectangle, cut int) { kit.FillCut(ops, bounds, cut, th.R.Sel) }},
		{"Inside stroke", func(ops *op.Ops, bounds image.Rectangle, cut int) { kit.StrokeCut(ops, bounds, cut, 2, th.R.Fg2) }},
		{"Top-left", func(ops *op.Ops, bounds image.Rectangle, cut int) {
			kit.FillCutCorners(ops, bounds, cut, 0, 0, 0, th.R.Stone4)
		}},
		{"Top-right", func(ops *op.Ops, bounds image.Rectangle, cut int) {
			kit.FillCutCorners(ops, bounds, 0, cut, 0, 0, th.R.Stone4)
		}},
		{"Bottom-right", func(ops *op.Ops, bounds image.Rectangle, cut int) {
			kit.FillCutCorners(ops, bounds, 0, 0, cut, 0, th.R.Stone4)
		}},
		{"Bottom-left", func(ops *op.Ops, bounds image.Rectangle, cut int) {
			kit.FillCutCorners(ops, bounds, 0, 0, 0, cut, th.R.Stone4)
		}},
	}
	return section(th, "CutRect", func(gtx layout.Context) layout.Dimensions {
		widgets := make([]layout.Widget, 0, len(items))
		for _, item := range items {
			item := item
			widgets = append(widgets, specimen(th, item.name, func(gtx layout.Context) layout.Dimensions {
				return canvas(gtx, 180, 80, func(ops *op.Ops, bounds image.Rectangle) {
					item.draw(ops, bounds.Inset(8), gtx.Dp(th.Cut.Large))
				})
			}))
		}
		return grid(gtx, 3, th.Sp.S2, widgets)
	})
}

func foundationNotch(th *theme.Theme) layout.Widget {
	return section(th, "Notch", func(gtx layout.Context) layout.Dimensions {
		return grid(gtx, 3, th.Sp.S2, []layout.Widget{
			specimen(th, "Alive", notchCanvas(th, th.R.Jade)),
			specimen(th, "Attention", notchCanvas(th, th.R.Amber)),
			specimen(th, "Failed", notchCanvas(th, th.R.Jasper)),
		})
	})
}

func notchCanvas(th *theme.Theme, ink color.NRGBA) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		return canvas(gtx, 180, 80, func(ops *op.Ops, bounds image.Rectangle) {
			kit.FillCut(ops, bounds.Inset(8), gtx.Dp(th.Cut.Mid), th.R.Stone2)
			kit.Notch(ops, image.Pt(8, 8), gtx.Dp(th.Cut.Notch), ink)
		})
	}
}

func foundationBlade(th *theme.Theme) layout.Widget {
	return section(th, "Blade", func(gtx layout.Context) layout.Dimensions {
		fractions := []struct {
			name string
			part int
		}{{"25%", 25}, {"50%", 50}, {"75%", 75}}
		widgets := make([]layout.Widget, 0, len(fractions))
		for _, item := range fractions {
			item := item
			widgets = append(widgets, specimen(th, item.name, func(gtx layout.Context) layout.Dimensions {
				return canvas(gtx, 240, 48, func(ops *op.Ops, bounds image.Rectangle) {
					track := image.Rect(8, 20, bounds.Dx()-8, 28)
					paint.FillShape(ops, th.R.Field, clip.Rect(track).Op())
					fill := track
					fill.Max.X = fill.Min.X + fill.Dx()*item.part/100
					kit.Blade(ops, fill, th.R.Lapis)
				})
			}))
		}
		return grid(gtx, 3, th.Sp.S2, widgets)
	})
}

func foundationFlake(th *theme.Theme) layout.Widget {
	items := []struct {
		name     string
		tone     palette.Tone
		fraction float64
	}{{"Idle", palette.Idle, 0}, {"Alive", palette.Alive, 1}, {"Becoming 40%", palette.Becoming, .4}, {"Attention", palette.Attention, 1}, {"Paused", palette.Paused, 1}, {"Failed", palette.Failed, 1}}
	return section(th, "Flake", func(gtx layout.Context) layout.Dimensions {
		widgets := make([]layout.Widget, 0, len(items))
		for _, item := range items {
			item := item
			widgets = append(widgets, specimen(th, item.name, func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints.Min = image.Point{}
				return layout.Inset{Top: 20, Bottom: 20}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return kit.Flake(gtx, th, item.tone, item.fraction, 22)
					})
				})
			}))
		}
		return grid(gtx, 3, th.Sp.S2, widgets)
	})
}

func foundationHatch(th *theme.Theme) layout.Widget {
	return section(th, "Hatch", func(gtx layout.Context) layout.Dimensions {
		return grid(gtx, 2, th.Sp.S2, []layout.Widget{
			specimen(th, "Engraved plane", func(gtx layout.Context) layout.Dimensions {
				return canvas(gtx, 320, 100, func(ops *op.Ops, bounds image.Rectangle) {
					area := bounds.Inset(8)
					kit.FillCut(ops, area, gtx.Dp(th.Cut.Mid), th.R.Stone2)
					path := kit.CutPath(ops, area, gtx.Dp(th.Cut.Mid), gtx.Dp(th.Cut.Mid), gtx.Dp(th.Cut.Mid), gtx.Dp(th.Cut.Mid))
					kit.Hatch(ops, path, area, gtx.Dp(6), max(1, gtx.Dp(1)), palette.WithAlpha(th.R.Fg, tokens.AlphaHatch))
				})
			}),
			specimen(th, "Hairline", func(gtx layout.Context) layout.Dimensions {
				return canvas(gtx, 320, 100, func(ops *op.Ops, bounds image.Rectangle) {
					area := bounds.Inset(8)
					path := kit.CutPath(ops, area, gtx.Dp(th.Cut.Mid), gtx.Dp(th.Cut.Mid), gtx.Dp(th.Cut.Mid), gtx.Dp(th.Cut.Mid))
					kit.Hatch(ops, path, area, gtx.Dp(10), max(1, gtx.Dp(1)), palette.WithAlpha(th.R.Fg2, tokens.AlphaHatch))
				})
			}),
		})
	})
}

func foundationFocus(th *theme.Theme) layout.Widget {
	return section(th, "FocusRing", func(gtx layout.Context) layout.Dimensions {
		return grid(gtx, 2, th.Sp.S2, []layout.Widget{
			specimen(th, "Pointer focus", focusCanvas(th, 0)),
			specimen(th, "Keyboard focus", focusCanvas(th, 1)),
		})
	})
}

func focusCanvas(th *theme.Theme, alpha float32) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		return canvas(gtx, 260, 92, func(ops *op.Ops, bounds image.Rectangle) {
			control := image.Rect(24, 24, bounds.Dx()-24, bounds.Dy()-24)
			kit.FillCut(ops, control, gtx.Dp(th.Cut.Mid), th.R.Field)
			kit.StrokeCut(ops, control, gtx.Dp(th.Cut.Mid), max(1, gtx.Dp(1)), th.R.Edge)
			kit.FocusRingAlpha(gtx, th, control, gtx.Dp(th.Cut.Mid), alpha)
		})
	}
}

func foundationGlyph(th *theme.Theme) layout.Widget {
	items := []struct{ name, glyph string }{{"Home", "home"}, {"Search", "search"}, {"Settings", "settings"}, {"Close", "close"}}
	return section(th, "Glyph", func(gtx layout.Context) layout.Dimensions {
		widgets := make([]layout.Widget, 0, len(items))
		for _, item := range items {
			item := item
			widgets = append(widgets, specimen(th, item.name, func(gtx layout.Context) layout.Dimensions {
				return canvas(gtx, 150, 72, func(ops *op.Ops, bounds image.Rectangle) {
					icon := kit.IconByName(item.glyph)
					if icon == nil {
						return
					}
					size := gtx.Dp(28)
					offset := op.Offset(image.Pt((bounds.Dx()-size)/2, (bounds.Dy()-size)/2)).Push(ops)
					iconGTX := gtx
					iconGTX.Constraints = layout.Exact(image.Pt(size, size))
					icon.Layout(iconGTX, th.R.Fg)
					offset.Pop()
				})
			}))
		}
		return grid(gtx, 4, th.Sp.S2, widgets)
	})
}

func foundationMotion(th *theme.Theme) layout.Widget {
	return section(th, "Motion", func(gtx layout.Context) layout.Dimensions {
		return specimen(th, "At rest", func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: th.Sp.S3, Bottom: th.Sp.S3}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return canvas(gtx, 220, 12, func(ops *op.Ops, bounds image.Rectangle) {
							paint.FillShape(ops, th.R.Field, clip.Rect(bounds).Op())
							fill := bounds
							fill.Max.X = fill.Min.X + fill.Dx()*3/5
							kit.Blade(ops, fill, th.R.Lapis)
						})
					}),
					layout.Rigid(layout.Spacer{Width: th.Sp.S3}.Layout),
					layout.Rigid(kit.Text(th, th.T.Body, th.R.Fg2, "No timer and no requested frame")),
				)
			})
		})(gtx)
	})
}

func section(th *theme.Theme, title string, body layout.Widget) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		return vstack(gtx, th.Sp.S3,
			kit.Text(th, th.T.Section, th.R.Fg, title),
			body,
		)
	}
}

func specimen(th *theme.Theme, label string, body layout.Widget) layout.Widget {
	return tile(th, func(gtx layout.Context) layout.Dimensions {
		return vstack(gtx, th.Sp.S2,
			kit.Text(th, th.T.Label, th.R.Fg2, label),
			body,
		)
	})
}

func tile(th *theme.Theme, body layout.Widget) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		record := op.Record(gtx.Ops)
		dimensions := layout.Inset{Top: th.Sp.S3, Right: th.Sp.S3, Bottom: th.Sp.S3, Left: th.Sp.S3}.Layout(gtx, body)
		call := record.Stop()
		bounds := image.Rectangle{Max: dimensions.Size}
		kit.FillCut(gtx.Ops, bounds, gtx.Dp(th.Cut.Mid), th.R.Slab)
		kit.StrokeCut(gtx.Ops, bounds, gtx.Dp(th.Cut.Mid), max(1, gtx.Dp(1)), th.R.Vein)
		call.Add(gtx.Ops)
		return dimensions
	}
}

func canvas(gtx layout.Context, width, height int, draw func(*op.Ops, image.Rectangle)) layout.Dimensions {
	size := gtx.Constraints.Constrain(image.Pt(gtx.Dp(unit.Dp(width)), gtx.Dp(unit.Dp(height))))
	draw(gtx.Ops, image.Rectangle{Max: size})
	return layout.Dimensions{Size: size}
}

func vstack(gtx layout.Context, gap unit.Dp, widgets ...layout.Widget) layout.Dimensions {
	children := make([]layout.FlexChild, 0, len(widgets)*2)
	for index, widget := range widgets {
		if index > 0 {
			children = append(children, layout.Rigid(layout.Spacer{Height: gap}.Layout))
		}
		children = append(children, layout.Rigid(widget))
	}
	return layout.Flex{Axis: layout.Vertical, Alignment: layout.Start}.Layout(gtx, children...)
}

func grid(gtx layout.Context, columns int, gap unit.Dp, widgets []layout.Widget) layout.Dimensions {
	rows := make([]layout.Widget, 0, (len(widgets)+columns-1)/columns)
	for start := 0; start < len(widgets); start += columns {
		end := min(start+columns, len(widgets))
		rowWidgets := append([]layout.Widget(nil), widgets[start:end]...)
		rows = append(rows, func(gtx layout.Context) layout.Dimensions {
			children := make([]layout.FlexChild, 0, columns*2-1)
			for index, widget := range rowWidgets {
				if index > 0 {
					children = append(children, layout.Rigid(layout.Spacer{Width: gap}.Layout))
				}
				widget := widget
				children = append(children, layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					gtx.Constraints.Min.X = gtx.Constraints.Max.X
					return widget(gtx)
				}))
			}
			for len(rowWidgets) < columns {
				if len(rowWidgets) > 0 {
					children = append(children, layout.Rigid(layout.Spacer{Width: gap}.Layout))
				}
				children = append(children, layout.Flexed(1, layout.Spacer{}.Layout))
				rowWidgets = append(rowWidgets, nil)
			}
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Start}.Layout(gtx, children...)
		})
	}
	return vstack(gtx, gap, rows...)
}
