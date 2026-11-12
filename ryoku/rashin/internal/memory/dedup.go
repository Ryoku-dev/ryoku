package memory

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
	"strings"
	"time"
	"unicode"

	"go.etcd.io/bbolt"

	"ryoku-rashin/internal/protocol"
	"ryoku-rashin/internal/rank"
)

// tombstone keeps a forgotten fact from being learned again: the hash of its
// normalised text, its vector and a hash of the details it cannot lose
// (numbers, names, negation), never the text itself.
type tombstone struct {
	Scope  string    `json:"scope"`
	Hash   string    `json:"hash"`
	Marks  string    `json:"marks"`
	Vector []byte    `json:"vector,omitempty"`
	At     time.Time `json:"at"`
}

// newTombstone buries text in scope.
func newTombstone(scope, text string, vector []byte, at time.Time) tombstone {
	return tombstone{Scope: scope, Hash: hashNormalized(text), Marks: factMarks(text).hash(), Vector: vector, At: at}
}

// key is where the tombstone is kept: one per scope and text.
func (t tombstone) key() string { return t.Scope + "\x00" + t.Hash }

// sameThreshold is the cosine at or above which two facts of a scope may be
// the same one; potion's int8 vectors run lower than float ones.
const sameThreshold = 0.9

// buriedThreshold is the cosine at or above which a fact with the same
// numbers, names and negation as a forgotten one counts as it, reworded.
// potion scores paraphrases from about 0.55; a false hit only keeps back
// something close to what the user deleted.
const buriedThreshold = 0.55

// Portions adapted from Gentleman-Programming/engram internal/store/store.go:14072-14087
// @ a5d4fa7be7f7 (MIT, Copyright (c) 2026 Alan Buscaglia). Modified by Ryoku:
// punctuation at the ends of words is dropped too.

// hashNormalized hashes text with case, spacing and trailing punctuation
// folded, so a fact written again in other spacing is the same fact.
func hashNormalized(text string) string {
	words := strings.Fields(strings.ToLower(text))
	for index, word := range words {
		words[index] = strings.TrimFunc(word, unicode.IsPunct)
	}
	sum := sha256.Sum256([]byte(strings.Join(words, " ")))
	return hex.EncodeToString(sum[:])
}

// Portions adapted from vectorize-io/hindsight
// hindsight-api-slim/hindsight_api/engine/consolidation/consolidator.py:136-165,204-223
// @ 7cec128b8fdb (MIT, Copyright (c) 2025 Vectorize AI, Inc.). Modified by
// Ryoku: the merge-or-keep rule ("keep if they differ in any number, named
// entity or negation") is checked in Go instead of asked of a model.

// sameFact reports whether two close texts state the same fact: neither
// adds a number, a name or a negation the other lacks.
func sameFact(left, right string) bool {
	return factMarks(left).hash() == factMarks(right).hash()
}

// marks are the details a fact cannot lose: its numbers and names, and
// whether it is negated.
type marks struct {
	marks   map[string]bool
	negated bool
}

var negations = map[string]bool{"not": true, "no": true, "never": true, "don't": true, "doesn't": true, "isn't": true, "won't": true, "can't": true, "stopped": true, "nor": true, "without": true}

// factMarks finds the numbers, capitalised words after the first and
// negations of a text.
func factMarks(text string) marks {
	m := marks{marks: map[string]bool{}}
	for index, word := range strings.Fields(text) {
		clean := strings.TrimFunc(word, func(r rune) bool { return unicode.IsPunct(r) && r != '\'' })
		lower := strings.ToLower(clean)
		if negations[lower] || strings.HasSuffix(lower, "n't") {
			m.negated = true
		}
		if clean == "" {
			continue
		}
		first := []rune(clean)[0]
		if strings.ContainsFunc(clean, unicode.IsDigit) || index > 0 && unicode.IsUpper(first) {
			m.marks[lower] = true
		}
	}
	return m
}

// hash names the marks without keeping them.
func (m marks) hash() string {
	words := make([]string, 0, len(m.marks)+1)
	for mark := range m.marks {
		words = append(words, mark)
	}
	sort.Strings(words)
	if m.negated {
		words = append(words, "\x00not")
	}
	return hashHex(strings.Join(words, "\x00"))
}

// Duplicate finds an active or suggested memory of scope that states text
// again: the same normalised text, else a close vector with no detail
// differing. ok is false when text is new.
func (s *Store) Duplicate(ctx context.Context, text, scope string) (id string, ok bool, err error) {
	hash := hashNormalized(text)
	s.mu.Lock()
	var candidates []*fact
	for _, f := range s.facts {
		if f.Scope != scope || f.Status != protocol.MemoryActive && f.Status != protocol.MemorySuggested {
			continue
		}
		if hashNormalized(f.Text) == hash {
			s.mu.Unlock()
			return f.ID, true, nil
		}
		candidates = append(candidates, f)
	}
	s.mu.Unlock()
	if len(candidates) == 0 {
		return "", false, nil
	}
	vectors, err := s.embedFacts(ctx, []string{text})
	if err != nil {
		return "", false, nil
	}
	if err := s.ensureVectors(ctx, candidates); err != nil {
		return "", false, nil
	}
	best, bestScore := "", 0.0
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, f := range candidates {
		score := rank.Cosine(vectors[0], f.Vector)
		if score >= sameThreshold && score > bestScore && sameFact(text, f.Text) {
			best, bestScore = f.ID, score
		}
	}
	return best, best != "", nil
}

// Tombstoned reports whether text was forgotten in scope: the same
// normalised text, or a vector close enough to a forgotten one with the same
// numbers, names and negation.
func (s *Store) Tombstoned(ctx context.Context, text, scope string) (bool, error) {
	key := newTombstone(scope, text, nil, time.Time{}).key()
	marks := factMarks(text).hash()
	var vectors [][]byte
	found := false
	err := s.db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket(bucketTombs)
		if bucket.Get([]byte(key)) != nil {
			found = true
			return nil
		}
		prefix := scope + "\x00"
		c := bucket.Cursor()
		for k, v := c.Seek([]byte(prefix)); k != nil && strings.HasPrefix(string(k), prefix); k, v = c.Next() {
			var tomb tombstone
			if err := json.Unmarshal(v, &tomb); err != nil {
				return err
			}
			if len(tomb.Vector) > 0 && tomb.Marks == marks {
				vectors = append(vectors, tomb.Vector)
			}
		}
		return nil
	})
	if err != nil || found || len(vectors) == 0 {
		return found, err
	}
	query, err := s.embedFacts(ctx, []string{text})
	if err != nil {
		return false, nil
	}
	for _, vector := range vectors {
		if rank.Cosine(query[0], vector) >= buriedThreshold {
			return true, nil
		}
	}
	return false, nil
}

// Bury tombstones a fact forgotten outside the store, in an external
// engine, so it is not learned or added by a model again.
func (s *Store) Bury(text, scope string) error {
	tomb := newTombstone(scope, text, nil, s.now())
	if vectors, err := s.embedFacts(context.Background(), []string{text}); err == nil {
		tomb.Vector = rank.Quantize(vectors[0])
	}
	return s.db.Update(func(tx *bbolt.Tx) error { return putJSON(tx, bucketTombs, tomb.key(), tomb) })
}
