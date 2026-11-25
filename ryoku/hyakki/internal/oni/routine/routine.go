// Package routine records short runs of reflex actions, offers to keep one
// once a run, and keeps the routines the user accepted so "run my routine"
// can replay them without asking a model. A routine holds each step's intent
// and its argument (an app name, a site, a search) and nothing else: no words
// that were spoken, no transcript, no window text.
//
// These are not Teams routines. Teams routines are scheduled agent tasks;
// these live in their own file, oni-reflex-routines.json, and run only when
// the user asks.
package routine

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

	"ryoku-rashin/internal/oni/reflex"
	"ryoku-rashin/internal/oni/spec"
	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/protocol"
)

// The spec's reflex.routines numbers.
var (
	// OfferMinSteps is how many replayable steps in a row earn the offer.
	OfferMinSteps = routines().OfferMinSteps
	// Gap is the longest pause between two steps of one run.
	Gap = routines().Gap()
	// MaxRoutines is the most routines kept; the least recently used goes
	// first.
	MaxRoutines = routines().MaxRoutines
	// MaxSteps is the most steps one routine holds.
	MaxSteps = routines().MaxSteps
)

func routines() spec.ReflexRoutines {
	s, err := spec.Load()
	if err != nil {
		panic(err)
	}
	return s.Reflex.Routines
}

// Bounds on what a routine holds.
const (
	MaxNameRunes = 60
	MaxArgRunes  = 200
	// MaxStoreBytes is the largest store file read. Every routine at its
	// bounds, with each byte escaped, still fits.
	MaxStoreBytes = 1 << 20
)

// ReasonDamaged is the protocol error reason for a routines file that cannot
// be read. The file is left as it is until Reset.
const ReasonDamaged = "routines_damaged"

const storeVersion = 1

func damaged() error {
	return &protocol.Error{
		Code:    protocol.CodeConflict,
		Reason:  ReasonDamaged,
		Message: i18n.T("Oni's routines could not be read. Reset them to start again; the old file is kept beside the new one."),
	}
}

func badRequest(message string) error {
	return protocol.Errorf(protocol.CodeBadRequest, "%s", message)
}

func notFound() error {
	return protocol.Errorf(protocol.CodeNotFound, "%s", i18n.T("That routine is not there any more."))
}

// Step is one replayable action: its intent and argument.
type Step struct {
	Intent reflex.Intent `json:"intent"`
	Arg    string        `json:"arg,omitempty"`
}

// Routine is a kept run of steps. Saved is when it was kept, Used when it was
// kept or last run.
type Routine struct {
	Name  string    `json:"name"`
	Steps []Step    `json:"steps"`
	Saved time.Time `json:"saved"`
	Used  time.Time `json:"used"`
}

// Replayable says a step with intent can be recorded, kept and replayed: the
// registry marks it replayable and it needs no Press.
func Replayable(intent reflex.Intent) bool {
	info, ok := reflex.Lookup(intent)
	return ok && info.Replayable && !info.PressOnly
}

// Recorder watches the reflex actions of one run and offers a routine once:
// the first time OfferMinSteps replayable steps follow each other with no
// pause longer than the gap. A step that is not replayable is not recorded
// and does not break the run; a pause longer than the gap starts it again.
type Recorder struct {
	mu       sync.Mutex
	minSteps int
	gap      time.Duration
	steps    []Step
	last     time.Time
	offered  bool
}

// NewRecorder watches for minSteps steps with gaps of at most gap.
func NewRecorder(minSteps int, gap time.Duration) *Recorder {
	return &Recorder{minSteps: max(minSteps, 1), gap: gap}
}

// Observe records a at time at and returns the steps to offer when this
// action completes the run. It offers at most once in the recorder's life.
func (r *Recorder) Observe(a reflex.Action, at time.Time) (offer []Step, ok bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.offered {
		return nil, false
	}
	st, err := checkStep(Step{Intent: a.Intent, Arg: a.Arg})
	if err != nil {
		return nil, false
	}
	if len(r.steps) > 0 && (at.Before(r.last) || at.Sub(r.last) > r.gap) {
		r.steps = r.steps[:0]
	}
	r.steps = append(r.steps, st)
	r.last = at
	if len(r.steps) < r.minSteps {
		return nil, false
	}
	r.offered = true
	offer = slices.Clone(r.steps)
	r.steps = nil
	return offer, true
}

