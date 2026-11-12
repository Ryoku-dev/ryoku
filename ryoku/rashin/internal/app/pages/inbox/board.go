package inbox

import (
	"gioui.org/layout"
	"gioui.org/unit"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/app/pages/teamsui"
	"ryoku-rashin/internal/app/state"
)

// the Now board of Teams mode
type board struct {
	header look.PaneHeader
	scroll look.Scroller
	needs  teamsui.NeedCards
	tiles  map[string]*tile
	links  map[string]*kit.Click

	templates teamsui.Templates
	start     teamsui.TeamStart
	// a need, or a task whose need, to bring into view on the next frame
	reveal string
	// every team's month, read once the Inbox opens and again when the snapshot says it moved
	month teamsui.MonthSpend
}

type boardItem struct {
	kind  int
	count int
	need  teamsui.Need
}

const (
	itemLead = iota
	itemNeeds
	itemNeed
	itemColumns
	itemStart
)

func (b *board) layout(gtx layout.Context, e *env.Env) layout.Dimensions {
	snapshot := e.Snapshot()
	needs := teamsui.Needs(snapshot)
	items := b.items(snapshot, needs)
	b.bringIntoView(items)
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			content := look.PaneHeaderContent{
				Title:    i18n.T("Inbox"),
				Actions:  []look.HeaderAction{{Icon: look.Icon.Add, Label: i18n.T("New task")}},
				Scrolled: b.scroll.Position.First > 0 || b.scroll.Position.Offset > 0,
			}
			dimensions := b.header.Layout(gtx, e.Theme, e.Overlays, content)
			if _, ok := b.header.ActionClicked(); ok {
				e.NewTeamTask("")
			}
			return dimensions
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return b.scroll.Layout(gtx, e.Theme, len(items), func(gtx layout.Context, index int) layout.Dimensions {
				return look.Centred(gtx, e.Class, look.BoardWidth, func(gtx layout.Context) layout.Dimensions {
					return b.layoutItem(gtx, e, snapshot, needs, items[index], index == len(items)-1)
				})
			})
		}),
	)
}

func (b *board) items(snapshot state.Snapshot, needs []teamsui.Need) []boardItem {
	items := []boardItem{{kind: itemLead}}
	// open needs and the ones answered here, so an answer folds where its card stood; a proposed team can wait here before any team exists
	shown := b.needs.Shown(needs)
	if len(shown) > 0 {
		items = append(items, boardItem{kind: itemNeeds, count: len(needs)})
		for _, need := range shown {
			items = append(items, boardItem{kind: itemNeed, need: need})
		}
	}
	if len(snapshot.Teams) == 0 {
		return append(items, boardItem{kind: itemStart, count: len(shown)})
	}
	return append(items, boardItem{kind: itemColumns})
}

// the sidebar or a task page asked for a need, by its own id or by its task's
func (b *board) bringIntoView(items []boardItem) {
	if b.reveal == "" {
		return
	}
	for index, item := range items {
		if item.kind != itemNeed {
			continue
		}
		need := item.need
		if teamsui.NeedKey(need) == b.reveal || need.Approval != nil && need.Approval.Task == b.reveal {
			b.scroll.ScrollTo(index)
			break
		}
	}
	b.reveal = ""
}

func (b *board) layoutItem(
	gtx layout.Context,
	e *env.Env,
	snapshot state.Snapshot,
	needs []teamsui.Need,
	item boardItem,
	last bool,
) layout.Dimensions {
	th := e.Theme
	switch item.kind {
	case itemLead:
		return layout.Inset{Top: th.Sp.S5, Bottom: th.Sp.S4}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layoutLead(gtx, th, snapshot, len(needs))
		})
	case itemNeeds:
		return layout.Inset{Top: th.Sp.S3, Bottom: th.Sp.S2}.Layout(gtx, look.SectionTitle(th, i18n.T("Needs you"), item.count))
	case itemNeed:
		return layout.Inset{Bottom: th.Sp.S2}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return b.needs.Layout(gtx, e, item.need)
		})
	case itemStart:
		// under cards it stands apart as the columns do
		top := unit.Dp(0)
		if item.count > 0 {
			top = th.Sp.S3
		}
		return layout.Inset{Top: top, Bottom: th.Sp.S5}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return b.start.Layout(gtx, e, b.templates.List(e))
		})
	}
	bottom := unit.Dp(0)
	if last {
		bottom = th.Sp.S5
	}
	return layout.Inset{Top: th.Sp.S3, Bottom: bottom}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return b.layoutColumns(gtx, e, snapshot)
	})
}

// the results and what comes next stand side by side, 60 to 40
func (b *board) layoutColumns(gtx layout.Context, e *env.Env, snapshot state.Snapshot) layout.Dimensions {
	return look.Split(gtx, e.Theme,
		func(gtx layout.Context) layout.Dimensions { return b.layoutResults(gtx, e, snapshot) },
		func(gtx layout.Context) layout.Dimensions { return b.layoutAhead(gtx, e, snapshot) },
	)
}

func (b *board) tile(key string) *tile {
	if b.tiles == nil {
		b.tiles = make(map[string]*tile)
	}
	item := b.tiles[key]
	if item == nil {
		item = new(tile)
		b.tiles[key] = item
	}
	return item
}

func (b *board) link(key string) *kit.Click {
	if b.links == nil {
		b.links = make(map[string]*kit.Click)
	}
	item := b.links[key]
	if item == nil {
		item = new(kit.Click)
		b.links[key] = item
	}
	return item
}
