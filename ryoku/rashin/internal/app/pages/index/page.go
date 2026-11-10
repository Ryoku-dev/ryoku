package index

import (
	"path/filepath"
	"strings"

	"gioui.org/layout"
	"gioui.org/widget"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/protocol"
)

type Page struct {
	selected     string
	showList     bool
	rows         map[string]*look.SummaryRow
	results      []protocol.SearchHit
	resultRows   []look.SummaryRow
	scroll       look.Scroller
	searchScroll look.Scroller
	addField     kit.TextField
	add          kit.Plaque
	refresh      kit.Plaque
	remove       kit.Plaque
	toggle       kit.Plaque
	search       kit.Search
	confirm      kit.Confirm
	confirmID    string
	error        string
	focusSearch  bool
}

func (p *Page) FocusSearch() { p.focusSearch = true }

func (p *Page) Layout(gtx layout.Context, e *env.Env, class kit.FrameClass) layout.Dimensions {
	if p.rows == nil {
		p.rows = make(map[string]*look.SummaryRow)
	}
	if p.toggle.Clicked(gtx) {
		p.showList = !p.showList
	}
	indexes := e.Snapshot().Indexes
	if p.selected == "" && len(indexes) > 0 {
		p.selected = indexes[0].ID
	}
	if class == kit.FrameExpanded {
		return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints.Min.X, gtx.Constraints.Max.X = gtx.Dp(384), gtx.Dp(384)
				return p.list(gtx, e, indexes)
			}),
			layout.Rigid(layout.Spacer{Width: 8}.Layout),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions { return p.work(gtx, e, indexes, class) }),
		)
	}
	if p.showList || p.selected == "" {
		return p.list(gtx, e, indexes)
	}
	return p.work(gtx, e, indexes, class)
}

func (p *Page) Overlay(gtx layout.Context, e *env.Env) layout.Dimensions {
	if p.confirmID == "" {
		return layout.Dimensions{}
	}
	name := p.confirmID
	for _, info := range e.Snapshot().Indexes {
		if info.ID == p.confirmID {
			name = info.Name
		}
	}
	p.confirm.Danger = true
	p.confirm.Dialog.Open = true
	if p.confirm.Layout(gtx, e.Theme.Theme, i18n.T("Remove index"), i18n.Tf("This removes %s and its saved index data.", name), i18n.T("Remove"), "") {
		id := p.confirmID
		p.confirmID = ""
		e.Call(protocol.MIndexRemove, protocol.IndexRef{ID: id}, new(protocol.Empty), func(err error) {
			if err != nil {
				p.error = err.Error()
			} else if p.selected == id {
				p.selected = ""
			}
		})
	}
	if p.confirm.Closed() {
		p.confirmID = ""
	}
	return layout.Dimensions{Size: gtx.Constraints.Max}
}

func (p *Page) list(gtx layout.Context, e *env.Env, indexes []protocol.IndexInfo) layout.Dimensions {
	return (kit.Pane{Kind: kit.PaneList}).Layout(gtx, e.Theme.Theme, func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Top: 21, Right: 10, Bottom: 13, Left: 10}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(kit.Text(e.Theme.Theme, e.Theme.T.Section, e.Theme.R.Fg, i18n.T("Indexes"))),
				layout.Rigid(layout.Spacer{Height: 13}.Layout),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return p.scroll.Layout(gtx, e.Theme, len(indexes), func(gtx layout.Context, index int) layout.Dimensions {
						info := indexes[index]
						row := p.rows[info.ID]
						if row == nil {
							row = new(look.SummaryRow)
							p.rows[info.ID] = row
						}
						if row.Clicked(gtx) || row.DoubleClicked(gtx) {
							p.selected, p.showList = info.ID, false
						}
						row.Title = info.Name
						row.State = indexState(info.State)
						row.Meta = info.Root
						row.Excerpt = i18n.Tf("%d files, %d symbols", info.Files, info.Symbols)
						row.Tone = indexTone(info.State)
						return layout.Inset{Bottom: 4}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return row.Layout(gtx, e.Theme, info.ID == p.selected)
						})
					})
				}),
			)
		})
	})
}

func (p *Page) work(gtx layout.Context, e *env.Env, indexes []protocol.IndexInfo, class kit.FrameClass) layout.Dimensions {
	return (kit.Pane{Kind: kit.PaneWork}).Layout(gtx, e.Theme.Theme, func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Top: 21, Right: 34, Bottom: 13, Left: 34}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
						layout.Flexed(1, kit.Text(e.Theme.Theme, e.Theme.T.Title, e.Theme.R.Fg, i18n.T("Code index"))),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							if class == kit.FrameExpanded {
								return layout.Dimensions{}
							}
							p.toggle.Kind, p.toggle.Label = kit.PlaqueQuiet, i18n.T("Indexes")
							return p.toggle.Layout(gtx, e.Theme.Theme)
						}),
					)
				}),
				layout.Rigid(layout.Spacer{Height: 13}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions { return p.addRow(gtx, e) }),
				layout.Rigid(layout.Spacer{Height: 21}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions { return p.searchRow(gtx, e) }),
				layout.Rigid(layout.Spacer{Height: 13}.Layout),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions { return p.resultsList(gtx, e, indexes) }),
			)
		})
	})
}

