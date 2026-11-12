// Package config reads and writes rashin.json: defaults, validation, merge
// patches and the one-time translation of Rashin 1's file.
package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	i18n "ryoku-i18n"
	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/protocol"
	"strings"
	"syscall"
)

// Version is the rashin.json schema this package writes.
const Version = 2

// Defaults are the settings of a machine that never changed one. Rashin is off
// until the user turns it on.
func Defaults() protocol.Settings {
	return protocol.Settings{
		Enabled: false,
		Ask:     protocol.AskSettings{Model: protocol.RouteAuto, Habits: true, Approvals: protocol.ApprovalsReadOnly},
		Agent: protocol.AgentSettings{
			Approvals:       protocol.ApprovalsReadOnly,
			IdleStopMinutes: 10,
		},
		Daemon:    protocol.DaemonSettings{IdleExitMinutes: 5},
		Index:     protocol.IndexSettings{System: true, Semantic: true},
		API:       protocol.APISettings{Port: protocol.DefaultAPIPort},
		Teams:     defaultTeams(),
		Providers: protocol.ProvidersSettings{Local: protocol.DefaultLocalAddresses()},
	}
}

// legacyKeys are Rashin 1 keys dropped on the first version 2 write.
var legacyKeys = []string{
	"port", "quick", "chatAgent", "fix", "prowlHarnesses", "keepProwl",
	"approvals", "intro", "habits", "optedOut",
}

// known are the version 2 keys this package owns.
var known = []string{"version", "enabled", "ask", "agent", "daemon", "index", "api", "teams", "providers"}

// Validate normalises s in place: unknown enums fall back to the defaults and
// numbers are clamped to sane ranges.
func Validate(s *protocol.Settings) {
	d := Defaults()
	s.Ask.Model = strings.TrimSpace(s.Ask.Model)
	if !validModel(s.Ask.Model) {
		s.Ask.Model = d.Ask.Model
	}
	if !protocol.KnownApprovals(s.Ask.Approvals) {
		s.Ask.Approvals = d.Ask.Approvals
	}
	if !protocol.KnownApprovals(s.Agent.Approvals) {
		s.Agent.Approvals = d.Agent.Approvals
	}
	s.Agent.Harness = strings.TrimSpace(s.Agent.Harness)
	seen := make(map[string]struct{}, len(s.Agent.Instances))
	instances := s.Agent.Instances[:0]
	for _, instance := range s.Agent.Instances {
		instance.ID = strings.TrimSpace(instance.ID)
		if instance.ID == "" || len(instance.Command) == 0 {
			continue
		}
		if _, exists := seen[instance.ID]; exists {
			continue
		}
		seen[instance.ID] = struct{}{}
		instances = append(instances, instance)
	}
	s.Agent.Instances = instances
	s.Agent.IdleStopMinutes = clamp(s.Agent.IdleStopMinutes, 1, 24*60, d.Agent.IdleStopMinutes)
	s.Daemon.IdleExitMinutes = clamp(s.Daemon.IdleExitMinutes, 1, 24*60, d.Daemon.IdleExitMinutes)
	if !validPort(s.API.Port) {
		s.API.Port = d.API.Port
	}
	validateTeams(&s.Teams, d.Teams)
	validateLocal(&s.Providers, d.Providers)
}

// below 1024 a user's daemon cannot listen
func validPort(port int) bool { return port >= 1024 && port <= 65535 }

func validModel(model string) bool {
	if model == protocol.RouteAuto || model == protocol.RouteFast || model == protocol.RouteSmart {
		return true
	}
	provider, name, ok := strings.Cut(model, "/")
	return ok && provider != "" && name != ""
}

