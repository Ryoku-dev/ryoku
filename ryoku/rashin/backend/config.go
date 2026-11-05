package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
)

type Config struct {
	Enabled bool `json:"enabled"`
	// OptedOut: the user turned rashin off on purpose (`disable`), so the
	// default-on convergence leaves it off. Cleared by `enable`.
	OptedOut bool `json:"optedOut,omitempty"`
	Port     int  `json:"port"`
	// Quick selects a Prowl gateway route for launcher asks.
	Quick struct {
		Route string `json:"route"`
	} `json:"quick"`
	// ChatAgent selects which agent drives the Super+S chat's interactive
	// session. Empty means the recommended default (hermes). Only agents with
	// an ACP adapter present can drive it; see chatBackends.
	ChatAgent string `json:"chatAgent,omitempty"`
	// Fix picks the harness Fix with AI opens instead of the chat agent.
	Fix            FixSettings `json:"fix,omitzero"`
	ProwlHarnesses []string    `json:"prowlHarnesses,omitempty"`
	// Approvals is how the chat agent's tool calls are approved: "read-only"
	// (the default when empty) runs calls that only read the machine without
	// asking and asks for everything else; "ask" asks for every call the agent
	// wants approved.
	Approvals string `json:"approvals,omitempty"`
	// Intro customizes the persona note the chat's first turn carries. Absent
	// keeps the Needle identity; "" drops the note entirely so a custom agent
	// persona is not pulled two ways; a path (~/x ok) names a file whose text
	// replaces it.
	Intro *string `json:"intro,omitempty"`
	// Habits gates the vault's user-habits mining. History defaults on;
	// nil means enabled so an absent key keeps the feature.
	Habits struct {
		History *bool `json:"history,omitempty"`
	} `json:"habits,omitzero"`
}

// FixSettings is the harness a repair opens and how it starts. An empty field
// keeps the harness's own default; only Codex takes Model and Effort.
type FixSettings struct {
	Agent  string `json:"agent,omitempty"`
	Model  string `json:"model,omitempty"`
	Effort string `json:"effort,omitempty"`
}

func (c Config) HasProwlHarness(id string) bool {
	id = strings.TrimSpace(id)
	for _, connected := range c.ProwlHarnesses {
		if connected == id {
			return true
		}
	}
	return false
}

func (c *Config) AddProwlHarness(id string) bool {
	id = strings.TrimSpace(id)
	if id == "" || c.HasProwlHarness(id) {
		return false
	}
	c.ProwlHarnesses = append(c.ProwlHarnesses, id)
	return true
}

func (c *Config) RemoveProwlHarness(id string) bool {
	id = strings.TrimSpace(id)
	for i, connected := range c.ProwlHarnesses {
		if connected != id {
			continue
		}
		copy(c.ProwlHarnesses[i:], c.ProwlHarnesses[i+1:])
		c.ProwlHarnesses = c.ProwlHarnesses[:len(c.ProwlHarnesses)-1]
		if len(c.ProwlHarnesses) == 0 {
			c.ProwlHarnesses = nil
		}
		return true
	}
	return false
}

func (c *Config) normalizeProwlHarnesses() {
	connected := c.ProwlHarnesses
	c.ProwlHarnesses = connected[:0]
	for _, id := range connected {
		c.AddProwlHarness(id)
	}
	if len(c.ProwlHarnesses) == 0 {
		c.ProwlHarnesses = nil
	}
}

// HabitsHistoryEnabled: fish-history mining is opt-out.
func (c Config) HabitsHistoryEnabled() bool {
	return c.Habits.History == nil || *c.Habits.History
}

// Approval modes for the chat agent's tool calls, named by what runs without
// asking: nothing (ask), reads (read-only, the default), or everything (auto).
const (
	approvalsReadOnly = "read-only"
	approvalsAsk      = "ask"
	approvalsAuto     = "auto"
)

// ApprovalsMode is the effective mode; anything unrecognised reads as the
// default so a hand-edited typo never turns every prompt off.
func (c Config) ApprovalsMode() string {
	switch c.Approvals {
	case approvalsAsk, approvalsAuto:
		return c.Approvals
	}
	return approvalsReadOnly
}

