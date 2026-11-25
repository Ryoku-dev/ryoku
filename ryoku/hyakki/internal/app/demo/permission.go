package demo

import (
	"encoding/json"
	"time"

	"ryoku-rashin/internal/protocol"
)

// a request that stays open after its answer makes every consent look ignored
func (s *server) resolvePermission(out *outbox, raw json.RawMessage) {
	var params protocol.PermissionParams
	if decode(raw, &params) != nil || params.ID == "" {
		return
	}
	s.mu.Lock()
	_, known := s.session(params.ID)
	seq := int64(1)
	if events := s.data.events[params.ID]; len(events) > 0 {
		seq = events[len(events)-1].Seq + 1
	}
	data, _ := json.Marshal(protocol.PermissionResolvedData{Request: params.Request, Option: params.Option, Outcome: "selected", By: "user"})
	resolved := protocol.SessionEvent{Seq: seq, TS: time.Now(), Kind: protocol.EvPermissionResolved, Data: data}
	if known {
		s.data.events[params.ID] = append(s.data.events[params.ID], resolved)
	}
	s.mu.Unlock()
	if !known {
		return
	}
	wire, _ := json.Marshal(protocol.SessionEventData{Session: params.ID, SessionEvent: resolved})
	if out.send(protocol.Event{Event: protocol.EvtSessionEvent, Data: wire}) != nil {
		return
	}
	s.setState(out, params.ID, protocol.StateIdle, "")
}
