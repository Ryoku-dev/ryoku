package assign

import "time"

// ClearRows forgets today's in-memory activity, not the aggregate spend
// that enforces hosted limits across restarts. Privacy is not a budget reset.
func (l *Ledger) ClearRows(now time.Time) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.roll(now)
	clear(l.rows)
	l.rows = nil
}
