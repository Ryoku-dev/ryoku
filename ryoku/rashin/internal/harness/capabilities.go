package harness

import (
	"encoding/json"
	"os"
	"time"

	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/protocol"
)

// capabilityEntry is what one harness id last reported in one home, kept
// while the binary it ran from is the same file, so two instances of one
// binary with different accounts keep their own lists.
type capabilityEntry struct {
	Path         string                       `json:"path"`
	MTime        int64                        `json:"mtime"`
	Capabilities protocol.HarnessCapabilities `json:"capabilities"`
}

// capabilityFile keeps the user's own entries by harness id, as the cache
// always did, and each team home's by team, then harness id. A team's
// account may offer other models than the user's, so neither ever stands in
// for the other.
type capabilityFile struct {
	Entries map[string]capabilityEntry            `json:"entries"`
	Homes   map[string]map[string]capabilityEntry `json:"homes,omitempty"`
}

// userHome keys the user's own entries among the homes.
const userHome = ""

func capabilitiesCachePath() string {
	return paths.HarnessCapabilities()
}

// capabilitiesFor is what harness id at path last reported in team's home,
// or the user's own home for userHome; nil when it has not run there since
// the binary changed.
func (r *Registry) capabilitiesFor(id, path, team string) *protocol.HarnessCapabilities {
	st, err := os.Stat(path)
	if err != nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.loadCapabilitiesLocked()
	entry, ok := r.capabilities[team][id]
	if !ok || entry.Path != path || entry.MTime != st.ModTime().UnixNano() {
		return nil
	}
	capabilities := cloneCapabilities(entry.Capabilities)
	return &capabilities
}

// recordCapabilities keeps what a harness just reported in team's home, or
// the user's own for userHome. A resumed session reports no default model,
// so the one learned from an earlier new session of the same binary there
// stays.
func (r *Registry) recordCapabilities(id, path, team string, capabilities protocol.HarnessCapabilities) {
	st, err := os.Stat(path)
	if err != nil {
		return
	}
	mtime := st.ModTime().UnixNano()
	r.mu.Lock()
	r.loadCapabilitiesLocked()
	previous, ok := r.capabilities[team][id]
	if capabilities.DefaultModel == "" && ok && previous.Path == path && previous.MTime == mtime {
		capabilities.DefaultModel = previous.Capabilities.DefaultModel
	}
	capabilities.Learned = time.Now()
	if r.capabilities[team] == nil {
		r.capabilities[team] = make(map[string]capabilityEntry)
	}
	r.capabilities[team][id] = capabilityEntry{Path: path, MTime: mtime, Capabilities: cloneCapabilities(capabilities)}
	cache := capabilityFile{Entries: r.capabilities[userHome], Homes: make(map[string]map[string]capabilityEntry)}
	for home, entries := range r.capabilities {
		if home != userHome {
			cache.Homes[home] = entries
		}
	}
	_ = writeCapabilitiesCache(cache)
	learned := r.learned
	r.mu.Unlock()
	if learned != nil {
		learned()
	}
}

// OnLearned sets what runs after a harness's capabilities are recorded.
func (r *Registry) OnLearned(fn func()) {
	r.mu.Lock()
	r.learned = fn
	r.mu.Unlock()
}

// loadCapabilitiesLocked reads the cache once. A cache from before team
// homes had their own holds only Entries, which are the user's. Entries
// written before they named their binary were keyed by path and are dropped.
func (r *Registry) loadCapabilitiesLocked() {
	if r.capabilitiesLoaded {
		return
	}
	r.capabilitiesLoaded = true
	r.capabilities = map[string]map[string]capabilityEntry{userHome: {}}
	data, err := os.ReadFile(capabilitiesCachePath())
	if err != nil {
		return
	}
	var cache capabilityFile
	if json.Unmarshal(data, &cache) != nil {
		return
	}
	keep := func(home string, entries map[string]capabilityEntry) {
		for id, entry := range entries {
			if entry.Path == "" {
				continue
			}
			if r.capabilities[home] == nil {
				r.capabilities[home] = make(map[string]capabilityEntry)
			}
			r.capabilities[home][id] = entry
		}
	}
	keep(userHome, cache.Entries)
	for home, entries := range cache.Homes {
		if home != userHome {
			keep(home, entries)
		}
	}
}

func writeCapabilitiesCache(cache capabilityFile) error {
	if err := paths.EnsureDir(paths.StateDir()); err != nil {
		return err
	}
	data, err := json.Marshal(cache)
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(paths.StateDir(), ".harness-capabilities-")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpPath, capabilitiesCachePath())
}

func cloneCapabilities(capabilities protocol.HarnessCapabilities) protocol.HarnessCapabilities {
	capabilities.Modes = append([]protocol.Choice(nil), capabilities.Modes...)
	capabilities.Started = append([]protocol.HarnessFact(nil), capabilities.Started...)
	capabilities.ModelList = append([]protocol.Choice(nil), capabilities.ModelList...)
	capabilities.Options = cloneOptions(capabilities.Options)
	return capabilities
}

func cloneOptions(options []protocol.SessionOption) []protocol.SessionOption {
	if options == nil {
		return nil
	}
	out := make([]protocol.SessionOption, len(options))
	for index, option := range options {
		option.Choices = append([]protocol.Choice(nil), option.Choices...)
		out[index] = option
	}
	return out
}
