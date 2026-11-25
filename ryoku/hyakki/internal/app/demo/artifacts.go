package demo

import (
	"context"
	"encoding/json"
	"strings"

	"ryoku-i18n"
	"ryoku-rashin/internal/artifacts"
	"ryoku-rashin/internal/protocol"
)

var demoPreviews artifacts.Manager

func (s *server) callArtifacts(method string, raw json.RawMessage) (any, bool, error) {
	switch method {
	case protocol.MArtifactOpen:
		var p protocol.ArtifactOpenParams
		if err := decode(raw, &p); err != nil {
			return nil, true, err
		}
		result, err := demoPreviews.Open(context.Background(), p, nil)
		return result, true, err
	case protocol.MArtifactErrors:
		var p protocol.ArtifactErrorsParams
		if err := decode(raw, &p); err != nil {
			return nil, true, err
		}
		items, err := demoPreviews.Errors(p.ID)
		return protocol.ArtifactErrorsResult{Errors: items}, true, err
	case protocol.MArtifactClose:
		var p protocol.ArtifactCloseParams
		if err := decode(raw, &p); err != nil {
			return nil, true, err
		}
		demoPreviews.Close(p.ID)
		return protocol.Empty{}, true, nil
	case protocol.MArtifactRepair:
		var p protocol.ArtifactRepairParams
		if err := decode(raw, &p); err != nil {
			return nil, true, err
		}
		id, err := demoPreviews.Session(p.ID)
		if err != nil {
			return nil, true, err
		}
		for _, info := range s.data.snapshot.Sessions {
			if info.ID != id {
				continue
			}
			turn := info.Turns + 1
			before := info.LastSeq
			text := i18n.T("The page now waits for the button before updating the count.") + "\n\n```html\n" + repairedArtifact + "\n```"
			events := []protocol.SessionEvent{
				event(before+1, protocol.EvUser, protocol.UserData{Text: i18n.T("Fix the problems reported by the preview.")}),
				event(before+2, protocol.EvTurnStart, protocol.TurnStartData{Turn: turn}),
				event(before+3, protocol.EvText, protocol.TextData{Message: "artifact-repair", Delta: text}),
				event(before+4, protocol.EvAnswer, protocol.AnswerData{Via: "demo"}),
				event(before+5, protocol.EvTurnEnd, protocol.TurnEndData{Turn: turn, Stop: protocol.StopEnd}),
			}
			s.data.events[id] = append(s.data.events[id], events...)
			for _, made := range events {
				s.told = append(s.told, protocol.SessionEventData{Session: id, SessionEvent: made})
			}
			info.Turns, info.LastSeq = turn, before+5
			s.replaceSession(info)
			return protocol.AskResult{Session: info, Turn: turn, Since: before}, true, nil
		}
		return nil, true, protocol.Errorf(protocol.CodeBadRequest, "Conversation not found")
	}
	return nil, false, nil
}

const repairedArtifact = `<!doctype html><html><head><meta charset="utf-8"><title>A little counter</title><style>body{font:18px system-ui;padding:3rem;background:#181715;color:#eee6da}button{padding:1rem;font:inherit}</style></head><body><h1>A little counter</h1><p id="count">0</p><button onclick="document.getElementById('count').textContent=++window.count">Add one</button><script>window.count=0;</script></body></html>`

func addArtifactFixtures(f *fixtures) {
	info := protocol.SessionInfo{ID: "ask-artifacts", Kind: protocol.KindAsk, Title: i18n.T("Explore a mind map and a page"), Model: protocol.RouteAuto, Origin: protocol.OriginApp, State: protocol.StateIdle, Created: Epoch, Updated: Epoch, Turns: 1, LastSeq: 5}
	answer := i18n.T("Here is how a small idea grows into a page. Select a branch to fold it, or save the map as a picture.") + "\n\n```mindmap\nA small project\n  Understand\n    The question\n    The people\n  Make\n    A clear page\n    A small interaction\n  Check\n    Read it\n    Try it\n```\n\n" + i18n.T("Preview this page, then choose Run scripts to try the counter. The demo has a small script error, so you can also try Fix them.") + "\n\n```html\n" + strings.Replace(repairedArtifact, "window.count=0;", "window.count=0;missingDemoFunction();", 1) + "\n```"
	f.snapshot.Sessions = append(f.snapshot.Sessions, info)
	f.snapshot.Status.Sessions.Ask++
	f.events[info.ID] = []protocol.SessionEvent{
		event(1, protocol.EvUser, protocol.UserData{Text: info.Title}),
		event(2, protocol.EvTurnStart, protocol.TurnStartData{Turn: 1}),
		event(3, protocol.EvText, protocol.TextData{Message: "artifact-map", Delta: answer}),
		event(4, protocol.EvAnswer, protocol.AnswerData{Via: "demo"}),
		event(5, protocol.EvTurnEnd, protocol.TurnEndData{Turn: 1, Stop: protocol.StopEnd}),
	}
}
