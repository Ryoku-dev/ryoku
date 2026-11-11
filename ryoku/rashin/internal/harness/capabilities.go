package harness

import (
	"encoding/json"
	"os"
	"time"

	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/protocol"
)

// capabilityEntry is what one harness id last reported, kept while the binary
// it ran from is the same file, so two instances of one binary with different
// accounts keep their own lists.
type capabilityEntry struct {
	Path         string                       `json:"path"`
	MTime        int64                        `json:"mtime"`
	Capabilities protocol.HarnessCapabilities `json:"capabilities"`
}

type capabilityFile struct {
	Entries map[string]capabilityEntry `json:"entries"`
}

func capabilitiesCachePath() string {
	return paths.HarnessCapabilities()
}

func (r *Registry) capabilitiesFor(id, path string) *protocol.HarnessCapabilities {
	st, err := os.Stat(path)
	if err != nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.loadCapabilitiesLocked()
	entry, ok := r.capabilities[id]
	if !ok || entry.Path != path || entry.MTime != st.ModTime().UnixNano() {
		return nil
	}
	capabilities := cloneCapabilities(entry.Capabilities)
	return &capabilities
}

// recordCapabilities keeps what a harness just reported. A resumed session
// reports no default model, so the one learned from an earlier new session
// of the same binary stays.
func (r *Registry) recordCapabilities(id, path string, capabilities protocol.HarnessCapabilities) {
	st, err := os.Stat(path)
	if err != nil {
		return
	}
	mtime := st.ModTime().UnixNano()
	r.mu.Lock()
	r.loadCapabilitiesLocked()
	previous, ok := r.capabilities[id]
	if capabilities.DefaultModel == "" && ok && previous.Path == path && previous.MTime == mtime {
		capabilities.DefaultModel = previous.Capabilities.DefaultModel
	}
	capabilities.Learned = time.Now()
	r.capabilities[id] = capabilityEntry{Path: path, MTime: mtime, Capabilities: cloneCapabilities(capabilities)}
	_ = writeCapabilitiesCache(capabilityFile{Entries: r.capabilities})
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

// loadCapabilitiesLocked reads the cache once. Entries written before they
// named their binary were keyed by path and are dropped.
func (r *Registry) loadCapabilitiesLocked() {
	if r.capabilitiesLoaded {
		return
	}
	r.capabilitiesLoaded = true
	r.capabilities = make(map[string]capabilityEntry)
	data, err := os.ReadFile(capabilitiesCachePath())
	if err != nil {
		return
	}
	var cache capabilityFile
	if json.Unmarshal(data, &cache) != nil {
		return
	}
	for id, entry := range cache.Entries {
		if entry.Path != "" {
			r.capabilities[id] = entry
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
