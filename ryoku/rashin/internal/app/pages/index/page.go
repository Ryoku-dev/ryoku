// Package index is the Index destination: the folders Rashin has read, and
// search across what it found in them.
package index

import (
	"fmt"
	"image"
	"path/filepath"
	"strings"
	"time"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/widget"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/anim"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/protocol"
)

type Page struct {
	selected string

	list    look.SideList
	rowMenu kit.Menu
	header  look.PaneHeader
	rise    look.Rise

	search  kit.Search
	modes   kit.Filters
	lift    anim.Tween
	scroll  look.Scroller
	query   string
	asked   string
	loading bool
	hits    []protocol.SearchHit
	rows    map[int]*hitRow
	open    int
	focus   bool

	add    addDialog
	remove removeDialog
	start  kit.Plaque
}

type hitRow struct {
	click  kit.Click
	hover  look.Hover
	unfold look.Unfold
	file   kit.Plaque
}

var searchModes = [...]string{protocol.SearchHybrid, protocol.SearchLexical, protocol.SearchSemantic}

func New() *Page { return &Page{open: -1} }

func (p *Page) New(e *env.Env) { p.add.ask(e, p) }

func (p *Page) NewLabel() string { return i18n.T("Add folder") }

func (p *Page) Overlay(gtx layout.Context, e *env.Env) bool {
	p.add.layout(gtx, e, p)
	p.remove.layout(gtx, e, p)
	return look.AnyOpen(&p.add.dialog, &p.remove.confirm.Dialog)
}

func (p *Page) selectedIndex(e *env.Env) (protocol.IndexInfo, bool) {
	indexes := e.Snapshot().Indexes
	for _, info := range indexes {
		if info.ID == p.selected {
			return info, true
		}
	}
	if len(indexes) > 0 {
		p.selected = indexes[0].ID
		return indexes[0], true
	}
	return protocol.IndexInfo{}, false
}

func (p *Page) List(gtx layout.Context, e *env.Env) look.ListDimensions {
	snapshot := e.Snapshot()
	selected, _ := p.selectedIndex(e)
	rows := make([]look.SideRow, 0, len(snapshot.Indexes))
	for _, info := range snapshot.Indexes {
		row := look.SideRow{ID: info.ID, Title: info.Name, Facts: i18n.Tf("%s files", grouped(info.Files)), Menu: true}
		switch info.State {
		case protocol.IndexIndexing:
			row.Working = true
			if progress, ok := snapshot.Progress[info.ID]; ok && progress.Total > 0 {
				row.Facts = i18n.Tf("%s of %s files", grouped(progress.Done), grouped(progress.Total))
			}
		case protocol.IndexError:
			row.Tone = palette.Failed
		}
		rows = append(rows, row)
	}
	p.list.Empty = i18n.T("Folders Rashin has read are listed here.")
	dimensions := p.list.Layout(gtx, e.Theme, e.Overlays, []look.SideGroup{{Title: i18n.T("Indexes"), Rows: rows}}, selected.ID)
	for _, event := range p.list.Events() {
		switch event.Kind {
		case look.SideOpen:
			if p.selected != event.ID {
				p.selected, p.hits, p.asked, p.open = event.ID, nil, "", -1
				p.runSearch(e)
			}
			p.focus = true
			e.Navigate(protocol.TabIndex, "")
		case look.SideMenu:
			p.openRowMenu(e, event.ID, event.Anchor)
		}
	}
	return dimensions
}

func (p *Page) openRowMenu(e *env.Env, id string, anchor image.Rectangle) {
	items := []kit.MenuItem{
		{Icon: look.Icon.Retry, Label: i18n.T("Read it again")},
		{Icon: look.Icon.Folder, Label: i18n.T("Open folder")},
		{Divider: true},
		{Icon: look.Icon.Delete, Label: i18n.T("Remove"), Danger: true},
	}
	tag := &p.rowMenu
	e.Overlays.Open(tag, anchor, look.Placement{}, func(gtx layout.Context) layout.Dimensions {
		dimensions := p.rowMenu.Layout(gtx, e.Theme.Theme, items)
		item, ok := p.rowMenu.Clicked(gtx)
		if !ok {
			return dimensions
		}
		e.Overlays.Close(tag)
		for _, info := range e.Snapshot().Indexes {
			if info.ID != id {
				continue
			}
			switch item {
			case 0:
				refresh(e, id)
			case 1:
				e.OpenFolder(info.Root)
			case 3:
				p.remove.open(info)
			}
		}
		return dimensions
	})
}

