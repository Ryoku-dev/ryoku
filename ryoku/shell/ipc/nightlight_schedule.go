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
// second geolocation. It can also follow the wall clock: on from a chosen
// start minute until a chosen stop minute, wrapping midnight, which is what
// a user wants when they keep fixed bed and wake hours. The mode persists in
// a state file beside the temp and marker files, which the watcher already
// owns; this loop only edges.
//
// A manual toggle while a schedule is on is honoured until the next edge:
// the loop compares against its own last desire (schedLastDesire), not the
// live state, so it never fights the user every tick. Turning a schedule on
// applies it immediately; turning it off leaves the current state alone.

const (
	nlSchedOff   = "off"
	nlSchedSun   = "sun"
	nlSchedClock = "clock"

	nlTickEvery        = 5 * time.Minute
	nlSunDataReconcile = 5 * time.Minute
	nlSlowReconcile    = 24 * time.Hour

	// nlDefaultMarginMin eases the warm window in after sunset and off before
	// sunrise, so it does not snap on right at the civil edge.
	nlDefaultMarginMin = 60

	// The clock schedule's defaults: 22:00 to 07:00, the conventional
	// bed-to-wake window.
	nlDefaultClockStart = 22 * 60
	nlDefaultClockStop  = 7 * 60
)

type nightlightSchedule struct {
	Mode      string `json:"mode"`
	MarginMin int    `json:"marginMin"`
	StartMin  *int   `json:"startMin"`
	StopMin   *int   `json:"stopMin"`
}

// inClockWindow reports whether the wall-clock minute of now sits in the
// half-open window [start, stop) of minutes-from-midnight. A start past its
// stop wraps midnight (the usual 22:00 to 07:00 bed window); equal bounds
// mean the light is wanted all day.
func inClockWindow(now time.Time, start, stop int) bool {
	m := now.Hour()*60 + now.Minute()
	if start == stop {
		return true
	}
	if start < stop {
		return m >= start && m < stop
	}
	return m >= start || m < stop
}

// scheduleMode reports the live mode under the lock.
func (n *nightlightState) scheduleMode() string {
	n.schedMu.Lock()
	defer n.schedMu.Unlock()
	return n.schedMode
}

func (n *nightlightState) kickSchedule() {
	select {
	case n.schedKick <- struct{}{}:
	default:
	}
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
	if s.Mode != nlSchedSun && s.Mode != nlSchedClock {
		s.Mode = nlSchedOff
	}
	if s.MarginMin < 0 {
		s.MarginMin = 0
	}
	n.schedMu.Lock()
	n.schedMode, n.schedMarginMin = s.Mode, s.MarginMin
	if s.StartMin != nil {
		n.schedStartMin = clampClockMin(*s.StartMin)
	}
	if s.StopMin != nil {
		n.schedStopMin = clampClockMin(*s.StopMin)
	}
	n.schedLastDesire = false
	n.schedMu.Unlock()
}

// clampClockMin folds a requested clock minute into the day; a wild value
// (a UI bug, a hand-edited state file) clamps rather than wrapping, so the
// window never silently shifts by a day.
func clampClockMin(m int) int {
	if m < 0 {
		return 0
	}
	if m > 23*60+59 {
		return 23*60 + 59
	}
	return m
}

