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

	i18n "ryoku-i18n"

	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/protocol"
)

// The spec's guide.hints numbers.
var (
	// HintCooldown is the least time between two hints, whichever kept
	// walkthroughs they offer.
	HintCooldown = time.Duration(guide().Hints.CooldownS * float64(time.Second))
	// HintOncePerDay says each kept walkthrough is offered as a hint at
	// most once a day.
	HintOncePerDay = guide().Hints.OncePerKeyPerDay
)

// MaxHintsBytes is the largest hints file read. Every mark at its bounds,
// with each byte escaped, still fits.
const MaxHintsBytes = 1 << 20

// ReasonHintsDamaged is the protocol error reason for a hints file that
// cannot be read. The file is left as it is, and no hint is due meanwhile.
const ReasonHintsDamaged = "hints_damaged"

const hintsVersion = 1

func hintsDamaged() error {
	return &protocol.Error{
		Code:    protocol.CodeConflict,
		Reason:  ReasonHintsDamaged,
		Message: i18n.T("Oni's hint record could not be read, so no hints are offered. Start over to move it aside; the old file is kept beside the new one."),
	}
}

// DefaultHintsPath is the hints file beside the keeps in Hyakki's state
// directory.
func DefaultHintsPath() string { return filepath.Join(paths.StateDir(), "oni-hints.json") }

// Hints remembers when kept walkthroughs were offered as hints, so hints
// keep to the spec: at most one every HintCooldown, each kept walkthrough at
// most once a day when HintOncePerDay, and none for an app the user said
// Not now to that day. A day is the calendar day of the clock handed in.
// It holds the keys of kept walkthroughs, app names and times, nothing
// else: never a window title, a picture or text read off the screen.
//
// The file is written whole through a synced temporary file and renamed
// into place, mode 0600 in a 0700 directory, as the keeps are. A file that
// cannot be read is never overwritten: nothing is due until Reset moves it
// aside.
type Hints struct {
	mu   sync.Mutex
	path string
}

// NewHints opens the hints record at path; an empty path is
// DefaultHintsPath.
func NewHints(path string) *Hints {
	if path == "" {
		path = DefaultHintsPath()
	}
	return &Hints{path: path}
}

// Hints is the hint record beside the store, oni-hints.json in its folder.
func (s *Store) Hints() *Hints {
	return NewHints(filepath.Join(filepath.Dir(s.path), "oni-hints.json"))
}

// Path is the hints file.
func (h *Hints) Path() string { return h.path }

// DamagedPath is where Reset moves a hints file.
func (h *Hints) DamagedPath() string { return h.path + ".damaged" }

// Check reads the record and reports why it cannot be used, nil when it
// can.
func (h *Hints) Check() error {
	h.mu.Lock()
	defer h.mu.Unlock()
	_, err := h.read()
	return err
}

