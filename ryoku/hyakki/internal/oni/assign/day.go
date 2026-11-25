package assign

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"time"

	"ryoku-rashin/internal/oni/spec"
	"ryoku-rashin/internal/protocol"
)

// Gate is the gate's numbers from the spec.
func Gate(g spec.AssignmentGate) GateConfig {
	return GateConfig{Still: g.StillDelta, Gone: g.GoneFraction, Critical: g.CriticalTokens}
}

// Cache keeps the last solved views, newest last, to go back to and to
// answer a view seen again with no model request.
type Cache struct {
	max   int
	views []Solved
}

// Solved is one solved view: what the gate compares and the solve shown.
// It holds no picture.
type Solved struct {
	View  View
	Solve Solve
}

// NewCache keeps at most n views.
func NewCache(n int) *Cache { return &Cache{max: n} }

// Put keeps s, dropping the oldest past the cache's size.
func (c *Cache) Put(s Solved) {
	if c.max <= 0 {
		return
	}
	if len(c.views) >= c.max {
		c.views = slices.Delete(c.views, 0, len(c.views)-c.max+1)
	}
	c.views = append(c.views, s)
}

// Find is the newest kept view the gate does not tell from v.
func (c *Cache) Find(g GateConfig, v View) (Solved, int, bool) {
	for i := len(c.views) - 1; i >= 0; i-- {
		if !g.NewProblem(c.views[i].View, v).New {
			return c.views[i], i, true
		}
	}
	return Solved{}, -1, false
}

// At is the view at i, oldest first.
func (c *Cache) At(i int) (Solved, bool) {
	if i < 0 || i >= len(c.views) {
		return Solved{}, false
	}
	return c.views[i], true
}

// Len is how many views are kept.
func (c *Cache) Len() int { return len(c.views) }

// Clear drops every view.
func (c *Cache) Clear() {
	clear(c.views)
	c.views = nil
}

// Caps are one day's limits on routes that leave the computer.
type Caps struct {
	Attempts, Tokens int
}

// Why a hosted solve may not start today.
var (
	ErrAttempts  = errors.New("assign: today's attempts are used up")
	ErrTokens    = errors.New("assign: today's tokens are used up")
	ErrUnmetered = errors.New("assign: today's hosted usage was not measured")
)

// Ledger is assignment mode's spend: the counts of the current local day,
// kept in one file so a restart does not reset them, and the rows of the
// day, kept in memory only and bounded. Local solves are written down but
// never stopped; a hosted or paid route stops at either cap, and once a
// hosted reply came back unmetered the token cap cannot be shown to hold,
// so further hosted solves wait for the next day.
type Ledger struct {
	mu   sync.Mutex
	path string
	caps Caps
	max  int
	loc  *time.Location
	day  dayFile
	rows []protocol.OniAssignmentRow
}

// dayFile is what is kept on disk: the day and its counts, never a row.
type dayFile struct {
	Day      string `json:"day"`
	Attempts int    `json:"attempts"`
	Tokens   int    `json:"tokens"`
	Hosted   int    `json:"hosted"`
	// HostedTokens are the measured tokens of hosted replies, the ones
	// the cap counts.
	HostedTokens int  `json:"hostedTokens"`
	Estimated    bool `json:"estimated,omitempty"`
	Unmetered    bool `json:"unmetered,omitempty"`
}

// NewLedger keeps the counts in path ("" keeps them in memory only), caps
// hosted spend by caps, keeps at most rows rows, and counts days in loc.
func NewLedger(path string, caps Caps, rows int, loc *time.Location) *Ledger {
	if loc == nil {
		loc = time.Local
	}
	l := &Ledger{path: path, caps: caps, max: rows, loc: loc}
	if path != "" {
		if b, err := os.ReadFile(path); err == nil {
			_ = json.Unmarshal(b, &l.day)
		}
	}
	return l
}

func (l *Ledger) roll(now time.Time) {
	day := now.In(l.loc).Format(time.DateOnly)
	if l.day.Day != day {
		l.day = dayFile{Day: day}
		l.rows = nil
	}
}

// Day is today's spend.
func (l *Ledger) Day(now time.Time) protocol.OniAssignmentDay {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.roll(now)
	return l.dayLocked()
}

func (l *Ledger) dayLocked() protocol.OniAssignmentDay {
	return protocol.OniAssignmentDay{Day: l.day.Day, Attempts: l.day.Attempts, Tokens: l.day.Tokens,
		AttemptsCap: l.caps.Attempts, TokensCap: l.caps.Tokens, Estimated: l.day.Estimated, Unmetered: l.day.Unmetered}
}

// Allow says whether a solve may start now: always on this computer; on a
// hosted route only within both caps and with today's hosted usage
// measured.
func (l *Ledger) Allow(now time.Time, local bool) error {
	if local {
		return nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	l.roll(now)
	switch {
	case l.day.Unmetered:
		return ErrUnmetered
	case l.day.Hosted >= l.caps.Attempts:
		return ErrAttempts
	case l.day.HostedTokens >= l.caps.Tokens:
		return ErrTokens
	}
	return nil
}

// Attempt counts one solve begun at now.
func (l *Ledger) Attempt(now time.Time, local bool) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.roll(now)
	l.day.Attempts++
	if !local {
		l.day.Hosted++
	}
	return l.saveLocked()
}

// Record writes one lane's row and adds its measured tokens. Estimated
// counts are added and flagged; an unmetered hosted reply flags the day.
func (l *Ledger) Record(row protocol.OniAssignmentRow, local bool) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.roll(row.At)
	l.day.Tokens += row.Input + row.Output
	if !local {
		l.day.HostedTokens += row.Input + row.Output
	}
	l.day.Estimated = l.day.Estimated || row.Estimated
	if row.Unmetered && !local {
		l.day.Unmetered = true
	}
	if l.max > 0 {
		if len(l.rows) >= l.max {
			l.rows = slices.Delete(l.rows, 0, len(l.rows)-l.max+1)
		}
		l.rows = append(l.rows, row)
	}
	return l.saveLocked()
}

// Rows are today's rows, newest first.
func (l *Ledger) Rows(now time.Time) []protocol.OniAssignmentRow {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.roll(now)
	out := slices.Clone(l.rows)
	slices.Reverse(out)
	return out
}

// saveLocked writes the counts, synced, through a temporary file, 0600 in a
// 0700 folder.
func (l *Ledger) saveLocked() error {
	if l.path == "" {
		return nil
	}
	b, err := json.Marshal(l.day)
	if err != nil {
		return err
	}
	dir := filepath.Dir(l.path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, ".assign-*")
	if err != nil {
		return err
	}
	name := f.Name()
	_, err = f.Write(b)
	if err == nil {
		err = f.Chmod(0o600)
	}
	if err == nil {
		err = f.Sync()
	}
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = os.Rename(name, l.path)
	}
	if err != nil {
		_ = os.Remove(name)
		return fmt.Errorf("assign: keep today's spend: %w", err)
	}
	return nil
}
