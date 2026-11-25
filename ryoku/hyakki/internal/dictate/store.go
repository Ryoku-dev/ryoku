package dictate

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
	"unicode"
	"unicode/utf8"

	i18n "ryoku-i18n"

	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/protocol"
)

// Personal is the user's own dictation vocabulary. It is dictation's alone:
// the writing dictionary only hints the recogniser at spellings.
type Personal struct {
	Dictionary []Entry   `json:"dictionary"`
	Snippets   []Snippet `json:"snippets"`
	Styles     []Style   `json:"styles"`
}

// Entry writes a spoken phrase the way the user spells it.
type Entry struct {
	Spoken  string `json:"spoken"`
	Written string `json:"written"`
}

// Snippet is text typed as stored when its trigger is said alone or after
// "insert" or "type".
type Snippet struct {
	Trigger string `json:"trigger"`
	Text    string `json:"text"`
}

// Style is how dictation writes into one app. Empty fields keep the
// defaults: the mode asked for, sentence capitals, the end as transcribed.
type Style struct {
	App    string `json:"app"`
	Mode   string `json:"mode,omitempty"`   // exact, clean or polished
	Casing string `json:"casing,omitempty"` // sentence, keep or lower
	End    string `json:"end,omitempty"`    // keep, period or none
}

// Bounds on the store.
const (
	MaxEntries      = 1000
	MaxSnippets     = 200
	MaxStyles       = 100
	MaxPhraseBytes  = 100 // a spoken phrase or snippet trigger
	MaxPhraseWords  = 6
	MaxWrittenBytes = 200
	MaxSnippetBytes = 2000
	MaxAppBytes     = 200
	// MaxStoreBytes is the largest store file read. Every field at its bound,
	// with each byte escaped, still fits.
	MaxStoreBytes = 2 << 20
)

// ReasonDamaged is the protocol error reason for a store file that cannot
// be read. The file is left as it is until Reset.
const ReasonDamaged = "dictation_damaged"

const storeVersion = 1

func damaged() error {
	return &protocol.Error{
		Code:    protocol.CodeConflict,
		Reason:  ReasonDamaged,
		Message: i18n.T("Your dictation words, snippets and styles could not be read. Reset them to start again; the old file is kept beside the new one."),
	}
}

// style returns the style for app, matched without regard to case.
func (p Personal) style(app string) (Style, bool) {
	if app == "" {
		return Style{}, false
	}
	for _, s := range p.Styles {
		if strings.EqualFold(s.App, app) {
			return s, true
		}
	}
	return Style{}, false
}

// DefaultStorePath is the dictation store in Hyakki's state directory.
func DefaultStorePath() string { return filepath.Join(paths.StateDir(), "dictation.json") }

// Store keeps Personal in one JSON file, written whole through a synced
// temporary file and renamed into place, mode 0600 in a 0700 directory. A
// file that cannot be read is never overwritten: every call reports it
// damaged until Reset moves it aside.
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

// Load reads the store; a missing file is an empty store.
func (s *Store) Load() (Personal, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.read()
}

// Save replaces the store with p, normalised: trimmed, one entry per key
// (the last wins), sorted.
func (s *Store) Save(p Personal) (Personal, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, err := s.read(); err != nil {
		return Personal{}, err
	}
	return s.write(p)
}

// SetWord adds or replaces the written form of a spoken phrase.
func (s *Store) SetWord(spoken, written string) (Personal, error) {
	return s.change(func(p *Personal) { p.Dictionary = append(p.Dictionary, Entry{spoken, written}) })
}

// RemoveWord drops a spoken phrase.
func (s *Store) RemoveWord(spoken string) (Personal, error) {
	key, err := phraseKey(spoken, false)
	if err != nil {
		return Personal{}, err
	}
	return s.change(func(p *Personal) {
		p.Dictionary = slices.DeleteFunc(p.Dictionary, func(e Entry) bool { k, _ := phraseKey(e.Spoken, false); return k == key })
	})
}

// SetSnippet adds or replaces a snippet.
func (s *Store) SetSnippet(trigger, text string) (Personal, error) {
	return s.change(func(p *Personal) { p.Snippets = append(p.Snippets, Snippet{trigger, text}) })
}

