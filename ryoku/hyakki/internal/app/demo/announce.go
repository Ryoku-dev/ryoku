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
	case protocol.MProvidersEnable, protocol.MProvidersAddKey, protocol.MProvidersRemoveKey, protocol.MProvidersLimits,
		protocol.MAccountStatus, protocol.MAccountSignIn, protocol.MAccountCancel, protocol.MAccountRefresh, protocol.MAccountSignOut:
		providers := append([]protocol.ProviderInfo(nil), s.data.snapshot.Providers...)
		if ember, ok := s.emberProvider(); ok {
			providers = append(providers, ember)
		}
		events[protocol.EvtProvidersChanged] = protocol.ProvidersResult{Providers: providers}
	case protocol.MLocalDownload, protocol.MLocalCancel, protocol.MLocalRemove, protocol.MLocalAdd,
		protocol.MLocalRuntimeInstall, protocol.MLocalStart, protocol.MLocalStop:
		// Local methods enqueue their own state and progress in wire order.
	case protocol.MLearningSkillDelete, protocol.MLearningDecisionDelete, protocol.MLearningRuleSave,
		protocol.MLearningRuleDelete, protocol.MLearningGrievancesClear:
		// Learning methods name the part they changed.
	case protocol.MTeamsCreate, protocol.MTeamsUpdate, protocol.MTeamsDelete, protocol.MTeamsPause, protocol.MTeamsImport,
		protocol.MTeamsMemberConfigSave, protocol.MTeamsMemberConfigRestore,
		protocol.MTeamsTaskCreate, protocol.MTeamsTaskStop, protocol.MTeamsTaskRetry, protocol.MTeamsTaskReassign, protocol.MTeamsTaskRead, protocol.MTeamsTaskDelete,
		protocol.MTeamsRoutineCreate, protocol.MTeamsRoutineUpdate, protocol.MTeamsRoutineDelete, protocol.MTeamsRoutineRun,
		protocol.MTeamsApprovalDecide, protocol.MTeamsPrune, protocol.MVaultAdd, protocol.MVaultUpdate, protocol.MVaultRemove, protocol.MVaultMail,
		protocol.MTeamsDreamCreate, protocol.MTeamsDreamAnswer:
		// each change queued the team, task, approval, routine, vault and session events the daemon would broadcast, in order
		teams = s.takeTeamsEvents()
	}
	// memory writes and settings that moved memory are heard after the response
	teams = append(teams, s.takeMemoryEvents()...)
	queued := s.takeEvents()
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
	for _, event := range queued {
		if out.send(event) != nil {
			return
		}
	}
}

func (s *server) queueEvent(name string, value any) {
	if raw, err := json.Marshal(value); err == nil {
		s.events = append(s.events, protocol.Event{Event: name, Data: raw})
	}
}

func (s *server) takeEvents() []protocol.Event {
	events := s.events
	s.events = nil
	return events
}
