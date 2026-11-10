package ask

import (
	"image"
	"time"

	"gioui.org/layout"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/protocol"
)

func (p *Page) List(gtx layout.Context, e *env.Env) look.ListDimensions {
	snapshot := e.Snapshot()
	titles := [4]string{i18n.T("Today"), i18n.T("Yesterday"), i18n.T("Earlier this week"), i18n.T("Earlier")}
	var rows [4][]look.SideRow
	for _, id := range snapshot.Order {
		info, ok := snapshot.Sessions[id]
		if !ok || info.Kind != protocol.KindAsk {
			continue
		}
		group := dayGroup(gtx.Now, info.Updated)
		rows[group] = append(rows[group], look.SideRow{
			ID: id, Title: sessionTitle(info), Working: info.State == protocol.StateWorking, Menu: true,
		})
	}
	groups := make([]look.SideGroup, 0, len(titles))
	for index, title := range titles {
		groups = append(groups, look.SideGroup{Title: title, Rows: rows[index]})
	}
	p.list.Empty = i18n.T("What you ask is kept here, newest first.")
	dimensions := p.list.Layout(gtx, e.Theme, e.Overlays, groups, p.selected)
	for _, event := range p.list.Events() {
		switch event.Kind {
		case look.SideOpen:
			e.Navigate(protocol.TabAsk, event.ID)
		case look.SideMenu:
			p.openRowMenu(e, event.ID, event.Anchor)
		case look.SideRename:
			renameSession(e, event.ID, event.Title)
		}
	}
	return dimensions
}

func (p *Page) openRowMenu(e *env.Env, id string, anchor image.Rectangle) {
	items := []kit.MenuItem{
		{Icon: look.Icon.Rename, Label: i18n.T("Rename"), Shortcut: "F2"},
		{Icon: look.Icon.Agents, Label: i18n.T("Move to Agent")},
		{Divider: true},
		{Icon: look.Icon.Delete, Label: i18n.T("Delete"), Danger: true},
	}
	tag := &p.rowMenu
	e.Overlays.Open(tag, anchor, look.Placement{}, func(gtx layout.Context) layout.Dimensions {
		dimensions := p.rowMenu.Layout(gtx, e.Theme.Theme, items)
		if item, ok := p.rowMenu.Clicked(gtx); ok {
			e.Overlays.Close(tag)
			switch item {
			case 0:
				p.list.Rename(id)
			case 1:
				p.handoff.open(e, id, "")
			case 3:
				p.remove.open(id)
			}
		}
		return dimensions
	})
}

func renameSession(e *env.Env, id, title string) {
	var changed protocol.SessionInfo
	e.Call(protocol.MSessionRename, protocol.RenameParams{ID: id, Title: title}, &changed, func(err error) {
		if err != nil {
			e.Notify(err.Error())
		}
	})
}

func sessionTitle(info protocol.SessionInfo) string {
	if info.Title == "" {
		return i18n.T("New ask")
	}
	return info.Title
}

// by calendar day in the reader's zone
func dayGroup(now, when time.Time) int {
	now, when = now.Local(), when.Local()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	switch {
	case !when.Before(today):
		return 0
	case !when.Before(today.AddDate(0, 0, -1)):
		return 1
	case !when.Before(today.AddDate(0, 0, -6)):
		return 2
	default:
		return 3
	}
}
