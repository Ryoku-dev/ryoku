package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"time"
)

// xkb starts every session on the first layout; cycling is the one switch every
// provider offers, and a frame carries only the active name

const kbRestoreWindow = 5 * time.Second

type kbLayoutState struct {
	Layout  string   `json:"layout"`
	Layouts []string `json:"layouts"`
}

type kbLayoutMemory struct {
	mu        sync.Mutex
	path      string
	saved     kbLayoutState
	fresh     bool
	restoring bool
	budget    int
	until     time.Time
	cycle     func() error
}

func kbLayoutPath() string {
	return filepath.Join(stateDir(), "ryoku", "keyboard-layout.json")
}

func newKbLayoutMemory(path string, cycle func() error) *kbLayoutMemory {
	m := &kbLayoutMemory{path: path, cycle: cycle}
	if body, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(body, &m.saved)
	}
	return m
}

// a reconnect to the same session finds the saved layout already active
func (m *kbLayoutMemory) newStream() {
	m.mu.Lock()
	m.fresh = true
	m.restoring = false
	m.mu.Unlock()
}

func (m *kbLayoutMemory) observe(current string, layouts []string) {
	if current == "" || len(layouts) < 2 {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.restoring && time.Now().Before(m.until) {
		switch {
		case current == m.saved.Layout:
			m.restoring = false
		case m.budget > 0:
			m.budget--
			m.fire()
		default:
			m.restoring = false
		}
		return
	}
	m.restoring = false

	if m.fresh {
		m.fresh = false
		// a changed set is new settings, and the layout it starts on is the pick
		if m.saved.Layout != "" && m.saved.Layout != current && slices.Equal(m.saved.Layouts, layouts) {
			m.restoring = true
			m.budget = len(layouts) - 2
			m.until = time.Now().Add(kbRestoreWindow)
			m.fire()
			return
		}
	}

	if current == m.saved.Layout && slices.Equal(layouts, m.saved.Layouts) {
		return
	}
	m.saved = kbLayoutState{Layout: current, Layouts: slices.Clone(layouts)}
	m.write()
}

// a failed switch ends the restore so the next real switch is remembered
func (m *kbLayoutMemory) fire() {
	go func() {
		if m.cycle() != nil {
			m.mu.Lock()
			m.restoring = false
			m.mu.Unlock()
		}
	}()
}

func (m *kbLayoutMemory) write() {
	body, err := json.Marshal(m.saved)
	if err != nil {
		return
	}
	if os.MkdirAll(filepath.Dir(m.path), 0o755) != nil {
		return
	}
	tmp := m.path + ".tmp"
	if os.WriteFile(tmp, body, 0o644) != nil {
		return
	}
	if os.Rename(tmp, m.path) != nil {
		_ = os.Remove(tmp)
	}
}
