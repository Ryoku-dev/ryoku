package settings

import (
	"fmt"
	"image"
	"image/color"

	"gioui.org/io/semantic"
	"gioui.org/layout"
	"gioui.org/op/clip"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/protocol"
)

type diskBlock struct {
	loaded, busy           bool
	usage                  protocol.DiskUsage
	refresh, clean, cancel kit.Plaque
	buttons                map[string]*kit.Plaque
	confirm                kit.Dialog
	danger                 kit.Confirm
	list                   layout.List
	selected               protocol.DiskItem
	err                    string
}

func diskSize(bytes uint64) string {
	const gib = 1024 * 1024 * 1024
	if bytes >= gib {
		return fmt.Sprintf("%.1f GB", float64(bytes)/gib)
	}
	return fmt.Sprintf("%.1f MB", float64(bytes)/(1024*1024))
}

func (d *diskBlock) load(e *env.Env, refresh bool) {
	if d.busy {
		return
	}
	d.loaded, d.busy = true, true
	result := new(protocol.DiskUsage)
	e.Call(protocol.MDiskUsage, protocol.DiskUsageParams{Refresh: refresh}, result, func(err error) {
		d.busy = false
		if err != nil {
			d.err = err.Error()
			return
		}
		d.usage, d.err = *result, ""
	})
}

func (p *Page) diskBlocks(gtx layout.Context, e *env.Env, blocks []layout.Widget) []layout.Widget {
	if !e.Feature("diskcare") {
		return blocks
	}
	d, th := &p.disk, e.Theme
	if !d.loaded {
		d.load(e, false)
	}
	if d.buttons == nil {
		d.buttons = make(map[string]*kit.Plaque)
	}
	rows := []layout.Widget{func(gtx layout.Context) layout.Dimensions {
		if d.refresh.Clicked(gtx) {
			d.load(e, true)
		}
		d.refresh.Busy = d.busy
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
							layout.Rigid(look.Paragraph(th, th.T.Row, th.R.Fg, i18n.T("Home filesystem"))),
							layout.Rigid(layout.Spacer{Height: th.Sp.S1}.Layout),
							layout.Rigid(look.Paragraph(th, th.T.Detail, th.R.Fg2, i18n.Tf("%s free of %s", diskSize(d.usage.Free), diskSize(d.usage.Total)))),
						)
					}),
					layout.Rigid(layout.Spacer{Width: th.Sp.S3}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return infraPlaque(gtx, e, &d.refresh, i18n.T("Measure disk use"))
					}),
				)
			}),
			layout.Rigid(layout.Spacer{Height: th.Sp.S3}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions { return d.bar(gtx, e) }),
		)
	}}
	if d.usage.Total > 0 && d.usage.Free < 5*1024*1024*1024 {
		rows = append(rows, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(look.Paragraph(th, th.T.Row, th.R.Fg, i18n.T("Less than 5 GB free"))),
				layout.Rigid(layout.Spacer{Height: th.Sp.S1}.Layout),
				layout.Rigid(look.Paragraph(th, th.T.Detail, th.R.Fg2, i18n.T("Clear old agent data before starting a long task or recording."))),
			)
		})
	}
	for _, item := range d.usage.Items {
		if d.buttons[item.ID] == nil {
			d.buttons[item.ID] = new(kit.Plaque)
		}
		button := d.buttons[item.ID]
		detail := diskSize(uint64(max(0, item.Bytes)))
		if item.ID == "checkpoints" {
			detail += i18n.T(" · Shared with project files, not added to the bar")
		}
		if item.Partial {
			detail += i18n.T(" · Incomplete measurement")
		}
		if !item.Cleanable {
			detail += i18n.T(" · Kept")
		}
		rows = append(rows, func(gtx layout.Context) layout.Dimensions {
			if item.Cleanable && button.Clicked(gtx) {
				d.selected, d.confirm.Open = item, true
				d.list.Axis = layout.Vertical
				d.list.Position.First = 0
			}
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					side := gtx.Dp(10)
					bounds := image.Rect(0, 0, side, side)
					if item.ID == "checkpoints" {
						kit.Stroke(gtx, th.Theme, bounds, gtx.Dp(th.Cut.Token), max(1, gtx.Dp(1)), th.R.Fg3)
					} else {
						kit.Fill(gtx, th.Theme, bounds, gtx.Dp(th.Cut.Token), diskCategoryInk(th, item.ID))
					}
					return layout.Dimensions{Size: image.Pt(side, side)}
				}),
				layout.Rigid(layout.Spacer{Width: th.Sp.S3}.Layout),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
						layout.Rigid(look.Paragraph(th, th.T.Row, th.R.Fg, item.Name)),
						layout.Rigid(layout.Spacer{Height: th.Sp.S1}.Layout),
						layout.Rigid(look.Paragraph(th, th.T.Detail, th.R.Fg2, detail)),
					)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					if !item.Cleanable {
						return layout.Dimensions{}
					}
					return layout.Inset{Left: th.Sp.S3}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return infraPlaque(gtx, e, button, i18n.T("Review cleanup"))
					})
				}),
			)
		})
	}
	return spacedBlocks(th, blocks, func(gtx layout.Context) layout.Dimensions {
		children := []layout.FlexChild{layout.Rigid(look.SectionHead(th, look.Head{Title: i18n.T("Disk")}))}
		if d.err != "" {
			children = append(children, layout.Rigid(layout.Spacer{Height: th.Sp.S1}.Layout), layout.Rigid(look.Paragraph(th, th.T.Detail, th.R.Fg2, d.err)))
		}
		children = append(children, layout.Rigid(layout.Spacer{Height: th.Sp.S2}.Layout), layout.Rigid(func(gtx layout.Context) layout.Dimensions { return kit.SlabRows(gtx, th.Theme, rows...) }))
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
	})
}

