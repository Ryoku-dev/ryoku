package memory

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"go.etcd.io/bbolt"

	i18n "ryoku-i18n"
	"ryoku-rashin/internal/memory/scan"
	"ryoku-rashin/internal/protocol"
)

// What an Op does.
const (
	OpAdd     = "add"
	OpReplace = "replace"
	OpForget  = "forget"
	OpEdit    = "edit"
	OpSuggest = "suggest"
	OpKeep    = "keep"
	OpDismiss = "dismiss"
	OpRestore = "restore"
	OpPin     = "pin"
	OpUnpin   = "unpin"
)

// Op is one step of a change. Add and suggest create a memory from Text in
// Scope; replace creates one from Text that supersedes ID; every other op
// acts on ID. Edit sets Text and, when it is not empty, Scope.
type Op struct {
	Kind      string
	ID        string
	Text      string
	Scope     string
	Retain    string // protocol.MemoryPermanent, MemoryDurable or MemoryEphemeral
	Pinned    bool
	Sensitive bool
	// Explicit marks a fact the user asked to keep; the card prefers it.
	Explicit bool
	Source   protocol.MemorySource
	// Replaces are ids an add supersedes.
	Replaces []string
	// Version is what restore brings back: 0 the replaced memory itself, n
	// the nth earlier text.
	Version int
}

// Origin says what made a change; By is a protocol.MemoryBy* value.
type Origin struct {
	By, Session, Team, Task, Model string
	Turn                           int
}

// ChatNote is the summary memory keeps of one conversation, with its
// quantized vector (rank.Quantize) for search.
type ChatNote struct {
	Session string
	Kind    protocol.SessionKind
	Title   string
	Summary string
	Updated time.Time
	Vector  []byte
}

// RefusedError is a write memory would not make: a secret, a sensitive
// topic or an instruction. Reason is a protocol.MemoryRefused* value.
type RefusedError struct {
	Reason string
	Text   string
	Detail string
}

func (e *RefusedError) Error() string { return e.Detail }

// maxVersions is how many earlier texts a memory keeps.
const maxVersions = 3

// fact is a memory as the ledger keeps it. Text is the text last written to
// or read from the file, so a hand edit is seen as one.
type fact struct {
	ID         string                   `json:"id"`
	Scope      string                   `json:"scope"`
	Text       string                   `json:"text"`
	Status     string                   `json:"status"`
	Kind       string                   `json:"kind,omitempty"`
	Pinned     bool                     `json:"pinned,omitempty"`
	Sensitive  bool                     `json:"sensitive,omitempty"`
	Explicit   bool                     `json:"explicit,omitempty"`
	Source     protocol.MemorySource    `json:"source"`
	Created    time.Time                `json:"created"`
	Updated    time.Time                `json:"updated"`
	Uses       int                      `json:"uses,omitempty"`
	LastUsed   *time.Time               `json:"lastUsed,omitempty"`
	LastUsedIn string                   `json:"lastUsedIn,omitempty"`
	Versions   []protocol.MemoryVersion `json:"versions,omitempty"`
	Replaced   *time.Time               `json:"replaced,omitempty"`
	ReplacedBy string                   `json:"replacedBy,omitempty"`
	Blocked    string                   `json:"blocked,omitempty"`
	Vector     []byte                   `json:"vector,omitempty"`
}

// clone copies f deeply enough that a change record never shares state.
func (f *fact) clone() *fact {
	c := *f
	c.Versions = append([]protocol.MemoryVersion(nil), f.Versions...)
	return &c
}

// inFile reports whether the fact has a line in its scope's file.
func (f *fact) inFile() bool {
	return f.Status == protocol.MemoryActive || f.Status == protocol.MemoryBlocked || f.Status == protocol.MemoryReplaced
}

// StoreConfig places a built-in store: Dir holds you.md and projects/,
// TeamDir the team files, DB the ledger. Names and Embed may be nil.
type StoreConfig struct {
	Dir     string
	TeamDir string
	DB      string
	Names   Names
	Embed   func() (Embedder, func(), error)
}

// fileStamp is what a memory file looked like when the store last read or
// wrote it.
type fileStamp struct {
	Mod  int64 `json:"mod"`
	Size int64 `json:"size"`
}

