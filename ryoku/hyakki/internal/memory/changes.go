package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"time"

	"go.etcd.io/bbolt"

	i18n "ryoku-i18n"
	"ryoku-rashin/internal/protocol"
	"ryoku-rashin/internal/rank"
)

// maxChanges is how many changes the ledger keeps for undo; the oldest go
// first.
const maxChanges = 2000

// changeRecord is a change with what undo and redo need: every memory it
// touched before and after (nil when absent), the tombstones it left and
// what it added to the day's counts.
type changeRecord struct {
	Change  protocol.MemoryChange `json:"change"`
	Before  map[string]*fact      `json:"before"`
	After   map[string]*fact      `json:"after"`
	Tombs   []tombstone           `json:"tombs,omitempty"`
	Day     string                `json:"day"`
	Learned int                   `json:"learned,omitempty"`
	Forgot  int                   `json:"forgot,omitempty"`
}

// dayCount is one day's learned and forgotten counts.
type dayCount struct {
	Learned int `json:"learned"`
	Forgot  int `json:"forgot"`
}

// txn gathers one change: the memories it touches, the files to write and
// the counts to add.
type txn struct {
	s       *Store
	origin  Origin
	now     time.Time
	rec     changeRecord
	dirty   map[string]bool
	learned int
	forgot  int
}

// begin starts a change made by o.
func (s *Store) begin(o Origin) *txn {
	now := s.now()
	return &txn{s: s, origin: o, now: now, dirty: map[string]bool{}, rec: changeRecord{
		Change: protocol.MemoryChange{At: now, Origin: o.By, Session: o.Session, Turn: o.Turn, Team: o.Team, Task: o.Task, Model: o.Model},
		Before: map[string]*fact{},
		After:  map[string]*fact{},
	}}
}

// touch keeps the first state of a memory the change touches.
func (t *txn) touch(id string) {
	if _, ok := t.rec.Before[id]; ok {
		return
	}
	var before *fact
	if f := t.s.facts[id]; f != nil {
		before = f.clone()
		t.dirty[f.Scope] = true
	}
	t.rec.Before[id] = before
}

// tomb keeps a forgotten memory from being learned again, reworded or not:
// a fact never searched has no vector yet, so it gets one now.
func (t *txn) tomb(f *fact) {
	vector := f.Vector
	if len(vector) == 0 {
		if vectors, err := t.s.embedFacts(context.Background(), []string{f.Text}); err == nil {
			vector = rank.Quantize(vectors[0])
		}
	}
	t.rec.Tombs = append(t.rec.Tombs, newTombstone(f.Scope, f.Text, vector, t.now))
}

// rollback undoes a change that failed before it was written.
func (t *txn) rollback() {
	for id, before := range t.rec.Before {
		if before == nil {
			delete(t.s.facts, id)
		} else {
			t.s.facts[id] = before
		}
	}
}

// commit writes the change: ledger first in one transaction, then the files
// of every scope it touched. Touches with no item to show, such as a hand
// edit that is blocked, are written without recording a change.
func (t *txn) commit() (protocol.MemoryChange, error) {
	s := t.s
	if len(t.rec.Before) == 0 {
		return t.rec.Change, nil
	}
	record := len(t.rec.Change.Items) > 0
	for id := range t.rec.Before {
		if f := s.facts[id]; f != nil {
			t.rec.After[id] = f.clone()
			t.dirty[f.Scope] = true
		} else {
			t.rec.After[id] = nil
		}
	}
	t.rec.Change.ID = fmt.Sprintf("c%016x", t.now.UnixNano())
	t.rec.Day = t.now.Format(time.DateOnly)
	t.rec.Learned, t.rec.Forgot = t.learned, t.forgot
	err := s.db.Update(func(tx *bbolt.Tx) error {
		if err := s.putFacts(tx, t.rec.After); err != nil {
			return err
		}
		if err := markPending(tx, t.dirty); err != nil {
			return err
		}
		if !record {
			return nil
		}
		for _, tomb := range t.rec.Tombs {
			if err := putJSON(tx, bucketTombs, tomb.key(), tomb); err != nil {
				return err
			}
		}
		if err := addDay(tx, t.rec.Day, t.learned, t.forgot); err != nil {
			return err
		}
		for tx.Bucket(bucketChanges).Get([]byte(t.rec.Change.ID)) != nil {
			t.now = t.now.Add(time.Nanosecond)
			t.rec.Change.ID = fmt.Sprintf("c%016x", t.now.UnixNano())
		}
		if err := putJSON(tx, bucketChanges, t.rec.Change.ID, t.rec); err != nil {
			return err
		}
		return pruneChanges(tx)
	})
	if err != nil {
		t.rollback()
		return protocol.MemoryChange{}, err
	}
	return t.rec.Change, s.writeChange(t.dirty, t.rec.After)
}

