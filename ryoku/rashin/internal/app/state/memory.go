package state

import (
	"encoding/json"

	"ryoku-rashin/internal/protocol"
)

// memoryState is the memory part of the store: memory's status, the changes
// seen so far by id, so a conversation's memory line knows whether it was
// undone, the latest memory.setup progress, and a revision that moves with
// every memory.changed.
type memoryState struct {
	status   *protocol.MemoryStatus
	changes  map[string]protocol.MemoryChange
	setup    *protocol.MemorySetupProgress
	revision uint64
}

func newMemoryState() memoryState {
	return memoryState{changes: make(map[string]protocol.MemoryChange)}
}

func (m *memoryState) applySnapshotLocked(snapshot protocol.Snapshot) {
	m.status, m.setup = nil, nil
	if snapshot.Memory != nil {
		status := cloneMemoryStatus(*snapshot.Memory)
		m.status = &status
	}
	m.revision++
}

// applyLocked applies memory.changed and memory.setup; handled is false for
// any other event.
func (m *memoryState) applyLocked(event protocol.Event) (handled bool, err error) {
	if event.Event == protocol.EvtMemorySetup {
		var progress protocol.MemorySetupProgress
		if err = json.Unmarshal(event.Data, &progress); err != nil {
			return true, err
		}
		m.setup = &progress
		m.revision++
		return true, nil
	}
	if event.Event != protocol.EvtMemoryChanged {
		return false, nil
	}
	var changed protocol.MemoryChanged
	if err = json.Unmarshal(event.Data, &changed); err != nil {
		return true, err
	}
	status := cloneMemoryStatus(changed.Status)
	m.status = &status
	if changed.Change != nil {
		m.changes[changed.Change.ID] = cloneMemoryChange(*changed.Change)
	}
	m.revision++
	return true, nil
}

func (m *memoryState) snapshotLocked(into *Snapshot) {
	if m.status != nil {
		status := cloneMemoryStatus(*m.status)
		into.Memory = &status
	}
	into.MemoryChanges = make(map[string]protocol.MemoryChange, len(m.changes))
	for id, change := range m.changes {
		into.MemoryChanges[id] = change
	}
	into.MemoryRevision = m.revision
	if m.setup != nil {
		setup := *m.setup
		into.MemorySetup = &setup
	}
}

// SetMemoryStatus stores memory's status as memory.status answered it, for
// a page that asked again after something only it knows of changed.
func (s *Store) SetMemoryStatus(status protocol.MemoryStatus) {
	s.mu.Lock()
	status = cloneMemoryStatus(status)
	s.memoryState.status = &status
	s.memoryState.revision++
	invalidate := s.changedLocked()
	s.mu.Unlock()
	if invalidate != nil {
		invalidate()
	}
}

// SetMemoryChanges stores changes loaded with memory.changes, so the memory
// lines of a conversation opened later show what became of them.
func (s *Store) SetMemoryChanges(changes []protocol.MemoryChange) {
	s.mu.Lock()
	for _, change := range changes {
		s.memoryState.changes[change.ID] = cloneMemoryChange(change)
	}
	invalidate := s.changedLocked()
	s.mu.Unlock()
	if invalidate != nil {
		invalidate()
	}
}

func cloneMemoryStatus(status protocol.MemoryStatus) protocol.MemoryStatus {
	status.Scopes = append([]protocol.MemoryScope(nil), status.Scopes...)
	status.Days = append([]protocol.MemoryDay(nil), status.Days...)
	if status.Learner.Last != nil {
		last := *status.Learner.Last
		status.Learner.Last = &last
	}
	return status
}

func cloneMemoryChange(change protocol.MemoryChange) protocol.MemoryChange {
	change.Items = append([]protocol.MemoryItem(nil), change.Items...)
	return change
}
