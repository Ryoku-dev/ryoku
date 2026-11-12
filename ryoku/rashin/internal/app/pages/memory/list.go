package memory

import (
	"image"
	"strconv"
	"strings"

	"gioui.org/layout"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/app/pages/memoryui"
	"ryoku-rashin/internal/protocol"
)

// one page of memory.list holds a whole list; the engine caps each memory at three lines
const listLimit = 1000

// what memory.list answered for the filter, the search and the sort shown, asked again whenever memory changes
type list struct {
	params   protocol.MemoryListParams
	revision uint64
	asked    bool
	asking   bool
	loaded   bool
	memories []protocol.Memory
}

func (l *list) ensure(e *env.Env, params protocol.MemoryListParams, revision uint64) {
	if l.asking || (l.asked && params == l.params && revision == l.revision) {
		return
	}
	l.params, l.revision, l.asked, l.asking = params, revision, true, true
	asked := params
	asked.Limit = listLimit
	e.ListMemory(asked, func(result protocol.MemoryListResult, err error) {
		if err != nil {
			l.asking = false
			e.Report(err)
			return
		}
		// a line the scanner blocked stands among the memories in use, so it can be read and forgotten
		if params.Status != "" {
			l.take(params, result.Memories)
			return
		}
		blocked := asked
		blocked.Status = protocol.MemoryBlocked
		e.ListMemory(blocked, func(more protocol.MemoryListResult, err error) {
			if err != nil {
				e.Report(err)
			}
			l.take(params, append(result.Memories, more.Memories...))
		})
	})
}

// an answer for a filter no longer shown is dropped, the next frame asks for the one that is
func (l *list) take(params protocol.MemoryListParams, memories []protocol.Memory) {
	l.asking = false
	if params == l.params {
		l.memories, l.loaded = memories, true
	}
}

var sorts = []string{protocol.MemorySortNewest, protocol.MemorySortOldest, protocol.MemorySortUsed, protocol.MemorySortUnused}

func sortLabels() []string {
	return []string{i18n.T("Newest"), i18n.T("Oldest"), i18n.T("Most used"), i18n.T("Least used")}
}

func filterLabel(filter string) string {
	switch filter {
	case env.MemoryAbout:
		return i18n.T("About you")
	case env.MemoryProjects:
		return i18n.T("Projects")
	case env.MemoryTeams:
		return i18n.T("Teams")
	case env.MemorySuggested:
		return i18n.T("Suggested")
	case env.MemoryReplaced:
		return i18n.T("Replaced")
	}
	return i18n.T("All")
}

func filterIndex(filter string) int {
	for index, known := range env.MemoryFilters {
		if known == filter {
			return index
		}
	}
	return 0
}

func filterCount(counts protocol.MemoryCounts, filter string) int {
	switch filter {
	case env.MemoryAbout:
		return counts.You
	case env.MemoryProjects:
		return counts.Projects
	case env.MemoryTeams:
		return counts.Teams
	case env.MemorySuggested:
		return counts.Suggested
	case env.MemoryReplaced:
		return counts.Replaced
	}
	return counts.All
}

func countFacts(count int) string {
	if count == 0 {
		return ""
	}
	return strconv.Itoa(count)
}

// the filter a scope belongs under
func scopeFilter(scope string) string {
	switch {
	case scope == protocol.MemoryYou:
		return env.MemoryAbout
	case strings.HasPrefix(scope, protocol.MemoryProjectPrefix):
		return env.MemoryProjects
	case strings.HasPrefix(scope, protocol.MemoryTeamPrefix):
		return env.MemoryTeams
	}
	return env.MemoryAll
}

// projects then teams, each a place of its own in the index
func scopeRows(scopes []protocol.MemoryScope) []look.SideRow {
	var rows []look.SideRow
	for _, scope := range memoryui.ScopeOptions(scopes, "")[1:] {
		icon := look.Icon.Projects
		if strings.HasPrefix(scope.ID, protocol.MemoryTeamPrefix) {
			icon = look.Icon.Teams
		}
		rows = append(rows, look.SideRow{ID: "scope:" + scope.ID, Title: scope.Name, Facts: countFacts(scope.Count), Icon: icon})
	}
	return rows
}

// a run of rows under one header
type group struct {
	scope, title string
	memories     []protocol.Memory
}

// under All, Projects and Teams the rows gather by scope, About you first; Suggested and Replaced stand as one run
func (p *Page) groups(status protocol.MemoryStatus) []group {
	memories := p.list.memories
	if p.place.Filter == env.MemorySuggested || p.place.Filter == env.MemoryReplaced {
		return []group{{memories: memories}}
	}
	order := memoryui.ScopeOptions(status.Scopes, "")
	var groups []group
	at := make(map[string]int)
	for _, scope := range order {
		at[scope.ID] = len(groups)
		groups = append(groups, group{scope: scope.ID, title: scope.Name})
	}
	for _, memory := range memories {
		index, ok := at[memory.Scope]
		if !ok {
			at[memory.Scope] = len(groups)
			index = len(groups)
			groups = append(groups, group{scope: memory.Scope, title: memory.ScopeName})
		}
		groups[index].memories = append(groups[index].memories, memory)
	}
	shown := groups[:0]
	for _, group := range groups {
		if len(group.memories) > 0 {
			shown = append(shown, group)
		}
	}
	return shown
}

