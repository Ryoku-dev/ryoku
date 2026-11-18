package settings

import (
	"fmt"
	"image"
	"image/color"

	"gioui.org/io/semantic"
	"gioui.org/layout"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
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
						kit.StrokeCut(gtx.Ops, bounds, gtx.Dp(2), max(1, gtx.Dp(1)), th.R.Fg3)
					} else {
						kit.FillCut(gtx.Ops, bounds, gtx.Dp(2), diskCategoryInk(th, item.ID))
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
		children := []layout.FlexChild{layout.Rigid(kit.Text(th.Theme, th.T.Title, th.R.Fg, i18n.T("Disk")))}
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
	kit.FillCut(gtx.Ops, bounds, gtx.Dp(th.Cut.Small), th.R.Stone4)
	var total int64
	for _, item := range d.usage.Items {
		if item.ID != "checkpoints" {
			total += max(0, item.Bytes)
		}
	}
	x := 0
	for _, item := range d.usage.Items {
		if item.ID == "checkpoints" {
			continue
		}
		if total == 0 {
			break
		}
		next := min(width, x+int(float64(width)*float64(max(0, item.Bytes))/float64(total)))
		ink := diskCategoryInk(th, item.ID)
		kit.FillCut(gtx.Ops, image.Rect(x, 0, max(x, next-2), height), min(gtx.Dp(th.Cut.Small), max(0, (next-x-2)/2)), ink)
		x = next
	}
	semantic.LabelOp(i18n.T("Agent disk use by category")).Add(gtx.Ops)
	return layout.Dimensions{Size: image.Pt(width, height)}
}

func diskCategoryInk(th *look.Theme, id string) color.NRGBA {
	switch id {
	case "teams":
		return th.R.Jade
	case "dreams":
		return th.R.Lapis
	case "harness":
		return th.R.Amber
	case "logs":
		return th.R.Jasper
	case "cache":
		return th.R.Fg2
	case "tasks":
		return th.R.Sel
	default:
		return th.R.Fg3
	}
}

func (d *diskBlock) overlay(gtx layout.Context, e *env.Env) bool {
	if !e.Feature("diskcare") {
		d.confirm.Open = false
		return false
	}
	if !d.confirm.Open {
		return false
	}
	if d.cancel.Clicked(gtx) {
		d.confirm.Dismiss(gtx)
	}
	if d.clean.Clicked(gtx) && !d.clean.Busy {
		d.clean.Busy = true
		result := new(protocol.DiskUsage)
		e.Call(protocol.MDiskClean, protocol.DiskCleanParams{Plan: d.selected.Plan}, result, func(err error) {
			d.clean.Busy = false
			if err != nil {
				d.err = err.Error()
				e.Report(err)
			} else {
				d.usage = *result
				d.err = ""
			}
			d.confirm.Open = false
		})
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
		return infraPlaque(gtx, e, &d.clean, i18n.T("Delete listed files"))
	}, func(gtx layout.Context) layout.Dimensions {
		return infraPlaque(gtx, e, &d.cancel, i18n.T("Keep files"))
	})
	return d.confirm.Open
}
