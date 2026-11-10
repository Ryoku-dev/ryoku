package demo

import (
	"encoding/json"
	"time"

	"ryoku-i18n"

	"ryoku-rashin/internal/protocol"
)

// appends one event to a session and tells the client, as the daemon does for anything a runner emits
func (s *server) emit(out *outbox, id, kind string, data any) {
	s.mu.Lock()
	if _, known := s.session(id); !known {
		s.mu.Unlock()
		return
	}
	seq := int64(1)
	if events := s.data.events[id]; len(events) > 0 {
		seq = events[len(events)-1].Seq + 1
	}
	raw, _ := json.Marshal(data)
	made := protocol.SessionEvent{Seq: seq, TS: time.Now(), Kind: kind, Data: raw}
	s.data.events[id] = append(s.data.events[id], made)
	s.mu.Unlock()
	wire, _ := json.Marshal(protocol.SessionEventData{Session: id, SessionEvent: made})
	_ = out.send(protocol.Event{Event: protocol.EvtSessionEvent, Data: wire})
}

func (s *server) tell(out *outbox, id string) {
	s.mu.Lock()
	info, known := s.session(id)
	s.mu.Unlock()
	if !known {
		return
	}
	wire, _ := json.Marshal(info)
	_ = out.send(protocol.Event{Event: protocol.EvtSessionInfo, Data: wire})
}

func (s *server) has(id, kind string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, event := range s.data.events[id] {
		if event.Kind == kind {
			return true
		}
	}
	return false
}

// a harness that comes up says what it will use, which is all starting one early is for
func (s *server) wake(out *outbox, raw json.RawMessage) {
	var params protocol.SessionRef
	if decode(raw, &params) != nil {
		return
	}
	s.mu.Lock()
	info, known := s.session(params.ID)
	if known {
		info.Alive, info.Model, info.Mode = true, "anthropic/opus", "default"
		s.replaceSession(info)
	}
	s.mu.Unlock()
	if !known || info.Kind != protocol.KindAgent {
		return
	}
	if !s.has(params.ID, protocol.EvModel) {
		s.emit(out, params.ID, protocol.EvModel, protocol.ChoiceData{Current: info.Model, Options: demoModels()})
		s.emit(out, params.ID, protocol.EvMode, protocol.ChoiceData{Current: info.Mode, Options: []protocol.Choice{
			{ID: "default", Name: i18n.T("Default")}, {ID: "plan", Name: i18n.T("Plan"), Description: i18n.T("Plans before it edits")},
		}})
		s.emit(out, params.ID, protocol.EvOptions, protocol.OptionsData{Options: demoOptions("auto")})
	}
	s.tell(out, params.ID)
}

// the select a choice was made in moves only when the daemon says the choice took
func (s *server) chose(out *outbox, method string, raw json.RawMessage) {
	switch method {
	case protocol.MSessionSetModel:
		var params protocol.SetModelParams
		if decode(raw, &params) == nil {
			s.rechoose(out, params.ID, protocol.EvModel, params.Model)
		}
	case protocol.MSessionSetMode:
		var params protocol.SetModeParams
		if decode(raw, &params) == nil {
			s.rechoose(out, params.ID, protocol.EvMode, params.Mode)
		}
	case protocol.MSessionSetOption:
		var params protocol.SetOptionParams
		if decode(raw, &params) == nil {
			s.emit(out, params.ID, protocol.EvOptions, protocol.OptionsData{Options: demoOptions(params.Value)})
		}
	case protocol.MSessionStop:
		var params protocol.SessionRef
		if decode(raw, &params) == nil {
			s.tell(out, params.ID)
		}
	}
}

// says the latest choice of kind again with a new current, keeping what was offered
func (s *server) rechoose(out *outbox, id, kind, current string) {
	s.mu.Lock()
	var latest protocol.ChoiceData
	found := false
	for _, event := range s.data.events[id] {
		if event.Kind == kind && json.Unmarshal(event.Data, &latest) == nil {
			found = true
		}
	}
	s.mu.Unlock()
	if found {
		latest.Current = current
		s.emit(out, id, kind, latest)
	}
	s.tell(out, id)
}