func refresh(e *env.Env, id string) {
	e.Call(protocol.MIndexRefresh, protocol.IndexRef{ID: id}, new(protocol.IndexInfo), func(err error) {
		if err != nil {
			e.Notify(err.Error())
		}
	})
}

func (p *Page) Content(gtx layout.Context, e *env.Env) layout.Dimensions {
	info, ok := p.selectedIndex(e)
	if !ok {
		return p.layoutEmpty(gtx, e)
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions { return p.layoutHeader(gtx, e, info) }),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return p.rise.Layout(gtx, e.Theme, info.ID, 8, func(gtx layout.Context) layout.Dimensions {
				return p.layoutBody(gtx, e, info)
			})
		}),
	)
}

func (p *Page) layoutEmpty(gtx layout.Context, e *env.Env) layout.Dimensions {
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return p.header.Layout(gtx, e.Theme, e.Overlays, look.PaneHeaderContent{Title: i18n.T("Index")})
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			p.start.Kind, p.start.Label = kit.PlaqueGold, i18n.T("Add folder")
			if p.start.Clicked(gtx) {
				p.add.ask(e, p)
			}
			return kit.Empty{
				Icon: look.Icon.Index, Title: i18n.T("Nothing is indexed yet"),
				Body:    i18n.T("Add a folder and Rashin reads its files, so you and the agents can search it by words or by meaning."),
				Actions: []*kit.Plaque{&p.start},
			}.Layout(gtx, e.Theme.Theme)
		}),
	)
}

func (p *Page) layoutHeader(gtx layout.Context, e *env.Env, info protocol.IndexInfo) layout.Dimensions {
	content := look.PaneHeaderContent{
		Title: info.Name, Scrolled: p.scroll.Position.First > 0 || p.scroll.Position.Offset > 0,
		Facts:   indexFacts(gtx.Now, info),
		Actions: []look.HeaderAction{{Icon: look.Icon.Retry, Description: i18n.T("Read it again"), Disabled: info.State == protocol.IndexIndexing}},
		Menu: []kit.MenuItem{
			{Icon: look.Icon.Folder, Label: i18n.T("Open folder")},
			{Divider: true},
			{Icon: look.Icon.Delete, Label: i18n.T("Remove"), Danger: true},
		},
	}
	switch info.State {
	case protocol.IndexIndexing:
		content.Claim, content.ClaimTone = i18n.T("Reading"), palette.Becoming
	case protocol.IndexError:
		content.Claim, content.ClaimTone = i18n.T("Failed"), palette.Failed
	}
	dimensions := p.header.Layout(gtx, e.Theme, e.Overlays, content)
	if _, ok := p.header.ActionClicked(); ok {
		refresh(e, info.ID)
	}
	if item, ok := p.header.MenuClicked(); ok {
		if item == 0 {
			e.OpenFolder(info.Root)
		} else {
			p.remove.open(info)
		}
	}
	return dimensions
}

func indexFacts(now time.Time, info protocol.IndexInfo) []look.HeaderFact {
	facts := make([]look.HeaderFact, 0, 4)
	if info.Root != "" {
		facts = append(facts, look.HeaderFact{Text: paths.Short(info.Root), Kind: look.FactToken})
	}
	facts = append(facts, look.HeaderFact{Text: i18n.Tf("%s files", grouped(info.Files))})
	if info.Symbols > 0 {
		facts = append(facts, look.HeaderFact{Text: i18n.Tf("%s symbols", grouped(info.Symbols))})
	}
	if info.Updated != nil {
		facts = append(facts, look.HeaderFact{Text: i18n.Tf("read %s", look.Ago(now, *info.Updated))})
	}
	return facts
}

// matches a new ask so the two homes line up, results use the reading column
const homeColumn = 680