// RemoveSnippet drops a snippet.
func (s *Store) RemoveSnippet(trigger string) (Personal, error) {
	key, err := phraseKey(trigger, true)
	if err != nil {
		return Personal{}, err
	}
	return s.change(func(p *Personal) {
		p.Snippets = slices.DeleteFunc(p.Snippets, func(e Snippet) bool { k, _ := phraseKey(e.Trigger, true); return k == key })
	})
}

// SetStyle adds or replaces an app's style.
func (s *Store) SetStyle(st Style) (Personal, error) {
	return s.change(func(p *Personal) { p.Styles = append(p.Styles, st) })
}

// RemoveStyle drops an app's style.
func (s *Store) RemoveStyle(app string) (Personal, error) {
	app = strings.TrimSpace(app)
	return s.change(func(p *Personal) {
		p.Styles = slices.DeleteFunc(p.Styles, func(e Style) bool { return strings.EqualFold(e.App, app) })
	})
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

// change applies edit to the stored Personal and writes it when it changed.
func (s *Store) change(edit func(*Personal)) (Personal, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	before, err := s.read()
	if err != nil {
		return Personal{}, err
	}
	p := Personal{
		Dictionary: slices.Clone(before.Dictionary),
		Snippets:   slices.Clone(before.Snippets),
		Styles:     slices.Clone(before.Styles),
	}
	edit(&p)
	after, err := normalize(p)
	if err != nil {
		return Personal{}, err
	}
	if equalPersonal(before, after) {
		return after, nil
	}
	return s.write(after)
}

type storeFile struct {
	V int `json:"v"`
	Personal
}

func (s *Store) read() (Personal, error) {
	f, err := os.Open(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return Personal{}, nil
	}
	if err != nil {
		return Personal{}, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, MaxStoreBytes+1))
	if err != nil {
		return Personal{}, err
	}
	if len(data) > MaxStoreBytes {
		return Personal{}, damaged()
	}
	var file storeFile
	if json.Unmarshal(data, &file) != nil || file.V != storeVersion {
		return Personal{}, damaged()
	}
	p, err := normalize(file.Personal)
	if err != nil {
		return Personal{}, damaged()
	}
	return p, nil
}