func (d *diskBlock) bar(gtx layout.Context, e *env.Env) layout.Dimensions {
	th := e.Theme
	width, height := gtx.Constraints.Max.X, gtx.Dp(18)
	bounds := image.Rect(0, 0, width, height)
	cut := gtx.Dp(th.Cut.Token)
	kit.Fill(gtx, th.Theme, bounds, cut, th.R.Field)
	var total int64
	for _, item := range d.usage.Items {
		if item.ID != "checkpoints" {
			total += max(0, item.Bytes)
		}
	}
	if total > 0 && width > 0 {
		shape := clip.Outline{Path: kit.Shape(gtx, th.Theme, bounds, cut, cut, cut, cut)}.Op().Push(gtx.Ops)
		x := 0
		var used int64
		for _, item := range d.usage.Items {
			if item.ID == "checkpoints" || item.Bytes <= 0 {
				continue
			}
			used += item.Bytes
			next := min(width, int(float64(width)*float64(used)/float64(total)))
			kit.Fill(gtx, th.Theme, image.Rect(x, 0, next, height), 0, diskCategoryInk(th, item.ID))
			x = next
		}
		shape.Pop()
	}
	semantic.LabelOp(i18n.T("Agent disk use by category")).Add(gtx.Ops)
	return layout.Dimensions{Size: image.Pt(width, height)}
}

func diskCategoryInk(th *look.Theme, id string) color.NRGBA {
	// one neutral progression keeps warning, failure and selection inks for their own meanings
	step := 0
	switch id {
	case "dreams":
		step = 1
	case "models":
		step = 2
	case "runtime":
		step = 3
	case "toolhomes", "tasks":
		step = 4
	case "logs":
		step = 5
	case "cache":
		step = 6
	}
	return palette.SRGBAlpha(th.R.Fg, th.R.Field, .26+float32(step)*.11)
}

func (d *diskBlock) overlay(gtx layout.Context, e *env.Env) bool {
	if !e.Feature("diskcare") {
		d.confirm.Open, d.danger.Dialog.Open = false, false
		return false
	}
	if d.danger.Dialog.Open {
		d.danger.Danger = true
		if d.danger.Layout(gtx, e.Theme.Theme, i18n.T("Delete listed files?"),
			i18n.T("Only the files you reviewed will be removed. Your project files and sign-ins stay. Changed files are not removed."),
			i18n.T("Delete listed files"), "") && !d.clean.Busy {
			d.clean.Busy = true
			result := new(protocol.DiskUsage)
			e.Call(protocol.MDiskClean, protocol.DiskCleanParams{Plan: d.selected.Plan}, result, func(err error) {
				d.clean.Busy = false
				if err != nil {
					d.err = err.Error()
					e.Report(err)
				} else {
					d.usage, d.err = *result, ""
				}
			})
		}
		return d.danger.Dialog.Open
	}
	if !d.confirm.Open {
		return false
	}
	if d.cancel.Clicked(gtx) {
		d.confirm.Dismiss(gtx)
	}
	if d.clean.Clicked(gtx) {
		d.confirm.Open, d.danger.Dialog.Open = false, true
	}
	d.confirm.Width = 640
	d.confirm.Layout(gtx, e.Theme.Theme, i18n.T("Delete only these files?"), func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(look.Paragraph(e.Theme, e.Theme.T.Detail, e.Theme.R.Fg2, i18n.T("Only the files listed below will be removed. Your project files and sign-ins stay. Changed files are not removed."))),
			layout.Rigid(layout.Spacer{Height: e.Theme.Sp.S2}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints.Max.Y = min(gtx.Constraints.Max.Y, gtx.Dp(340))
				return d.list.Layout(gtx, len(d.selected.Files), func(gtx layout.Context, index int) layout.Dimensions {
					file := d.selected.Files[index]
					return layout.Inset{Bottom: e.Theme.Sp.S2}.Layout(gtx, look.Paragraph(e.Theme, e.Theme.T.Detail, e.Theme.R.Fg, file.Path+" · "+diskSize(uint64(max(0, file.Bytes)))))
				})
			}),
		)
	}, func(gtx layout.Context) layout.Dimensions {
		d.clean.Kind, d.clean.Small, d.clean.Label = kit.PlaqueTonal, true, i18n.T("Continue to delete")
		return d.clean.Layout(gtx, e.Theme.Theme)
	}, func(gtx layout.Context) layout.Dimensions {
		return infraPlaque(gtx, e, &d.cancel, i18n.T("Keep files"))
	})
	return d.confirm.Open
}
