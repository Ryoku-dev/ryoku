// Package keep remembers walkthroughs the user chose to keep, so the same
// goal in the same app can be shown again without asking a model. An entry
// holds the goal, and for each step its sentence, the control's label and
// role, and where the control sat in the window. It never holds a picture, a
// window title, text read off the screen beyond each step's own label, or
// anything the user typed into a field.
package keep

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

	"ryoku-rashin/internal/oni/feature"
	"ryoku-rashin/internal/oni/screenask"
	"ryoku-rashin/internal/oni/spec"
	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/protocol"
)

// The spec's guide.keep numbers and step ceiling.
var (
	// MaxEntries is the most walkthroughs kept; the least recently used goes
	// first.
	MaxEntries = guide().Keep.MaxEntries
	// MatchMin is the least goal similarity, 0..1, at which a request
	// replays a kept walkthrough.
	MatchMin = guide().Keep.MatchMin
	// FindMin is the least Score, 0..100, at which a replayed step's
	// control counts as found.
	FindMin = guide().Keep.FindMin
	// MissesToStale is how many replays in a row may miss before the entry
	// retires.
	MissesToStale = guide().Keep.MissesToStale
	// MaxSteps is the most steps one walkthrough holds.
	MaxSteps = guide().MaxSteps
)

func guide() spec.Guide {
	s, err := spec.Load()
	if err != nil {
		panic(err)
	}
	return s.Guide
}

// Bounds on what an entry holds.
const (
	MaxAppBytes   = 200
	MaxGoalRunes  = 300
	MaxTextRunes  = 200
	MaxLabelRunes = 80
	MaxRoleBytes  = 40
	// MaxStoreBytes is the largest store file read. Every entry at its
	// bounds, with each byte escaped, still fits.
	MaxStoreBytes = 16 << 20
)

// ReasonDamaged is the protocol error reason for a keeps file that cannot be
// read. The file is left as it is until Reset.
const ReasonDamaged = "keeps_damaged"

const storeVersion = 1

func damaged() error {
	return &protocol.Error{
		Code:    protocol.CodeConflict,
		Reason:  ReasonDamaged,
		Message: i18n.T("Oni's kept walkthroughs could not be read. Reset them to start again; the old file is kept beside the new one."),
	}
}

func badRequest(message string) error {
	return protocol.Errorf(protocol.CodeBadRequest, "%s", message)
}

func notFound() error {
	return protocol.Errorf(protocol.CodeNotFound, "%s", i18n.T("That kept walkthrough is not there any more."))
}

// Step is one kept step. Box is the control in fractions of the window;
// Region is the named area a region step drew, also in fractions. Trusted
// says the step was verified, sure and followed by an observed change or a
// Next; it is never written, Put refuses a step without it, and a step read
// back from the store has it.
type Step struct {
	Text         string       `json:"text"`
	Label        string       `json:"label"`
	Role         string       `json:"role,omitempty"`
	Box          feature.Box  `json:"box"`
	Region       *feature.Box `json:"region,omitempty"`
	Mode         string       `json:"mode"`
	Irreversible bool         `json:"irreversible,omitempty"`
	Trusted      bool         `json:"-"`
}

// Entry is one kept walkthrough. Key is KeyFor(App, Goal).
type Entry struct {
	Key    string    `json:"key"`
	App    string    `json:"app"`
	Goal   string    `json:"goal"`
	Steps  []Step    `json:"steps"`
	Misses int       `json:"misses,omitempty"`
	Kept   time.Time `json:"kept"`
	Used   time.Time `json:"used"`
}

// DefaultStorePath is the keeps file in Hyakki's state directory.
func DefaultStorePath() string { return filepath.Join(paths.StateDir(), "oni-keeps.json") }

// Store keeps the entries in one JSON file, written whole through a synced
// temporary file and renamed into place, mode 0600 in a 0700 directory. A
// file that cannot be read is never overwritten: every call reports it
// damaged until Reset moves it aside.
type Store struct {
	mu   sync.Mutex
	path string
	now  func() time.Time
}

// NewStore opens the store at path; an empty path is DefaultStorePath.
func NewStore(path string) *Store {
	if path == "" {
		path = DefaultStorePath()
	}
	return &Store{path: path, now: time.Now}
}

// Path is the store's file.
func (s *Store) Path() string { return s.path }

// DamagedPath is where Reset moves a store file.
func (s *Store) DamagedPath() string { return s.path + ".damaged" }

