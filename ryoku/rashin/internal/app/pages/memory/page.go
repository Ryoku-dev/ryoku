// Package memory is the Memory page: everything Rashin remembers, how it grew,
// and the controls that change, pin, forget, keep or import it.
package memory

import (
	"image"
	"strings"

	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/app/pages/memoryui"
	"ryoku-rashin/internal/protocol"
)

// the column every part of the page stands in
const columnWidth = 760

type Page struct {
	place  env.MemoryPlace
	listed bool

	header look.PaneHeader
	scroll look.Scroller
	index  look.SideList

	search  kit.Search
	filters kit.Filters
	sort    look.Select
	scopes  look.Select
	sorting string

	list     list
	rows     memoryui.Rows
	headers  map[string]*groupHeader
	editor   memoryui.Editor
	selected string
	// the row that takes the keyboard the next time it is drawn
	focusRow string
	// an editor just opened scrolls into view once it is laid out
	reveal bool
	// the room left above an anchored editor, and how many items the scroller last laid out
	margin, laidOut int

	add, sees kit.Plaque
	empty     emptyActs

	sheet memoryui.Sheet
	// the teams a suggestion names, read once a frame
	teams   []protocol.Team
	card    *protocol.MemoryCard
	rebuild kit.Plaque
	close   kit.Plaque

	forgetAll memoryui.ForgetAll
	importing memoryui.Import
}

func New() *Page { return &Page{} }

// Open shows place, unfolding its memory's editor when it names one.
func (p *Page) Open(e *env.Env, place env.MemoryPlace) {
	if !env.ValidMemoryFilter(place.Filter) {
		place.Filter = env.MemoryAll
	}
	p.place = place
	p.filters.Selected = filterIndex(place.Filter)
	if place.Card {
		p.openSheet(e)
	}
	if place.ID != "" {
		p.selected, p.focusRow = place.ID, place.ID
		id := place.ID
		e.GetMemory(id, func(memory protocol.Memory, err error) {
			if err != nil {
				e.Report(err)
				return
			}
			if p.place.ID == id {
				p.editor.Open(e, memory)
				p.reveal = true
			}
		})
	} else if p.editor.IsOpen() && p.editor.ID() != "" {
		p.editor.Close()
	}
}

// AddMemory opens a blank editor at the top of the list, applying to what the
// filter shows.
func (p *Page) AddMemory() {
	scope := protocol.MemoryYou
	if p.place.Scope != "" {
		scope = p.place.Scope
	}
	p.editor.OpenNew(scope)
	p.reveal = true
}

func (p *Page) ListShown(shown bool) { p.listed = shown }

func (p *Page) NewLabel() string { return "" }

// the Memory page makes nothing new of its own, the New plaque belongs to where it was opened from
func (p *Page) New(*env.Env) {}

func (p *Page) Overlay(gtx layout.Context, e *env.Env) bool {
	open := p.forgetAll.Layout(gtx, e)
	return p.importing.Layout(gtx, e) || open
}

// the filters and the scopes stand in the sidebar while it is open
func (p *Page) List(gtx layout.Context, e *env.Env) look.ListDimensions {
	status := memoryStatus(e)
	groups := []look.SideGroup{{Rows: make([]look.SideRow, 0, len(env.MemoryFilters))}}
	for _, filter := range env.MemoryFilters {
		groups[0].Rows = append(groups[0].Rows, look.SideRow{ID: "filter:" + filter, Title: filterLabel(filter), Facts: countFacts(filterCount(status.Counts, filter))})
	}
	if scopes := scopeRows(status.Scopes); len(scopes) > 0 {
		groups = append(groups, look.SideGroup{Title: i18n.T("Applies to"), Rows: scopes})
	}
	selected := "filter:" + p.place.Filter
	if p.place.Scope != "" {
		selected = "scope:" + p.place.Scope
	}
	dimensions := p.index.Layout(gtx, e.Theme, e.Overlays, groups, selected)
	for _, event := range p.index.Events() {
		if event.Kind != look.SideOpen {
			continue
		}
		if filter, ok := strings.CutPrefix(event.ID, "filter:"); ok {
			e.OpenMemory(env.MemoryPlace{Filter: filter})
		} else if scope, ok := strings.CutPrefix(event.ID, "scope:"); ok {
			e.OpenMemory(env.MemoryPlace{Filter: scopeFilter(scope), Scope: scope})
		}
	}
	return dimensions
}