// Store is the built-in memory: Markdown files for the facts and a bbolt
// ledger for everything about them.
type Store struct {
	mu     sync.Mutex
	cfg    StoreConfig
	db     *bbolt.DB
	facts  map[string]*fact
	stamps map[string]fileStamp
	now    func() time.Time

	embedMu  sync.Mutex
	embedder Embedder
	release  func()
	// words are vectors of single content words, for matching a question's
	// words against a fact's; dropped with the model.
	words map[string][]float32

	// fileChanges are the changes found in hand-edited files, waiting for
	// the engine to take them.
	fileChanges []protocol.MemoryChange
	// pending are scopes whose file is behind the ledger after a failed
	// write; they are written again before any file is read.
	pending map[string]bool
	// syncing is set while a sync runs, so its own writes do not start
	// another.
	syncing bool
	// inflight are the memories a change is writing to its files right now.
	inflight map[string]bool
}

// OpenStore opens the ledger, creating it, and loads every fact. The files
// are read by the first Sync.
func OpenStore(cfg StoreConfig) (*Store, error) {
	db, err := openLedger(cfg.DB)
	if err != nil {
		return nil, err
	}
	s := &Store{cfg: cfg, db: db, facts: map[string]*fact{}, stamps: map[string]fileStamp{}, pending: map[string]bool{}, now: time.Now}
	err = db.View(func(tx *bbolt.Tx) error {
		if err := tx.Bucket(bucketFacts).ForEach(func(_, v []byte) error {
			f := &fact{}
			if err := json.Unmarshal(v, f); err != nil {
				return err
			}
			s.facts[f.ID] = f
			return nil
		}); err != nil {
			return err
		}
		if err := tx.Bucket(bucketFiles).ForEach(func(k, v []byte) error {
			var stamp fileStamp
			if err := json.Unmarshal(v, &stamp); err != nil {
				return err
			}
			s.stamps[string(k)] = stamp
			return nil
		}); err != nil {
			return err
		}
		return tx.Bucket(bucketPending).ForEach(func(k, _ []byte) error {
			s.pending[string(k)] = true
			return nil
		})
	})
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

// Close drops the vectors model and closes the ledger.
func (s *Store) Close() error {
	s.Release()
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.db.Close()
}

// newID is a fresh memory id: m and 7 hex characters.
func (s *Store) newID() string {
	for {
		var b [4]byte
		_, _ = rand.Read(b[:])
		id := "m" + hex.EncodeToString(b[:])[:7]
		if s.facts[id] == nil {
			return id
		}
	}
}

// cleanText makes a memory one line of at most protocol.MemoryTextMax
// characters.
func cleanText(text string) string {
	text = strings.Join(strings.Fields(text), " ")
	if utf8.RuneCountInString(text) > protocol.MemoryTextMax {
		runes := []rune(text)
		text = strings.TrimSpace(string(runes[:protocol.MemoryTextMax]))
	}
	return text
}

// versionBy is who an earlier text's change came from, in MemoryVersion's
// words.
func versionBy(by string) string {
	switch by {
	case protocol.MemoryByLearning, protocol.MemoryByFile:
		return by
	}
	return protocol.MemoryByYou
}

// sourceKind is where a memory comes from when an op does not say.
func sourceKind(by string) string {
	switch by {
	case protocol.MemoryByFile:
		return protocol.MemoryFromFile
	case protocol.MemoryByImport:
		return protocol.MemoryFromImport
	case protocol.MemoryByTeam:
		return protocol.MemoryFromTeam
	case protocol.MemoryByLearning:
		return protocol.MemoryFromAsk
	}
	return protocol.MemoryFromYou
}

// Apply makes ops one change, written to the ledger and the files together.
// An add that repeats a memory of its scope word for word is dropped; a
// change that does nothing has no items and is not recorded.
func (s *Store) Apply(o Origin, ops []Op) (protocol.MemoryChange, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.syncLocked()
	t := s.begin(o)
	for _, op := range ops {
		if err := t.apply(op); err != nil {
			t.rollback()
			return protocol.MemoryChange{}, err
		}
	}
	return t.commit()
}

// errNotFound is a memory id the store does not hold.
func errNotFound(id string) error {
	return protocol.Errorf(protocol.CodeNotFound, "%s", i18n.Tf("No memory %s.", id))
}

// apply runs one op inside a change.
func (t *txn) apply(op Op) error {
	s := t.s
	switch op.Kind {
	case OpAdd, OpSuggest, OpReplace:
		replaces := op.Replaces
		scope := op.Scope
		if op.Kind == OpReplace {
			old := s.facts[op.ID]
			if old == nil {
				return errNotFound(op.ID)
			}
			replaces = append([]string{op.ID}, replaces...)
			if scope == "" {
				scope = old.Scope
			}
		}
		return t.add(op, scope, replaces)
	}
	f := s.facts[op.ID]
	if f == nil && (op.Kind == OpForget || op.Kind == OpDismiss) {
		// a memory already gone is already forgotten, so a stale row or a second click is not an error
		return nil
	}
	if f == nil {
		return errNotFound(op.ID)
	}
	switch op.Kind {
	case OpForget:
		t.forget(f, protocol.MemoryOpForgot, true)
	case OpDismiss:
		if f.Status != protocol.MemorySuggested {
			return nil
		}
		t.forget(f, protocol.MemoryOpDismissed, true)
	case OpKeep:
		if f.Status != protocol.MemorySuggested {
			return nil
		}
		t.touch(f.ID)
		f.Status, f.Updated = protocol.MemoryActive, t.now
		t.learned++
		t.item(f, protocol.MemoryOpKept, "")
	case OpEdit:
		return t.edit(f, op)
	case OpRestore:
		return t.restore(f, op.Version)
	case OpPin, OpUnpin:
		pinned := op.Kind == OpPin
		if f.Pinned == pinned {
			return nil
		}
		t.touch(f.ID)
		f.Pinned = pinned
		kind := protocol.MemoryOpPinned
		if !pinned {
			kind = protocol.MemoryOpUnpinned
		}
		t.item(f, kind, "")
	default:
		return fmt.Errorf("memory: unknown op %q", op.Kind)
	}
	return nil
}

// add creates a memory, active or suggested, superseding replaces.
func (t *txn) add(op Op, scope string, replaces []string) error {
	s := t.s
	text := cleanText(op.Text)
	if text == "" {
		return protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("A memory needs some text."))
	}
	if !validScope(scope) {
		return protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.Tf("Memory can't keep anything in %s.", scope))
	}
	if th, hit := scan.Strict(text); hit {
		return &RefusedError{Reason: threatRefusal(th), Text: text, Detail: threatReason(th)}
	}
	hash := hashNormalized(text)
	for _, other := range s.facts {
		if other.Scope == scope && (other.Status == protocol.MemoryActive || other.Status == protocol.MemorySuggested) && hashNormalized(other.Text) == hash {
			return nil
		}
	}
	status := protocol.MemoryActive
	itemOp := protocol.MemoryOpRemembered
	if op.Kind == OpSuggest {
		status, itemOp = protocol.MemorySuggested, protocol.MemoryOpSuggested
	}
	source := op.Source
	if source.Kind == "" {
		source.Kind = sourceKind(t.origin.By)
	}
	if source.Session == "" {
		source.Session, source.Turn = t.origin.Session, t.origin.Turn
	}
	if source.Team == "" {
		source.Team, source.Task = t.origin.Team, t.origin.Task
	}
	if source.Model == "" {
		source.Model = t.origin.Model
	}
	retain := op.Retain
	if retain != protocol.MemoryPermanent && retain != protocol.MemoryEphemeral {
		retain = protocol.MemoryDurable
	}
	f := &fact{ID: s.newID(), Scope: scope, Text: text, Status: status, Kind: retain, Pinned: op.Pinned, Sensitive: op.Sensitive,
		Explicit: op.Explicit || source.Kind == protocol.MemoryFromYou, Source: source, Created: t.now, Updated: t.now}
	t.touch(f.ID)
	s.facts[f.ID] = f
	replacedText := ""
	for _, id := range replaces {
		old := s.facts[id]
		if old == nil || old.Status == protocol.MemoryReplaced || old.ID == f.ID {
			continue
		}
		t.touch(old.ID)
		at := t.now
		if replacedText == "" {
			replacedText = old.Text
		}
		if old.Status == protocol.MemorySuggested {
			delete(s.facts, old.ID)
			continue
		}
		old.Status, old.Replaced, old.ReplacedBy, old.Updated = protocol.MemoryReplaced, &at, f.ID, t.now
	}
	if status == protocol.MemoryActive {
		t.learned++
	}
	t.item(f, itemOp, replacedText)
	return nil
}

