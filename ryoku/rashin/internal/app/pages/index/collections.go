package index

import (
	"image"
	"strings"
	"time"

	"gioui.org/layout"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"
	"ryoku-i18n"
	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/protocol"
)

type collectionsPanel struct {
	loaded, loading bool
	selected        string
	list            []protocol.Collection
	items           []protocol.CollectionItem
	next            string
	total           int
	paging          bool
	more            kit.Plaque
	rows            map[string]*collectionRow
	header          look.PaneHeader
	scroll          look.Scroller
	reveal          look.Reveal
	sheet           look.Sheet
	chat            kit.Plaque
	picker          look.Select
	dialog          kit.Dialog
	name            kit.TextField
	editing         string
	save, cancel    kit.Plaque
}

type collectionRow struct {
	check, remove, open, expand  kit.Plaque
	checking, fetching, expanded bool
	full                         string
}

func (p *collectionsPanel) ensure(e *env.Env) {
	if p.loaded || p.loading {
		return
	}
	p.loading = true
	var result protocol.CollectionsListResult
	e.Call(protocol.MCollectionsList, protocol.CollectionsListParams{}, &result, func(err error) {
		p.loading, p.loaded = false, true
		if err != nil {
			e.Notify(err.Error())
			return
		}
		p.list = result.Collections
	})
}

func (p *collectionsPanel) load(e *env.Env) {
	id := p.selected
	p.items, p.rows = nil, make(map[string]*collectionRow)
	p.next, p.total, p.paging = "", 0, false
	p.scroll.ScrollTo(0)
	var result protocol.CollectionItemsResult
	e.Call(protocol.MCollectionsItems, protocol.CollectionItemsParams{Collection: id}, &result, func(err error) {
		if id != p.selected {
			return
		}
		if err != nil {
			e.Notify(err.Error())
			return
		}
		p.items, p.next, p.total = result.Items, result.Next, result.Total
	})
}

func (p *collectionsPanel) loadMore(e *env.Env) {
	if p.paging || p.next == "" {
		return
	}
	id, cursor := p.selected, p.next
	p.paging = true
	var result protocol.CollectionItemsResult
	e.Call(protocol.MCollectionsItems, protocol.CollectionItemsParams{Collection: id, Cursor: cursor}, &result, func(err error) {
		if id != p.selected {
			return
		}
		p.paging = false
		if err != nil {
			e.Notify(err.Error())
			return
		}
		p.items = append(p.items, result.Items...)
		p.next, p.total = result.Next, result.Total
	})
}

func (p *collectionsPanel) edit(id, name string) {
	p.editing = id
	p.name.SetText(name)
	p.name.Editor.SingleLine, p.name.Editor.MaxLen = true, 100
	p.dialog.Open = true
}

func (p *collectionsPanel) overlay(gtx layout.Context, e *env.Env) {
	if !p.dialog.Open {
		return
	}
	p.save.Kind, p.save.Label = kit.PlaqueGold, i18n.T("Save collection")
	p.cancel.Kind, p.cancel.Label = kit.PlaqueQuiet, i18n.T("Cancel")
	if p.cancel.Clicked(gtx) {
		p.dialog.Dismiss(gtx)
	}
	if p.save.Clicked(gtx) && strings.TrimSpace(p.name.Text()) != "" && !p.save.Busy {
		if p.editing == ":file" {
			p.addFile(e, strings.TrimSpace(p.name.Text()))
			p.dialog.Dismiss(gtx)
		} else {
			p.save.Busy = true
			var result protocol.Collection
			e.Call(protocol.MCollectionsSave, protocol.CollectionSaveParams{ID: p.editing, Name: p.name.Text()}, &result, func(err error) {
				p.save.Busy = false
				if err != nil {
					e.Notify(err.Error())
					return
				}
				p.dialog.Open = false
				p.selected, p.loaded = result.ID, false
				p.ensure(e)
				p.load(e)
			})
		}
	}
	title := i18n.T("New collection")
	if p.editing != "" {
		title = i18n.T("Rename collection")
	}
	label := i18n.T("Collection name")
	if p.editing == ":file" {
		title, label, p.save.Label = i18n.T("Add a file"), i18n.T("File path"), i18n.T("Save file")
	}
	p.dialog.Layout(gtx, e.Theme.Theme, title,
		func(gtx layout.Context) layout.Dimensions {
			return look.Named(gtx, label, func(gtx layout.Context) layout.Dimensions { return p.name.Layout(gtx, e.Theme.Theme) })
		},
		func(gtx layout.Context) layout.Dimensions { return p.cancel.Layout(gtx, e.Theme.Theme) },
		func(gtx layout.Context) layout.Dimensions { return p.save.Layout(gtx, e.Theme.Theme) })
}

