package remind

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	i18n "ryoku-i18n"

	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/protocol"
)

// Protocol error reasons.
const (
	// ReasonDamaged is a reminders file that cannot be read. It is left as
	// it is until Reset.
	ReasonDamaged = "reminders_damaged"
	// ReasonFull is a new reminder refused because MaxPending are kept.
	ReasonFull = "reminders_full"
)

// MaxStoreBytes is the largest reminders file read. MaxPending reminders at
// their bounds, with every byte escaped, still fit.
const MaxStoreBytes = 1 << 20

// dstSlack is how far a reminder kept within MaxDays on the wall clock may
// lie past MaxReminder elapsed, when the clock is set back in between.
const dstSlack = 2 * time.Hour

const storeVersion = 1

func damaged() error {
	return &protocol.Error{
		Code:    protocol.CodeConflict,
		Reason:  ReasonDamaged,
		Message: i18n.T("Oni's reminders could not be read. Reset them to start again; the old file is kept beside the new one."),
	}
}

func full() error {
	return &protocol.Error{
		Code:    protocol.CodeConflict,
		Reason:  ReasonFull,
		Message: i18n.T("You have too many reminders."),
	}
}

func badRequest(message string) error {
	return protocol.Errorf(protocol.CodeBadRequest, "%s", message)
}

// DefaultStorePath is the reminders file in Hyakki's state directory.
func DefaultStorePath() string { return filepath.Join(paths.StateDir(), "oni-reminders.json") }

// Store keeps pending reminders and timers in one JSON file, written whole
// through a synced temporary file and renamed into place, mode 0600 in a
// 0700 directory. A reminder holds its own words and times and nothing
// else. With nothing pending there is no file. A file that cannot be read
// is never overwritten: every call reports it damaged, or finds nothing,
// until Reset moves it aside.
type Store struct {
	mu   sync.Mutex
	path string
}

// NewStore opens the store at path; an empty path is DefaultStorePath.
func NewStore(path string) *Store {
	if path == "" {
		path = DefaultStorePath()
	}
	return &Store{path: path}
}

// Path is the store's file.
func (s *Store) Path() string { return s.path }

// DamagedPath is where Reset moves a store file.
func (s *Store) DamagedPath() string { return s.path + ".damaged" }

// Add keeps r. Its ID must be new, its kind known, its due time after its
// creation and within the limit for its kind, and its words bounded and
// free of control characters. At MaxPending the reminder is refused, and
// none kept is dropped.
func (s *Store) Add(r Reminder) error {
	r, err := check(r)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	rs, err := s.read()
	if err != nil {
		return err
	}
	if slices.ContainsFunc(rs, func(x Reminder) bool { return x.ID == r.ID }) {
		return badRequest(i18n.T("That reminder is already set."))
	}
	if len(rs) >= MaxPending {
		return full()
	}
	return s.write(append(rs, r))
}

// Cancel takes away the reminder with id, reporting whether it was there.
func (s *Store) Cancel(id string) bool {
	n, _ := s.remove(func(r Reminder) bool { return r.ID == id })
	return n == 1
}

// CancelTimers takes away every timer, returning how many.
func (s *Store) CancelTimers() int {
	n, _ := s.remove(func(r Reminder) bool { return r.Kind == KindTimer })
	return n
}

// CancelReminders takes away every reminder that is not a timer, returning
// how many.
func (s *Store) CancelReminders() int {
	n, _ := s.remove(func(r Reminder) bool { return r.Kind == KindReminder })
	return n
}

// CancelAll takes away every reminder and timer, returning how many.
func (s *Store) CancelAll() int {
	n, _ := s.remove(func(Reminder) bool { return true })
	return n
}

// CancelNext takes away the soonest due reminder of kind, returning it.
func (s *Store) CancelNext(kind string) (Reminder, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rs, err := s.read()
	if err != nil {
		return Reminder{}, false
	}
	i := slices.IndexFunc(rs, func(r Reminder) bool { return r.Kind == kind })
	if i < 0 {
		return Reminder{}, false
	}
	r := rs[i]
	if s.write(slices.Delete(rs, i, i+1)) != nil {
		return Reminder{}, false
	}
	return r, true
}

// CancelBy takes away what k names, returning how many went.
func (s *Store) CancelBy(k CancelKind) int {
	switch k {
	case CancelTimers:
		return s.CancelTimers()
	case CancelReminders:
		return s.CancelReminders()
	case CancelAll:
		return s.CancelAll()
	case CancelReminder:
		if _, ok := s.CancelNext(KindReminder); ok {
			return 1
		}
	}
	return 0
}

// List returns every pending reminder and timer, soonest first.
func (s *Store) List() ([]Reminder, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.read()
}

// Next is the soonest reminder due after now. Those due at or before now
// are Due's.
func (s *Store) Next(now time.Time) (Reminder, bool) {
	rs, err := s.List()
	if err != nil {
		return Reminder{}, false
	}
	i := slices.IndexFunc(rs, func(r Reminder) bool { return r.Due.After(now) })
	if i < 0 {
		return Reminder{}, false
	}
	return rs[i], true
}