// putFacts writes each memory, deleting the nil ones.
func (s *Store) putFacts(tx *bbolt.Tx, facts map[string]*fact) error {
	for id, f := range facts {
		if f == nil {
			if err := tx.Bucket(bucketFacts).Delete([]byte(id)); err != nil {
				return err
			}
			continue
		}
		if err := putJSON(tx, bucketFacts, id, f); err != nil {
			return err
		}
	}
	return nil
}

// addDay adds to one day's counts.
func addDay(tx *bbolt.Tx, day string, learned, forgot int) error {
	if learned == 0 && forgot == 0 {
		return nil
	}
	var count dayCount
	if _, err := getJSON(tx, bucketDays, day, &count); err != nil {
		return err
	}
	count.Learned = max(0, count.Learned+learned)
	count.Forgot = max(0, count.Forgot+forgot)
	return putJSON(tx, bucketDays, day, count)
}

// pruneChanges drops the oldest changes past maxChanges.
func pruneChanges(tx *bbolt.Tx) error {
	bucket := tx.Bucket(bucketChanges)
	extra := bucket.Stats().KeyN - maxChanges
	c := bucket.Cursor()
	for k, _ := c.First(); k != nil && extra > 0; k, _ = c.First() {
		if err := c.Delete(); err != nil {
			return err
		}
		extra--
	}
	return nil
}

// Undo reverses a change whole. It is refused when a later change has
// touched one of its memories since. Undoing a memory that was remembered
// leaves a tombstone, so it is not learned again.
func (s *Store) Undo(id string) (protocol.MemoryChange, error) {
	return s.flip(id, true)
}

// Redo does an undone change again.
func (s *Store) Redo(id string) (protocol.MemoryChange, error) {
	return s.flip(id, false)
}

