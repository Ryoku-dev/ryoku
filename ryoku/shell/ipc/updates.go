package main

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type updatesState struct {
	topic  *stateTopic
	wake   chan struct{}
	demand chan struct{}
	quit   chan struct{}

	now      func() time.Time
	after    func(time.Duration) (<-chan time.Time, func() bool)
	interval func() time.Duration
	check    func()

	mu     sync.Mutex
	active bool

	last time.Time // owned by run; zero until this daemon's first check
}

func newUpdatesState(topic *stateTopic, quit chan struct{}) *updatesState {
	s := &updatesState{
		topic:    topic,
		wake:     make(chan struct{}, 1),
		demand:   make(chan struct{}, 1),
		quit:     quit,
		now:      time.Now,
		after:    wallTimer,
		interval: updateCheckInterval,
	}
	s.check = s.runStatus
	return s
}

func (d *daemon) startUpdates() {
	s := newUpdatesState(d.registerTopic("updates"), d.quit)
	s.topic.onDemand(s.setActive)
	d.registerCall("updates.check", func(json.RawMessage) (any, error) {
		select {
		case s.wake <- struct{}{}:
		default:
		}
		return map[string]any{"ok": true}, nil
	})
	go s.run()
}

func (s *updatesState) setActive(active bool) {
	s.mu.Lock()
	s.active = active
	s.mu.Unlock()
	select {
	case s.demand <- struct{}{}:
	default:
	}
}

// the first subscriber gets a check at once as the login baseline, "off" only drops the periodic one
func (s *updatesState) run() {
	for {
		wait, armed := s.plan(s.now())
		if armed && wait <= 0 {
			s.runCheck()
			continue
		}
		var fire <-chan time.Time
		stop := func() bool { return false }
		if armed {
			fire, stop = s.after(wait)
		}
		select {
		case <-s.quit:
			stop()
			return
		case <-s.wake:
			stop()
			s.runCheck()
		case <-s.demand:
			stop()
		case <-fire:
			s.runCheck()
		}
	}
}

func (s *updatesState) plan(now time.Time) (wait time.Duration, armed bool) {
	s.mu.Lock()
	active := s.active
	s.mu.Unlock()
	if !active {
		return 0, false
	}
	if s.last.IsZero() {
		return 0, true
	}
	d := s.interval()
	if d <= 0 {
		return 0, false
	}
	return s.last.Add(d).Sub(now), true
}

func (s *updatesState) runCheck() {
	s.check()
	s.last = s.now()
}

func (s *updatesState) runStatus() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	out, err := exec.CommandContext(ctx, "ryoku", "status", "--json").Output()
	if err != nil || len(out) == 0 {
		return
	}
	writeUpdateCache(out)
	s.topic.publish(out)
}

func updateCheckInterval() time.Duration {
	switch readUpdateInterval() {
	case "off":
		return 0
	case "hourly":
		return time.Hour
	case "weekly":
		return 7 * 24 * time.Hour
	default:
		return 24 * time.Hour
	}
}

func readUpdateInterval() string {
	b, err := os.ReadFile(hubConfigPath())
	if err != nil {
		return "daily"
	}
	for _, ln := range strings.Split(string(b), "\n") {
		if !strings.Contains(ln, "update_interval") {
			continue
		}
		if i := strings.IndexByte(ln, '"'); i >= 0 {
			if j := strings.IndexByte(ln[i+1:], '"'); j >= 0 {
				return ln[i+1 : i+1+j]
			}
		}
	}
	return "daily"
}

func hubConfigPath() string {
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		base = filepath.Join(os.Getenv("HOME"), ".config")
	}
	return filepath.Join(base, "ryoku", "hub.toml")
}

func writeUpdateCache(b []byte) {
	dir := filepath.Join(matugenCacheHome(), "ryoku")
	if os.MkdirAll(dir, 0o755) != nil {
		return
	}
	tmp := filepath.Join(dir, "update-status.json.tmp")
	if os.WriteFile(tmp, b, 0o644) == nil {
		_ = os.Rename(tmp, filepath.Join(dir, "update-status.json"))
	}
}
