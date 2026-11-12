package daemon

import (
	"errors"
	"os"

	i18n "ryoku-i18n"
	"ryoku-rashin/internal/core"
	"ryoku-rashin/internal/protocol"
	store "ryoku-rashin/internal/session"
)

// the manager is what memory reads sessions through
var _ core.MemorySessions = (*sessionManager)(nil)

// memoryReach orders what a conversation does with memory by how far it
// reaches: remembering uses memory and is learned from, use only uses it,
// and no memory does neither. A narrower mode reaches less.
func memoryReach(mode string) (int, bool) {
	switch mode {
	case protocol.MemoryRemember:
		return 2, true
	case protocol.MemoryUseOnly:
		return 1, true
	case protocol.MemoryNone:
		return 0, true
	}
	return 0, false
}

func errMemoryMode() error {
	return protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("A conversation's memory must be remember, use or off."))
}

// startingMemory is the memory mode a new conversation starts with. A
// temporary conversation is never learned from, so it uses memory at most.
func startingMemory(mode string, temporary bool) (string, error) {
	if _, ok := memoryReach(mode); !ok {
		return "", errMemoryMode()
	}
	if temporary && mode == protocol.MemoryRemember {
		return protocol.MemoryUseOnly, nil
	}
	return mode, nil
}

// setMemory sets what a conversation does with memory. Before its first
// message any mode may be picked; after it only a narrower one, and the
// transcript shows where it narrowed.
func (m *sessionManager) setMemory(p protocol.SetMemoryParams) (protocol.SessionInfo, error) {
	next, ok := memoryReach(p.Memory)
	if !ok {
		return protocol.SessionInfo{}, errMemoryMode()
	}
	s, err := m.get(p.ID)
	if err != nil {
		return protocol.SessionInfo{}, err
	}
	s.mu.Lock()
	info := s.log.Info()
	if info.Temporary && p.Memory == protocol.MemoryRemember {
		s.mu.Unlock()
		return protocol.SessionInfo{}, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("A temporary conversation is never learned from."))
	}
	if p.Memory == info.Memory {
		s.mu.Unlock()
		return info, nil
	}
	spoken, err := userSpoke(s.log, info)
	if err != nil {
		s.mu.Unlock()
		return protocol.SessionInfo{}, err
	}
	if current, _ := memoryReach(info.Memory); spoken && next > current {
		s.mu.Unlock()
		return protocol.SessionInfo{}, protocol.Errorf(protocol.CodeConflict, "%s", i18n.T("A conversation's memory is set when it starts; after that it can only be narrowed."))
	}
	if err := s.log.Update(func(i *protocol.SessionInfo) { i.Memory = p.Memory }, nil); err != nil {
		s.mu.Unlock()
		return protocol.SessionInfo{}, err
	}
	m.emitInfoLocked(s)
	s.mu.Unlock()
	if spoken {
		if err := s.coalescer.Emit(protocol.EvMemory, protocol.MemoryEventData{Kind: protocol.MemoryNarrowed, Memory: p.Memory}); err != nil {
			return protocol.SessionInfo{}, err
		}
	}
	m.changed()
	return s.log.Info(), nil
}

// userSpoke reports whether the user has sent the session a message: a turn
// has run, or a handoff or fork already wrote one before any turn did.
func userSpoke(log *store.Log, info protocol.SessionInfo) (bool, error) {
	if info.Turns > 0 {
		return true, nil
	}
	events, err := log.Since(0)
	if err != nil {
		return false, err
	}
	for _, event := range events {
		if event.Kind == protocol.EvUser {
			return true, nil
		}
	}
	return false, nil
}

// Events is a session's stored events. A temporary session stores none. A
// session the manager does not hold is read from disk without being opened.
func (m *sessionManager) Events(id string) ([]protocol.SessionEvent, error) {
	m.mu.Lock()
	s := m.items[id]
	m.mu.Unlock()
	if s != nil {
		if s.log.Info().Temporary {
			return nil, nil
		}
		return s.log.Since(0)
	}
	events, err := store.Events(id)
	if errors.Is(err, os.ErrNotExist) {
		return nil, protocol.Errorf(protocol.CodeNotFound, "%s", i18n.Tf("No session %s.", id))
	}
	return events, err
}

// EmitMemory puts a memory line into a session's transcript and streams it
// to every client attached, whether a turn runs or not. It goes through the
// session's coalescer, which writes a delta it still holds first, so the
// transcript keeps the order things happened in.
func (m *sessionManager) EmitMemory(id string, data protocol.MemoryEventData) {
	s, err := m.get(id)
	if err == nil {
		err = s.coalescer.Emit(protocol.EvMemory, data)
	}
	if err != nil && m.warn != nil {
		m.warn("memory event session %s: %v", id, err)
	}
}
