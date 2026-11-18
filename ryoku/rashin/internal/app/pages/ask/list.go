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
	history := e.Feature("history")
	var starred, archived []look.SideRow
	for _, id := range snapshot.Order {
		info, ok := snapshot.Sessions[id]
		// a temporary conversation is never history, it shows only while it is the one open
		if !ok || info.Kind != protocol.KindAsk || (info.Temporary && id != p.selected) {
			continue
		}
		row := look.SideRow{
			ID: id, Title: sessionTitle(info), Facts: memoryMark(info), Working: info.State == protocol.StateWorking, Menu: !info.Temporary,
		}
		switch {
		case history && info.Archived:
			row.Depth = 1
			archived = append(archived, row)
		case history && info.Starred:
			starred = append(starred, row)
		default:
			group := dayGroup(gtx.Now, info.Updated)
			rows[group] = append(rows[group], row)
		}
	}
	groups := make([]look.SideGroup, 0, len(titles))
	if len(starred) > 0 {
		groups = append(groups, look.SideGroup{Title: i18n.T("Starred"), Rows: starred})
	}
	for index, title := range titles {
		groups = append(groups, look.SideGroup{Title: title, Rows: rows[index]})
	}
	if len(archived) > 0 {
		fold := []look.SideRow{{ID: "ask:archived", Title: i18n.Tf("Archived (%d)", len(archived)), Icon: look.Icon.Inbox, Branch: true, Open: p.archivedOpen}}
		if p.archivedOpen {
			fold = append(fold, archived...)
		}
		groups = append(groups, look.SideGroup{Rows: fold})
	}
	p.list.Empty = i18n.T("What you ask is kept here, newest first.")
	dimensions := p.list.Layout(gtx, e.Theme, e.Overlays, groups, p.selected)
	for _, event := range p.list.Events() {
		switch event.Kind {
		case look.SideOpen:
			if event.ID == "ask:archived" {
				p.archivedOpen = !p.archivedOpen
				continue
			}
			e.Navigate(protocol.TabAsk, event.ID)
		case look.SideMenu:
			p.openRowMenu(e, event.ID, event.Anchor)
		case look.SideRename:
			renameSession(e, event.ID, event.Title)
		case look.SideToggle:
			if event.ID == "ask:archived" {
				p.archivedOpen = !p.archivedOpen
			}
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
	if e.Feature("history") {
		info := e.Snapshot().Sessions[id]
		star, archive := i18n.T("Star"), i18n.T("Archive")
		if info.Starred {
			star = i18n.T("Unstar")
		}
		if info.Archived {
			archive = i18n.T("Unarchive")
		}
		items = append(items, kit.MenuItem{Divider: true},
			kit.MenuItem{Icon: look.Icon.Lead, Label: star},
			kit.MenuItem{Icon: look.Icon.Inbox, Label: archive})
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
				p.remove.open(e, id)
			case 5, 6:
				info := e.Snapshot().Sessions[id]
				params := protocol.SessionFlagsParams{ID: id}
				if item == 5 {
					value := !info.Starred
					params.Starred = &value
				} else {
					value := !info.Archived
					params.Archived = &value
				}
				e.Call(protocol.MSessionSetFlags, params, new(protocol.SessionInfo), e.Report)
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