// flip moves every memory of a change from one side of it to the other.
func (s *Store) flip(id string, undo bool) (protocol.MemoryChange, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.syncLocked()
	var rec changeRecord
	err := s.db.View(func(tx *bbolt.Tx) error {
		found, err := getJSON(tx, bucketChanges, id, &rec)
		if err == nil && !found {
			err = protocol.Errorf(protocol.CodeNotFound, "%s", i18n.T("That change is no longer kept."))
		}
		return err
	})
	if err != nil {
		return protocol.MemoryChange{}, err
	}
	if rec.Change.Undone == !undo {
		from, to := rec.After, rec.Before
		if !undo {
			from, to = rec.Before, rec.After
		}
		for memory, want := range from {
			if !sameState(s.facts[memory], want) {
				return protocol.MemoryChange{}, protocol.Errorf(protocol.CodeConflict, "%s", i18n.T("A later change touched this memory. Undo that one first."))
			}
		}
		dirty := map[string]bool{}
		next := map[string]*fact{}
		for memory, state := range to {
			if current := s.facts[memory]; current != nil {
				dirty[current.Scope] = true
			}
			if state == nil {
				next[memory] = nil
				continue
			}
			f := state.clone()
			if current := s.facts[memory]; current != nil {
				f.Uses, f.LastUsed, f.LastUsedIn = current.Uses, current.LastUsed, current.LastUsedIn
				// a vector belongs to its text; the state's own one stays when the text goes back
				if current.Text == f.Text {
					f.Vector = current.Vector
				}
			}
			next[memory] = f
			dirty[f.Scope] = true
		}
		// undo lifts the change's own tombstones and leaves one per memory it remembered; redo swaps them back
		lift, leave := rec.Tombs, undoTombs(rec, s)
		if !undo {
			lift, leave = leave, rec.Tombs
		}
		sign := 1
		if undo {
			sign = -1
		}
		rec.Change.Undone = undo
		err = s.db.Update(func(tx *bbolt.Tx) error {
			if err := s.putFacts(tx, next); err != nil {
				return err
			}
			if err := markPending(tx, dirty); err != nil {
				return err
			}
			for _, tomb := range lift {
				if err := tx.Bucket(bucketTombs).Delete([]byte(tomb.key())); err != nil {
					return err
				}
			}
			for _, tomb := range leave {
				if err := putJSON(tx, bucketTombs, tomb.key(), tomb); err != nil {
					return err
				}
			}
			if err := addDay(tx, rec.Day, sign*rec.Learned, sign*rec.Forgot); err != nil {
				return err
			}
			return putJSON(tx, bucketChanges, rec.Change.ID, rec)
		})
		if err != nil {
			return protocol.MemoryChange{}, err
		}
		for memory, f := range next {
			if f == nil {
				delete(s.facts, memory)
			} else {
				s.facts[memory] = f
			}
		}
		if err := s.writeChange(dirty, next); err != nil {
			return rec.Change, err
		}
	}
	return rec.Change, nil
}

// undoTombs are the tombstones undo leaves: one for each memory the change
// remembered, so the same fact is not learned again.
func undoTombs(rec changeRecord, s *Store) []tombstone {
	var out []tombstone
	for id, before := range rec.Before {
		after := rec.After[id]
		if before != nil || after == nil || after.Status != protocol.MemoryActive && after.Status != protocol.MemorySuggested {
			continue
		}
		vector := after.Vector
		if current := s.facts[id]; current != nil && current.Vector != nil && current.Text == after.Text {
			vector = current.Vector
		}
		out = append(out, newTombstone(after.Scope, after.Text, vector, s.now()))
	}
	return out
}

// sameState reports whether a memory is still as a change left it, by what
// a person sees: text, scope, state and pin.
func sameState(current, want *fact) bool {
	if current == nil || want == nil {
		return current == nil && want == nil
	}
	return current.Text == want.Text && current.Scope == want.Scope && current.Status == want.Status && current.Pinned == want.Pinned
}

// Changes are the kept changes among ids, each with whether it was undone.
func (s *Store) Changes(ids []string) ([]protocol.MemoryChange, error) {
	var out []protocol.MemoryChange
	err := s.db.View(func(tx *bbolt.Tx) error {
		for _, id := range ids {
			var rec changeRecord
			found, err := getJSON(tx, bucketChanges, id, &rec)
			if err != nil {
				return err
			}
			if found {
				out = append(out, rec.Change)
			}
		}
		return nil
	})
	return out, err
}

