package demo

import (
	"encoding/json"

	"ryoku-rashin/internal/protocol"
)

// a page only moves when it hears the daemon say so, a silent demo shows switches that snap back
func (s *server) announce(out *outbox, method string) {
	events := make(map[string]any, 2)
	var teams []protocol.Event
	s.mu.Lock()
	switch method {
	case protocol.MSettingsSet:
		s.data.snapshot.Status.API = demoAPI(s.data.snapshot.Settings.API)
		events[protocol.EvtSettingsChanged] = s.data.snapshot.Settings
		events[protocol.EvtAPIChanged] = s.data.snapshot.Status.API
		// a local provider's address lives in the settings and shows on its row
		events[protocol.EvtProvidersChanged] = protocol.ProvidersResult{Providers: append([]protocol.ProviderInfo(nil), s.data.snapshot.Providers...)}
		// pausing every team stops what works, and the pages hear each task stop
		teams = s.takeTeamsEvents()
	case protocol.MProjectsAdd, protocol.MProjectsRemove, protocol.MProjectsSettings:
		projects := append([]protocol.ProjectInfo(nil), s.data.snapshot.Projects...)
		events[protocol.EvtProjectsChanged] = protocol.ProjectsResult{Projects: projects}
	case protocol.MHarnessesProbe, protocol.MHarnessesTeamsOpen, protocol.MHarnessesTeamsDone:
		harnesses := append([]protocol.HarnessInfo(nil), s.data.snapshot.Harnesses...)
		events[protocol.EvtHarnessesChanged] = protocol.HarnessesResult{Harnesses: harnesses}
	case protocol.MProvidersEnable, protocol.MProvidersAddKey, protocol.MProvidersRemoveKey:
		providers := append([]protocol.ProviderInfo(nil), s.data.snapshot.Providers...)
		events[protocol.EvtProvidersChanged] = protocol.ProvidersResult{Providers: providers}
	case protocol.MTeamsCreate, protocol.MTeamsUpdate, protocol.MTeamsDelete, protocol.MTeamsPause, protocol.MTeamsImport,
		protocol.MTeamsTaskCreate, protocol.MTeamsTaskStop, protocol.MTeamsTaskRetry, protocol.MTeamsTaskRead, protocol.MTeamsTaskDelete,
		protocol.MTeamsRoutineCreate, protocol.MTeamsRoutineUpdate, protocol.MTeamsRoutineDelete, protocol.MTeamsRoutineRun,
		protocol.MTeamsApprovalDecide, protocol.MTeamsPrune, protocol.MVaultAdd, protocol.MVaultUpdate, protocol.MVaultRemove, protocol.MVaultMail:
		// each change queued the team, task, approval, routine, vault and session events the daemon would broadcast, in order
		teams = s.takeTeamsEvents()
	}
	// memory writes and settings that moved memory are heard after the response
	teams = append(teams, s.takeMemoryEvents()...)
	s.mu.Unlock()
	for name, value := range events {
		if wire, err := json.Marshal(value); err == nil {
			_ = out.send(protocol.Event{Event: name, Data: wire})
		}
	}
	for _, event := range teams {
		if out.send(event) != nil {
			return
		}
	}
}