func (p *Page) layoutBody(gtx layout.Context, e *env.Env, info protocol.IndexInfo) layout.Dimensions {
	th := e.Theme
	size := gtx.Constraints.Max
	// the sentence leaves under the header's edge, not over the header
	defer clip.Rect(image.Rectangle{Max: size}).Push(gtx.Ops).Pop()
	inset := gtx.Dp(look.ContentInset(e.Class))
	column := min(size.X-2*inset, gtx.Dp(look.ReadingColumnWidth))
	home := min(size.X-2*inset, gtx.Dp(homeColumn))
	lift := look.Transition(gtx, &p.lift, p.asked != "", th.Motion.Select, th.Motion.Select)

	width := home + int(float32(column-home)*lift+.5)
	record := op.Record(gtx.Ops)
	block := gtx
	block.Constraints = layout.Constraints{Min: image.Pt(width, 0), Max: image.Pt(width, size.Y)}
	search := p.layoutSearch(block, e, info)
	searchCall := record.Stop()

	top := gtx.Dp(21)
	lead := gtx.Sp(th.T.Sentence.Line) + gtx.Dp(8) + gtx.Sp(th.T.Proof.Line) + gtx.Dp(21)
	centre := min(max(size.Y*42/100, lead+top), max(lead, size.Y-search.Size.Y-top))
	y := centre + int(float32(top-centre)*lift+.5)
	x := (size.X - width) / 2

	if lift > 0 {
		p.layoutResults(gtx, e, info, image.Rect(0, y+search.Size.Y, size.X, size.Y), lift)
	}
	if lift < 1 {
		p.layoutHome(gtx, e, image.Rect(x, y, x+width, y+search.Size.Y), 1-lift)
	}
	offset := op.Offset(image.Pt(x, y)).Push(gtx.Ops)
	searchCall.Add(gtx.Ops)
	offset.Pop()
	return layout.Dimensions{Size: size}
}

func (p *Page) layoutHome(gtx layout.Context, e *env.Env, field image.Rectangle, alpha float32) {
	th := e.Theme
	opacity := paint.PushOpacity(gtx.Ops, alpha)
	defer opacity.Pop()
	proof, sentence := gtx.Sp(th.T.Proof.Line), gtx.Sp(th.T.Sentence.Line)
	proofTop := field.Min.Y - gtx.Dp(21) - proof
	sentenceTop := proofTop - gtx.Dp(8) - sentence
	look.Place(gtx, image.Rect(field.Min.X, sentenceTop, field.Max.X, sentenceTop+sentence),
		kit.Text(th.Theme, th.T.Sentence, th.R.Fg, i18n.T("Search what Rashin has read")))
	look.Place(gtx, image.Rect(field.Min.X, proofTop, field.Max.X, proofTop+proof),
		kit.Text(th.Theme, th.T.Proof, th.R.Fg2, i18n.T("By a name, by the words in a file, or by what the code does.")))
}

func (p *Page) layoutResults(gtx layout.Context, e *env.Env, info protocol.IndexInfo, area image.Rectangle, alpha float32) {
	th := e.Theme
	opacity := paint.PushOpacity(gtx.Ops, alpha)
	defer opacity.Pop()
	look.Place(gtx, area, func(gtx layout.Context) layout.Dimensions {
		if len(p.hits) == 0 {
			return look.ReadingColumn(gtx, e.Class, func(gtx layout.Context) layout.Dimensions {
				return p.layoutNoHits(gtx, e, info)
			})
		}
		return p.scroll.Layout(gtx, th, len(p.hits), func(gtx layout.Context, index int) layout.Dimensions {
			return look.ReadingColumn(gtx, e.Class, func(gtx layout.Context) layout.Dimensions {
				bottom := th.Sp.S1
				if index == len(p.hits)-1 {
					bottom = th.Sp.S5
				}
				return layout.Inset{Bottom: bottom}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return p.layoutHit(gtx, e, info, index)
				})
			})
		})
	})
}

func (p *Page) layoutSearch(gtx layout.Context, e *env.Env, info protocol.IndexInfo) layout.Dimensions {
	th := e.Theme
	p.search.Placeholder, p.search.Hint = i18n.Tf("Search %s", info.Name), "Enter"
	p.search.Editor.Submit = true
	if p.focus {
		p.focus = false
		p.search.Focus(gtx)
	}
	for {
		raw, ok := p.search.Update(gtx)
		if !ok {
			break
		}
		if submit, ok := raw.(widget.SubmitEvent); ok {
			p.query = strings.TrimSpace(submit.Text)
			p.runSearch(e)
		}
	}
	p.modes.Labels = []string{i18n.T("Words and meaning"), i18n.T("Words"), i18n.T("Meaning")}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			return look.PaneField(gtx, th, gtx.Focused(&p.search.Editor), func(gtx layout.Context) layout.Dimensions {
				return p.search.Layout(gtx, th.Theme)
			})
		}),
		layout.Rigid(layout.Spacer{Height: th.Sp.S3}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					dimensions := p.modes.Layout(gtx, th.Theme)
					if _, changed := p.modes.Changed(); changed {
						p.runSearch(e)
					}
					return dimensions
				}),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return layout.E.Layout(gtx, p.resultNote(th))
				}),
			)
		}),
		layout.Rigid(layout.Spacer{Height: th.Sp.S3}.Layout),
	)
}