func (p *collectionsPanel) addFile(e *env.Env, path string) {
	var result protocol.CollectionItem
	e.Call(protocol.MCollectionsAdd, protocol.CollectionAddParams{Collection: p.selected, Kind: "file", Path: path}, &result, func(err error) {
		if err != nil {
			e.Notify(err.Error())
			return
		}
		p.loaded = false
		p.ensure(e)
		p.load(e)
	})
}

func (p *collectionsPanel) layout(gtx layout.Context, e *env.Env) layout.Dimensions {
	th := e.Theme
	name := i18n.T("Saved")
	for _, item := range p.list {
		if item.ID == p.selected {
			name = item.Name
		}
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			d := p.header.Layout(gtx, th, e.Overlays, look.PaneHeaderContent{Title: name, Facts: []look.HeaderFact{{Text: i18n.Tf("%d saved items", p.total)}}, Actions: []look.HeaderAction{{Icon: look.Icon.Attach, Description: i18n.T("Add files")}}})
			if _, clicked := p.header.ActionClicked(); clicked {
				e.ChooseFiles(i18n.T("Add files to collection"), func(paths []string) {
					for _, path := range paths {
						p.addFile(e, path)
					}
				}, func() { p.edit(":file", ""); p.name.Editor.MaxLen = 4096 })
			}
			return d
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return look.ReadingColumn(gtx, e.Class, func(gtx layout.Context) layout.Dimensions {
				return p.controls(gtx, e, name)
			})
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			if len(p.items) == 0 {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						return kit.Empty{Icon: look.Icon.Index, Title: i18n.T("Keep what matters"), Body: i18n.T("Save an answer or excerpt from Theia to find it here.")}.Layout(gtx, th.Theme)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return p.sheet.Colophon(gtx, th, look.PageFor(protocol.TabIndex))
					}),
				)
			}
			count := len(p.items) + 1
			if p.next != "" {
				count++
			}
			return p.scroll.Layout(gtx, th, count, func(gtx layout.Context, index int) layout.Dimensions {
				return look.ReadingColumn(gtx, e.Class, func(gtx layout.Context) layout.Dimensions {
					if index == count-1 {
						return p.sheet.Colophon(gtx, th, look.PageFor(protocol.TabIndex))
					}
					if index == len(p.items) {
						p.more.Kind, p.more.Label, p.more.Busy = kit.PlaqueTonal, i18n.Tf("Load more, %d of %d items shown", len(p.items), p.total), p.paging
						if p.more.Clicked(gtx) {
							p.loadMore(e)
						}
						return p.more.Layout(gtx, th.Theme)
					}
					return p.item(gtx, e, p.items[index])
				})
			})
		}),
	)
}

