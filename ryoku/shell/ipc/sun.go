package main

import (
	"sync"
	"time"
)

// sun.go keeps the real sunrise and sunset the weather poll already fetches for
// the widget, so features that follow the day -- the night light schedule and
// the sun-driven light/dark theme mode -- read the same location-correct times
// instead of computing their own geolocation. The weather frame carries only
// display strings, so the raw ISO times are observed at the fetch site.

// sunState is the last observed sunrise/sunset pair for the user's location.
// Absent data (no location configured, offline, weather never loaded) leaves
// it unknown, and every consumer fails safe to its non-sun behaviour.
type sunState struct {
	mu       sync.Mutex
	sunrise  time.Time
	sunset   time.Time
	observed bool
}

// daySun is the package-wide window: the daemon's own state (d.sun) points at
// it, and the theme path (which resolves the mode without a daemon handle)
// reads it directly. One box, one location, one window.
var daySun = &sunState{}

// windowObserved reports whether a sun window has ever been observed.
func (s *sunState) windowObserved() bool {
	_, _, ok := s.window()
	return ok
}

// observe records a day's sunrise and sunset from the raw forecast strings
// (local ISO, "2006-01-02T15:04"). A day the strings cannot be parsed for is
// skipped rather than half-recorded.
func (s *sunState) observe(sunriseISO, sunsetISO string) {
	sr, ok1 := parseISOTime(sunriseISO)
	ss, ok2 := parseISOTime(sunsetISO)
	if !ok1 || !ok2 {
		return
	}
	s.mu.Lock()
	s.sunrise, s.sunset, s.observed = sr, ss, true
	s.mu.Unlock()
}

// window returns today's sunrise and sunset, in the local timezone the weather
// service resolved for the configured location.
func (s *sunState) window() (sunrise, sunset time.Time, ok bool) {
	if s == nil {
		return time.Time{}, time.Time{}, false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.sunrise, s.sunset, s.observed
}

// isDaytime reports whether now falls inside the recorded sunrise..sunset
// window. An unknown window reads as not-daytime is never asked: callers
// check ok first.
func isDaytime(now, sunrise, sunset time.Time) bool {
	return !now.Before(sunrise) && now.Before(sunset)
}

// inNocturnalWindow reports whether now is inside the warm-light window: from
// sunset+margin until sunrise-margin of the NEXT day. Sunrise and sunset are
// read per calendar day (the forecast reports today's pair; the values barely
// move day to day), so the window spans midnight by pairing yesterday's or
// today's sunset with today's or tomorrow's sunrise. A margin that would make
// the window negative (margin*2 longer than the night) collapses it to closed.
func inNocturnalWindow(now, sunriseToday, sunsetToday time.Time, margin time.Duration) bool {
	if margin < 0 {
		margin = 0
	}
	day := func(d time.Duration) (sr, ss time.Time) {
		base := now.Add(d * 24 * time.Hour)
		sr = time.Date(base.Year(), base.Month(), base.Day(),
			sunriseToday.Hour(), sunriseToday.Minute(), 0, 0, base.Location())
		ss = time.Date(base.Year(), base.Month(), base.Day(),
			sunsetToday.Hour(), sunsetToday.Minute(), 0, 0, base.Location())
		return
	}
	// The night that started yesterday ends at this morning's sunrise.
	_, sunsetY := day(-1)
	sunriseT, _ := day(0)
	if start, end := sunsetY.Add(margin), sunriseT.Add(-margin); !end.Before(start) {
		if !now.Before(start) && now.Before(end) {
			return true
		}
	}
	// The night that starts tonight ends at tomorrow's sunrise.
	sunriseT, sunsetT := day(0)
	sunriseN, _ := day(1)
	if start, end := sunsetT.Add(margin), sunriseN.Add(-margin); !end.Before(start) {
		if !now.Before(start) && now.Before(end) {
			return true
		}
	}
	return false
}