// ForgetAll drops every mark and removes the file. A damaged file is left
// for Reset.
func (h *Hints) ForgetAll() error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, err := h.read(); err != nil {
		return err
	}
	if err := os.Remove(h.path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

// Reset moves the hints file to DamagedPath, replacing any earlier one, and
// leaves the record empty.
func (h *Hints) Reset() error {
	h.mu.Lock()
	defer h.mu.Unlock()
	err := os.Rename(h.path, h.DamagedPath())
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

// Offer picks the first of keys, kept walkthroughs for app in the order
// they should be offered, that may be hinted at now, and records it
// offered. It returns "" when the cooldown since the last hint has not
// passed, when app was snoozed today, or when every key was hinted today
// already.
func (h *Hints) Offer(app string, keys []string, now time.Time) (string, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	file, err := h.read()
	if err != nil {
		return "", err
	}
	if !file.Last.IsZero() && absDuration(now.Sub(file.Last)) < HintCooldown {
		return "", nil
	}
	app = hintApp(app)
	if slices.ContainsFunc(file.Snoozed, func(m snoozeMark) bool { return m.App == app && sameDay(m.At, now) }) {
		return "", nil
	}
	for _, key := range keys {
		if HintOncePerDay && slices.ContainsFunc(file.Shown, func(m shownMark) bool { return m.Key == key && sameDay(m.At, now) }) {
			continue
		}
		if !validHintKey(key) {
			continue
		}
		at := stampOf(now)
		file.Last = at
		file.Shown = slices.DeleteFunc(file.Shown, func(m shownMark) bool { return m.Key == key })
		file.Shown = append(file.Shown, shownMark{Key: key, At: at})
		return key, h.write(file, now)
	}
	return "", nil
}

// Snooze records the user's Not now for app at now: no hint is offered for
// it for the rest of that day.
func (h *Hints) Snooze(app string, now time.Time) error {
	app = hintApp(app)
	if !validHintApp(app) {
		return protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("A hint names the app it was offered in."))
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	file, err := h.read()
	if err != nil {
		return err
	}
	file.Snoozed = slices.DeleteFunc(file.Snoozed, func(m snoozeMark) bool { return m.App == app })
	file.Snoozed = append(file.Snoozed, snoozeMark{App: app, At: stampOf(now)})
	return h.write(file, now)
}

type shownMark struct {
	Key string    `json:"key"`
	At  time.Time `json:"at"`
}

type snoozeMark struct {
	App string    `json:"app"`
	At  time.Time `json:"at"`
}

type hintsFile struct {
	V       int          `json:"v"`
	Last    time.Time    `json:"last,omitzero"`
	Shown   []shownMark  `json:"shown"`
	Snoozed []snoozeMark `json:"snoozed"`
}

func (h *Hints) read() (hintsFile, error) {
	f, err := os.Open(h.path)
	if errors.Is(err, os.ErrNotExist) {
		return hintsFile{V: hintsVersion}, nil
	}
	if err != nil {
		return hintsFile{}, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, MaxHintsBytes+1))
	if err != nil {
		return hintsFile{}, err
	}
	if len(data) > MaxHintsBytes {
		return hintsFile{}, hintsDamaged()
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var file hintsFile
	if dec.Decode(&file) != nil || dec.More() || file.V != hintsVersion || len(file.Shown) > MaxEntries || len(file.Snoozed) > MaxEntries {
		return hintsFile{}, hintsDamaged()
	}
	keys := make(map[string]bool, len(file.Shown))
	for _, m := range file.Shown {
		if !validHintKey(m.Key) || keys[m.Key] || m.At.IsZero() {
			return hintsFile{}, hintsDamaged()
		}
		keys[m.Key] = true
	}
	apps := make(map[string]bool, len(file.Snoozed))
	for _, m := range file.Snoozed {
		if !validHintApp(m.App) || apps[m.App] || m.At.IsZero() {
			return hintsFile{}, hintsDamaged()
		}
		apps[m.App] = true
	}
	return file, nil
}

// write keeps only today's marks, at most MaxEntries of each, the newest.
func (h *Hints) write(file hintsFile, now time.Time) error {
	file.V = hintsVersion
	file.Shown = slices.DeleteFunc(file.Shown, func(m shownMark) bool { return !sameDay(m.At, now) })
	file.Snoozed = slices.DeleteFunc(file.Snoozed, func(m snoozeMark) bool { return !sameDay(m.At, now) })
	slices.SortStableFunc(file.Shown, func(a, b shownMark) int { return b.At.Compare(a.At) })
	slices.SortStableFunc(file.Snoozed, func(a, b snoozeMark) int { return b.At.Compare(a.At) })
	file.Shown = file.Shown[:min(len(file.Shown), MaxEntries)]
	file.Snoozed = file.Snoozed[:min(len(file.Snoozed), MaxEntries)]
	if file.Shown == nil {
		file.Shown = []shownMark{}
	}
	if file.Snoozed == nil {
		file.Snoozed = []snoozeMark{}
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "\t")
	if err := enc.Encode(file); err != nil {
		return err
	}
	return writeAtomic(h.path, buf.Bytes())
}

func hintApp(app string) string { return strings.ToLower(strings.TrimSpace(app)) }

func validHintApp(app string) bool {
	return app != "" && len(app) <= MaxAppBytes && app == hintApp(app) && !hasControl(app) && !strings.Contains(app, "::")
}

// validHintKey says key is shaped as KeyFor makes keys.
func validHintKey(key string) bool {
	app, slug, ok := strings.Cut(key, "::")
	return ok && validHintApp(app) && slug != "" && len(slug) <= 4*MaxGoalRunes && !hasControl(slug) && !strings.Contains(slug, "::")
}

// sameDay says a falls on now's calendar day, in now's location.
func sameDay(a, now time.Time) bool {
	ay, am, ad := a.In(now.Location()).Date()
	ny, nm, nd := now.Date()
	return ay == ny && am == nm && ad == nd
}

func stampOf(t time.Time) time.Time { return t.UTC().Truncate(time.Millisecond) }

func absDuration(d time.Duration) time.Duration {
	if d < 0 {
		return -d
	}
	return d
}
