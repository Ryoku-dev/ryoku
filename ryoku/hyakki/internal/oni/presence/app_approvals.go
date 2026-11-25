package presence

import (
	"time"

	"ryoku-rashin/internal/oni/feature"
)

// These three owners each have at most one live question. Retention is a
// read model of their disclosure, not an approval or a second task store.
// Expiry uses the owner's original deadline and arms no additional timer.
type appApproval struct {
	frame feature.ConsentFrame
	until time.Time
}

func appApprovalIndex(id string) int {
	switch id {
	case feature.FeedGuard:
		return 0
	case feature.MemoryForget:
		return 1
	case feature.Recall:
		return 2
	}
	return -1
}

func (s *Service) keepAppApproval(frame feature.ConsentFrame) {
	if index := appApprovalIndex(frame.Feature); index >= 0 {
		until := s.clock.Now().Add(time.Duration(frame.ExpiresMs) * time.Millisecond)
		previous := s.appApprovals[index]
		if previous.frame.ID == frame.ID && previous.until.Before(until) {
			until = previous.until
		}
		s.appApprovals[index] = appApproval{frame: frame, until: until}
	}
}

// appApprovalFrames is called under the service lock only for an
// authenticated app observer. General snapshots do not contain these cards.
func (s *Service) appApprovalFrames() []feature.Payload {
	if !s.settings.Master || s.closed {
		return nil
	}
	now := s.clock.Now()
	var frames []feature.Payload
	for index := range s.appApprovals {
		pending := &s.appApprovals[index]
		if pending.frame.ID == "" {
			continue
		}
		left := pending.until.Sub(now).Milliseconds()
		if left <= 0 {
			*pending = appApproval{}
			continue
		}
		frame := pending.frame
		frame.ExpiresMs = left
		frames = append(frames, &frame)
	}
	return frames
}

// ConsentAnswer removes the exact card before publishing the owner's
// answer. It carries no recalled answer, memory contents or permission.
func (s *Service) ConsentAnswer(frame feature.ConsentAnswerFrame) error {
	if _, err := feature.EncodeFrame(&frame); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return ErrClosed
	}
	for index := range s.appApprovals {
		if s.appApprovals[index].frame.ID == frame.ID {
			s.appApprovals[index] = appApproval{}
		}
	}
	if s.settings.Master {
		s.send(&frame)
	}
	return nil
}