// ForgetAll forgets every memory in scope, or everything when scope is "",
// with their earlier versions, suggestions and every change that still holds
// their text. It cannot be undone and leaves no tombstones: the user starts
// over. Forgetting what is known about the user drops the card; forgetting
// everything also drops the conversation summaries, which are memory too.
func (s *Store) ForgetAll(scope string) (protocol.MemoryChange, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.syncLocked()
	now := s.now()
	change := protocol.MemoryChange{At: now, Origin: protocol.MemoryByYou}
	gone := map[string]*fact{}
	dirty := map[string]bool{}
	forgot := 0
	for id, f := range s.facts {
		if !inScope(f.Scope, scope) {
			continue
		}
		gone[id] = nil
		dirty[f.Scope] = true
		if f.Status != protocol.MemorySuggested {
			forgot++
		}
		change.Items = append(change.Items, protocol.MemoryItem{ID: f.ID, Text: f.Text, Scope: f.Scope, ScopeName: s.scopeName(f.Scope), Op: protocol.MemoryOpForgot})
	}
	sort.Slice(change.Items, func(i, j int) bool { return change.Items[i].ID < change.Items[j].ID })
	err := s.db.Update(func(tx *bbolt.Tx) error {
		if err := s.putFacts(tx, gone); err != nil {
			return err
		}
		if err := markPending(tx, dirty); err != nil {
			return err
		}
		if err := addDay(tx, now.Format(time.DateOnly), 0, forgot); err != nil {
			return err
		}
		if err := dropChanges(tx, scope); err != nil {
			return err
		}
		if inScope(protocol.MemoryYou, scope) {
			if err := tx.Bucket(bucketCard).Delete(cardKey); err != nil {
				return err
			}
		}
		if scope == "" {
			return emptyBucket(tx, bucketNotes)
		}
		return nil
	})
	if err != nil {
		return protocol.MemoryChange{}, err
	}
	for id := range gone {
		delete(s.facts, id)
	}
	return change, s.writeChange(dirty, gone)
}

// emptyBucket deletes every key of a bucket.
func emptyBucket(tx *bbolt.Tx, name []byte) error {
	if err := tx.DeleteBucket(name); err != nil {
		return err
	}
	_, err := tx.CreateBucket(name)
	return err
}

