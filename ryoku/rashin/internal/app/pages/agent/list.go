package agent

import (
	"image"

	"gioui.org/layout"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/protocol"
)

func (p *Page) List(gtx layout.Context, e *env.Env) look.ListDimensions {
	p.loadWorktrees(e)
	snapshot := e.Snapshot()
	sessions := groupSessions(snapshot)
	groups := make([]look.SideGroup, 0, groupCount)
	for at := groupNeeds; at < groupCount; at++ {
		rows := make([]look.SideRow, 0, len(sessions[at]))
		for _, info := range sessions[at] {
			row := look.SideRow{
				ID: info.ID, Title: sessionTitle(info), Facts: goalFact(info) + sessionPlace(snapshot, info),
				Working: at == groupWorking, Strong: at == groupDone, Menu: true,
			}
			// only a session that wants attention gets a mark
			if at == groupNeeds {
				row.Tone = sessionTone(snapshot, info)
			}
			rows = append(rows, row)
		}
		groups = append(groups, look.SideGroup{Title: at.label(), Count: len(rows), Rows: rows})
	}
	p.list.Empty = i18n.T("Sessions you start are kept here, sorted by what they need from you.")
	dimensions := p.list.Layout(gtx, e.Theme, e.Overlays, groups, p.selected)
	for _, event := range p.list.Events() {
		switch event.Kind {
		case look.SideOpen:
			open(e, event.ID)
		case look.SideMenu:
			p.openRowMenu(e, event.ID, event.Anchor)
		case look.SideRename:
			renameSession(e, event.ID, event.Title)
		}
	}
	return dimensions
}

func (p *Page) openRowMenu(e *env.Env, id string, anchor image.Rectangle) {
	snapshot := e.Snapshot()
	info := snapshot.Sessions[id]
	running := info.Alive || info.State == protocol.StateWorking || info.State == protocol.StateWaiting
	items := []kit.MenuItem{
		{Icon: look.Icon.Rename, Label: i18n.T("Rename"), Shortcut: "F2"},
		{Icon: look.Icon.Terminal, Label: i18n.T("Open terminal")},
		{Icon: look.Icon.Folder, Label: i18n.T("Open folder")},
		{Divider: true},
		{Icon: look.Icon.Stop, Label: i18n.T("Stop"), Disabled: !running},
		{Icon: look.Icon.Delete, Label: i18n.T("Delete"), Danger: true},
	}
	tag := &p.rowMenu
	e.Overlays.Open(tag, anchor, look.Placement{}, func(gtx layout.Context) layout.Dimensions {
		dimensions := p.rowMenu.Layout(gtx, e.Theme.Theme, items)
		item, ok := p.rowMenu.Clicked(gtx)
		if !ok {
			return dimensions
		}
		e.Overlays.Close(tag)
		switch item {
		case 0:
			p.list.Rename(id)
		case 1:
			e.OpenTerminal(sessionFolder(info))
		case 2:
			e.OpenFolder(sessionFolder(info))
		case 4:
			stopSession(e, id)
		case 5:
			p.remove.open(id)
		}
		return dimensions
	})
}

func sessionTitle(info protocol.SessionInfo) string {
	if info.Title == "" {
		return i18n.T("New session")
	}
	return info.Title
}

func renameSession(e *env.Env, id, title string) {
	e.Call(protocol.MSessionRename, protocol.RenameParams{ID: id, Title: title}, new(protocol.SessionInfo), e.Report)
}

func stopSession(e *env.Env, id string) {
	e.Call(protocol.MSessionStop, protocol.SessionRef{ID: id}, new(protocol.Empty), e.Report)
}