func clamp(v, lo, hi, def int) int {
	if v == 0 {
		return def
	}
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// Load reads rashin.json. A missing file yields the defaults (Rashin off). A
// Rashin 1 file is translated in memory; Save writes the translation.
func Load() (protocol.Settings, error) {
	raw, err := readRaw()
	if err != nil {
		return Defaults(), err
	}
	return fromRaw(raw), nil
}

func readRaw() (map[string]json.RawMessage, error) {
	b, err := os.ReadFile(paths.ConfigFile())
	if errors.Is(err, fs.ErrNotExist) {
		return map[string]json.RawMessage{}, nil
	}
	if err != nil {
		return map[string]json.RawMessage{}, err
	}
	raw := map[string]json.RawMessage{}
	if len(bytes.TrimSpace(b)) == 0 {
		return raw, nil
	}
	if err := json.Unmarshal(b, &raw); err != nil {
		return map[string]json.RawMessage{}, fmt.Errorf("read %s: %w", paths.ConfigFile(), err)
	}
	return raw, nil
}

func fromRaw(raw map[string]json.RawMessage) protocol.Settings {
	var version int
	_ = json.Unmarshal(raw["version"], &version)
	if version < Version && len(raw) > 0 {
		return MigrateV1(raw)
	}
	s := Defaults()
	_ = json.Unmarshal(raw["enabled"], &s.Enabled)
	decodeInto(raw["ask"], &s.Ask)
	decodeInto(raw["agent"], &s.Agent)
	decodeInto(raw["daemon"], &s.Daemon)
	decodeInto(raw["index"], &s.Index)
	decodeInto(raw["api"], &s.API)
	decodeInto(raw["teams"], &s.Teams)
	decodeInto(raw["providers"], &s.Providers)
	Validate(&s)
	return s
}

func decodeInto(b json.RawMessage, v any) {
	if len(b) > 0 {
		_ = json.Unmarshal(b, v)
	}
}

// MigrateV1 translates a Rashin 1 file. Rashin stays on only when it was on and
// the user had not opted out; chatAgent and approvals carry over.
func MigrateV1(raw map[string]json.RawMessage) protocol.Settings {
	s := Defaults()
	var enabled, optedOut bool
	_ = json.Unmarshal(raw["enabled"], &enabled)
	_ = json.Unmarshal(raw["optedOut"], &optedOut)
	s.Enabled = enabled && !optedOut
	_ = json.Unmarshal(raw["chatAgent"], &s.Agent.Harness)
	var approvals string
	_ = json.Unmarshal(raw["approvals"], &approvals)
	s.Agent.Approvals = approvals
	var habits struct {
		History *bool `json:"history"`
	}
	_ = json.Unmarshal(raw["habits"], &habits)
	if habits.History != nil {
		s.Ask.Habits = *habits.History
	}
	Validate(&s)
	return s
}

// Save writes s atomically, keeping keys this package does not know and
// dropping Rashin 1 keys.
func Save(s protocol.Settings) error {
	return withConfigLock(func() error { return saveUnlocked(s) })
}

// SaveMerged writes settings.set while preserving the on-disk enabled switch.
func SaveMerged(s protocol.Settings) (protocol.Settings, error) {
	err := withConfigLock(func() error {
		raw, err := readRaw()
		if err != nil {
			return err
		}
		current := fromRaw(raw)
		s.Enabled = current.Enabled
		return saveUnlocked(s)
	})
	return s, err
}

func saveUnlocked(s protocol.Settings) error {
	Validate(&s)
	raw, err := readRaw()
	if err != nil {
		raw = map[string]json.RawMessage{}
	}
	for _, k := range legacyKeys {
		delete(raw, k)
	}
	set := func(k string, v any) error {
		b, err := json.Marshal(v)
		if err != nil {
			return err
		}
		raw[k] = b
		return nil
	}
	for k, v := range map[string]any{
		"version":   Version,
		"enabled":   s.Enabled,
		"ask":       s.Ask,
		"agent":     s.Agent,
		"daemon":    s.Daemon,
		"index":     s.Index,
		"api":       s.API,
		"teams":     s.Teams,
		"providers": s.Providers,
	} {
		if err := set(k, v); err != nil {
			return err
		}
	}
	b, err := marshalOrdered(raw)
	if err != nil {
		return err
	}
	return writeAtomic(paths.ConfigFile(), b)
}

func withConfigLock(fn func() error) error {
	if err := paths.EnsureDir(paths.RuntimeDir()); err != nil {
		return err
	}
	lock, err := os.OpenFile(paths.ConfigLock(), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return err
	}
	defer lock.Close()
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX); err != nil {
		return err
	}
	defer syscall.Flock(int(lock.Fd()), syscall.LOCK_UN)
	return fn()
}