func (p *Page) resultNote(th *look.Theme) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		switch {
		case p.loading:
			return kit.Working{Active: true, Size: 16}.Layout(gtx, th.Theme)
		case p.asked == "" || len(p.hits) == 0:
			return layout.Dimensions{}
		case len(p.hits) == 1:
			return kit.Text(th.Theme, th.T.Caption, th.R.Fg3, i18n.T("One result"))(gtx)
		default:
			return kit.Text(th.Theme, th.T.Caption, th.R.Fg3, i18n.Tf("%d results", len(p.hits)))(gtx)
		}
	}
}

func (p *Page) layoutNoHits(gtx layout.Context, e *env.Env, info protocol.IndexInfo) layout.Dimensions {
	th := e.Theme
	if p.loading {
		return look.SkeletonRows(gtx, th, 4, true)
	}
	body := i18n.Tf("Nothing in %s matches “%s”.", info.Name, p.asked)
	if p.modes.Selected != 0 {
		body += " " + i18n.T("Words and meaning looks wider.")
	}
	gtx.Constraints.Max.Y = min(gtx.Constraints.Max.Y, gtx.Dp(260))
	gtx.Constraints.Min.Y = gtx.Constraints.Max.Y
	return kit.Empty{Icon: look.Icon.Search, Title: i18n.T("No results"), Body: body}.Layout(gtx, th.Theme)
}

func (p *Page) layoutHit(gtx layout.Context, e *env.Env, info protocol.IndexInfo, index int) layout.Dimensions {
	th := e.Theme
	hit := p.hits[index]
	row := p.rows[index]
	if row == nil {
		row = new(hitRow)
		p.rows[index] = row
	}
	if row.click.Clicked(gtx) {
		if p.open == index {
			p.open = -1
		} else {
			p.open = index
		}
	}
	row.file.Kind, row.file.Small, row.file.Icon, row.file.Label = kit.PlaqueLine, true, look.Icon.OpenInNew, i18n.T("Open file")
	if row.file.Clicked(gtx) {
		e.Open(filepath.Join(info.Root, hit.Path))
	}
	height := gtx.Dp(look.SideRowTall)
	head := func(gtx layout.Context) layout.Dimensions {
		size := image.Pt(gtx.Constraints.Max.X, height)
		gtx.Constraints = layout.Exact(size)
		dimensions, hover := row.hover.Layout(gtx, th, func(gtx layout.Context) layout.Dimensions {
			return row.click.Layout(gtx, func(gtx layout.Context) layout.Dimensions { return layout.Dimensions{Size: size} })
		})
		if fill := max(hover, boolFloat(p.open == index)); fill > 0 {
			kit.FillCut(gtx.Ops, image.Rectangle{Max: size}, gtx.Dp(th.Cut.Small), palette.WithAlpha(th.R.Stone3, fill))
		}
		inset := gtx.Dp(13)
		text := gtx
		text.Constraints = layout.Constraints{Max: image.Pt(size.X-2*inset, size.Y)}
		look.MiddleLeft(text, inset, size.Y, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Baseline}.Layout(gtx,
						layout.Rigid(kit.RawText(th.Theme, th.T.Mono, th.R.Fg, fmt.Sprintf("%s:%d", hit.Path, hit.Start))),
						layout.Rigid(layout.Spacer{Width: th.Sp.S3}.Layout),
						layout.Flexed(1, kit.RawText(th.Theme, th.T.Label, th.R.Fg2, hit.Symbol)),
					)
				}),
				layout.Rigid(kit.RawText(th.Theme, th.T.Caption, th.R.Fg3, firstLine(hit.Snippet))),
			)
		})
		return dimensions
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(head),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return row.unfold.Layout(gtx, th, p.open == index, func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Top: th.Sp.S1, Bottom: th.Sp.S2}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return kit.Slab(gtx, th.R.Stone1, th.Cut.Mid, func(gtx layout.Context) layout.Dimensions {
						return layout.UniformInset(13).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							gtx.Constraints.Min.X = gtx.Constraints.Max.X
							return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
								layout.Rigid(look.Paragraph(th, th.T.Mono, th.R.Fg2, strings.TrimRight(hit.Snippet, "\n"))),
								layout.Rigid(layout.Spacer{Height: th.Sp.S3}.Layout),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions { return row.file.Layout(gtx, th.Theme) }),
							)
						})
					})
				})
			})
		}),
	)
}

func boolFloat(value bool) float32 {
	if value {
		return 1
	}
	return 0
}

func firstLine(text string) string {
	for _, line := range strings.Split(text, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			return line
		}
	}
	return ""
}