// edit changes a memory's text or scope, keeping the old text as a version.
func (t *txn) edit(f *fact, op Op) error {
	s := t.s
	text := cleanText(op.Text)
	scope := op.Scope
	if scope == "" {
		scope = f.Scope
	}
	if text == "" {
		text = f.Text
	}
	if text == f.Text && scope == f.Scope {
		return nil
	}
	if !validScope(scope) {
		return protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.Tf("Memory can't keep anything in %s.", scope))
	}
	if th, hit := scan.Strict(text); hit {
		return &RefusedError{Reason: threatRefusal(th), Text: text, Detail: threatReason(th)}
	}
	t.touch(f.ID)
	if text != f.Text {
		f.Versions = pushVersion(f.Versions, protocol.MemoryVersion{Text: f.Text, At: f.Updated, By: versionBy(t.origin.By)})
		f.Text, f.Vector = text, nil
		if f.Status == protocol.MemoryBlocked {
			f.Status, f.Blocked = protocol.MemoryActive, ""
		}
	}
	if scope != f.Scope {
		t.dirty[f.Scope] = true
		f.Scope = scope
	}
	f.Updated = t.now
	s.facts[f.ID] = f
	t.item(f, protocol.MemoryOpEdited, "")
	return nil
}

// restore brings back a replaced memory (version 0) or an earlier text. Text
// that reads as an instruction or a secret is never brought back into use.
func (t *txn) restore(f *fact, version int) error {
	s := t.s
	if version == 0 {
		if f.Status != protocol.MemoryReplaced {
			return nil
		}
		if th, hit := scan.Strict(f.Text); hit {
			return &RefusedError{Reason: threatRefusal(th), Text: f.Text, Detail: threatReason(th)}
		}
		t.touch(f.ID)
		f.Status, f.Replaced, f.ReplacedBy, f.Blocked, f.Updated = protocol.MemoryActive, nil, "", "", t.now
		t.item(f, protocol.MemoryOpRestored, "")
		return nil
	}
	if version < 1 || version > len(f.Versions) {
		return protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("That earlier text is gone."))
	}
	earlier := f.Versions[version-1]
	if th, hit := scan.Strict(earlier.Text); hit {
		return &RefusedError{Reason: threatRefusal(th), Text: earlier.Text, Detail: threatReason(th)}
	}
	t.touch(f.ID)
	versions := append([]protocol.MemoryVersion(nil), f.Versions[:version-1]...)
	versions = append(versions, f.Versions[version:]...)
	f.Versions = pushVersion(versions, protocol.MemoryVersion{Text: f.Text, At: f.Updated, By: versionBy(t.origin.By)})
	f.Text, f.Vector, f.Updated = earlier.Text, nil, t.now
	if f.Status == protocol.MemoryBlocked {
		f.Status = protocol.MemoryActive
	}
	f.Blocked = ""
	s.facts[f.ID] = f
	t.item(f, protocol.MemoryOpRestored, earlier.Text)
	return nil
}

