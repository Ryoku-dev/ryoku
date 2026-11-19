package main

import (
	"strings"
	"time"
)

func (s *sightService) focusedApp() string {
	if s.daemon == nil {
		return ""
	}
	s.daemon.wmMu.Lock()
	defer s.daemon.wmMu.Unlock()
	for _, window := range s.daemon.wmWindows {
		if window.FocusOrder == 0 {
			return window.AppID
		}
	}
	return ""
}

func (s *sightService) quietBlockedLocked(app string, now time.Time) bool {
	if s.snoozedApp != "" && ((!s.snoozedUntil.IsZero() && !now.Before(s.snoozedUntil)) || (s.snoozedUntil.IsZero() && app != s.snoozedApp)) {
		s.snoozedApp, s.snoozedUntil = "", time.Time{}
	}
	if !s.options.feature("quiet") {
		return false
	}
	if app != "" && app == s.snoozedApp {
		return true
	}
	for _, excluded := range s.options.Quiet.Apps {
		if app != "" && app == excluded {
			return true
		}
	}
	return false
}

func (s *sightService) quietBlocked() bool {
	app := s.focusedApp()
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.quietBlockedLocked(app, time.Now())
}

func (s *sightService) quietNow(app string) string {
	app = strings.TrimSpace(app)
	if app == "" {
		app = s.focusedApp()
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.options.feature("quiet") {
		return app
	}
	s.snoozedApp, s.snoozedUntil = app, time.Time{}
	if s.options.Quiet.Snooze != 0 && app != "" {
		s.snoozedUntil = time.Now().Add(time.Duration(s.options.Quiet.Snooze) * time.Minute)
	}
	s.highlightEpoch++
	s.inplaceEpoch++
	s.frame.Highlight, s.frame.InPlace = nil, nil
	s.publishLocked()
	return app
}
