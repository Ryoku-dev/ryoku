package demo

import (
	"encoding/json"

	"ryoku-rashin/internal/protocol"
)

// a page only moves when it hears the daemon say so, a silent demo shows switches that snap back
func (s *server) announce(out *outbox, method string) {
	events := make(map[string]any, 2)
	s.mu.Lock()
	switch method {
	case protocol.MSettingsSet:
		s.data.snapshot.Status.API = demoAPI(s.data.snapshot.Settings.API)
		events[protocol.EvtSettingsChanged] = s.data.snapshot.Settings
		events[protocol.EvtAPIChanged] = s.data.snapshot.Status.API
	case protocol.MProjectsAdd, protocol.MProjectsRemove:
		projects := append([]protocol.ProjectInfo(nil), s.data.snapshot.Projects...)
		events[protocol.EvtProjectsChanged] = protocol.ProjectsResult{Projects: projects}
	case protocol.MProvidersEnable, protocol.MProvidersAddKey, protocol.MProvidersRemoveKey:
		providers := append([]protocol.ProviderInfo(nil), s.data.snapshot.Providers...)
		events[protocol.EvtProvidersChanged] = protocol.ProvidersResult{Providers: providers}
	}
	s.mu.Unlock()
	for name, value := range events {
		if wire, err := json.Marshal(value); err == nil {
			_ = out.send(protocol.Event{Event: name, Data: wire})
		}
	}
}