// pushVersion puts v first and keeps at most maxVersions.
func pushVersion(versions []protocol.MemoryVersion, v protocol.MemoryVersion) []protocol.MemoryVersion {
	out := append([]protocol.MemoryVersion{v}, versions...)
	if len(out) > maxVersions {
		out = out[:maxVersions]
	}
	return out
}

// forget removes a memory; with tomb, it is never learned again.
func (t *txn) forget(f *fact, itemOp string, tomb bool) {
	t.touch(f.ID)
	t.item(f, itemOp, "")
	if tomb {
		t.tomb(f)
	}
	if f.Status != protocol.MemorySuggested {
		t.forgot++
	}
	delete(t.s.facts, f.ID)
}

// item records what an op did to f.
func (t *txn) item(f *fact, op, replaces string) {
	t.rec.Change.Items = append(t.rec.Change.Items, protocol.MemoryItem{ID: f.ID, Text: f.Text, Scope: f.Scope, ScopeName: t.s.scopeName(f.Scope), Op: op, Replaces: replaces})
}

// Get is one memory with its versions.
func (s *Store) Get(id string) (protocol.Memory, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	f := s.facts[id]
	if f == nil {
		return protocol.Memory{}, false
	}
	return s.memory(f), true
}

// Replaced lists the memories a newer fact replaced in the scopes, newest
// first; no scopes lists every one.
func (s *Store) Replaced(scopes ...string) ([]protocol.Memory, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.collect(func(f *fact) bool { return f.Status == protocol.MemoryReplaced && inScopes(f.Scope, scopes) }), nil
}

