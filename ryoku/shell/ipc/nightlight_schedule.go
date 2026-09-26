package main

import (
	"encoding/json"
	"log"
	"os"
	"time"
)

// The night light can follow the real sun: on from sunset+margin until
// sunrise-margin, off outside it. The times are the weather poll's own
// sunrise/sunset (sun.go), so the schedule is location-correct without a
// second geolocation. The mode persists in a state file beside the temp and
// marker files, which the watcher already owns; this loop only edges.
//
// A manual toggle while the schedule is on is honoured until the next edge:
// the loop compares against its own last desire (schedLastDesire), not the
// live state, so it never fights the user every tick. Turning the schedule on
// applies it immediately; turning it off leaves the current state alone.

const (
	nlSchedOff = "off"
	nlSchedSun = "sun"

	nlTickEvery = 5 * time.Minute

	// nlDefaultMarginMin eases the warm window in after sunset and off before
	// sunrise, so it does not snap on right at the civil edge.
	nlDefaultMarginMin = 60
)

type nightlightSchedule struct {
	Mode      string `json:"mode"`
	MarginMin int    `json:"marginMin"`
}

// scheduleMode reports the live mode under the lock.
func (n *nightlightState) scheduleMode() string {
	n.schedMu.Lock()
	defer n.schedMu.Unlock()
	return n.schedMode
}

// loadSchedule restores the persisted mode at daemon start. A missing or
// unreadable file is the default: schedule off.
func (n *nightlightState) loadSchedule() {
	if n.schedFile == "" {
		return
	}
	b, err := os.ReadFile(n.schedFile)
	if err != nil {
		return
	}
	var s nightlightSchedule
	if json.Unmarshal(b, &s) != nil {
		return
	}
	if s.Mode != nlSchedSun {
		s.Mode = nlSchedOff
	}
	if s.MarginMin < 0 {
		s.MarginMin = 0
	}
	n.schedMu.Lock()
	n.schedMode, n.schedMarginMin = s.Mode, s.MarginMin
	n.schedLastDesire = false
	n.schedMu.Unlock()
}

// setSchedule moves the mode and margin, persisting the pair. An unknown mode
// is refused by name so a UI bug cannot silently disable the schedule.
func (n *nightlightState) setSchedule(mode string, marginMin *int) error {
	if mode != nlSchedOff && mode != nlSchedSun {
		return errNightlightSchedule
	}
	n.schedMu.Lock()
	if mode == nlSchedSun {
		n.schedLastDesire = false
	}
	n.schedMode = mode
	if marginMin != nil && *marginMin >= 0 {
		n.schedMarginMin = *marginMin
	}
	out := nightlightSchedule{Mode: n.schedMode, MarginMin: n.schedMarginMin}
	n.schedMu.Unlock()
	if n.schedFile == "" {
		return nil
	}
	b, err := json.Marshal(out)
	if err != nil {
		return err
	}
	return os.WriteFile(n.schedFile, b, 0o644)
}

// runSchedule ticks the follow-the-sun edge check for the life of the daemon.
// The 5-minute cadence matches the finest resolution the window needs (sun
// times are whole minutes); the weather poll keeps the sun window fresh.
func (n *nightlightState) runSchedule() {
	n.tickSchedule()
	tick := time.NewTicker(nlTickEvery)
	defer tick.Stop()
	for range tick.C {
		n.tickSchedule()
	}
}

// tickSchedule applies the schedule when it is on and the desired state moved.
// It never acts while the schedule is off, and it never repeats an intent the
// last tick already sent, so a healthy session sees one toggle per edge.
func (n *nightlightState) tickSchedule() {
	n.schedMu.Lock()
	mode, margin := n.schedMode, n.schedMarginMin
	last := n.schedLastDesire
	n.schedMu.Unlock()
	if mode != nlSchedSun {
		return
	}
	sunrise, sunset, ok := daySun.window()
	if !ok {
		return // no sun data yet: fail safe, do nothing
	}
	want := inNocturnalWindow(time.Now(), sunrise, sunset, time.Duration(margin)*time.Minute)
	if want == last {
		return
	}
	var err error
	if want {
		err = n.intent("on")
	} else {
		err = n.intent("off")
	}
	if err != nil {
		log.Printf("ryoku-shell: night light schedule tick failed: %v", err)
		return // retry on the next tick: do not record an action that did not land
	}
	n.schedMu.Lock()
	n.schedLastDesire = want
	n.schedMu.Unlock()
}