// Offered says the recorder has made its one offer.
func (r *Recorder) Offered() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.offered
}

// Replay is what running a routine does: the actions to take in order, and
// one line for each step that was left out.
type Replay struct {
	Actions []reflex.Action
	Lines   []string
}

// Plan turns r into the actions to replay. A step that opens an app the
// catalog no longer finds is skipped with a line saying so; a nil catalog
// finds no app. No model is asked.
func Plan(r Routine, cat reflex.Catalog) Replay {
	var out Replay
	for _, st := range r.Steps {
		if !Replayable(st.Intent) {
			continue
		}
		if st.Intent == reflex.OpenApp && !installed(cat, st.Arg) {
			// said aloud, so the first sentence keeps to five words
			out.Lines = append(out.Lines, i18n.Tf("Skipped a missing app. %s isn't installed.", st.Arg))
			continue
		}
		out.Actions = append(out.Actions, reflex.Action{Intent: st.Intent, Arg: st.Arg, Confidence: 1})
	}
	return out
}

func installed(cat reflex.Catalog, app string) bool {
	if cat == nil {
		return false
	}
	_, _, _, ok := cat.Resolve(app)
	return ok
}

// DefaultStorePath is the routines file in Hyakki's state directory.
func DefaultStorePath() string { return filepath.Join(paths.StateDir(), "oni-reflex-routines.json") }

// Store keeps the routines in one JSON file, written whole through a synced
// temporary file and renamed into place, mode 0600 in a 0700 directory. It
// writes only when asked to keep, use or forget a routine. A file that cannot
// be read is never overwritten: every call reports it damaged until Reset
// moves it aside.
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

// Put keeps r, saved and used now, replacing a routine with the same name.
// An empty name becomes the first free "Routine N". A step that is not
// replayable refuses the whole routine. Past MaxRoutines the least recently
// used other routine goes.
func (s *Store) Put(r Routine) error {
	r, err := checkRoutine(r, true)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	list, err := s.read()
	if err != nil {
		return err
	}
	if r.Name == "" {
		r.Name = freeName(list)
	}
	at := s.stamp()
	r.Saved, r.Used = at, at
	list = slices.DeleteFunc(list, func(x Routine) bool { return sameName(x.Name, r.Name) })
	for len(list) >= MaxRoutines {
		i := oldest(list)
		list = slices.Delete(list, i, i+1)
	}
	return s.write(append(list, r))
}

// Latest is the routine used most recently; false when there is none or the
// file cannot be read (List reports why).
func (s *Store) Latest() (Routine, bool) {
	list, err := s.List()
	if err != nil || len(list) == 0 {
		return Routine{}, false
	}
	return list[0], true
}

// Find is the routine named name, ignoring case and spacing.
func (s *Store) Find(name string) (Routine, bool, error) {
	list, err := s.List()
	if err != nil {
		return Routine{}, false, err
	}
	i := slices.IndexFunc(list, func(r Routine) bool { return sameName(r.Name, name) })
	if i < 0 {
		return Routine{}, false, nil
	}
	return list[i], true, nil
}

// Ran marks the routine named name used now.
func (s *Store) Ran(name string) error {
	return s.update(name, func(r *Routine) bool {
		r.Used = s.stamp()
		return false
	})
}

// List returns every routine, the most recently used first.
func (s *Store) List() ([]Routine, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	list, err := s.read()
	if err != nil {
		return nil, err
	}
	slices.SortStableFunc(list, func(a, b Routine) int {
		if c := b.Used.Compare(a.Used); c != 0 {
			return c
		}
		return b.Saved.Compare(a.Saved)
	})
	return list, nil
}

// Forget drops the routine named name.
func (s *Store) Forget(name string) error {
	return s.update(name, func(*Routine) bool { return true })
}

// ForgetAll drops every routine and removes the file.
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

// update applies edit to the routine named name and writes the store; edit
// returning true drops it.
func (s *Store) update(name string, edit func(*Routine) bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	list, err := s.read()
	if err != nil {
		return err
	}
	i := slices.IndexFunc(list, func(r Routine) bool { return sameName(r.Name, name) })
	if i < 0 {
		return notFound()
	}
	if edit(&list[i]) {
		list = slices.Delete(list, i, i+1)
	}
	return s.write(list)
}