func (p *collectionsPanel) controls(gtx layout.Context, e *env.Env, name string) layout.Dimensions {
	th := e.Theme
	labels, selected := make([]string, len(p.list)), 0
	for i, item := range p.list {
		labels[i] = item.Name
		if item.ID == p.selected {
			selected = i
		}
	}
	p.chat.Kind, p.chat.Label = kit.PlaqueGold, i18n.T("Chat with this collection")
	if p.chat.Clicked(gtx) && len(p.items) > 0 && !p.chat.Busy {
		p.chat.Busy = true
		var result protocol.AskResult
		e.Call(protocol.MCollectionsChat, protocol.CollectionChatParams{Collection: p.selected}, &result, func(err error) {
			p.chat.Busy = false
			if err != nil {
				e.Notify(err.Error())
				return
			}
			e.Attach(result.Session.ID, func(err error) {
				if err != nil {
					e.Notify(err.Error())
					return
				}
				e.Navigate(protocol.TabAsk, result.Session.ID)
			})
		})
	}
	selectRow := look.Setting{
		Title: i18n.T("Collection"), Detail: i18n.T("Choose the items you want to find or ask about."),
		Control: func(gtx layout.Context) layout.Dimensions {
			return look.Named(gtx, i18n.T("Choose collection"), func(gtx layout.Context) layout.Dimensions {
				return p.picker.Layout(gtx, th, e.Overlays, labels, selected)
			})
		},
	}
	chatRow := look.Setting{
		Title: i18n.T("Ask about these items"), Detail: i18n.T("Start a conversation with this collection as context."),
		Control: func(gtx layout.Context) layout.Dimensions {
			if len(p.items) == 0 {
				gtx = gtx.Disabled()
			}
			return p.chat.Layout(gtx, th.Theme)
		},
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: th.Sp.S3, Bottom: th.Sp.S4}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return look.SpecStrip(gtx, th, []look.Spec{
					{Label: i18n.T("Collections"), Value: kit.Pad2(len(p.list))},
					{Label: i18n.T("Items"), Value: kit.Pad2(p.total)},
				})
			})
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return p.reveal.Group(gtx, th, 0, func(gtx layout.Context) layout.Dimensions {
				return look.SettingsSection(gtx, th, look.Head{Title: i18n.T("Saved items"), Number: 1, Of: 2}, []look.Setting{selectRow, chatRow})
			})
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return p.reveal.Group(gtx, th, 1, func(gtx layout.Context) layout.Dimensions {
				return look.SettingsSection(gtx, th, look.Head{Title: i18n.T("Organize"), Number: 2, Of: 2}, []look.Setting{
					{Title: i18n.T("New collection"), Detail: i18n.T("Make a new place to keep related items."), Activate: func() { p.edit("", "") }},
					{Title: i18n.T("Rename"), Detail: i18n.T("Change this collection's name."), Activate: func() { p.edit(p.selected, name) }},
					{Title: i18n.T("Folders"), Detail: i18n.T("Return to indexed folders."), Activate: func() {
						p.selected = ""
						e.Navigate(protocol.TabIndex, "")
					}},
				})
			})
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if at, changed := p.picker.Changed(); changed && at >= 0 && at < len(p.list) {
				p.selected = p.list[at].ID
				p.load(e)
				e.Navigate(protocol.TabIndex, "collections/"+p.selected)
			}
			return layout.Dimensions{}
		}),
	)
}

