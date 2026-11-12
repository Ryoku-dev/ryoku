package demo

import (
	"encoding/json"
	"slices"
	"time"

	"ryoku-i18n"

	"ryoku-rashin/internal/protocol"
)

// a temporary conversation starts from use only, as the daemon starts it
func newMemoryMode(params protocol.SessionNewParams) string {
	if params.Temporary && params.Memory == protocol.MemoryRemember {
		return protocol.MemoryUseOnly
	}
	return params.Memory
}

// any mode before the first message, only a narrower one after, and the narrowing stands in the transcript where it was made
func (s *server) setMemory(params protocol.SetMemoryParams) (protocol.SessionInfo, error) {
	info, ok := s.session(params.ID)
	if !ok {
		return protocol.SessionInfo{}, notFound("session")
	}
	if params.Memory == info.Memory {
		return info, nil
	}
	rank := map[string]int{protocol.MemoryRemember: 0, protocol.MemoryUseOnly: 1, protocol.MemoryNone: 2}
	started := slices.ContainsFunc(s.data.events[info.ID], func(event protocol.SessionEvent) bool { return event.Kind == protocol.EvUser })
	if info.Temporary && params.Memory == protocol.MemoryRemember || started && rank[params.Memory] < rank[info.Memory] {
		return protocol.SessionInfo{}, protocol.Errorf(protocol.CodeConflict, "%s", i18n.T("This conversation's memory was set when it started; it can only be narrowed."))
	}
	info.Memory = params.Memory
	s.replaceSession(info)
	if started {
		events := s.data.events[info.ID]
		narrowed := protocol.SessionEvent{
			Seq: int64(len(events) + 1), TS: time.Now(), Kind: protocol.EvMemory,
			Data: marshal(protocol.MemoryEventData{Kind: protocol.MemoryNarrowed, Memory: params.Memory}),
		}
		s.data.events[info.ID] = append(events, narrowed)
		s.told = append(s.told, protocol.SessionEventData{Session: info.ID, SessionEvent: narrowed})
	}
	return info, nil
}

// what a session method changed beyond its answer, as the daemon broadcasts it
func (s *server) tellSessions(out *outbox, method string, raw json.RawMessage) {
	var events []protocol.Event
	s.mu.Lock()
	for _, told := range s.told {
		events = append(events, protocol.Event{Event: protocol.EvtSessionEvent, Data: marshal(told)})
	}
	s.told = nil
	s.mu.Unlock()
	if method == protocol.MSessionDelete {
		var params protocol.SessionRef
		if decode(raw, &params) == nil {
			events = append(events, protocol.Event{Event: protocol.EvtSessionsChanged, Data: marshal(protocol.SessionsChanged{Removed: []string{params.ID}})})
		}
	}
	if method == protocol.MSessionSetMemory {
		var params protocol.SetMemoryParams
		if decode(raw, &params) == nil {
			s.mu.Lock()
			info, ok := s.session(params.ID)
			s.mu.Unlock()
			if ok {
				events = append(events, protocol.Event{Event: protocol.EvtSessionInfo, Data: marshal(info)})
			}
		}
	}
	for _, event := range events {
		if out.send(event) != nil {
			return
		}
	}
}

// a kept temporary conversation goes to history whole, under its own title, still never learned from
func (s *server) forkFrom(source, info protocol.SessionInfo) protocol.SessionInfo {
	info.Memory = source.Memory
	if !source.Temporary {
		return info
	}
	info.Title = source.Title
	s.data.events[info.ID] = slices.Clone(s.data.events[source.ID])
	return info
}
