package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	i18n "ryoku-i18n"
	"ryoku-rashin/internal/client"
	"ryoku-rashin/internal/config"
	"ryoku-rashin/internal/cron"
	"ryoku-rashin/internal/oni/feature"
	"ryoku-rashin/internal/oni/remind"
	"ryoku-rashin/internal/oni/spec"
	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/protocol"
)

func init() {
	register(&Command{Name: "routines", Group: "lifecycle", Usage: "wake", Summary: i18n.T("Start the daemon when a routine is on or a reminder is due soon."), Run: runRoutines})
}

// runRoutines wakes the daemon for routines and Oni's reminders after login,
// a reboot or a daemon exit; the wake unit runs it every half hour, through
// systemd's timer or the runit worker alike. It decides from the teams'
// state and the reminders file on disk, so it never starts the daemon for
// nothing.
func runRoutines(args []string) int {
	if len(args) != 1 || args[0] != "wake" {
		return usageError("routines wake")
	}
	settings, err := config.Load()
	if err != nil {
		return report(err)
	}
	horizon, err := wakeHorizon()
	if err != nil {
		return report(err)
	}
	if !wakeFor(settings, time.Now(), horizon) {
		return 0
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	conn, err := client.Dial(ctx, client.Options{Client: protocol.ClientCLI, Start: true})
	if err != nil {
		return report(err)
	}
	conn.Close()
	return 0
}

// wakeHorizon is how soon a routine or a reminder must fall due for the
// wake path to start the daemon: the spec's reflex.reminders.wakeHorizonMin,
// the wake unit's own period.
func wakeHorizon() (time.Duration, error) {
	s, err := spec.Load()
	if err != nil {
		return 0, err
	}
	return s.Reflex.Reminders.WakeHorizon(), nil
}

// wakeFor reports whether the daemon should start now: Hyakki is on, and a
// routine is on or a timer or reminder falls due within horizon.
func wakeFor(settings protocol.Settings, now time.Time, horizon time.Duration) bool {
	if !settings.Enabled {
		return false
	}
	return routinesOn(settings.Features, horizon) || remindersDue(settings, now, horizon)
}

// remindersDue reports a timer or reminder of a kind that is on due within
// horizon of now, or already due: a timer while Oni and voice are on, a
// reminder while Oni, reflex and its reminders switch are on. It reads only
// the kinds and due times kept.
func remindersDue(settings protocol.Settings, now time.Time, horizon time.Duration) bool {
	oni := protocol.EffectiveOni(settings)
	on := map[string]bool{
		remind.KindTimer:    feature.Decide(feature.Request{Feature: feature.Voice}, oni).Allow,
		remind.KindReminder: feature.Decide(feature.Request{Feature: feature.Reminders}, oni).Allow,
	}
	if !on[remind.KindTimer] && !on[remind.KindReminder] {
		return false
	}
	rs, err := remind.NewStore("").List()
	if err != nil {
		return false
	}
	for _, r := range rs {
		if on[r.Kind] && !r.Due.After(now.Add(horizon)) {
			return true
		}
	}
	return false
}

// routinesOn reports any routine that is on in a team that is not paused,
// or a personal routine due within horizon, read from the teams' state file
// without the daemon.
func routinesOn(features map[string]bool, horizon time.Duration) bool {
	raw, err := os.ReadFile(filepath.Join(paths.TeamsDir(), "state.json"))
	if err != nil {
		return false
	}
	var state struct {
		Teams []struct {
			ID     string `json:"id"`
			Paused bool   `json:"paused"`
		} `json:"teams"`
		Routines []protocol.Routine `json:"routines"`
	}
	if json.Unmarshal(raw, &state) != nil {
		return false
	}
	working := make(map[string]bool, len(state.Teams))
	for _, team := range state.Teams {
		working[team.ID] = !team.Paused
	}
	now := time.Time{}
	if features["schedules"] {
		now = time.Now()
	}
	for _, r := range state.Routines {
		if !r.Enabled {
			continue
		}
		if r.Kind != "" && r.Kind != protocol.RoutineTask {
			if !features["schedules"] || (r.Kind != protocol.RoutineAsk && r.Kind != protocol.RoutineDream) || (r.Preset == "research" && !features["research"]) {
				continue
			}
			if r.Team == protocol.RoutinePersonal {
				if routineWakeSoon(r, now, horizon) {
					return true
				}
				continue
			}
		}
		if working[r.Team] {
			return true
		}
	}
	return false
}

func routineWakeSoon(r protocol.Routine, now time.Time, horizon time.Duration) bool {
	anchor := r.Updated
	if r.Created.After(anchor) {
		anchor = r.Created
	}
	if r.LastRun != nil && r.LastRun.After(anchor) {
		anchor = *r.LastRun
	}
	if anchor.IsZero() {
		return false
	}
	schedule, err := cron.Parse(r.Schedule)
	if err != nil {
		return false
	}
	next := schedule.Next(anchor)
	return !next.IsZero() && !next.After(now.Add(horizon))
}