func (p *collectionsPanel) item(gtx layout.Context, e *env.Env, item protocol.CollectionItem) layout.Dimensions {
	th := e.Theme
	row := p.rows[item.ID]
	if row == nil {
		row = &collectionRow{}
		p.rows[item.ID] = row
	}
	row.check.Kind, row.check.Small, row.check.Label = kit.PlaqueTonal, true, i18n.T("Check again")
	row.check.Busy = row.checking
	row.remove.Kind, row.remove.Small, row.remove.Label = kit.PlaqueQuiet, true, i18n.T("Remove")
	row.open.Kind, row.open.Small, row.open.Label = kit.PlaqueTonal, true, i18n.T("Open saved file")
	row.expand.Kind, row.expand.Small, row.expand.Label, row.expand.Busy = kit.PlaqueTonal, true, i18n.T("Read full item"), row.fetching
	if row.expanded {
		row.expand.Label = i18n.T("Show preview")
	}
	if row.expand.Clicked(gtx) {
		if row.expanded {
			row.expanded = false
		} else if row.full != "" {
			row.expanded = true
		} else {
			row.fetching = true
			var result protocol.CollectionItemsResult
			e.Call(protocol.MCollectionsItems, protocol.CollectionItemsParams{Collection: item.Collection, ID: item.ID}, &result, func(err error) {
				row.fetching = false
				if err != nil {
					e.Notify(err.Error())
					return
				}
				if result.Item != nil {
					row.full, row.expanded = result.Item.Text, true
				}
			})
		}
	}
	if row.check.Clicked(gtx) && !row.checking {
		row.checking = true
		var result protocol.CollectionItem
		e.Call(protocol.MCollectionsCheck, protocol.CollectionCheckParams{Collection: item.Collection, ID: item.ID}, &result, func(err error) {
			row.checking = false
			if err != nil {
				e.Notify(err.Error())
				return
			}
			for i := range p.items {
				if p.items[i].ID == result.ID {
					p.items[i].Changed, p.items[i].Checked = result.Changed, result.Checked
				}
			}
			if !result.Changed {
				e.Notify(i18n.T("The saved content is still there"))
			}
		})
	}
	if row.remove.Clicked(gtx) {
		e.Call(protocol.MCollectionsRemove, protocol.CollectionRemoveParams{Collection: item.Collection, ID: item.ID}, new(protocol.Empty), func(err error) {
			if err != nil {
				e.Notify(err.Error())
				return
			}
			p.loaded = false
			p.ensure(e)
			p.load(e)
		})
	}
	if row.open.Clicked(gtx) {
		path := item.Path
		if path == "" {
			path = item.Image
		}
		e.OpenPath(path)
	}
	date := item.Saved
	if parsed, err := time.Parse(time.RFC3339, item.Saved); err == nil {
		date = parsed.Local().Format("2 Jan 2006")
	}
	source := item.Source
	if item.URL != "" {
		source = item.URL
	}
	icon := look.Icon.File
	switch item.Kind {
	case "capture":
		icon = look.Icon.Screen
	case "url":
		icon = look.Icon.Link
	case "answer":
		icon = look.Icon.Ask
	case "excerpt":
		icon = look.Icon.Quote
	}
	return layout.Inset{Bottom: th.Sp.S4}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					gtx.Constraints = layout.Exact(image.Pt(gtx.Dp(20), gtx.Dp(20)))
					return icon.Layout(gtx, th.R.Fg2)
				}), layout.Rigid(layout.Spacer{Width: th.Sp.S2}.Layout), layout.Flexed(1, kit.Text(th.Theme, th.T.Title, th.R.Fg, item.Title)))
			}),
			layout.Rigid(layout.Spacer{Height: th.Sp.S1}.Layout),
			layout.Rigid(look.Paragraph(th, th.T.Detail, th.R.Fg2, i18n.Tf("%s · %s · %s", item.Kind, source, date))),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if !item.Changed {
					return layout.Dimensions{}
				}
				return kit.Claim(th.Theme, palette.Becoming, i18n.T("Changed since you saved it"))(gtx)
			}),
			layout.Rigid(layout.Spacer{Height: th.Sp.S2}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				text := item.Text
				if row.expanded {
					text = row.full
				}
				return look.Paragraph(th, th.T.Detail, th.R.Fg, text)(gtx)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if !item.Truncated {
					return layout.Dimensions{}
				}
				return row.expand.Layout(gtx, th.Theme)
			}),
			layout.Rigid(layout.Spacer{Height: th.Sp.S2}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						if item.URL == "" && !strings.HasPrefix(item.Source, "/") {
							gtx = gtx.Disabled()
						}
						return row.check.Layout(gtx, th.Theme)
					}),
					layout.Rigid(layout.Spacer{Width: th.Sp.S2}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						if item.Path == "" && item.Image == "" {
							return layout.Dimensions{}
						}
						return row.open.Layout(gtx, th.Theme)
					}),
					layout.Rigid(layout.Spacer{Width: th.Sp.S2}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions { return row.remove.Layout(gtx, th.Theme) }))
			}),
		)
	})
}