// memory is f as the protocol shows it.
func (s *Store) memory(f *fact) protocol.Memory {
	m := protocol.Memory{ID: f.ID, Scope: f.Scope, ScopeName: s.scopeName(f.Scope), Text: f.Text, Status: f.Status, Kind: f.Kind,
		Pinned: f.Pinned, Sensitive: f.Sensitive, Source: f.Source, Created: f.Created, Updated: f.Updated, Uses: f.Uses,
		LastUsed: f.LastUsed, Replaced: f.Replaced, ReplacedBy: f.ReplacedBy, Versions: append([]protocol.MemoryVersion(nil), f.Versions...), Blocked: f.Blocked}
	if f.LastUsedIn != "" {
		m.LastUsedIn = &protocol.MemoryRef{Session: f.LastUsedIn}
	}
	if next := s.facts[f.ReplacedBy]; next != nil {
		m.ReplacedByText = next.Text
	}
	return m
}

// Active lists the memories in use in the scopes, newest first; no scopes
// lists every one. Blocked and suggested memories are not in use.
func (s *Store) Active(scopes ...string) ([]protocol.Memory, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.collect(func(f *fact) bool { return f.Status == protocol.MemoryActive && inScopes(f.Scope, scopes) }), nil
}

// collect lists matching memories, newest first.
func (s *Store) collect(match func(*fact) bool) []protocol.Memory {
	var facts []*fact
	for _, f := range s.facts {
		if match(f) {
			facts = append(facts, f)
		}
	}
	sort.Slice(facts, func(i, j int) bool {
		if !facts[i].Updated.Equal(facts[j].Updated) {
			return facts[i].Updated.After(facts[j].Updated)
		}
		return facts[i].ID < facts[j].ID
	})
	out := make([]protocol.Memory, len(facts))
	for index, f := range facts {
		out[index] = s.memory(f)
	}
	return out
}

// Explicit reports which of ids the user asked to keep.
func (s *Store) Explicit(ids []string) map[string]bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := map[string]bool{}
	for _, id := range ids {
		if f := s.facts[id]; f != nil && f.Explicit {
			out[id] = true
		}
	}
	return out
}

// Used counts that an answer in session drew on ids.
func (s *Store) Used(session string, ids []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now()
	return s.db.Update(func(tx *bbolt.Tx) error {
		for _, id := range ids {
			f := s.facts[id]
			if f == nil {
				continue
			}
			f.Uses++
			f.LastUsed, f.LastUsedIn = &now, session
			if err := putJSON(tx, bucketFacts, f.ID, f); err != nil {
				return err
			}
		}
		return nil
	})
}

// Suppress stops a memory being used in one conversation.
func (s *Store) Suppress(session, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.facts[id] == nil {
		return errNotFound(id)
	}
	return s.db.Update(func(tx *bbolt.Tx) error {
		return tx.Bucket(bucketSuppressed).Put([]byte(session+"\x00"+id), []byte{1})
	})
}

// Suppressed are the memories a conversation must not use.
func (s *Store) Suppressed(session string) map[string]bool {
	out := map[string]bool{}
	prefix := []byte(session + "\x00")
	_ = s.db.View(func(tx *bbolt.Tx) error {
		c := tx.Bucket(bucketSuppressed).Cursor()
		for k, _ := c.Seek(prefix); k != nil && strings.HasPrefix(string(k), string(prefix)); k, _ = c.Next() {
			out[string(k[len(prefix):])] = true
		}
		return nil
	})
	return out
}

