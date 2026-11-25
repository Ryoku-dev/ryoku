package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/protocol"
)

func TestPersonalRoutineWakesOnlyWhenItsEnabledFeatureIsDue(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	now := time.Now()
	for _, check := range []struct {
		name     string
		kind     string
		preset   string
		features map[string]bool
		enabled  bool
		anchor   time.Time
		want     bool
	}{
		{"feature off", protocol.RoutineAsk, "", nil, true, now.Add(-time.Hour), false},
		{"due question", protocol.RoutineAsk, "", map[string]bool{"schedules": true}, true, now.Add(-time.Hour), true},
		{"not due", protocol.RoutineAsk, "", map[string]bool{"schedules": true}, true, now.Add(time.Hour), false},
		{"turned off", protocol.RoutineAsk, "", map[string]bool{"schedules": true}, false, now.Add(-time.Hour), false},
		{"research off", protocol.RoutineDream, "research", map[string]bool{"schedules": true}, true, now.Add(-time.Hour), false},
		{"research due", protocol.RoutineDream, "research", map[string]bool{"schedules": true, "research": true}, true, now.Add(-time.Hour), true},
	} {
		t.Run(check.name, func(t *testing.T) {
			state := struct {
				Routines []protocol.Routine `json:"routines"`
			}{Routines: []protocol.Routine{{ID: "personal-question", Team: protocol.RoutinePersonal, Kind: check.kind, Preset: check.preset, Enabled: check.enabled, Schedule: "*/15 * * * *", Created: check.anchor, Updated: check.anchor}}}
			data, err := json.Marshal(state)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.MkdirAll(paths.TeamsDir(), 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(paths.TeamsDir(), "state.json"), data, 0600); err != nil {
				t.Fatal(err)
			}
			if got := routinesOn(check.features, testWakeHorizon(t)); got != check.want {
				t.Fatalf("wake = %v, want %v", got, check.want)
			}
		})
	}
}

func TestExistingTeamRoutineStillWakesWithNewFeaturesOff(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	if err := os.MkdirAll(paths.TeamsDir(), 0700); err != nil {
		t.Fatal(err)
	}
	data := []byte(`{"teams":[{"id":"team-review","paused":false}],"routines":[{"team":"team-review","enabled":true}]}`)
	if err := os.WriteFile(filepath.Join(paths.TeamsDir(), "state.json"), data, 0600); err != nil {
		t.Fatal(err)
	}
	if !routinesOn(nil, testWakeHorizon(t)) {
		t.Fatal("the existing team routine no longer wakes the daemon")
	}
}

func TestPersonalRoutineWakesBeforeAnOffCycleDeadline(t *testing.T) {
	now := time.Date(2026, time.November, 18, 9, 0, 0, 0, time.Local)
	routine := protocol.Routine{Schedule: "15 9 * * *", Created: now.Add(-10 * time.Minute), Updated: now.Add(-10 * time.Minute)}
	if !routineWakeSoon(routine, now, testWakeHorizon(t)) {
		t.Fatal("the 09:00 wake did not start the precise timer for the 09:15 question")
	}
	routine.Schedule = "45 9 * * *"
	if routineWakeSoon(routine, now, testWakeHorizon(t)) {
		t.Fatal("a deadline beyond the next wake interval started the daemon early")
	}
}