// setSchedule moves the mode and, when given, the sun margin or the clock
// window's bounds, persisting the whole record. An unknown mode is refused by
// name so a UI bug cannot silently disable the schedule.
func (n *nightlightState) setSchedule(mode string, marginMin, startMin, stopMin *int) error {
	if mode != nlSchedOff && mode != nlSchedSun && mode != nlSchedClock {
		return errNightlightSchedule
	}
	defer n.kickSchedule()
	n.schedMu.Lock()
	if mode != n.schedMode {
		// A fresh schedule re-decides at the next tick, even against a
		// manual toggle: the user just asked for this window.
		n.schedLastDesire = false
	}
	n.schedMode = mode
	if marginMin != nil && *marginMin >= 0 {
		n.schedMarginMin = *marginMin
	}
	if startMin != nil {
		n.schedStartMin = clampClockMin(*startMin)
	}
	if stopMin != nil {
		n.schedStopMin = clampClockMin(*stopMin)
	}
	start, stop := n.schedStartMin, n.schedStopMin
	out := nightlightSchedule{
		Mode: n.schedMode, MarginMin: n.schedMarginMin,
		StartMin: &start, StopMin: &stop,
	}
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

// runSchedule sleeps to the next actual clock or sun boundary. Schedule edits
// rouse it immediately; unknown sun data retains a slow reconciliation until
// the weather worker has observed a window.
func (n *nightlightState) runSchedule() {
	timer := time.NewTimer(0)
	defer timer.Stop()
	for {
		n.tickSchedule()
		wait := n.nextScheduleWait(time.Now())
		if !timer.Stop() {
			select {
			case <-timer.C:
			default:
			}
		}
		timer.Reset(wait)
		select {
		case <-n.schedKick:
		case <-timer.C:
		}
	}
}

func (n *nightlightState) nextScheduleWait(now time.Time) time.Duration {
	n.schedMu.Lock()
	mode, margin := n.schedMode, n.schedMarginMin
	start, stop := n.schedStartMin, n.schedStopMin
	n.schedMu.Unlock()
	switch mode {
	case nlSchedClock:
		return nextClockScheduleEdge(now, start, stop)
	case nlSchedSun:
		sunrise, sunset, ok := daySun.window()
		if !ok {
			wantSunWindow()
			return nlSunDataReconcile
		}
		return nextSunScheduleEdge(now, sunrise, sunset, time.Duration(margin)*time.Minute)
	default:
		return nlSlowReconcile
	}
}

func nextClockScheduleEdge(now time.Time, start, stop int) time.Duration {
	if start == stop {
		return nlSlowReconcile
	}
	return nextMinuteEdge(now, start, stop)
}

func nextMinuteEdge(now time.Time, minutes ...int) time.Duration {
	best := nlSlowReconcile
	for _, minute := range minutes {
		for day := 0; day <= 1; day++ {
			date := now.AddDate(0, 0, day)
			at := time.Date(date.Year(), date.Month(), date.Day(),
				minute/60, minute%60, 0, 0, now.Location())
			if !at.After(now) {
				continue
			}
			if wait := at.Sub(now); wait < best {
				best = wait
			}
		}
	}
	return best
}

func nextSunScheduleEdge(now, sunrise, sunset time.Time, margin time.Duration) time.Duration {
	best := nlSlowReconcile
	for day := 0; day <= 1; day++ {
		date := now.AddDate(0, 0, day)
		rise := time.Date(date.Year(), date.Month(), date.Day(),
			sunrise.Hour(), sunrise.Minute(), 0, 0, now.Location()).Add(-margin)
		set := time.Date(date.Year(), date.Month(), date.Day(),
			sunset.Hour(), sunset.Minute(), 0, 0, now.Location()).Add(margin)
		for _, at := range []time.Time{rise, set} {
			if at.After(now) && at.Sub(now) < best {
				best = at.Sub(now)
			}
		}
	}
	return best
}

// tickSchedule applies the schedule when one is on and the desired state
// moved. It never acts while the schedule is off, and it never repeats an
// intent the last tick already sent, so a healthy session sees one toggle per
// edge. The sun window fails safe with no weather data; the clock window needs
// nothing beyond the wall clock.
func (n *nightlightState) tickSchedule() {
	n.schedMu.Lock()
	mode, margin := n.schedMode, n.schedMarginMin
	start, stop := n.schedStartMin, n.schedStopMin
	last := n.schedLastDesire
	n.schedMu.Unlock()
	var want bool
	switch mode {
	case nlSchedSun:
		sunrise, sunset, ok := daySun.window()
		if !ok {
			return // no sun data yet: fail safe, do nothing
		}
		want = inNocturnalWindow(time.Now(), sunrise, sunset, time.Duration(margin)*time.Minute)
	case nlSchedClock:
		want = inClockWindow(time.Now(), start, stop)
	default:
		return
	}
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