// Watermark is the last event seq learned from in session, 0 for none.
func (s *Store) Watermark(session string) int64 {
	var seq int64
	_ = s.db.View(func(tx *bbolt.Tx) error {
		_, err := getJSON(tx, bucketWatermarks, session, &seq)
		return err
	})
	return seq
}

// SetWatermark records that session was learned from up to seq.
func (s *Store) SetWatermark(session string, seq int64) error {
	return s.db.Update(func(tx *bbolt.Tx) error { return putJSON(tx, bucketWatermarks, session, seq) })
}

// PutNote keeps the summary of a conversation.
func (s *Store) PutNote(n ChatNote) error {
	return s.db.Update(func(tx *bbolt.Tx) error { return putJSON(tx, bucketNotes, n.Session, n) })
}

// Notes are every conversation summary, newest first.
func (s *Store) Notes() ([]ChatNote, error) {
	var out []ChatNote
	err := s.db.View(func(tx *bbolt.Tx) error {
		return tx.Bucket(bucketNotes).ForEach(func(_, v []byte) error {
			var n ChatNote
			if err := json.Unmarshal(v, &n); err != nil {
				return err
			}
			out = append(out, n)
			return nil
		})
	})
	sort.SliceStable(out, func(i, j int) bool { return out[i].Updated.After(out[j].Updated) })
	return out, err
}

// DeleteNote drops a conversation's summary.
func (s *Store) DeleteNote(session string) error {
	return s.db.Update(func(tx *bbolt.Tx) error { return tx.Bucket(bucketNotes).Delete([]byte(session)) })
}

// SessionDeleted drops what the ledger keeps of a deleted conversation: its
// summary, watermark and suppressions. Memories learned from it stay.
func (s *Store) SessionDeleted(session string) error {
	prefix := session + "\x00"
	return s.db.Update(func(tx *bbolt.Tx) error {
		if err := tx.Bucket(bucketNotes).Delete([]byte(session)); err != nil {
			return err
		}
		if err := tx.Bucket(bucketWatermarks).Delete([]byte(session)); err != nil {
			return err
		}
		c := tx.Bucket(bucketSuppressed).Cursor()
		for k, _ := c.Seek([]byte(prefix)); k != nil && strings.HasPrefix(string(k), prefix); k, _ = c.Seek([]byte(prefix)) {
			if err := c.Delete(); err != nil {
				return err
			}
		}
		return nil
	})
}

// Card is the card about the user as last built.
func (s *Store) Card() (protocol.MemoryCard, error) {
	var card protocol.MemoryCard
	err := s.db.View(func(tx *bbolt.Tx) error {
		_, err := getJSON(tx, bucketCard, string(cardKey), &card)
		return err
	})
	card.Limit = protocol.MemoryCardMax
	return card, err
}

// SetCard keeps a newly built card.
func (s *Store) SetCard(c protocol.MemoryCard) error {
	return s.db.Update(func(tx *bbolt.Tx) error { return putJSON(tx, bucketCard, string(cardKey), c) })
}

// nameScope remembers a project's name for its file title and for lists. It
// writes only when the name is new, so a recall costs no ledger write.
func (s *Store) nameScope(scope, name string) {
	if name == "" || !strings.HasPrefix(scope, protocol.MemoryProjectPrefix) || s.scopeName(scope) == name {
		return
	}
	_ = s.db.Update(func(tx *bbolt.Tx) error { return tx.Bucket(bucketScopes).Put([]byte(scope), []byte(name)) })
}

// Remove drops memories now kept by an external engine: no tombstone, no
// change to undo, and their lines leave the files.
func (s *Store) Remove(ids []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	gone := map[string]*fact{}
	dirty := map[string]bool{}
	for _, id := range ids {
		if f := s.facts[id]; f != nil {
			gone[id] = nil
			dirty[f.Scope] = true
		}
	}
	if len(gone) == 0 {
		return nil
	}
	err := s.db.Update(func(tx *bbolt.Tx) error {
		if err := s.putFacts(tx, gone); err != nil {
			return err
		}
		return markPending(tx, dirty)
	})
	if err != nil {
		return err
	}
	for id := range gone {
		delete(s.facts, id)
	}
	return s.writeChange(dirty, gone)
}