func (p *Page) runSearch(e *env.Env) {
	if p.query == "" {
		p.hits, p.asked, p.open = nil, "", -1
		return
	}
	query := p.query
	params := protocol.SearchParams{
		IndexRef: protocol.IndexRef{ID: p.selected}, Query: query, Limit: 40,
		Mode: searchModes[min(p.modes.Selected, len(searchModes)-1)],
	}
	p.loading, p.asked = true, query
	var result protocol.SearchResult
	e.Call(protocol.MIndexSearch, params, &result, func(err error) {
		p.loading = false
		if err != nil {
			e.Notify(err.Error())
			return
		}
		p.hits, p.rows, p.open = result.Hits, make(map[int]*hitRow), -1
		p.scroll.ScrollTo(0)
	})
}

func grouped(value int) string {
	text := fmt.Sprint(value)
	for index := len(text) - 3; index > 0; index -= 3 {
		text = text[:index] + "," + text[index:]
	}
	return text
}

// addDialog is the typed way to index a folder, for when no chooser answers
type addDialog struct {
	shown       bool
	dialog      kit.Dialog
	path        env.PathField
	add, cancel kit.Plaque
}

// ask goes straight to the system chooser: picking a folder indexes it
func (d *addDialog) ask(e *env.Env, page *Page) {
	e.ChooseFolder(i18n.T("Index a folder"), paths.Home(), func(path string) { d.submit(e, page, path) }, d.open)
}

func (d *addDialog) open() {
	d.shown, d.dialog.Open = true, true
	d.path.Editor.Submit = true
}

func (d *addDialog) submit(e *env.Env, page *Page, folder string) {
	var info protocol.IndexInfo
	e.Call(protocol.MIndexAdd, protocol.IndexAddParams{Path: folder}, &info, func(err error) {
		if err != nil {
			e.Notify(err.Error())
			return
		}
		d.path.SetText("")
		page.selected = info.ID
	})
}

func (d *addDialog) layout(gtx layout.Context, e *env.Env, page *Page) {
	if !d.shown {
		return
	}
	th := e.Theme
	submitted := false
	for {
		event, ok := d.path.Editor.Update(gtx)
		if !ok {
			break
		}
		if _, ok := event.(widget.SubmitEvent); ok {
			submitted = true
		}
	}
	d.path.Title = i18n.T("Index a folder")
	d.path.Label, d.path.Placeholder = i18n.T("Folder"), paths.Home()+"/Work/project"
	d.path.Help = i18n.T("Its files are read on this machine. Nothing leaves it.")
	d.cancel.Kind, d.cancel.Label = kit.PlaqueQuiet, i18n.T("Cancel")
	d.add.Kind, d.add.Label = kit.PlaqueGold, i18n.T("Add folder")
	folder := d.path.Path()
	if d.cancel.Clicked(gtx) {
		d.dialog.Dismiss(gtx)
	}
	if (d.add.Clicked(gtx) || submitted) && folder != "" {
		d.submit(e, page, folder)
		d.dialog.Dismiss(gtx)
	}
	d.dialog.Layout(gtx, th.Theme, i18n.T("Index a folder"),
		func(gtx layout.Context) layout.Dimensions { return d.path.Layout(gtx, e) },
		func(gtx layout.Context) layout.Dimensions { return d.cancel.Layout(gtx, th.Theme) },
		func(gtx layout.Context) layout.Dimensions { return d.add.Layout(gtx, th.Theme) },
	)
	if d.dialog.Closed() {
		d.shown = false
	}
}

type removeDialog struct {
	info    protocol.IndexInfo
	confirm kit.Confirm
}

func (d *removeDialog) open(info protocol.IndexInfo) {
	d.info, d.confirm.Dialog.Open, d.confirm.Danger = info, true, true
}

func (d *removeDialog) layout(gtx layout.Context, e *env.Env, page *Page) {
	if d.info.ID == "" {
		return
	}
	body := i18n.Tf("What Rashin read from %s is deleted. The folder and its files stay.", d.info.Name)
	if d.confirm.Layout(gtx, e.Theme.Theme, i18n.T("Remove this index?"), body, i18n.T("Remove"), "") {
		id := d.info.ID
		e.Call(protocol.MIndexRemove, protocol.IndexRef{ID: id}, new(protocol.Empty), func(err error) {
			if err != nil {
				e.Notify(err.Error())
				return
			}
			if page.selected == id {
				page.selected, page.hits, page.asked = "", nil, ""
			}
		})
	}
	if d.confirm.Closed() {
		d.info = protocol.IndexInfo{}
	}
}