func (p *Page) addRow(gtx layout.Context, e *env.Env) layout.Dimensions {
	p.addField.Label, p.addField.Placeholder = i18n.T("Folder"), i18n.T("Choose a project folder")
	p.add.Kind, p.add.Label, p.add.Small = kit.PlaqueGold, i18n.T("Add"), true
	if p.add.Clicked(gtx) && strings.TrimSpace(p.addField.Text()) != "" {
		var info protocol.IndexInfo
		e.Call(protocol.MIndexAdd, protocol.IndexAddParams{Path: strings.TrimSpace(p.addField.Text())}, &info, func(err error) {
			if err != nil {
				p.error = err.Error()
				return
			}
			p.selected = info.ID
			p.addField.SetText("")
		})
	}
	p.refresh.Kind, p.refresh.Label, p.refresh.Small = kit.PlaqueLine, i18n.T("Refresh"), true
	p.remove.Kind, p.remove.Label, p.remove.Small = kit.PlaqueDangerLine, i18n.T("Remove"), true
	if p.refresh.Clicked(gtx) && p.selected != "" {
		var info protocol.IndexInfo
		e.Call(protocol.MIndexRefresh, protocol.IndexRef{ID: p.selected}, &info, nil)
	}
	if p.remove.Clicked(gtx) && p.selected != "" {
		p.confirmID = p.selected
	}
	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.End}.Layout(gtx,
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions { return p.addField.Layout(gtx, e.Theme.Theme) }),
		layout.Rigid(layout.Spacer{Width: 8}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions { return p.add.Layout(gtx, e.Theme.Theme) }),
		layout.Rigid(layout.Spacer{Width: 8}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions { return p.refresh.Layout(gtx, e.Theme.Theme) }),
		layout.Rigid(layout.Spacer{Width: 8}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions { return p.remove.Layout(gtx, e.Theme.Theme) }),
	)
}

func (p *Page) searchRow(gtx layout.Context, e *env.Env) layout.Dimensions {
	p.search.Placeholder, p.search.Hint = i18n.T("Search indexed code"), i18n.T("Enter")
	if p.focusSearch {
		p.focusSearch = false
		p.search.Focus(gtx)
	}
	p.search.Editor.Submit = true
	dimensions := p.search.Layout(gtx, e.Theme.Theme)
	for {
		raw, ok := p.search.Update(gtx)
		if !ok {
			break
		}
		if submit, ok := raw.(widget.SubmitEvent); ok {
			p.runSearch(e, strings.TrimSpace(submit.Text))
		}
	}
	return dimensions
}

func (p *Page) runSearch(e *env.Env, query string) {
	if query == "" {
		p.results = nil
		return
	}
	var result protocol.SearchResult
	e.Call(protocol.MIndexSearch, protocol.SearchParams{IndexRef: protocol.IndexRef{ID: p.selected}, Query: query, Limit: 40, Mode: protocol.SearchHybrid}, &result, func(err error) {
		if err != nil {
			p.error = err.Error()
			return
		}
		p.results = result.Hits
		p.resultRows = make([]look.SummaryRow, len(result.Hits))
	})
}

func (p *Page) resultsList(gtx layout.Context, e *env.Env, indexes []protocol.IndexInfo) layout.Dimensions {
	if len(p.results) == 0 {
		body := i18n.T("Search an index to open a matching file.")
		if p.error != "" {
			body = p.error
		}
		return (kit.Empty{Title: i18n.T("No search results"), Body: body}).Layout(gtx, e.Theme.Theme)
	}
	root := ""
	for _, info := range indexes {
		if info.ID == p.selected {
			root = info.Root
		}
	}
	return p.searchScroll.Layout(gtx, e.Theme, len(p.results), func(gtx layout.Context, index int) layout.Dimensions {
		hit := p.results[index]
		row := &p.resultRows[index]
		if row.Clicked(gtx) || row.DoubleClicked(gtx) {
			env.Spawn("xdg-open", filepath.Join(root, hit.Path))
		}
		row.Title = hit.Path
		row.State = i18n.Tf("Lines %d to %d", hit.Start, hit.End)
		row.Meta = hit.Symbol
		row.Excerpt = hit.Snippet
		row.Tone = palette.Alive
		return layout.Inset{Bottom: 4}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return row.Layout(gtx, e.Theme, false)
		})
	})
}

func indexTone(state string) palette.Tone {
	switch state {
	case protocol.IndexIndexing:
		return palette.Becoming
	case protocol.IndexError:
		return palette.Failed
	case protocol.IndexReady:
		return palette.Alive
	default:
		return palette.Idle
	}
}

func indexState(state string) string {
	switch state {
	case protocol.IndexIndexing:
		return i18n.T("Indexing")
	case protocol.IndexError:
		return i18n.T("Failed")
	case protocol.IndexReady:
		return i18n.T("Ready")
	default:
		return i18n.T("Empty")
	}
}