// stamp is now in UTC to the millisecond, as the file keeps it.
func (s *Store) stamp() time.Time { return s.now().UTC().Truncate(time.Millisecond) }

// oldest is the index of the least recently used routine.
func oldest(list []Routine) int {
	at := 0
	for i, r := range list {
		if r.Used.Before(list[at].Used) || (r.Used.Equal(list[at].Used) && r.Saved.Before(list[at].Saved)) {
			at = i
		}
	}
	return at
}

// freeName is the first "Routine N" no kept routine uses.
func freeName(list []Routine) string {
	for n := 1; ; n++ {
		name := i18n.Tf("Routine %d", n)
		if !slices.ContainsFunc(list, func(r Routine) bool { return sameName(r.Name, name) }) {
			return name
		}
	}
}

func sameName(a, b string) bool { return strings.EqualFold(normal(a), normal(b)) }

func normal(s string) string { return strings.Join(strings.Fields(s), " ") }

type storeFile struct {
	V        int       `json:"v"`
	Routines []Routine `json:"routines"`
}

func (s *Store) read() ([]Routine, error) {
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
	if dec.Decode(&file) != nil || dec.More() || file.V != storeVersion || len(file.Routines) > MaxRoutines {
		return nil, damaged()
	}
	for i, r := range file.Routines {
		checked, err := checkRoutine(r, false)
		if err != nil || checked.Name != r.Name || r.Saved.IsZero() || r.Used.IsZero() ||
			slices.ContainsFunc(file.Routines[:i], func(x Routine) bool { return sameName(x.Name, r.Name) }) {
			return nil, damaged()
		}
		file.Routines[i] = checked
	}
	return file.Routines, nil
}

func (s *Store) write(list []Routine) error {
	if len(list) == 0 {
		list = []Routine{}
	}
	slices.SortFunc(list, func(a, b Routine) int { return strings.Compare(a.Name, b.Name) })
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "\t")
	if err := enc.Encode(storeFile{V: storeVersion, Routines: list}); err != nil {
		return err
	}
	if buf.Len() > MaxStoreBytes {
		return badRequest(i18n.T("Oni's routines are too large to save."))
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

// checkRoutine checks r against the bounds and returns it with its name and
// arguments tidied. An empty name is allowed only when Put will choose one.
func checkRoutine(r Routine, put bool) (Routine, error) {
	r.Name = normal(r.Name)
	if (r.Name == "" && !put) || utf8.RuneCountInString(r.Name) > MaxNameRunes || hasControl(r.Name) {
		return Routine{}, badRequest(i18n.Tf("A routine's name is at most %d characters.", MaxNameRunes))
	}
	if len(r.Steps) == 0 || len(r.Steps) > MaxSteps {
		return Routine{}, badRequest(i18n.Tf("A routine holds 1 to %d steps.", MaxSteps))
	}
	steps := make([]Step, len(r.Steps))
	for i, st := range r.Steps {
		checked, err := checkStep(st)
		if err != nil {
			return Routine{}, err
		}
		steps[i] = checked
	}
	r.Steps = steps
	return r, nil
}

func checkStep(st Step) (Step, error) {
	if !Replayable(st.Intent) {
		return Step{}, badRequest(i18n.T("Only actions that can be done again are kept in a routine."))
	}
	st.Arg = normal(st.Arg)
	if utf8.RuneCountInString(st.Arg) > MaxArgRunes || hasControl(st.Arg) {
		return Step{}, badRequest(i18n.Tf("A routine step's detail is at most %d characters.", MaxArgRunes))
	}
	if st.Intent == reflex.OpenApp && st.Arg == "" {
		return Step{}, badRequest(i18n.T("A routine step that opens an app names the app."))
	}
	return st, nil
}

// hasControl reports control and format runes, which no field keeps.
func hasControl(s string) bool {
	return !utf8.ValidString(s) || strings.ContainsFunc(s, func(r rune) bool {
		return unicode.IsControl(r) || unicode.Is(unicode.Cf, r)
	})
}
