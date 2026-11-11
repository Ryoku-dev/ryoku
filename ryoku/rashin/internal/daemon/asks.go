package daemon

import (
	"context"
	"crypto/rand"
	"encoding/hex"

	"ryoku-rashin/internal/protocol"
)

// ask puts a question of Rashin's own to the user in a session's transcript,
// drawn and answered like a harness's permission request, and waits for the
// option they pick
func (m *sessionManager) ask(ctx context.Context, id string, data protocol.PermissionData) (string, error) {
	s, err := m.get(id)
	if err != nil {
		return "", err
	}
	var random [8]byte
	if _, err := rand.Read(random[:]); err != nil {
		return "", err
	}
	data.Request = "rashin-" + hex.EncodeToString(random[:])
	answer := make(chan string, 1)
	s.mu.Lock()
	if s.asks == nil {
		s.asks = make(map[string]chan string)
	}
	s.asks[data.Request] = answer
	s.mu.Unlock()
	if err := s.coalescer.Emit(protocol.EvPermission, data); err != nil {
		s.mu.Lock()
		delete(s.asks, data.Request)
		s.mu.Unlock()
		return "", err
	}
	select {
	case option := <-answer:
		_ = s.coalescer.Emit(protocol.EvPermissionResolved, protocol.PermissionResolvedData{Request: data.Request, Option: option, Outcome: "selected", By: "user"})
		return option, nil
	case <-ctx.Done():
		s.mu.Lock()
		delete(s.asks, data.Request)
		s.mu.Unlock()
		_ = s.coalescer.Emit(protocol.EvPermissionResolved, protocol.PermissionResolvedData{Request: data.Request, Outcome: "cancelled"})
		return "", ctx.Err()
	}
}