// dropChanges deletes every change that touched a memory in scope.
func dropChanges(tx *bbolt.Tx, scope string) error {
	bucket := tx.Bucket(bucketChanges)
	var drop [][]byte
	err := bucket.ForEach(func(k, v []byte) error {
		var rec changeRecord
		if err := json.Unmarshal(v, &rec); err != nil {
			return err
		}
		for _, item := range rec.Change.Items {
			if inScope(item.Scope, scope) {
				drop = append(drop, append([]byte(nil), k...))
				return nil
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	for _, k := range drop {
		if err := bucket.Delete(k); err != nil {
			return err
		}
	}
	return nil
}

// markPending records, in the transaction that changes the ledger, that the
// files of these scopes are about to be written; a crash before the write
// leaves the mark for the next open.
func markPending(tx *bbolt.Tx, dirty map[string]bool) error {
	for scope := range dirty {
		if err := tx.Bucket(bucketPending).Put([]byte(scope), []byte{1}); err != nil {
			return err
		}
	}
	return nil
}

// writeChange writes the files of a change. ids are the memories it wrote,
// which a sync started meanwhile must not take for hand edits of the file.
func (s *Store) writeChange(dirty map[string]bool, ids map[string]*fact) error {
	previous := s.inflight
	s.inflight = map[string]bool{}
	for id := range previous {
		s.inflight[id] = true
	}
	for id := range ids {
		s.inflight[id] = true
	}
	defer func() { s.inflight = previous }()
	return s.writeScopes(dirty)
}

// writeScopes rewrites the file of every dirty scope from the ledger: lines
// of memories gone are dropped, the rest set in place, new ones appended.
// A scope with nothing left and no file is not created. A file edited by
// hand since it was last read is read first. A scope whose write fails stays
// pending and is written again before the next read of any file.
func (s *Store) writeScopes(dirty map[string]bool) error {
	var firstErr error
	for scope := range dirty {
		if !s.syncing && s.moved(s.scopePath(scope)) {
			if err := s.syncLocked(); err != nil && firstErr == nil {
				firstErr = err
			}
		}
		err := s.writeScope(scope, false)
		if err == nil {
			err = s.clearPending(scope)
		}
		if err != nil {
			s.pending[scope] = true
			if firstErr == nil {
				firstErr = err
			}
		}
	}
	return firstErr
}

// moved reports a file that changed since the store last read or wrote it.
func (s *Store) moved(path string) bool {
	stamp, exists := stat(path)
	known, ok := s.stamps[path]
	return exists != ok || exists && stamp != known
}

// clearPending forgets that a scope's file is behind.
func (s *Store) clearPending(scope string) error {
	delete(s.pending, scope)
	return s.db.Update(func(tx *bbolt.Tx) error { return tx.Bucket(bucketPending).Delete([]byte(scope)) })
}

// writePending brings the file of every pending scope up to the ledger, and
// has it read again, so lines added by hand meanwhile become memories.
func (s *Store) writePending() error {
	for scope := range s.pending {
		if err := s.writeScope(scope, true); err != nil {
			return err
		}
		if err := s.clearPending(scope); err != nil {
			return err
		}
		delete(s.stamps, s.scopePath(scope))
	}
	return nil
}

// writeScope rewrites one scope's file. loose keeps the lines without an id,
// which a pending rewrite leaves for the next read.
func (s *Store) writeScope(scope string, loose bool) error {
	path := s.scopePath(scope)
	var facts []*fact
	want := map[string]bool{}
	for _, f := range s.facts {
		if f.Scope == scope && f.inFile() {
			facts = append(facts, f)
			want[f.ID] = true
		}
	}
	sort.Slice(facts, func(i, j int) bool {
		if !facts[i].Created.Equal(facts[j].Created) {
			return facts[i].Created.Before(facts[j].Created)
		}
		return facts[i].ID < facts[j].ID
	})
	data, err := os.ReadFile(path)
	var file *mdFile
	switch {
	case err == nil:
		file = parseMarkdown(string(data))
	case os.IsNotExist(err) && len(facts) == 0:
		return nil
	case os.IsNotExist(err):
		file = newMarkdown(s.fileTitle(scope))
	default:
		return err
	}
	if !loose {
		claimLines(file, facts)
	}
	file.keep(want, loose)
	for _, f := range facts {
		section := sectionFacts
		if f.Status == protocol.MemoryReplaced {
			section = sectionReplaced
		}
		file.set(f.ID, f.Text, section)
	}
	out := file.String()
	if err == nil && out == string(data) {
		return s.recordStamp(path)
	}
	if err := writeFileAtomic(path, []byte(out)); err != nil {
		return err
	}
	return s.recordStamp(path)
}

// claimLines gives hand-written lines their new ids where they stand: a line
// without an id, or repeating an id an earlier line has, takes the id of the
// memory made from its text.
func claimLines(file *mdFile, facts []*fact) {
	placed := map[string]bool{}
	for _, line := range file.lines {
		if line.fact && line.id != "" {
			placed[line.id] = true
		}
	}
	unplaced := map[string][]string{}
	for _, f := range facts {
		if !placed[f.ID] {
			unplaced[f.Text] = append(unplaced[f.Text], f.ID)
		}
	}
	seen := map[string]bool{}
	for index, line := range file.lines {
		if !line.fact {
			continue
		}
		if line.id != "" && !seen[line.id] {
			seen[line.id] = true
			continue
		}
		text := cleanText(line.text)
		if ids := unplaced[text]; len(ids) > 0 {
			file.lines[index].id, file.lines[index].text = ids[0], text
			unplaced[text] = ids[1:]
			seen[ids[0]] = true
		}
	}
}

// fileTitle is the heading of a new memory file.
func (s *Store) fileTitle(scope string) string {
	return s.scopeName(scope)
}

// recordStamp keeps what a file looks like now the store has written or
// read it.
func (s *Store) recordStamp(path string) error {
	stamp, ok := stat(path)
	return s.db.Update(func(tx *bbolt.Tx) error {
		if !ok {
			delete(s.stamps, path)
			return tx.Bucket(bucketFiles).Delete([]byte(path))
		}
		s.stamps[path] = stamp
		return putJSON(tx, bucketFiles, path, stamp)
	})
}
