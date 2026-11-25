package memory

import (
	"time"

	"ryoku-rashin/internal/protocol"
)

// ephemeralDays is how long an ephemeral memory lasts unused.
const ephemeralDays = 30

// Fade forgets what has gone stale: an active memory nothing drew on for
// months (0 never fades; pinned and permanent memories never do), an
// ephemeral one unused for ephemeralDays, and a replaced memory kept past
// protocol.MemoryReplacedDays. Faded memories leave no tombstone, so they
// may be learned again when they matter again. The learner calls it when it
// runs; it has no timer of its own.
func (s *Store) Fade(months int) (protocol.MemoryChange, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.syncLocked()
	t := s.begin(Origin{By: protocol.MemoryByFading})
	now := t.now
	replacedBefore := now.AddDate(0, 0, -protocol.MemoryReplacedDays)
	unusedBefore := now.AddDate(0, -months, 0)
	ephemeralBefore := now.AddDate(0, 0, -ephemeralDays)
	for _, f := range s.facts {
		switch {
		case f.Status == protocol.MemoryReplaced && f.Replaced != nil && f.Replaced.Before(replacedBefore):
			t.forget(f, protocol.MemoryOpForgot, false)
		case months > 0 && f.Status == protocol.MemoryActive && !f.Pinned && f.Kind != protocol.MemoryPermanent && lastTouched(f).Before(unusedBefore):
			t.forget(f, protocol.MemoryOpForgot, false)
		case f.Status == protocol.MemoryActive && !f.Pinned && f.Kind == protocol.MemoryEphemeral && lastTouched(f).Before(ephemeralBefore):
			t.forget(f, protocol.MemoryOpForgot, false)
		}
	}
	return t.commit()
}

// lastTouched is when a memory was last used, or written when never used.
func lastTouched(f *fact) time.Time {
	return touchedAt(protocol.Memory{LastUsed: f.LastUsed, Updated: f.Updated})
}
