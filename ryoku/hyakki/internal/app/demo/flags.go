package demo

import (
	"encoding/json"
	"time"

	"ryoku-i18n"

	"ryoku-rashin/internal/protocol"
)

func (s *server) callFlags(method string, raw json.RawMessage) (any, bool, error) {
	if method != protocol.MSessionSetFlags {
		return nil, false, nil
	}
	if !s.data.snapshot.Settings.Features["history"] {
		return nil, true, protocol.Errorf(protocol.CodeDisabled, "Turn on Star and archive in Settings first.")
	}
	var params protocol.SessionFlagsParams
	if err := decode(raw, &params); err != nil {
		return nil, true, err
	}
	info, ok := s.session(params.ID)
	if !ok {
		return nil, true, notFound("session")
	}
	if info.Kind != protocol.KindAsk || info.Temporary {
		return nil, true, protocol.Errorf(protocol.CodeBadRequest, "Only saved Ask conversations can be starred or archived.")
	}
	if params.Starred != nil {
		info.Starred = *params.Starred
	}
	if params.Archived != nil {
		info.Archived = *params.Archived
	}
	s.replaceSession(info)
	return info, true, nil
}

func addFlagFixtures(data *fixtures) {
	for index := range data.snapshot.Sessions {
		if !data.snapshot.Settings.Features["history"] {
			break
		}
		info := &data.snapshot.Sessions[index]
		switch info.ID {
		case "ask-20261110-094200-a17c":
			info.Starred = true
		case "ask-20261020-090000-old":
			info.Archived = true
		}
	}
	if !data.snapshot.Settings.Features["searchsteps"] {
		return
	}
	for _, live := range []bool{true, false} {
		id, state := "ask-searchsteps-done", protocol.StateIdle
		if live {
			id, state = "ask-searchsteps-live", protocol.StateWorking
		}
		info := protocol.SessionInfo{
			ID: id, Kind: protocol.KindAsk, Origin: protocol.OriginApp, Model: protocol.RouteAuto,
			Title: i18n.T("How does the session cache work?"), State: state, Turns: 1,
			Created: Epoch.Add(-4 * time.Minute), Updated: Epoch.Add(-time.Minute),
			LastMessageAt: Epoch.Add(-time.Minute), Alive: live,
		}
		events := []protocol.SessionEvent{
			event(1, protocol.EvUser, protocol.UserData{Text: info.Title}),
			event(2, protocol.EvTurnStart, protocol.TurnStartData{Turn: 1}),
			event(3, protocol.EvTool, protocol.ToolData{Call: "cache-search", Title: i18n.T("Search session storage"), Kind: protocol.ToolSearch, Status: protocol.ToolDone, Input: "session cache"}),
			event(4, protocol.EvTool, protocol.ToolData{Call: "cache-page", Title: i18n.T("Read storage notes"), Kind: protocol.ToolFetch, Status: protocol.ToolDone, Input: "https://docs.ryoku.dev"}),
			event(5, protocol.EvTool, protocol.ToolData{Call: "cache-read", Title: i18n.T("Read the session store"), Kind: protocol.ToolRead, Status: protocol.ToolDone, Locations: []protocol.Location{{Path: "internal/session/store.go"}}}),
		}
		if live {
			events = append(events, event(6, protocol.EvTool, protocol.ToolData{Call: "cache-read-next", Title: i18n.T("Read cache invalidation"), Kind: protocol.ToolRead, Status: protocol.ToolRunning, Locations: []protocol.Location{{Path: "internal/app/state/store.go"}}}))
		} else {
			events = append(events,
				event(6, protocol.EvText, protocol.TextData{Message: "cache-answer", Delta: i18n.T("The daemon keeps session metadata on disk and sends changes to the app. Opening a conversation loads its event log, then follows new events.")}),
				event(7, protocol.EvTurnEnd, protocol.TurnEndData{Turn: 1, Stop: protocol.StopEnd}))
		}
		info.LastSeq = int64(len(events))
		data.snapshot.Sessions = append(data.snapshot.Sessions, info)
		data.events[id] = events
		data.snapshot.Status.Sessions.Ask++
		if live {
			data.snapshot.Status.Sessions.Working++
			data.snapshot.Status.Sessions.Alive++
		}
	}
}