// Due returns every reminder due at or before now, soonest first: the ones
// to fire. Each stays until Done.
func (s *Store) Due(now time.Time) []Reminder {
	rs, err := s.List()
	if err != nil {
		return nil
	}
	return slices.DeleteFunc(rs, func(r Reminder) bool { return r.Due.After(now) })
}

// Done takes away the reminder with id once it has fired.
func (s *Store) Done(id string) error {
	n, err := s.remove(func(r Reminder) bool { return r.ID == id })
	if err == nil && n == 0 {
		err = protocol.Errorf(protocol.CodeNotFound, "%s", i18n.T("That reminder is not there any more."))
	}
	return err
}

// DueWithin reports whether a reminder is due within d of now, or already
// due. Due times are instants, so a daylight saving change or a midnight in
// between changes nothing. The wake path reads this with no daemon running;
// a store that cannot be read is due for nothing.
func (s *Store) DueWithin(now time.Time, d time.Duration) bool {
	rs, err := s.List()
	if err != nil || len(rs) == 0 {
		return false
	}
	return !rs[0].Due.After(now.Add(d))
}

// Reset moves the store file to DamagedPath, replacing any earlier one, and
// leaves the store empty.
func (s *Store) Reset() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	err := os.Rename(s.path, s.DamagedPath())
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

// remove drops every reminder drop matches and writes the store when any
// went.
func (s *Store) remove(drop func(Reminder) bool) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rs, err := s.read()
	if err != nil {
		return 0, err
	}
	kept := slices.DeleteFunc(slices.Clone(rs), drop)
	n := len(rs) - len(kept)
	if n == 0 {
		return 0, nil
	}
	return n, s.write(kept)
}

type storeFile struct {
	V         int        `json:"v"`
	Reminders []Reminder `json:"reminders"`
}

// read returns the pending reminders, soonest first.
func (s *Store) read() ([]Reminder, error) {
	f, err := os.Open(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, MaxStoreBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > MaxStoreBytes {
		return nil, damaged()
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var file storeFile
	if dec.Decode(&file) != nil || dec.More() || file.V != storeVersion || len(file.Reminders) > MaxPending {
		return nil, damaged()
	}
	seen := make(map[string]bool, len(file.Reminders))
	for i, r := range file.Reminders {
		checked, err := check(r)
		if err != nil || checked.Text != r.Text || seen[r.ID] {
			return nil, damaged()
		}
		seen[r.ID] = true
		file.Reminders[i] = checked
	}
	sortDue(file.Reminders)
	return file.Reminders, nil
}

// write replaces the file with rs, or removes it when rs is empty, so
// nothing is kept at rest once nothing is pending.
func (s *Store) write(rs []Reminder) error {
	if len(rs) == 0 {
		if err := os.Remove(s.path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		return nil
	}
	sortDue(rs)
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "\t")
	if err := enc.Encode(storeFile{V: storeVersion, Reminders: rs}); err != nil {
		return err
	}
	if buf.Len() > MaxStoreBytes {
		return badRequest(i18n.T("Oni's reminders are too large to save."))
	}
	return writeAtomic(s.path, buf.Bytes())
}

func sortDue(rs []Reminder) {
	slices.SortStableFunc(rs, func(a, b Reminder) int {
		if c := a.Due.Compare(b.Due); c != 0 {
			return c
		}
		return strings.Compare(a.ID, b.ID)
	})
}

// writeAtomic replaces path with data through a synced temporary file.
func writeAtomic(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

// check checks r against the bounds and returns it with its times in UTC.
func check(r Reminder) (Reminder, error) {
	switch {
	case !validID(r.ID):
		return r, badRequest(i18n.T("A reminder needs a valid id."))
	case r.Kind != KindReminder && r.Kind != KindTimer:
		return r, badRequest(i18n.T("A reminder is a reminder or a timer."))
	case r.Created.IsZero() || r.Due.IsZero() || !r.Due.After(r.Created):
		return r, badRequest(i18n.T("A reminder must fall due after it is set."))
	}
	limit := MaxReminder + dstSlack
	if r.Kind == KindTimer {
		limit = MaxTimer
	}
	if r.Due.Sub(r.Created) > limit {
		return r, badRequest(i18n.T("That reminder lies too far ahead."))
	}
	if !utf8.ValidString(r.Text) || utf8.RuneCountInString(r.Text) > MaxTextRunes || strings.TrimSpace(r.Text) != r.Text ||
		strings.IndexFunc(r.Text, func(c rune) bool { return unicode.IsControl(c) || unicode.Is(unicode.Cf, c) }) >= 0 {
		return r, badRequest(i18n.T("A reminder's words must be short plain text."))
	}
	r.Due, r.Created = r.Due.UTC(), r.Created.UTC()
	return r, nil
}