func (p *Page) Content(gtx layout.Context, e *env.Env) layout.Dimensions {
	th := e.Theme
	snapshot := e.Snapshot()
	status := memoryStatus(e)
	p.teams = snapshot.Teams
	p.handle(gtx, e)
	p.list.ensure(e, p.params(), snapshot.MemoryRevision)
	items := p.items(e, status)
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			content := look.PaneHeaderContent{
				Title:    i18n.T("Memory"),
				Scrolled: p.scroll.Position.First > 0 || p.scroll.Position.Offset > 0,
				Menu: []kit.MenuItem{
					{Icon: look.Icon.Import, Label: i18n.T("Import from another assistant")},
					{Icon: look.Icon.FolderOpen, Label: i18n.T("Open the folder")},
					{Icon: look.Icon.Settings, Label: i18n.T("Settings")},
					{Divider: true},
					{Icon: look.Icon.Delete, Label: i18n.T("Forget everything"), Danger: true},
				},
			}
			if status.Caps.LeavesMachine {
				content.Facts = []look.HeaderFact{{Text: i18n.T("Leaves this computer"), Kind: look.FactMark}}
			}
			return p.header.Layout(gtx, th, e.Overlays, content)
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			p.margin = gtx.Dp(th.Sp.S4)
			dimensions := p.scroll.Layout(gtx, th, len(items), func(gtx layout.Context, index int) layout.Dimensions {
				return look.Centred(gtx, e.Class, columnWidth, items[index])
			})
			p.laidOut = len(items)
			if p.reveal {
				gtx.Execute(op.InvalidateCmd{})
			}
			return dimensions
		}),
	)
}

func (p *Page) handle(gtx layout.Context, e *env.Env) {
	for {
		event, ok := gtx.Event(
			key.Filter{Name: "F", Required: key.ModShortcut},
			key.Filter{Focus: &p.search.Editor, Name: key.NameEscape},
		)
		if !ok {
			break
		}
		pressed, ok := event.(key.Event)
		if !ok || pressed.State != key.Press {
			continue
		}
		if pressed.Name == key.NameEscape {
			p.search.SetText("")
			continue
		}
		p.search.Focus(gtx)
	}
	if item, ok := p.header.MenuClicked(); ok {
		switch item {
		case 0:
			p.importing.Open(p.place.Scope)
		case 1:
			openFolder(e)
		case 2:
			e.OpenSettings("memory")
		case 4:
			p.forgetAll.Open("")
		}
	}
	if picked, ok := p.filters.Changed(); ok && picked < len(env.MemoryFilters) {
		e.OpenMemory(env.MemoryPlace{Filter: env.MemoryFilters[picked]})
	}
	if picked, ok := p.sort.Changed(); ok && picked < len(sorts) {
		p.sorting = sorts[picked]
	}
	if p.add.Clicked(gtx) {
		p.AddMemory()
	}
	if p.sees.Clicked(gtx) {
		p.openSheet(e)
	}
	p.empty.handle(gtx, e, p)
	p.handleSheet(gtx, e)
}

func openFolder(e *env.Env) {
	if path := memoryStatus(e).Path; path != "" {
		e.OpenFolder(path)
	}
}

func (p *Page) params() protocol.MemoryListParams {
	params := env.MemoryListParams(p.place)
	params.Query, params.Sort = p.search.Text(), p.sorting
	return params
}

func memoryStatus(e *env.Env) protocol.MemoryStatus {
	if status := e.Snapshot().Memory; status != nil {
		return *status
	}
	return protocol.MemoryStatus{}
}

// the page's items in the order the column scrolls them
func (p *Page) items(e *env.Env, status protocol.MemoryStatus) []layout.Widget {
	th := e.Theme
	gap := func(height unit.Dp) layout.Widget {
		return func(gtx layout.Context) layout.Dimensions {
			return layout.Dimensions{Size: image.Pt(gtx.Constraints.Max.X, gtx.Dp(height))}
		}
	}
	empty, isEmpty := p.pageEmpty(e, status)
	items := []layout.Widget{
		gap(th.Sp.S5),
		// an empty page's acts stand in its empty state, which holds the gold
		func(gtx layout.Context) layout.Dimensions { return p.layoutLead(gtx, e, status, !isEmpty) },
		gap(th.Sp.S5),
	}
	if isEmpty {
		return append(items, empty)
	}
	items = append(items,
		func(gtx layout.Context) layout.Dimensions { return p.layoutTrend(gtx, e, status) },
		gap(th.Sp.S5),
		func(gtx layout.Context) layout.Dimensions { return p.layoutTools(gtx, e, status) },
		gap(th.Sp.S3),
	)
	editorAt := -1
	if p.editor.IsOpen() && p.editor.ID() == "" {
		editorAt = len(items)
		items = append(items,
			func(gtx layout.Context) layout.Dimensions { return p.editor.Layout(gtx, e, status.Scopes, status.Caps) },
			gap(th.Sp.S3),
		)
	}
	rows, editing := p.listItems(e, status)
	if editing >= 0 {
		editorAt = len(items) + editing
	}
	items = append(items, rows...)
	// the scroller measures a list it has laid out, so a list that just grew waits a frame before it is anchored
	if p.reveal && editorAt >= 0 && p.laidOut == len(items)+1 {
		p.reveal = false
		p.scroll.AnchorTo(editorAt, p.margin)
	}
	return append(items, gap(th.Sp.S6))
}