// the rows under their headers, and where among them the open editor stands, -1 for none
func (p *Page) listItems(e *env.Env, status protocol.MemoryStatus) ([]layout.Widget, int) {
	th := e.Theme
	if !p.list.loaded {
		return []layout.Widget{func(gtx layout.Context) layout.Dimensions { return look.SkeletonRows(gtx, th, 4, true) }}, -1
	}
	groups := p.groups(status)
	if len(groups) == 0 || (len(groups) == 1 && len(groups[0].memories) == 0) {
		return []layout.Widget{func(gtx layout.Context) layout.Dimensions { return p.layoutListEmpty(gtx, e) }}, -1
	}
	var items []layout.Widget
	var order []string
	editing := -1
	for _, group := range groups {
		for _, memory := range group.memories {
			order = append(order, memory.ID)
		}
	}
	p.rows.Keep(order)
	if p.headers == nil {
		p.headers = make(map[string]*groupHeader)
	}
	for index, group := range groups {
		if group.title != "" {
			header := p.headers[group.scope]
			if header == nil {
				header = new(groupHeader)
				p.headers[group.scope] = header
			}
			top := th.Sp.S4
			if index == 0 {
				top = 0
			}
			items = append(items, func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Top: top, Bottom: th.Sp.S2}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return header.layout(gtx, e, group, &p.forgetAll)
				})
			})
		}
		for _, memory := range group.memories {
			if p.editor.IsOpen() && p.editor.ID() == memory.ID {
				editing = len(items)
			}
			items = append(items, p.rowItem(e, status, memory, group.title == "", order))
		}
	}
	return items, editing
}

func (p *Page) rowItem(e *env.Env, status protocol.MemoryStatus, memory protocol.Memory, showScope bool, order []string) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Bottom: 2}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			if p.editor.IsOpen() && p.editor.ID() == memory.ID {
				return layout.Inset{Top: e.Theme.Sp.S2, Bottom: e.Theme.Sp.S2}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return p.editor.Layout(gtx, e, status.Scopes, status.Caps)
				})
			}
			row := p.rows.Row(memory.ID)
			if p.focusRow == memory.ID {
				p.focusRow = ""
				row.Focus(gtx)
			}
			dimensions, act := row.Layout(gtx, e, memory, memoryui.RowView{ShowScope: showScope, Selected: p.selected == memory.ID, Caps: status.Caps, Teams: p.teams})
			switch act {
			case memoryui.RowSelect:
				p.selected = memory.ID
			case memoryui.RowEdit:
				p.selected = memory.ID
				p.editor.Open(e, memory)
				p.reveal = true
			case memoryui.RowPrevious:
				p.step(order, memory.ID, -1)
			case memoryui.RowNext:
				p.step(order, memory.ID, 1)
			}
			return dimensions
		})
	}
}

// Up and Down carry the selection fill and the keyboard to the next row
func (p *Page) step(order []string, from string, by int) {
	for index, id := range order {
		if id == from {
			next := order[min(max(index+by, 0), len(order)-1)]
			p.selected, p.focusRow = next, next
			return
		}
	}
}

// a scope's header: its name and count, and a menu that forgets the whole scope
type groupHeader struct {
	more look.IconAction
	menu kit.Menu
}

func (h *groupHeader) layout(gtx layout.Context, e *env.Env, group group, forget *memoryui.ForgetAll) layout.Dimensions {
	th := e.Theme
	tag := &h.menu
	items := []kit.MenuItem{{Icon: look.Icon.Delete, Label: i18n.Tf("Forget everything about %s", group.title), Danger: true}}
	if h.more.Clicked(gtx) {
		scope := group.scope
		anchor := e.Overlays.AnchorOf(&h.more.Click.Clickable, image.Pt(gtx.Dp(28), gtx.Dp(28)))
		e.Overlays.Open(tag, anchor, look.Placement{}, func(gtx layout.Context) layout.Dimensions {
			dimensions := h.menu.Layout(gtx, th.Theme, items)
			if _, ok := h.menu.Clicked(gtx); ok {
				e.Overlays.Close(tag)
				forget.Open(scope)
			}
			return dimensions
		})
	}
	h.more.Icon, h.more.Description = look.Icon.More, i18n.Tf("More for %s", group.title)
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
		layout.Flexed(1, kit.GroupHeader(th.Theme, group.title, len(group.memories))),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions { return h.more.Layout(gtx, th, th.R.Fg2) }),
	)
}