// marshalOrdered writes the owned keys first, in schema order, then the rest
// sorted, so the file reads the same way every time.
func marshalOrdered(raw map[string]json.RawMessage) ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteString("{\n")
	keys := append([]string{}, known...)
	var rest []string
	for k := range raw {
		if !contains(known, k) {
			rest = append(rest, k)
		}
	}
	sortStrings(rest)
	keys = append(keys, rest...)
	first := true
	for _, k := range keys {
		v, ok := raw[k]
		if !ok {
			continue
		}
		if !first {
			buf.WriteString(",\n")
		}
		first = false
		kb, _ := json.Marshal(k)
		var vb bytes.Buffer
		if err := json.Compact(&vb, v); err != nil {
			return nil, err
		}
		buf.WriteString("  ")
		buf.Write(kb)
		buf.WriteString(": ")
		buf.Write(vb.Bytes())
	}
	buf.WriteString("\n}\n")
	return buf.Bytes(), nil
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}

func writeAtomic(path string, b []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".rashin.json.*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(b); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(0o644); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

// SetEnabled flips the on/off switch and saves.
func SetEnabled(on bool) error {
	return withConfigLock(func() error {
		raw, err := readRaw()
		if err != nil {
			return err
		}
		s := fromRaw(raw)
		s.Enabled = on
		return saveUnlocked(s)
	})
}

// Merge applies a JSON merge patch (RFC 7396) to cur and validates the result.
// Enabled cannot be changed this way.
func Merge(cur protocol.Settings, patch json.RawMessage) (protocol.Settings, error) {
	base, err := json.Marshal(cur)
	if err != nil {
		return cur, err
	}
	var doc, p any
	if err := json.Unmarshal(base, &doc); err != nil {
		return cur, err
	}
	if err := json.Unmarshal(patch, &p); err != nil {
		return cur, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.Tf("The settings patch is not JSON: %v.", err))
	}
	merged, err := json.Marshal(mergePatch(doc, p))
	if err != nil {
		return cur, err
	}
	next := Defaults()
	if err := json.Unmarshal(merged, &next); err != nil {
		return cur, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.Tf("The settings patch has the wrong shape: %v.", err))
	}
	next.Enabled = cur.Enabled
	if err := validatePatch(next); err != nil {
		return cur, err
	}
	Validate(&next)
	return next, nil
}

func validatePatch(s protocol.Settings) error {
	model := strings.TrimSpace(s.Ask.Model)
	if !validModel(model) {
		return protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("The Ask model is invalid."))
	}
	seen := make(map[string]struct{}, len(s.Agent.Instances))
	for _, instance := range s.Agent.Instances {
		id := strings.TrimSpace(instance.ID)
		if id == "" || len(instance.Command) == 0 {
			return protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("Every agent instance needs an id and command."))
		}
		if _, exists := seen[id]; exists {
			return protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.Tf("Agent instance id %s is duplicated.", id))
		}
		seen[id] = struct{}{}
	}
	if !validPort(s.API.Port) {
		return protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("The API port must be between 1024 and 65535."))
	}
	if err := checkLocal(s.Providers); err != nil {
		return err
	}
	return checkTeams(s.Teams)
}

func mergePatch(target, patch any) any {
	pm, ok := patch.(map[string]any)
	if !ok {
		return patch
	}
	tm, ok := target.(map[string]any)
	if !ok {
		tm = map[string]any{}
	}
	for k, v := range pm {
		if v == nil {
			delete(tm, k)
			continue
		}
		tm[k] = mergePatch(tm[k], v)
	}
	return tm
}