func (s *Store) write(p Personal) (Personal, error) {
	p, err := normalize(p)
	if err != nil {
		return Personal{}, err
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "\t")
	if err := enc.Encode(storeFile{V: storeVersion, Personal: p}); err != nil {
		return Personal{}, err
	}
	if buf.Len() > MaxStoreBytes {
		return Personal{}, badRequest(i18n.T("Your dictation words, snippets and styles are too large to save."))
	}
	if err := writeAtomic(s.path, buf.Bytes()); err != nil {
		return Personal{}, err
	}
	return p, nil
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

// normalize checks every field against its bounds, keeps the last of each
// key and sorts by key. Empty lists are written as [], not null.
func normalize(p Personal) (Personal, error) {
	var out Personal
	var err error
	if out.Dictionary, err = normalList(p.Dictionary, MaxEntries,
		i18n.Tf("Your dictionary can hold up to %d words.", MaxEntries),
		func(e Entry) (Entry, string, error) {
			key, err := phraseKey(e.Spoken, false)
			if err != nil {
				return e, "", err
			}
			e.Spoken = strings.Join(strings.Fields(e.Spoken), " ")
			e.Written = strings.TrimSpace(e.Written)
			if e.Written == "" || len(e.Written) > MaxWrittenBytes || !utf8.ValidString(e.Written) || hasControl(e.Written, false) {
				return e, "", badRequest(i18n.Tf("Write the spelling on one line, up to %d bytes.", MaxWrittenBytes))
			}
			return e, key, nil
		}); err != nil {
		return Personal{}, err
	}
	if out.Snippets, err = normalList(p.Snippets, MaxSnippets,
		i18n.Tf("You can keep up to %d snippets.", MaxSnippets),
		func(sn Snippet) (Snippet, string, error) {
			key, err := phraseKey(sn.Trigger, true)
			if err != nil {
				return sn, "", err
			}
			sn.Trigger = strings.Join(strings.Fields(sn.Trigger), " ")
			if strings.TrimSpace(sn.Text) == "" || len(sn.Text) > MaxSnippetBytes || !utf8.ValidString(sn.Text) || hasControl(sn.Text, true) {
				return sn, "", badRequest(i18n.Tf("Write the snippet text, up to %d bytes.", MaxSnippetBytes))
			}
			return sn, key, nil
		}); err != nil {
		return Personal{}, err
	}
	if out.Styles, err = normalList(p.Styles, MaxStyles,
		i18n.Tf("You can keep styles for up to %d apps.", MaxStyles),
		func(st Style) (Style, string, error) {
			st.App = strings.TrimSpace(st.App)
			if st.App == "" || len(st.App) > MaxAppBytes || !utf8.ValidString(st.App) || strings.IndexFunc(st.App, func(r rune) bool { return unicode.IsSpace(r) || unicode.IsControl(r) }) >= 0 {
				return st, "", badRequest(i18n.Tf("Name the app by its id, without spaces, up to %d bytes.", MaxAppBytes))
			}
			st.Mode, st.Casing, st.End = strings.ToLower(st.Mode), strings.ToLower(st.Casing), strings.ToLower(st.End)
			if st.Mode != "" && !slices.Contains(modeNames[:], st.Mode) {
				return st, "", badRequest(i18n.T("Choose exact, clean or polished dictation."))
			}
			if st.Casing != "" && st.Casing != CasingSentence && st.Casing != CasingKeep && st.Casing != CasingLower {
				return st, "", badRequest(i18n.T("Choose sentence, keep or lower casing."))
			}
			if st.End != "" && st.End != EndKeep && st.End != EndPeriod && st.End != EndNone {
				return st, "", badRequest(i18n.T("Choose keep, period or none for the ending."))
			}
			return st, strings.ToLower(st.App), nil
		}); err != nil {
		return Personal{}, err
	}
	return out, nil
}

// normalList checks each item, keeps the last per key, sorts by key and
// bounds the count.
func normalList[T any](items []T, limit int, tooMany string, check func(T) (T, string, error)) ([]T, error) {
	type keyed struct {
		key  string
		item T
	}
	byKey := make(map[string]int, len(items))
	list := make([]keyed, 0, len(items))
	for _, item := range items {
		item, key, err := check(item)
		if err != nil {
			return nil, err
		}
		if at, ok := byKey[key]; ok {
			list[at].item = item
			continue
		}
		byKey[key] = len(list)
		list = append(list, keyed{key, item})
	}
	if len(list) > limit {
		return nil, badRequest(tooMany)
	}
	slices.SortFunc(list, func(a, b keyed) int { return strings.Compare(a.key, b.key) })
	out := make([]T, len(list))
	for k, e := range list {
		out[k] = e.item
	}
	return out, nil
}

// phraseKey checks a spoken phrase or snippet trigger and returns its key:
// its words in lower case. A trigger cannot start with "insert" or "type",
// the words that ask for a snippet.
func phraseKey(phrase string, trigger bool) (string, error) {
	invalid := badRequest(i18n.Tf("Say it in up to %d words, without punctuation, up to %d bytes.", MaxPhraseWords, MaxPhraseBytes))
	if !utf8.ValidString(phrase) {
		return "", invalid
	}
	phrase = strings.TrimSpace(phrase)
	if phrase == "" || len(phrase) > MaxPhraseBytes || hasControl(phrase, false) {
		return "", invalid
	}
	said, ok := words(phrase)
	if !ok || len(said) > MaxPhraseWords {
		return "", invalid
	}
	if trigger && (said[0] == "insert" || said[0] == "type") {
		return "", badRequest(i18n.T("A snippet's trigger cannot start with “insert” or “type”."))
	}
	return strings.Join(said, " "), nil
}

// hasControl reports control characters; lines allows line breaks and tabs.
func hasControl(s string, lines bool) bool {
	return strings.IndexFunc(s, func(r rune) bool {
		return unicode.IsControl(r) && !(lines && (r == '\n' || r == '\t'))
	}) >= 0
}

func equalPersonal(a, b Personal) bool {
	return slices.Equal(a.Dictionary, b.Dictionary) && slices.Equal(a.Snippets, b.Snippets) && slices.Equal(a.Styles, b.Styles)
}
