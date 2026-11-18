package teams

import (
	i18n "ryoku-i18n"

	"ryoku-rashin/internal/protocol"
)

// SettingsChanged follows a change of the teams settings. Pausing every team
// stops their open tasks and routines; letting them work again starts what
// waits. A shorter time to keep finished tasks prunes at once, and limits
// and the wake unit apply again.
func (s *Service) SettingsChanged(settings protocol.Settings) {
	next := settings.Teams
	s.mu.Lock()
	prev := s.teamsSettings
	s.teamsSettings = next
	before := s.features
	s.features = readTeamFeatures(settings)
	s.mu.Unlock()
	s.followFeatureSettings(before, readTeamFeatures(settings))
	switch {
	case next.Paused && !prev.Paused:
		s.pauseEvery()
	case !next.Paused && prev.Paused:
		s.record(protocol.Activity{
			Actor: protocol.InitiatorYou, Action: "teams.resume", Target: i18n.T("Every team"), OK: true,
			Initiator: protocol.InitiatorYou, Detail: i18n.T("Let every team work again."),
		})
	}
	if next.KeepDays != prev.KeepDays {
		s.pruneTasks(s.background, protocol.TeamPruneParams{}, protocol.InitiatorRashin)
	}
	s.clock.rearm()
	if next.Paused != prev.Paused {
		// what a pause changes shows in every team's next routine
		for _, team := range s.state.teams() {
			s.emitTeam(team.ID)
		}
		for _, r := range s.listRoutines("").Routines {
			s.broadcast(protocol.EvtRoutine, r)
		}
	}
	s.pump()
}

// pauseEvery stops every team's open tasks once every team is paused.
func (s *Service) pauseEvery() {
	for _, team := range s.state.teams() {
		s.stopTeamTasks(team.ID, i18n.T("Every team was paused."))
	}
	s.record(protocol.Activity{
		Actor: protocol.InitiatorYou, Action: "teams.pause", Target: i18n.T("Every team"), OK: true,
		Initiator: protocol.InitiatorYou, Detail: i18n.T("Paused every team and stopped their tasks."),
	})
}

type teamFeatures struct {
	research, schedules, setupcards bool
}

func readTeamFeatures(settings protocol.Settings) teamFeatures {
	return teamFeatures{
		research: settings.Features["research"], schedules: settings.Features["schedules"], setupcards: settings.Features["setupcards"],
	}
}

func (s *Service) followFeatureSettings(before, next teamFeatures) {
	if before.setupcards && !next.setupcards {
		s.skipSetupCards()
	}
	if before.research && !next.research {
		for _, team := range s.state.teams() {
			if !s.dreamAvailable(team) {
				s.stopTeamTasks(team.ID, i18n.T("Deep research was turned off."))
			}
		}
	}
	if before.schedules && !next.schedules {
		s.stopScheduledQuestions()
	} else if !before.schedules && next.schedules {
		s.recoverRoutineHistory()
	}
}
