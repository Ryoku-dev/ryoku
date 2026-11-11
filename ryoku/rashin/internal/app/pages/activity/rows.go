package activity

import (
	"strings"

	"gioui.org/layout"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/app/pages/teamsui"
	"ryoku-rashin/internal/protocol"
)

type itemKind uint8

const (
	itemDay itemKind = iota
	itemEntry
	itemOlder
)

// one line of the page: a day's title, an entry, or the way to older ones
type item struct {
	kind  itemKind
	day   string
	count int
	entry protocol.Activity
}

// the entries grouped by the day they happened, newest first
func (p *Page) items(gtx layout.Context) []item {
	var items []item
	header := -1
	for _, entry := range p.ledger.entries {
		day := dayTitle(gtx.Now, entry.Time)
		if header < 0 || items[header].day != day {
			header = len(items)
			items = append(items, item{kind: itemDay, day: day})
		}
		items[header].count++
		items = append(items, item{kind: itemEntry, entry: entry})
	}
	if p.ledger.more && len(items) > 0 {
		items = append(items, item{kind: itemOlder})
	}
	return items
}

func (p *Page) layoutItem(gtx layout.Context, e *env.Env, filter string, it item) layout.Dimensions {
	th := e.Theme
	switch it.kind {
	case itemDay:
		return layout.Inset{Top: th.Sp.S4, Bottom: th.Sp.S1}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return kit.GroupHeader(th.Theme, it.day, it.count)(gtx)
		})
	case itemOlder:
		p.more.Kind, p.more.Small, p.more.Label, p.more.Busy = kit.PlaqueLine, true, i18n.T("Load older"), p.ledger.loading
		if p.more.Clicked(gtx) {
			p.ledger.older(e, filter, p.team)
		}
		return layout.Inset{Top: th.Sp.S4, Bottom: th.Sp.S6}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return p.more.Layout(gtx, th.Theme)
		})
	}
	return p.layoutEntry(gtx, e, it.entry)
}

// one entry: its time, then the team when every team is shown, before what its detail says
func (p *Page) layoutEntry(gtx layout.Context, e *env.Env, entry protocol.Activity) layout.Dimensions {
	detail := entry.Detail
	if p.team == "" && entry.Team != "" {
		parts := []string{teamsui.TeamName(e.Snapshot().Teams, entry.Team)}
		if detail != "" {
			parts = append(parts, detail)
		}
		detail = strings.Join(parts, " · ")
	}
	return teamsui.LedgerRow(e.Theme, entry.Time.Local().Format("15:04"), 52, entry, detail)(gtx)
}

// with nothing to show: the skeleton while the first page loads, else what will stand here
func (p *Page) layoutEmpty(gtx layout.Context, e *env.Env) layout.Dimensions {
	th := e.Theme
	if p.ledger.loading {
		return look.Centred(gtx, e.Class, columnWidth, func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: th.Sp.S3}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return look.SkeletonRows(gtx, th, 8, false)
			})
		})
	}
	empty := kit.Empty{
		Icon:  look.Icon.Activity,
		Title: i18n.T("Nothing has happened yet"),
		Body:  i18n.T("What your teams do is recorded here: tasks, reports, approvals and every use of a credential."),
	}
	if p.filters.Selected != 0 || p.team != "" {
		empty.Title, empty.Body = i18n.T("Nothing here"), i18n.T("Nothing matches this filter yet.")
	}
	return empty.Layout(gtx, th.Theme)
}