// scopeName says a scope in words, from the ledger for a project.
func (s *Store) scopeName(scope string) string {
	if strings.HasPrefix(scope, protocol.MemoryProjectPrefix) {
		var name string
		_ = s.db.View(func(tx *bbolt.Tx) error {
			name = string(tx.Bucket(bucketScopes).Get([]byte(scope)))
			return nil
		})
		if name != "" {
			return name
		}
	}
	return defaultScopeName(scope, s.cfg.Names)
}

// scopePath is the file holding a scope's memories: you.md, a project's
// file named for its key, or a team's.
func (s *Store) scopePath(scope string) string {
	switch {
	case scope == protocol.MemoryYou:
		return filepath.Join(s.cfg.Dir, "you.md")
	case strings.HasPrefix(scope, protocol.MemoryTeamPrefix):
		return filepath.Join(s.cfg.TeamDir, strings.TrimPrefix(scope, protocol.MemoryTeamPrefix)+".md")
	}
	key := strings.TrimPrefix(scope, protocol.MemoryProjectPrefix)
	dir := filepath.Join(s.cfg.Dir, "projects")
	if matches, _ := filepath.Glob(filepath.Join(dir, "*"+key+".md")); len(matches) > 0 {
		sort.Strings(matches)
		return matches[0]
	}
	name := slug(s.scopeName(scope))
	if name == "" || name == key {
		return filepath.Join(dir, key+".md")
	}
	return filepath.Join(dir, name+"-"+key+".md")
}

// pathScope is the scope a memory file holds, "" for a file that is not one.
func (s *Store) pathScope(path string) string {
	base := strings.TrimSuffix(filepath.Base(path), ".md")
	switch filepath.Dir(path) {
	case filepath.Clean(s.cfg.Dir):
		if base == "you" {
			return protocol.MemoryYou
		}
	case filepath.Join(s.cfg.Dir, "projects"):
		if len(base) >= 12 && projectKey.MatchString(base[len(base)-12:]) {
			return protocol.MemoryProjectPrefix + base[len(base)-12:]
		}
	case filepath.Clean(s.cfg.TeamDir):
		if scope := protocol.MemoryTeamPrefix + base; validScope(scope) {
			return scope
		}
	}
	return ""
}

// stat is a file's stamp, ok false when it is missing.
func stat(path string) (fileStamp, bool) {
	info, err := os.Stat(path)
	if err != nil {
		return fileStamp{}, false
	}
	return fileStamp{Mod: info.ModTime().UnixNano(), Size: info.Size()}, true
}

// errRefused unwraps a RefusedError.
func errRefused(err error) (*RefusedError, bool) {
	var refused *RefusedError
	ok := errors.As(err, &refused)
	return refused, ok
}

// Drop removes suggestions an external engine now keeps, without a
// tombstone or a change of their own.
func (s *Store) Drop(ids []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.db.Update(func(tx *bbolt.Tx) error {
		for _, id := range ids {
			if f := s.facts[id]; f == nil || f.Status != protocol.MemorySuggested {
				continue
			}
			delete(s.facts, id)
			if err := tx.Bucket(bucketFacts).Delete([]byte(id)); err != nil {
				return err
			}
		}
		return nil
	})
}

// putOffForget records that an external engine still has to forget scope,
// for when it next runs.
func (s *Store) putOffForget(scope string) error {
	return s.db.Update(func(tx *bbolt.Tx) error { return tx.Bucket(bucketForgets).Put([]byte(scope), []byte{1}) })
}

// pendingForgets are the scopes an external engine still has to forget.
func (s *Store) pendingForgets() []string {
	var out []string
	_ = s.db.View(func(tx *bbolt.Tx) error {
		return tx.Bucket(bucketForgets).ForEach(func(k, _ []byte) error {
			out = append(out, string(k))
			return nil
		})
	})
	return out
}

// clearPendingForget records that an external engine forgot scope.
func (s *Store) clearPendingForget(scope string) error {
	return s.db.Update(func(tx *bbolt.Tx) error { return tx.Bucket(bucketForgets).Delete([]byte(scope)) })
}