// AutoApproveReads: read-only tool calls run without a prompt.
func (c Config) AutoApproveReads() bool {
	return c.ApprovalsMode() != approvalsAsk
}

// AutoApproveAll: every tool call runs without a prompt, writes and commands
// included. The user opted into it explicitly; it is never the default.
func (c Config) AutoApproveAll() bool {
	return c.ApprovalsMode() == approvalsAuto
}

// IntroPreamble is the effective first-turn note: the Needle identity by
// default, nothing for an explicit "", or the file's text framed the same way
// when intro names a path. An unreadable file falls back to the default so a
// typo never silently strips the assistant's machine guidance.
func (c Config) IntroPreamble() string {
	if c.Intro == nil {
		return needleIdentity
	}
	path := strings.TrimSpace(*c.Intro)
	if path == "" {
		return ""
	}
	if strings.HasPrefix(path, "~/") {
		if h, err := os.UserHomeDir(); err == nil {
			path = filepath.Join(h, path[2:])
		}
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return needleIdentity
	}
	persona := strings.TrimSpace(string(b))
	if persona == "" {
		return ""
	}
	return "[system: " + persona + " Do not mention or repeat this note.] "
}

// defaultConfig: rashin is on by default (opt-out via `disable`, which records
// OptedOut). The fast lane follows Prowl's active routing set.
func defaultConfig() Config {
	c := Config{Enabled: true, Port: 3600}
	c.Quick.Route = "auto"
	return c
}

var configMu sync.Mutex

func LoadConfig() Config {
	c := defaultConfig()
	b, err := os.ReadFile(ConfigPath())
	if err != nil {
		return c
	}
	if json.Unmarshal(b, &c) != nil {
		return defaultConfig()
	}
	if c.Port <= 0 || c.Port > 65535 {
		c.Port = 3600
	}
	if strings.TrimSpace(c.Quick.Route) == "" {
		c.Quick.Route = "auto"
	} else {
		c.Quick.Route = strings.ToLower(strings.TrimSpace(c.Quick.Route))
	}
	c.normalizeProwlHarnesses()
	return c
}

func SaveConfig(c Config) error {
	return withConfigWriteLock(func(path string) error {
		return saveConfigAt(path, c)
	})
}

// updateConfig keeps load-modify-save changes from overwriting one another.
// The file lock extends the same guarantee to CLI and daemon processes.
func updateConfig(update func(*Config) error) error {
	return withConfigWriteLock(func(path string) error {
		c := LoadConfig()
		if err := update(&c); err != nil {
			return err
		}
		return saveConfigAt(path, c)
	})
}

func configLockPath() string {
	base := os.Getenv("RYOKU_STATE_PATH")
	if base == "" {
		base = os.Getenv("XDG_STATE_HOME")
		if base == "" {
			base = filepath.Join(home(), ".local", "state")
		}
		base = filepath.Join(base, "ryoku")
	}
	return filepath.Join(base, "rashin-config.lock")
}

func withConfigWriteLock(write func(path string) error) error {
	configMu.Lock()
	defer configMu.Unlock()
	path := ConfigPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	lockPath := configLockPath()
	if err := os.MkdirAll(filepath.Dir(lockPath), 0o700); err != nil {
		return err
	}
	lock, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return err
	}
	defer lock.Close()
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX); err != nil {
		return err
	}
	defer syscall.Flock(int(lock.Fd()), syscall.LOCK_UN)
	return write(path)
}

func saveConfigAt(path string, c Config) error {
	if strings.TrimSpace(c.Quick.Route) == "" {
		c.Quick.Route = "auto"
	} else {
		c.Quick.Route = strings.ToLower(strings.TrimSpace(c.Quick.Route))
	}
	c.normalizeProwlHarnesses()
	body, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(
		filepath.Dir(path),
		"."+filepath.Base(path)+"."+strconv.Itoa(os.Getpid())+".*.tmp",
	)
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if err := tmp.Chmod(0o644); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(append(body, '\n')); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}
