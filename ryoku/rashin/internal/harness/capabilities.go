package harness

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"

	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/protocol"
)

type capabilityEntry struct {
	MTime        int64                        `json:"mtime"`
	Capabilities protocol.HarnessCapabilities `json:"capabilities"`
}

type capabilityFile struct {
	Entries map[string]capabilityEntry `json:"entries"`
}

func capabilitiesCachePath() string {
	return paths.HarnessCapabilities()
}

func (r *Registry) capabilitiesFor(path string) *protocol.HarnessCapabilities {
	st, err := os.Stat(path)
	if err != nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.loadCapabilitiesLocked()
	entry, ok := r.capabilities[path]
	if !ok || entry.MTime != st.ModTime().UnixNano() {
		return nil
	}
	capabilities := cloneCapabilities(entry.Capabilities)
	return &capabilities
}

func (r *Registry) recordCapabilities(path string, capabilities protocol.HarnessCapabilities) {
	st, err := os.Stat(path)
	if err != nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.loadCapabilitiesLocked()
	r.capabilities[path] = capabilityEntry{
		MTime:        st.ModTime().UnixNano(),
		Capabilities: cloneCapabilities(capabilities),
	}
	_ = writeCapabilitiesCache(capabilityFile{Entries: r.capabilities})
}

func (r *Registry) loadCapabilitiesLocked() {
	if r.capabilitiesLoaded {
		return
	}
	r.capabilitiesLoaded = true
	r.capabilities = make(map[string]capabilityEntry)
	data, err := os.ReadFile(capabilitiesCachePath())
	if errors.Is(err, fs.ErrNotExist) || err != nil {
		return
	}
	var cache capabilityFile
	if json.Unmarshal(data, &cache) != nil {
		return
	}
	for path, entry := range cache.Entries {
		r.capabilities[path] = entry
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
	return capabilities
}