// Put keeps e, replacing an entry with the same key. App, Goal and every
// step are checked against their bounds; a step that is not Trusted, or
// whose label or role names a private field, refuses the whole entry. The
// entry is kept and used now with no misses. Past MaxEntries the least
// recently used other entry goes.
func (s *Store) Put(e Entry) error {
	e, err := checkEntry(e)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	entries, err := s.read()
	if err != nil {
		return err
	}
	at := s.stamp()
	e.Kept, e.Used, e.Misses = at, at, 0
	entries = slices.DeleteFunc(entries, func(x Entry) bool { return x.Key == e.Key })
	for len(entries) >= MaxEntries {
		i := oldest(entries)
		entries = slices.Delete(entries, i, i+1)
	}
	return s.write(append(entries, e))
}

// Match finds the kept walkthrough for goal in app: the same app, ignoring
// case, and the goal most like it, at least MatchMin; ties go to the one used
// last. A store that cannot be read matches nothing; List reports why.
func (s *Store) Match(app, goal string) (Entry, float64, bool) {
	want := goalTokens(goal)
	if len(want) == 0 {
		return Entry{}, 0, false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	entries, err := s.read()
	if err != nil {
		return Entry{}, 0, false
	}
	best, score := -1, 0.0
	for i, e := range entries {
		if !strings.EqualFold(e.App, strings.TrimSpace(app)) {
			continue
		}
		sim := similarity(want, goalTokens(e.Goal))
		if sim > score || (sim == score && best >= 0 && e.Used.After(entries[best].Used)) {
			best, score = i, sim
		}
	}
	if best < 0 || score < MatchMin {
		return Entry{}, score, false
	}
	return clone(entries[best]), score, true
}

// Hit records a replay that found every step: the entry is used now and its
// misses clear.
func (s *Store) Hit(key string) error {
	_, err := s.update(key, func(e *Entry) bool {
		e.Used, e.Misses = s.stamp(), 0
		return false
	})
	return err
}

// Miss records a replay that could not find a step. At MissesToStale misses
// in a row the entry retires and stale is true.
func (s *Store) Miss(key string) (stale bool, err error) {
	return s.update(key, func(e *Entry) bool {
		e.Misses++
		return e.Misses >= MissesToStale
	})
}

// List returns every entry, the most recently used first.
func (s *Store) List() ([]Entry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entries, err := s.read()
	if err != nil {
		return nil, err
	}
	slices.SortStableFunc(entries, func(a, b Entry) int { return b.Used.Compare(a.Used) })
	return entries, nil
}

// Forget drops the entry with key.
func (s *Store) Forget(key string) error {
	_, err := s.update(key, func(*Entry) bool { return true })
	return err
}

// ForgetAll drops every entry and removes the file.
func (s *Store) ForgetAll() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, err := s.read(); err != nil {
		return err
	}
	if err := os.Remove(s.path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
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

// update applies edit to the entry with key and writes the store; edit
// returning true drops the entry, and update reports that it did.
func (s *Store) update(key string, edit func(*Entry) bool) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entries, err := s.read()
	if err != nil {
		return false, err
	}
	i := slices.IndexFunc(entries, func(e Entry) bool { return e.Key == key })
	if i < 0 {
		return false, notFound()
	}
	drop := edit(&entries[i])
	if drop {
		entries = slices.Delete(entries, i, i+1)
	}
	return drop, s.write(entries)
}

// stamp is now in UTC to the millisecond, as the file keeps it.
func (s *Store) stamp() time.Time { return s.now().UTC().Truncate(time.Millisecond) }

// oldest is the index of the least recently used entry.
func oldest(entries []Entry) int {
	at := 0
	for i, e := range entries {
		if e.Used.Before(entries[at].Used) || (e.Used.Equal(entries[at].Used) && e.Kept.Before(entries[at].Kept)) {
			at = i
		}
	}
	return at
}

type storeFile struct {
	V       int     `json:"v"`
	Entries []Entry `json:"entries"`
}

func (s *Store) read() ([]Entry, error) {
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
	if dec.Decode(&file) != nil || dec.More() || file.V != storeVersion || len(file.Entries) > MaxEntries {
		return nil, damaged()
	}
	seen := make(map[string]bool, len(file.Entries))
	for i, e := range file.Entries {
		for j := range e.Steps {
			e.Steps[j].Trusted = true
		}
		checked, err := checkEntry(e)
		if err != nil || checked.Key != e.Key || seen[e.Key] || e.Misses < 0 || e.Misses >= MissesToStale || e.Kept.IsZero() || e.Used.IsZero() {
			return nil, damaged()
		}
		seen[e.Key] = true
		file.Entries[i] = checked
	}
	return file.Entries, nil
}

func (s *Store) write(entries []Entry) error {
	if len(entries) == 0 {
		entries = []Entry{}
	}
	slices.SortFunc(entries, func(a, b Entry) int { return strings.Compare(a.Key, b.Key) })
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "\t")
	if err := enc.Encode(storeFile{V: storeVersion, Entries: entries}); err != nil {
		return err
	}
	if buf.Len() > MaxStoreBytes {
		return badRequest(i18n.T("Oni's kept walkthroughs are too large to save."))
	}
	return writeAtomic(s.path, buf.Bytes())
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

// checkEntry checks e against the bounds and returns it trimmed, with its
// key derived from App and Goal. Every step must be Trusted.
func checkEntry(e Entry) (Entry, error) {
	e.App = strings.TrimSpace(e.App)
	if e.App == "" || len(e.App) > MaxAppBytes || hasControl(e.App) || strings.Contains(e.App, "::") {
		return Entry{}, badRequest(i18n.T("A kept walkthrough needs the app it was shown in."))
	}
	e.Goal = strings.Join(strings.Fields(e.Goal), " ")
	if e.Goal == "" || utf8.RuneCountInString(e.Goal) > MaxGoalRunes || hasControl(e.Goal) || len(goalTokens(e.Goal)) == 0 {
		return Entry{}, badRequest(i18n.Tf("A kept walkthrough needs its goal in at most %d characters.", MaxGoalRunes))
	}
	if len(e.Steps) == 0 || len(e.Steps) > MaxSteps {
		return Entry{}, badRequest(i18n.Tf("A kept walkthrough holds 1 to %d steps.", MaxSteps))
	}
	steps := make([]Step, len(e.Steps))
	for i, st := range e.Steps {
		checked, err := checkStep(st)
		if err != nil {
			return Entry{}, err
		}
		steps[i] = checked
	}
	e.Steps = steps
	e.Key = KeyFor(e.App, e.Goal)
	return e, nil
}

func checkStep(st Step) (Step, error) {
	if !st.Trusted {
		return Step{}, badRequest(i18n.T("Only steps that were checked on the screen can be kept."))
	}
	st.Text = strings.Join(strings.Fields(st.Text), " ")
	st.Label = strings.Join(strings.Fields(st.Label), " ")
	st.Role = strings.TrimSpace(st.Role)
	switch {
	case st.Text == "" || utf8.RuneCountInString(st.Text) > MaxTextRunes || hasControl(st.Text):
		return Step{}, badRequest(i18n.Tf("A kept step needs its sentence in at most %d characters.", MaxTextRunes))
	case utf8.RuneCountInString(st.Label) > MaxLabelRunes || hasControl(st.Label) || len(words(st.Label)) == 0:
		return Step{}, badRequest(i18n.Tf("A kept step needs the control's label in at most %d characters.", MaxLabelRunes))
	case len(st.Role) > MaxRoleBytes || hasControl(st.Role):
		return Step{}, badRequest(i18n.Tf("A kept step's role is at most %d bytes.", MaxRoleBytes))
	case !fraction(st.Box):
		return Step{}, badRequest(i18n.T("A kept step's control must lie inside the window."))
	}
	switch st.Mode {
	case feature.StrandModePoint, feature.StrandModeLasso:
		if st.Region != nil {
			return Step{}, badRequest(i18n.T("Only a region step keeps a region."))
		}
	case feature.StrandModeRegion:
		if st.Region == nil || !fraction(*st.Region) {
			return Step{}, badRequest(i18n.T("A region step keeps its region inside the window."))
		}
		r := *st.Region
		st.Region = &r
	default:
		return Step{}, badRequest(i18n.T("A kept step points, lassos or draws a region."))
	}
	if _, private := screenask.Sensitive(st.Role+" "+st.Label, nil, st.Box); private {
		return Step{}, badRequest(screenask.RefusalLine)
	}
	return st, nil
}

// fraction says b is a finite, non-empty box inside the unit square.
func fraction(b feature.Box) bool {
	return finite(b) && b.X >= 0 && b.Y >= 0 && b.W > 0 && b.H > 0 && b.X+b.W <= 1 && b.Y+b.H <= 1
}

// hasControl reports control and format runes, which no field keeps.
func hasControl(s string) bool {
	return !utf8.ValidString(s) || strings.ContainsFunc(s, func(r rune) bool {
		return unicode.IsControl(r) || unicode.Is(unicode.Cf, r)
	})
}

func clone(e Entry) Entry {
	e.Steps = slices.Clone(e.Steps)
	for i, st := range e.Steps {
		if st.Region != nil {
			r := *st.Region
			e.Steps[i].Region = &r
		}
	}
	return e
}
